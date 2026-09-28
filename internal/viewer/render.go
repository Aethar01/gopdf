package viewer

import (
	"container/list"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type renderRequest struct {
	generation int
	page       int
	scale      float64
	altColors  bool
	aaLevel    int
	cacheKey   string
	priority   int
}

type renderUpdate struct {
	request  renderRequest
	rendered *mupdf.RenderedPage
	err      error
}

type renderService struct {
	renderCache        map[string]*renderedPage
	renderLRU          *list.List
	renderLRUItems     map[string]*list.Element
	renderIndex        map[renderVariantKey]*renderedPage
	thumbnailCache     map[renderVariantKey]*renderedPage
	thumbnailLRU       *list.List
	thumbnailLRUItems  map[renderVariantKey]*list.Element
	cacheLimit         int
	cacheByteLimit     int64
	renderCacheBytes   int64
	thumbnailBytes     int64
	visibleCachePages  map[int]bool
	renderBaseScale    float64
	renderScaleTarget  float64
	renderScaleReadyAt time.Time
	minRenderBaseScale float64
	renderGeneration   int
	renderPending      map[string]renderRequest
}

type renderVariantKey struct {
	page      int
	altColors bool
	aaLevel   int
}

// renderWorker rasterises pages on a pool of goroutines, each with its own
// MuPDF renderer, taking the most urgent wanted request from a shared queue.
type renderWorker struct {
	workerLifecycle
	slots      []*renderSlot
	requests   chan renderRequest
	updates    chan renderUpdate
	generation atomic.Int32
	wanted     atomic.Value
	visible    atomic.Value

	mu    sync.Mutex
	queue []renderRequest
}

type renderSlot struct {
	renderer   *mupdf.Renderer
	activePage atomic.Int32 // page+1 while rendering, 0 when idle
}

func (s *renderSlot) active() (int, bool) {
	page := int(s.activePage.Load()) - 1
	return page, page >= 0
}

func (s *renderSlot) cancel() {
	if s.renderer != nil {
		s.renderer.Cancel()
	}
}

func newRenderWorker(doc *mupdf.Document, threads int) *renderWorker {
	w := &renderWorker{
		workerLifecycle: newWorkerLifecycle(),
		requests:        make(chan renderRequest, 128),
		updates:         make(chan renderUpdate, maxPendingPrefetchRenders),
	}
	if err := w.startSlots(doc, threads); err != nil {
		go func() {
			sendWorkerUpdate(&w.workerLifecycle, w.updates, renderUpdate{err: err})
			w.closeOnce.Do(func() { close(w.closing) })
			close(w.done)
		}()
		return w
	}
	var wg sync.WaitGroup
	for _, slot := range w.slots {
		wg.Add(1)
		go func() {
			defer wg.Done()
			w.run(slot)
		}()
	}
	go func() {
		wg.Wait()
		close(w.done)
	}()
	return w
}

func (w *renderWorker) startSlots(doc *mupdf.Document, threads int) error {
	if doc == nil {
		return fmt.Errorf("render worker: no document open")
	}
	for range max(1, threads) {
		renderer, err := doc.NewRenderer()
		if err != nil {
			for _, slot := range w.slots {
				slot.renderer.Close()
			}
			w.slots = nil
			return err
		}
		w.slots = append(w.slots, &renderSlot{renderer: renderer})
	}
	return nil
}

func (w *renderWorker) Close() {
	w.Cancel()
	w.workerLifecycle.Close()
}

func (w *renderWorker) Cancel() {
	if w == nil {
		return
	}
	for _, slot := range w.slots {
		slot.cancel()
	}
}

func (w *renderWorker) SetGeneration(generation int) {
	w.generation.Store(int32(generation))
	w.Cancel()
}

func (w *renderWorker) SetWantedPages(pages map[int]bool) {
	keep := make(map[int]bool, len(pages))
	for page, ok := range pages {
		if ok {
			keep[page] = true
		}
	}
	w.wanted.Store(keep)
	for _, slot := range w.slots {
		if page, ok := slot.active(); ok && !keep[page] {
			slot.cancel()
		}
	}
}

func (w *renderWorker) SetVisiblePages(pages map[int]bool) {
	visible := make(map[int]bool, len(pages))
	for page, ok := range pages {
		if ok {
			visible[page] = true
		}
	}
	w.visible.Store(visible)
}

// CancelNotVisible cancels renders of pages outside visible and returns them.
func (w *renderWorker) CancelNotVisible(visible map[int]bool) []int {
	var cancelled []int
	for _, slot := range w.slots {
		if page, ok := slot.active(); ok && !visible[page] {
			slot.cancel()
			cancelled = append(cancelled, page)
		}
	}
	return cancelled
}

func (w *renderWorker) Enqueue(req renderRequest) bool {
	select {
	case <-w.closing:
		return false
	case w.requests <- req:
		return true
	default:
		return false
	}
}

// DrainUnwanted drops queued requests that are stale or no longer wanted.
func (w *renderWorker) DrainUnwanted(gen int) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.drainRequestsLocked()
	queue := w.queue[:0]
	for _, req := range w.queue {
		if w.requestWanted(req, gen) {
			queue = append(queue, req)
		}
	}
	w.queue = queue
}

func (w *renderWorker) drainRequestsLocked() {
	for {
		select {
		case req := <-w.requests:
			w.queue = append(w.queue, req)
		default:
			return
		}
	}
}

func (w *renderWorker) requestWanted(req renderRequest, gen int) bool {
	if req.generation != gen {
		return false
	}
	value := w.wanted.Load()
	if value == nil {
		return true
	}
	wanted, ok := value.(map[int]bool)
	return !ok || wanted[req.page]
}

func (w *renderWorker) requestPriority(req renderRequest) int {
	value := w.visible.Load()
	if value == nil {
		return req.priority
	}
	visible, ok := value.(map[int]bool)
	if ok && visible[req.page] {
		return 0
	}
	if ok && req.priority <= 0 {
		return renderPrefetchPriority
	}
	return req.priority
}

func (w *renderWorker) run(slot *renderSlot) {
	defer slot.renderer.Close()
	for {
		req, ok := w.next()
		if !ok {
			return
		}
		slot.activePage.Store(int32(req.page + 1))
		rendered, err := slot.renderer.Render(req.page, req.scale, 0, req.aaLevel)
		slot.activePage.Store(0)
		sendWorkerUpdate(&w.workerLifecycle, w.updates, renderUpdate{request: req, rendered: rendered, err: err})
	}
}

// next blocks until a wanted request is queued, or reports false once the
// worker is closing.
func (w *renderWorker) next() (renderRequest, bool) {
	for {
		w.mu.Lock()
		w.drainRequestsLocked()
		req, queue, ok := w.popNextRequest(w.queue)
		w.queue = queue
		w.mu.Unlock()
		if ok {
			return req, true
		}
		select {
		case <-w.closing:
			return renderRequest{}, false
		case req := <-w.requests:
			w.mu.Lock()
			w.queue = append(w.queue, req)
			w.mu.Unlock()
		}
	}
}

func (w *renderWorker) popNextRequest(queue []renderRequest) (renderRequest, []renderRequest, bool) {
	gen := int(w.generation.Load())
	best := -1
	for i, req := range queue {
		if !w.requestWanted(req, gen) {
			continue
		}
		if best < 0 || w.requestPriority(req) < w.requestPriority(queue[best]) {
			best = i
		}
	}
	if best < 0 {
		return renderRequest{}, queue[:0], false
	}
	req := queue[best]
	copy(queue[best:], queue[best+1:])
	queue = queue[:len(queue)-1]
	return req, queue, true
}

func renderCacheKey(page int, scale float64, altColors bool, aaLevel int) string {
	return fmt.Sprintf("%d/%.4f/%t/%d", page, scale, altColors, aaLevel)
}

func (a *App) initRenderWorker() {
	a.logf("start render worker path=%q", a.docPath)
	a.renderPending = map[string]renderRequest{}
	a.renderWorker = newRenderWorker(a.doc, renderThreadCount(a.config))
	a.renderWorker.SetGeneration(a.renderGeneration)
}

func (a *App) pollRenderUpdates() {
	if a.renderWorker == nil {
		return
	}
	for {
		select {
		case update := <-a.renderWorker.updates:
			req := update.request
			if update.rendered != nil {
				defer update.rendered.Close()
			}
			if req.generation != a.renderGeneration {
				delete(a.renderPending, req.cacheKey)
				continue
			}
			if _, pending := a.renderPending[req.cacheKey]; !pending {
				continue
			}
			delete(a.renderPending, req.cacheKey)
			if update.err != nil {
				a.logf("render update failed err=%v", update.err)
				a.message = update.err.Error()
				continue
			}
			if update.rendered == nil {
				continue
			}
			if req.altColors {
				remapPageColors(update.rendered.Image, a.config.AltBackground, a.config.AltForeground)
			}
			a.removeRenderCacheEntry(req.cacheKey, true)
			tex, err := textureFromRGBA(a.renderer, update.rendered.Image)
			if err != nil {
				a.logf("render texture failed page=%d err=%v", req.page+1, err)
				a.message = err.Error()
				continue
			}
			bounds := update.rendered.Image.Bounds()
			rp := &renderedPage{
				texture:   tex,
				width:     float64(bounds.Dx()),
				height:    float64(bounds.Dy()),
				bytes:     estimatedTextureBytes(bounds.Dx(), bounds.Dy()),
				pixX:      float64(update.rendered.X),
				pixY:      float64(update.rendered.Y),
				key:       req.cacheKey,
				page:      req.page,
				scale:     req.scale,
				altColors: req.altColors,
				aaLevel:   req.aaLevel,
			}
			a.addRenderCacheEntry(req.cacheKey, rp)
			a.addThumbnailCacheEntry(rp)
			a.startPendingMetricLoader()
			a.pendingRedraw = true
			a.enforceRenderCacheLimit()
			a.enforceThumbnailCacheLimit()
		default:
			return
		}
	}
}

func (rs *renderService) touchRenderCacheEntry(key string) {
	rs.ensureRenderCacheState()
	if elem := rs.renderLRUItems[key]; elem != nil {
		rs.renderLRU.MoveToBack(elem)
	}
}

func (rs *renderService) ensureRenderCacheState() {
	if rs.renderCache == nil {
		rs.renderCache = map[string]*renderedPage{}
	}
	if rs.renderLRU == nil {
		rs.renderLRU = list.New()
	}
	if rs.renderLRUItems == nil {
		rs.renderLRUItems = map[string]*list.Element{}
	}
	if rs.renderIndex == nil {
		rs.renderIndex = map[renderVariantKey]*renderedPage{}
		for key, rp := range rs.renderCache {
			if rp == nil {
				continue
			}
			if rs.renderLRUItems[key] == nil {
				rs.renderLRUItems[key] = rs.renderLRU.PushBack(key)
			}
			rs.indexRenderPage(key, rp)
		}
	}
	if rs.thumbnailCache == nil {
		rs.thumbnailCache = map[renderVariantKey]*renderedPage{}
	}
	if rs.thumbnailLRU == nil {
		rs.thumbnailLRU = list.New()
	}
	if rs.thumbnailLRUItems == nil {
		rs.thumbnailLRUItems = map[renderVariantKey]*list.Element{}
	}
}

func (rs *renderService) indexRenderPage(key string, rp *renderedPage) {
	rp.key = key
	variant := renderVariantKey{page: rp.page, altColors: rp.altColors, aaLevel: rp.aaLevel}
	rs.renderIndex[variant] = rp
}

func (rs *renderService) touchThumbnailCacheEntry(key renderVariantKey) {
	rs.ensureRenderCacheState()
	if elem := rs.thumbnailLRUItems[key]; elem != nil {
		rs.thumbnailLRU.MoveToBack(elem)
	}
}

func (a *App) addThumbnailCacheEntry(source *renderedPage) {
	if source == nil || source.texture == nil || a.renderer == nil {
		return
	}
	a.ensureRenderCacheState()
	tw, th, ratio := thumbnailDimensions(int(source.width), int(source.height))
	if tw <= 0 || th <= 0 || ratio <= 0 {
		return
	}
	key := renderVariantKey{page: source.page, altColors: source.altColors, aaLevel: source.aaLevel}
	if a.thumbnailCache[key] != nil {
		a.touchThumbnailCacheEntry(key)
		return
	}
	tex := sdl.CreateTexture(a.renderer, sdl.PixelFormatRGBA32, sdl.TextureAccessTarget, int32(tw), int32(th))
	if tex == nil {
		return
	}
	if !sdl.SetTextureScaleMode(tex, sdl.ScaleModeLinear) {
		sdl.DestroyTexture(tex)
		return
	}
	oldTarget := sdl.GetRenderTarget(a.renderer)
	if !sdl.SetRenderTarget(a.renderer, tex) {
		sdl.DestroyTexture(tex)
		return
	}
	dst := sdl.FRect{W: float32(tw), H: float32(th)}
	ok := sdl.RenderTexture(a.renderer, source.texture, nil, &dst)
	if !sdl.SetRenderTarget(a.renderer, oldTarget) || !ok {
		sdl.DestroyTexture(tex)
		return
	}
	rp := &renderedPage{
		texture:   tex,
		width:     float64(tw),
		height:    float64(th),
		bytes:     estimatedTextureBytes(tw, th),
		pixX:      source.pixX * ratio,
		pixY:      source.pixY * ratio,
		page:      source.page,
		scale:     source.scale * ratio,
		altColors: source.altColors,
		aaLevel:   source.aaLevel,
	}
	a.thumbnailCache[key] = rp
	a.thumbnailBytes += rp.bytes
	a.thumbnailLRUItems[key] = a.thumbnailLRU.PushBack(key)
}

func (rs *renderService) removeThumbnailCacheEntry(key renderVariantKey, destroy bool) {
	rs.ensureRenderCacheState()
	rp := rs.thumbnailCache[key]
	if rp == nil {
		return
	}
	if elem := rs.thumbnailLRUItems[key]; elem != nil {
		rs.thumbnailLRU.Remove(elem)
		delete(rs.thumbnailLRUItems, key)
	}
	if destroy && rp.texture != nil {
		sdl.DestroyTexture(rp.texture)
	}
	rs.thumbnailBytes -= rp.bytes
	if rs.thumbnailBytes < 0 {
		rs.thumbnailBytes = 0
	}
	delete(rs.thumbnailCache, key)
}

func (rs *renderService) enforceThumbnailCacheLimit() {
	rs.ensureRenderCacheState()
	limit := rs.thumbnailCacheLimit()
	for limit > 0 && len(rs.thumbnailCache) > limit {
		front := rs.thumbnailLRU.Front()
		if front == nil {
			return
		}
		key, _ := front.Value.(renderVariantKey)
		rs.removeThumbnailCacheEntry(key, true)
	}
}

func (rs *renderService) thumbnailCacheLimit() int {
	if rs.cacheLimit <= 0 {
		return 0
	}
	return rs.cacheLimit * 2
}

func (rs *renderService) addRenderCacheEntry(key string, rp *renderedPage) {
	rs.ensureRenderCacheState()
	rs.removeRenderCacheVariants(renderVariantKey{page: rp.page, altColors: rp.altColors, aaLevel: rp.aaLevel})
	if rp.bytes <= 0 {
		rp.bytes = estimatedTextureBytes(int(rp.width), int(rp.height))
	}
	rs.renderCache[key] = rp
	rs.renderCacheBytes += rp.bytes
	rs.renderLRUItems[key] = rs.renderLRU.PushBack(key)
	rs.indexRenderPage(key, rp)
}

func (rs *renderService) removeRenderCacheVariants(variant renderVariantKey) {
	rs.ensureRenderCacheState()
	if rp := rs.renderIndex[variant]; rp != nil {
		rs.removeRenderCacheEntry(rp.key, true)
	}
}

func (rs *renderService) removeRenderCacheEntry(key string, destroy bool) {
	rs.ensureRenderCacheState()
	rp := rs.renderCache[key]
	if rp == nil {
		return
	}
	if elem := rs.renderLRUItems[key]; elem != nil {
		rs.renderLRU.Remove(elem)
		delete(rs.renderLRUItems, key)
	}
	variant := renderVariantKey{page: rp.page, altColors: rp.altColors, aaLevel: rp.aaLevel}
	if rs.renderIndex[variant] == rp {
		delete(rs.renderIndex, variant)
	}
	if destroy && rp.texture != nil {
		sdl.DestroyTexture(rp.texture)
	}
	rs.renderCacheBytes -= rp.bytes
	if rs.renderCacheBytes < 0 {
		rs.renderCacheBytes = 0
	}
	delete(rs.renderCache, key)
}

func (rs *renderService) enforceRenderCacheLimit() {
	rs.ensureRenderCacheState()
	for rs.renderCacheOverLimit() {
		attempts := len(rs.renderCache)
		evicted := false
		for attempts > 0 && rs.renderCacheOverLimit() {
			attempts--
			front := rs.renderLRU.Front()
			if front == nil {
				return
			}
			key, _ := front.Value.(string)
			rp := rs.renderCache[key]
			if rp != nil && rs.visibleCachePages[rp.page] {
				rs.renderLRU.MoveToBack(front)
				continue
			}
			if _, pending := rs.renderPending[key]; pending {
				rs.renderLRU.MoveToBack(front)
				continue
			}
			rs.removeRenderCacheEntry(key, true)
			evicted = true
		}
		if !evicted {
			return
		}
	}
}

func (rs *renderService) renderCacheOverLimit() bool {
	if rs.cacheLimit > 0 && len(rs.renderCache) > rs.cacheLimit {
		return true
	}
	return rs.cacheByteLimit > 0 && rs.renderCacheBytes > rs.cacheByteLimit && len(rs.renderCache) > 1
}

func (a *App) requestRender(page int, scale float64, priority ...int) bool {
	if a.renderWorker == nil || page < 0 || page >= a.pageCount {
		return false
	}
	renderScale := a.renderScaleFor(scale)
	cacheKey := renderCacheKey(page, renderScale, a.altColors, a.config.AntiAliasing)
	if _, ok := a.renderCache[cacheKey]; ok {
		a.touchRenderCacheEntry(cacheKey)
		return false
	}
	requestedPriority := 0
	if len(priority) > 0 {
		requestedPriority = priority[0]
	}
	if req, ok := a.renderPending[cacheKey]; ok {
		if requestedPriority < req.priority {
			req.priority = requestedPriority
			a.renderPending[cacheKey] = req
		}
		return false
	}
	req := renderRequest{
		generation: a.renderGeneration,
		page:       page,
		scale:      renderScale,
		altColors:  a.altColors,
		aaLevel:    a.config.AntiAliasing,
		cacheKey:   cacheKey,
	}
	req.priority = requestedPriority
	if !a.renderWorker.Enqueue(req) {
		a.logf("render enqueue skipped page=%d key=%s", page+1, cacheKey)
		return false
	}
	a.renderPending[cacheKey] = req
	return true
}

func (a *App) hasPendingVisibleRender() bool {
	for _, req := range a.renderPending {
		if req.generation == a.renderGeneration && req.priority <= 0 {
			return true
		}
	}
	return false
}

func (a *App) pendingBackgroundRenderCount() int {
	count := 0
	for _, req := range a.renderPending {
		if req.generation == a.renderGeneration && req.priority > 0 {
			count++
		}
	}
	return count
}

func (a *App) invalidateRenderRequests() {
	a.renderGeneration++
	a.renderPending = map[string]renderRequest{}
	if a.renderWorker != nil {
		a.renderWorker.SetGeneration(a.renderGeneration)
	}
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

func (a *App) clearCache() {
	for _, rp := range a.renderCache {
		if rp.texture != nil {
			sdl.DestroyTexture(rp.texture)
		}
	}
	for _, rp := range a.thumbnailCache {
		if rp.texture != nil {
			sdl.DestroyTexture(rp.texture)
		}
	}
	a.renderCache = map[string]*renderedPage{}
	a.thumbnailCache = map[renderVariantKey]*renderedPage{}
	a.renderCacheBytes = 0
	a.thumbnailBytes = 0
	a.renderLRU = list.New()
	a.thumbnailLRU = list.New()
	a.renderLRUItems = map[string]*list.Element{}
	a.thumbnailLRUItems = map[renderVariantKey]*list.Element{}
	a.renderIndex = map[renderVariantKey]*renderedPage{}
	a.invalidateRenderRequests()
}

func (rs *renderService) renderDrawScale(rp *renderedPage, layoutScale float64) float64 {
	if rp == nil || rp.scale <= 0 {
		return 1
	}
	return layoutScale / rp.scale
}

const (
	defaultMinRenderBaseScale = 0.25
	defaultRenderOversample   = 1
	defaultPageCacheSize      = 16
	defaultThumbnailMaxPixels = 4 * 1024 * 1024
	maxPendingPrefetchRenders = 4
	renderPrefetchPriority    = 10
	renderUpgradeTolerance    = 0.95
	renderDowngradeHeadroom   = 2.0
	renderScaleSettleDelay    = 75 * time.Millisecond
	thumbnailInitialZoom      = 0.5
	thumbnailMaxZoom          = 0.5
)

// renderThreadCount leaves a core for the UI when render_threads is 0 (auto).
func renderThreadCount(cfg config.Config) int {
	if cfg.RenderThreads > 0 {
		return cfg.RenderThreads
	}
	return min(4, max(1, runtime.NumCPU()-1))
}

func estimatedTextureBytes(width, height int) int64 {
	if width <= 0 || height <= 0 {
		return 0
	}
	return int64(width) * int64(height) * 4
}

func validRenderScale(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func pageCacheLimit(cfg config.Config, pageCount int) int {
	limit := cfg.PageCacheSize
	if limit <= 0 {
		limit = defaultPageCacheSize
	}
	if pageCount > 0 && limit > pageCount {
		return pageCount
	}
	return limit
}

func pageCacheByteLimit(cfg config.Config) int64 {
	return int64(cfg.PageCacheMemoryMB) << 20
}

func thumbnailDimensions(w, h int) (int, int, float64) {
	if w <= 0 || h <= 0 {
		return 0, 0, 0
	}
	scale := thumbnailMaxZoom
	pixels := w * h
	if pixels > defaultThumbnailMaxPixels {
		scale = math.Sqrt(float64(defaultThumbnailMaxPixels)/float64(pixels)) * thumbnailInitialZoom
		if scale > thumbnailMaxZoom {
			scale = thumbnailMaxZoom
		}
	}
	tw := max(1, int(float64(w)*scale))
	th := max(1, int(float64(h)*scale))
	return tw, th, scale
}

func (rs *renderService) renderScaleFloor() float64 {
	if validRenderScale(rs.minRenderBaseScale) {
		return rs.minRenderBaseScale
	}
	return defaultMinRenderBaseScale
}

func (a *App) renderOversampleFactor() float64 {
	if validRenderScale(a.config.RenderOversample) {
		return a.config.RenderOversample
	}
	return defaultRenderOversample
}

func (a *App) oversampledRenderScale(scale float64) float64 {
	if !validRenderScale(scale) {
		scale = 1
	}
	return math.Max(scale*a.renderOversampleFactor(), a.renderScaleFloor())
}

func (a *App) currentRenderTarget() float64 {
	target := a.scale
	if !validRenderScale(target) {
		target = 1
	}
	if a.fitMode != "manual" && validRenderScale(a.zoom) {
		target = math.Max(target, a.zoom)
	}
	return a.oversampledRenderScale(target)
}

func (a *App) ensureRenderBaseScale() {
	floor := a.renderScaleFloor()
	if validRenderScale(a.renderBaseScale) {
		if a.renderBaseScale < floor {
			a.renderBaseScale = floor
		}
		return
	}
	a.renderBaseScale = math.Max(a.oversampledRenderScale(a.currentRenderTarget()), floor)
}

func (a *App) maybeUpgradeRenderScale(target float64) bool {
	a.ensureRenderBaseScale()
	if !validRenderScale(target) {
		return false
	}
	target = a.oversampledRenderScale(target)
	if target <= a.renderBaseScale*renderUpgradeTolerance {
		return false
	}
	return a.applyRenderBaseScaleTarget(target)
}

func (a *App) maybeDowngradeRenderScale() {
	a.ensureRenderBaseScale()
	target := a.currentRenderTarget()
	if target*renderDowngradeHeadroom >= a.renderBaseScale {
		return
	}
	a.applyRenderBaseScaleTarget(target)
}

func (a *App) scheduleRenderScaleTarget(target float64) {
	a.ensureRenderBaseScale()
	if !validRenderScale(target) {
		return
	}
	target = a.oversampledRenderScale(target)
	if target <= a.renderBaseScale*renderUpgradeTolerance && target*renderDowngradeHeadroom >= a.renderBaseScale {
		a.renderScaleTarget = 0
		a.renderScaleReadyAt = time.Time{}
		return
	}
	if math.Abs(target-a.renderScaleTarget) < 0.01 && !a.renderScaleReadyAt.IsZero() {
		return
	}
	a.renderScaleTarget = target
	a.renderScaleReadyAt = time.Now().Add(renderScaleSettleDelay)
}

func (a *App) applyScheduledRenderScaleTarget() bool {
	if !validRenderScale(a.renderScaleTarget) || a.renderScaleReadyAt.IsZero() || time.Now().Before(a.renderScaleReadyAt) {
		return false
	}
	target := a.renderScaleTarget
	a.renderScaleTarget = 0
	a.renderScaleReadyAt = time.Time{}
	return a.applyRenderBaseScaleTarget(target)
}

func (a *App) applyRenderBaseScaleTarget(target float64) bool {
	a.ensureRenderBaseScale()
	if !validRenderScale(target) {
		return false
	}
	floor := a.renderScaleFloor()
	if target > a.renderBaseScale*renderUpgradeTolerance {
		next := math.Max(target, floor)
		if next > a.renderBaseScale+0.01 {
			a.renderBaseScale = next
			a.logf("upgrade render scale target=%.3f base=%.3f", target, next)
			a.invalidateRenderRequests()
			return true
		}
	}
	if target*renderDowngradeHeadroom < a.renderBaseScale {
		next := math.Max(target, floor)
		if next < a.renderBaseScale {
			a.renderBaseScale = next
			a.logf("downgrade render scale target=%.3f base=%.3f", target, next)
			a.invalidateRenderRequests()
			return true
		}
	}
	return false
}

func (a *App) adjustRenderBaseScaleForExtremeZoom(layoutScale float64) {
	a.scheduleRenderScaleTarget(layoutScale)
	if a.applyScheduledRenderScaleTarget() {
		return
	}
}
