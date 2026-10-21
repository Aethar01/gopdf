//go:build windows

package viewer

/*
#cgo LDFLAGS: -lole32 -luuid -lgdi32
#include <stdlib.h>
void ononSetJumpList(const char *appID);
*/
import "C"

import "unsafe"

// setJumpList gives onon's taskbar button and Start entry a jump list of
// the documents opened lately and an Open File… task.
func setJumpList() {
	id := C.CString(appUserModelID)
	defer C.free(unsafe.Pointer(id))
	C.ononSetJumpList(id)
}

// taskbarButtons are the actions of the buttons taskbar_windows.c puts on
// the taskbar's thumbnail of the window, by their ids less one.
var taskbarButtons = []string{"prev_page", "next_page"}

//export ononTaskbarButton
func ononTaskbarButton(id C.int) {
	if i := int(id) - 1; i >= 0 && i < len(taskbarButtons) {
		chooseSystemAction(taskbarButtons[i])
	}
}
