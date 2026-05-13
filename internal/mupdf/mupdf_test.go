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
