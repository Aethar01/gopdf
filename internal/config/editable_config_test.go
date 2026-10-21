package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestEditableConfigPathCreatesOnlyAMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.lua")
	rt, err := Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	got, err := rt.EditableConfigPath()
	if err != nil || got != path {
		t.Fatalf("EditableConfigPath() = %q, %v; want %q", got, err, path)
	}
	data, err := os.ReadFile(path)
	if err != nil || !strings.HasPrefix(string(data), "-- gopdf configuration") {
		t.Fatalf("created config holds %q, %v", data, err)
	}

	if err := os.WriteFile(path, []byte("gopdf.options.page_gap = 4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.EditableConfigPath(); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(path); string(data) != "gopdf.options.page_gap = 4\n" {
		t.Fatalf("an existing config was rewritten: %q", data)
	}
}
