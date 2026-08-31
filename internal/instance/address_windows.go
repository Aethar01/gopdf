//go:build windows

package instance

import (
	"os"
	"path/filepath"
)

// runtimeDir uses the per-user local application data directory. Windows has
// supported Unix domain sockets since version 1803; on older releases Listen
// fails and single-instance support is simply unavailable.
func runtimeDir() (string, error) {
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		return filepath.Join(dir, "gopdf"), nil
	}
	return filepath.Join(os.TempDir(), "gopdf"), nil
}
