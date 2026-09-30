//go:build !darwin && !windows

package viewer

import "github.com/jupiterrider/purego-sdl3/sdl"

func (a *App) configureNativeWindow(window *sdl.Window) {}
