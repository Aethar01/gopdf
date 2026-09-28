package viewer

import (
	"fmt"
	"image"
	"runtime"
	"sync"
	"sync/atomic"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

type renderRequest struct {
	generation int
	key        tileKey
	rect       image.Rectangle // device pixels of key's tile
	altColors  bool
	aaLevel    int
	priority   int

	// Colors the render is remapped to when altColors is set, leaving
	// raster images alone when keepImages is.
	altBackground, altForeground [3]uint8
	keepImages                   bool
}

type renderUpdate struct {
	request  renderRequest
	rendered *mupdf.RenderedPage
	err      error
}

// renderWorker rasterises tiles on a pool of goroutines, each with its own
// MuPDF renderer, taking the most urgent wanted request from a shared queue.
type renderWorker struct {
	workerLifecycle
	doc        *mupdf.Document
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
	renderer  *mupdf.Renderer
	rendering atomic.Pointer[tileKey] // nil when idle
}

func (s *renderSlot) active() (tileKey, bool) {
	key := s.rendering.Load()
	if key == nil {
		return tileKey{}, false
	}
	return *key, true
}

func (s *renderSlot) cancel() {
	if s.renderer != nil {
		s.renderer.Cancel()
	}
}

func newRenderWorker(doc *mupdf.Document, threads int) *renderWorker {
	w := &renderWorker{
		workerLifecycle: newWorkerLifecycle(),
		doc:             doc,
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

// SetWanted replaces the set of tiles worth rendering and cancels a render
// in progress that is no longer among them.
func (w *renderWorker) SetWanted(keys map[tileKey]bool) {
	w.wanted.Store(keys)
	for _, slot := range w.slots {
		if key, ok := slot.active(); ok && !keys[key] {
			slot.cancel()
		}
	}
}

// SetVisible replaces the set of on-screen tiles, which render first.
func (w *renderWorker) SetVisible(keys map[tileKey]bool) {
	w.visible.Store(keys)
}

// CancelNotVisible cancels renders of tiles outside visible and returns them.
func (w *renderWorker) CancelNotVisible(visible map[tileKey]bool) []tileKey {
	var cancelled []tileKey
	for _, slot := range w.slots {
		if key, ok := slot.active(); ok && !visible[key] {
			slot.cancel()
			cancelled = append(cancelled, key)
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
	wanted, ok := w.wanted.Load().(map[tileKey]bool)
	return !ok || wanted[req.key]
}

func (w *renderWorker) requestPriority(req renderRequest) int {
	visible, ok := w.visible.Load().(map[tileKey]bool)
	if !ok {
		return req.priority
	}
	if visible[req.key] {
		return 0
	}
	if req.priority <= 0 {
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
		slot.rendering.Store(&req.key)
		rendered, err := slot.renderer.Render(req.key.page, req.key.scale, req.rect, req.aaLevel)
		if err == nil && req.altColors {
			remapPageColors(rendered.Image, req.altBackground, req.altForeground, w.keptImageRects(req, rendered))
		}
		slot.rendering.Store(nil)
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

// keptImageRects returns the image areas a recolour should skip, in the
// rendered tile's pixel coordinates.
func (w *renderWorker) keptImageRects(req renderRequest, rendered *mupdf.RenderedPage) []image.Rectangle {
	if !req.keepImages || w.doc == nil {
		return nil
	}
	images, err := w.doc.ImageBounds(req.key.page)
	if err != nil {
		return nil
	}
	origin := image.Pt(rendered.X, rendered.Y)
	rects := make([]image.Rectangle, len(images))
	for i, bounds := range images {
		rects[i] = mupdf.DeviceRect(bounds, req.key.scale).Sub(origin)
	}
	return rects
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

// renderThreadCount leaves a core for the UI when render_threads is 0 (auto).
func renderThreadCount(cfg config.Config) int {
	if cfg.RenderThreads > 0 {
		return cfg.RenderThreads
	}
	return min(4, max(1, runtime.NumCPU()-1))
}
