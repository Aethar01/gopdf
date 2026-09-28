package viewer

import (
	"testing"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestHighlightPickerAnnotatesSelection(t *testing.T) {
	app, screenX, lineY := testSelectionApp(t, "select this text")
	app.config.AnnotationColors = config.Default().AnnotationColors
	app.highlightSelection(0)
	if app.message != "select text to highlight" || app.unsaved {
		t.Fatalf("highlight without selection: message=%q unsaved=%v", app.message, app.unsaved)
	}

	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: 2, X: screenX(120), Y: lineY})
	app.pickHighlightColor()
	view := app.activeUIView()
	if view == nil || len(view.rows) != 4 || view.rows[1].swatch == nil {
		t.Fatalf("picker = %+v", view)
	}
	view.onKey(app, &sdl.KeyboardEvent{Type: sdl.EventKeyDown, Key: sdl.Keycode2})

	if !app.unsaved || app.highlightColor != 1 || app.pageRevisions[0] != 1 {
		t.Fatalf("after picking: unsaved=%v colour=%d revision=%d message=%q", app.unsaved, app.highlightColor, app.pageRevisions[0], app.message)
	}
	if !app.selection.empty() || app.activeUIView() != nil {
		t.Fatal("selection or picker left open after highlighting")
	}
}
