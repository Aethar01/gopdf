package config

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type optionDesc struct {
	get   func(L *lua.LState, cfg *Config) lua.LValue
	apply func(cfg *Config, value lua.LValue) error
}

func boolOption(get func(*Config) bool, set func(*Config, bool)) optionDesc {
	return optionDesc{
		get: func(L *lua.LState, cfg *Config) lua.LValue { return lua.LBool(get(cfg)) },
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTBool {
				return fmt.Errorf("expected boolean")
			}
			set(cfg, lua.LVAsBool(value))
			return nil
		},
	}
}

func intOption(get func(*Config) int, set func(*Config, int)) optionDesc {
	return optionDesc{
		get: func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(get(cfg)) },
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTNumber {
				return fmt.Errorf("expected number")
			}
			set(cfg, int(lua.LVAsNumber(value)))
			return nil
		},
	}
}

func floatOption(get func(*Config) float64, set func(*Config, float64)) optionDesc {
	return optionDesc{
		get: func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(get(cfg)) },
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTNumber {
				return fmt.Errorf("expected number")
			}
			set(cfg, float64(lua.LVAsNumber(value)))
			return nil
		},
	}
}

func stringOption(get func(*Config) string, set func(*Config, string)) optionDesc {
	return optionDesc{
		get: func(L *lua.LState, cfg *Config) lua.LValue { return lua.LString(get(cfg)) },
		apply: func(cfg *Config, value lua.LValue) error {
			if value.Type() != lua.LTString {
				return fmt.Errorf("expected string")
			}
			set(cfg, value.String())
			return nil
		},
	}
}

func colorOption(get func(*Config) [3]uint8, set func(*Config, [3]uint8)) optionDesc {
	return optionDesc{
		get: func(L *lua.LState, cfg *Config) lua.LValue {
			tbl := L.NewTable()
			c := get(cfg)
			for i := range 3 {
				tbl.RawSetInt(i+1, lua.LNumber(c[i]))
			}
			return tbl
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

var configOptions = map[string]optionDesc{
	"status_bar_visible":     boolOption(func(c *Config) bool { return c.StatusBarVisible }, func(c *Config, v bool) { c.StatusBarVisible = v }),
	"mouse_text_select":      boolOption(func(c *Config) bool { return c.MouseTextSelect }, func(c *Config, v bool) { c.MouseTextSelect = v }),
	"natural_scroll":         boolOption(func(c *Config) bool { return c.NaturalScroll }, func(c *Config, v bool) { c.NaturalScroll = v }),
	"session_database":       boolOption(func(c *Config) bool { return c.SessionDatabase }, func(c *Config, v bool) { c.SessionDatabase = v }),
	"alt_colors":             boolOption(func(c *Config) bool { return c.AltColors }, func(c *Config, v bool) { c.AltColors = v }),
	"dual_page":              boolOption(func(c *Config) bool { return c.DualPage }, func(c *Config, v bool) { c.DualPage = v }),
	"first_page_offset":      boolOption(func(c *Config) bool { return c.FirstPageOffset }, func(c *Config, v bool) { c.FirstPageOffset = v }),
	"anti_aliasing":          intOption(func(c *Config) int { return c.AntiAliasing }, func(c *Config, v int) { c.AntiAliasing = v }),
	"page_cache_size":        intOption(func(c *Config) int { return c.PageCacheSize }, func(c *Config, v int) { c.PageCacheSize = max(1, v) }),
	"outline_initial_depth":  intOption(func(c *Config) int { return c.OutlineInitialDepth }, func(c *Config, v int) { c.OutlineInitialDepth = v }),
	"outline_width_percent":  intOption(func(c *Config) int { return c.OutlineWidthPercent }, func(c *Config, v int) { c.OutlineWidthPercent = v }),
	"outline_height_percent": intOption(func(c *Config) int { return c.OutlineHeightPercent }, func(c *Config, v int) { c.OutlineHeightPercent = v }),
	"completion_max_items":   intOption(func(c *Config) int { return c.CompletionMaxItems }, func(c *Config, v int) { c.CompletionMaxItems = max(1, v) }),
	"recent_files_max":       intOption(func(c *Config) int { return c.RecentFilesMax }, func(c *Config, v int) { c.RecentFilesMax = max(0, v) }),
	"scroll_step":            intOption(func(c *Config) int { return c.ScrollStep }, func(c *Config, v int) { c.ScrollStep = v }),
	"page_gap": intOption(func(c *Config) int { return c.PageGap }, func(c *Config, v int) {
		c.PageGap = v
		c.PageGapVertical = v
	}),
	"spread_gap": intOption(func(c *Config) int { return c.SpreadGap }, func(c *Config, v int) {
		c.SpreadGap = v
		c.PageGapHorizontal = v
	}),
	"page_gap_vertical": intOption(func(c *Config) int { return c.PageGapVertical }, func(c *Config, v int) {
		c.PageGapVertical = v
		c.PageGap = v
	}),
	"page_gap_horizontal": intOption(func(c *Config) int { return c.PageGapHorizontal }, func(c *Config, v int) {
		c.PageGapHorizontal = v
		c.SpreadGap = v
	}),
	"status_bar_height":    intOption(func(c *Config) int { return c.StatusBarHeight }, func(c *Config, v int) { c.StatusBarHeight = v }),
	"status_bar_padding":   intOption(func(c *Config) int { return c.StatusBarPadding }, func(c *Config, v int) { c.StatusBarPadding = v }),
	"ui_font_size":         intOption(func(c *Config) int { return c.UIFontSize }, func(c *Config, v int) { c.UIFontSize = v }),
	"sequence_timeout_ms":  intOption(func(c *Config) int { return c.SequenceTimeoutMS }, func(c *Config, v int) { c.SequenceTimeoutMS = v }),
	"render_oversample":    floatOption(func(c *Config) float64 { return c.RenderOversample }, func(c *Config, v float64) { c.RenderOversample = v }),
	"render_mode":          stringOption(func(c *Config) string { return c.RenderMode }, func(c *Config, v string) { c.RenderMode = NormalizeRenderMode(v) }),
	"fit_mode":             stringOption(func(c *Config) string { return c.FitMode }, func(c *Config, v string) { c.FitMode = NormalizeFitMode(v) }),
	"anchor_position":      stringOption(func(c *Config) string { return c.AnchorPosition }, func(c *Config, v string) { c.AnchorPosition = NormalizeAnchorPosition(v) }),
	"ui_font_path":         stringOption(func(c *Config) string { return c.UIFontPath }, func(c *Config, v string) { c.UIFontPath = v }),
	"status_bar_left":      stringOption(func(c *Config) string { return c.StatusBarLeft }, func(c *Config, v string) { c.StatusBarLeft = v }),
	"status_bar_right":     stringOption(func(c *Config) string { return c.StatusBarRight }, func(c *Config, v string) { c.StatusBarRight = v }),
	"background":           colorOption(func(c *Config) [3]uint8 { return c.Background }, func(c *Config, v [3]uint8) { c.Background = v }),
	"page_background":      colorOption(func(c *Config) [3]uint8 { return c.PageBackground }, func(c *Config, v [3]uint8) { c.PageBackground = v }),
	"foreground":           colorOption(func(c *Config) [3]uint8 { return c.Foreground }, func(c *Config, v [3]uint8) { c.Foreground = v }),
	"status_bar_color":     colorOption(func(c *Config) [3]uint8 { return c.StatusBarColor }, func(c *Config, v [3]uint8) { c.StatusBarColor = v }),
	"alt_background":       colorOption(func(c *Config) [3]uint8 { return c.AltBackground }, func(c *Config, v [3]uint8) { c.AltBackground = v }),
	"alt_page_background":  colorOption(func(c *Config) [3]uint8 { return c.AltPageBackground }, func(c *Config, v [3]uint8) { c.AltPageBackground = v }),
	"alt_foreground":       colorOption(func(c *Config) [3]uint8 { return c.AltForeground }, func(c *Config, v [3]uint8) { c.AltForeground = v }),
	"alt_status_bar_color": colorOption(func(c *Config) [3]uint8 { return c.AltStatusBarColor }, func(c *Config, v [3]uint8) { c.AltStatusBarColor = v }),
	"highlight_foreground": colorOption(func(c *Config) [3]uint8 { return c.HighlightForeground }, func(c *Config, v [3]uint8) { c.HighlightForeground = v }),
	"highlight_background": colorOption(func(c *Config) [3]uint8 { return c.HighlightBackground }, func(c *Config, v [3]uint8) { c.HighlightBackground = v }),
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
