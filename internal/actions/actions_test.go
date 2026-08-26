package actions

import "testing"

func TestClipboardDefaultBindingsByOS(t *testing.T) {
	nonDarwin := DefaultBindingsForOS("linux")
	if got := nonDarwin["<C-c>"]; got != "copy" {
		t.Fatalf("expected Ctrl+C copy default outside macOS, got %q", got)
	}
	if got := nonDarwin["<C-x>"]; got != "cut" {
		t.Fatalf("expected Ctrl+X cut default outside macOS, got %q", got)
	}
	if got := nonDarwin["<C-v>"]; got != "paste" {
		t.Fatalf("expected Ctrl+V paste default outside macOS, got %q", got)
	}

	darwin := DefaultBindingsForOS("darwin")
	if got := darwin["<D-c>"]; got != "copy" {
		t.Fatalf("expected Command+C copy default on macOS, got %q", got)
	}
	if got := darwin["<D-x>"]; got != "cut" {
		t.Fatalf("expected Command+X cut default on macOS, got %q", got)
	}
	if got := darwin["<D-v>"]; got != "paste" {
		t.Fatalf("expected Command+V paste default on macOS, got %q", got)
	}
	for _, key := range []string{"<C-c>", "<C-x>", "<C-v>"} {
		if _, ok := darwin[key]; ok {
			t.Fatalf("did not expect %s as a macOS clipboard default", key)
		}
	}
}
