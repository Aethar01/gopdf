package config

import (
	"fmt"
	"sort"
	"strings"

	lua "github.com/yuin/gopher-lua"
)

type OptionReference struct {
	Name        string
	Type        string
	Default     string
	Description string
}

type LuaReferenceEntry struct {
	Signature   string
	Description string
}

func OptionReferences() []OptionReference {
	defaults := Default()
	refs := make([]OptionReference, 0, len(configOptions))
	for _, name := range OptionNames() {
		desc := configOptions[name]
		value := desc.format(&defaults)
		if desc.kind == "color" {
			value = "{" + strings.ReplaceAll(value, ",", ", ") + "}"
		}
		refs = append(refs, OptionReference{Name: name, Type: desc.kind, Default: value, Description: desc.description})
	}
	return refs
}

func LuaReferences() []LuaReferenceEntry {
	luaFunctionReferencesMu.Lock()
	empty := len(luaFunctionReferences) == 0
	luaFunctionReferencesMu.Unlock()
	if empty {
		L := lua.NewState()
		cfg := Default()
		newLuaModule(L, &Runtime{}, &cfg)
		L.Close()
	}
	luaFunctionReferencesMu.Lock()
	refs := make([]LuaReferenceEntry, 0, len(luaFunctionReferences))
	for _, ref := range luaFunctionReferences {
		refs = append(refs, ref)
	}
	luaFunctionReferencesMu.Unlock()
	sort.Slice(refs, func(i, j int) bool { return refs[i].Signature < refs[j].Signature })
	return refs
}

func ValidateReferenceMetadata() error {
	for _, name := range OptionNames() {
		if strings.TrimSpace(configOptions[name].description) == "" {
			return fmt.Errorf("missing option documentation: %s", name)
		}
	}
	return nil
}
