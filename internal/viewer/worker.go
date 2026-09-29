package viewer

import (
	"sync"
	"time"
)

const workerCloseTimeout = 100 * time.Millisecond

type workerLifecycle struct {
	closing   chan struct{}
	done      chan struct{}
	closeOnce sync.Once
	wake      func() // wakes the event loop to collect an update; may be nil
}

func newWorkerLifecycle(wake func()) workerLifecycle {
	return workerLifecycle{closing: make(chan struct{}), done: make(chan struct{}), wake: wake}
}

func (w *workerLifecycle) Close() {
	w.closeOnce.Do(func() { close(w.closing) })
	select {
	case <-w.done:
		return
	case <-time.After(workerCloseTimeout):
		return
	}
}

func sendWorkerUpdate[T any](w *workerLifecycle, updates chan<- T, update T) bool {
	select {
	case <-w.closing:
		return false
	case updates <- update:
		if w.wake != nil {
			w.wake()
		}
		return true
	}
}
