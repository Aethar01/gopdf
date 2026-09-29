package viewer

import (
	"math"

	"gopdf/internal/config"
)

func (a *App) currentScaleFromRows(viewportW, viewportH int, baseRows []rowLayout) float64 {
	if a.fitMode == fitManual {
		return a.zoom
	}
	if a.renderMode == renderSingle && len(baseRows) > 0 && a.page >= 0 {
		row := baseRows[clampInt(a.baseRowIndexForPage(a.page, baseRows), 0, len(baseRows)-1)]
		return a.fitScale(viewportW, viewportH, []rowLayout{row})
	}
	return a.fitScale(viewportW, viewportH, baseRows)
}

// fitScale is the scale at which base rows fit the viewport under the
// current fit mode. Gaps between pages keep their size, so each row fits
// when its pages' scaled width plus its gaps does.
func (a *App) fitScale(viewportW, viewportH int, rows []rowLayout) float64 {
	available := float64(viewportW) - float64(a.horizontalGap()*2)
	widthScale, maxHeight := math.Inf(1), 1.0
	for _, row := range rows {
		widthScale = math.Min(widthScale, (available-row.gaps)/math.Max(1, row.width))
		maxHeight = math.Max(maxHeight, row.height)
	}
	if math.IsInf(widthScale, 1) {
		widthScale = available
	}
	heightScale := (float64(viewportH) - float64(a.verticalGap()*2)) / maxHeight
	switch a.fitMode {
	case fitWidth:
		return math.Max(0.05, widthScale)
	case fitHeight:
		return math.Max(0.05, heightScale)
	default:
		return math.Max(0.05, math.Min(widthScale, heightScale))
	}
}

func (a *App) nextPage() {
	if a.pageCount == 0 {
		return
	}
	if a.dualPage {
		a.nextSpread()
		return
	}
	page := clampInt(a.page, 0, a.pageCount-1)
	if page < a.pageCount-1 {
		a.alignPageToAnchor(page + 1)
	} else {
		a.alignPageToViewportEdge(page, true)
	}
}

func (a *App) prevPage() {
	if a.pageCount == 0 {
		return
	}
	if a.dualPage {
		a.prevSpread()
		return
	}
	page := clampInt(a.page, 0, a.pageCount-1)
	if page > 0 {
		a.alignPageToAnchor(page - 1)
	} else {
		a.alignPageToViewportEdge(page, false)
	}
}

func (a *App) nextSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.rowIndexForPage(a.anchorPage(a.page))
	if row < len(a.rows)-1 {
		a.alignPageToAnchor(a.rows[row+1].pages[0])
	} else {
		a.alignPageToViewportEdge(a.pageCount-1, true)
	}
}

func (a *App) prevSpread() {
	if a.pageCount == 0 {
		return
	}
	row := a.rowIndexForPage(a.anchorPage(a.page))
	if row > 0 {
		a.alignPageToAnchor(a.rows[row-1].pages[0])
	} else {
		a.alignPageToViewportEdge(0, false)
	}
}

func (a *App) scrollBy(dx, dy float64) {
	oldX, oldY := a.scrollX, a.scrollY
	a.scrollX += dx
	a.scrollY += dy
	a.clampScroll()
	if a.renderMode == renderSingle || (a.scrollX == oldX && a.scrollY == oldY) {
		return
	}
	a.updateCurrentPageFromScroll()
}

func (a *App) scrollByInput(dx, dy float64) {
	if !a.smoothScrollInputEnabled(a.currentSmoothInputSource()) {
		a.cancelSmoothScroll()
		a.scrollBy(dx, dy)
		return
	}
	a.queueSmoothScroll(dx, dy)
}

func (a *App) alignPageToAnchor(page int) {
	if page < 0 || page >= len(a.pageToRow) {
		return
	}
	page = a.anchorPage(page)
	if a.positionMatchesPageAnchor(page) {
		return
	}
	a.recordJump()
	if a.renderMode == renderSingle {
		a.page = page
		a.scrollX = 0
		a.scrollY = 0
		a.recomputeLayout(a.viewportSize())
		a.clampScroll()
		return
	}
	row := a.rows[a.pageToRow[page]]
	a.scrollY = a.scrollYForAnchoredRow(row)
	a.page = page
	a.clampScroll()
}

func (a *App) alignPageToViewportEdge(page int, bottom bool) {
	if page < 0 || page >= len(a.pageToRow) || len(a.rows) == 0 {
		return
	}
	row := a.rows[a.pageToRow[page]]
	for i, rowPage := range row.pages {
		if rowPage != page {
			continue
		}
		_, y := a.rowPageScreenOrigin(row, i)
		targetY := 0.0
		if bottom {
			_, viewportH := a.viewportSize()
			targetY = float64(viewportH) - row.pageH[i]
		}
		a.scrollY += y - targetY
		a.clampScroll()
		a.page = a.anchorPage(page)
		return
	}
}

// jumpToDestination navigates to a link or outline target. Missing
// coordinates default to the horizontal centre and top edge of the page.
func (a *App) jumpToDestination(page int, x, y float64, hasX, hasY bool) {
	if page < 0 || page >= len(a.pageMetrics) {
		return
	}
	if !hasX && !hasY {
		a.alignPageToAnchor(page)
		return
	}
	bounds := a.pageMetrics[page].bounds
	if !hasX {
		x = float64(bounds.X0+bounds.X1) / 2
	}
	if !hasY {
		y = float64(bounds.Y0)
	}
	a.alignPageToDocumentPoint(page, x, y)
}

func (a *App) alignPageToDocumentPoint(page int, x, y float64) {
	if page < 0 || page >= len(a.pageToRow) {
		return
	}
	page = a.anchorPage(page)
	a.recordJump()
	if a.renderMode == renderSingle {
		a.page = page
		a.recomputeLayout(a.viewportSize())
	}
	row := a.rows[a.pageToRow[page]]
	pageIndex := 0
	for i, rowPage := range row.pages {
		if rowPage == page {
			pageIndex = i
			break
		}
	}
	dx, dy := a.pageTransform(page).toScreen(x, y)
	viewportW, viewportH := a.viewportSize()
	a.scrollX = row.pageX[pageIndex] + dx - float64(viewportW)/2
	a.scrollY = row.pageY[pageIndex] + dy - float64(viewportH)/4
	a.page = page
	a.clampScroll()
}

func (a *App) recordJump() {
	pos := a.currentJumpPosition()
	if len(a.jumpBack) == 0 || a.jumpBack[len(a.jumpBack)-1] != pos {
		a.jumpBack = append(a.jumpBack, pos)
	}
	a.jumpAhead = nil
}

func (a *App) jumpForward() {
	if len(a.jumpAhead) == 0 {
		return
	}
	current := a.currentJumpPosition()
	jump := a.jumpAhead[len(a.jumpAhead)-1]
	a.jumpAhead = a.jumpAhead[:len(a.jumpAhead)-1]
	if len(a.jumpBack) == 0 || a.jumpBack[len(a.jumpBack)-1] != current {
		a.jumpBack = append(a.jumpBack, current)
	}
	a.restoreJump(jump)
}

func (a *App) jumpBackward() {
	if len(a.jumpBack) == 0 {
		return
	}
	current := a.currentJumpPosition()
	jump := a.jumpBack[len(a.jumpBack)-1]
	a.jumpBack = a.jumpBack[:len(a.jumpBack)-1]
	if len(a.jumpAhead) == 0 || a.jumpAhead[len(a.jumpAhead)-1] != current {
		a.jumpAhead = append(a.jumpAhead, current)
	}
	a.restoreJump(jump)
}

func (a *App) currentJumpPosition() jumpPosition {
	return jumpPosition{
		page:    a.page,
		scrollX: a.scrollX,
		scrollY: a.scrollY,
	}
}

func (a *App) restoreJump(jump jumpPosition) {
	a.page = jump.page
	a.scrollX = jump.scrollX
	a.scrollY = jump.scrollY
	a.recomputeLayout(a.viewportSize())
	a.clampScroll()
}

func (a *App) positionMatchesPageAnchor(page int) bool {
	if a.renderMode == renderSingle {
		return a.page == page && a.scrollX == 0 && a.scrollY == 0
	}
	row := a.rows[a.pageToRow[page]]
	expectedY := a.clampedScrollY(a.scrollYForAnchoredRow(row))
	return a.page == page && a.scrollY == expectedY
}

func (a *App) scrollYForAnchoredRow(row rowLayout) float64 {
	_, viewportH := a.viewportSize()
	switch a.config.AnchorPosition {
	case "top":
		return row.y
	case "bottom":
		return row.y + row.height - float64(viewportH)
	default:
		return row.y + row.height/2 - float64(viewportH)/2
	}
}

func (a *App) clampedScrollY(scrollY float64) float64 {
	_, viewportH := a.viewportSize()
	maxY := math.Max(0, a.contentH-float64(viewportH))
	return clampFloat(scrollY, 0, maxY)
}

func (a *App) anchorPage(page int) int {
	if page < 0 || page >= len(a.pageToRow) {
		return page
	}
	if !a.dualPage || len(a.rows) == 0 {
		return page
	}
	row := a.rows[a.pageToRow[page]]
	if len(row.pages) == 0 {
		return page
	}
	return row.pages[0]
}

func (a *App) clampZoom(zoom float64) float64 {
	minZoom := a.config.MinZoom
	if minZoom <= 0 {
		minZoom = config.Default().MinZoom
	}
	maxZoom := a.config.MaxZoom
	if maxZoom < minZoom {
		maxZoom = minZoom
	}
	return clampFloat(zoom, minZoom, maxZoom)
}

func (a *App) setFitMode(mode fitMode) {
	a.cancelSmoothZoom()
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = mode
		a.scheduleRenderScaleTarget(a.zoom)
	})
}

func (a *App) setAltColors(enabled bool) {
	if a.altColors == enabled {
		return
	}
	a.altColors = enabled
	a.restyleTiles()
}
