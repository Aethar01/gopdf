package viewer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopdf/internal/config"
)

func TestShowCompletionAcceptsUniqueCommand(t *testing.T) {
	a := &App{inputState: inputState{mode: modeCommand, input: textInput{Value: "qui", Cursor: 3}}, config: config.Default()}
	a.showCompletion()

	if a.input.Value != "quit" {
		t.Fatalf("expected quit completion, got %q", a.input.Value)
	}
	if a.completion.view != nil && a.completion.view.visible {
		t.Fatal("expected unique completion to close menu")
	}
}

func TestShowCompletionCyclesCommandMenu(t *testing.T) {
	a := &App{inputState: inputState{mode: modeCommand}, config: config.Default()}
	a.showCompletion()

	if a.completion.view == nil || !a.completion.view.visible {
		t.Fatal("expected command completion menu")
	}
	first := a.completion.items[a.completion.view.selected].value
	a.showCompletion()
	if got := a.completion.items[a.completion.view.selected].value; got == first {
		t.Fatalf("expected show_completion to cycle, still selected %q", got)
	}
	a.moveCompletion(-1)
	if got := a.completion.items[a.completion.view.selected].value; got != first {
		t.Fatalf("expected previous completion %q, got %q", first, got)
	}
}

func TestOpenPathCompletionsUseDocumentDirectory(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "docs"))
	mustWrite(t, filepath.Join(dir, "docs", "book.pdf"))
	mustWrite(t, filepath.Join(dir, "current.pdf"))
	mustWrite(t, filepath.Join(dir, "paper.pdf"))
	mustWrite(t, filepath.Join(dir, "notes.txt"))

	a := &App{documentState: documentState{docPath: filepath.Join(dir, "current.pdf")}}
	items := a.openPathCompletions("")
	got := completionValues(items)
	want := []string{"docs" + pathSeparator(), "current.pdf", "notes.txt", "paper.pdf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	items = a.openPathCompletions("docs" + pathSeparator())
	got = completionValues(items)
	want = []string{filepath.Join("docs", "book.pdf")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestOpenPathCompletionsPreserveRelativePrefixes(t *testing.T) {
	dir := t.TempDir()
	mustMkdir(t, filepath.Join(dir, "sub"))
	mustMkdir(t, filepath.Join(dir, "parent"))
	mustWrite(t, filepath.Join(dir, "sub", "file.pdf"))
	mustWrite(t, filepath.Join(dir, "parent", "paper.pdf"))

	a := &App{documentState: documentState{docPath: filepath.Join(dir, "sub", "current.pdf")}}
	items := a.openPathCompletions("." + pathSeparator())
	got := completionValues(items)
	want := []string{"." + pathSeparator() + "file.pdf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	items = a.openPathCompletions(".." + pathSeparator())
	got = completionValues(items)
	want = []string{filepath.Join("..", "parent") + pathSeparator(), filepath.Join("..", "sub") + pathSeparator()}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestOpenPathCompletionsEscapeSpaces(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "Lancer - Core Book.pdf"))

	a := &App{documentState: documentState{docPath: filepath.Join(dir, "current.pdf")}}
	items := a.openPathCompletions("Lancer")
	got := completionValues(items)
	want := []string{`Lancer\ -\ Core\ Book.pdf`}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	items = a.openPathCompletions(`Lancer\ `)
	got = completionValues(items)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected escaped prefix to complete %v, got %v", want, got)
	}
}

func TestDotPathCompletionsAddSeparator(t *testing.T) {
	a := &App{}
	if got := completionValues(a.openPathCompletions(".")); !reflect.DeepEqual(got, []string{"." + pathSeparator()}) {
		t.Fatalf("expected ./ completion, got %v", got)
	}
	if got := completionValues(a.openPathCompletions("..")); !reflect.DeepEqual(got, []string{".." + pathSeparator()}) {
		t.Fatalf("expected ../ completion, got %v", got)
	}
}

func TestExpandHomePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}
	if got := expandHomePath("~"); got != home {
		t.Fatalf("expected %q, got %q", home, got)
	}
	if got := expandHomePath(filepath.Join("~", "paper.pdf")); got != filepath.Join(home, "paper.pdf") {
		t.Fatalf("expected home-relative path, got %q", got)
	}
	abs := filepath.Join(string(filepath.Separator), "tmp", "paper.pdf")
	if got := expandHomePath(abs); got != abs {
		t.Fatalf("expected absolute path unchanged, got %q", got)
	}
}

func TestExpandHomePathAcceptsSlashSeparator(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		t.Skip("home directory unavailable")
	}
	if got := expandHomePath(filepath.ToSlash(filepath.Join("~", "paper.pdf"))); got != filepath.Join(home, "paper.pdf") {
		t.Fatalf("expected home-relative path, got %q", got)
	}
}

func TestDeleteInputWord(t *testing.T) {
	a := &App{inputState: inputState{input: textInput{Value: "open ../some file.pdf", Cursor: len([]rune("open ../some file"))}}}
	a.input.DeleteWordLeft()
	if a.input.Value != "open ../some .pdf" || a.input.Cursor != len([]rune("open ../some ")) {
		t.Fatalf("expected previous word deleted, input=%q cursor=%d", a.input.Value, a.input.Cursor)
	}

	a.input.Set("open ../some   ")
	a.input.DeleteWordLeft()
	if a.input.Value != "open " || a.input.Cursor != len([]rune("open ")) {
		t.Fatalf("expected word and trailing spaces deleted, input=%q cursor=%d", a.input.Value, a.input.Cursor)
	}
}

func completionValues(items []completionItem) []string {
	values := make([]string, len(items))
	for i, item := range items {
		values[i] = item.value
	}
	return values
}

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatal(err)
	}
}

func mustWrite(t *testing.T, path string) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestCompletionAcceptCloseAndVisibleRows(t *testing.T) {
	app := &App{
		inputState: inputState{input: textInput{Value: "open par", Cursor: 8}},
		config:     config.Config{CompletionMaxItems: 3},
		uiState: uiState{completion: completionState{
			view:  &uiView{visible: true, selected: 1},
			start: 5,
			end:   8,
			items: []completionItem{
				{display: "one", value: "one"},
				{display: "paper.pdf", value: "paper.pdf"},
			},
		}},
	}
	app.acceptCompletion()
	if app.input.Value != "open paper.pdf" || app.input.Cursor != len([]rune("open paper.pdf")) || app.completion.view != nil || !app.pendingRedraw {
		t.Fatalf("expected selected completion to replace range and close menu, input=%q cursor=%d completion=%+v redraw=%v", app.input.Value, app.input.Cursor, app.completion, app.pendingRedraw)
	}

	app.completion = completionState{view: &uiView{visible: true}, items: []completionItem{{display: "a", value: "a"}}}
	app.pendingRedraw = false
	app.closeCompletion()
	if app.completion.view != nil || len(app.completion.items) != 0 || !app.pendingRedraw {
		t.Fatalf("expected closeCompletion to clear menu and request redraw, completion=%+v redraw=%v", app.completion, app.pendingRedraw)
	}

	app.completion = completionState{view: &uiView{selected: 3}, items: []completionItem{{display: "a"}, {display: "b"}, {display: "c"}, {display: "d"}, {display: "e"}}}
	rows := app.visibleCompletionRows()
	if got, want := completionRowTexts(rows), []string{"...", "d", "e"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected truncated visible completion rows %v, got %v", want, got)
	}
	if !rows[1].selected {
		t.Fatalf("expected selected completion row to remain marked, rows=%+v", rows)
	}

	app.completion = completionState{view: &uiView{selected: 1}, items: []completionItem{{display: "old.pdf", recent: true}, {display: "paper.pdf", recent: true}, {display: "docs" + pathSeparator(), value: "docs" + pathSeparator()}}}
	rows = app.visibleCompletionRows()
	if got, want := completionRowTexts(rows), []string{"Recents:", "  old.pdf", "  paper.pdf", "Suggestions:", "  docs" + pathSeparator()}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected recent completion section %v, got %v", want, got)
	}
	if !rows[2].selected {
		t.Fatalf("expected selected recent row to remain marked under header, rows=%+v", rows)
	}
}

func completionRowTexts(rows []completionRow) []string {
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.text
	}
	return texts
}
