//go:build darwin

package viewer

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void gopdfConfigureMacOSWindow(void *window);
void gopdfSetWindowDocument(void *window, const char *path, int edited);
*/
import "C"

import (
	"image/color"
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) configureNativeWindow(window *sdl.Window) {
	if nativeWindow := cocoaWindow(window); nativeWindow != nil {
		C.gopdfConfigureMacOSWindow(nativeWindow)
	}
}

// setNativeWindowDocument gives the window the file it shows, for its
// proxy icon and path menu, and marks it edited while it has unsaved edits.
func setNativeWindowDocument(window *sdl.Window, path string, edited bool) {
	nativeWindow := cocoaWindow(window)
	if nativeWindow == nil {
		return
	}
	text := C.CString(path)
	defer C.free(unsafe.Pointer(text))
	flag := C.int(0)
	if edited {
		flag = 1
	}
	C.gopdfSetWindowDocument(nativeWindow, text, flag)
}

// setNativeWindowColors does nothing on macOS, whose hidden title bar
// leaves no frame to colour.
func setNativeWindowColors(window *sdl.Window, background, border color.RGBA) {}

func cocoaWindow(window *sdl.Window) unsafe.Pointer {
	if window == nil {
		return nil
	}
	properties := sdl.GetWindowProperties(window)
	return sdl.GetPointerProperty(properties, sdl.PropWindowCocoaWindowPointer, nil)
}
