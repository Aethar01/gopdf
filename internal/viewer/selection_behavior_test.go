package viewer

import (
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestGuiClipboardShortcutsUseDModifierBindings(t *testing.T) {
	tests := []struct {
		key  sdl.Keycode
		want string
	}{
		{key: sdl.KeycodeC, want: "<d-c>"},
		{key: sdl.KeycodeX, want: "<d-x>"},
		{key: sdl.KeycodeV, want: "<d-v>"},
	}
	for _, tt := range tests {
		got, ok := keyToken(tt.key, sdl.KeymodGui)
		if !ok || got != tt.want {
			t.Fatalf("keyToken(%v, Command) = %q, %v; want %s, true", tt.key, got, ok, tt.want)
		}
	}
}

func TestClipboardEditingMainTextInput(t *testing.T) {
	withTestClipboard(t)
	app := &App{inputState: inputState{mode: modeCommand}}
	app.input.Set("hello")

	if !app.copyActiveTextInputToClipboard() || sdlGetClipboardText() != "hello" {
		t.Fatalf("expected copy to place input text on clipboard")
	}
	if app.input.Value != "hello" {
		t.Fatalf("copy changed input: %q", app.input.Value)
	}

	if !app.cutActiveTextInputToClipboard() || app.input.Value != "" {
		t.Fatalf("expected cut to clear input, got %q", app.input.Value)
	}
	if sdlGetClipboardText() != "hello" {
		t.Fatalf("expected cut text on clipboard, got %q", sdlGetClipboardText())
	}

	sdlSetClipboardText("paste")
	app.input.Set("hi")
	app.input.Cursor = 1
	if !app.pasteIntoActiveTextInput() || app.input.Value != "hpastei" {
		t.Fatalf("expected cursor-aware paste, got %q", app.input.Value)
	}
}

func TestClipboardEditingOutlineSearch(t *testing.T) {
	withTestClipboard(t)
	app := &App{uiState: uiState{outlineMenu: outlineMenuState{visible: true, searching: true, query: "needle"}}}

	if !app.cutActiveTextInputToClipboard() || app.outlineMenu.query != "" {
		t.Fatalf("expected cut to clear outline query, got %q", app.outlineMenu.query)
	}
	if sdlGetClipboardText() != "needle" {
		t.Fatalf("expected outline query on clipboard, got %q", sdlGetClipboardText())
	}

	sdlSetClipboardText("filter")
	if !app.pasteIntoActiveTextInput() || app.outlineMenu.query != "filter" {
		t.Fatalf("expected paste into outline query, got %q", app.outlineMenu.query)
	}
}

func TestClipboardEditingLuaUISearch(t *testing.T) {
	withTestClipboard(t)
	app := &App{uiState: uiState{luaUI: luaUIState{visible: true, searching: true, query: "menu"}}}

	if !app.copyActiveTextInputToClipboard() || sdlGetClipboardText() != "menu" {
		t.Fatalf("expected Lua UI query on clipboard, got %q", sdlGetClipboardText())
	}
	sdlSetClipboardText(" item")
	if !app.pasteIntoActiveTextInput() || app.luaUI.query != "menu item" {
		t.Fatalf("expected paste into Lua UI query, got %q", app.luaUI.query)
	}
}

func TestCopyOnSelectDisabledKeepsSelectionAfterMouseCopyHook(t *testing.T) {
	app := &App{
		config: config.Config{CopyOnSelect: false},
		interactionState: interactionState{selection: textSelection{
			text:  "selected text",
			quads: []mupdf.Quad{{}},
		}},
	}

	app.copySelectionToClipboard()

	if app.selection.text != "selected text" || len(app.selection.quads) != 1 {
		t.Fatalf("expected selection to persist, got %+v", app.selection)
	}
}

func TestEscapeClearsPersistentSelection(t *testing.T) {
	app := &App{
		config: config.Config{CopyOnSelect: false},
		interactionState: interactionState{selection: textSelection{
			text:  "selected text",
			quads: []mupdf.Quad{{}},
		}},
	}

	app.closeActiveUI()

	if app.selection.text != "" || len(app.selection.quads) != 0 {
		t.Fatalf("expected escape to clear selection, got %+v", app.selection)
	}
}

func withTestClipboard(t *testing.T) {
	t.Helper()
	oldSet := sdlSetClipboardText
	oldGet := sdlGetClipboardText
	clipboard := ""
	sdlSetClipboardText = func(text string) bool {
		clipboard = text
		return true
	}
	sdlGetClipboardText = func() string { return clipboard }
	t.Cleanup(func() {
		sdlSetClipboardText = oldSet
		sdlGetClipboardText = oldGet
	})
}
