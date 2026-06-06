package viewer

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

func loadFont(path string, size int) font.Face {
	if path != "" {
		data, err := os.ReadFile(path)
		if err == nil {
			if col, err := opentype.ParseCollection(data); err == nil {
				if col.NumFonts() > 0 {
					fnt, err := col.Font(0)
					if err == nil {
						face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
							Size: float64(size),
							DPI:  72,
						})
						if err == nil {
							return face
						}
					}
				}
			} else if fnt, err := opentype.Parse(data); err == nil {
				face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
					Size: float64(size),
					DPI:  72,
				})
				if err == nil {
					return face
				}
			}
		}
	}
	return basicfont.Face7x13
}

func closeFontFace(face font.Face) {
	if closer, ok := face.(interface{ Close() error }); ok {
		_ = closer.Close()
	}
}

func textureFromRGBA(renderer *sdl.Renderer, rgba *image.RGBA) (*sdl.Texture, error) {
	tex := sdl.CreateTexture(renderer, sdl.PixelFormatRGBA32, sdl.TextureAccessStatic, int32(rgba.Bounds().Dx()), int32(rgba.Bounds().Dy()))
	if tex == nil {
		return nil, sdlError("create texture")
	}
	if len(rgba.Pix) > 0 {
		if !sdl.UpdateTexture(tex, nil, unsafe.Pointer(&rgba.Pix[0]), int32(rgba.Stride)) {
			sdl.DestroyTexture(tex)
			return nil, sdlError("update texture")
		}
	}
	if !sdl.SetTextureBlendMode(tex, sdl.BlendModeBlend) {
		sdl.DestroyTexture(tex)
		return nil, sdlError("set texture blend mode")
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
	tex, err := textureFromRGBA(renderer, img)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	return tex, width, height, ascent, nil
}

const maxTextTextureCacheEntries = 512

type textTextureKey struct {
	text       string
	r, g, b, a uint8
}

type cachedTextTexture struct {
	texture *sdl.Texture
	width   int
	height  int
	ascent  int
}

func (a *App) drawText(renderer *sdl.Renderer, s string, x, baselineY int, clr color.Color) error {
	entry, err := a.cachedTextTexture(renderer, s, clr)
	if err != nil {
		return err
	}
	dst := sdl.FRect{X: float32(x), Y: float32(baselineY - entry.ascent), W: float32(entry.width), H: float32(entry.height)}
	return renderBool(sdl.RenderTexture(renderer, entry.texture, nil, &dst), "render text")
}

func (a *App) cachedTextTexture(renderer *sdl.Renderer, s string, clr color.Color) (cachedTextTexture, error) {
	key := newTextTextureKey(s, clr)
	if a.textCache == nil {
		a.textCache = map[textTextureKey]cachedTextTexture{}
	}
	if entry, ok := a.textCache[key]; ok {
		return entry, nil
	}
	tex, w, h, ascent, err := textTexture(renderer, a.fontFace, s, clr)
	if err != nil {
		return cachedTextTexture{}, err
	}
	if len(a.textCache) >= maxTextTextureCacheEntries {
		a.clearTextTextureCache()
	}
	entry := cachedTextTexture{texture: tex, width: w, height: h, ascent: ascent}
	a.textCache[key] = entry
	return entry, nil
}

func newTextTextureKey(s string, clr color.Color) textTextureKey {
	r, g, b, a := clr.RGBA()
	return textTextureKey{text: s, r: uint8(r >> 8), g: uint8(g >> 8), b: uint8(b >> 8), a: uint8(a >> 8)}
}

func (a *App) clearTextTextureCache() {
	for _, entry := range a.textCache {
		if entry.texture != nil {
			sdl.DestroyTexture(entry.texture)
		}
	}
	a.textCache = nil
}

func fillRect(renderer *sdl.Renderer, rect sdl.FRect, clr color.RGBA) error {
	if !sdl.SetRenderDrawColor(renderer, clr.R, clr.G, clr.B, clr.A) {
		return sdlError("set draw color")
	}
	return renderBool(sdl.RenderFillRect(renderer, &rect), "fill rect")
}

func strokeRect(renderer *sdl.Renderer, rect sdl.FRect, clr color.RGBA, width int) error {
	if width < 1 {
		width = 1
	}
	if !sdl.SetRenderDrawColor(renderer, clr.R, clr.G, clr.B, clr.A) {
		return sdlError("set draw color")
	}
	for i := 0; i < width; i++ {
		inset := float32(i)
		r := sdl.FRect{X: rect.X + inset, Y: rect.Y + inset, W: rect.W - inset*2, H: rect.H - inset*2}
		if r.W <= 0 || r.H <= 0 {
			break
		}
		if !sdl.RenderRect(renderer, &r) {
			return sdlError("draw rect")
		}
	}
	return nil
}

func renderBool(ok bool, op string) error {
	if !ok {
		return sdlError(op)
	}
	return nil
}

func sdlError(op string) error {
	if err := sdl.GetError(); err != "" {
		return fmt.Errorf("SDL %s failed: %s", op, err)
	}
	return fmt.Errorf("SDL %s failed", op)
}
