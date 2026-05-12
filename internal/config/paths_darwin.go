//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

func platformStatePath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "gopdf", "state")
	}
	return ""
}

func platformConfigPaths() []string {
	if home, err := os.UserHomeDir(); err == nil {
		return []string{filepath.Join(home, "Library", "Application Support", "gopdf", "config.lua")}
	}
	return nil
}
