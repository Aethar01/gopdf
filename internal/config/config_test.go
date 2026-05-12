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

	rt, err := Open(path, "/tmp/special.pdf")
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

	rt, err := Open(path, "/tmp/first.pdf")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if got := rt.Config().PageGapVertical; got != 11 {
		t.Fatalf("expected first document config, got %d", got)
	}
	if err := rt.SetDocument("/tmp/second.pdf"); err != nil {
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	_, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	rt, err := Open(path, "/tmp/doc.pdf")
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

	_, err := Open(path, "/tmp/doc.pdf")
	if err == nil {
		t.Fatal("expected config load to fail")
	}
	if !strings.Contains(err.Error(), "viewer host unavailable") {
		t.Fatalf("expected host unavailable error, got %v", err)
	}
}
