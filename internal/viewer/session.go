package viewer

import (
	"os"
	"time"
)

const (
	documentStatInterval   = 500 * time.Millisecond
	documentReloadDebounce = 750 * time.Millisecond
	documentReloadRetry    = time.Second
)

type documentSession struct {
	path string
	mod  time.Time
	size int64

	lastStat time.Time
	pending  *documentChange
}

type documentChange struct {
	mod         time.Time
	size        int64
	firstSeen   time.Time
	lastAttempt time.Time
}

func (s *documentSession) record(path string) {
	s.path = path
	s.pending = nil
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		s.mod = time.Time{}
		s.size = 0
		return
	}
	s.mod = info.ModTime()
	s.size = info.Size()
	s.lastStat = time.Now()
}

func (s *documentSession) poll(now time.Time) (documentChange, bool) {
	if s.path == "" || now.Sub(s.lastStat) < documentStatInterval {
		return documentChange{}, false
	}
	s.lastStat = now
	info, err := os.Stat(s.path)
	if err != nil || info.IsDir() {
		return documentChange{}, false
	}
	mod, size := info.ModTime(), info.Size()
	if s.mod.IsZero() {
		s.mod = mod
		s.size = size
		return documentChange{}, false
	}
	if mod.Equal(s.mod) && size == s.size {
		s.pending = nil
		return documentChange{}, false
	}
	if s.pending == nil || !s.pending.mod.Equal(mod) || s.pending.size != size {
		s.pending = &documentChange{mod: mod, size: size, firstSeen: now}
		return documentChange{}, false
	}
	if now.Sub(s.pending.firstSeen) < documentReloadDebounce {
		return documentChange{}, false
	}
	if !s.pending.lastAttempt.IsZero() && now.Sub(s.pending.lastAttempt) < documentReloadRetry {
		return documentChange{}, false
	}
	s.pending.lastAttempt = now
	return *s.pending, true
}

func (s *documentSession) commit(change documentChange) {
	s.mod = change.mod
	s.size = change.size
	s.pending = nil
}

type viewState struct {
	page            int
	scrollX         float64
	scrollY         float64
	zoom            float64
	fitMode         string
	renderMode      string
	dualPage        bool
	firstPageOffset bool
	statusBarShown  bool
	altColors       bool
}

func (a *App) captureViewState() viewState {
	return viewState{
		page:            a.page,
		scrollX:         a.scrollX,
		scrollY:         a.scrollY,
		zoom:            a.zoom,
		fitMode:         a.fitMode,
		renderMode:      a.renderMode,
		dualPage:        a.dualPage,
		firstPageOffset: a.firstPageOffset,
		statusBarShown:  a.statusBarShown,
		altColors:       a.altColors,
	}
}

func (a *App) restoreViewState(state viewState) {
	a.zoom = state.zoom
	a.fitMode = state.fitMode
	a.renderMode = state.renderMode
	a.dualPage = state.dualPage
	a.firstPageOffset = state.firstPageOffset
	a.statusBarShown = state.statusBarShown
	a.altColors = state.altColors
	a.page = clampInt(state.page, 0, max(0, a.pageCount-1))
	a.recomputeLayout(a.viewportSize())
	a.scrollX = state.scrollX
	a.scrollY = state.scrollY
	a.clampScroll()
	if a.renderMode == "continuous" {
		a.updateCurrentPageFromScroll()
	}
}

func (state viewState) atDocumentStart() viewState {
	state.page = 0
	state.scrollX = 0
	state.scrollY = 0
	return state
}
