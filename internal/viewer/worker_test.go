package viewer

import "testing"

func TestWorkerUpdatesWakeTheLoop(t *testing.T) {
	woken := 0
	w := newWorkerLifecycle(func() { woken++ })
	updates := make(chan int, 1)
	if !sendWorkerUpdate(&w, updates, 1) || woken != 1 {
		t.Fatalf("sent update woke the loop %d times, want 1", woken)
	}
	w.Close()
	if sendWorkerUpdate(&w, updates, 2) || woken != 1 {
		t.Fatalf("closed worker woke the loop (woken=%d)", woken)
	}
}
