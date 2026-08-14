package viewer

import (
	"encoding/binary"
	"math"
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

func TestHandleSDLEventRedrawsExposedWindow(t *testing.T) {
	app := &App{}
	event := sdl.Event{}
	binary.NativeEndian.PutUint32(event[:], uint32(sdl.EventWindowExposed))

	if err := app.handleSDLEvent(&event); err != nil {
		t.Fatalf("handle window exposed event: %v", err)
	}
	if !app.pendingRedraw {
		t.Fatal("expected window exposed event to request a redraw")
	}
}

func TestHandleSDLEventPinchUpdatesZoom(t *testing.T) {
	app := &App{
		config:          config.Config{PinchSensitivity: 2, MinZoom: 0.5, MaxZoom: 8},
		viewStateFields: viewStateFields{zoom: 2, fitMode: "manual"},
	}
	begin := sdl.Event{}
	binary.NativeEndian.PutUint32(begin[:], uint32(sdl.EventPinchBegin))
	if err := app.handleSDLEvent(&begin); err != nil {
		t.Fatalf("handle pinch begin: %v", err)
	}
	event := sdl.Event{}
	binary.NativeEndian.PutUint32(event[:], uint32(sdl.EventPinchUpdate))
	binary.NativeEndian.PutUint32(event[16:], math.Float32bits(1.25))

	if err := app.handleSDLEvent(&event); err != nil {
		t.Fatalf("handle pinch event: %v", err)
	}
	if app.pinchVisualScale() <= 1 || app.pinchVisualScale() >= 1.5625 {
		t.Fatalf("expected pinch update to smoothly scale visually between 1 and 1.5625, got %v", app.pinchVisualScale())
	}
	if app.zoom != 2 || app.fitMode != "manual" {
		t.Fatalf("expected pinch update to leave committed zoom unchanged, got zoom=%v mode=%q", app.zoom, app.fitMode)
	}
	end := sdl.Event{}
	binary.NativeEndian.PutUint32(end[:], uint32(sdl.EventPinchEnd))
	if err := app.handleSDLEvent(&end); err != nil {
		t.Fatalf("handle pinch end: %v", err)
	}
	if app.zoom != 3.125 {
		t.Fatalf("expected pinch end to commit zoom 3.125, got %v", app.zoom)
	}
	if app.pinchActive {
		t.Fatal("expected pinch visual transform to end")
	}
}

func TestHandleSDLEventPinchOutDoesNotReverseDirection(t *testing.T) {
	app := &App{config: config.Config{MinZoom: 0.5, MaxZoom: 8}, viewStateFields: viewStateFields{zoom: 2, fitMode: "manual"}}
	for _, scale := range []float32{0.98, 0.99, 0.97} {
		event := sdl.Event{}
		binary.NativeEndian.PutUint32(event[:], uint32(sdl.EventPinchUpdate))
		binary.NativeEndian.PutUint32(event[16:], math.Float32bits(scale))
		if err := app.handleSDLEvent(&event); err != nil {
			t.Fatalf("handle pinch update: %v", err)
		}
	}
	if app.pinchVisualScale() >= 1 {
		t.Fatalf("expected pinch out to reduce visual scale, got %v", app.pinchVisualScale())
	}
}
