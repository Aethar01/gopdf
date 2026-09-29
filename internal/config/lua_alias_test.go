package config

import (
	"path/filepath"
	"testing"
)

func TestLuaOptionsShortAlias(t *testing.T) {
	dir := t.TempDir()
	rt, err := Open(filepath.Join(dir, "missing.lua"), filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if dirty, err := rt.Eval(`
assert(gopdf.o == gopdf.options)
gopdf.o.page_gap_vertical = 17
assert(gopdf.options.page_gap_vertical == 17)
`); !dirty || err != nil {
		t.Fatalf("expected gopdf.o to mutate options, dirty=%v err=%v", dirty, err)
	}
	if got := rt.Config().PageGapVertical; got != 17 {
		t.Fatalf("expected page_gap_vertical=17, got %d", got)
	}
	if got := rt.Config().PageGap; got != 17 {
		t.Fatalf("expected page_gap=17 mirror, got %d", got)
	}
}

func TestLuaStatusBarShortAlias(t *testing.T) {
	dir := t.TempDir()
	rt, err := Open(filepath.Join(dir, "missing.lua"), filepath.Join(dir, "doc.pdf"))
	if err != nil {
		t.Fatal(err)
	}
	defer rt.Close()

	if _, err := rt.Eval(`
assert(gopdf.sb == gopdf.status_bar)
gopdf.sb.visible = false
gopdf.sb.right = "{page}"
`); err != nil {
		t.Fatal(err)
	}
	if cfg := rt.Config(); cfg.StatusBarVisible || cfg.StatusBarRight != "{page}" {
		t.Fatalf("expected gopdf.sb to set the status bar, visible=%v right=%q", cfg.StatusBarVisible, cfg.StatusBarRight)
	}
}
