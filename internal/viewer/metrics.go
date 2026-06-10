package viewer

import (
	"fmt"
	"sync"

	"gopdf/internal/mupdf"
)

type pageMetricUpdate struct {
	page   int
	bounds mupdf.Rect
	width  float64
	height float64
	err    error
}

type metricLoader struct {
	updates   chan pageMetricUpdate
	closing   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
}

type metricsService struct {
	pageMetrics  []pageMetrics
	loader       *metricLoader
	pendingPath  string
	pendingPages int
	pendingStart int
}

func (l *metricLoader) run(docPath string, pageCount int, startPage int) {
	defer close(l.done)
	doc, err := mupdf.Open(docPath)
	if err != nil {
		l.send(pageMetricUpdate{err: fmt.Errorf("load page metrics: %w", err)})
		return
	}
	defer doc.Close()
	loadPage := func(i int) bool {
		select {
		case <-l.closing:
			return false
		default:
		}
		bounds, err := doc.Bounds(i)
		if err != nil {
			return l.send(pageMetricUpdate{page: i, err: fmt.Errorf("load page %d metrics: %w", i+1, err)})
		}
		w, h := rotatedBoundsSize(bounds, 0)
		return l.send(pageMetricUpdate{page: i, bounds: bounds, width: w, height: h})
	}
	startPage = clampInt(startPage, 0, max(0, pageCount-1))
	for _, page := range metricPageOrder(pageCount, startPage) {
		if !loadPage(page) {
			return
		}
	}
}

func metricPageOrder(pageCount int, startPage int) []int {
	if pageCount <= 1 {
		return nil
	}
	startPage = clampInt(startPage, 0, pageCount-1)
	pages := make([]int, 0, pageCount-1)
	for distance := 1; len(pages) < pageCount-1; distance++ {
		forward := startPage + distance
		if forward < pageCount {
			pages = append(pages, forward)
		}
		backward := startPage - distance
		if backward >= 0 {
			pages = append(pages, backward)
		}
	}
	return pages
}

func (l *metricLoader) Close() {
	closeWorker(l.closing, l.done, &l.closeOnce)
}

func (l *metricLoader) send(update pageMetricUpdate) bool {
	select {
	case l.updates <- update:
		return true
	case <-l.closing:
		return false
	}
}
