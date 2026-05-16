package viewer

import (
	_ "embed"
	"fmt"
	"image/color"
	"maps"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"gopdf/internal/config"
	"gopdf/internal/filepicker"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
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
	texture   *sdl.Texture
	width     float64
	height    float64
	pixX      float64
	pixY      float64
	key       string
	page      int
	scale     float64
	altColors bool
	aaLevel   int
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
	iconBytes   []byte

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
	actionKey     string
	lastKeyUpCode sdl.Keycode
	lastKeyUpAt   time.Time
	pageLinks     map[int][]mupdf.Link
	search       searchState
	searchWorker *searchWorker
	outline      []mupdf.OutlineItem
	outlineMenu  outlineMenuState
	keybindMenu  keybindMenuState
	luaUI        luaUIState
	completion   completionState

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

func New(docPath string, runtime *config.Runtime, startPage int, iconBytes []byte) (*App, error) {
	cfg := runtime.Config()
	if startPage < 0 {
		startPage = 0
	}
	app := &App{
		runtime:            runtime,
		page:               startPage,
		zoom:               1,
		scale:              1,
		pageStep:           64,
		renderCache:        map[string]*renderedPage{},
		cacheLimit:         0,
		minRenderBaseScale: 0.25,
		mouseBindings:      map[string]string{},
		pageLinks:          map[int][]mupdf.Link{},
		iconBytes:          iconBytes,
		sequenceLookup:     map[string]string{},
	}
	runtime.AttachHost(app)
	app.applyConfigState(cfg, false)
	app.message = cfg.NormalMessage
	if docPath != "" {
		if err := app.openDocument(docPath, startPage, false); err != nil {
			app.Close()
			return nil, err
		}
	}
	app.recomputeLayout(1400, 900-app.config.StatusBarHeight)
	if app.doc != nil {
		app.ensureRenderBaseScale()
		app.alignPageTop(startPage)
	}
	return app, nil
}

func (a *App) setWindowTitle() {
	if a.window == nil {
		return
	}
	if a.docName == "" {
		sdl.SetWindowTitle(a.window, "gopdf")
		return
	}
	sdl.SetWindowTitle(a.window, a.docName+" - gopdf")
}

func (a *App) Close() {
	a.closeDocumentResources()
	if a.cursorHand != nil {
		sdl.DestroyCursor(a.cursorHand)
		a.cursorHand = nil
	}
	if a.cursorArrow != nil {
		sdl.DestroyCursor(a.cursorArrow)
		a.cursorArrow = nil
	}
	if a.renderer != nil {
		sdl.DestroyRenderer(a.renderer)
		a.renderer = nil
	}
	if a.window != nil {
		sdl.DestroyWindow(a.window)
		a.window = nil
	}
	sdl.Quit()
	if a.docPath != "" {
		config.SetLastFile(a.docPath)
	}
}

func (a *App) closeDocumentResources() {
	a.closeRenderWorker()
	a.closeSearch()
	a.clearCache()
	if a.doc != nil {
		a.doc.Close()
		a.doc = nil
	}
}

func (a *App) PendingOpen() string {
	return a.pendingOpen
}

func (a *App) Run() error {
	if !sdl.Init(sdl.InitVideo) {
		return fmt.Errorf("SDL init failed: %s", sdl.GetError())
	}
	sdl.SetHint("SDL_RENDER_SCALE_QUALITY", "2")
	var window *sdl.Window
	var renderer *sdl.Renderer
	if !sdl.CreateWindowAndRenderer("gopdf", 1400, 900, sdl.WindowResizable|sdl.WindowHighPixelDensity, &window, &renderer) {
		sdl.Quit()
		return fmt.Errorf("SDL window creation failed: %s", sdl.GetError())
	}
	a.window = window
	a.renderer = renderer
	if rw := sdl.IOFromConstMem(a.iconBytes); rw != nil {
		if icon := sdl.LoadBMPIO(rw, true); icon != nil {
			sdl.SetWindowIcon(window, icon)
			sdl.DestroySurface(icon)
		}
	}
	a.cursorHand = sdl.CreateSystemCursor(sdl.SystemCursorPointer)
	a.cursorArrow = sdl.CreateSystemCursor(sdl.SystemCursorDefault)
	sdl.SetEventEnabled(sdl.EventDropFile, true)
	a.setWindowTitle()
	sdl.SetRenderDrawBlendMode(a.renderer, sdl.BlendModeBlend)
	sdl.SetDefaultTextureScaleMode(a.renderer, sdl.ScaleModeLinear)
	var outputW, outputH int32
	if sdl.GetRenderOutputSize(a.renderer, &outputW, &outputH) {
		w, h := outputW, outputH
		a.winW, a.winH = int(w), int(h)
	}
	a.recomputeLayout(a.viewportSize())
	a.pendingRedraw = true
	sdl.StartTextInput(a.window)
	defer sdl.StopTextInput(a.window)
	for !a.quit {
		var event sdl.Event
		for sdl.PollEvent(&event) {
			if err := a.handleSDLEvent(&event); err != nil {
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
			var event sdl.Event
			if sdl.WaitEventTimeout(&event, int32(a.eventWaitTimeoutMS())) {
				if err := a.handleSDLEvent(&event); err != nil {
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
			return max(1, int(remaining/time.Millisecond))
		}
	}
	return 100
}

func (a *App) handleSDLEvent(event *sdl.Event) error {
	defer func() { a.pendingRedraw = true }()
	switch event.Type() {
	case sdl.EventQuit:
		a.quit = true
	case sdl.EventWindowResized, sdl.EventWindowPixelSizeChanged:
		e := event.Window()
		{
			a.winW = int(e.Data1)
			a.winH = int(e.Data2)
			a.recomputeLayout(a.viewportSize())
		}
	case sdl.EventKeyUp:
		e := event.Key()
		a.handleSDLKeyUp(&e)
	case sdl.EventKeyDown:
		e := event.Key()
		a.handleSDLKeyDown(&e)
	case sdl.EventTextInput:
		e := event.Text()
		a.handleSDLTextInput(&e)
	case sdl.EventMouseWheel:
		e := event.Wheel()
		a.handleSDLMouseWheel(&e)
	case sdl.EventMouseButtonDown, sdl.EventMouseButtonUp:
		e := event.Button()
		a.handleSDLMouseButton(&e)
	case sdl.EventMouseMotion:
		e := event.Motion()
		a.handleSDLMouseMotion(&e)
	case sdl.EventDropFile:
		e := event.Drop()
		a.handleDroppedFile(e.Data())
	}
	return nil
}

func (a *App) handleDroppedFile(path string) {
	if path == "" {
		return
	}
	if err := a.Open(path); err != nil {
		a.message = err.Error()
	}
}

func (a *App) drawFrame() error {
	if a.renderer == nil {
		return nil
	}
	var w, h int32
	if sdl.GetRenderOutputSize(a.renderer, &w, &h) {
		a.winW, a.winH = int(w), int(h)
	}
	bg := a.backgroundColor()
	if !sdl.SetRenderDrawColor(a.renderer, bg.R, bg.G, bg.B, bg.A) {
		return fmt.Errorf("SDL draw color failed: %s", sdl.GetError())
	}
	if !sdl.RenderClear(a.renderer) {
		return fmt.Errorf("SDL clear failed: %s", sdl.GetError())
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
	if a.completion.visible {
		if err := a.drawCompletion(a.renderer); err != nil {
			return err
		}
	}
	if a.keybindMenu.visible {
		if err := a.drawKeybindMenu(a.renderer); err != nil {
			return err
		}
	}
	if a.outlineMenu.visible {
		if err := a.drawOutlineMenu(a.renderer); err != nil {
			return err
		}
	}
	if a.luaUI.visible {
		if err := a.drawLuaUI(a.renderer); err != nil {
			return err
		}
	}
	sdl.RenderPresent(a.renderer)
	return nil
}

func (a *App) handleSDLKeyDown(e *sdl.KeyboardEvent) {
	if e.Repeat && e.Key == a.lastKeyUpCode && time.Since(a.lastKeyUpAt) < 100*time.Millisecond {
		if a.ignoreText == "" {
			if token, ok := keyToken(e.Key, e.Mod); ok && utf8.RuneCountInString(token) == 1 {
				a.ignoreText = token
			}
		}
		return
	}
	if a.luaUI.visible && a.handleLuaUIKey(e) {
		return
	}
	if a.keybindMenu.visible && a.handleKeybindMenuKey(e) {
		return
	}
	if a.outlineMenu.visible && a.handleOutlineMenuKey(e) {
		return
	}
	if a.mode != modeNormal {
		if a.handleInputEditKey(e) {
			return
		}
		switch e.Key {
		case sdl.KeycodeLeft:
			if e.Mod&sdl.KeymodCtrl != 0 {
				a.moveInputCursorLeftWord()
			} else {
				a.moveInputCursor(-1)
			}
			return
		case sdl.KeycodeRight:
			if e.Mod&sdl.KeymodCtrl != 0 {
				a.moveInputCursorRightWord()
			} else {
				a.moveInputCursor(1)
			}
			return
		case sdl.KeycodeBackspace:
			a.backspaceInput()
			return
		case sdl.KeycodeDelete:
			a.deleteInput()
			return
		}
		if token, ok := keyToken(e.Key, e.Mod); ok && a.handleInputModeBinding(token) {
			return
		}
	}
	if a.mode == modeNormal {
		if token, ok := keyToken(e.Key, e.Mod); ok {
			prevMode := a.mode
			if a.handleCountToken(token) {
				return
			}
			if !e.Repeat {
				a.actionKey = token
				a.pushToken(token)
				a.actionKey = ""
			}
			if prevMode == modeNormal && a.mode != modeNormal && utf8.RuneCountInString(token) == 1 {
				a.ignoreText = token
			}
		}
		return
	}
}

func (a *App) handleInputEditKey(e *sdl.KeyboardEvent) bool {
	ctrl := e.Mod&sdl.KeymodCtrl != 0
	if ctrl && e.Key == sdl.KeycodeW {
		a.deleteInputWord()
		return true
	}
	if ctrl && e.Key == sdl.KeycodeBackspace {
		a.deleteInputWord()
		return true
	}
	return false
}

func (a *App) handleInputModeBinding(token string) bool {
	if !strings.HasPrefix(token, "<") {
		return false
	}
	action, ok := a.sequenceLookup[normalizeBinding(token)]
	if !ok {
		return false
	}
	if a.completion.visible {
		switch action {
		case "confirm", "show_completion", "next_completion", "prev_completion", "close":
		default:
			a.closeCompletion()
		}
	}
	a.runAction(action)
	return true
}

func (a *App) handleSDLKeyUp(e *sdl.KeyboardEvent) {
	a.lastKeyUpCode = e.Key
	a.lastKeyUpAt = time.Now()
	if !a.panning || a.panKey == "" {
		return
	}
	if token, ok := keyToken(e.Key, e.Mod); ok && token == a.panKey {
		a.stopPan()
	}
}

func (a *App) handleSDLTextInput(e *sdl.TextInputEvent) {
	text := e.Text()
	if text == "" {
		return
	}
	if a.ignoreText != "" {
		text, _ = strings.CutPrefix(text, a.ignoreText)
		a.ignoreText = ""
		if text == "" {
			return
		}
	}
	if a.outlineMenu.visible && a.outlineMenu.searching {
		a.insertOutlineSearchText(text)
		return
	}
	if a.keybindMenu.visible || a.luaUI.visible {
		return
	}
	if a.outlineMenu.visible && !a.outlineMenu.searching {
		return
	}
	if a.mode == modeNormal {
		return
	}
	for _, r := range text {
		if r >= 0x20 && r != 0x7f {
			a.insertInputRune(r)
		}
	}
}

func (a *App) handleSDLMouseWheel(e *sdl.MouseWheelEvent) {
	if a.luaUI.visible {
		if e.Y < 0 {
			a.scrollLuaUI(1)
		} else if e.Y > 0 {
			a.scrollLuaUI(-1)
		}
		return
	}
	if a.keybindMenu.visible {
		if e.Y < 0 {
			a.scrollKeybindMenu(1)
		} else if e.Y > 0 {
			a.scrollKeybindMenu(-1)
		}
		return
	}
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
	wx, wy := e.X, e.Y
	if wx == 0 {
		wx = float32(e.IntegerX)
	}
	if wy == 0 {
		wy = float32(e.IntegerY)
	}
	if e.Direction == sdl.MouseWheelFlipped {
		wx = -wx
		wy = -wy
	}
	ctrl := sdl.GetModState()&sdl.KeymodCtrl != 0
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
	if a.luaUI.visible {
		if e.Type == sdl.EventMouseButtonDown && e.Button == uint8(sdl.ButtonLeft) {
			a.clickLuaUI(int(e.X), int(e.Y))
		}
		return
	}
	if a.keybindMenu.visible {
		if e.Type == sdl.EventMouseButtonUp && e.Button == uint8(sdl.ButtonLeft) {
			a.keybindMenu.draggingScrollbar = false
			return
		}
		if e.Type == sdl.EventMouseButtonDown && e.Button == uint8(sdl.ButtonLeft) {
			a.clickKeybindMenu(int(e.X), int(e.Y))
		}
		return
	}
	if a.outlineMenu.visible {
		if e.Type == sdl.EventMouseButtonUp && e.Button == uint8(sdl.ButtonLeft) {
			a.outlineMenu.draggingScrollbar = false
			return
		}
		if e.Type == sdl.EventMouseButtonDown && e.Button == uint8(sdl.ButtonLeft) {
			a.clickOutlineMenu(int(e.X), int(e.Y))
		}
		return
	}
	if e.Type == sdl.EventMouseButtonUp && a.panning && e.Button == a.panButton {
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
	if e.Button != uint8(sdl.ButtonLeft) || !a.config.MouseTextSelect {
		if e.Button == uint8(sdl.ButtonLeft) && e.Type == sdl.EventMouseButtonDown {
			a.tryActivateLinkAt(float64(e.X), float64(e.Y))
		}
		return
	}
	if e.Type == sdl.EventMouseButtonDown {
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
	if e.Type == sdl.EventMouseButtonUp && a.selection.active {
		a.copySelectionToClipboard()
		a.selection.active = false
		a.selection.quads = nil
	}
}

func (a *App) handleSDLMouseMotion(e *sdl.MouseMotionEvent) {
	if a.luaUI.visible {
		a.hoverLuaUI(int(e.X), int(e.Y))
		return
	}
	if a.keybindMenu.visible {
		if a.keybindMenu.draggingScrollbar {
			a.dragKeybindScrollbar(int(e.Y))
			return
		}
		a.hoverKeybindMenu(int(e.X), int(e.Y))
		return
	}
	if a.outlineMenu.visible && a.outlineMenu.draggingScrollbar {
		a.dragOutlineScrollbar(int(e.Y))
		return
	}
	if a.outlineMenu.visible {
		a.hoverOutlineMenu(int(e.X), int(e.Y))
		return
	}
	if a.panning && (a.panButton == 0 || uint32(e.State)&buttonMask(a.panButton) != 0) {
		a.scrollBy(-float64(e.Xrel), -float64(e.Yrel))
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

	if !a.selection.active || uint32(e.State)&uint32(sdl.ButtonLMask) == 0 {
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
	for range count {
		a.runAction(action)
	}
	return true
}

func (a *App) commitInputMode() {
	if a.completion.visible {
		a.acceptCompletion()
		return
	}
	input := strings.TrimSpace(a.input)
	currentMode := a.mode
	a.mode = modeNormal
	a.input = ""
	a.inputCursor = 0
	a.closeCompletion()
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
	a.closeCompletion()
	left, right := splitAtRune(a.input, a.inputCursor)
	a.input = left + string(r) + right
	a.inputCursor++
}

func (a *App) backspaceInput() {
	a.closeCompletion()
	if a.inputCursor <= 0 || a.input == "" {
		return
	}
	left, right := splitAtRune(a.input, a.inputCursor)
	_, size := lastRune(left)
	a.input = left[:len(left)-size] + right
	a.inputCursor--
}

func (a *App) deleteInputWord() {
	a.closeCompletion()
	if a.inputCursor <= 0 || a.input == "" {
		return
	}
	runes := []rune(a.input)
	end := clampInt(a.inputCursor, 0, len(runes))
	start := end
	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	a.input = string(runes[:start]) + string(runes[end:])
	a.inputCursor = start
}

func (a *App) moveInputCursor(delta int) {
	a.closeCompletion()
	a.inputCursor = clampInt(a.inputCursor+delta, 0, utf8.RuneCountInString(a.input))
}

func (a *App) deleteInput() {
	a.closeCompletion()
	runes := []rune(a.input)
	if a.inputCursor >= len(runes) {
		return
	}
	left, right := splitAtRune(a.input, a.inputCursor)
	_, after := splitAtRune(right, 1)
	a.input = left + after
}

func (a *App) moveInputCursorLeftWord() {
	a.closeCompletion()
	if a.inputCursor <= 0 || a.input == "" {
		return
	}
	runes := []rune(a.input)
	pos := clampInt(a.inputCursor, 0, len(runes))
	for pos > 0 && unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	a.inputCursor = pos
}

func (a *App) moveInputCursorRightWord() {
	a.closeCompletion()
	runes := []rune(a.input)
	if a.inputCursor >= len(runes) {
		return
	}
	pos := clampInt(a.inputCursor, 0, len(runes))
	for pos < len(runes) && unicode.IsSpace(runes[pos]) {
		pos++
	}
	for pos < len(runes) && !unicode.IsSpace(runes[pos]) {
		pos++
	}
	a.inputCursor = pos
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
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
				continue
			}
			_ = a.drawPageBackground(renderer, x, y, page)
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
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
		x := baseX + (row.pageX[i] - row.x) - a.scrollX
		y := baseY + (row.pageY[i] - row.y) - a.scrollY
		if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
			continue
		}
		_ = a.drawPageBackground(renderer, x, y, page)
		rp, ok := a.cachedRenderPage(page, a.scale)
		if !ok {
			continue
		}
		drawScale := a.renderDrawScale(rp, a.scale)
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
	if normalizeRotation(a.rotation) == 0 {
		sdl.RenderTexture(renderer, rp.texture, nil, &dst)
		return
	}
	sdl.RenderTextureRotated(renderer, rp.texture, nil, &dst, a.rotation, nil, sdl.FlipNone)
}

func (a *App) drawPageBackground(renderer *sdl.Renderer, x, y float64, page int) error {
	clr := a.pageBackgroundColor()
	if normalizeRotation(a.rotation) == 0 {
		m := a.pageMetrics[page]
		return fillRect(renderer, sdl.FRect{X: float32(x), Y: float32(y), W: float32(m.width * a.scale), H: float32(m.height * a.scale)}, clr)
	}
	return renderBool(sdl.RenderGeometry(renderer, nil, pageBackgroundVertices(x, y, a.pageMetrics[page].bounds, a.scale, a.rotation, clr), []int32{0, 1, 2, 1, 3, 2}), "render geometry")
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
	color := sdl.FColor{R: float32(clr.R) / 255, G: float32(clr.G) / 255, B: float32(clr.B) / 255, A: float32(clr.A) / 255}
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
	key := renderCacheKey(page, renderScale, a.altColors, a.config.AntiAliasing)
	if rp, ok := a.renderCache[key]; ok {
		return rp, true
	}
	var bestHigher *renderedPage
	var bestLower *renderedPage
	for _, rp := range a.renderCache {
		if rp.page != page || rp.altColors != a.altColors || rp.aaLevel != a.config.AntiAliasing {
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
	visible := []int{}
	prefetch := []int{}
	seen := map[int]bool{}
	if a.renderMode == "single" {
		if a.page < 0 || a.page >= len(a.pageToRow) {
			return
		}
		row := a.rows[a.pageToRow[a.page]]
		for _, page := range row.pages {
			visible = append(visible, page)
			seen[page] = true
		}
	} else {
		viewportW, viewportH := a.viewportSize()
		offsetX, offsetY := a.contentViewportOffset()
		for _, row := range a.rows {
			rowY := row.y - a.scrollY + offsetY
			if rowY+row.height < 0 || rowY > float64(viewportH) {
				continue
			}
			for i, page := range row.pages {
				x := row.pageX[i] - a.scrollX + offsetX
				y := row.pageY[i] - a.scrollY + offsetY
				if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
					continue
				}
				visible = append(visible, page)
				seen[page] = true
			}
		}
		margin := math.Max(a.renderMargin()*2, float64(viewportH))
		minY := a.scrollY - margin
		maxY := a.scrollY + float64(viewportH) + margin
		for _, row := range a.rows {
			if row.y+row.height < minY || row.y > maxY {
				continue
			}
			for _, page := range row.pages {
				if seen[page] {
					continue
				}
				prefetch = append(prefetch, page)
				seen[page] = true
			}
		}
	}
	totalPrefetch := len(visible) + len(prefetch)
	if totalPrefetch*2 > a.cacheLimit {
		a.cacheLimit = min(a.pageCount*2, totalPrefetch*2)
	}
	if a.renderWorker != nil {
		a.renderWorker.DrainNotIn(seen, a.renderGeneration)
	}
	for key, req := range a.renderPending {
		if req.generation == a.renderGeneration && !seen[req.page] {
			delete(a.renderPending, key)
		}
	}
	for _, page := range visible {
		a.requestRender(page, a.scale)
	}
	for _, page := range prefetch {
		a.requestRender(page, a.scale)
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
		sdl.DestroyTexture(rp.texture)
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
		a.closeAllUI()
		a.mode = modeCommand
		a.input = ""
		a.inputCursor = 0
	case "search_prompt":
		a.closeAllUI()
		a.mode = modeSearch
		a.searchInput = searchModeForward
		a.input = ""
		a.inputCursor = 0
	case "search_prompt_backward":
		a.closeAllUI()
		a.mode = modeSearch
		a.searchInput = searchModeBackward
		a.input = ""
		a.inputCursor = 0
	case "goto_page_prompt":
		a.closeAllUI()
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
	case "keybinds":
		a.toggleKeybindMenu()
	case "confirm":
		if a.completion.visible {
			a.acceptCompletion()
		} else if a.outlineMenu.visible {
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
	case "show_completion":
		a.showCompletion()
	case "next_completion":
		a.moveCompletion(1)
	case "prev_completion":
		a.moveCompletion(-1)
	case "jump_forward":
		a.jumpForward()
	case "jump_backward":
		a.jumpBackward()
	case "open_file_picker":
		path, err := filepicker.PickPDF()
		if err != nil {
			return err
		}
		if path != "" {
			return a.Open(path)
		}
	case "clear_search":
		a.clearSearch()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

func (a *App) closeAllUI() {
	a.luaUI.visible = false
	a.keybindMenu.visible = false
	a.outlineMenu.visible = false
	if a.mode != modeNormal {
		a.closeCompletion()
		a.mode = modeNormal
		a.input = ""
		a.inputCursor = 0
		a.ignoreText = ""
	}
	if a.search.query != "" || len(a.search.order) > 0 || a.search.running {
		a.clearSearch()
	}
}

func (a *App) closeActiveUI() {
	if a.luaUI.visible {
		a.closeLuaUI(true)
		return
	}
	if a.keybindMenu.visible {
		a.keybindMenu = keybindMenuState{}
		return
	}
	if a.outlineMenu.visible {
		a.outlineMenu.visible = false
		return
	}
	if a.mode != modeNormal {
		if a.completion.visible {
			a.closeCompletion()
			return
		}
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
	path = a.resolveOpenPath(path)
	if a.runtime == nil {
		a.pendingOpen = path
		a.quit = true
		return nil
	}
	return a.openDocument(path, 0, true)
}

func (a *App) resolveOpenPath(path string) string {
	path = expandHomePath(path)
	if !filepath.IsAbs(path) {
		if a.docPath != "" {
			dir := filepath.Dir(a.docPath)
			path = filepath.Join(dir, path)
		}
	}
	return absoluteOpenPath(path)
}

func (a *App) openDocument(path string, startPage int, reloadConfig bool) error {
	path = absoluteOpenPath(path)
	doc, err := mupdf.Open(path)
	if err != nil {
		return err
	}
	pages, err := doc.PageCount()
	if err != nil {
		doc.Close()
		return err
	}
	if startPage < 0 {
		startPage = 0
	}
	if pages > 0 && startPage >= pages {
		startPage = pages - 1
	}
	metrics, err := pageMetricsForDocument(doc, pages, 0)
	if err != nil {
		doc.Close()
		return err
	}

	a.closeDocumentResources()

	a.docPath = path
	a.docName = filepath.Base(path)
	a.doc = doc
	a.pageCount = pages
	a.page = startPage
	a.rotation = 0
	a.zoom = 1
	a.scale = 1
	a.scrollX = 0
	a.scrollY = 0
	a.pageMetrics = metrics
	a.rows = nil
	a.pageToRow = nil
	a.contentW = 0
	a.contentH = 0
	a.cacheLimit = min(24, pages)
	a.renderBaseScale = 0
	a.renderScaleTime = 0
	a.pageLinks = map[int][]mupdf.Link{}
	a.search = searchState{}
	a.outline = nil
	a.outlineMenu = outlineMenuState{}
	a.keybindMenu = keybindMenuState{}
	a.luaUI = luaUIState{}
	a.completion = completionState{}
	a.selection = textSelection{}
	a.mode = modeNormal
	a.input = ""
	a.inputCursor = 0
	a.ignoreText = ""
	a.sequence = nil
	a.pendingCount = ""
	a.jumpBack = nil
	a.jumpAhead = nil
	a.pendingOpen = ""
	a.message = "opening " + path

	var configErr error
	if reloadConfig {
		configErr = a.runtime.SetDocument(path)
	}
	a.applyConfigState(a.runtime.Config(), false)
	a.message = a.config.NormalMessage

	a.setWindowTitle()
	a.initRenderWorker()
	a.initSearch()
	if outline, err := doc.Outline(); err == nil {
		a.outline = outline
	} else if configErr == nil {
		a.message = err.Error()
	}
	a.recomputeLayout(a.viewportSize())
	a.ensureRenderBaseScale()
	a.alignPageTop(startPage)
	a.pendingRedraw = true
	if configErr != nil {
		a.message = configErr.Error()
		return configErr
	}
	return nil
}

func absoluteOpenPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func (a *App) applyConfigState(cfg config.Config, preserveManualFit bool) {
	currentFitMode := a.fitMode
	a.config = cfg
	a.fitMode = sanitizeFitMode(cfg.FitMode)
	if preserveManualFit && currentFitMode == "manual" {
		a.fitMode = currentFitMode
	}
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
	maps.Copy(a.mouseBindings, cfg.MouseBindings)
	a.fontFace = loadFont(cfg.UIFontPath, cfg.UIFontSize)
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
		return boolError(sdl.SetWindowFullscreen(a.window, true), "set fullscreen")
	}
	return boolError(sdl.SetWindowFullscreen(a.window, false), "set fullscreen")
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
	command := strings.TrimPrefix(strings.TrimSpace(input), ":")
	command = strings.TrimSpace(command)
	if _, err := strconv.Atoi(command); err == nil {
		a.gotoPageInput(command)
		return
	}
	name, args, _ := strings.Cut(command, " ")
	args = strings.TrimSpace(args)
	fields := strings.Fields(args)
	if name == "" {
		return
	}
	switch name {
	case "q", "quit":
		a.quit = true
	case "page", "p":
		if len(fields) < 1 {
			a.message = "usage: :page <n>"
			return
		}
		a.gotoPageInput(fields[0])
	case "set":
		if len(fields) < 1 {
			return
		}
		a.runSet(fields[0])
	case "mode":
		if len(fields) < 1 {
			a.message = "usage: :mode continuous|single"
			return
		}
		page := a.page
		a.renderMode = sanitizeRenderMode(fields[0])
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(page)
	case "colors":
		if len(fields) < 1 {
			a.message = "usage: :colors normal|alt"
			return
		}
		a.altColors = strings.EqualFold(fields[0], "alt")
	case "fit":
		if len(fields) < 1 {
			return
		}
		a.setFitMode(sanitizeFitMode(fields[0]))
	case "reload-config":
		a.reloadConfig()
	case "keybinds":
		a.toggleKeybindMenu()
	case "search":
		a.startSearch(args, searchModeForward)
	case "open":
		if args == "" {
			a.message = "usage: :open <filename>"
			return
		}
		if err := a.Open(unescapeCommandArg(args)); err != nil {
			a.message = err.Error()
		}
	case "lua":
		if a.runtime == nil {
			a.message = "no Lua runtime"
			return
		}
		if err := a.runtime.Eval(args); err != nil {
			a.message = err.Error()
		}
	case "help":
		a.message = commandHelpMessage()
	default:
		a.message = "unknown command: " + name
	}
}

func unescapeCommandArg(arg string) string {
	if !strings.Contains(arg, `\`) {
		return arg
	}
	var b strings.Builder
	b.Grow(len(arg))
	for i := 0; i < len(arg); i++ {
		if arg[i] == '\\' && i+1 < len(arg) && (arg[i+1] == ' ' || arg[i+1] == '\\') {
			b.WriteByte(arg[i+1])
			i++
			continue
		}
		b.WriteByte(arg[i])
	}
	return b.String()
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
	return ":open file | :page N | :search text | :mode continuous|single | :colors normal|alt | :keybinds | :set render_mode!|alt_colors!|dual_page!|first_page_offset!|status_bar! | :fit width|page | :lua <code> | :reload-config | :quit"
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

func (a *App) runMouseBinding(event string) bool {
	if action, ok := a.mouseBindings[event]; ok {
		a.runAction(action)
		return true
	}
	return false
}

func boolWord(v bool, whenTrue, whenFalse string) string {
	if v {
		return whenTrue
	}
	return whenFalse
}

func (a *App) applyConfig(cfg config.Config) {
	currentPage := a.page
	a.applyConfigState(cfg, true)
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
