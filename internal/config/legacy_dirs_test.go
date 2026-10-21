package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeTestFile(t *testing.T, path, text string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func readTestFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestMigrateLegacyDirCopiesAndRewrites(t *testing.T) {
	parent := t.TempDir()
	old, dst := filepath.Join(parent, "gopdf"), filepath.Join(parent, "onon")
	writeTestFile(t, filepath.Join(old, "config.lua"), "local json = require(\"gopdf.json\")\ngopdf.bind(\"j\", \"next_page\")\n-- see ~/.config/gopdf/themes; gopdf_docs stays\n")
	writeTestFile(t, filepath.Join(old, "session.sqlite"), "gopdf binary data")
	writeTestFile(t, filepath.Join(old, "plugins", "clock", "gopdf-plugin.json"), `{"id":"clock","module":"gopdf_clock"}`)
	writeTestFile(t, filepath.Join(old, "plugins", "clock", "init.lua"), "gopdf.plugin.register(\"clock\")\n")

	note, err := migrateLegacyDir(old, dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note, "copied settings") {
		t.Fatalf("note = %q", note)
	}
	if got, want := readTestFile(t, filepath.Join(dst, "config.lua")), "local json = require(\"onon.json\")\nonon.bind(\"j\", \"next_page\")\n-- see ~/.config/onon/themes; gopdf_docs stays\n"; got != want {
		t.Errorf("config.lua = %q, want %q", got, want)
	}
	if got := readTestFile(t, filepath.Join(dst, "session.sqlite")); got != "gopdf binary data" {
		t.Errorf("session.sqlite was changed: %q", got)
	}
	if got := readTestFile(t, filepath.Join(dst, "plugins", "clock", "onon-plugin.json")); got != `{"id":"clock","module":"gopdf_clock"}` {
		t.Errorf("manifest = %q", got)
	}
	if got := readTestFile(t, filepath.Join(dst, "plugins", "clock", "init.lua")); got != "onon.plugin.register(\"clock\")\n" {
		t.Errorf("init.lua = %q", got)
	}
	if _, err := os.Stat(filepath.Join(dst, "plugins", "clock", "gopdf-plugin.json")); err == nil {
		t.Error("old manifest name was copied too")
	}
	if got := readTestFile(t, filepath.Join(old, "config.lua")); !strings.Contains(got, "gopdf.bind") {
		t.Error("the old directory was changed")
	}
	if _, err := os.Stat(dst + ".migrating"); err == nil {
		t.Error("temporary copy left behind")
	}
}

func TestMigrateLegacyDirLeavesExistingTarget(t *testing.T) {
	parent := t.TempDir()
	old, dst := filepath.Join(parent, "gopdf"), filepath.Join(parent, "onon")
	writeTestFile(t, filepath.Join(old, "config.lua"), "gopdf.bind(\"j\", \"next_page\")\n")
	writeTestFile(t, filepath.Join(dst, "config.lua"), "-- mine\n")
	note, err := migrateLegacyDir(old, dst)
	if err != nil || note != "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if got := readTestFile(t, filepath.Join(dst, "config.lua")); got != "-- mine\n" {
		t.Errorf("existing config.lua overwritten: %q", got)
	}
}

func TestMigrateLegacyDirNothingToDo(t *testing.T) {
	parent := t.TempDir()
	note, err := migrateLegacyDir(filepath.Join(parent, "gopdf"), filepath.Join(parent, "onon"))
	if err != nil || note != "" {
		t.Fatalf("note %q, err %v", note, err)
	}
	if _, err := os.Stat(filepath.Join(parent, "onon")); err == nil {
		t.Error("created onon with nothing to migrate")
	}
}

func TestMigrateLegacyDirLeavesLink(t *testing.T) {
	parent := t.TempDir()
	target := filepath.Join(parent, "dotfiles", "gopdf")
	writeTestFile(t, filepath.Join(target, "config.lua"), "gopdf.bind(\"j\", \"next_page\")\n")
	old, dst := filepath.Join(parent, "gopdf"), filepath.Join(parent, "onon")
	if err := os.Symlink(target, old); err != nil {
		t.Skip("symbolic links unavailable:", err)
	}
	note, err := migrateLegacyDir(old, dst)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note, "is a link") {
		t.Errorf("note = %q", note)
	}
	if _, err := os.Lstat(dst); err == nil {
		t.Error("followed the link")
	}
}

func TestMigrateLegacyDirsUnderHome(t *testing.T) {
	if os.PathSeparator == '\\' {
		t.Skip("Windows keeps settings under APPDATA, not HOME")
	}
	home := t.TempDir()
	t.Setenv("HOME", home)
	for _, name := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_CACHE_HOME"} {
		t.Setenv(name, "")
	}
	writeTestFile(t, filepath.Join(home, ".config", "gopdf", "config.lua"), "gopdf.bind(\"j\", \"next_page\")\n")
	migrationNotes = nil
	t.Cleanup(func() { migrationNotes = nil })

	MigrateLegacyDirs()
	if got := readTestFile(t, filepath.Join(home, ".config", "onon", "config.lua")); got != "onon.bind(\"j\", \"next_page\")\n" {
		t.Errorf("config.lua = %q", got)
	}
	if len(migrationNotes) != 1 {
		t.Errorf("notes = %q, want one", migrationNotes)
	}
	MigrateLegacyDirs()
	if len(migrationNotes) != 1 {
		t.Errorf("second run added notes: %q", migrationNotes)
	}
}
