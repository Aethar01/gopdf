package viewer

import "gopdf/internal/menubar"

// On systems with a menu bar, the viewer shows the one menubar describes,
// each item with a key bound to its action. setNativeMenuBar draws it again
// whenever the bindings change, and the items chosen reach the event loop
// through menuBarChoices, which wakes it.

var (
	menuBarChoices = make(chan string, 16)
	menuBarWaker   *loopWaker
)

// chooseMenuBarAction queues an action chosen from the menu bar. The
// system's menu code calls it, on the main thread.
func chooseMenuBarAction(action string) {
	select {
	case menuBarChoices <- action:
		menuBarWaker.wake()
	default: // a burst too quick to keep up with
	}
}

func (a *App) installMenuBar() {
	menuBarWaker = a.waker
	a.menuBarShown = true
	a.updateMenuBar()
}

func (a *App) updateMenuBar() {
	if a.menuBarShown {
		setNativeMenuBar(menubar.Menus(), a.sequenceLookup)
	}
}

// pollMenuBar runs the actions chosen from the menu bar.
func (a *App) pollMenuBar() bool {
	ran := false
	for {
		select {
		case action := <-menuBarChoices:
			a.runAction(action)
			ran = true
		default:
			return ran
		}
	}
}
