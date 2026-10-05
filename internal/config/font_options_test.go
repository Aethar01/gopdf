package config

import "testing"

func TestParseUIFontWeight(t *testing.T) {
	tests := map[string]int{
		"100":        100,
		"normal":     400,
		"regular":    400,
		"medium":     500,
		"semibold":   600,
		"semi-bold":  600,
		"bold":       700,
		"extra-bold": 800,
		"black":      900,
	}
	for input, want := range tests {
		got, err := parseUIFontWeight(input)
		if err != nil {
			t.Fatalf("parse %q: %v", input, err)
		}
		if got != want {
			t.Fatalf("parse %q = %d, want %d", input, got, want)
		}
	}
	for _, input := range []string{"", "99", "901", "very-bold"} {
		if _, err := parseUIFontWeight(input); err == nil {
			t.Fatalf("expected %q to be rejected", input)
		}
	}
}
