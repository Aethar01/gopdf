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

// waitForDocumentChange polls s, as the event loop does, until the watcher
// reports a change once its debounce has passed.
func waitForDocumentChange(t *testing.T, s *documentSession) documentChange {
	t.Helper()
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		if change, ok := s.poll(time.Now()); ok {
			return change
		}
	}
	t.Fatal("no change reported after the file was modified")
	return documentChange{}
}

func testDocumentSession(t *testing.T) (*documentSession, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &documentSession{}
	s.record(path) // watches the directory before returning
	t.Cleanup(s.Close)
	return s, path
}

func TestDocumentSessionDetectsChanges(t *testing.T) {
	s, path := testDocumentSession(t)
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	s.commit(waitForDocumentChange(t, s))
	if _, ok := s.poll(time.Now()); ok {
		t.Fatal("expected no change after commit")
	}
}

func TestDocumentSessionRateLimitsRetries(t *testing.T) {
	s, path := testDocumentSession(t)
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	waitForDocumentChange(t, s)
	if _, ok := s.poll(time.Now()); ok {
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
	app := &App{documentState: documentState{page: 3}, viewStateFields: viewStateFields{scrollX: 12, scrollY: 34, zoom: 1.5, fitMode: fitManual, renderMode: renderSingle, dualPage: true, firstPageOffset: false, statusBarShown: true, altColors: true}}
	state := app.captureViewState()

	if state.page != 3 || state.scrollX != 12 || state.scrollY != 34 || state.zoom != 1.5 || state.fitMode != fitManual || state.renderMode != renderSingle || !state.dualPage || state.firstPageOffset || !state.statusBarShown || !state.altColors {
		t.Fatalf("unexpected captured view state: %#v", state)
	}
}

func TestViewStateAtDocumentStartPreservesPreferencesOnly(t *testing.T) {
	state := viewState{page: 7, scrollX: 12, scrollY: 34, zoom: 1.5, fitMode: fitManual, renderMode: renderSingle, dualPage: true, firstPageOffset: true, statusBarShown: true, altColors: true}
	state = state.atDocumentStart()

	if state.page != 0 || state.scrollX != 0 || state.scrollY != 0 {
		t.Fatalf("expected document start location, got %#v", state)
	}
	if state.zoom != 1.5 || state.fitMode != fitManual || state.renderMode != renderSingle || !state.dualPage || !state.firstPageOffset || !state.statusBarShown || !state.altColors {
		t.Fatalf("expected viewer preferences to be preserved, got %#v", state)
	}
}

func TestEscCancelsPendingMark(t *testing.T) {
	app := &App{}
	app.pendingMark = "set"
	if !app.handleMarkToken("<Esc>") {
		t.Fatal("Esc was not handled")
	}
	if app.pendingMark != "" || app.message != "" {
		t.Fatalf("pendingMark=%q message=%q, want cancelled silently", app.pendingMark, app.message)
	}
}

func TestDocumentChangesWaitWhileAutoReloadIsOff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.pdf")
	if err := os.WriteFile(path, []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	app := &App{}
	app.document.setDelay(time.Millisecond)
	app.document.record(path)
	defer app.document.Close()
	if err := os.WriteFile(path, []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	app.pollDocumentUpdate() // auto_reload is off
	if app.message != "" {
		t.Fatalf("change handled with auto_reload off: %q", app.message)
	}
	waitForDocumentChange(t, &app.document) // still pending
}
