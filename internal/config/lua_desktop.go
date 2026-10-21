package config

import (
	"fmt"

	lua "github.com/yuin/gopher-lua"
)

func newLuaClipboardTable(L *lua.LState, rt *Runtime) *lua.LTable {
	clipboard := L.NewTable()
	registerLuaFunctions(L, clipboard, "onon.clipboard.", []luaFunctionSpec{
		{Signature: "onon.clipboard.get_text()", Function: luaClipboardGetText(rt)},
		{Signature: "onon.clipboard.set_text(text)", Function: luaClipboardSetText(rt)},
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
	registerLuaFunctions(L, formats, "onon.formats.", []luaFunctionSpec{
		{Signature: "onon.formats.extensions()", Function: luaFormatsExtensions(rt)},
		{Signature: "onon.formats.supports(path)", Function: luaFormatsSupports(rt)},
	})
	return formats
}
