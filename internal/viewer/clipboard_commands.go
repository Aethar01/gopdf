package viewer

import "fmt"

func (a *App) copyActiveTextInputToClipboard() bool {
	if a.mode != modeNormal {
		text, ok := a.input.SelectedText()
		if !ok || text == "" {
			return true
		}
		if err := setSDLClipboardText(text); err != nil {
			a.message = "clipboard unavailable"
			return true
		}
		a.message = fmt.Sprintf("copied %d chars", len(text))
		return true
	}
	text, active := a.activeTextInputValue()
	if !active {
		return false
	}
	if text == "" {
		return true
	}
	if err := setSDLClipboardText(text); err != nil {
		a.message = "clipboard unavailable"
		return true
	}
	a.message = fmt.Sprintf("copied %d chars", len(text))
	return true
}

func (a *App) cutActiveTextInputToClipboard() bool {
	if a.mode != modeNormal {
		text, ok := a.input.SelectedText()
		if !ok || text == "" {
			return true
		}
		if err := setSDLClipboardText(text); err != nil {
			a.message = "clipboard unavailable"
			return true
		}
		a.editInput(func(input *textInput) { input.DeleteSelection() })
		a.message = fmt.Sprintf("cut %d chars", len(text))
		return true
	}
	text, active := a.activeTextInputValue()
	if !active {
		return false
	}
	if text == "" {
		return true
	}
	if err := setSDLClipboardText(text); err != nil {
		a.message = "clipboard unavailable"
		return true
	}
	switch {
	case a.outlineMenu.visible && a.outlineMenu.searching:
		a.updateOutlineSearchQuery("")
	case a.luaUI.visible && a.luaUI.searching:
		a.updateLuaUISearchQuery("")
	}
	a.message = fmt.Sprintf("cut %d chars", len(text))
	return true
}

func (a *App) pasteIntoActiveTextInput() bool {
	_, active := a.activeTextInputValue()
	if !active {
		return false
	}
	text := sdlGetClipboardText()
	if text == "" {
		return true
	}
	switch {
	case a.outlineMenu.visible && a.outlineMenu.searching:
		a.updateOutlineSearchQuery(a.outlineMenu.query + text)
	case a.luaUI.visible && a.luaUI.searching:
		a.updateLuaUISearchQuery(a.luaUI.query + text)
	case a.mode != modeNormal:
		a.editInput(func(input *textInput) { input.InsertText(text) })
	}
	return true
}

func (a *App) activeTextInputValue() (string, bool) {
	switch {
	case a.outlineMenu.visible && a.outlineMenu.searching:
		return a.outlineMenu.query, true
	case a.luaUI.visible && a.luaUI.searching:
		return a.luaUI.query, true
	case a.mode != modeNormal:
		return a.input.Value, true
	default:
		return "", false
	}
}
