package viewer

import (
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestGuiCNormalizesToCopyShortcut(t *testing.T) {
	got, ok := keyToken(sdl.KeycodeC, sdl.KeymodGui)
	if !ok || got != "<c-c>" {
		t.Fatalf("keyToken(Command-C) = %q, %v; want <c-c>, true", got, ok)
	}
}

func TestCopyOnSelectDisabledKeepsSelectionAfterMouseCopyHook(t *testing.T) {
	app := &App{
		config: config.Config{CopyOnSelect: false},
		interactionState: interactionState{selection: textSelection{
			text:  "selected text",
			quads: []mupdf.Quad{{}},
		}},
	}

	app.copySelectionToClipboard()

	if app.selection.text != "selected text" || len(app.selection.quads) != 1 {
		t.Fatalf("expected selection to persist, got %+v", app.selection)
	}
}

func TestEscapeClearsPersistentSelection(t *testing.T) {
	app := &App{
		config: config.Config{CopyOnSelect: false},
		interactionState: interactionState{selection: textSelection{
			text:  "selected text",
			quads: []mupdf.Quad{{}},
		}},
	}

	app.closeActiveUI()

	if app.selection.text != "" || len(app.selection.quads) != 0 {
		t.Fatalf("expected escape to clear selection, got %+v", app.selection)
	}
}
