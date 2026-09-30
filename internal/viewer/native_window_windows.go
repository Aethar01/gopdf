//go:build windows

package viewer

import (
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/sys/windows"
)

var (
	dwmapi                           = windows.NewLazySystemDLL("dwmapi.dll")
	procDwmExtendFrameIntoClientArea = dwmapi.NewProc("DwmExtendFrameIntoClientArea")
)

// dwmMargins is the MARGINS of DwmExtendFrameIntoClientArea.
type dwmMargins struct {
	left, right, top, bottom int32
}

const dwmCornerRound = 2 // DWMWCP_ROUND

// configureNativeWindow drops the system title bar, keeping the window's
// frame for snapping and the system menu, and draws gopdf's own controls
// in its place.
func (a *App) configureNativeWindow(window *sdl.Window) {
	if window == nil || !sdl.SetWindowBordered(window, false) {
		return
	}
	a.titleBar = &titleBar{}
	// The binding makes a callback on every call, and never frees one.
	sdl.SetWindowHitTest(window, func(win *sdl.Window, point *sdl.Point, _ unsafe.Pointer) sdl.HitTestResult {
		density := float64(sdl.GetWindowPixelDensity(win))
		if density <= 0 {
			density = 1
		}
		return a.titleBarHitTest(float64(point.X)*density, float64(point.Y)*density)
	}, nil)

	properties := sdl.GetWindowProperties(window)
	hwnd := windows.HWND(uintptr(sdl.GetPointerProperty(properties, sdl.PropWindowWin32HwndPointer, nil)))
	if hwnd == 0 {
		return
	}
	// Extending the frame into the window brings back the shadow a window
	// without a frame loses. Windows 10 has no rounded corners and fails
	// the second call, which is fine.
	margins := dwmMargins{top: 1}
	procDwmExtendFrameIntoClientArea.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&margins)))
	corner := uint32(dwmCornerRound)
	_ = windows.DwmSetWindowAttribute(hwnd, windows.DWMWA_WINDOW_CORNER_PREFERENCE, unsafe.Pointer(&corner), uint32(unsafe.Sizeof(corner)))
}
