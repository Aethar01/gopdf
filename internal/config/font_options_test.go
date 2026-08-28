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

func TestUIFontPathOverridePrecedence(t *testing.T) {
	cfg := Default()
	cfg.UIFont = "Iosevka"
	cfg.UIFontStyle = "italic"
	cfg.UIFontWeight = 700
	syncUIFontPath(&cfg)
	if got := cfg.UIFontPath; got == "" || got == cfg.UIFontPathOverride {
		t.Fatalf("expected generated system font selector, got %q", got)
	}

	cfg.UIFontPathOverride = "/tmp/custom.ttf"
	syncUIFontPath(&cfg)
	if got := cfg.UIFontPath; got != "/tmp/custom.ttf" {
		t.Fatalf("path override = %q", got)
	}

	cfg.UIFontPathOverride = ""
	syncUIFontPath(&cfg)
	if got := cfg.UIFontPath; got == "" || got == "/tmp/custom.ttf" {
		t.Fatalf("expected system font selector after clearing override, got %q", got)
	}
}
