//go:build darwin

package viewer

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreServices
#include <stdlib.h>
void gopdfWatchLaunch(void);
int gopdfLaunchHandled(void);
char **gopdfTakeLaunchDocuments(int *count);
*/
import "C"

import (
	"log"
	"time"
	"unsafe"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

// launchDocumentsWait bounds how long startup waits to learn which documents
// macOS launched gopdf for. A request that comes later still arrives as a drop.
const launchDocumentsWait = 2 * time.Second

// LaunchDocuments returns the documents macOS launched gopdf to open. Finder
// and `open` send them in an Apple Event rather than as arguments, and it
// arrives once the application has finished launching, so this starts SDL; it
// must run once, before anything else does.
func LaunchDocuments(verbose bool) []string {
	start := time.Now()
	C.gopdfWatchLaunch()
	if err := initSDL(); err != nil {
		return nil // Run reports the failure
	}
	launchHandled := func() bool {
		sdl.PumpEvents()
		return C.gopdfLaunchHandled() != 0
	}
	deadline := start.Add(launchDocumentsWait)
	handled := launchHandled()
	for !handled && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
		handled = launchHandled()
	}
	paths := takeLaunchDocuments()
	if len(paths) > 0 {
		// AppKit may also have handed them to SDL; startup opens them instead.
		sdl.FlushEvents(sdl.EventDropFile, sdl.EventDropComplete)
	}
	if verbose {
		log.Printf("launch documents=%q handled=%t after %s", paths, handled, time.Since(start))
	}
	return paths
}

func takeLaunchDocuments() []string {
	var count C.int
	array := C.gopdfTakeLaunchDocuments(&count)
	if array == nil {
		return nil
	}
	defer C.free(unsafe.Pointer(array))
	paths := make([]string, 0, int(count))
	for _, path := range unsafe.Slice(array, int(count)) {
		paths = append(paths, C.GoString(path))
		C.free(unsafe.Pointer(path))
	}
	return paths
}
