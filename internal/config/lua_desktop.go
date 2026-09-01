package config

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func newLuaClipboardTable(L *lua.LState, rt *Runtime) *lua.LTable {
	clipboard := L.NewTable()
	registerLuaFunctions(L, clipboard, "gopdf.clipboard.", []luaFunctionSpec{
		{
			Signature:   "gopdf.clipboard.get_text()",
			Description: "Return the current UTF-8 clipboard text.",
			Function: func(L *lua.LState) int {
				getter, ok := rt.host.(ClipboardGetter)
				if !ok {
					L.RaiseError("clipboard.get_text: viewer host unavailable")
				}
				L.Push(lua.LString(getter.GetClipboard()))
				return 1
			},
		},
		{
			Signature:   "gopdf.clipboard.set_text(text)",
			Description: "Replace the clipboard with UTF-8 text.",
			Function: func(L *lua.LState) int {
				setter, ok := rt.host.(ClipboardSetter)
				if !ok {
					L.RaiseError("clipboard.set_text: viewer host unavailable")
				}
				if err := setter.SetClipboard(L.CheckString(1)); err != nil {
					L.RaiseError("clipboard.set_text: %v", err)
				}
				return 0
			},
		},
	})
	return clipboard
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprint(err)
}

// newLuaFormatsTable exposes what the document engine can open. The list comes
// from the host rather than a hard-coded set, so it matches the engine actually
// linked into this build.
func newLuaFormatsTable(L *lua.LState, rt *Runtime) *lua.LTable {
	formats := L.NewTable()
	registerLuaFunctions(L, formats, "gopdf.formats.", []luaFunctionSpec{
		{
			Signature:   "gopdf.formats.extensions()",
			Description: "Return the lower-case extensions, without a leading dot, that this build can open.",
			Function: func(L *lua.LState) int {
				host, ok := rt.host.(DocumentFormatHost)
				if !ok {
					L.RaiseError("formats.extensions: document engine unavailable")
				}
				result := L.NewTable()
				for _, ext := range host.SupportedExtensions() {
					result.Append(lua.LString(ext))
				}
				L.Push(result)
				return 1
			},
		},
		{
			Signature:   "gopdf.formats.supports(path)",
			Description: "Report whether a path's extension names an openable format. Opening also sniffs content, so a file without a known extension may still open.",
			Function: func(L *lua.LState) int {
				host, ok := rt.host.(DocumentFormatHost)
				if !ok {
					L.RaiseError("formats.supports: document engine unavailable")
				}
				L.Push(lua.LBool(host.SupportsPath(L.CheckString(1))))
				return 1
			},
		},
	})
	return formats
}
