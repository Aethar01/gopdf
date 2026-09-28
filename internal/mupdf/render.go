package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
*/
import "C"

import (
	"fmt"
	"image"
	"math"
	"sync"
	"unsafe"
)

// Renderer rasterises pages on a private MuPDF context. The document lock is
// held only while fetching a page's display list, so renderers run
// concurrently with each other and with other document calls.
//
// Render and Close must be called from one goroutine at a time; Cancel may be
// called from any goroutine.
type Renderer struct {
	doc    *Document
	mu     sync.Mutex // guards handle against Cancel racing Close
	handle *C.gopdf_renderer
}

func (d *Document) NewRenderer() (*Renderer, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return nil, err
	}
	var cerr *C.char
	handle := C.gopdf_new_renderer(d.handle, &cerr)
	if handle == nil {
		return nil, consumeError("new renderer", cerr)
	}
	return &Renderer{doc: d, handle: handle}, nil
}

func (r *Renderer) Close() {
	r.mu.Lock()
	defer r.mu.Unlock()
	C.gopdf_drop_renderer(r.handle)
	r.handle = nil
}

// Cancel aborts the render in progress, if any.
func (r *Renderer) Cancel() {
	r.mu.Lock()
	defer r.mu.Unlock()
	C.gopdf_cancel_renderer(r.handle)
}

func (r *Renderer) Render(page int, scale float64, rotation float64, aaLevel int) (*RenderedPage, error) {
	if r.handle == nil {
		return nil, fmt.Errorf("render page: renderer is closed")
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return nil, fmt.Errorf("render page: invalid scale %g", scale)
	}
	if math.IsNaN(rotation) || math.IsInf(rotation, 0) {
		return nil, fmt.Errorf("render page: invalid rotation %g", rotation)
	}
	if aaLevel < 0 {
		return nil, fmt.Errorf("render page: invalid antialias level %d", aaLevel)
	}
	list, err := r.doc.displayList(page)
	if err != nil {
		return nil, err
	}
	var samples *C.uchar
	var width, height, stride, x, y C.int
	var cerr *C.char
	if ok := C.gopdf_render_display_list(r.handle, list, C.float(scale), C.float(rotation), C.int(aaLevel), &samples, &width, &height, &stride, &x, &y, &cerr); ok == 0 {
		return nil, consumeError("render page", cerr)
	}
	if samples == nil {
		return &RenderedPage{Image: image.NewRGBA(image.Rect(0, 0, 0, 0)), X: int(x), Y: int(y)}, nil
	}
	img := &image.RGBA{Pix: unsafe.Slice((*byte)(unsafe.Pointer(samples)), int(stride)*int(height)), Stride: int(stride), Rect: image.Rect(0, 0, int(width), int(height))}
	return &RenderedPage{Image: img, X: int(x), Y: int(y), samples: unsafe.Pointer(samples)}, nil
}

// displayList returns a reference that gopdf_render_display_list consumes.
func (d *Document) displayList(page int) (*C.fz_display_list, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var list *C.fz_display_list
	var cerr *C.char
	if ok := C.gopdf_page_display_list(d.handle, C.int(page), &list, &cerr); ok == 0 {
		return nil, consumeError("render page", cerr)
	}
	return list, nil
}
