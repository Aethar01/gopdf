package viewer

import (
	"os"
	"path/filepath"
	"testing"

	"gopdf/internal/config"
)

// viewEventPlugin records page_changed and zoom_changed so the test can assert
// both that an event fired and what it carried.
const viewEventPlugin = `
local M = gopdf.plugin.register("viewevents")
M.pages = {}
M.zooms = {}
M:on("page_changed", function(event)
  M.pages[#M.pages + 1] = event
end)
M:on("zoom_changed", function(event)
  M.zooms[#M.zooms + 1] = event
end)
return M
`

func viewEventApp(t *testing.T, pageCount int) *App {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "viewevents")
	if err := os.MkdirAll(filepath.Join(dir, "lua", "viewevents"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gopdf-plugin.json"), []byte(`{"id":"viewevents","version":"0.1.0","api":2}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "lua", "viewevents", "init.lua"), []byte(viewEventPlugin), 0o644); err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(t.TempDir(), "config.lua")
	if err := os.WriteFile(configPath, []byte(`viewevents = require("viewevents")`), 0o644); err != nil {
		t.Fatal(err)
	}
	runtime, err := config.OpenWithOptions(configPath, "", config.OpenOptions{PluginPaths: []string{root}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	app := testLayoutApp(pageCount)
	app.runtime = runtime
	runtime.AttachHost(app)
	return app
}

func TestViewStateEventsReportPageAndZoomChanges(t *testing.T) {
	app := viewEventApp(t, 5)
	app.scale = 1

	// The first pass only establishes a baseline; opening a document is already
	// reported by document_opened.
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`assert(#viewevents.pages == 0 and #viewevents.zooms == 0)`); err != nil {
		t.Fatal(err)
	}

	app.page = 2
	app.pageMetrics[2].label = "iii"
	app.scale = 1.5
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`
assert(#viewevents.pages == 1, "expected one page_changed")
local page = viewevents.pages[1]
assert(page.page == 3 and page.previous_page == 1 and page.label == "iii" and page.page_count == 5)
assert(#viewevents.zooms == 1, "expected one zoom_changed")
local zoom = viewevents.zooms[1]
assert(zoom.scale == 1.5 and zoom.previous_scale == 1 and zoom.percent == 150)
`); err != nil {
		t.Fatal(err)
	}

	// An unchanged view emits nothing further.
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`assert(#viewevents.pages == 1 and #viewevents.zooms == 1)`); err != nil {
		t.Fatal(err)
	}
}

func TestViewStateEventsCoalesceWithinAFrame(t *testing.T) {
	app := viewEventApp(t, 10)
	app.scale = 1
	app.emitViewStateEvents()

	// Several moves between frames report only where the view settled.
	app.page = 3
	app.page = 7
	app.scale = 2
	app.scale = 4
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`
assert(#viewevents.pages == 1 and viewevents.pages[1].page == 8 and viewevents.pages[1].previous_page == 1)
assert(#viewevents.zooms == 1 and viewevents.zooms[1].scale == 4 and viewevents.zooms[1].previous_scale == 1)
`); err != nil {
		t.Fatal(err)
	}
}

func TestViewStateEventsRebaselineOnNewDocument(t *testing.T) {
	app := viewEventApp(t, 4)
	app.scale = 1
	app.emitViewStateEvents()

	// installDocument clears tracking and bumps the generation, so the first pass
	// against a new document re-baselines instead of reporting a jump.
	app.viewEvents = viewStateEvents{}
	app.generation++
	app.page = 3
	app.scale = 2.5
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`assert(#viewevents.pages == 0 and #viewevents.zooms == 0)`); err != nil {
		t.Fatal(err)
	}

	app.page = 0
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`
assert(#viewevents.pages == 1 and viewevents.pages[1].page == 1 and viewevents.pages[1].previous_page == 4)
`); err != nil {
		t.Fatal(err)
	}
}

func TestViewStateEventsIgnoreDocumentlessViewer(t *testing.T) {
	app := viewEventApp(t, 0)
	app.page = 2
	app.scale = 3
	app.emitViewStateEvents()
	if _, err := app.runtime.Eval(`assert(#viewevents.pages == 0 and #viewevents.zooms == 0)`); err != nil {
		t.Fatal(err)
	}
}
