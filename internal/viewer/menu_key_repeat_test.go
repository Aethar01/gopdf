package viewer

import (
	"testing"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestRepeatableMenuActionAllowsHeldNavigation(t *testing.T) {
	app := testLayoutApp(5)
	app.outlineMenu.visible = true
	app.sequenceLookup = map[string]string{
		normalizeBinding("<Down>"): "scroll_down",
	}

	action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeDown, Repeat: true})
	if !ok || action != "scroll_down" {
		t.Fatalf("expected repeated menu navigation to resolve scroll_down, got %q, ok=%v", action, ok)
	}
}

func TestRepeatableMenuActionRequiresActiveMenu(t *testing.T) {
	app := testLayoutApp(5)
	app.sequenceLookup = map[string]string{
		normalizeBinding("<Down>"): "scroll_down",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeDown, Repeat: true}); ok {
		t.Fatalf("expected document repeat handling to remain unchanged, got %q", action)
	}
}

func TestRepeatableMenuActionRejectsMultiKeyPrefix(t *testing.T) {
	app := testLayoutApp(5)
	app.luaUI.visible = true
	app.sequenceLookup = map[string]string{
		normalizeBinding("j"):   "scroll_down",
		normalizeBinding("j j"): "first_page",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeJ, Repeat: true}); ok {
		t.Fatalf("expected repeated prefix to remain ignored, got %q", action)
	}
}

func TestRepeatableMenuActionRejectsNonCountableAction(t *testing.T) {
	app := testLayoutApp(5)
	app.keybindMenu.visible = true
	app.sequenceLookup = map[string]string{
		normalizeBinding("<CR>"): "confirm",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeReturn, Repeat: true}); ok {
		t.Fatalf("expected confirm not to repeat, got %q", action)
	}
}

func TestRepeatableMenuActionIgnoredWhileCapturingKeybind(t *testing.T) {
	app := testLayoutApp(5)
	app.keybindMenu.visible = true
	app.keybindMenu.capturing = true
	app.sequenceLookup = map[string]string{
		normalizeBinding("<Down>"): "scroll_down",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeDown, Repeat: true}); ok {
		t.Fatalf("expected key capture repeat to remain ignored, got %q", action)
	}
}

func TestOutlineSearchRepeatsBackspace(t *testing.T) {
	app := &App{uiState: uiState{outlineMenu: outlineMenuState{
		visible:   true,
		searching: true,
		query:     "abc",
	}}}
	e := sdl.KeyboardEvent{CommonEvent: sdl.CommonEvent{Type: sdl.EventKeyDown}, Key: sdl.KeycodeBackspace, Repeat: true}

	app.handleOutlineMenuKey(&e)

	if app.outlineMenu.query != "ab" {
		t.Fatalf("expected repeated outline backspace to edit query, got %q", app.outlineMenu.query)
	}
}

func TestLuaUISearchRepeatsBackspace(t *testing.T) {
	app := &App{uiState: uiState{luaUI: luaUIState{
		visible:   true,
		searching: true,
		query:     "abc",
	}}}
	e := sdl.KeyboardEvent{CommonEvent: sdl.CommonEvent{Type: sdl.EventKeyDown}, Key: sdl.KeycodeBackspace, Repeat: true}

	app.handleLuaUIKey(&e)

	if app.luaUI.query != "ab" {
		t.Fatalf("expected repeated Lua UI backspace to edit query, got %q", app.luaUI.query)
	}
}
