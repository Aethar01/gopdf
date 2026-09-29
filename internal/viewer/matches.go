package viewer

import (
	"fmt"
	"strconv"
	"strings"

	"gopdf/internal/mupdf"
)

// showSearchMatches lists every match of the current search with the line
// it is on; choosing one makes it the current match.
func (a *App) showSearchMatches() {
	if len(a.search.order) == 0 {
		a.message = a.searchStatusMessage()
		if a.search.query == "" {
			a.message = "no active search"
		}
		return
	}
	rows := make([]uiRow, len(a.search.order))
	for i, ref := range a.search.order {
		rows[i] = uiRow{index: i, text: a.searchMatchContext(ref), secondary: "p. " + a.pageLabel(ref.page), value: strconv.Itoa(i)}
	}
	a.showRowList("matches", fmt.Sprintf("Matches for /%s", a.searchDisplayQuery()), rows, 80, 70, func(view *uiView) {
		view.onSelect = func(a *App, row uiRow) {
			a.closeUIView(view, false)
			a.search.current, _ = strconv.Atoi(row.value)
			a.focusSearchCurrent()
			a.message = a.searchStatusMessage()
		}
		view.selected = max(0, a.search.current)
	})
}

// searchMatchContext returns the text of the line a match is on.
func (a *App) searchMatchContext(ref searchHitRef) string {
	quads := a.search.matches[ref.page][ref.hit].Quads
	if len(quads) == 0 {
		return ""
	}
	first, last := quads[0], quads[len(quads)-1]
	sel, err := a.doc.ExtractSelection(ref.page, first.UL, last.LR, mupdf.SelectLines)
	if err != nil {
		return ""
	}
	return strings.Join(strings.Fields(sel.Text), " ")
}
