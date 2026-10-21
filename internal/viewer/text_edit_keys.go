package viewer

import (
	"runtime"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// While a prompt is open, the keys here edit its text before any binding
// of theirs runs. Every system has textEditKeys; a system's text fields
// have keys of their own, added from platformTextEditKeys there, as
// actions.Platforms adds a system's bindings.

// textEdit is an edit a key makes to the prompt's text.
type textEdit func(*App)

func editing(edit func(*textInput)) textEdit {
	return func(a *App) { a.editInput(edit) }
}

var (
	moveLeft        = editing(func(t *textInput) { t.Move(-1) })
	moveRight       = editing(func(t *textInput) { t.Move(1) })
	selectLeft      = editing(func(t *textInput) { t.MoveSelecting(-1, true) })
	selectRight     = editing(func(t *textInput) { t.MoveSelecting(1, true) })
	moveWordLeft    = editing((*textInput).MoveWordLeft)
	moveWordRight   = editing((*textInput).MoveWordRight)
	selectWordLeft  = editing(func(t *textInput) { t.MoveWordLeftSelecting(true) })
	selectWordRight = editing(func(t *textInput) { t.MoveWordRightSelecting(true) })
	moveToStart     = editing(func(t *textInput) { t.MoveToStart(false) })
	moveToEnd       = editing(func(t *textInput) { t.MoveToEnd(false) })
	selectToStart   = editing(func(t *textInput) { t.MoveToStart(true) })
	selectToEnd     = editing(func(t *textInput) { t.MoveToEnd(true) })
	deleteLeft      = editing((*textInput).Backspace)
	deleteRight     = editing((*textInput).Delete)
	deleteWordLeft  = editing((*textInput).DeleteWordLeft)
	deleteWordRight = editing((*textInput).DeleteWordRight)
	deleteToStart   = editing((*textInput).DeleteToStart)
	deleteToEnd     = editing((*textInput).DeleteToEnd)
	pasteClipboard  = textEdit(func(a *App) {
		a.editInput(func(t *textInput) { t.InsertText(a.GetClipboard()) })
	})
)

// textEditKeys are the editing keys every system's prompts have.
var textEditKeys = map[string]textEdit{
	"<Left>":      moveLeft,
	"<Right>":     moveRight,
	"<S-Left>":    selectLeft,
	"<S-Right>":   selectRight,
	"<C-Left>":    moveWordLeft,
	"<C-Right>":   moveWordRight,
	"<C-S-Left>":  selectWordLeft,
	"<C-S-Right>": selectWordRight,
	"<Home>":      moveToStart,
	"<End>":       moveToEnd,
	"<S-Home>":    selectToStart,
	"<S-End>":     selectToEnd,
	"<BS>":        deleteLeft,
	"<S-BS>":      deleteLeft,
	"<Del>":       deleteRight,
	"<C-BS>":      deleteWordLeft,
	"<C-w>":       deleteWordLeft,
	"<C-Del>":     deleteWordRight,
	"<C-v>":       pasteClipboard,
}

// platformTextEditKeys holds each system's own editing keys, by
// runtime.GOOS name.
var platformTextEditKeys = map[string]map[string]textEdit{
	"darwin": macOSTextEditKeys,
}

// macOSTextEditKeys are the keys Mac text fields edit with: Option moves
// and deletes by word, Cmd to the ends of the line, and Ctrl with a letter
// as in Emacs.
var macOSTextEditKeys = map[string]textEdit{
	"<A-Left>":    moveWordLeft,
	"<A-Right>":   moveWordRight,
	"<A-S-Left>":  selectWordLeft,
	"<A-S-Right>": selectWordRight,
	"<D-Left>":    moveToStart,
	"<D-Right>":   moveToEnd,
	"<D-S-Left>":  selectToStart,
	"<D-S-Right>": selectToEnd,
	"<D-Up>":      moveToStart,
	"<D-Down>":    moveToEnd,
	"<A-BS>":      deleteWordLeft,
	"<A-Del>":     deleteWordRight,
	"<D-BS>":      deleteToStart,
	"<C-a>":       moveToStart,
	"<C-e>":       moveToEnd,
	"<C-b>":       moveLeft,
	"<C-f>":       moveRight,
	"<C-h>":       deleteLeft,
	"<C-d>":       deleteRight,
	"<C-k>":       deleteToEnd,
}

// textEditFor returns the edit key makes in a prompt on goos.
func textEditFor(goos, key string) (textEdit, bool) {
	if edit, ok := platformTextEditKeys[goos][key]; ok {
		return edit, true
	}
	edit, ok := textEditKeys[key]
	return edit, ok
}

// handleInputEditKey makes the edit a key press makes in the prompt, if
// it makes one.
func (a *App) handleInputEditKey(e *sdl.KeyboardEvent) bool {
	key, ok := keyToken(e)
	if !ok {
		return false
	}
	edit, ok := textEditFor(runtime.GOOS, key.String())
	if ok {
		edit(a)
	}
	return ok
}
