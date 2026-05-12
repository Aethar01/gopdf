//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func platformStatePath() string {
	if dir := appDataDir(); dir != "" {
		return filepath.Join(dir, "gopdf", "state")
	}
	return ""
}

func platformConfigPaths() []string {
	if dir := appDataDir(); dir != "" {
		return []string{filepath.Join(dir, "gopdf", "config.lua")}
	}
	return nil
}

func appDataDir() string {
	if dir := os.Getenv("APPDATA"); dir != "" {
		return dir
	}
	return os.Getenv("USERPROFILE")
}
