package actions

import "testing"

func TestClipboardDefaultBindings(t *testing.T) {
	bindings := DefaultBindingsFor("linux")
	if got := bindings["<C-c>"]; got != "copy" {
		t.Fatalf("expected Ctrl+C copy default, got %q", got)
	}
	if got := bindings["<C-x>"]; got != "cut" {
		t.Fatalf("expected Ctrl+X cut default, got %q", got)
	}
	if got := bindings["<C-v>"]; got != "paste" {
		t.Fatalf("expected Ctrl+V paste default, got %q", got)
	}
	for _, key := range []string{"<D-c>", "<D-x>", "<D-v>"} {
		if _, ok := bindings[key]; ok {
			t.Fatalf("did not expect %s as a default binding on Linux", key)
		}
	}
}

func TestMacOSKeysAddToTheDefaults(t *testing.T) {
	bindings := DefaultBindingsFor("darwin")
	for key, want := range map[string]string{"<D-c>": "copy", "<C-c>": "copy", "<D-z>": "undo", "u": "undo"} {
		if got := bindings[key]; got != want {
			t.Errorf("macOS binds %s to %q, want %q", key, got, want)
		}
	}
}

func TestPlatformKeysAreTheirOwn(t *testing.T) {
	common := DefaultBindingsFor("")
	for _, platform := range Platforms() {
		seen := map[string]string{}
		for name, keys := range platform.Keys {
			if !IsBuiltin(name) {
				t.Errorf("%s binds keys to unknown action %q", platform.Name, name)
			}
			for _, key := range keys {
				if other, ok := common[key]; ok {
					t.Errorf("%s binds %s to %s, over the default %s", platform.Name, key, name, other)
				}
				if other, ok := seen[key]; ok {
					t.Errorf("%s binds %s to both %s and %s", platform.Name, key, other, name)
				}
				seen[key] = name
			}
		}
	}
}
