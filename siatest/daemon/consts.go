package daemon

import (
	"os"

	"go.thebigfile.com/bigd/persist"
	"go.thebigfile.com/bigd/siatest"
)

// daemonTestDir creates a temporary testing directory for daemon tests. This
// should only every be called once per test. Otherwise it will delete the
// directory again.
func daemonTestDir(testName string) string {
	path := siatest.TestDir("daemon", testName)
	if err := os.MkdirAll(path, persist.DefaultDiskPermissionsTest); err != nil {
		panic(err)
	}
	return path
}
