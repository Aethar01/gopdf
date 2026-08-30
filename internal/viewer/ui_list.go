package viewer

import (
	"fmt"
	"strconv"
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type uiRow struct {
	index     int
	text      string
	value     string
	secondary string
	depth     int
	marker    string
	disabled  bool
}

type uiView struct {
	id                   string
	owner                string
	title                string
	rows                 []uiRow
	selected             int
	scroll               int
	searchable           bool
	searching            bool
	query                string
	visible              bool
	draggingScrollbar    bool
	scrollbarDragOffsetY int
	modal                bool
	widthPercent         int
	heightPercent        int
	geometry             func(*App) (sdl.FRect, int)
	listGeometry         func(*App) (sdl.FRect, int)
	header               func(*App, *uiView, int) string
	empty                func(*App, *uiView) string
	onSelect             func(*App, uiRow)
	onClose              func(*App)
	onQueryChanged       func(*App, *uiView)
	onKey                func(*App, *sdl.KeyboardEvent) bool
	onMouseButton        func(*App, *sdl.MouseButtonEvent) bool
	onMouseMotion        func(*App, *sdl.MouseMotionEvent) bool
	draw                 func(*App, *sdl.Renderer) error
}

type uiManager struct {
	views  map[string]*uiView
	active *uiView
}

func (m *uiManager) add(view *uiView) {
	if m.views == nil {
		m.views = make(map[string]*uiView)
	}
	if old := m.views[view.id]; old != nil && old != view {
		old.visible = false
	}
	m.views[view.id] = view
}

func (m *uiManager) show(view *uiView) {
	m.add(view)
	if m.active != nil && m.active != view {
		m.active.visible = false
	}
	view.visible = true
	m.active = view
}

func (m *uiManager) close(view *uiView) {
	if view == nil {
		return
	}
	view.visible = false
	if m.active == view {
		m.active = nil
	}
}

func (m *uiManager) closeAll() {
	for _, view := range m.views {
		view.visible = false
	}
	m.active = nil
}

func (m *uiManager) removeOwner(owner string) {
	for id, view := range m.views {
		if view.owner != owner {
			continue
		}
		delete(m.views, id)
		if m.active == view {
			m.active = nil
		}
	}
}

func (v *uiView) visibleRows() []uiRow {
	query := strings.ToLower(strings.TrimSpace(v.query))
	if query == "" {
		return append([]uiRow(nil), v.rows...)
	}
	rows := make([]uiRow, 0, len(v.rows))
	for _, row := range v.rows {
		if strings.Contains(strings.ToLower(row.text), query) || strings.Contains(strings.ToLower(row.value), query) {
			rows = append(rows, row)
		}
	}
	return rows
}

func (v *uiView) frameGeometry(a *App) (sdl.FRect, int) {
	if v.geometry != nil {
		return v.geometry(a)
	}
	return a.modalListGeometry(v.widthPercent, v.heightPercent)
}

func (v *uiView) contentGeometry(a *App) (sdl.FRect, int) {
	if v.listGeometry != nil {
		return v.listGeometry(a)
	}
	return v.frameGeometry(a)
}

func (a *App) activeUIView() *uiView {
	if a.views.active == nil || !a.views.active.visible {
		return nil
	}
	return a.views.active
}

func (a *App) activeModalUIView() *uiView {
	view := a.activeUIView()
	if view == nil || !view.modal {
		return nil
	}
	return view
}

func (a *App) showUIView(view *uiView) {
	a.views.show(view)
	a.pendingRedraw = true
}

func (a *App) closeUIView(view *uiView, callCallback bool) {
	if view == nil || !view.visible {
		return
	}
	a.views.close(view)
	a.pendingRedraw = true
	a.syncTextInput()
	if callCallback && view.onClose != nil {
		view.onClose(a)
	}
}

func (a *App) closeAllUIViews() {
	a.views.closeAll()
	a.pendingRedraw = true
}

func (a *App) setUIViewRows(view *uiView, rows []uiRow) {
	if view == nil {
		return
	}
	view.rows = rows
	if len(rows) == 0 {
		view.selected = -1
		view.scroll = 0
	} else {
		view.selected = clampInt(view.selected, 0, len(rows)-1)
		a.ensureUIViewSelectionVisible(view)
	}
	a.pendingRedraw = true
}

func (a *App) setUIViewQuery(view *uiView, query string) {
	if view == nil {
		return
	}
	view.query = query
	if view.onQueryChanged != nil {
		view.onQueryChanged(a, view)
	}
	a.ensureUIViewSelectionVisible(view)
	a.pendingRedraw = true
}

func (a *App) moveUIViewSelection(view *uiView, delta int) {
	if view == nil {
		return
	}
	items := view.visibleRows()
	if len(items) == 0 {
		return
	}
	row := uiViewSelectedRow(view, items)
	for next := clampInt(row+delta, 0, len(items)-1); next != row; next = clampInt(next+delta, 0, len(items)-1) {
		row = next
		if !items[row].disabled {
			break
		}
	}
	if items[row].disabled {
		return
	}
	view.selected = items[row].index
	_, rows := view.contentGeometry(a)
	view.scroll = modalListScrollForSelection(view.scroll, row, rows, len(items))
	a.pendingRedraw = true
}

func (a *App) ensureUIViewSelectionVisible(view *uiView) {
	if view == nil {
		return
	}
	items := view.visibleRows()
	if len(items) == 0 {
		view.selected = -1
		view.scroll = 0
		return
	}
	row := uiViewSelectedRow(view, items)
	_, rows := view.contentGeometry(a)
	view.scroll = modalListScrollForSelection(view.scroll, row, rows, len(items))
}

func uiViewSelectedRow(view *uiView, items []uiRow) int {
	for i, item := range items {
		if item.index == view.selected && !item.disabled {
			return i
		}
	}
	start := clampInt(view.scroll, 0, len(items)-1)
	for offset := range len(items) {
		row := (start + offset) % len(items)
		if !items[row].disabled {
			view.selected = items[row].index
			return row
		}
	}
	view.selected = -1
	return start
}

func scrollUIView(view *uiView, delta, rows int) {
	if view == nil {
		return
	}
	view.scroll = clampInt(view.scroll+delta, 0, max(0, len(view.visibleRows())-rows))
}

func (a *App) drawUIView(renderer *sdl.Renderer, view *uiView) error {
	if view == nil || !view.visible {
		return nil
	}
	if view.draw != nil {
		return view.draw(a, renderer)
	}
	rect, _ := view.frameGeometry(a)
	listRect, rows := view.contentGeometry(a)
	if err := a.drawModalListFrame(renderer, rect); err != nil {
		return err
	}
	items := view.visibleRows()
	header := view.title
	if view.header != nil {
		header = view.header(a, view, len(items))
	}
	if header == "" {
		header = "Menu"
	}
	if view.header == nil && (view.searching || view.query != "") {
		header = fmt.Sprintf("%s /%s (%d/%d)", header, view.query, len(items), len(view.rows))
	}
	rowHeight := a.modalListRowHeight()
	baselineOffset := a.modalListBaselineOffset(rowHeight)
	if err := a.drawText(renderer, a.truncateModalListText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	if len(items) == 0 {
		empty := "No items"
		if view.empty != nil {
			empty = view.empty(a, view)
		}
		return a.drawText(renderer, empty, int(listRect.X)+16, int(listRect.Y)+rowHeight+baselineOffset, a.foregroundColor())
	}
	return a.drawUIListItems(renderer, listRect, rows, view, items)
}

func (a *App) drawUIListItems(renderer *sdl.Renderer, rect sdl.FRect, rows int, view *uiView, items []uiRow) error {
	if view == nil {
		return nil
	}
	rowHeight := a.modalListRowHeight()
	baselineOffset := a.modalListBaselineOffset(rowHeight)
	rows = max(1, rows)
	view.scroll = clampInt(view.scroll, 0, max(0, len(items)-rows))
	for row := 0; row < rows; row++ {
		itemIndex := view.scroll + row
		if itemIndex >= len(items) {
			break
		}
		item := items[itemIndex]
		y := int(rect.Y) + rowHeight + row*rowHeight
		if item.index == view.selected {
			if err := a.drawModalListSelection(renderer, rect, y, rowHeight); err != nil {
				return err
			}
		}
		clr := a.foregroundColor()
		if item.index == view.selected {
			clr = a.highlightForegroundColor()
		}
		text := strings.Repeat("  ", max(0, item.depth)) + item.marker + item.text
		textWidth := int(rect.W) - 32
		if item.secondary != "" {
			secondaryWidth := measureText(a.fontFace, item.secondary)
			textWidth = int(rect.W) - 36 - secondaryWidth
		}
		if item.disabled {
			clr.A /= 2
		}
		if err := a.drawText(renderer, a.truncateModalListText(text, textWidth), int(rect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
		if item.secondary != "" {
			secondaryWidth := measureText(a.fontFace, item.secondary)
			if err := a.drawText(renderer, item.secondary, int(rect.X+rect.W)-16-secondaryWidth, y+baselineOffset, clr); err != nil {
				return err
			}
		}
	}
	return a.drawModalListScrollbar(renderer, rect, rowHeight, rows, len(items), view.scroll)
}

func (a *App) uiViewIndexAt(view *uiView, x, y int) (uiRow, bool) {
	if view == nil {
		return uiRow{}, false
	}
	rect, rows := view.contentGeometry(a)
	rowHeight := a.modalListRowHeight()
	row, ok := a.modalListRowAt(rect, rows, rowHeight, x, y)
	if !ok {
		return uiRow{}, false
	}
	items := view.visibleRows()
	itemIndex := view.scroll + row
	if itemIndex < 0 || itemIndex >= len(items) {
		return uiRow{}, false
	}
	return items[itemIndex], true
}

func (a *App) uiViewStartScrollbarDrag(view *uiView, x, y int) bool {
	if view == nil {
		return false
	}
	rect, rows := view.contentGeometry(a)
	rowHeight := a.modalListRowHeight()
	return modalListStartScrollbarDrag(rect, rowHeight, rows, len(view.visibleRows()), x, y, &view.scroll, &view.scrollbarDragOffsetY, &view.draggingScrollbar)
}

func (a *App) uiViewDragScrollbar(view *uiView, y int) {
	if view == nil {
		return
	}
	rect, rows := view.contentGeometry(a)
	rowHeight := a.modalListRowHeight()
	modalListDragScrollbar(rect, rowHeight, rows, len(view.visibleRows()), y, &view.scroll, view.scrollbarDragOffsetY)
}

func (a *App) uiViewHover(view *uiView, x, y int) bool {
	if view == nil {
		return false
	}
	old := view.selected
	if item, ok := a.uiViewIndexAt(view, x, y); ok {
		if !item.disabled {
			view.selected = item.index
		}
	}
	return old != view.selected
}

func uiRowsFromConfig(rows []config.UIListRow) []uiRow {
	result := make([]uiRow, len(rows))
	for i, row := range rows {
		result[i] = uiRow{index: i, text: row.Text, value: row.Value, secondary: row.Secondary, depth: row.Depth, disabled: row.Disabled}
	}
	return result
}

func uiRowsFromStrings(rows []string) []uiRow {
	result := make([]uiRow, len(rows))
	for i, text := range rows {
		result[i] = uiRow{index: i, text: text, value: text}
	}
	return result
}

func (a *App) createCoreListView(id, title string, rows []uiRow, widthPercent, heightPercent int) *uiView {
	view := &uiView{id: id, owner: "core", title: title, rows: rows, selected: 0, searchable: true, modal: true, widthPercent: widthPercent, heightPercent: heightPercent}
	a.views.add(view)
	return view
}

func (a *App) showCoreList(id, title string, rows []string, onSelect func(string)) {
	view := a.createCoreListView(id, title, uiRowsFromStrings(rows), 70, 70)
	view.onKey = func(a *App, e *sdl.KeyboardEvent) bool { return a.handleGenericUIViewKey(view, e) }
	view.onMouseButton = func(a *App, e *sdl.MouseButtonEvent) bool { return a.handleGenericUIViewMouseButton(view, e) }
	view.onMouseMotion = func(a *App, e *sdl.MouseMotionEvent) bool { return a.handleGenericUIViewMouseMotion(view, e) }
	if onSelect != nil {
		view.onSelect = func(_ *App, row uiRow) { onSelect(row.value) }
	}
	a.showUIView(view)
}

func (a *App) createLuaListView(spec config.UIOverlay) *uiView {
	view := &uiView{
		id:            spec.ID,
		owner:         "lua",
		title:         spec.Title,
		rows:          uiRowsFromConfig(spec.Rows),
		selected:      clampInt(spec.Selected-1, 0, max(0, len(spec.Rows)-1)),
		scroll:        max(0, spec.Scroll),
		query:         spec.Query,
		searchable:    spec.Searchable,
		modal:         true,
		widthPercent:  70,
		heightPercent: 70,
	}
	if view.id == "" {
		view.id = "lua" + strconv.Itoa(len(a.views.views)+1)
	}
	if spec.OnSelect != "" {
		callback := spec.OnSelect
		view.onSelect = func(a *App, row uiRow) {
			if err := a.runtime.RunUISelect(callback, row.index+1, row.value, row.text); err != nil {
				a.message = err.Error()
			}
			a.applyRuntimeChanges("ui_select")
		}
	}
	if spec.OnClose != "" {
		callback := spec.OnClose
		view.onClose = func(a *App) {
			if err := a.runtime.RunUIClose(callback); err != nil {
				a.message = err.Error()
			}
			a.applyRuntimeChanges("ui_close")
		}
	}
	return view
}
