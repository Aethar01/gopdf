package config

import (
	"context"
	"os"
	"path/filepath"

	lua "github.com/yuin/gopher-lua"
)

func newLuaPluginFS(L *lua.LState, instance *pluginInstance) *lua.LTable {
	fs := L.NewTable()
	L.SetField(fs, "read_dir", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(2)
		callbackIndex := 3
		follow := false
		if options, ok := L.Get(3).(*lua.LTable); ok {
			follow = lua.LVAsBool(options.RawGetString("follow_symlinks"))
			callbackIndex = 4
		}
		callback, ok := L.Get(callbackIndex).(*lua.LFunction)
		if !ok {
			L.RaiseError("plugin %s fs.read_dir: expected callback", instance.manifest.ID)
		}
		id := instance.runtime.startPluginOperation(instance.manifest.ID, "fs.read_dir", callback, func(ctx context.Context) map[string]any {
			return readDirectoryResult(ctx, path, follow)
		})
		L.Push(newPluginOperationHandle(L, instance.runtime, id))
		return 1
	}))
	L.SetField(fs, "stat", L.NewFunction(func(L *lua.LState) int {
		path := L.CheckString(2)
		callbackIndex := 3
		follow := false
		if options, ok := L.Get(3).(*lua.LTable); ok {
			follow = lua.LVAsBool(options.RawGetString("follow_symlinks"))
			callbackIndex = 4
		}
		callback, ok := L.Get(callbackIndex).(*lua.LFunction)
		if !ok {
			L.RaiseError("plugin %s fs.stat: expected callback", instance.manifest.ID)
		}
		id := instance.runtime.startPluginOperation(instance.manifest.ID, "fs.stat", callback, func(ctx context.Context) map[string]any {
			return statPathResult(ctx, path, follow)
		})
		L.Push(newPluginOperationHandle(L, instance.runtime, id))
		return 1
	}))
	return fs
}

func readDirectoryResult(ctx context.Context, path string, follow bool) map[string]any {
	if !follow {
		if info, err := os.Lstat(path); err != nil {
			return operationError(err)
		} else if info.Mode()&os.ModeSymlink != 0 {
			return operationError(&os.PathError{Op: "read_dir", Path: path, Err: os.ErrInvalid})
		}
	}
	entries, err := os.ReadDir(path)
	if err != nil {
		return operationError(err)
	}
	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return operationError(ctx.Err())
		default:
		}
		entryPath := filepath.Join(path, entry.Name())
		var info os.FileInfo
		if follow {
			info, err = os.Stat(entryPath)
		} else {
			info, err = os.Lstat(entryPath)
		}
		if err != nil {
			// A broken symlink, or a file removed between the read and the stat,
			// describes only itself. Report it from the directory entry so one
			// unreadable name does not discard the whole listing.
			result = append(result, unstattableEntryResult(entryPath, entry, err))
			continue
		}
		result = append(result, fileInfoResult(entryPath, info))
	}
	return map[string]any{"success": true, "error": "", "entries": result}
}

func statPathResult(ctx context.Context, path string, follow bool) map[string]any {
	select {
	case <-ctx.Done():
		return operationError(ctx.Err())
	default:
	}
	var info os.FileInfo
	var err error
	if follow {
		info, err = os.Stat(path)
	} else {
		info, err = os.Lstat(path)
	}
	if err != nil {
		return operationError(err)
	}
	result := fileInfoResult(path, info)
	result["success"] = true
	result["error"] = ""
	return result
}

// unstattableEntryResult describes a directory entry whose stat failed. The
// name and kind still come from the directory itself, and "error" tells Lua the
// remaining fields are unknown rather than genuinely zero.
func unstattableEntryResult(path string, entry os.DirEntry, err error) map[string]any {
	kind := "other"
	switch {
	case entry.Type()&os.ModeSymlink != 0:
		kind = "symlink"
	case entry.IsDir():
		kind = "directory"
	case entry.Type().IsRegular():
		kind = "file"
	}
	return map[string]any{
		"name": entry.Name(), "path": path, "type": kind,
		"size_bytes": 0, "modified_unix": 0, "error": err.Error(),
	}
}

func fileInfoResult(path string, info os.FileInfo) map[string]any {
	kind := "other"
	switch {
	case info.Mode()&os.ModeSymlink != 0:
		kind = "symlink"
	case info.IsDir():
		kind = "directory"
	case info.Mode().IsRegular():
		kind = "file"
	}
	return map[string]any{
		"name": info.Name(), "path": path, "type": kind,
		"size_bytes": info.Size(), "modified_unix": info.ModTime().Unix(), "error": "",
	}
}

func operationError(err error) map[string]any {
	return map[string]any{"success": false, "error": err.Error()}
}
