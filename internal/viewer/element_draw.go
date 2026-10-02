package viewer

import (
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
	shape := boxShape{Shape: config.Shape{Kind: "path", Path: config.FormatPath(path)}, scale: c.app.px(1)}
	spec := maskSpec{shape: shape, w: c.box.W, h: c.box.H, stroke: stroke}
	if rgba := c.app.styleColor(clr, c.style.Opacity.V); rgba.A > 0 {
		c.app.drawMask(c.renderer, spec, c.box.X, c.box.Y, rgba)
	}
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
