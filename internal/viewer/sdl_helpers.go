package viewer

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"os"
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

type fileBackedFontFace struct {
	font.Face
	file *os.File
}

func (f *fileBackedFontFace) Close() error {
	var faceErr error
	if closer, ok := f.Face.(interface{ Close() error }); ok {
		faceErr = closer.Close()
	}
	fileErr := f.file.Close()
	if faceErr != nil {
		return faceErr
	}
	return fileErr
}

type ttcFontReaderAt struct {
	source    *os.File
	directory []byte
	shift     int64
}

func (r *ttcFontReaderAt) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 {
		return 0, fmt.Errorf("negative font read offset")
	}

	total := 0
	if off < r.shift {
		start := int(off)
		n := len(p)
		if remaining := len(r.directory) - start; n > remaining {
			n = remaining
		}
		copy(p[:n], r.directory[start:start+n])
		total += n
		p = p[n:]
		off += int64(n)
		if len(p) == 0 {
			return total, nil
		}
	}

	n, err := r.source.ReadAt(p, off-r.shift)
	return total + n, err
}

func loadFont(path string, size int) font.Face {
	if path != "" {
		if face, err := loadFontFile(path, size); err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func loadFontFile(path string, size int) (font.Face, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader, err := firstOpenTypeFontReader(file)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	fnt, err := opentype.ParseReaderAt(reader)
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
		Size: float64(size),
		DPI:  72,
	})
	if err != nil {
		_ = file.Close()
		return nil, err
	}
	return &fileBackedFontFace{Face: face, file: file}, nil
}

func firstOpenTypeFontReader(file *os.File) (io.ReaderAt, error) {
	var header [16]byte
	if _, err := file.ReadAt(header[:], 0); err != nil {
		return nil, err
	}
	if string(header[:4]) != "ttcf" {
		return file, nil
	}
	if binary.BigEndian.Uint32(header[8:12]) == 0 {
		return nil, fmt.Errorf("font collection contains no fonts")
	}
	return newTTCFontReaderAt(file, int64(binary.BigEndian.Uint32(header[12:16])))
}

func newTTCFontReaderAt(file *os.File, fontOffset int64) (io.ReaderAt, error) {
	var header [12]byte
	if _, err := file.ReadAt(header[:], fontOffset); err != nil {
		return nil, err
	}

	numTables := int(binary.BigEndian.Uint16(header[4:6]))
	directorySize := 12 + 16*numTables
	directory := make([]byte, directorySize)
	if _, err := file.ReadAt(directory, fontOffset); err != nil {
		return nil, err
	}

	shift := uint32(directorySize)
	for i := 0; i < numTables; i++ {
		offsetPos := 12 + i*16 + 8
		offset := binary.BigEndian.Uint32(directory[offsetPos : offsetPos+4])
		if offset > ^uint32(0)-shift {
			return nil, fmt.Errorf("font table offset overflow")
		}
		binary.BigEndian.PutUint32(directory[offsetPos:offsetPos+4], offset+shift)
	}

	return &ttcFontReaderAt{
		source:    file,
		directory: directory,
		shift:     int64(shift),
	}, nil
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

func (s *sdlState) clearTextTextureCache() {
	for _, entry := range s.textCache {
		if entry.texture != nil {
			sdl.DestroyTexture(entry.texture)
		}
	}
	s.textCache = nil
}

func (s *sdlState) Close() {
	s.clearTextTextureCache()
	closeFontFace(s.fontFace)
	s.fontFace = nil
	if s.cursorHand != nil {
		sdl.DestroyCursor(s.cursorHand)
		s.cursorHand = nil
	}
	if s.cursorArrow != nil {
		sdl.DestroyCursor(s.cursorArrow)
		s.cursorArrow = nil
	}
	if s.renderer != nil {
		sdl.DestroyRenderer(s.renderer)
		s.renderer = nil
	}
	if s.window != nil {
		sdl.DestroyWindow(s.window)
		s.window = nil
	}
	sdl.Quit()
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
