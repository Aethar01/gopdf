package config

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type optionDesc struct {
	kind        string
	description string
	get         func(L *lua.LState, cfg *Config) lua.LValue
	apply       func(cfg *Config, value lua.LValue) error
	format      func(*Config) string
	applyText   func(*Config, string) error
}

func boolOption(description string, get func(*Config) bool, set func(*Config, bool)) optionDesc {
	return optionDesc{
		kind:        "boolean",
		description: description,
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LBool(get(cfg)) },
		format:      func(cfg *Config) string { return strconv.FormatBool(get(cfg)) },
		applyText: func(cfg *Config, raw string) error {
			value, err := parseBoolOption(raw)
			if err != nil {
				return err
			}
			set(cfg, value)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTBool {
				return fmt.Errorf("expected boolean")
			}
			set(cfg, lua.LVAsBool(value))
			return nil
		},
	}
}

func intOption(description string, get func(*Config) int, set func(*Config, int)) optionDesc {
	return optionDesc{
		kind:        "integer",
		description: description,
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(get(cfg)) },
		format:      func(cfg *Config) string { return strconv.Itoa(get(cfg)) },
		applyText: func(cfg *Config, raw string) error {
			value, err := strconv.Atoi(strings.TrimSpace(raw))
			if err != nil {
				return fmt.Errorf("expected integer")
			}
			set(cfg, value)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTNumber {
				return fmt.Errorf("expected number")
			}
			set(cfg, int(lua.LVAsNumber(value)))
			return nil
		},
	}
}

func floatOption(description string, get func(*Config) float64, set func(*Config, float64)) optionDesc {
	return optionDesc{
		kind:        "number",
		description: description,
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(get(cfg)) },
		format:      func(cfg *Config) string { return strconv.FormatFloat(get(cfg), 'g', -1, 64) },
		applyText: func(cfg *Config, raw string) error {
			value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
			if err != nil {
				return fmt.Errorf("expected number")
			}
			set(cfg, value)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTNumber {
				return fmt.Errorf("expected number")
			}
			set(cfg, float64(lua.LVAsNumber(value)))
			return nil
		},
	}
}

func stringOption(description string, get func(*Config) string, set func(*Config, string)) optionDesc {
	return optionDesc{
		kind:        "string",
		description: description,
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LString(get(cfg)) },
		format:      func(cfg *Config) string { return strconv.Quote(get(cfg)) },
		applyText: func(cfg *Config, raw string) error {
			value, err := parseStringOption(raw)
			if err != nil {
				return err
			}
			set(cfg, value)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTString {
				return fmt.Errorf("expected string")
			}
			set(cfg, value.String())
			return nil
		},
	}
}

func colorOption(description string, get func(*Config) [3]uint8, set func(*Config, [3]uint8)) optionDesc {
	return optionDesc{
		kind:        "color",
		description: description,
		get: func(L *lua.LState, cfg *Config) lua.LValue {
			tbl := L.NewTable()
			c := get(cfg)
			for i := range 3 {
				tbl.RawSetInt(i+1, lua.LNumber(c[i]))
			}
			return tbl
		},
		format: func(cfg *Config) string {
			color := get(cfg)
			return fmt.Sprintf("%d,%d,%d", color[0], color[1], color[2])
		},
		applyText: func(cfg *Config, raw string) error {
			color, err := parseColorOption(raw)
			if err != nil {
				return err
			}
			set(cfg, color)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			tbl, ok := value.(*lua.LTable)
			if !ok {
				return fmt.Errorf("expected table")
			}
			set(cfg, readColor(tbl, get(cfg)))
			return nil
		},
	}
}

func OptionNames() []string {
	names := make([]string, 0, len(configOptions))
	for name := range configOptions {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func (r *Runtime) OptionValue(name string) (string, error) {
	desc, ok := configOptions[normalizeOptionName(name)]
	if !ok {
		return "", fmt.Errorf("unknown option: %s", name)
	}
	return desc.format(&r.cfg), nil
}

func (r *Runtime) SetOption(name, value string) error {
	name = normalizeOptionName(name)
	desc, ok := configOptions[name]
	if !ok {
		return fmt.Errorf("unknown option: %s", name)
	}
	if err := desc.applyText(&r.cfg, value); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	r.dirty = true
	return nil
}

func (r *Runtime) ToggleOption(name string) error {
	name = normalizeOptionName(name)
	desc, ok := configOptions[name]
	if !ok {
		return fmt.Errorf("unknown option: %s", name)
	}
	if desc.kind != "boolean" {
		return fmt.Errorf("%s: expected boolean option", name)
	}
	value := desc.get(r.state, &r.cfg)
	return r.SetOption(name, strconv.FormatBool(!lua.LVAsBool(value)))
}

func normalizeOptionName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func parseBoolOption(raw string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "true", "on", "yes", "1":
		return true, nil
	case "false", "off", "no", "0":
		return false, nil
	default:
		return false, fmt.Errorf("expected boolean")
	}
}

func parseStringOption(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", nil
	}
	if raw[0] == '\'' || raw[0] == '"' {
		if raw[0] == '\'' {
			if len(raw) < 2 || raw[len(raw)-1] != '\'' {
				return "", fmt.Errorf("unterminated string")
			}
			return raw[1 : len(raw)-1], nil
		}
		value, err := strconv.Unquote(raw)
		if err != nil {
			return "", fmt.Errorf("invalid quoted string")
		}
		return value, nil
	}
	return raw, nil
}

func parseColorOption(raw string) ([3]uint8, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "#") {
		if len(raw) != 7 {
			return [3]uint8{}, fmt.Errorf("expected #RRGGBB or r,g,b")
		}
		value, err := strconv.ParseUint(raw[1:], 16, 24)
		if err != nil {
			return [3]uint8{}, fmt.Errorf("expected #RRGGBB or r,g,b")
		}
		return [3]uint8{uint8(value >> 16), uint8(value >> 8), uint8(value)}, nil
	}
	parts := strings.Split(raw, ",")
	if len(parts) != 3 {
		return [3]uint8{}, fmt.Errorf("expected #RRGGBB or r,g,b")
	}
	var color [3]uint8
	for i, part := range parts {
		value, err := strconv.Atoi(strings.TrimSpace(part))
		if err != nil || value < 0 || value > 255 {
			return [3]uint8{}, fmt.Errorf("color channels must be between 0 and 255")
		}
		color[i] = uint8(value)
	}
	return color, nil
}

var configOptions = map[string]optionDesc{
	"status_bar_visible":     boolOption("Show the status bar at startup.", func(c *Config) bool { return c.StatusBarVisible }, func(c *Config, v bool) { c.StatusBarVisible = v }),
	"mouse_text_select":      boolOption("Enable text selection with the left mouse button.", func(c *Config) bool { return c.MouseTextSelect }, func(c *Config, v bool) { c.MouseTextSelect = v }),
	"natural_scroll":         boolOption("Reverse mouse-wheel scrolling direction.", func(c *Config) bool { return c.NaturalScroll }, func(c *Config, v bool) { c.NaturalScroll = v }),
	"session_database":       boolOption("Persist per-document view state, marks, and recent files.", func(c *Config) bool { return c.SessionDatabase }, func(c *Config, v bool) { c.SessionDatabase = v }),
	"alt_colors":             boolOption("Start with alternate colors enabled.", func(c *Config) bool { return c.AltColors }, func(c *Config, v bool) { c.AltColors = v }),
	"dual_page":              boolOption("Start in dual-page mode.", func(c *Config) bool { return c.DualPage }, func(c *Config, v bool) { c.DualPage = v }),
	"first_page_offset":      boolOption("Treat the first page as a standalone cover in dual-page mode.", func(c *Config) bool { return c.FirstPageOffset }, func(c *Config, v bool) { c.FirstPageOffset = v }),
	"anti_aliasing":          intOption("MuPDF antialiasing level from 0 through 8.", func(c *Config) int { return c.AntiAliasing }, func(c *Config, v int) { c.AntiAliasing = v }),
	"page_cache_size":        intOption("Maximum rendered pages retained in the cache.", func(c *Config) int { return c.PageCacheSize }, func(c *Config, v int) { c.PageCacheSize = max(1, v) }),
	"outline_initial_depth":  intOption("Outline levels expanded when the outline opens.", func(c *Config) int { return c.OutlineInitialDepth }, func(c *Config, v int) { c.OutlineInitialDepth = v }),
	"outline_width_percent":  intOption("Outline overlay width as a percentage of the window.", func(c *Config) int { return c.OutlineWidthPercent }, func(c *Config, v int) { c.OutlineWidthPercent = v }),
	"outline_height_percent": intOption("Outline overlay height as a percentage of the window.", func(c *Config) int { return c.OutlineHeightPercent }, func(c *Config, v int) { c.OutlineHeightPercent = v }),
	"completion_max_items":   intOption("Maximum command-completion rows.", func(c *Config) int { return c.CompletionMaxItems }, func(c *Config, v int) { c.CompletionMaxItems = max(1, v) }),
	"recent_files_max":       intOption("Maximum recent files retained and displayed.", func(c *Config) int { return c.RecentFilesMax }, func(c *Config, v int) { c.RecentFilesMax = max(0, v) }),
	"scroll_step":            intOption("Keyboard and mouse scroll distance in pixels.", func(c *Config) int { return c.ScrollStep }, func(c *Config, v int) { c.ScrollStep = v }),
	"page_gap": intOption("Vertical gap between pages; aliases page_gap_vertical.", func(c *Config) int { return c.PageGap }, func(c *Config, v int) {
		c.PageGap = v
		c.PageGapVertical = v
	}),
	"spread_gap": intOption("Horizontal spread gap; aliases page_gap_horizontal.", func(c *Config) int { return c.SpreadGap }, func(c *Config, v int) {
		c.SpreadGap = v
		c.PageGapHorizontal = v
	}),
	"page_gap_vertical": intOption("Vertical gap between page rows.", func(c *Config) int { return c.PageGapVertical }, func(c *Config, v int) {
		c.PageGapVertical = v
		c.PageGap = v
	}),
	"page_gap_horizontal": intOption("Horizontal gap between pages in a spread.", func(c *Config) int { return c.PageGapHorizontal }, func(c *Config, v int) {
		c.PageGapHorizontal = v
		c.SpreadGap = v
	}),
	"status_bar_height":    intOption("Status bar height in pixels.", func(c *Config) int { return c.StatusBarHeight }, func(c *Config, v int) { c.StatusBarHeight = v }),
	"status_bar_padding":   intOption("Horizontal status bar padding in pixels.", func(c *Config) int { return c.StatusBarPadding }, func(c *Config, v int) { c.StatusBarPadding = v }),
	"ui_font_size":         intOption("UI font size in pixels.", func(c *Config) int { return c.UIFontSize }, func(c *Config, v int) { c.UIFontSize = v }),
	"sequence_timeout_ms":  intOption("Maximum delay between keys in a binding sequence.", func(c *Config) int { return c.SequenceTimeoutMS }, func(c *Config, v int) { c.SequenceTimeoutMS = v }),
	"render_oversample":    floatOption("Render scale multiplier; values above 1 supersample.", func(c *Config) float64 { return c.RenderOversample }, func(c *Config, v float64) { c.RenderOversample = v }),
	"render_mode":          stringOption("Initial render mode: continuous or single.", func(c *Config) string { return c.RenderMode }, func(c *Config, v string) { c.RenderMode = NormalizeRenderMode(v) }),
	"fit_mode":             stringOption("Initial fit mode: page, width, or manual.", func(c *Config) string { return c.FitMode }, func(c *Config, v string) { c.FitMode = NormalizeFitMode(v) }),
	"anchor_position":      stringOption("Viewport anchor: center, top, or bottom.", func(c *Config) string { return c.AnchorPosition }, func(c *Config, v string) { c.AnchorPosition = NormalizeAnchorPosition(v) }),
	"ui_font_path":         stringOption("Path to a UI font; empty uses the built-in default.", func(c *Config) string { return c.UIFontPath }, func(c *Config, v string) { c.UIFontPath = v }),
	"status_bar_left":      stringOption("Left status bar template.", func(c *Config) string { return c.StatusBarLeft }, func(c *Config, v string) { c.StatusBarLeft = v }),
	"status_bar_right":     stringOption("Right status bar template.", func(c *Config) string { return c.StatusBarRight }, func(c *Config, v string) { c.StatusBarRight = v }),
	"background":           colorOption("Viewer background color.", func(c *Config) [3]uint8 { return c.Background }, func(c *Config, v [3]uint8) { c.Background = v }),
	"page_background":      colorOption("Normal page background color.", func(c *Config) [3]uint8 { return c.PageBackground }, func(c *Config, v [3]uint8) { c.PageBackground = v }),
	"foreground":           colorOption("UI foreground color.", func(c *Config) [3]uint8 { return c.Foreground }, func(c *Config, v [3]uint8) { c.Foreground = v }),
	"status_bar_color":     colorOption("Normal status bar background color.", func(c *Config) [3]uint8 { return c.StatusBarColor }, func(c *Config, v [3]uint8) { c.StatusBarColor = v }),
	"alt_background":       colorOption("Viewer background in alternate-color mode.", func(c *Config) [3]uint8 { return c.AltBackground }, func(c *Config, v [3]uint8) { c.AltBackground = v }),
	"alt_page_background":  colorOption("Page background in alternate-color mode.", func(c *Config) [3]uint8 { return c.AltPageBackground }, func(c *Config, v [3]uint8) { c.AltPageBackground = v }),
	"alt_foreground":       colorOption("UI foreground in alternate-color mode.", func(c *Config) [3]uint8 { return c.AltForeground }, func(c *Config, v [3]uint8) { c.AltForeground = v }),
	"alt_status_bar_color": colorOption("Status bar background in alternate-color mode.", func(c *Config) [3]uint8 { return c.AltStatusBarColor }, func(c *Config, v [3]uint8) { c.AltStatusBarColor = v }),
	"highlight_foreground": colorOption("Selection and search highlight border and text.", func(c *Config) [3]uint8 { return c.HighlightForeground }, func(c *Config, v [3]uint8) { c.HighlightForeground = v }),
	"highlight_background": colorOption("Selection and search highlight background.", func(c *Config) [3]uint8 { return c.HighlightBackground }, func(c *Config, v [3]uint8) { c.HighlightBackground = v }),
}

func luaSettingValue(L *lua.LState, name string, cfg *Config) (lua.LValue, error) {
	desc, ok := configOptions[name]
	if !ok {
		return lua.LNil, fmt.Errorf("unknown setting")
	}
	return desc.get(L, cfg), nil
}

func applyLuaSetting(name string, value lua.LValue, cfg *Config) error {
	desc, ok := configOptions[name]
	if !ok {
		return fmt.Errorf("unknown setting")
	}
	return desc.apply(cfg, value)
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

func NormalizeFitMode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "width" || s == "manual" {
		return s
	}
	return "page"
}

func NormalizeRenderMode(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "single" {
		return s
	}
	return "continuous"
}

func NormalizeAnchorPosition(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "top" || s == "bottom" {
		return s
	}
	return "center"
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
