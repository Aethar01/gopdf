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

func platformConfigPaths() []string {
	if home, err := os.UserHomeDir(); err == nil {
		return []string{filepath.Join(home, "Library", "Application Support", "gopdf", "config.lua")}
	}
	return nil
}

func platformAutogenPath() string {
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, "Library", "Application Support", "gopdf", "autogen.lua")
	}
	return ""
}
