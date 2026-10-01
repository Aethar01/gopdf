package config

import (
	"os"
	"path/filepath"
	"testing"
)

// A config can require the Lua files beside it, wherever gopdf runs from:
// require("helper") loads helper.lua, and dots in a name are directories.
func TestConfigRequiresLuaFilesBesideIt(t *testing.T) {
	dir := t.TempDir()
	write := func(name, source string) {
		t.Helper()
		path := filepath.Join(dir, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("settings/gaps.lua", `return { page = 14 }`)
	write("helper.lua", `gopdf.options.status_bar_left = "from helper"`)
	write("layout/init.lua", `return "width"`)
	write("config.lua", `
gopdf.options.page_gap = require("settings.gaps").page
require("helper")
gopdf.options.fit_mode = require("layout")
`)
	t.Chdir(t.TempDir())
	rt, err := OpenWithOptions(filepath.Join(dir, "config.lua"), "", OpenOptions{NoPlugins: true})
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()
	cfg := rt.Config()
	if cfg.PageGap != 14 || cfg.StatusBarLeft != "from helper" || cfg.FitMode != "width" {
		t.Fatalf("page_gap=%d status_bar_left=%q fit_mode=%q; want the required files' values", cfg.PageGap, cfg.StatusBarLeft, cfg.FitMode)
	}
}
