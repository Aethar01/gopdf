package viewer

import (
	"container/list"
	"testing"

	"gopdf/internal/config"
)

func listWithValues(values ...any) *list.List {
	l := list.New()
	for _, value := range values {
		l.PushBack(value)
	}
	return l
}

func TestRenderCacheEvictsByPageLimit(t *testing.T) {
	var rs renderService
	rs.cacheLimit = 2

	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 1, height: 1})
	rs.addRenderCacheEntry("b", &renderedPage{key: "b", page: 1, width: 1, height: 1})
	rs.addRenderCacheEntry("c", &renderedPage{key: "c", page: 2, width: 1, height: 1})
	rs.enforceRenderCacheLimit()

	if _, ok := rs.renderCache["a"]; ok {
		t.Fatal("oldest cache entry was not evicted")
	}
	if _, ok := rs.renderCache["b"]; !ok {
		t.Fatal("newer cache entry b was evicted")
	}
	if _, ok := rs.renderCache["c"]; !ok {
		t.Fatal("newer cache entry c was evicted")
	}
	if len(rs.renderCache) > rs.cacheLimit {
		t.Fatalf("cache entries = %d, want <= %d", len(rs.renderCache), rs.cacheLimit)
	}
}

func TestRenderCacheDisablesLimitWhenUnset(t *testing.T) {
	var rs renderService

	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 1, height: 1})
	rs.addRenderCacheEntry("b", &renderedPage{key: "b", page: 1, width: 1, height: 1})
	rs.enforceRenderCacheLimit()

	if len(rs.renderCache) != 2 {
		t.Fatalf("cache entries = %d, want 2", len(rs.renderCache))
	}
}

func TestRenderCacheBytesUpdatedOnReplacementAndRemoval(t *testing.T) {
	var rs renderService

	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 1, height: 1})
	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 2, height: 1})
	if rs.renderCacheBytes != 8 {
		t.Fatalf("cache bytes after replacement = %d, want 8", rs.renderCacheBytes)
	}

	rs.removeRenderCacheEntry("a", true)
	if rs.renderCacheBytes != 0 {
		t.Fatalf("cache bytes after removal = %d, want 0", rs.renderCacheBytes)
	}
}

func TestRenderCacheReplacesSamePageVariant(t *testing.T) {
	var rs renderService

	rs.addRenderCacheEntry("old", &renderedPage{key: "old", page: 0, scale: 1, aaLevel: 8, width: 1, height: 1})
	rs.addRenderCacheEntry("new", &renderedPage{key: "new", page: 0, scale: 2, aaLevel: 8, width: 1, height: 1})

	if _, ok := rs.renderCache["old"]; ok {
		t.Fatal("old same-page render variant was not replaced")
	}
	if _, ok := rs.renderCache["new"]; !ok {
		t.Fatal("new render variant was not cached")
	}
	if len(rs.renderCache) != 1 {
		t.Fatalf("cache entries = %d, want 1", len(rs.renderCache))
	}
}

func TestRenderCacheProtectsVisiblePagesFromEviction(t *testing.T) {
	var rs renderService
	rs.cacheLimit = 1
	rs.visibleCachePages = map[int]bool{0: true}

	rs.addRenderCacheEntry("visible", &renderedPage{key: "visible", page: 0, width: 1, height: 1})
	rs.addRenderCacheEntry("hidden", &renderedPage{key: "hidden", page: 1, width: 1, height: 1})
	rs.enforceRenderCacheLimit()

	if _, ok := rs.renderCache["visible"]; !ok {
		t.Fatal("visible page was evicted")
	}
	if _, ok := rs.renderCache["hidden"]; ok {
		t.Fatal("hidden page was not evicted")
	}
}

func TestRenderCacheCanTemporarilyExceedLimitForVisiblePages(t *testing.T) {
	var rs renderService
	rs.cacheLimit = 1
	rs.visibleCachePages = map[int]bool{0: true, 1: true}

	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 1, height: 1})
	rs.addRenderCacheEntry("b", &renderedPage{key: "b", page: 1, width: 1, height: 1})
	rs.enforceRenderCacheLimit()

	if len(rs.renderCache) != 2 {
		t.Fatalf("cache entries = %d, want 2 visible entries kept", len(rs.renderCache))
	}
}

func TestThumbnailCacheEvictsByDerivedLimit(t *testing.T) {
	var rs renderService
	rs.cacheLimit = 1

	a := renderVariantKey{page: 0}
	b := renderVariantKey{page: 1}
	c := renderVariantKey{page: 2}
	rs.thumbnailCache = map[renderVariantKey]*renderedPage{
		a: {page: 0, width: 1, height: 1, bytes: 4},
		b: {page: 1, width: 1, height: 1, bytes: 4},
		c: {page: 2, width: 1, height: 1, bytes: 4},
	}
	rs.thumbnailBytes = 12
	rs.thumbnailLRU = listWithValues(a, b, c)
	rs.thumbnailLRUItems = map[renderVariantKey]*list.Element{}
	for elem := rs.thumbnailLRU.Front(); elem != nil; elem = elem.Next() {
		rs.thumbnailLRUItems[elem.Value.(renderVariantKey)] = elem
	}

	rs.enforceThumbnailCacheLimit()

	if _, ok := rs.thumbnailCache[a]; ok {
		t.Fatal("oldest thumbnail was not evicted")
	}
	if len(rs.thumbnailCache) != 2 {
		t.Fatalf("thumbnail entries = %d, want 2", len(rs.thumbnailCache))
	}
}

func TestRequestRenderPromotesPendingRequest(t *testing.T) {
	key := renderCacheKey(0, 1, false, 8)
	app := &App{
		documentState:   documentState{pageCount: 1},
		documentWorkers: documentWorkers{renderWorker: &renderWorker{}},
		config:          config.Config{AntiAliasing: 8},
		renderService: renderService{
			renderCache:     map[string]*renderedPage{},
			renderPending:   map[string]renderRequest{key: {page: 0, priority: 10}},
			renderBaseScale: 1,
		},
	}

	if app.requestRender(0, 1, 0) {
		t.Fatal("pending request should be promoted rather than enqueued again")
	}
	if got := app.renderPending[key].priority; got != 0 {
		t.Fatalf("pending request priority = %d, want 0", got)
	}
}

func TestPrefetchVisiblePagesQueuesBoundedLookahead(t *testing.T) {
	app := testLayoutApp(20)
	app.winW = 100
	app.winH = 800
	app.cacheLimit = 16
	app.renderBaseScale = 1
	app.renderCache = map[string]*renderedPage{}
	app.renderPending = map[string]renderRequest{}
	app.renderWorker = &renderWorker{requests: make(chan renderRequest, 128)}
	app.recomputeLayout(app.viewportSize())

	app.prefetchVisiblePages()
	for key, req := range app.renderPending {
		if req.priority != 0 {
			t.Fatalf("queued background render before visible pages completed: %#v", req)
		}
		app.addRenderCacheEntry(key, &renderedPage{key: key, page: req.page, scale: req.scale, aaLevel: req.aaLevel, width: 1, height: 1})
	}
	app.renderPending = map[string]renderRequest{}
	app.renderWorker.requests = make(chan renderRequest, 128)

	app.prefetchVisiblePages()
	if got := app.pendingBackgroundRenderCount(); got != maxPendingPrefetchRenders {
		t.Fatalf("pending prefetch renders = %d, want %d", got, maxPendingPrefetchRenders)
	}
}

func TestVisibleRequestPreemptsPreviouslyVisibleRender(t *testing.T) {
	worker := &renderWorker{}
	worker.activePage.Store(1)
	app := &App{
		documentWorkers: documentWorkers{renderWorker: worker},
		renderService: renderService{
			renderPending: map[string]renderRequest{
				"old": {generation: 2, page: 0, priority: 0},
				"new": {generation: 2, page: 1, priority: 0},
			},
			renderGeneration:  2,
			visibleCachePages: map[int]bool{1: true},
		},
	}

	app.preemptNonVisibleRender()
	if _, ok := app.renderPending["old"]; ok {
		t.Fatal("preempted render remained pending")
	}
	if _, ok := app.renderPending["new"]; !ok {
		t.Fatal("visible render was removed")
	}
}
