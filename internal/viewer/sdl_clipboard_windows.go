//go:build windows

package viewer

import (
	"syscall"

	"github.com/ebitengine/purego"
)

func loadSDLSetClipboardTextSymbol(dst *func(string) bool) error {
	handle, err := syscall.LoadLibrary("SDL3.dll")
	if err != nil {
		return err
	}
	sym, err := syscall.GetProcAddress(handle, "SDL_SetClipboardText")
	if err != nil {
		return err
	}
	purego.RegisterFunc(dst, sym)
	return nil
}
