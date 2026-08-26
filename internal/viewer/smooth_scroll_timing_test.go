package viewer

import "testing"

func TestTimestampedSmoothScrollAdvancesBeforeExtendingTarget(t *testing.T) {
	app := testLayoutApp(5)
	app.config.SmoothScrollDampening = 0.35
	app.recomputeLayout(1000, 100)
	app.scrollY = 100
	defer app.cancelSmoothScroll()

	const startNS uint64 = 1_000_000_000
	app.queueSmoothScrollAt(0, 64, startNS)
	app.queueSmoothScrollAt(0, 64, startNS+uint64(smoothScrollFrame))

	assertClose(t, app.scrollY, 122.4)
	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll to remain active")
	}
	assertClose(t, state.targetY, 228)
	if state.lastAdvanceNS != startNS+uint64(smoothScrollFrame) {
		t.Fatalf("last advance = %d, want %d", state.lastAdvanceNS, startNS+uint64(smoothScrollFrame))
	}
}

func TestTimestampedSmoothScrollIgnoresOutOfOrderTimestamp(t *testing.T) {
	app := testLayoutApp(5)
	app.config.SmoothScrollDampening = 0.35
	app.recomputeLayout(1000, 100)
	app.scrollY = 100
	defer app.cancelSmoothScroll()

	const startNS uint64 = 2_000_000_000
	app.queueSmoothScrollAt(0, 64, startNS)
	app.queueSmoothScrollAt(0, 64, startNS-1)

	assertClose(t, app.scrollY, 100)
	state := app.smoothScrollState()
	if state == nil {
		t.Fatal("expected smooth scroll to remain active")
	}
	assertClose(t, state.targetY, 228)
	if state.lastAdvanceNS != startNS {
		t.Fatalf("last advance moved backward to %d", state.lastAdvanceNS)
	}
}
