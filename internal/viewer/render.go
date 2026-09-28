package viewer

import (
	"image"
	"math"
	"time"

	"gopdf/internal/config"
)

type renderService struct {
	cache              tileCache
	renderPending      map[tileKey]renderRequest
	renderBaseScale    float64
	renderScaleTarget  float64
	renderScaleReadyAt time.Time
	minRenderBaseScale float64
	renderGeneration   int
}

func (a *App) initRenderWorker() {
	a.logf("start render worker path=%q", a.docPath)
	a.renderPending = map[tileKey]renderRequest{}
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
			if update.rendered != nil {
				defer update.rendered.Close()
			}
			a.acceptRenderUpdate(update)
		default:
			return
		}
	}
}

func (a *App) acceptRenderUpdate(update renderUpdate) {
	req := update.request
	if _, pending := a.renderPending[req.key]; !pending || req.generation != a.renderGeneration {
		return
	}
	delete(a.renderPending, req.key)
	if update.err != nil {
		a.logf("render update failed err=%v", update.err)
		a.message = update.err.Error()
		return
	}
	img := update.rendered.Image
	if img.Bounds().Empty() {
		return
	}
	tex, err := textureFromRGBA(a.renderer, img)
	if err != nil {
		a.logf("render texture failed page=%d err=%v", req.key.page+1, err)
		a.message = err.Error()
		return
	}
	origin := image.Pt(update.rendered.X, update.rendered.Y)
	tile := &renderedTile{key: req.key, texture: tex, rect: img.Bounds().Add(origin), scale: req.key.scale}
	a.cache.add(tile)
	a.updateThumbnail(tile)
	a.cache.evict()
	a.startPendingMetricLoader()
	a.pendingRedraw = true
}

// requestTile queues a render of the tile at key unless it is cached or
// already queued, in which case a more urgent priority is kept. It reports
// whether a new render was queued.
func (a *App) requestTile(key tileKey, rect image.Rectangle, priority int) bool {
	if a.renderWorker == nil {
		return false
	}
	if _, ok := a.cache.get(key); ok {
		return false
	}
	if req, ok := a.renderPending[key]; ok {
		if priority < req.priority {
			req.priority = priority
			a.renderPending[key] = req
		}
		return false
	}
	req := renderRequest{
		generation: a.renderGeneration,
		key:        key,
		rect:       rect,
		altColors:  a.altColors,
		aaLevel:    a.config.AntiAliasing,
		priority:   priority,
	}
	if req.altColors {
		req.altBackground, req.altForeground = a.config.AltBackground, a.config.AltForeground
	}
	if !a.renderWorker.Enqueue(req) {
		a.logf("render enqueue skipped page=%d tile=%d,%d", key.page+1, key.x, key.y)
		return false
	}
	a.renderPending[key] = req
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
	a.renderPending = map[tileKey]renderRequest{}
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
	a.cache.clear()
	a.invalidateRenderRequests()
}

const (
	defaultMinRenderBaseScale = 0.25
	defaultRenderOversample   = 1
	maxPendingPrefetchRenders = 4
	renderPrefetchPriority    = 10
	renderUpgradeTolerance    = 0.95
	renderDowngradeHeadroom   = 2.0
	renderScaleSettleDelay    = 75 * time.Millisecond
)

func validRenderScale(v float64) bool {
	return v > 0 && !math.IsNaN(v) && !math.IsInf(v, 0)
}

func pageCacheByteLimit(cfg config.Config) int64 {
	return int64(cfg.PageCacheMemoryMB) << 20
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
