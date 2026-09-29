// Package keys parses and formats the key sequences and mouse events that
// bindings are written in. The configuration, the viewer and the generated
// reference all go through it, so every key has exactly one spelling.
package keys

import (
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Mod is a set of modifier keys.
type Mod uint8

const (
	Ctrl Mod = 1 << iota
	Cmd      // Cmd on macOS; the Super or Windows key elsewhere.
	Shift
)

var modPrefixes = map[rune]Mod{'c': Ctrl, 'd': Cmd, 's': Shift}

// Key is one key press. Name is either a single printable character or the
// canonical name of a named key, such as "CR" or "F5".
//
// An unmodified printable key never carries Shift: Shift+j is the key "J".
type Key struct {
	Mod  Mod
	Name string
}

// String formats k the one way the configuration stores it: a printable
// character on its own, anything else in angle brackets with modifiers in
// C, D, S order, as in "j", "<CR>" or "<C-S-Tab>".
func (k Key) String() string {
	name := k.Name
	if k.Mod == 0 && utf8.RuneCountInString(name) == 1 && name != "<" {
		return name
	}
	switch name {
	case "<":
		name = "lt"
	case ">":
		name = "gt"
	}
	var b strings.Builder
	b.WriteByte('<')
	if k.Mod&Ctrl != 0 {
		b.WriteString("C-")
	}
	if k.Mod&Cmd != 0 {
		b.WriteString("D-")
	}
	if k.Mod&Shift != 0 {
		b.WriteString("S-")
	}
	b.WriteString(name)
	b.WriteByte('>')
	return b.String()
}

// Text returns the character k types, when it is an unmodified printable
// key or Space.
func (k Key) Text() (string, bool) {
	switch {
	case k.Mod != 0:
		return "", false
	case k.Name == "Space":
		return " ", true
	case utf8.RuneCountInString(k.Name) == 1:
		return k.Name, true
	}
	return "", false
}

// Format writes a sequence in canonical form.
func Format(seq []Key) string {
	var b strings.Builder
	for _, key := range seq {
		b.WriteString(key.String())
	}
	return b.String()
}

// Normalize parses s and returns it in canonical form, so that every
// spelling of a sequence names the same binding.
func Normalize(s string) (string, error) {
	seq, err := ParseSequence(s)
	if err != nil {
		return "", err
	}
	return Format(seq), nil
}

// ParseSequence parses keys written one after another, such as "gg",
// "<C-w>j" or "<S-Tab>".
func ParseSequence(s string) ([]Key, error) {
	if s == "" {
		return nil, fmt.Errorf("empty key sequence")
	}
	var seq []Key
	for s != "" {
		var key Key
		var err error
		if s[0] == '<' {
			key, s, err = parseBracketed(s)
		} else {
			r, size := utf8.DecodeRuneInString(s)
			key, err = printableKey(0, r)
			s = s[size:]
		}
		if err != nil {
			return nil, err
		}
		seq = append(seq, key)
	}
	return seq, nil
}

// parseBracketed parses the <...> key at the start of s and returns the
// rest of s after it.
func parseBracketed(s string) (Key, string, error) {
	end := strings.IndexByte(s, '>')
	if end < 0 {
		return Key{}, "", fmt.Errorf("%q has no closing >", s)
	}
	inner, rest := s[1:end], s[end+1:]
	var mod Mod
	for len(inner) > 2 && inner[1] == '-' {
		m, ok := modPrefixes[unicode.ToLower(rune(inner[0]))]
		if !ok {
			return Key{}, "", fmt.Errorf("unknown modifier %q", inner[:2])
		}
		mod |= m
		inner = inner[2:]
	}
	var key Key
	var err error
	switch lower := strings.ToLower(inner); {
	case utf8.RuneCountInString(inner) == 1:
		r, _ := utf8.DecodeRuneInString(inner)
		key, err = printableKey(mod, r)
	case lower == "lt":
		key, err = printableKey(mod, '<')
	case lower == "gt":
		key, err = printableKey(mod, '>')
	default:
		name, ok := canonicalNames[lower]
		if !ok {
			err = fmt.Errorf("unknown key name %q", inner)
		}
		key = Key{Mod: mod, Name: name}
	}
	return key, rest, err
}

// printableKey applies the modifier rules for a character key. With Ctrl or
// Cmd, a letter's case is ignored because Shift is written as S-. Shift on
// its own turns a letter upper case; on other characters it is refused,
// since what Shift types there depends on the keyboard layout.
func printableKey(mod Mod, r rune) (Key, error) {
	switch {
	case r == ' ':
		return Key{Mod: mod, Name: "Space"}, nil
	case r == utf8.RuneError || !unicode.IsPrint(r):
		return Key{}, fmt.Errorf("%q is not a printable key", r)
	case mod&(Ctrl|Cmd) != 0:
		r = unicode.ToLower(r)
	case mod == Shift:
		if !unicode.IsLetter(r) {
			return Key{}, fmt.Errorf("write the character Shift+%c types instead", r)
		}
		r, mod = unicode.ToUpper(r), 0
	}
	return Key{Mod: mod, Name: string(r)}, nil
}

// NamedKey is a key written by name in angle brackets. Consecutive entries
// with the same description share a row in the reference.
type NamedKey struct {
	Name        string
	Aliases     []string
	Description string
}

// NamedKeys lists every named key in reference order.
func NamedKeys() []NamedKey {
	return append([]NamedKey(nil), namedKeys...)
}

var namedKeys = func() []NamedKey {
	named := []NamedKey{
		{Name: "CR", Aliases: []string{"Enter", "Return"}, Description: "Enter, Return or keypad Enter."},
		{Name: "Esc", Aliases: []string{"Escape"}, Description: "Escape."},
		{Name: "Tab", Description: "Tab."},
		{Name: "BS", Aliases: []string{"Backspace"}, Description: "Backspace."},
		{Name: "Del", Aliases: []string{"Delete"}, Description: "Delete."},
		{Name: "Ins", Aliases: []string{"Insert"}, Description: "Insert."},
		{Name: "Space", Description: "Space."},
		{Name: "Up", Description: "Arrow keys."},
		{Name: "Down", Description: "Arrow keys."},
		{Name: "Left", Description: "Arrow keys."},
		{Name: "Right", Description: "Arrow keys."},
		{Name: "PgUp", Aliases: []string{"PageUp"}, Description: "Page Up."},
		{Name: "PgDn", Aliases: []string{"PageDown"}, Description: "Page Down."},
		{Name: "Home", Description: "Home."},
		{Name: "End", Description: "End."},
	}
	for i := 1; i <= 24; i++ {
		named = append(named, NamedKey{Name: fmt.Sprintf("F%d", i), Description: "Function keys."})
	}
	for i := 0; i <= 9; i++ {
		named = append(named, NamedKey{Name: fmt.Sprintf("k%d", i), Description: "Keypad digits."})
	}
	for _, name := range []string{"kPlus", "kMinus", "kMultiply", "kDivide", "kPoint"} {
		named = append(named, NamedKey{Name: name, Description: "Keypad operators."})
	}
	return named
}()

// canonicalNames maps each lower-case name and alias to its canonical name.
var canonicalNames = func() map[string]string {
	names := map[string]string{}
	for _, key := range namedKeys {
		names[strings.ToLower(key.Name)] = key.Name
		for _, alias := range key.Aliases {
			names[strings.ToLower(alias)] = key.Name
		}
	}
	return names
}()

// MouseEvent is a mouse event that can be bound. Consecutive entries with
// the same description share a row in the reference.
type MouseEvent struct {
	Name        string
	Description string
}

// MouseEvents lists every mouse event in reference order.
func MouseEvents() []MouseEvent {
	return append([]MouseEvent(nil), mouseEvents...)
}

var mouseEvents = []MouseEvent{
	{Name: "left_down", Description: "Pressing or releasing the left button."},
	{Name: "left_up", Description: "Pressing or releasing the left button."},
	{Name: "middle_down", Description: "The middle button."},
	{Name: "middle_up", Description: "The middle button."},
	{Name: "right_down", Description: "The right button."},
	{Name: "right_up", Description: "The right button."},
	{Name: "x1_down", Description: "The side buttons, usually back and forward."},
	{Name: "x1_up", Description: "The side buttons, usually back and forward."},
	{Name: "x2_down", Description: "The side buttons, usually back and forward."},
	{Name: "x2_up", Description: "The side buttons, usually back and forward."},
	{Name: "wheel_up", Description: "Each step of the vertical wheel."},
	{Name: "wheel_down", Description: "Each step of the vertical wheel."},
	{Name: "wheel_left", Description: "Each step of the horizontal wheel or a sideways tilt."},
	{Name: "wheel_right", Description: "Each step of the horizontal wheel or a sideways tilt."},
	{Name: "<C-wheel_up>", Description: "The vertical wheel with Ctrl held."},
	{Name: "<C-wheel_down>", Description: "The vertical wheel with Ctrl held."},
}

// NormalizeMouseEvent returns the canonical name of a mouse event. Names are
// case-insensitive, may be written in angle brackets like keys, accept - for
// _, and accept ctrl_ for <C-...>.
func NormalizeMouseEvent(s string) (string, error) {
	name := strings.ToLower(strings.TrimSpace(s))
	ctrl := false
	if inner, ok := strings.CutPrefix(name, "<"); ok && strings.HasSuffix(inner, ">") {
		name, ctrl = strings.CutPrefix(strings.TrimSuffix(inner, ">"), "c-")
	}
	name = strings.ReplaceAll(name, "-", "_")
	if inner, ok := strings.CutPrefix(name, "ctrl_"); ok {
		name, ctrl = inner, true
	}
	if ctrl {
		name = "<C-" + name + ">"
	}
	for _, event := range mouseEvents {
		if event.Name == name {
			return name, nil
		}
	}
	return "", fmt.Errorf("unknown mouse event %q", s)
}
