package viewer

import (
	"math"
	"time"
)

const (
	smoothScrollResponse = 36.0
	smoothScrollSnap     = 0.25
)

type smoothScrollState struct {
	active  bool
	targetX float64
	targetY float64
	appliedX float64
	appliedY float64
	last    time.Time
}

func (a *App) queueSmoothScroll(dx, dy float64, now time.Time) {
	state := &a.smoothScroll
	if !state.active || a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		state.active = true
		state.targetX = a.scrollX
		state.targetY = a.scrollY
		state.appliedX = a.scrollX
		state.appliedY = a.scrollY
		state.last = now
	}

	maxX, maxY := a.maxScrollOffsets()
	state.targetX = clampFloat(state.targetX+dx, 0, maxX)
	state.targetY = clampFloat(state.targetY+dy, 0, maxY)
	if state.targetX == a.scrollX && state.targetY == a.scrollY {
		a.cancelSmoothScroll()
	}
}

func (a *App) advanceSmoothScroll(now time.Time) bool {
	state := &a.smoothScroll
	if !state.active {
		return false
	}

	// Any direct navigation (keyboard scrolling, panning, page jumps, relayouts)
	// owns the viewport immediately. Do not let an old wheel target pull the
	// document back afterward.
	if a.scrollX != state.appliedX || a.scrollY != state.appliedY {
		a.cancelSmoothScroll()
		return false
	}

	dt := now.Sub(state.last)
	if dt <= 0 {
		return false
	}
	state.last = now

	alpha := 1 - math.Exp(-smoothScrollResponse*dt.Seconds())
	nextX := a.scrollX + (state.targetX-a.scrollX)*alpha
	nextY := a.scrollY + (state.targetY-a.scrollY)*alpha
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
