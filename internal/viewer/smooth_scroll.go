package viewer

import (
	"math"
	"sync"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

const (
	gestureSmoothing = 0.35
	smoothScrollSnap = 0.25
)

type smoothScrollState struct {
	targetX  float64
	targetY  float64
	appliedX float64
	appliedY float64
}

var smoothScrollStates sync.Map // map[*App]*smoothScrollState

func smoothToward(current, target float64) float64 {
	return current + (target-current)*gestureSmoothing
}

func (a *App) handleAnimatedMouseWheel(e *sdl.MouseWheelEvent) {
	if a.luaUI.visible || a.keybindMenu.visible || a.outlineMenu.visible {
		a.runDiscreteMouseWheel(e)
		return
	}

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

	if sdl.GetModState()&sdl.KeymodCtrl != 0 {
		a.runDiscreteMouseWheel(e)
		return
	}

	if !a.canSmoothWheel(wx, wy) {
		a.runDiscreteMouseWheel(e)
		return
	}

	dy := -float64(wy) * a.pageStep
	if a.config.NaturalScroll {
		dy = -dy
	}
	a.queueSmoothScroll(float64(wx)*a.pageStep, dy)
}

func (a *App) runDiscreteMouseWheel(e *sdl.MouseWheelEvent) {
	a.cancelSmoothScroll()
	a.handleSDLMouseWheel(e)
	a.pendingRedraw = true
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

	// Keyboard navigation, panning, page jumps and relayouts take ownership of
	// the viewport immediately. An old wheel target must never pull it back.
	if a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		a.cancelSmoothScroll()
		return false
	}

	nextX := smoothToward(a.scrollX, state.targetX)
	nextY := smoothToward(a.scrollY, state.targetY)
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
