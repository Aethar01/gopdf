package viewer

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestDocumentSessionPollIsNonBlocking(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	var s documentSession
	s.record(path)
	defer s.Close()

	start := time.Now()
	if _, ok := s.poll(start); ok {
		t.Fatal("expected no change without modification")
	}
	if elapsed := time.Since(start); elapsed > 20*time.Millisecond {
		t.Fatalf("poll blocked for %s", elapsed)
	}
}

func TestDocumentSessionDetectsChanges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	var s documentSession
	s.record(path)
	defer s.Close()

	// Give the goroutine time to start watching
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Wait for debounce + poll timeout
	time.Sleep(documentReloadDebounce + 200*time.Millisecond)

	change, ok := s.poll(time.Now())
	if !ok {
		t.Fatal("expected a change after file modification")
	}

	// Committing should clear the pending state
	s.commit(change)
	time.Sleep(50 * time.Millisecond)

	if _, ok := s.poll(time.Now()); ok {
		t.Fatal("expected no change after commit")
	}
}

func TestDocumentSessionRateLimitsRetries(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	var s documentSession
	s.record(path)
	defer s.Close()

	time.Sleep(50 * time.Millisecond)

	// Modify and wait for debounce
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(documentReloadDebounce + 200*time.Millisecond)

	now := time.Now()
	_, ok := s.poll(now)
	if !ok {
		t.Fatal("expected a change")
	}

	// Immediately polling again should be rate limited
	if _, ok := s.poll(now); ok {
		t.Fatal("expected rate limiting on consecutive polls")
	}
}

func TestDocumentSessionNoChangeWithoutModification(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}

	var s documentSession
	s.record(path)
	defer s.Close()

	time.Sleep(50 * time.Millisecond)

	if _, ok := s.poll(time.Now()); ok {
		t.Fatal("expected no change without modification")
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
