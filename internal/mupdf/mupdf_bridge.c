#include "mupdf_bridge.h"

#include <math.h>
#include <limits.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <mupdf/fitz/util.h>
#include <mupdf/pdf.h>

typedef struct {
	gopdf_search_hit *hits;
	int hit_count;
	int hit_cap;
} gopdf_search_builder;

/* MuPDF requires lock callbacks before contexts can be cloned. The mutexes
 * are shared by every context; gopdf_init_locks runs before first use. */
#ifdef _WIN32
#include <windows.h>
static SRWLOCK gopdf_mutexes[FZ_LOCK_MAX]; /* zeroed == SRWLOCK_INIT */

static void gopdf_init_locks(void) {}

static void gopdf_lock(void *user, int lock) {
	(void)user;
	AcquireSRWLockExclusive(&gopdf_mutexes[lock]);
}

static void gopdf_unlock(void *user, int lock) {
	(void)user;
	ReleaseSRWLockExclusive(&gopdf_mutexes[lock]);
}
#else
#include <pthread.h>
static pthread_mutex_t gopdf_mutexes[FZ_LOCK_MAX];
static pthread_once_t gopdf_locks_once = PTHREAD_ONCE_INIT;

static void gopdf_init_mutexes(void) {
	for (int i = 0; i < FZ_LOCK_MAX; i++) {
		pthread_mutex_init(&gopdf_mutexes[i], NULL);
	}
}

static void gopdf_init_locks(void) {
	pthread_once(&gopdf_locks_once, gopdf_init_mutexes);
}

static void gopdf_lock(void *user, int lock) {
	(void)user;
	pthread_mutex_lock(&gopdf_mutexes[lock]);
}

static void gopdf_unlock(void *user, int lock) {
	(void)user;
	pthread_mutex_unlock(&gopdf_mutexes[lock]);
}
#endif

static fz_locks_context gopdf_locks = { NULL, gopdf_lock, gopdf_unlock };

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

static void gopdf_clear_page_entry(fz_context *ctx, gopdf_page_entry *entry) {
	fz_drop_stext_page(ctx, entry->text);
	entry->text = NULL;
	fz_drop_display_list(ctx, entry->list);
	entry->list = NULL;
	fz_drop_page(ctx, entry->page);
	entry->page = NULL;
	entry->number = -1;
	entry->used = 0;
}

/* Returns the cache entry for a page, loading it into the least recently
 * used slot when needed. Throws when the page cannot be loaded. */
static gopdf_page_entry *gopdf_page_entry_for(gopdf_doc *handle, int page_number) {
	gopdf_page_entry *victim = &handle->pages[0];
	fz_page *page;
	if (page_number < 0 || page_number >= handle->page_count) {
		fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "page number out of range");
	}
	for (int i = 0; i < GOPDF_PAGE_CACHE_SIZE; i++) {
		gopdf_page_entry *entry = &handle->pages[i];
		if (entry->number == page_number) {
			entry->used = ++handle->clock;
			return entry;
		}
		if (entry->used < victim->used) {
			victim = entry;
		}
	}
	page = fz_load_page(handle->ctx, handle->doc, page_number);
	gopdf_clear_page_entry(handle->ctx, victim);
	victim->number = page_number;
	victim->page = page;
	victim->used = ++handle->clock;
	return victim;
}

gopdf_doc *gopdf_open_document(const char *path, const char *password, size_t store_size, char **err) {
	gopdf_doc *handle = NULL;
	fz_context *ctx = NULL;
	fz_document *doc = NULL;
	const fz_document_handler *handler = NULL;
	int needs_password = 0;
	int authenticated = 1;
	*err = NULL;
	gopdf_init_locks();
	ctx = fz_new_context(NULL, &gopdf_locks, store_size);
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
	fz_try(ctx) {
		pdf_document *pdf = pdf_specifics(ctx, doc);
		if (pdf != NULL) {
			pdf_enable_journal(ctx, pdf);
		}
	} fz_catch(ctx) {
		/* Editing still works without undo. */
	}
	memset(handle, 0, sizeof(*handle));
	handle->ctx = ctx;
	handle->doc = doc;
	for (int i = 0; i < GOPDF_PAGE_CACHE_SIZE; i++) {
		handle->pages[i].number = -1;
	}
	return handle;
}

void gopdf_close_document(gopdf_doc *handle) {
	if (handle == NULL) {
		return;
	}
	for (int i = 0; i < GOPDF_PAGE_CACHE_SIZE; i++) {
		gopdf_clear_page_entry(handle->ctx, &handle->pages[i]);
	}
	fz_drop_document(handle->ctx, handle->doc);
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

/* Page metrics are read for every page up front, so the page is loaded
 * transiently instead of filling the page cache. */
/* A page's bounds and label. PDF pages are read from the page tree, which
 * is much cheaper than loading the page with its annotations. */
int gopdf_page_info(gopdf_doc *handle, int page_number, gopdf_rect *bounds, char **label, char **err) {
	pdf_document *pdf = pdf_specifics(handle->ctx, handle->doc);
	fz_page *page = NULL;
	fz_rect rect = fz_empty_rect;
	fz_matrix ctm;
	char buf[64] = { 0 };
	*label = NULL;
	*err = NULL;
	fz_var(page);
	fz_try(handle->ctx) {
		if (pdf) {
			pdf_page_obj_transform(handle->ctx, pdf_lookup_page_obj(handle->ctx, pdf, page_number), &rect, &ctm);
			rect = fz_transform_rect(rect, ctm);
			pdf_page_label(handle->ctx, pdf, page_number, buf, sizeof(buf));
		} else {
			page = fz_load_page(handle->ctx, handle->doc, page_number);
			rect = fz_bound_page(handle->ctx, page);
			fz_page_label(handle->ctx, page, buf, sizeof(buf));
		}
	} fz_always(handle->ctx) {
		fz_drop_page(handle->ctx, page);
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	bounds->x0 = rect.x0;
	bounds->y0 = rect.y0;
	bounds->x1 = rect.x1;
	bounds->y1 = rect.y1;
	if (buf[0] != '\0') {
		*label = gopdf_dup_string(buf);
		if (*label == NULL) {
			*err = gopdf_dup_string("malloc failed");
			return 0;
		}
	}
	return 1;
}

/* Bounds of everything a page draws, from a transient load like
 * gopdf_page_info. */
int gopdf_page_content_bounds(gopdf_doc *handle, int page_number, gopdf_rect *out, char **err) {
	fz_page *page = NULL;
	fz_device *dev = NULL;
	fz_rect rect = fz_empty_rect;
	*err = NULL;
	fz_var(page);
	fz_var(dev);
	fz_try(handle->ctx) {
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		dev = fz_new_bbox_device(handle->ctx, &rect);
		fz_run_page(handle->ctx, page, dev, fz_identity, NULL);
		fz_close_device(handle->ctx, dev);
	} fz_always(handle->ctx) {
		fz_drop_device(handle->ctx, dev);
		fz_drop_page(handle->ctx, page);
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	if (fz_is_empty_rect(rect)) {
		rect = fz_make_rect(0, 0, 0, 0);
	}
	out->x0 = rect.x0;
	out->y0 = rect.y0;
	out->x1 = rect.x1;
	out->y1 = rect.y1;
	return 1;
}

int gopdf_lookup_metadata(gopdf_doc *handle, const char *key, char **out, char **err) {
	int size = 0;
	*out = NULL;
	*err = NULL;
	fz_try(handle->ctx) {
		size = fz_lookup_metadata(handle->ctx, handle->doc, key, NULL, 0);
		if (size > 0) {
			*out = (char *)malloc((size_t)size);
			if (*out == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "malloc failed");
			}
			fz_lookup_metadata(handle->ctx, handle->doc, key, *out, (size_t)size);
		}
	} fz_catch(handle->ctx) {
		free(*out);
		*out = NULL;
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_string(char *value) {
	free(value);
}

static fz_display_list *gopdf_entry_display_list(fz_context *ctx, gopdf_page_entry *entry) {
	if (entry->list == NULL) {
		entry->list = fz_new_display_list_from_page(ctx, entry->page);
	}
	return entry->list;
}

/* The cached text keeps image blocks so their bounds are known; text
 * lookups skip them by block type. */
static fz_stext_page *gopdf_entry_text(fz_context *ctx, gopdf_page_entry *entry) {
	if (entry->text == NULL) {
		fz_stext_options opts;
		memset(&opts, 0, sizeof(opts));
		opts.flags = FZ_STEXT_PRESERVE_IMAGES;
		entry->text = fz_new_stext_page_from_display_list(ctx, gopdf_entry_display_list(ctx, entry), &opts);
	}
	return entry->text;
}

int gopdf_page_image_bounds(gopdf_doc *handle, int page_number, gopdf_rect **out, int *count, char **err) {
	*out = NULL;
	*count = 0;
	*err = NULL;
	fz_try(handle->ctx) {
		fz_stext_page *text = gopdf_entry_text(handle->ctx, gopdf_page_entry_for(handle, page_number));
		int n = 0;
		for (fz_stext_block *block = text->first_block; block != NULL; block = block->next) {
			n += block->type == FZ_STEXT_BLOCK_IMAGE;
		}
		if (n > 0) {
			*out = (gopdf_rect *)calloc((size_t)n, sizeof(gopdf_rect));
			if (*out == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "calloc failed");
			}
			for (fz_stext_block *block = text->first_block; block != NULL; block = block->next) {
				if (block->type == FZ_STEXT_BLOCK_IMAGE) {
					gopdf_rect *r = &(*out)[(*count)++];
					r->x0 = block->bbox.x0;
					r->y0 = block->bbox.y0;
					r->x1 = block->bbox.x1;
					r->y1 = block->bbox.y1;
				}
			}
		}
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Drops what was derived from a page so an edit shows on the next use. */
static void gopdf_invalidate_page(gopdf_doc *handle, int page_number) {
	for (int i = 0; i < GOPDF_PAGE_CACHE_SIZE; i++) {
		gopdf_page_entry *entry = &handle->pages[i];
		if (entry->number == page_number) {
			fz_drop_stext_page(handle->ctx, entry->text);
			entry->text = NULL;
			fz_drop_display_list(handle->ctx, entry->list);
			entry->list = NULL;
		}
	}
}

/* Each edit runs as one journal operation, so it can be undone. */
static pdf_document *gopdf_begin_edit(gopdf_doc *handle, const char *name) {
	pdf_document *pdf = pdf_specifics(handle->ctx, handle->doc);
	if (pdf == NULL) {
		fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "only PDF documents can be edited");
	}
	pdf_begin_operation(handle->ctx, pdf, name);
	return pdf;
}

static void gopdf_abandon_edit(gopdf_doc *handle, pdf_document *pdf) {
	if (pdf != NULL) {
		fz_try(handle->ctx) {
			pdf_abandon_operation(handle->ctx, pdf);
		} fz_catch(handle->ctx) {
		}
	}
}

/* Adds a highlight annotation covering quads, in page coordinates. */
int gopdf_add_highlight(gopdf_doc *handle, int page_number, const gopdf_quad *quads, int count, const float *rgb, char **err) {
	pdf_document *pdf = NULL;
	pdf_annot *annot = NULL;
	fz_quad *fq = NULL;
	*err = NULL;
	fz_var(pdf);
	fz_var(annot);
	fz_var(fq);
	fz_try(handle->ctx) {
		pdf = gopdf_begin_edit(handle, "Highlight");
		pdf_page *page = pdf_page_from_fz_page(handle->ctx, gopdf_page_entry_for(handle, page_number)->page);
		if (page == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "only PDF pages can be annotated");
		}
		fq = fz_malloc_array(handle->ctx, count, fz_quad);
		for (int i = 0; i < count; i++) {
			fq[i] = fz_make_quad(quads[i].ul.x, quads[i].ul.y, quads[i].ur.x, quads[i].ur.y, quads[i].ll.x, quads[i].ll.y, quads[i].lr.x, quads[i].lr.y);
		}
		annot = pdf_create_annot(handle->ctx, page, PDF_ANNOT_HIGHLIGHT);
		pdf_set_annot_color(handle->ctx, annot, 3, rgb);
		pdf_set_annot_quad_points(handle->ctx, annot, count, fq);
		pdf_update_annot(handle->ctx, annot);
		pdf_end_operation(handle->ctx, pdf);
		pdf = NULL;
		gopdf_invalidate_page(handle, page_number);
	} fz_always(handle->ctx) {
		pdf_drop_annot(handle->ctx, annot);
		fz_free(handle->ctx, fq);
	} fz_catch(handle->ctx) {
		gopdf_abandon_edit(handle, pdf);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

static int gopdf_annot_contains(fz_context *ctx, pdf_annot *annot, fz_point p) {
	if (pdf_annot_has_quad_points(ctx, annot)) {
		int n = pdf_annot_quad_point_count(ctx, annot);
		for (int i = 0; i < n; i++) {
			if (fz_is_point_inside_rect(p, fz_rect_from_quad(pdf_annot_quad_point(ctx, annot, i)))) {
				return 1;
			}
		}
		return 0;
	}
	return fz_is_point_inside_rect(p, pdf_bound_annot(ctx, annot));
}

/* Finds the topmost annotation under a point, reporting its index among the
 * page's annotations (-1 if none) and a malloc'd name of its type. */
int gopdf_annot_at(gopdf_doc *handle, int page_number, float x, float y, int *index, char **type, char **err) {
	*index = -1;
	*type = NULL;
	*err = NULL;
	fz_try(handle->ctx) {
		pdf_page *page = pdf_page_from_fz_page(handle->ctx, gopdf_page_entry_for(handle, page_number)->page);
		fz_point p = fz_make_point(x, y);
		int i = 0;
		for (pdf_annot *a = page ? pdf_first_annot(handle->ctx, page) : NULL; a != NULL; a = pdf_next_annot(handle->ctx, a), i++) {
			enum pdf_annot_type t = pdf_annot_type(handle->ctx, a);
			if (t != PDF_ANNOT_POPUP && t != PDF_ANNOT_LINK && gopdf_annot_contains(handle->ctx, a, p)) {
				*index = i; /* later annotations draw on top */
				free(*type);
				*type = gopdf_dup_string_or_throw(handle->ctx, pdf_string_from_annot_type(handle->ctx, t));
			}
		}
	} fz_catch(handle->ctx) {
		free(*type);
		*type = NULL;
		*index = -1;
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Deletes the index-th annotation of a page, or recolours it when rgb is
 * given. */
int gopdf_edit_annot(gopdf_doc *handle, int page_number, int index, const float *rgb, char **err) {
	pdf_document *pdf = NULL;
	*err = NULL;
	fz_var(pdf);
	fz_try(handle->ctx) {
		pdf = gopdf_begin_edit(handle, rgb ? "Recolour annotation" : "Delete annotation");
		pdf_page *page = pdf_page_from_fz_page(handle->ctx, gopdf_page_entry_for(handle, page_number)->page);
		pdf_annot *annot = page ? pdf_first_annot(handle->ctx, page) : NULL;
		for (int i = 0; annot != NULL && i < index; i++) {
			annot = pdf_next_annot(handle->ctx, annot);
		}
		if (annot == NULL) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "no such annotation");
		}
		if (rgb) {
			pdf_set_annot_color(handle->ctx, annot, 3, rgb);
			pdf_update_annot(handle->ctx, annot);
		} else {
			pdf_delete_annot(handle->ctx, page, annot);
		}
		pdf_end_operation(handle->ctx, pdf);
		pdf = NULL;
		gopdf_invalidate_page(handle, page_number);
	} fz_catch(handle->ctx) {
		gopdf_abandon_edit(handle, pdf);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Returns the index-th form widget of a PDF page, or NULL. The widget is
 * borrowed from the page. */
static pdf_annot *gopdf_widget(gopdf_doc *handle, int page_number, int index, pdf_page **page_out) {
	pdf_page *page = pdf_page_from_fz_page(handle->ctx, gopdf_page_entry_for(handle, page_number)->page);
	pdf_annot *widget = page ? pdf_first_widget(handle->ctx, page) : NULL;
	for (int i = 0; widget != NULL && i < index; i++) {
		widget = pdf_next_widget(handle->ctx, widget);
	}
	if (widget == NULL) {
		fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "no such form field");
	}
	*page_out = page;
	return widget;
}

/* Finds the form widget under a point, reporting its index (-1 if none),
 * type, read-only flag, bounds and value. */
int gopdf_widget_at(gopdf_doc *handle, int page_number, float x, float y, gopdf_widget_info *out, char **err) {
	*err = NULL;
	memset(out, 0, sizeof(*out));
	out->index = -1;
	fz_try(handle->ctx) {
		pdf_page *page = pdf_page_from_fz_page(handle->ctx, gopdf_page_entry_for(handle, page_number)->page);
		int i = 0;
		for (pdf_annot *w = page ? pdf_first_widget(handle->ctx, page) : NULL; w != NULL; w = pdf_next_widget(handle->ctx, w), i++) {
			fz_rect r = pdf_bound_widget(handle->ctx, w);
			if (x >= r.x0 && x <= r.x1 && y >= r.y0 && y <= r.y1) {
				out->index = i;
				out->type = pdf_widget_type(handle->ctx, w);
				out->readonly = pdf_widget_is_readonly(handle->ctx, w);
				out->rect.x0 = r.x0;
				out->rect.y0 = r.y0;
				out->rect.x1 = r.x1;
				out->rect.y1 = r.y1;
				out->value = gopdf_dup_string_or_throw(handle->ctx, pdf_annot_field_value(handle->ctx, w));
				break;
			}
		}
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Returns a choice widget's options as a malloc'd array of malloc'd strings. */
int gopdf_widget_options(gopdf_doc *handle, int page_number, int index, char ***out, int *count, char **err) {
	const char **opts = NULL;
	*out = NULL;
	*count = 0;
	*err = NULL;
	fz_var(opts);
	fz_try(handle->ctx) {
		pdf_page *page;
		pdf_annot *widget = gopdf_widget(handle, page_number, index, &page);
		int n = pdf_choice_widget_options(handle->ctx, widget, 0, NULL);
		if (n > 0) {
			opts = fz_malloc_array(handle->ctx, n, const char *);
			pdf_choice_widget_options(handle->ctx, widget, 0, opts);
			*out = (char **)calloc((size_t)n, sizeof(char *));
			if (*out == NULL) {
				fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "calloc failed");
			}
			for (int i = 0; i < n; i++) {
				(*out)[i] = gopdf_dup_string_or_throw(handle->ctx, opts[i]);
				*count = i + 1;
			}
		}
	} fz_always(handle->ctx) {
		fz_free(handle->ctx, opts);
	} fz_catch(handle->ctx) {
		gopdf_free_strings(*out, *count);
		*out = NULL;
		*count = 0;
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_strings(char **values, int count) {
	for (int i = 0; values != NULL && i < count; i++) {
		free(values[i]);
	}
	free(values);
}

/* Edits a form widget: sets a text or choice value, or toggles a check box
 * or radio button when value is NULL, then refreshes the page. */
int gopdf_widget_edit(gopdf_doc *handle, int page_number, int index, const char *value, char **err) {
	pdf_document *pdf = NULL;
	*err = NULL;
	fz_var(pdf);
	fz_try(handle->ctx) {
		pdf_page *page;
		pdf = gopdf_begin_edit(handle, "Fill form field");
		pdf_annot *widget = gopdf_widget(handle, page_number, index, &page);
		switch (pdf_widget_type(handle->ctx, widget)) {
		case PDF_WIDGET_TYPE_TEXT:
			if (!pdf_set_text_field_value(handle->ctx, widget, value ? value : "")) {
				fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "value rejected by the form");
			}
			break;
		case PDF_WIDGET_TYPE_COMBOBOX:
		case PDF_WIDGET_TYPE_LISTBOX: {
			const char *opts[1] = { value ? value : "" };
			pdf_choice_widget_set_value(handle->ctx, widget, 1, opts);
			break;
		}
		case PDF_WIDGET_TYPE_CHECKBOX:
		case PDF_WIDGET_TYPE_RADIOBUTTON:
			pdf_toggle_widget(handle->ctx, widget);
			break;
		default:
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, "this form field cannot be edited");
		}
		pdf_update_page(handle->ctx, page);
		pdf_end_operation(handle->ctx, pdf);
		pdf = NULL;
		gopdf_invalidate_page(handle, page_number);
	} fz_catch(handle->ctx) {
		gopdf_abandon_edit(handle, pdf);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Undoes (or redoes) the last edit. Which pages it touched is unknown, so
 * every cached page is reloaded. */
int gopdf_undo(gopdf_doc *handle, int redo, char **err) {
	pdf_document *pdf = pdf_specifics(handle->ctx, handle->doc);
	*err = NULL;
	fz_try(handle->ctx) {
		if (pdf == NULL || !(redo ? pdf_can_redo(handle->ctx, pdf) : pdf_can_undo(handle->ctx, pdf))) {
			fz_throw(handle->ctx, FZ_ERROR_ARGUMENT, redo ? "nothing to redo" : "nothing to undo");
		}
		if (redo) {
			pdf_redo(handle->ctx, pdf);
		} else {
			pdf_undo(handle->ctx, pdf);
		}
		for (int i = 0; i < GOPDF_PAGE_CACHE_SIZE; i++) {
			gopdf_clear_page_entry(handle->ctx, &handle->pages[i]);
		}
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

int gopdf_is_pdf(gopdf_doc *handle) {
	return pdf_specifics(handle->ctx, handle->doc) != NULL;
}

/* Saves a PDF to path, incrementally when asked and possible. Writing a
 * document over the file it reads from is only safe incrementally, which
 * the caller arranges. */
int gopdf_save(gopdf_doc *handle, const char *path, int incremental, char **err) {
	pdf_document *pdf = pdf_specifics(handle->ctx, handle->doc);
	*err = NULL;
	if (pdf == NULL) {
		*err = gopdf_dup_string("only PDF documents can be saved");
		return 0;
	}
	fz_try(handle->ctx) {
		pdf_write_options opts = pdf_default_write_options;
		opts.do_incremental = incremental;
		pdf_save_document(handle->ctx, pdf, path, &opts);
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

int gopdf_can_save_incrementally(gopdf_doc *handle) {
	pdf_document *pdf = pdf_specifics(handle->ctx, handle->doc);
	int ok = 0;
	if (pdf == NULL) {
		return 0;
	}
	fz_try(handle->ctx) {
		ok = pdf_can_be_saved_incrementally(handle->ctx, pdf);
	} fz_catch(handle->ctx) {
		ok = 0;
	}
	return ok;
}

/* Returns a new reference to the page's cached display list, which the
 * caller passes to gopdf_render_display_list. */
int gopdf_page_display_list(gopdf_doc *handle, int page_number, fz_display_list **out, char **err) {
	*out = NULL;
	*err = NULL;
	fz_try(handle->ctx) {
		gopdf_page_entry *entry = gopdf_page_entry_for(handle, page_number);
		*out = fz_keep_display_list(handle->ctx, gopdf_entry_display_list(handle->ctx, entry));
	} fz_catch(handle->ctx) {
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

/* Must be called with the document lock held, since cloning reads the
 * document context. */
gopdf_renderer *gopdf_new_renderer(gopdf_doc *handle, char **err) {
	gopdf_renderer *renderer = (gopdf_renderer *)calloc(1, sizeof(gopdf_renderer));
	*err = NULL;
	if (renderer == NULL) {
		*err = gopdf_dup_string("calloc failed");
		return NULL;
	}
	renderer->ctx = fz_clone_context(handle->ctx);
	if (renderer->ctx == NULL) {
		free(renderer);
		*err = gopdf_dup_string("fz_clone_context failed");
		return NULL;
	}
	return renderer;
}

void gopdf_drop_renderer(gopdf_renderer *renderer) {
	if (renderer == NULL) {
		return;
	}
	fz_drop_context(renderer->ctx);
	free(renderer);
}

void gopdf_arm_renderer(gopdf_renderer *renderer) {
	if (renderer != NULL) {
		memset(&renderer->cookie, 0, sizeof(renderer->cookie));
	}
}

void gopdf_cancel_renderer(gopdf_renderer *renderer) {
	if (renderer != NULL) {
		renderer->cookie.abort = 1;
	}
}

/* Rasterises the part of a display list inside clip, in device pixels at
 * scale, into a malloc'd RGBA buffer. Consumes the caller's reference to
 * list. Returns 2, with no buffer, when the render was cancelled since the
 * renderer was last armed. */
int gopdf_render_display_list(gopdf_renderer *renderer, fz_display_list *list, float scale, gopdf_irect clip, int aa_level, unsigned char **samples, int *width, int *height, int *stride, int *x, int *y, char **err) {
	fz_context *ctx = renderer->ctx;
	fz_pixmap *pix = NULL;
	fz_device *dev = NULL;
	fz_matrix ctm = fz_scale(scale, scale);
	int cancelled = 0;
	fz_irect clip_rect = fz_make_irect(clip.x0, clip.y0, clip.x1, clip.y1);
	fz_irect bbox = fz_empty_irect;
	*err = NULL;
	*samples = NULL;
	*width = 0;
	*height = 0;
	*stride = 0;
	*x = 0;
	*y = 0;
	if (renderer->cookie.abort) {
		fz_drop_display_list(ctx, list);
		return 2;
	}
	fz_var(pix);
	fz_var(dev);
	fz_try(ctx) {
		fz_set_aa_level(ctx, aa_level);
		bbox = fz_round_rect(fz_transform_rect(fz_bound_display_list(ctx, list), ctm));
		bbox = fz_intersect_irect(bbox, clip_rect);
		if (fz_is_empty_irect(bbox)) {
			bbox = fz_make_irect(clip.x0, clip.y0, clip.x0, clip.y0);
		}
		*width = bbox.x1 - bbox.x0;
		*height = bbox.y1 - bbox.y0;
		if (*width < 0 || *height < 0 || *width > INT_MAX / 4) {
			fz_throw(ctx, FZ_ERROR_LIMIT, "rendered page dimensions are invalid");
		}
		*stride = *width * 4;
		*x = bbox.x0;
		*y = bbox.y0;
		if (*width > 0 && *height > 0) {
			if (*height > INT_MAX / *stride) {
				fz_throw(ctx, FZ_ERROR_LIMIT, "rendered page buffer is too large");
			}
			*samples = (unsigned char *)malloc((size_t)*stride * (size_t)*height);
			if (*samples == NULL) {
				fz_throw(ctx, FZ_ERROR_SYSTEM, "malloc failed");
			}
			pix = fz_new_pixmap_with_bbox_and_data(ctx, fz_device_rgb(ctx), bbox, NULL, 1, *samples);
			fz_clear_pixmap_with_value(ctx, pix, 0xff);
			dev = fz_new_draw_device(ctx, fz_identity, pix);
			fz_run_display_list(ctx, list, dev, ctm, fz_rect_from_irect(bbox), &renderer->cookie);
			fz_close_device(ctx, dev);
			cancelled = renderer->cookie.abort;
		}
	} fz_always(ctx) {
		fz_drop_device(ctx, dev);
		fz_drop_pixmap(ctx, pix);
		fz_drop_display_list(ctx, list);
	} fz_catch(ctx) {
		free(*samples);
		*samples = NULL;
		*err = gopdf_dup_string(fz_caught_message(ctx));
		return 0;
	}
	if (cancelled) {
		free(*samples);
		*samples = NULL;
		return 2;
	}
	return 1;
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
	fz_var(links);
	fz_var(items);
	fz_try(handle->ctx) {
		page = gopdf_page_entry_for(handle, page_number)->page;
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

typedef struct {
	fz_point p1;
	fz_point hdir;
	float length;
	float tangent;
} gopdf_selection_line;

static float gopdf_selection_dot(fz_point a, fz_point b) {
	return a.x * b.x + a.y * b.y;
}

static fz_point gopdf_selection_sub(fz_point a, fz_point b) {
	fz_point out = { a.x - b.x, a.y - b.y };
	return out;
}

static int gopdf_selection_find_line(fz_stext_page *text, fz_point point, gopdf_selection_line *out) {
	fz_stext_block *block;
	fz_stext_line *line;
	int found = 0;
	float best_horizontal = INFINITY;
	float best_perpendicular = INFINITY;

	for (block = text->first_block; block != NULL; block = block->next) {
		if (block->type != FZ_STEXT_BLOCK_TEXT) {
			continue;
		}
		for (line = block->u.t.first_line; line != NULL; line = line->next) {
			fz_point p1;
			fz_point p2;
			fz_point delta;
			fz_point hdir;
			fz_point vdir;
			float length;
			float tangent;
			float horizontal_distance;
			float perpendicular_distance;

			if (line->first_char == NULL || line->last_char == NULL) {
				continue;
			}

			hdir = line->dir;
			vdir.x = -hdir.y;
			vdir.y = hdir.x;
			p1.x = (line->first_char->quad.ll.x + line->first_char->quad.ul.x) / 2;
			p1.y = (line->first_char->quad.ll.y + line->first_char->quad.ul.y) / 2;
			p2.x = (line->last_char->quad.lr.x + line->last_char->quad.ur.x) / 2;
			p2.y = (line->last_char->quad.lr.y + line->last_char->quad.ur.y) / 2;
			delta = gopdf_selection_sub(p2, p1);
			length = gopdf_selection_dot(delta, hdir);
			if (length <= 0.01f) {
				continue;
			}

			delta = gopdf_selection_sub(point, p1);
			tangent = gopdf_selection_dot(delta, hdir);
			perpendicular_distance = fabsf(gopdf_selection_dot(delta, vdir));
			if (tangent < 0) {
				horizontal_distance = -tangent;
			} else if (tangent > length) {
				horizontal_distance = tangent - length;
			} else {
				horizontal_distance = 0;
			}

			if (!found || horizontal_distance < best_horizontal ||
				(horizontal_distance == best_horizontal && perpendicular_distance < best_perpendicular)) {
				found = 1;
				best_horizontal = horizontal_distance;
				best_perpendicular = perpendicular_distance;
				out->p1 = p1;
				out->hdir = hdir;
				out->length = length;
				out->tangent = tangent;
			}
		}
	}

	return found;
}

static fz_point gopdf_selection_normalize_point(fz_stext_page *text, fz_point point) {
	gopdf_selection_line candidate;
	float tangent;
	const float edge_epsilon = 0.01f;

	if (!gopdf_selection_find_line(text, point, &candidate)) {
		return point;
	}

	tangent = candidate.tangent;
	if (tangent < edge_epsilon) {
		tangent = edge_epsilon;
	} else if (tangent > candidate.length - edge_epsilon) {
		tangent = candidate.length - edge_epsilon;
	}

	point.x = candidate.p1.x + candidate.hdir.x * tangent;
	point.y = candidate.p1.y + candidate.hdir.y * tangent;
	return point;
}

/* mode is an fz_select_mode: characters, or whole words or lines. */
int gopdf_extract_selection(gopdf_doc *handle, int page_number, float ax, float ay, float bx, float by, int mode, gopdf_selection *out, char **err) {
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
	fz_var(copied);
	fz_var(quads);
	fz_var(heap_quads);
	fz_try(handle->ctx) {
		/* Cached, since selection is re-extracted on every pointer move. */
		text = gopdf_entry_text(handle->ctx, gopdf_page_entry_for(handle, page_number));
		a = gopdf_selection_normalize_point(text, a);
		b = gopdf_selection_normalize_point(text, b);
		if (mode != FZ_SELECT_CHARS) {
			fz_snap_selection(handle->ctx, text, &a, &b, mode);
		}
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
		if (quads != NULL) {
			fz_free(handle->ctx, quads);
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

/* Lines that are only digits, such as page numbers. */
static int gopdf_line_is_number(fz_stext_line *line) {
	for (fz_stext_char *ch = line->first_char; ch != NULL; ch = ch->next) {
		if (ch->c != ' ' && (ch->c < '0' || ch->c > '9')) {
			return 0;
		}
	}
	return 1;
}

/* Finds where a page's text starts and ends in reading order: the left edge
 * of its first character and the right edge of its last, skipping rotated
 * lines such as margin stamps and page numbers. found is 0 for a page
 * without such text. */
int gopdf_page_text_ends(gopdf_doc *handle, int page_number, gopdf_point *start, gopdf_point *end, int *found, char **err) {
	*found = 0;
	*err = NULL;
	fz_try(handle->ctx) {
		fz_stext_page *text = gopdf_entry_text(handle->ctx, gopdf_page_entry_for(handle, page_number));
		for (fz_stext_block *block = text->first_block; block != NULL; block = block->next) {
			if (block->type != FZ_STEXT_BLOCK_TEXT) {
				continue;
			}
			for (fz_stext_line *line = block->u.t.first_line; line != NULL; line = line->next) {
				if (line->first_char == NULL || fabsf(line->dir.y) > 0.1f || gopdf_line_is_number(line)) {
					continue;
				}
				fz_quad first = line->first_char->quad, last = line->last_char->quad;
				if (!*found) {
					start->x = (first.ul.x + first.ll.x) / 2;
					start->y = (first.ul.y + first.ll.y) / 2;
					*found = 1;
				}
				end->x = (last.ur.x + last.lr.x) / 2;
				end->y = (last.ur.y + last.lr.y) / 2;
			}
		}
	} fz_catch(handle->ctx) {
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

int gopdf_extract_page_chars(gopdf_doc *handle, int page_number, gopdf_char_result *out, char **err) {
	fz_page *page = NULL;
	fz_stext_page *text = NULL;
	gopdf_char *chars = NULL;
	int count = 0;
	int cap = 0;
	*err = NULL;
	out->chars = NULL;
	out->char_count = 0;
	fz_var(page);
	fz_var(text);
	fz_var(chars);
	fz_var(count);
	fz_var(cap);
	fz_try(handle->ctx) {
		int block_index = 0;
		int line_index = 0;
		page = fz_load_page(handle->ctx, handle->doc, page_number);
		text = fz_new_stext_page_from_page(handle->ctx, page, NULL);
		for (fz_stext_block *block = text->first_block; block != NULL; block = block->next) {
			if (block->type != FZ_STEXT_BLOCK_TEXT) {
				continue;
			}
			for (fz_stext_line *line = block->u.t.first_line; line != NULL; line = line->next) {
				for (fz_stext_char *ch = line->first_char; ch != NULL; ch = ch->next) {
					if (count == cap) {
						int next_cap = cap == 0 ? 256 : cap * 2;
						gopdf_char *next = (gopdf_char *)realloc(chars, sizeof(gopdf_char) * next_cap);
						if (next == NULL) {
							fz_throw(handle->ctx, FZ_ERROR_SYSTEM, "realloc failed");
						}
						chars = next;
						cap = next_cap;
					}
					chars[count].c = ch->c;
					chars[count].line = line_index;
					chars[count].block = block_index;
					gopdf_copy_quad(&chars[count].quad, &ch->quad);
					count++;
				}
				line_index++;
			}
			block_index++;
		}
		out->chars = chars;
		out->char_count = count;
		chars = NULL;
	} fz_always(handle->ctx) {
		if (text != NULL) {
			fz_drop_stext_page(handle->ctx, text);
		}
		if (page != NULL) {
			fz_drop_page(handle->ctx, page);
		}
	} fz_catch(handle->ctx) {
		free(chars);
		*err = gopdf_dup_string(fz_caught_message(handle->ctx));
		return 0;
	}
	return 1;
}

void gopdf_free_char_result(gopdf_char_result *result) {
	if (result == NULL) {
		return;
	}
	free(result->chars);
	result->chars = NULL;
	result->char_count = 0;
}

/* A single probe context is enough: callers serialise access, and this is only
 * used to ask MuPDF which names its compiled-in handlers recognise. */
static fz_context *gopdf_probe_ctx = NULL;

int gopdf_recognize_document_name(const char *name) {
	const fz_document_handler *handler = NULL;
	if (gopdf_probe_ctx == NULL) {
		gopdf_probe_ctx = fz_new_context(NULL, NULL, FZ_STORE_DEFAULT);
		if (gopdf_probe_ctx == NULL) {
			return 0;
		}
		fz_set_warning_callback(gopdf_probe_ctx, gopdf_silent_callback, NULL);
		fz_set_error_callback(gopdf_probe_ctx, gopdf_silent_callback, NULL);
		fz_try(gopdf_probe_ctx) {
			fz_register_document_handlers(gopdf_probe_ctx);
		} fz_catch(gopdf_probe_ctx) {
			fz_drop_context(gopdf_probe_ctx);
			gopdf_probe_ctx = NULL;
			return 0;
		}
	}
	fz_try(gopdf_probe_ctx) {
		handler = fz_recognize_document(gopdf_probe_ctx, name);
	} fz_catch(gopdf_probe_ctx) {
		return 0;
	}
	return handler != NULL ? 1 : 0;
}
