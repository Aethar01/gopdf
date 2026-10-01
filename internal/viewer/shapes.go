package viewer

import (
	"image/color"
	"math"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// px converts logical pixels, as the theme measures, to output pixels at
// the scale the UI font was loaded for.
func (a *App) px(v float64) float32 {
	scale := a.uiScale
	if scale <= 0 {
		scale = 1
	}
	return float32(v * scale)
}

// ipx is px rounded to whole output pixels.
func (a *App) ipx(v float64) int {
	return int(math.Round(float64(a.px(v))))
}

// hairline is the width of a one-logical-pixel line, never under one
// output pixel.
func (a *App) hairline() float32 {
	return max(1, float32(math.Round(float64(a.px(1)))))
}

// uiRadius is the theme's corner radius in output pixels.
func (a *App) uiRadius() float32 { return a.px(float64(a.config.Theme.Radius)) }

// uiPadding is the theme's padding in output pixels.
func (a *App) uiPadding() int { return a.ipx(float64(a.config.Theme.Padding)) }

// fillRoundedRect fills rect with corners of the given radius. The edge
// fades out over feather pixels centred on it, which antialiases it at one
// pixel and makes a soft shadow when wider.
func fillRoundedRect(renderer *sdl.Renderer, rect sdl.FRect, radius, feather float32, clr color.RGBA) {
	if rect.W <= 0 || rect.H <= 0 || clr.A == 0 {
		return
	}
	half := max(feather, 0) / 2
	shape := newRoundedShape(rect, radius, half)
	solid, clear := fcolor(clr), fcolor(clr)
	clear.A = 0
	inner := shape.outline(-half)
	outer := shape.outline(half)
	vertices := make([]sdl.Vertex, 0, 1+len(inner)+len(outer))
	vertices = append(vertices, sdl.Vertex{Position: sdl.FPoint{X: rect.X + rect.W/2, Y: rect.Y + rect.H/2}, Color: solid})
	for _, p := range inner {
		vertices = append(vertices, sdl.Vertex{Position: p, Color: solid})
	}
	for _, p := range outer {
		vertices = append(vertices, sdl.Vertex{Position: p, Color: clear})
	}
	n := int32(len(inner))
	indices := make([]int32, 0, 9*n)
	for i := range n {
		j := (i + 1) % n
		indices = append(indices, 0, 1+i, 1+j)                        // the fan inside
		indices = append(indices, 1+i, 1+n+i, 1+n+j, 1+i, 1+n+j, 1+j) // the fading edge
	}
	sdl.RenderGeometry(renderer, nil, vertices, indices)
}

// strokeRoundedRect draws a line width pixels wide just inside the edge of
// rect, antialiased.
func strokeRoundedRect(renderer *sdl.Renderer, rect sdl.FRect, radius, width float32, clr color.RGBA) {
	if rect.W <= 0 || rect.H <= 0 || width <= 0 || clr.A == 0 {
		return
	}
	shape := newRoundedShape(rect, radius, width+0.5)
	solid, clear := fcolor(clr), fcolor(clr)
	clear.A = 0
	// Bands from the inside out: fade in, solid, fade out.
	grows := []float32{-width - 0.5, -width + 0.5, -0.5, 0.5}
	colors := []sdl.FColor{clear, solid, solid, clear}
	if width <= 1 {
		grows, colors = []float32{-width - 0.5, -0.5, 0.5}, []sdl.FColor{clear, solid, clear}
	}
	var vertices []sdl.Vertex
	var n int32
	for i, grow := range grows {
		points := shape.outline(grow)
		n = int32(len(points))
		for _, p := range points {
			vertices = append(vertices, sdl.Vertex{Position: p, Color: colors[i]})
		}
	}
	var indices []int32
	for band := range int32(len(grows) - 1) {
		in, out := band*n, (band+1)*n
		for i := range n {
			j := (i + 1) % n
			indices = append(indices, in+i, out+i, out+j, in+i, out+j, in+j)
		}
	}
	sdl.RenderGeometry(renderer, nil, vertices, indices)
}

func fcolor(c color.RGBA) sdl.FColor {
	return sdl.FColor{R: float32(c.R) / 255, G: float32(c.G) / 255, B: float32(c.B) / 255, A: float32(c.A) / 255}
}

// roundedShape is a rounded rectangle whose outline can be grown or shrunk
// by a distance, with the same number of points at every distance so that
// outlines join into bands.
type roundedShape struct {
	centers [4]sdl.FPoint // corner centres, clockwise from the top left
	radius  float32       // of the corners at distance 0
	steps   int           // points per corner
}

// newRoundedShape makes the shape of rect; reach is the furthest any
// outline will shrink, which the corner centres must allow for.
func newRoundedShape(rect sdl.FRect, radius, reach float32) roundedShape {
	r := min(max(radius, reach), rect.W/2, rect.H/2)
	r = max(r, 0)
	return roundedShape{
		centers: [4]sdl.FPoint{
			{X: rect.X + r, Y: rect.Y + r},
			{X: rect.X + rect.W - r, Y: rect.Y + r},
			{X: rect.X + rect.W - r, Y: rect.Y + rect.H - r},
			{X: rect.X + r, Y: rect.Y + rect.H - r},
		},
		radius: r,
		steps:  max(2, min(16, int(r/2)+2)),
	}
}

// outline is the points of the shape grown outwards by grow pixels.
func (s roundedShape) outline(grow float32) []sdl.FPoint {
	r := float64(max(0, s.radius+grow))
	points := make([]sdl.FPoint, 0, 4*s.steps)
	for corner, c := range s.centers {
		start := math.Pi + float64(corner)*math.Pi/2 // top left starts pointing left
		for i := range s.steps {
			angle := start + float64(i)/float64(s.steps-1)*math.Pi/2
			points = append(points, sdl.FPoint{X: c.X + float32(r*math.Cos(angle)), Y: c.Y + float32(r*math.Sin(angle))})
		}
	}
	return points
}

// drawShadow draws the theme's soft shadow under a floating rect.
func (a *App) drawShadow(renderer *sdl.Renderer, rect sdl.FRect, radius float32) {
	if !a.config.Theme.Shadow {
		return
	}
	alpha := uint8(0x22)
	if !isLight(a.backgroundColor()) {
		alpha = 0x55
	}
	blur := a.px(14)
	offset := a.px(3)
	shadow := sdl.FRect{X: rect.X, Y: rect.Y + offset, W: rect.W, H: rect.H}
	fillRoundedRect(renderer, shadow, radius+blur/2, blur, color.RGBA{A: alpha})
	fillRoundedRect(renderer, sdl.FRect{X: rect.X, Y: rect.Y + offset/3, W: rect.W, H: rect.H}, radius+a.px(1), a.px(3), color.RGBA{A: alpha / 2})
}

// drawPanel draws a floating panel: shadow, fill and hairline border.
func (a *App) drawPanel(renderer *sdl.Renderer, rect sdl.FRect, radius float32) {
	a.drawShadow(renderer, rect, radius)
	fillRoundedRect(renderer, rect, radius, 1, a.panelColor())
	strokeRoundedRect(renderer, rect, radius, a.hairline(), a.borderColor())
}

// drawTextHighlight marks rect on a page the way a highlighter would: on a
// light page it multiplies, so the text under it stays dark, and on a dark
// page it adds, so light text stays light.
func (a *App) drawTextHighlight(renderer *sdl.Renderer, rect sdl.FRect, clr color.RGBA) {
	mode := sdl.BlendMode(sdl.BlendModeMod)
	if !isLight(a.pageBackgroundColor()) {
		mode = sdl.BlendModeAdd
		clr.A = 0xa0
	}
	sdl.SetRenderDrawBlendMode(renderer, mode)
	fillRect(renderer, rect, clr)
	sdl.SetRenderDrawBlendMode(renderer, sdl.BlendModeBlend)
}

// withClip runs draw with drawing limited to rect, inside any clip already
// in place, so content such as scrolled text stays within its box.
func (a *App) withClip(renderer *sdl.Renderer, rect sdl.FRect, draw func() error) error {
	clip := sdl.Rect{
		X: int32(math.Floor(float64(rect.X))),
		Y: int32(math.Floor(float64(rect.Y))),
		W: int32(math.Ceil(float64(rect.X+rect.W))) - int32(math.Floor(float64(rect.X))),
		H: int32(math.Ceil(float64(rect.Y+rect.H))) - int32(math.Floor(float64(rect.Y))),
	}
	if n := len(a.clips); n > 0 {
		clip = intersectRects(clip, a.clips[n-1])
	}
	a.clips = append(a.clips, clip)
	sdl.SetRenderClipRect(renderer, &clip)
	defer func() {
		a.clips = a.clips[:len(a.clips)-1]
		if n := len(a.clips); n > 0 {
			sdl.SetRenderClipRect(renderer, &a.clips[n-1])
		} else {
			sdl.SetRenderClipRect(renderer, nil)
		}
	}()
	return draw()
}

func intersectRects(a, b sdl.Rect) sdl.Rect {
	x0, y0 := max(a.X, b.X), max(a.Y, b.Y)
	x1, y1 := min(a.X+a.W, b.X+b.W), min(a.Y+a.H, b.Y+b.H)
	return sdl.Rect{X: x0, Y: y0, W: max(0, x1-x0), H: max(0, y1-y0)}
}
