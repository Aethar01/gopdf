package viewer

import "testing"

func TestPromptHistoryStepsThroughMatchingEntries(t *testing.T) {
	app := testLayoutApp(1) // session database off: history stays in memory
	for _, entry := range []string{"fit width", "open a.pdf", "fit page"} {
		app.recordPromptHistory(modeCommand, entry)
	}
	app.recordPromptHistory(modeSearch, "needle")
	app.mode = modeCommand

	app.input.Set("fit")
	app.stepPromptHistory(1)
	if app.input.Value != "fit page" {
		t.Fatalf("first Up = %q, want newest match", app.input.Value)
	}
	app.stepPromptHistory(1)
	if app.input.Value != "fit width" {
		t.Fatalf("second Up = %q, skipping non-matching entries", app.input.Value)
	}
	app.stepPromptHistory(1) // no older match: stays
	if app.input.Value != "fit width" {
		t.Fatalf("Up past oldest = %q", app.input.Value)
	}
	app.stepPromptHistory(-1)
	app.stepPromptHistory(-1)
	if app.input.Value != "fit" {
		t.Fatalf("Down past newest = %q, want the typed prefix back", app.input.Value)
	}

	app.input.Set("")
	app.stepPromptHistory(1)
	if app.input.Value != "fit page" {
		t.Fatalf("command history leaked search entries: %q", app.input.Value)
	}
	app.recordPromptHistory(modeCommand, "fit width") // resubmitting moves it to newest
	app.input.Set("")
	app.stepPromptHistory(1)
	if app.input.Value != "fit width" {
		t.Fatalf("resubmitted entry not newest: %q", app.input.Value)
	}
}
