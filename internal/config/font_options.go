package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

const uiFontSelectorScheme = "gopdf-font"

func init() {
	configOptions["ui_font"] = stringOption("Installed UI font family; empty uses the built-in font.", func(c *Config) string { return c.UIFont }, func(c *Config, v string) {
		c.UIFont = strings.TrimSpace(v)
		syncUIFontPath(c)
	})
	configOptions["ui_font_style"] = stringOption("UI font style: normal, italic, or oblique.", func(c *Config) string { return c.UIFontStyle }, func(c *Config, v string) {
		c.UIFontStyle = normalizeUIFontStyle(v)
		syncUIFontPath(c)
	})
	configOptions["ui_font_weight"] = uiFontWeightOption()
	configOptions["ui_font_path"] = stringOption("Explicit UI font file path; overrides ui_font, ui_font_style, and ui_font_weight.", func(c *Config) string { return c.UIFontPathOverride }, func(c *Config, v string) {
		c.UIFontPathOverride = strings.TrimSpace(v)
		syncUIFontPath(c)
	})
}

func uiFontWeightOption() optionDesc {
	return optionDesc{
		kind:        "font-weight",
		description: "UI font weight as CSS number 100-900 or alias such as normal, medium, semibold, bold, or black.",
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(cfg.UIFontWeight) },
		format:      func(cfg *Config) string { return strconv.Itoa(cfg.UIFontWeight) },
		applyText: func(cfg *Config, raw string) error {
			weight, err := parseUIFontWeight(raw)
			if err != nil {
				return err
			}
			cfg.UIFontWeight = weight
			syncUIFontPath(cfg)
			return nil
		},
		apply: func(cfg *Config, value lua.LValue) error {
			var raw string
			switch value.Type() {
			case lua.LTNumber:
				raw = strconv.Itoa(int(lua.LVAsNumber(value)))
			case lua.LTString:
				raw = value.String()
			default:
				return fmt.Errorf("expected number or string")
			}
			weight, err := parseUIFontWeight(raw)
			if err != nil {
				return err
			}
			cfg.UIFontWeight = weight
			syncUIFontPath(cfg)
			return nil
		},
	}
}

func normalizeUIFontStyle(style string) string {
	switch strings.ToLower(strings.TrimSpace(style)) {
	case "italic":
		return "italic"
	case "oblique":
		return "oblique"
	default:
		return "normal"
	}
}

func parseUIFontWeight(raw string) (int, error) {
	raw = strings.ToLower(strings.TrimSpace(raw))
	aliases := map[string]int{
		"thin":        100,
		"extralight":  200,
		"extra-light": 200,
		"ultralight":  200,
		"light":       300,
		"normal":      400,
		"regular":     400,
		"medium":      500,
		"semibold":    600,
		"semi-bold":   600,
		"demibold":    600,
		"bold":        700,
		"extrabold":   800,
		"extra-bold":  800,
		"ultrabold":   800,
		"black":       900,
		"heavy":       900,
	}
	if weight, ok := aliases[raw]; ok {
		return weight, nil
	}
	weight, err := strconv.Atoi(raw)
	if err != nil || weight < 100 || weight > 900 {
		return 0, fmt.Errorf("expected font weight 100-900 or a named alias")
	}
	return weight, nil
}

func syncUIFontPath(c *Config) {
	if c.UIFontPathOverride != "" {
		c.UIFontPath = c.UIFontPathOverride
		return
	}
	if c.UIFont == "" {
		c.UIFontPath = ""
		return
	}
	query := url.Values{}
	query.Set("family", c.UIFont)
	query.Set("style", normalizeUIFontStyle(c.UIFontStyle))
	query.Set("weight", strconv.Itoa(c.UIFontWeight))
	c.UIFontPath = uiFontSelectorScheme + "://system?" + query.Encode()
}
