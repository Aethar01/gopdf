package viewer

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font"
)

func (a *App) drawStatusBar(renderer *sdl.Renderer) error {
	h := a.statusBarHeight()
	y := a.winH - h
	if err := fillRect(renderer, sdl.FRect{X: 0, Y: float32(y), W: float32(a.winW), H: float32(h)}, a.statusBarColor()); err != nil {
		return err
	}
	pad := a.config.StatusBarPadding
	left, right := fitStatusText(a.fontFace, a.formatStatusBar(a.config.StatusBarLeft), a.formatStatusBar(a.config.StatusBarRight), a.winW-2*pad, 2*pad, a.mode != modeNormal)
	vertOffset := (h + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
	textX := pad
	if a.mode != modeNormal {
		textX = a.promptOrigin() - a.promptStart()
	}
	if err := a.drawInputSelection(renderer, y, vertOffset); err != nil {
		return err
	}
	if err := a.drawText(renderer, left, textX, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	if err := a.drawInputCursor(renderer, y, vertOffset); err != nil {
		return err
	}
	rw := measureText(a.fontFace, right)
	if err := a.drawText(renderer, right, a.winW-rw-pad, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	return nil
}

// promptStart is how far into the left status text the prompt begins: the
// width of whatever the template shows before {message}.
func (a *App) promptStart() int {
	before, _, found := strings.Cut(a.config.StatusBarLeft, "{message}")
	if !found {
		return 0
	}
	return measureText(a.fontFace, a.formatStatusBar(before))
}

// inputDisplay returns the input as shown, masked for passwords, and the
// part of it before the cursor.
func (a *App) inputDisplay() (display, left string) {
	display, left = a.input.Value, a.input.Left()
	if a.mode == modePassword {
		display = strings.Repeat("*", utf8.RuneCountInString(display))
		left = strings.Repeat("*", utf8.RuneCountInString(left))
	}
	return display, left
}

// promptOrigin is the x at which the prompt starts. Long input scrolls left,
// keeping its start hidden, so the cursor stays on screen.
func (a *App) promptOrigin() int {
	pad := a.config.StatusBarPadding
	start := pad + a.promptStart()
	display, left := a.inputDisplay()
	prefix := a.inputPrefix()
	prefixWidth := measureText(a.fontFace, prefix)
	cursor := start + measureText(a.fontFace, prefix+left)
	limit := a.winW - pad - measureText(a.fontFace, "  ") // leave room past the cursor
	end := start + measureText(a.fontFace, prefix+display)
	switch {
	case cursor-a.inputScroll > limit:
		a.inputScroll = cursor - limit
	case cursor-a.inputScroll < start+prefixWidth: // keep a prefix's width of context
		a.inputScroll = cursor - start - prefixWidth
	}
	a.inputScroll = clampInt(a.inputScroll, 0, max(0, end-limit))
	return start - a.inputScroll
}

func (a *App) drawInputSelection(renderer *sdl.Renderer, barY, vertOffset int) error {
	if a.mode == modeNormal {
		return nil
	}
	start, end, ok := a.input.SelectionRange()
	if !ok {
		return nil
	}
	display, _ := a.inputDisplay()
	left, rest := splitAtRune(display, start)
	selected, _ := splitAtRune(rest, end-start)
	x := a.promptOrigin() + measureText(a.fontFace, a.inputPrefix()+left)
	w := max(1, measureText(a.fontFace, selected))
	mt := a.fontFace.Metrics()
	top := barY + vertOffset - mt.Ascent.Ceil()
	bottom := barY + vertOffset + mt.Descent.Ceil()
	return fillRect(renderer, sdl.FRect{X: float32(x), Y: float32(top), W: float32(w), H: float32(max(1, bottom-top))}, a.selectionColor())
}

func (a *App) drawInputCursor(renderer *sdl.Renderer, barY, vertOffset int) error {
	if a.mode == modeNormal {
		return nil
	}
	_, left := a.inputDisplay()
	x := a.promptOrigin() + measureText(a.fontFace, a.inputPrefix()+left)
	fg := a.foregroundColor()
	if !sdl.SetRenderDrawColor(renderer, fg.R, fg.G, fg.B, fg.A) {
		return sdlError("set draw color")
	}
	mt := a.fontFace.Metrics()
	cursorTop := barY + vertOffset - mt.Ascent.Ceil()
	cursorBot := barY + vertOffset + mt.Descent.Ceil()
	return renderBool(sdl.RenderLine(renderer, float32(x), float32(cursorTop), float32(x), float32(cursorBot)), "draw line")
}

// fitStatusText keeps the two sides of the status bar from overlapping in
// width. Each side is truncated towards a fair share of the space; while an
// input prompt is on the left it stays whole, and the right side is hidden
// if the two would collide.
func fitStatusText(face font.Face, left, right string, width, gap int, keepLeft bool) (string, string) {
	lw, rw := measureText(face, left), measureText(face, right)
	if right == "" || lw+gap+rw <= width {
		return left, right
	}
	if keepLeft {
		return left, ""
	}
	right = truncateText(face, right, max(width/2, width-gap-lw))
	left = truncateText(face, left, width-gap-measureText(face, right))
	if measureText(face, left)+gap+measureText(face, right) > width {
		return truncateText(face, left, width), "" // too narrow for both
	}
	return left, right
}

func (a *App) statusBarHeight() int {
	if a.fontFace == nil {
		return 4
	}
	metrics := a.fontFace.Metrics()
	return max(metrics.Height.Ceil(), metrics.Ascent.Ceil()+metrics.Descent.Ceil()) + 4
}

func (a *App) formatStatusBar(template string) string {
	message := a.message
	var inputToken, promptToken string
	switch a.mode {
	case modeCommand:
		message = ":" + a.input.Value
		inputToken = a.input.Value
	case modeGotoPage:
		message = " GOTO " + a.input.Value
		inputToken = a.input.Value
	case modeSearch:
		promptToken = a.searchPromptToken()
		message = promptToken + a.input.Value
		inputToken = a.input.Value
	case modePassword:
		message = a.inputPrefix() + strings.Repeat("*", len([]rune(a.input.Value)))
		inputToken = message
	case modePrompt:
		message = a.inputPrefix() + a.input.Value
		inputToken = a.input.Value
	}

	page := fmt.Sprintf("%d", a.page+1)
	label := a.pageLabel(a.page)
	if a.dualPage && len(a.rows) > 0 && a.page >= 0 && a.page < len(a.pageToRow) {
		row := a.rows[a.pageToRow[a.page]]
		if len(row.pages) >= 2 {
			page = fmt.Sprintf("%d-%d", row.pages[0]+1, row.pages[len(row.pages)-1]+1)
			label = a.pageLabel(row.pages[0]) + "-" + a.pageLabel(row.pages[len(row.pages)-1])
		}
	}

	replacer := strings.NewReplacer(
		"{message}", message,
		"{page}", page,
		"{label}", label,
		"{total}", fmt.Sprintf("%d", a.pageCount),
		"{mode}", a.renderMode.String(),
		"{fit}", a.fitMode.String(),
		"{rot}", fmt.Sprintf("%.0f", a.rotation),
		"{zoom}", fmt.Sprintf("%.0f%%", a.zoom*100),
		"{dual}", boolWord(a.dualPage, "dual", "single"),
		"{cover}", boolWord(a.firstPageOffset, "cover", "flat"),
		"{search}", a.searchStatusCounter(),
		"{document}", a.docName,
		"{modified}", boolWord(a.unsaved, "[+] ", ""),
		"{input}", inputToken,
		"{prompt}", promptToken,
		"$$", "$",
	)
	return replacer.Replace(template)
}

func (a *App) pageLabel(page int) string {
	if page >= 0 && page < len(a.pageMetrics) && a.pageMetrics[page].label != "" {
		return a.pageMetrics[page].label
	}
	return fmt.Sprintf("%d", page+1)
}

func (a *App) inputPrefix() string {
	switch a.mode {
	case modeCommand:
		return ":"
	case modeGotoPage:
		return " GOTO "
	case modeSearch:
		return a.searchPromptToken()
	case modePassword:
		return " Password: "
	case modePrompt:
		return " " + a.promptState.label + ": "
	default:
		return ""
	}
}

func (a *App) searchPromptToken() string {
	if a.searchInput == searchModeBackward {
		return "?"
	}
	return "/"
}
