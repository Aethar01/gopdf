//go:build darwin

package viewer

/*
#cgo LDFLAGS: -framework Cocoa
void gopdfConfigureMacOSWindow(void *window);
*/
import "C"

import "github.com/jupiterrider/purego-sdl3/sdl"

func configureNativeWindow(window *sdl.Window) {
	if window == nil {
		return
	}
	properties := sdl.GetWindowProperties(window)
	nativeWindow := sdl.GetPointerProperty(properties, sdl.PropWindowCocoaWindowPointer, nil)
	if nativeWindow == nil {
		return
	}
	C.gopdfConfigureMacOSWindow(nativeWindow)
}
