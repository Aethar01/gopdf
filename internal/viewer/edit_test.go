package viewer

import (
	"os"
	"path/filepath"
	"testing"

	"gopdf/internal/mupdf"
	"gopdf/internal/testpdf"
)

func TestQuitWithUnsavedEditsNeedsConfirmation(t *testing.T) {
	app := testLayoutApp(1)
	app.markEdited()
	app.runCommand(":q")
	if app.quit {
		t.Fatal("quit with unsaved edits did not warn first")
	}
	app.runCommand(":q")
	if !app.quit {
		t.Fatal("second :q did not quit")
	}

	app = testLayoutApp(1)
	app.markEdited()
	app.runCommand(":q!")
	if !app.quit {
		t.Fatal(":q! did not quit")
	}
}

func TestWriteSavesCopyAndInPlace(t *testing.T) {
	path := testpdf.Write(t, "hello")
	doc, err := mupdf.Open(path, mupdf.OpenOptions{})
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close()
	app := testLayoutApp(1)
	app.doc, app.docPath = doc, path

	copyPath := filepath.Join(filepath.Dir(path), "copy.pdf")
	app.markEdited()
	app.runCommand(":w " + copyPath)
	if _, err := os.Stat(copyPath); err != nil || !app.unsaved {
		t.Fatalf("copy: stat err=%v unsaved=%v (a copy leaves the edits unsaved here)", err, app.unsaved)
	}
	app.runCommand(":w")
	if app.unsaved {
		t.Fatalf("in-place write left unsaved set: %s", app.message)
	}
	if reopened, err := mupdf.Open(path, mupdf.OpenOptions{}); err != nil {
		t.Fatalf("saved file does not open: %v", err)
	} else {
		reopened.Close()
	}
}
