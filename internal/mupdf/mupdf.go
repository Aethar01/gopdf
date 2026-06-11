package mupdf

/*
#cgo pkg-config: mupdf
#include <stdlib.h>
#include "mupdf_bridge.h"
*/
import "C"

import (
	"fmt"
	"image"
	"math"
	"strings"
	"sync"
	"unsafe"
)

const passwordErrorText = "invalid or missing document password"

type Rect struct {
	X0 float32
	Y0 float32
	X1 float32
	Y1 float32
}

func (r Rect) Width() float64 {
	return float64(r.X1 - r.X0)
}

func (r Rect) Height() float64 {
	return float64(r.Y1 - r.Y0)
}

type Document struct {
	mu     sync.Mutex
	handle *C.gopdf_doc
	pages  int
}

type RenderedPage struct {
	Image   *image.RGBA
	X       int
	Y       int
	samples unsafe.Pointer
}

func (p *RenderedPage) Close() {
	if p == nil || p.samples == nil {
		return
	}
	C.gopdf_free_rendered_page((*C.uchar)(p.samples))
	p.samples = nil
	if p.Image != nil {
		p.Image.Pix = nil
	}
}

type Point struct {
	X float64
	Y float64
}

type Quad struct {
	UL Point
	UR Point
	LL Point
	LR Point
}

type Selection struct {
	Text  string
	Quads []Quad
}

type SearchHit struct {
	Quads []Quad
}

type Link struct {
	Bounds   Rect
	URI      string
	External bool
	Page     int
	X        float64
	Y        float64
	HasX     bool
	HasY     bool
}

type OutlineItem struct {
	Title       string
	URI         string
	External    bool
	Page        int
	X           float64
	Y           float64
	HasX        bool
	HasY        bool
	Depth       int
	Parent      int
	HasChildren bool
}

func (d *Document) ensureOpenLocked() error {
	if d == nil || d.handle == nil {
		return fmt.Errorf("document is closed")
	}
	return nil
}

func (d *Document) validatePageLocked(page int) error {
	if err := d.ensureOpenLocked(); err != nil {
		return err
	}
	if page < 0 || page >= d.pages {
		return fmt.Errorf("page %d out of range [0,%d)", page, d.pages)
	}
	return nil
}

func Open(path string, password string) (*Document, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cpassword *C.char
	if password != "" {
		cpassword = C.CString(password)
		defer C.free(unsafe.Pointer(cpassword))
	}
	var cerr *C.char
	handle := C.gopdf_open_document(cpath, cpassword, &cerr)
	if handle == nil {
		return nil, consumeError("open document", cerr)
	}
	d := &Document{handle: handle}
	count, err := d.PageCount()
	if err != nil {
		d.Close()
		return nil, err
	}
	d.pages = count
	return d, nil
}

func IsPasswordError(err error) bool {
	return err != nil && strings.Contains(err.Error(), passwordErrorText)
}

func (d *Document) Close() {
	if d == nil {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.handle == nil {
		return
	}
	C.gopdf_close_document(d.handle)
	d.handle = nil
}

func (d *Document) PageCount() (int, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return 0, err
	}
	var count C.int
	var cerr *C.char
	if ok := C.gopdf_count_pages(d.handle, &count, &cerr); ok == 0 {
		return 0, consumeError("count pages", cerr)
	}
	return int(count), nil
}

func (d *Document) CachedPageCount() int {
	if d == nil {
		return 0
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.pages
}

func (d *Document) Bounds(page int) (Rect, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return Rect{}, err
	}
	var rect C.gopdf_rect
	var cerr *C.char
	if ok := C.gopdf_page_bounds(d.handle, C.int(page), &rect, &cerr); ok == 0 {
		return Rect{}, consumeError("page bounds", cerr)
	}
	return Rect{X0: float32(rect.x0), Y0: float32(rect.y0), X1: float32(rect.x1), Y1: float32(rect.y1)}, nil
}

func (d *Document) PageLabel(page int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return "", err
	}
	var label *C.char
	var cerr *C.char
	if ok := C.gopdf_page_label(d.handle, C.int(page), &label, &cerr); ok == 0 {
		return "", consumeError("page label", cerr)
	}
	if label == nil {
		return "", nil
	}
	defer C.free(unsafe.Pointer(label))
	return C.GoString(label), nil
}

func (d *Document) Render(page int, scale float64, rotation float64, aaLevel int) (*RenderedPage, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
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
	var samples *C.uchar
	var width, height, stride, x, y C.int
	var cerr *C.char
	if ok := C.gopdf_render_page_alloc(d.handle, C.int(page), C.float(scale), C.float(rotation), C.int(aaLevel), &samples, &width, &height, &stride, &x, &y, &cerr); ok == 0 {
		return nil, consumeError("render page", cerr)
	}
	if width <= 0 || height <= 0 || stride <= 0 {
		return &RenderedPage{Image: image.NewRGBA(image.Rect(0, 0, 0, 0)), X: int(x), Y: int(y)}, nil
	}
	if int(stride) != int(width)*4 {
		return nil, fmt.Errorf("render page: unsupported pixmap stride %d for width %d", int(stride), int(width))
	}
	if samples == nil {
		return nil, fmt.Errorf("render page: missing pixel buffer")
	}
	bufLen := int(stride) * int(height)
	img := &image.RGBA{Pix: unsafe.Slice((*byte)(unsafe.Pointer(samples)), bufLen), Stride: int(stride), Rect: image.Rect(0, 0, int(width), int(height))}
	return &RenderedPage{Image: img, X: int(x), Y: int(y), samples: unsafe.Pointer(samples)}, nil
}

func (d *Document) CancelRender() {
	if d == nil || d.handle == nil {
		return
	}
	C.gopdf_cancel_render(d.handle)
}

func (d *Document) ExtractSelection(page int, a, b Point) (*Selection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var sel C.gopdf_selection
	var cerr *C.char
	if ok := C.gopdf_extract_selection(d.handle, C.int(page), C.float(a.X), C.float(a.Y), C.float(b.X), C.float(b.Y), &sel, &cerr); ok == 0 {
		return nil, consumeError("extract selection", cerr)
	}
	defer C.gopdf_free_selection(d.handle, &sel)
	result := &Selection{}
	if sel.text != nil {
		result.Text = C.GoString(sel.text)
	}
	result.Quads = copyQuads(sel.quads, int(sel.quad_count))
	return result, nil
}

func (d *Document) SearchPage(page int, needle string) ([]SearchHit, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	if needle == "" {
		return nil, nil
	}
	cneedle := C.CString(needle)
	defer C.free(unsafe.Pointer(cneedle))
	var result C.gopdf_search_result
	var cerr *C.char
	if ok := C.gopdf_search_page(d.handle, C.int(page), cneedle, &result, &cerr); ok == 0 {
		return nil, consumeError("search page", cerr)
	}
	defer C.gopdf_free_search_result(&result)
	if result.hit_count == 0 || result.hits == nil {
		return nil, nil
	}
	rawHits := unsafe.Slice(result.hits, int(result.hit_count))
	hits := make([]SearchHit, len(rawHits))
	for i, rawHit := range rawHits {
		hits[i].Quads = copyQuads(rawHit.quads, int(rawHit.quad_count))
	}
	return hits, nil
}

func (d *Document) PageText(page int) (string, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return "", err
	}
	var text *C.char
	var cerr *C.char
	if ok := C.gopdf_extract_page_text(d.handle, C.int(page), &text, &cerr); ok == 0 {
		return "", consumeError("extract page text", cerr)
	}
	defer C.gopdf_free_text(d.handle, text)
	if text == nil {
		return "", nil
	}
	return C.GoString(text), nil
}

func (d *Document) Links(page int) ([]Link, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var result C.gopdf_link_result
	var cerr *C.char
	if ok := C.gopdf_load_links(d.handle, C.int(page), &result, &cerr); ok == 0 {
		return nil, consumeError("load links", cerr)
	}
	defer C.gopdf_free_link_result(&result)
	if result.link_count == 0 || result.links == nil {
		return nil, nil
	}
	raw := unsafe.Slice(result.links, int(result.link_count))
	links := make([]Link, len(raw))
	for i, link := range raw {
		x, y := float64(link.x), float64(link.y)
		hasX, hasY := link.has_x != 0 && !math.IsNaN(x), link.has_y != 0 && !math.IsNaN(y)
		if !hasX {
			x = 0
		}
		if !hasY {
			y = 0
		}
		links[i] = Link{
			Bounds:   Rect{X0: float32(link.rect.x0), Y0: float32(link.rect.y0), X1: float32(link.rect.x1), Y1: float32(link.rect.y1)},
			URI:      goString(link.uri),
			External: link.is_external != 0,
			Page:     int(link.page_number),
			X:        x,
			Y:        y,
			HasX:     hasX,
			HasY:     hasY,
		}
	}
	return links, nil
}

func (d *Document) Outline() ([]OutlineItem, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return nil, err
	}
	var result C.gopdf_outline_result
	var cerr *C.char
	if ok := C.gopdf_load_outline(d.handle, &result, &cerr); ok == 0 {
		return nil, consumeError("load outline", cerr)
	}
	defer C.gopdf_free_outline_result(&result)
	if result.item_count == 0 || result.items == nil {
		return nil, nil
	}
	raw := unsafe.Slice(result.items, int(result.item_count))
	items := make([]OutlineItem, len(raw))
	for i, item := range raw {
		x, y := float64(item.x), float64(item.y)
		hasX, hasY := item.has_x != 0 && !math.IsNaN(x), item.has_y != 0 && !math.IsNaN(y)
		if !hasX {
			x = 0
		}
		if !hasY {
			y = 0
		}
		items[i] = OutlineItem{
			Title:       goString(item.title),
			URI:         goString(item.uri),
			External:    item.is_external != 0,
			Page:        int(item.page_number),
			X:           x,
			Y:           y,
			HasX:        hasX,
			HasY:        hasY,
			Depth:       int(item.depth),
			Parent:      int(item.parent),
			HasChildren: item.has_children != 0,
		}
	}
	return items, nil
}

func copyQuads(raw *C.gopdf_quad, count int) []Quad {
	if count <= 0 || raw == nil {
		return nil
	}
	rawQuads := unsafe.Slice(raw, count)
	quads := make([]Quad, len(rawQuads))
	for i, q := range rawQuads {
		quads[i] = Quad{
			UL: Point{X: float64(q.ul.x), Y: float64(q.ul.y)},
			UR: Point{X: float64(q.ur.x), Y: float64(q.ur.y)},
			LL: Point{X: float64(q.ll.x), Y: float64(q.ll.y)},
			LR: Point{X: float64(q.lr.x), Y: float64(q.lr.y)},
		}
	}
	return quads
}

func goString(s *C.char) string {
	if s == nil {
		return ""
	}
	return C.GoString(s)
}

func consumeError(prefix string, cerr *C.char) error {
	if cerr == nil {
		return fmt.Errorf("%s: unknown mupdf error", prefix)
	}
	defer C.free(unsafe.Pointer(cerr))
	return fmt.Errorf("%s: %s", prefix, C.GoString(cerr))
}
