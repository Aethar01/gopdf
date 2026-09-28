package viewer

import (
	"math"
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"
)

func TestSearchDocumentPageOptions(t *testing.T) {
	doc, err := mupdf.Open(testpdf.Write(t, "the cat sat in the category", "a wrapped", "phrase here"), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()

	tests := []struct {
		query     string
		wantHits  int
		wantQuads int
	}{
		{query: "-w cat", wantHits: 1, wantQuads: 1},
		{query: "-i CAT", wantHits: 2, wantQuads: 2},
		{query: "-r wrapped\\s+phrase", wantHits: 1, wantQuads: 2},
		{query: "-w wrapped phrase", wantHits: 1, wantQuads: 2},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			query, options := parseSearchQuery(tt.query)
			re, err := compileSearchPattern(query, options)
			if err != nil {
				t.Fatal(err)
			}
			hits, err := searchDocumentPage(doc, 0, query, re, options)
			if err != nil {
				t.Fatal(err)
			}
			quads := 0
			for _, hit := range hits {
				quads += len(hit.Quads)
			}
			if len(hits) != tt.wantHits || quads != tt.wantQuads {
				t.Fatalf("got %d hits / %d quads, want %d / %d", len(hits), quads, tt.wantHits, tt.wantQuads)
			}
		})
	}
}

func TestFocusSearchCurrentCentersHitOnUnrenderedPage(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(1000, 100)
	quad := mupdf.Quad{UL: mupdf.Point{X: 40, Y: 140}, UR: mupdf.Point{X: 60, Y: 140}, LL: mupdf.Point{X: 40, Y: 160}, LR: mupdf.Point{X: 60, Y: 160}}
	app.search = searchState{
		query:   "needle",
		matches: map[int][]mupdf.SearchHit{3: {{Quads: []mupdf.Quad{quad}}}},
		order:   []searchHitRef{{page: 3, hit: 0}},
		current: 0,
	}

	app.focusSearchCurrent()

	x, y, ok := app.pageScreenOrigin(3)
	if !ok {
		t.Fatal("page 3 not laid out")
	}
	_, minY, _, maxY := app.quadScreenBounds(quad, 3, x, y)
	_, viewportH := app.viewportSize()
	if center := (minY + maxY) / 2; math.Abs(center-float64(viewportH)/2) > 1 {
		t.Fatalf("hit centre at y=%.1f, want viewport centre %.1f", center, float64(viewportH)/2)
	}
}
