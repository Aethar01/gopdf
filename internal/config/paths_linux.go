//go:build linux

package config

import (
	"os"
	"path/filepath"
	"strings"
)

func platformStatePath() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "gopdf", "state")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "gopdf", "state")
	}
	return ""
}

func platformConfigPaths() []string {
	paths := make([]string, 0, 6)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "gopdf", "config.lua"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "gopdf", "config.lua"))
	}
	for _, dir := range strings.Split(os.Getenv("XDG_CONFIG_DIRS"), ":") {
		if dir == "" {
			continue
		}
		paths = append(paths, filepath.Join(dir, "gopdf", "config.lua"))
	}
	paths = append(paths, filepath.Join(string(filepath.Separator), "etc", "xdg", "gopdf", "config.lua"))
	return paths
}
