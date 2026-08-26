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
