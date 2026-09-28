package viewer

import (
	"strings"
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// testSelectionApp lays out a one-page document with a line of text and
// returns it with the screen x of a page point and the line's screen y.
func testSelectionApp(t *testing.T, line string) (app *App, screenX func(float64) float32, lineY float32) {
	t.Helper()
	doc, err := mupdf.Open(testpdf.Write(t, line), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(doc.Close)
	info, err := doc.PageInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	app = testLayoutApp(1)
	app.doc = doc
	app.pageLinks = map[int][]mupdf.Link{}
	app.pageMetrics[0] = newPageMetrics(info)
	app.config.MouseTextSelect = true
	app.recomputeLayout(1000, 1000)
	x, y, _ := app.pageScreenOrigin(0)
	// The line's baseline is 700pt up a 792pt page; the text sits just above.
	return app, func(px float64) float32 { return float32(x + px*app.scale) }, float32(y + (792-704)*app.scale)
}

func TestFinishedSelectionStaysHighlightedUntilCleared(t *testing.T) {
	app, screenX, lineY := testSelectionApp(t, "select this text")
	button := func(kind sdl.EventType, px float64) {
		app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: kind, Button: uint8(sdl.ButtonLeft), Clicks: 1, X: screenX(px), Y: lineY})
	}
	drag := func(px float64) {
		app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: screenX(px), Y: lineY})
	}

	button(sdl.EventMouseButtonDown, 70)
	drag(200)
	button(sdl.EventMouseButtonUp, 200)
	if !strings.Contains(app.selection.text, "select") || len(app.selection.quads) == 0 {
		t.Fatalf("after release: text=%q quads=%d, want selection kept", app.selection.text, len(app.selection.quads))
	}

	app.closeActiveUI()
	if app.selection.text != "" || len(app.selection.quads) != 0 {
		t.Fatal("close did not clear the selection")
	}

	button(sdl.EventMouseButtonDown, 70)
	drag(200)
	button(sdl.EventMouseButtonUp, 200)
	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: 1, X: -10, Y: -10})
	if len(app.selection.quads) != 0 {
		t.Fatal("clicking off the page did not clear the selection")
	}
}

func TestMultiClickSelectsWordsAndLines(t *testing.T) {
	app, screenX, lineY := testSelectionApp(t, "select this text")
	click := func(clicks uint8, px float64) string {
		app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: clicks, X: screenX(px), Y: lineY})
		app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonUp, Button: uint8(sdl.ButtonLeft), Clicks: clicks, X: screenX(px), Y: lineY})
		return strings.TrimSpace(app.selection.text)
	}
	// Helvetica 12pt puts "this" at roughly 110-130pt from the left edge.
	if got := click(2, 120); got != "this" {
		t.Fatalf("double click selected %q, want %q", got, "this")
	}
	if got := click(3, 120); got != "select this text" {
		t.Fatalf("triple click selected %q, want the line", got)
	}
}
