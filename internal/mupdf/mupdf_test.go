package mupdf

import "testing"

func TestClosedDocumentReturnsErrors(t *testing.T) {
	doc := &Document{}

	if _, err := doc.PageCount(); err == nil {
		t.Fatal("expected PageCount on closed document to fail")
	}
	if _, err := doc.Bounds(0); err == nil {
		t.Fatal("expected Bounds on closed document to fail")
	}
	if _, err := doc.PageLabel(0); err == nil {
		t.Fatal("expected PageLabel on closed document to fail")
	}
	if _, err := doc.Render(0, 1, 0, 8); err == nil {
		t.Fatal("expected Render on closed document to fail")
	}
	if _, err := doc.ExtractSelection(0, Point{}, Point{}); err == nil {
		t.Fatal("expected ExtractSelection on closed document to fail")
	}
	if _, err := doc.SearchPage(0, "needle"); err == nil {
		t.Fatal("expected SearchPage on closed document to fail")
	}
	if _, err := doc.PageText(0); err == nil {
		t.Fatal("expected PageText on closed document to fail")
	}
	if _, err := doc.Links(0); err == nil {
		t.Fatal("expected Links on closed document to fail")
	}
	if _, err := doc.Outline(); err == nil {
		t.Fatal("expected Outline on closed document to fail")
	}
}
