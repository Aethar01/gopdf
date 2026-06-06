package viewer

import (
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
