package viewer

import (
	_ "embed"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
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
	bytes     int64
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
	loaded bool
}

type textSelection struct {
	active bool
	page   int
	anchor mupdf.Point
	focus  mupdf.Point
	quads  []mupdf.Quad
	text   string
}

type viewportAnchor struct {
	page  int
	point mupdf.Point
	valid bool
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
	runtime *config.Runtime
	config  config.Config
	verbose bool

	documentState
	viewStateFields
	layoutState
	sdlState
	renderService
	metricsService
	inputState
	interactionState
	uiState
	navigationState
}

type documentState struct {
	docPath string
	docName string
	doc     *mupdf.Document

	pageCount int
	page      int
	pageLinks map[int][]mupdf.Link
	outline   []mupdf.OutlineItem

	initialDocPath   string
	initialStartPage int
	document         documentSession
}

type viewStateFields struct {
	rotation        float64
	zoom            float64
	fitMode         string
	renderMode      string
	scale           float64
	dualPage        bool
	firstPageOffset bool
	statusBarShown  bool
	fullscreen      bool
	scrollX         float64
	scrollY         float64
	pageStep        float64
	altColors       bool
}

type layoutState struct {
	rows      []rowLayout
	pageToRow []int
	contentW  float64
	contentH  float64
	winW      int
	winH      int
}

type sdlState struct {
	window      *sdl.Window
	renderer    *sdl.Renderer
	cursorHand  *sdl.Cursor
	cursorArrow *sdl.Cursor
	iconBytes   []byte
	fontFace    font.Face
}

type inputState struct {
	mode           mode
	input          textInput
	ignoreText     string
	message        string
	mouseBindings  map[string]string
	searchInput    searchMode
	sequence       []string
	sequenceAt     time.Time
	sequenceLookup map[string]string
	pendingCount   string
}

type interactionState struct {
	selection     textSelection
	panning       bool
	panButton     uint8
	panKey        string
	mouseButton   uint8
	actionKey     string
	lastKeyUpCode sdl.Keycode
	lastKeyUpAt   time.Time
}

type uiState struct {
	pendingRedraw bool
	search        searchState
	searchWorker  *searchWorker
	outlineMenu   outlineMenuState
	keybindMenu   keybindMenuState
	luaUI         luaUIState
	completion    completionState
}

type navigationState struct {
	quit        bool
	pendingOpen string
	jumpBack    []jumpPosition
	jumpAhead   []jumpPosition
}

type jumpPosition struct {
	page    int
	scrollX float64
	scrollY float64
}

func New(docPath string, runtime *config.Runtime, startPage int, iconBytes []byte, verbose ...bool) (*App, error) {
	cfg := runtime.Config()
	if startPage < 0 {
		startPage = 0
	}
	app := &App{
		runtime: runtime,
		verbose: len(verbose) > 0 && verbose[0],
		documentState: documentState{
			page:      startPage,
			pageLinks: map[int][]mupdf.Link{},
		},
		viewStateFields: viewStateFields{
			zoom:     1,
			scale:    1,
			pageStep: 64,
		},
		sdlState: sdlState{
			iconBytes: iconBytes,
		},
		renderService: renderService{
			renderCache:        map[string]*renderedPage{},
			cacheLimit:         0,
			cacheByteLimit:     defaultRenderCacheByteLimit,
			minRenderBaseScale: 0.25,
		},
		metricsService: metricsService{},
		inputState: inputState{
			mouseBindings:  map[string]string{},
			sequenceLookup: map[string]string{},
		},
	}
	app.logf("create viewer doc=%q startPage=%d", docPath, startPage+1)
	runtime.AttachHost(app)
	app.applyConfigState(cfg, false)
	app.message = cfg.NormalMessage
	if docPath != "" {
		app.initialDocPath = docPath
		app.initialStartPage = startPage
	}
	app.recomputeLayout(1400, 900-app.config.StatusBarHeight)
	if app.doc != nil {
		app.ensureRenderBaseScale()
		app.alignPageTop(startPage)
	}
	return app, nil
}

func (a *App) logf(format string, args ...any) {
	if a != nil && a.verbose {
		log.Printf(format, args...)
	}
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
	a.document.Close()
	a.closeDocumentResources()
	closeFontFace(a.fontFace)
	a.fontFace = nil
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
		config.SetLastState(config.LastState{Path: a.docPath, Page: a.page + 1})
	}
}

func (a *App) closeDocumentResources() {
	a.closeRenderWorker()
	a.closeMetricLoader()
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
				a.editInput(func(input *textInput) { input.MoveWordLeft() })
			} else {
				a.editInput(func(input *textInput) { input.Move(-1) })
			}
			return
		case sdl.KeycodeRight:
			if e.Mod&sdl.KeymodCtrl != 0 {
				a.editInput(func(input *textInput) { input.MoveWordRight() })
			} else {
				a.editInput(func(input *textInput) { input.Move(1) })
			}
			return
		case sdl.KeycodeBackspace:
			a.editInput(func(input *textInput) { input.Backspace() })
			return
		case sdl.KeycodeDelete:
			a.editInput(func(input *textInput) { input.Delete() })
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
	if ctrl && e.Key == sdl.KeycodeV {
		a.editInput(func(input *textInput) { input.InsertText(sdlGetClipboardText()) })
		return true
	}
	if ctrl && e.Key == sdl.KeycodeW {
		a.editInput(func(input *textInput) { input.DeleteWordLeft() })
		return true
	}
	if ctrl && e.Key == sdl.KeycodeBackspace {
		a.editInput(func(input *textInput) { input.DeleteWordLeft() })
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
			a.editInput(func(input *textInput) { input.InsertRune(r) })
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
	input := strings.TrimSpace(a.input.Value)
	currentMode := a.mode
	a.mode = modeNormal
	a.input.Reset()
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

func (a *App) editInput(edit func(*textInput)) {
	a.closeCompletion()
	edit(&a.input)
}

func (a *App) currentScale(viewportW, viewportH int) float64 {
	return a.currentScaleFromRows(viewportW, viewportH, a.baseRows())
}

func (a *App) currentScaleFromRows(viewportW, viewportH int, baseRows []rowLayout) float64 {
	if a.fitMode == "manual" {
		return a.zoom
	}
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
	a.relayoutWithViewportAnchor(func() {
		baseZoom := a.zoom
		if a.fitMode != "manual" {
			baseZoom = a.scale
		}
		a.fitMode = "manual"
		a.zoom = math.Max(0.75, math.Min(4.0, baseZoom*delta))
		a.maybeUpgradeRenderScale(a.zoom)
	})
}

func (a *App) setFitMode(mode string) {
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = mode
		a.maybeUpgradeRenderScale(a.zoom)
	})
}
