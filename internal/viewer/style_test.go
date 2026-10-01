package viewer

import (
	"image/color"
	"path/filepath"
	"strings"
	"testing"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font/basicfont"
)

func testStyleApp(style string) *App {
	cfg := config.Default()
	cfg.Theme.StatusBarStyle = style
	cfg.StatusBarLeft = "{message}"
	cfg.StatusBarRight = "{page}/{total}"
	return &App{
		config:          cfg,
		viewStateFields: viewStateFields{statusBarShown: true},
		layoutState:     layoutState{winW: 800, winH: 600},
		documentState:   documentState{pageCount: 9},
		sdlState:        sdlState{fontFace: basicfont.Face7x13},
	}
}

func TestStatusBarStyleBarSpansTheBottom(t *testing.T) {
	app := testStyleApp("bar")
	l := app.statusLayout()
	h := float32(app.statusBarHeight())
	if l.pill || l.leftArea != (sdl.FRect{X: 0, Y: 600 - h, W: 800, H: h}) {
		t.Fatalf("bar layout = %+v", l)
	}
	if _, viewportH := app.viewportSize(); viewportH != 600-int(h) {
		t.Fatalf("viewport height = %d, want the window less the bar", viewportH)
	}
	if l.right != "1/9" || l.rightX+measureText(app.fontFace, "1/9") != 800-app.ipx(float64(app.config.Theme.StatusBarPadding)) {
		t.Fatalf("right text %q at %d", l.right, l.rightX)
	}
}

func TestStatusBarStylePillFloatsOverThePage(t *testing.T) {
	app := testStyleApp("pill")
	if _, viewportH := app.viewportSize(); viewportH != 600 {
		t.Fatalf("viewport height = %d, want the whole window under the pills", viewportH)
	}
	l := app.statusLayout()
	if !l.pill || l.leftArea.W != 0 {
		t.Fatalf("with no message the left pill should be hidden: %+v", l.leftArea)
	}
	if l.rightArea.W == 0 || l.rightArea.X+l.rightArea.W >= 800 || l.rightArea.Y+l.rightArea.H >= 600 {
		t.Fatalf("right pill %+v should float inside the window", l.rightArea)
	}

	app.message = "hello"
	if l := app.statusLayout(); l.leftArea.W == 0 || l.leftArea.X+l.leftArea.W >= l.rightArea.X {
		t.Fatalf("message pill %+v should sit left of %+v", l.leftArea, l.rightArea)
	}

	// A prompt widens the left pill up to the right one.
	app.mode = modeCommand
	app.input.Set("open")
	l = app.statusLayout()
	gap := l.rightArea.X - (l.leftArea.X + l.leftArea.W)
	if gap != float32(app.ipx(8)) {
		t.Fatalf("prompt pill %+v leaves a gap of %v before %+v", l.leftArea, gap, l.rightArea)
	}
	if app.statusTop() != int(l.leftArea.Y) {
		t.Fatalf("completion opens above %d, want the pill's top %v", app.statusTop(), l.leftArea.Y)
	}
	if pos, ok := app.inputPositionAt(float64(l.textX+app.promptStart()+measureText(app.fontFace, ":op")), float64(l.leftArea.Y+l.leftArea.H/2), false); !ok || pos != 2 {
		t.Fatalf("click in the prompt pill = %d, %v; want rune 2", pos, ok)
	}
}

func TestSplitHeader(t *testing.T) {
	for header, want := range map[string][2]string{
		"Help":                          {"Help", ""},
		"Outline (12)":                  {"Outline", "12"},
		"Outline /intro (3/12)":         {"Outline", "/intro  3/12"},
		"Press keys for zoom (Esc ...)": {"Press keys for zoom", "Esc ..."},
	} {
		if title, detail := splitHeader(header); title != want[0] || detail != want[1] {
			t.Errorf("splitHeader(%q) = %q, %q; want %q, %q", header, title, detail, want[0], want[1])
		}
	}
}

func TestShortListPanelFitsItsRows(t *testing.T) {
	app := testStyleApp("bar")
	view := app.createCoreListView("test", "Test", uiRowsFromStrings([]string{"a", "b"}), 70, 70)
	full, fullRows := app.modalListGeometry(70, 70)
	rect, rows := view.frameGeometry(app)
	if rows != 2 || rect.Y != full.Y || rect.H != full.H-float32((fullRows-2)*app.modalListRowHeight()) {
		t.Fatalf("two-row panel %+v (%d rows), full %+v (%d rows)", rect, rows, full, fullRows)
	}
	view.rows = nil
	if _, rows := view.frameGeometry(app); rows != 1 {
		t.Fatalf("empty list rows = %d, want one for its message", rows)
	}
}

func TestUIFontFallsBackWithWarning(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.ttf")
	regular, heading, warning := loadUIFonts(config.ThemeFont{Path: missing, Size: 13, Weight: 400}, 13)
	if regular == nil || heading == nil {
		t.Fatal("expected a fallback font")
	}
	if warning == nil || !strings.Contains(warning.Error(), "missing.ttf") {
		t.Fatalf("warning = %v, want it to name the font path", warning)
	}
	closeFontFace(regular)

	regular, _, warning = loadUIFonts(config.ThemeFont{Family: "No Such Font Family Anywhere", Size: 13, Weight: 400}, 13)
	if regular == nil || warning == nil || !strings.Contains(warning.Error(), "No Such Font Family Anywhere") {
		t.Fatalf("warning = %v, want it to name the missing family", warning)
	}
	closeFontFace(regular)
}

func TestRoundedShapeOutlinesLineUp(t *testing.T) {
	rect := sdl.FRect{X: 10, Y: 20, W: 100, H: 40}
	for _, radius := range []float32{0, 8, 50} {
		shape := newRoundedShape(rect, radius, 1)
		inner, outer := shape.outline(-1), shape.outline(1)
		if len(inner) != len(outer) {
			t.Fatalf("radius %v: outlines of %d and %d points cannot be joined", radius, len(inner), len(outer))
		}
		edge := shape.outline(0)
		minX, minY, maxX, maxY := edge[0].X, edge[0].Y, edge[0].X, edge[0].Y
		for _, p := range edge {
			minX, minY, maxX, maxY = min(minX, p.X), min(minY, p.Y), max(maxX, p.X), max(maxY, p.Y)
		}
		const eps = 0.01
		if minX < rect.X-eps || minY < rect.Y-eps || maxX > rect.X+rect.W+eps || maxY > rect.Y+rect.H+eps ||
			maxX-minX < rect.W-eps || maxY-minY < rect.H-eps {
			t.Fatalf("radius %v: outline spans %v,%v to %v,%v, want the rect %+v", radius, minX, minY, maxX, maxY, rect)
		}
	}
}

func TestIsLight(t *testing.T) {
	if !isLight(color.RGBA{R: 0xff, G: 0xff, B: 0xff}) || isLight(color.RGBA{R: 0x1f, G: 0x24, B: 0x20}) {
		t.Fatal("isLight misjudges white or the dark page")
	}
	if got := mixRGBA(color.RGBA{A: 0xff}, color.RGBA{R: 200, A: 0xff}, 0.5); got != (color.RGBA{R: 100, A: 0xff}) {
		t.Fatalf("mixRGBA = %v", got)
	}
}
