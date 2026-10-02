package viewer

import (
	"encoding/binary"
	"math"
	"time"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// runDrawFunction draws a box with its style's draw function, reporting
// whether the function drew the default box. If the function fails, the
// error is reported, the function is not called again until the config
// changes, and the box is drawn as it would be without it.
func (a *App) runDrawFunction(renderer *sdl.Renderer, st *config.Style, rect sdl.FRect, parts boxParts) bool {
	return a.drawElement(renderer, st, rect, func() { a.drawDefaultBox(renderer, st, rect, parts) })
}

// drawElement draws an element over rect: with its style's draw function
// if it has one, canvas:default() running drawDefault, and otherwise with
// drawDefault. It reports whether the default was drawn.
func (a *App) drawElement(renderer *sdl.Renderer, st *config.Style, rect sdl.FRect, drawDefault func()) bool {
	if st.Draw.V == nil || a.themeErrors[st.Draw.V] {
		drawDefault()
		return true
	}
	box := pixelRect(rect)
	canvas := &elementCanvas{app: a, renderer: renderer, style: st, box: box, drawDefault: drawDefault}
	scale := float64(a.px(1))
	state := config.DrawState{
		Element: st.Element,
		X:       float64(box.X) / scale, Y: float64(box.Y) / scale,
		W: float64(box.W) / scale, H: float64(box.H) / scale,
		Alt:  a.altColors,
		Time: time.Since(startTime).Seconds(),
	}
	if err := a.runtime.RunDraw(st.Draw.V, canvas, state); err != nil {
		a.reportThemeError(st.Draw.V, "theme draw "+st.Element.String(), err)
		if !canvas.drewDefault {
			canvas.Default()
		}
	}
	return canvas.drewDefault
}

// elementCanvas is the canvas a draw function draws an element's box on.
type elementCanvas struct {
	app         *App
	renderer    *sdl.Renderer
	style       *config.Style
	box         sdl.Rect
	drawDefault func()
	drewDefault bool
}

func (c *elementCanvas) Default() {
	if c.drewDefault {
		return
	}
	c.drewDefault = true
	c.drawDefault()
}

func (c *elementCanvas) Fill(path []config.PathOp, clr config.Color) {
	c.drawPath(path, clr, 0)
}

func (c *elementCanvas) Stroke(path []config.PathOp, clr config.Color, width float64) {
	c.drawPath(path, clr, c.app.lineWidth(width))
}

// drawPath fills path, or with stroke strokes it, as a shape on the box,
// so its mask is kept as a shape's is.
func (c *elementCanvas) drawPath(path []config.PathOp, clr config.Color, stroke float32) {
	if len(path) == 0 || c.box.W <= 0 || c.box.H <= 0 {
		return
	}
	shape := boxShape{Shape: config.Shape{Kind: "path"}, ops: c.app.pathKeys.pack(path), scale: c.app.px(1)}
	spec := maskSpec{shape: shape, w: c.box.W, h: c.box.H, stroke: stroke}
	if rgba := c.app.styleColor(clr, c.style.Opacity.V); rgba.A > 0 {
		c.app.drawMask(c.renderer, spec, c.box.X, c.box.Y, rgba)
	}
}

// pathKeys packs paths for the mask cache's keys, keeping the strings
// it made so a path drawn on every frame is packed without allocating.
type pathKeys struct {
	buf  []byte
	keys map[string]string
}

// maxPathKeys bounds the strings kept, which are let go when full, in
// case a function draws ever new paths.
const maxPathKeys = 256

// pathOpBytes is the size of a packed path op: its op and three points
// of two lengths of two float64s.
const pathOpBytes = 1 + 3*4*8

// pack is ops packed: each op's byte and the exact bits of its points,
// cheaper to make than its path data and as unique.
func (k *pathKeys) pack(ops []config.PathOp) string {
	buf := k.buf[:0]
	for _, op := range ops {
		buf = append(buf, op.Op)
		for _, p := range op.Pts {
			for _, v := range [4]float64{p.X.Frac, p.X.Px, p.Y.Frac, p.Y.Px} {
				buf = binary.LittleEndian.AppendUint64(buf, math.Float64bits(v))
			}
		}
	}
	k.buf = buf
	if key, ok := k.keys[string(buf)]; ok {
		return key
	}
	if k.keys == nil || len(k.keys) >= maxPathKeys {
		k.keys = map[string]string{}
	}
	key := string(buf)
	k.keys[key] = key
	return key
}

// unpackPath is the path pack packed.
func unpackPath(packed string) []config.PathOp {
	ops := make([]config.PathOp, 0, len(packed)/pathOpBytes)
	for b := []byte(packed); len(b) >= pathOpBytes; b = b[pathOpBytes:] {
		op := config.PathOp{Op: b[0]}
		next := func(i int) float64 { return math.Float64frombits(binary.LittleEndian.Uint64(b[1+8*i:])) }
		for i := range op.Pts {
			op.Pts[i] = config.PathPoint{
				X: config.Length{Frac: next(4 * i), Px: next(4*i + 1)},
				Y: config.Length{Frac: next(4*i + 2), Px: next(4*i + 3)},
			}
		}
		ops = append(ops, op)
	}
	return ops
}

func (c *elementCanvas) Text(x, y float64, text string, clr config.Color, bold bool) float64 {
	face := c.app.fontFace
	if bold {
		face = c.app.headingFont()
	}
	if face == nil {
		return 0
	}
	left := int(c.box.X) + int(math.Round(float64(c.app.px(x))))
	baseline := int(c.box.Y) + int(math.Round(float64(c.app.px(y)))) + face.Metrics().Ascent.Ceil()
	c.app.drawTextFace(c.renderer, text, left, baseline, c.app.styleColor(clr, c.style.Opacity.V), bold)
	return float64(measureText(face, text)) / float64(c.app.px(1))
}

func (c *elementCanvas) Measure(text string, bold bool) (w, h float64) {
	face := c.app.fontFace
	if bold {
		face = c.app.headingFont()
	}
	if face == nil {
		return 0, 0
	}
	scale := float64(c.app.px(1))
	return float64(measureText(face, text)) / scale, float64(c.app.uiLineHeight()) / scale
}
