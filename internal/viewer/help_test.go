package viewer

import (
	"testing"

	commandmeta "gopdf/internal/commands"
)

func TestHelpRowsCoverEverySection(t *testing.T) {
	app := testLayoutApp(1)
	app.config.KeyBindings = map[string]string{"j": "scroll_down", "<Down>": "scroll_down"}
	rows := app.helpRows()
	find := func(id, text string) (uiRow, bool) {
		for _, row := range rows {
			if row.id == id && row.text == text {
				return row, true
			}
		}
		return uiRow{}, false
	}
	if row, ok := find(helpRowCommand, ":fit width|height|page|manual"); !ok || row.value != "fit " {
		t.Fatalf("fit command row = %+v, %v", row, ok)
	}
	if row, ok := find(helpRowAction, "scroll_down"); !ok || row.secondary != "<Down> j" {
		t.Fatalf("scroll_down row = %+v, %v", row, ok)
	}
	if row, ok := find(helpRowSearch, "/-w …"); !ok || row.value != "-w " {
		t.Fatalf("whole-word flag row = %+v, %v", row, ok)
	}
	if row, ok := find(helpRowCommand, "fit_mode"); !ok || row.value != "set fit_mode=" {
		t.Fatalf("fit_mode option row = %+v, %v", row, ok)
	}
	for i, row := range rows {
		if row.index != i {
			t.Fatalf("row %d has index %d", i, row.index)
		}
	}
}

func TestActivateHelpRowFillsPrompts(t *testing.T) {
	app := testLayoutApp(1)
	app.activateHelpRow(uiRow{id: helpRowCommand, value: "fit "})
	if app.mode != modeCommand || app.input.Value != "fit " {
		t.Fatalf("command row: mode=%v input=%q", app.mode, app.input.Value)
	}
	app.activateHelpRow(uiRow{id: helpRowSearch, value: "-r "})
	if app.mode != modeSearch || app.input.Value != "-r " {
		t.Fatalf("search row: mode=%v input=%q", app.mode, app.input.Value)
	}
}

func TestDocumentedSearchFlagsParse(t *testing.T) {
	for _, flag := range commandmeta.SearchFlags() {
		if _, options := parseSearchQuery("-" + flag.Flag + " x"); options == (searchOptions{}) {
			t.Errorf("documented flag -%s is not parsed", flag.Flag)
		}
	}
}
