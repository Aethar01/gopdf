package viewer

import (
	"math"
	"time"
)

const smoothZoomSnap = 0.0001

type smoothZoomState struct {
	active      bool
	targetLog   float64
	appliedLog  float64
	appliedZoom float64
	hasApplied  bool
	lastAdvance time.Time
}

func (a *App) displayedZoom() float64 {
	zoom := a.zoom
	if a.fitMode != "manual" {
		zoom = a.scale
	}
	if zoom <= 0 {
		zoom = 1
	}
	return zoom
}

func newSmoothZoomState(zoom float64) *smoothZoomState {
	logZoom := math.Log(zoom)
	return &smoothZoomState{
		targetLog:  logZoom,
		appliedLog: logZoom,
	}
}

func (a *App) beginPinch() {
	a.cancelSmoothZoom()
	state := newSmoothZoomState(a.displayedZoom())
	state.active = true
	a.smoothZoom = state
}

func (a *App) updatePinch(scale float64) {
	if scale <= 0 || math.IsNaN(scale) || math.IsInf(scale, 0) {
		return
	}
	if a.smoothZoom == nil || !a.smoothZoom.active {
		a.beginPinch()
	}

	state := a.smoothZoom
	sensitivity := a.config.PinchSensitivity
	if sensitivity <= 0 {
		sensitivity = 1
	}
	// SDL reports a delta scale. Accumulating in log space preserves the
	// multiplicative gesture while allowing the target to animate per frame.
	state.targetLog += math.Log(scale) * sensitivity
	a.clampSmoothZoomTarget(state)
	if !a.smoothZoomInputEnabled(smoothInputSourceTrackpad) {
		a.applySmoothZoom(math.Exp(state.targetLog))
		state.appliedLog = state.targetLog
		state.appliedZoom = a.zoom
		state.hasApplied = true
	}
}

func (a *App) endPinch() {
	if a.smoothZoom == nil || !a.smoothZoom.active {
		return
	}
	a.smoothZoom.active = false
	if a.smoothZoom.targetLog == a.smoothZoom.appliedLog {
		a.cancelSmoothZoom()
	}
}

func (a *App) setManualZoom(delta float64) {
	if delta <= 0 || math.IsNaN(delta) || math.IsInf(delta, 0) {
		return
	}
	if !a.smoothZoomInputEnabled(a.currentSmoothInputSource()) {
		a.cancelSmoothZoom()
		a.applySmoothZoom(a.clampZoom(a.displayedZoom() * delta))
		return
	}
	state := a.smoothZoom
	if state == nil || state.hasApplied && a.zoom != state.appliedZoom {
		state = newSmoothZoomState(a.displayedZoom())
		a.smoothZoom = state
	}
	state.active = false
	state.targetLog += math.Log(delta)
	a.clampSmoothZoomTarget(state)
	if state.targetLog == state.appliedLog {
		a.cancelSmoothZoom()
	}
}

func (a *App) setManualZoomTarget(target float64) {
	if target <= 0 || math.IsNaN(target) || math.IsInf(target, 0) {
		return
	}
	if !a.smoothZoomInputEnabled(a.currentSmoothInputSource()) {
		a.cancelSmoothZoom()
		a.applySmoothZoom(a.clampZoom(target))
		return
	}
	state := a.smoothZoom
	if state == nil || state.hasApplied && a.zoom != state.appliedZoom {
		state = newSmoothZoomState(a.displayedZoom())
		a.smoothZoom = state
	}
	state.active = false
	state.targetLog = math.Log(a.clampZoom(target))
	a.clampSmoothZoomTarget(state)
	if state.targetLog == state.appliedLog {
		a.cancelSmoothZoom()
	}
}

func (a *App) clampSmoothZoomTarget(state *smoothZoomState) {
	state.targetLog = clampFloat(state.targetLog, math.Log(a.clampZoom(0.000001)), math.Log(a.clampZoom(1000000)))
}

func (a *App) advanceSmoothZoom() bool {
	state := a.smoothZoom
	if state == nil || state.targetLog == state.appliedLog {
		return false
	}

	now := time.Now()
	elapsed := a.animationFrameDuration()
	if !state.lastAdvance.IsZero() {
		elapsed = now.Sub(state.lastAdvance)
	}
	state.lastAdvance = now
	return a.advanceSmoothZoomBy(elapsed)
}

func (a *App) advanceSmoothZoomBy(elapsed time.Duration) bool {
	state := a.smoothZoom
	if state == nil || state.targetLog == state.appliedLog || elapsed <= 0 {
		return false
	}
	if state.hasApplied && a.zoom != state.appliedZoom {
		// Another zoom action took ownership of the viewport.
		a.cancelSmoothZoom()
		return false
	}

	next := smoothToward(state.appliedLog, state.targetLog, a.config.SmoothZoomDampening, elapsed, a.animationFrameDuration())
	if math.Abs(state.targetLog-next) <= smoothZoomSnap {
		next = state.targetLog
	}

	oldZoom := a.zoom
	a.applySmoothZoom(math.Exp(next))
	state.appliedLog = next
	state.appliedZoom = a.zoom
	state.hasApplied = true
	if state.appliedLog == state.targetLog {
		a.cancelSmoothZoom()
	}
	if a.zoom != oldZoom {
		a.pendingRedraw = true
		return true
	}
	return false
}

func (a *App) applySmoothZoom(zoom float64) {
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = "manual"
		a.zoom = a.clampZoom(zoom)
		a.scheduleRenderScaleTarget(a.zoom)
	})
}

func (a *App) cancelSmoothZoom() {
	a.smoothZoom = nil
}

func (a *App) smoothZoomAnimating() bool {
	state := a.smoothZoom
	return state != nil && state.targetLog != state.appliedLog
}
