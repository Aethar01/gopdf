package viewer

import (
	"fmt"
	"image/color"
	"testing"
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
