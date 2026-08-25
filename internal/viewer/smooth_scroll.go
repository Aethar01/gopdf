package viewer

import (
	"math"
	"sync"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

const (
	smoothScrollFrame = 16 * time.Millisecond
	smoothScrollSnap  = 0.25
)

type smoothScrollState struct {
	targetX     float64
	targetY     float64
	appliedX    float64
	appliedY    float64
	lastAdvance time.Time
}

var smoothScrollStates sync.Map // map[*App]*smoothScrollState

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
	a.queueSmoothScroll(float64(wx)*a.pageStep, -float64(wy)*a.pageStep)
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
	state := a.smoothScrollState()
	if state == nil || a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		state = &smoothScrollState{
			targetX:  a.scrollX,
			targetY:  a.scrollY,
			appliedX: a.scrollX,
			appliedY: a.scrollY,
		}
		smoothScrollStates.Store(a, state)
	}

	maxX, maxY := a.maxScrollOffsets()
	state.targetX = clampFloat(state.targetX+dx, 0, maxX)
	state.targetY = clampFloat(state.targetY+dy, 0, maxY)
	if state.targetX == a.scrollX && state.targetY == a.scrollY {
		a.cancelSmoothScroll()
	}
}

func (a *App) advanceSmoothScroll() bool {
	state := a.smoothScrollState()
	if state == nil {
		return false
	}

	now := time.Now()
	elapsed := smoothScrollFrame
	if !state.lastAdvance.IsZero() {
		elapsed = now.Sub(state.lastAdvance)
	}
	state.lastAdvance = now
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
	value, ok := smoothScrollStates.Load(a)
	if !ok {
		return nil
	}
	return value.(*smoothScrollState)
}

func (a *App) cancelSmoothScroll() {
	smoothScrollStates.Delete(a)
}

func (a *App) smoothScrollActive() bool {
	_, ok := smoothScrollStates.Load(a)
	return ok
}

func (a *App) maxScrollOffsets() (float64, float64) {
	viewportW, viewportH := a.viewportSize()
	return math.Max(0, a.contentW-float64(viewportW)), math.Max(0, a.contentH-float64(viewportH))
}
