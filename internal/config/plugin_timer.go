package config

import (
	"time"

	lua "github.com/yuin/gopher-lua"
)

func newLuaPluginTimer(L *lua.LState, instance *pluginInstance) *lua.LTable {
	timer := L.NewTable()
	add := func(repeat bool) lua.LGFunction {
		return func(L *lua.LState) int {
			ms := L.CheckInt(2)
			if ms < 0 || repeat && ms == 0 {
				L.RaiseError("plugin %s timer: invalid interval", instance.manifest.ID)
			}
			callback, ok := L.Get(3).(*lua.LFunction)
			if !ok {
				L.RaiseError("plugin %s timer: expected callback", instance.manifest.ID)
			}
			id := instance.runtime.startPluginTimer(instance.manifest.ID, time.Duration(ms)*time.Millisecond, repeat, callback)
			L.Push(newPluginOperationHandle(L, instance.runtime, id))
			return 1
		}
	}
	L.SetField(timer, "after", L.NewFunction(add(false)))
	L.SetField(timer, "every", L.NewFunction(add(true)))
	return timer
}
