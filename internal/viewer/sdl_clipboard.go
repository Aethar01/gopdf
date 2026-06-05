package viewer

import (
	"fmt"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

var sdlSetClipboardText func(string) bool
var sdlGetClipboardText = sdl.GetClipboardText

func setSDLClipboardText(text string) error {
	if sdlSetClipboardText == nil {
		if err := loadSDLSetClipboardText(); err != nil {
			return err
		}
	}
	if !sdlSetClipboardText(text) {
		return sdlError("set clipboard text")
	}
	return nil
}

func loadSDLSetClipboardText() error {
	if err := loadSDLSetClipboardTextSymbol(&sdlSetClipboardText); err != nil {
		return fmt.Errorf("SDL_SetClipboardText unavailable: %w", err)
	}
	return nil
}
