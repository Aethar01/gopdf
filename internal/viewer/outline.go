package viewer

import (
	"fmt"
	"strings"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type outlineMenuState struct {
	visible              bool
	selected             int
	scroll               int
	expanded             map[int]bool
	draggingScrollbar    bool
	scrollbarDragOffsetY int
	searching            bool
	query                string
}

func (a *App) toggleOutlineMenu() {
	if a.outlineMenu.visible {
		a.outlineMenu.visible = false
		return
	}
	a.closeAllUI()
	a.outlineMenu.visible = true
	a.outlineMenu.selected = -1
	a.outlineMenu.scroll = 0
	a.outlineMenu.expanded = map[int]bool{}
	if len(a.outline) == 0 {
		return
	}
	a.outlineMenu.selected = a.outlineIndexForPage(a.page)
	for i, item := range a.outline {
		if item.HasChildren && item.Depth < a.config.OutlineInitialDepth {
			a.outlineMenu.expanded[i] = true
		}
	}
	for parent := a.outline[a.outlineMenu.selected].Parent; parent >= 0; parent = a.outline[parent].Parent {
		a.outlineMenu.expanded[parent] = true
	}
	a.ensureOutlineSelectionVisible()
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
	visible := make([]int, 0, len(a.outline))
	query := strings.ToLower(strings.TrimSpace(a.outlineMenu.query))
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
	return visible
}

func (a *App) selectedVisibleOutlineRow(visible []int) int {
	for i, index := range visible {
		if index == a.outlineMenu.selected {
			return i
		}
	}
	if len(visible) == 0 {
		return 0
	}
	a.outlineMenu.selected = visible[clampInt(a.outlineMenu.scroll, 0, len(visible)-1)]
	return clampInt(a.outlineMenu.scroll, 0, len(visible)-1)
}

func (a *App) updateOutlineSearchQuery(query string) {
	a.outlineMenu.query = query
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		a.outlineMenu.selected = -1
		a.outlineMenu.scroll = 0
		return
	}
	a.outlineMenu.selected = visible[0]
	a.outlineMenu.scroll = 0
	a.ensureOutlineSelectionVisible()
}

func (a *App) insertOutlineSearchText(text string) {
	if !a.outlineMenu.visible || !a.outlineMenu.searching {
		return
	}
	a.updateOutlineSearchQuery(a.outlineMenu.query + text)
}

func (a *App) backspaceOutlineSearch() {
	if !a.outlineMenu.visible || !a.outlineMenu.searching || a.outlineMenu.query == "" {
		return
	}
	runes := []rune(a.outlineMenu.query)
	a.updateOutlineSearchQuery(string(runes[:len(runes)-1]))
}

func (a *App) closeOutlineSearch() bool {
	if !a.outlineMenu.searching && a.outlineMenu.query == "" {
		return false
	}
	a.outlineMenu.searching = false
	a.updateOutlineSearchQuery("")
	return true
}

func (a *App) ensureOutlineSelectionVisible() {
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		a.outlineMenu.scroll = 0
		return
	}
	_, rows := a.outlineMenuGeometry()
	if rows < 1 {
		rows = 1
	}
	row := a.selectedVisibleOutlineRow(visible)
	if row < a.outlineMenu.scroll {
		a.outlineMenu.scroll = row
	}
	if row >= a.outlineMenu.scroll+rows {
		a.outlineMenu.scroll = row - rows + 1
	}
	maxScroll := max(0, len(visible)-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll, 0, maxScroll)
}

func (a *App) moveOutlineSelection(delta int) {
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		return
	}
	row := a.selectedVisibleOutlineRow(visible)
	row = clampInt(row+delta, 0, len(visible)-1)
	a.outlineMenu.selected = visible[row]
	a.ensureOutlineSelectionVisible()
}

func (a *App) scrollOutlineMenu(delta int) {
	_, rows := a.outlineMenuGeometry()
	maxScroll := max(0, len(a.visibleOutlineIndices())-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll+delta, 0, maxScroll)
}

func (a *App) startOutlineScrollbarDrag(x, y int) bool {
	rect, rows := a.outlineMenuGeometry()
	rowHeight := a.outlineMenuRowHeight()
	visible := a.visibleOutlineIndices()
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, len(visible), a.outlineMenu.scroll)
	if !ok || !pointInRect(x, y, track) {
		return false
	}
	if pointInRect(x, y, thumb) {
		a.outlineMenu.scrollbarDragOffsetY = int(float32(y) - thumb.Y)
	} else {
		a.outlineMenu.scrollbarDragOffsetY = int(thumb.H / 2)
		a.outlineMenu.scroll = modalListScrollbarScrollForY(track, thumb, rows, len(visible), y, a.outlineMenu.scrollbarDragOffsetY)
	}
	a.outlineMenu.draggingScrollbar = true
	return true
}

func (a *App) dragOutlineScrollbar(y int) {
	rect, rows := a.outlineMenuGeometry()
	rowHeight := a.outlineMenuRowHeight()
	visible := a.visibleOutlineIndices()
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, len(visible), a.outlineMenu.scroll)
	if !ok {
		return
	}
	a.outlineMenu.scroll = modalListScrollbarScrollForY(track, thumb, rows, len(visible), y, a.outlineMenu.scrollbarDragOffsetY)
}

func (a *App) activateSelectedOutline() {
	if a.outlineMenu.selected < 0 || a.outlineMenu.selected >= len(a.outline) {
		return
	}
	item := a.outline[a.outlineMenu.selected]
	if item.Page < 0 {
		return
	}
	a.outlineMenu.visible = false
	a.alignPageTop(item.Page)
}

func (a *App) collapseSelectedOutline() {
	selected := a.outlineMenu.selected
	if selected < 0 || selected >= len(a.outline) {
		return
	}
	if a.outline[selected].HasChildren && a.outlineMenu.expanded[selected] {
		delete(a.outlineMenu.expanded, selected)
		a.ensureOutlineSelectionVisible()
		return
	}
	if parent := a.outline[selected].Parent; parent >= 0 {
		a.outlineMenu.selected = parent
		a.ensureOutlineSelectionVisible()
	}
}

func (a *App) expandSelectedOutline() {
	selected := a.outlineMenu.selected
	if selected < 0 || selected >= len(a.outline) || !a.outline[selected].HasChildren {
		return
	}
	a.outlineMenu.expanded[selected] = true
	a.ensureOutlineSelectionVisible()
}

func (a *App) handleOutlineMenuKey(e *sdl.KeyboardEvent) bool {
	if e.Type != sdl.EventKeyDown || e.Repeat {
		return true
	}
	if a.outlineMenu.searching {
		switch e.Key {
		case sdl.KeycodeBackspace:
			a.backspaceOutlineSearch()
			return true
		case sdl.KeycodeEscape:
			a.closeOutlineSearch()
			return true
		case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
			a.outlineMenu.searching = false
			return true
		}
		if token, ok := keyToken(e.Key, e.Mod); ok && !strings.HasPrefix(token, "<") && len([]rune(token)) == 1 {
			return true
		}
	}
	if token, ok := keyToken(e.Key, e.Mod); ok {
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			prevMode := a.mode
			a.runOutlineMenuAction(action)
			if (action == "search_prompt" || action == "search_prompt_backward" || prevMode == modeNormal && a.mode != modeNormal) && len([]rune(token)) == 1 {
				a.ignoreText = token
			}
		}
	}
	return true
}

func (a *App) runOutlineMenuAction(action string) {
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
		a.outlineMenu.searching = true
		a.updateOutlineSearchQuery("")
	case "close", "outline":
		if a.closeOutlineSearch() {
			return
		}
		a.closeActiveUI()
	default:
		a.runAction(action)
	}
}

func (a *App) clickOutlineMenu(x, y int) {
	if a.startOutlineScrollbarDrag(x, y) {
		return
	}
	rect, rows := a.outlineMenuGeometry()
	rowHeight := a.outlineMenuRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
			a.outlineMenu.visible = false
		}
		return
	}
	visible := a.visibleOutlineIndices()
	index := a.outlineMenu.scroll + row
	if index < 0 || index >= len(visible) {
		return
	}
	a.outlineMenu.selected = visible[index]
	a.activateSelectedOutline()
}

func (a *App) hoverOutlineMenu(x, y int) {
	rect, rows := a.outlineMenuGeometry()
	rowHeight := a.outlineMenuRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		return
	}
	visible := a.visibleOutlineIndices()
	index := a.outlineMenu.scroll + row
	if index < 0 || index >= len(visible) {
		return
	}
	a.outlineMenu.selected = visible[index]
}

func (a *App) outlineMenuGeometry() (sdl.FRect, int) {
	return a.modalListGeometry(a.config.OutlineWidthPercent, a.config.OutlineHeightPercent)
}

func (a *App) outlineMenuRowHeight() int {
	return a.modalListRowHeight()
}

func (a *App) drawOutlineMenu(renderer *sdl.Renderer) error {
	rect, rows := a.outlineMenuGeometry()
	if err := a.drawModalListFrame(renderer, rect); err != nil {
		return err
	}
	rowHeight := a.outlineMenuRowHeight()
	baselineOffset := a.modalListBaselineOffset(rowHeight)
	visible := a.visibleOutlineIndices()
	header := fmt.Sprintf(" Outline (%d)", len(a.outline))
	if a.outlineMenu.searching || a.outlineMenu.query != "" {
		header = fmt.Sprintf(" Outline /%s (%d/%d)", a.outlineMenu.query, len(visible), len(a.outline))
	}
	if err := drawText(renderer, a.fontFace, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	if len(visible) == 0 {
		text := "No PDF outline found"
		if a.outlineMenu.query != "" {
			text = "No matching outline entries"
		}
		if err := drawText(renderer, a.fontFace, text, int(rect.X)+16, int(rect.Y)+rowHeight+baselineOffset, a.foregroundColor()); err != nil {
			return err
		}
		return nil
	}
	maxScroll := max(0, len(visible)-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll, 0, maxScroll)
	for row := 0; row < rows; row++ {
		visibleIndex := a.outlineMenu.scroll + row
		if visibleIndex >= len(visible) {
			break
		}
		outlineIndex := visible[visibleIndex]
		item := a.outline[outlineIndex]
		y := int(rect.Y) + rowHeight + row*rowHeight
		if outlineIndex == a.outlineMenu.selected {
			if err := a.drawModalListSelection(renderer, rect, y, rowHeight); err != nil {
				return err
			}
		}
		marker := "  "
		if item.HasChildren {
			marker = "+ "
			if a.outlineMenu.expanded[outlineIndex] {
				marker = "- "
			}
		}
		page := ""
		if item.Page >= 0 {
			page = fmt.Sprintf("%d", item.Page+1)
		}
		indent := strings.Repeat("  ", item.Depth)
		text := indent + marker + strings.TrimSpace(item.Title)
		if strings.TrimSpace(text) == strings.TrimSpace(indent+marker) {
			text += "untitled"
		}
		pageW := measureText(a.fontFace, page)
		maxTextW := int(rect.W) - 36 - pageW
		text = a.truncateModalListText(text, maxTextW)
		clr := a.foregroundColor()
		if outlineIndex == a.outlineMenu.selected {
			clr = a.highlightForegroundColor()
		}
		if err := drawText(renderer, a.fontFace, text, int(rect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
		if page != "" {
			if err := drawText(renderer, a.fontFace, page, int(rect.X+rect.W)-16-pageW, y+baselineOffset, clr); err != nil {
				return err
			}
		}
	}
	if err := a.drawModalListScrollbar(renderer, rect, rowHeight, rows, len(visible), a.outlineMenu.scroll); err != nil {
		return err
	}
	return nil
}
