package viewer

import (
	"fmt"
	"image/color"
	"math"
	"os/exec"
	"runtime"
	"strings"

	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type pageHit struct {
	page      int
	x         float64
	y         float64
	width     float64
	height    float64
	drawScale float64
	render    *renderedPage
}

func (a *App) captureViewportAnchor() viewportAnchor {
	screenX, screenY := a.viewportAnchorScreenPoint()
	page, x, y, ok := a.pageAtAnchorScreenPoint(screenX, screenY)
	if !ok {
		return viewportAnchor{}
	}
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[page].bounds, a.scale, a.rotation)
	pageX, pageY := inverseTransformPoint(x+originX, y+originY, a.scale, a.rotation)
	return viewportAnchor{page: page, point: mupdf.Point{X: pageX, Y: pageY}, valid: true}
}

func (a *App) restoreViewportAnchor(anchor viewportAnchor) {
	if !anchor.valid || anchor.page < 0 || anchor.page >= len(a.pageToRow) || len(a.rows) == 0 {
		a.clampScroll()
		return
	}
	pageX, pageY, ok := a.pageScreenOrigin(anchor.page)
	if !ok {
		a.clampScroll()
		return
	}
	tx, ty := transformPoint(anchor.point.X, anchor.point.Y, a.scale, a.rotation)
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[anchor.page].bounds, a.scale, a.rotation)
	targetX, targetY := a.viewportAnchorScreenPoint()
	a.scrollX += pageX + tx - originX - targetX
	a.scrollY += pageY + ty - originY - targetY
	a.clampScroll()
	if a.renderMode == "continuous" {
		pageX, pageY, ok = a.pageScreenOrigin(anchor.page)
		if ok && (math.Abs(pageX+tx-originX-targetX) > 0.5 || math.Abs(pageY+ty-originY-targetY) > 0.5) {
			a.page = anchor.page
		} else {
			a.updateCurrentPageFromScroll()
		}
	}
}

func (a *App) relayoutWithViewportAnchor(update func()) {
	fallbackPage := a.page
	anchor := a.captureViewportAnchor()
	if update != nil {
		update()
	}
	a.recomputeLayout(a.viewportSize())
	if !anchor.valid {
		a.page = clampInt(fallbackPage, 0, a.pageCount-1)
		a.clampScroll()
		return
	}
	a.restoreViewportAnchor(anchor)
}

func (a *App) viewportAnchorScreenPoint() (float64, float64) {
	viewportW, _ := a.viewportSize()
	return float64(viewportW) / 2, a.viewportAnchorScreenY()
}

func (a *App) viewportAnchorScreenY() float64 {
	_, viewportH := a.viewportSize()
	switch a.config.AnchorPosition {
	case "top":
		return 0
	case "bottom":
		return float64(viewportH)
	default:
		return float64(viewportH) / 2
	}
}

func (a *App) pageAtAnchorScreenPoint(screenX, screenY float64) (int, float64, float64, bool) {
	rowIndex := a.anchorRowIndex()
	if rowIndex < 0 || rowIndex >= len(a.rows) {
		return 0, 0, 0, false
	}
	row := a.rows[rowIndex]
	for i, page := range row.pages {
		x, y := a.rowPageScreenOrigin(row, i)
		if screenX >= x && screenX <= x+row.pageW[i] && screenY >= y && screenY <= y+row.pageH[i] {
			return page, screenX - x, screenY - y, true
		}
	}
	if len(row.pages) == 0 {
		return 0, 0, 0, false
	}
	pageIndex := 0
	for i := range row.pages {
		x, _ := a.rowPageScreenOrigin(row, i)
		bestX, _ := a.rowPageScreenOrigin(row, pageIndex)
		if math.Abs(screenX-(x+row.pageW[i]/2)) < math.Abs(screenX-(bestX+row.pageW[pageIndex]/2)) {
			pageIndex = i
		}
	}
	x, y := a.rowPageScreenOrigin(row, pageIndex)
	return row.pages[pageIndex], screenX - x, screenY - y, true
}

func (a *App) pageScreenOrigin(page int) (float64, float64, bool) {
	if page < 0 || page >= len(a.pageToRow) || len(a.rows) == 0 {
		return 0, 0, false
	}
	row := a.rows[a.pageToRow[page]]
	for i, candidate := range row.pages {
		if candidate == page {
			x, y := a.rowPageScreenOrigin(row, i)
			return x, y, true
		}
	}
	return 0, 0, false
}

func (a *App) rowPageScreenOrigin(row rowLayout, pageIndex int) (float64, float64) {
	if a.renderMode == "single" {
		viewportW, viewportH := a.viewportSize()
		baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
		baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
		return baseX + (row.pageX[pageIndex] - row.x) - a.scrollX, baseY + (row.pageY[pageIndex] - row.y) - a.scrollY
	}
	offsetX, offsetY := a.contentViewportOffset()
	return row.pageX[pageIndex] - a.scrollX + offsetX, row.pageY[pageIndex] - a.scrollY + offsetY
}

func (a *App) viewportSize() (int, int) {
	h := a.winH
	if a.statusVisible() {
		h -= a.config.StatusBarHeight
	}
	if h < 1 {
		h = 1
	}
	w := max(a.winW, 1)
	return w, h
}

func (a *App) contentViewportOffset() (float64, float64) {
	viewportW, viewportH := a.viewportSize()
	offsetX := math.Max(0, (float64(viewportW)-a.contentW)/2)
	offsetY := math.Max(0, (float64(viewportH)-a.contentH)/2)
	if a.renderMode == "continuous" {
		offsetY = 0
	}
	return offsetX, offsetY
}

func (a *App) renderMargin() float64 {
	_, viewportH := a.viewportSize()
	return math.Max(float64(viewportH)/2, a.pageStep*2)
}

func (a *App) pagePointAtScreen(sx, sy float64) (int, mupdf.Point, bool) {
	for _, hit := range a.visiblePageHits() {
		if sx < hit.x || sy < hit.y || sx > hit.x+hit.width || sy > hit.y+hit.height {
			continue
		}
		originX, originY := rotatedBoundsOrigin(a.pageMetrics[hit.page].bounds, a.scale, a.rotation)
		transformedX := sx - hit.x + originX
		transformedY := sy - hit.y + originY
		pageX, pageY := inverseTransformPoint(transformedX, transformedY, a.scale, a.rotation)
		return hit.page, mupdf.Point{X: pageX, Y: pageY}, true
	}
	return 0, mupdf.Point{}, false
}

func (a *App) visiblePageHits() []pageHit {
	hits := []pageHit{}
	if a.renderMode == "single" {
		if len(a.rows) == 0 || a.page < 0 || a.page >= len(a.pageToRow) {
			return hits
		}
		viewportW, viewportH := a.viewportSize()
		row := a.rows[a.pageToRow[a.page]]
		baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
		baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
		for i, page := range row.pages {
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				a.requestRender(page, a.scale)
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			x := baseX + (row.pageX[i] - row.x) - a.scrollX
			y := baseY + (row.pageY[i] - row.y) - a.scrollY
			hits = append(hits, pageHit{page: page, x: x, y: y, width: row.pageW[i], height: row.pageH[i], drawScale: drawScale, render: rp})
		}
		return hits
	}
	_, viewportH := a.viewportSize()
	margin := a.renderMargin()
	minY := a.scrollY - margin
	maxY := a.scrollY + float64(viewportH) + margin
	offsetX, offsetY := a.contentViewportOffset()
	start, end := a.rowRangeForContentY(minY, maxY)
	for _, row := range a.rows[start:end] {
		if row.y+row.height < minY || row.y > maxY {
			continue
		}
		for i, page := range row.pages {
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				a.requestRender(page, a.scale)
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			hits = append(hits, pageHit{page: page, x: x, y: y, width: row.pageW[i], height: row.pageH[i], drawScale: drawScale, render: rp})
		}
	}
	return hits
}

func (a *App) refreshSelection() {
	sel, err := a.doc.ExtractSelection(a.selection.page, a.selection.anchor, a.selection.focus)
	if err != nil {
		a.message = err.Error()
		return
	}
	a.selection.text = sel.Text
	a.selection.quads = sel.Quads
}

func (a *App) copySelectionToClipboard() {
	if strings.TrimSpace(a.selection.text) == "" {
		return
	}
	if err := setSDLClipboardText(a.selection.text); err != nil {
		a.message = "clipboard unavailable"
		return
	}
	a.message = fmt.Sprintf("copied %d chars", len(a.selection.text))
	a.selection.text = ""
}

func (a *App) tryActivateLinkAt(sx, sy float64) bool {
	page, point, ok := a.pagePointAtScreen(sx, sy)
	if !ok {
		return false
	}
	links, err := a.linksForPage(page)
	if err != nil {
		a.message = err.Error()
		return false
	}
	for _, link := range links {
		if point.X < float64(link.Bounds.X0) || point.X > float64(link.Bounds.X1) || point.Y < float64(link.Bounds.Y0) || point.Y > float64(link.Bounds.Y1) {
			continue
		}
		a.activateLink(link)
		return true
	}
	return false
}

func (a *App) isLinkAt(sx, sy float64) bool {
	page, point, ok := a.pagePointAtScreen(sx, sy)
	if !ok {
		return false
	}
	links, err := a.linksForPage(page)
	if err != nil {
		return false
	}
	for _, link := range links {
		if point.X < float64(link.Bounds.X0) || point.X > float64(link.Bounds.X1) || point.Y < float64(link.Bounds.Y0) || point.Y > float64(link.Bounds.Y1) {
			continue
		}
		return true
	}
	return false
}

func (a *App) linksForPage(page int) ([]mupdf.Link, error) {
	if links, ok := a.pageLinks[page]; ok {
		return links, nil
	}
	links, err := a.doc.Links(page)
	if err != nil {
		return nil, err
	}
	a.pageLinks[page] = links
	return links, nil
}

func (a *App) activateLink(link mupdf.Link) {
	if link.External {
		if link.URI == "" {
			return
		}
		if err := openExternalURL(link.URI); err != nil {
			a.message = err.Error()
			return
		}
		a.message = link.URI
		return
	}
	if link.Page >= 0 {
		a.alignPageTop(link.Page)
		return
	}
	if link.URI != "" {
		a.message = link.URI
	}
}

func openExternalURL(uri string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", uri)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", uri)
	default:
		cmd = exec.Command("xdg-open", uri)
	}
	return cmd.Start()
}

func (a *App) drawSelection(renderer *sdl.Renderer) {
	if len(a.selection.quads) == 0 {
		return
	}
	x, y, rp, ok := a.pagePlacement(a.selection.page)
	if !ok {
		return
	}
	a.drawHighlightQuads(renderer, a.selection.quads, x, y, rp)
}

func (a *App) drawHighlightQuads(renderer *sdl.Renderer, quads []mupdf.Quad, x, y float64, rp *renderedPage) {
	a.drawHighlightQuadsWithStyle(renderer, quads, x, y, rp, false)
}

func (a *App) highlightForegroundColor() color.RGBA {
	return rgb(a.config.HighlightForeground)
}

func (a *App) highlightBackgroundColor() color.RGBA {
	bg := rgb(a.config.HighlightBackground)
	bg.A = 0xaa
	return bg
}

func (a *App) pagePlacement(page int) (float64, float64, *renderedPage, bool) {
	if page < 0 || page >= len(a.pageToRow) || len(a.rows) == 0 {
		return 0, 0, nil, false
	}
	row := a.rows[a.pageToRow[page]]
	index := -1
	for i, candidate := range row.pages {
		if candidate == page {
			index = i
			break
		}
	}
	if index < 0 {
		return 0, 0, nil, false
	}
	rp, ok := a.cachedRenderPage(page, a.scale)
	if !ok {
		a.requestRender(page, a.scale)
		return 0, 0, nil, false
	}
	if a.renderMode == "single" {
		viewportW, viewportH := a.viewportSize()
		baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
		baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
		return baseX + (row.pageX[index] - row.x) - a.scrollX, baseY + (row.pageY[index] - row.y) - a.scrollY, rp, true
	}
	offsetX, offsetY := a.contentViewportOffset()
	return row.pageX[index] - a.scrollX + offsetX, row.pageY[index] - a.scrollY + offsetY, rp, true
}

func (a *App) quadScreenBounds(quad mupdf.Quad, x, y float64, rp *renderedPage) (float64, float64, float64, float64) {
	pts := []mupdf.Point{quad.UL, quad.UR, quad.LL, quad.LR}
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64
	originX, originY := rotatedBoundsOrigin(a.pageMetrics[rp.page].bounds, a.scale, a.rotation)
	for _, pt := range pts {
		tx, ty := transformPoint(pt.X, pt.Y, a.scale, a.rotation)
		sx := x + tx - originX
		sy := y + ty - originY
		minX = math.Min(minX, sx)
		minY = math.Min(minY, sy)
		maxX = math.Max(maxX, sx)
		maxY = math.Max(maxY, sy)
	}
	return minX, minY, maxX, maxY
}
