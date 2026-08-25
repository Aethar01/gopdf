package viewer

import "math"

const (
	gestureSmoothing = 0.35
	smoothScrollSnap = 0.25
)

type smoothScrollState struct {
	active   bool
	targetX  float64
	targetY  float64
	appliedX float64
	appliedY float64
}

func smoothToward(current, target float64) float64 {
	return current + (target-current)*gestureSmoothing
}

func (a *App) queueSmoothScroll(dx, dy float64) {
	state := &a.smoothScroll
	if !state.active || a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		state.active = true
		state.targetX = a.scrollX
		state.targetY = a.scrollY
		state.appliedX = a.scrollX
		state.appliedY = a.scrollY
	}

	maxX, maxY := a.maxScrollOffsets()
	state.targetX = clampFloat(state.targetX+dx, 0, maxX)
	state.targetY = clampFloat(state.targetY+dy, 0, maxY)
	if state.targetX == a.scrollX && state.targetY == a.scrollY {
		a.cancelSmoothScroll()
	}
}

func (a *App) advanceSmoothScroll() bool {
	state := &a.smoothScroll
	if !state.active {
		return false
	}

	// Direct navigation owns the viewport immediately. If anything other than
	// this smoother moved the scroll position, discard the stale wheel target.
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

func (a *App) cancelSmoothScroll() {
	a.smoothScroll = smoothScrollState{}
}

func (a *App) smoothScrollActive() bool {
	return a.smoothScroll.active
}

func (a *App) maxScrollOffsets() (float64, float64) {
	viewportW, viewportH := a.viewportSize()
	return math.Max(0, a.contentW-float64(viewportW)), math.Max(0, a.contentH-float64(viewportH))
}
