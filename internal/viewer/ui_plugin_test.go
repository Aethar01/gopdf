package viewer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestLuaUIViewNormalizesInitialFilteredSelection(t *testing.T) {
	app := testLayoutApp(0)
	view := &uiView{
		id:       "filtered",
		owner:    "lua",
		modal:    true,
		query:    "visible",
		selected: 0,
		rows: []uiRow{
			{index: 0, text: "hidden"},
			{index: 1, text: "visible"},
		},
	}
	selected := -1
	view.onSelect = func(_ *App, row uiRow) { selected = row.index }

	app.showUIView(view)
	app.activateUIView(view)

	if view.selected != 1 || selected != 1 {
		t.Fatalf("expected visible row 2 to be selected and activated, selected=%d callback=%d", view.selected, selected)
	}
}

func TestLuaUIEmptyViewHasNoSelection(t *testing.T) {
	app := testLayoutApp(0)
	if err := app.ShowUI(config.UIOverlay{ID: "empty", Searchable: true}); err != nil {
		t.Fatal(err)
	}
	if got := app.UISelected("empty"); got != 0 {
		t.Fatalf("empty view selection = %d, want 0", got)
	}
	app.SetUISelected("empty", 1)
	if got := app.UISelected("empty"); got != 0 {
		t.Fatalf("empty view selection after set = %d, want 0", got)
	}
}

func TestLuaUIRowsPreserveID(t *testing.T) {
	rows := uiRowsFromConfig([]config.UIListRow{{ID: "stable-id", Text: "row"}})
	if len(rows) != 1 || rows[0].id != "stable-id" {
		t.Fatalf("row ID was not preserved: %+v", rows)
	}
}

func TestLuaUISelectionCallbackReceivesRowID(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	source := `
bind("u", function()
  local view = gopdf.ui.create({
    rows = { { id = "stable-id", text = "row", value = "value" } },
    on_select = function(index, value, text, id)
      gopdf.message(tostring(index) .. ":" .. value .. ":" .. text .. ":" .. id)
    end,
  })
  view:show()
end)
`
	if err := os.WriteFile(configPath, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.Open(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	app := testLayoutApp(0)
	app.runtime = runtime
	runtime.AttachHost(app)
	if handled, _, err := runtime.RunAction(runtime.Config().KeyBindings["u"]); !handled || err != nil {
		t.Fatalf("show view action: handled=%v err=%v", handled, err)
	}
	app.activateUIView(app.activeUIView())
	if app.message != "1:value:row:stable-id" {
		t.Fatalf("selection callback arguments = %q", app.message)
	}
}

func TestLuaUIClosePathsInvokeCallback(t *testing.T) {
	app := testLayoutApp(0)
	closed := 0
	view := &uiView{id: "view", owner: "lua", modal: true, visible: true, onClose: func(*App) { closed++ }}
	app.views.show(view)
	app.CloseUI("view")
	if closed != 1 {
		t.Fatalf("programmatic close callbacks = %d, want 1", closed)
	}

	view.visible = true
	app.views.active = view
	app.closeAllUI()
	if closed != 2 {
		t.Fatalf("teardown close callbacks = %d, want 2", closed)
	}
}

func TestLuaUIViewGenerationSurvivesSuccessfulReload(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(configPath, []byte(`-- initial config`), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.Open(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	app := testLayoutApp(0)
	app.runtime = runtime
	runtime.AttachHost(app)
	if _, err := runtime.Eval(`local view = gopdf.ui.create({ title = "old", rows = { "old" } }); view:show()`); err != nil {
		t.Fatal(err)
	}
	old := app.activeUIView()

	if err := os.WriteFile(configPath, []byte(`local view = gopdf.ui.create({ title = "new", rows = { "new" } }); view:show()`), 0o644); err != nil {
		t.Fatal(err)
	}
	app.reloadConfig()
	active := app.activeUIView()
	if active == nil || active.title != "new" || active.generation != runtime.Generation() {
		t.Fatalf("new-generation view was not retained: %+v", active)
	}
	if _, exists := app.views.views[old.id]; exists {
		t.Fatal("old-generation view was retained")
	}
}

func TestLuaUIViewGenerationRollsBackAfterFailedReload(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(configPath, []byte(`-- initial config`), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.Open(configPath, "")
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	app := testLayoutApp(0)
	app.runtime = runtime
	runtime.AttachHost(app)
	if _, err := runtime.Eval(`local view = gopdf.ui.create({ title = "old", rows = { "old" } }); view:show()`); err != nil {
		t.Fatal(err)
	}
	old := app.activeUIView()

	broken := `local view = gopdf.ui.create({ title = "broken", rows = { "broken" } }); view:show(); error("reload failed")`
	if err := os.WriteFile(configPath, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	app.reloadConfig()
	if active := app.activeUIView(); active != old {
		t.Fatalf("failed reload did not restore old view: %+v", active)
	}
	for _, view := range app.views.views {
		if view.title == "broken" {
			t.Fatal("failed-generation view was retained")
		}
	}
	if !strings.Contains(app.message, "reload failed") {
		t.Fatalf("expected reload error, got %q", app.message)
	}
}

func TestMouseButtonPreCanConsumeButtonUp(t *testing.T) {
	root := t.TempDir()
	pluginDir := filepath.Join(root, "mouse")
	if err := os.MkdirAll(filepath.Join(pluginDir, "lua", "mouse"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(pluginDir, "gopdf-plugin.json"), []byte(`{"id":"mouse"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	plugin := `
local M = gopdf.plugin.register("mouse")
M:on("mouse_button_pre", function(event)
  if event.phase == "up" then
    gopdf.message("consumed up")
    return true
  end
end)
return M
`
	if err := os.WriteFile(filepath.Join(pluginDir, "lua", "mouse", "init.lua"), []byte(plugin), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(configPath, []byte(`require("mouse")`), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.OpenWithOptions(configPath, "", config.OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()
	app := testLayoutApp(1)
	app.runtime = runtime
	app.config.MouseTextSelect = true
	app.selection.active = true
	runtime.AttachHost(app)

	event := &sdl.MouseButtonEvent{CommonEvent: sdl.CommonEvent{Type: sdl.EventMouseButtonUp}, Button: uint8(sdl.ButtonLeft)}
	app.handleMouseButtonEvent(event)

	if !app.selection.active {
		t.Fatal("consumed button-up reached normal selection handling")
	}
	if app.message != "consumed up" {
		t.Fatalf("pre-event callback did not run, message=%q", app.message)
	}
}
