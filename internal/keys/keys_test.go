package keys

import (
	"slices"
	"testing"
)

func TestNormalize(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"gg", "gg"},
		{"J", "J"},
		{"g?", "g?"},
		{" ", "<Space>"},
		{"<space>", "<Space>"},
		{"<C-w>j", "<C-w>j"},
		{"<c-P>", "<C-p>"},
		{"<C-S-r>", "<C-S-r>"},
		{"<S-C-R>", "<C-S-r>"},
		{"<S-D-C-x>", "<C-D-S-x>"},
		{"<C-S>", "<C-s>"},
		{"<S-j>", "J"},
		{"<j>", "j"},
		{"<Enter>", "<CR>"},
		{"<return>", "<CR>"},
		{"<C-Enter>", "<C-CR>"},
		{"<S-Return>", "<S-CR>"},
		{"<Escape><PageDown>", "<Esc><PgDn>"},
		{"<S-tab>", "<S-Tab>"},
		{"<f5>", "<F5>"},
		{"<KPLUS>", "<kPlus>"},
		{"<C-->", "<C-->"},
		{"<C-=>", "<C-=>"},
		{"<lt>", "<lt>"},
		{"<C-lt>", "<C-lt>"},
		{"<C-gt>", "<C-gt>"},
		{"<gt>", ">"},
		{">", ">"},
		{"é<C-É>", "é<C-é>"},
	}
	for _, tt := range tests {
		got, err := Normalize(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestNormalizeRejectsInvalidKeys(t *testing.T) {
	for _, in := range []string{
		"",
		"<",
		"a<C-x",
		"<>",
		"<C-Sapce>",
		"<Keypad-1>",
		"<A-j>",
		"<M-x>",
		"<C->>",
		"<S-/>",
		"<S-1>",
		"\t",
		"\xff",
	} {
		if got, err := Normalize(in); err == nil {
			t.Errorf("Normalize(%q) = %q, want an error", in, got)
		}
	}
}

func TestFormatRoundTrips(t *testing.T) {
	var keys []Key
	for _, named := range namedKeys {
		keys = append(keys, Key{Name: named.Name}, Key{Mod: Ctrl | Cmd | Shift, Name: named.Name})
	}
	for _, r := range "a-<>é" {
		keys = append(keys, Key{Name: string(r)}, Key{Mod: Ctrl | Shift, Name: string(r)})
	}
	for _, key := range keys {
		seq, err := ParseSequence(key.String())
		if err != nil || !slices.Equal(seq, []Key{key}) {
			t.Errorf("ParseSequence(%q) = %v, %v; want [%v]", key.String(), seq, err, key)
		}
	}
}

func TestText(t *testing.T) {
	tests := []struct {
		key  Key
		want string
		ok   bool
	}{
		{Key{Name: "j"}, "j", true},
		{Key{Name: "<"}, "<", true},
		{Key{Name: "Space"}, " ", true},
		{Key{Mod: Shift, Name: "Space"}, "", false},
		{Key{Mod: Ctrl, Name: "j"}, "", false},
		{Key{Name: "CR"}, "", false},
	}
	for _, tt := range tests {
		if got, ok := tt.key.Text(); got != tt.want || ok != tt.ok {
			t.Errorf("%v.Text() = %q, %v; want %q, %v", tt.key, got, ok, tt.want, tt.ok)
		}
	}
}

func TestNamedKeysAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, key := range namedKeys {
		for _, name := range append([]string{key.Name}, key.Aliases...) {
			if seen[name] {
				t.Errorf("key name %q is listed twice", name)
			}
			seen[name] = true
		}
	}
	if len(seen) != len(canonicalNames) {
		t.Errorf("%d names differ only in case", len(seen)-len(canonicalNames))
	}
}

func TestNormalizeMouseEvent(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{" wheel_down ", "wheel_down"},
		{"Middle_Down", "middle_down"},
		{"left-down", "left_down"},
		{"<c-wheel_down>", "<C-wheel_down>"},
		{"<C-wheel-up>", "<C-wheel_up>"},
		{"CTRL_wheel_up", "<C-wheel_up>"},
		{"ctrl-wheel-down", "<C-wheel_down>"},
	}
	for _, tt := range tests {
		got, err := NormalizeMouseEvent(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("NormalizeMouseEvent(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
	for _, in := range []string{"", "left_dwn", "wheel", "<C-left_down>", "ctrl_wheel_left"} {
		if got, err := NormalizeMouseEvent(in); err == nil {
			t.Errorf("NormalizeMouseEvent(%q) = %q, want an error", in, got)
		}
	}
	for _, event := range mouseEvents {
		if got, err := NormalizeMouseEvent(event.Name); err != nil || got != event.Name {
			t.Errorf("NormalizeMouseEvent(%q) = %q, %v; want it unchanged", event.Name, got, err)
		}
	}
}
