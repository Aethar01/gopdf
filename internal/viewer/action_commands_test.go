package viewer

import (
	"math"
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

func TestLayoutToggleCommands(t *testing.T) {
	app := testLayoutApp(4)
	app.winW, app.winH = 1000, 800
	app.recomputeLayout(app.viewportSize())
	app.runCommand(":dual on")
	app.runCommand(":cover off")
	app.runCommand(":statusbar off")
	if !app.dualPage || app.firstPageOffset || app.statusBarShown {
		t.Fatalf("dual=%v cover=%v statusbar=%v", app.dualPage, app.firstPageOffset, app.statusBarShown)
	}
	if len(app.rows[0].pages) != 2 {
		t.Fatalf("first row has %d pages with :dual on :cover off", len(app.rows[0].pages))
	}
	app.runCommand(":dual")
	if app.dualPage {
		t.Fatal(":dual did not toggle off")
	}
}

func TestViewCommands(t *testing.T) {
	app := testLayoutApp(5)
	app.config.MinZoom, app.config.MaxZoom = 0.5, 8
	app.winW, app.winH = 1000, 800
	app.recomputeLayout(app.viewportSize())

	app.runCommand(":rotate")
	app.runCommand(":rotate cw")
	if app.rotation != 180 {
		t.Fatalf("rotation = %v after two :rotate", app.rotation)
	}
	app.runCommand(":rotate 270")
	app.runCommand(":rotate 45")
	if app.rotation != 270 || app.message != "usage: :rotate [cw|ccw|0|90|180|270]" {
		t.Fatalf("rotation = %v, message %q", app.rotation, app.message)
	}

	app.runCommand(":zoom 150%")
	if app.fitMode != "manual" || math.Abs(app.zoom-1.5) > 1e-9 {
		t.Fatalf(":zoom 150%%: fit=%q zoom=%v", app.fitMode, app.zoom)
	}
	app.runCommand(":zoom 200")
	if math.Abs(app.zoom-2) > 1e-9 {
		t.Fatalf(":zoom 200: zoom=%v", app.zoom)
	}

	before := app.page
	app.alignPageToAnchor(4)
	app.runCommand(":back")
	if app.page != before {
		t.Fatalf(":back left page %d, want %d", app.page, before)
	}
	app.runCommand(":forward")
	if app.page != 4 {
		t.Fatalf(":forward left page %d", app.page)
	}

	app.search = searchState{query: "x", order: []searchHitRef{{}}}
	app.runCommand(":noh")
	if app.search.query != "" {
		t.Fatal(":noh kept the search")
	}
}
