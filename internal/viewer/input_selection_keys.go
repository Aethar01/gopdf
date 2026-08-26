package viewer

import "github.com/jupiterrider/purego-sdl3/sdl"

func (a *App) handleTextInputSelectionKey(e *sdl.KeyboardEvent) bool {
	if a.mode == modeNormal || e == nil || e.Mod&sdl.KeymodShift == 0 {
		return false
	}

	ctrl := e.Mod&sdl.KeymodCtrl != 0
	switch e.Key {
	case sdl.KeycodeLeft:
		if ctrl {
			a.editInput(func(input *textInput) { input.MoveWordLeftSelecting(true) })
		} else {
			a.editInput(func(input *textInput) { input.MoveSelecting(-1, true) })
		}
		return true
	case sdl.KeycodeRight:
		if ctrl {
			a.editInput(func(input *textInput) { input.MoveWordRightSelecting(true) })
		} else {
			a.editInput(func(input *textInput) { input.MoveSelecting(1, true) })
		}
		return true
	default:
		return false
	}
}
