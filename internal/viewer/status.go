package viewer

import (
	"fmt"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

func (a *App) drawStatusBar(renderer *sdl.Renderer) error {
	h := a.config.StatusBarHeight
	y := a.winH - h
	if err := fillRect(renderer, sdl.FRect{X: 0, Y: float32(y), W: float32(a.winW), H: float32(h)}, a.statusBarColor()); err != nil {
		return err
	}
	left := a.formatStatusBar(a.config.StatusBarLeft)
	right := a.formatStatusBar(a.config.StatusBarRight)
	vertOffset := (h + a.fontFace.Metrics().Ascent.Ceil() - a.fontFace.Metrics().Descent.Ceil()) / 2
	if err := drawText(renderer, a.fontFace, left, 8, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	if err := a.drawInputCursor(renderer, y); err != nil {
		return err
	}
	rw := measureText(a.fontFace, right)
	if err := drawText(renderer, a.fontFace, right, a.winW-rw-8, y+vertOffset, a.foregroundColor()); err != nil {
		return err
	}
	return nil
}

func (a *App) formatStatusBar(template string) string {
	result := template
	replacements := map[string]string{
		"{message}":  a.message,
		"{page}":     fmt.Sprintf("%d", a.page+1),
		"{total}":    fmt.Sprintf("%d", a.pageCount),
		"{mode}":     a.renderMode,
		"{fit}":      a.fitMode,
		"{rot}":      fmt.Sprintf("%.0f", a.rotation),
		"{zoom}":     fmt.Sprintf("%.0f%%", a.zoom*100),
		"{dual}":     boolWord(a.dualPage, "dual", "single"),
		"{cover}":    boolWord(a.firstPageOffset, "cover", "flat"),
		"{search}":   a.searchStatusCounter(),
		"{document}": a.docName,
		"$$":         "\x00",
	}

	switch a.mode {
	case modeCommand:
		replacements["{message}"] = ":" + a.input
		replacements["{input}"] = a.input
	case modeGotoPage:
		replacements["{message}"] = " GOTO " + a.input
		replacements["{input}"] = a.input
	case modeSearch:
		prompt := a.searchPromptToken()
		replacements["{message}"] = prompt + a.input
		replacements["{input}"] = a.input
		replacements["{prompt}"] = prompt
	default:
		replacements["{input}"] = ""
		replacements["{prompt}"] = ""
	}

	if a.dualPage && len(a.rows) > 0 && a.page >= 0 && a.page < len(a.pageToRow) {
		row := a.rows[a.pageToRow[a.page]]
		if len(row.pages) >= 2 {
			replacements["{page}"] = fmt.Sprintf("%d-%d", row.pages[0]+1, row.pages[len(row.pages)-1]+1)
		}
	}

	for k, v := range replacements {
		result = strings.ReplaceAll(result, k, v)
	}
	return strings.ReplaceAll(result, "\x00", "$")
}

func (a *App) drawInputCursor(renderer *sdl.Renderer, barY int) error {
	if a.mode == modeNormal {
		return nil
	}
	prefix := a.inputPrefix()
	left, _ := splitAtRune(a.input, a.inputCursor)
	x := 8 + measureText(a.fontFace, prefix+left)
	fg := a.foregroundColor()
	if err := renderer.SetDrawColor(fg.R, fg.G, fg.B, fg.A); err != nil {
		return err
	}
	return renderer.DrawLine(int32(x), int32(barY+6), int32(x), int32(barY+22))
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
