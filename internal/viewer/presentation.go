package viewer

import (
	"image/color"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Presentation mode shows one page at a time, fitted to a full screen with
// a black surround, and restores the previous view when it ends.

type presentationState struct {
	saved      viewState
	fullscreen bool
}

func (a *App) togglePresentation() {
	if a.presentation != nil {
		a.closePresentation()
		return
	}
	a.closeViewMode()
	a.closeAllUI()
	a.presentation = &presentationState{saved: a.captureViewState(), fullscreen: a.fullscreen}
	a.relayoutWithViewportAnchor(func() {
		a.renderMode, a.fitMode, a.dualPage, a.statusBarShown = "single", "page", false, false
	})
	a.SetFullscreen(true)
	a.settleRenderScale()
}

// presentationReturnState is the view to return to: the one from before
// presenting, keeping the settings changed meanwhile that presentation
// mode does not set itself.
func (a *App) presentationReturnState() viewState {
	s := a.presentation.saved
	s.firstPageOffset, s.rotation, s.altColors = a.firstPageOffset, a.rotation, a.altColors
	return s
}

func (a *App) closePresentation() {
	saved, fullscreen := a.presentationReturnState(), a.presentation.fullscreen
	a.presentation = nil
	page := a.page
	a.restoreViewState(saved)
	a.SetFullscreen(fullscreen)
	a.alignPageToAnchor(page)
	a.settleRenderScale()
}

// runPresentationAction turns movement into page steps, reporting false for
// actions that should run as usual.
func (a *App) runPresentationAction(action string) bool {
	switch action {
	case "scroll_down", "scroll_right", "next_page", "next_spread":
		a.nextPage()
	case "scroll_up", "scroll_left", "prev_page", "prev_spread":
		a.prevPage()
	case "close", "presentation":
		a.closePresentation()
	default:
		return false
	}
	return true
}

// clickPresentation advances on a left click and goes back on a right one.
func (a *App) clickPresentation(e *sdl.MouseButtonEvent) {
	if e.Type != sdl.EventMouseButtonDown {
		return
	}
	switch e.Button {
	case uint8(sdl.ButtonLeft):
		a.nextPage()
	case uint8(sdl.ButtonRight):
		a.prevPage()
	}
}

var presentationBackground = color.RGBA{A: 0xff}
