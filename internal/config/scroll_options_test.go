package config

import "testing"

func TestScrollOptionDefaults(t *testing.T) {
	cfg := Default()
	if cfg.InvertScroll {
		t.Fatal("expected invert_scroll to default to false")
	}
	if cfg.InvertSmoothScroll {
		t.Fatal("expected invert_smooth_scroll to default to false")
	}
	if !cfg.SmoothScroll {
		t.Fatal("expected smooth_scroll to default to true")
	}
	if cfg.SmoothScrollDampening != 0.8 {
		t.Fatalf("expected smooth_scroll_dampening=0.8, got %v", cfg.SmoothScrollDampening)
	}
}

func TestSmoothScrollOptionCanBeDisabled(t *testing.T) {
	cfg := Default()
	desc := configOptions["smooth_scroll"]

	if err := desc.applyText(&cfg, "off"); err != nil {
		t.Fatal(err)
	}
	if cfg.SmoothScroll {
		t.Fatal("expected smooth_scroll to be disabled")
	}

	if err := desc.applyText(&cfg, "on"); err != nil {
		t.Fatal(err)
	}
	if !cfg.SmoothScroll {
		t.Fatal("expected smooth_scroll to be enabled")
	}
}

func TestSmoothScrollDampeningOptionIsClamped(t *testing.T) {
	cfg := Default()
	desc := configOptions["smooth_scroll_dampening"]

	if err := desc.applyText(&cfg, "0.6"); err != nil {
		t.Fatal(err)
	}
	if cfg.SmoothScrollDampening != 0.6 {
		t.Fatalf("expected dampening=0.6, got %v", cfg.SmoothScrollDampening)
	}

	if err := desc.applyText(&cfg, "2"); err != nil {
		t.Fatal(err)
	}
	if cfg.SmoothScrollDampening != 1 {
		t.Fatalf("expected upper clamp=1, got %v", cfg.SmoothScrollDampening)
	}

	if err := desc.applyText(&cfg, "0"); err != nil {
		t.Fatal(err)
	}
	if cfg.SmoothScrollDampening != 0.01 {
		t.Fatalf("expected lower clamp=0.01, got %v", cfg.SmoothScrollDampening)
	}
}
