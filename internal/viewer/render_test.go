package viewer

import "testing"

func TestRenderCacheEvictsByByteLimit(t *testing.T) {
	var rs renderService
	rs.cacheByteLimit = 10

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
	if rs.renderCacheBytes > rs.cacheByteLimit {
		t.Fatalf("cache bytes = %d, want <= %d", rs.renderCacheBytes, rs.cacheByteLimit)
	}
}

func TestRenderCacheKeepsSingleOversizedEntry(t *testing.T) {
	var rs renderService
	rs.cacheByteLimit = 10

	rs.addRenderCacheEntry("a", &renderedPage{key: "a", page: 0, width: 2, height: 2})
	rs.enforceRenderCacheLimit()

	if _, ok := rs.renderCache["a"]; !ok {
		t.Fatal("single oversized cache entry was evicted")
	}
	if rs.renderCacheBytes != 16 {
		t.Fatalf("cache bytes = %d, want 16", rs.renderCacheBytes)
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
