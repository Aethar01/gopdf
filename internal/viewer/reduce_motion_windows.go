//go:build windows

package viewer

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

var procSystemParametersInfo = windows.NewLazySystemDLL("user32.dll").NewProc("SystemParametersInfoW")

const spiGetClientAreaAnimation = 0x1042 // SPI_GETCLIENTAREAANIMATION

// osReducesMotion reports whether Windows' "Animation effects" setting
// is off.
func osReducesMotion() bool {
	var animate int32
	ok, _, _ := procSystemParametersInfo.Call(spiGetClientAreaAnimation, 0, uintptr(unsafe.Pointer(&animate)), 0)
	return ok != 0 && animate == 0
}
