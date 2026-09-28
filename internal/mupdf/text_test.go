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
	sel, err := doc.ExtractSelection(0, Point{X: 0, Y: 0}, Point{X: 612, Y: 792}, SelectChars)
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
		if _, err := doc.ExtractSelection(i%len(pages), Point{}, Point{X: 612, Y: 792}, SelectChars); err != nil {
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

func TestImageBounds(t *testing.T) {
	doc, err := Open(testpdf.WriteImage(t), OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	images, err := doc.ImageBounds(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Rect{X0: 100, Y0: 100, X1: 200, Y1: 200}); len(images) != 1 || images[0] != want {
		t.Fatalf("ImageBounds = %v, want [%v]", images, want)
	}
}

func TestContentBounds(t *testing.T) {
	doc, err := Open(testpdf.WriteImage(t), OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	got, err := doc.ContentBounds(0)
	if err != nil {
		t.Fatal(err)
	}
	if want := (Rect{X0: 100, Y0: 100, X1: 200, Y1: 200}); got != want {
		t.Fatalf("ContentBounds = %v, want %v", got, want)
	}
}

func TestAddHighlightRendersAndSaves(t *testing.T) {
	path := testpdf.Write(t, "hello")
	doc, err := Open(path, OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	quad := Quad{UL: Point{X: 72, Y: 80}, UR: Point{X: 150, Y: 80}, LL: Point{X: 72, Y: 95}, LR: Point{X: 150, Y: 95}}
	if err := doc.AddHighlight(0, []Quad{quad}, [3]uint8{255, 0, 0}); err != nil {
		t.Fatal(err)
	}
	copyPath := path + ".copy.pdf"
	if err := doc.Save(copyPath, path); err != nil {
		t.Fatal(err)
	}
	for _, d := range []*Document{doc, mustOpen(t, copyPath)} {
		renderer, err := d.NewRenderer()
		if err != nil {
			t.Fatal(err)
		}
		rendered, err := renderer.Render(0, 1, image.Rect(140, 85, 145, 90), 8)
		if err != nil {
			t.Fatal(err)
		}
		px := rendered.Image.RGBAAt(0, 0) // blank page under the highlight
		if px.R < 200 || px.G > 80 || px.B > 80 {
			t.Errorf("pixel under highlight = %v, want red", px)
		}
		rendered.Close()
		renderer.Close()
	}
}

func mustOpen(t *testing.T, path string) *Document {
	t.Helper()
	doc, err := Open(path, OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(doc.Close)
	return doc
}

func TestFormWidgets(t *testing.T) {
	path := testpdf.WriteForm(t)
	doc := mustOpen(t, path)

	text, ok, err := doc.WidgetAt(0, Point{X: 150, Y: 80})
	if err != nil || !ok || text.Kind != WidgetText || text.Value != "Ada" {
		t.Fatalf("text field = %+v, %v, %v", text, ok, err)
	}
	if err := doc.SetWidgetValue(0, text.Index, "Grace"); err != nil {
		t.Fatal(err)
	}
	check, _, _ := doc.WidgetAt(0, Point{X: 107, Y: 135})
	if check.Kind != WidgetCheckbox || check.Value != "Off" {
		t.Fatalf("check box = %+v", check)
	}
	if err := doc.ToggleWidget(0, check.Index); err != nil {
		t.Fatal(err)
	}
	choice, _, _ := doc.WidgetAt(0, Point{X: 150, Y: 180})
	options, err := doc.WidgetOptions(0, choice.Index)
	if choice.Kind != WidgetChoice || err != nil || len(options) != 2 || options[1] != "Green" {
		t.Fatalf("choice = %+v options=%q err=%v", choice, options, err)
	}
	if err := doc.SetWidgetValue(0, choice.Index, "Green"); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := doc.WidgetAt(0, Point{X: 400, Y: 400}); ok {
		t.Fatal("found a widget over empty page")
	}

	copyPath := path + ".filled.pdf"
	if err := doc.Save(copyPath, path); err != nil {
		t.Fatal(err)
	}
	saved := mustOpen(t, copyPath)
	for _, c := range []struct {
		at   Point
		want string
	}{{Point{X: 150, Y: 80}, "Grace"}, {Point{X: 107, Y: 135}, "Yes"}, {Point{X: 150, Y: 180}, "Green"}} {
		if w, _, _ := saved.WidgetAt(0, c.at); w.Value != c.want {
			t.Errorf("saved field at %v = %q, want %q", c.at, w.Value, c.want)
		}
	}
}

func TestUndoRedoHighlight(t *testing.T) {
	doc := mustOpen(t, testpdf.Write(t, "hello"))
	renderer, err := doc.NewRenderer()
	if err != nil {
		t.Fatal(err)
	}
	defer renderer.Close()
	red := func() bool {
		rendered, err := renderer.Render(0, 1, image.Rect(140, 85, 145, 90), 8)
		if err != nil {
			t.Fatal(err)
		}
		defer rendered.Close()
		return rendered.Image.RGBAAt(0, 0).G < 80
	}
	if err := doc.Undo(); err == nil || err.Error() != "nothing to undo" {
		t.Fatalf("Undo with no edits = %v", err)
	}
	quad := Quad{UL: Point{X: 72, Y: 80}, UR: Point{X: 150, Y: 80}, LL: Point{X: 72, Y: 95}, LR: Point{X: 150, Y: 95}}
	if err := doc.AddHighlight(0, []Quad{quad}, [3]uint8{255, 0, 0}); err != nil {
		t.Fatal(err)
	}
	if !red() {
		t.Fatal("highlight not drawn")
	}
	if err := doc.Undo(); err != nil || red() {
		t.Fatalf("after undo: err=%v highlighted=%v", err, red())
	}
	if err := doc.Redo(); err != nil || !red() {
		t.Fatalf("after redo: err=%v highlighted=%v", err, red())
	}
}

func TestUndoFormEdit(t *testing.T) {
	doc := mustOpen(t, testpdf.WriteForm(t))
	at := Point{X: 150, Y: 80}
	w, _, _ := doc.WidgetAt(0, at)
	if err := doc.SetWidgetValue(0, w.Index, "Grace"); err != nil {
		t.Fatal(err)
	}
	if err := doc.Undo(); err != nil {
		t.Fatal(err)
	}
	if w, _, _ := doc.WidgetAt(0, at); w.Value != "Ada" {
		t.Fatalf("value after undo = %q, want Ada", w.Value)
	}
}

func TestFindRecolourAndDeleteAnnotation(t *testing.T) {
	doc := mustOpen(t, testpdf.Write(t, "hello"))
	quad := Quad{UL: Point{X: 72, Y: 80}, UR: Point{X: 150, Y: 80}, LL: Point{X: 72, Y: 95}, LR: Point{X: 150, Y: 95}}
	if err := doc.AddHighlight(0, []Quad{quad}, [3]uint8{255, 0, 0}); err != nil {
		t.Fatal(err)
	}
	annot, ok, err := doc.AnnotationAt(0, Point{X: 100, Y: 90})
	if err != nil || !ok || annot.Type != "Highlight" {
		t.Fatalf("AnnotationAt = %+v, %v, %v", annot, ok, err)
	}
	if _, ok, _ := doc.AnnotationAt(0, Point{X: 100, Y: 200}); ok {
		t.Fatal("found an annotation outside the highlight")
	}
	if err := doc.RecolorAnnotation(0, annot.Index, [3]uint8{0, 0, 255}); err != nil {
		t.Fatal(err)
	}
	if err := doc.DeleteAnnotation(0, annot.Index); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := doc.AnnotationAt(0, Point{X: 100, Y: 90}); ok {
		t.Fatal("annotation still there after delete")
	}
	if err := doc.Undo(); err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := doc.AnnotationAt(0, Point{X: 100, Y: 90}); !ok {
		t.Fatal("undo did not restore the deleted annotation")
	}
}
