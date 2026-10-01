package viewer

import (
	"sort"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
)

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
	bottom := a.modalListBottomPadding()
	rows := max(1, (h-rowHeight-bottom)/rowHeight)
	// Fit the panel to its rows, keeping it centred.
	fitted := min(h, rowHeight+rows*rowHeight+bottom)
	y += (h - fitted) / 2
	return sdl.FRect{X: float32(x), Y: float32(y), W: float32(w), H: float32(fitted)}, rows
}

func (a *App) modalListRowHeight() int {
	return a.statusBarHeight()
}

// modalListBottomPadding is the space below a list's last row.
func (a *App) modalListBottomPadding() int {
	return max(a.ipx(4), a.uiPadding())
}

// modalListRowInset is how far a row's highlight is inset from the sides
// of its panel.
func (a *App) modalListRowInset() float32 { return a.px(6) }

// modalListTextInset is how far row text is inset from the sides of its
// panel.
func (a *App) modalListTextInset() int {
	return int(a.modalListRowInset()) + max(a.ipx(4), a.uiPadding())
}

func (a *App) modalListBaselineOffset(rowHeight int) int {
	return (rowHeight + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
}

func (a *App) drawModalListFrame(renderer *sdl.Renderer, rect sdl.FRect) error {
	a.drawPanel(renderer, rect, a.uiRadius())
	return nil
}

// drawModalListHeader draws a panel's title in its first row, with detail
// such as a count after it in the muted colour, over a hairline.
func (a *App) drawModalListHeader(renderer *sdl.Renderer, rect sdl.FRect, title, detail string) error {
	rowHeight := a.modalListRowHeight()
	baseline := int(rect.Y) + a.modalListBaselineOffset(rowHeight)
	x := int(rect.X) + a.modalListTextInset()
	width := int(rect.W) - 2*a.modalListTextInset()
	title = truncateText(a.headingFont(), title, width)
	if err := a.drawHeading(renderer, title, x, baseline, a.foregroundColor()); err != nil {
		return err
	}
	if detail != "" {
		x += measureText(a.headingFont(), title) + a.ipx(8)
		if err := a.drawText(renderer, a.truncateModalListText(detail, int(rect.X)+int(rect.W)-a.modalListTextInset()-x), x, baseline, a.mutedColor()); err != nil {
			return err
		}
	}
	line := sdl.FRect{X: rect.X, Y: rect.Y + float32(rowHeight) - a.hairline(), W: rect.W, H: a.hairline()}
	return fillRect(renderer, line, a.borderColor())
}

func (a *App) drawModalListSelection(renderer *sdl.Renderer, rect sdl.FRect, y, rowHeight int) error {
	inset := a.modalListRowInset()
	row := sdl.FRect{X: rect.X + inset, Y: float32(y), W: rect.W - 2*inset, H: float32(rowHeight)}
	fillRoundedRect(renderer, row, a.uiRadius()*0.75, 1, a.rowSelectionColor())
	return nil
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

func modalListStartScrollbarDrag(rect sdl.FRect, rowHeight, rows, total, x, y int, scroll, dragOffset *int, dragging *bool) bool {
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, total, *scroll)
	if !ok || !pointInRect(x, y, track) {
		return false
	}
	if pointInRect(x, y, thumb) {
		*dragOffset = int(float32(y) - thumb.Y)
	} else {
		*dragOffset = int(thumb.H / 2)
		*scroll = modalListScrollbarScrollForY(track, thumb, rows, total, y, *dragOffset)
	}
	*dragging = true
	return true
}

func modalListDragScrollbar(rect sdl.FRect, rowHeight, rows, total, y int, scroll *int, dragOffset int) {
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, total, *scroll)
	if !ok {
		return
	}
	*scroll = modalListScrollbarScrollForY(track, thumb, rows, total, y, dragOffset)
}

// modalListScrollForSelection returns the first row to show so the selected
// row has scrollOff rows of context above and below; like vim, a scrollOff of
// half the window or more keeps the selection centred.
func modalListScrollForSelection(scroll, selected, rows, total, scrollOff int) int {
	if rows < 1 {
		rows = 1
	}
	maxScroll := max(0, total-rows)
	scrollOff = max(0, scrollOff)
	if scrollOff*2 >= rows {
		middleRow := (rows - 1) / 2
		return clampInt(selected-middleRow, 0, maxScroll)
	}
	if selected < scroll+scrollOff {
		scroll = selected - scrollOff
	}
	if selected >= scroll+rows-scrollOff {
		scroll = selected - rows + scrollOff + 1
	}
	return clampInt(scroll, 0, maxScroll)
}

func pointInRect(x, y int, rect sdl.FRect) bool {
	return float32(x) >= rect.X && float32(x) <= rect.X+rect.W && float32(y) >= rect.Y && float32(y) <= rect.Y+rect.H
}

func (a *App) drawModalListScrollbar(renderer *sdl.Renderer, rect sdl.FRect, rowHeight, rows, total, scroll int) error {
	track, thumb, ok := modalListScrollbarRects(rect, rowHeight, rows, total, scroll)
	if !ok {
		return nil
	}
	// A slim thumb clear of the panel's edge; the track stays the area that
	// takes clicks.
	_ = track
	w := min(thumb.W, max(2, a.px(4)))
	thumb.X = rect.X + rect.W - a.px(5) - w
	thumb.W = w
	clr := a.mutedColor()
	clr.A = 0x90
	fillRoundedRect(renderer, thumb, w/2, 1, clr)
	return nil
}

// truncateModalListText shortens s with an ellipsis to fit maxWidth. The
// cut is found by binary search, as rows are truncated on every frame.
func (a *App) truncateModalListText(s string, maxWidth int) string {
	return truncateText(a.fontFace, s, maxWidth)
}

func truncateText(face font.Face, s string, maxWidth int) string {
	if maxWidth <= 0 || measureText(face, s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	// The longest prefix, of at least one rune, that fits with the ellipsis.
	keep := sort.Search(len(runes), func(n int) bool {
		return measureText(face, string(runes[:n+1])+"...") > maxWidth
	})
	return string(runes[:max(1, keep)]) + "..."
}
