package compatibility

// siag.go checks that any changes made to the code retain compatibility with
// old versions of siag.

import (
	"errors"
	"path/filepath"
	"strconv"
	"testing"

	"gitlab.com/NebulousLabs/encoding"
	"go.thebigfile.com/bigd/crypto"
	"go.thebigfile.com/bigd/types"
)

// KeyPairSiag_1_0 matches the KeyPair struct of the siag 1.0 code.
type KeyPairSiag_1_0 struct {
	Header           string
	Version          string
	Index            int
	SecretKey        crypto.SecretKey
	UnlockConditions types.UnlockConditions
}

// verifyKeysSiag_1_0 is a copy-pasted version of the verifyKeys method
// from siag 1.0.
func verifyKeysSiag_1_0(uc types.UnlockConditions, folder string, keyname string) error {
	keysRequired := uc.SignaturesRequired
	totalKeys := uint64(len(uc.PublicKeys))
	loadedKeys := make([]KeyPairSiag_1_0, totalKeys)
	for i := 0; i < len(loadedKeys); i++ {
		err := encoding.ReadFile(filepath.Join(folder, keyname+"_Key"+strconv.Itoa(i)+".siakey"), &loadedKeys[i])
		if err != nil {
			return err
		}
	}
	for _, loadedKey := range loadedKeys {
		if loadedKey.UnlockConditions.UnlockHash() != uc.UnlockHash() {
			return errors.New("ErrCorruptedKey")
		}
	}
	txn := types.Transaction{
		SiafundInputs: []types.SiafundInput{
			{
				UnlockConditions: loadedKeys[0].UnlockConditions,
			},
		},
	}
	var i uint64
	for i != totalKeys {
		if i+keysRequired > totalKeys {
			i = totalKeys - keysRequired
		}
		var j uint64
		for j < keysRequired {
			txn.TransactionSignatures = append(txn.TransactionSignatures, types.TransactionSignature{
				PublicKeyIndex: i,
				CoveredFields:  types.CoveredFields{WholeTransaction: true},
			})
			sigHash := txn.SigHash(int(j), 0)
			sig := crypto.SignHash(sigHash, loadedKeys[i].SecretKey)
			txn.TransactionSignatures[j].Signature = sig[:]
			i++
			j++
		}
		err := txn.StandaloneValid(0)
		if err != nil {
			return err
		}
		txn.TransactionSignatures = nil
	}
	return nil
}

// TestVerifyKeysSiag_1_0 loads some keys generated by siag1.0.
// Verification must still work.
func TestVerifyKeysSiag_1_0(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	var kp KeyPairSiag_1_0

	// 1 of 1
	err := encoding.ReadFile("siag_1.0_1of1_Key0.siakey", &kp)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyKeysSiag_1_0(kp.UnlockConditions, "", "siag_1.0_1of1")
	if err != nil {
		t.Fatal(err)
	}

	// 1 of 2
	err = encoding.ReadFile("siag_1.0_1of2_Key0.siakey", &kp)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyKeysSiag_1_0(kp.UnlockConditions, "", "siag_1.0_1of2")
	if err != nil {
		t.Fatal(err)
	}

	// 2 of 3
	err = encoding.ReadFile("siag_1.0_2of3_Key0.siakey", &kp)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyKeysSiag_1_0(kp.UnlockConditions, "", "siag_1.0_2of3")
	if err != nil {
		t.Fatal(err)
	}

	// 3 of 3
	err = encoding.ReadFile("siag_1.0_3of3_Key0.siakey", &kp)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyKeysSiag_1_0(kp.UnlockConditions, "", "siag_1.0_3of3")
	if err != nil {
		t.Fatal(err)
	}

	// 4 of 9
	err = encoding.ReadFile("siag_1.0_4of9_Key0.siakey", &kp)
	if err != nil {
		t.Fatal(err)
	}
	err = verifyKeysSiag_1_0(kp.UnlockConditions, "", "siag_1.0_4of9")
	if err != nil {
		t.Fatal(err)
	}
}
