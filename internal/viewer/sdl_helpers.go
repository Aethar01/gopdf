package viewer

import (
	"container/list"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"net/url"
	"os"
	"strconv"
	"strings"
	"unsafe"

	textfont "github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/fontscan"
	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

// fileBackedFontFace reads its font from an open file as needed, so it
// caches the per-rune metrics that measuring text looks up, which would
// otherwise read the file for every rune measured.
type fileBackedFontFace struct {
	font.Face
	file     *os.File
	advances map[rune]glyphAdvance
	kerns    map[[2]rune]fixed.Int26_6
}

type glyphAdvance struct {
	advance fixed.Int26_6
	ok      bool
}

func (f *fileBackedFontFace) GlyphAdvance(r rune) (fixed.Int26_6, bool) {
	if a, ok := f.advances[r]; ok {
		return a.advance, a.ok
	}
	advance, ok := f.Face.GlyphAdvance(r)
	f.advances[r] = glyphAdvance{advance, ok}
	return advance, ok
}

func (f *fileBackedFontFace) Kern(r0, r1 rune) fixed.Int26_6 {
	pair := [2]rune{r0, r1}
	if k, ok := f.kerns[pair]; ok {
		return k
	}
	k := f.Face.Kern(r0, r1)
	f.kerns[pair] = k
	return k
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
		var face font.Face
		var err error
		if strings.HasPrefix(path, "gopdf-font://") {
			face, err = loadSystemFont(path, size)
		} else {
			face, err = loadFontFile(path, size)
		}
		if err == nil {
			return face
		}
	}
	return basicfont.Face7x13
}

func loadSystemFont(selector string, size int) (font.Face, error) {
	location, err := resolveSystemFont(selector)
	if err != nil {
		return nil, err
	}
	return loadFontFileAt(location.File, size, int(location.Index))
}

func resolveSystemFont(selector string) (fontscan.Location, error) {
	u, err := url.Parse(selector)
	if err != nil {
		return fontscan.Location{}, err
	}
	family := strings.TrimSpace(u.Query().Get("family"))
	if family == "" {
		return fontscan.Location{}, fmt.Errorf("empty UI font family")
	}
	weight, err := strconv.Atoi(u.Query().Get("weight"))
	if err != nil || weight < 100 || weight > 900 {
		weight = 400
	}
	style := textfont.StyleNormal
	switch strings.ToLower(u.Query().Get("style")) {
	case "italic", "oblique":
		style = textfont.StyleItalic
	}

	fontMap := fontscan.NewFontMap(log.New(io.Discard, "", 0))
	if err := fontMap.UseSystemFonts(""); err != nil {
		return fontscan.Location{}, err
	}
	fontMap.SetQuery(fontscan.Query{
		Families: []string{family},
		Aspect: textfont.Aspect{
			Style:   style,
			Weight:  textfont.Weight(weight),
			Stretch: textfont.StretchNormal,
		},
	})
	face := fontMap.ResolveFace('M')
	if face == nil || face.Font == nil {
		return fontscan.Location{}, fmt.Errorf("no installed font matches %q", family)
	}
	location := fontMap.FontLocation(face.Font)
	if location.File == "" {
		return fontscan.Location{}, fmt.Errorf("no installed font location for %q", family)
	}
	return location, nil
}

func loadFontFile(path string, size int) (font.Face, error) {
	return loadFontFileAt(path, size, 0)
}

func loadFontFileAt(path string, size, collectionIndex int) (font.Face, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	reader, err := openTypeFontReaderAt(file, collectionIndex)
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
	return &fileBackedFontFace{Face: face, file: file, advances: map[rune]glyphAdvance{}, kerns: map[[2]rune]fixed.Int26_6{}}, nil
}

func openTypeFontReaderAt(file *os.File, collectionIndex int) (io.ReaderAt, error) {
	if collectionIndex < 0 {
		return nil, fmt.Errorf("negative font collection index")
	}
	var header [12]byte
	if _, err := file.ReadAt(header[:], 0); err != nil {
		return nil, err
	}
	if string(header[:4]) != "ttcf" {
		if collectionIndex != 0 {
			return nil, fmt.Errorf("font is not a collection")
		}
		return file, nil
	}
	numFonts := int(binary.BigEndian.Uint32(header[8:12]))
	if numFonts == 0 {
		return nil, fmt.Errorf("font collection contains no fonts")
	}
	if collectionIndex >= numFonts {
		return nil, fmt.Errorf("font collection index %d out of range %d", collectionIndex, numFonts)
	}
	var offsetBytes [4]byte
	if _, err := file.ReadAt(offsetBytes[:], int64(12+collectionIndex*4)); err != nil {
		return nil, err
	}
	return newTTCFontReaderAt(file, int64(binary.BigEndian.Uint32(offsetBytes[:])))
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
	return textureFromPixels(renderer, rgba.Bounds().Dx(), rgba.Bounds().Dy(), rgba.Pix, rgba.Stride)
}

func textureFromNRGBA(renderer *sdl.Renderer, img *image.NRGBA) (*sdl.Texture, error) {
	return textureFromPixels(renderer, img.Bounds().Dx(), img.Bounds().Dy(), img.Pix, img.Stride)
}

func textureFromPixels(renderer *sdl.Renderer, w, h int, pix []byte, stride int) (*sdl.Texture, error) {
	tex := sdl.CreateTexture(renderer, sdl.PixelFormatRGBA32, sdl.TextureAccessStatic, int32(w), int32(h))
	if tex == nil {
		return nil, sdlError("create texture")
	}
	if len(pix) > 0 {
		if !sdl.UpdateTexture(tex, nil, unsafe.Pointer(&pix[0]), int32(stride)) {
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
	if entry, ok := a.textCache.get(key); ok {
		return entry, nil
	}
	tex, w, h, ascent, err := textTexture(renderer, a.fontFace, s, clr)
	if err != nil {
		return cachedTextTexture{}, err
	}
	entry := cachedTextTexture{texture: tex, width: w, height: h, ascent: ascent}
	a.textCache.add(key, entry)
	return entry, nil
}

// textTextureCache keeps rendered strings, evicting the least recently used
// once full. Its zero value is empty and ready to use.
type textTextureCache struct {
	entries map[textTextureKey]*list.Element // values are textCacheEntry
	order   list.List
}

type textCacheEntry struct {
	key textTextureKey
	tex cachedTextTexture
}

func (c *textTextureCache) get(key textTextureKey) (cachedTextTexture, bool) {
	elem, ok := c.entries[key]
	if !ok {
		return cachedTextTexture{}, false
	}
	c.order.MoveToBack(elem)
	return elem.Value.(textCacheEntry).tex, true
}

func (c *textTextureCache) add(key textTextureKey, tex cachedTextTexture) {
	if c.entries == nil {
		c.entries = map[textTextureKey]*list.Element{}
	}
	if len(c.entries) >= maxTextTextureCacheEntries {
		oldest := c.order.Front()
		entry := c.order.Remove(oldest).(textCacheEntry)
		delete(c.entries, entry.key)
		destroyTexture(entry.tex.texture)
	}
	c.entries[key] = c.order.PushBack(textCacheEntry{key: key, tex: tex})
}

func (c *textTextureCache) clear() {
	for _, elem := range c.entries {
		destroyTexture(elem.Value.(textCacheEntry).tex.texture)
	}
	c.entries = nil
	c.order.Init()
}

func newTextTextureKey(s string, clr color.Color) textTextureKey {
	r, g, b, a := clr.RGBA()
	return textTextureKey{text: s, r: uint8(r >> 8), g: uint8(g >> 8), b: uint8(b >> 8), a: uint8(a >> 8)}
}

func (s *sdlState) clearTextTextureCache() {
	s.textCache.clear()
}

func (s *sdlState) Close() {
	s.clearTextTextureCache()
	closeFontFace(s.fontFace)
	s.fontFace = nil
	s.destroyCursors()
	if s.autoscrollMarker != nil {
		sdl.DestroyTexture(s.autoscrollMarker)
		s.autoscrollMarker = nil
	}
	s.altColorsShader.Close()
	s.altColorsShader = nil
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
