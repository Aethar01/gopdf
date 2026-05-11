package viewer

import (
	"image"
	"image/color"
)

func (a *App) statusVisible() bool {
	return a.statusBarShown || a.mode != modeNormal
}

func (a *App) backgroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltBackground)
	}
	return rgb(a.config.Background)
}

func (a *App) pageBackgroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltPageBackground)
	}
	return rgb(a.config.PageBackground)
}

func (a *App) foregroundColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltForeground)
	}
	return rgb(a.config.Foreground)
}

func (a *App) statusBarColor() color.RGBA {
	if a.altColors {
		return rgb(a.config.AltStatusBarColor)
	}
	return rgb(a.config.StatusBarColor)
}

func rgb(c [3]uint8) color.RGBA {
	return color.RGBA{R: c[0], G: c[1], B: c[2], A: 0xff}
}

func remapPageColors(img *image.RGBA, bg, fg [3]uint8) {
	for i := 0; i+3 < len(img.Pix); i += 4 {
		a := img.Pix[i+3]
		if a == 0 {
			continue
		}
		r := img.Pix[i]
		g := img.Pix[i+1]
		b := img.Pix[i+2]
		lum := uint16(r)*77 + uint16(g)*150 + uint16(b)*29
		t := uint8(lum >> 8)
		img.Pix[i] = mixChannel(fg[0], bg[0], t)
		img.Pix[i+1] = mixChannel(fg[1], bg[1], t)
		img.Pix[i+2] = mixChannel(fg[2], bg[2], t)
	}
}

func mixChannel(fg, bg, t uint8) uint8 {
	return uint8((uint16(fg)*(255-uint16(t)) + uint16(bg)*uint16(t)) / 255)
}
