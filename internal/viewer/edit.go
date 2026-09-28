package viewer

import (
	"fmt"
	"path/filepath"
)

// Edits such as highlights and form values stay in memory until :write.

// markEdited records an unsaved change to page.
func (a *App) markEdited() {
	a.unsaved = true
	a.discardWarned = false
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
		a.unsaved = false
		a.document.record(a.docPath) // our own write is not an outside change
	}
	a.message = "wrote " + filepath.Base(target)
}
