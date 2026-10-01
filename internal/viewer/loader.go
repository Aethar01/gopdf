package viewer

import (
	"image/color"
	"math"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// A page with nothing rendered yet shows three ink drops bouncing in its
// middle, squashing as they land and stretching as they fall.
const (
	inkBouncePeriod = 900 * time.Millisecond
	inkDropCount    = 3
	inkSquashWindow = 0.12 // fraction of the cycle either side of landing
	inkEllipseSides = 24
)

var loaderEpoch = time.Now()

// drawInkLoader draws the loader centred on the page at (x, y) with the
// given on-screen size.
func (a *App) drawInkLoader(renderer *sdl.Renderer, x, y, width, height float64, elapsed time.Duration) {
	radius := math.Max(3, math.Min(10, math.Min(width, height)*0.018))
	ground := y + height/2 + radius*2
	bounce := radius * 5
	ink := a.mutedColor()
	shadow := ink
	for i := range inkDropCount {
		// Each drop runs a phase behind the last; t is 0 and 1 at landing.
		offset := time.Duration(i) * inkBouncePeriod / 6
		t := math.Mod(float64(elapsed+offset)/float64(inkBouncePeriod), 1)
		lift := 4 * t * (1 - t)
		speed := math.Abs(1 - 2*t)
		squash := math.Max(0, 1-math.Min(t, 1-t)/inkSquashWindow)
		stretch := 0.2 * speed * (1 - squash)
		rx := radius * (1 + 0.35*squash - 0.5*stretch)
		ry := radius * (1 - 0.35*squash + stretch)
		cx := x + width/2 + float64(i-(inkDropCount-1)/2)*radius*3.2

		shadow.A = uint8(50 * (1 - 0.7*lift))
		fillEllipse(renderer, cx, ground, radius*(1.2-0.6*lift), radius*0.25, shadow)
		fillEllipse(renderer, cx, ground-ry-lift*bounce, rx, ry, ink)
	}
}

func fillEllipse(renderer *sdl.Renderer, cx, cy, rx, ry float64, clr color.RGBA) {
	fc := sdl.FColor{R: float32(clr.R) / 255, G: float32(clr.G) / 255, B: float32(clr.B) / 255, A: float32(clr.A) / 255}
	vertices := make([]sdl.Vertex, 0, inkEllipseSides+1)
	indices := make([]int32, 0, inkEllipseSides*3)
	vertices = append(vertices, sdl.Vertex{Position: sdl.FPoint{X: float32(cx), Y: float32(cy)}, Color: fc})
	for i := range inkEllipseSides {
		angle := 2 * math.Pi * float64(i) / inkEllipseSides
		vertices = append(vertices, sdl.Vertex{Position: sdl.FPoint{X: float32(cx + rx*math.Cos(angle)), Y: float32(cy + ry*math.Sin(angle))}, Color: fc})
		indices = append(indices, 0, int32(1+i), int32(1+(i+1)%inkEllipseSides))
	}
	sdl.RenderGeometry(renderer, nil, vertices, indices)
}
