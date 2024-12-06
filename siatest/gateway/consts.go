package gateway

import (
	"os"

	"go.thebigfile.com/bigd/persist"
	"go.thebigfile.com/bigd/siatest"
)

// gatewayTestDir creates a temporary testing directory for a gateway. This
// should only every be called once per test. Otherwise it will delete the
// directory again.
func gatewayTestDir(testName string) string {
	path := siatest.TestDir("gateway", testName)
	if err := os.MkdirAll(path, persist.DefaultDiskPermissionsTest); err != nil {
		panic(err)
	}
	return path
}
