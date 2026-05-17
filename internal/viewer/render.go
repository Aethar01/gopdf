package viewer

import (
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

type renderWorker struct {
	requests   chan renderRequest
	updates    chan renderUpdate
	closing    chan struct{}
	done       chan struct{}
	closeOnce  sync.Once
	generation atomic.Int32
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

func (w *renderWorker) Close() {
	w.closeOnce.Do(func() { close(w.closing) })
	<-w.done
}

func (w *renderWorker) SetGeneration(generation int) {
	w.generation.Store(int32(generation))
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
		if req.generation != gen {
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
	a.renderPending = map[string]renderRequest{}
	a.renderWorker = newRenderWorker(a.docPath)
	a.renderWorker.SetGeneration(a.renderGeneration)
}

func (a *App) closeRenderWorker() {
	if a.renderWorker != nil {
		a.renderWorker.Close()
		a.renderWorker = nil
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
			oldRP := a.renderCache[update.cacheKey]
			if oldRP != nil {
				sdl.DestroyTexture(oldRP.texture)
			}
			tex, err := textureFromImage(a.renderer, update.rendered.Image)
			if err != nil {
				a.message = err.Error()
				continue
			}
			rp := &renderedPage{
				texture:   tex,
				width:     float64(update.rendered.Image.Bounds().Dx()),
				height:    float64(update.rendered.Image.Bounds().Dy()),
				pixX:      float64(update.rendered.X),
				pixY:      float64(update.rendered.Y),
				key:       update.cacheKey,
				page:      update.page,
				scale:     update.scale,
				altColors: update.altColors,
				aaLevel:   update.aaLevel,
			}
			a.renderCache[update.cacheKey] = rp
			a.renderOrder = append(a.renderOrder, update.cacheKey)
			a.startPendingMetricLoader()
			a.pendingRedraw = true
			for len(a.renderOrder) > a.cacheLimit {
				oldest := a.renderOrder[0]
				a.renderOrder = a.renderOrder[1:]
				if _, pending := a.renderPending[oldest]; pending {
					continue
				}
				if oldRP := a.renderCache[oldest]; oldRP != nil {
					sdl.DestroyTexture(oldRP.texture)
				}
				delete(a.renderCache, oldest)
			}
		default:
			return
		}
	}
}

func (a *App) touchRenderCacheEntry(key string) {
	for i, k := range a.renderOrder {
		if k == key {
			a.renderOrder = append(a.renderOrder[:i], a.renderOrder[i+1:]...)
			a.renderOrder = append(a.renderOrder, key)
			return
		}
	}
}

func (a *App) evictRenderCacheEntry(key string) {
	for i, k := range a.renderOrder {
		if k == key {
			a.renderOrder = append(a.renderOrder[:i], a.renderOrder[i+1:]...)
			if oldRP := a.renderCache[key]; oldRP != nil {
				sdl.DestroyTexture(oldRP.texture)
			}
			delete(a.renderCache, key)
			return
		}
	}
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
		return
	}
	a.renderPending[cacheKey] = req
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

func (a *App) renderDrawScale(rp *renderedPage, layoutScale float64) float64 {
	if rp == nil || rp.scale <= 0 {
		return 1
	}
	return layoutScale / rp.scale
}

const (
	defaultMinRenderBaseScale = 0.25
	defaultRenderOversample   = 1
	renderUpgradeTolerance    = 0.95
	renderDowngradeHeadroom   = 2.0
	renderScaleStep           = 1.5
)

func validRenderScale(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func (a *App) renderScaleFloor() float64 {
	if validRenderScale(a.minRenderBaseScale) {
		return a.minRenderBaseScale
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
