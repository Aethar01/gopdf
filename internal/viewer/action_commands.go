package viewer

import (
	"fmt"
	"strings"
)

// Commands that run a bindable action, for the actions worth reaching from
// the : prompt.

// actionCommands take no arguments and run an action.
var actionCommands = map[string]string{
	"present":  "presentation",
	"overview": "overview",
	"outline":  "outline",
}

// toggleCommand switches a setting: on, off, or with no argument to the
// other state, running its toggle action when the setting is not already as
// asked.
type toggleCommand struct {
	action string
	on     func(*App) bool
}

var toggleCommands = map[string]toggleCommand{
	"fullscreen": {action: "toggle_fullscreen", on: func(a *App) bool { return a.fullscreen }},
}

// runActionCommand runs name if it is an action or toggle command,
// reporting whether it was one.
func (a *App) runActionCommand(name, args string) bool {
	if action, ok := actionCommands[name]; ok {
		a.runAction(action)
		return true
	}
	toggle, ok := toggleCommands[name]
	if !ok {
		return false
	}
	want, err := parseToggle(args, toggle.on(a))
	if err != nil {
		a.message = fmt.Sprintf(":%s %v", name, err)
		return true
	}
	if want != toggle.on(a) {
		a.runAction(toggle.action)
	}
	return true
}

// parseToggle reads an on/off argument; none flips current.
func parseToggle(arg string, current bool) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "":
		return !current, nil
	case "on", "true", "yes", "1":
		return true, nil
	case "off", "false", "no", "0":
		return false, nil
	}
	return false, fmt.Errorf("expects on or off, not %q", arg)
}
