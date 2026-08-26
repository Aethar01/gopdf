package viewer

import (
	"unicode"
	"unicode/utf8"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

type textInput struct {
	Value          string
	Cursor         int
	Anchor         int
	mouseSelecting bool
}

func (t *textInput) Reset() {
	t.Value = ""
	t.Cursor = 0
	t.Anchor = 0
	t.mouseSelecting = false
}

func (t *textInput) Set(value string) {
	t.Value = value
	t.Cursor = utf8.RuneCountInString(value)
	t.Anchor = t.Cursor
	t.mouseSelecting = false
}

func (t *textInput) InsertRune(r rune) {
	t.InsertText(string(r))
}

func (t *textInput) InsertText(text string) {
	if text == "" {
		return
	}
	t.DeleteSelection()
	left, right := splitAtRune(t.Value, t.Cursor)
	t.Value = left + text + right
	t.Cursor += utf8.RuneCountInString(text)
	t.Anchor = t.Cursor
}

func (t *textInput) ReplaceRange(start, end int, value string) {
	left, _ := splitAtRune(t.Value, start)
	_, after := splitAtRune(t.Value, end)
	t.Value = left + value + after
	t.Cursor = start + utf8.RuneCountInString(value)
	t.Anchor = t.Cursor
}

func (t *textInput) Backspace() {
	if t.DeleteSelection() {
		return
	}
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	left, right := splitAtRune(t.Value, t.Cursor)
	_, size := lastRune(left)
	t.Value = left[:len(left)-size] + right
	t.Cursor--
	t.Anchor = t.Cursor
}

func (t *textInput) Delete() {
	if t.DeleteSelection() {
		return
	}
	runes := []rune(t.Value)
	if t.Cursor >= len(runes) {
		return
	}
	left, right := splitAtRune(t.Value, t.Cursor)
	_, after := splitAtRune(right, 1)
	t.Value = left + after
	t.Anchor = t.Cursor
}

func (t *textInput) DeleteWordLeft() {
	if t.DeleteSelection() {
		return
	}
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	runes := []rune(t.Value)
	end := clampInt(t.Cursor, 0, len(runes))
	start := wordLeftPosition(runes, end)
	t.Value = string(runes[:start]) + string(runes[end:])
	t.Cursor = start
	t.Anchor = start
}

func (t *textInput) Move(delta int) {
	t.MoveSelecting(delta, textInputShiftDown())
}

func (t *textInput) MoveSelecting(delta int, extend bool) {
	length := utf8.RuneCountInString(t.Value)
	if !extend && t.HasSelection() {
		start, end, _ := t.SelectionRange()
		if delta < 0 {
			t.SetCursor(start, false)
		} else if delta > 0 {
			t.SetCursor(end, false)
		}
		return
	}
	t.SetCursor(clampInt(t.Cursor+delta, 0, length), extend)
}

func (t *textInput) MoveWordLeft() {
	t.MoveWordLeftSelecting(textInputShiftDown())
}

func (t *textInput) MoveWordLeftSelecting(extend bool) {
	if !extend && t.HasSelection() {
		start, _, _ := t.SelectionRange()
		t.SetCursor(start, false)
		return
	}
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	runes := []rune(t.Value)
	pos := wordLeftPosition(runes, clampInt(t.Cursor, 0, len(runes)))
	t.SetCursor(pos, extend)
}

func (t *textInput) MoveWordRight() {
	t.MoveWordRightSelecting(textInputShiftDown())
}

func (t *textInput) MoveWordRightSelecting(extend bool) {
	if !extend && t.HasSelection() {
		_, end, _ := t.SelectionRange()
		t.SetCursor(end, false)
		return
	}
	runes := []rune(t.Value)
	if t.Cursor >= len(runes) {
		return
	}
	pos := wordRightPosition(runes, clampInt(t.Cursor, 0, len(runes)))
	t.SetCursor(pos, extend)
}

func (t *textInput) SetCursor(pos int, extend bool) {
	pos = clampInt(pos, 0, utf8.RuneCountInString(t.Value))
	if !extend {
		t.Anchor = pos
	}
	t.Cursor = pos
}

func (t textInput) HasSelection() bool {
	return t.Cursor != t.Anchor
}

func (t textInput) SelectionRange() (int, int, bool) {
	if !t.HasSelection() {
		return 0, 0, false
	}
	start, end := t.Anchor, t.Cursor
	if start > end {
		start, end = end, start
	}
	length := utf8.RuneCountInString(t.Value)
	start = clampInt(start, 0, length)
	end = clampInt(end, 0, length)
	return start, end, start != end
}

func (t textInput) SelectedText() (string, bool) {
	start, end, ok := t.SelectionRange()
	if !ok {
		return "", false
	}
	_, rest := splitAtRune(t.Value, start)
	selected, _ := splitAtRune(rest, end-start)
	return selected, true
}

func (t *textInput) DeleteSelection() bool {
	start, end, ok := t.SelectionRange()
	if !ok {
		return false
	}
	left, _ := splitAtRune(t.Value, start)
	_, right := splitAtRune(t.Value, end)
	t.Value = left + right
	t.Cursor = start
	t.Anchor = start
	return true
}

func (t textInput) Left() string {
	left, _ := splitAtRune(t.Value, t.Cursor)
	return left
}

func wordLeftPosition(runes []rune, pos int) int {
	for pos > 0 && unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	return pos
}

func wordRightPosition(runes []rune, pos int) int {
	for pos < len(runes) && unicode.IsSpace(runes[pos]) {
		pos++
	}
	for pos < len(runes) && !unicode.IsSpace(runes[pos]) {
		pos++
	}
	return pos
}

func textInputShiftDown() bool {
	return sdl.GetModState()&sdl.KeymodShift != 0
}
