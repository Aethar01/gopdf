package viewer

import (
	"path/filepath"
	"testing"
	"time"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestRepeatableMenuActionAllowsHeldNavigation(t *testing.T) {
	app := testLayoutApp(5)
	view := &uiView{visible: true, modal: true}
	app.views.active = view
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
	view := &uiView{visible: true, modal: true}
	app.views.active = view
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
	view := &uiView{visible: true, modal: true}
	app.views.active = view
	app.sequenceLookup = map[string]string{
		normalizeBinding("<CR>"): "confirm",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeReturn, Repeat: true}); ok {
		t.Fatalf("expected confirm not to repeat, got %q", action)
	}
}

func TestRepeatableMenuActionIgnoredWhileCapturingKeybind(t *testing.T) {
	app := testLayoutApp(5)
	view := &uiView{visible: true, modal: true}
	app.views.active = view
	app.keybindMenu.view = view
	app.keybindMenu.capturing = true
	app.sequenceLookup = map[string]string{
		normalizeBinding("<Down>"): "scroll_down",
	}

	if action, ok := app.repeatableMenuAction(&sdl.KeyboardEvent{Key: sdl.KeycodeDown, Repeat: true}); ok {
		t.Fatalf("expected key capture repeat to remain ignored, got %q", action)
	}
}

func TestUIViewSearchRepeatsBackspace(t *testing.T) {
	view := &uiView{visible: true, modal: true, searchable: true, searching: true, query: "abc"}
	app := &App{uiState: uiState{views: uiManager{active: view}}}
	e := sdl.KeyboardEvent{CommonEvent: sdl.CommonEvent{Type: sdl.EventKeyDown}, Key: sdl.KeycodeBackspace, Repeat: true}

	app.handleUIViewSearchKey(view, &e)

	if view.query != "ab" {
		t.Fatalf("expected repeated backspace to edit query, got %q", view.query)
	}
}

func TestPluginUIViewSearchRepeatsBackspace(t *testing.T) {
	view := &uiView{visible: true, modal: true, searchable: true, searching: true, query: "abc"}
	app := &App{uiState: uiState{views: uiManager{active: view}}}
	e := sdl.KeyboardEvent{CommonEvent: sdl.CommonEvent{Type: sdl.EventKeyDown}, Key: sdl.KeycodeBackspace, Repeat: true}

	app.handleGenericUIViewKey(view, &e)

	if view.query != "ab" {
		t.Fatalf("expected repeated plugin UI backspace to edit query, got %q", view.query)
	}
}

func TestKeybindEditorCapturesChords(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	t.Setenv("XDG_CONFIG_HOME", dir)
	rt, err := config.Open(filepath.Join(dir, "config.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	app, err := New("", rt, 0, nil, NewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	app.toggleKeybindMenu()
	app.startNewKeybind()
	for i, row := range app.keybindMenu.rows {
		if row.action == "overview" {
			app.keybindMenu.view.selected = i
		}
	}
	app.confirmKeybindMenuSelection()

	start := time.Now()
	for _, key := range []sdl.Keycode{sdl.KeycodeZ, sdl.KeycodeX} {
		app.handleKeybindMenuKey(&sdl.KeyboardEvent{Type: sdl.EventKeyDown, Key: key})
	}
	app.finishKeybindCapture(start) // still within the pause: nothing bound yet
	if !app.keybindMenu.capturing {
		t.Fatal("bound before the chord's pause elapsed")
	}
	app.finishKeybindCapture(start.Add(time.Duration(app.config.SequenceTimeoutMS+1) * time.Millisecond))
	if app.keybindMenu.capturing {
		t.Fatal("still capturing after the pause")
	}
	if got := app.config.KeyBindings["zx"]; got != "overview" {
		t.Fatalf("zx bound to %q, want overview; message %q", got, app.message)
	}
}
