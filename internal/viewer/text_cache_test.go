package viewer

import (
	"fmt"
	"image/color"
	"strings"
	"testing"

	"golang.org/x/image/font/basicfont"
)

func TestTextTextureKeyIncludesTextAndColor(t *testing.T) {
	base := newTextTextureKey("hello", color.RGBA{R: 1, G: 2, B: 3, A: 4})
	if base == newTextTextureKey("world", color.RGBA{R: 1, G: 2, B: 3, A: 4}) {
		t.Fatal("text texture key ignored text")
	}
	if base == newTextTextureKey("hello", color.RGBA{R: 2, G: 2, B: 3, A: 4}) {
		t.Fatal("text texture key ignored color")
	}
}

func TestStoreTextTextureSurvivesAFullCache(t *testing.T) {
	var s sdlState
	for i := range maxTextTextureCacheEntries + 1 { // the last store empties a full cache
		s.storeTextTexture(newTextTextureKey(fmt.Sprint(i), color.Black), cachedTextTexture{})
	}
	if len(s.textCache) != 1 {
		t.Fatalf("entries after overflowing = %d, want 1", len(s.textCache))
	}
}

func TestTruncateTextFitsWidth(t *testing.T) {
	face := basicfont.Face7x13 // 7px per rune
	if got := truncateText(face, "short", 100); got != "short" {
		t.Fatalf("fitting text changed to %q", got)
	}
	if got := truncateText(face, "abcdefghijklmnop", 70); got != "abcdefg..." {
		t.Fatalf("truncated to %q, want 7 runes plus ellipsis in 70px", got)
	}
	if got := truncateText(face, "abcdef", 10); got != "a..." {
		t.Fatalf("narrow truncation = %q, want at least one rune", got)
	}
}

func BenchmarkTruncateLongRow(b *testing.B) {
	row := strings.Repeat("a long line of matched context text ", 8)
	for b.Loop() {
		truncateText(basicfont.Face7x13, row, 400)
	}
}

func TestFitStatusTextNeverOverlaps(t *testing.T) {
	face := basicfont.Face7x13
	left, right := "a fairly long status message", "12/100 continuous fit=page"
	for _, width := range []int{400, 200, 120, 60} {
		l, r := fitStatusText(face, left, right, width, 16, false)
		if w := measureText(face, l) + 16 + measureText(face, r); r != "" && w > width {
			t.Errorf("width %d: %q + %q is %dpx", width, l, r, w)
		}
	}
	if l, r := fitStatusText(face, ":open some/long/path.pdf", right, 200, 16, true); l != ":open some/long/path.pdf" || r != "" {
		t.Errorf("prompt: %q, %q; want the prompt whole and the right side hidden", l, r)
	}
}
