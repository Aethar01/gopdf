package viewer

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type documentWatcher struct {
	path    string
	mod     time.Time
	size    int64
	changed chan struct{}
	pending watcherChange

	watcher *fsnotify.Watcher
	done    chan struct{}
	mu      sync.Mutex
	started bool
}

type watcherChange struct {
	mod       time.Time
	size      int64
	firstSeen time.Time
}

func newDocumentWatcher(path string) (*documentWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	dw := &documentWatcher{
		path:    path,
		watcher: w,
		done:    make(chan struct{}),
		changed: make(chan struct{}, 1),
	}

	dir := filepath.Dir(path)
	if dir == "" {
		dir = "."
	}
	if err := w.Add(dir); err != nil {
		w.Close()
		return nil, err
	}

	return dw, nil
}

func (dw *documentWatcher) record(path string) {
	dw.mu.Lock()
	dw.path = path
	dw.mu.Unlock()

	if info, err := os.Stat(path); err == nil {
		dw.mu.Lock()
		dw.mod = info.ModTime()
		dw.size = info.Size()
		dw.mu.Unlock()
	}

	dw.mu.Lock()
	if !dw.started {
		dw.started = true
		dw.mu.Unlock()
		go dw.loop()
	} else {
		dw.mu.Unlock()
	}
}

func (dw *documentWatcher) loop() {
	debounce := time.NewTimer(0)
	if !debounce.Stop() {
		<-debounce.C
	}

	for {
		select {
		case <-dw.done:
			debounce.Stop()
			return
		case event, ok := <-dw.watcher.Events:
			if !ok {
				return
			}
			if !dw.isRelevantEvent(event) {
				continue
			}
			if !debounce.Stop() {
				select {
				case <-debounce.C:
				default:
				}
			}
			debounce.Reset(documentReloadDebounce)
		case <-dw.watcher.Errors:
			// keep running
		case <-debounce.C:
			dw.checkAndNotify()
		}
	}
}

func (dw *documentWatcher) isRelevantEvent(event fsnotify.Event) bool {
	return event.Has(fsnotify.Write) || event.Has(fsnotify.Chmod) || event.Has(fsnotify.Create) || event.Has(fsnotify.Rename)
}

func (dw *documentWatcher) checkAndNotify() {
	dw.mu.Lock()
	defer dw.mu.Unlock()

	info, err := os.Stat(dw.path)
	if err != nil || info.IsDir() {
		return
	}

	mod, size := info.ModTime(), info.Size()
	if mod.Equal(dw.mod) && size == dw.size {
		return
	}

	dw.mod = mod
	dw.size = size
	dw.pending = watcherChange{
		mod:       mod,
		size:      size,
		firstSeen: time.Now(),
	}

	select {
	case dw.changed <- struct{}{}:
	default:
	}
}

func (dw *documentWatcher) waitForChange(timeout time.Duration) (watcherChange, bool) {
	if timeout <= 0 {
		select {
		case <-dw.changed:
			dw.mu.Lock()
			change := dw.pending
			dw.pending = watcherChange{}
			dw.mu.Unlock()
			return change, true
		default:
			return watcherChange{}, false
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-dw.changed:
		dw.mu.Lock()
		change := dw.pending
		dw.pending = watcherChange{}
		dw.mu.Unlock()
		return change, true
	case <-timer.C:
		return watcherChange{}, false
	}
}

func (dw *documentWatcher) Close() {
	select {
	case <-dw.done:
		return
	default:
		close(dw.done)
		dw.watcher.Close()
	}
}
