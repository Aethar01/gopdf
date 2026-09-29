//go:build !windows

package viewer

import (
	"fmt"
	"runtime"

	"github.com/ebitengine/purego"
)

// sdlSymbol looks up a function in the SDL library, for the few the SDL
// bindings leave out.
func sdlSymbol(symbol string) (uintptr, error) {
	var lastErr error
	for _, name := range sdlLibraryNames() {
		handle, err := purego.Dlopen(name, purego.RTLD_LAZY)
		if err != nil {
			lastErr = err
			continue
		}
		sym, err := purego.Dlsym(handle, symbol)
		if err != nil {
			lastErr = err
			continue
		}
		return sym, nil
	}
	if lastErr != nil {
		return 0, lastErr
	}
	return 0, fmt.Errorf("symbol not found")
}

func sdlLibraryNames() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"./libSDL3.dylib", "libSDL3.dylib"}
	default:
		return []string{"./libSDL3.so.0", "libSDL3.so.0"}
	}
}
