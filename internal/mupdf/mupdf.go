package mupdf

/*
#cgo !darwin pkg-config: mupdf
#cgo darwin LDFLAGS: -lmupdf -lmupdf-third -lm
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

// pageCacheSize is how many loaded pages the bridge keeps per document.
const pageCacheSize = C.GOPDF_PAGE_CACHE_SIZE

type Rect struct {
	X0 float32
	Y0 float32
	X1 float32
	Y1 float32
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

func (r Rect) Empty() bool { return r.X0 >= r.X1 || r.Y0 >= r.Y1 }

func (r Rect) Contains(p Point) bool {
	return p.X >= float64(r.X0) && p.X <= float64(r.X1) && p.Y >= float64(r.Y0) && p.Y <= float64(r.Y1)
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

type Metadata struct {
	Format           string
	Encryption       string
	Title            string
	Author           string
	Subject          string
	Keywords         string
	Creator          string
	Producer         string
	CreationDate     string
	ModificationDate string
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

type OpenOptions struct {
	Password string
	// StoreBytes caps MuPDF's cache of fonts and decoded images; 0 uses
	// MuPDF's default of 256 MiB.
	StoreBytes int64
}

func Open(path string, opts OpenOptions) (*Document, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cpassword *C.char
	if opts.Password != "" {
		cpassword = C.CString(opts.Password)
		defer C.free(unsafe.Pointer(cpassword))
	}
	store := C.size_t(C.FZ_STORE_DEFAULT)
	if opts.StoreBytes > 0 {
		store = C.size_t(opts.StoreBytes)
	}
	var cerr *C.char
	handle := C.gopdf_open_document(cpath, cpassword, store, &cerr)
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

// ContentBounds returns the bounds of everything drawn on a page, or an
// empty rect for a blank page.
func (d *Document) ContentBounds(page int) (Rect, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return Rect{}, err
	}
	var r C.gopdf_rect
	var cerr *C.char
	if ok := C.gopdf_page_content_bounds(d.handle, C.int(page), &r, &cerr); ok == 0 {
		return Rect{}, consumeError("page content bounds", cerr)
	}
	return Rect{X0: float32(r.x0), Y0: float32(r.y0), X1: float32(r.x1), Y1: float32(r.y1)}, nil
}

// ImageBounds returns the bounds of the raster images drawn on a page.
func (d *Document) ImageBounds(page int) ([]Rect, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var raw *C.gopdf_rect
	var count C.int
	var cerr *C.char
	if ok := C.gopdf_page_image_bounds(d.handle, C.int(page), &raw, &count, &cerr); ok == 0 {
		return nil, consumeError("page images", cerr)
	}
	defer C.free(unsafe.Pointer(raw))
	rects := make([]Rect, int(count))
	for i, r := range unsafe.Slice(raw, int(count)) {
		rects[i] = Rect{X0: float32(r.x0), Y0: float32(r.y0), X1: float32(r.x1), Y1: float32(r.y1)}
	}
	return rects, nil
}

type PageInfo struct {
	Bounds Rect
	Label  string
}

func (d *Document) PageInfo(page int) (PageInfo, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return PageInfo{}, err
	}
	var rect C.gopdf_rect
	var label, cerr *C.char
	if ok := C.gopdf_page_info(d.handle, C.int(page), &rect, &label, &cerr); ok == 0 {
		return PageInfo{}, consumeError("page info", cerr)
	}
	info := PageInfo{Bounds: Rect{X0: float32(rect.x0), Y0: float32(rect.y0), X1: float32(rect.x1), Y1: float32(rect.y1)}}
	if label != nil {
		info.Label = C.GoString(label)
		C.free(unsafe.Pointer(label))
	}
	return info, nil
}

func (d *Document) Metadata() (Metadata, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.ensureOpenLocked(); err != nil {
		return Metadata{}, err
	}
	values := make([]string, 10)
	keys := []string{"format", "encryption", "info:Title", "info:Author", "info:Subject", "info:Keywords", "info:Creator", "info:Producer", "info:CreationDate", "info:ModDate"}
	for i, key := range keys {
		ckey := C.CString(key)
		var value, cerr *C.char
		ok := C.gopdf_lookup_metadata(d.handle, ckey, &value, &cerr)
		C.free(unsafe.Pointer(ckey))
		if ok == 0 {
			return Metadata{}, consumeError("lookup metadata", cerr)
		}
		if value != nil {
			values[i] = C.GoString(value)
			C.gopdf_free_string(value)
		}
	}
	return Metadata{
		Format: values[0], Encryption: values[1], Title: values[2], Author: values[3], Subject: values[4],
		Keywords: values[5], Creator: values[6], Producer: values[7], CreationDate: values[8], ModificationDate: values[9],
	}, nil
}

// SelectMode sets what a selection snaps to.
type SelectMode int

const (
	SelectChars SelectMode = C.FZ_SELECT_CHARS
	SelectWords SelectMode = C.FZ_SELECT_WORDS
	SelectLines SelectMode = C.FZ_SELECT_LINES
)

func (d *Document) ExtractSelection(page int, a, b Point, mode SelectMode) (*Selection, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.validatePageLocked(page); err != nil {
		return nil, err
	}
	var sel C.gopdf_selection
	var cerr *C.char
	if ok := C.gopdf_extract_selection(d.handle, C.int(page), C.float(a.X), C.float(a.Y), C.float(b.X), C.float(b.Y), C.int(mode), &sel, &cerr); ok == 0 {
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
		x, y, hasX, hasY := decodeDestination(link.x, link.y, link.has_x, link.has_y)
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
		x, y, hasX, hasY := decodeDestination(item.x, item.y, item.has_x, item.has_y)
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

func decodeDestination(rawX, rawY C.float, rawHasX, rawHasY C.int) (x, y float64, hasX, hasY bool) {
	x, y = float64(rawX), float64(rawY)
	hasX, hasY = rawHasX != 0 && !math.IsNaN(x), rawHasY != 0 && !math.IsNaN(y)
	if !hasX {
		x = 0
	}
	if !hasY {
		y = 0
	}
	return
}

func copyQuads(raw *C.gopdf_quad, count int) []Quad {
	if count <= 0 || raw == nil {
		return nil
	}
	rawQuads := unsafe.Slice(raw, count)
	quads := make([]Quad, len(rawQuads))
	for i, q := range rawQuads {
		quads[i] = copyQuad(q)
	}
	return quads
}

func copyQuad(q C.gopdf_quad) Quad {
	return Quad{
		UL: Point{X: float64(q.ul.x), Y: float64(q.ul.y)},
		UR: Point{X: float64(q.ur.x), Y: float64(q.ur.y)},
		LL: Point{X: float64(q.ll.x), Y: float64(q.ll.y)},
		LR: Point{X: float64(q.lr.x), Y: float64(q.lr.y)},
	}
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
