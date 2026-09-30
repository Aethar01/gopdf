//go:build darwin

package viewer

/*
#cgo LDFLAGS: -framework Cocoa -framework CoreServices
#include <stdlib.h>
void gopdfWatchLaunchEvents(void);
int gopdfLaunchSettled(void);
int gopdfLaunchDocumentCount(void);
char *gopdfLaunchDocument(int index);
void gopdfFinishLaunchDocuments(void);
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
// must run before anything else does.
func LaunchDocuments(verbose bool) []string {
	start := time.Now()
	C.gopdfWatchLaunchEvents()
	if err := initSDL(); err != nil {
		return nil // Run reports the failure
	}
	settled := false
	deadline := start.Add(launchDocumentsWait)
	for {
		sdl.PumpEvents()
		if C.gopdfLaunchSettled() != 0 {
			settled = true
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	var paths []string
	for i := range int(C.gopdfLaunchDocumentCount()) {
		path := C.gopdfLaunchDocument(C.int(i))
		paths = append(paths, C.GoString(path))
		C.free(unsafe.Pointer(path))
	}
	C.gopdfFinishLaunchDocuments()
	if len(paths) > 0 {
		// AppKit may also have handed them to SDL; startup opens them instead.
		sdl.FlushEvents(sdl.EventDropFile, sdl.EventDropComplete)
	}
	if verbose {
		log.Printf("launch documents=%q settled=%t after %s", paths, settled, time.Since(start))
	}
	return paths
}
