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
	view            *uiView
	capturing       bool
	selectingAction bool
	captureAction   string
	rows            []keybindRow
}

type keybindRow struct {
	key    string
	action string
}

func (a *App) toggleKeybindMenu() {
	if a.keybindMenu.view != nil && a.keybindMenu.view.visible {
		a.closeUIView(a.keybindMenu.view, false)
		return
	}
	a.closeAllUI()
	a.keybindMenu = keybindMenuState{view: a.newKeybindView()}
	a.keybindMenu.view.selected = -1
	a.refreshKeybindRows()
	a.showUIView(a.keybindMenu.view)
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
	a.keybindMenu.view.rows = keybindUIRows(rows)
	a.keybindMenu.view.selected = clampInt(a.keybindMenu.view.selected, -1, max(-1, len(rows)-1))
	a.ensureKeybindSelectionVisible()
}

func (a *App) refreshKeybindActionRows() {
	actions := config.Actions()
	if a.runtime != nil {
		actions = a.runtime.ActionNames()
	}
	sort.Strings(actions)
	rows := make([]keybindRow, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, keybindRow{action: action})
	}
	a.keybindMenu.rows = rows
	a.keybindMenu.view.rows = keybindUIRows(rows)
	a.keybindMenu.view.selected = clampInt(a.keybindMenu.view.selected, 0, max(0, len(rows)-1))
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
	if a.runtime == nil || a.keybindMenu.view == nil || a.keybindMenu.view.selected < 0 || a.keybindMenu.view.selected >= len(a.keybindMenu.rows) {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.view.selected]
	if row.key == "" {
		return
	}
	if err := a.runtime.UnbindKey(row.key); err != nil {
		a.message = err.Error()
	} else {
		a.config = a.runtime.Config()
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
			a.keybindMenu.view.selected = -1
			a.keybindMenu.view.scroll = 0
			a.refreshKeybindRows()
			return
		}
		a.closeActiveUI()
	default:
		a.runAction(action)
	}
}

func (a *App) confirmKeybindMenuSelection() {
	if !a.keybindMenu.selectingAction && a.keybindMenu.view.selected == -1 {
		a.startNewKeybind()
		return
	}
	if len(a.keybindMenu.rows) == 0 {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.view.selected]
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
	a.keybindMenu.view.selected = 0
	a.keybindMenu.view.scroll = 0
	a.refreshKeybindRows()
}

func (a *App) rebindSelectedKey(key string) {
	if a.runtime == nil || a.keybindMenu.view == nil || a.keybindMenu.view.selected < 0 || a.keybindMenu.view.selected >= len(a.keybindMenu.rows) {
		return
	}
	row := a.keybindMenu.rows[a.keybindMenu.view.selected]
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
	var err error
	if row.key == "" {
		err = a.runtime.SetKeyBinding(key, action)
	} else {
		err = a.runtime.RebindKey(row.key, key, action)
	}
	if err != nil {
		a.message = err.Error()
	} else {
		a.config = a.runtime.Config()
		a.message = fmt.Sprintf("bound %s to %s", key, action)
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
	if a.keybindMenu.view.selected < minSelection {
		a.keybindMenu.view.selected = minSelection
	}
	_, rows := a.keybindMenuListGeometry()
	if a.keybindMenu.selectingAction {
		a.moveUIViewSelection(a.keybindMenu.view, delta)
		return
	}
	a.keybindMenu.view.selected = clampInt(a.keybindMenu.view.selected+delta, minSelection, len(a.keybindMenu.rows)-1)
	if a.keybindMenu.view.selected >= 0 {
		a.keybindMenu.view.scroll = modalListScrollForSelection(a.keybindMenu.view.scroll, a.keybindMenu.view.selected, rows, len(a.keybindMenu.rows))
	}
}

func (a *App) scrollKeybindMenu(delta int) {
	_, rows := a.keybindMenuListGeometry()
	scrollUIView(a.keybindMenu.view, delta, rows)
}

func (a *App) ensureKeybindSelectionVisible() {
	if a.keybindMenu.view.selected < 0 {
		a.keybindMenu.view.scroll = 0
		return
	}
	a.ensureUIViewSelectionVisible(a.keybindMenu.view)
}

func (a *App) startKeybindScrollbarDrag(x, y int) bool {
	return a.uiViewStartScrollbarDrag(a.keybindMenu.view, x, y)
}

func (a *App) dragKeybindScrollbar(y int) {
	a.uiViewDragScrollbar(a.keybindMenu.view, y)
}

func (a *App) clickKeybindMenu(x, y int) {
	view := a.keybindMenu.view
	if a.uiViewStartScrollbarDrag(view, x, y) {
		return
	}
	menuRect, _ := a.keybindMenuGeometry()
	if !a.keybindMenu.selectingAction {
		if pointInRect(x, y, a.keybindNewButtonRect(menuRect)) {
			view.selected = -1
			a.startNewKeybind()
			return
		}
	}
	item, ok := a.uiViewIndexAt(view, x, y)
	if !ok {
		if !pointInRect(x, y, menuRect) {
			a.closeUIView(view, false)
		}
		return
	}
	view.selected = item.index
	a.confirmKeybindMenuSelection()
}

func (a *App) hoverKeybindMenu(x, y int) {
	if !a.keybindMenu.selectingAction {
		menuRect, _ := a.keybindMenuGeometry()
		if pointInRect(x, y, a.keybindNewButtonRect(menuRect)) {
			a.keybindMenu.view.selected = -1
			return
		}
	}
	a.uiViewHover(a.keybindMenu.view, x, y)
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
		action := a.keybindMenu.rows[a.keybindMenu.view.selected].action
		if a.keybindMenu.captureAction != "" {
			action = a.keybindMenu.captureAction
		}
		header = " Press key for " + action
	}
	if err := a.drawText(renderer, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	listRect, listRows := a.keybindMenuListGeometry()
	if !a.keybindMenu.selectingAction {
		button := a.keybindNewButtonRect(rect)
		if a.keybindMenu.view.selected == -1 {
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
		if a.keybindMenu.view.selected == -1 {
			clr = a.highlightForegroundColor()
		}
		if err := a.drawText(renderer, "+ "+newKeybindLabel, int(button.X)+10, int(button.Y)+baselineOffset, clr); err != nil {
			return err
		}
	}
	view := a.keybindListView()
	return a.drawUIListItems(renderer, listRect, listRows, view, view.visibleRows())
}

func (a *App) newKeybindView() *uiView {
	view := &uiView{
		id:            "core:keybinds",
		owner:         "core",
		modal:         true,
		searchable:    false,
		widthPercent:  76,
		heightPercent: 80,
		listGeometry: func(a *App) (sdl.FRect, int) {
			return a.keybindMenuListGeometry()
		},
		draw: func(a *App, renderer *sdl.Renderer) error {
			return a.drawKeybindMenu(renderer)
		},
	}
	view.onKey = func(a *App, e *sdl.KeyboardEvent) bool { return a.handleKeybindMenuKey(e) }
	view.onMouseButton = func(a *App, e *sdl.MouseButtonEvent) bool {
		if e.Type == sdl.EventMouseButtonUp && e.Button == uint8(sdl.ButtonLeft) {
			view.draggingScrollbar = false
			return true
		}
		if e.Type == sdl.EventMouseButtonDown && e.Button == uint8(sdl.ButtonLeft) {
			a.clickKeybindMenu(int(e.X), int(e.Y))
		}
		return true
	}
	view.onMouseMotion = func(a *App, e *sdl.MouseMotionEvent) bool {
		if view.draggingScrollbar {
			old := view.scroll
			a.dragKeybindScrollbar(int(e.Y))
			return old != view.scroll
		}
		oldSelected := view.selected
		a.hoverKeybindMenu(int(e.X), int(e.Y))
		return oldSelected != view.selected
	}
	return view
}

func keybindUIRows(rows []keybindRow) []uiRow {
	items := make([]uiRow, 0, len(rows))
	for index, row := range rows {
		text := row.action
		items = append(items, uiRow{index: index, text: text, value: row.action})
	}
	return items
}

func (a *App) keybindListView() *uiView {
	view := a.keybindMenu.view
	if view == nil {
		return nil
	}
	view.rows = keybindUIRows(a.keybindMenu.rows)
	if !a.keybindMenu.selectingAction {
		for index, row := range a.keybindMenu.rows {
			view.rows[index].text = fmt.Sprintf("%-12s %s", row.key, row.action)
		}
	}
	return view
}
