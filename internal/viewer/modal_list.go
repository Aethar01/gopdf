package viewer

import (
	"math"
	"unicode/utf8"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
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
	rowHeight, head := a.modalListRowHeight(), a.modalListHeadHeight()
	bottom := a.modalListBottomPadding()
	rows := max(1, (h-head-bottom)/rowHeight)
	// Fit the panel to its rows, keeping it centred.
	fitted := min(h, head+rows*rowHeight+bottom)
	y += (h - fitted) / 2
	return sdl.FRect{X: float32(x), Y: float32(y), W: float32(w), H: float32(fitted)}, rows
}

// modalListRowHeight is the height of a menu row: a line of text and the
// row's padding above and below it.
func (a *App) modalListRowHeight() int {
	top, _, bottom, _ := a.insets(a.style(config.ElementRow).Padding.V)
	return a.uiLineHeight() + int(math.Round(float64(top+bottom)))
}

// modalListHeaderHeight is the height of a menu's header: a line of text
// and the header's padding above and below it.
func (a *App) modalListHeaderHeight() int {
	top, _, bottom, _ := a.insets(a.style(config.ElementHeader).Padding.V)
	return a.uiLineHeight() + int(math.Round(float64(top+bottom)))
}

// modalListHeadHeight is how far a menu's rows start below its top: the
// panel's padding above the header, and the header.
func (a *App) modalListHeadHeight() int {
	top, _, _, _ := a.insets(a.style(config.ElementPanel).Padding.V)
	return int(math.Round(float64(top))) + a.modalListHeaderHeight()
}

// modalListBottomPadding is the space below a list's last row.
func (a *App) modalListBottomPadding() int {
	_, _, bottom, _ := a.insets(a.style(config.ElementPanel).Padding.V)
	return int(math.Round(float64(bottom)))
}

// modalListRowInset is how far a row's box is inset from the sides of its
// panel.
func (a *App) modalListRowInset() float32 {
	_, _, _, left := a.insets(a.style(config.ElementPanel).Padding.V)
	return left
}

// rowBox is the box of a row at y across panel, less the panel's padding.
func (a *App) rowBox(panel sdl.FRect, y, rowHeight int) sdl.FRect {
	_, right, _, left := a.insets(a.style(config.ElementPanel).Padding.V)
	return sdl.FRect{X: panel.X + left, Y: float32(y), W: panel.W - left - right, H: float32(rowHeight)}
}

// modalListTextInset is how far row text is inset from the sides of its
// panel.
func (a *App) modalListTextInset() int {
	_, _, _, padding := a.insets(a.style(config.ElementRow).Padding.V)
	return int(math.Round(float64(a.modalListRowInset() + padding)))
}

func (a *App) modalListBaselineOffset(rowHeight int) int {
	return (rowHeight + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
}

// drawPanel draws a panel styled as element over rect, with drawContent
// clipped to it, and its border over that.
func (a *App) drawPanel(renderer *sdl.Renderer, element config.Element, rect sdl.FRect, drawContent func() error) error {
	st := a.style(element)
	border := a.drawBoxParts(renderer, &st, rect, boxBody)
	if err := a.withClip(renderer, rect, drawContent); err != nil {
		return err
	}
	if border {
		a.drawBoxParts(renderer, &st, rect, boxBorder)
	}
	return nil
}

// drawModalListHeader draws a panel's title at its top, below the panel's
// padding, with detail such as a count after it in the secondary colour.
func (a *App) drawModalListHeader(renderer *sdl.Renderer, rect sdl.FRect, title, detail string) error {
	st := a.style(config.ElementHeader)
	height := a.modalListHeaderHeight()
	top := rect.Y + float32(a.modalListHeadHeight()-height)
	a.drawBox(renderer, &st, sdl.FRect{X: rect.X, Y: top, W: rect.W, H: float32(height)})
	_, padRight, _, padLeft := a.insets(st.Padding.V)
	baseline := int(top) + a.modalListBaselineOffset(height)
	x := int(rect.X + padLeft)
	end := int(rect.X + rect.W - padRight)
	face := a.styleFace(&st)
	title = truncateText(face, title, end-x)
	if err := a.drawTextFace(renderer, title, x, baseline, a.textColor(&st, false), st.Bold.V); err != nil {
		return err
	}
	if detail != "" {
		x += measureText(face, title) + a.ipx(st.Gap.V)
		if err := a.drawText(renderer, a.truncateModalListText(detail, end-x), x, baseline, a.textColor(&st, true)); err != nil {
			return err
		}
	}
	return nil
}

// styleFace is the face st's text is drawn in.
func (a *App) styleFace(st *config.Style) font.Face {
	if st.Bold.V {
		return a.headingFont()
	}
	return a.fontFace
}

// drawModalListSelection draws the selected row's box at y in rect.
func (a *App) drawModalListSelection(renderer *sdl.Renderer, rect sdl.FRect, y, rowHeight int) error {
	st := a.style(config.ElementRowSelected)
	a.drawBox(renderer, &st, a.rowBox(rect, y, rowHeight))
	return nil
}

// modalListRowAt is the row of a list at x, y, its rows starting head
// pixels below the top of rect.
func (a *App) modalListRowAt(rect sdl.FRect, rows, head, rowHeight, x, y int) (int, bool) {
	if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
		return 0, false
	}
	if float32(y) < rect.Y+float32(head) {
		return 0, false
	}
	row := (y - int(rect.Y) - head) / rowHeight
	if row < 0 || row >= rows {
		return 0, false
	}
	return row, true
}

// modalListScrollbarRects is the scrollbar of a list whose rows start head
// pixels below the top of rect.
func modalListScrollbarRects(rect sdl.FRect, head, rows, total int, offset float64) (sdl.FRect, sdl.FRect, bool) {
	trackTop := rect.Y + float32(head)
	return listScrollbarRects(sdl.FRect{X: rect.X, Y: trackTop, W: rect.W, H: rect.H - float32(head) - 8}, rows, total, offset)
}

// listScrollbarRects is the scrollbar's track and thumb for a list in
// area showing rows of total rows from offset, if it needs one.
func listScrollbarRects(area sdl.FRect, rows, total int, offset float64) (sdl.FRect, sdl.FRect, bool) {
	if rows <= 0 || total <= rows || area.H < 20 {
		return sdl.FRect{}, sdl.FRect{}, false
	}
	track := sdl.FRect{X: area.X + area.W - modalScrollbarWidth - 6, Y: area.Y, W: modalScrollbarWidth, H: area.H}
	thumbH := max(20, track.H*float32(rows)/float32(total))
	maxScroll := max(0, total-rows)
	thumbTravel := track.H - thumbH
	thumbY := track.Y
	if maxScroll > 0 && thumbTravel > 0 {
		thumbY += thumbTravel * float32(clampFloat(offset, 0, float64(maxScroll))) / float32(maxScroll)
	}
	return track, sdl.FRect{X: track.X, Y: thumbY, W: track.W, H: thumbH}, true
}

// modalListScrollbarScrollForY is the offset that puts the thumb at y,
// grabbed dragOffset pixels from its top; it follows the pointer exactly,
// so it may fall part-way into a row.
func modalListScrollbarScrollForY(track, thumb sdl.FRect, rows, total, y, dragOffset int) float64 {
	maxScroll := max(0, total-rows)
	if maxScroll == 0 {
		return 0
	}
	travel := track.H - thumb.H
	if travel <= 0 {
		return 0
	}
	rel := clampFloat(float64(float32(y)-track.Y-float32(dragOffset)), 0, float64(travel))
	return rel / float64(travel) * float64(maxScroll)
}

func modalListStartScrollbarDrag(rect sdl.FRect, head, rows, total, x, y int, scroll *float64, dragOffset *int, dragging *bool) bool {
	track, thumb, ok := modalListScrollbarRects(rect, head, rows, total, *scroll)
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

func modalListDragScrollbar(rect sdl.FRect, head, rows, total, y int, scroll *float64, dragOffset int) {
	track, thumb, ok := modalListScrollbarRects(rect, head, rows, total, *scroll)
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

func (a *App) drawModalListScrollbar(renderer *sdl.Renderer, rect sdl.FRect, head, rows, total int, offset float64) error {
	if _, thumb, ok := modalListScrollbarRects(rect, head, rows, total, offset); ok {
		a.drawScrollbarThumb(renderer, rect, thumb)
	}
	return nil
}

// drawScrollbarThumb draws thumb as a slim bar set off the right edge of
// panel; the wider track around it is what takes clicks.
func (a *App) drawScrollbarThumb(renderer *sdl.Renderer, panel, thumb sdl.FRect) {
	st := a.style(config.ElementScrollbar)
	_, margin, _, _ := a.insets(st.Margin.V)
	w := max(1, a.px(st.Width.V))
	thumb.X = panel.X + panel.W - margin - w
	thumb.W = w
	a.drawBox(renderer, &st, thumb)
}

// truncateModalListText shortens s with an ellipsis to fit maxWidth.
func (a *App) truncateModalListText(s string, maxWidth int) string {
	return truncateText(a.fontFace, s, maxWidth)
}

// truncateText shortens s with an ellipsis to fit maxWidth. Rows are
// truncated on every frame, so the cut is found in one pass over s,
// measuring each prefix with the ellipsis as the drawer would.
func truncateText(face font.Face, s string, maxWidth int) string {
	if maxWidth <= 0 || measureText(face, s) <= maxWidth {
		return s
	}
	const ellipsis = "..."
	// The longest prefix, of at least one rune, that fits with the ellipsis.
	dots, limit := font.MeasureString(face, ellipsis), fixed.I(maxWidth)
	var width fixed.Int26_6
	prev, keep := rune(-1), 0
	for i := 0; i < len(s); {
		r, size := utf8.DecodeRuneInString(s[i:])
		if prev >= 0 {
			width += face.Kern(prev, r)
		}
		advance, _ := face.GlyphAdvance(r)
		width += advance
		if keep > 0 && width+face.Kern(r, '.')+dots > limit {
			break
		}
		prev, i = r, i+size
		keep = i
	}
	return s[:keep] + ellipsis
}
