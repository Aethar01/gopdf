//go:build !windows

package viewer

import (
	"slices"
	"testing"
)

func TestPrintCommandPassesOptionsBeforeFile(t *testing.T) {
	cmd := printCommand("/tmp/-odd name.pdf", []string{"-d", "office"})
	if want := []string{"lp", "-d", "office", "--", "/tmp/-odd name.pdf"}; !slices.Equal(cmd.Args, want) {
		t.Fatalf("args = %q, want %q", cmd.Args, want)
	}
}

func TestPrintSettingsLpArgs(t *testing.T) {
	s := printSettings{printer: "Epson_ET_3850", pages: "1-3,5", copies: 2, sides: "two-sided-long-edge"}
	want := []string{"-d", "Epson_ET_3850", "-P", "1-3,5", "-n", "2", "-o", "sides=two-sided-long-edge"}
	if got := s.lpArgs(); !slices.Equal(got, want) {
		t.Fatalf("lpArgs = %q, want %q", got, want)
	}
	if got := (printSettings{copies: 1}).lpArgs(); len(got) != 0 {
		t.Fatalf("defaults give %q, want no options", got)
	}
}

func TestParseDefaultPrinter(t *testing.T) {
	if name, ok := parseDefaultPrinter("system default destination: Office\n"); !ok || name != "Office" {
		t.Fatalf("parsed %q, %v", name, ok)
	}
	if _, ok := parseDefaultPrinter("no system default destination\n"); ok {
		t.Fatal("parsed a default where there is none")
	}
}
