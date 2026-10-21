package viewer

// The system's own UI, such as the menu bar, the taskbar or a window's
// frame, reaches the viewer through code the system calls on the main
// thread while the event loop waits in its event handling. That code finds
// the app in nativeApp, and queues the actions chosen with
// chooseSystemAction for the event loop, which it wakes.

var (
	// nativeApp is the app the system's UI acts on, once it has a window.
	nativeApp     *App
	systemActions = make(chan string, 16)
)

// chooseSystemAction queues an action chosen in the system's UI.
func chooseSystemAction(action string) {
	select {
	case systemActions <- action:
		if nativeApp != nil {
			nativeApp.wakeLoop()
		}
	default: // a burst too quick to keep up with
	}
}

// pollSystemActions runs the actions chosen in the system's UI.
func (a *App) pollSystemActions() bool {
	ran := false
	for {
		select {
		case action := <-systemActions:
			a.runAction(action)
			ran = true
		default:
			return ran
		}
	}
}
