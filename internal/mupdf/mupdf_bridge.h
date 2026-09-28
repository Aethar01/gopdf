#ifndef GOPDF_MUPDF_BRIDGE_H
#define GOPDF_MUPDF_BRIDGE_H

#include <mupdf/fitz.h>

enum { GOPDF_PAGE_CACHE_SIZE = 32 };

/* A recently used page and the objects derived from it. */
typedef struct {
	int number; /* -1 when the slot is empty */
	unsigned long used;
	fz_page *page;
	fz_display_list *list;
	fz_stext_page *text;
} gopdf_page_entry;

typedef struct {
	fz_context *ctx;
	fz_document *doc;
	int page_count;
	unsigned long clock;
	gopdf_page_entry pages[GOPDF_PAGE_CACHE_SIZE];
} gopdf_doc;

/* A private context cloned from a document's, used to rasterise display
 * lists concurrently with other renderers and document access. */
typedef struct {
	fz_context *ctx;
	fz_cookie cookie;
} gopdf_renderer;

typedef struct {
	float x0;
	float y0;
	float x1;
	float y1;
} gopdf_rect;

typedef struct {
	int x0;
	int y0;
	int x1;
	int y1;
} gopdf_irect;

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
	int c;
	int line;
	int block;
	gopdf_quad quad;
} gopdf_char;

typedef struct {
	gopdf_char *chars;
	int char_count;
} gopdf_char_result;

typedef struct {
	gopdf_rect rect;
	char *uri;
	int is_external;
	int page_number;
	float x;
	float y;
	int has_x;
	int has_y;
} gopdf_link;

typedef struct {
	gopdf_link *links;
	int link_count;
} gopdf_link_result;

typedef struct {
	char *title;
	char *uri;
	int is_external;
	int page_number;
	float x;
	float y;
	int has_x;
	int has_y;
	int depth;
	int parent;
	int has_children;
} gopdf_outline_item;

typedef struct {
	gopdf_outline_item *items;
	int item_count;
} gopdf_outline_result;

gopdf_doc *gopdf_open_document(const char *path, const char *password, size_t store_size, char **err);
void gopdf_close_document(gopdf_doc *handle);
int gopdf_count_pages(gopdf_doc *handle, int *count, char **err);
int gopdf_page_content_bounds(gopdf_doc *handle, int page_number, gopdf_rect *out, char **err);
int gopdf_page_info(gopdf_doc *handle, int page_number, gopdf_rect *bounds, char **label, char **err);
int gopdf_lookup_metadata(gopdf_doc *handle, const char *key, char **out, char **err);
int gopdf_page_image_bounds(gopdf_doc *handle, int page_number, gopdf_rect **out, int *count, char **err);
int gopdf_page_display_list(gopdf_doc *handle, int page_number, fz_display_list **out, char **err);
gopdf_renderer *gopdf_new_renderer(gopdf_doc *handle, char **err);
void gopdf_drop_renderer(gopdf_renderer *renderer);
void gopdf_cancel_renderer(gopdf_renderer *renderer);
int gopdf_render_display_list(gopdf_renderer *renderer, fz_display_list *list, float scale, gopdf_irect clip, int aa_level, unsigned char **samples, int *width, int *height, int *stride, int *x, int *y, char **err);
void gopdf_free_rendered_page(unsigned char *samples);
int gopdf_extract_selection(gopdf_doc *handle, int page_number, float ax, float ay, float bx, float by, int mode, gopdf_selection *out, char **err);
void gopdf_free_selection(gopdf_doc *handle, gopdf_selection *sel);
int gopdf_search_page(gopdf_doc *handle, int page_number, const char *needle, gopdf_search_result *out, char **err);
void gopdf_free_search_result(gopdf_search_result *result);
int gopdf_extract_page_text(gopdf_doc *handle, int page_number, char **out, char **err);
void gopdf_free_text(gopdf_doc *handle, char *text);
int gopdf_extract_page_chars(gopdf_doc *handle, int page_number, gopdf_char_result *out, char **err);
void gopdf_free_char_result(gopdf_char_result *result);
int gopdf_load_links(gopdf_doc *handle, int page_number, gopdf_link_result *out, char **err);
void gopdf_free_link_result(gopdf_link_result *result);
int gopdf_load_outline(gopdf_doc *handle, gopdf_outline_result *out, char **err);
void gopdf_free_outline_result(gopdf_outline_result *result);
int gopdf_recognize_document_name(const char *name);
void gopdf_free_string(char *value);

#endif
