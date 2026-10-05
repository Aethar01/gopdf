package config

import (
	"fmt"
	"log"
	"sort"

	"gopdf/internal/actions"
	"gopdf/internal/keys"
)

// Keybindings changed in the viewer are kept in the session database, each
// key with the action it runs, or with "" when it was unbound. They apply
// over the defaults before config.lua runs, so the configuration still has
// the last word.

func (r *Runtime) SetKeyBinding(key, action string) error {
	if !r.actionExists(action) {
		return fmt.Errorf("cannot persist unknown action %q", action)
	}
	key, err := keys.Normalize(key)
	if err != nil {
		return err
	}
	if err := r.setKeyBinding(key, action); err != nil {
		return err
	}
	return storeKeyBindings(map[string]string{key: action})
}

func (r *Runtime) RebindKey(oldKey, newKey, action string) error {
	if !r.actionExists(action) {
		return fmt.Errorf("cannot persist unknown action %q", action)
	}
	oldKey, err := keys.Normalize(oldKey)
	if err != nil {
		return err
	}
	newKey, err = keys.Normalize(newKey)
	if err != nil {
		return err
	}
	if err := r.unbindKey(oldKey); err != nil {
		return err
	}
	if err := r.setKeyBinding(newKey, action); err != nil {
		return err
	}
	changes := map[string]string{oldKey: ""}
	changes[newKey] = action // after, so rebinding a key to itself keeps it
	return storeKeyBindings(changes)
}

func (r *Runtime) UnbindKey(key string) error {
	key, err := keys.Normalize(key)
	if err != nil {
		return err
	}
	if err := r.unbindKey(key); err != nil {
		return err
	}
	return storeKeyBindings(map[string]string{key: ""})
}

// applyStoredKeyBindings applies the keybindings changed in the viewer.
// One a newer or older version cannot read is dropped, so it cannot stop
// gopdf from starting. They are not essential, so a database that cannot
// be read leaves the defaults rather than failing.
func (r *Runtime) applyStoredKeyBindings() {
	r.importAutogen()
	stored, err := storedKeyBindings()
	if err != nil {
		log.Printf("stored keybindings: %v", err)
		return
	}
	names := make([]string, 0, len(stored))
	for key := range stored {
		names = append(names, key)
	}
	sort.Strings(names)
	var invalid []string
	for _, key := range names {
		var err error
		if action := stored[key]; action == "" {
			err = r.unbindKey(key)
		} else {
			err = r.setKeyBinding(key, action)
		}
		if err != nil {
			log.Printf("stored keybindings: dropping %q: %v", key, err)
			invalid = append(invalid, key)
		}
	}
	if err := deleteStoredKeyBindings(invalid); err != nil {
		log.Printf("stored keybindings: %v", err)
	}
}

func Actions() []string {
	return actions.Names()
}
