package mupdf

/*
#include <stdlib.h>
#include "mupdf_bridge.h"
*/
import "C"

import (
	"errors"
	"fmt"
	"image"
	"math"
	"sync"
	"unsafe"
)

// ErrCancelled reports a render stopped by Cancel; its partial output is
// discarded.
var ErrCancelled = errors.New("render cancelled")

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

// Render rasterises the part of page inside clip, given in device pixels at
// scale. The result's X and Y give its origin in the same space.
func (r *Renderer) Render(page int, scale float64, clip image.Rectangle, aaLevel int) (*RenderedPage, error) {
	if r.handle == nil {
		return nil, fmt.Errorf("render page: renderer is closed")
	}
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return nil, fmt.Errorf("render page: invalid scale %g", scale)
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
	switch C.gopdf_render_display_list(r.handle, list, C.float(scale), irect(clip), C.int(aaLevel), &samples, &width, &height, &stride, &x, &y, &cerr) {
	case 0:
		return nil, consumeError("render page", cerr)
	case 2:
		return nil, ErrCancelled
	}
	if samples == nil {
		return &RenderedPage{Image: image.NewRGBA(image.Rect(0, 0, 0, 0)), X: int(x), Y: int(y)}, nil
	}
	img := &image.RGBA{Pix: unsafe.Slice((*byte)(unsafe.Pointer(samples)), int(stride)*int(height)), Stride: int(stride), Rect: image.Rect(0, 0, int(width), int(height))}
	return &RenderedPage{Image: img, X: int(x), Y: int(y), samples: unsafe.Pointer(samples)}, nil
}

func irect(r image.Rectangle) C.gopdf_irect {
	return C.gopdf_irect{x0: C.int(r.Min.X), y0: C.int(r.Min.Y), x1: C.int(r.Max.X), y1: C.int(r.Max.Y)}
}

// DeviceRect is the pixel bounding box of a page at scale, rounded as
// MuPDF rounds it when rendering.
func DeviceRect(bounds Rect, scale float64) image.Rectangle {
	const epsilon = 0.001
	s := float32(scale)
	return image.Rect(
		int(math.Floor(float64(bounds.X0*s+epsilon))),
		int(math.Floor(float64(bounds.Y0*s+epsilon))),
		int(math.Ceil(float64(bounds.X1*s-epsilon))),
		int(math.Ceil(float64(bounds.Y1*s-epsilon))),
	)
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
