//go:build darwin

package viewer

/*
#cgo LDFLAGS: -framework Cocoa
#include <stdlib.h>
void gopdfSetMenuBar(const char *menus);
*/
import "C"

import (
	"encoding/json"
	"fmt"
	"unsafe"

	"gopdf/internal/keys"
	"gopdf/internal/menubar"
)

// menuBarActions is the action each item in the menu bar runs, by the
// item's tag less one; a tag of 0 is a system item.
var menuBarActions []string

// cocoaMenu and cocoaItem are the menus as the Objective-C side builds them.
type cocoaMenu struct {
	Title string      `json:"title"`
	Role  string      `json:"role,omitempty"`
	Items []cocoaItem `json:"items"`
}

type cocoaItem struct {
	Title string `json:"title,omitempty"`
	Role  string `json:"role,omitempty"`
	Tag   int    `json:"tag,omitempty"`
	Key   string `json:"key,omitempty"`
	Cmd   bool   `json:"cmd,omitempty"`
	Ctrl  bool   `json:"ctrl,omitempty"`
	Shift bool   `json:"shift,omitempty"`
}

func setNativeMenuBar(menus []menubar.Menu, bindings map[string]string) {
	var actions []string
	cocoa := make([]cocoaMenu, 0, len(menus))
	for _, menu := range menus {
		items := make([]cocoaItem, 0, len(menu.Items))
		for _, item := range menu.Items {
			native := cocoaItem{Title: item.Title, Role: string(item.Role)}
			if item.Action != "" {
				actions = append(actions, item.Action)
				native.Tag = len(actions)
				if key, ok := menubar.Shortcut(bindings, item, keys.Cmd); ok {
					native.setKey(key)
				}
			}
			items = append(items, native)
		}
		cocoa = append(cocoa, cocoaMenu{Title: menu.Title, Role: string(menu.Role), Items: items})
	}
	encoded, err := json.Marshal(cocoa)
	if err != nil {
		return
	}
	menuBarActions = actions
	text := C.CString(string(encoded))
	defer C.free(unsafe.Pointer(text))
	C.gopdfSetMenuBar(text)
}

// setKey shows key as the item's key equivalent, if Cocoa can show it.
func (i *cocoaItem) setKey(key keys.Key) {
	name, ok := cocoaKeyEquivalent(key.Name)
	if !ok {
		return
	}
	i.Key = name
	i.Cmd = key.Mod&keys.Cmd != 0
	i.Ctrl = key.Mod&keys.Ctrl != 0
	i.Shift = key.Mod&keys.Shift != 0
}

// cocoaKeyEquivalent returns the character Cocoa writes a key as in a menu.
// A capital letter stands for the letter with Shift, as in keys.
func cocoaKeyEquivalent(name string) (string, bool) {
	if len([]rune(name)) == 1 {
		return name, true
	}
	if r, ok := cocoaNamedKeys[name]; ok {
		return string(r), true
	}
	var n int
	if _, err := fmt.Sscanf(name, "F%d", &n); err == nil && n >= 1 && n <= 35 {
		return string(rune(0xF704 + n - 1)), true // NSF1FunctionKey onwards
	}
	return "", false
}

// cocoaNamedKeys are the characters NSEvent.h gives named keys.
var cocoaNamedKeys = map[string]rune{
	"CR":    '\r',
	"Esc":   0x1b,
	"Tab":   '\t',
	"BS":    0x08,
	"Space": ' ',
	"Up":    0xF700,
	"Down":  0xF701,
	"Left":  0xF702,
	"Right": 0xF703,
	"Ins":   0xF727,
	"Del":   0xF728,
	"Home":  0xF729,
	"End":   0xF72B,
	"PgUp":  0xF72C,
	"PgDn":  0xF72D,
}

//export gopdfMenuBarChose
func gopdfMenuBarChose(tag C.int) {
	if i := int(tag) - 1; i >= 0 && i < len(menuBarActions) {
		chooseMenuBarAction(menuBarActions[i])
	}
}
