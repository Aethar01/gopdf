package mupdf

import "testing"

func TestRectWidthAndHeightUseCoordinateDifference(t *testing.T) {
	rect := Rect{X0: 10, Y0: 20, X1: 42.5, Y1: 65.25}

	if got, want := rect.Width(), 32.5; got != want {
		t.Fatalf("expected width %.2f, got %.2f", want, got)
	}
	if got, want := rect.Height(), 45.25; got != want {
		t.Fatalf("expected height %.2f, got %.2f", want, got)
	}
}

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
