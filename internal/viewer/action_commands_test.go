package viewer

import (
	"slices"
	"testing"

	commandmeta "gopdf/internal/commands"
	"gopdf/internal/mupdf"
)

func TestActionAndToggleCommandsAreDocumented(t *testing.T) {
	names := commandmeta.Names()
	for name := range actionCommands {
		if !slices.Contains(names, name) {
			t.Errorf(":%s has no command spec", name)
		}
	}
	for name := range toggleCommands {
		if !slices.Contains(names, name) {
			t.Errorf(":%s has no command spec", name)
		}
	}
}

func TestToggleCommandArguments(t *testing.T) {
	app := testLayoutApp(1)
	app.runCommand(":fullscreen on")
	app.runCommand(":fullscreen on") // already on: stays on
	if !app.fullscreen {
		t.Fatal(":fullscreen on left fullscreen off")
	}
	app.runCommand(":fullscreen")
	if app.fullscreen {
		t.Fatal(":fullscreen did not toggle")
	}
	app.runCommand(":fullscreen maybe")
	if app.fullscreen || app.message != `:fullscreen expects on or off, not "maybe"` {
		t.Fatalf("bad argument: fullscreen=%v message=%q", app.fullscreen, app.message)
	}
}

func TestModeCommands(t *testing.T) {
	app := testLayoutApp(3)
	app.winW, app.winH = 1000, 800
	app.recomputeLayout(app.viewportSize())
	app.runCommand(":overview")
	if app.overview == nil {
		t.Fatal(":overview did not open the overview")
	}
	app.runCommand(":overview")
	app.runCommand(":present")
	if app.overview != nil || app.presentation == nil {
		t.Fatalf(":overview then :present: overview=%v presentation=%v", app.overview != nil, app.presentation != nil)
	}
	app.runCommand(":present")
	app.outline = []mupdf.OutlineItem{{Title: "One", Parent: -1}}
	app.runCommand(":outline")
	if view := app.activeUIView(); view == nil || view != app.outlineMenu.view {
		t.Fatal(":outline did not open the outline")
	}
}
