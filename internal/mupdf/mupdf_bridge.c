#include "mupdf_bridge.h"

#include <math.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <mupdf/fitz/util.h>

typedef struct {
	gopdf_search_hit *hits;
	int hit_count;
	int hit_cap;
} gopdf_search_builder;

enum { GOPDF_MUPDF_STORE_SIZE = 32 << 20 };

static char *gopdf_dup_string(const char *src) {
	if (src == NULL) {
		return NULL;
	}
	size_t n = strlen(src) + 1;
	char *dst = (char *)malloc(n);
	if (dst != NULL) {
		memcpy(dst, src, n);
	}
	return dst;
}

static char *gopdf_dup_string_or_throw(fz_context *ctx, const char *src) {
	char *dst = gopdf_dup_string(src);
	if (src != NULL && dst == NULL) {
		fz_throw(ctx, FZ_ERROR_SYSTEM, "malloc failed");
	}
	return dst;
}

static void gopdf_silent_callback(void *user, const char *message) {
	(void)user;
	(void)message;
}

static char *gopdf_missing_handler_error(const char *path) {
	const char *prefix = "cannot find document handler for file: ";
	size_t n = strlen(prefix) + strlen(path) + 1;
	char *dst = (char *)malloc(n);
	if (dst != NULL) {
		snprintf(dst, n, "%s%s", prefix, path);
	}
	return dst;
}

static void gopdf_copy_quad(gopdf_quad *dst, const fz_quad *src) {
	dst->ul.x = src->ul.x;
	dst->ul.y = src->ul.y;
	dst->ur.x = src->ur.x;
	dst->ur.y = src->ur.y;
	dst->ll.x = src->ll.x;
	dst->ll.y = src->ll.y;
	dst->lr.x = src->lr.x;
	dst->lr.y = src->lr.y;
}

static fz_matrix gopdf_render_ctm(float scale, float rotation) {
	return fz_concat(fz_scale(scale, scale), fz_rotate(rotation));
}

static fz_page *gopdf_load_cached_page(gopdf_doc *handle, int page_number) {
	if (handle == NULL || page_number < 0) {
		return NULL;
	}
	if (handle->page_count <= 0) {
		handle->page_count = fz_count_pages(handle->ctx, handle->doc);
	}
	if (page_number >= handle->page_count) {
		return NULL;
	}
	if (handle->pages == NULL) {
		handle->pages = (fz_page **)calloc((size_t)handle->page_count, sizeof(fz_page *));
		if (handle->pages == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "calloc failed");
		}
	}
	if (handle->pages[page_number] == NULL) {
		handle->pages[page_number] = fz_load_page(handle->ctx, handle->doc, page_number);
	}
	return handle->pages[page_number];
}

gopdf_doc *gopdf_open_document(const char *path, const char *password, char **err) {
	gopdf_doc *handle = NULL;
	fz_context *ctx = NULL;
	fz_document *doc = NULL;
	const fz_document_handler *handler = NULL;
	int needs_password = 0;
	int authenticated = 1;
	*err = NULL;
	ctx = fz_new_context(NULL, NULL, GOPDF_MUPDF_STORE_SIZE);
	if (ctx == NULL) {
		*err = gopdf_dup_string("fz_new_context failed");
		return NULL;
	}
	fz_set_warning_callback(ctx, gopdf_silent_callback, NULL);
	fz_set_error_callback(ctx, gopdf_silent_callback, NULL);
	fz_try(ctx) {
		fz_register_document_handlers(ctx);
		handler = fz_recognize_document_content(ctx, path);
		if (handler != NULL) {
			doc = fz_open_document(ctx, path);
			needs_password = fz_needs_password(ctx, doc);
			if (needs_password != 0) {
				authenticated = password != NULL && fz_authenticate_password(ctx, doc, password) != 0;
			}
		}
	} fz_catch(ctx) {
		*err = gopdf_dup_string(fz_caught_message(ctx));
	}
	if (*err == NULL && handler == NULL) {
		*err = gopdf_missing_handler_error(path);
	}
	if (*err == NULL && needs_password != 0 && authenticated == 0) {
		*err = gopdf_dup_string("invalid or missing document password");
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
	handle->pages = NULL;
	handle->page_count = 0;
	handle->render_cookie = NULL;
	return handle;
}

void gopdf_close_document(gopdf_doc *handle) {
	if (handle == NULL) {
		return;
	}
	if (handle->doc != NULL) {
		if (handle->pages != NULL) {
			for (int i = 0; i < handle->page_count; i++) {
				if (handle->pages[i] != NULL) {
					fz_drop_page(handle->ctx, handle->pages[i]);
				}
			}
			free(handle->pages);
		}
		fz_drop_document(handle->ctx, handle->doc);
	}
	if (handle->ctx != NULL) {
		fz_drop_context(handle->ctx);
	}
	free(handle);
}

int gopdf_count_pages(gopdf_doc *handle, int *count, char **err) {
	*err = NULL;
	*count = 0;
	fz_try(handle->ctx) {
		*count = fz_count_pages(handle->ctx, handle->doc);
		handle->page_count = *count;
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

int gopdf_page_bounds(gopdf_doc *handle, int page_number, gopdf_rect *out, char **err) {
	fz_page *page = NULL;
	fz_rect bounds = fz_empty_rect;
	*err = NULL;
	fz_var(page);
	fz_try(handle->ctx) {
		page = gopdf_load_cached_page(handle, page_number);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
		}
		bounds = fz_bound_page(handle->ctx, page);
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

int gopdf_page_label(gopdf_doc *handle, int page_number, char **out, char **err) {
	fz_page *page = NULL;
	char buf[64] = { 0 };
	*out = NULL;
	*err = NULL;
	fz_var(page);
	fz_try(handle->ctx) {
		page = gopdf_load_cached_page(handle, page_number);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
		}
		fz_page_label(handle->ctx, page, buf, sizeof(buf));
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	if (buf[0] != '\0') {
		*out = gopdf_dup_string(buf);
		if (*out == NULL) {
			*err = gopdf_dup_string("malloc failed");
			return 0;
		}
	}
	return 1;
}

int gopdf_render_page_info(gopdf_doc *handle, int page_number, float scale, float rotation, int *width, int *height, int *stride, int *x, int *y, char **err) {
	fz_page *page = NULL;
	fz_rect bounds = fz_empty_rect;
	fz_irect bbox = fz_empty_irect;
	fz_matrix ctm = gopdf_render_ctm(scale, rotation);
	*err = NULL;
	*width = 0;
	*height = 0;
	*stride = 0;
	*x = 0;
	*y = 0;
	fz_try(handle->ctx) {
		page = gopdf_load_cached_page(handle, page_number);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
		}
		bounds = fz_bound_page(handle->ctx, page);
		bounds = fz_transform_rect(bounds, ctm);
		bbox = fz_round_rect(bounds);
		*width = bbox.x1 - bbox.x0;
		*height = bbox.y1 - bbox.y0;
		if (*width < 0 || *height < 0 || *width > INT_MAX / 4) {
			fz_throw(handle->ctx, FZ_ERROR_LIMIT, "rendered page dimensions are invalid");
		}
		*stride = *width * 4;
		*x = bbox.x0;
		*y = bbox.y0;
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

int gopdf_render_page_to_buffer(gopdf_doc *handle, int page_number, float scale, float rotation, int aa_level, unsigned char *samples, int width, int height, int stride, char **err) {
	fz_page *page = NULL;
	fz_pixmap *pix = NULL;
	fz_device *dev = NULL;
	fz_rect bounds = fz_empty_rect;
	fz_irect bbox = fz_empty_irect;
	fz_matrix ctm = gopdf_render_ctm(scale, rotation);
	int old_aa = 0;
	int have_old_aa = 0;
	fz_cookie cookie = { 0 };
	*err = NULL;
	fz_var(pix);
	fz_var(dev);
	fz_try(handle->ctx) {
		old_aa = fz_aa_level(handle->ctx);
		have_old_aa = 1;
		fz_set_aa_level(handle->ctx, aa_level);
		page = gopdf_load_cached_page(handle, page_number);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
		}
		bounds = fz_bound_page(handle->ctx, page);
		bounds = fz_transform_rect(bounds, ctm);
		bbox = fz_round_rect(bounds);
		if (bbox.x1 - bbox.x0 != width || bbox.y1 - bbox.y0 != height || stride != width * 4) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "render buffer dimensions do not match page bounds");
		}
		pix = fz_new_pixmap_with_bbox_and_data(handle->ctx, fz_device_rgb(handle->ctx), bbox, NULL, 1, samples);
		fz_clear_pixmap_with_value(handle->ctx, pix, 0xff);
		dev = fz_new_draw_device(handle->ctx, fz_identity, pix);
		handle->render_cookie = &cookie;
		fz_run_page(handle->ctx, page, dev, ctm, &cookie);
		fz_close_device(handle->ctx, dev);
	} fz_always(handle->ctx) {
		handle->render_cookie = NULL;
		if (have_old_aa) {
			fz_set_aa_level(handle->ctx, old_aa);
		}
		if (dev != NULL) {
			fz_drop_device(handle->ctx, dev);
		}
		if (pix != NULL) {
			fz_drop_pixmap(handle->ctx, pix);
		}
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

int gopdf_render_page_alloc(gopdf_doc *handle, int page_number, float scale, float rotation, int aa_level, unsigned char **samples, int *width, int *height, int *stride, int *x, int *y, char **err) {
	fz_page *page = NULL;
	fz_pixmap *pix = NULL;
	fz_device *dev = NULL;
	fz_rect bounds = fz_empty_rect;
	fz_irect bbox = fz_empty_irect;
	fz_matrix ctm = gopdf_render_ctm(scale, rotation);
	int old_aa = 0;
	int have_old_aa = 0;
	fz_cookie cookie = { 0 };
	*err = NULL;
	*samples = NULL;
	*width = 0;
	*height = 0;
	*stride = 0;
	*x = 0;
	*y = 0;
	fz_var(pix);
	fz_var(dev);
	fz_try(handle->ctx) {
		old_aa = fz_aa_level(handle->ctx);
		have_old_aa = 1;
		fz_set_aa_level(handle->ctx, aa_level);
		page = gopdf_load_cached_page(handle, page_number);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
		}
		bounds = fz_bound_page(handle->ctx, page);
		bounds = fz_transform_rect(bounds, ctm);
		bbox = fz_round_rect(bounds);
		*width = bbox.x1 - bbox.x0;
		*height = bbox.y1 - bbox.y0;
		if (*width < 0 || *height < 0 || *width > INT_MAX / 4) {
			fz_throw(handle->ctx, FZ_ERROR_LIMIT, "rendered page dimensions are invalid");
		}
		*stride = *width * 4;
		*x = bbox.x0;
		*y = bbox.y0;
		if (*width > 0 && *height > 0) {
			if (*height > INT_MAX / *stride) {
				fz_throw(handle->ctx, FZ_ERROR_LIMIT, "rendered page buffer is too large");
			}
			*samples = (unsigned char *)malloc((size_t)*stride * (size_t)*height);
			if (*samples == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "malloc failed");
			}
			pix = fz_new_pixmap_with_bbox_and_data(handle->ctx, fz_device_rgb(handle->ctx), bbox, NULL, 1, *samples);
			fz_clear_pixmap_with_value(handle->ctx, pix, 0xff);
			dev = fz_new_draw_device(handle->ctx, fz_identity, pix);
			handle->render_cookie = &cookie;
			fz_run_page(handle->ctx, page, dev, ctm, &cookie);
			fz_close_device(handle->ctx, dev);
		}
	} fz_always(handle->ctx) {
		handle->render_cookie = NULL;
		if (have_old_aa) {
			fz_set_aa_level(handle->ctx, old_aa);
		}
		if (dev != NULL) {
			fz_drop_device(handle->ctx, dev);
		}
		if (pix != NULL) {
			fz_drop_pixmap(handle->ctx, pix);
		}
	} fz_catch(handle->ctx) {
		if (*samples != NULL) {
			free(*samples);
			*samples = NULL;
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_cancel_render(gopdf_doc *handle) {
	if (handle != NULL && handle->render_cookie != NULL) {
		handle->render_cookie->abort = 1;
	}
}

void gopdf_free_rendered_page(unsigned char *samples) {
	free(samples);
}

static void gopdf_free_search_builder(gopdf_search_builder *builder) {
	if (builder == NULL) {
		return;
	}
	for (int i = 0; i < builder->hit_count; i++) {
		free(builder->hits[i].quads);
	}
	free(builder->hits);
	builder->hits = NULL;
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
		gopdf_copy_quad(&quads[i], &hit_bbox[i]);
	}
	builder->hits[builder->hit_count].quads = quads;
	builder->hits[builder->hit_count].quad_count = num_quads;
	builder->hit_count++;
	return 0;
}

int gopdf_search_page(gopdf_doc *handle, int page_number, const char *needle, gopdf_search_result *out, char **err) {
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

void gopdf_free_search_result(gopdf_search_result *result) {
	if (result == NULL) {
		return;
	}
	for (int i = 0; i < result->hit_count; i++) {
		free(result->hits[i].quads);
	}
	free(result->hits);
	result->hits = NULL;
	result->hit_count = 0;
}

int gopdf_load_links(gopdf_doc *handle, int page_number, gopdf_link_result *out, char **err) {
	fz_page *page = NULL;
	fz_link *links = NULL;
	gopdf_link *items = NULL;
	int count = 0;
	*err = NULL;
	out->links = NULL;
	out->link_count = 0;
	fz_var(page);
	fz_var(links);
	fz_var(items);
	fz_try(handle->ctx) {
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
				float xp = NAN;
				float yp = NAN;
				items[i].rect.x0 = link->rect.x0;
				items[i].rect.y0 = link->rect.y0;
				items[i].rect.x1 = link->rect.x1;
				items[i].rect.y1 = link->rect.y1;
				items[i].uri = gopdf_dup_string_or_throw(handle->ctx, link->uri);
				items[i].is_external = link->uri ? fz_is_external_link(handle->ctx, link->uri) : 0;
				items[i].page_number = -1;
				if (link->uri != NULL && !items[i].is_external) {
					fz_location loc = fz_resolve_link(handle->ctx, handle->doc, link->uri, &xp, &yp);
					items[i].page_number = fz_page_number_from_location(handle->ctx, handle->doc, loc);
					if (!isnan(xp)) {
						items[i].x = xp;
						items[i].has_x = 1;
					}
					if (!isnan(yp)) {
						items[i].y = yp;
						items[i].has_y = 1;
					}
				}
			}
		}
		out->links = items;
		out->link_count = count;
		items = NULL;
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
				free(items[i].uri);
			}
			free(items);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_link_result(gopdf_link_result *result) {
	if (result == NULL) {
		return;
	}
	for (int i = 0; i < result->link_count; i++) {
		free(result->links[i].uri);
	}
	free(result->links);
	result->links = NULL;
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
		items[current].title = gopdf_dup_string_or_throw(handle->ctx, node->title ? node->title : "");
		items[current].uri = gopdf_dup_string_or_throw(handle->ctx, node->uri);
		items[current].is_external = node->uri ? fz_is_external_link(handle->ctx, node->uri) : 0;
		items[current].page_number = -1;
		items[current].depth = depth;
		items[current].parent = parent;
		items[current].has_children = node->down != NULL;
		if (node->uri != NULL && !items[current].is_external) {
			float xp = NAN;
			float yp = NAN;
			fz_location loc = fz_resolve_link(handle->ctx, handle->doc, node->uri, &xp, &yp);
			items[current].page_number = fz_page_number_from_location(handle->ctx, handle->doc, loc);
			if (!isnan(xp)) {
				items[current].x = xp;
				items[current].has_x = 1;
			}
			if (!isnan(yp)) {
				items[current].y = yp;
				items[current].has_y = 1;
			}
		} else if (node->page.page >= 0) {
			items[current].page_number = fz_page_number_from_location(handle->ctx, handle->doc, node->page);
		}
		(*index)++;
		if (node->down != NULL) {
			gopdf_fill_outline_items(handle, node->down, depth + 1, current, items, index);
		}
	}
}

int gopdf_load_outline(gopdf_doc *handle, gopdf_outline_result *out, char **err) {
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
		items = NULL;
	} fz_always(handle->ctx) {
		if (outline != NULL) {
			fz_drop_outline(handle->ctx, outline);
		}
	} fz_catch(handle->ctx) {
		if (items != NULL) {
			for (int i = 0; i < count; i++) {
				free(items[i].title);
				free(items[i].uri);
			}
			free(items);
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_outline_result(gopdf_outline_result *result) {
	if (result == NULL) {
		return;
	}
	for (int i = 0; i < result->item_count; i++) {
		free(result->items[i].title);
		free(result->items[i].uri);
	}
	free(result->items);
	result->items = NULL;
	result->item_count = 0;
}

int gopdf_extract_selection(gopdf_doc *handle, int page_number, float ax, float ay, float bx, float by, gopdf_selection *out, char **err) {
	fz_page *page = NULL;
	fz_stext_page *text = NULL;
	fz_point a = { ax, ay };
	fz_point b = { bx, by };
	char *copied = NULL;
	fz_quad *quads = NULL;
	gopdf_quad *heap_quads = NULL;
	int count = 0;
	int cap = 64;
	*err = NULL;
	out->text = NULL;
	out->quads = NULL;
	out->quad_count = 0;
	fz_var(page);
	fz_var(text);
	fz_var(copied);
	fz_var(quads);
	fz_var(heap_quads);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		text = fz_new_stext_page_from_page(handle->ctx, page, NULL);
		copied = fz_copy_selection(handle->ctx, text, a, b, 0);
		quads = fz_malloc_array(handle->ctx, cap, fz_quad);
		for (;;) {
			count = fz_highlight_selection(handle->ctx, text, a, b, quads, cap);
			if (count < cap || cap > INT_MAX / 2) {
				break;
			}
			cap *= 2;
			quads = fz_realloc_array(handle->ctx, quads, cap, fz_quad);
		}
		if (count > cap) {
			count = cap;
		}
		if (count > 0) {
			heap_quads = (gopdf_quad *)malloc(sizeof(gopdf_quad) * count);
			if (heap_quads == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "malloc failed");
			}
			for (int i = 0; i < count; i++) {
				gopdf_copy_quad(&heap_quads[i], &quads[i]);
			}
		}
		out->text = copied;
		out->quads = heap_quads;
		out->quad_count = count;
		copied = NULL;
		heap_quads = NULL;
	} fz_always(handle->ctx) {
		if (text != NULL) {
			fz_drop_stext_page(handle->ctx, text);
		}
		if (quads != NULL) {
			fz_free(handle->ctx, quads);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		if (copied != NULL) {
			fz_free(handle->ctx, copied);
		}
		free(heap_quads);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_selection(gopdf_doc *handle, gopdf_selection *sel) {
	if (sel == NULL) {
		return;
	}
	if (sel->text != NULL) {
		fz_free(handle->ctx, sel->text);
		sel->text = NULL;
	}
	free(sel->quads);
	sel->quads = NULL;
	sel->quad_count = 0;
}

int gopdf_extract_page_text(gopdf_doc *handle, int page_number, char **out, char **err) {
	fz_page *page = NULL;
	fz_stext_page *text = NULL;
	fz_rect bounds = fz_empty_rect;
	fz_point a;
	fz_point b;
	*out = NULL;
	*err = NULL;
	fz_var(page);
	fz_var(text);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		bounds = fz_bound_page(handle->ctx, page);
		text = fz_new_stext_page_from_page(handle->ctx, page, NULL);
		a.x = bounds.x0;
		a.y = bounds.y0;
		b.x = bounds.x1;
		b.y = bounds.y1;
		*out = fz_copy_selection(handle->ctx, text, a, b, 0);
	} fz_always(handle->ctx) {
		if (text != NULL) {
			fz_drop_stext_page(handle->ctx, text);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		if (*out != NULL) {
			fz_free(handle->ctx, *out);
			*out = NULL;
		}
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_text(gopdf_doc *handle, char *text) {
	if (text != NULL) {
		fz_free(handle->ctx, text);
	}
}
