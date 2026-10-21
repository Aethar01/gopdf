package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRendererOption(t *testing.T) {
	if got := Default().Renderer; got != "auto" {
		t.Fatalf("default renderer = %q, want auto", got)
	}
	for lua, want := range map[string]string{
		`gopdf.options.renderer = " Vulkan "`: "vulkan",
		`gopdf.options.renderer = "gpu"`:      "gpu",
		`gopdf.options.renderer = ""`:         "auto",
	} {
		path := filepath.Join(t.TempDir(), "config.lua")
		if err := os.WriteFile(path, []byte(lua), 0o644); err != nil {
			t.Fatal(err)
		}
		rt, err := Open(path, "")
		if err != nil {
			t.Fatal(err)
		}
		if got := rt.Config().Renderer; got != want {
			t.Errorf("%s gives %q, want %q", lua, got, want)
		}
		rt.Close()
	}
}
