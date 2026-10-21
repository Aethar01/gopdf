package viewer

import (
	"math"
	"testing"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func touchTestApp(t *testing.T) *App {
	t.Helper()
	old := touchIsScreen
	t.Cleanup(func() { touchIsScreen = old })
	touchIsScreen = func(sdl.TouchID) bool { return true }
	app := testLayoutApp(10)
	app.recomputeLayout(400, 400)
	app.winW, app.winH = 400, 400
	return app
}

// finger sends a finger event at x, y in render pixels.
func finger(app *App, kind sdl.EventType, id sdl.FingerID, x, y float32) {
	app.handleTouch(kind, &sdl.TouchFingerEvent{FingerID: id, X: x / float32(app.winW), Y: y / float32(app.winH)})
}

func TestTouchDragScrollsWithTheFinger(t *testing.T) {
	app := touchTestApp(t)
	finger(app, sdl.EventFingerDown, 1, 200, 300)
	finger(app, sdl.EventFingerMotion, 1, 200, 280)
	finger(app, sdl.EventFingerMotion, 1, 200, 200)
	if app.scrollY != 100 {
		t.Fatalf("dragging up 100 pixels scrolled to %v", app.scrollY)
	}
	finger(app, sdl.EventFingerUp, 1, 200, 200)
}

func TestTouchTapClicksWithoutScrolling(t *testing.T) {
	app := touchTestApp(t)
	finger(app, sdl.EventFingerDown, 1, 120, 140)
	finger(app, sdl.EventFingerMotion, 1, 123, 142) // within the slop
	finger(app, sdl.EventFingerUp, 1, 123, 142)
	if app.scrollY != 0 {
		t.Fatalf("a tap scrolled to %v", app.scrollY)
	}
	if math.Abs(float64(app.pointer.X-120)) > 0.01 || math.Abs(float64(app.pointer.Y-140)) > 0.01 {
		t.Fatalf("a tap clicked at %v, want where the finger came down", app.pointer)
	}
}

func TestTouchPinchZooms(t *testing.T) {
	app := touchTestApp(t)
	app.config.MinZoom, app.config.MaxZoom = 0.1, 8
	start := math.Log(app.displayedZoom())
	finger(app, sdl.EventFingerDown, 1, 150, 200)
	finger(app, sdl.EventFingerDown, 2, 250, 200)
	finger(app, sdl.EventFingerMotion, 2, 350, 200)
	if app.smoothZoom == nil || math.Abs(app.smoothZoom.targetLog-start-math.Log(2)) > 1e-6 {
		t.Fatalf("spreading two fingers twice as far apart zooms to %+v", app.smoothZoom)
	}
	finger(app, sdl.EventFingerUp, 2, 350, 200)
	finger(app, sdl.EventFingerUp, 1, 150, 200)
	if len(app.touch.order) != 0 || app.touch.pinch {
		t.Fatalf("fingers still held after lifting: %+v", app.touch)
	}
}

func TestTouchIgnoresTrackpads(t *testing.T) {
	app := touchTestApp(t)
	touchIsScreen = func(sdl.TouchID) bool { return false }
	finger(app, sdl.EventFingerDown, 1, 200, 300)
	finger(app, sdl.EventFingerMotion, 1, 200, 100)
	if app.scrollY != 0 || len(app.touch.order) != 0 {
		t.Fatalf("a trackpad finger scrolled to %v", app.scrollY)
	}
}
