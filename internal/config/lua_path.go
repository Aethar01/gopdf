package config

import (
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

// NewLuaPathModule returns portable path operations backed by path/filepath.
func NewLuaPathModule(L *lua.LState) *lua.LTable {
	mod := L.NewTable()
	L.SetField(mod, "separator", lua.LString(string(filepath.Separator)))
	L.SetField(mod, "list_separator", lua.LString(string(filepath.ListSeparator)))
	functions := map[string]lua.LGFunction{
		"join": func(L *lua.LState) int {
			parts := make([]string, L.GetTop())
			for i := range parts {
				parts[i] = L.CheckString(i + 1)
			}
			L.Push(lua.LString(filepath.Join(parts...)))
			return 1
		},
		"clean":     stringPathFunc(filepath.Clean),
		"basename":  stringPathFunc(filepath.Base),
		"dirname":   stringPathFunc(filepath.Dir),
		"extension": stringPathFunc(filepath.Ext),
		"is_absolute": func(L *lua.LState) int {
			L.Push(lua.LBool(filepath.IsAbs(L.CheckString(1))))
			return 1
		},
		"absolute": func(L *lua.LState) int {
			path, err := filepath.Abs(L.CheckString(1))
			if err != nil {
				L.RaiseError("path.abs: %v", err)
			}
			L.Push(lua.LString(path))
			return 1
		},
		"relative": func(L *lua.LState) int {
			path, err := filepath.Rel(L.CheckString(1), L.CheckString(2))
			if err != nil {
				L.RaiseError("path.rel: %v", err)
			}
			L.Push(lua.LString(path))
			return 1
		},
		"expand_home": func(L *lua.LState) int {
			path := L.CheckString(1)
			if path != "~" && !strings.HasPrefix(path, "~/") && !strings.HasPrefix(path, `~\`) {
				L.Push(lua.LString(path))
				return 1
			}
			home, err := os.UserHomeDir()
			if err != nil {
				L.RaiseError("path.expand_home: %v", err)
			}
			switch {
			case path == "~":
				path = home
			case strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`):
				path = filepath.Join(home, path[2:])
			}
			L.Push(lua.LString(path))
			return 1
		},
	}
	// Short names remain useful when requiring the standalone module; gopdf.path
	// documents the explicit API 2 names below.
	functions["base"] = functions["basename"]
	functions["dir"] = functions["dirname"]
	functions["ext"] = functions["extension"]
	functions["is_abs"] = functions["is_absolute"]
	functions["abs"] = functions["absolute"]
	functions["rel"] = functions["relative"]
	L.SetFuncs(mod, functions)
	return mod
}

func stringPathFunc(fn func(string) string) lua.LGFunction {
	return func(L *lua.LState) int {
		L.Push(lua.LString(fn(L.CheckString(1))))
		return 1
	}
}
