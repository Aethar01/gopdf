package config

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type LastState struct {
	Path string
	Page int
}

func StatePath() string {
	return platformStatePath()
}

func GetLastFile() string {
	return GetLastState().Path
}

func GetLastState() LastState {
	path := StatePath()
	if path == "" {
		return LastState{Page: 1}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return LastState{Page: 1}
	}
	return parseLastState(string(data))
}

func SetLastFile(path string) error {
	return SetLastState(LastState{Path: path, Page: 1})
}

func SetLastState(state LastState) error {
	statePath := StatePath()
	if statePath == "" {
		return nil
	}
	path := AbsoluteDocumentPath(state.Path)
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	page := state.Page
	if page < 1 {
		page = 1
	}
	return os.WriteFile(statePath, []byte(path+"\n"+strconv.Itoa(page)+"\n"), 0644)
}

func parseLastState(data string) LastState {
	state := LastState{Page: 1}
	lines := strings.Split(strings.ReplaceAll(data, "\r\n", "\n"), "\n")
	if len(lines) > 0 {
		state.Path = strings.TrimSpace(lines[0])
	}
	if len(lines) > 1 {
		if page, err := strconv.Atoi(strings.TrimSpace(lines[1])); err == nil && page > 0 {
			state.Page = page
		}
	}
	return state
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
