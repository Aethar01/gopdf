package viewer

import (
	"slices"

	"gopdf/internal/config"
)

// fitMode is how the zoom follows the window: fitting the page, its width
// or its height, or not at all.
type fitMode uint8

const (
	fitPage fitMode = iota
	fitWidth
	fitHeight
	fitManual
)

var fitModeNames = [...]string{fitPage: "page", fitWidth: "width", fitHeight: "height", fitManual: "manual"}

func (m fitMode) String() string { return fitModeNames[m] }

// parseFitMode reads a fit mode's name, taking an unknown one as page.
func parseFitMode(name string) fitMode {
	return fitMode(max(0, slices.Index(fitModeNames[:], config.NormalizeFitMode(name))))
}

// renderMode is whether pages scroll past continuously or show one spread
// at a time.
type renderMode uint8

const (
	renderContinuous renderMode = iota
	renderSingle
)

var renderModeNames = [...]string{renderContinuous: "continuous", renderSingle: "single"}

func (m renderMode) String() string { return renderModeNames[m] }

// parseRenderMode reads a render mode's name, taking an unknown one as
// continuous.
func parseRenderMode(name string) renderMode {
	return renderMode(max(0, slices.Index(renderModeNames[:], config.NormalizeRenderMode(name))))
}
