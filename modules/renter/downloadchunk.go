package renter

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/NebulousLabs/errors"

	"go.thebigfile.com/bigd/build"
	"go.thebigfile.com/bigd/crypto"
	"go.thebigfile.com/bigd/modules"
	"go.thebigfile.com/bigd/modules/renter/filesystem/siafile"
)

// downloadPieceInfo contains all the information required to download and
// recover a piece of a chunk from a host. It is a value in a map where the key
// is the file contract id.
type downloadPieceInfo struct {
	index uint64
	root  crypto.Hash
}

// unfinishedDownloadChunk contains a chunk for a download that is in progress.
//
// TODO: Currently, if a standby worker is needed, all of the standby workers
// are added and the first one that is available will pick up the slack. But,
// depending on the situation, we may only want to add a handful of workers to
// make sure that a fast / optimal worker is initially able to pick up the
// slack. This could potentially be streamlined by turning the standby array
// into a standby heap, and then having some general scoring system for figuring
// out how useful a worker is, and then having some threshold that a worker
// needs to be pulled from standby to work on the download. That threshold
// should go up every time that a worker fails, to make sure that if you have
// repeated failures, you keep pulling in the fresh workers instead of getting
// stuck and always rejecting all the standby workers.
type unfinishedDownloadChunk struct {
	// Fetch + Write instructions - read only or otherwise thread safe.
	destination downloadDestination // Where to write the recovered logical chunk.
	erasureCode modules.ErasureCoder
	masterKey   crypto.CipherKey

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticChunkIndex  uint64                       // Required for deriving the encryption keys for each piece.
	staticCacheID     string                       // Used to uniquely identify a chunk in the chunk cache.
	staticChunkMap    map[string]downloadPieceInfo // Maps from host PubKey to the info for the piece associated with that host
	staticChunkSize   uint64
	staticFetchLength uint64 // Length within the logical chunk to fetch.
	staticFetchOffset uint64 // Offset within the logical chunk that is being downloaded.
	staticPieceSize   uint64
	staticWriteOffset int64 // Offset within the writer to write the completed data.

	// Spending details.
	staticSpendingCategory spendingCategory

	// Fetch + Write instructions - read only or otherwise thread safe.
	staticDisableDiskFetch bool
	staticLatencyTarget    time.Duration
	staticNeedsMemory      bool // Set to true if memory was not pre-allocated for this chunk.
	staticMemoryManager    *memoryManager
	staticOverdrive        int
	staticPriority         uint64

	// Download chunk state - need mutex to access.
	completedPieces   []bool    // Which pieces were downloaded successfully.
	failed            bool      // Indicates if the chunk has been marked as failed.
	physicalChunkData [][]byte  // Used to recover the logical data.
	pieceUsage        []bool    // Which pieces are being actively fetched.
	piecesCompleted   int       // Number of pieces that have successfully completed.
	piecesRegistered  int       // Number of pieces that workers are actively fetching.
	recoveryComplete  bool      // Whether or not the recovery has completed and the chunk memory released.
	workersRemaining  int       // Number of workers still able to fetch the chunk.
	workersStandby    []*worker // Set of workers that are able to work on this download, but are not needed unless other workers fail.

	// Memory management variables.
	memoryAllocated uint64

	// The download object, mostly to update download progress.
	download *download
	mu       sync.Mutex

	// The SiaFile from which data is being downloaded.
	renterFile *siafile.Snapshot
}

// fail will set the chunk status to failed. The physical chunk memory will be
// wiped and any memory allocation will be returned to the renter. The download
// as a whole will be failed as well.
func (udc *unfinishedDownloadChunk) fail(err error) {
	udc.failed = true
	udc.recoveryComplete = true
	for i := range udc.physicalChunkData {
		udc.physicalChunkData[i] = nil
	}
	udc.download.managedFail(fmt.Errorf("chunk %v failed: %v", udc.staticChunkIndex, err))
	udc.destination = nil
}

// managedCleanUp will check if the download has failed, and if not it will add
// any standby workers which need to be added. Calling managedCleanUp too many
// times is not harmful, however missing a call to managedCleanUp can lead to
// dealocks.
func (udc *unfinishedDownloadChunk) managedCleanUp() {
	// Check if the chunk is newly failed.
	udc.mu.Lock()
	if udc.workersRemaining+udc.piecesCompleted < udc.erasureCode.MinPieces() && !udc.failed {
		str := fmt.Sprintf("workers remaining %v, pieces completed %v, min pieces %v", udc.workersRemaining, udc.piecesCompleted, udc.erasureCode.MinPieces())
		udc.fail(errors.AddContext(errNotEnoughWorkers, str))
	}
	// Return any excess memory.
	udc.returnMemory()

	// Nothing to do if the chunk has failed.
	if udc.failed {
		udc.mu.Unlock()
		return
	}

	// Check whether standby workers are required.
	chunkComplete := udc.piecesCompleted >= udc.erasureCode.MinPieces()
	desiredPiecesRegistered := udc.erasureCode.MinPieces() + udc.staticOverdrive - udc.piecesCompleted
	standbyWorkersRequired := !chunkComplete && udc.piecesRegistered < desiredPiecesRegistered
	if !standbyWorkersRequired {
		udc.mu.Unlock()
		return
	}

	// Assemble a list of standby workers, release the udc lock, and then queue
	// the chunk into the workers. The lock needs to be released early because
	// holding the udc lock and the worker lock at the same time is a deadlock
	// risk (they interact with eachother, call functions on eachother).
	var standbyWorkers []*worker
	for i := 0; i < len(udc.workersStandby); i++ {
		standbyWorkers = append(standbyWorkers, udc.workersStandby[i])
	}
	udc.workersStandby = udc.workersStandby[:0] // Workers have been taken off of standby.
	udc.mu.Unlock()
	for i := 0; i < len(standbyWorkers); i++ {
		go standbyWorkers[i].threadedPerformDownloadChunkJob(udc)
	}
}

// managedFinalizeRecovery sets recoveryComplete to 'true' and also marks
// the download as complete if there are no more chunks remaining.
func (udc *unfinishedDownloadChunk) managedFinalizeRecovery() {
	// Directly nil out the physical chunk data, it's not going to be used
	// anymore. Also signal that data recovery has completed.
	udc.mu.Lock()
	udc.physicalChunkData = nil
	udc.recoveryComplete = true
	udc.mu.Unlock()

	// Update the download and signal completion of this chunk.
	udc.download.mu.Lock()
	defer udc.download.mu.Unlock()
	udc.download.chunksRemaining--
	if udc.download.chunksRemaining == 0 {
		// Download is complete, send out a notification.
		udc.download.markComplete()
	}
}

// managedRemoveWorker will decrement a worker from the set of remaining workers
// in the udc. After a worker has been removed, the udc needs to be cleaned up.
func (udc *unfinishedDownloadChunk) managedRemoveWorker() {
	udc.mu.Lock()
	udc.workersRemaining--
	udc.mu.Unlock()
	udc.managedCleanUp()
}

// markPieceCompleted marks the piece with pieceIndex as completed.
func (udc *unfinishedDownloadChunk) markPieceCompleted(pieceIndex uint64) {
	udc.completedPieces[pieceIndex] = true
	udc.piecesCompleted++

	// Sanity check to make sure the slice and counter are consistent.
	if !build.DEBUG {
		return
	}
	completed := 0
	for _, b := range udc.completedPieces {
		if b {
			completed++
		}
	}
	if completed != udc.piecesCompleted {
		build.Critical(fmt.Sprintf("pieces completed and completedPieces out of sync %v != %v",
			completed, udc.piecesCompleted))
	}
}

// returnMemory will check on the status of all the workers and pieces, and
// determine how much memory is safe to return to the renter. This should be
// called each time a worker returns, and also after the chunk is recovered.
func (udc *unfinishedDownloadChunk) returnMemory() {
	// The maximum amount of memory is the pieces completed plus the number of
	// workers remaining.
	maxMemory := uint64(udc.workersRemaining+udc.piecesCompleted) * udc.staticPieceSize
	// If enough pieces have completed, max memory is the number of registered
	// pieces plus the number of completed pieces.
	if udc.piecesCompleted >= udc.erasureCode.MinPieces() {
		// udc.piecesRegistered is guaranteed to be at most equal to the number
		// of overdrive pieces, meaning it will be equal to or less than
		// initialMemory.
		maxMemory = uint64(udc.piecesCompleted+udc.piecesRegistered) * udc.staticPieceSize
	}
	// If the chunk recovery has completed, the maximum number of pieces is the
	// number of registered.
	if udc.recoveryComplete {
		maxMemory = uint64(udc.piecesRegistered) * udc.staticPieceSize
	}
	// Return any memory we don't need.
	if uint64(udc.memoryAllocated) > maxMemory {
		udc.staticMemoryManager.Return(udc.memoryAllocated - maxMemory)
		udc.memoryAllocated = maxMemory
	}
}

// threadedRecoverLogicalData will take all of the pieces that have been
// downloaded and encode them into the logical data which is then written to the
// underlying writer for the download.
func (udc *unfinishedDownloadChunk) threadedRecoverLogicalData() error {
	// Ensure cleanup occurs after the data is recovered, whether recovery
	// succeeds or fails.
	defer udc.managedCleanUp()

	// Write the pieces to the requested output.
	dataOffset := recoveredDataOffset(udc.staticFetchOffset, udc.erasureCode)
	err := udc.destination.WritePieces(udc.erasureCode, udc.physicalChunkData, dataOffset, udc.staticWriteOffset, udc.staticFetchLength)
	if err != nil {
		udc.mu.Lock()
		udc.fail(err)
		udc.mu.Unlock()
		return errors.AddContext(err, "unable to write to download destination")
	}
	// finalize the chunk.
	udc.managedFinalizeRecovery()
	return nil
}

// bytesToRecover returns the number of bytes we need to recover from the
// erasure coded segments. The number of bytes we need to recover doesn't
// always match the chunkFetchLength. e.g. a user might want to fetch 500 bytes
// from a segment that is 640 bytes large after recovery. Then the number of
// bytes to recover would be 640 instead of 500 and the 140 bytes we don't need
// would be discarded after recovery.
func bytesToRecover(chunkFetchOffset, chunkFetchLength, chunkSize uint64, rs modules.ErasureCoder) uint64 {
	// If partialDecoding is not available we downloaded the whole sector and
	// recovered the whole chunk.
	segmentSize, supportsPartial := rs.SupportsPartialEncoding()
	if !supportsPartial {
		return chunkSize
	}
	// Else we need to calculate how much data we need to recover.
	recoveredSegmentSize := uint64(rs.MinPieces()) * segmentSize
	_, numSegments := segmentsForRecovery(chunkFetchOffset, chunkFetchLength, rs)
	return numSegments * recoveredSegmentSize
}

// recoveredDataOffset translates the fetch offset of the chunk into the offset
// within the recovered data.
func recoveredDataOffset(chunkFetchOffset uint64, rs modules.ErasureCoder) uint64 {
	// If partialDecoding is not available we downloaded the whole sector and
	// recovered the whole chunk which means the offset and length are actually
	// equal to the chunkFetchOffset and chunkFetchLength.
	segmentSize, supportsPartial := rs.SupportsPartialEncoding()
	if !supportsPartial {
		return chunkFetchOffset
	}
	// Else we need to adjust the offset a bit.
	recoveredSegmentSize := uint64(rs.MinPieces()) * segmentSize
	return chunkFetchOffset % recoveredSegmentSize
}
