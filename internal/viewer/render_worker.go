package viewer

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

type renderRequest struct {
	generation int
	page       int
	scale      float64
	altColors  bool
	aaLevel    int
	cacheKey   string
	priority   int

	// Colors the render is remapped to when altColors is set.
	altBackground, altForeground [3]uint8
}

type renderUpdate struct {
	request  renderRequest
	rendered *mupdf.RenderedPage
	err      error
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
		if err == nil && req.altColors {
			remapPageColors(rendered.Image, req.altBackground, req.altForeground)
		}
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

// renderThreadCount leaves a core for the UI when render_threads is 0 (auto).
func renderThreadCount(cfg config.Config) int {
	if cfg.RenderThreads > 0 {
		return cfg.RenderThreads
	}
	return min(4, max(1, runtime.NumCPU()-1))
}
