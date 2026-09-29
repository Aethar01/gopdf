package viewer

import (
	"path/filepath"
	"slices"
	"testing"

	"gopdf/internal/config"
)

func TestRunCommandValidationPaths(t *testing.T) {
	app := testLayoutApp(3)
	app.runCommand(":page")
	if app.message != "usage: :page <n>" {
		t.Fatalf("expected page usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":page nope")
	if app.message != "invalid page or label: nope" {
		t.Fatalf("expected invalid page message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":mode")
	if app.message != "usage: :mode continuous|single" {
		t.Fatalf("expected mode usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":colors")
	if app.message != "usage: :colors normal|alt" {
		t.Fatalf("expected colors usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":wat")
	if app.message != "unknown command: wat" {
		t.Fatalf("expected unknown command message, got %q", app.message)
	}

	app.runCommand(":quit")
	if !app.quit {
		t.Fatal("expected quit command to set quit flag")
	}
}

func TestGotoPageInputAcceptsPageLabel(t *testing.T) {
	app := testLayoutApp(4)
	app.pageMetrics[0].label = "i"
	app.pageMetrics[1].label = "ii"
	app.pageMetrics[2].label = "1"
	app.pageMetrics[3].label = "A-2"

	if page, ok := app.resolvePageInput("a-2"); !ok || page != 3 {
		t.Fatalf("expected label A-2 to resolve to page 4, got %d, %t", page+1, ok)
	}
	if page, ok := app.resolvePageInput("1"); !ok || page != 2 {
		t.Fatalf("expected numeric label 1 to resolve to page 3, got %d, %t", page+1, ok)
	}
}

func TestRunCommandAppliesViewerSettings(t *testing.T) {
	app := testLayoutApp(4)
	app.winW = 800
	app.winH = 600
	app.recomputeLayout(app.viewportSize())
	app.page = 2

	app.runCommand(":mode single")
	if app.renderMode != "single" || app.page != 2 {
		t.Fatalf("expected :mode single to preserve page 2, mode=%q page=%d", app.renderMode, app.page)
	}

	app.runCommand(":mode sideways")
	if app.renderMode != "continuous" {
		t.Fatalf("expected invalid render mode to fall back to continuous, got %q", app.renderMode)
	}

	app.runCommand(":fit width")
	if app.fitMode != "width" {
		t.Fatalf("expected :fit width to set fit mode, got %q", app.fitMode)
	}

	app.runCommand(":fit unknown")
	if app.fitMode != "page" {
		t.Fatalf("expected invalid fit mode to fall back to page, got %q", app.fitMode)
	}

	app.runCommand(":colors alt")
	if !app.altColors {
		t.Fatal("expected :colors alt to enable alternate colors")
	}

	app.runCommand(":colors normal")
	if app.altColors {
		t.Fatal("expected :colors normal to disable alternate colors")
	}
}

func TestSetCommandInspectsAndAssignsRegisteredOptions(t *testing.T) {
	rt, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	app := testLayoutApp(3)
	app.runtime = rt
	app.applyConfigState(rt.Config())

	app.runCommand(":set scroll_step=80")
	if app.config.ScrollStep != 80 || app.pageStep != 80 || app.message != "scroll_step=80" {
		t.Fatalf("expected applied scroll_step, config=%d step=%v message=%q", app.config.ScrollStep, app.pageStep, app.message)
	}
	app.runCommand(":set invert_scroll!")
	if !app.config.InvertScroll || app.message != "invert_scroll=true" {
		t.Fatalf("expected toggled invert_scroll, value=%t message=%q", app.config.InvertScroll, app.message)
	}
	app.runCommand(":set invert_smooth_scroll!")
	if !app.config.InvertSmoothScroll || app.message != "invert_smooth_scroll=true" {
		t.Fatalf("expected toggled invert_smooth_scroll, value=%t message=%q", app.config.InvertSmoothScroll, app.message)
	}
	app.runCommand(":set fit_mode?")
	if app.message != `fit_mode="page"` {
		t.Fatalf("expected inspected fit_mode, got %q", app.message)
	}
	app.runCommand(":set")
	view := app.activeUIView()
	if view == nil || view.title != "Options" || len(view.rows) != len(config.OptionNames()) {
		t.Fatalf("expected options inspector, view=%+v", view)
	}
	if dirty, _ := rt.ConsumeDirty(); dirty {
		t.Fatal("expected :set changes to consume runtime dirty state")
	}
}

func TestLuaCommandAppliesChangedOptions(t *testing.T) {
	app := testLayoutApp(4)
	app.winW = 800
	app.winH = 600
	app.dualPage = true
	app.recomputeLayout(app.viewportSize())

	rt, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	app.runtime = rt
	rt.AttachHost(app)

	app.runCommand(":lua gopdf.options.dual_page = false")
	if app.dualPage {
		t.Fatal("expected inline lua option assignment to disable dual-page mode")
	}
}

func TestRunCommandSearchAndOpenMessages(t *testing.T) {
	app := &App{}
	app.runCommand(":search needle")
	if app.message != "no document open" {
		t.Fatalf("expected search without document message, got %q", app.message)
	}

	app.runCommand(":open")
	if app.message != "usage: :open <filename>" {
		t.Fatalf("expected open usage message, got %q", app.message)
	}

	app.runCommand(":help")
	view := app.activeUIView()
	if view == nil {
		t.Fatal("expected help command to open the help window")
	}
	if view.title != "Help" {
		t.Fatalf("expected help window title, got %q", view.title)
	}
	if len(view.rows) == 0 {
		t.Fatal("expected help window rows")
	}
	if !slices.ContainsFunc(view.rows, func(row uiRow) bool {
		return row.text == ":open_file_picker" && row.secondary == "Open the PDF file picker"
	}) {
		t.Fatal("expected help rows to include open_file_picker command")
	}
}

func TestRecentFilesCommandReportsDisabledDatabase(t *testing.T) {
	app := &App{config: config.Config{SessionDatabase: false}}
	app.runCommand(":recent")
	if app.message != "session database disabled" {
		t.Fatalf("expected disabled database message, got %q", app.message)
	}
}

func TestConfigChangesKeepActionSettingsUnlessAssigned(t *testing.T) {
	rt, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	app := testLayoutApp(4)
	app.winW, app.winH = 800, 600
	app.runtime = rt
	rt.AttachHost(app)
	app.applyConfigState(rt.Config())
	app.cache.add(testTile(0, 1, 0, 0, 16))

	app.runAction("toggle_dual_page")
	app.runCommand(":set scroll_step=80")
	app.runCommand(":lua gopdf.o.status_bar_right = '{page}'")
	if !app.dualPage {
		t.Fatal("unrelated config changes undid dual-page mode set by an action")
	}
	if len(app.cache.entries) != 1 {
		t.Fatal("config changes that do not affect rendering dropped the tiles")
	}
	app.runCommand(":set dual_page=false")
	if app.dualPage {
		t.Fatal("assigning dual_page its current config value did not apply it")
	}
}

func TestSetStatusBarVisibleAppliesAfterToggle(t *testing.T) {
	rt, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	app := testLayoutApp(4)
	app.winW, app.winH = 800, 600
	app.runtime = rt
	rt.AttachHost(app)
	app.applyConfigState(rt.Config())

	app.runAction("toggle_status_bar") // hidden, while the config still says shown
	app.runCommand(":set status_bar_visible=true")
	if !app.statusBarShown {
		t.Fatal(":set status_bar_visible=true did not show the status bar")
	}
	app.runCommand(":set status_bar_visible=false")
	if app.statusBarShown {
		t.Fatal(":set status_bar_visible=false did not hide the status bar")
	}
}
