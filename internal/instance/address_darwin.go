//go:build darwin

package instance

import (
	"os"
	"path/filepath"
)

// runtimeDir uses the per-user temporary directory, which launchd sets to a
// 0700 path under /var/folders and clears periodically.
func runtimeDir() (string, error) {
	return filepath.Join(os.TempDir(), "gopdf"), nil
}
