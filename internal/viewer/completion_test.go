package viewer

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"gopdf/internal/config"
)

func TestShowCompletionAcceptsUniqueCommand(t *testing.T) {
	a := &App{mode: modeCommand, input: "op", inputCursor: 2, config: config.Default()}
	a.showCompletion()

	if a.input != "open" {
		t.Fatalf("expected open completion, got %q", a.input)
	}
	if a.completion.visible {
		t.Fatal("expected unique completion to close menu")
	}
}

func TestShowCompletionCyclesCommandMenu(t *testing.T) {
	a := &App{mode: modeCommand, input: "", inputCursor: 0, config: config.Default()}
	a.showCompletion()

	if !a.completion.visible {
		t.Fatal("expected command completion menu")
	}
	first := a.completion.items[a.completion.selected].value
	a.showCompletion()
	if got := a.completion.items[a.completion.selected].value; got == first {
		t.Fatalf("expected show_completion to cycle, still selected %q", got)
	}
	a.moveCompletion(-1)
	if got := a.completion.items[a.completion.selected].value; got != first {
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

	a := &App{docPath: filepath.Join(dir, "current.pdf")}
	items := a.openPathCompletions("")
	got := completionValues(items)
	want := []string{"docs" + string(os.PathSeparator), "current.pdf", "notes.txt", "paper.pdf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	items = a.openPathCompletions("docs" + string(os.PathSeparator))
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

	a := &App{docPath: filepath.Join(dir, "sub", "current.pdf")}
	items := a.openPathCompletions("./")
	got := completionValues(items)
	want := []string{"./file.pdf"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}

	items = a.openPathCompletions("../")
	got = completionValues(items)
	want = []string{"../parent" + string(os.PathSeparator), "../sub" + string(os.PathSeparator)}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected %v, got %v", want, got)
	}
}

func TestDotPathCompletionsAddSeparator(t *testing.T) {
	a := &App{}
	if got := completionValues(a.openPathCompletions(".")); !reflect.DeepEqual(got, []string{"." + string(os.PathSeparator)}) {
		t.Fatalf("expected ./ completion, got %v", got)
	}
	if got := completionValues(a.openPathCompletions("..")); !reflect.DeepEqual(got, []string{".." + string(os.PathSeparator)}) {
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
	if got := expandHomePath("~/paper.pdf"); got != filepath.Join(home, "paper.pdf") {
		t.Fatalf("expected home-relative path, got %q", got)
	}
	if got := expandHomePath("/tmp/paper.pdf"); got != "/tmp/paper.pdf" {
		t.Fatalf("expected absolute path unchanged, got %q", got)
	}
}

func TestDeleteInputWord(t *testing.T) {
	a := &App{input: "open ../some file.pdf", inputCursor: len([]rune("open ../some file"))}
	a.deleteInputWord()
	if a.input != "open ../some .pdf" || a.inputCursor != len([]rune("open ../some ")) {
		t.Fatalf("expected previous word deleted, input=%q cursor=%d", a.input, a.inputCursor)
	}

	a.input = "open ../some   "
	a.inputCursor = len([]rune(a.input))
	a.deleteInputWord()
	if a.input != "open " || a.inputCursor != len([]rune("open ")) {
		t.Fatalf("expected word and trailing spaces deleted, input=%q cursor=%d", a.input, a.inputCursor)
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
