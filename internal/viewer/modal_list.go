package viewer

import "github.com/jupiterrider/purego-sdl3/sdl"

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
