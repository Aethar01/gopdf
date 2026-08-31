package viewer

import "math"

// viewStateEvents tracks the last values reported to plugins so page_changed
// and zoom_changed can be emitted from one place rather than from every site
// that moves the view.
type viewStateEvents struct {
	page       int
	scale      float64
	generation int
	known      bool
}

// resetViewStateEvents rebaselines tracking without emitting, so installing a
// document does not report a page or zoom change alongside document_opened.
func (a *App) resetViewStateEvents() {
	a.viewEvents = viewStateEvents{page: a.page, scale: a.scale, generation: a.generation, known: true}
}

// emitViewStateEvents reports page and zoom movement once per frame. Both are
// driven from many call sites and can change several times within one frame,
// so plugins observe the settled value rather than every intermediate step.
func (a *App) emitViewStateEvents() {
	if a.runtime == nil || a.pageCount == 0 {
		return
	}
	if !a.viewEvents.known || a.viewEvents.generation != a.generation {
		a.resetViewStateEvents()
		return
	}
	if a.page != a.viewEvents.page {
		previous := a.viewEvents.page
		a.viewEvents.page = a.page
		a.emitPluginEvent("page_changed", map[string]any{
			"page":          a.page + 1,
			"label":         a.pageLabel(a.page),
			"previous_page": previous + 1,
			"page_count":    a.pageCount,
		})
	}
	if !scalesEqual(a.scale, a.viewEvents.scale) {
		previous := a.viewEvents.scale
		a.viewEvents.scale = a.scale
		a.emitPluginEvent("zoom_changed", map[string]any{
			"scale":          a.scale,
			"previous_scale": previous,
			"percent":        a.scale * 100,
		})
	}
}

// scalesEqual ignores differences too small to be meaningful zoom changes,
// keeping continuous gestures from emitting an event for every pixel.
func scalesEqual(a, b float64) bool {
	return math.Abs(a-b) < 1e-9
}
