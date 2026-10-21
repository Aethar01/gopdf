//go:build linux

package viewer

import "gopdf/internal/xdgrecent"

// noteSystemRecentDocument adds path to the recent files the desktop
// shares, which file managers and GTK's file chooser list. It writes a
// file, so it does so away from the event loop.
func noteSystemRecentDocument(path string) {
	go func() { _ = xdgrecent.Add(path) }()
}
