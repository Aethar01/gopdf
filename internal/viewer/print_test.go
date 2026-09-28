package viewer

import (
	"runtime"
	"slices"
	"testing"
)

func TestPrintCommandPassesOptionsBeforeFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows prints through the shell verb")
	}
	cmd := printCommand("/tmp/-odd name.pdf", []string{"-d", "office"})
	if want := []string{"lp", "-d", "office", "--", "/tmp/-odd name.pdf"}; !slices.Equal(cmd.Args, want) {
		t.Fatalf("args = %q, want %q", cmd.Args, want)
	}
}
