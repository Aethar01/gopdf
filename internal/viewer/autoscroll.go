package viewer

import (
	"math"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Autoscroll distances and speeds are in logical pixels, scaled to the
// display. The autoscroll_speed_factor and autoscroll_max_speed options
// shape the curve.
const (
	// autoscrollDeadZone is how far the pointer can stray from the anchor
	// on an axis before that axis scrolls.
	autoscrollDeadZone = 10.0
	autoscrollGain     = 4.0
	autoscrollExponent = 1.3
	// autoscrollMaxStep bounds one frame's travel after a stall, such as a
	// slow render, so the view does not leap.
	autoscrollMaxStep = 100 * time.Millisecond
	// autoscrollMarkerSize is the logical size of the anchor marker.
	autoscrollMarkerSize = 32
)

// autoscrollState tracks browser-style autoscroll: the view scrolls
// continuously toward the pointer, faster the further it is from the
// anchor where autoscroll started.
type autoscrollState struct {
	anchor  sdl.FPoint
	button  uint8       // mouse button holding autoscroll, or 0
	keycode sdl.Keycode // key holding autoscroll, or 0
	// sticky autoscroll outlives the button or key that started it and
	// runs until the next click, key press or wheel.
	sticky bool
	// dragged records that the pointer left the dead zone, so releasing
	// the button ends autoscroll instead of making it sticky.
	dragged     bool
	lastAdvance time.Time // zero while the pointer rests in the dead zone
}

type autoscrollMarkerKey struct {
	size                 int
	horizontal, vertical bool
}

// startAutoscroll autoscrolls from the pointer. Releasing the key or mouse
// button that ran the action ends it once the pointer has moved; a release
// without moving leaves it running until the next input. Run any other
// way, it starts that way.
func (a *App) startAutoscroll() {
	a.stopPan()
	a.cancelSmoothScroll()
	s := &autoscrollState{anchor: a.pointer}
	switch {
	case a.actionKeycode != 0:
		s.keycode = a.actionKeycode
	case a.mouseButton != 0:
		s.button = a.mouseButton
	default:
		s.sticky = true
	}
	a.autoscroll = s
	a.pendingRedraw = true
	a.updateCursor()
}

func (a *App) stopAutoscroll() {
	if a.autoscroll == nil {
		return
	}
	a.autoscroll = nil
	a.pendingRedraw = true
	a.updateCursor()
}

// autoscrollAllowed reports whether the document view, the overview or a
// menu is showing, the places autoscroll applies.
func (a *App) autoscrollAllowed() bool {
	return a.doc != nil && a.mode == modeNormal && a.presentation == nil
}

// releaseAutoscroll handles the release of the key or button holding
// autoscroll.
func (a *App) releaseAutoscroll() {
	s := a.autoscroll
	a.noteAutoscrollPointer()
	if s.dragged {
		a.stopAutoscroll()
		return
	}
	s.sticky, s.button, s.keycode = true, 0, 0
}

// noteAutoscrollPointer records whether the pointer has left the dead zone.
func (a *App) noteAutoscrollPointer() {
	s := a.autoscroll
	if s == nil || s.dragged {
		return
	}
	limit := autoscrollDeadZone * a.displayScale()
	s.dragged = math.Abs(float64(a.pointer.X-s.anchor.X)) > limit || math.Abs(float64(a.pointer.Y-s.anchor.Y)) > limit
}

// handleAutoscrollButton gives autoscroll every mouse button while it
// runs: releasing the holding button ends or pins it and any press ends it.
func (a *App) handleAutoscrollButton(e *sdl.MouseButtonEvent) bool {
	if e.Type == sdl.EventMouseButtonUp && a.swallowButtonUp != 0 && e.Button == a.swallowButtonUp {
		a.swallowButtonUp = 0
		return true
	}
	s := a.autoscroll
	if s == nil {
		return false
	}
	switch {
	case e.Type == sdl.EventMouseButtonDown:
		a.stopAutoscroll()
		a.swallowButtonUp = e.Button
	case s.button != 0 && e.Button == s.button:
		a.releaseAutoscroll()
	}
	return true
}

// handleAutoscrollKey ends autoscroll on a key press, which it consumes.
// Held keys repeat without ending it.
func (a *App) handleAutoscrollKey(e *sdl.KeyboardEvent) bool {
	if a.autoscroll == nil {
		return false
	}
	if !e.Repeat {
		a.stopAutoscroll()
	}
	return true
}

func (a *App) advanceAutoscroll() {
	s := a.autoscroll
	if s == nil {
		return
	}
	if !a.autoscrollAllowed() {
		a.stopAutoscroll()
		return
	}
	now := time.Now()
	elapsed := a.animationFrameDuration()
	if !s.lastAdvance.IsZero() {
		elapsed = min(now.Sub(s.lastAdvance), autoscrollMaxStep)
	}
	if a.advanceAutoscrollBy(elapsed) {
		s.lastAdvance = now
	} else {
		s.lastAdvance = time.Time{}
	}
}

// advanceAutoscrollBy scrolls for elapsed at the pointer's speed. It
// reports whether the pointer is outside the dead zone.
func (a *App) advanceAutoscrollBy(elapsed time.Duration) bool {
	a.noteAutoscrollPointer()
	a.updateCursor()
	vx, vy := a.autoscrollVelocity()
	if vx == 0 && vy == 0 {
		return false
	}
	if view := a.activeModalUIView(); view != nil {
		a.scrollListBy(view, vy*elapsed.Seconds()/float64(a.modalListRowHeight()))
		return true
	}
	oldX, oldY := a.scrollX, a.scrollY
	a.scrollBy(vx*elapsed.Seconds(), vy*elapsed.Seconds())
	if a.scrollX != oldX || a.scrollY != oldY {
		a.pendingRedraw = true
	}
	return true
}

// autoscrollMoving reports whether autoscroll needs animation frames.
func (a *App) autoscrollMoving() bool {
	return a.autoscroll != nil && !a.autoscroll.lastAdvance.IsZero()
}

// autoscrollVelocity is the scroll speed in pixels per second along the
// axes that can scroll.
func (a *App) autoscrollVelocity() (float64, float64) {
	s := a.autoscroll
	horizontal, vertical := a.autoscrollAxes()
	var vx, vy float64
	scale := a.displayScale()
	speed := func(offset float32) float64 {
		return autoscrollSpeed(float64(offset)/scale, a.config.AutoscrollSpeedFactor, a.config.AutoscrollMaxSpeed) * scale
	}
	if horizontal {
		vx = speed(a.pointer.X - s.anchor.X)
	}
	if vertical {
		vy = speed(a.pointer.Y - s.anchor.Y)
	}
	return vx, vy
}

// autoscrollSpeed maps a logical pointer offset from the anchor to a
// logical speed, growing faster than linearly past the dead zone. The
// factor scales the curve and maxSpeed, when positive, caps it.
func autoscrollSpeed(offset, factor, maxSpeed float64) float64 {
	past := math.Abs(offset) - autoscrollDeadZone
	if past <= 0 {
		return 0
	}
	speed := autoscrollGain * math.Pow(past, autoscrollExponent) * factor
	if maxSpeed > 0 {
		speed = math.Min(speed, maxSpeed)
	}
	return math.Copysign(speed, offset)
}

// autoscrollAxes reports which axes the view can scroll along.
func (a *App) autoscrollAxes() (horizontal, vertical bool) {
	if view := a.activeModalUIView(); view != nil {
		maxScroll, _ := a.modalSmoothScrollBounds(view)
		return false, maxScroll > 0
	}
	maxX, maxY := a.maxScrollOffsets()
	return maxX > 0, maxY > 0
}

func (a *App) autoscrollCursor() cursorKind {
	vx, vy := a.autoscrollVelocity()
	if vx == 0 && vy == 0 {
		switch horizontal, vertical := a.autoscrollAxes(); {
		case horizontal && vertical:
			return cursorAutoscrollAll
		case horizontal:
			return cursorAutoscrollHorizontal
		default:
			return cursorAutoscrollVertical
		}
	}
	// Clockwise from north, in eighths of a turn.
	octant := int(math.Round(math.Atan2(vx, -vy)/(math.Pi/4))+8) % 8
	return cursorAutoscrollN + cursorKind(octant)
}

func (a *App) displayScale() float64 {
	if a.window == nil {
		return 1
	}
	if scale := float64(sdl.GetWindowDisplayScale(a.window)); scale > 0 {
		return scale
	}
	return 1
}

// drawAutoscrollMarker marks the autoscroll anchor.
func (a *App) drawAutoscrollMarker(renderer *sdl.Renderer) {
	if a.autoscroll == nil {
		return
	}
	horizontal, vertical := a.autoscrollAxes()
	key := autoscrollMarkerKey{
		size:       int(math.Round(autoscrollMarkerSize * a.displayScale())),
		horizontal: horizontal,
		// A view that cannot scroll at all still shows vertical arrows.
		vertical: vertical || !horizontal,
	}
	if a.autoscrollMarker == nil || a.autoscrollMarkerKey != key {
		if a.autoscrollMarker != nil {
			sdl.DestroyTexture(a.autoscrollMarker)
			a.autoscrollMarker = nil
		}
		tex, err := textureFromNRGBA(renderer, autoscrollMarkerGlyph(key.horizontal, key.vertical).rasterize(key.size))
		if err != nil {
			a.logf("autoscroll marker: %v", err)
			return
		}
		a.autoscrollMarker, a.autoscrollMarkerKey = tex, key
	}
	size := float32(key.size)
	dst := sdl.FRect{X: a.autoscroll.anchor.X - size/2, Y: a.autoscroll.anchor.Y - size/2, W: size, H: size}
	sdl.RenderTexture(renderer, a.autoscrollMarker, nil, &dst)
}
