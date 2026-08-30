//go:build windows

package config

import (
	"os"
	"path/filepath"
)

func platformDataDir() string {
	if dir := appDataDir(); dir != "" {
		return filepath.Join(dir, "gopdf")
	}
	return ""
}

func platformPluginPaths() []string {
	paths := make([]string, 0, 3)
	if dir := os.Getenv("LOCALAPPDATA"); dir != "" {
		paths = append(paths, filepath.Join(dir, "gopdf", "plugins"))
	}
	if dir := os.Getenv("APPDATA"); dir != "" {
		paths = append(paths, filepath.Join(dir, "gopdf", "plugins"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "gopdf", "plugins"))
	}
	return paths
}

func platformConfigPaths() []string {
	if dir := appDataDir(); dir != "" {
		return []string{filepath.Join(dir, "gopdf", "config.lua")}
	}
	return nil
}

func platformAutogenPath() string {
	if dir := appDataDir(); dir != "" {
		return filepath.Join(dir, "gopdf", "autogen.lua")
	}
	return ""
}

func appDataDir() string {
	if dir := os.Getenv("APPDATA"); dir != "" {
		return dir
	}
	return os.Getenv("USERPROFILE")
}
