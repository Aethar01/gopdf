package viewer

import (
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"
)

func TestSearchDocumentPageOptions(t *testing.T) {
	doc, err := mupdf.Open(testpdf.Write(t, "the cat sat in the category", "a wrapped", "phrase here"), "")
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
