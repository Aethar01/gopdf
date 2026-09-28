package mupdf

import (
	"fmt"
	"strings"
	"testing"

	"gopdf/internal/testpdf"
)

func TestNewTextLayerJoinsLinesAndBlocks(t *testing.T) {
	layer := newTextLayer([]pageChar{
		{r: 'a', line: 0, block: 0},
		{r: 'b', line: 1, block: 0},
		{r: ' ', line: 1, block: 0},
		{r: 'c', line: 2, block: 0},
		{r: 'd', line: 3, block: 1},
	})
	if want := "a b c\nd"; layer.Text != want {
		t.Fatalf("Text = %q, want %q", layer.Text, want)
	}
}

func TestTextLayerQuadsMergePerLine(t *testing.T) {
	q := func(x0, x1 float64) Quad {
		return Quad{UL: Point{X: x0}, LL: Point{X: x0, Y: 10}, UR: Point{X: x1}, LR: Point{X: x1, Y: 10}}
	}
	layer := newTextLayer([]pageChar{
		{r: 'a', line: 0, quad: q(0, 5)},
		{r: 'b', line: 0, quad: q(5, 10)},
		{r: 'é', line: 1, quad: q(0, 5)},
		{r: 'c', line: 1, quad: q(5, 10)},
	})
	quads := layer.Quads(0, len(layer.Text))
	if len(quads) != 2 {
		t.Fatalf("got %d quads, want one per line: %+v", len(quads), quads)
	}
	if quads[0].UL.X != 0 || quads[0].UR.X != 10 {
		t.Fatalf("first line quad not merged: %+v", quads[0])
	}
	// "c" starts after the two-byte é.
	if got := layer.Quads(len("ab é"), len(layer.Text)); len(got) != 1 || got[0].UL.X != 5 {
		t.Fatalf("Quads for c = %+v", got)
	}
}

func TestTextLayerFromDocument(t *testing.T) {
	doc, err := Open(testpdf.Write(t, "hello", "world"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	layer, err := doc.TextLayer(0)
	if err != nil {
		t.Fatal(err)
	}
	if layer.Text != "hello world" {
		t.Fatalf("Text = %q", layer.Text)
	}
	if quads := layer.Quads(0, len(layer.Text)); len(quads) != 2 {
		t.Fatalf("got %d quads for two lines", len(quads))
	}
}

func TestPageCacheEvictionKeepsPagesUsable(t *testing.T) {
	pages := make([][]string, 3*pageCacheSize)
	for i := range pages {
		pages[i] = []string{fmt.Sprintf("page %d", i+1)}
	}
	doc, err := Open(testpdf.WritePages(t, pages...), "")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	// Walk forward twice so every slot is evicted and reloaded.
	for pass := 0; pass < 2; pass++ {
		for page := range pages {
			rendered, err := doc.Render(page, 0.25, 0, 8)
			if err != nil {
				t.Fatalf("render page %d: %v", page+1, err)
			}
			rendered.Close()
		}
	}
	sel, err := doc.ExtractSelection(0, Point{X: 0, Y: 0}, Point{X: 612, Y: 792})
	if err != nil {
		t.Fatal(err)
	}
	if want := "page 1"; !strings.Contains(sel.Text, want) {
		t.Fatalf("selection after eviction = %q, want %q", sel.Text, want)
	}
	info, err := doc.PageInfo(len(pages) - 1)
	if err != nil || info.Bounds.X1 != 612 {
		t.Fatalf("PageInfo = %+v, %v", info, err)
	}
}
