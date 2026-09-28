package viewer

import (
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"
)

func TestClickingFormFieldsEditsThem(t *testing.T) {
	doc, err := mupdf.Open(testpdf.WriteForm(t), mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(1)
	app.doc = doc
	info, _ := doc.PageInfo(0)
	app.pageMetrics[0] = newPageMetrics(info)
	app.recomputeLayout(1000, 1000)
	x, y, _ := app.pageScreenOrigin(0)
	click := func(px, py float64) bool { return app.clickFormField(x+px*app.scale, y+py*app.scale) }
	value := func(px, py float64) string {
		w, _, _ := doc.WidgetAt(0, mupdf.Point{X: px, Y: py})
		return w.Value
	}

	if !click(150, 80) || app.mode != modeFormField || app.input.Value != "Ada" {
		t.Fatalf("text field: mode=%v input=%q", app.mode, app.input.Value)
	}
	app.input.Set("")
	app.commitInputMode() // a blank value clears the field
	if value(150, 80) != "" || !app.unsaved {
		t.Fatalf("text field after blank submit = %q, unsaved=%v", value(150, 80), app.unsaved)
	}

	click(107, 135)
	if value(107, 135) != "Yes" {
		t.Fatalf("check box after click = %q", value(107, 135))
	}

	click(150, 180)
	view := app.activeUIView()
	if view == nil || len(view.rows) != 2 || view.selected != 0 {
		t.Fatalf("choice list = %+v", view)
	}
	view.onSelect(app, view.rows[1])
	if value(150, 180) != "Green" {
		t.Fatalf("choice after picking = %q", value(150, 180))
	}

	if click(400, 400) {
		t.Fatal("a click off the fields was taken as a field click")
	}
}
