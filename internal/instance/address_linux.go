//go:build linux

package instance

import (
	"fmt"
	"os"
	"path/filepath"
)

// runtimeDir prefers XDG_RUNTIME_DIR, which is already private to the user and
// cleared at logout. The temporary directory is shared by every user, so the
// fallback is namespaced by uid to avoid one user squatting another's path.
func runtimeDir() (string, error) {
	if dir := os.Getenv("XDG_RUNTIME_DIR"); dir != "" {
		return filepath.Join(dir, "gopdf"), nil
	}
	return filepath.Join(os.TempDir(), fmt.Sprintf("gopdf-%d", os.Getuid())), nil
}
