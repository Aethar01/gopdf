//go:build linux

package config

import (
	"os"
	"path/filepath"
	"strings"
)

func platformDataDir() string {
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		return filepath.Join(xdgData, "gopdf")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".local", "share", "gopdf")
	}
	return ""
}

func platformPluginPaths() []string {
	paths := make([]string, 0, 4)
	if xdgData := os.Getenv("XDG_DATA_HOME"); xdgData != "" {
		paths = append(paths, filepath.Join(xdgData, "gopdf", "plugins"))
	} else if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".local", "share", "gopdf", "plugins"))
	}
	if xdgConfig := os.Getenv("XDG_CONFIG_HOME"); xdgConfig != "" {
		paths = append(paths, filepath.Join(xdgConfig, "gopdf", "plugins"))
	}
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "gopdf", "plugins"))
	}
	for _, dir := range strings.Split(os.Getenv("XDG_CONFIG_DIRS"), ":") {
		if dir != "" {
			paths = append(paths, filepath.Join(dir, "gopdf", "plugins"))
		}
	}
	return paths
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

func platformAutogenPath() string {
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		return filepath.Join(xdg, "gopdf", "autogen.lua")
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, ".config", "gopdf", "autogen.lua")
	}
	return ""
}
