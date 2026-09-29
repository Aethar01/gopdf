package viewer

import (
	"fmt"
	"unicode"

	"gopdf/internal/keys"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// namedKeycodes gives each named key in keys.NamedKeys its keycode.
var namedKeycodes = func() map[sdl.Keycode]string {
	named := map[sdl.Keycode]string{
		sdl.KeycodeReturn:     "CR",
		sdl.KeycodeKpEnter:    "CR",
		sdl.KeycodeEscape:     "Esc",
		sdl.KeycodeTab:        "Tab",
		sdl.KeycodeBackspace:  "BS",
		sdl.KeycodeDelete:     "Del",
		sdl.KeycodeInsert:     "Ins",
		sdl.KeycodeSpace:      "Space",
		sdl.KeycodeUp:         "Up",
		sdl.KeycodeDown:       "Down",
		sdl.KeycodeLeft:       "Left",
		sdl.KeycodeRight:      "Right",
		sdl.KeycodePageUp:     "PgUp",
		sdl.KeycodePageDown:   "PgDn",
		sdl.KeycodeHome:       "Home",
		sdl.KeycodeEnd:        "End",
		sdl.KeycodeKp0:        "k0",
		sdl.KeycodeKpPlus:     "kPlus",
		sdl.KeycodeKpMinus:    "kMinus",
		sdl.KeycodeKpMultiply: "kMultiply",
		sdl.KeycodeKpDivide:   "kDivide",
		sdl.KeycodeKpPeriod:   "kPoint",
	}
	// SDL numbers F1-F12, F13-F24 and keypad 1-9 consecutively.
	for i := range sdl.Keycode(12) {
		named[sdl.KeycodeF1+i] = fmt.Sprintf("F%d", i+1)
		named[sdl.KeycodeF13+i] = fmt.Sprintf("F%d", i+13)
	}
	for i := range sdl.Keycode(9) {
		named[sdl.KeycodeKp1+i] = fmt.Sprintf("k%d", i+1)
	}
	return named
}()

// keyToken returns the key a keydown or keyup names, or false for keys
// that have no name, such as the modifiers themselves, and for keys pressed
// with Alt, which bindings cannot express.
//
// A printable key pressed without Ctrl or Cmd is the character it types.
// Letters come from the event's keycode, which SDL gives as the QWERTY
// letter on layouts without Latin letters, upper case with Shift or Caps
// Lock. Other characters come from the layout with Shift and AltGr applied,
// because the event's keycode is always the unshifted one.
func keyToken(e *sdl.KeyboardEvent) (keys.Key, bool) {
	if e.Mod&sdl.KeymodAlt != 0 {
		return keys.Key{}, false
	}
	var m keys.Mod
	if e.Mod&sdl.KeymodCtrl != 0 {
		m |= keys.Ctrl
	}
	if e.Mod&sdl.KeymodGui != 0 {
		m |= keys.Cmd
	}
	if e.Mod&sdl.KeymodShift != 0 {
		m |= keys.Shift
	}
	if name, ok := namedKeycodes[e.Key]; ok {
		return keys.Key{Mod: m, Name: name}, true
	}
	// Keycodes of printable keys are the character; the others carry
	// sdl.KeycodeScancodeMask, which puts them out of the rune range.
	r := rune(e.Key)
	if !unicode.IsPrint(r) {
		return keys.Key{}, false
	}
	switch {
	case m&(keys.Ctrl|keys.Cmd) != 0:
		r = unicode.ToLower(r)
	case unicode.IsLetter(r):
		if (e.Mod&sdl.KeymodShift != 0) != (e.Mod&sdl.KeymodCaps != 0) {
			r = unicode.ToUpper(r)
		}
		m = 0
	default:
		if typed := rune(sdl.GetKeyFromScancode(e.Scancode, e.Mod&(sdl.KeymodShift|sdl.KeymodMode), false)); unicode.IsPrint(typed) {
			r = typed
		}
		m = 0
	}
	return keys.Key{Mod: m, Name: string(r)}, true
}

func mouseButtonEvent(button uint8, eventType sdl.EventType) (string, bool) {
	name, ok := mouseButtonName(button)
	if !ok {
		return "", false
	}
	switch eventType {
	case sdl.EventMouseButtonDown:
		return name + "_down", true
	case sdl.EventMouseButtonUp:
		return name + "_up", true
	default:
		return "", false
	}
}

func mouseButtonName(button uint8) (string, bool) {
	switch button {
	case uint8(sdl.ButtonLeft):
		return "left", true
	case uint8(sdl.ButtonMiddle):
		return "middle", true
	case uint8(sdl.ButtonRight):
		return "right", true
	case uint8(sdl.ButtonX1):
		return "x1", true
	case uint8(sdl.ButtonX2):
		return "x2", true
	default:
		return "", false
	}
}

func buttonMask(button uint8) uint32 {
	switch button {
	case uint8(sdl.ButtonLeft):
		return uint32(sdl.ButtonLMask)
	case uint8(sdl.ButtonMiddle):
		return uint32(sdl.ButtonMMask)
	case uint8(sdl.ButtonRight):
		return uint32(sdl.ButtonRMask)
	case uint8(sdl.ButtonX1):
		return uint32(sdl.ButtonX1Mask)
	case uint8(sdl.ButtonX2):
		return uint32(sdl.ButtonX2Mask)
	default:
		return 0
	}
}
