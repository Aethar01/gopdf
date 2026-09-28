package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
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
