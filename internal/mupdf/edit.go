package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
#include <mupdf/pdf.h>
*/
import "C"

import (
	"os"
	"path/filepath"
	"unsafe"
)

// IsPDF reports whether the document is a PDF, which can be edited and saved.
func (d *Document) IsPDF() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.ensureOpenLocked() == nil && C.gopdf_is_pdf(d.handle) != 0
}

// Save writes the document to path. Saving over the open file appends the
// changes incrementally when the PDF allows it, and otherwise writes a copy
// beside it and renames it into place.
func (d *Document) Save(path, openPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return err
	}
	if !sameFile(path, openPath) {
		return d.saveLocked(path, false)
	}
	if C.gopdf_can_save_incrementally(d.handle) != 0 {
		return d.saveLocked(path, true)
	}
	tmp := path + ".gopdf-tmp"
	if err := d.saveLocked(tmp, false); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, path)
}

// AddHighlight adds a highlight annotation over quads on page in color.
func (d *Document) AddHighlight(page int, quads []Quad, color [3]uint8) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return err
	}
	if len(quads) == 0 {
		return nil
	}
	cquads := make([]C.gopdf_quad, len(quads))
	for i, q := range quads {
		cquads[i] = C.gopdf_quad{ul: cPoint(q.UL), ur: cPoint(q.UR), ll: cPoint(q.LL), lr: cPoint(q.LR)}
	}
	rgb := [3]C.float{C.float(color[0]) / 255, C.float(color[1]) / 255, C.float(color[2]) / 255}
	var cerr *C.char
	if C.gopdf_add_highlight(d.handle, C.int(page), &cquads[0], C.int(len(cquads)), &rgb[0], &cerr) == 0 {
		return consumeError("add highlight", cerr)
	}
	return nil
}

func cPoint(p Point) C.gopdf_point { return C.gopdf_point{x: C.float(p.X), y: C.float(p.Y)} }

func (d *Document) saveLocked(path string, incremental bool) error {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	inc := C.int(0)
	if incremental {
		inc = 1
	}
	var cerr *C.char
	if C.gopdf_save(d.handle, cpath, inc, &cerr) == 0 {
		return consumeError("save", cerr)
	}
	return nil
}

func sameFile(a, b string) bool {
	if a == b {
		return true
	}
	ai, errA := os.Stat(a)
	bi, errB := os.Stat(b)
	if errA == nil && errB == nil {
		return os.SameFile(ai, bi)
	}
	absA, _ := filepath.Abs(a)
	absB, _ := filepath.Abs(b)
	return absA == absB
}

// WidgetKind is the kind of a PDF form field.
type WidgetKind int

const (
	WidgetOther WidgetKind = iota
	WidgetText
	WidgetCheckbox
	WidgetRadio
	WidgetChoice
)

// Widget is a form field on a page; Index identifies it for editing.
type Widget struct {
	Index    int
	Kind     WidgetKind
	ReadOnly bool
	Bounds   Rect
	Value    string
}

// WidgetAt returns the form field under point p on page, if any.
func (d *Document) WidgetAt(page int, p Point) (Widget, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return Widget{}, false, err
	}
	if C.gopdf_is_pdf(d.handle) == 0 {
		return Widget{}, false, nil
	}
	var info C.gopdf_widget_info
	var cerr *C.char
	if C.gopdf_widget_at(d.handle, C.int(page), C.float(p.X), C.float(p.Y), &info, &cerr) == 0 {
		return Widget{}, false, consumeError("find form field", cerr)
	}
	if info.index < 0 {
		return Widget{}, false, nil
	}
	defer C.free(unsafe.Pointer(info.value))
	w := Widget{
		Index:    int(info.index),
		ReadOnly: info.readonly != 0,
		Bounds:   Rect{X0: float32(info.rect.x0), Y0: float32(info.rect.y0), X1: float32(info.rect.x1), Y1: float32(info.rect.y1)},
		Value:    goString(info.value),
	}
	switch info._type {
	case C.PDF_WIDGET_TYPE_TEXT:
		w.Kind = WidgetText
	case C.PDF_WIDGET_TYPE_CHECKBOX:
		w.Kind = WidgetCheckbox
	case C.PDF_WIDGET_TYPE_RADIOBUTTON:
		w.Kind = WidgetRadio
	case C.PDF_WIDGET_TYPE_COMBOBOX, C.PDF_WIDGET_TYPE_LISTBOX:
		w.Kind = WidgetChoice
	}
	return w, true, nil
}

// WidgetOptions returns the choices of a choice field.
func (d *Document) WidgetOptions(page, index int) ([]string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var raw **C.char
	var count C.int
	var cerr *C.char
	if C.gopdf_widget_options(d.handle, C.int(page), C.int(index), &raw, &count, &cerr) == 0 {
		return nil, consumeError("form field options", cerr)
	}
	defer C.gopdf_free_strings(raw, count)
	options := make([]string, int(count))
	for i, s := range unsafe.Slice(raw, int(count)) {
		options[i] = C.GoString(s)
	}
	return options, nil
}

// SetWidgetValue sets a text or choice field's value.
func (d *Document) SetWidgetValue(page, index int, value string) error {
	cvalue := C.CString(value)
	defer C.free(unsafe.Pointer(cvalue))
	return d.editWidget(page, index, cvalue)
}

// ToggleWidget toggles a check box or radio button.
func (d *Document) ToggleWidget(page, index int) error { return d.editWidget(page, index, nil) }

func (d *Document) editWidget(page, index int, value *C.char) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return err
	}
	var cerr *C.char
	if C.gopdf_widget_edit(d.handle, C.int(page), C.int(index), value, &cerr) == 0 {
		return consumeError("edit form field", cerr)
	}
	return nil
}

// Undo reverts the last edit.
func (d *Document) Undo() error { return d.undo(false) }

// Redo reapplies the last undone edit.
func (d *Document) Redo() error { return d.undo(true) }

func (d *Document) undo(redo bool) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return err
	}
	r := C.int(0)
	if redo {
		r = 1
	}
	var cerr *C.char
	if C.gopdf_undo(d.handle, r, &cerr) == 0 {
		return consumeError("", cerr)
	}
	return nil
}

// Annotation is a markup annotation on a page, such as a highlight; Index
// identifies it for editing.
type Annotation struct {
	Index int
	Type  string // as named by PDF, e.g. "Highlight"
}

// AnnotationAt returns the topmost annotation under point p on page, if any.
// Links, popups and form fields are not included.
func (d *Document) AnnotationAt(page int, p Point) (Annotation, bool, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return Annotation{}, false, err
	}
	if C.gopdf_is_pdf(d.handle) == 0 {
		return Annotation{}, false, nil
	}
	var index C.int
	var ctype, cerr *C.char
	if C.gopdf_annot_at(d.handle, C.int(page), C.float(p.X), C.float(p.Y), &index, &ctype, &cerr) == 0 {
		return Annotation{}, false, consumeError("find annotation", cerr)
	}
	defer C.free(unsafe.Pointer(ctype))
	if index < 0 {
		return Annotation{}, false, nil
	}
	return Annotation{Index: int(index), Type: goString(ctype)}, true, nil
}

// DeleteAnnotation removes an annotation.
func (d *Document) DeleteAnnotation(page, index int) error { return d.editAnnotation(page, index, nil) }

// RecolorAnnotation changes an annotation's colour.
func (d *Document) RecolorAnnotation(page, index int, color [3]uint8) error {
	rgb := [3]C.float{C.float(color[0]) / 255, C.float(color[1]) / 255, C.float(color[2]) / 255}
	return d.editAnnotation(page, index, &rgb[0])
}

func (d *Document) editAnnotation(page, index int, rgb *C.float) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return err
	}
	var cerr *C.char
	if C.gopdf_edit_annot(d.handle, C.int(page), C.int(index), rgb, &cerr) == 0 {
		return consumeError("edit annotation", cerr)
	}
	return nil
}
