package viewer

import "github.com/jupiterrider/purego-sdl3/sdl"

const modalScrollbarWidth = 8

func (a *App) modalListGeometry(widthPct, heightPct int) (sdl.FRect, int) {
	viewportW, viewportH := a.viewportSize()
	widthPct = clampInt(widthPct, 20, 100)
	heightPct = clampInt(heightPct, 20, 100)
	w := int(float64(viewportW) * float64(widthPct) / 100)
	h := int(float64(viewportH) * float64(heightPct) / 100)
	w = clampInt(w, 260, viewportW)
	h = clampInt(h, 160, viewportH)
	x := (viewportW - w) / 2
	y := (viewportH - h) / 2
	rowHeight := a.modalListRowHeight()
	rows := max(1, (h-rowHeight-16)/rowHeight)
	return sdl.FRect{X: float32(x), Y: float32(y), W: float32(w), H: float32(h)}, rows
}

func (a *App) modalListRowHeight() int {
	return max(a.fontFace.Metrics().Height.Ceil()+4, a.config.StatusBarHeight)
}

func (a *App) modalListBaselineOffset(rowHeight int) int {
	return (rowHeight + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
}

func (a *App) drawModalListFrame(renderer *sdl.Renderer, rect sdl.FRect) error {
	bg := a.backgroundColor()
	bg.A = 0xf2
	if err := fillRect(renderer, rect, bg); err != nil {
		return err
	}
	return strokeRect(renderer, rect, a.statusBarColor(), 2)
}

func (a *App) drawModalListSelection(renderer *sdl.Renderer, rect sdl.FRect, y, rowHeight int) error {
	hl := a.highlightBackgroundColor()
	hl.A = 0xd8
	return fillRect(renderer, sdl.FRect{X: rect.X + 6, Y: float32(y), W: rect.W - 12, H: float32(rowHeight)}, hl)
}

func (a *App) modalListRowAt(rect sdl.FRect, rows, rowHeight, x, y int) (int, bool) {
	if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
		return 0, false
	}
	if float32(y) < rect.Y+float32(rowHeight) {
		return 0, false
	}
	row := (y - int(rect.Y) - rowHeight) / rowHeight
	if row < 0 || row >= rows {
		return 0, false
	}
	return row, true
}

func modalListScrollbarRects(rect sdl.FRect, rowHeight, rows, total, scroll int) (sdl.FRect, sdl.FRect, bool) {
	if rows <= 0 || total <= rows {
		return sdl.FRect{}, sdl.FRect{}, false
	}
	trackTop := rect.Y + float32(rowHeight)
	trackH := rect.H - float32(rowHeight) - 8
	if trackH < 20 {
		return sdl.FRect{}, sdl.FRect{}, false
	}
	track := sdl.FRect{X: rect.X + rect.W - modalScrollbarWidth - 6, Y: trackTop, W: modalScrollbarWidth, H: trackH}
	thumbH := track.H * float32(rows) / float32(total)
	if thumbH < 20 {
		thumbH = 20
	}
	maxScroll := max(0, total-rows)
	thumbTravel := track.H - thumbH
	thumbY := track.Y
	if maxScroll > 0 && thumbTravel > 0 {
		thumbY += thumbTravel * float32(clampInt(scroll, 0, maxScroll)) / float32(maxScroll)
	}
	thumb := sdl.FRect{X: track.X, Y: thumbY, W: track.W, H: thumbH}
	return track, thumb, true
}

func modalListScrollbarScrollForY(track, thumb sdl.FRect, rows, total, y, dragOffset int) int {
	maxScroll := max(0, total-rows)
	if maxScroll == 0 {
		return 0
	}
	travel := track.H - thumb.H
	if travel <= 0 {
		return 0
	}
	rel := clampFloat(float64(float32(y)-track.Y-float32(dragOffset)), 0, float64(travel))
	return clampInt(int(rel/float64(travel)*float64(maxScroll)+0.5), 0, maxScroll)
}

func pointInRect(x, y int, rect sdl.FRect) bool {
	return float32(x) >= rect.X && float32(x) <= rect.X+rect.W && float32(y) >= rect.Y && float32(y) <= rect.Y+rect.H
}

func (a *App) drawModalListScrollbar(renderer *sdl.Renderer, rect sdl.FRect, rowHeight, rows, total, scroll int) error {
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, total, scroll)
	if !ok {
		return nil
	}
	trackColor := a.statusBarColor()
	trackColor.A = 0x80
	if err := fillRect(renderer, track, trackColor); err != nil {
		return err
	}
	thumbColor := a.foregroundColor()
	thumbColor.A = 0xb0
	return fillRect(renderer, thumb, thumbColor)
}

func (a *App) truncateModalListText(s string, maxWidth int) string {
	if maxWidth <= 0 || measureText(a.fontFace, s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for len(runes) > 1 && measureText(a.fontFace, string(runes)+"...") > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
