package viewer

import (
	"slices"
	"testing"

	"gopdf/internal/mupdf"
)

func TestHintLabelsAreUniqueAndShortest(t *testing.T) {
	if got := hintLabels(3, "ab"); !slices.Equal(got, []string{"aa", "ab", "ba"}) {
		t.Fatalf("hintLabels(3, ab) = %v", got)
	}
	if got := hintLabels(2, "asd"); !slices.Equal(got, []string{"a", "s"}) {
		t.Fatalf("hintLabels(2, asd) = %v", got)
	}
	// Duplicate and too few characters fall back sensibly.
	if got := hintLabels(2, "aa"); len(got[0]) != 1 || got[0] == got[1] {
		t.Fatalf("hintLabels with a one-letter alphabet = %v", got)
	}
}

func TestLinkHintsFollowTypedLabel(t *testing.T) {
	app := testLayoutApp(3)
	app.config.HintChars = "ab"
	app.winW, app.winH = 1000, 1000
	app.recomputeLayout(app.viewportSize())
	app.pageLinks = map[int][]mupdf.Link{0: {
		{Bounds: mupdf.Rect{X1: 50, Y1: 10}, Page: 1},
		{Bounds: mupdf.Rect{Y0: 20, X1: 50, Y1: 30}, Page: 2},
		{Bounds: mupdf.Rect{Y0: 40, X1: 50, Y1: 50}, Page: 1},
	}, 1: nil, 2: nil}
	app.startLinkHints()
	if app.hints == nil || len(app.hints.hints) < 3 {
		t.Fatalf("hints = %+v", app.hints)
	}
	app.handleHintToken("z") // not a hint character: ignored
	app.handleHintToken("a")
	app.handleHintToken("<BS>")
	app.handleHintToken("a")
	if app.hints == nil || app.hints.typed != "a" {
		t.Fatalf("typed = %+v", app.hints)
	}
	app.handleHintToken("b") // "ab" is the second link, to page 2
	if app.hints != nil {
		t.Fatal("hints still shown after a full label")
	}
	if app.page != 2 {
		t.Fatalf("page = %d, want 2", app.page)
	}

	app.startLinkHints()
	app.handleHintToken("<Esc>")
	if app.hints != nil {
		t.Fatal("Esc did not cancel hints")
	}
}
