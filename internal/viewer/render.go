package viewer

import (
	"container/list"
	"fmt"
	"math"
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
	generation int
	page       int
	scale      float64
	altColors  bool
	aaLevel    int
	cacheKey   string
	rendered   *mupdf.RenderedPage
	err        error
}

type renderService struct {
	renderCache        map[string]*renderedPage
	renderLRU          *list.List
	renderLRUItems     map[string]*list.Element
	renderIndex        map[renderVariantKey]map[string]*renderedPage
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
	renderWorker       *renderWorker
}

type renderVariantKey struct {
	page      int
	altColors bool
	aaLevel   int
}

type renderWorker struct {
	doc        *mupdf.Document
	requests   chan renderRequest
	updates    chan renderUpdate
	closing    chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	generation atomic.Int32
	wanted     atomic.Value
}

func newRenderWorker(doc *mupdf.Document) *renderWorker {
	w := &renderWorker{
		doc:      doc,
		requests: make(chan renderRequest, 128),
		updates:  make(chan renderUpdate, 1),
		closing:  make(chan struct{}),
		done:     make(chan struct{}),
	}
	go w.run(doc)
	return w
}

func (w *renderWorker) Close() {
	w.Cancel()
	closeWorker(w.closing, w.done, &w.closeOnce)
}

func (w *renderWorker) Cancel() {
	if w != nil && w.doc != nil {
		w.doc.CancelRender()
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

func (w *renderWorker) DrainUnwanted(gen int) {
	var keep []renderRequest
	for {
		select {
		case req := <-w.requests:
			if w.requestWanted(req, gen) {
				keep = append(keep, req)
			}
		default:
			for _, req := range keep {
				select {
				case <-w.closing:
					return
				case w.requests <- req:
				}
			}
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

func (w *renderWorker) run(doc *mupdf.Document) {
	defer close(w.done)
	if doc == nil {
		w.send(renderUpdate{err: fmt.Errorf("render worker: no document open")})
		w.closeOnce.Do(func() { close(w.closing) })
		return
	}
	var queue []renderRequest
	for {
		if len(queue) == 0 {
			select {
			case <-w.closing:
				return
			case req := <-w.requests:
				queue = append(queue, req)
			}
		}
		drain := true
		for drain {
			select {
			case req := <-w.requests:
				queue = append(queue, req)
			default:
				drain = false
			}
		}
		req, nextQueue, ok := w.popNextRequest(queue)
		queue = nextQueue
		if !ok {
			continue
		}
		rendered, err := doc.Render(req.page, req.scale, 0, req.aaLevel)
		w.send(renderUpdate{
			generation: req.generation,
			page:       req.page,
			scale:      req.scale,
			altColors:  req.altColors,
			aaLevel:    req.aaLevel,
			cacheKey:   req.cacheKey,
			rendered:   rendered,
			err:        err,
		})
	}
}

func (w *renderWorker) popNextRequest(queue []renderRequest) (renderRequest, []renderRequest, bool) {
	gen := int(w.generation.Load())
	best := -1
	for i, req := range queue {
		if !w.requestWanted(req, gen) {
			continue
		}
		if best < 0 || req.priority < queue[best].priority {
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

func (w *renderWorker) send(update renderUpdate) {
	select {
	case <-w.closing:
		return
	case w.updates <- update:
	}
}

func renderCacheKey(page int, scale float64, altColors bool, aaLevel int) string {
	return fmt.Sprintf("%d/%.4f/%t/%d", page, scale, altColors, aaLevel)
}

func (a *App) initRenderWorker() {
	a.logf("start render worker path=%q", a.docPath)
	a.renderPending = map[string]renderRequest{}
	a.renderWorker = newRenderWorker(a.doc)
	a.renderWorker.SetGeneration(a.renderGeneration)
}

func (rs *renderService) closeRenderWorker() {
	if rs.renderWorker != nil {
		rs.renderWorker.Close()
		rs.renderWorker = nil
	}
}

func (a *App) pollRenderUpdates() {
	if a.renderWorker == nil {
		return
	}
	for {
		select {
		case update := <-a.renderWorker.updates:
			if update.rendered != nil {
				defer update.rendered.Close()
			}
			if update.err != nil {
				a.logf("render update failed err=%v", update.err)
				a.message = update.err.Error()
				continue
			}
			if update.generation != a.renderGeneration {
				delete(a.renderPending, update.cacheKey)
				continue
			}
			if _, pending := a.renderPending[update.cacheKey]; !pending {
				continue
			}
			if update.rendered == nil {
				delete(a.renderPending, update.cacheKey)
				continue
			}
			if update.altColors {
				remapPageColors(update.rendered.Image, a.config.AltBackground, a.config.AltForeground)
			}
			delete(a.renderPending, update.cacheKey)
			a.removeRenderCacheEntry(update.cacheKey, true)
			tex, err := textureFromRGBA(a.renderer, update.rendered.Image)
			if err != nil {
				a.logf("render texture failed page=%d err=%v", update.page+1, err)
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
				key:       update.cacheKey,
				page:      update.page,
				scale:     update.scale,
				altColors: update.altColors,
				aaLevel:   update.aaLevel,
			}
			a.addRenderCacheEntry(update.cacheKey, rp)
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

func (rs *renderService) evictRenderCacheEntry(key string) {
	rs.removeRenderCacheEntry(key, true)
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
		rs.renderIndex = map[renderVariantKey]map[string]*renderedPage{}
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
	variant := renderVariantKey{page: rp.page, altColors: rp.altColors, aaLevel: rp.aaLevel}
	pages := rs.renderIndex[variant]
	if pages == nil {
		pages = map[string]*renderedPage{}
		rs.renderIndex[variant] = pages
	}
	pages[key] = rp
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
	keys := make([]string, 0, len(rs.renderIndex[variant]))
	for key := range rs.renderIndex[variant] {
		keys = append(keys, key)
	}
	for _, key := range keys {
		rs.removeRenderCacheEntry(key, true)
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
	if pages := rs.renderIndex[variant]; pages != nil {
		delete(pages, key)
		if len(pages) == 0 {
			delete(rs.renderIndex, variant)
		}
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

func (a *App) requestRender(page int, scale float64, priority ...int) {
	if a.renderWorker == nil || page < 0 || page >= a.pageCount {
		return
	}
	renderScale := a.renderScaleFor(scale)
	cacheKey := renderCacheKey(page, renderScale, a.altColors, a.config.AntiAliasing)
	if _, ok := a.renderCache[cacheKey]; ok {
		a.touchRenderCacheEntry(cacheKey)
		return
	}
	if _, ok := a.renderPending[cacheKey]; ok {
		return
	}
	req := renderRequest{
		generation: a.renderGeneration,
		page:       page,
		scale:      renderScale,
		altColors:  a.altColors,
		aaLevel:    a.config.AntiAliasing,
		cacheKey:   cacheKey,
	}
	if len(priority) > 0 {
		req.priority = priority[0]
	}
	if !a.renderWorker.Enqueue(req) {
		a.logf("render enqueue skipped page=%d key=%s", page+1, cacheKey)
		return
	}
	a.renderPending[cacheKey] = req
}

func (rs *renderService) invalidateRenderRequests() {
	rs.renderGeneration++
	rs.renderPending = map[string]renderRequest{}
	if rs.renderWorker != nil {
		rs.renderWorker.SetGeneration(rs.renderGeneration)
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

func (rs *renderService) clearCache() {
	for _, rp := range rs.renderCache {
		if rp.texture != nil {
			sdl.DestroyTexture(rp.texture)
		}
	}
	for _, rp := range rs.thumbnailCache {
		if rp.texture != nil {
			sdl.DestroyTexture(rp.texture)
		}
	}
	rs.renderCache = map[string]*renderedPage{}
	rs.thumbnailCache = map[renderVariantKey]*renderedPage{}
	rs.renderCacheBytes = 0
	rs.thumbnailBytes = 0
	rs.renderLRU = list.New()
	rs.thumbnailLRU = list.New()
	rs.renderLRUItems = map[string]*list.Element{}
	rs.thumbnailLRUItems = map[renderVariantKey]*list.Element{}
	rs.renderIndex = map[renderVariantKey]map[string]*renderedPage{}
	rs.invalidateRenderRequests()
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
	renderUpgradeTolerance    = 0.95
	renderDowngradeHeadroom   = 2.0
	renderScaleSettleDelay    = 75 * time.Millisecond
	thumbnailInitialZoom      = 0.5
	thumbnailMaxZoom          = 0.5
)

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
	floor := a.renderScaleFloor()
	if !validRenderScale(target) {
		return false
	}
	target = a.oversampledRenderScale(target)
	if target <= a.renderBaseScale*renderUpgradeTolerance {
		return false
	}
	next := math.Max(target, floor)
	if next <= a.renderBaseScale+0.01 {
		return false
	}
	a.renderBaseScale = next
	a.logf("upgrade render scale target=%.3f base=%.3f", target, next)
	a.invalidateRenderRequests()
	return true
}

func (a *App) maybeDowngradeRenderScale() {
	a.ensureRenderBaseScale()
	floor := a.renderScaleFloor()
	if a.renderBaseScale <= floor {
		return
	}
	target := a.currentRenderTarget()
	if target*renderDowngradeHeadroom >= a.renderBaseScale {
		return
	}
	next := math.Max(target, floor)
	if next >= a.renderBaseScale {
		return
	}
	a.renderBaseScale = next
	a.logf("downgrade render scale target=%.3f base=%.3f", target, next)
	a.invalidateRenderRequests()
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
