#ifndef ONON_MUPDF_BRIDGE_H
#define ONON_MUPDF_BRIDGE_H

#include <mupdf/fitz.h>

enum { ONON_PAGE_CACHE_SIZE = 32 };

/* A recently used page and the objects derived from it. */
typedef struct {
	int number; /* -1 when the slot is empty */
	unsigned long used;
	fz_page *page;
	fz_display_list *list;
	fz_stext_page *text;
} onon_page_entry;

typedef struct {
	fz_context *ctx;
	fz_document *doc;
	int page_count;
	unsigned long clock;
	onon_page_entry pages[ONON_PAGE_CACHE_SIZE];
} onon_doc;

/* A private context cloned from a document's, used to rasterise display
 * lists concurrently with other renderers and document access. */
typedef struct {
	fz_context *ctx;
	fz_cookie cookie;
} onon_renderer;

typedef struct {
	float x0;
	float y0;
	float x1;
	float y1;
} onon_rect;

typedef struct {
	int x0;
	int y0;
	int x1;
	int y1;
} onon_irect;

typedef struct {
	float x;
	float y;
} onon_point;

typedef struct {
	onon_point ul;
	onon_point ur;
	onon_point ll;
	onon_point lr;
} onon_quad;

typedef struct {
	char *text;
	onon_quad *quads;
	int quad_count;
} onon_selection;

typedef struct {
	onon_quad *quads;
	int quad_count;
} onon_search_hit;

typedef struct {
	onon_search_hit *hits;
	int hit_count;
} onon_search_result;

typedef struct {
	int c;
	int line;
	int block;
	onon_quad quad;
} onon_char;

typedef struct {
	onon_char *chars;
	int char_count;
} onon_char_result;

typedef struct {
	onon_rect rect;
	char *uri;
	int is_external;
	int page_number;
	float x;
	float y;
	int has_x;
	int has_y;
} onon_link;

typedef struct {
	int index; /* -1 when no widget was found */
	int type;  /* enum pdf_widget_type */
	int readonly;
	onon_rect rect;
	char *value;
} onon_widget_info;

typedef struct {
	onon_link *links;
	int link_count;
} onon_link_result;

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
} onon_outline_item;

typedef struct {
	onon_outline_item *items;
	int item_count;
} onon_outline_result;

onon_doc *onon_open_document(const char *path, const char *password, size_t store_size, char **err);
void onon_close_document(onon_doc *handle);
int onon_count_pages(onon_doc *handle, int *count, char **err);
int onon_page_content_bounds(onon_doc *handle, int page_number, onon_rect *out, char **err);
int onon_page_info(onon_doc *handle, int page_number, onon_rect *bounds, char **label, char **err);
int onon_lookup_metadata(onon_doc *handle, const char *key, char **out, char **err);
int onon_page_image_bounds(onon_doc *handle, int page_number, onon_rect **out, int *count, char **err);
int onon_is_pdf(onon_doc *handle);
int onon_undo(onon_doc *handle, int redo, char **err);
int onon_annot_at(onon_doc *handle, int page_number, float x, float y, int *index, char **type, char **err);
int onon_edit_annot(onon_doc *handle, int page_number, int index, const float *rgb, char **err);
int onon_widget_at(onon_doc *handle, int page_number, float x, float y, onon_widget_info *out, char **err);
int onon_widget_options(onon_doc *handle, int page_number, int index, char ***out, int *count, char **err);
int onon_widget_edit(onon_doc *handle, int page_number, int index, const char *value, char **err);
void onon_free_strings(char **values, int count);
int onon_add_highlight(onon_doc *handle, int page_number, const onon_quad *quads, int count, const float *rgb, char **err);
int onon_save(onon_doc *handle, const char *path, int incremental, char **err);
int onon_can_save_incrementally(onon_doc *handle);
int onon_page_display_list(onon_doc *handle, int page_number, fz_display_list **out, char **err);
onon_renderer *onon_new_renderer(onon_doc *handle, char **err);
void onon_drop_renderer(onon_renderer *renderer);
void onon_arm_renderer(onon_renderer *renderer);
void onon_cancel_renderer(onon_renderer *renderer);
int onon_render_display_list(onon_renderer *renderer, fz_display_list *list, float scale, onon_irect clip, int aa_level, unsigned char **samples, int *width, int *height, int *stride, int *x, int *y, char **err);
void onon_free_rendered_page(unsigned char *samples);
int onon_extract_selection(onon_doc *handle, int page_number, float ax, float ay, float bx, float by, int mode, onon_selection *out, char **err);
int onon_page_text_ends(onon_doc *handle, int page_number, onon_point *start, onon_point *end, int *found, char **err);
void onon_free_selection(onon_doc *handle, onon_selection *sel);
int onon_page_text_list(onon_doc *handle, int page_number, fz_display_list **out, char **err);
int onon_search_display_list(onon_renderer *renderer, fz_display_list *list, const char *needle, onon_search_result *out, char **err);
void onon_free_search_result(onon_search_result *result);
int onon_extract_page_text(onon_doc *handle, int page_number, char **out, char **err);
void onon_free_text(onon_doc *handle, char *text);
int onon_display_list_chars(onon_renderer *renderer, fz_display_list *list, onon_char_result *out, char **err);
void onon_free_char_result(onon_char_result *result);
int onon_load_links(onon_doc *handle, int page_number, onon_link_result *out, char **err);
void onon_free_link_result(onon_link_result *result);
int onon_load_outline(onon_doc *handle, onon_outline_result *out, char **err);
void onon_free_outline_result(onon_outline_result *result);
int onon_recognize_document_name(const char *name);
void onon_free_string(char *value);

#endif
