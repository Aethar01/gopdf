package viewer

import (
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func testOverviewApp() *App {
	app := testLayoutApp(20)
	app.winW, app.winH = 1000, 800
	app.fitMode, app.renderMode = "page", "single"
	app.page = 5
	app.recomputeLayout(app.viewportSize())
	return app
}

func TestOverviewGridNavigationAndConfirm(t *testing.T) {
	app := testOverviewApp()
	app.toggleOverview()
	columns := app.overviewColumns()
	if columns != 1000/(overviewThumbWidth+overviewGap) {
		t.Fatalf("columns = %d", columns)
	}
	if len(app.rows[0].pages) != columns || app.renderMode != "continuous" {
		t.Fatalf("first row has %d pages, mode %q", len(app.rows[0].pages), app.renderMode)
	}
	if state := app.captureViewState(); state.renderMode != "single" || state.page != 5 {
		t.Fatalf("persisted state = %+v, want the view from before the overview", state)
	}

	app.runAction("scroll_down") // one row down
	app.runAction("scroll_right")
	if want := 5 + columns + 1; app.overview.selected != want {
		t.Fatalf("selected = %d, want %d", app.overview.selected, want)
	}
	app.runAction("zoom_out") // one more column
	if app.overviewColumns() != columns+1 {
		t.Fatalf("zoom_out left %d columns", app.overviewColumns())
	}

	app.runAction("confirm")
	if app.overview != nil || app.renderMode != "single" || app.fitMode != "page" {
		t.Fatalf("after confirm: overview=%v mode=%q fit=%q", app.overview, app.renderMode, app.fitMode)
	}
	if app.page != 5+columns+1 {
		t.Fatalf("page = %d, want the selected page", app.page)
	}
}

func TestOverviewCloseReturnsToOriginalPage(t *testing.T) {
	app := testOverviewApp()
	app.toggleOverview()
	app.runAction("last_page")
	app.runAction("close")
	if app.overview != nil || app.page != 5 {
		t.Fatalf("after close: overview=%v page=%d, want page 5", app.overview, app.page)
	}
}

func TestOverviewPagesByScreenAndKeepsScrollOff(t *testing.T) {
	app := testLayoutApp(200)
	app.winW, app.winH = 1000, 2400 // several rows of the tall test pages
	app.config.ScrollOff = 1
	app.recomputeLayout(app.viewportSize())
	app.toggleOverview()
	columns, rows := app.overviewColumns(), app.overviewVisibleRows()
	if rows < 2 {
		t.Fatalf("visible rows = %d", rows)
	}
	app.runAction("next_page")
	if app.overview.selected != app.overview.saved.page+rows*columns {
		t.Fatalf("PgDn moved to %d, want %d", app.overview.selected, app.overview.saved.page+rows*columns)
	}
	// The row after the selection stays on screen.
	next := app.rows[app.pageToRow[app.overview.selected]+1]
	_, viewportH := app.viewportSize()
	if next.y+next.height > app.scrollY+float64(viewportH) {
		t.Fatalf("scroll_off row below the selection is off screen")
	}
	app.runAction("prev_page")
	if app.overview.selected != app.overview.saved.page {
		t.Fatalf("PgUp moved to %d", app.overview.selected)
	}
}

func TestOverviewLargeScrollOffCentresSelection(t *testing.T) {
	app := testLayoutApp(200)
	app.winW, app.winH = 1000, 1500
	app.config.ScrollOff = 8
	app.recomputeLayout(app.viewportSize())
	app.toggleOverview()
	rows := app.overviewVisibleRows()
	if rows < 3 {
		t.Fatalf("visible rows = %d, want at least 3 for the test", rows)
	}
	for range 10 {
		app.runAction("scroll_down")
	}
	first := app.rowIndexAtContentY(app.scrollY + overviewGap)
	if got, want := app.pageToRow[app.overview.selected]-first, (rows-1)/2; got != want {
		t.Fatalf("selected row is %d rows below the top, want %d (centred)", got, want)
	}
}

func TestPromptOverOverviewReceivesEnter(t *testing.T) {
	app := testOverviewApp()
	app.applyConfigState(config.Default(), false) // default key bindings
	app.toggleOverview()
	app.runAction("command_mode")
	app.input.Set("colors alt")
	app.handleSDLKeyDown(&sdl.KeyboardEvent{Type: sdl.EventKeyDown, Key: sdl.KeycodeReturn})
	if app.mode != modeNormal || app.overview == nil {
		t.Fatalf("Enter went to the overview: mode=%v overview=%v", app.mode, app.overview != nil)
	}
	if !app.altColors {
		t.Fatal("command not run")
	}
}

func TestLeavingOverviewRestoresRenderScaleAtOnce(t *testing.T) {
	app := testOverviewApp()
	app.fitMode = "width" // scale 10, well above the overview's thumbnails
	app.recomputeLayout(app.viewportSize())
	app.config.RenderOversample = 1
	app.ensureRenderBaseScale()
	before := app.renderBaseScale
	app.toggleOverview()
	if app.renderBaseScale >= before {
		t.Fatalf("overview kept render scale %.3f (was %.3f)", app.renderBaseScale, before)
	}
	app.runAction("confirm")
	if app.renderBaseScale < app.scale*renderUpgradeTolerance {
		t.Fatalf("render scale %.3f after leaving the overview, view scale %.3f", app.renderBaseScale, app.scale)
	}
}

func TestOverviewGridFitsWidthAndAlignsLastRow(t *testing.T) {
	app := testLayoutApp(13)         // 5 columns leave a short last row
	for i := range app.pageMetrics { // Letter pages, so thumbnails are below 1x
		app.pageMetrics[i] = newPageMetrics(mupdf.PageInfo{Bounds: mupdf.Rect{X1: 612, Y1: 792}})
	}
	app.winW, app.winH = 1300, 800
	app.recomputeLayout(app.viewportSize())
	app.toggleOverview()
	app.overview.columns = 5
	app.relayoutOverview()
	viewportW, _ := app.viewportSize()
	for i, row := range app.rows {
		last := len(row.pages) - 1
		if right := row.pageX[last] + row.pageW[last] + overviewGap; right > float64(viewportW)+0.5 {
			t.Fatalf("row %d ends at %.1f, past the %dpx viewport", i, right, viewportW)
		}
	}
	first, lastRow := app.rows[0], app.rows[len(app.rows)-1]
	if len(lastRow.pages) == 5 || lastRow.pageX[0] != first.pageX[0] {
		t.Fatalf("last row starts at %.1f, first row at %.1f", lastRow.pageX[0], first.pageX[0])
	}
}
