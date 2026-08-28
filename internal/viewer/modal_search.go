package viewer

import (
	"strings"
	"unicode/utf8"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// handleModalSearchKey owns the key policy shared by searchable modal lists.
// Component handlers continue with their own navigation and actions when this
// returns false.
func (a *App) handleModalSearchKey(e *sdl.KeyboardEvent, searching *bool, backspace func(), close func() bool) bool {
	if e.Type != sdl.EventKeyDown {
		return true
	}
	if *searching && e.Key == sdl.KeycodeBackspace {
		backspace()
		return true
	}
	if e.Repeat {
		return true
	}
	if !*searching {
		return false
	}
	switch e.Key {
	case sdl.KeycodeEscape:
		close()
		return true
	case sdl.KeycodeReturn, sdl.KeycodeKpEnter:
		*searching = false
		return true
	}
	if token, ok := keyToken(e.Key, e.Mod); ok && !strings.HasPrefix(token, "<") && utf8.RuneCountInString(token) == 1 {
		return true
	}
	return false
}
