package config

import (
	"runtime"
	"testing"
)

func TestDocumentSessionRoundTrip(t *testing.T) {
	setTestStateDir(t)
	path := "/tmp/example.pdf"
	want := DocumentSession{
		Page:            4,
		ScrollX:         12.5,
		ScrollY:         99.25,
		AnchorPage:      5,
		AnchorX:         10.25,
		AnchorY:         20.5,
		AnchorValid:     true,
		Zoom:            1.75,
		FitMode:         "manual",
		RenderMode:      "single",
		Rotation:        90,
		DualPage:        true,
		FirstPageOffset: false,
		StatusBarShown:  true,
		AltColors:       true,
	}
	if err := SetDocumentSession(path, want); err != nil {
		t.Fatal(err)
	}
	got, ok := GetDocumentSession(path)
	if !ok {
		t.Fatal("saved document session was not found")
	}
	if got != want {
		t.Fatalf("document session = %#v, want %#v", got, want)
	}
}

func TestSessionDatabaseDefaultOff(t *testing.T) {
	if Default().SessionDatabase {
		t.Fatal("session database should default to off")
	}
}

func setTestStateDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	switch runtime.GOOS {
	case "windows":
		t.Setenv("APPDATA", dir)
	case "darwin":
		t.Setenv("HOME", dir)
	default:
		t.Setenv("XDG_DATA_HOME", dir)
	}
}
