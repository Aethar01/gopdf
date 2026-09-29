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
		app.refreshStaleSelection() // as the event loop does each frame
	}

	button(sdl.EventMouseButtonDown, 70)
	drag(200)
	button(sdl.EventMouseButtonUp, 200)
	if !strings.Contains(app.selection.text, "select") || len(app.selection.parts) == 0 {
		t.Fatalf("after release: text=%q quads=%d, want selection kept", app.selection.text, len(app.selection.parts))
	}

	app.closeActiveUI()
	if app.selection.text != "" || !app.selection.empty() {
		t.Fatal("close did not clear the selection")
	}

	button(sdl.EventMouseButtonDown, 70)
	drag(200)
	button(sdl.EventMouseButtonUp, 200)
	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: 1, X: -10, Y: -10})
	if !app.selection.empty() {
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

func TestSelectionSpansPages(t *testing.T) {
	doc, err := mupdf.Open(testpdf.WritePages(t, []string{"alpha one"}, []string{"beta two"}, []string{"gamma three"}), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(3)
	app.doc = doc
	app.pageLinks = map[int][]mupdf.Link{}
	for page := range 3 {
		info, err := doc.PageInfo(page)
		if err != nil {
			t.Fatal(err)
		}
		app.pageMetrics[page] = newPageMetrics(info)
	}
	app.config.MouseTextSelect = true
	app.fitMode = "width"
	app.winW, app.winH = 300, 2000 // all three pages on screen
	app.recomputeLayout(app.viewportSize())
	at := func(page int, px float64) (float32, float32) {
		x, y, _ := app.pageScreenOrigin(page)
		return float32(x + px*app.scale), float32(y + (792-704)*app.scale)
	}

	x, y := at(0, 110) // in "one"
	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: 1, X: x, Y: y})
	x, y = at(2, 100) // in "gamma"
	app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: x, Y: y})
	app.refreshStaleSelection()

	var pages []int
	for _, part := range app.selection.parts {
		pages = append(pages, part.page)
	}
	if len(pages) != 3 || pages[0] != 0 || pages[2] != 2 {
		t.Fatalf("selected pages %v, want 0 1 2", pages)
	}
	for _, want := range []string{"ne", "beta two", "gam"} {
		if !strings.Contains(app.selection.text, want) {
			t.Errorf("selection %q lacks %q", app.selection.text, want)
		}
	}
	if strings.Contains(app.selection.text, "alpha") || strings.Contains(app.selection.text, "three") {
		t.Errorf("selection %q runs past its end points", app.selection.text)
	}
	if _, ok := app.selection.wholePages[1]; !ok {
		t.Error("middle page was not kept for later drag motions")
	}

	x, y = at(1, 80) // back into "beta", making page 1 an end again
	app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: x, Y: y})
	app.refreshStaleSelection()
	if strings.Contains(app.selection.text, "two") || strings.Contains(app.selection.text, "gam") {
		t.Errorf("selection %q still uses the whole of its new end page", app.selection.text)
	}
}

func TestCrossPageSelectionFollowsReadingOrder(t *testing.T) {
	doc, err := mupdf.Open(testpdf.WritePages(t, []string{"alpha", "the sentence starts"}, []string{"2", "continues here", "more"}), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(2)
	app.doc = doc
	for page := range 2 {
		info, _ := doc.PageInfo(page)
		app.pageMetrics[page] = newPageMetrics(info)
	}
	// Second-line text on each page sits around y=102 (baseline 686pt up).
	app.selection = textSelection{anchorPage: 0, anchor: mupdf.Point{X: 100, Y: 102}, focusPage: 1, focus: mupdf.Point{X: 120, Y: 102}}
	app.refreshSelection()
	if !strings.Contains(app.selection.text, "starts\ncontinue") {
		t.Fatalf("selection = %q, want it to run from the anchor to the page end and on past the page number", app.selection.text)
	}
	if strings.Contains(app.selection.text, "2") || strings.Contains(app.selection.text, "alpha") {
		t.Fatalf("selection %q picked up text outside the reading-order span", app.selection.text)
	}
}

func TestDragExtractsSelectionOncePerFrame(t *testing.T) {
	app, screenX, lineY := testSelectionApp(t, "select this text")
	app.handleSDLMouseButton(&sdl.MouseButtonEvent{Type: sdl.EventMouseButtonDown, Button: uint8(sdl.ButtonLeft), Clicks: 1, X: screenX(70), Y: lineY})
	for _, px := range []float64{120, 160, 200} {
		app.handleSDLMouseMotion(&sdl.MouseMotionEvent{State: sdl.ButtonLMask, X: screenX(px), Y: lineY})
	}
	if app.selection.text != "" {
		t.Fatalf("motion extracted the selection %q before the frame", app.selection.text)
	}
	app.refreshStaleSelection()
	if !strings.Contains(app.selection.text, "select") || app.selection.stale {
		t.Fatalf("after the frame: text=%q stale=%v", app.selection.text, app.selection.stale)
	}
}
