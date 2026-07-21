package viewer

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestOpenInitialDocumentDisplaysOpenError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.pdf")
	runtime, err := config.Open(filepath.Join(t.TempDir(), "missing.lua"), path)
	if err != nil {
		t.Fatal(err)
	}
	defer runtime.Close()

	app, err := New(path, runtime, 0, nil, NewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()

	if err := app.openInitialDocument(); err != nil {
		t.Fatalf("initial open should not close the viewer: %v", err)
	}
	if app.doc != nil {
		t.Fatal("expected the viewer to remain blank")
	}
	if !strings.Contains(app.message, "open document") || !strings.Contains(app.message, path) {
		t.Fatalf("expected document open error in status message, got %q", app.message)
	}
}

func TestHandleSDLEventTracksSystemFullscreenChanges(t *testing.T) {
	app := &App{}

	enter := sdl.Event{}
	binary.NativeEndian.PutUint32(enter[:], uint32(sdl.EventWindowEnterFullscreen))
	if err := app.handleSDLEvent(&enter); err != nil {
		t.Fatalf("handle enter fullscreen event: %v", err)
	}
	if !app.Fullscreen() {
		t.Fatal("expected system fullscreen event to update app state")
	}

	if err := app.ExecuteAction("toggle_fullscreen"); err != nil {
		t.Fatalf("toggle fullscreen: %v", err)
	}
	if app.Fullscreen() {
		t.Fatal("expected app toggle to leave fullscreen after system entered it")
	}

	app.fullscreen = true
	leave := sdl.Event{}
	binary.NativeEndian.PutUint32(leave[:], uint32(sdl.EventWindowLeaveFullscreen))
	if err := app.handleSDLEvent(&leave); err != nil {
		t.Fatalf("handle leave fullscreen event: %v", err)
	}
	if app.Fullscreen() {
		t.Fatal("expected system leave fullscreen event to update app state")
	}
}
