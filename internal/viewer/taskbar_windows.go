//go:build windows

package viewer

/*
#cgo LDFLAGS: -lole32 -luuid -lgdi32
*/
import "C"

// taskbarButtons are the actions of the buttons taskbar_windows.c puts on
// the taskbar's thumbnail of the window, by their ids less one.
var taskbarButtons = []string{"prev_page", "next_page"}

//export gopdfTaskbarButton
func gopdfTaskbarButton(id C.int) {
	if i := int(id) - 1; i >= 0 && i < len(taskbarButtons) {
		chooseSystemAction(taskbarButtons[i])
	}
}
