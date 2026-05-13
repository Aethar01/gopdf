package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type stubHost struct {
	actions          []string
	opened           string
	ui               UIOverlay
	uiVisible        bool
	uiClosed         bool
	page             int
	pages            int
	message          string
	commands         []string
	mode             string
	searchQuery      string
	searchBackward   bool
	searchMatchCount int
	searchMatchIndex int
	currentCount     string
	pendingKeys      []string
	fitMode          string
	renderMode       string
	zoom             float64
	rotation         float64
	fullscreen       bool
	statusBarVisible bool
	cacheEntries     int
	cachePending     int
	cacheLimit       int
	cacheCleared     bool
}

func (h *stubHost) ExecuteAction(action string) error {
	h.actions = append(h.actions, action)
	return nil
}

func (h *stubHost) Open(path string) error {
	h.opened = path
	return nil
}

func (h *stubHost) ShowUI(overlay UIOverlay) error {
	h.ui = overlay
	h.uiVisible = true
	return nil
}

func (h *stubHost) CloseUI() {
	h.uiVisible = false
	h.uiClosed = true
}

func (h *stubHost) UIVisible() bool { return h.uiVisible }

func (h *stubHost) SetUIRows(rows []string) { h.ui.Rows = append([]string(nil), rows...) }

func (h *stubHost) SetUISelected(selected int) { h.ui.Selected = selected }

func (h *stubHost) Page() int {
	return h.page
}

func (h *stubHost) PageCount() int {
	return h.pages
}

func (h *stubHost) GotoPage(page int) error {
	h.page = page
	return nil
}

func (h *stubHost) Message() string {
	return h.message
}

func (h *stubHost) SetMessage(message string) {
	h.message = message
}

func (h *stubHost) RunCommand(command string) error {
	h.commands = append(h.commands, command)
	return nil
}

func (h *stubHost) Mode() string { return h.mode }

func (h *stubHost) Search(query string, backward bool) error {
	h.searchQuery = query
	h.searchBackward = backward
	return nil
}

func (h *stubHost) SearchQuery() string { return h.searchQuery }

func (h *stubHost) SearchMatchCount() int { return h.searchMatchCount }

func (h *stubHost) SearchMatchIndex() int { return h.searchMatchIndex }

func (h *stubHost) CurrentCount() string { return h.currentCount }

func (h *stubHost) PendingKeys() []string { return append([]string(nil), h.pendingKeys...) }

func (h *stubHost) ClearPendingKeys() {
	h.pendingKeys = nil
	h.currentCount = ""
}

func (h *stubHost) FitMode() string { return h.fitMode }

func (h *stubHost) SetFitMode(mode string) error {
	h.fitMode = mode
	return nil
}

func (h *stubHost) RenderMode() string { return h.renderMode }

func (h *stubHost) SetRenderMode(mode string) error {
	h.renderMode = mode
	return nil
}

func (h *stubHost) Zoom() float64 { return h.zoom }

func (h *stubHost) SetZoom(zoom float64) error {
	h.zoom = zoom
	return nil
}

func (h *stubHost) Rotation() float64 { return h.rotation }

func (h *stubHost) SetRotation(rotation float64) error {
	h.rotation = rotation
	return nil
}

func (h *stubHost) Fullscreen() bool { return h.fullscreen }

func (h *stubHost) SetFullscreen(fullscreen bool) error {
	h.fullscreen = fullscreen
	return nil
}

func (h *stubHost) StatusBarVisible() bool { return h.statusBarVisible }

func (h *stubHost) SetStatusBarVisible(visible bool) error {
	h.statusBarVisible = visible
	return nil
}

func (h *stubHost) CacheEntries() int { return h.cacheEntries }

func (h *stubHost) CachePending() int { return h.cachePending }

func (h *stubHost) CacheLimit() int { return h.cacheLimit }

func (h *stubHost) SetCacheLimit(limit int) error {
	h.cacheLimit = limit
	return nil
}

func (h *stubHost) ClearCache() { h.cacheCleared = true }

func TestOpenAppliesDocumentSpecificLuaConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
if gopdf.document.name == "special.pdf" then
  options.first_page_offset = false
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "special.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if rt.Config().FirstPageOffset {
		t.Fatalf("expected first_page_offset to be false for matching document.name")
	}
}

func TestSetDocumentReloadsDocumentSpecificLuaConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
if gopdf.document.name == "first.pdf" then
  options.page_gap_vertical = 11
end
if gopdf.document.name == "second.pdf" then
  options.page_gap_vertical = 22
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "first.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if got := rt.Config().PageGapVertical; got != 11 {
		t.Fatalf("expected first document config, got %d", got)
	}
	if err := rt.SetDocument(filepath.Join(dir, "second.pdf")); err != nil {
		t.Fatal(err)
	}
	if got := rt.Config().PageGapVertical; got != 22 {
		t.Fatalf("expected second document config, got %d", got)
	}
}

func TestLuaFunctionBindingMutatesConfigAtRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
bind("h", function()
  options.page_gap_vertical = 10
end)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	action := rt.Config().KeyBindings["h"]
	if action == "" {
		t.Fatal("expected key binding for h")
	}
	if handled, dirty, err := rt.RunAction(action); !handled || !dirty || err != nil {
		t.Fatalf("expected lua callback to run, handled=%v dirty=%v err=%v", handled, dirty, err)
	}
	if got := rt.Config().PageGapVertical; got != 10 {
		t.Fatalf("expected page_gap_vertical=10, got %d", got)
	}
	if got := rt.Config().PageGap; got != 10 {
		t.Fatalf("expected page_gap=10 mirror, got %d", got)
	}
}

func TestActionValuesBindAndExecuteAgainstViewerHost(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
bind("J", gopdf.next_page)
bind("H", function()
  gopdf.next_page()
  gopdf.message("page " .. gopdf.page() .. "/" .. gopdf.page_count())
  gopdf.command(":fit width")
end)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if got := rt.Config().KeyBindings["J"]; got != "next_page" {
		t.Fatalf("expected J to bind next_page, got %q", got)
	}

	host := &stubHost{page: 3, pages: 12}
	rt.AttachHost(host)
	action := rt.Config().KeyBindings["H"]
	if handled, dirty, err := rt.RunAction(action); !handled || dirty || err != nil {
		t.Fatalf("expected lua callback to run cleanly, handled=%v dirty=%v err=%v", handled, dirty, err)
	}
	if len(host.actions) != 1 || host.actions[0] != "next_page" {
		t.Fatalf("expected next_page action, got %v", host.actions)
	}
	if host.message != "page 3/12" {
		t.Fatalf("expected message to be updated, got %q", host.message)
	}
	if len(host.commands) != 1 || host.commands[0] != ":fit width" {
		t.Fatalf("expected command to be forwarded, got %v", host.commands)
	}
	if handled, _, err := rt.RunAction("missing"); handled || err != nil {
		t.Fatalf("expected missing action to be ignored, handled=%v err=%v", handled, err)
	}
}

func TestMouseInteractionOptions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
options.natural_scroll = true
options.anti_aliasing = 4
options.render_oversample = 0.75
bind_mouse("right_down", gopdf.pan)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if got := rt.Config().MouseBindings["right_down"]; got != "pan" {
		t.Fatalf("expected right_down to bind pan, got %q", got)
	}
	if !rt.Config().NaturalScroll {
		t.Fatal("expected natural_scroll=true")
	}
	if got := rt.Config().AntiAliasing; got != 4 {
		t.Fatalf("expected anti_aliasing=4, got %d", got)
	}
	if got := rt.Config().RenderOversample; got != 0.75 {
		t.Fatalf("expected render_oversample=0.75, got %.2f", got)
	}
}

func TestOutlineConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
options.outline_initial_depth = 2
options.outline_width_percent = 60
options.outline_height_percent = 75
bind("O", gopdf.outline)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	cfg := rt.Config()
	if cfg.OutlineInitialDepth != 2 || cfg.OutlineWidthPercent != 60 || cfg.OutlineHeightPercent != 75 {
		t.Fatalf("unexpected outline config: %+v", cfg)
	}
	if got := cfg.KeyBindings["O"]; got != "outline" {
		t.Fatalf("expected outline binding, got %q", got)
	}
}

func TestCallingActionDuringConfigLoadFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`bind("J", gopdf.next_page())`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err == nil {
		t.Fatal("expected config load to fail")
	}
	if !strings.Contains(err.Error(), "cannot execute during config load") {
		t.Fatalf("expected config-load action error, got %v", err)
	}
}

func TestDocumentMetadataExposesFileFacts(t *testing.T) {
	dir := t.TempDir()
	docPath := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(docPath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
if gopdf.document.exists and gopdf.document.extension == ".txt" and gopdf.document.size_bytes > 0 then
  options.page_gap_vertical = 12
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, docPath)
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if got := rt.Config().PageGapVertical; got != 12 {
		t.Fatalf("expected document metadata to drive config, got page_gap_vertical=%d", got)
	}
	if got := rt.Config().PageGap; got != 12 {
		t.Fatalf("expected mirrored page_gap=12, got %d", got)
	}
}

func TestExpandedLuaHostAPI(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
bind("X", function()
  gopdf.goto_page(9)
  gopdf.set_fit_mode("width")
  gopdf.set_render_mode("single")
  gopdf.set_zoom(1.75)
  gopdf.set_rotation(180)
  gopdf.set_fullscreen(true)
  gopdf.set_status_bar_visible(false)
  gopdf.search("needle", true)
  local keys = gopdf.pending_keys()
  gopdf.cache.set_limit(48)
  gopdf.cache.clear()
  gopdf.message(
    gopdf.mode() .. ":" ..
    gopdf.fit_mode() .. ":" ..
    gopdf.render_mode() .. ":" ..
    tostring(gopdf.zoom()) .. ":" ..
    tostring(gopdf.rotation()) .. ":" ..
    tostring(gopdf.search_match_index()) .. "/" .. tostring(gopdf.search_match_count()) .. ":" ..
    gopdf.current_count() .. ":" ..
    keys[1]
  )
  gopdf.clear_pending_keys()
end)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	host := &stubHost{
		mode:             "normal",
		searchMatchCount: 5,
		searchMatchIndex: 2,
		currentCount:     "12",
		pendingKeys:      []string{"g", "g"},
		fitMode:          "page",
		renderMode:       "continuous",
		zoom:             1.25,
		rotation:         90,
		statusBarVisible: true,
		cacheEntries:     7,
		cachePending:     3,
		cacheLimit:       24,
	}
	rt.AttachHost(host)
	action := rt.Config().KeyBindings["X"]
	if handled, dirty, err := rt.RunAction(action); !handled || dirty || err != nil {
		t.Fatalf("expected expanded callback to run, handled=%v dirty=%v err=%v", handled, dirty, err)
	}
	if host.page != 9 || host.fitMode != "width" || host.renderMode != "single" {
		t.Fatalf("expected navigation/view setters to run, host=%+v", host)
	}
	if host.zoom != 1.75 || host.rotation != 180 || !host.fullscreen || host.statusBarVisible {
		t.Fatalf("expected zoom/rotation/fullscreen/status updates, host=%+v", host)
	}
	if host.searchQuery != "needle" || !host.searchBackward {
		t.Fatalf("expected backward search to run, host=%+v", host)
	}
	if host.cacheLimit != 48 || !host.cacheCleared {
		t.Fatalf("expected cache controls to run, host=%+v", host)
	}
	if got := host.message; got != "normal:width:single:1.75:180:2/5:12:g" {
		t.Fatalf("unexpected message %q", got)
	}
	if len(host.pendingKeys) != 0 || host.currentCount != "" {
		t.Fatalf("expected pending input to clear, host=%+v", host)
	}
}

func TestLuaUIShowsMenuAndRunsSelectionCallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
bind("u", function()
  gopdf.ui.show({
    title = "Open PDF",
    rows = { "a.pdf", "b.pdf" },
    selected = 2,
    on_select = function(index, value)
      gopdf.message(tostring(index) .. ":" .. value)
      gopdf.open(value)
    end,
    on_close = function()
      gopdf.message("closed")
    end,
  })
end)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	host := &stubHost{}
	rt.AttachHost(host)
	if handled, dirty, err := rt.RunAction(rt.Config().KeyBindings["u"]); !handled || dirty || err != nil {
		t.Fatalf("expected ui callback to run, handled=%v dirty=%v err=%v", handled, dirty, err)
	}
	if !host.uiVisible || host.ui.Title != "Open PDF" || host.ui.Selected != 2 {
		t.Fatalf("unexpected ui state: %+v visible=%v", host.ui, host.uiVisible)
	}
	if got := strings.Join(host.ui.Rows, ","); got != "a.pdf,b.pdf" {
		t.Fatalf("unexpected rows %q", got)
	}
	if host.ui.OnSelect == "" || host.ui.OnClose == "" {
		t.Fatalf("expected ui callbacks, got %+v", host.ui)
	}
	if err := rt.RunUISelect(host.ui.OnSelect, 2, "b.pdf"); err != nil {
		t.Fatal(err)
	}
	if host.message != "2:b.pdf" || host.opened != "b.pdf" {
		t.Fatalf("expected selection callback to update host, host=%+v", host)
	}
	if err := rt.RunUIClose(host.ui.OnClose); err != nil {
		t.Fatal(err)
	}
	if host.message != "closed" {
		t.Fatalf("expected close callback, got %q", host.message)
	}
}

func TestLuaUIHostControls(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
bind("u", function()
  gopdf.ui.menu({ rows = { "old" } })
  if gopdf.ui.visible() then
    gopdf.ui.set_rows({ "new", "items" })
    gopdf.ui.set_selected(2)
    gopdf.ui.close()
  end
end)
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	host := &stubHost{}
	rt.AttachHost(host)
	if handled, _, err := rt.RunAction(rt.Config().KeyBindings["u"]); !handled || err != nil {
		t.Fatalf("expected ui controls to run, handled=%v err=%v", handled, err)
	}
	if !host.uiClosed || host.uiVisible {
		t.Fatalf("expected ui to close, visible=%v closed=%v", host.uiVisible, host.uiClosed)
	}
	if got := strings.Join(host.ui.Rows, ","); got != "new,items" {
		t.Fatalf("unexpected rows %q", got)
	}
	if host.ui.Selected != 2 {
		t.Fatalf("unexpected selected %d", host.ui.Selected)
	}
}

func TestLuaUIShowDuringConfigLoadFails(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`gopdf.ui.show({ rows = { "x" } })`), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err == nil {
		t.Fatal("expected config load to fail")
	}
	if !strings.Contains(err.Error(), "viewer host unavailable") {
		t.Fatalf("expected host unavailable error, got %v", err)
	}
}

func TestLuaUnbindRemovesDefaultBindings(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
unbind("j")
unbind_mouse("middle_down")
unbind_mouse("ctrl_wheel_down")
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	cfg := rt.Config()
	if _, ok := cfg.KeyBindings["j"]; ok {
		t.Fatalf("expected j binding to be removed, got %q", cfg.KeyBindings["j"])
	}
	if _, ok := cfg.MouseBindings["middle_down"]; ok {
		t.Fatalf("expected middle_down mouse binding to be removed, got %q", cfg.MouseBindings["middle_down"])
	}
	if _, ok := cfg.MouseBindings["<c-wheel_down>"]; ok {
		t.Fatalf("expected ctrl_wheel_down alias to remove <c-wheel_down>, got %q", cfg.MouseBindings["<c-wheel_down>"])
	}
}

func TestLuaOptionTypeErrorsIncludeSettingName(t *testing.T) {
	tests := []struct {
		name    string
		lua     string
		wantErr string
	}{
		{name: "boolean", lua: `options.natural_scroll = "yes"`, wantErr: "options.natural_scroll: expected boolean"},
		{name: "number", lua: `options.page_gap = true`, wantErr: "options.page_gap: expected number"},
		{name: "string", lua: `options.fit_mode = false`, wantErr: "options.fit_mode: expected string"},
		{name: "table", lua: `options.background = 10`, wantErr: "options.background: expected table"},
		{name: "unknown", lua: `options.no_such_setting = 1`, wantErr: "options.no_such_setting: unknown setting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "config.lua")
			if err := os.WriteFile(path, []byte(tt.lua), 0o644); err != nil {
				t.Fatal(err)
			}

			rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
			if rt != nil {
				rt.Close()
			}
			if err == nil {
				t.Fatal("expected config load to fail")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("expected error containing %q, got %v", tt.wantErr, err)
			}
		})
	}
}

func TestLuaOptionNormalizationAndColorClamping(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
options.fit_mode = " WIDTH "
options.render_mode = "SINGLE"
options.completion_max_items = 0
options.background = { -5, 128, 999 }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	cfg := rt.Config()
	if cfg.FitMode != "width" {
		t.Fatalf("expected normalized fit mode width, got %q", cfg.FitMode)
	}
	if cfg.RenderMode != "single" {
		t.Fatalf("expected normalized render mode single, got %q", cfg.RenderMode)
	}
	if cfg.CompletionMaxItems != 1 {
		t.Fatalf("expected completion_max_items to clamp to 1, got %d", cfg.CompletionMaxItems)
	}
	if cfg.Background != [3]uint8{0, 128, 255} {
		t.Fatalf("expected clamped background color, got %v", cfg.Background)
	}
}

func TestLoadReturnsConfigFromExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`options.status_bar_visible = false`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.StatusBarVisible {
		t.Fatal("expected explicit config to hide status bar")
	}
	if cfg.ConfigPath != path {
		t.Fatalf("expected config path %q, got %q", path, cfg.ConfigPath)
	}
}

func TestNormalizeMouseEventAliases(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: " wheel_down ", want: "wheel_down"},
		{input: "CTRL_wheel_up", want: "<c-wheel_up>"},
		{input: "<c-wheel_down>", want: "<c-wheel_down>"},
		{input: "Middle_Down", want: "middle_down"},
	}

	for _, tt := range tests {
		if got := normalizeMouseEvent(tt.input); got != tt.want {
			t.Fatalf("normalizeMouseEvent(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestLuaCanReadCurrentOptionValues(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
options.status_bar_visible = false
options.page_gap = 14
if options.status_bar_visible == false and options.page_gap == 14 then
  options.status_bar_left = "read"
end
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	cfg := rt.Config()
	if cfg.StatusBarVisible || cfg.PageGap != 14 || cfg.StatusBarLeft != "read" {
		t.Fatalf("expected Lua to read option values and default message, got %+v", cfg)
	}
}

func TestLuaColorOptionsClampEveryColorField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(`
options.background = { -1, 10, 300 }
options.page_background = { 1, 2, 3 }
options.foreground = { 4, 5, 6 }
options.status_bar_color = { 7, 8, 9 }
options.alt_background = { 10, 11, 12 }
options.alt_page_background = { 13, 14, 15 }
options.alt_foreground = { 16, 17, 18 }
options.alt_status_bar_color = { 19, 20, 21 }
options.highlight_foreground = { 22, 23, 24 }
options.highlight_background = { 25, 26, 27 }
`), 0o644); err != nil {
		t.Fatal(err)
	}

	rt, err := Open(path, filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	cfg := rt.Config()
	checks := []struct {
		name string
		got  [3]uint8
		want [3]uint8
	}{
		{name: "background", got: cfg.Background, want: [3]uint8{0, 10, 255}},
		{name: "page background", got: cfg.PageBackground, want: [3]uint8{1, 2, 3}},
		{name: "foreground", got: cfg.Foreground, want: [3]uint8{4, 5, 6}},
		{name: "status bar", got: cfg.StatusBarColor, want: [3]uint8{7, 8, 9}},
		{name: "alt background", got: cfg.AltBackground, want: [3]uint8{10, 11, 12}},
		{name: "alt page background", got: cfg.AltPageBackground, want: [3]uint8{13, 14, 15}},
		{name: "alt foreground", got: cfg.AltForeground, want: [3]uint8{16, 17, 18}},
		{name: "alt status bar", got: cfg.AltStatusBarColor, want: [3]uint8{19, 20, 21}},
		{name: "highlight foreground", got: cfg.HighlightForeground, want: [3]uint8{22, 23, 24}},
		{name: "highlight background", got: cfg.HighlightBackground, want: [3]uint8{25, 26, 27}},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Fatalf("%s = %v, want %v", check.name, check.got, check.want)
		}
	}
}
