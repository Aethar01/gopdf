package viewer

import (
	"image"
	"image/color"
	"testing"

	"gopdf/internal/config"
)

func TestColorHelpersUseNormalAndAltPalettes(t *testing.T) {
	app := &App{config: config.Config{
		Background:        [3]uint8{1, 2, 3},
		PageBackground:    [3]uint8{4, 5, 6},
		Foreground:        [3]uint8{7, 8, 9},
		StatusBarColor:    [3]uint8{10, 11, 12},
		AltBackground:     [3]uint8{13, 14, 15},
		AltPageBackground: [3]uint8{16, 17, 18},
		AltForeground:     [3]uint8{19, 20, 21},
		AltStatusBarColor: [3]uint8{22, 23, 24},
	}}

	if app.statusVisible() {
		t.Fatal("expected status bar hidden in normal mode unless explicitly shown")
	}
	app.mode = modeCommand
	if !app.statusVisible() {
		t.Fatal("expected input mode to show status bar")
	}
	app.mode = modeNormal

	assertColor(t, app.backgroundColor(), color.RGBA{R: 1, G: 2, B: 3, A: 0xff})
	assertColor(t, app.pageBackgroundColor(), color.RGBA{R: 4, G: 5, B: 6, A: 0xff})
	assertColor(t, app.foregroundColor(), color.RGBA{R: 7, G: 8, B: 9, A: 0xff})
	assertColor(t, app.statusBarColor(), color.RGBA{R: 10, G: 11, B: 12, A: 0xff})

	app.altColors = true
	assertColor(t, app.backgroundColor(), color.RGBA{R: 13, G: 14, B: 15, A: 0xff})
	assertColor(t, app.pageBackgroundColor(), color.RGBA{R: 16, G: 17, B: 18, A: 0xff})
	assertColor(t, app.foregroundColor(), color.RGBA{R: 19, G: 20, B: 21, A: 0xff})
	assertColor(t, app.statusBarColor(), color.RGBA{R: 22, G: 23, B: 24, A: 0xff})
}

func TestRemapPageColorsPreservesAlphaAndMapsLuminance(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 0xff})
	img.SetRGBA(1, 0, color.RGBA{R: 255, G: 255, B: 255, A: 0xff})
	img.SetRGBA(2, 0, color.RGBA{R: 30, G: 40, B: 50, A: 0})

	remapPageColors(img, [3]uint8{200, 210, 220}, [3]uint8{10, 20, 30}, nil)

	assertColor(t, img.RGBAAt(0, 0), color.RGBA{R: 10, G: 20, B: 30, A: 0xff})
	assertColor(t, img.RGBAAt(1, 0), color.RGBA{R: 200, G: 210, B: 220, A: 0xff})
	assertColor(t, img.RGBAAt(2, 0), color.RGBA{R: 30, G: 40, B: 50, A: 0})
}

func assertColor(t *testing.T, got, want color.RGBA) {
	t.Helper()
	if got != want {
		t.Fatalf("got color %+v, want %+v", got, want)
	}
}

func TestRemapPageColorsKeepsImageAreas(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 2, 1))
	red := color.RGBA{R: 255, A: 0xff}
	img.SetRGBA(0, 0, red)
	img.SetRGBA(1, 0, red)
	remapPageColors(img, [3]uint8{0, 0, 0}, [3]uint8{255, 255, 255}, []image.Rectangle{image.Rect(1, 0, 2, 1)})
	if img.RGBAAt(0, 0) == red {
		t.Fatal("pixel outside the image area was not remapped")
	}
	assertColor(t, img.RGBAAt(1, 0), red)
}
