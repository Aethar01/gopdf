package viewer

import (
	"fmt"
	"strings"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) drawStatusBar(renderer *sdl.Renderer) error {
	h := a.config.StatusBarHeight
	y := a.winH - h
	if err := fillRect(renderer, sdl.FRect{X: 0, Y: float32(y), W: float32(a.winW), H: float32(h)}, a.statusBarColor()); err != nil {
		return err
	}
	left := a.formatStatusBar(a.config.StatusBarLeft)
	right := a.formatStatusBar(a.config.StatusBarRight)
	pad := a.config.StatusBarPadding
	vertOffset := (h + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
	if err := drawText(renderer, a.fontFace, left, pad, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	if err := a.drawInputCursor(renderer, y, pad, vertOffset); err != nil {
		return err
	}
	rw := measureText(a.fontFace, right)
	if err := drawText(renderer, a.fontFace, right, a.winW-rw-pad, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	return nil
}

func (a *App) formatStatusBar(template string) string {
	message := a.message
	var inputToken, promptToken string
	switch a.mode {
	case modeCommand:
		message = ":" + a.input
		inputToken = a.input
	case modeGotoPage:
		message = " GOTO " + a.input
		inputToken = a.input
	case modeSearch:
		promptToken = a.searchPromptToken()
		message = promptToken + a.input
		inputToken = a.input
	}

	page := fmt.Sprintf("%d", a.page+1)
	if a.dualPage && len(a.rows) > 0 && a.page >= 0 && a.page < len(a.pageToRow) {
		row := a.rows[a.pageToRow[a.page]]
		if len(row.pages) >= 2 {
			page = fmt.Sprintf("%d-%d", row.pages[0]+1, row.pages[len(row.pages)-1]+1)
		}
	}

	replacer := strings.NewReplacer(
		"{message}", message,
		"{page}", page,
		"{total}", fmt.Sprintf("%d", a.pageCount),
		"{mode}", a.renderMode,
		"{fit}", a.fitMode,
		"{rot}", fmt.Sprintf("%.0f", a.rotation),
		"{zoom}", fmt.Sprintf("%.0f%%", a.zoom*100),
		"{dual}", boolWord(a.dualPage, "dual", "single"),
		"{cover}", boolWord(a.firstPageOffset, "cover", "flat"),
		"{search}", a.searchStatusCounter(),
		"{document}", a.docName,
		"{input}", inputToken,
		"{prompt}", promptToken,
		"$$", "$",
	)
	return replacer.Replace(template)
}

func (a *App) drawInputCursor(renderer *sdl.Renderer, barY, pad, vertOffset int) error {
	if a.mode == modeNormal {
		return nil
	}
	prefix := a.inputPrefix()
	left, _ := splitAtRune(a.input, a.inputCursor)
	x := pad + measureText(a.fontFace, prefix+left)
	fg := a.foregroundColor()
	if !sdl.SetRenderDrawColor(renderer, fg.R, fg.G, fg.B, fg.A) {
		return sdlError("set draw color")
	}
	mt := a.fontFace.Metrics()
	cursorTop := barY + vertOffset - mt.Ascent.Ceil()
	cursorBot := barY + vertOffset + mt.Descent.Ceil()
	return renderBool(sdl.RenderLine(renderer, float32(x), float32(cursorTop), float32(x), float32(cursorBot)), "draw line")
}

func (a *App) inputPrefix() string {
	switch a.mode {
	case modeCommand:
		return ":"
	case modeGotoPage:
		return " GOTO "
	case modeSearch:
		return a.searchPromptToken()
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
