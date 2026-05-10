package viewer

import (
	"image"
	"image/color"
	"image/draw"
	"unsafe"

	"github.com/veandco/go-sdl2/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

func imageToRGBA(src image.Image) *image.RGBA {
	if rgba, ok := src.(*image.RGBA); ok {
		return rgba
	}
	bounds := src.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, src, bounds.Min, draw.Src)
	return rgba
}

func textureFromImage(renderer *sdl.Renderer, src image.Image) (*sdl.Texture, error) {
	rgba := imageToRGBA(src)
	tex, err := renderer.CreateTexture(uint32(sdl.PIXELFORMAT_RGBA32), sdl.TEXTUREACCESS_STATIC, int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()))
	if err != nil {
		return nil, err
	}
	if len(rgba.Pix) > 0 {
		if err := tex.Update(nil, unsafe.Pointer(&rgba.Pix[0]), rgba.Stride); err != nil {
			tex.Destroy()
			return nil, err
		}
	}
	if err := tex.SetBlendMode(sdl.BLENDMODE_BLEND); err != nil {
		tex.Destroy()
		return nil, err
	}
	return tex, nil
}

func measureText(face font.Face, s string) int {
	if s == "" {
		return 0
	}
	var d font.Drawer
	d.Face = face
	return d.MeasureString(s).Ceil()
}

func textTexture(renderer *sdl.Renderer, face font.Face, s string, clr color.Color) (*sdl.Texture, int, int, int, error) {
	width := measureText(face, s)
	metrics := face.Metrics()
	ascent := metrics.Ascent.Ceil()
	height := metrics.Height.Ceil()
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(clr),
		Face: face,
		Dot:  fixed.P(0, ascent),
	}
	d.DrawString(s)
	tex, err := textureFromImage(renderer, img)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return tex, width, height, ascent, nil
}

func drawText(renderer *sdl.Renderer, face font.Face, s string, x, baselineY int, clr color.Color) error {
	tex, w, h, ascent, err := textTexture(renderer, face, s, clr)
	if err != nil {
		return err
	}
	defer tex.Destroy()
	return renderer.Copy(tex, nil, &sdl.Rect{X: int32(x), Y: int32(baselineY - ascent), W: int32(w), H: int32(h)})
}

func fillRect(renderer *sdl.Renderer, rect sdl.FRect, clr color.RGBA) error {
	if err := renderer.SetDrawColor(clr.R, clr.G, clr.B, clr.A); err != nil {
		return err
	}
	return renderer.FillRectF(&rect)
}

func strokeRect(renderer *sdl.Renderer, rect sdl.FRect, clr color.RGBA, width int) error {
	if width < 1 {
		width = 1
	}
	if err := renderer.SetDrawColor(clr.R, clr.G, clr.B, clr.A); err != nil {
		return err
	}
	for i := 0; i < width; i++ {
		inset := float32(i)
		r := sdl.FRect{X: rect.X + inset, Y: rect.Y + inset, W: rect.W - inset*2, H: rect.H - inset*2}
		if r.W <= 0 || r.H <= 0 {
			break
		}
		if err := renderer.DrawRectF(&r); err != nil {
			return err
		}
	}
	return nil
}
