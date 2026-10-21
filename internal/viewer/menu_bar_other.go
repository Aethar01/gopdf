//go:build !darwin

package viewer

import "gopdf/internal/menubar"

// setNativeMenuBar shows menus as the system's menu bar. Only macOS has one.
func setNativeMenuBar(menus []menubar.Menu, bindings map[string]string) {}
