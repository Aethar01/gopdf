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
}

func newWorkerLifecycle() workerLifecycle {
	return workerLifecycle{closing: make(chan struct{}), done: make(chan struct{})}
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
		return true
	}
}
