package viewer

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) Run() error {
	a.logf("init SDL")
	// On macOS, prefer normal key repeat over the system press-and-hold accent
	// menu. SDL requires this hint to be set before initialization.
	sdl.SetHint("SDL_MAC_PRESS_AND_HOLD", "0")
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
	configureNativeWindow(window)
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
	if err := a.openInitialDocument(); err != nil {
		return err
	}
	a.recomputeLayout(a.viewportSize())
	a.pendingRedraw = true
	a.syncTextInput()
	if a.runtime != nil {
		a.emitPluginEvent("app_ready", a.documentEventPayload())
	}
	defer a.stopTextInput()
	defer a.cancelSmoothScroll()
	for !a.quit {
		var event sdl.Event
		for sdl.PollEvent(&event) {
			if err := a.handleSDLEvent(&event); err != nil {
				return err
			}
		}
		a.advanceSmoothScroll()
		if a.runtime != nil {
			if a.runtime.PollPluginOperations() {
				a.applyRuntimeChanges("plugin operation")
				a.pendingRedraw = true
			}
		}
		a.pollRenderUpdates()
		a.pollMetricUpdates()
		a.pollSearchUpdates()
		a.pollDocumentUpdate()
		a.expireSequence()
		a.prefetchVisiblePages()
		a.adjustRenderBaseScaleForExtremeZoom(a.scale)
		a.emitViewStateEvents()
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

func (a *App) openInitialDocument() error {
	if a.initialDocPath == "" {
		return nil
	}
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
		a.message = err.Error()
		a.pendingRedraw = true
	}
	return nil
}

func (a *App) eventWaitTimeoutMS() int {
	if a.hasPendingVisibleRender() || a.search.running || a.smoothScrollActive() || a.runtime != nil && a.runtime.PluginOperationsActive() {
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

func (a *App) convertPointerEventToRenderCoordinates(event *sdl.Event) {
	if a.renderer == nil || event == nil {
		return
	}
	switch event.Type() {
	case sdl.EventMouseButtonDown, sdl.EventMouseButtonUp, sdl.EventMouseMotion:
		sdl.ConvertEventToRenderCoordinates(a.renderer, event)
	}
}

func (a *App) handleSDLEvent(event *sdl.Event) error {
	a.convertPointerEventToRenderCoordinates(event)
	defer a.syncTextInput()

	redraw := true
	switch event.Type() {
	case sdl.EventQuit:
		a.quit = true
		redraw = false
	case sdl.EventWindowExposed:
		redraw = true
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
	case sdl.EventWindowFocusLost:
		a.stopPan()
		redraw = false
	case sdl.EventKeyUp:
		e := event.Key()
		a.handleSDLKeyUp(&e)
		redraw = false
	case sdl.EventKeyDown:
		e := event.Key()
		if _, ok := a.repeatableMenuAction(&e); ok {
			e.Repeat = false
		}
		if !a.handleTextInputSelectionKey(&e) {
			a.handleSDLKeyDown(&e)
		}
	case sdl.EventTextInput:
		e := event.Text()
		a.handleSDLTextInput(&e)
	case sdl.EventMouseWheel:
		e := event.Wheel()
		a.handleAnimatedMouseWheel(&e)
		redraw = false
	case sdl.EventPinchBegin:
		a.beginPinch()
	case sdl.EventPinchUpdate:
		a.updatePinch(float64(pinchEventScale(event)))
	case sdl.EventPinchEnd:
		a.endPinch()
	case sdl.EventMouseButtonDown, sdl.EventMouseButtonUp:
		e := event.Button()
		a.handleMouseButtonEvent(&e)
	case sdl.EventMouseMotion:
		e := event.Motion()
		if a.handleInputMouseMotion(&e) {
			redraw = true
		} else {
			redraw = a.handleSDLMouseMotion(&e)
		}
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

func (a *App) handleMouseButtonEvent(e *sdl.MouseButtonEvent) {
	if a.runtime == nil {
		if !a.handleInputMouseButton(e) {
			a.handleSDLMouseButton(e)
		}
		return
	}
	payload := a.pluginMouseEventPayload(e)
	consumed := a.emitPluginEvent("mouse_button_pre", payload)
	if !consumed {
		consumed = a.handleInputMouseButton(e)
	}
	if !consumed {
		a.handleSDLMouseButton(e)
	}
	a.emitPluginEvent("mouse_button", payload)
}

func (a *App) pluginMouseEventPayload(e *sdl.MouseButtonEvent) map[string]any {
	button, _ := mouseButtonName(e.Button)
	phase := "up"
	if e.Type == sdl.EventMouseButtonDown {
		phase = "down"
	}
	mod := sdl.GetModState()
	page, point, ok := a.pagePointAtScreen(float64(e.X), float64(e.Y))
	payload := map[string]any{
		"phase":  phase,
		"button": button,
		"x":      float64(e.X),
		"y":      float64(e.Y),
		"modifiers": map[string]any{
			"ctrl":  mod&sdl.KeymodCtrl != 0,
			"shift": mod&sdl.KeymodShift != 0,
			"alt":   mod&sdl.KeymodAlt != 0,
			"gui":   mod&sdl.KeymodGui != 0,
		},
		"inside_document": ok,
	}
	if ok {
		payload["page"] = page + 1
		payload["page_x"] = point.X
		payload["page_y"] = point.Y
	}
	return payload
}

func (a *App) textInputNeeded() bool {
	return a.mode != modeNormal ||
		func() bool {
			view := a.activeModalUIView()
			return view != nil && view.searching
		}()
}

func (a *App) syncTextInput() {
	if a.window == nil {
		return
	}
	want := a.textInputNeeded()
	if want == sdl.TextInputActive(a.window) {
		return
	}
	if want {
		if !sdl.StartTextInput(a.window) {
			a.logf("start text input failed: %s", sdl.GetError())
		}
		return
	}
	if !sdl.StopTextInput(a.window) {
		a.logf("stop text input failed: %s", sdl.GetError())
	}
}

func (a *App) stopTextInput() {
	if a.window == nil || !sdl.TextInputActive(a.window) {
		return
	}
	if !sdl.StopTextInput(a.window) {
		a.logf("stop text input failed: %s", sdl.GetError())
	}
}

// The SDL binding currently does not expose Event.Pinch(). SDL_Event stores
// the pinch scale immediately after the common event header.
func pinchEventScale(event *sdl.Event) float32 {
	const commonEventSize = 16 // type, reserved, timestamp
	return math.Float32frombits(binary.NativeEndian.Uint32(event[commonEventSize:]))
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
	if view := a.activeUIView(); view != nil {
		if err := a.drawUIView(a.renderer, view); err != nil {
			return err
		}
	}
	sdl.RenderPresent(a.renderer)
	return nil
}
