package viewer

import (
	"runtime"
	"slices"
	"testing"
)

func TestPrintCommandPassesOptionsBeforeFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows prints through the shell verb")
	}
	cmd := printCommand("/tmp/-odd name.pdf", []string{"-d", "office"})
	if want := []string{"lp", "-d", "office", "--", "/tmp/-odd name.pdf"}; !slices.Equal(cmd.Args, want) {
		t.Fatalf("args = %q, want %q", cmd.Args, want)
	}
}

func TestPrintSettingsLpArgs(t *testing.T) {
	s := printSettings{printer: "Epson_ET_3850", pages: "1-3,5", copies: 2, sides: "two-sided-long-edge"}
	want := []string{"-d", "Epson_ET_3850", "-P", "1-3,5", "-n", "2", "-o", "sides=two-sided-long-edge"}
	if got := s.lpArgs(); !slices.Equal(got, want) {
		t.Fatalf("lpArgs = %q, want %q", got, want)
	}
	if got := (printSettings{copies: 1}).lpArgs(); len(got) != 0 {
		t.Fatalf("defaults give %q, want no options", got)
	}
}

func TestParseDefaultPrinter(t *testing.T) {
	if name, ok := parseDefaultPrinter("system default destination: Office\n"); !ok || name != "Office" {
		t.Fatalf("parsed %q, %v", name, ok)
	}
	if _, ok := parseDefaultPrinter("no system default destination\n"); ok {
		t.Fatal("parsed a default where there is none")
	}
}

func TestPrintDialogEditsSettings(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows prints through the shell verb")
	}
	app := testLayoutApp(1)
	app.docPath, app.docName = "/tmp/doc.pdf", "doc.pdf"
	app.printSettings.printer = "Office"
	app.printDocument("")
	row := func(value string) uiRow {
		for _, r := range app.activeUIView().rows {
			if r.value == value {
				return r
			}
		}
		t.Fatalf("no %s row", value)
		return uiRow{}
	}
	app.activeUIView().onSelect(app, row("sides"))
	if app.printSettings.sides != "two-sided-long-edge" || row("sides").secondary != "long edge" {
		t.Fatalf("sides = %q", app.printSettings.sides)
	}
	app.activeUIView().onSelect(app, row("copies"))
	app.input.Set("3")
	app.commitInputMode()
	if app.printSettings.copies != 3 || row("copies").secondary != "3" {
		t.Fatalf("copies = %d", app.printSettings.copies)
	}
	app.activeUIView().onSelect(app, row("pages"))
	app.input.Set("1-2, 4")
	app.commitInputMode()
	if app.printSettings.pages != "1-2,4" {
		t.Fatalf("pages = %q", app.printSettings.pages)
	}
}

func TestPrintDialogFillsInFoundPrinters(t *testing.T) {
	app := testLayoutApp(1)
	app.docPath, app.docName = "/tmp/doc.pdf", "doc.pdf"
	found := make(chan printerList, 1)
	app.printersFound = found
	app.showPrintDialog(0)
	if got := app.activeUIView().rows[0].secondary; got != "looking…" {
		t.Fatalf("printer before lookup = %q", got)
	}
	found <- printerList{names: []string{"Epson", "Office"}, defaultName: "Office"}
	app.pollPrintResult()
	if app.printSettings.printer != "Office" || app.activeUIView().rows[0].secondary != "Office" {
		t.Fatalf("printer after lookup = %q", app.printSettings.printer)
	}
}
