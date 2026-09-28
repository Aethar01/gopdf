package mupdf

import (
	"fmt"
	"image"
	"strings"
	"sync"
	"testing"

	"gopdf/internal/testpdf"
)

var wholePage = image.Rect(-1<<30, -1<<30, 1<<30, 1<<30)

func TestRenderClipsToTile(t *testing.T) {
	doc, err := Open(testpdf.Write(t, "tile"), OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	renderer, err := doc.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	info, err := doc.PageInfo(0)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := DeviceRect(info.Bounds, 2), image.Rect(0, 0, 1224, 1584); got != want {
		t.Fatalf("DeviceRect = %v, want %v", got, want)
	}
	tile := image.Rect(1024, 1024, 2048, 2048)
	rendered, err := renderer.Render(0, 2, tile, 8)
	if err != nil {
		t.Fatal(err)
	}
	defer rendered.Close()
	// The tile is clipped to the page's 1224x1584 device box.
	if rendered.X != 1024 || rendered.Y != 1024 || rendered.Image.Bounds().Dx() != 200 || rendered.Image.Bounds().Dy() != 560 {
		t.Fatalf("tile at (%d,%d) size %v", rendered.X, rendered.Y, rendered.Image.Bounds().Size())
	}
	empty, err := renderer.Render(0, 2, image.Rect(5000, 5000, 6000, 6000), 8)
	if err != nil {
		t.Fatal(err)
	}
	if empty.Image.Bounds().Dx() != 0 {
		t.Fatalf("tile outside the page rendered %v", empty.Image.Bounds())
	}
}

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
	doc, err := Open(testpdf.Write(t, "hello", "world"), OpenOptions{})
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
	doc, err := Open(testpdf.WritePages(t, pages...), OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	renderer, err := doc.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	// Walk forward twice so every slot is evicted and reloaded.
	for pass := 0; pass < 2; pass++ {
		for page := range pages {
			rendered, err := renderer.Render(page, 0.25, wholePage, 8)
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

func TestRenderersRunConcurrentlyWithDocumentCalls(t *testing.T) {
	pages := make([][]string, 8)
	for i := range pages {
		pages[i] = []string{fmt.Sprintf("page %d", i+1), "some more text to rasterise"}
	}
	doc, err := Open(testpdf.WritePages(t, pages...), OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for w := 0; w < 4; w++ {
		renderer, err := doc.NewRenderer()
		if err != nil {
			t.Fatal(err)
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer renderer.Close()
			for i := 0; i < 20; i++ {
				rendered, err := renderer.Render((w+i)%len(pages), 1, wholePage, 8)
				if err != nil {
					errs <- err
					return
				}
				rendered.Close()
			}
		}()
	}
	for i := 0; i < 20; i++ {
		if _, err := doc.ExtractSelection(i%len(pages), Point{}, Point{X: 612, Y: 792}); err != nil {
			t.Fatal(err)
		}
		if _, err := doc.TextLayer(i % len(pages)); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
}
