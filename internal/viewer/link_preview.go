package viewer

import (
	"math"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Resting the pointer on an internal link previews its destination in a
// popup, drawn from the target page's tiles at the current zoom.

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
		a.wakeAfter(a.linkPreviewDelay())
	}
}

func (a *App) linkPreviewDelay() time.Duration {
	return time.Duration(a.config.LinkPreviewDelayMS) * time.Millisecond
}

// revealLinkPreview asks for a frame once a pending preview is due.
func (a *App) revealLinkPreview() {
	if a.previewShown() && !a.preview.drawn {
		a.pendingRedraw = true
	}
}

func (a *App) previewShown() bool {
	return a.preview != nil && time.Since(a.preview.since) >= a.linkPreviewDelay()
}

// previewDeadline is when a pending preview appears, or zero.
func (a *App) previewDeadline() time.Time {
	if a.preview == nil || a.previewShown() {
		return time.Time{}
	}
	return a.preview.since.Add(a.linkPreviewDelay())
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
	dx, dy := newPageTransform(m.bounds, a.scale, a.rotation).toScreen(destX, destY)
	const inset = 16
	screenX := float64(popup.X + popup.W/2) // centre the page when x is not given
	if link.HasX {
		screenX = float64(popup.X) + inset
	}
	return popup, screenX - dx, float64(popup.Y) + inset - dy
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
	st := a.style(config.ElementLinkPreview)
	border := a.drawBoxParts(renderer, &st, popup, boxBody)
	viewportW, viewportH := a.viewportSize()
	page := a.preview.link.Page
	// A rounded preview clips each tile to its outline as geometry, which
	// any renderer draws; the border drawn over it smooths the cut edge.
	// A page turned at all keeps to the square clip.
	shape := a.boxShape(&st, pixelRect(popup))
	rounded := shape.radius != [4]float32{} && normalizeRotation(a.rotation) == 0
	var outline []point32
	if rounded {
		var radius config.Corners
		for i, r := range shape.radius {
			radius[i] = float64(r)
		}
		box := pixelRect(popup)
		outline = flattenPath(config.RoundedRectPath(float64(box.X), float64(box.Y), float64(box.W), float64(box.H), radius), pathPlacer(0, 0, 0, 0, 1))[0]
	}
	a.withClip(renderer, popup, func() error {
		for _, tile := range a.cache.pageTiles(page, a.tileVersion(page)) {
			if !rounded {
				a.drawTile(renderer, tile, x, y, viewportW, viewportH)
			} else if dst, ok := a.tileRect(tile, x, y, viewportW, viewportH); ok {
				drawClippedTexture(renderer, tile.texture, dst, outline)
			}
		}
		return nil
	})
	if border {
		a.drawBoxParts(renderer, &st, popup, boxBorder)
	}
}

// drawClippedTexture draws texture over dst, only where it falls inside
// the convex polygon outline.
func drawClippedTexture(renderer *sdl.Renderer, texture *sdl.Texture, dst sdl.FRect, outline []point32) {
	polygon := clipConvex([]point32{{dst.X, dst.Y}, {dst.X + dst.W, dst.Y}, {dst.X + dst.W, dst.Y + dst.H}, {dst.X, dst.Y + dst.H}}, outline)
	if len(polygon) < 3 {
		return
	}
	white := sdl.FColor{R: 1, G: 1, B: 1, A: 1}
	vertices := make([]sdl.Vertex, len(polygon))
	for i, p := range polygon {
		vertices[i] = sdl.Vertex{Position: sdl.FPoint{X: p.x, Y: p.y}, Color: white, TexCoord: sdl.FPoint{X: (p.x - dst.X) / dst.W, Y: (p.y - dst.Y) / dst.H}}
	}
	indices := make([]int32, 0, 3*(len(polygon)-2))
	for i := 1; i+1 < len(polygon); i++ {
		indices = append(indices, 0, int32(i), int32(i+1))
	}
	sdl.RenderGeometry(renderer, texture, vertices, indices)
}

// clipConvex is the part of the convex polygon subject inside the convex
// polygon clip, by Sutherland and Hodgman's method.
func clipConvex(subject, clip []point32) []point32 {
	// Which side is inside depends on which way clip is wound.
	var area float32
	for i, p := range clip {
		q := clip[(i+1)%len(clip)]
		area += p.x*q.y - q.x*p.y
	}
	side := func(a, b, p point32) float32 {
		d := (b.x-a.x)*(p.y-a.y) - (b.y-a.y)*(p.x-a.x)
		if area < 0 {
			return -d
		}
		return d
	}
	out := subject
	for i, a := range clip {
		b := clip[(i+1)%len(clip)]
		if a == b {
			continue
		}
		in := out
		out = nil
		for j, p := range in {
			q := in[(j+1)%len(in)]
			sp, sq := side(a, b, p), side(a, b, q)
			if sp >= 0 {
				out = append(out, p)
			}
			if (sp >= 0) != (sq >= 0) {
				t := sp / (sp - sq)
				out = append(out, point32{p.x + (q.x-p.x)*t, p.y + (q.y-p.y)*t})
			}
		}
		if len(out) == 0 {
			return nil
		}
	}
	return out
}
