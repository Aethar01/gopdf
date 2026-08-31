package viewer

import (
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) ShowUI(spec config.UIOverlay) error {
	callCallbacks := true
	if active := a.activeUIView(); active != nil && active.owner == "lua" && active.generation != spec.Generation {
		callCallbacks = false
	}
	a.closeAllUIWithCallbacks(callCallbacks)
	view := a.createLuaListView(spec)
	view.onKey = func(a *App, e *sdl.KeyboardEvent) bool { return a.handleGenericUIViewKey(view, e) }
	view.onMouseButton = func(a *App, e *sdl.MouseButtonEvent) bool { return a.handleGenericUIViewMouseButton(view, e) }
	view.onMouseMotion = func(a *App, e *sdl.MouseMotionEvent) bool { return a.handleGenericUIViewMouseMotion(view, e) }
	a.showUIView(view)
	return nil
}

func (a *App) CloseUI(id string) {
	view := a.views.views[id]
	a.closeUIView(view, true)
}

func (a *App) UIVisible(id string) bool {
	view := a.views.views[id]
	return view != nil && view.visible
}

func (a *App) SetUIRows(id string, rows []config.UIListRow) {
	view := a.views.views[id]
	if view == nil {
		return
	}
	a.setUIViewRows(view, uiRowsFromConfig(rows))
}

func (a *App) SetUISelected(id string, selected int) {
	view := a.views.views[id]
	if view == nil {
		return
	}
	if len(view.rows) == 0 {
		view.selected = -1
	} else {
		view.selected = clampInt(selected-1, 0, len(view.rows)-1)
	}
	a.ensureUIViewSelectionVisible(view)
	a.pendingRedraw = true
}

func (a *App) SetUIScroll(id string, scroll int) {
	view := a.views.views[id]
	if view == nil {
		return
	}
	_, rows := view.contentGeometry(a)
	view.scroll = clampInt(scroll, 0, max(0, len(view.visibleRows())-rows))
	a.pendingRedraw = true
}

func (a *App) SetUIQuery(id string, query string) {
	view := a.views.views[id]
	if view == nil {
		return
	}
	a.setUIViewQuery(view, query)
}

func (a *App) UISelected(id string) int {
	view := a.views.views[id]
	if view == nil {
		return 0
	}
	return view.selected + 1
}

func (a *App) UIScroll(id string) int {
	view := a.views.views[id]
	if view == nil {
		return 0
	}
	return view.scroll
}

func (a *App) UIQuery(id string) string {
	view := a.views.views[id]
	if view == nil {
		return ""
	}
	return view.query
}

func (a *App) handleGenericUIViewKey(view *uiView, e *sdl.KeyboardEvent) bool {
	if view == nil || e == nil {
		return true
	}
	if view.searchable && a.handleUIViewSearchKey(view, e) {
		return true
	}
	if e.Type != sdl.EventKeyDown || e.Repeat {
		return true
	}
	if e.Key == sdl.KeycodeDown {
		a.moveUIViewSelection(view, 1)
		return true
	}
	if e.Key == sdl.KeycodeUp {
		a.moveUIViewSelection(view, -1)
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok {
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			wasSearching := view.searching
			a.runUIViewAction(view, action)
			if !wasSearching && view.searching && len([]rune(token)) == 1 {
				a.ignoreText = token
			}
		}
	}
	return true
}

func (a *App) handleUIViewSearchKey(view *uiView, e *sdl.KeyboardEvent) bool {
	if e.Type != sdl.EventKeyDown {
		return true
	}
	if view.searching && e.Key == sdl.KeycodeBackspace {
		if view.query != "" {
			runes := []rune(view.query)
			a.setUIViewQuery(view, string(runes[:len(runes)-1]))
		}
		return true
	}
	if e.Repeat || !view.searching {
		return false
	}
	switch e.Key {
	case sdl.KeycodeEscape:
		view.searching = false
		a.setUIViewQuery(view, "")
		return true
	case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
		view.searching = false
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok && !strings.HasPrefix(token, "<") && len([]rune(token)) == 1 {
		return true
	}
	return false
}

func (a *App) runUIViewAction(view *uiView, action string) {
	if view == nil {
		return
	}
	switch action {
	case "scroll_down":
		a.moveUIViewSelection(view, 1)
	case "scroll_up":
		a.moveUIViewSelection(view, -1)
	case "scroll_left":
		a.scrollUIViewBy(view, -1)
	case "scroll_right":
		a.scrollUIViewBy(view, 1)
	case "search_prompt", "search_prompt_backward":
		if view.searchable {
			view.searching = true
			a.setUIViewQuery(view, "")
		}
	case "confirm":
		a.activateUIView(view)
	case "close":
		if view.searching || view.query != "" {
			view.searching = false
			a.setUIViewQuery(view, "")
			return
		}
		a.closeUIView(view, true)
	default:
		a.runAction(action)
	}
}

func (a *App) scrollUIViewBy(view *uiView, delta int) {
	if view == nil {
		return
	}
	_, rows := view.contentGeometry(a)
	scrollUIView(view, delta, rows)
	a.pendingRedraw = true
}

func (a *App) activateUIView(view *uiView) {
	if view == nil || view.onSelect == nil {
		return
	}
	for _, row := range view.visibleRows() {
		if row.index == view.selected && !row.disabled {
			view.onSelect(a, row)
			return
		}
	}
}

func (a *App) handleGenericUIViewMouseButton(view *uiView, e *sdl.MouseButtonEvent) bool {
	if view == nil || e == nil {
		return true
	}
	if e.Type == sdl.EventMouseButtonUp && e.Button == uint8(sdl.ButtonLeft) {
		view.draggingScrollbar = false
		return true
	}
	if e.Type != sdl.EventMouseButtonDown || e.Button != uint8(sdl.ButtonLeft) {
		return true
	}
	if a.uiViewStartScrollbarDrag(view, int(e.X), int(e.Y)) {
		return true
	}
	if item, ok := a.uiViewIndexAt(view, int(e.X), int(e.Y)); ok {
		if item.disabled {
			return true
		}
		view.selected = item.index
		a.activateUIView(view)
		return true
	}
	rect, _ := view.frameGeometry(a)
	if !pointInRect(int(e.X), int(e.Y), rect) {
		a.closeUIView(view, true)
	}
	return true
}

func (a *App) handleGenericUIViewMouseMotion(view *uiView, e *sdl.MouseMotionEvent) bool {
	if view == nil || e == nil {
		return true
	}
	if view.draggingScrollbar {
		old := view.scroll
		a.uiViewDragScrollbar(view, int(e.Y))
		return old != view.scroll
	}
	return a.uiViewHover(view, int(e.X), int(e.Y))
}
