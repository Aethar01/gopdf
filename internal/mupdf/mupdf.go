package mupdf

/*
#cgo pkg-config: mupdf
#include <stdlib.h>
#include <string.h>
#include <mupdf/fitz.h>
#include <mupdf/fitz/util.h>

typedef struct {
	fz_context *ctx;
	fz_document *doc;
} gopdf_doc;

typedef struct {
	float x0;
	float y0;
	float x1;
	float y1;
} gopdf_rect;

typedef struct {
	int width;
	int height;
	int stride;
	int x;
	int y;
	unsigned char *samples;
} gopdf_pixmap;

typedef struct {
	float x;
	float y;
} gopdf_point;

typedef struct {
	gopdf_point ul;
	gopdf_point ur;
	gopdf_point ll;
	gopdf_point lr;
} gopdf_quad;

typedef struct {
	char *text;
	gopdf_quad *quads;
	int quad_count;
} gopdf_selection;

static char *gopdf_dup_string(const char *src) {
	size_t n = strlen(src) + 1;
	char *dst = (char *)malloc(n);
	if (dst != NULL) {
		memcpy(dst, src, n);
	}
	return dst;
}

static gopdf_doc *gopdf_open_document(const char *path, char **err) {
	gopdf_doc *handle = NULL;
	fz_context *ctx = NULL;
	fz_document *doc = NULL;
	*err = NULL;
	ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
	if (ctx == NULL) {
		*err = gopdf_dup_string("fz_new_context failed");
		return NULL;
	}
	fz_try(ctx) {
		fz_register_document_handlers(ctx);
		doc = fz_open_document(ctx, path);
	} fz_catch(ctx) {
		*err = gopdf_dup_string(fz_caught_message(ctx));
	}
	if (*err != NULL) {
		if (doc != NULL) {
			fz_drop_document(ctx, doc);
		}
		fz_drop_context(ctx);
		return NULL;
	}
	handle = (gopdf_doc *)malloc(sizeof(gopdf_doc));
	if (handle == NULL) {
		fz_drop_document(ctx, doc);
		fz_drop_context(ctx);
		*err = gopdf_dup_string("malloc failed");
		return NULL;
	}
	handle->ctx = ctx;
	handle->doc = doc;
	return handle;
}

static void gopdf_close_document(gopdf_doc *handle) {
	if (handle == NULL) {
		return;
	}
	if (handle->doc != NULL) {
		fz_drop_document(handle->ctx, handle->doc);
	}
	if (handle->ctx != NULL) {
		fz_drop_context(handle->ctx);
	}
	free(handle);
}

static int gopdf_count_pages(gopdf_doc *handle, int *count, char **err) {
	*err = NULL;
	*count = 0;
	fz_try(handle->ctx) {
		*count = fz_count_pages(handle->ctx, handle->doc);
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static int gopdf_page_bounds(gopdf_doc *handle, int page_number, gopdf_rect *out, char **err) {
	fz_page *page = NULL;
	fz_rect bounds = fz_empty_rect;
	*err = NULL;
	fz_var(page);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		bounds = fz_bound_page(handle->ctx, page);
	} fz_always(handle->ctx) {
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	out->x0 = bounds.x0;
	out->y0 = bounds.y0;
	out->x1 = bounds.x1;
	out->y1 = bounds.y1;
	return 1;
}

static int gopdf_render_page(gopdf_doc *handle, int page_number, float scale, float rotation, gopdf_pixmap *out, char **err) {
	fz_page *page = NULL;
	fz_pixmap *pix = NULL;
	fz_matrix ctm;
	int size = 0;
	unsigned char *samples = NULL;
	*err = NULL;
	out->width = 0;
	out->height = 0;
	out->stride = 0;
	out->x = 0;
	out->y = 0;
	out->samples = NULL;
	fz_var(page);
	fz_var(pix);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		ctm = fz_concat(fz_scale(scale, scale), fz_rotate(rotation));
		pix = fz_new_pixmap_from_page(handle->ctx, page, ctm, fz_device_rgb(handle->ctx), 1);
		out->width = fz_pixmap_width(handle->ctx, pix);
		out->height = fz_pixmap_height(handle->ctx, pix);
		out->stride = fz_pixmap_stride(handle->ctx, pix);
		out->x = fz_pixmap_x(handle->ctx, pix);
		out->y = fz_pixmap_y(handle->ctx, pix);
		size = out->stride * out->height;
		samples = (unsigned char *)malloc(size);
		if (samples == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "malloc failed");
		}
		memcpy(samples, fz_pixmap_samples(handle->ctx, pix), size);
		out->samples = samples;
	} fz_always(handle->ctx) {
		if (pix != NULL) {
			fz_drop_pixmap(handle->ctx, pix);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		if (samples != NULL) {
			free(samples);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static void gopdf_free_pixmap(gopdf_pixmap *pix) {
	if (pix->samples != NULL) {
		free(pix->samples);
		pix->samples = NULL;
	}
}

static int gopdf_extract_selection(gopdf_doc *handle, int page_number, float ax, float ay, float bx, float by, gopdf_selection *out, char **err) {
	fz_page *page = NULL;
	fz_stext_page *text = NULL;
	fz_point a = { ax, ay };
	fz_point b = { bx, by };
	char *copied = NULL;
	fz_quad stack_quads[512];
	gopdf_quad *heap_quads = NULL;
	int count = 0;
	*err = NULL;
	out->text = NULL;
	out->quads = NULL;
	out->quad_count = 0;
	fz_var(page);
	fz_var(text);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		text = fz_new_stext_page_from_page(handle->ctx, page, NULL);
		copied = fz_copy_selection(handle->ctx, text, a, b, 0);
		count = fz_highlight_selection(handle->ctx, text, a, b, stack_quads, 512);
		if (count > 0) {
			heap_quads = (gopdf_quad *)malloc(sizeof(gopdf_quad) * count);
			if (heap_quads == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "malloc failed");
			}
			for (int i = 0; i < count; i++) {
				heap_quads[i].ul.x = stack_quads[i].ul.x;
				heap_quads[i].ul.y = stack_quads[i].ul.y;
				heap_quads[i].ur.x = stack_quads[i].ur.x;
				heap_quads[i].ur.y = stack_quads[i].ur.y;
				heap_quads[i].ll.x = stack_quads[i].ll.x;
				heap_quads[i].ll.y = stack_quads[i].ll.y;
				heap_quads[i].lr.x = stack_quads[i].lr.x;
				heap_quads[i].lr.y = stack_quads[i].lr.y;
			}
		}
		out->text = copied;
		out->quads = heap_quads;
		out->quad_count = count;
	} fz_always(handle->ctx) {
		if (text != NULL) {
			fz_drop_stext_page(handle->ctx, text);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		if (copied != NULL) {
			fz_free(handle->ctx, copied);
		}
		if (heap_quads != NULL) {
			free(heap_quads);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static void gopdf_free_selection(gopdf_doc *handle, gopdf_selection *sel) {
	if (sel->text != NULL) {
		fz_free(handle->ctx, sel->text);
		sel->text = NULL;
	}
	if (sel->quads != NULL) {
		free(sel->quads);
		sel->quads = NULL;
	}
	sel->quad_count = 0;
}

*/
import "C"

import (
	"fmt"
	"image"
	"unsafe"
)

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
	handle *C.gopdf_doc
	pages  int
}

type RenderedPage struct {
	Image *image.RGBA
	X     int
	Y     int
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

func Open(path string) (*Document, error) {
	cpath := C.CString(path)
	defer C.free(unsafe.Pointer(cpath))
	var cerr *C.char
	handle := C.gopdf_open_document(cpath, &cerr)
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

func (d *Document) Close() {
	if d.handle == nil {
		return
	}
	C.gopdf_close_document(d.handle)
	d.handle = nil
}

func (d *Document) PageCount() (int, error) {
	var count C.int
	var cerr *C.char
	if ok := C.gopdf_count_pages(d.handle, &count, &cerr); ok == 0 {
		return 0, consumeError("count pages", cerr)
	}
	return int(count), nil
}

func (d *Document) Bounds(page int) (Rect, error) {
	var rect C.gopdf_rect
	var cerr *C.char
	if ok := C.gopdf_page_bounds(d.handle, C.int(page), &rect, &cerr); ok == 0 {
		return Rect{}, consumeError("page bounds", cerr)
	}
	return Rect{X0: float32(rect.x0), Y0: float32(rect.y0), X1: float32(rect.x1), Y1: float32(rect.y1)}, nil
}

func (d *Document) Render(page int, scale float64, rotation float64) (*RenderedPage, error) {
	var pix C.gopdf_pixmap
	var cerr *C.char
	if ok := C.gopdf_render_page(d.handle, C.int(page), C.float(scale), C.float(rotation), &pix, &cerr); ok == 0 {
		return nil, consumeError("render page", cerr)
	}
	defer C.gopdf_free_pixmap(&pix)
	size := int(pix.stride) * int(pix.height)
	buf := C.GoBytes(unsafe.Pointer(pix.samples), C.int(size))
	img := &image.RGBA{
		Pix:    buf,
		Stride: int(pix.stride),
		Rect:   image.Rect(0, 0, int(pix.width), int(pix.height)),
	}
	return &RenderedPage{Image: img, X: int(pix.x), Y: int(pix.y)}, nil
}

func (d *Document) ExtractSelection(page int, a, b Point) (*Selection, error) {
	var sel C.gopdf_selection
	var cerr *C.char
	if ok := C.gopdf_extract_selection(d.handle, C.int(page), C.float(a.X), C.float(a.Y), C.float(b.X), C.float(b.Y), &sel, &cerr); ok == 0 {
		return nil, consumeError("extract selection", cerr)
	}
	defer C.gopdf_free_selection(d.handle, &sel)
	result := &Selection{Text: C.GoString(sel.text)}
	if sel.quad_count > 0 && sel.quads != nil {
		quads := unsafe.Slice(sel.quads, int(sel.quad_count))
		result.Quads = make([]Quad, len(quads))
		for i, q := range quads {
			result.Quads[i] = Quad{
				UL: Point{X: float64(q.ul.x), Y: float64(q.ul.y)},
				UR: Point{X: float64(q.ur.x), Y: float64(q.ur.y)},
				LL: Point{X: float64(q.ll.x), Y: float64(q.ll.y)},
				LR: Point{X: float64(q.lr.x), Y: float64(q.lr.y)},
			}
		}
	}
	return result, nil
}

func consumeError(prefix string, cerr *C.char) error {
	if cerr == nil {
		return fmt.Errorf("%s: unknown mupdf error", prefix)
	}
	defer C.free(unsafe.Pointer(cerr))
	return fmt.Errorf("%s: %s", prefix, C.GoString(cerr))
}
