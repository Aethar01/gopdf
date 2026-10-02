package viewer

import (
	"image"
	"log"

	textfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

// fallbackFace is the UI font, drawing each character it lacks, as in a
// Japanese filename, in an installed font that has it. The installed
// fonts are only searched once such a character turns up.
type fallbackFace struct {
	font.Face // the UI font
	families  []string
	aspect    textfont.Aspect
	size      int
	fontMap   *fontscan.FontMap
	scanned   bool                            // whether fontMap has been looked for
	byRune    map[rune]font.Face              // the face for each character the UI font lacks
	loaded    map[fontscan.Location]font.Face // the fallback fonts loaded
}

func newFallbackFace(face font.Face, families []string, aspect textfont.Aspect, size int) *fallbackFace {
	return &fallbackFace{Face: face, families: families, aspect: aspect, size: size}
}

// faceFor is the face that draws r: the UI font if it has r, otherwise an
// installed font that does, and otherwise the UI font's missing glyph.
func (f *fallbackFace) faceFor(r rune) font.Face {
	if _, ok := f.Face.GlyphAdvance(r); ok || r < 0x20 {
		return f.Face
	}
	if face, ok := f.byRune[r]; ok {
		return face
	}
	face := f.find(r)
	if f.byRune == nil {
		f.byRune = map[rune]font.Face{}
	}
	f.byRune[r] = face
	return face
}

// find loads an installed font that has r, or returns the UI font.
func (f *fallbackFace) find(r rune) font.Face {
	if !f.scanned {
		f.scanned = true
		fontMap, err := systemFontMap()
		if err != nil {
			log.Printf("UI font fallback: %v", err)
			return f.Face
		}
		f.fontMap = fontMap
	}
	if f.fontMap == nil {
		return f.Face
	}
	f.fontMap.SetQuery(fontscan.Query{Families: f.families, Aspect: f.aspect})
	found := f.fontMap.ResolveFace(r)
	if found == nil || found.Font == nil {
		return f.Face
	}
	location := f.fontMap.FontLocation(found.Font)
	if face, ok := f.loaded[location]; ok {
		return face
	}
	face, err := loadFontFileAt(location.File, f.size, int(location.Index))
	if err != nil {
		log.Printf("UI font fallback %s: %v", location.File, err)
		return f.Face
	}
	if _, ok := face.GlyphAdvance(r); !ok {
		closeFontFace(face)
		return f.Face
	}
	if f.loaded == nil {
		f.loaded = map[fontscan.Location]font.Face{}
	}
	f.loaded[location] = face
	return face
}

func (f *fallbackFace) Glyph(dot fixed.Point26_6, r rune) (image.Rectangle, image.Image, image.Point, fixed.Int26_6, bool) {
	return f.faceFor(r).Glyph(dot, r)
}

func (f *fallbackFace) GlyphBounds(r rune) (fixed.Rectangle26_6, fixed.Int26_6, bool) {
	return f.faceFor(r).GlyphBounds(r)
}

func (f *fallbackFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	return f.faceFor(r).GlyphAdvance(r)
}

// Kern kerns two characters drawn in the same font.
func (f *fallbackFace) Kern(r0, r1 rune) fixed.Int26_6 {
	face := f.faceFor(r0)
	if face != f.faceFor(r1) {
		return 0
	}
	return face.Kern(r0, r1)
}

func (f *fallbackFace) Close() error {
	for _, face := range f.loaded {
		closeFontFace(face)
	}
	f.loaded, f.byRune = nil, nil
	closeFontFace(f.Face)
	return nil
}
