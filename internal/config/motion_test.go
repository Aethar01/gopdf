package config

import (
	"math"
	"strings"
	"testing"
)

func TestParseTransition(t *testing.T) {
	for raw, want := range map[string]string{
		"140ms ease-out":                     "140ms ease-out",
		"0.2s":                               "200ms ease",
		"linear 50":                          "50ms linear",
		`"cubic-bezier(0.2, 0, 0, 1) 300ms"`: "300ms cubic-bezier(0.2, 0, 0, 1)",
		"none":                               "none",
		"0ms ease-in":                        "none",
	} {
		got, err := parseTransition(raw)
		if err != nil {
			t.Errorf("%s: %v", raw, err)
			continue
		}
		if formatTransition(got) != want {
			t.Errorf("%s = %q, want %q", raw, formatTransition(got), want)
		}
	}
	for _, raw := range []string{"ease-out", "140ms bounce", "140ms ease-in ease-out", "-5ms", "cubic-bezier(2, 0, 0, 1) 1s"} {
		if _, err := parseTransition(raw); err == nil {
			t.Errorf("parseTransition(%q) succeeded", raw)
		}
	}
}

func TestTransitionEase(t *testing.T) {
	linear, easeOut := transition(100, "linear"), transition(100, "ease-out")
	if got := linear.Ease(0.25); math.Abs(got-0.25) > 1e-6 {
		t.Errorf("linear(0.25) = %v", got)
	}
	if easeOut.Ease(0) != 0 || easeOut.Ease(1) != 1 || easeOut.Ease(0.5) <= 0.6 {
		t.Errorf("ease-out: %v %v %v", easeOut.Ease(0), easeOut.Ease(0.5), easeOut.Ease(1))
	}
	// An overshooting curve goes past its target on the way.
	back, err := parseTransition("100ms cubic-bezier(0.3, 1.6, 0.6, 1)")
	if err != nil {
		t.Fatal(err)
	}
	if back.Ease(0.6) <= 1 {
		t.Errorf("overshooting curve at 0.6 = %v", back.Ease(0.6))
	}
}

func TestMotionInTheTheme(t *testing.T) {
	if theme := mustPreset(t, "classic"); theme.Motion.Scale != 0 {
		t.Errorf("classic should not move: %+v", theme.Motion)
	}
	rt := mustLoadThemeTestConfig(t, `
gopdf.theme.motion = { scale = 2, panel = "1s linear", cursor = { duration = 30 } }
gopdf.theme.motion.message = { easing = "ease-in" }
assert(gopdf.theme.motion.panel == "1000ms linear", gopdf.theme.motion.panel)
`)
	m := rt.Config().Theme.Motion
	if m.Scale != 2 || m.Panel.Duration != 1000 || m.Panel.Easing != "linear" || m.Cursor.Duration != 30 || m.Cursor.Easing != "ease-out" {
		t.Fatalf("motion = %+v", m)
	}
	if m.Message.Easing != "ease-in" || m.Message.Duration != defaultMotion.Message.Duration {
		t.Fatalf("message = %+v", m.Message)
	}
	if err := rt.SetOption("theme.motion.selection", "250ms ease-in-out"); err != nil {
		t.Fatal(err)
	}
	if got, _ := rt.OptionValue("theme.motion.selection"); got != `"250ms ease-in-out"` {
		t.Fatalf("selection = %s", got)
	}
	if _, err := loadThemeTestConfig(t, `gopdf.theme.motion.panel = "fast"`); err == nil || !strings.Contains(err.Error(), "expected a duration") {
		t.Fatalf("bad transition: %v", err)
	}
}
