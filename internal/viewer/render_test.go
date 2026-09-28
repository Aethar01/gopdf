package viewer

import (
	"image"
	"slices"
	"testing"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"
)

func testTile(page int, scale float64, x, y, size int) *renderedTile {
	return &renderedTile{key: tileKey{page: page, scale: scale, x: x, y: y}, rect: image.Rect(0, 0, size, size), scale: scale}
}

func TestTileCacheEvictsLeastRecentlyUsedByMemory(t *testing.T) {
	cfg := config.Default()
	cfg.PageCacheMemoryMB = 2
	c := tileCache{byteLimit: pageCacheByteLimit(cfg)}
	// Each 512x512 RGBA tile is exactly 1 MiB.
	a, b, d := testTile(0, 1, 0, 0, 512), testTile(1, 1, 0, 0, 512), testTile(2, 1, 0, 0, 512)
	c.add(a)
	c.add(b)
	c.get(a.key) // a is now more recent than b
	c.add(d)
	c.evict()

	if _, ok := c.entries[b.key]; ok {
		t.Fatal("least recently used tile kept past the memory limit")
	}
	if _, ok := c.entries[a.key]; !ok {
		t.Fatal("recently used tile was evicted")
	}
	if c.bytes != 2<<20 {
		t.Fatalf("cache bytes = %d, want %d", c.bytes, 2<<20)
	}
}

func TestTileCacheKeepsProtectedTilesOverLimit(t *testing.T) {
	c := tileCache{byteLimit: 1 << 20}
	a, b := testTile(0, 1, 0, 0, 512), testTile(1, 1, 0, 0, 512)
	c.add(a)
	c.add(b)
	c.protected = map[tileKey]bool{a.key: true, b.key: true}
	c.evict()
	if len(c.entries) != 2 {
		t.Fatalf("entries = %d, want both protected tiles kept", len(c.entries))
	}
}

func TestTileCacheReplacesAndRemovesByKey(t *testing.T) {
	var c tileCache // the zero value is ready to use
	c.add(testTile(0, 1, 0, 0, 512))
	c.add(testTile(0, 1, 0, 0, 256))
	if len(c.entries) != 1 || c.bytes != 256*256*4 {
		t.Fatalf("after replace: entries=%d bytes=%d", len(c.entries), c.bytes)
	}
	c.clear()
	if len(c.entries) != 0 || len(c.byPage) != 0 || c.bytes != 0 || c.lru.Len() != 0 {
		t.Fatalf("after clear: entries=%d pages=%d bytes=%d lru=%d", len(c.entries), len(c.byPage), c.bytes, c.lru.Len())
	}
}

func TestPageTilesDrawSharpestCurrentTilesLast(t *testing.T) {
	var c tileCache
	current := testTile(0, 2, 0, 0, 8)
	sharper := testTile(0, 4, 0, 0, 8)
	blurrier := testTile(0, 1, 0, 0, 8)
	thumb := &renderedTile{key: thumbnailKey(0), rect: image.Rect(0, 0, 8, 8), scale: 0.5}
	for _, tile := range []*renderedTile{current, sharper, thumb, blurrier, testTile(1, 2, 0, 0, 8)} {
		c.add(tile)
	}
	got := c.pageTiles(0, 2)
	if want := []*renderedTile{thumb, blurrier, sharper, current}; !slices.Equal(got, want) {
		t.Fatalf("draw order = %v, want thumbnail, other scales ascending, then current", got)
	}
}

func TestTilesCoveringClipsToPage(t *testing.T) {
	page := image.Rect(10, 20, 2510, 1120) // 2500x1100: 3x2 tiles
	if got := tilesCovering(page, image.Rect(-100, -100, 5000, 5000)); len(got) != 6 {
		t.Fatalf("whole page covered by %d tiles, want 6", len(got))
	}
	got := tilesCovering(page, image.Rect(1100, 30, 1200, 40))
	if want := []image.Point{{1, 0}}; !slices.Equal(got, want) {
		t.Fatalf("tiles = %v, want %v", got, want)
	}
	if r := tileRect(page, 2, 1); r != image.Rect(2058, 1044, 2510, 1120) {
		t.Fatalf("edge tile rect = %v", r)
	}
	if got := tilesCovering(page, image.Rect(3000, 3000, 3100, 3100)); got != nil {
		t.Fatalf("area off the page covered by %v", got)
	}
}

func TestRequestTilePromotesPendingRequest(t *testing.T) {
	key := tileKey{page: 0, scale: 1}
	app := &App{
		documentWorkers: documentWorkers{renderWorker: &renderWorker{}},
		renderService:   renderService{renderPending: map[tileKey]renderRequest{key: {key: key, priority: 10}}},
	}
	if app.requestTile(key, image.Rect(0, 0, 1, 1), 0) {
		t.Fatal("pending request should be promoted rather than enqueued again")
	}
	if got := app.renderPending[key].priority; got != 0 {
		t.Fatalf("pending request priority = %d, want 0", got)
	}
}

func testPrefetchApp(pages int, zoom float64) *App {
	app := testLayoutApp(pages)
	app.winW, app.winH = 1000, 800
	app.zoom = zoom
	app.renderBaseScale = zoom
	app.renderPending = map[tileKey]renderRequest{}
	app.renderWorker = &renderWorker{requests: make(chan renderRequest, 128)}
	app.recomputeLayout(app.viewportSize())
	return app
}

func TestPrefetchQueuesVisibleTilesBeforeLookahead(t *testing.T) {
	app := testPrefetchApp(20, 1)
	app.pageStep = 400 // widens the prefetch margin to several pages
	app.prefetchVisiblePages()
	if len(app.renderPending) == 0 {
		t.Fatal("no visible tiles requested")
	}
	for key, req := range app.renderPending {
		if req.priority != 0 {
			t.Fatalf("queued background render before visible tiles completed: %#v", req)
		}
		app.cache.add(&renderedTile{key: key, rect: req.rect, scale: key.scale})
	}
	app.renderPending = map[tileKey]renderRequest{}
	app.renderWorker.requests = make(chan renderRequest, 128)

	app.prefetchVisiblePages()
	if got := app.pendingBackgroundRenderCount(); got != maxPendingPrefetchRenders {
		t.Fatalf("pending prefetch renders = %d, want %d", got, maxPendingPrefetchRenders)
	}
}

func TestPrefetchAtHighZoomRequestsBoundedTiles(t *testing.T) {
	app := testPrefetchApp(3, 40) // a 100x200pt page becomes 4000x8000px
	app.prefetchVisiblePages()
	if len(app.renderPending) == 0 {
		t.Fatal("no tiles requested")
	}
	for key, req := range app.renderPending {
		if req.rect.Dx() > renderTileSize || req.rect.Dy() > renderTileSize {
			t.Fatalf("tile %v is %v, larger than %d", key, req.rect.Size(), renderTileSize)
		}
		if key.scale != 40 {
			t.Fatalf("tile %v not at the render scale", key)
		}
	}
	// A 1000x800 viewport needs at most 2x2 tiles, plus one per axis
	// when they straddle tile boundaries.
	if n := len(app.renderPending); n > 9 {
		t.Fatalf("requested %d visible tiles for a 1000x800 viewport", n)
	}
}

func TestVisibleRequestPreemptsPreviouslyVisibleRender(t *testing.T) {
	old, visible := tileKey{page: 0}, tileKey{page: 1}
	worker := &renderWorker{slots: []*renderSlot{{}}}
	worker.slots[0].rendering.Store(&old)
	app := &App{
		documentWorkers: documentWorkers{renderWorker: worker},
		renderService: renderService{
			renderPending: map[tileKey]renderRequest{
				old:     {generation: 2, key: old},
				visible: {generation: 2, key: visible},
			},
			renderGeneration: 2,
		},
	}

	app.preemptNonVisibleRender(map[tileKey]bool{visible: true})
	if _, ok := app.renderPending[old]; ok {
		t.Fatal("preempted render remained pending")
	}
	if _, ok := app.renderPending[visible]; !ok {
		t.Fatal("visible render was removed")
	}
}

func TestRenderWorkerPoolRendersEveryRequest(t *testing.T) {
	pages := make([][]string, 6)
	for i := range pages {
		pages[i] = []string{"page"}
	}
	doc, err := mupdf.Open(testpdf.WritePages(t, pages...), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	w := newRenderWorker(doc, 3)
	defer w.Close()
	if len(w.slots) != 3 {
		t.Fatalf("slots = %d, want 3", len(w.slots))
	}
	for page := range pages {
		if !w.Enqueue(renderRequest{key: tileKey{page: page, scale: 0.5}, rect: image.Rect(0, 0, renderTileSize, renderTileSize)}) {
			t.Fatalf("enqueue page %d failed", page)
		}
	}
	seen := map[int]bool{}
	timeout := time.After(5 * time.Second)
	for len(seen) < len(pages) {
		select {
		case update := <-w.updates:
			if update.err != nil {
				t.Fatal(update.err)
			}
			update.rendered.Close()
			if seen[update.request.key.page] {
				t.Fatalf("page %d rendered twice", update.request.key.page)
			}
			seen[update.request.key.page] = true
		case <-timeout:
			t.Fatalf("rendered %d of %d pages", len(seen), len(pages))
		}
	}
}

func TestCancelNotVisibleChecksEverySlot(t *testing.T) {
	visible, offscreen := tileKey{page: 0}, tileKey{page: 4}
	w := &renderWorker{slots: []*renderSlot{{}, {}, {}}}
	w.slots[0].rendering.Store(&visible)
	w.slots[1].rendering.Store(&offscreen)
	if got := w.CancelNotVisible(map[tileKey]bool{visible: true}); !slices.Equal(got, []tileKey{offscreen}) {
		t.Fatalf("cancelled = %v, want [%v]", got, offscreen)
	}
}
