package viewer

import (
	"strings"

	"github.com/veandco/go-sdl2/sdl"
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
	ctrl := mod&sdl.KMOD_CTRL != 0
	shift := mod&sdl.KMOD_SHIFT != 0
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
			return "<s-" + strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(token), "<"), ">") + ">", true
		}
		return normalizeAngleToken(token), true
	}
	if token, ok := printableKeyToken(key, shift); ok {
		return token, true
	}
	return "", false
}

func printableKeyToken(key sdl.Keycode, shift bool) (string, bool) {
	if key >= sdl.K_a && key <= sdl.K_z {
		r := rune('a' + (key - sdl.K_a))
		if shift {
			r -= 'a' - 'A'
		}
		return string(r), true
	}
	if key >= sdl.K_0 && key <= sdl.K_9 {
		return string(rune('0' + (key - sdl.K_0))), true
	}
	switch key {
	case sdl.K_SPACE:
		return " ", true
	case sdl.K_SLASH:
		if shift {
			return "?", true
		}
		return "/", true
	case sdl.K_SEMICOLON:
		if shift {
			return ":", true
		}
		return ";", true
	case sdl.K_EQUALS:
		if shift {
			return "+", true
		}
		return "=", true
	case sdl.K_MINUS:
		return "-", true
	default:
		return "", false
	}
}

func specialKeyToken(key sdl.Keycode) (string, bool) {
	switch key {
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		return "<CR>", true
	case sdl.K_ESCAPE:
		return "<Esc>", true
	case sdl.K_BACKSPACE:
		return "<BS>", true
	case sdl.K_PAGEDOWN:
		return "<PgDn>", true
	case sdl.K_PAGEUP:
		return "<PgUp>", true
	case sdl.K_TAB:
		return "<Tab>", true
	default:
		return "", false
	}
}

func mouseButtonEvent(button uint8, eventType uint32) (string, bool) {
	name, ok := mouseButtonName(button)
	if !ok {
		return "", false
	}
	switch eventType {
	case sdl.MOUSEBUTTONDOWN:
		return name + "_down", true
	case sdl.MOUSEBUTTONUP:
		return name + "_up", true
	default:
		return "", false
	}
}

func mouseButtonName(button uint8) (string, bool) {
	switch button {
	case sdl.BUTTON_LEFT:
		return "left", true
	case sdl.BUTTON_MIDDLE:
		return "middle", true
	case sdl.BUTTON_RIGHT:
		return "right", true
	case sdl.BUTTON_X1:
		return "x1", true
	case sdl.BUTTON_X2:
		return "x2", true
	default:
		return "", false
	}
}

func buttonMask(button uint8) uint32 {
	switch button {
	case sdl.BUTTON_LEFT:
		return sdl.ButtonLMask()
	case sdl.BUTTON_MIDDLE:
		return sdl.ButtonMMask()
	case sdl.BUTTON_RIGHT:
		return sdl.ButtonRMask()
	case sdl.BUTTON_X1:
		return sdl.ButtonX1Mask()
	case sdl.BUTTON_X2:
		return sdl.ButtonX2Mask()
	default:
		return 0
	}
}

func baseKeyName(key sdl.Keycode) (string, bool) {
	if key >= sdl.K_a && key <= sdl.K_z {
		return string(rune('a' + (key - sdl.K_a))), true
	}
	if key >= sdl.K_0 && key <= sdl.K_9 {
		return string(rune('0' + (key - sdl.K_0))), true
	}
	switch key {
	case sdl.K_SPACE:
		return "space", true
	case sdl.K_TAB:
		return "tab", true
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		return "enter", true
	case sdl.K_ESCAPE:
		return "esc", true
	case sdl.K_BACKSPACE:
		return "bs", true
	case sdl.K_PAGEDOWN:
		return "pgdn", true
	case sdl.K_PAGEUP:
		return "pgup", true
	default:
		return "", false
	}
}
