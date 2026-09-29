package viewer

import "testing"

func TestPresentationStepsPagesAndRestoresView(t *testing.T) {
	app := testLayoutApp(5)
	app.winW, app.winH = 1000, 800
	app.renderMode, app.fitMode, app.statusBarShown = "continuous", "width", true
	app.recomputeLayout(app.viewportSize())
	app.alignPageToAnchor(1)

	app.togglePresentation()
	if app.renderMode != "single" || app.fitMode != "page" || app.statusBarShown || !app.fullscreen {
		t.Fatalf("presenting: mode=%q fit=%q status=%v fullscreen=%v", app.renderMode, app.fitMode, app.statusBarShown, app.fullscreen)
	}
	if app.backgroundColor() != presentationBackground {
		t.Fatal("presentation background not black")
	}
	app.runAction("scroll_down")
	app.runAction("scroll_right")
	if app.page != 3 {
		t.Fatalf("page = %d after two steps, want 3", app.page)
	}
	if state := app.captureViewState(); state.renderMode != "continuous" {
		t.Fatalf("persisted state %+v, want the pre-presentation view", state)
	}

	app.runAction("close")
	if app.presentation != nil || app.renderMode != "continuous" || app.fitMode != "width" || !app.statusBarShown || app.fullscreen {
		t.Fatalf("after close: mode=%q fit=%q status=%v fullscreen=%v", app.renderMode, app.fitMode, app.statusBarShown, app.fullscreen)
	}
	if app.page != 3 {
		t.Fatalf("page = %d, want to stay on the presented page", app.page)
	}
}

func TestSettingsChangedWhilePresentingCarryOver(t *testing.T) {
	app := testLayoutApp(5)
	app.winW, app.winH = 1000, 800
	app.dualPage, app.statusBarShown = true, true
	app.recomputeLayout(app.viewportSize())
	app.togglePresentation()
	app.runAction("rotate_cw")
	app.runAction("close")
	if app.rotation != 90 {
		t.Fatalf("rotation = %v after presenting, want 90 kept", app.rotation)
	}
	if !app.dualPage || !app.statusBarShown {
		t.Fatalf("presentation's own settings not restored: dual=%v status=%v", app.dualPage, app.statusBarShown)
	}
}
