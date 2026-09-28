package viewer

import (
	"strings"
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestFinishedSelectionStaysHighlightedUntilCleared(t *testing.T) {
	doc, err := mupdf.Open(testpdf.Write(t, "select this text"), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	info, err := doc.PageInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	app := testLayoutApp(1)
	app.doc = doc
	app.pageLinks = map[int][]mupdf.Link{}
	app.pageMetrics[0] = newPageMetrics(info)
	app.config.MouseTextSelect = true
	app.recomputeLayout(1000, 1000)
	x, y, _ := app.pageScreenOrigin(0)
	// The line's baseline is 700pt up a 792pt page; the text sits just above.
	lineY := y + (792-704)*app.scale
	button := func(kind sdl.EventType, px float64) {
		app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: kind, Button: uint8(sdl.ButtonLeft), X: float32(x + px*app.scale), Y: float32(lineY)})
	}

	button(sdl.EventMouseButtonDown, 70)
	app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: float32(x + 200*app.scale), Y: float32(lineY)})
	button(sdl.EventMouseButtonUp, 200)
	if !strings.Contains(app.selection.text, "select") || len(app.selection.quads) == 0 {
		t.Fatalf("after release: text=%q quads=%d, want selection kept", app.selection.text, len(app.selection.quads))
	}

	app.closeActiveUI()
	if app.selection.text != "" || len(app.selection.quads) != 0 {
		t.Fatal("close did not clear the selection")
	}

	button(sdl.EventMouseButtonDown, 70)
	app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: float32(x + 200*app.scale), Y: float32(lineY)})
	button(sdl.EventMouseButtonUp, 200)
	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), X: -10, Y: -10})
	if len(app.selection.quads) != 0 {
		t.Fatal("clicking off the page did not clear the selection")
	}
}
