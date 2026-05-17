package config

import (
	"os"
	"path/filepath"
	"strings"
)

func StatePath() string {
	return platformStatePath()
}

func GetLastFile() string {
	path := StatePath()
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

func SetLastFile(path string) error {
	statePath := StatePath()
	if statePath == "" {
		return nil
	}
	path = AbsoluteDocumentPath(path)
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(statePath, []byte(path), 0644)
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
