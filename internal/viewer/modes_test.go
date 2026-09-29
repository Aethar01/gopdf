package viewer

import "testing"

func TestModesParseTheirNames(t *testing.T) {
	for _, mode := range []fitMode{fitPage, fitWidth, fitHeight, fitManual} {
		if got := parseFitMode(mode.String()); got != mode {
			t.Errorf("parseFitMode(%q) = %v", mode, got)
		}
	}
	for _, mode := range []renderMode{renderContinuous, renderSingle} {
		if got := parseRenderMode(mode.String()); got != mode {
			t.Errorf("parseRenderMode(%q) = %v", mode, got)
		}
	}
	if parseFitMode(" Width ") != fitWidth || parseFitMode("sideways") != fitPage || parseRenderMode("scroll") != renderContinuous {
		t.Fatal("names are not normalised as config options are")
	}
}
