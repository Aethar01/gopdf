package viewer

import (
	"testing"

	"gopdf/internal/keys"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestKeyToken(t *testing.T) {
	tests := []struct {
		name string
		key  sdl.Keycode
		mod  sdl.Keymod
		want string
	}{
		{name: "letter", key: sdl.KeycodeJ, want: "j"},
		{name: "shift letter", key: sdl.KeycodeJ, mod: sdl.KeymodShift, want: "J"},
		{name: "shift slash", key: sdl.KeycodeSlash, mod: sdl.KeymodShift, want: "?"},
		{name: "shift apostrophe", key: sdl.KeycodeApostrophe, mod: sdl.KeymodShift, want: "\""},
		{name: "shift digit", key: sdl.Keycode1, mod: sdl.KeymodShift, want: "1"},
		{name: "less than", key: sdl.KeycodeLess, want: "<lt>"},
		{name: "space", key: sdl.KeycodeSpace, want: "<Space>"},
		{name: "ctrl letter", key: sdl.KeycodeJ, mod: sdl.KeymodCtrl, want: "<C-j>"},
		{name: "ctrl shift letter", key: sdl.KeycodeR, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<C-S-r>"},
		{name: "cmd letter", key: sdl.KeycodeC, mod: sdl.KeymodGui, want: "<D-c>"},
		{name: "ctrl symbol", key: sdl.KeycodeMinus, mod: sdl.KeymodCtrl, want: "<C-->"},
		{name: "ctrl shift special", key: sdl.KeycodeTab, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<C-S-Tab>"},
		{name: "shift return", key: sdl.KeycodeReturn, mod: sdl.KeymodShift, want: "<S-CR>"},
		{name: "keypad enter", key: sdl.KeycodeKpEnter, want: "<CR>"},
		{name: "ctrl delete", key: sdl.KeycodeDelete, mod: sdl.KeymodCtrl, want: "<C-Del>"},
		{name: "function key", key: sdl.KeycodeF13, mod: sdl.KeymodShift, want: "<S-F13>"},
		{name: "keypad digit", key: sdl.KeycodeKp7, want: "<k7>"},
		{name: "alt ignored", key: sdl.KeycodeJ, mod: sdl.KeymodAlt, want: "j"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := keyToken(tt.key, tt.mod)
			if !ok || got.String() != tt.want {
				t.Fatalf("keyToken(%v, %v) = %q, %v; want %q, true", tt.key, tt.mod, got, ok, tt.want)
			}
		})
	}
}

func TestKeyTokenIgnoresUnnamedKeys(t *testing.T) {
	for _, key := range []sdl.Keycode{sdl.KeycodeLCtrl, sdl.KeycodeRCtrl, sdl.KeycodeLShift, sdl.KeycodeRShift, sdl.KeycodeLAlt, sdl.KeycodeRAlt, sdl.KeycodeLGui, sdl.KeycodeRGui, sdl.KeycodeCapsLock, sdl.KeycodeUnknown} {
		if got, ok := keyToken(key, sdl.KeymodCtrl); ok {
			t.Fatalf("expected key %v to be ignored, got %q", key, got)
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
