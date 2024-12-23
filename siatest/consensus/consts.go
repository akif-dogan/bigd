package consensus

import (
	"os"

	"go.thebigfile.com/bigd/persist"
	"go.thebigfile.com/bigd/siatest"
)

// consensusTestDir creates a temporary testing directory for a consensus. This
// should only every be called once per test. Otherwise it will delete the
// directory again.
func consensusTestDir(testName string) string {
	path := siatest.TestDir("consensus", testName)
	if err := os.MkdirAll(path, persist.DefaultDiskPermissionsTest); err != nil {
		panic(err)
	}
	return path
}
