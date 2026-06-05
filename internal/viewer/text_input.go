package viewer

import (
	"unicode"
	"unicode/utf8"
)

type textInput struct {
	Value  string
	Cursor int
}

func (t *textInput) Reset() {
	t.Value = ""
	t.Cursor = 0
}

func (t *textInput) Set(value string) {
	t.Value = value
	t.Cursor = utf8.RuneCountInString(value)
}

func (t *textInput) InsertRune(r rune) {
	t.InsertText(string(r))
}

func (t *textInput) InsertText(text string) {
	if text == "" {
		return
	}
	left, right := splitAtRune(t.Value, t.Cursor)
	t.Value = left + text + right
	t.Cursor += utf8.RuneCountInString(text)
}

func (t *textInput) ReplaceRange(start, end int, value string) {
	left, _ := splitAtRune(t.Value, start)
	_, after := splitAtRune(t.Value, end)
	t.Value = left + value + after
	t.Cursor = start + utf8.RuneCountInString(value)
}

func (t *textInput) Backspace() {
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	left, right := splitAtRune(t.Value, t.Cursor)
	_, size := lastRune(left)
	t.Value = left[:len(left)-size] + right
	t.Cursor--
}

func (t *textInput) Delete() {
	runes := []rune(t.Value)
	if t.Cursor >= len(runes) {
		return
	}
	left, right := splitAtRune(t.Value, t.Cursor)
	_, after := splitAtRune(right, 1)
	t.Value = left + after
}

func (t *textInput) DeleteWordLeft() {
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	runes := []rune(t.Value)
	end := clampInt(t.Cursor, 0, len(runes))
	start := end
	for start > 0 && unicode.IsSpace(runes[start-1]) {
		start--
	}
	for start > 0 && !unicode.IsSpace(runes[start-1]) {
		start--
	}
	t.Value = string(runes[:start]) + string(runes[end:])
	t.Cursor = start
}

func (t *textInput) Move(delta int) {
	t.Cursor = clampInt(t.Cursor+delta, 0, utf8.RuneCountInString(t.Value))
}

func (t *textInput) MoveWordLeft() {
	if t.Cursor <= 0 || t.Value == "" {
		return
	}
	runes := []rune(t.Value)
	pos := clampInt(t.Cursor, 0, len(runes))
	for pos > 0 && unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	for pos > 0 && !unicode.IsSpace(runes[pos-1]) {
		pos--
	}
	t.Cursor = pos
}

func (t *textInput) MoveWordRight() {
	runes := []rune(t.Value)
	if t.Cursor >= len(runes) {
		return
	}
	pos := clampInt(t.Cursor, 0, len(runes))
	for pos < len(runes) && unicode.IsSpace(runes[pos]) {
		pos++
	}
	for pos < len(runes) && !unicode.IsSpace(runes[pos]) {
		pos++
	}
	t.Cursor = pos
}

func (t textInput) Left() string {
	left, _ := splitAtRune(t.Value, t.Cursor)
	return left
}
