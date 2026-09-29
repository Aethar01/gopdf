package viewer

import (
	_ "embed"
	"log"
	"math"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"gopdf/internal/actions"
	"gopdf/internal/config"
	"gopdf/internal/instance"
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
	modePassword
	modePrompt // a one-off question answered through a.promptState
)

// pageMetrics holds a page's geometry. bounds is what layout and rendering
// use: the full page, or its content box while margins are trimmed.
type pageMetrics struct {
	bounds     mupdf.Rect
	full       mupdf.Rect
	content    mupdf.Rect
	hasContent bool
	width      float64
	height     float64
	label      string
	loaded     bool
}

// textSelection runs from anchor on anchorPage to focus on focusPage,
// either of which may come first.
type textSelection struct {
	active     bool
	mode       mupdf.SelectMode // words or lines after a double or triple click
	anchorPage int
	anchor     mupdf.Point
	focusPage  int
	focus      mupdf.Point
	parts      []selectionPart // highlighted quads, one entry per page in order
	text       string
}

type selectionPart struct {
	page  int
	quads []mupdf.Quad
}

func (s textSelection) empty() bool { return s.text == "" && len(s.parts) == 0 }

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
	documentWorkers
	renderService
	metricsService
	inputState
	interactionState
	uiState
	navigationState
}

type documentWorkers struct {
	renderWorker *renderWorker
	metricLoader *metricLoader
	searchWorker *searchWorker
}

type documentState struct {
	documentAPIMu sync.Mutex
	viewEvents    viewStateEvents
	// instanceServer is nil unless single-instance handling is enabled.
	singleInstance         bool
	instanceServer         *instance.Server
	instanceAddress        string
	pendingInstanceJump    *instanceJump
	pendingInstanceCommand string
	docPath                string
	docName                string
	docPassword            string
	doc                    *mupdf.Document

	pageCount  int
	page       int
	generation int
	pageLinks  map[int][]mupdf.Link
	outline    []mupdf.OutlineItem

	initialDocPath   string
	initialStartPage int
	initialPageSet   bool
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
	trimMargins     bool
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
	waker        *loopWaker
	window       *sdl.Window
	renderer     *sdl.Renderer
	cursorHand   *sdl.Cursor
	cursorArrow  *sdl.Cursor
	cursorIsHand bool
	iconBytes    []byte
	fontFace     font.Face
	textCache    map[textTextureKey]cachedTextTexture
}

// linkInputState tracks the link under the pointer.
type linkInputState struct {
	pressed *mupdf.Link
	// hoverMessage is the target shown while hovering a link, and
	// messageBeforeHover the status message it replaced.
	hoverMessage       string
	messageBeforeHover string
}

type inputState struct {
	mode           mode
	input          textInput
	ignoreText     string
	message        string
	links          linkInputState
	hints          *hintState     // non-nil while link hints are shown
	overview       *overviewState // non-nil while the page overview is shown
	presentation   *presentationState
	histories      map[string]*promptHistory
	unsaved        bool        // the document has edits not yet written
	highlightColor int         // palette index last used for highlights
	promptState    promptState // the question asked in modePrompt
	printSettings  printSettings
	printResult    chan string // delivers the outcome of a print in progress
	printers       printerList
	printersFound  chan printerList // delivers a printer lookup in progress
	preview        *linkPreview
	discardWarned  bool // the user was told that unsaved edits would be lost
	editPos        int  // see markEdited
	savedPos       int
	pointer        sdl.FPoint // last pointer position, for actions on what is under it
	inputScroll    int        // pixels the status-bar prompt is scrolled left
	passwordPrompt pendingPasswordPrompt
	mouseBindings  map[string]string
	searchInput    searchMode
	sequence       []string
	sequenceAt     time.Time
	sequenceLookup map[string]string
	pendingCount   string
	pendingMark    string
	inputSource    smoothInputSource
	smoothZoom     *smoothZoomState
	smoothScroll   *smoothScrollState
}

type pendingPasswordPrompt struct {
	path string
	opts openDocumentOptions
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
	views         uiManager
	outlineMenu   outlineMenuState
	keybindMenu   keybindMenuState
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

type NewOptions struct {
	Verbose           bool
	StartPageExplicit bool
}

func New(docPath string, runtime *config.Runtime, startPage int, iconBytes []byte, opts NewOptions) (*App, error) {
	cfg := runtime.Config()
	if startPage < 0 {
		startPage = 0
	}
	app := &App{
		runtime: runtime,
		verbose: opts.Verbose,
		documentState: documentState{
			page:      startPage,
			pageLinks: map[int][]mupdf.Link{},
		},
		viewStateFields: viewStateFields{
			zoom:     cfg.MinZoom,
			scale:    1,
			pageStep: 64,
		},
		sdlState: sdlState{
			iconBytes: iconBytes,
		},
		renderService: renderService{
			cache:              tileCache{byteLimit: pageCacheByteLimit(cfg)},
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
		app.initialPageSet = opts.StartPageExplicit
	}
	app.recomputeLayout(1400, 900-app.statusBarHeight())
	if app.doc != nil {
		app.ensureRenderBaseScale()
		app.alignPageToAnchor(startPage)
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
	hadDocument := a.doc != nil
	payload := a.documentEventPayload()
	if hadDocument && a.runtime != nil {
		a.emitPluginEvent("document_close_pre", payload)
	}
	a.saveDocumentSession()
	a.document.Close()
	a.closeDocumentResources()
	if hadDocument && a.runtime != nil {
		a.emitPluginEvent("document_closed", payload)
	}
	if a.runtime != nil {
		a.emitPluginEvent("shutdown", map[string]any{})
	}
	a.stopTextInput()
	a.sdlState.Close()
}

func (a *App) emitPluginEvent(event string, payload map[string]any) bool {
	if a.runtime == nil {
		return false
	}
	consumed := a.runtime.EmitPluginEvent(event, payload)
	a.applyRuntimeChanges(event)
	return consumed
}

func (a *App) applyRuntimeChanges(source string) {
	if a.runtime == nil || !a.runtime.ConsumeDirty() {
		return
	}
	a.applyConfig(a.runtime.Config())
	if source == "option_changed" {
		return
	}
	a.runtime.EmitPluginEvent("option_changed", map[string]any{"source": source})
	if a.runtime.ConsumeDirty() {
		a.applyConfig(a.runtime.Config())
	}
}

func (a *App) closeDocumentResources() {
	a.closeDocument()
	a.clearCache()
}

// closeDocument stops the document's workers and closes it, leaving
// rendered tiles cached.
func (a *App) closeDocument() {
	a.closeDocumentWorkers()
	a.documentAPIMu.Lock()
	defer a.documentAPIMu.Unlock()
	if a.doc != nil {
		a.doc.Close()
		a.doc = nil
	}
}

func (a *App) closeDocumentWorkers() {
	if a.renderWorker != nil {
		a.renderWorker.Close()
		a.renderWorker = nil
	}
	if a.metricLoader != nil {
		a.logf("close metric loader")
		a.metricLoader.Close()
		a.metricLoader = nil
	}
	if a.searchWorker != nil {
		a.logf("close search worker")
		a.searchWorker.Close()
		a.searchWorker = nil
	}
	a.pendingLoad = false
	a.pendingPages = 0
	a.pendingStart = 0
}

func (a *App) handleSDLKeyDown(e *sdl.KeyboardEvent) {
	if runtime.GOOS == "darwin" && e.Repeat && e.Key == a.lastKeyUpCode && time.Since(a.lastKeyUpAt) < 100*time.Millisecond {
		if a.ignoreText == "" {
			if token, ok := keyToken(e.Key, e.Mod); ok && utf8.RuneCountInString(token) == 1 {
				a.ignoreText = token
			}
		}
		return
	}
	// A prompt is always opened after any menu still showing, so it takes
	// the keys first.
	if view := a.activeModalUIView(); view != nil && view.onKey != nil && a.mode == modeNormal {
		view.onKey(a, e)
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
		case sdl.KeycodeUp, sdl.KeycodeDown:
			older := 1
			if e.Key == sdl.KeycodeDown {
				older = -1
			}
			completing := a.completion.view != nil && a.completion.view.visible
			if !completing && a.stepPromptHistory(older) {
				return
			}
		}
		if token, ok := keyToken(e.Key, e.Mod); ok && a.handleInputModeBinding(token) {
			return
		}
	}
	if a.mode == modeNormal {
		if token, ok := keyToken(e.Key, e.Mod); ok {
			prevMode := a.mode
			if a.handleHintToken(token) {
				return
			}
			if a.handleMarkToken(token) {
				return
			}
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
		a.editInput(func(input *textInput) { input.InsertText(a.GetClipboard()) })
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
	if a.completion.view != nil && a.completion.view.visible {
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
	if view := a.activeModalUIView(); view != nil {
		if view.searching && view.searchable {
			a.setUIViewQuery(view, view.query+text)
		}
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
	if view := a.activeModalUIView(); view != nil {
		_, wy := normalizedWheelDeltas(e)
		if wy != 0 {
			_, rows := view.contentGeometry(a)
			delta := -1
			if wy < 0 {
				delta = 1
			}
			scrollUIView(view, delta, rows)
			a.pendingRedraw = true
		}
		return
	}
	wx, wy := normalizedWheelDeltas(e)
	if sdl.GetModState()&sdl.KeymodCtrl != 0 {
		if wy > 0 {
			a.runMouseBinding("<c-wheel_up>")
		}
		if wy < 0 {
			a.runMouseBinding("<c-wheel_down>")
		}
		return
	}
	a.runDiscreteMouseWheel(wx, wy)
}

func (a *App) handleSDLMouseButton(e *sdl.MouseButtonEvent) {
	if view := a.activeModalUIView(); view != nil {
		if view.onMouseButton != nil {
			view.onMouseButton(a, e)
		}
		return
	}
	if a.overview != nil {
		a.clickOverview(e)
		return
	}
	if a.presentation != nil {
		a.clickPresentation(e)
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
	if e.Button == uint8(sdl.ButtonLeft) && e.Type == sdl.EventMouseButtonDown && a.clickFormField(float64(e.X), float64(e.Y)) {
		return
	}
	if e.Button == uint8(sdl.ButtonRight) && e.Type == sdl.EventMouseButtonDown && a.openAnnotationMenu(e) {
		return
	}
	if e.Button != uint8(sdl.ButtonLeft) || a.handleLinkButton(e) || !a.config.MouseTextSelect {
		return
	}
	// A press starts a new selection, replacing any previous one; the
	// finished selection stays highlighted until the next press or close.
	if e.Type == sdl.EventMouseButtonDown {
		a.selection = textSelection{}
		if page, point, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y)); ok {
			a.selection = textSelection{active: true, mode: clickSelectMode(e.Clicks), anchorPage: page, anchor: point, focusPage: page, focus: point}
			if a.selection.mode != mupdf.SelectChars {
				a.refreshSelection() // a double or triple click selects without a drag
			}
		}
		a.emitSelectionChanged()
		return
	}
	if e.Type == sdl.EventMouseButtonUp && a.selection.active {
		a.selection.active = false
		a.copySelectionToClipboard()
		a.emitSelectionChanged()
	}
}

func clickSelectMode(clicks uint8) mupdf.SelectMode {
	switch {
	case clicks >= 3:
		return mupdf.SelectLines
	case clicks == 2:
		return mupdf.SelectWords
	default:
		return mupdf.SelectChars
	}
}

// handleLinkButton follows a link once the left button is pressed and
// released over it, so a drag that starts on a link does not fire it.
func (a *App) handleLinkButton(e *sdl.MouseButtonEvent) bool {
	link, over := a.linkAt(float64(e.X), float64(e.Y))
	if e.Type == sdl.EventMouseButtonDown {
		a.links.pressed = nil
		if !over {
			return false
		}
		a.links.pressed = &link
		if a.selection.active || !a.selection.empty() {
			a.clearSelection()
		}
		return true
	}
	pressed := a.links.pressed
	a.links.pressed = nil
	if pressed == nil {
		return false
	}
	if over && link == *pressed {
		a.activateLink(link)
	}
	return true
}

func (a *App) handleSDLMouseMotion(e *sdl.MouseMotionEvent) bool {
	a.pointer = sdl.FPoint{X: e.X, Y: e.Y}
	if view := a.activeModalUIView(); view != nil {
		if view.onMouseMotion != nil {
			return view.onMouseMotion(a, e)
		}
		return a.uiViewHover(view, int(e.X), int(e.Y))
	}
	if a.panning && (a.panButton == 0 || uint32(e.State)&buttonMask(a.panButton) != 0) {
		oldX, oldY := a.scrollX, a.scrollY
		a.scrollBy(-float64(e.Xrel), -float64(e.Yrel))
		return a.scrollX != oldX || a.scrollY != oldY
	}
	a.stopPan()

	link, overLink := a.linkAt(float64(e.X), float64(e.Y))
	hoverChanged := a.setHoveredLink(link, overLink)
	a.hoverLinkPreview(link, overLink, e.X, e.Y)

	if !a.selection.active || uint32(e.State)&uint32(sdl.ButtonLMask) == 0 {
		return hoverChanged
	}
	page, point, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y))
	if ok {
		a.selection.focusPage, a.selection.focus = page, point
		a.refreshSelection()
		return true
	}
	return false
}

// setHoveredLink shows the hovered link's target in the status bar and
// restores the previous message when the pointer leaves it. It reports
// whether the message changed.
func (a *App) setHoveredLink(link mupdf.Link, over bool) bool {
	a.setLinkCursor(over)
	target := ""
	if over {
		target = a.linkTarget(link)
	}
	if target == a.links.hoverMessage {
		return false
	}
	if a.links.hoverMessage == "" || a.message != a.links.hoverMessage {
		a.links.messageBeforeHover = a.message
	}
	a.links.hoverMessage = target
	if target == "" {
		a.message = a.links.messageBeforeHover
	} else {
		a.message = target
	}
	return true
}

func (a *App) linkTarget(link mupdf.Link) string {
	if link.External || link.Page < 0 {
		return link.URI
	}
	return "page " + a.pageLabel(link.Page)
}

func (a *App) setLinkCursor(hand bool) {
	if hand == a.cursorIsHand {
		return
	}
	if hand {
		if a.cursorHand != nil {
			sdl.SetCursor(a.cursorHand)
			a.cursorIsHand = true
		}
		return
	}
	if a.cursorArrow != nil {
		sdl.SetCursor(a.cursorArrow)
		a.cursorIsHand = false
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
	if !ok || !a.isCountableAction(action) {
		return false
	}
	for range count {
		a.runAction(action)
	}
	return true
}

func (a *App) isCountableAction(action string) bool {
	if a.runtime != nil {
		return a.runtime.IsCountableAction(action)
	}
	return actions.IsCountable(action)
}

func (a *App) commitInputMode() {
	if a.completion.view != nil && a.completion.view.visible {
		a.acceptCompletion()
		return
	}
	currentMode := a.mode
	input := strings.TrimSpace(a.input.Value)
	// Passwords and prompt answers are taken as typed, blank included.
	verbatim := currentMode == modePassword || currentMode == modePrompt
	if verbatim {
		input = a.input.Value
	}
	a.mode = modeNormal
	a.input.Reset()
	a.closeCompletion()
	if input == "" && !verbatim {
		return
	}
	a.recordPromptHistory(currentMode, input)
	switch currentMode {
	case modeCommand:
		a.runCommand(input)
	case modeGotoPage:
		a.gotoPageInput(input)
	case modeSearch:
		a.startSearch(input, a.searchInput)
	case modePassword:
		a.submitDocumentPassword(input)
	case modePrompt:
		a.answerPrompt(input)
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
		row := baseRows[clampInt(a.baseRowIndexForPage(a.page, baseRows), 0, len(baseRows)-1)]
		return a.fitScale(viewportW, viewportH, row.width, row.height)
	}
	maxRowWidth, maxRowHeight := 1.0, 1.0
	for _, row := range baseRows {
		maxRowWidth = math.Max(maxRowWidth, row.width)
		maxRowHeight = math.Max(maxRowHeight, row.height)
	}
	return a.fitScale(viewportW, viewportH, maxRowWidth, maxRowHeight)
}

// fitScale is the scale at which content of the given unscaled size fits
// the viewport under the current fit mode.
func (a *App) fitScale(viewportW, viewportH int, width, height float64) float64 {
	widthScale := (float64(viewportW) - float64(a.horizontalGap()*2)) / math.Max(1, width)
	heightScale := (float64(viewportH) - float64(a.verticalGap()*2)) / math.Max(1, height)
	switch a.fitMode {
	case "width":
		return math.Max(0.05, widthScale)
	case "height":
		return math.Max(0.05, heightScale)
	default:
		return math.Max(0.05, math.Min(widthScale, heightScale))
	}
}

func (a *App) nextPage() {
	if a.pageCount == 0 {
		return
	}
	if a.dualPage {
		a.nextSpread()
		return
	}
	page := clampInt(a.page, 0, a.pageCount-1)
	if page < a.pageCount-1 {
		a.alignPageToAnchor(page + 1)
	} else {
		a.alignPageToViewportEdge(page, true)
	}
}

func (a *App) prevPage() {
	if a.pageCount == 0 {
		return
	}
	if a.dualPage {
		a.prevSpread()
		return
	}
	page := clampInt(a.page, 0, a.pageCount-1)
	if page > 0 {
		a.alignPageToAnchor(page - 1)
	} else {
		a.alignPageToViewportEdge(page, false)
	}
}

func (a *App) nextSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.rowIndexForPage(a.anchorPage(a.page))
	if row < len(a.rows)-1 {
		a.alignPageToAnchor(a.rows[row+1].pages[0])
	} else {
		a.alignPageToViewportEdge(a.pageCount-1, true)
	}
}

func (a *App) prevSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.rowIndexForPage(a.anchorPage(a.page))
	if row > 0 {
		a.alignPageToAnchor(a.rows[row-1].pages[0])
	} else {
		a.alignPageToViewportEdge(0, false)
	}
}

func (a *App) scrollBy(dx, dy float64) {
	oldX, oldY := a.scrollX, a.scrollY
	a.scrollX += dx
	a.scrollY += dy
	a.clampScroll()
	if a.renderMode == "single" || (a.scrollX == oldX && a.scrollY == oldY) {
		return
	}
	a.updateCurrentPageFromScroll()
}

func (a *App) scrollByInput(dx, dy float64) {
	if !a.smoothScrollInputEnabled(a.currentSmoothInputSource()) {
		a.cancelSmoothScroll()
		a.scrollBy(dx, dy)
		return
	}
	a.queueSmoothScroll(dx, dy)
}

func (a *App) alignPageToAnchor(page int) {
	if page < 0 || page >= len(a.pageToRow) {
		return
	}
	page = a.anchorPage(page)
	if a.positionMatchesPageAnchor(page) {
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
	a.scrollY = a.scrollYForAnchoredRow(row)
	a.page = page
	a.clampScroll()
}

func (a *App) alignPageToViewportEdge(page int, bottom bool) {
	if page < 0 || page >= len(a.pageToRow) || len(a.rows) == 0 {
		return
	}
	row := a.rows[a.pageToRow[page]]
	for i, rowPage := range row.pages {
		if rowPage != page {
			continue
		}
		_, y := a.rowPageScreenOrigin(row, i)
		targetY := 0.0
		if bottom {
			_, viewportH := a.viewportSize()
			targetY = float64(viewportH) - row.pageH[i]
		}
		a.scrollY += y - targetY
		a.clampScroll()
		a.page = a.anchorPage(page)
		return
	}
}

// jumpToDestination navigates to a link or outline target. Missing
// coordinates default to the horizontal centre and top edge of the page.
func (a *App) jumpToDestination(page int, x, y float64, hasX, hasY bool) {
	if page < 0 || page >= len(a.pageMetrics) {
		return
	}
	if !hasX && !hasY {
		a.alignPageToAnchor(page)
		return
	}
	bounds := a.pageMetrics[page].bounds
	if !hasX {
		x = float64(bounds.X0+bounds.X1) / 2
	}
	if !hasY {
		y = float64(bounds.Y0)
	}
	a.alignPageToDocumentPoint(page, x, y)
}

func (a *App) alignPageToDocumentPoint(page int, x, y float64) {
	if page < 0 || page >= len(a.pageToRow) {
		return
	}
	page = a.anchorPage(page)
	a.recordJump()
	if a.renderMode == "single" {
		a.page = page
		a.recomputeLayout(a.viewportSize())
	}
	row := a.rows[a.pageToRow[page]]
	pageIndex := 0
	for i, rowPage := range row.pages {
		if rowPage == page {
			pageIndex = i
			break
		}
	}
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[page].bounds, a.scale, a.rotation)
	tx, ty := transformPoint(x, y, a.scale, a.rotation)
	viewportW, viewportH := a.viewportSize()
	a.scrollX = row.pageX[pageIndex] + tx - originX - float64(viewportW)/2
	a.scrollY = row.pageY[pageIndex] + ty - originY - float64(viewportH)/4
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

func (a *App) positionMatchesPageAnchor(page int) bool {
	if a.renderMode == "single" {
		return a.page == page && a.scrollX == 0 && a.scrollY == 0
	}
	row := a.rows[a.pageToRow[page]]
	expectedY := a.clampedScrollY(a.scrollYForAnchoredRow(row))
	return a.page == page && a.scrollY == expectedY
}

func (a *App) scrollYForAnchoredRow(row rowLayout) float64 {
	_, viewportH := a.viewportSize()
	switch a.config.AnchorPosition {
	case "top":
		return row.y
	case "bottom":
		return row.y + row.height - float64(viewportH)
	default:
		return row.y + row.height/2 - float64(viewportH)/2
	}
}

func (a *App) clampedScrollY(scrollY float64) float64 {
	_, viewportH := a.viewportSize()
	maxY := math.Max(0, a.contentH-float64(viewportH))
	return clampFloat(scrollY, 0, maxY)
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

func (a *App) clampZoom(zoom float64) float64 {
	minZoom := a.config.MinZoom
	if minZoom <= 0 {
		minZoom = config.Default().MinZoom
	}
	maxZoom := a.config.MaxZoom
	if maxZoom < minZoom {
		maxZoom = minZoom
	}
	return clampFloat(zoom, minZoom, maxZoom)
}

func (a *App) setFitMode(mode string) {
	a.cancelSmoothZoom()
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = mode
		a.scheduleRenderScaleTarget(a.zoom)
	})
}

func (a *App) setAltColors(enabled bool) {
	if a.altColors == enabled {
		return
	}
	a.altColors = enabled
	a.clearCache()
}
