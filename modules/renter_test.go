package modules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gitlab.com/NebulousLabs/fastrand"

	"go.thebigfile.com/bigd/build"
	"go.thebigfile.com/bigd/crypto"
	"go.thebigfile.com/bigd/persist"
	"go.thebigfile.com/bigd/types"
)

// TestMerkleRootSetCompatibility checks that the persist encoding for the
// MerkleRootSet type is compatible with the previous encoding for the data,
// which was a slice of type crypto.Hash.
func TestMerkleRootSetCompatibility(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// Create some fake headers for the files.
	meta := persist.Metadata{
		Header:  "Test Header",
		Version: "1.1.1",
	}

	// Try multiple sizes of array.
	for i := 0; i < 10; i++ {
		// Create a []crypto.Hash of length i.
		type chStruct struct {
			Hashes []crypto.Hash
		}
		var chs chStruct
		for j := 0; j < i; j++ {
			var ch crypto.Hash
			fastrand.Read(ch[:])
			chs.Hashes = append(chs.Hashes, ch)
		}

		// Save and load, check that they are the same.
		dir := build.TempDir("modules", t.Name())
		err := os.MkdirAll(dir, 0700)
		if err != nil {
			t.Fatal(err)
		}
		filename := filepath.Join(dir, "file")
		err = persist.SaveJSON(meta, chs, filename)
		if err != nil {
			t.Fatal(err)
		}

		// Load and verify equivalence.
		var loadCHS chStruct
		err = persist.LoadJSON(meta, &loadCHS, filename)
		if err != nil {
			t.Fatal(err)
		}
		if len(chs.Hashes) != len(loadCHS.Hashes) {
			t.Fatal("arrays should be the same size")
		}
		for j := range chs.Hashes {
			if chs.Hashes[j] != loadCHS.Hashes[j] {
				t.Error("loading failed", i, j)
			}
		}

		// Load into MerkleRootSet and verify equivalence.
		type mrStruct struct {
			Hashes MerkleRootSet
		}
		var loadMRS mrStruct
		err = persist.LoadJSON(meta, &loadMRS, filename)
		if err != nil {
			t.Fatal(err)
		}
		if len(chs.Hashes) != len(loadMRS.Hashes) {
			t.Fatal("arrays should be the same size")
		}
		for j := range chs.Hashes {
			if chs.Hashes[j] != loadMRS.Hashes[j] {
				t.Error("loading failed", i, j)
			}
		}

		// Save as a MerkleRootSet and verify it can be loaded again.
		var mrs mrStruct
		mrs.Hashes = MerkleRootSet(chs.Hashes)
		err = persist.SaveJSON(meta, mrs, filename)
		if err != nil {
			t.Fatal(err)
		}
		err = persist.LoadJSON(meta, &loadMRS, filename)
		if err != nil {
			t.Fatal(err)
		}
		if len(mrs.Hashes) != len(loadMRS.Hashes) {
			t.Fatal("arrays should be the same size")
		}
		for j := range mrs.Hashes {
			if mrs.Hashes[j] != loadMRS.Hashes[j] {
				t.Error("loading failed", i, j)
			}
		}
	}
}

// TestMaintenanceSpending_Add is a small unit test for the Add method on
// MaintenanceSpending
func TestMaintenanceSpending_Add(t *testing.T) {
	t.Parallel()

	x := MaintenanceSpending{}
	y := MaintenanceSpending{}
	sum := x.Add(y)
	if !sum.AccountBalanceCost.Equals(x.AccountBalanceCost) ||
		!sum.FundAccountCost.Equals(x.FundAccountCost) ||
		!sum.UpdatePriceTableCost.Equals(x.UpdatePriceTableCost) {
		t.Fatal("unexpected")
	}

	y = MaintenanceSpending{
		AccountBalanceCost:   types.NewCurrency64(1),
		FundAccountCost:      types.NewCurrency64(2),
		UpdatePriceTableCost: types.NewCurrency64(3),
	}

	// verify associative property
	sum = x.Add(y)
	if !sum.AccountBalanceCost.Equals64(1) ||
		!sum.FundAccountCost.Equals64(2) ||
		!sum.UpdatePriceTableCost.Equals64(3) {
		t.Fatal("unexpected")
	}
	sum = y.Add(x)
	if !sum.AccountBalanceCost.Equals64(1) ||
		!sum.FundAccountCost.Equals64(2) ||
		!sum.UpdatePriceTableCost.Equals64(3) {
		t.Fatal("unexpected")
	}
}

// TestMaintenanceSpending_Sum is a small unit test for the Sum method on
// MaintenanceSpending
func TestMaintenanceSpending_Sum(t *testing.T) {
	t.Parallel()

	ms := MaintenanceSpending{}
	if !ms.Sum().IsZero() {
		t.Fatal("unexpected")
	}

	ms = MaintenanceSpending{
		AccountBalanceCost:   types.NewCurrency64(1),
		FundAccountCost:      types.NewCurrency64(2),
		UpdatePriceTableCost: types.NewCurrency64(3),
	}
	if !ms.Sum().Equals64(6) {
		t.Fatal("unexpected")
	}
}

// BenchmarkMerkleRootSetEncode clocks how fast large MerkleRootSets can be
// encoded and written to disk.
func BenchmarkMerkleRootSetEncode(b *testing.B) {
	// Create a []crypto.Hash of length i.
	type chStruct struct {
		Hashes MerkleRootSet
	}
	var chs chStruct
	for i := 0; i < 1e3; i++ {
		var ch crypto.Hash
		fastrand.Read(ch[:])
		chs.Hashes = append(chs.Hashes, ch)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(chs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSliceCryptoHashEncode clocks how fast large []crypto.Hashes can be
// encoded and written to disk.
func BenchmarkSliceCryptoHashEncode(b *testing.B) {
	// Create a []crypto.Hash of length i.
	type chStruct struct {
		Hashes []crypto.Hash
	}
	var chs chStruct
	for i := 0; i < 1e3; i++ {
		var ch crypto.Hash
		fastrand.Read(ch[:])
		chs.Hashes = append(chs.Hashes, ch)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := json.Marshal(chs)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkMerkleRootSetSave clocks how fast large MerkleRootSets can be
// encoded and written to disk.
func BenchmarkMerkleRootSetSave(b *testing.B) {
	// Create some fake headers for the files.
	meta := persist.Metadata{
		Header:  "Bench Header",
		Version: "1.1.1",
	}

	// Create a []crypto.Hash of length i.
	type chStruct struct {
		Hashes MerkleRootSet
	}
	var chs chStruct
	for i := 0; i < 1e3; i++ {
		var ch crypto.Hash
		fastrand.Read(ch[:])
		chs.Hashes = append(chs.Hashes, ch)
	}

	// Save through the persist.
	dir := build.TempDir("modules", "BenchmarkSliceCryptoHashSave")
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		b.Fatal(err)
	}
	filename := filepath.Join(dir, "file")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = persist.SaveJSON(meta, chs, filename)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSliceCryptoHashSave clocks how fast large []crypto.Hashes can be
// encoded and written to disk.
func BenchmarkSliceCryptoHashSave(b *testing.B) {
	// Create some fake headers for the files.
	meta := persist.Metadata{
		Header:  "Bench Header",
		Version: "1.1.1",
	}

	// Create a []crypto.Hash of length i.
	type chStruct struct {
		Hashes []crypto.Hash
	}
	var chs chStruct
	for i := 0; i < 1e3; i++ {
		var ch crypto.Hash
		fastrand.Read(ch[:])
		chs.Hashes = append(chs.Hashes, ch)
	}

	// Save through the persist.
	dir := build.TempDir("modules", "BenchmarkSliceCryptoHashSave")
	err := os.MkdirAll(dir, 0700)
	if err != nil {
		b.Fatal(err)
	}
	filename := filepath.Join(dir, "file")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err = persist.SaveJSON(meta, chs, filename)
		if err != nil {
			b.Fatal(err)
		}
	}
}
