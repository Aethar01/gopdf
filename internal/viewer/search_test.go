package viewer

import (
	"math"
	"reflect"
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

func TestSearchPageOrderWrapsFromStartPage(t *testing.T) {
	tests := []struct {
		name  string
		start int
		count int
		want  []int
	}{
		{name: "empty", start: 0, count: 0, want: []int{}},
		{name: "from middle", start: 2, count: 5, want: []int{2, 3, 4, 0, 1}},
		{name: "negative start clamps", start: -4, count: 3, want: []int{0, 1, 2}},
		{name: "too large start clamps", start: 9, count: 4, want: []int{3, 0, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchPageOrder(tt.start, tt.count); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("searchPageOrder(%d, %d) = %v, want %v", tt.start, tt.count, got, tt.want)
			}
		})
	}
}

func TestSearchStatusStrings(t *testing.T) {
	app := &App{}
	if got := app.searchStatusMessage(); got != "" {
		t.Fatalf("expected empty message without query, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "" {
		t.Fatalf("expected empty counter without query, got %q", got)
	}

	app.search = searchState{query: "needle", current: -1}
	if got := app.searchStatusMessage(); got != "search /needle" {
		t.Fatalf("expected pending search message, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "" {
		t.Fatalf("expected empty counter before current match, got %q", got)
	}

	app.search = searchState{query: "needle", current: 1, order: []searchHitRef{{page: 0}, {page: 2}, {page: 4}}}
	if got := app.searchStatusMessage(); got != "match 2/3 /needle" {
		t.Fatalf("expected active match message, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "[2/3]" {
		t.Fatalf("expected active match counter, got %q", got)
	}
}

func TestParseSearchQuery(t *testing.T) {
	query, options := parseSearchQuery("-ri -w foo.*bar")
	if query != "foo.*bar" || !options.regex || !options.caseInsensitive || !options.wholeWord || options.currentPageOnly {
		t.Fatalf("parseSearchQuery flags = %q, %+v", query, options)
	}
	query, options = parseSearchQuery("-- -plain text")
	if query != "-plain text" || options != (searchOptions{}) {
		t.Fatalf("parseSearchQuery escaped plain = %q, %+v", query, options)
	}
	query, options = parseSearchQuery("-unknown text")
	if query != "-unknown text" || options != (searchOptions{}) {
		t.Fatalf("parseSearchQuery unknown flag = %q, %+v", query, options)
	}
}

func TestWholeWordMatchUsesUnicodeWordCharacters(t *testing.T) {
	text := "foo foobar café_2 café"
	tests := []struct {
		start int
		end   int
		want  bool
	}{
		{start: 0, end: 3, want: true},
		{start: 4, end: 7, want: false},
		{start: 11, end: 16, want: false},
		{start: 19, end: len(text), want: true},
	}
	for _, tt := range tests {
		if got := isWholeWordMatch(text, tt.start, tt.end); got != tt.want {
			t.Fatalf("isWholeWordMatch(%d, %d) = %t, want %t", tt.start, tt.end, got, tt.want)
		}
	}
}

func TestMoveSearchReportsExpectedState(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(800, 600)

	app.moveSearch(1)
	if app.message != "no active search" {
		t.Fatalf("expected no active search message, got %q", app.message)
	}

	app.search = searchState{query: "needle", running: true}
	app.moveSearch(1)
	if app.message != "searching for needle" {
		t.Fatalf("expected running search message, got %q", app.message)
	}

	app.search = searchState{query: "needle"}
	app.moveSearch(1)
	if app.message != "no matches for needle" {
		t.Fatalf("expected no matches message, got %q", app.message)
	}

	app.search = searchState{query: "needle", current: -1, order: []searchHitRef{{page: 0}, {page: 1}, {page: 2}}}
	app.moveSearch(-1)
	if app.search.current != 2 || app.message != "match 3/3 /needle" {
		t.Fatalf("expected backward move to wrap to last match, current=%d message=%q", app.search.current, app.message)
	}

	app.moveSearch(1)
	if app.search.current != 0 || app.message != "match 1/3 /needle" {
		t.Fatalf("expected forward move to wrap to first match, current=%d message=%q", app.search.current, app.message)
	}
}

func TestRepeatSearchHonorsOriginalSearchDirection(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(800, 600)
	app.search = searchState{query: "needle", mode: searchModeBackward, current: 1, order: []searchHitRef{{page: 0}, {page: 1}, {page: 2}}}

	app.repeatSearch(true)
	if app.search.current != 0 {
		t.Fatalf("expected repeating backward search to move backward, got current=%d", app.search.current)
	}

	app.repeatSearch(false)
	if app.search.current != 1 {
		t.Fatalf("expected reversing backward search to move forward, got current=%d", app.search.current)
	}
}

func TestClearSearchResetsSearchState(t *testing.T) {
	app := &App{inputState: inputState{message: "match 1/2 /needle"}}
	app.search = searchState{
		query:      "needle",
		matches:    map[int][]mupdf.SearchHit{0: {{}}},
		order:      []searchHitRef{{page: 0}},
		current:    0,
		running:    true,
		generation: 10,
		mode:       searchModeBackward,
	}

	app.clearSearch()

	if app.search.query != "" || len(app.search.matches) != 0 || len(app.search.order) != 0 || app.search.current != -1 || app.search.running {
		t.Fatalf("expected clear search to reset state, got %+v", app.search)
	}
	if app.search.generation != 11 || app.search.mode != searchModeForward || app.message != "" {
		t.Fatalf("expected generation increment, forward mode, and empty message; search=%+v message=%q", app.search, app.message)
	}
}
