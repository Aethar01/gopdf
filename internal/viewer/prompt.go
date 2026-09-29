package viewer

// A prompt asks one question in the status bar and passes the answer, taken
// exactly as typed, to a callback when Enter is pressed.

type promptState struct {
	label  string
	answer func(string)
}

func (a *App) askPrompt(label, initial string, answer func(string)) {
	a.closeAllUI()
	a.promptState = promptState{label: label, answer: answer}
	a.mode = modePrompt
	a.input.Set(initial)
}

func (a *App) answerPrompt(value string) {
	answer := a.promptState.answer
	a.promptState = promptState{}
	if answer != nil {
		answer(value)
	}
}
