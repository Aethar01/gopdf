//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

func platformDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "onon")
	}
	return ""
}

// platformSearchPaths are the directories searched for kind, "plugins" or
// "themes".
func platformSearchPaths(kind string) []string {
	paths := make([]string, 0, 2)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "onon", kind),
			filepath.Join(home, ".config", "onon", kind),
		)
	}
	return paths
}

func platformConfigPaths() []string {
	if home, err := os.UserHomeDir(); err == nil {
		return []string{
			filepath.Join(home, "Library", "Application Support", "onon", "config.lua"),
			filepath.Join(home, ".config", "onon", "config.lua"),
		}
	}
	return nil
}

// platformAutogenPath is where onon used to keep keybindings changed in
// the viewer; see importAutogen.
func platformAutogenPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "onon", "autogen.lua")
	}
	return ""
}

// platformAppDirParents are the directories that hold an onon directory
// of the user's: config and data, and the ~/.config some people prefer.
func platformAppDirParents() []string {
	if home, err := os.UserHomeDir(); err == nil {
		return []string{filepath.Join(home, "Library", "Application Support"), filepath.Join(home, ".config")}
	}
	return nil
}
