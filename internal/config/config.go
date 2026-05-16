package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopdf/internal/filepicker"
	"gopdf/internal/mupdf"

	lua "github.com/yuin/gopher-lua"
)

type Config struct {
	ConfigPath           string
	AutogenPath          string
	StatusBarVisible     bool
	RenderMode           string
	RenderOversample     float64
	DualPage             bool
	FirstPageOffset      bool
	FitMode              string
	Background           [3]uint8
	PageBackground       [3]uint8
	Foreground           [3]uint8
	StatusBarColor       [3]uint8
	AltBackground        [3]uint8
	AltPageBackground    [3]uint8
	AltForeground        [3]uint8
	AltStatusBarColor    [3]uint8
	HighlightForeground  [3]uint8
	HighlightBackground  [3]uint8
	AltColors            bool
	PageGap              int
	SpreadGap            int
	PageGapVertical      int
	PageGapHorizontal    int
	StatusBarHeight      int
	UIFontSize           int
	UIFontPath           string
	StatusBarLeft        string
	StatusBarRight       string
	SequenceTimeoutMS    int
	NormalMessage        string
	KeyBindings          map[string]string
	MouseBindings        map[string]string
	MouseTextSelect      bool
	NaturalScroll        bool
	AntiAliasing         int
	OutlineInitialDepth  int
	OutlineWidthPercent  int
	OutlineHeightPercent int
	CompletionMaxItems   int
}

type Runtime struct {
	explicitPath string
	docPath      string
	docName      string
	docMeta      documentMeta
	cfg          Config
	state        *lua.LState
	host         Host
	callbacks    map[string]*lua.LFunction
	callbackSeq  int
	dirty        bool
}

type UIOverlay struct {
	Title    string
	Rows     []string
	Selected int
	OnSelect string
	OnClose  string
}

type documentMeta struct {
	exists    bool
	sizeBytes int64
	ext       string
	pageCount int
	hasPages  bool
}

type Host interface {
	ExecuteAction(action string) error
	Open(path string) error
	ShowUI(overlay UIOverlay) error
	CloseUI()
	UIVisible() bool
	SetUIRows(rows []string)
	SetUISelected(selected int)
	Page() int
	PageCount() int
	GotoPage(page int) error
	Message() string
	SetMessage(message string)
	RunCommand(command string) error
	Mode() string
	Search(query string, backward bool) error
	SearchQuery() string
	SearchMatchCount() int
	SearchMatchIndex() int
	CurrentCount() string
	PendingKeys() []string
	ClearPendingKeys()
	FitMode() string
	SetFitMode(mode string) error
	RenderMode() string
	SetRenderMode(mode string) error
	Zoom() float64
	SetZoom(zoom float64) error
	Rotation() float64
	SetRotation(rotation float64) error
	Fullscreen() bool
	SetFullscreen(fullscreen bool) error
	StatusBarVisible() bool
	SetStatusBarVisible(visible bool) error
	CacheEntries() int
	CachePending() int
	CacheLimit() int
	SetCacheLimit(limit int) error
	ClearCache()
}

func Default() Config {
	return Config{
		StatusBarVisible:    true,
		RenderMode:          "continuous",
		RenderOversample:    1,
		DualPage:            false,
		FirstPageOffset:     true,
		FitMode:             "page",
		Background:          [3]uint8{220, 220, 220},
		PageBackground:      [3]uint8{255, 255, 255},
		Foreground:          [3]uint8{20, 20, 20},
		StatusBarColor:      [3]uint8{220, 220, 220},
		AltBackground:       [3]uint8{20, 20, 20},
		AltPageBackground:   [3]uint8{17, 17, 17},
		AltForeground:       [3]uint8{255, 255, 255},
		AltStatusBarColor:   [3]uint8{20, 20, 20},
		HighlightForeground: [3]uint8{0, 0, 0},
		HighlightBackground: [3]uint8{255, 224, 102},
		AltColors:           false,
		PageGap:             0,
		SpreadGap:           0,
		PageGapVertical:     0,
		PageGapHorizontal:   0,
		StatusBarHeight:     28,
		UIFontSize:          14,
		UIFontPath:          "",
		StatusBarLeft:       "{message}",
		StatusBarRight:      "{page}/{total} {mode} fit={fit} rot={rot} {zoom}",
		SequenceTimeoutMS:   700,
		NormalMessage:       "",
		KeyBindings:         defaultBindings(),
		MouseBindings: map[string]string{
			"wheel_up":       "scroll_up",
			"wheel_down":     "scroll_down",
			"wheel_left":     "scroll_left",
			"wheel_right":    "scroll_right",
			"<c-wheel_up>":   "zoom_in",
			"<c-wheel_down>": "zoom_out",
			"middle_down":    "pan",
		},
		MouseTextSelect:      true,
		NaturalScroll:        false,
		AntiAliasing:         8,
		OutlineInitialDepth:  1,
		OutlineWidthPercent:  70,
		OutlineHeightPercent: 80,
		CompletionMaxItems:   10,
	}
}

func Load(explicitPath string) (Config, error) {
	rt, err := Open(explicitPath, "")
	if err != nil {
		return Config{}, err
	}
	defer rt.Close()
	return rt.Config(), nil
}

func Open(explicitPath, docPath string) (*Runtime, error) {
	docPath = absoluteDocumentPath(docPath)
	docName := ""
	if docPath != "" {
		docName = filepath.Base(docPath)
	}
	rt := &Runtime{
		explicitPath: explicitPath,
		docPath:      docPath,
		docName:      docName,
		docMeta:      loadDocumentMeta(docPath),
	}
	if err := rt.Reload(); err != nil {
		return nil, err
	}
	return rt, nil
}

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
	path = absoluteDocumentPath(path)
	dir := filepath.Dir(statePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(statePath, []byte(path), 0644)
}

func absoluteDocumentPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	if abs, err := filepath.Abs(path); err == nil {
		return abs
	}
	return path
}

func candidatePaths(explicitPath string) []string {
	if explicitPath != "" {
		return []string{explicitPath}
	}
	return unique(platformConfigPaths())
}

func unique(paths []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(paths))
	for _, path := range paths {
		if _, ok := seen[path]; ok {
			continue
		}
		seen[path] = struct{}{}
		out = append(out, path)
	}
	return out
}

func (r *Runtime) Close() {
	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
}

func (r *Runtime) Config() Config {
	return r.cfg
}

func (r *Runtime) AttachHost(host Host) {
	r.host = host
}

func (r *Runtime) SetDocument(path string) error {
	path = absoluteDocumentPath(path)
	r.docPath = path
	r.docName = ""
	if path != "" {
		r.docName = filepath.Base(path)
	}
	r.docMeta = loadDocumentMeta(path)
	return r.Reload()
}

func (r *Runtime) Reload() error {
	if r.state != nil {
		r.state.Close()
		r.state = nil
	}
	r.cfg = Default()
	r.callbacks = map[string]*lua.LFunction{}
	r.callbackSeq = 0
	r.dirty = false
	autogenPath := r.autogenPath()
	if autogenPath != "" {
		if info, err := os.Stat(autogenPath); err == nil && !info.IsDir() {
			if err := r.applyLuaConfig(autogenPath); err != nil {
				r.Close()
				return err
			}
			r.cfg.AutogenPath = autogenPath
		} else if err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	paths := candidatePaths(r.explicitPath)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if info.IsDir() {
			continue
		}
		if err := r.applyLuaConfig(path); err != nil {
			r.Close()
			return err
		}
		r.cfg.ConfigPath = path
		r.cfg.AutogenPath = autogenPath
		r.dirty = false
		return nil
	}
	r.cfg.AutogenPath = autogenPath
	r.dirty = false
	return nil
}

func (r *Runtime) autogenPath() string {
	if r.explicitPath != "" {
		return filepath.Join(filepath.Dir(r.explicitPath), "autogen.lua")
	}
	return platformAutogenPath()
}

func (r *Runtime) SetKeyBinding(key, action string) error {
	if !isBuiltinAction(action) {
		return fmt.Errorf("cannot persist non-builtin action %q", action)
	}
	r.setKeyBinding(key, action)
	return r.WriteAutogen()
}

func (r *Runtime) RebindKey(oldKey, newKey, action string) error {
	if !isBuiltinAction(action) {
		return fmt.Errorf("cannot persist non-builtin action %q", action)
	}
	if oldKey != "" && oldKey != newKey {
		r.unbindKey(oldKey)
	}
	r.setKeyBinding(newKey, action)
	return r.WriteAutogen()
}

func (r *Runtime) UnbindKey(key string) error {
	r.unbindKey(key)
	return r.WriteAutogen()
}

func (r *Runtime) WriteAutogen() error {
	path := r.autogenPath()
	if path == "" {
		return fmt.Errorf("autogen path unavailable")
	}
	data := generatedKeybindLua(r.cfg.KeyBindings)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(data), 0644); err != nil {
		return err
	}
	r.cfg.AutogenPath = path
	return nil
}

func generatedKeybindLua(bindings map[string]string) string {
	defaults := defaultBindings()
	keys := make([]string, 0, len(defaults)+len(bindings))
	seen := map[string]struct{}{}
	for key := range defaults {
		keys = append(keys, key)
		seen[key] = struct{}{}
	}
	for key := range bindings {
		if _, ok := seen[key]; !ok {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("-- This file is generated by gopdf. Manual edits may be overwritten.\n")
	for _, key := range keys {
		current, ok := bindings[key]
		def, isDefault := defaults[key]
		if !ok {
			if isDefault {
				fmt.Fprintf(&b, "gopdf.unbind(%q)\n", key)
			}
			continue
		}
		if isDefault && current == def {
			continue
		}
		if !isBuiltinAction(current) {
			continue
		}
		fmt.Fprintf(&b, "gopdf.bind(%q, gopdf.%s)\n", key, current)
	}
	return b.String()
}

func isBuiltinAction(action string) bool {
	for _, candidate := range allActions() {
		if action == candidate {
			return true
		}
	}
	return false
}

func Actions() []string {
	actions := allActions()
	return append([]string(nil), actions...)
}

func (r *Runtime) RunAction(action string) (bool, bool, error) {
	if r == nil {
		return false, false, nil
	}
	fn, ok := r.callbacks[action]
	if !ok {
		return false, false, nil
	}
	r.dirty = false
	if err := r.state.CallByParam(lua.P{Fn: fn, NRet: 0, Protect: true}); err != nil {
		return true, r.dirty, err
	}
	return true, r.dirty, nil
}

func (r *Runtime) Eval(code string) error {
	if r == nil || r.state == nil {
		return fmt.Errorf("no Lua state")
	}
	return r.state.DoString(code)
}

func (r *Runtime) RunUISelect(callback string, index int, value string) error {
	return r.runCallback(callback, lua.LNumber(index), lua.LString(value))
}

func (r *Runtime) RunUIClose(callback string) error {
	return r.runCallback(callback)
}

func (r *Runtime) runCallback(callback string, args ...lua.LValue) error {
	if r == nil || callback == "" {
		return nil
	}
	fn, ok := r.callbacks[callback]
	if !ok {
		return fmt.Errorf("unknown lua callback: %s", callback)
	}
	params := lua.P{Fn: fn, NRet: 0, Protect: true}
	return r.state.CallByParam(params, args...)
}

func (r *Runtime) applyLuaConfig(path string) error {
	L := lua.NewState()
	r.state = L
	mod := newLuaModule(L, r, &r.cfg)
	L.SetGlobal("gopdf", mod)
	L.SetGlobal("bind", L.GetField(mod, "bind"))
	L.SetGlobal("unbind", L.GetField(mod, "unbind"))
	L.SetGlobal("bind_mouse", L.GetField(mod, "bind_mouse"))
	L.SetGlobal("unbind_mouse", L.GetField(mod, "unbind_mouse"))
	L.SetGlobal("options", L.GetField(mod, "options"))
	if err := L.DoFile(path); err != nil {
		L.Close()
		r.state = nil
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
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
			if rt.host == nil {
				L.RaiseError("open: viewer host unavailable")
			}
			if err := rt.host.Open(L.CheckString(1)); err != nil {
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
	for _, action := range allActions() {
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
		"menu": show,
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
	for _, candidate := range allActions() {
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

func loadDocumentMeta(docPath string) documentMeta {
	meta := documentMeta{ext: strings.ToLower(filepath.Ext(docPath))}
	if docPath == "" {
		return meta
	}
	info, err := os.Stat(docPath)
	if err == nil && !info.IsDir() {
		meta.exists = true
		meta.sizeBytes = info.Size()
	}
	doc, err := mupdf.Open(docPath)
	if err != nil {
		return meta
	}
	defer doc.Close()
	pages, err := doc.PageCount()
	if err == nil {
		meta.pageCount = pages
		meta.hasPages = true
	}
	return meta
}

func luaSettingValue(L *lua.LState, name string, cfg *Config) (lua.LValue, error) {
	switch name {
	case "status_bar_visible":
		return lua.LBool(cfg.StatusBarVisible), nil
	case "mouse_text_select":
		return lua.LBool(cfg.MouseTextSelect), nil
	case "natural_scroll":
		return lua.LBool(cfg.NaturalScroll), nil
	case "anti_aliasing":
		return lua.LNumber(cfg.AntiAliasing), nil
	case "outline_initial_depth":
		return lua.LNumber(cfg.OutlineInitialDepth), nil
	case "outline_width_percent":
		return lua.LNumber(cfg.OutlineWidthPercent), nil
	case "outline_height_percent":
		return lua.LNumber(cfg.OutlineHeightPercent), nil
	case "completion_max_items":
		return lua.LNumber(cfg.CompletionMaxItems), nil
	case "alt_colors":
		return lua.LBool(cfg.AltColors), nil
	case "render_mode":
		return lua.LString(cfg.RenderMode), nil
	case "render_oversample":
		return lua.LNumber(cfg.RenderOversample), nil
	case "dual_page":
		return lua.LBool(cfg.DualPage), nil
	case "first_page_offset":
		return lua.LBool(cfg.FirstPageOffset), nil
	case "fit_mode":
		return lua.LString(cfg.FitMode), nil
	case "page_gap":
		return lua.LNumber(cfg.PageGap), nil
	case "spread_gap":
		return lua.LNumber(cfg.SpreadGap), nil
	case "page_gap_vertical":
		return lua.LNumber(cfg.PageGapVertical), nil
	case "page_gap_horizontal":
		return lua.LNumber(cfg.PageGapHorizontal), nil
	case "status_bar_height":
		return lua.LNumber(cfg.StatusBarHeight), nil
	case "ui_font_size":
		return lua.LNumber(cfg.UIFontSize), nil
	case "ui_font_path":
		return lua.LString(cfg.UIFontPath), nil
	case "status_bar_left":
		return lua.LString(cfg.StatusBarLeft), nil
	case "status_bar_right":
		return lua.LString(cfg.StatusBarRight), nil
	case "sequence_timeout_ms":
		return lua.LNumber(cfg.SequenceTimeoutMS), nil
	case "background":
		tbl := L.NewTable()
		for i, c := range cfg.Background {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "page_background":
		tbl := L.NewTable()
		for i, c := range cfg.PageBackground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "foreground":
		tbl := L.NewTable()
		for i, c := range cfg.Foreground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "status_bar_color":
		tbl := L.NewTable()
		for i, c := range cfg.StatusBarColor {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "alt_background":
		tbl := L.NewTable()
		for i, c := range cfg.AltBackground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "alt_page_background":
		tbl := L.NewTable()
		for i, c := range cfg.AltPageBackground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "alt_foreground":
		tbl := L.NewTable()
		for i, c := range cfg.AltForeground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "alt_status_bar_color":
		tbl := L.NewTable()
		for i, c := range cfg.AltStatusBarColor {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "highlight_foreground":
		tbl := L.NewTable()
		for i, c := range cfg.HighlightForeground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	case "highlight_background":
		tbl := L.NewTable()
		for i, c := range cfg.HighlightBackground {
			tbl.RawSetInt(i+1, lua.LNumber(c))
		}
		return tbl, nil
	default:
		return lua.LNil, fmt.Errorf("unknown setting")
	}
}

func applyLuaSetting(name string, value lua.LValue, cfg *Config) error {
	switch name {
	case "status_bar_visible":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.StatusBarVisible = lua.LVAsBool(value)
	case "mouse_text_select":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.MouseTextSelect = lua.LVAsBool(value)
	case "natural_scroll":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.NaturalScroll = lua.LVAsBool(value)
	case "anti_aliasing":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.AntiAliasing = int(lua.LVAsNumber(value))
	case "outline_initial_depth":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.OutlineInitialDepth = int(lua.LVAsNumber(value))
	case "outline_width_percent":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.OutlineWidthPercent = int(lua.LVAsNumber(value))
	case "outline_height_percent":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.OutlineHeightPercent = int(lua.LVAsNumber(value))
	case "completion_max_items":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.CompletionMaxItems = max(1, int(lua.LVAsNumber(value)))
	case "alt_colors":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.AltColors = lua.LVAsBool(value)
	case "render_mode":
		if value.Type() != lua.LTString {
			return fmt.Errorf("expected string")
		}
		cfg.RenderMode = normalizeRenderMode(value.String())
	case "render_oversample":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.RenderOversample = float64(lua.LVAsNumber(value))
	case "dual_page":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.DualPage = lua.LVAsBool(value)
	case "first_page_offset":
		if value.Type() != lua.LTBool {
			return fmt.Errorf("expected boolean")
		}
		cfg.FirstPageOffset = lua.LVAsBool(value)
	case "fit_mode":
		if value.Type() != lua.LTString {
			return fmt.Errorf("expected string")
		}
		cfg.FitMode = normalizeFitMode(value.String())
	case "page_gap":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.PageGap = int(lua.LVAsNumber(value))
		cfg.PageGapVertical = cfg.PageGap
	case "spread_gap":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.SpreadGap = int(lua.LVAsNumber(value))
		cfg.PageGapHorizontal = cfg.SpreadGap
	case "page_gap_vertical":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.PageGapVertical = int(lua.LVAsNumber(value))
		cfg.PageGap = cfg.PageGapVertical
	case "page_gap_horizontal":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.PageGapHorizontal = int(lua.LVAsNumber(value))
		cfg.SpreadGap = cfg.PageGapHorizontal
	case "status_bar_height":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.StatusBarHeight = int(lua.LVAsNumber(value))
	case "ui_font_size":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.UIFontSize = int(lua.LVAsNumber(value))
	case "ui_font_path":
		if value.Type() != lua.LTString {
			return fmt.Errorf("expected string")
		}
		cfg.UIFontPath = lua.LVAsString(value)
	case "status_bar_left":
		if value.Type() != lua.LTString {
			return fmt.Errorf("expected string")
		}
		cfg.StatusBarLeft = lua.LVAsString(value)
	case "status_bar_right":
		if value.Type() != lua.LTString {
			return fmt.Errorf("expected string")
		}
		cfg.StatusBarRight = lua.LVAsString(value)
	case "sequence_timeout_ms":
		if value.Type() != lua.LTNumber {
			return fmt.Errorf("expected number")
		}
		cfg.SequenceTimeoutMS = int(lua.LVAsNumber(value))
	case "background":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.Background = readColor(tbl, cfg.Background)
	case "page_background":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.PageBackground = readColor(tbl, cfg.PageBackground)
	case "foreground":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.Foreground = readColor(tbl, cfg.Foreground)
	case "status_bar_color":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.StatusBarColor = readColor(tbl, cfg.StatusBarColor)
	case "alt_background":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.AltBackground = readColor(tbl, cfg.AltBackground)
	case "alt_page_background":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.AltPageBackground = readColor(tbl, cfg.AltPageBackground)
	case "alt_foreground":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.AltForeground = readColor(tbl, cfg.AltForeground)
	case "alt_status_bar_color":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.AltStatusBarColor = readColor(tbl, cfg.AltStatusBarColor)
	case "highlight_foreground":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.HighlightForeground = readColor(tbl, cfg.HighlightForeground)
	case "highlight_background":
		tbl, ok := value.(*lua.LTable)
		if !ok {
			return fmt.Errorf("expected table")
		}
		cfg.HighlightBackground = readColor(tbl, cfg.HighlightBackground)
	default:
		return fmt.Errorf("unknown setting")
	}
	return nil
}

func readColor(tbl *lua.LTable, fallback [3]uint8) [3]uint8 {
	out := fallback
	for i := 1; i <= 3; i++ {
		if v := tbl.RawGetInt(i); v.Type() == lua.LTNumber {
			n := int(lua.LVAsNumber(v))
			if n < 0 {
				n = 0
			}
			if n > 255 {
				n = 255
			}
			out[i-1] = uint8(n)
		}
	}
	return out
}

func normalizeFitMode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "width" || s == "manual" {
		return s
	}
	return "page"
}

func normalizeRenderMode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "single" {
		return s
	}
	return "continuous"
}

func normalizeMouseEvent(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if strings.HasPrefix(s, "<c-") && strings.HasSuffix(s, ">") {
		return s
	}
	if after, ok := strings.CutPrefix(s, "ctrl_"); ok {
		return "<c-" + after + ">"
	}
	return s
}

func defaultBindings() map[string]string {
	return map[string]string{
		"j":       "scroll_down",
		"<Down>":  "scroll_down",
		"k":       "scroll_up",
		"<Up>":    "scroll_up",
		"h":       "scroll_left",
		"<Left>":  "scroll_left",
		"l":       "scroll_right",
		"<Right>": "scroll_right",
		"J":       "next_page",
		"K":       "prev_page",
		" ":       "next_page",
		"<PgDn>":  "next_page",
		"<PgUp>":  "prev_page",
		"gg":      "first_page",
		"G":       "last_page",
		":":       "command_mode",
		"/":       "search_prompt",
		"?":       "search_prompt_backward",
		"n":       "search_next",
		"N":       "search_prev",
		"d":       "toggle_dual_page",
		"m":       "toggle_render_mode",
		"<C-r>":   "toggle_alt_colors",
		"co":      "toggle_first_page_offset",
		"<C-n>":   "toggle_status_bar",
		"f":       "toggle_fullscreen",
		"o":       "outline",
		"<CR>":    "confirm",
		"<Tab>":   "show_completion",
		"<S-Tab>": "prev_completion",
		"+":       "zoom_in",
		"=":       "zoom_in",
		"-":       "zoom_out",
		"0":       "reset_zoom",
		"w":       "fit_width",
		"z":       "fit_page",
		"r":       "rotate_cw",
		"R":       "rotate_ccw",
		"<C-g>":   "goto_page_prompt",
		"q":       "quit",
		"<Esc>":   "close",
		"<C-i>":   "jump_forward",
		"<C-o>":   "jump_backward",
		"<C-S-o>": "open_file_picker",
		"<F1>":    "keybinds",
		"<C-S-r>": "reload_config",
	}
}

func allActions() []string {
	return []string{
		"next_page",
		"prev_page",
		"scroll_down",
		"scroll_up",
		"scroll_left",
		"scroll_right",
		"next_spread",
		"prev_spread",
		"first_page",
		"last_page",
		"command_mode",
		"search_prompt",
		"search_prompt_backward",
		"search_next",
		"search_prev",
		"toggle_dual_page",
		"toggle_render_mode",
		"toggle_alt_colors",
		"toggle_first_page_offset",
		"toggle_status_bar",
		"toggle_fullscreen",
		"outline",
		"confirm",
		"zoom_in",
		"zoom_out",
		"reset_zoom",
		"fit_width",
		"fit_page",
		"reload_config",
		"rotate_cw",
		"rotate_ccw",
		"goto_page_prompt",
		"clear_search",
		"show_completion",
		"next_completion",
		"prev_completion",
		"close",
		"jump_forward",
		"jump_backward",
		"open_file_picker",
		"keybinds",
		"pan",
		"quit",
	}
}
