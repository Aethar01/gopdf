package viewer

import (
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
			return true
		}
		w, h := rotatedBoundsSize(bounds, 0)
		select {
		case l.updates <- pageMetricUpdate{page: i, bounds: bounds, width: w, height: h}:
		case <-l.closing:
			return false
		}
		return true
	}
	startPage = clampInt(startPage, 0, max(0, pageCount-1))
	for i := startPage + 1; i < pageCount; i++ {
		if !loadPage(i) {
			return
		}
	}
	for i := 0; i < startPage; i++ {
		if !loadPage(i) {
			return
		}
	}
}

func (l *metricLoader) Close() {
	l.closeOnce.Do(func() { close(l.closing) })
	<-l.done
}
