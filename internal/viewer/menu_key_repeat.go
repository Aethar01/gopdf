package viewer

import (
	"gopdf/internal/actions"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// repeatableMenuAction returns the action for a repeated keydown only when the
// binding is an unambiguous countable single-key action. This mirrors the
// normal document input path: held navigation keys may repeat, while repeated
// prefixes cannot accidentally complete a multi-key sequence.
func (a *App) repeatableMenuAction(e *sdl.KeyboardEvent) (string, bool) {
	if e == nil || !e.Repeat {
		return "", false
	}
	token, ok := keyToken(e.Key, e.Mod)
	if !ok {
		return "", false
	}
	binding := normalizeBinding(token)
	action, ok := a.sequenceLookup[binding]
	if !ok || !actions.IsCountable(action) || a.hasPrefix(binding) {
		return "", false
	}
	return action, true
}
