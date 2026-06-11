package config

import (
	"fmt"
	"strings"

	"gopdf/internal/actions"
	"gopdf/internal/filepicker"

	lua "github.com/yuin/gopher-lua"
)

func (r *Runtime) applyLuaConfig(path string) error {
	L := r.initLuaState()
	if err := L.DoFile(path); err != nil {
		L.Close()
		r.state = nil
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (r *Runtime) initLuaState() *lua.LState {
	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
	L := lua.NewState()
	r.state = L
	mod := newLuaModule(L, r, &r.cfg)
	L.SetGlobal("gopdf", mod)
	L.SetGlobal("bind", L.GetField(mod, "bind"))
	L.SetGlobal("unbind", L.GetField(mod, "unbind"))
	L.SetGlobal("bind_mouse", L.GetField(mod, "bind_mouse"))
	L.SetGlobal("unbind_mouse", L.GetField(mod, "unbind_mouse"))
	L.SetGlobal("options", L.GetField(mod, "options"))
	return L
}

func newLuaModule(L *lua.LState, rt *Runtime, cfg *Config) *lua.LTable {
	mod := L.NewTable()
	document := L.NewTable()
	L.SetField(document, "path", lua.LString(rt.docPath))
	L.SetField(document, "name", lua.LString(rt.docName))
	L.SetField(document, "extension", lua.LString(rt.docMeta.ext))
	L.SetField(document, "exists", lua.LBool(rt.docMeta.exists))
	if rt.docMeta.exists {
		L.SetField(document, "size_bytes", lua.LNumber(rt.docMeta.sizeBytes))
	}
	if rt.docMeta.hasPages {
		L.SetField(document, "page_count", lua.LNumber(rt.docMeta.pageCount))
	}
	L.SetField(mod, "document", document)
	L.SetField(mod, "cache", newLuaCacheTable(L, rt))
	L.SetField(mod, "ui", newLuaUITable(L, rt))
	L.SetFuncs(mod, map[string]lua.LGFunction{
		"bind": func(L *lua.LState) int {
			key := L.CheckString(1)
			action := L.CheckAny(2)
			actionName, err := luaActionName(rt, action)
			if err != nil {
				L.RaiseError("bind %q: %v", key, err)
			}
			rt.setKeyBinding(key, actionName)
			return 0
		},
		"unbind": func(L *lua.LState) int {
			key := L.CheckString(1)
			rt.unbindKey(key)
			return 0
		},
		"bind_mouse": func(L *lua.LState) int {
			event := normalizeMouseEvent(L.CheckString(1))
			action := L.CheckAny(2)
			actionName, err := luaActionName(rt, action)
			if err != nil {
				L.RaiseError("bind_mouse %q: %v", event, err)
			}
			rt.setMouseBinding(event, actionName)
			return 0
		},
		"unbind_mouse": func(L *lua.LState) int {
			event := normalizeMouseEvent(L.CheckString(1))
			rt.unbindMouse(event)
			return 0
		},

		"message": func(L *lua.LState) int {
			if L.GetTop() > 0 {
				if rt.host == nil {
					L.RaiseError("message: viewer host unavailable")
				}
				rt.host.SetMessage(L.CheckString(1))
				return 0
			}
			if rt.host == nil {
				L.Push(lua.LString(cfg.NormalMessage))
				return 1
			}
			L.Push(lua.LString(rt.host.Message()))
			return 1
		},
		"command": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("command: viewer host unavailable")
			}
			if err := rt.host.RunCommand(L.CheckString(1)); err != nil {
				L.RaiseError("command: %v", err)
			}
			return 0
		},
		"open": func(L *lua.LState) int {
			if err := rt.open(L.CheckString(1)); err != nil {
				L.RaiseError("open: %v", err)
			}
			return 0
		},
		"pick_file": func(L *lua.LState) int {
			path, err := filepicker.PickPDF()
			if err != nil {
				L.RaiseError("pick_file: %v", err)
			}
			if fn, ok := L.Get(1).(*lua.LFunction); ok && path != "" {
				if err := rt.state.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}, lua.LString(path)); err != nil {
					L.RaiseError("pick_file: %v", err)
				}
				return 0
			}
			L.Push(lua.LString(path))
			return 1
		},
		"page": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(rt.host.Page()))
			return 1
		},
		"page_count": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(rt.host.PageCount()))
			return 1
		},
		"goto_page": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("goto_page: viewer host unavailable")
			}
			if err := rt.host.GotoPage(L.CheckInt(1)); err != nil {
				L.RaiseError("goto_page: %v", err)
			}
			return 0
		},
		"mode": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LString(rt.host.Mode()))
			return 1
		},
		"search": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("search: viewer host unavailable")
			}
			backward := false
			if L.GetTop() >= 2 {
				backward = lua.LVAsBool(L.CheckAny(2))
			}
			if err := rt.host.Search(L.CheckString(1), backward); err != nil {
				L.RaiseError("search: %v", err)
			}
			return 0
		},
		"search_query": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LString(""))
				return 1
			}
			L.Push(lua.LString(rt.host.SearchQuery()))
			return 1
		},
		"search_match_count": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(rt.host.SearchMatchCount()))
			return 1
		},
		"search_match_index": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			index := rt.host.SearchMatchIndex()
			if index <= 0 {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(index))
			return 1
		},
		"current_count": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LString(""))
				return 1
			}
			L.Push(lua.LString(rt.host.CurrentCount()))
			return 1
		},
		"pending_keys": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(L.NewTable())
				return 1
			}
			L.Push(luaStringsTable(L, rt.host.PendingKeys()))
			return 1
		},
		"recent_files": func(L *lua.LState) int {
			if !cfg.SessionDatabase {
				L.Push(L.NewTable())
				return 1
			}
			limit := cfg.RecentFilesMax
			if L.GetTop() > 0 {
				limit = L.CheckInt(1)
			}
			L.Push(luaStringsTable(L, RecentFiles(limit)))
			return 1
		},
		"clear_pending_keys": func(L *lua.LState) int {
			if rt.host == nil {
				return 0
			}
			rt.host.ClearPendingKeys()
			return 0
		},
		"fit_mode": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LString(cfg.FitMode))
				return 1
			}
			L.Push(lua.LString(rt.host.FitMode()))
			return 1
		},
		"set_fit_mode": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_fit_mode: viewer host unavailable")
			}
			if err := rt.host.SetFitMode(L.CheckString(1)); err != nil {
				L.RaiseError("set_fit_mode: %v", err)
			}
			return 0
		},
		"render_mode": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LString(cfg.RenderMode))
				return 1
			}
			L.Push(lua.LString(rt.host.RenderMode()))
			return 1
		},
		"set_render_mode": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_render_mode: viewer host unavailable")
			}
			if err := rt.host.SetRenderMode(L.CheckString(1)); err != nil {
				L.RaiseError("set_render_mode: %v", err)
			}
			return 0
		},
		"zoom": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(rt.host.Zoom()))
			return 1
		},
		"set_zoom": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_zoom: viewer host unavailable")
			}
			if err := rt.host.SetZoom(float64(L.CheckNumber(1))); err != nil {
				L.RaiseError("set_zoom: %v", err)
			}
			return 0
		},
		"rotation": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNil)
				return 1
			}
			L.Push(lua.LNumber(rt.host.Rotation()))
			return 1
		},
		"set_rotation": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_rotation: viewer host unavailable")
			}
			if err := rt.host.SetRotation(float64(L.CheckNumber(1))); err != nil {
				L.RaiseError("set_rotation: %v", err)
			}
			return 0
		},
		"fullscreen": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LFalse)
				return 1
			}
			L.Push(lua.LBool(rt.host.Fullscreen()))
			return 1
		},
		"set_fullscreen": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_fullscreen: viewer host unavailable")
			}
			if err := rt.host.SetFullscreen(lua.LVAsBool(L.CheckAny(1))); err != nil {
				L.RaiseError("set_fullscreen: %v", err)
			}
			return 0
		},
		"status_bar_visible": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LBool(cfg.StatusBarVisible))
				return 1
			}
			L.Push(lua.LBool(rt.host.StatusBarVisible()))
			return 1
		},
		"set_status_bar_visible": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("set_status_bar_visible: viewer host unavailable")
			}
			if err := rt.host.SetStatusBarVisible(lua.LVAsBool(L.CheckAny(1))); err != nil {
				L.RaiseError("set_status_bar_visible: %v", err)
			}
			return 0
		},
		"jump_forward": func(L *lua.LState) int {
			L.Push(lua.LString("jump_forward"))
			return 1
		},
		"jump_backward": func(L *lua.LState) int {
			L.Push(lua.LString("jump_backward"))
			return 1
		},
	})
	L.SetField(mod, "options", newLuaOptionsTable(L, rt, cfg))
	for _, action := range actions.Names() {
		name := action
		L.SetField(mod, name, newLuaActionValue(L, rt, name))
	}
	L.SetField(mod, "status_bar", newLuaStatusBarTable(L, rt, cfg))
	return mod
}

func newLuaOptionsTable(L *lua.LState, rt *Runtime, cfg *Config) *lua.LTable {
	tbl := L.NewTable()
	mt := L.NewTable()
	L.SetField(mt, "__newindex", L.NewFunction(func(L *lua.LState) int {
		name := strings.ToLower(strings.TrimSpace(L.CheckString(2)))
		value := L.CheckAny(3)
		if err := rt.setOption(name, value); err != nil {
			L.RaiseError("options.%s: %v", name, err)
		}
		return 0
	}))
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		name := strings.ToLower(strings.TrimSpace(L.CheckString(2)))
		value, err := luaSettingValue(L, name, cfg)
		if err != nil {
			L.RaiseError("options.%s: %v", name, err)
		}
		L.Push(value)
		return 1
	}))
	L.SetMetatable(tbl, mt)
	return tbl
}

func newLuaCacheTable(L *lua.LState, rt *Runtime) *lua.LTable {
	tbl := L.NewTable()
	L.SetFuncs(tbl, map[string]lua.LGFunction{
		"entries": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(rt.host.CacheEntries()))
			return 1
		},
		"pending": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(rt.host.CachePending()))
			return 1
		},
		"limit": func(L *lua.LState) int {
			if rt.host == nil {
				L.Push(lua.LNumber(0))
				return 1
			}
			L.Push(lua.LNumber(rt.host.CacheLimit()))
			return 1
		},
		"set_limit": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("cache.set_limit: viewer host unavailable")
			}
			if err := rt.host.SetCacheLimit(L.CheckInt(1)); err != nil {
				L.RaiseError("cache.set_limit: %v", err)
			}
			return 0
		},
		"clear": func(L *lua.LState) int {
			if rt.host != nil {
				rt.host.ClearCache()
			}
			return 0
		},
	})
	return tbl
}

func newLuaUITable(L *lua.LState, rt *Runtime) *lua.LTable {
	tbl := L.NewTable()
	show := func(L *lua.LState) int {
		if rt.host == nil {
			L.RaiseError("ui.show: viewer host unavailable")
		}
		spec, ok := L.CheckAny(1).(*lua.LTable)
		if !ok {
			L.RaiseError("ui.show: expected table")
		}
		overlay := UIOverlay{
			Title:    lua.LVAsString(spec.RawGetString("title")),
			Rows:     luaTableStrings(spec.RawGetString("rows")),
			Selected: 1,
		}
		if selected := spec.RawGetString("selected"); selected.Type() == lua.LTNumber {
			overlay.Selected = int(lua.LVAsNumber(selected))
		}
		if fn, ok := spec.RawGetString("on_select").(*lua.LFunction); ok {
			overlay.OnSelect = rt.registerCallback(fn)
		}
		if fn, ok := spec.RawGetString("on_close").(*lua.LFunction); ok {
			overlay.OnClose = rt.registerCallback(fn)
		}
		if err := rt.host.ShowUI(overlay); err != nil {
			L.RaiseError("ui.show: %v", err)
		}
		return 0
	}
	L.SetFuncs(tbl, map[string]lua.LGFunction{
		"show": show,
		"close": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("ui.close: viewer host unavailable")
			}
			rt.host.CloseUI()
			return 0
		},
		"visible": func(L *lua.LState) int {
			L.Push(lua.LBool(rt.host != nil && rt.host.UIVisible()))
			return 1
		},
		"set_rows": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("ui.set_rows: viewer host unavailable")
			}
			rt.host.SetUIRows(luaTableStrings(L.CheckAny(1)))
			return 0
		},
		"set_selected": func(L *lua.LState) int {
			if rt.host == nil {
				L.RaiseError("ui.set_selected: viewer host unavailable")
			}
			rt.host.SetUISelected(L.CheckInt(1))
			return 0
		},
	})
	return tbl
}

func luaTableStrings(value lua.LValue) []string {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	values := make([]string, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		values = append(values, lua.LVAsString(tbl.RawGetInt(i)))
	}
	return values
}

func newLuaStatusBarTable(L *lua.LState, rt *Runtime, cfg *Config) *lua.LTable {
	tbl := L.NewTable()
	mt := L.NewTable()
	L.SetField(mt, "__newindex", L.NewFunction(func(L *lua.LState) int {
		name := strings.ToLower(strings.TrimSpace(L.CheckString(2)))
		value := L.CheckAny(3)
		switch name {
		case "left":
			cfg.StatusBarLeft = lua.LVAsString(value)
		case "right":
			cfg.StatusBarRight = lua.LVAsString(value)
		case "height":
			cfg.StatusBarHeight = int(lua.LVAsNumber(value))
		case "visible":
			if rt.host != nil {
				rt.host.SetStatusBarVisible(lua.LVAsBool(value))
			}
		default:
			L.RaiseError("status_bar.%s: unknown option", name)
		}
		rt.dirty = true
		return 0
	}))
	L.SetField(mt, "__index", L.NewFunction(func(L *lua.LState) int {
		name := strings.ToLower(strings.TrimSpace(L.CheckString(2)))
		switch name {
		case "left":
			L.Push(lua.LString(cfg.StatusBarLeft))
		case "right":
			L.Push(lua.LString(cfg.StatusBarRight))
		case "height":
			L.Push(lua.LNumber(cfg.StatusBarHeight))
		case "visible":
			if rt.host != nil {
				L.Push(lua.LBool(rt.host.StatusBarVisible()))
			} else {
				L.Push(lua.LBool(cfg.StatusBarVisible))
			}
		default:
			L.RaiseError("status_bar.%s: unknown option", name)
		}
		return 1
	}))
	L.SetMetatable(tbl, mt)
	return tbl
}

func luaStringsTable(L *lua.LState, values []string) *lua.LTable {
	tbl := L.NewTable()
	for i, value := range values {
		tbl.RawSetInt(i+1, lua.LString(value))
	}
	return tbl
}

func newLuaActionValue(L *lua.LState, rt *Runtime, action string) *lua.LTable {
	tbl := L.NewTable()
	L.SetField(tbl, "__gopdf_action", lua.LString(action))
	mt := L.NewTable()
	L.SetField(mt, "__call", L.NewFunction(func(L *lua.LState) int {
		if rt.host == nil {
			L.RaiseError("%s: cannot execute during config load; pass gopdf.%s to bind(...) or call it from a callback", action, action)
		}
		if err := rt.host.ExecuteAction(action); err != nil {
			L.RaiseError("%s: %v", action, err)
		}
		return 0
	}))
	L.SetField(mt, "__tostring", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LString(action))
		return 1
	}))
	L.SetMetatable(tbl, mt)
	return tbl
}

func luaActionName(rt *Runtime, value lua.LValue) (string, error) {
	if fn, ok := value.(*lua.LFunction); ok {
		return rt.registerCallback(fn), nil
	}
	if tbl, ok := value.(*lua.LTable); ok {
		if action := tbl.RawGetString("__gopdf_action"); action.Type() == lua.LTString {
			return action.String(), nil
		}
	}
	if value.Type() != lua.LTString {
		return "", fmt.Errorf("expected action string, action value, or function")
	}
	action := value.String()
	for _, candidate := range actions.Names() {
		if action == candidate {
			return action, nil
		}
	}
	return "", fmt.Errorf("unknown action %q", action)
}

func (r *Runtime) registerCallback(fn *lua.LFunction) string {
	r.callbackSeq++
	id := fmt.Sprintf("__lua_callback_%d", r.callbackSeq)
	r.callbacks[id] = fn
	return id
}

func (r *Runtime) setKeyBinding(key, action string) {
	r.cfg.KeyBindings[key] = action
	r.dirty = true
}

func (r *Runtime) unbindKey(key string) {
	delete(r.cfg.KeyBindings, key)
	r.dirty = true
}

func (r *Runtime) setMouseBinding(event, action string) {
	r.cfg.MouseBindings[event] = action
	r.dirty = true
}

func (r *Runtime) unbindMouse(event string) {
	delete(r.cfg.MouseBindings, event)
	r.dirty = true
}

func (r *Runtime) setOption(name string, value lua.LValue) error {
	if err := applyLuaSetting(name, value, &r.cfg); err != nil {
		return err
	}
	r.dirty = true
	return nil
}
