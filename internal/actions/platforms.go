package actions

import "runtime"

// Platform is an operating system whose users expect keys of their own,
// such as Cmd shortcuts on macOS. Its keys are bound there on top of each
// action's Keys, never in their place, so the keys gopdf has everywhere
// keep working on every system.
type Platform struct {
	GOOS string              // as runtime.GOOS names it
	Name string              // as the reference names it
	Keys map[string][]string // by action name
}

// platforms lists the systems with keys of their own. Another system's
// keys go in a table like macOSKeys and an entry here.
var platforms = []Platform{
	{GOOS: "darwin", Name: "macOS", Keys: macOSKeys},
}

// macOSKeys are the Cmd shortcuts macOS applications share.
var macOSKeys = map[string][]string{
	"open_file_picker":  {"<D-o>"},
	"save":              {"<D-s>"},
	"print":             {"<D-p>"},
	"quit":              {"<D-q>", "<D-w>"},
	"undo":              {"<D-z>"},
	"redo":              {"<D-S-z>"},
	"cut":               {"<D-x>"},
	"copy":              {"<D-c>"},
	"paste":             {"<D-v>"},
	"search_prompt":     {"<D-f>"},
	"search_next":       {"<D-g>"},
	"search_prev":       {"<D-S-g>"},
	"zoom_in":           {"<D-=>", "<D-S-=>"},
	"zoom_out":          {"<D-->"},
	"reset_zoom":        {"<D-0>"},
	"first_page":        {"<D-Up>"},
	"last_page":         {"<D-Down>"},
	"jump_backward":     {"<D-[>"},
	"jump_forward":      {"<D-]>"},
	"toggle_fullscreen": {"<C-D-f>"},
}

// Platforms returns the systems with keys of their own.
func Platforms() []Platform {
	return append([]Platform(nil), platforms...)
}

// DefaultBindings returns the default key bindings on this system.
func DefaultBindings() map[string]string {
	return DefaultBindingsFor(runtime.GOOS)
}

// DefaultBindingsFor returns the default key bindings on goos: every
// action's Keys, and the system's own keys when it has some.
func DefaultBindingsFor(goos string) map[string]string {
	bindings := map[string]string{}
	for _, action := range registry {
		for _, key := range action.Keys {
			bindings[key] = action.Name
		}
	}
	for _, platform := range platforms {
		if platform.GOOS != goos {
			continue
		}
		for name, keys := range platform.Keys {
			for _, key := range keys {
				bindings[key] = name
			}
		}
	}
	return bindings
}
