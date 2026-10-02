package config

import (
	"fmt"
	"strconv"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

func uiFontWeightOption() optionDesc {
	return optionDesc{
		kind:        "font-weight",
		description: "UI font weight as CSS number 100-900 or alias such as normal, medium, semibold, bold, or black.",
		get:         func(L *lua.LState, cfg *Config) lua.LValue { return lua.LNumber(cfg.Theme.Font.Weight) },
		format:      func(cfg *Config) string { return strconv.Itoa(cfg.Theme.Font.Weight) },
		applyText: func(cfg *Config, raw string) error {
			weight, err := parseUIFontWeight(raw)
			if err != nil {
				return err
			}
			cfg.Theme.Font.Weight = weight
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
			cfg.Theme.Font.Weight = weight
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
