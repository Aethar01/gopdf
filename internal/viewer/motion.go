package viewer

import (
	"fmt"
	"math"
	"sync/atomic"
	"time"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// motionState is the UI's transitions in progress. Drawing code asks
// where a value heading for its target is now, and frames keep coming
// while any is still on its way.
type motionState struct {
	frame     uint64    // frames drawn
	now       time.Time // when the frame being drawn is drawn
	animating bool      // a transition is still running this frame
	tweens    map[string]*tween
	fade      float64 // how far what is drawn now is faded out, from 0 to 1
	// shownView is the view drawn last frame, and closingView one drawn
	// fading out after it closed.
	shownView, closingView *uiView
	// fadingText is the last status text shown, faded out once cleared.
	fadingText string
	osReduced  atomic.Bool // the OS asks for less motion; see motionScale
}

// tween is a value moving from one place to another.
type tween struct {
	from, to, value float64
	start           time.Time
	seen            uint64 // the last frame the value was asked for
}

// beginMotionFrame starts a frame's transitions.
func (a *App) beginMotionFrame() {
	m := &a.motion
	m.frame++
	m.now = time.Now()
	m.animating = false
	m.fade = 0
	if len(m.tweens) > 64 {
		for key, tw := range m.tweens {
			if tw.seen+1 < m.frame {
				delete(m.tweens, key)
			}
		}
	}
}

// animate returns where a value heading for target is now, moving as tr
// says. A value not asked for last frame is already at its target.
func (a *App) animate(key string, target float64, tr config.Transition) float64 {
	return a.animateFrom(key, math.NaN(), target, tr)
}

// animateFrom is animate for a value that, when not asked for last frame,
// starts from from, as a panel's opacity does when it opens.
func (a *App) animateFrom(key string, from, target float64, tr config.Transition) float64 {
	m := &a.motion
	tw := m.tweens[key]
	switch {
	case tw == nil || tw.seen+1 < m.frame:
		if math.IsNaN(from) {
			from = target
		}
		tw = &tween{from: from, to: target, value: from, start: m.now}
		if m.tweens == nil {
			m.tweens = map[string]*tween{}
		}
		m.tweens[key] = tw
	case tw.to != target:
		tw.from, tw.to, tw.start = tw.value, target, m.now
	}
	tw.seen = m.frame
	duration := tr.Scaled(a.motionScale())
	progress := 1.0
	if duration > 0 {
		progress = float64(m.now.Sub(tw.start)) / float64(time.Duration(duration*float64(time.Millisecond)))
	}
	if progress >= 1 || tw.from == tw.to {
		tw.value = tw.to
	} else {
		tw.value = tw.from + (tw.to-tw.from)*tr.Ease(progress)
		m.animating = true
	}
	return tw.value
}

// faded runs draw with what it draws faded to opacity, within any fade
// already in place.
func (a *App) faded(opacity float64, draw func() error) error {
	m := &a.motion
	old := m.fade
	m.fade = 1 - (1-old)*max(0, min(1, opacity))
	defer func() { m.fade = old }()
	return draw()
}

// viewKey names a view's transitions.
func viewKey(view *uiView) string { return fmt.Sprintf("%p", view) }

// drawUIViews draws the open view, fading in as it opens, and one just
// closed fading out.
func (a *App) drawUIViews(renderer *sdl.Renderer) error {
	m := &a.motion
	view := a.activeUIView()
	if view != m.shownView {
		m.closingView = nil
		if view == nil && m.shownView != nil && m.shownView.modal {
			m.closingView = m.shownView
		}
		m.shownView = view
	}
	tr := a.config.Theme.Motion.Panel
	if closing := m.closingView; closing != nil {
		p := a.animateFrom("panel out "+viewKey(closing), 1, 0, tr)
		if p <= 0 {
			m.closingView = nil
		} else if err := a.faded(p, func() error { return a.drawUIViewFrame(renderer, closing) }); err != nil {
			return err
		}
	}
	if view == nil {
		return nil
	}
	p := a.animateFrom("panel in "+viewKey(view), 0, 1, tr)
	return a.faded(p, func() error { return a.drawUIView(renderer, view) })
}
