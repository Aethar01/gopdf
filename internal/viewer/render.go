package viewer

import (
	"fmt"

	"gopdf/internal/mupdf"

	"github.com/hajimehoshi/ebiten/v2"
)

type renderRequest struct {
	generation int
	page       int
	scale      float64
	rotation   float64
	altColors  bool
	cacheKey   string
}

type renderUpdate struct {
	generation int
	page       int
	scale      float64
	altColors  bool
	cacheKey   string
	rendered   *mupdf.RenderedPage
	err        error
}

type renderWorker struct {
	requests chan renderRequest
	updates  chan renderUpdate
	closing  chan struct{}
	done     chan struct{}
}

func newRenderWorker(docPath string) *renderWorker {
	w := &renderWorker{
		requests: make(chan renderRequest, 128),
		updates:  make(chan renderUpdate, 128),
		closing:  make(chan struct{}),
		done:    make(chan struct{}),
	}
	go w.run(docPath)
	return w
}

func (w *renderWorker) Close() {
	close(w.closing)
	<-w.done
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
		return
	}
	defer doc.Close()
	for {
		select {
		case <-w.closing:
			return
		case req := <-w.requests:
			rendered, err := doc.Render(req.page, req.scale, req.rotation)
			w.send(renderUpdate{
				generation: req.generation,
				page:       req.page,
				scale:      req.scale,
				altColors:  req.altColors,
				cacheKey:   req.cacheKey,
				rendered:   rendered,
				err:        err,
			})
		}
	}
}

func (w *renderWorker) send(update renderUpdate) {
	select {
	case <-w.closing:
		return
	case w.updates <- update:
	}
}

func renderCacheKey(page int, scale, rotation float64, altColors bool) string {
	return fmt.Sprintf("%d/%.4f/%.1f/%t", page, scale, rotation, altColors)
}

func (a *App) initRenderWorker() {
	a.renderPending = map[string]renderRequest{}
	a.renderWorker = newRenderWorker(a.docPath)
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
				a.lastErr = update.err
				a.message = update.err.Error()
				continue
			}
			if update.generation != a.renderGeneration {
				delete(a.renderPending, update.cacheKey)
				continue
			}
			delete(a.renderPending, update.cacheKey)
			if update.rendered == nil {
				continue
			}
			if update.altColors {
				remapPageColors(update.rendered.Image, a.config.AltBackground, a.config.AltForeground)
			}
			oldRP := a.renderCache[update.cacheKey]
			if oldRP != nil {
				oldRP.image.Dispose()
			}
			rp := &renderedPage{
				image:  ebiten.NewImageFromImage(update.rendered.Image),
				width:  float64(update.rendered.Image.Bounds().Dx()),
				height: float64(update.rendered.Image.Bounds().Dy()),
				pixX:   float64(update.rendered.X),
				pixY:   float64(update.rendered.Y),
				key:    update.cacheKey,
				page:   update.page,
				scale:  update.scale,
			}
			a.renderCache[update.cacheKey] = rp
			a.renderOrder = append(a.renderOrder, update.cacheKey)
			a.pendingRedraw = true
			for len(a.renderOrder) > a.cacheLimit {
				oldest := a.renderOrder[0]
				a.renderOrder = a.renderOrder[1:]
				if _, pending := a.renderPending[oldest]; pending {
					continue
				}
				if oldRP := a.renderCache[oldest]; oldRP != nil {
					oldRP.image.Dispose()
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
				oldRP.image.Dispose()
			}
			delete(a.renderCache, key)
			return
		}
	}
}

func (a *App) requestRender(page int, scale float64) {
	if a.renderWorker == nil || page < 0 || page >= a.pageCount {
		return
	}
	renderScale := a.renderScaleFor(scale)
	cacheKey := renderCacheKey(page, renderScale, a.rotation, a.altColors)
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
		rotation:   a.rotation,
		altColors:  a.altColors,
		cacheKey:   cacheKey,
	}
	if !a.renderWorker.Enqueue(req) {
		return
	}
	a.renderPending[cacheKey] = req
}

func (a *App) invalidateRenderRequests() {
	a.renderGeneration++
	a.renderPending = map[string]renderRequest{}
}
