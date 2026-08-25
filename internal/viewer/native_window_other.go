//go:build !darwin

package viewer

import "github.com/jupiterrider/purego-sdl3/sdl"

func configureNativeWindow(window *sdl.Window) {}
