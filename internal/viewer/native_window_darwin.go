//go:build darwin

package viewer

/*
#cgo pkg-config: sdl3
#cgo LDFLAGS: -framework Cocoa
void gopdfConfigureMacOSWindow(void *window);
*/
import "C"

import (
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func configureNativeWindow(window *sdl.Window) {
	if window == nil {
		return
	}
	C.gopdfConfigureMacOSWindow(unsafe.Pointer(window))
}
