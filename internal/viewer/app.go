package viewer

import (
	"fmt"
	"image"
	"image/color"
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"golang.design/x/clipboard"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
)

type mode int

const (
	modeNormal mode = iota
	modeCommand
	modeGotoPage
	modeSearch
)

type renderedPage struct {
	image  *ebiten.Image
	width  float64
	height float64
	pixX   float64
	pixY   float64
	key    string
	page   int
	scale  float64
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
	docPath string
	docName string
	doc     *mupdf.Document
	config  config.Config

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
	message       string
	mouseBindings map[string]string
	searchInput   searchMode

	sequence       []string
	sequenceAt     time.Time
	sequenceLookup map[string]string
	pendingCount   string

	lastErr        error
	quit           bool
	selection      textSelection
	clipboardReady bool
	search         searchState
	searchWorker   *searchWorker
}

func New(docPath string, cfg config.Config, startPage int) (*App, error) {
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
		docPath:         docPath,
		docName:         filepath.Base(docPath),
		doc:             doc,
		config:          cfg,
		pageCount:       pages,
		page:            startPage,
		zoom:            1,
		fitMode:         sanitizeFitMode(cfg.FitMode),
		renderMode:      sanitizeRenderMode(cfg.RenderMode),
		scale:           1,
		altColors:       cfg.AltColors,
		dualPage:        cfg.DualPage,
		firstPageOffset: cfg.FirstPageOffset,
		statusBarShown:  cfg.StatusBarVisible,
		pageStep:        64,
		renderCache:        map[string]*renderedPage{},
		cacheLimit:         minInt(24, pages),
		minRenderBaseScale: 0.25,
		fontFace:        basicfont.Face7x13,
		message:         cfg.NormalMessage,
		mouseBindings:   map[string]string{},
		sequenceLookup:  map[string]string{},
	}
	for k, v := range cfg.KeyBindings {
		app.sequenceLookup[normalizeBinding(k)] = v
	}
	for k, v := range cfg.MouseBindings {
		app.mouseBindings[k] = v
	}
	if err := clipboard.Init(); err == nil {
		app.clipboardReady = true
	}
	app.initRenderWorker()
	app.initSearch()
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
	if a.doc != nil {
		a.doc.Close()
	}
}

func (a *App) Run() error {
	ebiten.SetScreenClearedEveryFrame(false)
	ebiten.SetWindowSize(1400, 900)
	ebiten.SetWindowTitle(a.docName + " - gopdf")
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)
	if err := ebiten.RunGame(a); err != nil {
		if a.quit {
			return nil
		}
		return err
	}
	return nil
}

func (a *App) Update() error {
	if a.quit {
		return ebiten.Termination
	}
	a.pollRenderUpdates()
	a.pollSearchUpdates()
	a.handleMouse()
	a.expireSequence()
	switch a.mode {
	case modeNormal:
		a.handleNormalMode()
	default:
		a.handleInputMode()
	}
	a.prefetchVisiblePages()
	a.adjustRenderBaseScaleForExtremeZoom(a.scale)
	return nil
}

func (a *App) Draw(screen *ebiten.Image) {
	bg := a.backgroundColor()
	screen.Fill(bg)
	a.winW, a.winH = screen.Bounds().Dx(), screen.Bounds().Dy()
	a.drawPages(screen)
	if a.pendingRedraw {
		a.pendingRedraw = false
	}
	if a.statusVisible() {
		a.drawStatusBar(screen)
	}
}

func (a *App) Layout(outsideWidth, outsideHeight int) (int, int) {
	a.winW, a.winH = outsideWidth, outsideHeight
	a.recomputeLayout(a.viewportSize())
	return outsideWidth, outsideHeight
}

func (a *App) handleNormalMode() {
	for _, token := range collectTokens() {
		if a.handleCountToken(token) {
			continue
		}
		a.pushToken(token)
	}
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

func (a *App) handleInputMode() {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		a.mode = modeNormal
		a.input = ""
		a.inputCursor = 0
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEnter) || inpututil.IsKeyJustPressed(ebiten.KeyNumpadEnter) {
		a.commitInputMode()
		return
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowLeft) {
		a.moveInputCursor(-1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyArrowRight) {
		a.moveInputCursor(1)
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyBackspace) {
		a.backspaceInput()
	}
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 0x20 && r != 0x7f {
			a.insertInputRune(r)
		}
	}
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

func (a *App) drawPages(screen *ebiten.Image) {
	if a.renderMode == "single" {
		a.drawSinglePage(screen)
		return
	}
	a.drawContinuousPages(screen)
}

func (a *App) drawContinuousPages(screen *ebiten.Image) {
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
			drawW := rp.width * drawScale
			drawH := rp.height * drawScale
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			drawX, drawY := pageImageOrigin(x, y, rp, drawScale)
			if drawX+drawW < 0 || drawX > float64(viewportW) || drawY+drawH < 0 || drawY > float64(viewportH) {
				continue
			}
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Scale(drawScale, drawScale)
			op.GeoM.Translate(drawX, drawY)
			screen.DrawImage(rp.image, op)
			a.drawSearchHighlightsForPage(screen, page, x, y, rp)
		}
	}
	a.drawSelection(screen)
}

func (a *App) drawSinglePage(screen *ebiten.Image) {
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
		drawW := rp.width * drawScale
		drawH := rp.height * drawScale
		x := baseX + (row.pageX[i] - row.x) - a.scrollX
		y := baseY + (row.pageY[i] - row.y) - a.scrollY
		drawX, drawY := pageImageOrigin(x, y, rp, drawScale)
		if drawX+drawW < 0 || drawX > float64(viewportW) || drawY+drawH < 0 || drawY > float64(viewportH) {
			continue
		}
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Scale(drawScale, drawScale)
		op.GeoM.Translate(drawX, drawY)
		screen.DrawImage(rp.image, op)
		a.drawSearchHighlightsForPage(screen, page, x, y, rp)
	}
	a.drawSelection(screen)
}

func (a *App) cachedRenderPage(page int, scale float64) (*renderedPage, bool) {
	renderScale := a.renderScaleFor(scale)
	key := renderCacheKey(page, renderScale, a.rotation, a.altColors)
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
		rp.image.Dispose()
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
		if a.renderMode == "single" {
			a.renderMode = "continuous"
		} else {
			a.renderMode = "single"
		}
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
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
		ebiten.SetFullscreen(a.fullscreen)
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
		a.rotation = math.Mod(a.rotation+90, 360)
		if err := a.loadPageMetrics(); err != nil {
			a.message = err.Error()
			return
		}
		a.clearCache()
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
	case "rotate_ccw":
		a.rotation = math.Mod(a.rotation+270, 360)
		if err := a.loadPageMetrics(); err != nil {
			a.message = err.Error()
			return
		}
		a.clearCache()
		a.recomputeLayout(a.viewportSize())
		a.alignPageTop(a.page)
	case "quit":
		a.quit = true
	case "escape":
		a.mode = modeNormal
		a.input = ""
		a.inputCursor = 0
	}
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
	case "help":
		a.message = commandHelpMessage()
	default:
		a.message = "unknown command: " + fields[0]
	}
}

func (a *App) reloadConfig() {
	cfg, err := config.Load(a.config.ConfigPath)
	if err != nil {
		a.message = err.Error()
		return
	}
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

func (a *App) drawStatusBar(screen *ebiten.Image) {
	h := a.config.StatusBarHeight
	y := a.winH - h
	bar := ebiten.NewImage(a.winW, h)
	bar.Fill(a.statusBarColor())
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(0, float64(y))
	screen.DrawImage(bar, op)
	left := a.statusLeft()
	right := a.statusRight()
	a.drawText(screen, left, 8, y+19, a.foregroundColor())
	a.drawInputCursor(screen, y)
	rw := text.BoundString(a.fontFace, right).Dx()
	a.drawText(screen, right, a.winW-rw-8, y+19, a.foregroundColor())
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

func (a *App) drawInputCursor(screen *ebiten.Image, barY int) {
	if a.mode == modeNormal {
		return
	}
	prefix := a.inputPrefix()
	left, _ := splitAtRune(a.input, a.inputCursor)
	x := 8 + text.BoundString(a.fontFace, prefix+left).Dx()
	vector.StrokeLine(screen, float32(x), float32(barY+6), float32(x), float32(barY+22), 1, a.foregroundColor(), false)
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

func (a *App) drawText(screen *ebiten.Image, s string, x, y int, clr color.Color) {
	text.Draw(screen, s, a.fontFace, x, y, clr)
}

func (a *App) handleMouse() {
	a.handleWheel()
	if !a.config.MouseTextSelect {
		a.selection.active = false
		return
	}
	mx, my := ebiten.CursorPosition()
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		page, point, ok := a.pagePointAtScreen(float64(mx), float64(my))
		if ok {
			a.selection = textSelection{active: true, page: page, anchor: point, focus: point}
		}
	}
	if a.selection.active && ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		page, point, ok := a.pagePointAtScreen(float64(mx), float64(my))
		if ok && page == a.selection.page {
			a.selection.focus = point
			a.refreshSelection()
		}
	}
	if a.selection.active && inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		a.copySelectionToClipboard()
		a.selection.active = false
		a.selection.quads = nil
	}
}

func (a *App) handleWheel() {
	wx, wy := ebiten.Wheel()
	if wx == 0 && wy == 0 {
		return
	}
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	if ctrl {
		if wy > 0 {
			a.runMouseBinding("<c-wheel_up>")
		}
		if wy < 0 {
			a.runMouseBinding("<c-wheel_down>")
		}
		return
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

func (a *App) runMouseBinding(event string) {
	if action, ok := a.mouseBindings[event]; ok {
		a.runAction(action)
	}
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
	drawScale := a.renderDrawScale(rp, a.scale)
	a.scrollX += (x + tx - rp.pixX*drawScale) - anchor.centerX
	a.scrollY += (y + ty - rp.pixY*drawScale) - anchor.centerY
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
		transformedX := sx - hit.x + hit.render.pixX*hit.drawScale
		transformedY := sy - hit.y + hit.render.pixY*hit.drawScale
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
			drawX, drawY := pageImageOrigin(x, y, rp, drawScale)
			hits = append(hits, pageHit{page: page, x: drawX, y: drawY, width: rp.width * drawScale, height: rp.height * drawScale, drawScale: drawScale, render: rp})
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
			drawX, drawY := pageImageOrigin(x, y, rp, drawScale)
			hits = append(hits, pageHit{page: page, x: drawX, y: drawY, width: rp.width * drawScale, height: rp.height * drawScale, drawScale: drawScale, render: rp})
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
	if !a.clipboardReady {
		a.message = "clipboard unavailable"
		return
	}
	clipboard.Write(clipboard.FmtText, []byte(a.selection.text))
	a.message = fmt.Sprintf("copied %d chars", len(a.selection.text))
	a.selection.text = ""
}

func (a *App) drawSelection(screen *ebiten.Image) {
	if len(a.selection.quads) == 0 {
		return
	}
	x, y, rp, ok := a.pageScreenTransform(a.selection.page)
	if !ok {
		return
	}
	a.drawHighlightQuads(screen, a.selection.quads, x, y, rp)
}

func (a *App) drawHighlightQuads(screen *ebiten.Image, quads []mupdf.Quad, x, y float64, rp *renderedPage) {
	a.drawHighlightQuadsWithStyle(screen, quads, x, y, rp, false)
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
	drawScale := a.renderDrawScale(rp, a.scale)
	for _, pt := range pts {
		tx, ty := transformPoint(pt.X, pt.Y, a.scale, a.rotation)
		sx := x + tx - rp.pixX*drawScale
		sy := y + ty - rp.pixY*drawScale
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
		w, h := bounds.Width(), bounds.Height()
		rot := math.Mod(math.Abs(a.rotation), 180)
		if math.Abs(rot-90) < 0.1 {
			w, h = h, w
		}
		a.pageMetrics[i] = pageMetrics{bounds: bounds, width: w, height: h}
	}
	return nil
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

func pageImageOrigin(pageX, pageY float64, rp *renderedPage, drawScale float64) (float64, float64) {
	if rp == nil {
		return pageX, pageY
	}
	return pageX - rp.pixX*drawScale, pageY - rp.pixY*drawScale
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
	return "<" + strings.Join(parts, "-") + ">"
}

func keyToken(key ebiten.Key, ctrl, shift bool) (string, bool) {
	if isModifierKey(key) {
		return "", false
	}
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
	return "", false
}

func isModifierKey(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight, ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
		return true
	default:
		return false
	}
}

func specialKeyToken(key ebiten.Key) (string, bool) {
	switch key {
	case ebiten.KeyEnter:
		return "<Enter>", true
	case ebiten.KeyEscape:
		return "<Esc>", true
	case ebiten.KeyBackspace:
		return "<BS>", true
	case ebiten.KeyPageDown:
		return "<PgDn>", true
	case ebiten.KeyPageUp:
		return "<PgUp>", true
	case ebiten.KeySpace:
		return "<Space>", true
	default:
		return "", false
	}
}

func baseKeyName(key ebiten.Key) (string, bool) {
	if key >= ebiten.KeyA && key <= ebiten.KeyZ {
		return strings.ToLower(key.String()), true
	}
	if key >= ebiten.Key0 && key <= ebiten.Key9 {
		return key.String(), true
	}
	switch key {
	case ebiten.KeySpace:
		return "space", true
	case ebiten.KeyEnter:
		return "enter", true
	case ebiten.KeyEscape:
		return "esc", true
	case ebiten.KeyBackspace:
		return "bs", true
	case ebiten.KeyPageDown:
		return "pgdn", true
	case ebiten.KeyPageUp:
		return "pgup", true
	default:
		return "", false
	}
}

func collectTokens() []string {
	var out []string
	ctrl := ebiten.IsKeyPressed(ebiten.KeyControl) || ebiten.IsKeyPressed(ebiten.KeyControlLeft) || ebiten.IsKeyPressed(ebiten.KeyControlRight)
	shift := ebiten.IsKeyPressed(ebiten.KeyShift) || ebiten.IsKeyPressed(ebiten.KeyShiftLeft) || ebiten.IsKeyPressed(ebiten.KeyShiftRight)
	for _, r := range ebiten.AppendInputChars(nil) {
		if r >= 0x20 && r != 0x7f {
			if ctrl {
				continue
			}
			out = append(out, string(r))
		}
	}
	for _, key := range inpututil.AppendJustPressedKeys(nil) {
		if token, ok := keyToken(key, ctrl, shift); ok {
			out = append(out, token)
		}
	}
	return out
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

func transformPoint(x, y, scale, rotation float64) (float64, float64) {
	x *= scale
	y *= scale
	switch int(math.Mod(rotation+360, 360)) {
	case 90:
		return -y, x
	case 180:
		return -x, -y
	case 270:
		return y, -x
	default:
		return x, y
	}
}

func inverseTransformPoint(x, y, scale, rotation float64) (float64, float64) {
	if scale == 0 {
		return x, y
	}
	switch int(math.Mod(rotation+360, 360)) {
	case 90:
		return y / scale, -x / scale
	case 180:
		return -x / scale, -y / scale
	case 270:
		return -y / scale, x / scale
	default:
		return x / scale, y / scale
	}
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
