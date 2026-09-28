package viewer

import (
	"fmt"
	"path/filepath"
)

// Edits such as highlights and form values stay in memory until :write.

// Edits are counted along the undo history: editPos is the number of edits
// applied and savedPos the count when the file was last written, or -1 once
// that state can no longer be reached by undo and redo.

// markEdited records a new edit.
func (a *App) markEdited() {
	if a.editPos < a.savedPos {
		a.savedPos = -1 // the edit discards the redo history holding the saved state
	}
	a.moveEditPos(1)
}

func (a *App) moveEditPos(delta int) {
	a.editPos += delta
	a.unsaved = a.editPos != a.savedPos
	a.discardWarned = false
}

// undoEdit undoes (or redoes) an edit and re-renders the pages on screen.
func (a *App) undoEdit(redo bool) {
	if a.doc == nil {
		a.message = "no document open"
		return
	}
	undo, delta, done := a.doc.Undo, -1, "undone"
	if redo {
		undo, delta, done = a.doc.Redo, 1, "redone"
	}
	if err := undo(); err != nil {
		a.message = err.Error()
		return
	}
	a.moveEditPos(delta)
	for page := range a.cache.byPage {
		a.bumpPageRevision(page)
	}
	a.pendingRedraw = true
	a.message = done
}

// confirmDiscard reports whether an action that would lose unsaved edits
// may go ahead: at once when nothing is unsaved, otherwise on the second
// attempt, after a warning.
func (a *App) confirmDiscard(action string) bool {
	if !a.unsaved || a.discardWarned {
		return true
	}
	a.discardWarned = true
	a.message = fmt.Sprintf("unsaved changes: :w saves them, %s again discards them", action)
	return false
}

// writeDocument saves the document to path, or over itself when path is
// empty.
func (a *App) writeDocument(path string) {
	if a.doc == nil {
		a.message = "no document open"
		return
	}
	if !a.doc.IsPDF() {
		a.message = "only PDF documents can be saved"
		return
	}
	target := a.docPath
	if path != "" {
		target = a.resolveOpenPath(path)
	}
	if err := a.doc.Save(target, a.docPath); err != nil {
		a.message = err.Error()
		return
	}
	if target == a.docPath {
		a.savedPos = a.editPos
		a.unsaved = false
		a.document.record(a.docPath) // our own write is not an outside change
	}
	a.message = "wrote " + filepath.Base(target)
}
