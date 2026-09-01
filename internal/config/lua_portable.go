package config

import lua "github.com/yuin/gopher-lua"

// PreloadPortableLuaModules installs the synchronous modules in package.preload.
func PreloadPortableLuaModules(L *lua.LState) {
	preloadLuaModule(L, "gopdf.platform", NewLuaPlatformModule)
	preloadLuaModule(L, "gopdf.path", NewLuaPathModule)
	preloadLuaModule(L, "gopdf.json", NewLuaJSONModule)
}

func preloadLuaModule(L *lua.LState, name string, constructor func(*lua.LState) *lua.LTable) {
	L.PreloadModule(name, func(L *lua.LState) int {
		L.Push(constructor(L))
		return 1
	})
}
