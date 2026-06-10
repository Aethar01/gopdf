package config

import (
	"runtime"
	"testing"
)

func TestDocumentSessionRoundTrip(t *testing.T) {
	setTestDataDir(t)
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

func TestSessionDatabaseDefaultOn(t *testing.T) {
	if !Default().SessionDatabase {
		t.Fatal("session database should default to on")
	}
}

func TestRecentFiles(t *testing.T) {
	setTestDataDir(t)
	one := AbsoluteDocumentPath("/tmp/one.pdf")
	two := AbsoluteDocumentPath("/tmp/two.pdf")
	if err := RecordRecentFile("/tmp/one.pdf", 10); err != nil {
		t.Fatal(err)
	}
	if err := RecordRecentFile("/tmp/two.pdf", 10); err != nil {
		t.Fatal(err)
	}
	if err := RecordRecentFile("/tmp/one.pdf", 10); err != nil {
		t.Fatal(err)
	}
	got := RecentFiles(10)
	if len(got) != 2 {
		t.Fatalf("RecentFiles returned %d entries: %v", len(got), got)
	}
	if got[0] != one || got[1] != two {
		t.Fatalf("RecentFiles = %v, want [%s %s]", got, one, two)
	}
}

func TestRecentFilesMaxEntries(t *testing.T) {
	setTestDataDir(t)
	two := AbsoluteDocumentPath("/tmp/two.pdf")
	three := AbsoluteDocumentPath("/tmp/three.pdf")
	if err := RecordRecentFile("/tmp/one.pdf", 2); err != nil {
		t.Fatal(err)
	}
	if err := RecordRecentFile("/tmp/two.pdf", 2); err != nil {
		t.Fatal(err)
	}
	if err := RecordRecentFile("/tmp/three.pdf", 2); err != nil {
		t.Fatal(err)
	}
	got := RecentFiles(10)
	if len(got) != 2 {
		t.Fatalf("RecentFiles returned %d entries: %v", len(got), got)
	}
	if got[0] != three || got[1] != two {
		t.Fatalf("RecentFiles = %v, want [%s %s]", got, three, two)
	}
}

func TestDocumentMarkRoundTrip(t *testing.T) {
	setTestDataDir(t)
	want := DocumentMark{Page: 2, ScrollX: 3, ScrollY: 4, AnchorPage: 5, AnchorX: 6, AnchorY: 7, AnchorValid: true}
	if err := SetDocumentMark("/tmp/doc.pdf", "a", want); err != nil {
		t.Fatal(err)
	}
	got, ok := GetDocumentMark("/tmp/doc.pdf", "a")
	if !ok {
		t.Fatal("saved mark was not found")
	}
	if got != want {
		t.Fatalf("mark = %#v, want %#v", got, want)
	}
}

func setTestDataDir(t *testing.T) {
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
