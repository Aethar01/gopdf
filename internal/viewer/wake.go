package viewer

import (
	"sync/atomic"
	"time"
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// Background workers wake the event loop with an SDL user event when they
// have something for it, instead of it polling on a timer. At most one wake
// event is queued at a time.

type loopWaker struct {
	eventType sdl.EventType
	queued    atomic.Bool
}

func newLoopWaker() *loopWaker {
	eventType := sdl.RegisterEvents(1)
	if eventType == 0 || eventType == ^uint32(0) {
		return nil
	}
	return &loopWaker{eventType: sdl.EventType(eventType)}
}

// wakeLoop may be called from any goroutine.
func (a *App) wakeLoop() {
	w := a.waker
	if w == nil || !w.queued.CompareAndSwap(false, true) {
		return
	}
	var event sdl.Event
	*(*sdl.EventType)(unsafe.Pointer(&event)) = w.eventType
	if !sdl.PushEvent(&event) {
		w.queued.Store(false)
	}
}

// wakeAfter wakes the loop once d has passed, for work due at a deadline,
// so it happens even if the wait for events outlasts its timeout.
func (a *App) wakeAfter(d time.Duration) {
	if a.waker != nil {
		time.AfterFunc(d, a.wakeLoop)
	}
}

// isWakeEvent reports whether event is a wake-up, clearing its queued flag.
func (a *App) isWakeEvent(event *sdl.Event) bool {
	if a.waker == nil || event.Type() != a.waker.eventType {
		return false
	}
	a.waker.queued.Store(false)
	return true
}
