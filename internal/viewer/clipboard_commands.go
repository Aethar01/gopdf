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
	if view := a.activeModalUIView(); view != nil && view.searching {
		a.setUIViewQuery(view, "")
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
	if view := a.activeModalUIView(); view != nil && view.searching {
		a.setUIViewQuery(view, view.query+text)
	} else if a.mode != modeNormal {
		a.editInput(func(input *textInput) { input.InsertText(text) })
	}
	return true
}

func (a *App) activeTextInputValue() (string, bool) {
	if view := a.activeModalUIView(); view != nil && view.searching {
		return view.query, true
	}
	if a.mode != modeNormal {
		return a.input.Value, true
	}
	return "", false
}
