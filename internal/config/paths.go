package config

import "path/filepath"

func DataDir() string {
	return platformDataDir()
}

func PluginPaths() []string {
	return unique(platformSearchPaths("plugins"))
}

// ThemePaths are the directories :theme looks in after the one beside
// config.lua, the same as PluginPaths with themes in place of plugins.
func ThemePaths() []string {
	return unique(platformSearchPaths("themes"))
}

func AbsoluteDocumentPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}
