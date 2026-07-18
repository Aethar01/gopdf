package config

import (
	"fmt"
	"sort"
	"strings"
)

type OptionReference struct {
	Name        string
	Type        string
	Default     string
	Description string
}

type LuaReferenceEntry struct {
	Signature   string
	Description string
}

var optionDescriptions = map[string]string{
	"alt_background":         "Viewer background in alternate-color mode.",
	"alt_colors":             "Start with alternate colors enabled.",
	"alt_foreground":         "UI foreground in alternate-color mode.",
	"alt_page_background":    "Page background in alternate-color mode.",
	"alt_status_bar_color":   "Status bar background in alternate-color mode.",
	"anchor_position":        "Viewport anchor: center, top, or bottom.",
	"anti_aliasing":          "MuPDF antialiasing level from 0 through 8.",
	"background":             "Viewer background color.",
	"completion_max_items":   "Maximum command-completion rows.",
	"dual_page":              "Start in dual-page mode.",
	"first_page_offset":      "Treat the first page as a standalone cover in dual-page mode.",
	"fit_mode":               "Initial fit mode: page, width, or manual.",
	"foreground":             "UI foreground color.",
	"highlight_background":   "Selection and search highlight background.",
	"highlight_foreground":   "Selection and search highlight border and text.",
	"mouse_text_select":      "Enable text selection with the left mouse button.",
	"natural_scroll":         "Reverse mouse-wheel scrolling direction.",
	"outline_height_percent": "Outline overlay height as a percentage of the window.",
	"outline_initial_depth":  "Outline levels expanded when the outline opens.",
	"outline_width_percent":  "Outline overlay width as a percentage of the window.",
	"page_background":        "Normal page background color.",
	"page_cache_size":        "Maximum rendered pages retained in the cache.",
	"page_gap":               "Vertical gap between pages; aliases page_gap_vertical.",
	"page_gap_horizontal":    "Horizontal gap between pages in a spread.",
	"page_gap_vertical":      "Vertical gap between page rows.",
	"recent_files_max":       "Maximum recent files retained and displayed.",
	"render_mode":            "Initial render mode: continuous or single.",
	"render_oversample":      "Render scale multiplier; values above 1 supersample.",
	"scroll_step":            "Keyboard and mouse scroll distance in pixels.",
	"sequence_timeout_ms":    "Maximum delay between keys in a binding sequence.",
	"session_database":       "Persist per-document view state, marks, and recent files.",
	"spread_gap":             "Horizontal spread gap; aliases page_gap_horizontal.",
	"status_bar_color":       "Normal status bar background color.",
	"status_bar_height":      "Status bar height in pixels.",
	"status_bar_left":        "Left status bar template.",
	"status_bar_padding":     "Horizontal status bar padding in pixels.",
	"status_bar_right":       "Right status bar template.",
	"status_bar_visible":     "Show the status bar at startup.",
	"ui_font_path":           "Path to a UI font; empty uses the built-in default.",
	"ui_font_size":           "UI font size in pixels.",
}

var luaReference = []LuaReferenceEntry{
	{Signature: "gopdf.bind(key, action)", Description: "Bind a key sequence to an action or Lua callback."},
	{Signature: "gopdf.unbind(key)", Description: "Remove a key binding."},
	{Signature: "gopdf.bind_mouse(event, action)", Description: "Bind a mouse event to an action or Lua callback."},
	{Signature: "gopdf.unbind_mouse(event)", Description: "Remove a mouse binding."},
	{Signature: "gopdf.message([text])", Description: "Get the current message or set it when text is supplied."},
	{Signature: "gopdf.command(command)", Description: "Execute a viewer command."},
	{Signature: "gopdf.open(path)", Description: "Open another document."},
	{Signature: "gopdf.pick_file([callback])", Description: "Open the native PDF picker; returns a path or invokes callback."},
	{Signature: "gopdf.page()", Description: "Return the current 1-based physical page number."},
	{Signature: "gopdf.page_count()", Description: "Return the document page count."},
	{Signature: "gopdf.goto_page(page)", Description: "Jump to a 1-based physical page number."},
	{Signature: "gopdf.mode()", Description: "Return the current input mode."},
	{Signature: "gopdf.search(query[, backward])", Description: "Search using the same flags as :search."},
	{Signature: "gopdf.search_query()", Description: "Return the active search query."},
	{Signature: "gopdf.search_match_count()", Description: "Return the number of discovered search matches."},
	{Signature: "gopdf.search_match_index()", Description: "Return the current 1-based match index or nil."},
	{Signature: "gopdf.current_count()", Description: "Return the pending numeric action count."},
	{Signature: "gopdf.pending_keys()", Description: "Return pending key-sequence tokens."},
	{Signature: "gopdf.clear_pending_keys()", Description: "Clear the pending sequence, mark, and numeric count."},
	{Signature: "gopdf.recent_files([limit])", Description: "Return recent document paths."},
	{Signature: "gopdf.fit_mode()", Description: "Return the current fit mode."},
	{Signature: "gopdf.set_fit_mode(mode)", Description: "Set page, width, or manual fit mode."},
	{Signature: "gopdf.render_mode()", Description: "Return continuous or single render mode."},
	{Signature: "gopdf.set_render_mode(mode)", Description: "Set continuous or single render mode."},
	{Signature: "gopdf.zoom()", Description: "Return the current render scale."},
	{Signature: "gopdf.set_zoom(scale)", Description: "Set manual zoom scale."},
	{Signature: "gopdf.rotation()", Description: "Return clockwise rotation in degrees."},
	{Signature: "gopdf.set_rotation(degrees)", Description: "Set clockwise rotation in degrees."},
	{Signature: "gopdf.fullscreen()", Description: "Return fullscreen state."},
	{Signature: "gopdf.set_fullscreen(enabled)", Description: "Set fullscreen state."},
	{Signature: "gopdf.status_bar_visible()", Description: "Return status bar visibility."},
	{Signature: "gopdf.set_status_bar_visible(visible)", Description: "Set status bar visibility."},
	{Signature: "gopdf.cache.entries()", Description: "Return the number of cached rendered pages."},
	{Signature: "gopdf.cache.pending()", Description: "Return the number of pending renders."},
	{Signature: "gopdf.cache.limit()", Description: "Return the rendered-page cache limit."},
	{Signature: "gopdf.cache.set_limit(limit)", Description: "Set the rendered-page cache limit."},
	{Signature: "gopdf.cache.clear()", Description: "Clear rendered-page caches."},
	{Signature: "gopdf.ui.show(spec)", Description: "Show a searchable modal list overlay."},
	{Signature: "gopdf.ui.close()", Description: "Close the active Lua overlay without on_close."},
	{Signature: "gopdf.ui.visible()", Description: "Return whether a Lua overlay is visible."},
	{Signature: "gopdf.ui.set_rows(rows)", Description: "Replace overlay rows."},
	{Signature: "gopdf.ui.set_selected(index)", Description: "Select a 1-based overlay row."},
}

func OptionReferences() []OptionReference {
	defaults := Default()
	refs := make([]OptionReference, 0, len(configOptions))
	for _, name := range OptionNames() {
		desc := configOptions[name]
		description, ok := optionDescriptions[name]
		if !ok {
			panic("missing option documentation: " + name)
		}
		value := desc.format(&defaults)
		if desc.kind == "color" {
			value = "{" + strings.ReplaceAll(value, ",", ", ") + "}"
		}
		refs = append(refs, OptionReference{Name: name, Type: desc.kind, Default: value, Description: description})
	}
	return refs
}

func LuaReferences() []LuaReferenceEntry {
	refs := append([]LuaReferenceEntry(nil), luaReference...)
	sort.Slice(refs, func(i, j int) bool { return refs[i].Signature < refs[j].Signature })
	return refs
}

func ValidateReferenceMetadata() error {
	for _, name := range OptionNames() {
		if strings.TrimSpace(optionDescriptions[name]) == "" {
			return fmt.Errorf("missing option documentation: %s", name)
		}
	}
	if len(optionDescriptions) != len(configOptions) {
		return fmt.Errorf("option documentation count %d does not match registry count %d", len(optionDescriptions), len(configOptions))
	}
	return nil
}
