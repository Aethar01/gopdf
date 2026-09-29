//go:build windows

package viewer

import "syscall"

// sdlSymbol looks up a function in the SDL library, for the few the SDL
// bindings leave out.
func sdlSymbol(symbol string) (uintptr, error) {
	handle, err := syscall.LoadLibrary("SDL3.dll")
	if err != nil {
		return 0, err
	}
	return syscall.GetProcAddress(handle, symbol)
}
