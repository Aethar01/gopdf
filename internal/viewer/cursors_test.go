package viewer

import (
	"testing"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func TestSystemCursorForFallsBackWithoutCSSCursors(t *testing.T) {
	tests := []struct {
		kind     cursorKind
		css, old sdl.SystemCursor
	}{
		{cursorDefault, sdl.SystemCursorDefault, sdl.SystemCursorDefault},
		{cursorPointer, sdl.SystemCursorPointer, sdl.SystemCursorPointer},
		{cursorGrabbing, systemCursorGrabbing, sdl.SystemCursorMove},
		{cursorAutoscrollAll, systemCursorAllScroll, sdl.SystemCursorMove},
		{cursorAutoscrollVertical, sdl.SystemCursorNSResize, sdl.SystemCursorNSResize},
		{cursorAutoscrollHorizontal, sdl.SystemCursorEWResize, sdl.SystemCursorEWResize},
		{cursorAutoscrollN, sdl.SystemCursorNResize, sdl.SystemCursorNResize},
		{cursorAutoscrollSE, sdl.SystemCursorSEResize, sdl.SystemCursorSEResize},
		{cursorAutoscrollNW, sdl.SystemCursorNWResize, sdl.SystemCursorNWResize},
	}
	for _, tt := range tests {
		if got := systemCursorFor(tt.kind, true); got != tt.css {
			t.Errorf("kind %d with CSS cursors: got %d, want %d", tt.kind, got, tt.css)
		}
		if got := systemCursorFor(tt.kind, false); got != tt.old {
			t.Errorf("kind %d without CSS cursors: got %d, want %d", tt.kind, got, tt.old)
		}
	}
}
