package viewer

import (
	"fmt"
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type luaUIState struct {
	visible  bool
	title    string
	rows     []string
	selected int
	scroll   int
	onSelect string
	onClose  string
}

func (a *App) ShowUI(overlay config.UIOverlay) error {
	a.outlineMenu.visible = false
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
	if len(rows) == 0 {
		a.luaUI.selected = 0
		a.luaUI.scroll = 0
	} else {
		a.luaUI.selected = clampInt(a.luaUI.selected, 0, len(rows)-1)
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
			a.runLuaUIAction(action)
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
	case "close":
		a.closeLuaUI(true)
	default:
		a.runAction(action)
	}
}

func (a *App) moveLuaUISelection(delta int) {
	if len(a.luaUI.rows) == 0 {
		return
	}
	a.luaUI.selected = clampInt(a.luaUI.selected+delta, 0, len(a.luaUI.rows)-1)
	a.ensureLuaUISelectionVisible()
}

func (a *App) scrollLuaUI(delta int) {
	_, rows := a.luaUIGeometry()
	maxScroll := max(0, len(a.luaUI.rows)-rows)
	a.luaUI.scroll = clampInt(a.luaUI.scroll+delta, 0, maxScroll)
}

func (a *App) ensureLuaUISelectionVisible() {
	_, rows := a.luaUIGeometry()
	if rows < 1 {
		rows = 1
	}
	if a.luaUI.selected < a.luaUI.scroll {
		a.luaUI.scroll = a.luaUI.selected
	}
	if a.luaUI.selected >= a.luaUI.scroll+rows {
		a.luaUI.scroll = a.luaUI.selected - rows + 1
	}
	maxScroll := max(0, len(a.luaUI.rows)-rows)
	a.luaUI.scroll = clampInt(a.luaUI.scroll, 0, maxScroll)
}

func (a *App) activateLuaUISelection() {
	if a.luaUI.selected < 0 || a.luaUI.selected >= len(a.luaUI.rows) {
		return
	}
	callback := a.luaUI.onSelect
	if callback == "" {
		return
	}
	index := a.luaUI.selected + 1
	value := a.luaUI.rows[a.luaUI.selected]
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
	rect, rows := a.luaUIGeometry()
	rowHeight := a.luaUIRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
			a.closeLuaUI(true)
		}
		return
	}
	index := a.luaUI.scroll + row
	if index < 0 || index >= len(a.luaUI.rows) {
		return
	}
	a.luaUI.selected = index
	a.activateLuaUISelection()
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
	header = fmt.Sprintf(" %s (%d)", header, len(a.luaUI.rows))
	if err := drawText(renderer, a.fontFace, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	if len(a.luaUI.rows) == 0 {
		if err := drawText(renderer, a.fontFace, "No items", int(rect.X)+16, int(rect.Y)+rowHeight+baselineOffset, a.foregroundColor()); err != nil {
			return err
		}
		return nil
	}
	maxScroll := max(0, len(a.luaUI.rows)-rows)
	a.luaUI.scroll = clampInt(a.luaUI.scroll, 0, maxScroll)
	for row := 0; row < rows; row++ {
		index := a.luaUI.scroll + row
		if index >= len(a.luaUI.rows) {
			break
		}
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
		if err := drawText(renderer, a.fontFace, text, int(rect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
	}
	return nil
}
