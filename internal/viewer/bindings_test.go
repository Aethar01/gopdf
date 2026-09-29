package viewer

import (
	"testing"

	"gopdf/internal/keys"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestKeyToken(t *testing.T) {
	tests := []struct {
		name     string
		key      sdl.Keycode
		scancode sdl.Scancode
		mod      sdl.Keymod
		want     string
	}{
		{name: "letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, want: "j"},
		{name: "shift letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, mod: sdl.KeymodShift, want: "J"},
		{name: "caps lock letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, mod: sdl.KeymodCaps, want: "J"},
		{name: "caps lock shift letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, mod: sdl.KeymodCaps | sdl.KeymodShift, want: "j"},
		{name: "shift slash", key: sdl.KeycodeSlash, scancode: sdl.ScancodeSlash, mod: sdl.KeymodShift, want: "?"},
		{name: "shift apostrophe", key: sdl.KeycodeApostrophe, scancode: sdl.ScancodeApostrophe, mod: sdl.KeymodShift, want: "\""},
		{name: "shift digit", key: sdl.Keycode1, scancode: sdl.Scancode1, mod: sdl.KeymodShift, want: "!"},
		{name: "shift comma", key: sdl.KeycodeComma, scancode: sdl.ScancodeComma, mod: sdl.KeymodShift, want: "<lt>"},
		{name: "caps lock digit", key: sdl.Keycode5, scancode: sdl.Scancode5, mod: sdl.KeymodCaps, want: "5"},
		{name: "no scancode", key: sdl.KeycodeMinus, mod: sdl.KeymodShift, want: "-"},
		{name: "space", key: sdl.KeycodeSpace, scancode: sdl.ScancodeSpace, want: "<Space>"},
		{name: "ctrl letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, mod: sdl.KeymodCtrl, want: "<C-j>"},
		{name: "ctrl caps lock letter", key: sdl.KeycodeJ, scancode: sdl.ScancodeJ, mod: sdl.KeymodCtrl | sdl.KeymodCaps, want: "<C-j>"},
		{name: "ctrl shift letter", key: sdl.KeycodeR, scancode: sdl.ScancodeR, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<C-S-r>"},
		{name: "ctrl shift digit", key: sdl.Keycode1, scancode: sdl.Scancode1, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<C-S-1>"},
		{name: "cmd letter", key: sdl.KeycodeC, scancode: sdl.ScancodeC, mod: sdl.KeymodGui, want: "<D-c>"},
		{name: "ctrl symbol", key: sdl.KeycodeMinus, scancode: sdl.ScancodeMinus, mod: sdl.KeymodCtrl, want: "<C-->"},
		{name: "ctrl shift special", key: sdl.KeycodeTab, scancode: sdl.ScancodeTab, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<C-S-Tab>"},
		{name: "shift return", key: sdl.KeycodeReturn, scancode: sdl.ScancodeReturn, mod: sdl.KeymodShift, want: "<S-CR>"},
		{name: "keypad enter", key: sdl.KeycodeKpEnter, scancode: sdl.ScancodeKpEnter, want: "<CR>"},
		{name: "ctrl delete", key: sdl.KeycodeDelete, scancode: sdl.ScancodeDelete, mod: sdl.KeymodCtrl, want: "<C-Del>"},
		{name: "function key", key: sdl.KeycodeF13, scancode: sdl.ScancodeF13, mod: sdl.KeymodShift, want: "<S-F13>"},
		{name: "keypad digit", key: sdl.KeycodeKp7, scancode: sdl.ScancodeKp7, want: "<k7>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := keyToken(&sdl.KeyboardEvent{Key: tt.key, Scancode: tt.scancode, Mod: tt.mod})
			if !ok || got.String() != tt.want {
				t.Fatalf("keyToken(%v, %v) = %q, %v; want %q, true", tt.key, tt.mod, got, ok, tt.want)
			}
		})
	}
}

func TestKeyTokenIgnoresUnnamedKeysAndAlt(t *testing.T) {
	for _, key := range []sdl.Keycode{sdl.KeycodeLCtrl, sdl.KeycodeRCtrl, sdl.KeycodeLShift, sdl.KeycodeRShift, sdl.KeycodeLAlt, sdl.KeycodeRAlt, sdl.KeycodeLGui, sdl.KeycodeRGui, sdl.KeycodeCapsLock, sdl.KeycodeUnknown} {
		if got, ok := keyToken(&sdl.KeyboardEvent{Key: key, Mod: sdl.KeymodCtrl}); ok {
			t.Fatalf("expected key %v to be ignored, got %q", key, got)
		}
	}
	for _, mod := range []sdl.Keymod{sdl.KeymodLAlt, sdl.KeymodRAlt, sdl.KeymodCtrl | sdl.KeymodRAlt} {
		if got, ok := keyToken(&sdl.KeyboardEvent{Key: sdl.KeycodeJ, Scancode: sdl.ScancodeJ, Mod: mod}); ok {
			t.Fatalf("expected j with modifiers %v to be ignored, got %q", mod, got)
		}
	}
}

// Every named key must be reachable from the keyboard, and every key the
// viewer names must be one the configuration accepts.
func TestNamedKeycodesMatchKeysPackage(t *testing.T) {
	reachable := map[string]bool{}
	for _, name := range namedKeycodes {
		reachable[name] = true
	}
	for _, named := range keys.NamedKeys() {
		if !reachable[named.Name] {
			t.Errorf("named key %q has no keycode", named.Name)
		}
		delete(reachable, named.Name)
	}
	for name := range reachable {
		t.Errorf("keycode name %q is not a named key", name)
	}
}

func TestMouseEventsAreKnown(t *testing.T) {
	events := []string{"<C-wheel_up>", "<C-wheel_down>", "wheel_up", "wheel_down", "wheel_left", "wheel_right"}
	for _, button := range []sdl.MouseButtonFlags{sdl.ButtonLeft, sdl.ButtonMiddle, sdl.ButtonRight, sdl.ButtonX1, sdl.ButtonX2} {
		for _, eventType := range []sdl.EventType{sdl.EventMouseButtonDown, sdl.EventMouseButtonUp} {
			event, ok := mouseButtonEvent(uint8(button), eventType)
			if !ok {
				t.Fatalf("button %d has no event name", button)
			}
			events = append(events, event)
		}
	}
	for _, event := range events {
		if got, err := keys.NormalizeMouseEvent(event); err != nil || got != event {
			t.Errorf("viewer mouse event %q normalizes to %q, %v", event, got, err)
		}
	}
}

func TestMouseButtonHelpers(t *testing.T) {
	if got, ok := mouseButtonEvent(uint8(sdl.ButtonX1), sdl.EventMouseButtonDown); !ok || got != "x1_down" {
		t.Fatalf("expected x1_down event, got %q, %v", got, ok)
	}
	if got, ok := mouseButtonEvent(uint8(sdl.ButtonRight), sdl.EventMouseButtonUp); !ok || got != "right_up" {
		t.Fatalf("expected right_up event, got %q, %v", got, ok)
	}
	if got, ok := mouseButtonEvent(99, sdl.EventMouseButtonDown); ok || got != "" {
		t.Fatalf("expected unknown button to fail, got %q, %v", got, ok)
	}
	if got := buttonMask(99); got != 0 {
		t.Fatalf("expected unknown button mask 0, got %d", got)
	}
}
