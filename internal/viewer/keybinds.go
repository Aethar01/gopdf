package viewer

import (
	"fmt"
	"sort"
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

const newKeybindLabel = "New keybind..."

type keybindMenuState struct {
	visible              bool
	selected             int
	scroll               int
	draggingScrollbar    bool
	scrollbarDragOffsetY int
	capturing            bool
	selectingAction      bool
	captureAction        string
	rows                 []keybindRow
}

type keybindRow struct {
	key    string
	action string
}

func (a *App) toggleKeybindMenu() {
	if a.keybindMenu.visible {
		a.keybindMenu = keybindMenuState{}
		return
	}
	a.outlineMenu.visible = false
	a.luaUI.visible = false
	a.keybindMenu.selected = -1
	a.refreshKeybindRows()
	a.keybindMenu.visible = true
}

func (a *App) refreshKeybindRows() {
	if a.keybindMenu.selectingAction {
		a.refreshKeybindActionRows()
		return
	}
	rows := make([]keybindRow, 0, len(a.config.KeyBindings))
	for key, action := range a.config.KeyBindings {
		rows = append(rows, keybindRow{key: key, action: action})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].action == rows[j].action {
			return rows[i].key < rows[j].key
		}
		return rows[i].action < rows[j].action
	})
	a.keybindMenu.rows = rows
	a.keybindMenu.selected = clampInt(a.keybindMenu.selected, -1, max(-1, len(rows)-1))
	a.ensureKeybindSelectionVisible()
}

func (a *App) refreshKeybindActionRows() {
	actions := config.Actions()
	sort.Strings(actions)
	rows := make([]keybindRow, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, keybindRow{action: action})
	}
	a.keybindMenu.rows = rows
	a.keybindMenu.selected = clampInt(a.keybindMenu.selected, 0, max(0, len(rows)-1))
	a.ensureKeybindSelectionVisible()
}

func (a *App) handleKeybindMenuKey(e *sdl.KeyboardEvent) bool {
	if e.Type != sdl.EventKeyDown || e.Repeat {
		return true
	}
	if !a.keybindMenu.capturing && !a.keybindMenu.selectingAction && (e.Key == sdl.KeycodeDelete || e.Key == sdl.KeycodeBackspace) {
		a.deleteSelectedKeybind()
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok {
		if a.keybindMenu.capturing {
			if normalizeBinding(token) == normalizeBinding("<Esc>") {
				a.keybindMenu.capturing = false
				return true
			}
			a.rebindSelectedKey(token)
			return true
		}
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			prevMode := a.mode
			a.runKeybindMenuAction(action)
			if prevMode == modeNormal && a.mode != modeNormal && len([]rune(token)) == 1 {
				a.ignoreText = token
			}
		}
	}
	return true
}

func (a *App) deleteSelectedKeybind() {
	if a.runtime == nil || a.keybindMenu.selected < 0 || a.keybindMenu.selected >= len(a.keybindMenu.rows) {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.selected]
	if row.key == "" {
		return
	}
	delete(a.config.KeyBindings, row.key)
	if err := a.runtime.UnbindKey(row.key); err != nil {
		a.message = err.Error()
	} else {
		a.message = fmt.Sprintf("unbound %s", row.key)
	}
	a.applyConfigState(a.config, true)
	a.refreshKeybindRows()
}

func (a *App) runKeybindMenuAction(action string) {
	switch action {
	case "scroll_down":
		a.moveKeybindSelection(1)
	case "scroll_up":
		a.moveKeybindSelection(-1)
	case "confirm":
		a.confirmKeybindMenuSelection()
	case "close", "keybinds":
		if a.keybindMenu.selectingAction {
			a.keybindMenu.selectingAction = false
			a.keybindMenu.selected = -1
			a.keybindMenu.scroll = 0
			a.refreshKeybindRows()
			return
		}
		a.keybindMenu = keybindMenuState{}
	default:
		a.runAction(action)
	}
}

func (a *App) confirmKeybindMenuSelection() {
	if !a.keybindMenu.selectingAction && a.keybindMenu.selected == -1 {
		a.startNewKeybind()
		return
	}
	if len(a.keybindMenu.rows) == 0 {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.selected]
	if a.keybindMenu.selectingAction {
		a.keybindMenu.captureAction = row.action
		a.keybindMenu.capturing = true
		return
	}
	a.keybindMenu.captureAction = row.action
	a.keybindMenu.capturing = true
}

func (a *App) startNewKeybind() {
	a.keybindMenu.selectingAction = true
	a.keybindMenu.selected = 0
	a.keybindMenu.scroll = 0
	a.refreshKeybindRows()
}

func (a *App) rebindSelectedKey(key string) {
	if a.runtime == nil || a.keybindMenu.selected < 0 || a.keybindMenu.selected >= len(a.keybindMenu.rows) {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.selected]
	action := row.action
	if a.keybindMenu.captureAction != "" {
		action = a.keybindMenu.captureAction
	}
	if strings.HasPrefix(action, "__lua_callback_") {
		a.message = "cannot persist callback keybind"
		a.keybindMenu.capturing = false
		a.keybindMenu.captureAction = ""
		return
	}
	a.config.KeyBindings[key] = action
	if err := a.runtime.SetKeyBinding(key, action); err != nil {
		a.message = err.Error()
	} else {
		a.message = fmt.Sprintf("added %s for %s", key, action)
	}
	a.applyConfigState(a.config, true)
	a.keybindMenu.capturing = false
	a.keybindMenu.captureAction = ""
	a.keybindMenu.selectingAction = false
	a.refreshKeybindRows()
}

func (a *App) moveKeybindSelection(delta int) {
	if len(a.keybindMenu.rows) == 0 && a.keybindMenu.selectingAction {
		return
	}
	minSelection := 0
	if !a.keybindMenu.selectingAction {
		minSelection = -1
	}
	a.keybindMenu.selected = clampInt(a.keybindMenu.selected+delta, minSelection, len(a.keybindMenu.rows)-1)
	a.ensureKeybindSelectionVisible()
}

func (a *App) scrollKeybindMenu(delta int) {
	_, rows := a.keybindMenuListGeometry()
	a.keybindMenu.scroll = clampInt(a.keybindMenu.scroll+delta, 0, max(0, len(a.keybindMenu.rows)-rows))
}

func (a *App) ensureKeybindSelectionVisible() {
	_, rows := a.keybindMenuListGeometry()
	if rows < 1 {
		rows = 1
	}
	if a.keybindMenu.selected < 0 {
		a.keybindMenu.scroll = 0
		return
	}
	if a.keybindMenu.selected < a.keybindMenu.scroll {
		a.keybindMenu.scroll = a.keybindMenu.selected
	}
	if a.keybindMenu.selected >= a.keybindMenu.scroll+rows {
		a.keybindMenu.scroll = a.keybindMenu.selected - rows + 1
	}
	a.keybindMenu.scroll = clampInt(a.keybindMenu.scroll, 0, max(0, len(a.keybindMenu.rows)-rows))
}

func (a *App) startKeybindScrollbarDrag(x, y int) bool {
	rect, rows := a.keybindMenuListGeometry()
	rowHeight := a.keybindMenuRowHeight()
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, len(a.keybindMenu.rows), a.keybindMenu.scroll)
	if !ok || !pointInRect(x, y, track) {
		return false
	}
	if pointInRect(x, y, thumb) {
		a.keybindMenu.scrollbarDragOffsetY = int(float32(y) - thumb.Y)
	} else {
		a.keybindMenu.scrollbarDragOffsetY = int(thumb.H / 2)
		a.keybindMenu.scroll = modalListScrollbarScrollForY(track, thumb, rows, len(a.keybindMenu.rows), y, a.keybindMenu.scrollbarDragOffsetY)
	}
	a.keybindMenu.draggingScrollbar = true
	return true
}

func (a *App) dragKeybindScrollbar(y int) {
	rect, rows := a.keybindMenuListGeometry()
	rowHeight := a.keybindMenuRowHeight()
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, len(a.keybindMenu.rows), a.keybindMenu.scroll)
	if !ok {
		return
	}
	a.keybindMenu.scroll = modalListScrollbarScrollForY(track, thumb, rows, len(a.keybindMenu.rows), y, a.keybindMenu.scrollbarDragOffsetY)
}

func (a *App) clickKeybindMenu(x, y int) {
	if a.startKeybindScrollbarDrag(x, y) {
		return
	}
	menuRect, _ := a.keybindMenuGeometry()
	if !a.keybindMenu.selectingAction {
		if pointInRect(x, y, a.keybindNewButtonRect(menuRect)) {
			a.keybindMenu.selected = -1
			a.startNewKeybind()
			return
		}
	}
	rect, rows := a.keybindMenuListGeometry()
	rowHeight := a.keybindMenuRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		if !pointInRect(x, y, menuRect) {
			a.keybindMenu = keybindMenuState{}
		}
		return
	}
	index := a.keybindMenu.scroll + row
	if index < 0 || index >= len(a.keybindMenu.rows) {
		return
	}
	a.keybindMenu.selected = index
	a.confirmKeybindMenuSelection()
}

func (a *App) hoverKeybindMenu(x, y int) {
	if !a.keybindMenu.selectingAction {
		menuRect, _ := a.keybindMenuGeometry()
		if pointInRect(x, y, a.keybindNewButtonRect(menuRect)) {
			a.keybindMenu.selected = -1
			return
		}
	}
	rect, rows := a.keybindMenuListGeometry()
	rowHeight := a.keybindMenuRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		return
	}
	index := a.keybindMenu.scroll + row
	if index < 0 || index >= len(a.keybindMenu.rows) {
		return
	}
	a.keybindMenu.selected = index
}

func (a *App) keybindMenuGeometry() (sdl.FRect, int) {
	return a.modalListGeometry(76, 80)
}

func (a *App) keybindMenuListGeometry() (sdl.FRect, int) {
	rect, rows := a.keybindMenuGeometry()
	if a.keybindMenu.selectingAction {
		return rect, rows
	}
	rowHeight := a.keybindMenuRowHeight()
	rect.Y += float32(rowHeight * 2)
	rect.H -= float32(rowHeight * 2)
	return rect, max(1, rows-2)
}

func (a *App) keybindNewButtonRect(rect sdl.FRect) sdl.FRect {
	rowHeight := a.keybindMenuRowHeight()
	return sdl.FRect{X: rect.X + 6, Y: rect.Y + float32(rowHeight), W: rect.W - 12, H: float32(rowHeight)}
}

func (a *App) keybindMenuRowHeight() int {
	return a.modalListRowHeight()
}

func (a *App) drawKeybindMenu(renderer *sdl.Renderer) error {
	rect, _ := a.keybindMenuGeometry()
	if err := a.drawModalListFrame(renderer, rect); err != nil {
		return err
	}
	rowHeight := a.keybindMenuRowHeight()
	baselineOffset := a.modalListBaselineOffset(rowHeight)
	header := fmt.Sprintf(" Keybinds (%d)", len(a.keybindMenu.rows))
	if a.keybindMenu.selectingAction {
		header = fmt.Sprintf(" Select action (%d)", len(a.keybindMenu.rows))
	}
	if a.keybindMenu.capturing && len(a.keybindMenu.rows) > 0 {
		action := a.keybindMenu.rows[a.keybindMenu.selected].action
		if a.keybindMenu.captureAction != "" {
			action = a.keybindMenu.captureAction
		}
		header = " Press key for " + action
	}
	if err := drawText(renderer, a.fontFace, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	listRect, listRows := a.keybindMenuListGeometry()
	if !a.keybindMenu.selectingAction {
		button := a.keybindNewButtonRect(rect)
		if a.keybindMenu.selected == -1 {
			if err := a.drawModalListSelection(renderer, rect, int(button.Y), rowHeight); err != nil {
				return err
			}
		} else {
			buttonColor := a.statusBarColor()
			buttonColor.A = 0xa0
			if err := fillRect(renderer, button, buttonColor); err != nil {
				return err
			}
		}
		clr := a.foregroundColor()
		if a.keybindMenu.selected == -1 {
			clr = a.highlightForegroundColor()
		}
		if err := drawText(renderer, a.fontFace, "+ "+newKeybindLabel, int(button.X)+10, int(button.Y)+baselineOffset, clr); err != nil {
			return err
		}
	}
	for row := 0; row < listRows; row++ {
		index := a.keybindMenu.scroll + row
		if index >= len(a.keybindMenu.rows) {
			break
		}
		y := int(listRect.Y) + rowHeight + row*rowHeight
		if index == a.keybindMenu.selected {
			if err := a.drawModalListSelection(renderer, listRect, y, rowHeight); err != nil {
				return err
			}
		}
		clr := a.foregroundColor()
		if index == a.keybindMenu.selected {
			clr = a.highlightForegroundColor()
		}
		row := a.keybindMenu.rows[index]
		text := row.action
		if !a.keybindMenu.selectingAction {
			text = fmt.Sprintf("%-12s %s", row.key, row.action)
		}
		text = a.truncateModalListText(text, int(listRect.W)-32)
		if err := drawText(renderer, a.fontFace, text, int(listRect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
	}
	return a.drawModalListScrollbar(renderer, listRect, rowHeight, listRows, len(a.keybindMenu.rows), a.keybindMenu.scroll)
}
