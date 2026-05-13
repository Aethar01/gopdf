//go:build !windows

package viewer

import (
	"fmt"
	"runtime"

	"github.com/ebitengine/purego"
)

func loadSDLSetClipboardTextSymbol(dst *func(string) bool) error {
	var lastErr error
	for _, name := range sdlLibraryNames() {
		handle, err := purego.Dlopen(name, 1)
		if err != nil {
			lastErr = err
			continue
		}
		sym, err := purego.Dlsym(handle, "SDL_SetClipboardText")
		if err != nil {
			lastErr = err
			continue
		}
		purego.RegisterFunc(dst, sym)
		return nil
	}
	if lastErr != nil {
		return lastErr
	}
	return fmt.Errorf("symbol not found")
}

func sdlLibraryNames() []string {
	switch runtime.GOOS {
	case "darwin":
		return []string{"./libSDL3.dylib", "libSDL3.dylib"}
	default:
		return []string{"./libSDL3.so.0", "libSDL3.so.0"}
	}
}
