package viewer

import "testing"

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
