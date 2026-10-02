package viewer

import (
	"image"
	"image/color"
	"math"
)

// glyphGrid is the size of the square grid glyph shapes are laid out on.
const glyphGrid = 32

// glyph is a small overlay shape drawn from signed distance fields on a
// glyphGrid unit square, so it rasterizes crisply at any display scale.
// Distances are negative inside a shape.
type glyph struct {
	shape   func(x, y float64) float64 // the filled silhouette
	detail  func(x, y float64) float64 // marks drawn in the outline color over the fill, or nil
	fill    color.NRGBA
	outline color.NRGBA
	stroke  float64 // outline width in grid units
}

// rasterize draws the glyph as a size by size straight-alpha image.
func (g glyph) rasterize(size int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	unit := glyphGrid / float64(size)
	coverage := func(d float64) float64 { return clamp01(0.5 - d/unit) }
	for py := range size {
		for px := range size {
			x, y := (float64(px)+0.5)*unit, (float64(py)+0.5)*unit
			d := g.shape(x, y)
			outer := coverage(d - g.stroke)
			if outer == 0 {
				continue
			}
			inner := coverage(d)
			if g.detail != nil {
				inner *= 1 - coverage(g.detail(x, y))
			}
			// The fill composited over the outline.
			top := inner * float64(g.fill.A) / 255
			bottom := outer * float64(g.outline.A) / 255 * (1 - top)
			alpha := top + bottom
			if alpha == 0 {
				continue
			}
			mix := func(f, o uint8) uint8 {
				return uint8(math.Round((float64(f)*top + float64(o)*bottom) / alpha))
			}
			img.SetNRGBA(px, py, color.NRGBA{
				R: mix(g.fill.R, g.outline.R),
				G: mix(g.fill.G, g.outline.G),
				B: mix(g.fill.B, g.outline.B),
				A: uint8(math.Round(alpha * 255)),
			})
		}
	}
	return img
}

func clamp01(v float64) float64 {
	return math.Max(0, math.Min(1, v))
}

func sdCircle(x, y, cx, cy, r float64) float64 {
	return math.Hypot(x-cx, y-cy) - r
}

// sdPolygon is the signed distance to a closed polygon.
func sdPolygon(x, y float64, v [][2]float64) float64 {
	d := (x-v[0][0])*(x-v[0][0]) + (y-v[0][1])*(y-v[0][1])
	sign := 1.0
	for i, j := 0, len(v)-1; i < len(v); j, i = i, i+1 {
		ex, ey := v[j][0]-v[i][0], v[j][1]-v[i][1]
		wx, wy := x-v[i][0], y-v[i][1]
		h := clamp01((wx*ex + wy*ey) / (ex*ex + ey*ey))
		bx, by := wx-ex*h, wy-ey*h
		d = math.Min(d, bx*bx+by*by)
		below, above, left := y >= v[i][1], y < v[j][1], ex*wy > ey*wx
		if below && above && left || !below && !above && !left {
			sign = -sign
		}
	}
	return sign * math.Sqrt(d)
}

// autoscrollArrow is a triangle pointing away from the grid center at
// angle radians clockwise from north, spanning inner to outer grid units
// from the center.
func autoscrollArrow(x, y, angle, inner, outer, halfWidth float64) float64 {
	const c = glyphGrid / 2
	sin, cos := math.Sincos(angle)
	point := func(along, across float64) [2]float64 {
		return [2]float64{c + along*sin + across*cos, c - along*cos + across*sin}
	}
	return sdPolygon(x, y, [][2]float64{point(outer, 0), point(inner, halfWidth), point(inner, -halfWidth)})
}

// autoscrollArrows are the arrows for the scrollable axes, as angles.
func autoscrollArrows(horizontal, vertical bool) []float64 {
	var angles []float64
	if vertical {
		angles = append(angles, 0, math.Pi)
	}
	if horizontal {
		angles = append(angles, math.Pi/2, -math.Pi/2)
	}
	return angles
}

// autoscrollMarkerGlyph is drawn on the page at the autoscroll anchor, in
// fill with an outline stroke grid units wide.
func autoscrollMarkerGlyph(horizontal, vertical bool, fill, outline color.NRGBA, stroke float64) glyph {
	angles := autoscrollArrows(horizontal, vertical)
	return glyph{
		shape: func(x, y float64) float64 {
			return sdCircle(x, y, glyphGrid/2, glyphGrid/2, 14)
		},
		detail: func(x, y float64) float64 {
			d := sdCircle(x, y, glyphGrid/2, glyphGrid/2, 1.8)
			for _, angle := range angles {
				d = math.Min(d, autoscrollArrow(x, y, angle, 6.5, 11.5, 3.6))
			}
			return d
		},
		fill:    fill,
		outline: outline,
		stroke:  stroke,
	}
}
