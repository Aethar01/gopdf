package viewer

import "testing"

func TestModalListScrollForSelectionUsesScrollOff(t *testing.T) {
	old := modalListScrollOff
	t.Cleanup(func() { modalListScrollOff = old })
	modalListScrollOff = 2

	if got := modalListScrollForSelection(0, 2, 5, 20); got != 0 {
		t.Fatalf("expected selection at row 2 to keep scroll=0, got %d", got)
	}
	if got := modalListScrollForSelection(0, 3, 5, 20); got != 1 {
		t.Fatalf("expected selection at row 3 to scroll to 1, got %d", got)
	}
	if got := modalListScrollForSelection(5, 6, 5, 20); got != 4 {
		t.Fatalf("expected upward move to preserve two rows above, got %d", got)
	}
}

func TestModalListScrollOffClampsToWindow(t *testing.T) {
	old := modalListScrollOff
	t.Cleanup(func() { modalListScrollOff = old })
	modalListScrollOff = 99

	if got := modalListScrollForSelection(0, 3, 4, 20); got != 2 {
		t.Fatalf("expected large scroll_off to keep selection near center, got %d", got)
	}
	if got := modalListScrollForSelection(0, 0, 4, 20); got != 0 {
		t.Fatalf("expected top edge to remain clamped, got %d", got)
	}
	if got := modalListScrollForSelection(16, 19, 4, 20); got != 16 {
		t.Fatalf("expected bottom edge to remain clamped, got %d", got)
	}
}
