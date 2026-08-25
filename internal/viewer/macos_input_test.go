package viewer

import (
	"testing"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestRepeatedArrowKeyRunsCountableBinding(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.sequenceLookup = map[string]string{
		normalizeBinding("<Up>"): "scroll_up",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 192

	app.handleSDLKeyDown(&sdl.KeyboardEvent{Key: sdl.KeycodeUp})
	app.handleSDLKeyDown(&sdl.KeyboardEvent{Key: sdl.KeycodeUp, Repeat: true})

	assertClose(t, app.scrollY, 64)
}

func TestRepeatedKeyDoesNotExpandMultiKeySequence(t *testing.T) {
	app := testLayoutApp(5)
	app.sequenceLookup = map[string]string{
		normalizeBinding("g g"): "first_page",
	}

	app.handleSDLKeyDown(&sdl.KeyboardEvent{Key: sdl.KeycodeG})
	app.handleSDLKeyDown(&sdl.KeyboardEvent{Key: sdl.KeycodeG, Repeat: true})

	if got := len(app.sequence); got != 1 {
		t.Fatalf("expected one pending sequence token, got %d (%v)", got, app.sequence)
	}
}

func TestSmoothScrollQueuesTargetWithoutImmediateJump(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(1000, 100)
	app.scrollY = 100
	defer app.cancelSmoothScroll()

	app.queueSmoothScroll(0, 64)

	assertClose(t, app.scrollY, 100)
	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll to become active")
	}
	assertClose(t, state.targetY, 164)

	if !app.advanceSmoothScroll() {
		t.Fatal("expected first animation frame to move the viewport")
	}
	assertClose(t, app.scrollY, 122.4)
}

func TestSmoothScrollAccumulatesWheelBurstIntoOneTarget(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(1000, 100)
	app.scrollY = 100
	defer app.cancelSmoothScroll()

	app.queueSmoothScroll(0, 16)
	app.queueSmoothScroll(0, 24)
	app.queueSmoothScroll(0, 8)

	assertClose(t, app.scrollY, 100)
	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll to become active")
	}
	assertClose(t, state.targetY, 148)

	app.advanceSmoothScroll()
	assertClose(t, app.scrollY, 116.8)
}

func TestDirectNavigationCancelsPendingSmoothScroll(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(1000, 100)
	app.scrollY = 100
	defer app.cancelSmoothScroll()

	app.queueSmoothScroll(0, 64)
	app.advanceSmoothScroll()
	app.scrollBy(0, 10)
	position := app.scrollY

	if app.advanceSmoothScroll() {
		t.Fatal("expected direct navigation to cancel the pending wheel target")
	}
	if app.smoothScrollActive() {
		t.Fatal("expected smooth scroll state to be cleared")
	}
	assertClose(t, app.scrollY, position)
}

func TestSmoothWheelOnlyHandlesDefaultScrollBindings(t *testing.T) {
	app := testLayoutApp(5)
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}

	if !app.canSmoothWheel(0, 0.5) {
		t.Fatal("expected default wheel binding to use smooth scrolling")
	}

	app.mouseBindings["wheel_up"] = "next_page"
	if app.canSmoothWheel(0, 0.5) {
		t.Fatal("expected custom wheel binding to remain discrete")
	}
}

func TestSmoothTowardUsesConfiguredDampening(t *testing.T) {
	assertClose(t, smoothToward(0, 1, 0.35, 16*time.Millisecond), 0.35)
	assertClose(t, smoothToward(10, 20, 0.5, 16*time.Millisecond), 15)
}

func TestSmoothTowardNormalizesForElapsedTime(t *testing.T) {
	oneFrame := smoothToward(0, 1, 0.35, 16*time.Millisecond)
	twoFrames := smoothToward(0, 1, 0.35, 32*time.Millisecond)
	steppedTwice := smoothToward(oneFrame, 1, 0.35, 16*time.Millisecond)

	assertClose(t, twoFrames, steppedTwice)
}

func TestSmoothTowardClampsDampening(t *testing.T) {
	assertClose(t, smoothToward(0, 1, 2, 16*time.Millisecond), 1)
	assertClose(t, smoothToward(0, 1, 0, 16*time.Millisecond), 0.01)
}
