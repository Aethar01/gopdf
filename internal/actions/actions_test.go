package actions

import "testing"

func TestClipboardDefaultBindings(t *testing.T) {
	bindings := DefaultBindings()
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
			t.Fatalf("did not expect %s as a default binding", key)
		}
	}
}
