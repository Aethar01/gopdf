package viewer

import (
	"fmt"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

type outlineMenuState struct {
	visible  bool
	selected int
	scroll   int
	expanded map[int]bool
}

func (a *App) toggleOutlineMenu() {
	if a.outlineMenu.visible {
		a.outlineMenu.visible = false
		return
	}
	a.outlineMenu.visible = true
	a.outlineMenu.selected = -1
	a.outlineMenu.scroll = 0
	a.outlineMenu.expanded = map[int]bool{}
	if len(a.outline) == 0 {
		return
	}
	a.outlineMenu.selected = a.outlineIndexForPage(a.page)
	for i, item := range a.outline {
		if item.HasChildren && item.Depth < a.config.OutlineInitialDepth {
			a.outlineMenu.expanded[i] = true
		}
	}
	for parent := a.outline[a.outlineMenu.selected].Parent; parent >= 0; parent = a.outline[parent].Parent {
		a.outlineMenu.expanded[parent] = true
	}
	a.ensureOutlineSelectionVisible()
}

func (a *App) outlineIndexForPage(page int) int {
	best := 0
	bestPage := -1
	for i, item := range a.outline {
		if item.Page >= 0 && item.Page <= page && item.Page >= bestPage {
			best = i
			bestPage = item.Page
		}
	}
	return best
}

func (a *App) visibleOutlineIndices() []int {
	visible := make([]int, 0, len(a.outline))
	for i, item := range a.outline {
		show := true
		parent := item.Parent
		for parent >= 0 {
			if !a.outlineMenu.expanded[parent] {
				show = false
				break
			}
			parent = a.outline[parent].Parent
		}
		if show {
			visible = append(visible, i)
		}
	}
	return visible
}

func (a *App) selectedVisibleOutlineRow(visible []int) int {
	for i, index := range visible {
		if index == a.outlineMenu.selected {
			return i
		}
	}
	if len(visible) == 0 {
		return 0
	}
	a.outlineMenu.selected = visible[clampInt(a.outlineMenu.scroll, 0, len(visible)-1)]
	return clampInt(a.outlineMenu.scroll, 0, len(visible)-1)
}

func (a *App) ensureOutlineSelectionVisible() {
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		a.outlineMenu.scroll = 0
		return
	}
	_, rows := a.outlineMenuGeometry()
	if rows < 1 {
		rows = 1
	}
	row := a.selectedVisibleOutlineRow(visible)
	if row < a.outlineMenu.scroll {
		a.outlineMenu.scroll = row
	}
	if row >= a.outlineMenu.scroll+rows {
		a.outlineMenu.scroll = row - rows + 1
	}
	maxScroll := max(0, len(visible)-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll, 0, maxScroll)
}

func (a *App) moveOutlineSelection(delta int) {
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		return
	}
	row := a.selectedVisibleOutlineRow(visible)
	row = clampInt(row+delta, 0, len(visible)-1)
	a.outlineMenu.selected = visible[row]
	a.ensureOutlineSelectionVisible()
}

func (a *App) scrollOutlineMenu(delta int) {
	_, rows := a.outlineMenuGeometry()
	maxScroll := max(0, len(a.visibleOutlineIndices())-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll+delta, 0, maxScroll)
}

func (a *App) activateSelectedOutline() {
	if a.outlineMenu.selected < 0 || a.outlineMenu.selected >= len(a.outline) {
		return
	}
	item := a.outline[a.outlineMenu.selected]
	if item.Page < 0 {
		return
	}
	a.outlineMenu.visible = false
	a.alignPageTop(item.Page)
}

func (a *App) collapseSelectedOutline() {
	selected := a.outlineMenu.selected
	if selected < 0 || selected >= len(a.outline) {
		return
	}
	if a.outline[selected].HasChildren && a.outlineMenu.expanded[selected] {
		delete(a.outlineMenu.expanded, selected)
		a.ensureOutlineSelectionVisible()
		return
	}
	if parent := a.outline[selected].Parent; parent >= 0 {
		a.outlineMenu.selected = parent
		a.ensureOutlineSelectionVisible()
	}
}

func (a *App) expandSelectedOutline() {
	selected := a.outlineMenu.selected
	if selected < 0 || selected >= len(a.outline) || !a.outline[selected].HasChildren {
		return
	}
	a.outlineMenu.expanded[selected] = true
	a.ensureOutlineSelectionVisible()
}

func (a *App) handleOutlineMenuKey(e *sdl.KeyboardEvent) bool {
	if e.Type != sdl.KEYDOWN || e.Repeat != 0 {
		return true
	}
	if token, ok := keyToken(e.Keysym.Sym, sdl.Keymod(e.Keysym.Mod)); ok {
		if action, ok := a.sequenceLookup[normalizeBinding(token)]; ok {
			a.runOutlineMenuAction(action)
		}
	}
	return true
}

func (a *App) runOutlineMenuAction(action string) {
	switch action {
	case "scroll_down":
		a.moveOutlineSelection(1)
	case "scroll_up":
		a.moveOutlineSelection(-1)
	case "scroll_left":
		a.collapseSelectedOutline()
	case "scroll_right":
		a.expandSelectedOutline()
	case "confirm":
		a.activateSelectedOutline()
	case "close", "outline":
		a.outlineMenu.visible = false
	default:
		a.runAction(action)
	}
}

func (a *App) clickOutlineMenu(x, y int) {
	rect, rows := a.outlineMenuGeometry()
	if float32(x) < rect.X || float32(x) > rect.X+rect.W || float32(y) < rect.Y || float32(y) > rect.Y+rect.H {
		a.outlineMenu.visible = false
		return
	}
	rowHeight := a.outlineMenuRowHeight()
	row := (y - int(rect.Y) - rowHeight) / rowHeight
	if row < 0 || row >= rows {
		return
	}
	visible := a.visibleOutlineIndices()
	index := a.outlineMenu.scroll + row
	if index < 0 || index >= len(visible) {
		return
	}
	a.outlineMenu.selected = visible[index]
	a.activateSelectedOutline()
}

func (a *App) outlineMenuGeometry() (sdl.FRect, int) {
	viewportW, viewportH := a.viewportSize()
	widthPct := clampInt(a.config.OutlineWidthPercent, 20, 100)
	heightPct := clampInt(a.config.OutlineHeightPercent, 20, 100)
	w := int(float64(viewportW) * float64(widthPct) / 100)
	h := int(float64(viewportH) * float64(heightPct) / 100)
	w = clampInt(w, 260, viewportW)
	h = clampInt(h, 160, viewportH)
	x := (viewportW - w) / 2
	y := (viewportH - h) / 2
	rowHeight := a.outlineMenuRowHeight()
	rows := max(1, (h-rowHeight-16)/rowHeight)
	return sdl.FRect{X: float32(x), Y: float32(y), W: float32(w), H: float32(h)}, rows
}

func (a *App) outlineMenuRowHeight() int {
	return max(a.fontFace.Metrics().Height.Ceil()+4, a.config.StatusBarHeight)
}

func (a *App) drawOutlineMenu(renderer *sdl.Renderer) error {
	rect, rows := a.outlineMenuGeometry()
	bg := a.backgroundColor()
	bg.A = 0xf2
	if err := fillRect(renderer, rect, bg); err != nil {
		return err
	}
	if err := strokeRect(renderer, rect, a.statusBarColor(), 2); err != nil {
		return err
	}
	rowHeight := a.outlineMenuRowHeight()
	baselineOffset := (rowHeight + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
	header := fmt.Sprintf(" Outline (%d)", len(a.outline))
	if err := drawText(renderer, a.fontFace, a.truncateOutlineText(header, int(rect.W)-24), int(rect.X)+12, int(rect.Y)+baselineOffset, a.foregroundColor()); err != nil {
		return err
	}
	visible := a.visibleOutlineIndices()
	if len(visible) == 0 {
		text := "No PDF outline found"
		if err := drawText(renderer, a.fontFace, text, int(rect.X)+16, int(rect.Y)+rowHeight+baselineOffset, a.foregroundColor()); err != nil {
			return err
		}
		return nil
	}
	maxScroll := max(0, len(visible)-rows)
	a.outlineMenu.scroll = clampInt(a.outlineMenu.scroll, 0, maxScroll)
	for row := 0; row < rows; row++ {
		visibleIndex := a.outlineMenu.scroll + row
		if visibleIndex >= len(visible) {
			break
		}
		outlineIndex := visible[visibleIndex]
		item := a.outline[outlineIndex]
		y := int(rect.Y) + rowHeight + row*rowHeight
		if outlineIndex == a.outlineMenu.selected {
			hl := a.highlightBackgroundColor()
			hl.A = 0xd8
			if err := fillRect(renderer, sdl.FRect{X: rect.X + 6, Y: float32(y), W: rect.W - 12, H: float32(rowHeight)}, hl); err != nil {
				return err
			}
		}
		marker := "  "
		if item.HasChildren {
			marker = "+ "
			if a.outlineMenu.expanded[outlineIndex] {
				marker = "- "
			}
		}
		page := ""
		if item.Page >= 0 {
			page = fmt.Sprintf("%d", item.Page+1)
		}
		indent := strings.Repeat("  ", item.Depth)
		text := indent + marker + strings.TrimSpace(item.Title)
		if strings.TrimSpace(text) == strings.TrimSpace(indent+marker) {
			text += "untitled"
		}
		pageW := measureText(a.fontFace, page)
		maxTextW := int(rect.W) - 36 - pageW
		text = a.truncateOutlineText(text, maxTextW)
		clr := a.foregroundColor()
		if outlineIndex == a.outlineMenu.selected {
			clr = a.highlightForegroundColor()
		}
		if err := drawText(renderer, a.fontFace, text, int(rect.X)+16, y+baselineOffset, clr); err != nil {
			return err
		}
		if page != "" {
			if err := drawText(renderer, a.fontFace, page, int(rect.X+rect.W)-16-pageW, y+baselineOffset, clr); err != nil {
				return err
			}
		}
	}
	return nil
}

func (a *App) truncateOutlineText(s string, maxWidth int) string {
	if maxWidth <= 0 || measureText(a.fontFace, s) <= maxWidth {
		return s
	}
	runes := []rune(s)
	for len(runes) > 1 && measureText(a.fontFace, string(runes)+"...") > maxWidth {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "..."
}
