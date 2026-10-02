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
	if l.bar != (sdl.FRect{X: 0, Y: 600 - h, W: 800, H: h}) {
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
	if l.bar.X == 0 || l.leftArea.W != 0 {
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
		"Outline /foo(bar) (3/10)":      {"Outline", "/foo(bar)  3/10"},
		"Outline /a (b) (1/2)":          {"Outline", "/a (b)  1/2"},
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

func TestIsLight(t *testing.T) {
	if !isLight(color.RGBA{R: 0xff, G: 0xff, B: 0xff}) || isLight(color.RGBA{R: 0x1f, G: 0x24, B: 0x20}) {
		t.Fatal("isLight misjudges white or the dark page")
	}
}

func TestBorderMaskIsTheShapesEdge(t *testing.T) {
	app := &App{}
	rect := boxShape{Shape: config.Shape{Kind: "rect"}}
	img, _ := app.rasterMask(maskSpec{shape: rect, w: 20, h: 10, border: 2, sides: config.SidesAll}, 0)
	for _, p := range []struct{ x, y, want int }{{0, 5, 255}, {1, 5, 255}, {2, 5, 0}, {10, 5, 0}, {10, 9, 255}, {19, 0, 255}} {
		if got := int(img.AlphaAt(p.x, p.y).A); got != p.want {
			t.Errorf("all sides: alpha at %d,%d = %d, want %d", p.x, p.y, got, p.want)
		}
	}
	img, _ = app.rasterMask(maskSpec{shape: rect, w: 20, h: 10, border: 1, sides: config.SideTop}, 0)
	for _, p := range []struct{ x, y, want int }{{10, 0, 255}, {0, 5, 0}, {10, 9, 0}} {
		if got := int(img.AlphaAt(p.x, p.y).A); got != p.want {
			t.Errorf("top only: alpha at %d,%d = %d, want %d", p.x, p.y, got, p.want)
		}
	}
}

func TestShadowMaskBlursPastTheBox(t *testing.T) {
	app := &App{}
	spec := maskSpec{shape: boxShape{Shape: config.Shape{Kind: "rect"}}, w: 40, h: 40, blur: 8}
	pad := blurMargin(spec.blur)
	img, _ := app.rasterMask(spec, pad)
	if img.Rect.Dx() != 40+2*int(pad) {
		t.Fatalf("mask width = %d, want the box and its blur", img.Rect.Dx())
	}
	centre, edge, corner := img.AlphaAt(int(pad)+20, int(pad)+20).A, img.AlphaAt(int(pad), int(pad)+20).A, img.AlphaAt(0, 0).A
	if centre < 250 || edge < 100 || edge > 155 || corner > 2 {
		t.Fatalf("blurred alpha: centre %d, edge %d, corner %d", centre, edge, corner)
	}
}

func TestCutShadowMaskIsHollow(t *testing.T) {
	app := &App{}
	shape := boxShape{Shape: config.Shape{Kind: "rect"}, radius: [4]float32{6, 6, 6, 6}}
	for _, c := range []struct {
		name string
		spec maskSpec
		want bool
	}{
		{"cut shadow", maskSpec{shape: shape, w: 600, h: 800, blur: 8, cut: true, cutY: -3}, true},
		{"shadow", maskSpec{shape: shape, w: 600, h: 800, blur: 8}, false},
		{"inverted corners", maskSpec{shape: shape, w: 600, h: 800, invert: true}, true},
	} {
		pad := blurMargin(c.spec.blur)
		key, sliceX, sliceY := maskSlices(c.spec, pad)
		img, _ := app.rasterMask(key, pad)
		if got := hollowMiddle(img, sliceX, sliceY); got != c.want {
			t.Errorf("%s: hollow = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestPathShapeMask(t *testing.T) {
	app := &App{}
	// A triangle filling the top left half of the box.
	shape := boxShape{Shape: config.Shape{Kind: "path", Path: "M0,0 L100%,0 L0,100% Z"}, scale: 1}
	img, _ := app.rasterMask(maskSpec{shape: shape, w: 20, h: 20}, 0)
	if img.AlphaAt(2, 2).A != 255 || img.AlphaAt(17, 17).A != 0 {
		t.Fatalf("triangle mask: %d at the top left, %d at the bottom right", img.AlphaAt(2, 2).A, img.AlphaAt(17, 17).A)
	}
}

func TestBoxShapeFitsRadii(t *testing.T) {
	app := &App{}
	st := config.Style{Shape: config.Opt[config.Shape]{V: config.Shape{Kind: "rect"}, Set: true}, Radius: config.Opt[config.Corners]{V: config.Corners{30, 30, 0, 0}, Set: true}}
	if got := app.boxShape(&st, sdl.Rect{W: 40, H: 100}).radius; got != [4]float32{20, 20, 0, 0} {
		t.Fatalf("radii %v should shrink to fit a 40 pixel width", got)
	}
	st.Shape.V.Kind = "pill"
	if got := app.boxShape(&st, sdl.Rect{W: 100, H: 24}).radius; got != [4]float32{12, 12, 12, 12} {
		t.Fatalf("pill radii = %v", got)
	}
}

func TestStatusElementPlacesTheBar(t *testing.T) {
	app := testStyleApp("bar")
	app.config.Theme.Elements[config.ElementStatus].Margin = config.Opt[config.Insets]{V: config.Insets{Right: 20, Bottom: 6, Left: 30}, Set: true}
	l := app.statusLayout()
	h := float32(app.statusBarHeight())
	if l.bar != (sdl.FRect{X: 30, Y: 600 - 6 - h, W: 750, H: h}) {
		t.Fatalf("bar = %+v", l.bar)
	}
	if _, viewportH := app.viewportSize(); viewportH != 600-6-int(h) {
		t.Fatalf("viewport height = %d, want the window less the bar and its margin", viewportH)
	}
	app.config.Theme.Elements[config.ElementStatus].Floating = config.Opt[bool]{V: true, Set: true}
	app.styles = nil // as applying a changed config does
	if _, viewportH := app.viewportSize(); viewportH != 600 {
		t.Fatalf("floating bar leaves a viewport %d high", viewportH)
	}
}

func TestStrokeMaskFollowsThePath(t *testing.T) {
	app := &App{}
	shape := boxShape{Shape: config.Shape{Kind: "path", Path: "M0,10 H100%"}, scale: 1}
	img, pad := app.rasterMask(maskSpec{shape: shape, w: 40, h: 20, stroke: 4}, 0)
	if pad < 2 {
		t.Fatalf("pad = %d, want room for half the stroke", pad)
	}
	at := func(x, y int) uint8 { return img.AlphaAt(x+int(pad), y+int(pad)).A }
	if at(20, 10) != 255 || at(20, 9) != 255 || at(20, 14) != 0 || at(20, 5) != 0 {
		t.Fatalf("stroke alpha across the line: %d %d %d %d", at(20, 5), at(20, 9), at(20, 10), at(20, 14))
	}
}

func TestPathReachingPastItsBoxIsPadded(t *testing.T) {
	app := &App{}
	// A tail pointing up out of the box.
	shape := boxShape{Shape: config.Shape{Kind: "path", Path: "M0,0 L5,-6 L10,0 H100% V100% H0 Z"}, scale: 1}
	img, pad := app.rasterMask(maskSpec{shape: shape, w: 20, h: 10}, 0)
	if pad < 6 {
		t.Fatalf("pad = %d, want the tail's height", pad)
	}
	if got := img.AlphaAt(5+int(pad), int(pad)-3).A; got < 200 {
		t.Fatalf("tail alpha = %d", got)
	}
}

func TestClipConvexKeepsWhatIsInside(t *testing.T) {
	square := []point32{{0, 0}, {10, 0}, {10, 10}, {0, 10}}
	// A diamond inside the square, wound either way, cuts its corners.
	diamond := []point32{{5, 0}, {10, 5}, {5, 10}, {0, 5}}
	for _, clip := range [][]point32{diamond, {diamond[3], diamond[2], diamond[1], diamond[0]}} {
		got := clipConvex(square, clip)
		var area float32
		for i, p := range got {
			q := got[(i+1)%len(got)]
			area += p.x*q.y - q.x*p.y
		}
		if area = max(area, -area) / 2; area < 49.9 || area > 50.1 {
			t.Fatalf("clipped to %v, area %v; want the diamond's 50", got, area)
		}
	}
	if got := clipConvex(square, []point32{{20, 20}, {30, 20}, {30, 30}, {20, 30}}); len(got) != 0 {
		t.Fatalf("a square clipped to one beside it = %v, want nothing", got)
	}
}

func TestMenuHeadFollowsPaddingAndHeader(t *testing.T) {
	app := testStyleApp("bar")
	rowHeight := app.modalListRowHeight()
	if head := app.modalListHeadHeight(); head != rowHeight {
		t.Fatalf("default head = %d, want a row's %d", head, rowHeight)
	}
	app.config.Theme.Elements[config.ElementPanel].Padding = config.Opt[config.Insets]{V: config.Insets{Top: 12, Bottom: 8}, Set: true}
	app.config.Theme.Elements[config.ElementHeader].Padding = config.Opt[config.Insets]{V: config.Insets{Top: 10, Right: 14, Bottom: 10, Left: 14}, Set: true}
	app.styles = nil
	want := 12 + app.uiLineHeight() + 20
	if head := app.modalListHeadHeight(); head != want {
		t.Fatalf("head = %d, want %d", head, want)
	}
	view := app.createCoreListView("test", "Test", uiRowsFromStrings([]string{"a", "b", "c"}), 70, 70)
	app.showUIView(view)
	rect, _ := view.contentGeometry(app)
	if row, ok := app.uiViewIndexAt(view, int(rect.X)+20, int(rect.Y)+want+1); !ok || row.index != 0 {
		t.Fatalf("just below the head = row %d, %v; want the first", row.index, ok)
	}
	if _, ok := app.uiViewIndexAt(view, int(rect.X)+20, int(rect.Y)+want-1); ok {
		t.Fatal("the head took a click meant for it")
	}
}

func TestUIFontFallsBackForMissingCharacters(t *testing.T) {
	regular, _, _ := loadUIFonts(config.ThemeFont{Size: 16, Weight: 400, Style: "normal"}, 16)
	defer closeFontFace(regular)
	face, ok := regular.(*fallbackFace)
	if !ok {
		t.Fatalf("UI font is %T, want a fallback face", regular)
	}
	if _, ok := face.Face.GlyphAdvance('漢'); ok {
		t.Skip("the UI font has CJK itself")
	}
	if advance, ok := regular.GlyphAdvance('漢'); !ok || advance == 0 {
		if face.faceFor('漢') == face.Face {
			t.Skip("no installed font has CJK")
		}
		t.Fatalf("no advance for a CJK character: %v %v", advance, ok)
	}
	if face.faceFor('a') != face.Face {
		t.Fatal("a character the UI font has came from a fallback")
	}
	// Measuring mixed text counts the fallback's widths.
	if w := measureText(regular, "a漢"); w <= measureText(regular, "a") {
		t.Fatalf("measured %d", w)
	}
}
