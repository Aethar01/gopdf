package viewer

import (
	"fmt"
	"strconv"
	"strings"
)

// Commands that run a bindable action, for the actions worth reaching from
// the : prompt.

// actionCommands take no arguments and run an action.
var actionCommands = map[string]string{
	"present":  "presentation",
	"overview": "overview",
	"outline":  "outline",
	"noh":      "clear_search",
	"back":     "jump_backward",
	"forward":  "jump_forward",
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
	"dual":       {action: "toggle_dual_page", on: func(a *App) bool { return a.dualPage }},
	"cover":      {action: "toggle_first_page_offset", on: func(a *App) bool { return a.firstPageOffset }},
	"trim":       {action: "toggle_trim_margins", on: func(a *App) bool { return a.trimMargins }},
	"statusbar":  {action: "toggle_status_bar", on: func(a *App) bool { return a.statusBarShown }},
}

// runActionCommand runs name if it is an action, toggle or view command,
// reporting whether it was one.
func (a *App) runActionCommand(name, args string) bool {
	switch name {
	case "rotate":
		a.runRotateCommand(args)
		return true
	case "zoom":
		a.runZoomCommand(args)
		return true
	}
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

// runRotateCommand rotates clockwise by default, or by ccw, or to an
// absolute multiple of 90 degrees.
func (a *App) runRotateCommand(arg string) {
	switch arg = strings.ToLower(strings.TrimSpace(arg)); arg {
	case "", "cw":
		a.runAction("rotate_cw")
	case "ccw":
		a.runAction("rotate_ccw")
	default:
		degrees, err := strconv.Atoi(arg)
		if err != nil || degrees%90 != 0 {
			a.message = "usage: :rotate [cw|ccw|0|90|180|270]"
			return
		}
		a.SetRotation(float64(degrees))
	}
}

// runZoomCommand zooms in or out a step, resets, or sets a percentage such
// as 150 or 150%.
func (a *App) runZoomCommand(arg string) {
	switch arg = strings.ToLower(strings.TrimSpace(arg)); arg {
	case "in":
		a.runAction("zoom_in")
	case "out":
		a.runAction("zoom_out")
	case "reset":
		a.runAction("reset_zoom")
	default:
		percent, err := strconv.ParseFloat(strings.TrimSuffix(arg, "%"), 64)
		if err != nil || percent <= 0 {
			a.message = "usage: :zoom in|out|reset|PERCENT"
			return
		}
		a.SetZoom(percent / 100)
	}
}
