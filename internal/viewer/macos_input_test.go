package viewer

import (
	"testing"

	"gopdf/internal/config"

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

func TestPreciseTrackpadWheelUsesFractionalDelta(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleSDLMouseWheel(&sdl.MouseWheelEvent{Y: 0.125, IntegerY: 0})

	assertClose(t, app.scrollY, 92)
}

func TestFlippedPreciseTrackpadWheelIsNormalizedOnce(t *testing.T) {
	app := testLayoutApp(5)
	app.pageStep = 64
	app.config = config.Config{NaturalScroll: false}
	app.mouseBindings = map[string]string{
		"wheel_up":    "scroll_up",
		"wheel_down":  "scroll_down",
		"wheel_left":  "scroll_left",
		"wheel_right": "scroll_right",
	}
	app.recomputeLayout(1000, 100)
	app.scrollY = 100

	app.handleSDLMouseWheel(&sdl.MouseWheelEvent{
		Y:         -0.125,
		IntegerY: 0,
		Direction: sdl.MouseWheelFlipped,
	})

	assertClose(t, app.scrollY, 92)
}
