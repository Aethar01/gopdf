package config

import (
	"fmt"
	"strings"
	"sync"

	"gopdf/internal/actions"

	lua "github.com/yuin/gopher-lua"
)

type luaFunctionSpec struct {
	Signature string
	Function  lua.LGFunction
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
		{Signature: "gopdf.bind(key, action)", Function: luaBind(rt)},
		{Signature: "gopdf.unbind(key)", Function: luaUnbind(rt)},
		{Signature: "gopdf.bind_mouse(event, action)", Function: luaBindMouse(rt)},
		{Signature: "gopdf.unbind_mouse(event)", Function: luaUnbindMouse(rt)},
		{Signature: "gopdf.message([text])", Function: luaMessage(rt, cfg)},
		{Signature: "gopdf.command(command)", Function: luaCommand(rt)},
		{Signature: "gopdf.open(path)", Function: luaOpen(rt)},
		{Signature: "gopdf.pick_file(callback)", Function: luaPickFile(rt)},
		{Signature: "gopdf.pick_directory(callback)", Function: luaPickDirectory(rt)},
		{Signature: "gopdf.schedule(callback)", Function: luaSchedule(rt)},
		{Signature: "gopdf.log(level, message)", Function: luaLog(rt)},
		{Signature: "gopdf.open_external(uri_or_path)", Function: luaOpenExternal(rt)},
		{Signature: "gopdf.page([page])", Function: luaPage(rt)},
		{Signature: "gopdf.page_count()", Function: luaPageCount(rt)},
		{Signature: "gopdf.goto_document_point(spec)", Function: luaGotoDocumentPoint(rt)},
		{Signature: "gopdf.mode()", Function: luaMode(rt)},
		{Signature: "gopdf.search(query[, backward])", Function: luaSearch(rt)},
		{Signature: "gopdf.search_query()", Function: luaSearchQuery(rt)},
		{Signature: "gopdf.search_match_count()", Function: luaSearchMatchCount(rt)},
		{Signature: "gopdf.search_match_index()", Function: luaSearchMatchIndex(rt)},
		{Signature: "gopdf.current_count()", Function: luaCurrentCount(rt)},
		{Signature: "gopdf.pending_keys()", Function: luaPendingKeys(rt)},
		{Signature: "gopdf.recent_files([limit])", Function: luaRecentFiles(cfg)},
		{Signature: "gopdf.clear_pending_keys()", Function: luaClearPendingKeys(rt)},
		{Signature: "gopdf.fit_mode([mode])", Function: luaFitMode(rt, cfg)},
		{Signature: "gopdf.render_mode([mode])", Function: luaRenderMode(rt, cfg)},
		{Signature: "gopdf.zoom([scale])", Function: luaZoom(rt)},
		{Signature: "gopdf.rotation([degrees])", Function: luaRotation(rt)},
		{Signature: "gopdf.fullscreen([enabled])", Function: luaFullscreen(rt)},
		{Signature: "gopdf.status_bar_visible([visible])", Function: luaStatusBarVisible(rt, cfg)},
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
		{Signature: "gopdf.cache.entries()", Function: luaCacheEntries(rt)},
		{Signature: "gopdf.cache.pending()", Function: luaCachePending(rt)},
		{Signature: "gopdf.cache.limit([limit])", Function: luaCacheLimit(rt)},
		{Signature: "gopdf.cache.clear()", Function: luaCacheClear(rt)},
	})
	return tbl
}

func newLuaViewAPI(L *lua.LState, rt *Runtime) *lua.LTable {
	tbl := L.NewTable()
	registerLuaFunctions(L, tbl, "gopdf.ui.", []luaFunctionSpec{
		{Signature: "gopdf.ui.create(spec)", Function: luaUiCreate(rt)},
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
		if !ok || name == "" {
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
		luaFunctionReferences[spec.Signature] = LuaReferenceEntry{Signature: spec.Signature}
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
