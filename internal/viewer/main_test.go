package viewer

import (
	"fmt"
	"os"
	"testing"

	"gopdf/internal/config"
)

// TestMain runs the tests with the user's directories pointed at a
// temporary one, so no test reads or writes the real configuration or
// session database.
func TestMain(m *testing.M) {
	os.Exit(runIsolated(m))
}

func runIsolated(m *testing.M) int {
	dir, err := os.MkdirTemp("", "gopdf-viewer-test-")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	defer os.RemoveAll(dir)
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "APPDATA", "LOCALAPPDATA"} {
		os.Setenv(name, dir)
	}
	os.Unsetenv("XDG_CONFIG_DIRS")
	defer config.CloseSessionDatabase()
	return m.Run()
}
