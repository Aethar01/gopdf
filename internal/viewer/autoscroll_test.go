package viewer

import (
	"math"
	"testing"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// testAutoscrollApp is a window of pages narrower than it and taller.
func testAutoscrollApp() *App {
	app := testLayoutApp(5)
	app.winW, app.winH = 1000, 100
	app.config.AutoscrollSpeedFactor = 1
	app.config.AutoscrollMaxSpeed = 20000
	app.recomputeLayout(app.viewportSize())
	return app
}

func startTestAutoscroll(t *testing.T, app *App, x, y float32) {
	t.Helper()
	app.pointer = sdl.FPoint{X: x, Y: y}
	app.mouseButton = uint8(sdl.ButtonMiddle)
	if err := app.runBuiltinAction("autoscroll"); err != nil {
		t.Fatal(err)
	}
	app.mouseButton = 0
	if app.autoscroll == nil {
		t.Fatal("expected autoscroll to start")
	}
}

func middleButton(eventType sdl.EventType) *sdl.MouseButtonEvent {
	return &sdl.MouseButtonEvent{Type: eventType, Button: uint8(sdl.ButtonMiddle)}
}

func TestAutoscrollSpeedGrowsPastDeadZone(t *testing.T) {
	if got := autoscrollSpeed(autoscrollDeadZone, 1, 0); got != 0 {
		t.Fatalf("expected no speed inside the dead zone, got %v", got)
	}
	near, far := autoscrollSpeed(autoscrollDeadZone+20, 1, 0), autoscrollSpeed(autoscrollDeadZone+80, 1, 0)
	if near <= 0 || far <= 4*near {
		t.Fatalf("expected speed to grow faster than distance, near=%v far=%v", near, far)
	}
	if got := autoscrollSpeed(-autoscrollDeadZone-20, 1, 0); got != -near {
		t.Fatalf("expected upward speed %v, got %v", -near, got)
	}
	if got := autoscrollSpeed(autoscrollDeadZone+20, 3, 0); math.Abs(got-3*near) > 1e-9 {
		t.Fatalf("expected the factor to scale the speed to %v, got %v", 3*near, got)
	}
	if got := autoscrollSpeed(-1e6, 1, 5000); got != -5000 {
		t.Fatalf("expected speed capped at 5000, got %v", got)
	}
	if got := autoscrollSpeed(1e6, 1, 0); got <= 5000 {
		t.Fatalf("expected no cap with a zero max speed, got %v", got)
	}
}

func TestAutoscrollScrollsTowardPointer(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	app.pointer = sdl.FPoint{X: 505, Y: 55}
	if app.advanceAutoscrollBy(100 * time.Millisecond) {
		t.Fatal("expected the dead zone to hold the view still")
	}
	assertClose(t, app.scrollY, 0)

	app.pointer = sdl.FPoint{X: 700, Y: 50 + autoscrollDeadZone + 50}
	if !app.advanceAutoscrollBy(100 * time.Millisecond) {
		t.Fatal("expected the pointer below the anchor to scroll")
	}
	// The pages are narrower than the view, so only the vertical offset counts.
	assertClose(t, app.scrollX, 0)
	assertClose(t, app.scrollY, autoscrollGain*math.Pow(50, autoscrollExponent)/10)
	if got := app.autoscrollCursor(); got != cursorAutoscrollS {
		t.Fatalf("expected the south cursor, got %v", got)
	}
}

func TestAutoscrollSpeedOptionsShapeVelocity(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)
	app.pointer = sdl.FPoint{X: 500, Y: 50 + autoscrollDeadZone + 50}
	_, base := app.autoscrollVelocity()

	app.config.AutoscrollSpeedFactor = 2.5
	if _, got := app.autoscrollVelocity(); math.Abs(got-2.5*base) > 1e-9 {
		t.Fatalf("expected speed %v, got %v", 2.5*base, got)
	}
	app.config.AutoscrollMaxSpeed = base
	if _, got := app.autoscrollVelocity(); got != base {
		t.Fatalf("expected speed capped at %v, got %v", base, got)
	}
}

func TestAutoscrollReleaseAfterDragStops(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	app.pointer = sdl.FPoint{X: 500, Y: 90}
	if !app.handleAutoscrollButton(middleButton(sdl.EventMouseButtonUp)) {
		t.Fatal("expected autoscroll to take the release")
	}
	if app.autoscroll != nil {
		t.Fatal("expected releasing after a drag to stop autoscroll")
	}
}

func TestAutoscrollClickWithoutMovingSticksUntilNextClick(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	app.handleAutoscrollButton(middleButton(sdl.EventMouseButtonUp))
	if app.autoscroll == nil || !app.autoscroll.sticky {
		t.Fatal("expected a release inside the dead zone to leave autoscroll running")
	}
	// Moving out after the release does not end it.
	app.pointer = sdl.FPoint{X: 500, Y: 200}
	app.handleSDLMouseMotion(&sdl.MouseMotionEvent{X: 500, Y: 200})
	if app.autoscroll == nil {
		t.Fatal("expected sticky autoscroll to survive pointer motion")
	}

	left := func(eventType sdl.EventType) *sdl.MouseButtonEvent {
		return &sdl.MouseButtonEvent{Type: eventType, Button: uint8(sdl.ButtonLeft)}
	}
	if !app.handleAutoscrollButton(left(sdl.EventMouseButtonDown)) || app.autoscroll != nil {
		t.Fatal("expected the next click to stop autoscroll and be consumed")
	}
	if !app.handleAutoscrollButton(left(sdl.EventMouseButtonUp)) {
		t.Fatal("expected the release of the stopping click to be consumed")
	}
	if app.handleAutoscrollButton(left(sdl.EventMouseButtonDown)) {
		t.Fatal("expected later clicks to pass through")
	}
}

func TestAutoscrollKeyPressStops(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	if !app.handleAutoscrollKey(&sdl.KeyboardEvent{Key: sdl.KeycodeJ, Repeat: true}) || app.autoscroll == nil {
		t.Fatal("expected a key repeat to be consumed without stopping autoscroll")
	}
	if !app.handleAutoscrollKey(&sdl.KeyboardEvent{Key: sdl.KeycodeEscape}) || app.autoscroll != nil {
		t.Fatal("expected a key press to stop autoscroll and be consumed")
	}
	if app.handleAutoscrollKey(&sdl.KeyboardEvent{Key: sdl.KeycodeJ}) {
		t.Fatal("expected keys to pass through once autoscroll stopped")
	}
}

func TestAutoscrollCanBeHeldByKey(t *testing.T) {
	app := testAutoscrollApp()
	app.pointer = sdl.FPoint{X: 500, Y: 50}
	app.actionKey = "a"
	if err := app.runBuiltinAction("autoscroll"); err != nil {
		t.Fatal(err)
	}
	app.actionKey = ""

	app.pointer = sdl.FPoint{X: 500, Y: 90}
	app.handleSDLKeyUp(&sdl.KeyboardEvent{Key: sdl.KeycodeA})
	if app.autoscroll != nil {
		t.Fatal("expected releasing the key after a drag to stop autoscroll")
	}
}

func TestAutoscrollStopsOutsideDocumentView(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	app.advanceAutoscroll() // no document is open
	if app.autoscroll != nil {
		t.Fatal("expected autoscroll to stop without a document view")
	}
}

func TestPanAndAutoscrollExcludeEachOther(t *testing.T) {
	app := testAutoscrollApp()
	startTestAutoscroll(t, app, 500, 50)

	app.actionKey = " "
	if err := app.runBuiltinAction("pan"); err != nil {
		t.Fatal(err)
	}
	app.actionKey = ""
	if !app.panning || app.autoscroll != nil {
		t.Fatalf("expected pan to replace autoscroll, panning=%v autoscroll=%v", app.panning, app.autoscroll != nil)
	}

	startTestAutoscroll(t, app, 500, 50)
	if app.panning {
		t.Fatal("expected autoscroll to replace pan")
	}
}

func TestPresentationPanAdvances(t *testing.T) {
	app := testAutoscrollApp()
	app.presentation = &presentationState{}

	if !app.runPresentationAction("pan") || app.page != 1 {
		t.Fatalf("expected pan to advance the presentation, page=%d", app.page)
	}
}
