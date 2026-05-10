package viewer

import (
	"fmt"
	"math"
	"strings"

	"gopdf/internal/mupdf"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type searchMode int

const (
	searchModeForward searchMode = iota
	searchModeBackward
)

type searchHitRef struct {
	page int
	hit  int
}

type searchState struct {
	query      string
	matches    map[int][]mupdf.SearchHit
	order      []searchHitRef
	current    int
	running    bool
	generation int
	mode       searchMode
}

type searchRequest struct {
	generation int
	query      string
	startPage  int
	pageCount  int
}

type searchUpdate struct {
	generation int
	page       int
	hits       []mupdf.SearchHit
	done       bool
	err        error
}

type searchWorker struct {
	requests chan searchRequest
	updates  chan searchUpdate
	closing  chan struct{}
	done     chan struct{}
}

func newSearchWorker(docPath string) *searchWorker {
	w := &searchWorker{
		requests: make(chan searchRequest, 1),
		updates:  make(chan searchUpdate, 64),
		closing:  make(chan struct{}),
		done:     make(chan struct{}),
	}
	go w.run(docPath)
	return w
}

func (w *searchWorker) Close() {
	close(w.closing)
	<-w.done
}

func (w *searchWorker) Start(req searchRequest) {
	select {
	case w.requests <- req:
	default:
		select {
		case <-w.requests:
		default:
		}
		w.requests <- req
	}
}

func (w *searchWorker) run(docPath string) {
	defer close(w.done)
	doc, err := mupdf.Open(docPath)
	if err != nil {
		w.send(searchUpdate{done: true, err: err})
		return
	}
	defer doc.Close()
	for {
		var req searchRequest
		select {
		case <-w.closing:
			return
		case req = <-w.requests:
		}
		for {
			if strings.TrimSpace(req.query) == "" {
				w.send(searchUpdate{generation: req.generation, done: true})
				break
			}
			restarted := false
			for _, page := range searchPageOrder(req.startPage, req.pageCount) {
				select {
				case <-w.closing:
					return
				case next := <-w.requests:
					req = next
					restarted = true
				default:
				}
				if restarted {
					break
				}
				hits, err := doc.SearchPage(page, req.query)
				if err != nil {
					w.send(searchUpdate{generation: req.generation, done: true, err: err})
					restarted = true
					break
				}
				w.send(searchUpdate{generation: req.generation, page: page, hits: hits})
			}
			if restarted {
				continue
			}
			w.send(searchUpdate{generation: req.generation, done: true})
			break
		}
	}
}

func (w *searchWorker) send(update searchUpdate) {
	select {
	case <-w.closing:
		return
	case w.updates <- update:
	}
}

func searchPageOrder(start, count int) []int {
	pages := make([]int, 0, count)
	if count <= 0 {
		return pages
	}
	start = clampInt(start, 0, count-1)
	for page := start; page < count; page++ {
		pages = append(pages, page)
	}
	for page := 0; page < start; page++ {
		pages = append(pages, page)
	}
	return pages
}

func (a *App) initSearch() {
	a.search = searchState{matches: map[int][]mupdf.SearchHit{}, current: -1, mode: searchModeForward}
	a.searchWorker = newSearchWorker(a.docPath)
}

func (a *App) closeSearch() {
	if a.searchWorker != nil {
		a.searchWorker.Close()
		a.searchWorker = nil
	}
}

func (a *App) pollSearchUpdates() {
	if a.searchWorker == nil {
		return
	}
	for {
		select {
		case update := <-a.searchWorker.updates:
			if update.generation != a.search.generation {
				continue
			}
			if update.err != nil {
				a.search.running = false
				a.message = update.err.Error()
				continue
			}
			if update.done {
				a.search.running = false
				if len(a.search.order) == 0 && a.search.query != "" {
					a.message = fmt.Sprintf("no matches for /%s", a.search.query)
				} else if len(a.search.order) > 0 {
					a.message = a.searchStatusMessage()
				}
				continue
			}
			if len(update.hits) > 0 {
				a.search.matches[update.page] = update.hits
				for i := range update.hits {
					a.search.order = append(a.search.order, searchHitRef{page: update.page, hit: i})
				}
				if a.search.current < 0 {
					a.search.current = 0
					a.focusSearchCurrent()
				}
				a.message = a.searchStatusMessage()
			}
		default:
			return
		}
	}
}

func (a *App) startSearch(query string, mode searchMode) {
	query = strings.TrimSpace(query)
	a.search.generation++
	a.search.query = query
	a.search.matches = map[int][]mupdf.SearchHit{}
	a.search.order = nil
	a.search.current = -1
	a.search.running = false
	a.search.mode = mode
	if query == "" {
		a.message = ""
		return
	}
	a.search.running = true
	a.message = fmt.Sprintf("searching /%s", query)
	if a.searchWorker != nil {
		a.searchWorker.Start(searchRequest{
			generation: a.search.generation,
			query:      query,
			startPage:  a.page,
			pageCount:  a.pageCount,
		})
	}
}

func (a *App) repeatSearch(forward bool) {
	delta := 1
	if a.search.mode == searchModeBackward {
		delta = -1
	}
	if !forward {
		delta = -delta
	}
	a.moveSearch(delta)
}

func (a *App) moveSearch(delta int) {
	if a.search.query == "" {
		a.message = "no active search"
		return
	}
	if len(a.search.order) == 0 {
		if a.search.running {
			a.message = fmt.Sprintf("searching /%s", a.search.query)
			return
		}
		a.message = fmt.Sprintf("no matches for /%s", a.search.query)
		return
	}
	if a.search.current < 0 {
		if delta >= 0 {
			a.search.current = 0
		} else {
			a.search.current = len(a.search.order) - 1
		}
	} else {
		a.search.current = (a.search.current + delta + len(a.search.order)) % len(a.search.order)
	}
	a.focusSearchCurrent()
	a.message = a.searchStatusMessage()
}

func (a *App) focusSearchCurrent() {
	if a.search.current < 0 || a.search.current >= len(a.search.order) {
		return
	}
	ref := a.search.order[a.search.current]
	a.alignPageTop(ref.page)
	x, y, rp, ok := a.pagePlacement(ref.page)
	if !ok || rp == nil {
		return
	}
	hits := a.search.matches[ref.page]
	if ref.hit < 0 || ref.hit >= len(hits) {
		return
	}
	minX, minY, maxX, maxY := a.searchHitBounds(hits[ref.hit], x, y, rp)
	viewportW, viewportH := a.viewportSize()
	centerX := (minX + maxX) / 2
	centerY := (minY + maxY) / 2
	a.scrollBy(centerX-float64(viewportW)/2, centerY-float64(viewportH)/2)
	if a.renderMode == "single" {
		a.page = ref.page
	}
}

func (a *App) searchHitBounds(hit mupdf.SearchHit, x, y float64, rp *renderedPage) (float64, float64, float64, float64) {
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, quad := range hit.Quads {
		quadMinX, quadMinY, quadMaxX, quadMaxY := a.quadScreenBounds(quad, x, y, rp)
		minX = math.Min(minX, quadMinX)
		minY = math.Min(minY, quadMinY)
		maxX = math.Max(maxX, quadMaxX)
		maxY = math.Max(maxY, quadMaxY)
	}
	return minX, minY, maxX, maxY
}

func (a *App) drawSearchHighlightsForPage(screen *ebiten.Image, page int, x, y float64, rp *renderedPage) {
	hits := a.search.matches[page]
	if len(hits) == 0 {
		return
	}
	for i, hit := range hits {
		active := false
		if a.search.current >= 0 && a.search.current < len(a.search.order) {
			ref := a.search.order[a.search.current]
			active = ref.page == page && ref.hit == i
		}
		a.drawHighlightQuadsWithStyle(screen, hit.Quads, x, y, rp, active)
	}
}

func (a *App) drawHighlightQuadsWithStyle(screen *ebiten.Image, quads []mupdf.Quad, x, y float64, rp *renderedPage, active bool) {
	bg := a.highlightBackgroundColor()
	fg := a.highlightForegroundColor()
	stroke := float32(1)
	if active {
		bg.A = 0xdd
		stroke = 2
	}
	for _, quad := range quads {
		minX, minY, maxX, maxY := a.quadScreenBounds(quad, x, y, rp)
		vector.DrawFilledRect(screen, float32(minX), float32(minY), float32(maxX-minX), float32(maxY-minY), bg, false)
		vector.StrokeRect(screen, float32(minX), float32(minY), float32(maxX-minX), float32(maxY-minY), stroke, fg, false)
	}
}

func (a *App) searchStatusMessage() string {
	if a.search.query == "" {
		return ""
	}
	if len(a.search.order) == 0 || a.search.current < 0 {
		return fmt.Sprintf("search /%s", a.search.query)
	}
	return fmt.Sprintf("match %d/%d /%s", a.search.current+1, len(a.search.order), a.search.query)
}

func (a *App) searchStatusCounter() string {
	if a.search.query == "" || len(a.search.order) == 0 || a.search.current < 0 {
		return ""
	}
	return fmt.Sprintf("[%d/%d]", a.search.current+1, len(a.search.order))
}
