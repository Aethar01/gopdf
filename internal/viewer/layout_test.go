package viewer

import (
	"image/color"
	"math"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
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
		documentState:   documentState{pageCount: 5, page: 0},
		viewStateFields: viewStateFields{zoom: 1, fitMode: "manual", renderMode: "continuous", dualPage: true, firstPageOffset: true},
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1},
		metricsService:  metricsService{pageMetrics: metrics},
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

	app, err := New("", rt, 0, nil, NewOptions{})
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

	if vertices[0].Color != (sdl.FColor{R: 1.0 / 255, G: 2.0 / 255, B: 3.0 / 255, A: 4.0 / 255}) {
		t.Fatalf("unexpected vertex color: %+v", vertices[0].Color)
	}
}

func TestSmoothWheelQueuesPreciseScrollDelta(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.SmoothScrollDampening = 0.35
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{X: 0.25, Y: 0.1})

	assertClose(t, app.scrollX, 0)
	assertClose(t, app.scrollY, 100)
	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll target")
	}
	assertClose(t, state.targetX, 16)
	assertClose(t, state.targetY, 93.6)
}

func TestSmoothWheelPrefersPreciseDeltaOverAccumulatedIntegerTicks(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.SmoothScrollDampening = 0.35
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{Y: 0.1, IntegerY: 1})

	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll target")
	}
	assertClose(t, state.targetY, 93.6)
}

func TestSmoothWheelUsesScrollStepForIntegerDelta(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.SmoothScrollDampening = 0.35
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{Y: 1, IntegerY: 1})

	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll target")
	}
	assertClose(t, state.targetY, 36)
}

func TestTrackpadSmoothingCanBeSelectedWithoutMouseWheelSmoothing(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.SmoothScrollSources = config.SmoothInputTrackpad
	app.config.SmoothScrollDampening = 0.35
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{Y: 0.1})
	if !app.smoothScrollActive() {
		t.Fatal("expected fractional trackpad input to animate")
	}
	app.cancelSmoothScroll()
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{Y: 1})
	assertClose(t, app.scrollY, 36)
	if app.smoothScrollActive() {
		t.Fatal("expected detented mouse-wheel input to remain immediate")
	}
}

func TestInvertSmoothScrollInvertsBothWheelAxes(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config.InvertSmoothScroll = true
	app.config.SmoothScrollDampening = 0.35
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollX = 32
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{X: 0.25, Y: 0.5})

	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll target")
	}
	assertClose(t, state.targetX, 16)
	assertClose(t, state.targetY, 132)
}

func TestDisabledSmoothScrollUsesDiscreteWheelPath(t *testing.T) {
	app := testLayoutApp(5)
	app.config.SmoothScrollSources = 0
	app.pageStep = 64
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleAnimatedMouseWheel(&sdl.MouseWheelEvent{Y: 1})

	assertClose(t, app.scrollY, 36)
	if app.smoothScrollActive() {
		t.Fatal("expected disabled smooth scrolling to leave no animation state")
	}
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
	app.handleSDLKeyUp(&sdl.KeyboardEvent{Key: sdl.KeycodeSpace})
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

func TestRowPageScreenOrigin(t *testing.T) {
	row := rowLayout{x: 20, y: 30, width: 200, height: 300, pageX: []float64{45}, pageY: []float64{65}}
	app := testLayoutApp(1)
	app.rows = []rowLayout{row}
	app.pageToRow = []int{0}
	app.winW = 500
	app.winH = 600
	app.contentW = 300
	app.contentH = 700
	app.scrollX = 10
	app.scrollY = 15

	x, y := app.rowPageScreenOrigin(row, 0)
	assertClose(t, x, 135)
	assertClose(t, y, 50)

	app.renderMode = "single"
	x, y = app.rowPageScreenOrigin(row, 0)
	assertClose(t, x, 165)
	assertClose(t, y, 170)
}

func testLayoutApp(pageCount int) *App {
	metrics := make([]pageMetrics, pageCount)
	for i := range metrics {
		metrics[i] = pageMetrics{bounds: mupdf.Rect{X1: 100, Y1: 200}, width: 100, height: 200}
	}
	return &App{
		documentState:   documentState{pageCount: pageCount},
		viewStateFields: viewStateFields{zoom: 1, fitMode: "manual", renderMode: "continuous", firstPageOffset: true},
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1, SmoothScrollSources: config.SmoothInputAll},
		metricsService:  metricsService{pageMetrics: metrics},
	}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("got %.3f, want %.3f", got, want)
	}
}

func TestActivateLinkHonorsDestinationPoint(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(1000, 100)
	app.activateLink(mupdf.Link{Page: 3, Y: 150, HasY: true})

	want := testLayoutApp(5)
	want.recomputeLayout(1000, 100)
	want.alignPageToDocumentPoint(3, 50, 150)

	assertClose(t, app.scrollY, want.scrollY)
	if app.page != 3 {
		t.Fatalf("page = %d, want 3", app.page)
	}

	top := testLayoutApp(5)
	top.recomputeLayout(1000, 100)
	top.alignPageToAnchor(3)
	if math.Abs(app.scrollY-top.scrollY) < 1 {
		t.Fatalf("link jump ignored destination y: scrollY=%.1f matches page top", app.scrollY)
	}
}

func TestLinkSchemeAllowed(t *testing.T) {
	schemes := config.Default().LinkSchemes
	for uri, want := range map[string]bool{
		"https://example.com":   true,
		"HTTP://example.com":    true,
		"mailto:a@example.com":  true,
		"file:///etc/passwd":    false,
		"javascript:alert(1)":   false,
		"relative/path.pdf":     false,
		"launch:calc.exe":       false,
		"https://[invalid-host": false,
	} {
		if got := linkSchemeAllowed(uri, schemes); got != want {
			t.Errorf("linkSchemeAllowed(%q) = %t, want %t", uri, got, want)
		}
	}
}

func TestLinkFollowsOnReleaseOverSameLink(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(1000, 1000)
	app.pageLinks = map[int][]mupdf.Link{0: {{Bounds: mupdf.Rect{X1: 100, Y1: 50}, Page: 2}}}
	x, y := app.rowPageScreenOrigin(app.rows[0], 0)
	click := func(kind sdl.EventType, dx, dy float64) {
		app.handleLinkButton(&sdl.MouseButtonEvent{Type: kind, Button: uint8(sdl.ButtonLeft), X: float32(x + dx), Y: float32(y + dy)})
	}

	click(sdl.EventMouseButtonDown, 10, 10)
	if app.page != 0 {
		t.Fatal("link followed on press")
	}
	click(sdl.EventMouseButtonUp, 400, 400)
	if app.page != 0 {
		t.Fatal("link followed after releasing elsewhere")
	}
	click(sdl.EventMouseButtonDown, 10, 10)
	click(sdl.EventMouseButtonUp, 20, 20)
	if app.page != 2 {
		t.Fatalf("page = %d, want link target 2", app.page)
	}
}

func TestHoveredLinkShowsTargetAndRestoresMessage(t *testing.T) {
	app := testLayoutApp(3)
	app.message = "ready"
	link := mupdf.Link{External: true, URI: "https://example.com"}

	if !app.setHoveredLink(link, true) || app.message != link.URI {
		t.Fatalf("hover message = %q, want %q", app.message, link.URI)
	}
	if app.setHoveredLink(link, true) {
		t.Fatal("re-hovering the same link reported a change")
	}
	app.setHoveredLink(mupdf.Link{}, false)
	if app.message != "ready" {
		t.Fatalf("message after leaving = %q, want restored %q", app.message, "ready")
	}

	app.setHoveredLink(mupdf.Link{Page: 1}, true)
	if app.message != "page 2" {
		t.Fatalf("internal link message = %q", app.message)
	}
	app.message = "copied 3 chars"
	app.setHoveredLink(mupdf.Link{}, false)
	if app.message != "copied 3 chars" {
		t.Fatalf("leaving a link clobbered a newer message: %q", app.message)
	}
}

func TestFitModesScaleToViewport(t *testing.T) {
	app := testLayoutApp(3) // 100x200pt pages
	for mode, want := range map[string]float64{"width": 10, "height": 4, "page": 4} {
		app.fitMode = mode
		app.recomputeLayout(1000, 800)
		if math.Abs(app.scale-want) > 1e-9 {
			t.Errorf("fit %s: scale = %v, want %v", mode, app.scale, want)
		}
	}
}

func TestToggleTrimMarginsLaysPagesOutByContent(t *testing.T) {
	doc, err := mupdf.Open(testpdf.WriteImage(t), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(1)
	app.doc = doc
	info, _ := doc.PageInfo(0)
	app.pageMetrics[0] = newPageMetrics(info)
	app.recomputeLayout(1000, 1000)

	app.toggleTrimMargins()
	for deadline := time.Now().Add(2 * time.Second); !app.contentBoxesLoaded() && time.Now().Before(deadline); {
		app.pollMetricUpdates()
		time.Sleep(time.Millisecond)
	}
	want := mupdf.Rect{X0: 100 - trimPadding, Y0: 100 - trimPadding, X1: 200 + trimPadding, Y1: 200 + trimPadding}
	if got := app.pageMetrics[0].bounds; got != want {
		t.Fatalf("trimmed bounds = %v, want %v", got, want)
	}
	assertClose(t, app.pageMetrics[0].width, 116)

	app.toggleTrimMargins()
	if got := app.pageMetrics[0].bounds; got != info.Bounds {
		t.Fatalf("untrimmed bounds = %v, want the full page", got)
	}
}
