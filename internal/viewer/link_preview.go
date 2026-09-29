package viewer

import (
	"math"
	"time"

	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Resting the pointer on an internal link previews its destination in a
// popup, drawn from the target page's tiles at the current zoom.

const linkPreviewDelay = 400 * time.Millisecond

type linkPreview struct {
	link   mupdf.Link
	since  time.Time
	anchor sdl.FPoint // pointer position when the hover began
	drawn  bool
}

// hoverLinkPreview starts or ends a preview as the pointer moves.
func (a *App) hoverLinkPreview(link mupdf.Link, over bool, x, y float32) {
	internal := over && !link.External && link.Page >= 0 && link.Page < a.pageCount
	switch {
	case !internal || !a.config.LinkPreview:
		a.preview = nil
	case a.preview == nil || a.preview.link != link:
		a.preview = &linkPreview{link: link, since: time.Now(), anchor: sdl.FPoint{X: x, Y: y}}
		a.wakeAfter(linkPreviewDelay)
	}
}

// revealLinkPreview asks for a frame once a pending preview is due.
func (a *App) revealLinkPreview() {
	if a.previewShown() && !a.preview.drawn {
		a.pendingRedraw = true
	}
}

func (a *App) previewShown() bool {
	return a.preview != nil && time.Since(a.preview.since) >= linkPreviewDelay
}

// previewDeadline is when a pending preview appears, or zero.
func (a *App) previewDeadline() time.Time {
	if a.preview == nil || a.previewShown() {
		return time.Time{}
	}
	return a.preview.since.Add(linkPreviewDelay)
}

// previewPlacement returns the popup rect and the screen origin at which to
// draw the target page so its destination sits near the popup's top left.
func (a *App) previewPlacement() (popup sdl.FRect, x, y float64) {
	viewportW, viewportH := a.viewportSize()
	link := a.preview.link
	m := a.pageMetrics[link.Page]
	popup.W = float32(math.Min(float64(viewportW)*0.5, m.width*a.scale))
	popup.H = float32(float64(viewportH) * 0.35)
	popup.X = min(max(0, a.preview.anchor.X-popup.W/2), float32(viewportW)-popup.W)
	popup.Y = a.preview.anchor.Y + 20
	if popup.Y+popup.H > float32(viewportH) {
		popup.Y = max(0, a.preview.anchor.Y-20-popup.H)
	}

	destX, destY := float64(m.bounds.X0+m.bounds.X1)/2, float64(m.bounds.Y0)
	if link.HasX {
		destX = link.X
	}
	if link.HasY {
		destY = link.Y
	}
	originX, originY := rotatedBoundsOrigin(m.bounds, a.scale, a.rotation)
	tx, ty := transformPoint(destX, destY, a.scale, a.rotation)
	const inset = 16
	screenX := float64(popup.X + popup.W/2) // centre the page when x is not given
	if link.HasX {
		screenX = float64(popup.X) + inset
	}
	return popup, screenX - (tx - originX), float64(popup.Y) + inset - (ty - originY)
}

// previewTiles lists the target page's tiles that the popup shows.
func (a *App) previewTiles(scale float64) []tileCandidate {
	if !a.previewShown() {
		return nil
	}
	popup, x, y := a.previewPlacement()
	page := a.preview.link.Page
	pageRect := mupdf.DeviceRect(a.pageMetrics[page].bounds, scale)
	var tiles []tileCandidate
	for _, pt := range tilesCovering(pageRect, a.pageDeviceArea(page, x, y, popup, scale)) {
		tiles = append(tiles, tileCandidate{key: tileKey{page: page, scale: scale, x: pt.X, y: pt.Y, version: a.tileVersion(page)}, rect: tileRect(pageRect, pt.X, pt.Y)})
	}
	return tiles
}

func (a *App) drawLinkPreview(renderer *sdl.Renderer) {
	if !a.previewShown() {
		return
	}
	a.preview.drawn = true
	popup, x, y := a.previewPlacement()
	fillRect(renderer, popup, a.pageBackgroundColor())
	clip := sdl.Rect{X: int32(popup.X), Y: int32(popup.Y), W: int32(popup.W), H: int32(popup.H)}
	sdl.SetRenderClipRect(renderer, &clip)
	viewportW, viewportH := a.viewportSize()
	page := a.preview.link.Page
	for _, tile := range a.cache.pageTiles(page, a.tileVersion(page)) {
		a.drawTile(renderer, tile, x, y, viewportW, viewportH)
	}
	sdl.SetRenderClipRect(renderer, nil)
	strokeRect(renderer, popup, a.foregroundColor(), 1)
}
