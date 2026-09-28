package viewer

import (
	"image"
	"image/color"
	"slices"
)

func (a *App) statusVisible() bool {
	return a.statusBarShown || a.mode != modeNormal
}

func (a *App) backgroundColor() color.RGBA {
	if a.presentation != nil {
		return presentationBackground
	}
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

// remapPageColors maps each pixel's luminance onto the fg-bg range, leaving
// pixels inside keep, in img's coordinates, as they are.
func remapPageColors(img *image.RGBA, bg, fg [3]uint8, keep []image.Rectangle) {
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		row := img.Pix[(y-bounds.Min.Y)*img.Stride:]
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if !slices.ContainsFunc(keep, image.Pt(x, y).In) {
				remapPixel(row[(x-bounds.Min.X)*4:], bg, fg)
			}
		}
	}
}

func remapPixel(px []uint8, bg, fg [3]uint8) {
	if px[3] == 0 {
		return
	}
	lum := uint16(px[0])*77 + uint16(px[1])*150 + uint16(px[2])*29
	t := uint8(lum >> 8)
	px[0] = mixChannel(fg[0], bg[0], t)
	px[1] = mixChannel(fg[1], bg[1], t)
	px[2] = mixChannel(fg[2], bg[2], t)
}

func mixChannel(fg, bg, t uint8) uint8 {
	return uint8((uint16(fg)*(255-uint16(t)) + uint16(bg)*uint16(t)) / 255)
}
