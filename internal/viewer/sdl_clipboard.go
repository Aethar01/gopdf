package viewer

import (
	"fmt"
)

var sdlSetClipboardText func(string) bool

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
