package config

import "testing"

func TestScrollOffOption(t *testing.T) {
	cfg := Default()
	if cfg.ScrollOff != 0 {
		t.Fatalf("expected scroll_off=0 by default, got %d", cfg.ScrollOff)
	}

	desc, ok := configOptions["scroll_off"]
	if !ok {
		t.Fatal("scroll_off option is not registered")
	}
	if err := desc.applyText(&cfg, "3"); err != nil {
		t.Fatal(err)
	}
	if cfg.ScrollOff != 3 {
		t.Fatalf("expected scroll_off=3, got %d", cfg.ScrollOff)
	}
	if err := desc.applyText(&cfg, "-2"); err != nil {
		t.Fatal(err)
	}
	if cfg.ScrollOff != 0 {
		t.Fatalf("expected negative scroll_off to clamp to 0, got %d", cfg.ScrollOff)
	}
}
