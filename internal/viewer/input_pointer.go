package viewer

import (
	"strings"
	"unicode/utf8"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) handleInputMouseButton(e *sdl.MouseButtonEvent) bool {
	if a.mode == modeNormal || !a.statusVisible() || e.Button != uint8(sdl.ButtonLeft) {
		return false
	}
	if e.Type == sdl.EventMouseButtonDown {
		pos, ok := a.inputPositionAt(float64(e.X), float64(e.Y), false)
		if !ok {
			return false
		}
		a.editInput(func(input *textInput) {
			input.SetCursor(pos, false)
			input.mouseSelecting = true
		})
		return true
	}
	if e.Type == sdl.EventMouseButtonUp && a.input.mouseSelecting {
		pos, _ := a.inputPositionAt(float64(e.X), float64(e.Y), true)
		a.editInput(func(input *textInput) {
			input.SetCursor(pos, true)
			input.mouseSelecting = false
		})
		return true
	}
	return false
}

func (a *App) handleInputMouseMotion(e *sdl.MouseMotionEvent) bool {
	if a.mode == modeNormal || !a.input.mouseSelecting {
		return false
	}
	if uint32(e.State)&uint32(sdl.ButtonLMask) == 0 {
		a.input.mouseSelecting = false
		return false
	}
	pos, _ := a.inputPositionAt(float64(e.X), float64(e.Y), true)
	a.editInput(func(input *textInput) { input.SetCursor(pos, true) })
	return true
}

func (a *App) inputPositionAt(x, y float64, dragging bool) (int, bool) {
	barY := a.winH - a.config.StatusBarHeight
	if !dragging && (y < float64(barY) || y > float64(a.winH)) {
		return 0, false
	}
	pad := a.config.StatusBarPadding
	prefix := a.inputPrefix()
	startX := float64(pad + measureText(a.fontFace, prefix))
	display := a.input.Value
	if a.mode == modePassword {
		display = strings.Repeat("*", utf8.RuneCountInString(display))
	}
	endX := startX + float64(measureText(a.fontFace, display))
	if !dragging && (x < startX-3 || x > maxFloat64(endX+5, startX+8)) {
		return 0, false
	}
	if x <= startX {
		return 0, true
	}
	runes := []rune(display)
	for i := range runes {
		left := float64(measureText(a.fontFace, string(runes[:i])))
		right := float64(measureText(a.fontFace, string(runes[:i+1])))
		if x-startX < (left+right)/2 {
			return i, true
		}
	}
	return len(runes), true
}

func maxFloat64(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
