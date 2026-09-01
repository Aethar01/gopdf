package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"strconv"

	lua "github.com/yuin/gopher-lua"
)

type luaJSONShape uint8

const (
	luaJSONObject luaJSONShape = iota
	luaJSONArray
)

// NewLuaJSONModule returns a JSON codec with a state-local, stable null sentinel.
func NewLuaJSONModule(L *lua.LState) *lua.LTable {
	mod := L.NewTable()
	null := L.NewUserData()
	null.Value = &struct{}{}
	shapes := make(map[*lua.LTable]luaJSONShape)

	L.SetField(mod, "null", null)
	L.SetField(mod, "encode", L.NewFunction(func(L *lua.LState) int {
		value, err := luaJSONValue(L.CheckAny(1), null, shapes, make(map[*lua.LTable]bool), "$")
		if err != nil {
			L.RaiseError("json.encode: %v", err)
		}
		encoded, err := json.Marshal(value)
		if err != nil {
			L.RaiseError("json.encode: %v", err)
		}
		L.Push(lua.LString(encoded))
		return 1
	}))
	L.SetField(mod, "decode", L.NewFunction(func(L *lua.LState) int {
		decoder := json.NewDecoder(bytes.NewBufferString(L.CheckString(1)))
		decoder.UseNumber()
		var value any
		if err := decoder.Decode(&value); err != nil {
			L.RaiseError("json.decode: %v", err)
		}
		if err := ensureJSONEOF(decoder); err != nil {
			L.RaiseError("json.decode: %v", err)
		}
		decoded, err := jsonLuaValue(L, value, null, shapes)
		if err != nil {
			L.RaiseError("json.decode: %v", err)
		}
		L.Push(decoded)
		return 1
	}))
	return mod
}

func ensureJSONEOF(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); err == nil {
		return fmt.Errorf("unexpected data after JSON value")
	} else if err != io.EOF {
		return err
	}
	return nil
}

func luaJSONValue(value lua.LValue, null *lua.LUserData, shapes map[*lua.LTable]luaJSONShape, active map[*lua.LTable]bool, path string) (any, error) {
	switch value := value.(type) {
	case *lua.LNilType:
		return nil, fmt.Errorf("%s: nil is not a JSON value; use json.null", path)
	case lua.LBool:
		return bool(value), nil
	case lua.LString:
		return string(value), nil
	case lua.LNumber:
		n := float64(value)
		if math.IsNaN(n) || math.IsInf(n, 0) {
			return nil, fmt.Errorf("%s: non-finite number is not supported", path)
		}
		return n, nil
	case *lua.LUserData:
		if value == null {
			return nil, nil
		}
		return nil, fmt.Errorf("%s: unsupported userdata", path)
	case *lua.LTable:
		if active[value] {
			return nil, fmt.Errorf("%s: cycle detected", path)
		}
		active[value] = true
		defer delete(active, value)
		return luaJSONTable(value, null, shapes, active, path)
	default:
		return nil, fmt.Errorf("%s: unsupported %s", path, value.Type().String())
	}
}

func luaJSONTable(table *lua.LTable, null *lua.LUserData, shapes map[*lua.LTable]luaJSONShape, active map[*lua.LTable]bool, path string) (any, error) {
	shape, marked := shapes[table]
	if !marked {
		shape = luaJSONObject
		table.ForEach(func(key, _ lua.LValue) {
			if key.Type() == lua.LTNumber {
				shape = luaJSONArray
			}
		})
	}

	if shape == luaJSONArray {
		length := table.Len()
		result := make([]any, length)
		count := 0
		var tableErr error
		table.ForEach(func(key, value lua.LValue) {
			if tableErr != nil {
				return
			}
			index, ok := luaJSONIndex(key)
			if !ok || index > length {
				tableErr = fmt.Errorf("%s: arrays require contiguous positive integer keys", path)
				return
			}
			converted, err := luaJSONValue(value, null, shapes, active, fmt.Sprintf("%s[%d]", path, index))
			if err != nil {
				tableErr = err
				return
			}
			result[index-1] = converted
			count++
		})
		if tableErr != nil {
			return nil, tableErr
		}
		if count != length {
			return nil, fmt.Errorf("%s: arrays require contiguous positive integer keys", path)
		}
		return result, nil
	}

	result := make(map[string]any)
	var tableErr error
	table.ForEach(func(key, value lua.LValue) {
		if tableErr != nil {
			return
		}
		name, ok := key.(lua.LString)
		if !ok {
			tableErr = fmt.Errorf("%s: object keys must be strings", path)
			return
		}
		converted, err := luaJSONValue(value, null, shapes, active, path+"."+string(name))
		if err != nil {
			tableErr = err
			return
		}
		result[string(name)] = converted
	})
	return result, tableErr
}

func luaJSONIndex(value lua.LValue) (int, bool) {
	number, ok := value.(lua.LNumber)
	if !ok || number < 1 || number != lua.LNumber(int(number)) {
		return 0, false
	}
	return int(number), true
}

func jsonLuaValue(L *lua.LState, value any, null *lua.LUserData, shapes map[*lua.LTable]luaJSONShape) (lua.LValue, error) {
	switch value := value.(type) {
	case nil:
		return null, nil
	case bool:
		return lua.LBool(value), nil
	case string:
		return lua.LString(value), nil
	case json.Number:
		n, err := strconv.ParseFloat(string(value), 64)
		if err != nil {
			return nil, fmt.Errorf("invalid number %q: %w", value, err)
		}
		return lua.LNumber(n), nil
	case []any:
		table := L.NewTable()
		shapes[table] = luaJSONArray
		for _, item := range value {
			converted, err := jsonLuaValue(L, item, null, shapes)
			if err != nil {
				return nil, err
			}
			table.Append(converted)
		}
		return table, nil
	case map[string]any:
		table := L.NewTable()
		shapes[table] = luaJSONObject
		for key, item := range value {
			converted, err := jsonLuaValue(L, item, null, shapes)
			if err != nil {
				return nil, err
			}
			L.SetField(table, key, converted)
		}
		return table, nil
	default:
		return nil, fmt.Errorf("unsupported decoded value %T", value)
	}
}
