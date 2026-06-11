package viewer

import (
	"container/list"
	"testing"
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
