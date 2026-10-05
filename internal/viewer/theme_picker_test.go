package viewer

import (
	"os"
	"path/filepath"
	"testing"

	"gopdf/internal/config"
)

func testThemePickerApp(t *testing.T, source string) *App {
	t.Helper()
	dir := t.TempDir()
	for _, name := range []string{"HOME", "XDG_DATA_HOME", "APPDATA"} {
		t.Setenv(name, dir) // a session database of its own, for the theme chosen
	}
	t.Cleanup(config.CloseSessionDatabase)
	path := filepath.Join(dir, "config.lua")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatal(err)
	}
	rt, err := config.Open(path, "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(rt.Close)
	app, err := New("", rt, 0, nil, NewOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return app
}

// selectTheme moves the picker's selection to name, as keys would.
func selectTheme(t *testing.T, app *App, name string) {
	t.Helper()
	view := app.activeModalUIView()
	for _, row := range view.visibleRows() {
		if row.value == name {
			app.moveUIViewSelection(view, row.index-view.selected)
			app.notifySelectionChange()
			return
		}
	}
	t.Fatalf("no theme %q in the picker", name)
}

func TestThemePickerPreviewsAndPutsBack(t *testing.T) {
	app := testThemePickerApp(t, "")
	moss, _ := config.Preset("moss")
	birch, _ := config.Preset("birch")

	app.runCommand(":theme")
	view := app.activeModalUIView()
	if view == nil || view.id != "themes" || view.visibleRows()[view.selected].value != "moss" {
		t.Fatalf("picker = %+v", view)
	}
	selectTheme(t, app, "birch")
	if app.config.Theme.Background != birch.Background {
		t.Fatal("selecting birch did not show it")
	}
	app.closeActiveUI()
	if app.config.Theme.Background != moss.Background || app.runtime.ChosenTheme() != "" {
		t.Fatalf("closing kept birch: chosen %q", app.runtime.ChosenTheme())
	}

	// Choosing one keeps it, and the picker opens on it next time.
	app.runCommand(":theme")
	selectTheme(t, app, "birch")
	app.activateUIView(app.activeModalUIView())
	if app.activeModalUIView() != nil || app.config.Theme.Background != birch.Background || app.runtime.ChosenTheme() != "birch" {
		t.Fatalf("after choosing: open %v, chosen %q", app.activeModalUIView() != nil, app.runtime.ChosenTheme())
	}
	app.runCommand(":theme")
	if view := app.activeModalUIView(); view.visibleRows()[view.selected].value != "birch" {
		t.Fatal("the picker did not open on the chosen theme")
	}
}

func TestThemeCommandSaysWhenTheConfigSetsTheTheme(t *testing.T) {
	app := testThemePickerApp(t, `gopdf.theme = "classic"`)
	app.runCommand(":theme birch")
	if want := "theme birch; config.lua sets gopdf.theme, so this lasts until gopdf restarts"; app.message != want {
		t.Fatalf("message %q", app.message)
	}
	app.runCommand(":theme nonesuch")
	if app.runtime.ChosenTheme() != "birch" || app.message == "" {
		t.Fatalf("an unknown theme: chosen %q, message %q", app.runtime.ChosenTheme(), app.message)
	}
}
