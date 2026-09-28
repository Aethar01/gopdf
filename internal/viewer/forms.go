package viewer

import (
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Clicking a form field edits it: text fields open a prompt with the value,
// check boxes and radio buttons toggle, and choice fields open a list.

type formTarget struct{ page, index int }

// clickFormField handles a press at a screen point, reporting whether it
// landed on an editable kind of form field.
func (a *App) clickFormField(sx, sy float64) bool {
	if a.doc == nil {
		return false
	}
	page, point, ok := a.pagePointAtScreen(sx, sy)
	if !ok {
		return false
	}
	w, found, err := a.doc.WidgetAt(page, point)
	if err != nil {
		a.logf("find form field page=%d err=%v", page+1, err)
		return false
	}
	if !found || w.Kind == mupdf.WidgetOther {
		return false
	}
	if w.ReadOnly {
		a.message = "read-only field"
		return true
	}
	switch w.Kind {
	case mupdf.WidgetText:
		a.closeAllUI()
		a.formField = &formTarget{page: page, index: w.Index}
		a.mode = modeFormField
		a.input.Set(w.Value)
	case mupdf.WidgetCheckbox, mupdf.WidgetRadio:
		a.editFormField(page, func() error { return a.doc.ToggleWidget(page, w.Index) })
	case mupdf.WidgetChoice:
		a.pickFormChoice(page, w)
	}
	return true
}

func (a *App) editFormField(page int, edit func() error) {
	if err := edit(); err != nil {
		a.message = err.Error()
		return
	}
	a.pageEdited(page)
}

func (a *App) submitFormField(value string) {
	target := a.formField
	a.formField = nil
	if target != nil {
		a.editFormField(target.page, func() error { return a.doc.SetWidgetValue(target.page, target.index, value) })
	}
}

func (a *App) pickFormChoice(page int, w mupdf.Widget) {
	options, err := a.doc.WidgetOptions(page, w.Index)
	if err != nil || len(options) == 0 {
		a.message = "no choices for this field"
		return
	}
	a.closeAllUI()
	rows := make([]uiRow, len(options))
	selected := 0
	for i, option := range options {
		rows[i] = uiRow{index: i, text: option, value: option}
		if option == w.Value {
			selected = i
		}
	}
	view := a.createCoreListView("form-choice", "Choose", rows, 40, 50)
	view.selected = selected
	view.onKey = func(a *App, e *sdl.KeyboardEvent) bool { return a.handleGenericUIViewKey(view, e) }
	view.onMouseButton = func(a *App, e *sdl.MouseButtonEvent) bool { return a.handleGenericUIViewMouseButton(view, e) }
	view.onMouseMotion = func(a *App, e *sdl.MouseMotionEvent) bool { return a.handleGenericUIViewMouseMotion(view, e) }
	view.onSelect = func(a *App, row uiRow) {
		a.closeUIView(view, false)
		a.editFormField(page, func() error { return a.doc.SetWidgetValue(page, w.Index, row.value) })
	}
	a.showUIView(view)
}
