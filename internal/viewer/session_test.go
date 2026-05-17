package viewer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDocumentSessionDebouncesChangesAndRetries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	var s documentSession
	s.record(path)
	now := s.lastStat.Add(documentStatInterval + time.Millisecond)
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, ok := s.poll(now); ok {
		t.Fatal("expected first changed stat to wait for debounce")
	}
	if _, ok := s.poll(now.Add(documentReloadDebounce / 2)); ok {
		t.Fatal("expected reload to remain debounced")
	}
	change, ok := s.poll(now.Add(documentReloadDebounce + time.Millisecond))
	if !ok {
		t.Fatal("expected debounced change to become reloadable")
	}
	if _, ok := s.poll(now.Add(documentReloadDebounce + documentReloadRetry/2)); ok {
		t.Fatal("expected failed reload attempts to be rate limited")
	}
	s.commit(change)
	if _, ok := s.poll(now.Add(documentReloadDebounce + documentReloadRetry + documentStatInterval)); ok {
		t.Fatal("expected committed change not to reload again")
	}
}

func TestViewStateRoundTrip(t *testing.T) {
	app := &App{page: 3, scrollX: 12, scrollY: 34, zoom: 1.5, fitMode: "manual", renderMode: "single", dualPage: true, firstPageOffset: false, statusBarShown: true, altColors: true}
	state := app.captureViewState()

	if state.page != 3 || state.scrollX != 12 || state.scrollY != 34 || state.zoom != 1.5 || state.fitMode != "manual" || state.renderMode != "single" || !state.dualPage || state.firstPageOffset || !state.statusBarShown || !state.altColors {
		t.Fatalf("unexpected captured view state: %#v", state)
	}
}

func TestViewStateAtDocumentStartPreservesPreferencesOnly(t *testing.T) {
	state := viewState{page: 7, scrollX: 12, scrollY: 34, zoom: 1.5, fitMode: "manual", renderMode: "single", dualPage: true, firstPageOffset: true, statusBarShown: true, altColors: true}
	state = state.atDocumentStart()

	if state.page != 0 || state.scrollX != 0 || state.scrollY != 0 {
		t.Fatalf("expected document start location, got %#v", state)
	}
	if state.zoom != 1.5 || state.fitMode != "manual" || state.renderMode != "single" || !state.dualPage || !state.firstPageOffset || !state.statusBarShown || !state.altColors {
		t.Fatalf("expected viewer preferences to be preserved, got %#v", state)
	}
}
