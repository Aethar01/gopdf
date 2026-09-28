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
	return pageMetrics{bounds: info.Bounds, full: info.Bounds, width: w, height: h, label: info.Label, loaded: true}
}

// trimPadding is the margin, in points, kept around trimmed content.
const trimPadding = 8

// setPageBounds picks a page's layout bounds, its padded content box while
// trimming margins or else the full page, and sizes it for the rotation.
func (a *App) setPageBounds(m *pageMetrics) {
	m.bounds = m.full
	if a.trimMargins && m.hasContent && !m.content.Empty() {
		m.bounds = mupdf.Rect{
			X0: max(m.full.X0, m.content.X0-trimPadding),
			Y0: max(m.full.Y0, m.content.Y0-trimPadding),
			X1: min(m.full.X1, m.content.X1+trimPadding),
			Y1: min(m.full.Y1, m.content.Y1+trimPadding),
		}
	}
	m.width, m.height = rotatedBoundsSize(m.bounds, a.rotation)
}

// loadPageMetrics reads a page's metrics, with its content box when asked.
func loadPageMetrics(doc *mupdf.Document, page int, withContent bool) (pageMetrics, error) {
	info, err := doc.PageInfo(page)
	if err != nil {
		return pageMetrics{}, err
	}
	m := newPageMetrics(info)
	if withContent {
		if m.content, err = doc.ContentBounds(page); err != nil {
			return pageMetrics{}, err
		}
		m.hasContent = true
	}
	return m, nil
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

func (l *metricLoader) run(doc *mupdf.Document, pages []int, withContent bool) {
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
		m, err := loadPageMetrics(doc, i, withContent)
		if err != nil {
			return sendWorkerUpdate(&l.workerLifecycle, l.updates, pageMetricUpdate{page: i, err: fmt.Errorf("load page %d metrics: %w", i+1, err)})
		}
		return sendWorkerUpdate(&l.workerLifecycle, l.updates, pageMetricUpdate{page: i, metrics: m})
	}
	for _, page := range pages {
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
