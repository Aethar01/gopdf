package viewer

import (
	"os"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

const (
	documentReloadDebounce = 200 * time.Millisecond
	documentReloadRetry    = time.Second
)

type documentSession struct {
	watcher *documentWatcher
	mod     time.Time
	size    int64

	pending     *documentChange
	lastAttempt time.Time
}

type documentChange struct {
	mod       time.Time
	size      int64
	firstSeen time.Time
}

func (s *documentSession) record(path string) {
	if s.watcher != nil {
		s.watcher.Close()
		s.watcher = nil
	}
	s.pending = nil
	s.lastAttempt = time.Time{}
	watcher, err := newDocumentWatcher(path)
	if err != nil {
		// Fall back gracefully if watcher can't be created
		return
	}
	s.watcher = watcher
	s.watcher.record(path)
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		s.mod = time.Time{}
		s.size = 0
		return
	}
	s.mod = info.ModTime()
	s.size = info.Size()
}

func (s *documentSession) poll(now time.Time) (documentChange, bool) {
	if s.watcher == nil {
		return documentChange{}, false
	}

	if s.pending != nil {
		if !s.lastAttempt.IsZero() && now.Sub(s.lastAttempt) < documentReloadRetry {
			return documentChange{}, false
		}
		s.lastAttempt = now
		return *s.pending, true
	}

	change, ok := s.watcher.waitForChange(0)
	if !ok {
		return documentChange{}, false
	}

	s.pending = &documentChange{
		mod:       change.mod,
		size:      change.size,
		firstSeen: change.firstSeen,
	}
	s.lastAttempt = now
	return *s.pending, true
}

func (s *documentSession) commit(change documentChange) {
	s.mod = change.mod
	s.size = change.size
	s.pending = nil
	s.lastAttempt = time.Time{}
}

func (s *documentSession) Close() {
	if s.watcher != nil {
		s.watcher.Close()
	}
}

type viewState struct {
	page            int
	scrollX         float64
	scrollY         float64
	anchor          viewportAnchor
	zoom            float64
	fitMode         string
	renderMode      string
	rotation        float64
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
		anchor:          a.captureViewportAnchor(),
		zoom:            a.zoom,
		fitMode:         a.fitMode,
		renderMode:      a.renderMode,
		rotation:        a.rotation,
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
	a.rotation = normalizeRotation(state.rotation)
	a.dualPage = state.dualPage
	a.firstPageOffset = state.firstPageOffset
	a.statusBarShown = state.statusBarShown
	a.altColors = state.altColors
	a.updatePageMetricSizes()
	a.page = clampInt(state.page, 0, max(0, a.pageCount-1))
	a.recomputeLayout(a.viewportSize())
	if state.anchor.valid {
		a.restoreViewportAnchor(state.anchor)
	} else {
		a.scrollX = state.scrollX
		a.scrollY = state.scrollY
		a.clampScroll()
	}
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

func (a *App) saveDocumentSession() {
	if a == nil || !a.config.SessionDatabase || a.docPath == "" || a.pageCount == 0 {
		return
	}
	state := a.captureViewState()
	_ = config.SetDocumentSession(a.docPath, config.DocumentSession{
		Page:            state.page,
		ScrollX:         state.scrollX,
		ScrollY:         state.scrollY,
		AnchorPage:      state.anchor.page,
		AnchorX:         state.anchor.point.X,
		AnchorY:         state.anchor.point.Y,
		AnchorValid:     state.anchor.valid,
		Zoom:            state.zoom,
		FitMode:         state.fitMode,
		RenderMode:      state.renderMode,
		Rotation:        state.rotation,
		DualPage:        state.dualPage,
		FirstPageOffset: state.firstPageOffset,
		StatusBarShown:  state.statusBarShown,
		AltColors:       state.altColors,
	})
}

func (a *App) restoreDocumentSession() bool {
	state, ok := a.documentSessionViewState(a.docPath)
	if !ok {
		return false
	}
	a.restoreViewState(state)
	return true
}

func (a *App) documentSessionViewState(path string) (viewState, bool) {
	if a == nil || !a.config.SessionDatabase || path == "" {
		return viewState{}, false
	}
	session, ok := config.GetDocumentSession(path)
	if !ok {
		return viewState{}, false
	}
	return viewState{
		page:    session.Page,
		scrollX: session.ScrollX,
		scrollY: session.ScrollY,
		anchor: viewportAnchor{
			page:  session.AnchorPage,
			point: mupdf.Point{X: session.AnchorX, Y: session.AnchorY},
			valid: session.AnchorValid,
		},
		zoom:            session.Zoom,
		fitMode:         session.FitMode,
		renderMode:      session.RenderMode,
		rotation:        session.Rotation,
		dualPage:        session.DualPage,
		firstPageOffset: session.FirstPageOffset,
		statusBarShown:  session.StatusBarShown,
		altColors:       session.AltColors,
	}, true
}
