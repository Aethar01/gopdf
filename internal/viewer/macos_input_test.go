package viewer

import (
	"testing"

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
