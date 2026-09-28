package viewer

import (
	"fmt"

	"gopdf/internal/mupdf"
)

type pageMetricUpdate struct {
	page    int
	metrics pageMetrics
	err     error
}

func newPageMetrics(info mupdf.PageInfo) pageMetrics {
	w, h := rotatedBoundsSize(info.Bounds, 0)
	return pageMetrics{bounds: info.Bounds, width: w, height: h, label: info.Label, loaded: true}
}

type metricLoader struct {
	workerLifecycle
	updates chan pageMetricUpdate
}

type metricsService struct {
	pageMetrics  []pageMetrics
	pendingLoad  bool
	pendingPages int
	pendingStart int
}

func (l *metricLoader) run(doc *mupdf.Document, pageCount int, startPage int) {
	defer close(l.done)
	if doc == nil {
		sendWorkerUpdate(&l.workerLifecycle, l.updates, pageMetricUpdate{err: fmt.Errorf("load page metrics: no document open")})
		return
	}
	loadPage := func(i int) bool {
		select {
		case <-l.closing:
			return false
		default:
		}
		info, err := doc.PageInfo(i)
		if err != nil {
			return sendWorkerUpdate(&l.workerLifecycle, l.updates, pageMetricUpdate{page: i, err: fmt.Errorf("load page %d metrics: %w", i+1, err)})
		}
		return sendWorkerUpdate(&l.workerLifecycle, l.updates, pageMetricUpdate{page: i, metrics: newPageMetrics(info)})
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
