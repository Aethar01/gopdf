package viewer

import (
	"fmt"
	"strings"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type outlineMenuState struct {
	view                 *uiView
	expanded             map[int]bool
	visibleIndices       []int
	visibleQuery         string
	visibleExpandedCount int
}

func (a *App) toggleOutlineMenu() {
	if a.outlineMenu.view != nil && a.outlineMenu.view.visible {
		a.closeUIView(a.outlineMenu.view, false)
		return
	}
	a.closeAllUI()
	if a.outline == nil && a.doc != nil {
		outline, err := a.doc.Outline()
		if err == nil {
			a.outline = outline
		}
	}
	a.outlineMenu = outlineMenuState{expanded: map[int]bool{}}
	a.outlineMenu.view = a.newOutlineView()
	if len(a.outline) > 0 {
		a.outlineMenu.view.selected = a.outlineIndexForPage(a.page)
		for i, item := range a.outline {
			if item.HasChildren && item.Depth < a.config.OutlineInitialDepth {
				a.outlineMenu.expanded[i] = true
			}
		}
		selected := a.outlineMenu.view.selected
		for selected >= 0 && selected < len(a.outline) && a.outline[selected].Parent >= 0 {
			selected = a.outline[selected].Parent
			a.outlineMenu.expanded[selected] = true
		}
	}
	a.refreshOutlineView()
	a.showUIView(a.outlineMenu.view)
}

func (a *App) newOutlineView() *uiView {
	view := &uiView{
		id:            "core:outline",
		owner:         "core",
		modal:         true,
		searchable:    true,
		widthPercent:  a.config.OutlineWidthPercent,
		heightPercent: a.config.OutlineHeightPercent,
	}
	view.header = func(a *App, view *uiView, visible int) string {
		if view.searching || view.query != "" {
			return fmt.Sprintf("Outline /%s (%d/%d)", view.query, visible, len(a.outline))
		}
		return fmt.Sprintf("Outline (%d)", len(a.outline))
	}
	view.empty = func(_ *App, view *uiView) string {
		if view.query != "" {
			return "No matching outline entries"
		}
		return "No PDF outline found"
	}
	view.geometry = func(a *App) (sdl.FRect, int) { return a.outlineMenuGeometry() }
	view.onKey = func(a *App, e *sdl.KeyboardEvent) bool { return a.handleOutlineViewKey(view, e) }
	view.onMouseButton = func(a *App, e *sdl.MouseButtonEvent) bool { return a.handleOutlineViewMouseButton(view, e) }
	view.onMouseMotion = func(a *App, e *sdl.MouseMotionEvent) bool { return a.handleGenericUIViewMouseMotion(view, e) }
	view.onQueryChanged = func(a *App, _ *uiView) {
		a.invalidateVisibleOutlineIndices()
		a.refreshOutlineView()
	}
	return view
}

func (a *App) outlineIndexForPage(page int) int {
	best := 0
	bestPage := -1
	for i, item := range a.outline {
		if item.Page >= 0 && item.Page <= page && item.Page >= bestPage {
			best = i
			bestPage = item.Page
		}
	}
	return best
}

func (a *App) visibleOutlineIndices() []int {
	query := ""
	if a.outlineMenu.view != nil {
		query = strings.ToLower(strings.TrimSpace(a.outlineMenu.view.query))
	}
	expandedCount := len(a.outlineMenu.expanded)
	if a.outlineMenu.visibleIndices != nil && a.outlineMenu.visibleQuery == query && a.outlineMenu.visibleExpandedCount == expandedCount {
		return a.outlineMenu.visibleIndices
	}
	visible := make([]int, 0, len(a.outline))
	for i, item := range a.outline {
		if query != "" {
			if strings.Contains(strings.ToLower(item.Title), query) {
				visible = append(visible, i)
			}
			continue
		}
		show := true
		parent := item.Parent
		for parent >= 0 {
			if !a.outlineMenu.expanded[parent] {
				show = false
				break
			}
			parent = a.outline[parent].Parent
		}
		if show {
			visible = append(visible, i)
		}
	}
	a.outlineMenu.visibleIndices = visible
	a.outlineMenu.visibleQuery = query
	a.outlineMenu.visibleExpandedCount = expandedCount
	return visible
}

func (a *App) invalidateVisibleOutlineIndices() {
	a.outlineMenu.visibleIndices = nil
	a.outlineMenu.visibleQuery = ""
	a.outlineMenu.visibleExpandedCount = 0
}

func (a *App) refreshOutlineView() {
	view := a.outlineMenu.view
	if view == nil {
		return
	}
	visible := a.visibleOutlineIndices()
	rows := make([]uiRow, 0, len(visible))
	for _, index := range visible {
		item := a.outline[index]
		marker := "  "
		if item.HasChildren {
			marker = "+ "
			if a.outlineMenu.expanded[index] {
				marker = "- "
			}
		}
		indent := strings.Repeat("  ", item.Depth)
		text := indent + marker + strings.TrimSpace(item.Title)
		if strings.TrimSpace(text) == strings.TrimSpace(indent+marker) {
			text += "untitled"
		}
		secondary := ""
		if item.Page >= 0 {
			secondary = fmt.Sprintf("%d", item.Page+1)
		}
		rows = append(rows, uiRow{index: index, text: text, value: item.Title, secondary: secondary})
	}
	view.rows = rows
	view.selected = clampOutlineSelection(view.selected, visible)
	a.ensureUIViewSelectionVisible(view)
}

func clampOutlineSelection(selected int, visible []int) int {
	for _, index := range visible {
		if index == selected {
			return selected
		}
	}
	if len(visible) == 0 {
		return -1
	}
	return visible[0]
}

func (a *App) updateOutlineSearchQuery(query string) {
	if a.outlineMenu.view == nil {
		return
	}
	a.outlineMenu.view.query = query
	a.invalidateVisibleOutlineIndices()
	a.refreshOutlineView()
}

func (a *App) insertOutlineSearchText(text string) {
	if a.outlineMenu.view == nil || !a.outlineMenu.view.searching {
		return
	}
	a.updateOutlineSearchQuery(a.outlineMenu.view.query + text)
}

func (a *App) backspaceOutlineSearch() {
	if a.outlineMenu.view == nil || !a.outlineMenu.view.searching || a.outlineMenu.view.query == "" {
		return
	}
	runes := []rune(a.outlineMenu.view.query)
	a.updateOutlineSearchQuery(string(runes[:len(runes)-1]))
}

func (a *App) closeOutlineSearch() bool {
	if a.outlineMenu.view == nil || (!a.outlineMenu.view.searching && a.outlineMenu.view.query == "") {
		return false
	}
	a.outlineMenu.view.searching = false
	a.updateOutlineSearchQuery("")
	return true
}

func (a *App) ensureOutlineSelectionVisible() {
	a.refreshOutlineView()
}

func (a *App) moveOutlineSelection(delta int) {
	a.moveUIViewSelection(a.outlineMenu.view, delta)
}

func (a *App) scrollOutlineMenu(delta int) {
	if a.outlineMenu.view == nil {
		return
	}
	_, rows := a.outlineMenuGeometry()
	scrollUIView(a.outlineMenu.view, delta, rows)
}

func (a *App) startOutlineScrollbarDrag(x, y int) bool {
	return a.uiViewStartScrollbarDrag(a.outlineMenu.view, x, y)
}

func (a *App) dragOutlineScrollbar(y int) {
	a.uiViewDragScrollbar(a.outlineMenu.view, y)
}

func (a *App) activateSelectedOutline() {
	view := a.outlineMenu.view
	if view == nil || view.selected < 0 || view.selected >= len(a.outline) {
		return
	}
	item := a.outline[view.selected]
	if item.External {
		if item.URI == "" {
			return
		}
		if err := a.OpenExternal(item.URI); err != nil {
			a.message = err.Error()
			return
		}
		a.closeUIView(view, false)
		a.message = item.URI
		return
	}
	if item.Page < 0 {
		return
	}
	a.closeUIView(view, false)
	if item.HasX || item.HasY {
		x, y := item.X, item.Y
		bounds := a.pageMetrics[item.Page].bounds
		if !item.HasX {
			x = float64(bounds.X0+bounds.X1) / 2
		}
		if !item.HasY {
			y = float64(bounds.Y0)
		}
		a.alignPageToDocumentPoint(item.Page, x, y)
		return
	}
	a.alignPageToAnchor(item.Page)
}

func (a *App) collapseSelectedOutline() {
	view := a.outlineMenu.view
	if view == nil || view.selected < 0 || view.selected >= len(a.outline) {
		return
	}
	selected := view.selected
	if a.outline[selected].HasChildren && a.outlineMenu.expanded[selected] {
		delete(a.outlineMenu.expanded, selected)
		a.invalidateVisibleOutlineIndices()
		a.refreshOutlineView()
		return
	}
	if parent := a.outline[selected].Parent; parent >= 0 {
		view.selected = parent
		a.ensureOutlineSelectionVisible()
	}
}

func (a *App) expandSelectedOutline() {
	view := a.outlineMenu.view
	if view == nil || view.selected < 0 || view.selected >= len(a.outline) || !a.outline[view.selected].HasChildren {
		return
	}
	a.outlineMenu.expanded[view.selected] = true
	a.invalidateVisibleOutlineIndices()
	a.refreshOutlineView()
}

func (a *App) handleOutlineViewKey(view *uiView, e *sdl.KeyboardEvent) bool {
	if a.handleUIViewSearchKey(view, e) {
		return true
	}
	if e.Type != sdl.EventKeyDown || e.Repeat {
		return true
	}
	if e.Key == sdl.KeycodeDown {
		a.moveOutlineSelection(1)
		return true
	}
	if e.Key == sdl.KeycodeUp {
		a.moveOutlineSelection(-1)
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok {
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			wasSearching := view.searching
			a.runOutlineViewAction(action)
			if !wasSearching && view.searching && len([]rune(token)) == 1 {
				a.ignoreText = token
			}
		}
	}
	return true
}

func (a *App) runOutlineViewAction(action string) {
	switch action {
	case "scroll_down":
		a.moveOutlineSelection(1)
	case "scroll_up":
		a.moveOutlineSelection(-1)
	case "scroll_left":
		a.collapseSelectedOutline()
	case "scroll_right":
		a.expandSelectedOutline()
	case "confirm":
		a.activateSelectedOutline()
	case "search_prompt", "search_prompt_backward":
		if a.outlineMenu.view != nil && a.outlineMenu.view.searchable {
			a.outlineMenu.view.searching = true
			a.updateOutlineSearchQuery("")
		}
	case "close", "outline":
		if a.closeOutlineSearch() {
			return
		}
		a.closeActiveUI()
	default:
		a.runAction(action)
	}
}

func (a *App) handleOutlineViewMouseButton(view *uiView, e *sdl.MouseButtonEvent) bool {
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
		view.selected = item.index
		a.activateSelectedOutline()
		return true
	}
	rect, _ := view.frameGeometry(a)
	if !pointInRect(int(e.X), int(e.Y), rect) {
		a.closeUIView(view, true)
	}
	return true
}

func (a *App) outlineMenuGeometry() (sdl.FRect, int) {
	return a.modalListGeometry(a.config.OutlineWidthPercent, a.config.OutlineHeightPercent)
}

func (a *App) outlineMenuRowHeight() int {
	return a.modalListRowHeight()
}
