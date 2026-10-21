//go:build !darwin && !windows

package viewer

import (
	"image/color"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) configureNativeWindow(window *sdl.Window) {}

func setNativeWindowDocument(window *sdl.Window, path string, edited bool) {}

func setNativeWindowColors(window *sdl.Window, background, border color.RGBA) {}
