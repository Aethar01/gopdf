package config

import (
	"fmt"
	"log"
	"strings"
	"sync"

	"gopdf/internal/actions"
	"gopdf/internal/filepicker"

	lua "github.com/yuin/gopher-lua"
)

type luaFunctionSpec struct {
	Signature   string
	Description string
	Function    lua.LGFunction
}

var luaFunctionReferences = map[string]LuaReferenceEntry{}
var luaFunctionReferencesMu sync.Mutex

func (r *Runtime) applyLuaConfig(path string) error {
	L := r.state
	if L == nil {
		L = r.initLuaState()
	}
	if err := L.DoFile(path); err != nil {
		r.closeLuaState()
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func (r *Runtime) initLuaState() *lua.LState {
	if r.state != nil {
		r.closeLuaState()
	}
	L := lua.NewState()
	PreloadPortableLuaModules(L)
	r.state = L
	r.pluginGeneration++
	if r.plugins == nil {
		r.plugins = newPluginState(r)
	}
	installPluginPackagePath(L, r)
	mod := newLuaModule(L, r, &r.cfg)
	L.SetGlobal("gopdf", mod)
	L.SetGlobal("bind", L.GetField(mod, "bind"))
	L.SetGlobal("unbind", L.GetField(mod, "unbind"))
	L.SetGlobal("bind_mouse", L.GetField(mod, "bind_mouse"))
	L.SetGlobal("unbind_mouse", L.GetField(mod, "unbind_mouse"))
	L.SetGlobal("options", L.GetField(mod, "options"))
	return L
}

func (r *Runtime) updateLuaDocument() {
	if r.state == nil {
		return
	}
	mod, ok := r.state.GetGlobal("gopdf").(*lua.LTable)
	if ok {
		r.state.SetField(mod, "document", newLuaDocumentTable(r.state, r))
	}
}

func newLuaModule(L *lua.LState, rt *Runtime, cfg *Config) *lua.LTable {
	mod := L.NewTable()
	L.SetField(mod, "document", newLuaDocumentTable(L, rt))
	L.SetField(mod, "cache", newLuaCacheTable(L, rt))
	L.SetField(mod, "ui", newLuaViewAPI(L, rt))
	L.SetField(mod, "plugin", newLuaPluginAPI(L, rt))
	L.SetField(mod, "platform", NewLuaPlatformModule(L))
	L.SetField(mod, "path", NewLuaPathModule(L))
	L.SetField(mod, "json", NewLuaJSONModule(L))
	L.SetField(mod, "clipboard", newLuaClipboardTable(L, rt))
	L.SetField(mod, "formats", newLuaFormatsTable(L, rt))
	registerLuaFunctions(L, mod, "gopdf.", []luaFunctionSpec{
		{
			Signature:   "gopdf.bind(key, action)",
			Description: "Bind a key sequence to an action or Lua callback.",
			Function: func(L *lua.LState) int {
				key := L.CheckString(1)
				action := L.CheckAny(2)
				actionName, err := luaActionName(rt, action)
				if err != nil {
					L.RaiseError("bind %q: %v", key, err)
				}
				rt.setKeyBinding(key, actionName)
				return 0
			},
		},
		{
			Signature:   "gopdf.unbind(key)",
			Description: "Remove a key binding.",
			Function: func(L *lua.LState) int {
				key := L.CheckString(1)
				rt.unbindKey(key)
				return 0
			},
		},
		{
			Signature:   "gopdf.bind_mouse(event, action)",
			Description: "Bind a mouse event to an action or Lua callback.",
			Function: func(L *lua.LState) int {
				event := normalizeMouseEvent(L.CheckString(1))
				action := L.CheckAny(2)
				actionName, err := luaActionName(rt, action)
				if err != nil {
					L.RaiseError("bind_mouse %q: %v", event, err)
				}
				rt.setMouseBinding(event, actionName)
				return 0
			},
		},
		{
			Signature:   "gopdf.unbind_mouse(event)",
			Description: "Remove a mouse binding.",
			Function: func(L *lua.LState) int {
				event := normalizeMouseEvent(L.CheckString(1))
				rt.unbindMouse(event)
				return 0
			},
		},

		{
			Signature:   "gopdf.message([text])",
			Description: "Return the current message, or set and return it when text is supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("message: viewer host unavailable")
					}
					rt.host.SetMessage(L.CheckString(1))
				}
				if rt.host == nil {
					L.Push(lua.LString(cfg.NormalMessage))
					return 1
				}
				L.Push(lua.LString(rt.host.Message()))
				return 1
			},
		},
		{
			Signature:   "gopdf.command(command)",
			Description: "Execute a viewer command.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.RaiseError("command: viewer host unavailable")
				}
				if err := rt.host.RunCommand(L.CheckString(1)); err != nil {
					L.RaiseError("command: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.open(path)",
			Description: "Open another document.",
			Function: func(L *lua.LState) int {
				if err := rt.open(L.CheckString(1)); err != nil {
					L.RaiseError("open: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.pick_file(callback)",
			Description: "Open the native document picker and invoke callback with a structured result. The picker is filtered to the formats this build can open.",
			Function: func(L *lua.LState) int {
				fn, ok := L.Get(1).(*lua.LFunction)
				if !ok {
					L.RaiseError("pick_file: expected callback")
				}
				var extensions []string
				if host, ok := rt.host.(DocumentFormatHost); ok {
					extensions = host.SupportedExtensions()
				}
				path, err := filepicker.PickDocument(extensions)
				result := L.NewTable()
				L.SetField(result, "success", lua.LBool(err == nil && path != ""))
				L.SetField(result, "path", lua.LString(path))
				L.SetField(result, "cancelled", lua.LBool(err == nil && path == ""))
				if err != nil {
					L.SetField(result, "error", lua.LString(err.Error()))
				} else {
					L.SetField(result, "error", lua.LString(""))
				}
				if err := rt.callLua(lua.P{Fn: fn, NRet: 0, Protect: true}, result); err != nil {
					L.RaiseError("pick_file: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.pick_directory(callback)",
			Description: "Open the native directory picker and invoke callback with a structured result.",
			Function: func(L *lua.LState) int {
				fn, ok := L.Get(1).(*lua.LFunction)
				if !ok {
					L.RaiseError("pick_directory: expected callback")
				}
				path := ""
				var err error
				if picker, ok := rt.host.(DirectoryPicker); ok {
					path, err = picker.PickDirectory()
				} else {
					path, err = filepicker.PickDirectory()
				}
				result := luaTableFromMap(L, map[string]any{"success": err == nil && path != "", "path": path, "cancelled": err == nil && path == "", "error": errorString(err)})
				if err := rt.callLua(lua.P{Fn: fn, NRet: 0, Protect: true}, result); err != nil {
					L.RaiseError("pick_directory: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.schedule(callback)",
			Description: "Schedule a callback on the main Lua thread after the current dispatch.",
			Function: func(L *lua.LState) int {
				fn, ok := L.Get(1).(*lua.LFunction)
				if !ok {
					L.RaiseError("schedule: expected callback")
				}
				id, err := rt.schedule(fn)
				if err != nil {
					L.RaiseError("schedule: %v", err)
				}
				L.Push(newPluginOperationHandle(L, rt, id))
				return 1
			},
		},
		{
			Signature:   "gopdf.log(level, message)",
			Description: "Write a plugin diagnostic without changing the user-facing message.",
			Function: func(L *lua.LState) int {
				level := strings.ToLower(strings.TrimSpace(L.CheckString(1)))
				if level != "debug" && level != "info" && level != "warn" && level != "error" {
					L.RaiseError("log: invalid level %q", level)
				}
				log.Printf("plugin=%s level=%s %s", rt.diagnosticPluginID(), level, L.CheckString(2))
				return 0
			},
		},
		{
			Signature:   "gopdf.open_external(uri_or_path)",
			Description: "Open a URI or path with the operating system's default application.",
			Function: func(L *lua.LState) int {
				opener, ok := rt.host.(ExternalOpener)
				if !ok {
					L.RaiseError("open_external: viewer host unavailable")
				}
				if err := opener.OpenExternal(L.CheckString(1)); err != nil {
					L.RaiseError("open_external: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.page([page])",
			Description: "Return the current 1-based physical page number, or go to and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("page: viewer host unavailable")
					}
					if err := rt.host.GotoPage(L.CheckInt(1)); err != nil {
						L.RaiseError("page: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(lua.LNumber(rt.host.Page()))
				return 1
			},
		},
		{
			Signature:   "gopdf.page_count()",
			Description: "Return the document page count.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(lua.LNumber(rt.host.PageCount()))
				return 1
			},
		},
		{
			Signature:   "gopdf.goto_document_point(spec)",
			Description: "Move to a 1-based page and document coordinate.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.RaiseError("goto_document_point: viewer host unavailable")
				}
				spec, ok := L.CheckAny(1).(*lua.LTable)
				if !ok {
					L.RaiseError("goto_document_point: expected table")
				}
				if err := rt.host.GotoDocumentPoint(int(lua.LVAsNumber(spec.RawGetString("page"))), float64(lua.LVAsNumber(spec.RawGetString("x"))), float64(lua.LVAsNumber(spec.RawGetString("y")))); err != nil {
					L.RaiseError("goto_document_point: %v", err)
				}
				return 0
			},
		},
		{
			Signature:   "gopdf.mode()",
			Description: "Return the current input mode.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(lua.LString(rt.host.Mode()))
				return 1
			},
		},
		{
			Signature:   "gopdf.search(query[, backward])",
			Description: "Search using the same flags as :search.",
			Function: func(L *lua.LState) int {
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
		},
		{
			Signature:   "gopdf.search_query()",
			Description: "Return the active search query.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LString(""))
					return 1
				}
				L.Push(lua.LString(rt.host.SearchQuery()))
				return 1
			},
		},
		{
			Signature:   "gopdf.search_match_count()",
			Description: "Return the number of discovered search matches.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LNumber(0))
					return 1
				}
				L.Push(lua.LNumber(rt.host.SearchMatchCount()))
				return 1
			},
		},
		{
			Signature:   "gopdf.search_match_index()",
			Description: "Return the current 1-based match index or nil.",
			Function: func(L *lua.LState) int {
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
		},
		{
			Signature:   "gopdf.current_count()",
			Description: "Return the pending numeric action count.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LString(""))
					return 1
				}
				L.Push(lua.LString(rt.host.CurrentCount()))
				return 1
			},
		},
		{
			Signature:   "gopdf.pending_keys()",
			Description: "Return pending key-sequence tokens.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(L.NewTable())
					return 1
				}
				L.Push(luaStringsTable(L, rt.host.PendingKeys()))
				return 1
			},
		},
		{
			Signature:   "gopdf.recent_files([limit])",
			Description: "Return recent document paths.",
			Function: func(L *lua.LState) int {
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
		},
		{
			Signature:   "gopdf.clear_pending_keys()",
			Description: "Clear the pending sequence, mark, and numeric count.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					return 0
				}
				rt.host.ClearPendingKeys()
				return 0
			},
		},
		{
			Signature:   "gopdf.fit_mode([mode])",
			Description: "Return the current fit mode, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("fit_mode: viewer host unavailable")
					}
					if err := rt.host.SetFitMode(L.CheckString(1)); err != nil {
						L.RaiseError("fit_mode: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LString(cfg.FitMode))
					return 1
				}
				L.Push(lua.LString(rt.host.FitMode()))
				return 1
			},
		},
		{
			Signature:   "gopdf.render_mode([mode])",
			Description: "Return the current render mode, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("render_mode: viewer host unavailable")
					}
					if err := rt.host.SetRenderMode(L.CheckString(1)); err != nil {
						L.RaiseError("render_mode: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LString(cfg.RenderMode))
					return 1
				}
				L.Push(lua.LString(rt.host.RenderMode()))
				return 1
			},
		},
		{
			Signature:   "gopdf.zoom([scale])",
			Description: "Return the current render scale, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("zoom: viewer host unavailable")
					}
					if err := rt.host.SetZoom(float64(L.CheckNumber(1))); err != nil {
						L.RaiseError("zoom: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(lua.LNumber(rt.host.Zoom()))
				return 1
			},
		},
		{
			Signature:   "gopdf.rotation([degrees])",
			Description: "Return clockwise rotation, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("rotation: viewer host unavailable")
					}
					if err := rt.host.SetRotation(float64(L.CheckNumber(1))); err != nil {
						L.RaiseError("rotation: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LNil)
					return 1
				}
				L.Push(lua.LNumber(rt.host.Rotation()))
				return 1
			},
		},
		{
			Signature:   "gopdf.fullscreen([enabled])",
			Description: "Return fullscreen state, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("fullscreen: viewer host unavailable")
					}
					if err := rt.host.SetFullscreen(lua.LVAsBool(L.CheckAny(1))); err != nil {
						L.RaiseError("fullscreen: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LFalse)
					return 1
				}
				L.Push(lua.LBool(rt.host.Fullscreen()))
				return 1
			},
		},
		{
			Signature:   "gopdf.status_bar_visible([visible])",
			Description: "Return status bar visibility, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("status_bar_visible: viewer host unavailable")
					}
					if err := rt.host.SetStatusBarVisible(lua.LVAsBool(L.CheckAny(1))); err != nil {
						L.RaiseError("status_bar_visible: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LBool(cfg.StatusBarVisible))
					return 1
				}
				L.Push(lua.LBool(rt.host.StatusBarVisible()))
				return 1
			},
		},
	})
	options := newLuaOptionsTable(L, rt, cfg)
	L.SetField(mod, "options", options)
	L.SetField(mod, "o", options)
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
		if desc, ok := configOptions[name]; ok {
			L.Push(desc.get(L, cfg))
			return 1
		}
		if value, ok := rt.pluginOption(name); ok {
			L.Push(value.value)
			return 1
		}
		L.RaiseError("options.%s: unknown setting", name)
		return 0
	}))
	L.SetMetatable(tbl, mt)
	return tbl
}

func newLuaCacheTable(L *lua.LState, rt *Runtime) *lua.LTable {
	tbl := L.NewTable()
	registerLuaFunctions(L, tbl, "gopdf.cache.", []luaFunctionSpec{
		{
			Signature:   "gopdf.cache.entries()",
			Description: "Return the number of cached rendered pages.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LNumber(0))
					return 1
				}
				L.Push(lua.LNumber(rt.host.CacheEntries()))
				return 1
			},
		},
		{
			Signature:   "gopdf.cache.pending()",
			Description: "Return the number of pending renders.",
			Function: func(L *lua.LState) int {
				if rt.host == nil {
					L.Push(lua.LNumber(0))
					return 1
				}
				L.Push(lua.LNumber(rt.host.CachePending()))
				return 1
			},
		},
		{
			Signature:   "gopdf.cache.limit([limit])",
			Description: "Return the rendered-page cache limit, or set and return it when supplied.",
			Function: func(L *lua.LState) int {
				if L.GetTop() > 0 {
					if rt.host == nil {
						L.RaiseError("cache.limit: viewer host unavailable")
					}
					if err := rt.host.SetCacheLimit(L.CheckInt(1)); err != nil {
						L.RaiseError("cache.limit: %v", err)
					}
				}
				if rt.host == nil {
					L.Push(lua.LNumber(0))
					return 1
				}
				L.Push(lua.LNumber(rt.host.CacheLimit()))
				return 1
			},
		},
		{
			Signature:   "gopdf.cache.clear()",
			Description: "Clear rendered-page caches.",
			Function: func(L *lua.LState) int {
				if rt.host != nil {
					rt.host.ClearCache()
				}
				return 0
			},
		},
	})
	return tbl
}

func newLuaViewAPI(L *lua.LState, rt *Runtime) *lua.LTable {
	tbl := L.NewTable()
	create := func(L *lua.LState) int {
		spec, ok := L.CheckAny(1).(*lua.LTable)
		if !ok {
			L.RaiseError("ui.create: expected table")
		}
		overlay := uiOverlayFromLuaSpec(L, rt, spec)
		rt.uiSeq++
		overlay.ID = fmt.Sprintf("lua:%s:%d", overlay.ID, rt.uiSeq)
		L.Push(newLuaView(L, rt, overlay))
		return 1
	}
	registerLuaFunctions(L, tbl, "gopdf.ui.", []luaFunctionSpec{
		{
			Signature:   "gopdf.ui.create(spec)",
			Description: "Create a list view using the same UI model as built-in viewer screens.",
			Function:    create},
	})
	return tbl
}

func uiOverlayFromLuaSpec(L *lua.LState, rt *Runtime, spec *lua.LTable) UIOverlay {
	overlay := UIOverlay{
		ID:         lua.LVAsString(spec.RawGetString("id")),
		Title:      lua.LVAsString(spec.RawGetString("title")),
		Rows:       luaTableUIRows(spec.RawGetString("rows")),
		Selected:   1,
		Searchable: true,
		Generation: rt.pluginGeneration,
	}
	if selected := spec.RawGetString("selected"); selected.Type() == lua.LTNumber {
		overlay.Selected = int(lua.LVAsNumber(selected))
	}
	if scroll := spec.RawGetString("scroll"); scroll.Type() == lua.LTNumber {
		overlay.Scroll = int(lua.LVAsNumber(scroll))
	}
	if query := spec.RawGetString("query"); query.Type() == lua.LTString {
		overlay.Query = query.String()
	}
	if searchable := spec.RawGetString("searchable"); searchable.Type() == lua.LTBool {
		overlay.Searchable = lua.LVAsBool(searchable)
	}
	if fn, ok := spec.RawGetString("on_select").(*lua.LFunction); ok {
		overlay.OnSelect = rt.registerCallback(fn)
	}
	if fn, ok := spec.RawGetString("on_close").(*lua.LFunction); ok {
		overlay.OnClose = rt.registerCallback(fn)
	}
	return overlay
}

func newLuaView(L *lua.LState, rt *Runtime, overlay UIOverlay) *lua.LTable {
	view := L.NewTable()
	L.SetField(view, "id", lua.LString(overlay.ID))
	L.SetField(view, "title", lua.LString(overlay.Title))
	L.SetField(view, "show", L.NewFunction(func(L *lua.LState) int {
		if rt.host == nil {
			L.RaiseError("ui.view.show: viewer host unavailable")
		}
		if err := rt.host.ShowUI(overlay); err != nil {
			L.RaiseError("ui.view.show: %v", err)
		}
		return 0
	}))
	L.SetField(view, "close", L.NewFunction(func(L *lua.LState) int {
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			rt.host.CloseUI(overlay.ID)
		}
		return 0
	}))
	L.SetField(view, "set_rows", L.NewFunction(func(L *lua.LState) int {
		overlay.Rows = luaTableUIRows(L.CheckAny(2))
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			rt.host.SetUIRows(overlay.ID, overlay.Rows)
		}
		return 0
	}))
	L.SetField(view, "set_selected", L.NewFunction(func(L *lua.LState) int {
		overlay.Selected = L.CheckInt(2)
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			rt.host.SetUISelected(overlay.ID, overlay.Selected)
		}
		return 0
	}))
	L.SetField(view, "set_scroll", L.NewFunction(func(L *lua.LState) int {
		overlay.Scroll = L.CheckInt(2)
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			rt.host.SetUIScroll(overlay.ID, overlay.Scroll)
		}
		return 0
	}))
	L.SetField(view, "set_query", L.NewFunction(func(L *lua.LState) int {
		overlay.Query = L.CheckString(2)
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			rt.host.SetUIQuery(overlay.ID, overlay.Query)
		}
		return 0
	}))
	L.SetField(view, "visible", L.NewFunction(func(L *lua.LState) int {
		L.Push(lua.LBool(rt.host != nil && rt.host.UIVisible(overlay.ID)))
		return 1
	}))
	L.SetField(view, "selected", L.NewFunction(func(L *lua.LState) int {
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			L.Push(lua.LNumber(rt.host.UISelected(overlay.ID)))
		} else {
			L.Push(lua.LNumber(overlay.Selected))
		}
		return 1
	}))
	L.SetField(view, "scroll", L.NewFunction(func(L *lua.LState) int {
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			L.Push(lua.LNumber(rt.host.UIScroll(overlay.ID)))
		} else {
			L.Push(lua.LNumber(overlay.Scroll))
		}
		return 1
	}))
	L.SetField(view, "query", L.NewFunction(func(L *lua.LState) int {
		if rt.host != nil && rt.host.UIVisible(overlay.ID) {
			L.Push(lua.LString(rt.host.UIQuery(overlay.ID)))
		} else {
			L.Push(lua.LString(overlay.Query))
		}
		return 1
	}))
	return view
}

func registerLuaFunctions(L *lua.LState, table *lua.LTable, prefix string, functions []luaFunctionSpec) {
	registered := make(map[string]lua.LGFunction, len(functions))
	for _, spec := range functions {
		name, ok := strings.CutPrefix(spec.Signature, prefix)
		if !ok {
			panic("Lua function signature has wrong prefix: " + spec.Signature)
		}
		name, _, ok = strings.Cut(name, "(")
		if !ok || name == "" || strings.TrimSpace(spec.Description) == "" {
			panic("invalid Lua function metadata: " + spec.Signature)
		}
		if _, exists := registered[name]; exists {
			panic("duplicate Lua function: " + prefix + name)
		}
		if spec.Function == nil {
			panic("missing Lua function implementation: " + spec.Signature)
		}
		registered[name] = spec.Function
		luaFunctionReferencesMu.Lock()
		luaFunctionReferences[spec.Signature] = LuaReferenceEntry{Signature: spec.Signature, Description: spec.Description}
		luaFunctionReferencesMu.Unlock()
	}
	L.SetFuncs(table, registered)
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

func luaTableUIRows(value lua.LValue) []UIListRow {
	tbl, ok := value.(*lua.LTable)
	if !ok {
		return nil
	}
	rows := make([]UIListRow, 0, tbl.Len())
	for i := 1; i <= tbl.Len(); i++ {
		item := tbl.RawGetInt(i)
		if text, ok := item.(lua.LString); ok {
			rows = append(rows, UIListRow{Text: string(text), Value: string(text)})
			continue
		}
		spec, ok := item.(*lua.LTable)
		if !ok {
			continue
		}
		text := lua.LVAsString(spec.RawGetString("text"))
		value := lua.LVAsString(spec.RawGetString("value"))
		if value == "" {
			value = text
		}
		rows = append(rows, UIListRow{
			Text:      text,
			Value:     value,
			ID:        lua.LVAsString(spec.RawGetString("id")),
			Secondary: lua.LVAsString(spec.RawGetString("secondary")),
			Depth:     int(lua.LVAsNumber(spec.RawGetString("depth"))),
			Disabled:  lua.LVAsBool(spec.RawGetString("disabled")),
		})
	}
	return rows
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
		if err := rt.executeAction(action); err != nil {
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
	if rt.actionExists(action) {
		return action, nil
	}
	if rt.loadingAutogen && isPluginActionName(action) {
		return action, nil
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
	name = normalizeOptionName(name)
	if desc, ok := configOptions[name]; ok {
		if err := desc.apply(&r.cfg, value); err != nil {
			return err
		}
	} else {
		if _, ok := r.pluginOption(name); !ok {
			return fmt.Errorf("unknown setting")
		}
		if err := r.setPluginOptionValue(name, value); err != nil {
			return err
		}
	}
	r.dirty = true
	return nil
}
