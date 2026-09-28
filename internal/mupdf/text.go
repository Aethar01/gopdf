package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
*/
import "C"

import (
	"sort"
	"strings"
	"unicode"
	"unsafe"
)

// TextLayer is a page's text together with the position of every character,
// so byte ranges of Text can be mapped back to highlight quads.
//
// Lines within a block are joined by a space and blocks by a newline, so
// phrases wrapped across lines still match.
type TextLayer struct {
	Text    string
	chars   []layerChar
	offsets []int // byte offset in Text of each chars entry
}

type layerChar struct {
	quad Quad
	line int // -1 for separators inserted between lines and blocks
}

type pageChar struct {
	r     rune
	line  int
	block int
	quad  Quad
}

func (d *Document) TextLayer(page int) (*TextLayer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var result C.gopdf_char_result
	var cerr *C.char
	if ok := C.gopdf_extract_page_chars(d.handle, C.int(page), &result, &cerr); ok == 0 {
		return nil, consumeError("extract page chars", cerr)
	}
	defer C.gopdf_free_char_result(&result)
	var chars []pageChar
	if result.char_count > 0 && result.chars != nil {
		raw := unsafe.Slice(result.chars, int(result.char_count))
		chars = make([]pageChar, len(raw))
		for i, ch := range raw {
			chars[i] = pageChar{r: rune(ch.c), line: int(ch.line), block: int(ch.block), quad: copyQuad(ch.quad)}
		}
	}
	return newTextLayer(chars), nil
}

func newTextLayer(chars []pageChar) *TextLayer {
	var b strings.Builder
	layer := &TextLayer{
		chars:   make([]layerChar, 0, len(chars)),
		offsets: make([]int, 0, len(chars)),
	}
	add := func(r rune, c layerChar) {
		layer.offsets = append(layer.offsets, b.Len())
		layer.chars = append(layer.chars, c)
		b.WriteRune(r)
	}
	last := rune(0)
	for i, ch := range chars {
		if i > 0 && ch.line != chars[i-1].line {
			switch {
			case ch.block != chars[i-1].block:
				add('\n', layerChar{line: -1})
			case !unicode.IsSpace(last):
				add(' ', layerChar{line: -1})
			}
		}
		add(ch.r, layerChar{quad: ch.quad, line: ch.line})
		last = ch.r
	}
	layer.Text = b.String()
	return layer
}

// Quads returns one quad per line covered by the byte range [start, end) of
// Text, skipping separators.
func (t *TextLayer) Quads(start, end int) []Quad {
	var quads []Quad
	line := -1
	for i := sort.SearchInts(t.offsets, start); i < len(t.chars) && t.offsets[i] < end; i++ {
		c := t.chars[i]
		if c.line < 0 {
			line = -1
			continue
		}
		if c.line != line {
			quads = append(quads, c.quad)
			line = c.line
			continue
		}
		last := &quads[len(quads)-1]
		last.UR, last.LR = c.quad.UR, c.quad.LR
	}
	return quads
}
