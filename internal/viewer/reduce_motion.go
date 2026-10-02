package viewer

// The OS's setting for less motion, as macOS's Reduce motion or GNOME's
// animations switch, turns the theme's motion off as motion.scale = 0
// would. It is read in the background when the viewer starts and each
// time its window comes forward, as reading it can mean running a
// program.

// refreshReducedMotion reads the OS's setting again.
func (a *App) refreshReducedMotion() {
	go func() { a.motion.osReduced.Store(osReducesMotion()) }()
}

// motionScale is the theme's motion scale, or 0 when the OS asks for less
// motion.
func (a *App) motionScale() float64 {
	if a.motion.osReduced.Load() {
		return 0
	}
	return a.config.Theme.Motion.Scale
}
