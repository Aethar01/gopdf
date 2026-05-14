package viewer

import (
	"fmt"
	"strings"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func isCountableAction(action string) bool {
	switch action {
	case "next_page", "prev_page", "scroll_down", "scroll_up", "scroll_left", "scroll_right", "next_spread", "prev_spread", "zoom_in", "zoom_out", "search_next", "search_prev":
		return true
	default:
		return false
	}
}

func normalizeBinding(binding string) string {
	return strings.Join(tokenizeBinding(binding), " ")
}

func tokenizeBinding(binding string) []string {
	tokens := make([]string, 0, len(binding))
	for i := 0; i < len(binding); {
		if binding[i] == '<' {
			if end := strings.IndexByte(binding[i:], '>'); end > 0 {
				tokens = append(tokens, normalizeAngleToken(binding[i:i+end+1]))
				i += end + 1
				continue
			}
		}
		tokens = append(tokens, string(binding[i]))
		i++
	}
	return tokens
}

func normalizeAngleToken(token string) string {
	inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">")
	parts := strings.Split(inner, "-")
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	if len(parts) == 1 && parts[0] == "space" {
		return " "
	}
	if len(parts) == 1 && (parts[0] == "enter" || parts[0] == "return") {
		return "<cr>"
	}
	return "<" + strings.Join(parts, "-") + ">"
}

func keyToken(key sdl.Keycode, mod sdl.Keymod) (string, bool) {
	if isModifierKey(key) {
		return "", false
	}
	ctrl := mod&sdl.KeymodCtrl != 0
	shift := mod&sdl.KeymodShift != 0
	if ctrl {
		if base, ok := baseKeyName(key); ok {
			if shift {
				return "<c-s-" + base + ">", true
			}
			return "<c-" + base + ">", true
		}
	}
	if token, ok := specialKeyToken(key); ok {
		if shift {
			inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">")
			return normalizeAngleToken("<s-" + inner + ">"), true
		}
		return normalizeAngleToken(token), true
	}
	if token, ok := printableKeyToken(key, shift); ok {
		return token, true
	}
	return "", false
}

func isModifierKey(key sdl.Keycode) bool {
	switch key {
	case sdl.KeycodeLCtrl, sdl.KeycodeRCtrl, sdl.KeycodeLShift, sdl.KeycodeRShift, sdl.KeycodeLAlt, sdl.KeycodeRAlt, sdl.KeycodeLGui, sdl.KeycodeRGui, sdl.KeycodeMode, sdl.KeycodeLevel5Shift:
		return true
	default:
		return false
	}
}

func printableKeyToken(key sdl.Keycode, shift bool) (string, bool) {
	if key >= sdl.KeycodeA && key <= sdl.KeycodeZ {
		r := rune('a' + (key - sdl.KeycodeA))
		if shift {
			r -= 'a' - 'A'
		}
		return string(r), true
	}
	if key >= sdl.Keycode0 && key <= sdl.Keycode9 {
		return string(rune('0' + (key - sdl.Keycode0))), true
	}
	switch key {
	case sdl.KeycodeSpace:
		return " ", true
	case sdl.KeycodeSlash:
		if shift {
			return "?", true
		}
		return "/", true
	case sdl.KeycodeSemicolon:
		if shift {
			return ":", true
		}
		return ";", true
	case sdl.KeycodeEquals:
		if shift {
			return "+", true
		}
		return "=", true
	case sdl.KeycodeMinus:
		return "-", true
	case sdl.KeycodeApostrophe, sdl.KeycodeDblApostrophe, sdl.KeycodeComma, sdl.KeycodePeriod, sdl.KeycodeBackslash, sdl.KeycodeLeftBracket, sdl.KeycodeRightBracket, sdl.KeycodeGrave, sdl.KeycodeExclaim, sdl.KeycodeHash, sdl.KeycodeDollar, sdl.KeycodePercent, sdl.KeycodeAmpersand, sdl.KeycodeLeftParen, sdl.KeycodeRightParen, sdl.KeycodeAsterisk, sdl.KeycodePlus, sdl.KeycodeColon, sdl.KeycodeLess, sdl.KeycodeGreater, sdl.KeycodeQuestion, sdl.KeycodeAt, sdl.KeycodeCaret, sdl.KeycodeUnderscore, sdl.KeycodePipe, sdl.KeycodeTilde:
		return string(rune(key)), true
	default:
		return "", false
	}
}

func specialKeyToken(key sdl.Keycode) (string, bool) {
	switch key {
	case sdl.KeycodeUp:
		return "<Up>", true
	case sdl.KeycodeDown:
		return "<Down>", true
	case sdl.KeycodeLeft:
		return "<Left>", true
	case sdl.KeycodeRight:
		return "<Right>", true
	case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
		return "<CR>", true
	case sdl.KeycodeEscape:
		return "<Esc>", true
	case sdl.KeycodeBackspace:
		return "<BS>", true
	case sdl.KeycodePageDown:
		return "<PgDn>", true
	case sdl.KeycodePageUp:
		return "<PgUp>", true
	case sdl.KeycodeTab:
		return "<Tab>", true
	case sdl.KeycodeDelete:
		return "<Del>", true
	case sdl.KeycodeInsert:
		return "<Ins>", true
	case sdl.KeycodeHome:
		return "<Home>", true
	case sdl.KeycodeEnd:
		return "<End>", true
	default:
		return fallbackSpecialKeyToken(key)
	}
}

func fallbackSpecialKeyToken(key sdl.Keycode) (string, bool) {
	if key < 0x80 {
		return "", false
	}
	name := strings.TrimSpace(sdl.GetKeyName(key))
	if name == "" {
		return "", false
	}
	name = strings.ReplaceAll(name, " ", "-")
	return "<" + name + ">", true
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

func baseKeyName(key sdl.Keycode) (string, bool) {
	if key >= sdl.KeycodeA && key <= sdl.KeycodeZ {
		return string(rune('a' + (key - sdl.KeycodeA))), true
	}
	if key >= sdl.Keycode0 && key <= sdl.Keycode9 {
		return string(rune('0' + (key - sdl.Keycode0))), true
	}
	switch key {
	case sdl.KeycodeSpace:
		return "space", true
	case sdl.KeycodeTab:
		return "tab", true
	case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
		return "enter", true
	case sdl.KeycodeEscape:
		return "esc", true
	case sdl.KeycodeBackspace:
		return "bs", true
	case sdl.KeycodePageDown:
		return "pgdn", true
	case sdl.KeycodePageUp:
		return "pgup", true
	default:
		if token, ok := fallbackSpecialKeyToken(key); ok {
			return strings.TrimSuffix(strings.TrimPrefix(normalizeAngleToken(token), "<"), ">"), true
		}
		return fmt.Sprintf("keycode-%d", key), true
	}
}
