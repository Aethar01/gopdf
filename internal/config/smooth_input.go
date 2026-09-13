package config

import (
	"fmt"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type SmoothInputSources uint8

const (
	SmoothInputMouse SmoothInputSources = 1 << iota
	SmoothInputTrackpad
	SmoothInputKeyboard
	SmoothInputAll = SmoothInputMouse | SmoothInputTrackpad | SmoothInputKeyboard
)

func (s SmoothInputSources) Has(source SmoothInputSources) bool {
	return s&source != 0
}

func FormatSmoothInputSources(sources SmoothInputSources) string {
	return fmt.Sprintf("{ mouse = %t, trackpad = %t, keyboard = %t }",
		sources.Has(SmoothInputMouse),
		sources.Has(SmoothInputTrackpad),
		sources.Has(SmoothInputKeyboard),
	)
}

func parseSmoothInputSourcesText(raw string) (SmoothInputSources, SmoothInputSources, error) {
	state := lua.NewState()
	defer state.Close()

	fn, err := state.Load(strings.NewReader("return "+strings.TrimSpace(raw)), "smooth_input_sources")
	if err != nil {
		return 0, 0, fmt.Errorf("expected table")
	}
	if err := state.CallByParam(lua.P{Fn: fn, NRet: 1, Protect: true}); err != nil {
		return 0, 0, fmt.Errorf("expected table")
	}
	table, ok := state.Get(-1).(*lua.LTable)
	if !ok {
		return 0, 0, fmt.Errorf("expected table")
	}
	return parseSmoothInputSourcesTable(table)
}

func smoothInputSourcesTable(L *lua.LState, sources SmoothInputSources) *lua.LTable {
	table := L.NewTable()
	table.RawSetString("mouse", lua.LBool(sources.Has(SmoothInputMouse)))
	table.RawSetString("trackpad", lua.LBool(sources.Has(SmoothInputTrackpad)))
	table.RawSetString("keyboard", lua.LBool(sources.Has(SmoothInputKeyboard)))
	return table
}

func parseSmoothInputSourcesTable(table *lua.LTable) (SmoothInputSources, SmoothInputSources, error) {
	var sources SmoothInputSources
	var present SmoothInputSources
	known := map[string]SmoothInputSources{
		"mouse":    SmoothInputMouse,
		"trackpad": SmoothInputTrackpad,
		"keyboard": SmoothInputKeyboard,
	}
	var parseErr error
	table.ForEach(func(key, value lua.LValue) {
		if parseErr != nil {
			return
		}
		name, ok := key.(lua.LString)
		if !ok {
			parseErr = fmt.Errorf("smooth_scroll table keys must be mouse, trackpad, or keyboard")
			return
		}
		source, ok := known[string(name)]
		if !ok {
			parseErr = fmt.Errorf("unknown smooth_scroll source %q", string(name))
			return
		}
		if value.Type() != lua.LTBool {
			parseErr = fmt.Errorf("smooth_scroll.%s must be boolean", string(name))
			return
		}
		present |= source
		if lua.LVAsBool(value) {
			sources |= source
		}
	})
	return sources, present, parseErr
}
