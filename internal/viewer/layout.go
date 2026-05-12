package viewer

import (
	"math"
	"slices"

	"gopdf/internal/mupdf"
)

func (a *App) loadPageMetrics() error {
	metrics, err := pageMetricsForDocument(a.doc, a.pageCount, a.rotation)
	if err != nil {
		return err
	}
	a.pageMetrics = metrics
	return nil
}

func pageMetricsForDocument(doc *mupdf.Document, pageCount int, rotation float64) ([]pageMetrics, error) {
	metrics := make([]pageMetrics, pageCount)
	for i := 0; i < pageCount; i++ {
		bounds, err := doc.Bounds(i)
		if err != nil {
			return nil, err
		}
		w, h := rotatedBoundsSize(bounds, rotation)
		metrics[i] = pageMetrics{bounds: bounds, width: w, height: h}
	}
	return metrics, nil
}

func (a *App) updatePageMetricSizes() {
	for i := range a.pageMetrics {
		a.pageMetrics[i].width, a.pageMetrics[i].height = rotatedBoundsSize(a.pageMetrics[i].bounds, a.rotation)
	}
}

func (a *App) baseRows() []rowLayout {
	rows := make([]rowLayout, 0, a.pageCount)
	appendRow := func(pages ...int) {
		row := rowLayout{pages: append([]int(nil), pages...), pageX: make([]float64, len(pages)), pageY: make([]float64, len(pages)), pageW: make([]float64, len(pages)), pageH: make([]float64, len(pages))}
		for i, page := range pages {
			m := a.pageMetrics[page]
			row.pageW[i] = m.width
			row.pageH[i] = m.height
			row.width += m.width
			if i > 0 {
				row.width += float64(a.horizontalGap())
			}
			if m.height > row.height {
				row.height = m.height
			}
		}
		rows = append(rows, row)
	}
	if !a.dualPage {
		for page := 0; page < a.pageCount; page++ {
			appendRow(page)
		}
		return rows
	}
	for page := 0; page < a.pageCount; {
		if a.firstPageOffset && page == 0 {
			appendRow(page)
			page++
			continue
		}
		if page+1 < a.pageCount {
			appendRow(page, page+1)
			page += 2
			continue
		}
		appendRow(page)
		page++
	}
	return rows
}

func (a *App) baseRowIndexForPage(page int, rows []rowLayout) int {
	index := 0
	for i, row := range rows {
		if slices.Contains(row.pages, page) {
			return i
		}
		index = i
	}
	return index
}

func (a *App) recomputeLayout(viewportW, viewportH int) {
	if len(a.pageMetrics) == 0 {
		return
	}
	base := a.baseRows()
	a.scale = a.currentScale(viewportW, viewportH)
	a.rows = make([]rowLayout, len(base))
	a.pageToRow = make([]int, a.pageCount)
	maxRowWidth := 0.0
	for _, row := range base {
		if row.width > maxRowWidth {
			maxRowWidth = row.width
		}
	}
	a.contentW = maxRowWidth*a.scale + float64(a.horizontalGap()*2)
	y := float64(a.verticalGap())
	for i, row := range base {
		row.width *= a.scale
		row.height *= a.scale
		row.x = float64(a.horizontalGap()) + (maxRowWidth*a.scale-row.width)/2
		row.y = y
		x := row.x
		for j, page := range row.pages {
			pw := row.pageW[j] * a.scale
			ph := row.pageH[j] * a.scale
			row.pageW[j] = pw
			row.pageH[j] = ph
			row.pageX[j] = x
			row.pageY[j] = y + (row.height-ph)/2
			x += pw + float64(a.horizontalGap())
			a.pageToRow[page] = i
		}
		a.rows[i] = row
		y += row.height + float64(a.verticalGap())
	}
	a.contentH = y
	if a.renderMode == "single" && len(a.rows) > 0 {
		row := a.rows[clampInt(a.pageToRow[a.page], 0, len(a.rows)-1)]
		a.contentW = row.width + float64(a.horizontalGap()*2)
		a.contentH = row.height + float64(a.verticalGap()*2)
	}
	a.clampScroll()
	if a.renderMode == "continuous" {
		a.updateCurrentPageFromScroll()
	}
}

func (a *App) clampScroll() {
	viewportW, viewportH := a.viewportSize()
	maxX := math.Max(0, a.contentW-float64(viewportW))
	maxY := math.Max(0, a.contentH-float64(viewportH))
	a.scrollX = clampFloat(a.scrollX, 0, maxX)
	a.scrollY = clampFloat(a.scrollY, 0, maxY)
}

func (a *App) updateCurrentPageFromScroll() {
	if len(a.rows) == 0 {
		return
	}
	a.page = a.rows[a.currentRowIndex()].pages[0]
}

func (a *App) currentRowIndex() int {
	if len(a.rows) == 0 {
		return 0
	}
	if a.renderMode == "single" {
		if a.page < 0 || a.page >= len(a.pageToRow) {
			return 0
		}
		return clampInt(a.pageToRow[a.page], 0, len(a.rows)-1)
	}
	_, offsetY := a.contentViewportOffset()
	marker := a.scrollY - offsetY + math.Max(1, float64(a.verticalGap())/2+1)
	if marker < 0 {
		marker = 0
	}
	current := 0
	for i, row := range a.rows {
		if row.y <= marker {
			current = i
			continue
		}
		break
	}
	return current
}

func (a *App) verticalGap() int {
	if a.config.PageGapVertical >= 0 {
		return a.config.PageGapVertical
	}
	if a.config.PageGap >= 0 {
		return a.config.PageGap
	}
	return 0
}

func (a *App) horizontalGap() int {
	if a.config.PageGapHorizontal >= 0 {
		return a.config.PageGapHorizontal
	}
	if a.config.SpreadGap >= 0 {
		return a.config.SpreadGap
	}
	return 0
}

func normalizeRotation(rotation float64) float64 {
	rotation = math.Mod(rotation, 360)
	if rotation < 0 {
		rotation += 360
	}
	return rotation
}

func rotatedBoundsSize(bounds mupdf.Rect, rotation float64) (float64, float64) {
	minX, minY, maxX, maxY := rotatedBounds(bounds, rotation)
	return maxX - minX, maxY - minY
}

func rotatedBoundsOrigin(bounds mupdf.Rect, scale, rotation float64) (float64, float64) {
	minX, minY, _, _ := rotatedBounds(bounds, rotation)
	return minX * scale, minY * scale
}

func rotatedBounds(bounds mupdf.Rect, rotation float64) (float64, float64, float64, float64) {
	points := []mupdf.Point{
		{X: float64(bounds.X0), Y: float64(bounds.Y0)},
		{X: float64(bounds.X1), Y: float64(bounds.Y0)},
		{X: float64(bounds.X0), Y: float64(bounds.Y1)},
		{X: float64(bounds.X1), Y: float64(bounds.Y1)},
	}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	for _, point := range points {
		x, y := transformPoint(point.X, point.Y, 1, rotation)
		minX = math.Min(minX, x)
		minY = math.Min(minY, y)
		maxX = math.Max(maxX, x)
		maxY = math.Max(maxY, y)
	}
	return minX, minY, maxX, maxY
}

func transformPoint(x, y, scale, rotation float64) (float64, float64) {
	x *= scale
	y *= scale
	radians := normalizeRotation(rotation) * math.Pi / 180
	sin, cos := math.Sin(radians), math.Cos(radians)
	return x*cos - y*sin, x*sin + y*cos
}

func inverseTransformPoint(x, y, scale, rotation float64) (float64, float64) {
	if scale == 0 {
		return x, y
	}
	radians := normalizeRotation(rotation) * math.Pi / 180
	sin, cos := math.Sin(radians), math.Cos(radians)
	return (x*cos + y*sin) / scale, (-x*sin + y*cos) / scale
}
