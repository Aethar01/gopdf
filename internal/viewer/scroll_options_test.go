package viewer

import (
	"testing"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestInvertWheelDeltasFlipsBothAxes(t *testing.T) {
	wx, wy := invertWheelDeltas(1.5, -2.5, true)
	assertClose(t, float64(wx), -1.5)
	assertClose(t, float64(wy), 2.5)
}

func TestInvertWheelDeltasLeavesBothAxesAloneWhenDisabled(t *testing.T) {
	wx, wy := invertWheelDeltas(1.5, -2.5, false)
	assertClose(t, float64(wx), 1.5)
	assertClose(t, float64(wy), -2.5)
}

func TestNormalizedWheelDeltasHandlesSDLFlippedDirection(t *testing.T) {
	wx, wy := normalizedWheelDeltas(&sdl.MouseWheelEvent{
		X:         1.25,
		Y:         -0.75,
		Direction: sdl.MouseWheelFlipped,
	})
	assertClose(t, float64(wx), -1.25)
	assertClose(t, float64(wy), 0.75)
}
