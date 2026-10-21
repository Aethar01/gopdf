//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func platformDataDir() string {
	if dir := appDataDir(); dir != "" {
		return filepath.Join(dir, "onon")
	}
	return ""
}

// platformSearchPaths are the directories searched for kind, "plugins" or
// "themes".
func platformSearchPaths(kind string) []string {
	paths := make([]string, 0, 2)
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		paths = append(paths, filepath.Join(dir, "onon", kind))
	}
	if dir := os.Getenv("APPDATA"); dir != "" {
		paths = append(paths, filepath.Join(dir, "onon", kind))
	}
	return paths
}

func platformConfigPaths() []string {
	if dir := appDataDir(); dir != "" {
		return []string{filepath.Join(dir, "onon", "config.lua")}
	}
	return nil
}

// platformAutogenPath is where onon used to keep keybindings changed in
// the viewer; see importAutogen.
func platformAutogenPath() string {
	if dir := appDataDir(); dir != "" {
		return filepath.Join(dir, "onon", "autogen.lua")
	}
	return ""
}

func appDataDir() string {
	if dir := os.Getenv("APPDATA"); dir != "" {
		return dir
	}
	return os.Getenv("USERPROFILE")
}
