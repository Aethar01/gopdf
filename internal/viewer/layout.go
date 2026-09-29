package viewer

import (
	"math"
	"slices"
	"sort"

	"gopdf/internal/mupdf"
)

func (a *App) updatePageMetricSizes() {
	for i := range a.pageMetrics {
		if a.pageMetrics[i].loaded {
			a.setPageBounds(&a.pageMetrics[i])
		}
	}
}

// toggleTrimMargins switches between laying pages out by their full size and
// by their content, loading content boxes the first time.
func (a *App) toggleTrimMargins() {
	a.setTrimMargins(!a.trimMargins)
	a.message = boolWord(a.trimMargins, "trim margins on", "trim margins off")
}

func (a *App) setTrimMargins(enabled bool) {
	if a.trimMargins == enabled {
		return
	}
	a.relayoutWithViewportAnchor(func() {
		a.trimMargins = enabled
		a.updatePageMetricSizes()
	})
	a.clearCache() // tile grids follow the page bounds
	if a.trimMargins && a.doc != nil && !a.contentBoxesLoaded() {
		if a.metricLoader != nil {
			a.metricLoader.Close()
		}
		a.pendingLoad = false
		pages := make([]int, 0, a.pageCount)
		pages = append(pages, a.page)
		a.initMetricLoader(append(pages, metricPageOrder(a.pageCount, a.page)...))
	}
}

func (a *App) contentBoxesLoaded() bool {
	for _, m := range a.pageMetrics {
		if !m.hasContent {
			return false
		}
	}
	return true
}

// spreads groups pages as they are read: in dual-page mode the cover alone
// when first_page_offset is set, then pairs; otherwise one page each.
func (a *App) spreads() [][]int {
	spreads := make([][]int, 0, a.pageCount)
	for page := 0; page < a.pageCount; {
		if !a.dualPage || a.firstPageOffset && page == 0 || page+1 >= a.pageCount {
			spreads = append(spreads, []int{page})
			page++
			continue
		}
		spreads = append(spreads, []int{page, page + 1})
		page += 2
	}
	return spreads
}

// baseRows groups pages into unscaled rows: a spread per row, or in the
// overview a grid of spreads, their pages edge to edge.
func (a *App) baseRows() []rowLayout {
	spreads := a.spreads()
	if a.overview != nil {
		// Every cell takes the widest cell's width so columns line up.
		slot := 0.0
		for _, spread := range spreads {
			slot = math.Max(slot, a.spreadWidth(spread))
		}
		columns := a.overviewColumns()
		rows := make([]rowLayout, 0, len(spreads)/columns+1)
		for i := 0; i < len(spreads); i += columns {
			rows = append(rows, a.baseRow(spreads[i:min(len(spreads), i+columns)], overviewGap, 0, slot))
		}
		return rows
	}
	rows := make([]rowLayout, len(spreads))
	for i, spread := range spreads {
		rows[i] = a.baseRow([][]int{spread}, 0, float64(a.horizontalGap()), 0)
	}
	return rows
}

func (a *App) spreadWidth(spread []int) float64 {
	width := 0.0
	for _, page := range spread {
		width += a.pageMetrics[page].width
	}
	return width
}

// baseRow lays cells of pages side by side, cellGap pixels apart, with
// pageGap pixels between the pages of a cell. With a slot width, each cell
// is padded to it: a lone dual-mode cover to the right, as a book's first
// page, another lone page to the left, and single-page cells centred.
func (a *App) baseRow(cells [][]int, cellGap, pageGap, slot float64) rowLayout {
	var row rowLayout
	trail := 0.0 // padding left over from the previous cell
	for c, cell := range cells {
		spare := math.Max(0, slot-a.spreadWidth(cell))
		lead := spare / 2
		if a.dualPage {
			lead = 0
			if a.firstPageOffset && cell[0] == 0 {
				lead = spare
			}
		}
		for i, page := range cell {
			gap, pad := pageGap, 0.0
			switch {
			case c == 0 && i == 0:
				gap, pad = 0, lead
			case i == 0:
				gap, pad = cellGap, trail+lead
			}
			m := a.pageMetrics[page]
			row.pages = append(row.pages, page)
			row.gapBefore = append(row.gapBefore, gap)
			row.padBefore = append(row.padBefore, pad)
			row.pageW = append(row.pageW, m.width)
			row.pageH = append(row.pageH, m.height)
			row.width += pad + m.width
			row.gaps += gap
			row.height = math.Max(row.height, m.height)
		}
		trail = spare - lead
	}
	row.width += trail
	row.pageX = make([]float64, len(row.pages))
	row.pageY = make([]float64, len(row.pages))
	return row
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
	a.scale = a.currentScaleFromRows(viewportW, viewportH, base)
	a.rows = make([]rowLayout, len(base))
	a.pageToRow = make([]int, a.pageCount)
	maxRowWidth := 0.0
	for _, row := range base {
		maxRowWidth = math.Max(maxRowWidth, row.width*a.scale+row.gaps)
	}
	a.contentW = maxRowWidth + float64(a.horizontalGap()*2)
	y := float64(a.verticalGap())
	for i, row := range base {
		row.width = row.width*a.scale + row.gaps
		row.height *= a.scale
		row.x = float64(a.horizontalGap()) + (maxRowWidth-row.width)/2
		if a.overview != nil {
			row.x = float64(a.horizontalGap()) // a grid: a short last row starts at the left
		}
		row.y = y
		x := row.x
		for j, page := range row.pages {
			pw := row.pageW[j] * a.scale
			ph := row.pageH[j] * a.scale
			x += row.gapBefore[j] + row.padBefore[j]*a.scale
			row.pageW[j] = pw
			row.pageH[j] = ph
			row.pageX[j] = x
			row.pageY[j] = y + (row.height-ph)/2
			x += pw
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
	a.page = a.rows[a.viewportAnchorRowIndex()].pages[0]
}

func (a *App) rowIndexForPage(page int) int {
	if len(a.rows) == 0 {
		return 0
	}
	if page < 0 || page >= len(a.pageToRow) {
		return 0
	}
	return clampInt(a.pageToRow[page], 0, len(a.rows)-1)
}

func (a *App) viewportAnchorRowIndex() int {
	if len(a.rows) == 0 {
		return 0
	}
	if a.renderMode == "single" {
		return a.rowIndexForPage(a.page)
	}
	_, viewportH := a.viewportSize()
	maxY := math.Max(0, a.contentH-float64(viewportH))
	if maxY > 0 {
		if a.scrollY <= 0 {
			return 0
		}
		if a.scrollY >= maxY {
			return len(a.rows) - 1
		}
	}
	_, offsetY := a.contentViewportOffset()
	return a.rowIndexAtContentY(a.scrollY + a.viewportAnchorScreenY() - offsetY)
}

func (a *App) rowIndexAtContentY(marker float64) int {
	if len(a.rows) == 0 {
		return 0
	}
	if marker < 0 {
		marker = 0
	}
	index := sort.Search(len(a.rows), func(i int) bool { return a.rows[i].y > marker }) - 1
	return clampInt(index, 0, len(a.rows)-1)
}

func (a *App) rowRangeForContentY(minY, maxY float64) (int, int) {
	if len(a.rows) == 0 {
		return 0, 0
	}
	start := sort.Search(len(a.rows), func(i int) bool { return a.rows[i].y+a.rows[i].height >= minY })
	end := sort.Search(len(a.rows), func(i int) bool { return a.rows[i].y > maxY })
	if start > end {
		start = end
	}
	return start, end
}

func (a *App) forEachContinuousPage(minY, maxY float64, visit func(page int, x, y, width, height float64)) {
	start, end := a.rowRangeForContentY(minY, maxY)
	for _, row := range a.rows[start:end] {
		if row.y+row.height < minY || row.y > maxY {
			continue
		}
		for i, page := range row.pages {
			x, y := a.rowPageScreenOrigin(row, i)
			visit(page, x, y, row.pageW[i], row.pageH[i])
		}
	}
}

func (a *App) verticalGap() int {
	if a.overview != nil {
		return overviewGap
	}
	if a.config.PageGapVertical >= 0 {
		return a.config.PageGapVertical
	}
	if a.config.PageGap >= 0 {
		return a.config.PageGap
	}
	return 0
}

func (a *App) horizontalGap() int {
	if a.overview != nil {
		return overviewGap
	}
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
