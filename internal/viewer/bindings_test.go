package viewer

import (
	"testing"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestNormalizeAndTokenizeBindings(t *testing.T) {
	tests := []struct {
		binding string
		want    string
	}{
		{binding: "gg", want: "g g"},
		{binding: "<Space>", want: " "},
		{binding: "<Return>", want: "<cr>"},
		{binding: "< C-S-Tab >x", want: "<c-s-tab> x"},
	}

	for _, tt := range tests {
		if got := normalizeBinding(tt.binding); got != tt.want {
			t.Fatalf("normalizeBinding(%q) = %q, want %q", tt.binding, got, tt.want)
		}
	}
}

func TestKeyTokenIncludesModifiedAndPrintableKeys(t *testing.T) {
	tests := []struct {
		name string
		key  sdl.Keycode
		mod  sdl.Keymod
		want string
	}{
		{name: "ctrl letter", key: sdl.KeycodeJ, mod: sdl.KeymodCtrl, want: "<c-j>"},
		{name: "ctrl d", key: sdl.KeycodeD, mod: sdl.KeymodCtrl, want: "<c-d>"},
		{name: "ctrl shift special", key: sdl.KeycodeTab, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<c-s-tab>"},
		{name: "shift slash", key: sdl.KeycodeSlash, mod: sdl.KeymodShift, want: "?"},
		{name: "shift apostrophe", key: sdl.KeycodeApostrophe, mod: sdl.KeymodShift, want: "\""},
		{name: "shift return", key: sdl.KeycodeReturn, mod: sdl.KeymodShift, want: "<s-cr>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := keyToken(tt.key, tt.mod)
			if !ok || got != tt.want {
				t.Fatalf("keyToken(%v, %v) = %q, %v; want %q, true", tt.key, tt.mod, got, ok, tt.want)
			}
		})
	}
}

func TestKeyTokenIgnoresStandaloneModifiers(t *testing.T) {
	for _, key := range []sdl.Keycode{sdl.KeycodeLCtrl, sdl.KeycodeRCtrl, sdl.KeycodeLShift, sdl.KeycodeRShift, sdl.KeycodeLAlt, sdl.KeycodeRAlt, sdl.KeycodeLGui, sdl.KeycodeRGui} {
		if got, ok := keyToken(key, sdl.KeymodCtrl); ok || got != "" {
			t.Fatalf("expected modifier key %v to be ignored, got %q, %v", key, got, ok)
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
