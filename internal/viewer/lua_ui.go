package viewer

import (
	"fmt"
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type luaUIState struct {
	visible              bool
	title                string
	rows                 []string
	selected             int
	scroll               int
	draggingScrollbar    bool
	scrollbarDragOffsetY int
	searching            bool
	query                string
	onSelect             string
	onClose              string
	onSelectBuiltin      func(string)
}

func (a *App) ShowUI(overlay config.UIOverlay) error {
	a.closeAllUI()
	a.luaUI = luaUIState{
		visible:  true,
		title:    strings.TrimSpace(overlay.Title),
		rows:     append([]string(nil), overlay.Rows...),
		selected: clampInt(overlay.Selected-1, 0, max(0, len(overlay.Rows)-1)),
		onSelect: overlay.OnSelect,
		onClose:  overlay.OnClose,
	}
	a.ensureLuaUISelectionVisible()
	a.pendingRedraw = true
	return nil
}

func (a *App) CloseUI() {
	a.closeLuaUI(false)
}

func (a *App) UIVisible() bool {
	return a.luaUI.visible
}

func (a *App) SetUIRows(rows []string) {
	a.luaUI.rows = append([]string(nil), rows...)
	a.luaUI.selected = clampInt(a.luaUI.selected, -1, max(-1, len(rows)-1))
	visible := a.visibleLuaUIIndices()
	if len(visible) == 0 {
		a.luaUI.selected = -1
		a.luaUI.scroll = 0
	} else {
		found := false
		for _, index := range visible {
			if index == a.luaUI.selected {
				found = true
				break
			}
		}
		if !found {
			a.luaUI.selected = visible[0]
		}
		a.ensureLuaUISelectionVisible()
	}
	a.pendingRedraw = true
}

func (a *App) SetUISelected(selected int) {
	a.luaUI.selected = clampInt(selected-1, 0, max(0, len(a.luaUI.rows)-1))
	a.ensureLuaUISelectionVisible()
	a.pendingRedraw = true
}

func (a *App) handleLuaUIKey(e *sdl.KeyboardEvent) bool {
	if e.Type != sdl.EventKeyDown || e.Repeat {
		return true
	}
	if a.luaUI.searching {
		switch e.Key {
		case sdl.KeycodeBackspace:
			a.backspaceLuaUISearch()
			return true
		case sdl.KeycodeEscape:
			a.closeLuaUISearch()
			return true
		case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
			a.luaUI.searching = false
			return true
		}
		if token, ok := keyToken(e.Key, e.Mod); ok && !strings.HasPrefix(token, "<") && len([]rune(token)) == 1 {
			return true
		}
	}
	switch e.Key {
	case sdl.KeycodeDown:
		a.moveLuaUISelection(1)
		return true
	case sdl.KeycodeUp:
		a.moveLuaUISelection(-1)
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok {
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			prevMode := a.mode
			a.runLuaUIAction(action)
			if (action == "search_prompt" || action == "search_prompt_backward" || prevMode == modeNormal && a.mode != modeNormal) && len([]rune(token)) == 1 {
				a.ignoreText = token
			}
		}
	}
	return true
}

func (a *App) runLuaUIAction(action string) {
	switch action {
	case "scroll_down":
		a.moveLuaUISelection(1)
	case "scroll_up":
		a.moveLuaUISelection(-1)
	case "confirm":
		a.activateLuaUISelection()
	case "search_prompt", "search_prompt_backward":
		a.luaUI.searching = true
		a.updateLuaUISearchQuery("")
	case "close":
		if a.closeLuaUISearch() {
			return
		}
		a.closeActiveUI()
	default:
		a.runAction(action)
	}
}

func (a *App) visibleLuaUIIndices() []int {
	query := strings.ToLower(strings.TrimSpace(a.luaUI.query))
	visible := make([]int, 0, len(a.luaUI.rows))
	for i, row := range a.luaUI.rows {
		if query == "" || strings.Contains(strings.ToLower(row), query) {
			visible = append(visible, i)
		}
	}
	return visible
}

func (a *App) selectedVisibleLuaUIRow(visible []int) int {
	for i, index := range visible {
		if index == a.luaUI.selected {
			return i
		}
	}
	if len(visible) == 0 {
		return 0
	}
	a.luaUI.selected = visible[clampInt(a.luaUI.scroll, 0, len(visible)-1)]
	return clampInt(a.luaUI.scroll, 0, len(visible)-1)
}

func (a *App) updateLuaUISearchQuery(query string) {
	a.luaUI.query = query
	visible := a.visibleLuaUIIndices()
	if len(visible) == 0 {
		a.luaUI.selected = -1
		a.luaUI.scroll = 0
		return
	}
	a.luaUI.selected = visible[0]
	a.luaUI.scroll = 0
	a.ensureLuaUISelectionVisible()
}

func (a *App) insertLuaUISearchText(text string) {
	if !a.luaUI.visible || !a.luaUI.searching {
		return
	}
	a.updateLuaUISearchQuery(a.luaUI.query + text)
}

func (a *App) backspaceLuaUISearch() {
	if !a.luaUI.visible || !a.luaUI.searching || a.luaUI.query == "" {
		return
	}
	runes := []rune(a.luaUI.query)
	a.updateLuaUISearchQuery(string(runes[:len(runes)-1]))
}

func (a *App) closeLuaUISearch() bool {
	if !a.luaUI.searching && a.luaUI.query == "" {
		return false
	}
	a.luaUI.searching = false
	a.updateLuaUISearchQuery("")
	return true
}

func (a *App) moveLuaUISelection(delta int) {
	visible := a.visibleLuaUIIndices()
	if len(visible) == 0 {
		return
	}
	row := a.selectedVisibleLuaUIRow(visible)
	row = clampInt(row+delta, 0, len(visible)-1)
	a.luaUI.selected = visible[row]
	a.ensureLuaUISelectionVisible()
}

func (a *App) scrollLuaUI(delta int) {
	_, rows := a.luaUIGeometry()
	maxScroll := max(0, len(a.visibleLuaUIIndices())-rows)
	a.luaUI.scroll = clampInt(a.luaUI.scroll+delta, 0, maxScroll)
}

func (a *App) startLuaUIScrollbarDrag(x, y int) bool {
	rect, rows := a.luaUIGeometry()
	return modalListStartScrollbarDrag(rect, a.luaUIRowHeight(), rows, len(a.visibleLuaUIIndices()), x, y, &a.luaUI.scroll, &a.luaUI.scrollbarDragOffsetY, &a.luaUI.draggingScrollbar)
}

func (a *App) dragLuaUIScrollbar(y int) {
	rect, rows := a.luaUIGeometry()
	modalListDragScrollbar(rect, a.luaUIRowHeight(), rows, len(a.visibleLuaUIIndices()), y, &a.luaUI.scroll, a.luaUI.scrollbarDragOffsetY)
}

func (a *App) ensureLuaUISelectionVisible() {
	_, rows := a.luaUIGeometry()
	visible := a.visibleLuaUIIndices()
	if len(visible) == 0 {
		a.luaUI.scroll = 0
		return
	}
	a.luaUI.scroll = modalListScrollForSelection(a.luaUI.scroll, a.selectedVisibleLuaUIRow(visible), rows, len(visible))
}

func (a *App) activateLuaUISelection() {
	if a.luaUI.selected < 0 || a.luaUI.selected >= len(a.luaUI.rows) {
		return
	}
	callback := a.luaUI.onSelect
	builtin := a.luaUI.onSelectBuiltin
	if callback == "" && builtin == nil {
		return
	}
	index := a.luaUI.selected + 1
	value := a.luaUI.rows[a.luaUI.selected]
	if builtin != nil {
		builtin(value)
		return
	}
	if err := a.runtime.RunUISelect(callback, index, value); err != nil {
		a.message = err.Error()
	}
}

func (a *App) closeLuaUI(callCallback bool) {
	if !a.luaUI.visible {
		return
	}
	callback := a.luaUI.onClose
	a.luaUI = luaUIState{}
	a.pendingRedraw = true
	if callCallback && callback != "" {
		if err := a.runtime.RunUIClose(callback); err != nil {
			a.message = err.Error()
		}
	}
}

func (a *App) clickLuaUI(x, y int) {
	if a.startLuaUIScrollbarDrag(x, y) {
		return
	}
	rect, rows := a.luaUIGeometry()
	rowHeight := a.luaUIRowHeight()
	visible := a.visibleLuaUIIndices()
	index, ok := a.modalListIndexAt(rect, rows, rowHeight, x, y, a.luaUI.scroll, len(visible))
	if !ok {
		if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
			a.closeLuaUI(true)
		}
		return
	}
	a.luaUI.selected = visible[index]
	a.activateLuaUISelection()
}

func (a *App) hoverLuaUI(x, y int) {
	rect, rows := a.luaUIGeometry()
	rowHeight := a.luaUIRowHeight()
	visible := a.visibleLuaUIIndices()
	index, ok := a.modalListIndexAt(rect, rows, rowHeight, x, y, a.luaUI.scroll, len(visible))
	if !ok {
		return
	}
	a.luaUI.selected = visible[index]
}

func (a *App) luaUIGeometry() (sdl.FRect, int) {
	return a.modalListGeometry(70, 70)
}

func (a *App) luaUIRowHeight() int {
	return a.modalListRowHeight()
}

func (a *App) drawLuaUI(renderer *sdl.Renderer) error {
	rect, rows := a.luaUIGeometry()
	if err := a.drawModalListFrame(renderer, rect); err != nil {
		return err
	}
	rowHeight := a.luaUIRowHeight()
	baselineOffset := a.modalListBaselineOffset(rowHeight)
	header := a.luaUI.title
	if header == "" {
		header = "Menu"
	}
	visible := a.visibleLuaUIIndices()
	if a.luaUI.searching || a.luaUI.query != "" {
		header = fmt.Sprintf(" %s /%s (%d/%d)", header, a.luaUI.query, len(visible), len(a.luaUI.rows))
	} else {
		header = fmt.Sprintf(" %s (%d)", header, len(a.luaUI.rows))
	}
	if err := a.drawText(renderer, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	if len(visible) == 0 {
		text := "No items"
		if a.luaUI.query != "" {
			text = "No matching items"
		}
		if err := a.drawText(renderer, text, int(rect.X)+16, int(rect.Y)+rowHeight+baselineOffset, a.foregroundColor()); err != nil {
			return err
		}
		return nil
	}
	maxScroll := max(0, len(visible)-rows)
	a.luaUI.scroll = clampInt(a.luaUI.scroll, 0, maxScroll)
	for row := 0; row < rows; row++ {
		visibleIndex := a.luaUI.scroll + row
		if visibleIndex >= len(visible) {
			break
		}
		index := visible[visibleIndex]
		y := int(rect.Y) + rowHeight + row*rowHeight
		if index == a.luaUI.selected {
			if err := a.drawModalListSelection(renderer, rect, y, rowHeight); err != nil {
				return err
			}
		}
		clr := a.foregroundColor()
		if index == a.luaUI.selected {
			clr = a.highlightForegroundColor()
		}
		text := a.truncateModalListText(a.luaUI.rows[index], int(rect.W)-32)
		if err := a.drawText(renderer, text, int(rect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
	}
	return a.drawModalListScrollbar(renderer, rect, rowHeight, rows, len(visible), a.luaUI.scroll)
}
