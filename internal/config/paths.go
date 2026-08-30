package config

import "path/filepath"

func DataDir() string {
	return platformDataDir()
}

func PluginPaths() []string {
	return unique(platformPluginPaths())
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
