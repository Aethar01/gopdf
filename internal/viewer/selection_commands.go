package viewer

import (
	"fmt"
	"strings"
)

func (a *App) copyPersistentSelectionToClipboard() {
	if strings.TrimSpace(a.selection.text) == "" {
		return
	}
	if err := a.SetClipboard(a.selection.text); err != nil {
		a.message = "clipboard unavailable"
		return
	}
	a.message = fmt.Sprintf("copied %d chars", len(a.selection.text))
}

func (a *App) clearSelection() {
	a.selection = textSelection{}
	a.emitSelectionChanged()
}
