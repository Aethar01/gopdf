package viewer

import (
	"sync"
	"time"
)

const workerCloseTimeout = 100 * time.Millisecond

func closeWorker(closing chan struct{}, done chan struct{}, closeOnce *sync.Once) {
	closeOnce.Do(func() { close(closing) })
	select {
	case <-done:
		return
	case <-time.After(workerCloseTimeout):
		return
	}
}
