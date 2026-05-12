package viewer

import (
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/veandco/go-sdl2/sdl"
)

func TestRecomputeLayoutUsesRotatedPageDimensions(t *testing.T) {
	width, height := rotatedBoundsSize(mupdf.Rect{X1: 100, Y1: 200}, 90)
	assertClose(t, width, 200)
	assertClose(t, height, 100)
	metrics := make([]pageMetrics, 5)
	for i := range metrics {
		metrics[i] = pageMetrics{bounds: mupdf.Rect{X1: 100, Y1: 200}, width: width, height: height}
	}

	app := &App{
		pageCount:       5,
		page:            0,
		zoom:            1,
		fitMode:         "manual",
		renderMode:      "continuous",
		dualPage:        true,
		firstPageOffset: true,
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1},
		pageMetrics:     metrics,
	}

	app.recomputeLayout(1000, 1000)

	if len(app.rows) != 3 {
		t.Fatalf("expected first offset row plus two spreads, got %d rows", len(app.rows))
	}
	assertClose(t, app.rows[0].width, 200)
	assertClose(t, app.rows[0].height, 100)
	assertClose(t, app.rows[1].width, 400)
	assertClose(t, app.rows[1].height, 100)
	assertClose(t, app.rows[0].x, 100)
	assertClose(t, app.rows[1].x, 0)
}

func TestNewAllowsBlankViewerWithoutDocument(t *testing.T) {
	rt, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	app, err := New("", rt, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if app.doc != nil || app.docPath != "" || app.docName != "" {
		t.Fatalf("expected blank viewer without document, app=%+v", app)
	}
	if app.pageCount != 0 || len(app.pageMetrics) != 0 || len(app.rows) != 0 {
		t.Fatalf("expected no document layout, pageCount=%d metrics=%d rows=%d", app.pageCount, len(app.pageMetrics), len(app.rows))
	}
	if app.renderWorker != nil || app.searchWorker != nil {
		t.Fatalf("expected no document workers, render=%v search=%v", app.renderWorker, app.searchWorker)
	}
}

func TestSetRotationKeepsCurrentPage(t *testing.T) {
	app := testLayoutApp(5)
	app.renderMode = "continuous"
	app.page = 3

	if err := app.SetRotation(90); err != nil {
		t.Fatal(err)
	}

	if app.page != 3 {
		t.Fatalf("expected rotation to keep page 3, got %d", app.page)
	}
}

func TestSetRenderModeKeepsCurrentPage(t *testing.T) {
	app := testLayoutApp(5)
	app.renderMode = "single"
	app.page = 3
	app.recomputeLayout(1000, 1000)

	if err := app.SetRenderMode("continuous"); err != nil {
		t.Fatal(err)
	}

	if app.page != 3 {
		t.Fatalf("expected render mode change to keep page 3, got %d", app.page)
	}
}

func TestPageBackgroundVerticesUseRotatedPageCorners(t *testing.T) {
	vertices := pageBackgroundVertices(10, 20, mupdf.Rect{X1: 100, Y1: 100}, 1, 45, color.RGBA{R: 1, G: 2, B: 3, A: 4})

	assertClose(t, float64(vertices[0].Position.X), 80.711)
	assertClose(t, float64(vertices[0].Position.Y), 20)
	assertClose(t, float64(vertices[1].Position.X), 151.421)
	assertClose(t, float64(vertices[1].Position.Y), 90.711)
	assertClose(t, float64(vertices[2].Position.X), 10)
	assertClose(t, float64(vertices[2].Position.Y), 90.711)
	assertClose(t, float64(vertices[3].Position.X), 80.711)
	assertClose(t, float64(vertices[3].Position.Y), 161.421)

	if vertices[0].Color != (sdl.Color{R: 1, G: 2, B: 3, A: 4}) {
		t.Fatalf("unexpected vertex color: %+v", vertices[0].Color)
	}
}

func TestSmoothWheelUsesPreciseScrollDelta(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	if !app.handleSmoothWheel(0.25, 0.5) {
		t.Fatal("expected default wheel bindings to use smooth scrolling")
	}

	assertClose(t, app.scrollX, 16)
	assertClose(t, app.scrollY, 68)
}

func TestNaturalScrollInvertsVerticalWheelDelta(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.NaturalScroll = true
	app.mouseBindings = map[string]string{
		"wheel_up":   "scroll_up",
		"wheel_down": "scroll_down",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	if !app.handleSmoothWheel(0, 0.5) {
		t.Fatal("expected default wheel bindings to use smooth scrolling")
	}

	assertClose(t, app.scrollY, 132)
}

func TestPanCanBeHeldByKey(t *testing.T) {
	app := testLayoutApp(5)
	app.actionKey = " "

	if err := app.runBuiltinAction("pan"); err != nil {
		t.Fatal(err)
	}

	if !app.panning || app.panKey != " " || app.panButton != 0 {
		t.Fatalf("expected key pan state, panning=%v panKey=%q panButton=%d", app.panning, app.panKey, app.panButton)
	}
	app.handleSDLKeyUp(&sdl.KeyboardEvent{Keysym: sdl.Keysym{Sym: sdl.K_SPACE}})
	if app.panning {
		t.Fatal("expected key release to stop panning")
	}
}

func TestOpenCommandKeepsSpacesInPath(t *testing.T) {
	app := &App{}
	path := filepath.Join(t.TempDir(), "Lancer - Core Book.pdf")

	app.runCommand(":open " + path)

	if app.pendingOpen != path {
		t.Fatalf("expected pending open path %q, got %q", path, app.pendingOpen)
	}

	app = &App{}
	app.runCommand(":open " + escapeCommandPath(path))
	if app.pendingOpen != path {
		t.Fatalf("expected escaped pending open path %q, got %q", path, app.pendingOpen)
	}
}

func escapeCommandPath(path string) string {
	return strings.ReplaceAll(strings.ReplaceAll(path, `\`, `\\`), " ", `\ `)
}

func TestRenderTargetOversamplesZoom(t *testing.T) {
	app := testLayoutApp(1)
	app.scale = 2
	app.zoom = 2
	app.minRenderBaseScale = 0.25
	app.config.RenderOversample = 1

	assertClose(t, app.currentRenderTarget(), 2)
}

func TestRenderTargetAllowsUndersampling(t *testing.T) {
	app := testLayoutApp(1)
	app.scale = 2
	app.zoom = 2
	app.minRenderBaseScale = 0.25
	app.config.RenderOversample = 0.75

	assertClose(t, app.currentRenderTarget(), 1.5)
}

func testLayoutApp(pageCount int) *App {
	metrics := make([]pageMetrics, pageCount)
	for i := range metrics {
		metrics[i] = pageMetrics{bounds: mupdf.Rect{X1: 100, Y1: 200}, width: 100, height: 200}
	}
	return &App{
		pageCount:       pageCount,
		zoom:            1,
		fitMode:         "manual",
		renderMode:      "continuous",
		firstPageOffset: true,
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1},
		pageMetrics:     metrics,
	}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("got %.3f, want %.3f", got, want)
	}
}
