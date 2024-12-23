package renter

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/cipher"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gitlab.com/NebulousLabs/errors"
	"gitlab.com/NebulousLabs/fastrand"
	"golang.org/x/crypto/twofish"

	"go.thebigfile.com/bigd/crypto"
	"go.thebigfile.com/bigd/modules"
	"go.thebigfile.com/bigd/modules/renter/filesystem"
	"go.thebigfile.com/bigd/modules/renter/filesystem/siadir"
)

// backupHeader defines the structure of the backup's JSON header.
type backupHeader struct {
	Version    string `json:"version"`
	Encryption string `json:"encryption"`
	IV         []byte `json:"iv"`
}

// The following specifiers are options for the encryption of backups.
var (
	encryptionPlaintext = "plaintext"
	encryptionTwofish   = "twofish-ctr"
	encryptionVersion   = "1.0"
)

// CreateBackup creates a backup of the renter's siafiles. If a secret is not
// nil, the backup will be encrypted using the provided secret.
func (r *Renter) CreateBackup(dst string, secret []byte) error {
	if err := r.tg.Add(); err != nil {
		return err
	}
	defer r.tg.Done()
	return r.managedCreateBackup(dst, secret)
}

// managedCreateBackup creates a backup of the renter's siafiles. If a secret is
// not nil, the backup will be encrypted using the provided secret.
func (r *Renter) managedCreateBackup(dst string, secret []byte) (err error) {
	// Create the gzip file.
	f, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Compose(err, f.Close())
	}()
	archive := io.Writer(f)

	// Prepare a header for the backup and default to no encryption. This will
	// potentially be overwritten later.
	bh := backupHeader{
		Version:    encryptionVersion,
		Encryption: encryptionPlaintext,
	}

	// Wrap it for encryption if required.
	if secret != nil {
		bh.Encryption = encryptionTwofish
		bh.IV = fastrand.Bytes(twofish.BlockSize)
		c, err := twofish.NewCipher(secret)
		if err != nil {
			return err
		}
		sw := cipher.StreamWriter{
			S: cipher.NewCTR(c, bh.IV),
			W: archive,
		}
		archive = sw
	}

	// Skip the checkum for now.
	if _, err := f.Seek(crypto.HashSize, io.SeekStart); err != nil {
		return err
	}
	// Write the header.
	enc := json.NewEncoder(f)
	if err := enc.Encode(bh); err != nil {
		return err
	}
	// Wrap the archive in a multiwriter to hash the contents of the archive
	// before encrypting it.
	h := crypto.NewHash()
	archive = io.MultiWriter(archive, h)
	// Wrap the potentially encrypted writer into a gzip writer.
	gzw := gzip.NewWriter(archive)
	// Wrap the gzip writer into a tar writer.
	tw := tar.NewWriter(gzw)
	// Add the files to the archive.
	if err := r.managedTarSiaFiles(tw); err != nil {
		twErr := tw.Close()
		gzwErr := gzw.Close()
		return errors.Compose(err, twErr, gzwErr)
	}
	// Close tar writer to flush it before writing the allowance.
	twErr := tw.Close()
	// Write the allowance.
	allowanceBytes, err := json.Marshal(r.hostContractor.Allowance())
	if err != nil {
		gzwErr := gzw.Close()
		return errors.Compose(err, twErr, gzwErr)
	}
	_, err = gzw.Write(allowanceBytes)
	if err != nil {
		gzwErr := gzw.Close()
		return errors.Compose(err, twErr, gzwErr)
	}
	// Close the gzip writer to flush it.
	gzwErr := gzw.Close()
	// Write the hash to the beginning of the file.
	_, err = f.WriteAt(h.Sum(nil), 0)
	return errors.Compose(err, twErr, gzwErr)
}

// LoadBackup loads the siafiles of a previously created backup into the
// renter. If the backup is encrypted, secret will be used to decrypt it.
// Otherwise the argument is ignored.
func (r *Renter) LoadBackup(src string, secret []byte) (err error) {
	if err := r.tg.Add(); err != nil {
		return err
	}
	defer r.tg.Done()

	// Only load a backup if there are no siafiles yet.
	root, err := r.staticFileSystem.OpenSiaDir(modules.UserFolder)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Compose(err, root.Close())
	}()

	// Open the gzip file.
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Compose(err, f.Close())
	}()
	archive := io.Reader(f)

	// Read the checksum.
	var chks crypto.Hash
	_, err = io.ReadFull(f, chks[:])
	if err != nil {
		return err
	}
	// Read the header.
	dec := json.NewDecoder(archive)
	var bh backupHeader
	if err := dec.Decode(&bh); err != nil {
		return err
	}
	// Check the version number.
	if bh.Version != encryptionVersion {
		return errors.New("unknown version")
	}
	// Wrap the file in the correct streamcipher. Consider the data remaining in
	// the decoder's buffer by using a multireader.
	archive = io.MultiReader(dec.Buffered(), archive)
	_, err = archive.Read(make([]byte, 1)) // Ignore first byte of buffer to get to the body of the backup
	if err != nil {
		return err
	}
	archive, err = wrapReaderInCipher(io.MultiReader(archive, f), bh, secret)
	if err != nil {
		return err
	}
	// Pipe the remaining file into the hasher to verify that the hash is
	// correct.
	h := crypto.NewHash()
	n, err := io.Copy(h, archive)
	if err != nil {
		return err
	}
	// Verify the hash.
	if !bytes.Equal(h.Sum(nil), chks[:]) {
		return errors.New("checksum doesn't match")
	}
	// Seek back to the beginning of the body.
	if _, err := f.Seek(-n, io.SeekCurrent); err != nil {
		return err
	}
	// Wrap the file again.
	archive, err = wrapReaderInCipher(f, bh, secret)
	if err != nil {
		return err
	}
	// Wrap the potentially encrypted reader in a gzip reader.
	gzr, err := gzip.NewReader(archive)
	if err != nil {
		return err
	}
	defer func() {
		err = errors.Compose(err, gzr.Close())
	}()
	// Wrap the gzip reader in a tar reader.
	tr := tar.NewReader(gzr)
	// Untar the files.
	if err := r.managedUntarDir(tr); err != nil {
		return errors.AddContext(err, "failed to untar dir")
	}
	// Unmarshal the allowance if available. This needs to happen after adding
	// decryption and confirming the hash but before adding decompression.
	dec = json.NewDecoder(gzr)
	var allowance modules.Allowance
	if err := dec.Decode(&allowance); err != nil {
		// legacy backup without allowance
		r.log.Println("WARN: Decoding the backup's allowance failed: ", err)
	}
	// If the backup contained a valid allowance and we currently don't have an
	// allowance set, import it.
	if !reflect.DeepEqual(allowance, modules.Allowance{}) &&
		reflect.DeepEqual(r.hostContractor.Allowance(), modules.Allowance{}) {
		if err := r.hostContractor.SetAllowance(allowance); err != nil {
			return errors.AddContext(err, "unable to set allowance from backup")
		}
	}
	return nil
}

// managedTarSiaFiles creates a tarball from the renter's siafiles and writes
// it to dst.
func (r *Renter) managedTarSiaFiles(tw *tar.Writer) error {
	// Walk over all the siafiles in in the user's home and add them to the
	// tarball.
	return r.staticFileSystem.Walk(modules.UserFolder, func(path string, info os.FileInfo, statErr error) (err error) {
		// This error is non-nil if filepath.Walk couldn't stat a file or
		// folder.
		if statErr != nil {
			return statErr
		}
		// Nothing to do for non-folders and non-siafiles.
		if !info.IsDir() && filepath.Ext(path) != modules.SiaFileExtension &&
			filepath.Ext(path) != modules.SiaDirExtension {
			return nil
		}
		// Create the header for the file/dir.
		header, err := tar.FileInfoHeader(info, info.Name())
		if err != nil {
			return err
		}
		relPath := strings.TrimPrefix(path, r.staticFileSystem.DirPath(modules.UserFolder))
		header.Name = relPath
		// If the info is a dir there is nothing more to do besides writing the
		// header.
		if info.IsDir() {
			return tw.WriteHeader(header)
		}
		// Handle siafiles and siadirs differently.
		var file io.Reader
		if filepath.Ext(path) == modules.SiaFileExtension {
			// Get the siafile.
			siaPath, err := modules.UserFolder.Join(strings.TrimSuffix(relPath, modules.SiaFileExtension))
			if err != nil {
				return err
			}
			entry, err := r.staticFileSystem.OpenSiaFile(siaPath)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Compose(err, entry.Close())
			}()
			// Get a reader to read from the siafile.
			sr, err := entry.SnapshotReader()
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Compose(err, sr.Close())
			}()
			file = sr
			// Update the size of the file within the header since it might have changed
			// while we weren't holding the lock.
			fi, err := sr.Stat()
			if err != nil {
				return err
			}
			header.Size = fi.Size()
		} else if filepath.Ext(path) == modules.SiaDirExtension {
			// Get the siadir.
			var siaPath modules.SiaPath
			siaPathStr := strings.TrimSuffix(relPath, modules.SiaDirExtension)
			if siaPathStr == string(filepath.Separator) {
				siaPath = modules.UserFolder
			} else {
				siaPath, err = modules.UserFolder.Join(siaPathStr)
				if err != nil {
					return err
				}
			}
			entry, err := r.staticFileSystem.OpenSiaDir(siaPath)
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Compose(err, entry.Close())
			}()
			// Get a reader to read from the siadir.
			dr, err := entry.DirReader()
			if err != nil {
				return err
			}
			defer func() {
				err = errors.Compose(err, dr.Close())
			}()
			file = dr
			// Update the size of the file within the header since it might have changed
			// while we weren't holding the lock.
			fi, err := dr.Stat()
			if err != nil {
				return err
			}
			header.Size = fi.Size()
		}
		// Write the header.
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		// Add the file to the archive.
		_, err = io.Copy(tw, file)
		return err
	})
}

// managedUntarDir untars the archive from src and writes the contents to dstFolder
// while preserving the relative paths within the archive.
func (r *Renter) managedUntarDir(tr *tar.Reader) (err error) {
	// dirsToUpdate are all the directories that will need bubble to be called
	// on them so that the renter's directory metadata from the back up is
	// updated
	dirsToUpdate := r.newUniqueRefreshPaths()
	defer func() {
		err = errors.Compose(err, dirsToUpdate.callRefreshAll())
	}()

	// Copy the files from the tarball to the new location.
	dir := r.staticFileSystem.DirPath(modules.UserFolder)
	for {
		header, err := tr.Next()
		if errors.Contains(err, io.EOF) {
			break
		} else if err != nil {
			return errors.AddContext(err, "could not get next entry in the tar archive")
		}

		// nolint:gosec // Disable gosec for this line since directory traversal
		// is checked below.
		dst := filepath.Join(dir, header.Name)

		// Check for directory traversal.
		if header.Name != "" && !strings.HasPrefix(dst, filepath.Clean(dir)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", dst)
		}

		// Check for dir.
		info := header.FileInfo()
		if info.IsDir() {
			if err = os.MkdirAll(dst, info.Mode()); err != nil {
				return errors.AddContext(err, fmt.Sprintf("could not make directory %v", dst))
			}
			continue
		}
		// Load the new file in memory.
		b, err := ioutil.ReadAll(tr)
		if err != nil {
			return errors.AddContext(err, "could not load the new file in memory")
		}
		if name := filepath.Base(info.Name()); name == modules.SiaDirExtension {
			// Verify there is enough data for a checksum
			if len(b) < crypto.HashSize {
				return siadir.ErrCorruptFile
			}

			// Verify checksum
			checksum := b[:crypto.HashSize]
			mdBytes := b[crypto.HashSize:]
			fileChecksum := crypto.HashBytes(mdBytes)
			if !bytes.Equal(checksum, fileChecksum[:]) {
				return siadir.ErrInvalidChecksum
			}
			// Load the file as a .siadir
			var md siadir.Metadata
			err = json.Unmarshal(mdBytes, &md)
			if err != nil {
				return errors.AddContext(err, "could not unmarshal")
			}
			// Try creating a new SiaDir.
			var siaPath modules.SiaPath
			if err := siaPath.LoadSysPath(r.staticFileSystem.DirPath(modules.UserFolder), dst); err != nil {
				return errors.AddContext(err, "could not load system path")
			}
			siaPath, err = siaPath.Dir()
			if err != nil {
				return errors.AddContext(err, "could not get directory")
			}
			err := r.staticFileSystem.NewSiaDir(siaPath, modules.DefaultDirPerm)
			if errors.Contains(err, filesystem.ErrExists) {
				// .siadir exists already
				continue
			} else if err != nil {
				// unexpected error
				return errors.AddContext(err, fmt.Sprintf("could not create dir at  %v", siaPath))
			}
			// Update the metadata.
			dirEntry, err := r.staticFileSystem.OpenSiaDir(siaPath)
			if err != nil {
				return errors.AddContext(err, fmt.Sprintf("could not open dir at %v", siaPath))
			}
			if err := dirEntry.UpdateMetadata(md); err != nil {
				dirEntry.Close()
				return errors.AddContext(err, "could not update metadata")
			}
			// Metadata was updated so add to list of directories to be updated
			err = dirsToUpdate.callAdd(siaPath)
			if err != nil {
				return errors.AddContext(err, fmt.Sprintf("could not add directory %v to the list of directories to be updated", siaPath))
			}
			// Close Directory
			dirEntry.Close()
		} else if filepath.Ext(info.Name()) == modules.SiaFileExtension {
			// Add the file to the SiaFileSet.
			reader := bytes.NewReader(b)
			siaPath, err := modules.UserFolder.Join(strings.TrimSuffix(header.Name, modules.SiaFileExtension))
			if err != nil {
				return errors.AddContext(err, "could not join folders")
			}
			err = r.staticFileSystem.AddSiaFileFromReader(reader, siaPath)
			if err != nil {
				return errors.AddContext(err, "could not add siafile from reader")
			}
			// Add directory that siafile resides in to the list of directories
			// to be updated
			err = dirsToUpdate.callAdd(siaPath)
			if err != nil {
				return errors.AddContext(err, fmt.Sprintf("could not add directory %v to the list of directories to be updated", siaPath))
			}
		}
	}
	return nil
}

// wrapReaderInCipher wraps the reader r into another reader according to the
// used encryption specified in the backupHeader.
func wrapReaderInCipher(r io.Reader, bh backupHeader, secret []byte) (io.Reader, error) {
	// Check if encryption is required and wrap the archive into a cipher if
	// necessary.
	switch bh.Encryption {
	case encryptionTwofish:
		c, err := twofish.NewCipher(secret)
		if err != nil {
			return nil, err
		}
		return cipher.StreamReader{
			S: cipher.NewCTR(c, bh.IV),
			R: r,
		}, nil
	case encryptionPlaintext:
		return r, nil
	default:
		return nil, errors.New("unknown cipher")
	}
}
