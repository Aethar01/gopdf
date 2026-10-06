//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

func platformDataDir() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "gopdf")
	}
	return ""
}

// platformSearchPaths are the directories searched for kind, "plugins" or
// "themes".
func platformSearchPaths(kind string) []string {
	paths := make([]string, 0, 2)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths,
			filepath.Join(home, "Library", "Application Support", "gopdf", kind),
			filepath.Join(home, ".config", "gopdf", kind),
		)
	}
	return paths
}

func platformConfigPaths() []string {
	if home, err := os.UserHomeDir(); err == nil {
		return []string{
			filepath.Join(home, "Library", "Application Support", "gopdf", "config.lua"),
			filepath.Join(home, ".config", "gopdf", "config.lua"),
		}
	}
	return nil
}

// platformAutogenPath is where gopdf used to keep keybindings changed in
// the viewer; see importAutogen.
func platformAutogenPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "gopdf", "autogen.lua")
	}
	return ""
}
