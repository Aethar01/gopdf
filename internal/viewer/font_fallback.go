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
	ascii     [128]fallbackRune               // how each ASCII character is drawn, once looked up
	runes     map[rune]fallbackRune           // how each other character is drawn, once looked up
	loaded    map[fontscan.Location]font.Face // the fallback fonts loaded
}

// fallbackRune is the face that draws a character and its advance there.
type fallbackRune struct {
	face    font.Face
	advance fixed.Int26_6
	ok      bool
}

func newFallbackFace(face font.Face, families []string, aspect textfont.Aspect, size int) *fallbackFace {
	return &fallbackFace{Face: face, families: families, aspect: aspect, size: size}
}

// faceFor is the face that draws r: the UI font if it has r, otherwise an
// installed font that does, and otherwise the UI font's missing glyph.
func (f *fallbackFace) faceFor(r rune) font.Face { return f.glyph(r).face }

// glyph is how r is drawn: its face, as faceFor says, and its advance
// there. Each character is looked up once, since text is measured over
// and over as it is laid out.
func (f *fallbackFace) glyph(r rune) fallbackRune {
	if r >= 0 && int(r) < len(f.ascii) {
		if g := f.ascii[r]; g.face != nil {
			return g
		}
	} else if g, ok := f.runes[r]; ok {
		return g
	}
	face := f.Face
	if _, ok := f.Face.GlyphAdvance(r); !ok && r >= 0x20 {
		face = f.find(r)
	}
	advance, ok := face.GlyphAdvance(r)
	g := fallbackRune{face: face, advance: advance, ok: ok}
	if r >= 0 && int(r) < len(f.ascii) {
		f.ascii[r] = g
	} else {
		if f.runes == nil {
			f.runes = map[rune]fallbackRune{}
		}
		f.runes[r] = g
	}
	return g
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
	g := f.glyph(r)
	return g.advance, g.ok
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
	f.loaded, f.runes, f.ascii = nil, nil, [128]fallbackRune{}
	closeFontFace(f.Face)
	return nil
}
