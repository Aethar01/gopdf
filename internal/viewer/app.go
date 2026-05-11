package viewer

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/veandco/go-sdl2/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/font/opentype"
)

type mode int

const (
	modeNormal mode = iota
	modeCommand
	modeGotoPage
	modeSearch
)

type renderedPage struct {
	texture *sdl.Texture
	width   float64
	height  float64
	pixX    float64
	pixY    float64
	key     string
	page    int
	scale   float64
}

type pageMetrics struct {
	bounds mupdf.Rect
	width  float64
	height float64
}

type textSelection struct {
	active bool
	page   int
	anchor mupdf.Point
	focus  mupdf.Point
	quads  []mupdf.Quad
	text   string
}

type zoomAnchor struct {
	page    int
	point   mupdf.Point
	valid   bool
	centerX float64
	centerY float64
}

type rowLayout struct {
	pages  []int
	x      float64
	y      float64
	width  float64
	height float64
	pageX  []float64
	pageY  []float64
	pageW  []float64
	pageH  []float64
}

type App struct {
	docPath     string
	docName     string
	doc         *mupdf.Document
	runtime     *config.Runtime
	config      config.Config
	window      *sdl.Window
	renderer    *sdl.Renderer
	cursorHand  *sdl.Cursor
	cursorArrow *sdl.Cursor

	pageCount  int
	page       int
	rotation   float64
	zoom       float64
	fitMode    string
	renderMode string
	scale      float64

	dualPage        bool
	firstPageOffset bool
	statusBarShown  bool
	fullscreen      bool
	scrollX         float64
	scrollY         float64
	pageStep        float64
	altColors       bool

	pageMetrics []pageMetrics
	rows        []rowLayout
	pageToRow   []int
	contentW    float64
	contentH    float64

	winW int
	winH int

	renderCache        map[string]*renderedPage
	renderOrder        []string
	cacheLimit         int
	renderBaseScale    float64
	minRenderBaseScale float64
	renderGeneration   int
	renderPending      map[string]renderRequest
	renderWorker       *renderWorker
	renderScaleTime    float64
	pendingRedraw      bool

	fontFace      font.Face
	mode          mode
	input         string
	inputCursor   int
	ignoreText    string
	message       string
	mouseBindings map[string]string
	searchInput   searchMode

	sequence       []string
	sequenceAt     time.Time
	sequenceLookup map[string]string
	pendingCount   string

	lastErr      error
	quit         bool
	pendingOpen  string
	selection    textSelection
	panning      bool
	panButton    uint8
	panKey       string
	mouseButton  uint8
	actionKey    string
	pageLinks    map[int][]mupdf.Link
	search       searchState
	searchWorker *searchWorker
	outline      []mupdf.OutlineItem
	outlineMenu  outlineMenuState

	jumpBack  []jumpPosition
	jumpAhead []jumpPosition
}

type jumpPosition struct {
	page    int
	scrollX float64
	scrollY float64
}

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

func New(docPath string, runtime *config.Runtime, startPage int) (*App, error) {
	cfg := runtime.Config()
	doc, err := mupdf.Open(docPath)
	if err != nil {
		return nil, err
	}
	pages, err := doc.PageCount()
	if err != nil {
		doc.Close()
		return nil, err
	}
	if startPage < 0 {
		startPage = 0
	}
	if startPage >= pages {
		startPage = pages - 1
	}
	app := &App{
		docPath:            docPath,
		docName:            filepath.Base(docPath),
		doc:                doc,
		runtime:            runtime,
		config:             cfg,
		pageCount:          pages,
		page:               startPage,
		zoom:               1,
		fitMode:            sanitizeFitMode(cfg.FitMode),
		renderMode:         sanitizeRenderMode(cfg.RenderMode),
		scale:              1,
		altColors:          cfg.AltColors,
		dualPage:           cfg.DualPage,
		firstPageOffset:    cfg.FirstPageOffset,
		statusBarShown:     cfg.StatusBarVisible,
		pageStep:           64,
		renderCache:        map[string]*renderedPage{},
		cacheLimit:         minInt(24, pages),
		minRenderBaseScale: 0.25,
		fontFace:           loadFont(cfg.UIFontPath, cfg.UIFontSize),
		message:            cfg.NormalMessage,
		mouseBindings:      map[string]string{},
		pageLinks:          map[int][]mupdf.Link{},
		sequenceLookup:     map[string]string{},
	}
	runtime.AttachHost(app)
	for k, v := range cfg.KeyBindings {
		app.sequenceLookup[normalizeBinding(k)] = v
	}
	for k, v := range cfg.MouseBindings {
		app.mouseBindings[k] = v
	}
	app.initRenderWorker()
	app.initSearch()
	if outline, err := doc.Outline(); err == nil {
		app.outline = outline
	} else {
		app.message = err.Error()
	}
	if err := app.loadPageMetrics(); err != nil {
		app.closeRenderWorker()
		app.closeSearch()
		doc.Close()
		return nil, err
	}
	app.recomputeLayout(1400, 900-app.config.StatusBarHeight)
	app.ensureRenderBaseScale()
	app.alignPageTop(startPage)
	return app, nil
}

func (a *App) Close() {
	a.closeRenderWorker()
	a.closeSearch()
	a.clearCache()
	if a.cursorHand != nil {
		sdl.FreeCursor(a.cursorHand)
		a.cursorHand = nil
	}
	if a.cursorArrow != nil {
		sdl.FreeCursor(a.cursorArrow)
		a.cursorArrow = nil
	}
	if a.renderer != nil {
		a.renderer.Destroy()
		a.renderer = nil
	}
	if a.window != nil {
		a.window.Destroy()
		a.window = nil
	}
	sdl.Quit()
	if a.doc != nil {
		a.doc.Close()
	}
	if a.docPath != "" {
		config.SetLastFile(a.docPath)
	}
}

func (a *App) PendingOpen() string {
	return a.pendingOpen
}

func (a *App) Run() error {
	if err := sdl.Init(sdl.INIT_VIDEO); err != nil {
		return err
	}
	window, renderer, err := sdl.CreateWindowAndRenderer(1400, 900, sdl.WINDOW_RESIZABLE|sdl.WINDOW_ALLOW_HIGHDPI)
	if err != nil {
		sdl.Quit()
		return err
	}
	a.window = window
	a.renderer = renderer
	a.cursorHand = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_HAND)
	a.cursorArrow = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	a.window.SetTitle(a.docName + " - gopdf")
	a.renderer.SetDrawBlendMode(sdl.BLENDMODE_BLEND)
	if w, h, err := a.renderer.GetOutputSize(); err == nil {
		a.winW, a.winH = int(w), int(h)
	}
	a.recomputeLayout(a.viewportSize())
	a.pendingRedraw = true
	sdl.StartTextInput()
	defer sdl.StopTextInput()
	for !a.quit {
		for event := sdl.PollEvent(); event != nil; event = sdl.PollEvent() {
			if err := a.handleSDLEvent(event); err != nil {
				return err
			}
		}
		a.pollRenderUpdates()
		a.pollSearchUpdates()
		a.expireSequence()
		a.prefetchVisiblePages()
		a.adjustRenderBaseScaleForExtremeZoom(a.scale)
		if a.pendingRedraw {
			if err := a.drawFrame(); err != nil {
				return err
			}
		}
		if !a.quit {
			if event := sdl.WaitEventTimeout(a.eventWaitTimeoutMS()); event != nil {
				if err := a.handleSDLEvent(event); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (a *App) eventWaitTimeoutMS() int {
	if len(a.renderPending) > 0 || a.search.running {
		return 16
	}
	if len(a.sequence) > 0 {
		elapsed := time.Since(a.sequenceAt)
		remaining := time.Duration(a.config.SequenceTimeoutMS)*time.Millisecond - elapsed
		if remaining <= 0 {
			return 1
		}
		if remaining < 100*time.Millisecond {
			return maxInt(1, int(remaining/time.Millisecond))
		}
	}
	return 100
}

func (a *App) handleSDLEvent(event sdl.Event) error {
	defer func() { a.pendingRedraw = true }()
	switch e := event.(type) {
	case *sdl.QuitEvent:
		a.quit = true
	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_SIZE_CHANGED {
			a.winW = int(e.Data1)
			a.winH = int(e.Data2)
			a.recomputeLayout(a.viewportSize())
		}
	case *sdl.KeyboardEvent:
		if e.Type == sdl.KEYUP {
			a.handleSDLKeyUp(e)
		}
		if e.Type == sdl.KEYDOWN && e.Repeat == 0 {
			a.handleSDLKeyDown(e)
		}
	case *sdl.TextInputEvent:
		a.handleSDLTextInput(e)
	case *sdl.MouseWheelEvent:
		a.handleSDLMouseWheel(e)
	case *sdl.MouseButtonEvent:
		a.handleSDLMouseButton(e)
	case *sdl.MouseMotionEvent:
		a.handleSDLMouseMotion(e)
	}
	return nil
}

func (a *App) drawFrame() error {
	if a.renderer == nil {
		return nil
	}
	if w, h, err := a.renderer.GetOutputSize(); err == nil {
		a.winW, a.winH = int(w), int(h)
	}
	bg := a.backgroundColor()
	if err := a.renderer.SetDrawColor(bg.R, bg.G, bg.B, bg.A); err != nil {
		return err
	}
	if err := a.renderer.Clear(); err != nil {
		return err
	}
	a.drawPages(a.renderer)
	if a.pendingRedraw {
		a.pendingRedraw = false
	}
	if a.statusVisible() {
		if err := a.drawStatusBar(a.renderer); err != nil {
			return err
		}
	}
	if a.outlineMenu.visible {
		if err := a.drawOutlineMenu(a.renderer); err != nil {
			return err
		}
	}
	a.renderer.Present()
	return nil
}

func (a *App) handleSDLKeyDown(e *sdl.KeyboardEvent) {
	if a.outlineMenu.visible && a.handleOutlineMenuKey(e) {
		return
	}
	if a.mode != modeNormal {
		if token, ok := keyToken(e.Keysym.Sym, sdl.Keymod(e.Keysym.Mod)); ok && a.handleInputModeBinding(token) {
			return
		}
	}
	if a.mode == modeNormal {
		if token, ok := keyToken(e.Keysym.Sym, sdl.Keymod(e.Keysym.Mod)); ok {
			prevMode := a.mode
			if a.handleCountToken(token) {
				return
			}
			a.actionKey = token
			a.pushToken(token)
			a.actionKey = ""
			if prevMode == modeNormal && a.mode != modeNormal && utf8.RuneCountInString(token) == 1 {
				a.ignoreText = token
			}
		}
		return
	}
	switch e.Keysym.Sym {
	case sdl.K_LEFT:
		a.moveInputCursor(-1)
	case sdl.K_RIGHT:
		a.moveInputCursor(1)
	case sdl.K_BACKSPACE:
		a.backspaceInput()
	}
}

func (a *App) handleInputModeBinding(token string) bool {
	if !strings.HasPrefix(token, "<") {
		return false
	}
	action, ok := a.sequenceLookup[normalizeBinding(token)]
	if !ok {
		return false
	}
	a.runAction(action)
	return true
}

func (a *App) handleSDLKeyUp(e *sdl.KeyboardEvent) {
	if !a.panning || a.panKey == "" {
		return
	}
	if token, ok := keyToken(e.Keysym.Sym, sdl.Keymod(e.Keysym.Mod)); ok && token == a.panKey {
		a.stopPan()
	}
}

func (a *App) handleSDLTextInput(e *sdl.TextInputEvent) {
	text := e.GetText()
	if text == "" {
		return
	}
	if a.mode == modeNormal {
		return
	}
	if a.ignoreText != "" {
		if strings.HasPrefix(text, a.ignoreText) {
			text = strings.TrimPrefix(text, a.ignoreText)
		}
		a.ignoreText = ""
		if text == "" {
			return
		}
	}
	for _, r := range text {
		if r >= 0x20 && r != 0x7f {
			a.insertInputRune(r)
		}
	}
}

func (a *App) handleSDLMouseWheel(e *sdl.MouseWheelEvent) {
	if a.outlineMenu.visible {
		_, rows := a.outlineMenuGeometry()
		if rows > 0 {
			if e.Y < 0 {
				a.scrollOutlineMenu(1)
			} else if e.Y > 0 {
				a.scrollOutlineMenu(-1)
			}
		}
		return
	}
	wx, wy := e.PreciseX, e.PreciseY
	if wx == 0 {
		wx = float32(e.X)
	}
	if wy == 0 {
		wy = float32(e.Y)
	}
	if e.Direction == sdl.MOUSEWHEEL_FLIPPED {
		wx = -wx
		wy = -wy
	}
	ctrl := sdl.GetModState()&sdl.KMOD_CTRL != 0
	if ctrl {
		if wy > 0 {
			a.runMouseBinding("<c-wheel_up>")
		}
		if wy < 0 {
			a.runMouseBinding("<c-wheel_down>")
		}
		return
	}
	if a.handleSmoothWheel(wx, wy) {
		return
	}
	if a.config.NaturalScroll {
		wy = -wy
	}
	if wy > 0 {
		a.runMouseBinding("wheel_up")
	}
	if wy < 0 {
		a.runMouseBinding("wheel_down")
	}
	if wx > 0 {
		a.runMouseBinding("wheel_right")
	}
	if wx < 0 {
		a.runMouseBinding("wheel_left")
	}
}

func (a *App) handleSmoothWheel(wx, wy float32) bool {
	if wx == 0 && wy == 0 {
		return true
	}
	if wy > 0 && a.mouseBindings["wheel_up"] != "scroll_up" {
		return false
	}
	if wy < 0 && a.mouseBindings["wheel_down"] != "scroll_down" {
		return false
	}
	if wx > 0 && a.mouseBindings["wheel_right"] != "scroll_right" {
		return false
	}
	if wx < 0 && a.mouseBindings["wheel_left"] != "scroll_left" {
		return false
	}
	dy := -float64(wy) * a.pageStep
	if a.config.NaturalScroll {
		dy = -dy
	}
	a.scrollBy(float64(wx)*a.pageStep, dy)
	return true
}

func (a *App) handleSDLMouseButton(e *sdl.MouseButtonEvent) {
	if a.outlineMenu.visible {
		if e.Type == sdl.MOUSEBUTTONDOWN && e.Button == sdl.BUTTON_LEFT {
			a.clickOutlineMenu(int(e.X), int(e.Y))
		}
		return
	}
	if e.Type == sdl.MOUSEBUTTONUP && a.panning && e.Button == a.panButton {
		a.stopPan()
		return
	}
	if event, ok := mouseButtonEvent(e.Button, e.Type); ok {
		a.mouseButton = e.Button
		handled := a.runMouseBinding(event)
		a.mouseButton = 0
		if handled {
			return
		}
	}
	if a.panning {
		return
	}
	if e.Button != sdl.BUTTON_LEFT || !a.config.MouseTextSelect {
		if e.Button == sdl.BUTTON_LEFT && e.Type == sdl.MOUSEBUTTONDOWN {
			a.tryActivateLinkAt(float64(e.X), float64(e.Y))
		}
		return
	}
	if e.Type == sdl.MOUSEBUTTONDOWN {
		if a.tryActivateLinkAt(float64(e.X), float64(e.Y)) {
			a.selection.active = false
			a.selection.quads = nil
			return
		}
		page, point, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y))
		if ok {
			a.selection = textSelection{active: true, page: page, anchor: point, focus: point}
		}
		return
	}
	if e.Type == sdl.MOUSEBUTTONUP && a.selection.active {
		a.copySelectionToClipboard()
		a.selection.active = false
		a.selection.quads = nil
	}
}

func (a *App) handleSDLMouseMotion(e *sdl.MouseMotionEvent) {
	if a.panning && (a.panButton == 0 || e.State&buttonMask(a.panButton) != 0) {
		a.scrollBy(-float64(e.XRel), -float64(e.YRel))
		return
	}
	a.stopPan()

	if a.isLinkAt(float64(e.X), float64(e.Y)) {
		if a.cursorHand != nil {
			sdl.SetCursor(a.cursorHand)
		}
	} else {
		if a.cursorArrow != nil {
			sdl.SetCursor(a.cursorArrow)
		}
	}

	if !a.selection.active || e.State&sdl.ButtonLMask() == 0 {
		return
	}
	page, point, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y))
	if ok && page == a.selection.page {
		a.selection.focus = point
		a.refreshSelection()
	}
}

func (a *App) stopPan() {
	a.panning = false
	a.panButton = 0
	a.panKey = ""
}

func (a *App) handleCountToken(token string) bool {
	if len(token) == 1 && token[0] >= '1' && token[0] <= '9' {
		a.pendingCount += token
		a.message = a.pendingCount
		return true
	}
	if token == "0" && a.pendingCount != "" {
		a.pendingCount += token
		a.message = a.pendingCount
		return true
	}
	if token == "g" && a.pendingCount != "" {
		a.gotoPageInput(a.pendingCount)
		a.pendingCount = ""
		return true
	}
	if a.pendingCount != "" {
		if a.runCountAction(token) {
			a.pendingCount = ""
			return true
		}
		a.pendingCount = ""
	}
	return false
}

func (a *App) runCountAction(token string) bool {
	count, err := strconv.Atoi(a.pendingCount)
	if err != nil || count <= 0 {
		return false
	}
	action, ok := a.sequenceLookup[normalizeBinding(token)]
	if !ok || !isCountableAction(action) {
		return false
	}
	for i := 0; i < count; i++ {
		a.runAction(action)
	}
	return true
}

func (a *App) commitInputMode() {
	input := strings.TrimSpace(a.input)
	currentMode := a.mode
	a.mode = modeNormal
	a.input = ""
	a.inputCursor = 0
	if input == "" {
		return
	}
	switch currentMode {
	case modeCommand:
		a.runCommand(input)
	case modeGotoPage:
		a.gotoPageInput(input)
	case modeSearch:
		a.startSearch(input, a.searchInput)
	}
}

func (a *App) insertInputRune(r rune) {
	left, right := splitAtRune(a.input, a.inputCursor)
	a.input = left + string(r) + right
	a.inputCursor++
}

func (a *App) backspaceInput() {
	if a.inputCursor <= 0 || a.input == "" {
		return
	}
	left, right := splitAtRune(a.input, a.inputCursor)
	_, size := lastRune(left)
	a.input = left[:len(left)-size] + right
	a.inputCursor--
}

func (a *App) moveInputCursor(delta int) {
	a.inputCursor = clampInt(a.inputCursor+delta, 0, utf8.RuneCountInString(a.input))
}

func (a *App) drawPages(renderer *sdl.Renderer) {
	if a.renderMode == "single" {
		a.drawSinglePage(renderer)
		return
	}
	a.drawContinuousPages(renderer)
}

func (a *App) drawContinuousPages(renderer *sdl.Renderer) {
	viewportW, viewportH := a.viewportSize()
	margin := a.renderMargin()
	minY := a.scrollY - margin
	maxY := a.scrollY + float64(viewportH) + margin
	offsetX, offsetY := a.contentViewportOffset()
	for _, row := range a.rows {
		if row.y+row.height < minY || row.y > maxY {
			continue
		}
		for i, page := range row.pages {
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
				continue
			}
			a.drawPageTexture(renderer, x, y, row.pageW[i], row.pageH[i], rp, drawScale)
			a.drawSearchHighlightsForPage(renderer, page, x, y, rp)
		}
	}
	a.drawSelection(renderer)
}

func (a *App) drawSinglePage(renderer *sdl.Renderer) {
	if len(a.rows) == 0 || a.page < 0 || a.page >= len(a.pageToRow) {
		return
	}
	viewportW, viewportH := a.viewportSize()
	row := a.rows[a.pageToRow[a.page]]
	baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
	baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
	for i, page := range row.pages {
		rp, ok := a.cachedRenderPage(page, a.scale)
		if !ok {
			continue
		}
		drawScale := a.renderDrawScale(rp, a.scale)
		x := baseX + (row.pageX[i] - row.x) - a.scrollX
		y := baseY + (row.pageY[i] - row.y) - a.scrollY
		if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
			continue
		}
		a.drawPageTexture(renderer, x, y, row.pageW[i], row.pageH[i], rp, drawScale)
		a.drawSearchHighlightsForPage(renderer, page, x, y, rp)
	}
	a.drawSelection(renderer)
}

func (a *App) drawPageTexture(renderer *sdl.Renderer, x, y, width, height float64, rp *renderedPage, drawScale float64) {
	drawW := rp.width * drawScale
	drawH := rp.height * drawScale
	centerX := x + width/2
	centerY := y + height/2
	if rp.scale > 0 {
		originX, originY := rotatedBoundsOrigin(a.pageMetrics[rp.page].bounds, a.scale, a.rotation)
		pageX := (rp.pixX + rp.width/2) / rp.scale
		pageY := (rp.pixY + rp.height/2) / rp.scale
		tx, ty := transformPoint(pageX, pageY, a.scale, a.rotation)
		centerX = x + tx - originX
		centerY = y + ty - originY
	}
	dst := sdl.FRect{
		X: float32(centerX - drawW/2),
		Y: float32(centerY - drawH/2),
		W: float32(drawW),
		H: float32(drawH),
	}
	_ = a.drawPageBackground(renderer, x, y, rp.page)
	if normalizeRotation(a.rotation) == 0 {
		renderer.CopyF(rp.texture, nil, &dst)
		return
	}
	renderer.CopyExF(rp.texture, nil, &dst, a.rotation, nil, sdl.FLIP_NONE)
}

func (a *App) drawPageBackground(renderer *sdl.Renderer, x, y float64, page int) error {
	clr := a.pageBackgroundColor()
	if normalizeRotation(a.rotation) == 0 {
		m := a.pageMetrics[page]
		return fillRect(renderer, sdl.FRect{X: float32(x), Y: float32(y), W: float32(m.width * a.scale), H: float32(m.height * a.scale)}, clr)
	}
	return renderer.RenderGeometry(nil, pageBackgroundVertices(x, y, a.pageMetrics[page].bounds, a.scale, a.rotation, clr), []int32{0, 1, 2, 1, 3, 2})
}

func pageBackgroundVertices(x, y float64, bounds mupdf.Rect, scale, rotation float64, clr color.RGBA) []sdl.Vertex {
	originX, originY := rotatedBoundsOrigin(bounds, scale, rotation)
	points := []mupdf.Point{
		{X: float64(bounds.X0), Y: float64(bounds.Y0)},
		{X: float64(bounds.X1), Y: float64(bounds.Y0)},
		{X: float64(bounds.X0), Y: float64(bounds.Y1)},
		{X: float64(bounds.X1), Y: float64(bounds.Y1)},
	}
	vertices := make([]sdl.Vertex, len(points))
	color := sdl.Color{R: clr.R, G: clr.G, B: clr.B, A: clr.A}
	for i, point := range points {
		tx, ty := transformPoint(point.X, point.Y, scale, rotation)
		vertices[i] = sdl.Vertex{
			Position: sdl.FPoint{X: float32(x + tx - originX), Y: float32(y + ty - originY)},
			Color:    color,
		}
	}
	return vertices
}

func (a *App) cachedRenderPage(page int, scale float64) (*renderedPage, bool) {
	renderScale := a.renderScaleFor(scale)
	key := renderCacheKey(page, renderScale, a.altColors)
	if rp, ok := a.renderCache[key]; ok {
		return rp, true
	}
	var bestHigher *renderedPage
	var bestLower *renderedPage
	for _, rp := range a.renderCache {
		if rp.page != page {
			continue
		}
		if math.Abs(rp.scale-renderScale) < 0.0001 {
			return rp, true
		}
		if rp.scale >= renderScale {
			if bestHigher == nil || rp.scale < bestHigher.scale {
				bestHigher = rp
			}
			continue
		}
		if bestLower == nil || rp.scale > bestLower.scale {
			bestLower = rp
		}
	}
	if bestHigher != nil {
		return bestHigher, true
	}
	if bestLower != nil {
		return bestLower, true
	}
	return nil, false
}

func (a *App) prefetchVisiblePages() {
	if len(a.rows) == 0 {
		return
	}
	if a.renderMode == "single" {
		if a.page < 0 || a.page >= len(a.pageToRow) {
			return
		}
		row := a.rows[a.pageToRow[a.page]]
		for _, page := range row.pages {
			a.requestRender(page, a.scale)
		}
		return
	}
	_, viewportH := a.viewportSize()
	margin := math.Max(a.renderMargin()*2, float64(viewportH))
	minY := a.scrollY - margin
	maxY := a.scrollY + float64(viewportH) + margin
	for _, row := range a.rows {
		if row.y+row.height < minY || row.y > maxY {
			continue
		}
		for _, page := range row.pages {
			a.requestRender(page, a.scale)
		}
	}
}

func (a *App) currentScale(viewportW, viewportH int) float64 {
	if a.fitMode == "manual" {
		return a.zoom
	}
	baseRows := a.baseRows()
	if a.renderMode == "single" && len(baseRows) > 0 && a.page >= 0 {
		rowIndex := clampInt(a.baseRowIndexForPage(a.page, baseRows), 0, len(baseRows)-1)
		row := baseRows[rowIndex]
		marginW := float64(a.horizontalGap() * 2)
		marginH := float64(a.verticalGap() * 2)
		widthScale := math.Max(0.05, (float64(viewportW)-marginW)/math.Max(1, row.width))
		if a.fitMode == "width" {
			return widthScale
		}
		heightScale := math.Max(0.05, (float64(viewportH)-marginH)/math.Max(1, row.height))
		return math.Max(0.05, math.Min(widthScale, heightScale))
	}
	maxRowWidth := 1.0
	maxRowHeight := 1.0
	for _, row := range baseRows {
		if row.width > maxRowWidth {
			maxRowWidth = row.width
		}
		if row.height > maxRowHeight {
			maxRowHeight = row.height
		}
	}
	marginW := float64(a.horizontalGap() * 2)
	marginH := float64(a.verticalGap() * 2)
	widthScale := math.Max(0.05, (float64(viewportW)-marginW)/maxRowWidth)
	if a.fitMode == "width" {
		return widthScale
	}
	heightScale := math.Max(0.05, (float64(viewportH)-marginH)/maxRowHeight)
	return math.Max(0.05, math.Min(widthScale, heightScale))
}

func (a *App) nextPage() {
	if a.dualPage {
		a.nextSpread()
		return
	}
	if a.page < a.pageCount-1 {
		a.page++
		a.alignPageTop(a.page)
	}
}

func (a *App) prevPage() {
	if a.dualPage {
		a.prevSpread()
		return
	}
	if a.page > 0 {
		a.page--
		a.alignPageTop(a.page)
	}
}

func (a *App) nextSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.currentRowIndex()
	if row < len(a.rows)-1 {
		a.alignPageTop(a.rows[row+1].pages[0])
	}
}

func (a *App) nextSpreadFrom(page int) {
	if a.pageCount == 0 {
		return
	}
	page = a.anchorPage(page)
	row := a.pageToRow[page]
	if row < len(a.rows)-1 {
		next := a.rows[row+1].pages[0]
		a.page = next
		a.alignPageTop(next)
	}
}

func (a *App) prevSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.currentRowIndex()
	if row > 0 {
		a.alignPageTop(a.rows[row-1].pages[0])
	}
}

func (a *App) prevSpreadFrom(page int) {
	if a.pageCount == 0 {
		return
	}
	page = a.anchorPage(page)
	row := a.pageToRow[page]
	if row > 0 {
		prev := a.rows[row-1].pages[0]
		a.page = prev
		a.alignPageTop(prev)
	}
}

func (a *App) scrollBy(dx, dy float64) {
	a.scrollX += dx
	a.scrollY += dy
	a.clampScroll()
	if a.renderMode == "single" {
		return
	}
	a.updateCurrentPageFromScroll()
}

func (a *App) alignPageTop(page int) {
	if page < 0 || page >= len(a.pageToRow) {
		return
	}
	page = a.anchorPage(page)
	if a.positionMatchesPageTop(page) {
		return
	}
	a.recordJump()
	if a.renderMode == "single" {
		a.page = page
		a.scrollX = 0
		a.scrollY = 0
		a.recomputeLayout(a.viewportSize())
		a.clampScroll()
		return
	}
	row := a.rows[a.pageToRow[page]]
	a.scrollY = row.y - float64(a.verticalGap())/2
	a.page = page
	a.clampScroll()
}

func (a *App) recordJump() {
	pos := a.currentJumpPosition()
	if len(a.jumpBack) == 0 || a.jumpBack[len(a.jumpBack)-1] != pos {
		a.jumpBack = append(a.jumpBack, pos)
	}
	a.jumpAhead = nil
}

func (a *App) jumpForward() {
	if len(a.jumpAhead) == 0 {
		return
	}
	current := a.currentJumpPosition()
	jump := a.jumpAhead[len(a.jumpAhead)-1]
	a.jumpAhead = a.jumpAhead[:len(a.jumpAhead)-1]
	if len(a.jumpBack) == 0 || a.jumpBack[len(a.jumpBack)-1] != current {
		a.jumpBack = append(a.jumpBack, current)
	}
	a.restoreJump(jump)
}

func (a *App) jumpBackward() {
	if len(a.jumpBack) == 0 {
		return
	}
	current := a.currentJumpPosition()
	jump := a.jumpBack[len(a.jumpBack)-1]
	a.jumpBack = a.jumpBack[:len(a.jumpBack)-1]
	if len(a.jumpAhead) == 0 || a.jumpAhead[len(a.jumpAhead)-1] != current {
		a.jumpAhead = append(a.jumpAhead, current)
	}
	a.restoreJump(jump)
}

func (a *App) currentJumpPosition() jumpPosition {
	return jumpPosition{
		page:    a.page,
		scrollX: a.scrollX,
		scrollY: a.scrollY,
	}
}

func (a *App) restoreJump(jump jumpPosition) {
	a.page = jump.page
	a.scrollX = jump.scrollX
	a.scrollY = jump.scrollY
	a.recomputeLayout(a.viewportSize())
	a.clampScroll()
}

func (a *App) positionMatchesPageTop(page int) bool {
	if a.renderMode == "single" {
		return a.page == page && a.scrollX == 0 && a.scrollY == 0
	}
	row := a.rows[a.pageToRow[page]]
	expectedY := row.y - float64(a.verticalGap())/2
	return a.page == page && a.scrollY == expectedY
}

func (a *App) anchorPage(page int) int {
	if page < 0 || page >= len(a.pageToRow) {
		return page
	}
	if !a.dualPage || len(a.rows) == 0 {
		return page
	}
	row := a.rows[a.pageToRow[page]]
	if len(row.pages) == 0 {
		return page
	}
	return row.pages[0]
}

func (a *App) setManualZoom(delta float64) {
	anchor := a.captureZoomAnchor()
	baseZoom := a.zoom
	if a.fitMode != "manual" {
		baseZoom = a.scale
	}
	a.fitMode = "manual"
	a.zoom = math.Max(0.75, math.Min(4.0, baseZoom*delta))
	a.maybeUpgradeRenderScale(a.zoom)
	a.recomputeLayout(a.viewportSize())
	a.restoreZoomAnchor(anchor)
}

func (a *App) setFitMode(mode string) {
	anchor := a.captureZoomAnchor()
	a.fitMode = mode
	a.maybeUpgradeRenderScale(a.zoom)
	a.recomputeLayout(a.viewportSize())
	a.restoreZoomAnchor(anchor)
}

func (a *App) clearCache() {
	for _, rp := range a.renderCache {
		rp.texture.Destroy()
	}
	a.renderCache = map[string]*renderedPage{}
	a.renderOrder = nil
	a.invalidateRenderRequests()
}

func (a *App) pushToken(token string) {
	a.sequence = append(a.sequence, token)
	a.sequenceAt = time.Now()
	for len(a.sequence) > 0 {
		joined := strings.Join(a.sequence, " ")
		cmd, exact := a.sequenceLookup[joined]
		prefix := a.hasPrefix(joined)
		if exact && !prefix {
			a.sequence = nil
			a.runAction(cmd)
			return
		}
		if exact && prefix {
			return
		}
		if prefix {
			return
		}
		if len(a.sequence) == 1 {
			a.sequence = nil
			return
		}
		a.sequence = a.sequence[1:]
	}
}

func (a *App) expireSequence() {
	if len(a.sequence) == 0 {
		return
	}
	if time.Since(a.sequenceAt) < time.Duration(a.config.SequenceTimeoutMS)*time.Millisecond {
		return
	}
	joined := strings.Join(a.sequence, " ")
	if cmd, ok := a.sequenceLookup[joined]; ok {
		a.sequence = nil
		a.runAction(cmd)
		return
	}
	a.sequence = nil
}

func (a *App) hasPrefix(joined string) bool {
	for key := range a.sequenceLookup {
		if key != joined && strings.HasPrefix(key, joined+" ") {
			return true
		}
	}
	return false
}

func (a *App) runAction(action string) {
	if handled, dirty, err := a.runtime.RunAction(action); handled {
		if err != nil {
			a.message = err.Error()
			return
		}
		if dirty {
			a.applyConfig(a.runtime.Config())
		}
		return
	}
	if err := a.runBuiltinAction(action); err != nil {
		a.message = err.Error()
	}
}

func (a *App) runBuiltinAction(action string) error {
	switch action {
	case "next_page":
		a.nextPage()
	case "prev_page":
		a.prevPage()
	case "scroll_down":
		a.scrollBy(0, a.pageStep)
	case "scroll_up":
		a.scrollBy(0, -a.pageStep)
	case "scroll_left":
		a.scrollBy(-a.pageStep, 0)
	case "scroll_right":
		a.scrollBy(a.pageStep, 0)
	case "pan":
		if a.actionKey != "" {
			a.panning = true
			a.panKey = a.actionKey
			a.panButton = 0
			return nil
		}
		if a.mouseButton != 0 {
			a.panning = true
			a.panButton = a.mouseButton
			a.panKey = ""
		}
	case "next_spread":
		a.nextSpread()
	case "prev_spread":
		a.prevSpread()
	case "first_page":
		a.alignPageTop(0)
	case "last_page":
		a.alignPageTop(a.pageCount - 1)
	case "command_mode":
		a.mode = modeCommand
		a.input = ""
		a.inputCursor = 0
	case "search_prompt":
		a.mode = modeSearch
		a.searchInput = searchModeForward
		a.input = ""
		a.inputCursor = 0
	case "search_prompt_backward":
		a.mode = modeSearch
		a.searchInput = searchModeBackward
		a.input = ""
		a.inputCursor = 0
	case "goto_page_prompt":
		a.mode = modeGotoPage
		a.input = ""
		a.inputCursor = 0
	case "search_next":
		a.repeatSearch(true)
	case "search_prev":
		a.repeatSearch(false)
	case "toggle_dual_page":
		a.dualPage = !a.dualPage
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
		a.message = boolWord(a.dualPage, "dual-page on", "dual-page off")
	case "toggle_render_mode":
		page := a.page
		if a.renderMode == "single" {
			a.renderMode = "continuous"
		} else {
			a.renderMode = "single"
		}
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(page)
		a.message = "render mode " + a.renderMode
	case "toggle_alt_colors":
		a.altColors = !a.altColors
		a.clearCache()
		a.message = boolWord(a.altColors, "alt colors on", "alt colors off")
	case "toggle_first_page_offset":
		a.firstPageOffset = !a.firstPageOffset
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
		a.message = boolWord(a.firstPageOffset, "first-page offset on", "first-page offset off")
	case "toggle_status_bar":
		a.statusBarShown = !a.statusBarShown
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
	case "toggle_fullscreen":
		a.fullscreen = !a.fullscreen
		a.SetFullscreen(a.fullscreen)
	case "outline":
		a.toggleOutlineMenu()
	case "confirm":
		if a.outlineMenu.visible {
			a.activateSelectedOutline()
		} else if a.mode != modeNormal {
			a.commitInputMode()
		}
	case "zoom_in":
		a.setManualZoom(1.15)
	case "zoom_out":
		a.setManualZoom(1 / 1.15)
	case "reset_zoom":
		a.zoom = 1
		a.setFitMode("manual")
	case "fit_width":
		a.setFitMode("width")
	case "fit_page":
		a.setFitMode("page")
	case "reload_config":
		a.reloadConfig()
	case "rotate_cw":
		page := a.page
		a.rotation = normalizeRotation(a.rotation + 90)
		a.updatePageMetricSizes()
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(page)
	case "rotate_ccw":
		page := a.page
		a.rotation = normalizeRotation(a.rotation + 270)
		a.updatePageMetricSizes()
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(page)
	case "quit":
		a.quit = true
	case "close":
		a.closeActiveUI()
	case "jump_forward":
		a.jumpForward()
	case "jump_backward":
		a.jumpBackward()
	case "clear_search":
		a.clearSearch()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

func (a *App) closeActiveUI() {
	if a.outlineMenu.visible {
		a.outlineMenu.visible = false
		return
	}
	if a.mode != modeNormal {
		a.mode = modeNormal
		a.input = ""
		a.inputCursor = 0
		a.ignoreText = ""
		return
	}
	if a.search.query != "" || len(a.search.order) > 0 || a.search.running {
		a.clearSearch()
		return
	}
	a.sequence = nil
	a.pendingCount = ""
}

func (a *App) ExecuteAction(action string) error {
	return a.runBuiltinAction(action)
}

func (a *App) Page() int {
	return a.page + 1
}

func (a *App) PageCount() int {
	return a.pageCount
}

func (a *App) GotoPage(page int) error {
	if a.pageCount == 0 {
		return nil
	}
	a.alignPageTop(clampInt(page-1, 0, a.pageCount-1))
	return nil
}

func (a *App) Message() string {
	return a.message
}

func (a *App) SetMessage(message string) {
	a.message = message
}

func (a *App) RunCommand(command string) error {
	a.runCommand(command)
	return nil
}

func (a *App) Open(path string) error {
	if path == "" {
		return fmt.Errorf("open: empty path")
	}
	if !filepath.IsAbs(path) {
		if a.docPath != "" {
			dir := filepath.Dir(a.docPath)
			path = filepath.Join(dir, path)
		}
	}
	a.message = "opening " + path
	a.quit = true
	a.pendingOpen = path
	return nil
}

func (a *App) Mode() string {
	switch a.mode {
	case modeCommand:
		return "command"
	case modeGotoPage:
		return "goto"
	case modeSearch:
		return "search"
	default:
		return "normal"
	}
}

func (a *App) Search(query string, backward bool) error {
	mode := searchModeForward
	if backward {
		mode = searchModeBackward
	}
	a.startSearch(query, mode)
	return nil
}

func (a *App) SearchQuery() string {
	return a.search.query
}

func (a *App) SearchMatchCount() int {
	return len(a.search.order)
}

func (a *App) SearchMatchIndex() int {
	if a.search.current < 0 || a.search.current >= len(a.search.order) {
		return 0
	}
	return a.search.current + 1
}

func (a *App) CurrentCount() string {
	return a.pendingCount
}

func (a *App) PendingKeys() []string {
	return append([]string(nil), a.sequence...)
}

func (a *App) ClearPendingKeys() {
	a.sequence = nil
	a.pendingCount = ""
	if a.mode == modeNormal {
		a.message = ""
	}
}

func (a *App) FitMode() string {
	return a.fitMode
}

func (a *App) SetFitMode(mode string) error {
	a.setFitMode(sanitizeFitMode(mode))
	return nil
}

func (a *App) RenderMode() string {
	return a.renderMode
}

func (a *App) SetRenderMode(mode string) error {
	mode = sanitizeRenderMode(mode)
	if a.renderMode == mode {
		return nil
	}
	page := a.page
	a.renderMode = mode
	a.recomputeLayout(a.viewportSize())
	a.alignPageTop(page)
	return nil
}

func (a *App) Zoom() float64 {
	return a.scale
}

func (a *App) SetZoom(zoom float64) error {
	if zoom <= 0 {
		return fmt.Errorf("zoom must be positive")
	}
	anchor := a.captureZoomAnchor()
	a.fitMode = "manual"
	a.zoom = clampFloat(zoom, 0.05, 8.0)
	a.maybeUpgradeRenderScale(a.zoom)
	a.recomputeLayout(a.viewportSize())
	a.restoreZoomAnchor(anchor)
	return nil
}

func (a *App) Rotation() float64 {
	return normalizeRotation(a.rotation)
}

func (a *App) SetRotation(rotation float64) error {
	page := a.page
	a.rotation = normalizeRotation(rotation)
	a.updatePageMetricSizes()
	a.recomputeLayout(a.viewportSize())
	a.alignPageTop(page)
	return nil
}

func (a *App) Fullscreen() bool {
	return a.fullscreen
}

func (a *App) SetFullscreen(fullscreen bool) error {
	a.fullscreen = fullscreen
	if a.window == nil {
		return nil
	}
	if fullscreen {
		return a.window.SetFullscreen(sdl.WINDOW_FULLSCREEN_DESKTOP)
	}
	return a.window.SetFullscreen(0)
}

func (a *App) StatusBarVisible() bool {
	return a.statusBarShown
}

func (a *App) SetStatusBarVisible(visible bool) error {
	if a.statusBarShown == visible {
		return nil
	}
	a.statusBarShown = visible
	a.recomputeLayout(a.viewportSize())
	a.alignPageTop(a.page)
	return nil
}

func (a *App) CacheEntries() int {
	return len(a.renderCache)
}

func (a *App) CachePending() int {
	return len(a.renderPending)
}

func (a *App) CacheLimit() int {
	return a.cacheLimit
}

func (a *App) SetCacheLimit(limit int) error {
	if limit < 1 {
		return fmt.Errorf("cache limit must be at least 1")
	}
	a.cacheLimit = limit
	for len(a.renderOrder) > a.cacheLimit {
		a.evictRenderCacheEntry(a.renderOrder[0])
	}
	return nil
}

func (a *App) ClearCache() {
	a.clearCache()
}

func (a *App) gotoPageInput(input string) {
	n, err := strconv.Atoi(input)
	if err != nil {
		a.message = fmt.Sprintf("invalid page: %s", input)
		return
	}
	a.alignPageTop(clampInt(n-1, 0, a.pageCount-1))
}

func (a *App) runCommand(input string) {
	if _, err := strconv.Atoi(input); err == nil {
		a.gotoPageInput(input)
		return
	}
	fields := strings.Fields(strings.TrimPrefix(input, ":"))
	if len(fields) == 0 {
		return
	}
	switch fields[0] {
	case "q", "quit":
		a.quit = true
	case "page", "p":
		if len(fields) < 2 {
			a.message = "usage: :page <n>"
			return
		}
		a.gotoPageInput(fields[1])
	case "set":
		if len(fields) < 2 {
			return
		}
		a.runSet(fields[1])
	case "mode":
		if len(fields) < 2 {
			a.message = "usage: :mode continuous|single"
			return
		}
		a.renderMode = sanitizeRenderMode(fields[1])
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
	case "colors":
		if len(fields) < 2 {
			a.message = "usage: :colors normal|alt"
			return
		}
		a.altColors = strings.EqualFold(fields[1], "alt")
	case "fit":
		if len(fields) < 2 {
			return
		}
		a.setFitMode(sanitizeFitMode(fields[1]))
	case "reload-config":
		a.reloadConfig()
	case "search":
		query := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(input, ":"), "search"))
		a.startSearch(query, searchModeForward)
	case "open":
		if len(fields) < 2 {
			a.message = "usage: :open <filename>"
			return
		}
		newPath := fields[1]
		if !filepath.IsAbs(newPath) {
			if a.docPath != "" {
				dir := filepath.Dir(a.docPath)
				newPath = filepath.Join(dir, newPath)
			}
		}
		a.message = "opening " + newPath
		a.quit = true
		a.pendingOpen = newPath
	case "help":
		a.message = commandHelpMessage()
	default:
		a.message = "unknown command: " + fields[0]
	}
}

func (a *App) reloadConfig() {
	if err := a.runtime.Reload(); err != nil {
		a.message = err.Error()
		return
	}
	cfg := a.runtime.Config()
	a.applyConfig(cfg)
	a.message = boolWord(cfg.ConfigPath != "", "config reloaded", "defaults reloaded")
}

func commandHelpMessage() string {
	return ":page N | :search text | :mode continuous|single | :colors normal|alt | :set render_mode!|alt_colors!|dual_page!|first_page_offset!|status_bar! | :fit width|page | :reload-config | :quit"
}

func (a *App) runSet(setting string) {
	switch setting {
	case "dual_page!":
		a.runAction("toggle_dual_page")
	case "alt_colors!":
		a.runAction("toggle_alt_colors")
	case "render_mode!":
		a.runAction("toggle_render_mode")
	case "first_page_offset!":
		a.runAction("toggle_first_page_offset")
	case "status_bar!":
		a.runAction("toggle_status_bar")
	default:
		a.message = "unknown setting: " + setting
	}
}

func (a *App) drawStatusBar(renderer *sdl.Renderer) error {
	h := a.config.StatusBarHeight
	y := a.winH - h
	if err := fillRect(renderer, sdl.FRect{X: 0, Y: float32(y), W: float32(a.winW), H: float32(h)}, a.statusBarColor()); err != nil {
		return err
	}
	left := a.formatStatusBar(a.config.StatusBarLeft)
	right := a.formatStatusBar(a.config.StatusBarRight)
	vertOffset := (h + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
	if err := drawText(renderer, a.fontFace, left, 8, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	if err := a.drawInputCursor(renderer, y); err != nil {
		return err
	}
	rw := measureText(a.fontFace, right)
	if err := drawText(renderer, a.fontFace, right, a.winW-rw-8, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	return nil
}

func (a *App) formatStatusBar(template string) string {
	result := template

	replacements := map[string]string{
		"{message}":       a.message,
		"{page}":          fmt.Sprintf("%d", a.page+1),
		"{total}":         fmt.Sprintf("%d", a.pageCount),
		"{mode}":          a.renderMode,
		"{fit}":           a.fitMode,
		"{rot}":           fmt.Sprintf("%.0f", a.rotation),
		"{zoom}":          fmt.Sprintf("%.0f%%", a.zoom*100),
		"{dual}":          boolWord(a.dualPage, "dual", "single"),
		"{cover}":         boolWord(a.firstPageOffset, "cover", "flat"),
		"{search}":        a.searchStatusCounter(),
		"{document}":      a.docName,
		"$$":              "\x00",
	}

	if a.mode == modeCommand {
		replacements["{message}"] = " COMMAND :" + a.input
		replacements["{input}"] = a.input
	} else if a.mode == modeGotoPage {
		replacements["{message}"] = " GOTO " + a.input
		replacements["{input}"] = a.input
	} else if a.mode == modeSearch {
		replacements["{message}"] = " SEARCH " + a.searchPromptToken() + a.input
		replacements["{input}"] = a.input
		replacements["{prompt}"] = a.searchPromptToken()
	} else {
		replacements["{input}"] = ""
		replacements["{prompt}"] = ""
	}

	if a.dualPage && len(a.rows) > 0 && a.page >= 0 && a.page < len(a.pageToRow) {
		row := a.rows[a.pageToRow[a.page]]
		if len(row.pages) >= 2 {
			replacements["{page}"] = fmt.Sprintf("%d-%d", row.pages[0]+1, row.pages[len(row.pages)-1]+1)
		}
	}

	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}

	result = strings.ReplaceAll(result, "\x00", "$")

	return result
}

func (a *App) statusLeft() string {
	content := a.message
	switch a.mode {
	case modeCommand:
		return " COMMAND :" + a.input
	case modeGotoPage:
		return " GOTO " + a.input
	case modeSearch:
		return " SEARCH " + a.searchPromptToken() + a.input
	}
	if content == "" {
		return " "
	}
	return " " + content
}

func (a *App) statusRight() string {
	pageDisplay := fmt.Sprintf("%d/%d", a.page+1, a.pageCount)
	if a.dualPage && len(a.rows) > 0 && a.page >= 0 && a.page < len(a.pageToRow) {
		row := a.rows[a.pageToRow[a.page]]
		if len(row.pages) >= 2 {
			pageDisplay = fmt.Sprintf("%d-%d/%d", row.pages[0]+1, row.pages[len(row.pages)-1]+1, a.pageCount)
		}
	}
	parts := []string{
		pageDisplay,
		fmt.Sprintf("mode=%s", a.renderMode),
		fmt.Sprintf("fit=%s", a.fitMode),
		fmt.Sprintf("rot=%.0f", a.rotation),
		boolWord(a.dualPage, "dual", "single"),
		boolWord(a.firstPageOffset, "cover", "flat"),
	}
	if counter := a.searchStatusCounter(); counter != "" {
		parts = append(parts, counter)
	}
	if a.fitMode == "manual" {
		parts = append(parts, fmt.Sprintf("zoom=%.0f%%", a.zoom*100))
	}
	return strings.Join(parts, "  ")
}

func (a *App) drawInputCursor(renderer *sdl.Renderer, barY int) error {
	if a.mode == modeNormal {
		return nil
	}
	prefix := a.inputPrefix()
	left, _ := splitAtRune(a.input, a.inputCursor)
	x := 8 + measureText(a.fontFace, prefix+left)
	fg := a.foregroundColor()
	if err := renderer.SetDrawColor(fg.R, fg.G, fg.B, fg.A); err != nil {
		return err
	}
	return renderer.DrawLine(int32(x), int32(barY+6), int32(x), int32(barY+22))
}

func (a *App) inputPrefix() string {
	switch a.mode {
	case modeCommand:
		return " COMMAND :"
	case modeGotoPage:
		return " GOTO "
	case modeSearch:
		return " SEARCH " + a.searchPromptToken()
	default:
		return ""
	}
}

func (a *App) searchPromptToken() string {
	if a.searchInput == searchModeBackward {
		return "?"
	}
	return "/"
}

func (a *App) runMouseBinding(event string) bool {
	if action, ok := a.mouseBindings[event]; ok {
		a.runAction(action)
		return true
	}
	return false
}

func (a *App) captureZoomAnchor() zoomAnchor {
	viewportW, viewportH := a.viewportSize()
	cx := float64(viewportW) / 2
	cy := float64(viewportH) / 2
	page, point, ok := a.pagePointAtScreen(cx, cy)
	if !ok {
		return zoomAnchor{centerX: cx, centerY: cy}
	}
	return zoomAnchor{page: page, point: point, valid: true, centerX: cx, centerY: cy}
}

func (a *App) restoreZoomAnchor(anchor zoomAnchor) {
	if !anchor.valid {
		a.clampScroll()
		return
	}
	x, y, rp, ok := a.pagePlacement(anchor.page)
	if !ok || rp == nil {
		a.clampScroll()
		return
	}
	tx, ty := transformPoint(anchor.point.X, anchor.point.Y, a.scale, a.rotation)
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[rp.page].bounds, a.scale, a.rotation)
	a.scrollX += (x + tx - originX) - anchor.centerX
	a.scrollY += (y + ty - originY) - anchor.centerY
	a.clampScroll()
	if a.renderMode == "continuous" {
		a.updateCurrentPageFromScroll()
	}
}

func (a *App) viewportSize() (int, int) {
	h := a.winH
	if a.statusVisible() {
		h -= a.config.StatusBarHeight
	}
	if h < 1 {
		h = 1
	}
	w := a.winW
	if w < 1 {
		w = 1
	}
	return w, h
}

func (a *App) contentViewportOffset() (float64, float64) {
	viewportW, viewportH := a.viewportSize()
	offsetX := math.Max(0, (float64(viewportW)-a.contentW)/2)
	offsetY := math.Max(0, (float64(viewportH)-a.contentH)/2)
	if a.renderMode == "continuous" {
		offsetY = 0
	}
	return offsetX, offsetY
}

func (a *App) renderMargin() float64 {
	_, viewportH := a.viewportSize()
	return math.Max(float64(viewportH)/2, a.pageStep*2)
}

func (a *App) pagePointAtScreen(sx, sy float64) (int, mupdf.Point, bool) {
	for _, hit := range a.visiblePageHits() {
		if sx < hit.x || sy < hit.y || sx > hit.x+hit.width || sy > hit.y+hit.height {
			continue
		}
		originX, originY := rotatedBoundsOrigin(a.pageMetrics[hit.page].bounds, a.scale, a.rotation)
		transformedX := sx - hit.x + originX
		transformedY := sy - hit.y + originY
		pageX, pageY := inverseTransformPoint(transformedX, transformedY, a.scale, a.rotation)
		return hit.page, mupdf.Point{X: pageX, Y: pageY}, true
	}
	return 0, mupdf.Point{}, false
}

type pageHit struct {
	page      int
	x         float64
	y         float64
	width     float64
	height    float64
	drawScale float64
	render    *renderedPage
}

func (a *App) visiblePageHits() []pageHit {
	hits := []pageHit{}
	if a.renderMode == "single" {
		if len(a.rows) == 0 || a.page < 0 || a.page >= len(a.pageToRow) {
			return hits
		}
		viewportW, viewportH := a.viewportSize()
		row := a.rows[a.pageToRow[a.page]]
		baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
		baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
		for i, page := range row.pages {
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				a.requestRender(page, a.scale)
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			x := baseX + (row.pageX[i] - row.x) - a.scrollX
			y := baseY + (row.pageY[i] - row.y) - a.scrollY
			hits = append(hits, pageHit{page: page, x: x, y: y, width: row.pageW[i], height: row.pageH[i], drawScale: drawScale, render: rp})
		}
		return hits
	}
	viewportW, viewportH := a.viewportSize()
	margin := a.renderMargin()
	minY := a.scrollY - margin
	maxY := a.scrollY + float64(viewportH) + margin
	offsetX, offsetY := a.contentViewportOffset()
	for _, row := range a.rows {
		if row.y+row.height < minY || row.y > maxY {
			continue
		}
		for i, page := range row.pages {
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				a.requestRender(page, a.scale)
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			hits = append(hits, pageHit{page: page, x: x, y: y, width: row.pageW[i], height: row.pageH[i], drawScale: drawScale, render: rp})
		}
	}
	if len(hits) == 0 {
		_ = viewportW
	}
	return hits
}

func (a *App) refreshSelection() {
	sel, err := a.doc.ExtractSelection(a.selection.page, a.selection.anchor, a.selection.focus)
	if err != nil {
		a.message = err.Error()
		return
	}
	a.selection.text = sel.Text
	a.selection.quads = sel.Quads
}

func (a *App) copySelectionToClipboard() {
	if strings.TrimSpace(a.selection.text) == "" {
		return
	}
	if err := sdl.SetClipboardText(a.selection.text); err != nil {
		a.message = "clipboard unavailable"
		return
	}
	a.message = fmt.Sprintf("copied %d chars", len(a.selection.text))
	a.selection.text = ""
}

func (a *App) tryActivateLinkAt(sx, sy float64) bool {
	page, point, ok := a.pagePointAtScreen(sx, sy)
	if !ok {
		return false
	}
	links, err := a.linksForPage(page)
	if err != nil {
		a.message = err.Error()
		return false
	}
	for _, link := range links {
		if point.X < float64(link.Bounds.X0) || point.X > float64(link.Bounds.X1) || point.Y < float64(link.Bounds.Y0) || point.Y > float64(link.Bounds.Y1) {
			continue
		}
		a.activateLink(link)
		return true
	}
	return false
}

func (a *App) isLinkAt(sx, sy float64) bool {
	page, point, ok := a.pagePointAtScreen(sx, sy)
	if !ok {
		return false
	}
	links, err := a.linksForPage(page)
	if err != nil {
		return false
	}
	for _, link := range links {
		if point.X < float64(link.Bounds.X0) || point.X > float64(link.Bounds.X1) || point.Y < float64(link.Bounds.Y0) || point.Y > float64(link.Bounds.Y1) {
			continue
		}
		return true
	}
	return false
}

func (a *App) linksForPage(page int) ([]mupdf.Link, error) {
	if links, ok := a.pageLinks[page]; ok {
		return links, nil
	}
	links, err := a.doc.Links(page)
	if err != nil {
		return nil, err
	}
	a.pageLinks[page] = links
	return links, nil
}

func (a *App) activateLink(link mupdf.Link) {
	if link.External {
		if link.URI == "" {
			return
		}
		if err := openExternalURL(link.URI); err != nil {
			a.message = err.Error()
			return
		}
		a.message = link.URI
		return
	}
	if link.Page >= 0 {
		a.alignPageTop(link.Page)
		return
	}
	if link.URI != "" {
		a.message = link.URI
	}
}

func openExternalURL(uri string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", uri)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", uri)
	default:
		cmd = exec.Command("xdg-open", uri)
	}
	return cmd.Start()
}

func (a *App) drawSelection(renderer *sdl.Renderer) {
	if len(a.selection.quads) == 0 {
		return
	}
	x, y, rp, ok := a.pageScreenTransform(a.selection.page)
	if !ok {
		return
	}
	a.drawHighlightQuads(renderer, a.selection.quads, x, y, rp)
}

func (a *App) drawHighlightQuads(renderer *sdl.Renderer, quads []mupdf.Quad, x, y float64, rp *renderedPage) {
	a.drawHighlightQuadsWithStyle(renderer, quads, x, y, rp, false)
}

func (a *App) highlightForegroundColor() color.RGBA {
	return rgb(a.config.HighlightForeground)
}

func (a *App) highlightBackgroundColor() color.RGBA {
	bg := rgb(a.config.HighlightBackground)
	bg.A = 0xaa
	return bg
}

func (a *App) pageScreenTransform(page int) (float64, float64, *renderedPage, bool) {
	return a.pagePlacement(page)
}

func (a *App) pagePlacement(page int) (float64, float64, *renderedPage, bool) {
	if page < 0 || page >= len(a.pageToRow) || len(a.rows) == 0 {
		return 0, 0, nil, false
	}
	row := a.rows[a.pageToRow[page]]
	index := -1
	for i, candidate := range row.pages {
		if candidate == page {
			index = i
			break
		}
	}
	if index < 0 {
		return 0, 0, nil, false
	}
	rp, ok := a.cachedRenderPage(page, a.scale)
	if !ok {
		a.requestRender(page, a.scale)
		return 0, 0, nil, false
	}
	if a.renderMode == "single" {
		viewportW, viewportH := a.viewportSize()
		baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
		baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
		return baseX + (row.pageX[index] - row.x) - a.scrollX, baseY + (row.pageY[index] - row.y) - a.scrollY, rp, true
	}
	offsetX, offsetY := a.contentViewportOffset()
	return row.pageX[index] - a.scrollX + offsetX, row.pageY[index] - a.scrollY + offsetY, rp, true
}

func (a *App) quadScreenBounds(quad mupdf.Quad, x, y float64, rp *renderedPage) (float64, float64, float64, float64) {
	pts := []mupdf.Point{quad.UL, quad.UR, quad.LL, quad.LR}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[rp.page].bounds, a.scale, a.rotation)
	for _, pt := range pts {
		tx, ty := transformPoint(pt.X, pt.Y, a.scale, a.rotation)
		sx := x + tx - originX
		sy := y + ty - originY
		minX = math.Min(minX, sx)
		minY = math.Min(minY, sy)
		maxX = math.Max(maxX, sx)
		maxY = math.Max(maxY, sy)
	}
	return minX, minY, maxX, maxY
}

func (a *App) loadPageMetrics() error {
	a.pageMetrics = make([]pageMetrics, a.pageCount)
	for i := 0; i < a.pageCount; i++ {
		bounds, err := a.doc.Bounds(i)
		if err != nil {
			return err
		}
		w, h := rotatedBoundsSize(bounds, a.rotation)
		a.pageMetrics[i] = pageMetrics{bounds: bounds, width: w, height: h}
	}
	return nil
}

func (a *App) updatePageMetricSizes() {
	for i := range a.pageMetrics {
		a.pageMetrics[i].width, a.pageMetrics[i].height = rotatedBoundsSize(a.pageMetrics[i].bounds, a.rotation)
	}
}

func (a *App) baseRows() []rowLayout {
	rows := make([]rowLayout, 0, a.pageCount)
	appendRow := func(pages ...int) {
		row := rowLayout{pages: append([]int(nil), pages...), pageX: make([]float64, len(pages)), pageY: make([]float64, len(pages)), pageW: make([]float64, len(pages)), pageH: make([]float64, len(pages))}
		for i, page := range pages {
			m := a.pageMetrics[page]
			row.pageW[i] = m.width
			row.pageH[i] = m.height
			row.width += m.width
			if i > 0 {
				row.width += float64(a.horizontalGap())
			}
			if m.height > row.height {
				row.height = m.height
			}
		}
		rows = append(rows, row)
	}
	if !a.dualPage {
		for page := 0; page < a.pageCount; page++ {
			appendRow(page)
		}
		return rows
	}
	for page := 0; page < a.pageCount; {
		if a.firstPageOffset && page == 0 {
			appendRow(page)
			page++
			continue
		}
		if page+1 < a.pageCount {
			appendRow(page, page+1)
			page += 2
			continue
		}
		appendRow(page)
		page++
	}
	return rows
}

func (a *App) baseRowIndexForPage(page int, rows []rowLayout) int {
	index := 0
	for i, row := range rows {
		for _, candidate := range row.pages {
			if candidate == page {
				return i
			}
		}
		index = i
	}
	return index
}

func (a *App) recomputeLayout(viewportW, viewportH int) {
	if len(a.pageMetrics) == 0 {
		return
	}
	base := a.baseRows()
	a.scale = a.currentScale(viewportW, viewportH)
	a.rows = make([]rowLayout, len(base))
	a.pageToRow = make([]int, a.pageCount)
	maxRowWidth := 0.0
	for _, row := range base {
		if row.width > maxRowWidth {
			maxRowWidth = row.width
		}
	}
	a.contentW = maxRowWidth*a.scale + float64(a.horizontalGap()*2)
	y := float64(a.verticalGap())
	for i, row := range base {
		row.width *= a.scale
		row.height *= a.scale
		row.x = float64(a.horizontalGap()) + (maxRowWidth*a.scale-row.width)/2
		row.y = y
		x := row.x
		for j, page := range row.pages {
			pw := row.pageW[j] * a.scale
			ph := row.pageH[j] * a.scale
			row.pageW[j] = pw
			row.pageH[j] = ph
			row.pageX[j] = x
			row.pageY[j] = y + (row.height-ph)/2
			x += pw + float64(a.horizontalGap())
			a.pageToRow[page] = i
		}
		a.rows[i] = row
		y += row.height + float64(a.verticalGap())
	}
	a.contentH = y
	if a.renderMode == "single" && len(a.rows) > 0 {
		row := a.rows[clampInt(a.pageToRow[a.page], 0, len(a.rows)-1)]
		a.contentW = row.width + float64(a.horizontalGap()*2)
		a.contentH = row.height + float64(a.verticalGap()*2)
	}
	a.clampScroll()
	if a.renderMode == "continuous" {
		a.updateCurrentPageFromScroll()
	}
}

func (a *App) clampScroll() {
	viewportW, viewportH := a.viewportSize()
	maxX := math.Max(0, a.contentW-float64(viewportW))
	maxY := math.Max(0, a.contentH-float64(viewportH))
	a.scrollX = clampFloat(a.scrollX, 0, maxX)
	a.scrollY = clampFloat(a.scrollY, 0, maxY)
}

func (a *App) renderScaleFor(layoutScale float64) float64 {
	if a.renderBaseScale <= 0 {
		a.ensureRenderBaseScale()
	}
	if a.renderBaseScale <= 0 {
		return math.Max(1, layoutScale)
	}
	if layoutScale < 0.1 {
		return math.Max(layoutScale, a.minRenderBaseScale)
	}
	if layoutScale < a.renderBaseScale/4 {
		return math.Max(layoutScale*2, a.minRenderBaseScale)
	}
	return a.renderBaseScale
}

func (a *App) renderDrawScale(rp *renderedPage, layoutScale float64) float64 {
	if rp == nil || rp.scale <= 0 {
		return 1
	}
	return layoutScale / rp.scale
}

func (a *App) ensureRenderBaseScale() {
	if a.renderBaseScale > 0 {
		return
	}
	base := math.Max(2, a.scale)
	if a.minRenderBaseScale > 0 && a.minRenderBaseScale < 0.25 {
		base = a.minRenderBaseScale
	} else if base < 0.25 {
		base = 0.25
	}
	a.renderBaseScale = base
}

func (a *App) maybeUpgradeRenderScale(target float64) bool {
	if target <= a.renderBaseScale*0.95 {
		return false
	}
	next := math.Max(a.renderBaseScale, 2)
	for next < target {
		next *= 1.5
	}
	next = math.Max(next, target)
	if next <= a.renderBaseScale+0.01 {
		return false
	}
	a.renderBaseScale = next
	a.invalidateRenderRequests()
	return true
}

func (a *App) maybeDowngradeRenderScale() {
	if a.renderBaseScale <= a.minRenderBaseScale {
		return
	}
	target := a.scale
	if a.fitMode != "manual" {
		target = math.Max(target, a.zoom)
	}
	safeLevel := target * 2.5
	if safeLevel >= a.renderBaseScale*0.95 {
		return
	}
	newScale := a.renderBaseScale / 1.5
	if newScale < a.minRenderBaseScale {
		newScale = a.minRenderBaseScale
	}
	if newScale >= a.renderBaseScale {
		return
	}
	a.renderBaseScale = newScale
	a.invalidateRenderRequests()
}

func (a *App) adjustRenderBaseScaleForExtremeZoom(layoutScale float64) {
	if layoutScale > a.renderBaseScale {
		return
	}
	if layoutScale < a.renderBaseScale/4 && a.renderBaseScale > 1 {
		a.maybeDowngradeRenderScale()
	}
}

func (a *App) updateCurrentPageFromScroll() {
	if len(a.rows) == 0 {
		return
	}
	a.page = a.rows[a.currentRowIndex()].pages[0]
}

func (a *App) currentRowIndex() int {
	if len(a.rows) == 0 {
		return 0
	}
	if a.renderMode == "single" {
		if a.page < 0 || a.page >= len(a.pageToRow) {
			return 0
		}
		return clampInt(a.pageToRow[a.page], 0, len(a.rows)-1)
	}
	_, offsetY := a.contentViewportOffset()
	marker := a.scrollY - offsetY + math.Max(1, float64(a.verticalGap())/2+1)
	if marker < 0 {
		marker = 0
	}
	current := 0
	for i, row := range a.rows {
		if row.y <= marker {
			current = i
			continue
		}
		break
	}
	return current
}

func (a *App) statusVisible() bool {
	return a.statusBarShown || a.mode != modeNormal
}

func (a *App) backgroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltBackground)
	}
	return rgb(a.config.Background)
}

func (a *App) pageBackgroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltPageBackground)
	}
	return rgb(a.config.PageBackground)
}

func (a *App) foregroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltForeground)
	}
	return rgb(a.config.Foreground)
}

func (a *App) statusBarColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltStatusBarColor)
	}
	return rgb(a.config.StatusBarColor)
}

func rgb(c [3]uint8) color.RGBA {
	return color.RGBA{R: c[0], G: c[1], B: c[2], A: 0xff}
}

func remapPageColors(img *image.RGBA, bg, fg [3]uint8) {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		if a == 0 {
			continue
		}
		r := img.Pix[i]
		g := img.Pix[i+1]
		b := img.Pix[i+2]
		lum := uint16(r)*77 + uint16(g)*150 + uint16(b)*29
		t := uint8(lum >> 8)
		img.Pix[i] = mixChannel(fg[0], bg[0], t)
		img.Pix[i+1] = mixChannel(fg[1], bg[1], t)
		img.Pix[i+2] = mixChannel(fg[2], bg[2], t)
	}
}

func mixChannel(fg, bg, t uint8) uint8 {
	return uint8((uint16(fg)*(255-uint16(t)) + uint16(bg)*uint16(t)) / 255)
}

func isCountableAction(action string) bool {
	switch action {
	case "next_page", "prev_page", "scroll_down", "scroll_up", "scroll_left", "scroll_right", "next_spread", "prev_spread", "zoom_in", "zoom_out", "search_next", "search_prev":
		return true
	default:
		return false
	}
}

func normalizeBinding(binding string) string {
	tokens := tokenizeBinding(binding)
	return strings.Join(tokens, " ")
}

func tokenizeBinding(binding string) []string {
	tokens := make([]string, 0, len(binding))
	for i := 0; i < len(binding); {
		if binding[i] == '<' {
			if end := strings.IndexByte(binding[i:], '>'); end > 0 {
				tokens = append(tokens, normalizeAngleToken(binding[i:i+end+1]))
				i += end + 1
				continue
			}
		}
		tokens = append(tokens, string(binding[i]))
		i++
	}
	return tokens
}

func normalizeAngleToken(token string) string {
	inner := strings.TrimSuffix(strings.TrimPrefix(token, "<"), ">")
	parts := strings.Split(inner, "-")
	for i, part := range parts {
		parts[i] = strings.ToLower(strings.TrimSpace(part))
	}
	if len(parts) == 1 && parts[0] == "space" {
		return " "
	}
	if len(parts) == 1 && (parts[0] == "enter" || parts[0] == "return") {
		return "<cr>"
	}
	return "<" + strings.Join(parts, "-") + ">"
}

func keyToken(key sdl.Keycode, mod sdl.Keymod) (string, bool) {
	ctrl := mod&sdl.KMOD_CTRL != 0
	shift := mod&sdl.KMOD_SHIFT != 0
	if ctrl {
		if base, ok := baseKeyName(key); ok {
			if shift {
				return "<c-s-" + base + ">", true
			}
			return "<c-" + base + ">", true
		}
	}
	if token, ok := specialKeyToken(key); ok {
		if shift {
			return "<s-" + strings.TrimSuffix(strings.TrimPrefix(strings.ToLower(token), "<"), ">") + ">", true
		}
		return normalizeAngleToken(token), true
	}
	if token, ok := printableKeyToken(key, shift); ok {
		return token, true
	}
	return "", false
}

func printableKeyToken(key sdl.Keycode, shift bool) (string, bool) {
	if key >= sdl.K_a && key <= sdl.K_z {
		r := rune('a' + (key - sdl.K_a))
		if shift {
			r -= 'a' - 'A'
		}
		return string(r), true
	}
	if key >= sdl.K_0 && key <= sdl.K_9 {
		return string(rune('0' + (key - sdl.K_0))), true
	}
	switch key {
	case sdl.K_SPACE:
		return " ", true
	case sdl.K_SLASH:
		if shift {
			return "?", true
		}
		return "/", true
	case sdl.K_SEMICOLON:
		if shift {
			return ":", true
		}
		return ";", true
	case sdl.K_EQUALS:
		if shift {
			return "+", true
		}
		return "=", true
	case sdl.K_MINUS:
		return "-", true
	default:
		return "", false
	}
}

func specialKeyToken(key sdl.Keycode) (string, bool) {
	switch key {
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		return "<CR>", true
	case sdl.K_ESCAPE:
		return "<Esc>", true
	case sdl.K_BACKSPACE:
		return "<BS>", true
	case sdl.K_PAGEDOWN:
		return "<PgDn>", true
	case sdl.K_PAGEUP:
		return "<PgUp>", true
	case sdl.K_TAB:
		return "<Tab>", true
	default:
		return "", false
	}
}

func mouseButtonEvent(button uint8, eventType uint32) (string, bool) {
	name, ok := mouseButtonName(button)
	if !ok {
		return "", false
	}
	switch eventType {
	case sdl.MOUSEBUTTONDOWN:
		return name + "_down", true
	case sdl.MOUSEBUTTONUP:
		return name + "_up", true
	default:
		return "", false
	}
}

func mouseButtonName(button uint8) (string, bool) {
	switch button {
	case sdl.BUTTON_LEFT:
		return "left", true
	case sdl.BUTTON_MIDDLE:
		return "middle", true
	case sdl.BUTTON_RIGHT:
		return "right", true
	case sdl.BUTTON_X1:
		return "x1", true
	case sdl.BUTTON_X2:
		return "x2", true
	default:
		return "", false
	}
}

func buttonMask(button uint8) uint32 {
	switch button {
	case sdl.BUTTON_LEFT:
		return sdl.ButtonLMask()
	case sdl.BUTTON_MIDDLE:
		return sdl.ButtonMMask()
	case sdl.BUTTON_RIGHT:
		return sdl.ButtonRMask()
	case sdl.BUTTON_X1:
		return sdl.ButtonX1Mask()
	case sdl.BUTTON_X2:
		return sdl.ButtonX2Mask()
	default:
		return 0
	}
}

func baseKeyName(key sdl.Keycode) (string, bool) {
	if key >= sdl.K_a && key <= sdl.K_z {
		return string(rune('a' + (key - sdl.K_a))), true
	}
	if key >= sdl.K_0 && key <= sdl.K_9 {
		return string(rune('0' + (key - sdl.K_0))), true
	}
	switch key {
	case sdl.K_SPACE:
		return "space", true
	case sdl.K_TAB:
		return "tab", true
	case sdl.K_RETURN, sdl.K_KP_ENTER:
		return "enter", true
	case sdl.K_ESCAPE:
		return "esc", true
	case sdl.K_BACKSPACE:
		return "bs", true
	case sdl.K_PAGEDOWN:
		return "pgdn", true
	case sdl.K_PAGEUP:
		return "pgup", true
	default:
		return "", false
	}
}

func boolWord(v bool, whenTrue, whenFalse string) string {
	if v {
		return whenTrue
	}
	return whenFalse
}

func (a *App) applyConfig(cfg config.Config) {
	currentPage := a.page
	a.config = cfg
	a.fontFace = loadFont(cfg.UIFontPath, cfg.UIFontSize)
	a.renderMode = sanitizeRenderMode(cfg.RenderMode)
	a.altColors = cfg.AltColors
	a.dualPage = cfg.DualPage
	a.firstPageOffset = cfg.FirstPageOffset
	a.statusBarShown = cfg.StatusBarVisible
	a.sequenceLookup = map[string]string{}
	a.mouseBindings = map[string]string{}
	for k, v := range cfg.KeyBindings {
		a.sequenceLookup[normalizeBinding(k)] = v
	}
	for k, v := range cfg.MouseBindings {
		a.mouseBindings[k] = v
	}
	if a.fitMode != "manual" {
		a.fitMode = sanitizeFitMode(cfg.FitMode)
	}
	a.clearCache()
	a.recomputeLayout(a.viewportSize())
	a.alignPageTop(clampInt(currentPage, 0, a.pageCount-1))
}

func sanitizeFitMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "width" || mode == "manual" {
		return mode
	}
	return "page"
}

func sanitizeRenderMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "single" {
		return mode
	}
	return "continuous"
}

func (a *App) verticalGap() int {
	if a.config.PageGapVertical >= 0 {
		return a.config.PageGapVertical
	}
	if a.config.PageGap >= 0 {
		return a.config.PageGap
	}
	return 0
}

func (a *App) horizontalGap() int {
	if a.config.PageGapHorizontal >= 0 {
		return a.config.PageGapHorizontal
	}
	if a.config.SpreadGap >= 0 {
		return a.config.SpreadGap
	}
	return 0
}

func lastRune(s string) (rune, int) {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i]&0xc0 != 0x80 {
			return []rune(s[i:])[0], len(s) - i
		}
	}
	return 0, 0
}

func splitAtRune(s string, pos int) (string, string) {
	if pos <= 0 {
		return "", s
	}
	runes := []rune(s)
	if pos >= len(runes) {
		return s, ""
	}
	return string(runes[:pos]), string(runes[pos:])
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func normalizeRotation(rotation float64) float64 {
	rotation = math.Mod(rotation, 360)
	if rotation < 0 {
		rotation += 360
	}
	return rotation
}

func rotatedBoundsSize(bounds mupdf.Rect, rotation float64) (float64, float64) {
	minX, minY, maxX, maxY := rotatedBounds(bounds, rotation)
	return maxX - minX, maxY - minY
}

func rotatedBoundsOrigin(bounds mupdf.Rect, scale, rotation float64) (float64, float64) {
	minX, minY, _, _ := rotatedBounds(bounds, rotation)
	return minX * scale, minY * scale
}

func rotatedBounds(bounds mupdf.Rect, rotation float64) (float64, float64, float64, float64) {
	points := []mupdf.Point{
		{X: float64(bounds.X0), Y: float64(bounds.Y0)},
		{X: float64(bounds.X1), Y: float64(bounds.Y0)},
		{X: float64(bounds.X0), Y: float64(bounds.Y1)},
		{X: float64(bounds.X1), Y: float64(bounds.Y1)},
	}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, point := range points {
		x, y := transformPoint(point.X, point.Y, 1, rotation)
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func transformPoint(x, y, scale, rotation float64) (float64, float64) {
	x *= scale
	y *= scale
	radians := normalizeRotation(rotation) * math.Pi / 180
	sin, cos := math.Sin(radians), math.Cos(radians)
	return x*cos - y*sin, x*sin + y*cos
}

func inverseTransformPoint(x, y, scale, rotation float64) (float64, float64) {
	if scale == 0 {
		return x, y
	}
	radians := normalizeRotation(rotation) * math.Pi / 180
	sin, cos := math.Sin(radians), math.Cos(radians)
	return (x*cos + y*sin) / scale, (-x*sin + y*cos) / scale
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
