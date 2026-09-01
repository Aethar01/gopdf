package config

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"

	lua "github.com/yuin/gopher-lua"
)

const pluginStorageValueLimit = 64 << 10

func initPluginStorage(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS plugin_storage (
			plugin_id TEXT NOT NULL,
			key TEXT NOT NULL,
			value BLOB NOT NULL,
			PRIMARY KEY (plugin_id, key)
		)
	`)
	return err
}

func newLuaPluginStorage(L *lua.LState, pluginID string) *lua.LTable {
	storage := L.NewTable()
	L.SetField(storage, "get", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		data, ok, err := getPluginStorageValue(pluginID, key)
		if err != nil {
			L.RaiseError("plugin storage get: %v", err)
		}
		if !ok {
			L.Push(lua.LNil)
			return 1
		}
		value, err := decodePluginStorageValue(L, data)
		if err != nil {
			L.RaiseError("plugin storage get: %v", err)
		}
		L.Push(value)
		return 1
	}))
	L.SetField(storage, "set", L.NewFunction(func(L *lua.LState) int {
		key := L.CheckString(2)
		data, err := encodePluginStorageValue(L.Get(3))
		if err != nil {
			L.RaiseError("plugin storage set: %v", err)
		}
		if len(data) > pluginStorageValueLimit {
			L.RaiseError("plugin storage set: value exceeds %d byte limit", pluginStorageValueLimit)
		}
		if err := setPluginStorageValue(pluginID, key, data); err != nil {
			L.RaiseError("plugin storage set: %v", err)
		}
		return 0
	}))
	L.SetField(storage, "delete", L.NewFunction(func(L *lua.LState) int {
		if err := deletePluginStorageValue(pluginID, L.CheckString(2)); err != nil {
			L.RaiseError("plugin storage delete: %v", err)
		}
		return 0
	}))
	L.SetField(storage, "keys", L.NewFunction(func(L *lua.LState) int {
		keys, err := pluginStorageKeys(pluginID)
		if err != nil {
			L.RaiseError("plugin storage keys: %v", err)
		}
		result := L.NewTable()
		for i, key := range keys {
			result.RawSetInt(i+1, lua.LString(key))
		}
		L.Push(result)
		return 1
	}))
	return storage
}

// errPluginStorageUnavailable reports that no session database backs storage,
// so a write cannot be persisted. Reads treat the same condition as "absent".
var errPluginStorageUnavailable = errors.New("plugin storage is unavailable; no session database location could be determined")

func getPluginStorageValue(pluginID, key string) ([]byte, bool, error) {
	db, err := openSessionDatabase()
	if err != nil || db == nil {
		return nil, false, err
	}
	defer db.Close()
	var data []byte
	err = db.QueryRow(`SELECT value FROM plugin_storage WHERE plugin_id = ? AND key = ?`, pluginID, key).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	return data, err == nil, err
}

func setPluginStorageValue(pluginID, key string, data []byte) error {
	db, err := openSessionDatabase()
	if err != nil {
		return err
	}
	if db == nil {
		return errPluginStorageUnavailable
	}
	defer db.Close()
	_, err = db.Exec(`
		INSERT INTO plugin_storage (plugin_id, key, value) VALUES (?, ?, ?)
		ON CONFLICT(plugin_id, key) DO UPDATE SET value = excluded.value
	`, pluginID, key, data)
	return err
}

func deletePluginStorageValue(pluginID, key string) error {
	db, err := openSessionDatabase()
	if err != nil {
		return err
	}
	if db == nil {
		return errPluginStorageUnavailable
	}
	defer db.Close()
	_, err = db.Exec(`DELETE FROM plugin_storage WHERE plugin_id = ? AND key = ?`, pluginID, key)
	return err
}

func pluginStorageKeys(pluginID string) ([]string, error) {
	db, err := openSessionDatabase()
	if err != nil || db == nil {
		return nil, err
	}
	defer db.Close()
	rows, err := db.Query(`SELECT key FROM plugin_storage WHERE plugin_id = ? ORDER BY key`, pluginID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func encodePluginStorageValue(value lua.LValue) ([]byte, error) {
	converted, err := luaPluginStorageValue(value, make(map[*lua.LTable]bool))
	if err != nil {
		return nil, err
	}
	return json.Marshal(converted)
}

func luaPluginStorageValue(value lua.LValue, active map[*lua.LTable]bool) (any, error) {
	switch value := value.(type) {
	case *lua.LNilType:
		return nil, nil
	case lua.LString:
		return string(value), nil
	case lua.LNumber:
		n := float64(value)
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, fmt.Errorf("number must be finite")
		}
		return n, nil
	case lua.LBool:
		return bool(value), nil
	case *lua.LTable:
		if active[value] {
			return nil, fmt.Errorf("cyclic table")
		}
		active[value] = true
		defer delete(active, value)

		kind := 0 // 0 empty, 1 array, 2 object
		maxIndex, count := 0, 0
		var tableErr error
		value.ForEach(func(key, _ lua.LValue) {
			if tableErr != nil {
				return
			}
			count++
			switch key := key.(type) {
			case lua.LString:
				if kind == 1 {
					tableErr = fmt.Errorf("table cannot mix array and object keys")
					return
				}
				kind = 2
			case lua.LNumber:
				index := int(key)
				if kind == 2 || float64(key) != float64(index) || index < 1 {
					tableErr = fmt.Errorf("table keys must be strings or contiguous positive integers")
					return
				}
				kind = 1
				if index > maxIndex {
					maxIndex = index
				}
			default:
				tableErr = fmt.Errorf("table keys must be strings or contiguous positive integers")
			}
		})
		if tableErr != nil {
			return nil, tableErr
		}
		if kind == 1 {
			if maxIndex != count {
				return nil, fmt.Errorf("array must not contain gaps")
			}
			items := make([]any, maxIndex)
			for i := 1; i <= maxIndex; i++ {
				item, err := luaPluginStorageValue(value.RawGetInt(i), active)
				if err != nil {
					return nil, err
				}
				items[i-1] = item
			}
			return items, nil
		}
		object := make(map[string]any, count)
		value.ForEach(func(key, item lua.LValue) {
			if tableErr != nil {
				return
			}
			converted, err := luaPluginStorageValue(item, active)
			if err != nil {
				tableErr = err
				return
			}
			object[string(key.(lua.LString))] = converted
		})
		return object, tableErr
	default:
		return nil, fmt.Errorf("unsupported Lua value type %s", value.Type())
	}
}

func decodePluginStorageValue(L *lua.LState, data []byte) (lua.LValue, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}
	return pluginStorageLuaValue(L, value), nil
}

func pluginStorageLuaValue(L *lua.LState, value any) lua.LValue {
	switch value := value.(type) {
	case nil:
		return lua.LNil
	case string:
		return lua.LString(value)
	case float64:
		return lua.LNumber(value)
	case bool:
		return lua.LBool(value)
	case []any:
		table := L.NewTable()
		for i, item := range value {
			table.RawSetInt(i+1, pluginStorageLuaValue(L, item))
		}
		return table
	case map[string]any:
		table := L.NewTable()
		for key, item := range value {
			table.RawSetString(key, pluginStorageLuaValue(L, item))
		}
		return table
	default:
		panic(fmt.Sprintf("unexpected decoded plugin storage type %T", value))
	}
}
