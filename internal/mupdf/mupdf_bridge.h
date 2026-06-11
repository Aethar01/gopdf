#ifndef GOPDF_MUPDF_BRIDGE_H
#define GOPDF_MUPDF_BRIDGE_H

#include <mupdf/fitz.h>

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

gopdf_doc *gopdf_open_document(const char *path, const char *password, char **err);
void gopdf_close_document(gopdf_doc *handle);
int gopdf_count_pages(gopdf_doc *handle, int *count, char **err);
int gopdf_page_bounds(gopdf_doc *handle, int page_number, gopdf_rect *out, char **err);
int gopdf_page_label(gopdf_doc *handle, int page_number, char **out, char **err);
int gopdf_render_page_info(gopdf_doc *handle, int page_number, float scale, float rotation, int *width, int *height, int *stride, int *x, int *y, char **err);
int gopdf_render_page_to_buffer(gopdf_doc *handle, int page_number, float scale, float rotation, int aa_level, unsigned char *samples, int width, int height, int stride, char **err);
int gopdf_extract_selection(gopdf_doc *handle, int page_number, float ax, float ay, float bx, float by, gopdf_selection *out, char **err);
void gopdf_free_selection(gopdf_doc *handle, gopdf_selection *sel);
int gopdf_search_page(gopdf_doc *handle, int page_number, const char *needle, gopdf_search_result *out, char **err);
void gopdf_free_search_result(gopdf_search_result *result);
int gopdf_extract_page_text(gopdf_doc *handle, int page_number, char **out, char **err);
void gopdf_free_text(gopdf_doc *handle, char *text);
int gopdf_load_links(gopdf_doc *handle, int page_number, gopdf_link_result *out, char **err);
void gopdf_free_link_result(gopdf_link_result *result);
int gopdf_load_outline(gopdf_doc *handle, gopdf_outline_result *out, char **err);
void gopdf_free_outline_result(gopdf_outline_result *result);

#endif
