package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type Config struct {
	ConfigPath          string
	StatusBarVisible    bool
	RenderMode          string
	DualPage            bool
	FirstPageOffset     bool
	FitMode             string
	Background          [3]uint8
	Foreground          [3]uint8
	StatusBarColor      [3]uint8
	AltBackground       [3]uint8
	AltForeground       [3]uint8
	AltStatusBarColor   [3]uint8
	HighlightForeground [3]uint8
	HighlightBackground [3]uint8
	AltColors           bool
	PageGap             int
	SpreadGap           int
	PageGapVertical     int
	PageGapHorizontal   int
	StatusBarHeight     int
	SequenceTimeoutMS   int
	NormalMessage       string
	KeyBindings         map[string]string
	MouseBindings       map[string]string
	MouseTextSelect     bool
}

func Default() Config {
	return Config{
		StatusBarVisible:    true,
		RenderMode:          "continuous",
		DualPage:            true,
		FirstPageOffset:     true,
		FitMode:             "page",
		Background:          [3]uint8{0xff, 0xff, 0xff},
		Foreground:          [3]uint8{0x11, 0x11, 0x11},
		StatusBarColor:      [3]uint8{0x11, 0x11, 0x11},
		AltBackground:       [3]uint8{0x11, 0x11, 0x11},
		AltForeground:       [3]uint8{0xff, 0xff, 0xff},
		AltStatusBarColor:   [3]uint8{0x11, 0x11, 0x11},
		HighlightForeground: [3]uint8{0x00, 0x00, 0x00},
		HighlightBackground: [3]uint8{0xff, 0xe0, 0x66},
		AltColors:           false,
		PageGap:             0,
		SpreadGap:           0,
		PageGapVertical:     0,
		PageGapHorizontal:   0,
		StatusBarHeight:     28,
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
		},
		MouseTextSelect: true,
	}
}

func Load(explicitPath string) (Config, error) {
	cfg := Default()
	paths := candidatePaths(explicitPath)
	for _, path := range paths {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return cfg, err
		}
		if info.IsDir() {
			continue
		}
		if err := applyLuaConfig(path, &cfg); err != nil {
			return cfg, err
		}
		cfg.ConfigPath = path
		break
	}
	return cfg, nil
}

func candidatePaths(explicitPath string) []string {
	if explicitPath != "" {
		return []string{explicitPath}
	}
	paths := make([]string, 0, 6)
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".config", "gopdf", "config.lua"))
	}
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		paths = append(paths, filepath.Join(xdg, "gopdf", "config.lua"))
	}
	for _, dir := range strings.Split(os.Getenv("XDG_CONFIG_DIRS"), ":") {
		if dir == "" {
			continue
		}
		paths = append(paths, filepath.Join(dir, "gopdf", "config.lua"))
	}
	paths = append(paths, filepath.Join("/etc/xdg", "gopdf", "config.lua"))
	return unique(paths)
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

func applyLuaConfig(path string, cfg *Config) error {
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("gopdf", newLuaModule(L, cfg))
	if err := L.DoFile(path); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

func newLuaModule(L *lua.LState, cfg *Config) *lua.LTable {
	mod := L.NewTable()
	L.SetFuncs(mod, map[string]lua.LGFunction{
		"bind": func(L *lua.LState) int {
			key := L.CheckString(1)
			action := L.CheckAny(2)
			actionName, err := luaActionName(action)
			if err != nil {
				L.RaiseError("bind %q: %v", key, err)
			}
			cfg.KeyBindings[key] = actionName
			return 0
		},
		"unbind": func(L *lua.LState) int {
			key := L.CheckString(1)
			delete(cfg.KeyBindings, key)
			return 0
		},
		"bind_mouse": func(L *lua.LState) int {
			event := normalizeMouseEvent(L.CheckString(1))
			action := L.CheckAny(2)
			actionName, err := luaActionName(action)
			if err != nil {
				L.RaiseError("bind_mouse %q: %v", event, err)
			}
			cfg.MouseBindings[event] = actionName
			return 0
		},
		"unbind_mouse": func(L *lua.LState) int {
			event := normalizeMouseEvent(L.CheckString(1))
			delete(cfg.MouseBindings, event)
			return 0
		},
		"set": func(L *lua.LState) int {
			name := strings.ToLower(strings.TrimSpace(L.CheckString(1)))
			value := L.CheckAny(2)
			if err := applyLuaSetting(name, value, cfg); err != nil {
				L.RaiseError("set %s: %v", name, err)
			}
			return 0
		},
	})
	L.SetField(mod, "options", newLuaOptionsTable(L, cfg))
	for _, action := range allActions() {
		name := action
		L.SetField(mod, name, L.NewFunction(func(L *lua.LState) int {
			L.Push(lua.LString(name))
			return 1
		}))
	}
	return mod
}

func newLuaOptionsTable(L *lua.LState, cfg *Config) *lua.LTable {
	tbl := L.NewTable()
	mt := L.NewTable()
	L.SetField(mt, "__newindex", L.NewFunction(func(L *lua.LState) int {
		name := strings.ToLower(strings.TrimSpace(L.CheckString(2)))
		value := L.CheckAny(3)
		if err := applyLuaSetting(name, value, cfg); err != nil {
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

func luaActionName(value lua.LValue) (string, error) {
	if value.Type() != lua.LTString {
		return "", fmt.Errorf("expected action string")
	}
	action := value.String()
	for _, candidate := range allActions() {
		if action == candidate {
			return action, nil
		}
	}
	return "", fmt.Errorf("unknown action %q", action)
}

func luaSettingValue(L *lua.LState, name string, cfg *Config) (lua.LValue, error) {
	switch name {
	case "status_bar_visible":
		return lua.LBool(cfg.StatusBarVisible), nil
	case "mouse_text_select":
		return lua.LBool(cfg.MouseTextSelect), nil
	case "alt_colors":
		return lua.LBool(cfg.AltColors), nil
	case "render_mode":
		return lua.LString(cfg.RenderMode), nil
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
	case "sequence_timeout_ms":
		return lua.LNumber(cfg.SequenceTimeoutMS), nil
	case "background":
		tbl := L.NewTable()
		for i, c := range cfg.Background {
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
	if strings.HasPrefix(s, "ctrl_") {
		return "<c-" + strings.TrimPrefix(s, "ctrl_") + ">"
	}
	return s
}

func defaultBindings() map[string]string {
	return map[string]string{
		"j":      "scroll_down",
		"k":      "scroll_up",
		"h":      "scroll_left",
		"l":      "scroll_right",
		"J":      "next_page",
		"K":      "prev_page",
		" ":      "next_page",
		"<PgDn>": "next_page",
		"<PgUp>": "prev_page",
		"gg":     "first_page",
		"G":      "last_page",
		":":      "command_mode",
		"/":      "search_prompt",
		"?":      "search_prompt_backward",
		"n":      "search_next",
		"N":      "search_prev",
		"d":      "toggle_dual_page",
		"m":      "toggle_render_mode",
		"tb":     "toggle_alt_colors",
		"co":     "toggle_first_page_offset",
		"s":      "toggle_status_bar",
		"f":      "toggle_fullscreen",
		"+":      "zoom_in",
		"=":      "zoom_in",
		"-":      "zoom_out",
		"0":      "reset_zoom",
		"w":      "fit_width",
		"z":      "fit_page",
		"r":      "rotate_cw",
		"R":      "rotate_ccw",
		"g":      "goto_page_prompt",
		"q":      "quit",
		"<Esc>":  "escape",
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
		"zoom_in",
		"zoom_out",
		"reset_zoom",
		"fit_width",
		"fit_page",
		"reload_config",
		"rotate_cw",
		"rotate_ccw",
		"goto_page_prompt",
		"quit",
		"escape",
	}
}
