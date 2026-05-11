package viewer

import (
	"math"
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

func TestRecomputeLayoutUsesRotatedPageDimensions(t *testing.T) {
	width, height := rotatedBoundsSize(mupdf.Rect{X1: 100, Y1: 200}, 90)
	assertClose(t, width, 200)
	assertClose(t, height, 100)
	metrics := make([]pageMetrics, 5)
	for i := range metrics {
		metrics[i] = pageMetrics{bounds: mupdf.Rect{X1: 100, Y1: 200}, width: width, height: height}
	}

	app := &App{
		pageCount:       5,
		page:            0,
		zoom:            1,
		fitMode:         "manual",
		renderMode:      "continuous",
		dualPage:        true,
		firstPageOffset: true,
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1},
		pageMetrics:     metrics,
	}

	app.recomputeLayout(1000, 1000)

	if len(app.rows) != 3 {
		t.Fatalf("expected first offset row plus two spreads, got %d rows", len(app.rows))
	}
	assertClose(t, app.rows[0].width, 200)
	assertClose(t, app.rows[0].height, 100)
	assertClose(t, app.rows[1].width, 400)
	assertClose(t, app.rows[1].height, 100)
	assertClose(t, app.rows[0].x, 100)
	assertClose(t, app.rows[1].x, 0)
}

func TestSetRotationKeepsCurrentPage(t *testing.T) {
	app := testLayoutApp(5)
	app.renderMode = "continuous"
	app.page = 3

	if err := app.SetRotation(90); err != nil {
		t.Fatal(err)
	}

	if app.page != 3 {
		t.Fatalf("expected rotation to keep page 3, got %d", app.page)
	}
}

func TestSetRenderModeKeepsCurrentPage(t *testing.T) {
	app := testLayoutApp(5)
	app.renderMode = "single"
	app.page = 3
	app.recomputeLayout(1000, 1000)

	if err := app.SetRenderMode("continuous"); err != nil {
		t.Fatal(err)
	}

	if app.page != 3 {
		t.Fatalf("expected render mode change to keep page 3, got %d", app.page)
	}
}

func testLayoutApp(pageCount int) *App {
	metrics := make([]pageMetrics, pageCount)
	for i := range metrics {
		metrics[i] = pageMetrics{bounds: mupdf.Rect{X1: 100, Y1: 200}, width: 100, height: 200}
	}
	return &App{
		pageCount:       pageCount,
		zoom:            1,
		fitMode:         "manual",
		renderMode:      "continuous",
		firstPageOffset: true,
		config:          config.Config{PageGap: -1, PageGapHorizontal: -1, PageGapVertical: -1, SpreadGap: -1},
		pageMetrics:     metrics,
	}
}

func assertClose(t *testing.T, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.001 {
		t.Fatalf("got %.3f, want %.3f", got, want)
	}
}
