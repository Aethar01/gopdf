package viewer

import (
	"fmt"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) Run() error {
	a.logf("init SDL")
	if !sdl.Init(sdl.InitVideo) {
		return fmt.Errorf("SDL init failed: %s", sdl.GetError())
	}
	sdl.SetHint("SDL_RENDER_SCALE_QUALITY", "2")
	var window *sdl.Window
	var renderer *sdl.Renderer
	if !sdl.CreateWindowAndRenderer("gopdf", 1400, 900, sdl.WindowResizable|sdl.WindowHighPixelDensity, &window, &renderer) {
		sdl.Quit()
		return fmt.Errorf("SDL window creation failed: %s", sdl.GetError())
	}
	a.logf("created SDL window 1400x900")
	a.window = window
	a.renderer = renderer
	if rw := sdl.IOFromConstMem(a.iconBytes); rw != nil {
		if icon := sdl.LoadBMPIO(rw, true); icon != nil {
			sdl.SetWindowIcon(window, icon)
			sdl.DestroySurface(icon)
		}
	}
	a.cursorHand = sdl.CreateSystemCursor(sdl.SystemCursorPointer)
	a.cursorArrow = sdl.CreateSystemCursor(sdl.SystemCursorDefault)
	sdl.SetEventEnabled(sdl.EventDropFile, true)
	a.setWindowTitle()
	sdl.SetRenderDrawBlendMode(a.renderer, sdl.BlendModeBlend)
	sdl.SetDefaultTextureScaleMode(a.renderer, sdl.ScaleModeLinear)
	var outputW, outputH int32
	if sdl.GetRenderOutputSize(a.renderer, &outputW, &outputH) {
		w, h := outputW, outputH
		a.winW, a.winH = int(w), int(h)
	}
	if a.initialDocPath != "" {
		a.message = "opening " + a.initialDocPath
		a.pendingRedraw = true
		if err := a.drawFrame(); err != nil {
			return err
		}
		path := a.initialDocPath
		startPage := a.initialStartPage
		pageSet := a.initialPageSet
		a.initialDocPath = ""
		a.initialPageSet = false
		a.logf("open initial document path=%q page=%d", path, startPage+1)
		if err := a.openDocument(path, openDocumentOptions{startPage: startPage, startPageExplicit: pageSet}); err != nil {
			return err
		}
	}
	a.recomputeLayout(a.viewportSize())
	a.pendingRedraw = true
	sdl.StartTextInput(a.window)
	defer sdl.StopTextInput(a.window)
	for !a.quit {
		var event sdl.Event
		for sdl.PollEvent(&event) {
			if err := a.handleSDLEvent(&event); err != nil {
				return err
			}
		}
		a.pollRenderUpdates()
		a.pollMetricUpdates()
		a.pollSearchUpdates()
		a.pollDocumentUpdate()
		a.expireSequence()
		a.prefetchVisiblePages()
		a.adjustRenderBaseScaleForExtremeZoom(a.scale)
		if a.pendingRedraw {
			if err := a.drawFrame(); err != nil {
				return err
			}
		}
		if !a.quit {
			var event sdl.Event
			if sdl.WaitEventTimeout(&event, int32(a.eventWaitTimeoutMS())) {
				if err := a.handleSDLEvent(&event); err != nil {
					return err
				}
			}
		}
	}
	a.logf("viewer exiting")
	return nil
}

func (a *App) eventWaitTimeoutMS() int {
	if a.hasPendingVisibleRender() || a.search.running {
		return 16
	}
	if len(a.sequence) > 0 {
		elapsed := time.Since(a.sequenceAt)
		remaining := time.Duration(a.config.SequenceTimeoutMS)*time.Millisecond - elapsed
		if remaining <= 0 {
			return 1
		}
		if remaining < 100*time.Millisecond {
			return max(1, int(remaining/time.Millisecond))
		}
	}
	return 100
}

func (a *App) handleSDLEvent(event *sdl.Event) error {
	redraw := true
	switch event.Type() {
	case sdl.EventQuit:
		a.quit = true
		redraw = false
	case sdl.EventWindowResized, sdl.EventWindowPixelSizeChanged:
		e := event.Window()
		a.relayoutWithViewportAnchor(func() {
			a.winW = int(e.Data1)
			a.winH = int(e.Data2)
		})
	case sdl.EventWindowEnterFullscreen:
		a.fullscreen = true
		redraw = false
	case sdl.EventWindowLeaveFullscreen:
		a.fullscreen = false
		redraw = false
	case sdl.EventKeyUp:
		e := event.Key()
		a.handleSDLKeyUp(&e)
		redraw = false
	case sdl.EventKeyDown:
		e := event.Key()
		a.handleSDLKeyDown(&e)
	case sdl.EventTextInput:
		e := event.Text()
		a.handleSDLTextInput(&e)
	case sdl.EventMouseWheel:
		e := event.Wheel()
		a.handleSDLMouseWheel(&e)
	case sdl.EventMouseButtonDown, sdl.EventMouseButtonUp:
		e := event.Button()
		a.handleSDLMouseButton(&e)
	case sdl.EventMouseMotion:
		e := event.Motion()
		redraw = a.handleSDLMouseMotion(&e)
	case sdl.EventDropFile:
		e := event.Drop()
		a.handleDroppedFile(e.Data())
		redraw = e.Data() != ""
	default:
		redraw = false
	}
	if redraw {
		a.pendingRedraw = true
	}
	return nil
}

func (a *App) handleDroppedFile(path string) {
	if path == "" {
		return
	}
	if err := a.Open(path); err != nil {
		a.message = err.Error()
	}
}

func (a *App) drawFrame() error {
	if a.renderer == nil {
		return nil
	}
	var w, h int32
	if sdl.GetRenderOutputSize(a.renderer, &w, &h) {
		a.winW, a.winH = int(w), int(h)
	}
	bg := a.backgroundColor()
	if !sdl.SetRenderDrawColor(a.renderer, bg.R, bg.G, bg.B, bg.A) {
		return fmt.Errorf("SDL draw color failed: %s", sdl.GetError())
	}
	if !sdl.RenderClear(a.renderer) {
		return fmt.Errorf("SDL clear failed: %s", sdl.GetError())
	}
	a.drawPages(a.renderer)
	if a.pendingRedraw {
		a.pendingRedraw = false
	}
	if a.statusVisible() {
		if err := a.drawStatusBar(a.renderer); err != nil {
			return err
		}
	}
	if a.completion.visible {
		if err := a.drawCompletion(a.renderer); err != nil {
			return err
		}
	}
	if a.keybindMenu.visible {
		if err := a.drawKeybindMenu(a.renderer); err != nil {
			return err
		}
	}
	if a.outlineMenu.visible {
		if err := a.drawOutlineMenu(a.renderer); err != nil {
			return err
		}
	}
	if a.luaUI.visible {
		if err := a.drawLuaUI(a.renderer); err != nil {
			return err
		}
	}
	sdl.RenderPresent(a.renderer)
	return nil
}
