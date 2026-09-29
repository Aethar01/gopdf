package viewer

import (
	"github.com/jupiterrider/purego-sdl3/sdl"
)

// The page overview lays pages out as a grid of thumbnails. It reuses the
// normal continuous layout with several pages per row, so rendering and
// scrolling work as usual; only navigation and the selection outline differ.

const (
	overviewThumbWidth = 220 // target on-screen page width for auto columns
	overviewGap        = 16
	overviewMaxColumns = 12
)

type overviewState struct {
	selected int
	columns  int       // 0 picks a count from the window width
	saved    viewState // the view to return to
}

func (a *App) toggleOverview() {
	if a.overview != nil {
		a.closeOverview(a.overview.saved.page)
		return
	}
	a.closeAllUI()
	a.overview = &overviewState{selected: a.page, saved: a.captureViewState()}
	a.renderMode = "continuous"
	a.fitMode = "width"
	a.relayoutOverview()
}

// closeOverview restores the view from before the overview, at page.
func (a *App) closeOverview(page int) {
	saved := a.overview.saved
	a.overview = nil
	a.restoreViewState(saved)
	if page != saved.page {
		a.alignPageToAnchor(page)
	}
}

func (a *App) overviewColumns() int {
	if a.overview.columns > 0 {
		return min(a.overview.columns, max(1, a.pageCount))
	}
	viewportW, _ := a.viewportSize()
	return clampInt(viewportW/(overviewThumbWidth+overviewGap), 2, max(2, min(a.pageCount, overviewMaxColumns)))
}

func (a *App) relayoutOverview() {
	a.recomputeLayout(a.viewportSize())
	a.page = a.overview.selected
	a.scrollOverviewSelectionIntoView()
}

// runOverviewAction handles actions while the overview is shown, reporting
// false for those that should run as usual.
func (a *App) runOverviewAction(action string) bool {
	columns := a.overviewColumns()
	switch action {
	case "scroll_down":
		a.moveOverviewSelection(columns)
	case "scroll_up":
		a.moveOverviewSelection(-columns)
	case "scroll_right", "next_spread":
		a.moveOverviewSelection(1)
	case "scroll_left", "prev_spread":
		a.moveOverviewSelection(-1)
	case "next_page":
		a.moveOverviewSelection(a.overviewVisibleRows() * columns)
	case "prev_page":
		a.moveOverviewSelection(-a.overviewVisibleRows() * columns)
	case "first_page":
		a.moveOverviewSelection(-a.overview.selected)
	case "last_page":
		a.moveOverviewSelection(a.pageCount - 1 - a.overview.selected)
	case "zoom_in":
		a.overview.columns = max(1, columns-1)
		a.relayoutOverview()
	case "zoom_out":
		a.overview.columns = min(overviewMaxColumns, columns+1)
		a.relayoutOverview()
	case "reset_zoom":
		a.overview.columns = 0
		a.relayoutOverview()
	case "confirm":
		a.closeOverview(a.overview.selected)
	case "close", "overview":
		a.closeOverview(a.overview.saved.page)
	default:
		return false
	}
	return true
}

func (a *App) moveOverviewSelection(delta int) {
	a.overview.selected = clampInt(a.overview.selected+delta, 0, max(0, a.pageCount-1))
	a.page = a.overview.selected
	a.scrollOverviewSelectionIntoView()
}

// overviewVisibleRows is how many whole rows of thumbnails fit on screen.
func (a *App) overviewVisibleRows() int {
	if len(a.rows) == 0 {
		return 1
	}
	_, viewportH := a.viewportSize()
	return max(1, int(float64(viewportH)/(a.rows[0].height+overviewGap)))
}

// scrollOverviewSelectionIntoView scrolls by whole rows so the selected row
// keeps scroll_off rows of context, the same way menus do.
func (a *App) scrollOverviewSelectionIntoView() {
	if a.overview.selected >= len(a.pageToRow) {
		return
	}
	first := a.rowIndexAtContentY(a.scrollY + overviewGap)
	first = modalListScrollForSelection(first, a.pageToRow[a.overview.selected], a.overviewVisibleRows(), len(a.rows), a.config.ScrollOff)
	a.scrollY = a.rows[first].y - overviewGap
	a.clampScroll()
}

// clickOverview jumps to the page under a click.
func (a *App) clickOverview(e *sdl.MouseButtonEvent) {
	if e.Type != sdl.EventMouseButtonDown || e.Button != uint8(sdl.ButtonLeft) {
		return
	}
	if page, _, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y)); ok {
		a.closeOverview(page)
	}
}

func (a *App) drawOverviewSelection(renderer *sdl.Renderer) {
	if a.overview == nil {
		return
	}
	x, y, ok := a.pageScreenOrigin(a.overview.selected)
	if !ok {
		return
	}
	row := a.rows[a.pageToRow[a.overview.selected]]
	for i, page := range row.pages {
		if page == a.overview.selected {
			const border = 3
			rect := sdl.FRect{X: float32(x - border), Y: float32(y - border), W: float32(row.pageW[i] + 2*border), H: float32(row.pageH[i] + 2*border)}
			strokeRect(renderer, rect, rgb(a.config.HighlightBackground), border)
		}
	}
}
