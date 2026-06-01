package viewer

import (
	"image/color"
	"math"

	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) drawPages(renderer *sdl.Renderer) {
	if a.renderMode == "single" {
		a.drawSinglePage(renderer)
		return
	}
	a.drawContinuousPages(renderer)
}

func (a *App) drawContinuousPages(renderer *sdl.Renderer) {
	viewportW, viewportH := a.viewportSize()
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
			x := row.pageX[i] - a.scrollX + offsetX
			y := row.pageY[i] - a.scrollY + offsetY
			if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
				continue
			}
			_ = a.drawPageBackground(renderer, x, y, page)
			rp, ok := a.cachedRenderPage(page, a.scale)
			if !ok {
				continue
			}
			drawScale := a.renderDrawScale(rp, a.scale)
			a.drawPageTexture(renderer, x, y, row.pageW[i], row.pageH[i], rp, drawScale)
			a.drawSearchHighlightsForPage(renderer, page, x, y, rp)
		}
	}
	a.drawSelection(renderer)
}

func (a *App) drawSinglePage(renderer *sdl.Renderer) {
	if len(a.rows) == 0 || a.page < 0 || a.page >= len(a.pageToRow) {
		return
	}
	viewportW, viewportH := a.viewportSize()
	row := a.rows[a.pageToRow[a.page]]
	baseX := math.Max(float64(a.horizontalGap()), (float64(viewportW)-row.width)/2)
	baseY := math.Max(float64(a.verticalGap()), (float64(viewportH)-row.height)/2)
	for i, page := range row.pages {
		x := baseX + (row.pageX[i] - row.x) - a.scrollX
		y := baseY + (row.pageY[i] - row.y) - a.scrollY
		if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
			continue
		}
		_ = a.drawPageBackground(renderer, x, y, page)
		rp, ok := a.cachedRenderPage(page, a.scale)
		if !ok {
			continue
		}
		drawScale := a.renderDrawScale(rp, a.scale)
		a.drawPageTexture(renderer, x, y, row.pageW[i], row.pageH[i], rp, drawScale)
		a.drawSearchHighlightsForPage(renderer, page, x, y, rp)
	}
	a.drawSelection(renderer)
}

func (a *App) drawPageTexture(renderer *sdl.Renderer, x, y, width, height float64, rp *renderedPage, drawScale float64) {
	drawW := rp.width * drawScale
	drawH := rp.height * drawScale
	centerX := x + width/2
	centerY := y + height/2
	if rp.scale > 0 {
		originX, originY := rotatedBoundsOrigin(a.pageMetrics[rp.page].bounds, a.scale, a.rotation)
		pageX := (rp.pixX + rp.width/2) / rp.scale
		pageY := (rp.pixY + rp.height/2) / rp.scale
		tx, ty := transformPoint(pageX, pageY, a.scale, a.rotation)
		centerX = x + tx - originX
		centerY = y + ty - originY
	}
	dst := sdl.FRect{
		X: float32(centerX - drawW/2),
		Y: float32(centerY - drawH/2),
		W: float32(drawW),
		H: float32(drawH),
	}
	if normalizeRotation(a.rotation) == 0 {
		sdl.RenderTexture(renderer, rp.texture, nil, &dst)
		return
	}
	sdl.RenderTextureRotated(renderer, rp.texture, nil, &dst, a.rotation, nil, sdl.FlipNone)
}

func (a *App) drawPageBackground(renderer *sdl.Renderer, x, y float64, page int) error {
	clr := a.pageBackgroundColor()
	if normalizeRotation(a.rotation) == 0 {
		m := a.pageMetrics[page]
		return fillRect(renderer, sdl.FRect{X: float32(x), Y: float32(y), W: float32(m.width * a.scale), H: float32(m.height * a.scale)}, clr)
	}
	return renderBool(sdl.RenderGeometry(renderer, nil, pageBackgroundVertices(x, y, a.pageMetrics[page].bounds, a.scale, a.rotation, clr), []int32{0, 1, 2, 1, 3, 2}), "render geometry")
}

func pageBackgroundVertices(x, y float64, bounds mupdf.Rect, scale, rotation float64, clr color.RGBA) []sdl.Vertex {
	originX, originY := rotatedBoundsOrigin(bounds, scale, rotation)
	points := []mupdf.Point{
		{X: float64(bounds.X0), Y: float64(bounds.Y0)},
		{X: float64(bounds.X1), Y: float64(bounds.Y0)},
		{X: float64(bounds.X0), Y: float64(bounds.Y1)},
		{X: float64(bounds.X1), Y: float64(bounds.Y1)},
	}
	vertices := make([]sdl.Vertex, len(points))
	color := sdl.FColor{R: float32(clr.R) / 255, G: float32(clr.G) / 255, B: float32(clr.B) / 255, A: float32(clr.A) / 255}
	for i, point := range points {
		tx, ty := transformPoint(point.X, point.Y, scale, rotation)
		vertices[i] = sdl.Vertex{
			Position: sdl.FPoint{X: float32(x + tx - originX), Y: float32(y + ty - originY)},
			Color:    color,
		}
	}
	return vertices
}

func (a *App) cachedRenderPage(page int, scale float64) (*renderedPage, bool) {
	renderScale := a.renderScaleFor(scale)
	key := renderCacheKey(page, renderScale, a.altColors, a.config.AntiAliasing)
	if rp, ok := a.renderCache[key]; ok {
		return rp, true
	}
	var bestHigher *renderedPage
	var bestLower *renderedPage
	a.ensureRenderCacheState()
	for _, rp := range a.renderIndex[renderVariantKey{page: page, altColors: a.altColors, aaLevel: a.config.AntiAliasing}] {
		if math.Abs(rp.scale-renderScale) < 0.0001 {
			return rp, true
		}
		if rp.scale >= renderScale {
			if bestHigher == nil || rp.scale < bestHigher.scale {
				bestHigher = rp
			}
			continue
		}
		if bestLower == nil || rp.scale > bestLower.scale {
			bestLower = rp
		}
	}
	if bestHigher != nil {
		return bestHigher, true
	}
	if bestLower != nil {
		return bestLower, true
	}
	return nil, false
}

func (a *App) prefetchVisiblePages() {
	if len(a.rows) == 0 {
		return
	}
	visible := []int{}
	prefetch := []int{}
	seen := map[int]bool{}
	if a.renderMode == "single" {
		if a.page < 0 || a.page >= len(a.pageToRow) {
			return
		}
		row := a.rows[a.pageToRow[a.page]]
		for _, page := range row.pages {
			visible = append(visible, page)
			seen[page] = true
		}
	} else {
		viewportW, viewportH := a.viewportSize()
		offsetX, offsetY := a.contentViewportOffset()
		start, end := a.rowRangeForContentY(a.scrollY-offsetY, a.scrollY+float64(viewportH)-offsetY)
		for _, row := range a.rows[start:end] {
			rowY := row.y - a.scrollY + offsetY
			if rowY+row.height < 0 || rowY > float64(viewportH) {
				continue
			}
			for i, page := range row.pages {
				x := row.pageX[i] - a.scrollX + offsetX
				y := row.pageY[i] - a.scrollY + offsetY
				if x+row.pageW[i] < 0 || x > float64(viewportW) || y+row.pageH[i] < 0 || y > float64(viewportH) {
					continue
				}
				visible = append(visible, page)
				seen[page] = true
			}
		}
		margin := math.Max(a.renderMargin()*2, float64(viewportH))
		minY := a.scrollY - margin
		maxY := a.scrollY + float64(viewportH) + margin
		start, end = a.rowRangeForContentY(minY, maxY)
		for _, row := range a.rows[start:end] {
			if row.y+row.height < minY || row.y > maxY {
				continue
			}
			for _, page := range row.pages {
				if seen[page] {
					continue
				}
				prefetch = append(prefetch, page)
				seen[page] = true
			}
		}
	}
	totalPrefetch := len(visible) + len(prefetch)
	if totalPrefetch*2 > a.cacheLimit {
		a.cacheLimit = min(a.pageCount*2, totalPrefetch*2)
	}
	if a.renderWorker != nil {
		a.renderWorker.SetWantedPages(seen)
		a.renderWorker.DrainUnwanted(a.renderGeneration)
	}
	for key, req := range a.renderPending {
		if req.generation == a.renderGeneration && !seen[req.page] {
			delete(a.renderPending, key)
		}
	}
	for _, page := range visible {
		a.requestRender(page, a.scale, 0)
	}
	for _, page := range prefetch {
		a.requestRender(page, a.scale, 10)
	}
}
