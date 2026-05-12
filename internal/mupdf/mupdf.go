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

typedef struct {
	gopdf_quad *quads;
	int quad_count;
} gopdf_search_hit;

typedef struct {
	gopdf_search_hit *hits;
	int hit_count;
} gopdf_search_result;

typedef struct {
	gopdf_rect rect;
	char *uri;
	int is_external;
	int page_number;
	float x;
	float y;
} gopdf_link;

typedef struct {
	gopdf_link *links;
	int link_count;
} gopdf_link_result;

typedef struct {
	char *title;
	int page_number;
	int depth;
	int parent;
	int has_children;
} gopdf_outline_item;

typedef struct {
	gopdf_outline_item *items;
	int item_count;
} gopdf_outline_result;

typedef struct {
	gopdf_search_hit *hits;
	int hit_count;
	int hit_cap;
} gopdf_search_builder;

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

static int gopdf_render_page(gopdf_doc *handle, int page_number, float scale, float rotation, int aa_level, gopdf_pixmap *out, char **err) {
	fz_page *page = NULL;
	fz_pixmap *pix = NULL;
	fz_matrix ctm;
	int size = 0;
	int old_aa = 0;
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
		old_aa = fz_aa_level(handle->ctx);
		fz_set_aa_level(handle->ctx, aa_level);
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
		fz_set_aa_level(handle->ctx, old_aa);
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

static void gopdf_free_search_builder(gopdf_search_builder *builder) {
	if (builder == NULL) {
		return;
	}
	for (int i = 0; i < builder->hit_count; i++) {
		if (builder->hits[i].quads != NULL) {
			free(builder->hits[i].quads);
			builder->hits[i].quads = NULL;
		}
		builder->hits[i].quad_count = 0;
	}
	if (builder->hits != NULL) {
		free(builder->hits);
		builder->hits = NULL;
	}
	builder->hit_count = 0;
	builder->hit_cap = 0;
}

static int gopdf_collect_search_hit(fz_context *ctx, void *opaque, int num_quads, fz_quad *hit_bbox) {
	gopdf_search_builder *builder = (gopdf_search_builder *)opaque;
	gopdf_search_hit *hits = NULL;
	gopdf_quad *quads = NULL;
	if (num_quads <= 0) {
		return 0;
	}
	if (builder->hit_count == builder->hit_cap) {
		int next_cap = builder->hit_cap == 0 ? 8 : builder->hit_cap * 2;
		hits = (gopdf_search_hit *)realloc(builder->hits, sizeof(gopdf_search_hit) * next_cap);
		if (hits == NULL) {
			fz_throw(ctx, FZ_ERROR_SYSTEM, "realloc failed");
		}
		builder->hits = hits;
		builder->hit_cap = next_cap;
	}
	quads = (gopdf_quad *)malloc(sizeof(gopdf_quad) * num_quads);
	if (quads == NULL) {
		fz_throw(ctx, FZ_ERROR_SYSTEM, "malloc failed");
	}
	for (int i = 0; i < num_quads; i++) {
		quads[i].ul.x = hit_bbox[i].ul.x;
		quads[i].ul.y = hit_bbox[i].ul.y;
		quads[i].ur.x = hit_bbox[i].ur.x;
		quads[i].ur.y = hit_bbox[i].ur.y;
		quads[i].ll.x = hit_bbox[i].ll.x;
		quads[i].ll.y = hit_bbox[i].ll.y;
		quads[i].lr.x = hit_bbox[i].lr.x;
		quads[i].lr.y = hit_bbox[i].lr.y;
	}
	builder->hits[builder->hit_count].quads = quads;
	builder->hits[builder->hit_count].quad_count = num_quads;
	builder->hit_count++;
	return 0;
}

static int gopdf_search_page(gopdf_doc *handle, int page_number, const char *needle, gopdf_search_result *out, char **err) {
	gopdf_search_builder builder = { 0 };
	*err = NULL;
	out->hits = NULL;
	out->hit_count = 0;
	fz_try(handle->ctx) {
		fz_search_page_number_cb(handle->ctx, handle->doc, page_number, needle, gopdf_collect_search_hit, &builder);
		out->hits = builder.hits;
		out->hit_count = builder.hit_count;
	} fz_catch(handle->ctx) {
		gopdf_free_search_builder(&builder);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static void gopdf_free_search_result(gopdf_search_result *result) {
	if (result == NULL) {
		return;
	}
	for (int i = 0; i < result->hit_count; i++) {
		if (result->hits[i].quads != NULL) {
			free(result->hits[i].quads);
			result->hits[i].quads = NULL;
		}
		result->hits[i].quad_count = 0;
	}
	if (result->hits != NULL) {
		free(result->hits);
		result->hits = NULL;
	}
	result->hit_count = 0;
}

static int gopdf_load_links(gopdf_doc *handle, int page_number, gopdf_link_result *out, char **err) {
	fz_page *page = NULL;
	fz_link *links = NULL;
	gopdf_link *items = NULL;
	int count = 0;
	*err = NULL;
	out->links = NULL;
	out->link_count = 0;
	fz_var(page);
	fz_var(links);
	fz_try(handle->ctx) {
		float xp = 0;
		float yp = 0;
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		links = fz_load_links(handle->ctx, page);
		for (fz_link *link = links; link != NULL; link = link->next) {
			count++;
		}
		if (count > 0) {
			items = (gopdf_link *)calloc(count, sizeof(gopdf_link));
			if (items == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "calloc failed");
			}
			int i = 0;
			for (fz_link *link = links; link != NULL; link = link->next, i++) {
				items[i].rect.x0 = link->rect.x0;
				items[i].rect.y0 = link->rect.y0;
				items[i].rect.x1 = link->rect.x1;
				items[i].rect.y1 = link->rect.y1;
				items[i].uri = link->uri ? gopdf_dup_string(link->uri) : NULL;
				items[i].is_external = link->uri ? fz_is_external_link(handle->ctx, link->uri) : 0;
				items[i].page_number = -1;
				items[i].x = 0;
				items[i].y = 0;
				if (link->uri != NULL && !items[i].is_external) {
					fz_location loc = fz_resolve_link(handle->ctx, handle->doc, link->uri, &xp, &yp);
					items[i].page_number = fz_page_number_from_location(handle->ctx, handle->doc, loc);
					items[i].x = xp;
					items[i].y = yp;
				}
			}
		}
		out->links = items;
		out->link_count = count;
	} fz_always(handle->ctx) {
		if (links != NULL) {
			fz_drop_link(handle->ctx, links);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		if (items != NULL) {
			for (int i = 0; i < count; i++) {
				if (items[i].uri != NULL) {
					free(items[i].uri);
				}
			}
			free(items);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static void gopdf_free_link_result(gopdf_link_result *result) {
	if (result == NULL) {
		return;
	}
	if (result->links != NULL) {
		for (int i = 0; i < result->link_count; i++) {
			if (result->links[i].uri != NULL) {
				free(result->links[i].uri);
				result->links[i].uri = NULL;
			}
		}
		free(result->links);
		result->links = NULL;
	}
	result->link_count = 0;
}

static int gopdf_count_outline_items(fz_outline *outline) {
	int count = 0;
	for (fz_outline *node = outline; node != NULL; node = node->next) {
		count++;
		if (node->down != NULL) {
			count += gopdf_count_outline_items(node->down);
		}
	}
	return count;
}

static void gopdf_fill_outline_items(gopdf_doc *handle, fz_outline *outline, int depth, int parent, gopdf_outline_item *items, int *index) {
	for (fz_outline *node = outline; node != NULL; node = node->next) {
		int current = *index;
		items[current].title = node->title ? gopdf_dup_string(node->title) : gopdf_dup_string("");
		items[current].page_number = -1;
		items[current].depth = depth;
		items[current].parent = parent;
		items[current].has_children = node->down != NULL;
		if (node->uri != NULL) {
			float xp = 0;
			float yp = 0;
			fz_location loc = fz_resolve_link(handle->ctx, handle->doc, node->uri, &xp, &yp);
			items[current].page_number = fz_page_number_from_location(handle->ctx, handle->doc, loc);
		} else if (node->page.page >= 0) {
			items[current].page_number = fz_page_number_from_location(handle->ctx, handle->doc, node->page);
		}
		(*index)++;
		if (node->down != NULL) {
			gopdf_fill_outline_items(handle, node->down, depth + 1, current, items, index);
		}
	}
}

static int gopdf_load_outline(gopdf_doc *handle, gopdf_outline_result *out, char **err) {
	fz_outline *outline = NULL;
	gopdf_outline_item *items = NULL;
	int count = 0;
	int index = 0;
	*err = NULL;
	out->items = NULL;
	out->item_count = 0;
	fz_var(outline);
	fz_var(items);
	fz_try(handle->ctx) {
		outline = fz_load_outline(handle->ctx, handle->doc);
		count = gopdf_count_outline_items(outline);
		if (count > 0) {
			items = (gopdf_outline_item *)calloc(count, sizeof(gopdf_outline_item));
			if (items == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "calloc failed");
			}
			gopdf_fill_outline_items(handle, outline, 0, -1, items, &index);
		}
		out->items = items;
		out->item_count = count;
	} fz_always(handle->ctx) {
		if (outline != NULL) {
			fz_drop_outline(handle->ctx, outline);
		}
	} fz_catch(handle->ctx) {
		if (items != NULL) {
			for (int i = 0; i < count; i++) {
				if (items[i].title != NULL) {
					free(items[i].title);
				}
			}
			free(items);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static void gopdf_free_outline_result(gopdf_outline_result *result) {
	if (result == NULL || result->items == NULL) {
		return;
	}
	for (int i = 0; i < result->item_count; i++) {
		if (result->items[i].title != NULL) {
			free(result->items[i].title);
			result->items[i].title = NULL;
		}
	}
	free(result->items);
	result->items = NULL;
	result->item_count = 0;
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
		if (count > 512) {
			count = 512;
		}
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
}

type OutlineItem struct {
	Title       string
	Page        int
	Depth       int
	Parent      int
	HasChildren bool
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

func (d *Document) Render(page int, scale float64, rotation float64, aaLevel int) (*RenderedPage, error) {
	var pix C.gopdf_pixmap
	var cerr *C.char
	if ok := C.gopdf_render_page(d.handle, C.int(page), C.float(scale), C.float(rotation), C.int(aaLevel), &pix, &cerr); ok == 0 {
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

func (d *Document) SearchPage(page int, needle string) ([]SearchHit, error) {
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
		if rawHit.quad_count <= 0 || rawHit.quads == nil {
			continue
		}
		rawQuads := unsafe.Slice(rawHit.quads, int(rawHit.quad_count))
		hits[i].Quads = make([]Quad, len(rawQuads))
		for j, q := range rawQuads {
			hits[i].Quads[j] = Quad{
				UL: Point{X: float64(q.ul.x), Y: float64(q.ul.y)},
				UR: Point{X: float64(q.ur.x), Y: float64(q.ur.y)},
				LL: Point{X: float64(q.ll.x), Y: float64(q.ll.y)},
				LR: Point{X: float64(q.lr.x), Y: float64(q.lr.y)},
			}
		}
	}
	return hits, nil
}

func (d *Document) Links(page int) ([]Link, error) {
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
		links[i] = Link{
			Bounds:   Rect{X0: float32(link.rect.x0), Y0: float32(link.rect.y0), X1: float32(link.rect.x1), Y1: float32(link.rect.y1)},
			URI:      C.GoString(link.uri),
			External: link.is_external != 0,
			Page:     int(link.page_number),
			X:        float64(link.x),
			Y:        float64(link.y),
		}
	}
	return links, nil
}

func (d *Document) Outline() ([]OutlineItem, error) {
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
		items[i] = OutlineItem{
			Title:       C.GoString(item.title),
			Page:        int(item.page_number),
			Depth:       int(item.depth),
			Parent:      int(item.parent),
			HasChildren: item.has_children != 0,
		}
	}
	return items, nil
}

func consumeError(prefix string, cerr *C.char) error {
	if cerr == nil {
		return fmt.Errorf("%s: unknown mupdf error", prefix)
	}
	defer C.free(unsafe.Pointer(cerr))
	return fmt.Errorf("%s: %s", prefix, C.GoString(cerr))
}
