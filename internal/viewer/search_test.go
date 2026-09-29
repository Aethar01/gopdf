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

func TestSearchMatchesListsContextAndSelects(t *testing.T) {
	doc, err := mupdf.Open(testpdf.WritePages(t, []string{"first needle here"}, []string{"nothing"}, []string{"a second needle"}), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(3)
	app.doc = doc
	app.winW, app.winH = 1000, 800
	app.recomputeLayout(app.viewportSize())
	app.search = searchState{query: "needle", current: 0, matches: map[int][]mupdf.SearchHit{}}
	for page := range 3 {
		hits, err := searchDocumentPage(doc, page, "needle", nil, searchOptions{})
		if err != nil {
			t.Fatal(err)
		}
		app.search.matches[page] = hits
		for i := range hits {
			app.search.order = append(app.search.order, searchHitRef{page: page, hit: i})
		}
	}

	app.showSearchMatches()
	view := app.activeUIView()
	if view == nil || len(view.rows) != 2 {
		t.Fatalf("match list = %+v", view)
	}
	if view.rows[1].text != "a second needle" || view.rows[1].secondary != "p. 3" {
		t.Fatalf("second row = %+v", view.rows[1])
	}
	view.selected = 1
	app.activateUIView(view) // Enter on the second match
	if app.search.current != 1 || app.activeUIView() != nil || len(app.search.order) != 2 {
		t.Fatalf("after choosing: current=%d view=%v matches=%d", app.search.current, app.activeUIView(), len(app.search.order))
	}
	if app.page != 2 {
		t.Fatalf("page = %d after choosing the match on page 3", app.page+1)
	}
}

func TestOpeningPromptsKeepsSearch(t *testing.T) {
	app := testLayoutApp(1)
	app.search = searchState{query: "needle", order: []searchHitRef{{}}, matches: map[int][]mupdf.SearchHit{0: {{}}}}
	app.runAction("command_mode")
	app.runAction("close")
	if app.search.query != "needle" {
		t.Fatal("opening the command prompt cleared the search")
	}
}
