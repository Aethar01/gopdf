package viewer

import (
	"container/list"
	"fmt"
	"math"
	"sync"
	"sync/atomic"

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
	cacheLimit         int
	cacheByteLimit     int64
	renderCacheBytes   int64
	renderBaseScale    float64
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
	requests   chan renderRequest
	updates    chan renderUpdate
	closing    chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	generation atomic.Int32
	wanted     atomic.Value
}

func newRenderWorker(docPath string) *renderWorker {
	w := &renderWorker{
		requests: make(chan renderRequest, 128),
		updates:  make(chan renderUpdate, 128),
		closing:  make(chan struct{}),
		done:     make(chan struct{}),
	}
	go w.run(docPath)
	return w
}

func (w *renderWorker) Close() bool {
	return closeWorker(w.closing, w.done, &w.closeOnce)
}

func (w *renderWorker) SetGeneration(generation int) {
	w.generation.Store(int32(generation))
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

func (w *renderWorker) run(docPath string) {
	defer close(w.done)
	doc, err := mupdf.Open(docPath)
	if err != nil {
		w.send(renderUpdate{err: err})
		w.closeOnce.Do(func() { close(w.closing) })
		return
	}
	defer doc.Close()
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
	a.renderWorker = newRenderWorker(a.docPath)
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
			tex, err := textureFromImage(a.renderer, update.rendered.Image)
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
			a.startPendingMetricLoader()
			a.pendingRedraw = true
			a.enforceRenderCacheLimit()
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

func (rs *renderService) addRenderCacheEntry(key string, rp *renderedPage) {
	rs.ensureRenderCacheState()
	rs.removeRenderCacheEntry(key, true)
	if rp.bytes <= 0 {
		rp.bytes = estimatedTextureBytes(int(rp.width), int(rp.height))
	}
	rs.renderCache[key] = rp
	rs.renderCacheBytes += rp.bytes
	rs.renderLRUItems[key] = rs.renderLRU.PushBack(key)
	rs.indexRenderPage(key, rp)
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
		front := rs.renderLRU.Front()
		if front == nil {
			return
		}
		key, _ := front.Value.(string)
		if _, pending := rs.renderPending[key]; pending {
			rs.renderLRU.MoveToBack(front)
			continue
		}
		rs.removeRenderCacheEntry(key, true)
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
	rs.renderCache = map[string]*renderedPage{}
	rs.renderCacheBytes = 0
	rs.renderLRU = list.New()
	rs.renderLRUItems = map[string]*list.Element{}
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
	defaultMinRenderBaseScale   = 0.25
	defaultRenderOversample     = 1
	defaultRenderCacheByteLimit = 512 << 20
	renderUpgradeTolerance      = 0.95
	renderDowngradeHeadroom     = 2.0
	renderScaleStep             = 1.5
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
	a.renderBaseScale = math.Max(math.Max(2, a.currentRenderTarget()), floor)
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
	next := math.Max(a.renderBaseScale, 2)
	next = math.Max(next, a.renderScaleFloor())
	for next < target {
		next *= renderScaleStep
	}
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
	next := a.renderBaseScale / renderScaleStep
	next = math.Max(next, floor)
	next = math.Max(next, target)
	if next >= a.renderBaseScale {
		return
	}
	a.renderBaseScale = next
	a.logf("downgrade render scale target=%.3f base=%.3f", target, next)
	a.invalidateRenderRequests()
}

func (a *App) adjustRenderBaseScaleForExtremeZoom(layoutScale float64) {
	if a.maybeUpgradeRenderScale(layoutScale) {
		return
	}
	if layoutScale < a.renderBaseScale/4 && a.renderBaseScale > 1 {
		a.maybeDowngradeRenderScale()
	}
}
