package config

import (
	"strings"
	"testing"

	lua "github.com/yuin/gopher-lua"
)

func TestLuaPluginStorageIsolationKeysAndDeletion(t *testing.T) {
	setTestDataDir(t)
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("first", newLuaPluginStorage(L, "first"))
	L.SetGlobal("second", newLuaPluginStorage(L, "second"))

	if err := L.DoString(`
first:set("shared", "first value")
first:set("alpha", true)
second:set("shared", "second value")
assert(first:get("shared") == "first value")
assert(second:get("shared") == "second value")
assert(second:get("alpha") == nil)
local keys = first:keys()
assert(#keys == 2 and keys[1] == "alpha" and keys[2] == "shared")
first:delete("shared")
assert(first:get("shared") == nil)
assert(second:get("shared") == "second value")
assert(#first:keys() == 1)
`); err != nil {
		t.Fatal(err)
	}
}

func TestLuaPluginStorageSupportedTypes(t *testing.T) {
	setTestDataDir(t)
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("storage", newLuaPluginStorage(L, "types"))

	if err := L.DoString(`
storage:set("string", "hello")
storage:set("number", 12.5)
storage:set("bool", false)
storage:set("nil", nil)
storage:set("list", {"one", 2, true, {nested = "value"}})
storage:set("object", {name = "sample", nested = {1, 2, 3}})
assert(storage:get("string") == "hello")
assert(storage:get("number") == 12.5)
assert(storage:get("bool") == false)
assert(storage:get("nil") == nil)
local list = storage:get("list")
assert(#list == 4 and list[1] == "one" and list[2] == 2 and list[3] == true)
assert(list[4].nested == "value")
local object = storage:get("object")
assert(object.name == "sample" and #object.nested == 3 and object.nested[3] == 3)
`); err != nil {
		t.Fatal(err)
	}

	data, ok, err := getPluginStorageValue("types", "list")
	if err != nil || !ok {
		t.Fatalf("stored list missing: ok=%v err=%v", ok, err)
	}
	if !strings.HasPrefix(string(data), "[") {
		t.Fatalf("list encoded as %q, want JSON array", data)
	}
	data, ok, err = getPluginStorageValue("types", "object")
	if err != nil || !ok {
		t.Fatalf("stored object missing: ok=%v err=%v", ok, err)
	}
	if !strings.HasPrefix(string(data), "{") {
		t.Fatalf("object encoded as %q, want JSON object", data)
	}
}

func TestLuaPluginStorageRejectsUnsupportedValuesAndCycles(t *testing.T) {
	setTestDataDir(t)
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("storage", newLuaPluginStorage(L, "invalid"))

	for name, script := range map[string]string{
		"function": `storage:set("bad", function() end)`,
		"cycle":    `local value = {}; value.self = value; storage:set("bad", value)`,
		"mixed":    `storage:set("bad", {[1] = "one", name = "value"})`,
		"sparse":   `storage:set("bad", {[2] = "two"})`,
	} {
		t.Run(name, func(t *testing.T) {
			if err := L.DoString(script); err == nil {
				t.Fatal("expected storage set to fail")
			}
		})
	}
}

func TestLuaPluginStorageValueSizeLimit(t *testing.T) {
	setTestDataDir(t)
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("storage", newLuaPluginStorage(L, "limit"))
	L.SetGlobal("large", lua.LString(strings.Repeat("x", pluginStorageValueLimit)))

	if err := L.DoString(`storage:set("large", large)`); err == nil || !strings.Contains(err.Error(), "byte limit") {
		t.Fatalf("expected size limit error, got %v", err)
	}
	if _, ok, err := getPluginStorageValue("limit", "large"); err != nil || ok {
		t.Fatalf("oversized value was stored: ok=%v err=%v", ok, err)
	}
}

func TestLuaPluginStorageReportsUnavailableWrites(t *testing.T) {
	// With no data directory there is no session database, so a write cannot be
	// persisted and must say so rather than silently discarding the value.
	t.Setenv("XDG_DATA_HOME", "")
	t.Setenv("HOME", "")
	t.Setenv("USERPROFILE", "")
	if SessionDatabasePath() != "" {
		t.Skip("platform still resolves a data directory without HOME")
	}
	L := lua.NewState()
	defer L.Close()
	L.SetGlobal("store", newLuaPluginStorage(L, "sample"))

	if err := L.DoString(`assert(store:get("missing") == nil)`); err != nil {
		t.Fatalf("read should treat unavailable storage as absent: %v", err)
	}
	err := L.DoString(`store:set("key", "value")`)
	if err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("set error = %v, want one containing \"unavailable\"", err)
	}
	if err := L.DoString(`store:delete("key")`); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("delete error = %v, want one containing \"unavailable\"", err)
	}
}
