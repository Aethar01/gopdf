package viewer

import (
	"math"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

const (
	smoothScrollFrame = 16 * time.Millisecond
	smoothScrollSnap  = 0.25
)

type smoothScrollState struct {
	targetX       float64
	targetY       float64
	appliedX      float64
	appliedY      float64
	lastAdvanceNS uint64
}

func smoothToward(current, target, dampening float64, elapsed time.Duration) float64 {
	if elapsed <= 0 {
		return current
	}
	dampening = clampFloat(dampening, 0.01, 1)
	if dampening >= 1 {
		return target
	}
	factor := 1 - math.Pow(1-dampening, float64(elapsed)/float64(smoothScrollFrame))
	return current + (target-current)*factor
}

func normalizedWheelDeltas(e *sdl.MouseWheelEvent) (float32, float32) {
	wx, wy := e.X, e.Y
	if wx == 0 {
		wx = float32(e.IntegerX)
	}
	if wy == 0 {
		wy = float32(e.IntegerY)
	}
	if e.Direction == sdl.MouseWheelFlipped {
		wx = -wx
		wy = -wy
	}
	return wx, wy
}

func invertWheelDeltas(wx, wy float32, invert bool) (float32, float32) {
	if invert {
		return -wx, -wy
	}
	return wx, wy
}

func (a *App) handleAnimatedMouseWheel(e *sdl.MouseWheelEvent) {
	if a.luaUI.visible || a.keybindMenu.visible || a.outlineMenu.visible {
		a.cancelSmoothScroll()
		a.handleSDLMouseWheel(e)
		a.pendingRedraw = true
		return
	}

	wx, wy := normalizedWheelDeltas(e)
	if sdl.GetModState()&sdl.KeymodCtrl != 0 {
		a.cancelSmoothScroll()
		a.handleSDLMouseWheel(e)
		a.pendingRedraw = true
		return
	}

	if !a.canSmoothWheel(wx, wy) {
		a.runDiscreteMouseWheel(wx, wy)
		return
	}

	wx, wy = invertWheelDeltas(wx, wy, a.config.InvertSmoothScroll)
	a.queueSmoothScrollAt(float64(wx)*a.pageStep, -float64(wy)*a.pageStep, e.Timestamp)
}

func (a *App) runDiscreteMouseWheel(wx, wy float32) {
	a.cancelSmoothScroll()
	wx, wy = invertWheelDeltas(wx, wy, a.config.InvertScroll)
	a.dispatchDiscreteWheel(wx, wy)
	a.pendingRedraw = true
}

func (a *App) dispatchDiscreteWheel(wx, wy float32) {
	if wy > 0 {
		a.runMouseBinding("wheel_up")
	}
	if wy < 0 {
		a.runMouseBinding("wheel_down")
	}
	if wx > 0 {
		a.runMouseBinding("wheel_right")
	}
	if wx < 0 {
		a.runMouseBinding("wheel_left")
	}
}

func (a *App) canSmoothWheel(wx, wy float32) bool {
	if wx == 0 && wy == 0 {
		return true
	}
	if wy > 0 && a.mouseBindings["wheel_up"] != "scroll_up" {
		return false
	}
	if wy < 0 && a.mouseBindings["wheel_down"] != "scroll_down" {
		return false
	}
	if wx > 0 && a.mouseBindings["wheel_right"] != "scroll_right" {
		return false
	}
	if wx < 0 && a.mouseBindings["wheel_left"] != "scroll_left" {
		return false
	}
	return true
}

func (a *App) queueSmoothScroll(dx, dy float64) {
	a.queueSmoothScrollAt(dx, dy, 0)
}

func (a *App) queueSmoothScrollAt(dx, dy float64, timestampNS uint64) {
	// Wheel events can be delivered in bursts when the event queue is drained.
	// Advance to each event's original SDL timestamp before moving the target so
	// changes in event cadence do not turn into changes in scroll velocity.
	if timestampNS != 0 {
		a.advanceSmoothScrollTo(timestampNS)
	}

	state := a.smoothScrollState()
	if state == nil || a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		state = &smoothScrollState{
			targetX:       a.scrollX,
			targetY:       a.scrollY,
			appliedX:      a.scrollX,
			appliedY:      a.scrollY,
			lastAdvanceNS: timestampNS,
		}
		a.smoothScroll = state
	} else if state.lastAdvanceNS == 0 && timestampNS != 0 {
		state.lastAdvanceNS = timestampNS
	}

	maxX, maxY := a.maxScrollOffsets()
	state.targetX = clampFloat(state.targetX+dx, 0, maxX)
	state.targetY = clampFloat(state.targetY+dy, 0, maxY)
	if state.targetX == a.scrollX && state.targetY == a.scrollY {
		a.cancelSmoothScroll()
	}
}

func (a *App) advanceSmoothScroll() bool {
	return a.advanceSmoothScrollTo(sdl.GetTicksNS())
}

func (a *App) advanceSmoothScrollTo(timestampNS uint64) bool {
	state := a.smoothScrollState()
	if state == nil || timestampNS == 0 {
		return false
	}
	if state.lastAdvanceNS == 0 {
		state.lastAdvanceNS = timestampNS
		return false
	}
	if timestampNS <= state.lastAdvanceNS {
		return false
	}

	elapsed := time.Duration(timestampNS - state.lastAdvanceNS)
	state.lastAdvanceNS = timestampNS
	return a.advanceSmoothScrollBy(elapsed)
}

func (a *App) advanceSmoothScrollBy(elapsed time.Duration) bool {
	state := a.smoothScrollState()
	if state == nil {
		return false
	}

	// Keyboard navigation, panning, page jumps and relayouts take ownership of
	// the viewport immediately. An old wheel target must never pull it back.
	if a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		a.cancelSmoothScroll()
		return false
	}

	nextX := smoothToward(a.scrollX, state.targetX, a.config.SmoothScrollDampening, elapsed)
	nextY := smoothToward(a.scrollY, state.targetY, a.config.SmoothScrollDampening, elapsed)
	if math.Abs(state.targetX-nextX) <= smoothScrollSnap {
		nextX = state.targetX
	}
	if math.Abs(state.targetY-nextY) <= smoothScrollSnap {
		nextY = state.targetY
	}

	oldX, oldY := a.scrollX, a.scrollY
	a.scrollX = nextX
	a.scrollY = nextY
	a.clampScroll()
	if a.renderMode != "single" && (a.scrollX != oldX || a.scrollY != oldY) {
		a.updateCurrentPageFromScroll()
	}
	state.appliedX = a.scrollX
	state.appliedY = a.scrollY

	if a.scrollX == state.targetX && a.scrollY == state.targetY {
		a.cancelSmoothScroll()
	}
	if a.scrollX != oldX || a.scrollY != oldY {
		a.pendingRedraw = true
		return true
	}
	return false
}

func (a *App) smoothScrollState() *smoothScrollState {
	return a.smoothScroll
}

func (a *App) cancelSmoothScroll() {
	a.smoothScroll = nil
}

func (a *App) smoothScrollActive() bool {
	return a.smoothScroll != nil
}

func (a *App) maxScrollOffsets() (float64, float64) {
	viewportW, viewportH := a.viewportSize()
	return math.Max(0, a.contentW-float64(viewportW)), math.Max(0, a.contentH-float64(viewportH))
}
