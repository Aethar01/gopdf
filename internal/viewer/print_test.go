package viewer

import "testing"

func TestPrintDialogEditsSettings(t *testing.T) {
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
