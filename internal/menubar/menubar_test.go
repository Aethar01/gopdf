package menubar

import (
	"testing"

	"gopdf/internal/actions"
	"gopdf/internal/keys"
)

func TestItemsRunKnownActions(t *testing.T) {
	for _, menu := range Menus() {
		for _, item := range menu.Items {
			if item.Action != "" && !actions.IsBuiltin(item.Action) {
				t.Errorf("%s menu item %q runs unknown action %q", menu.Title, item.Title, item.Action)
			}
			if item.Action != "" && item.Role != "" {
				t.Errorf("%s menu item %q has both an action and a role", menu.Title, item.Title)
			}
		}
	}
}

func TestShortcut(t *testing.T) {
	bindings := map[string]string{
		"=":       "zoom_in",
		"+":       "zoom_in",
		"<D-S-=>": "zoom_in",
		"<D-=>":   "zoom_in",
		"gg":      "first_page",
		"<C-g>":   "goto_page_prompt",
		"<D-q>":   "quit",
		"<D-w>":   "quit",
	}
	tests := []struct {
		item   Item
		prefer keys.Mod
		want   string
		ok     bool
	}{
		{item: Item{Action: "zoom_in"}, prefer: keys.Cmd, want: "<D-=>", ok: true},
		{item: Item{Action: "zoom_in"}, prefer: 0, want: "+", ok: true},
		{item: Item{Action: "goto_page_prompt"}, prefer: keys.Cmd, want: "<C-g>", ok: true},
		{item: Item{Action: "first_page"}, prefer: keys.Cmd},
		{item: Item{Action: "save"}, prefer: keys.Cmd},
		{item: Item{Action: "quit", Key: "<D-w>"}, prefer: keys.Cmd, want: "<D-w>", ok: true},
		{item: Item{Action: "quit", Key: "<D-q>"}, prefer: keys.Cmd, want: "<D-q>", ok: true},
		{item: Item{Action: "zoom_in", Key: "<D-w>"}, prefer: keys.Cmd, want: "<D-=>", ok: true},
	}
	for _, tt := range tests {
		key, ok := Shortcut(bindings, tt.item, tt.prefer)
		if ok != tt.ok || ok && key.String() != tt.want {
			t.Errorf("Shortcut(%+v, %v) = %s, %t; want %s, %t", tt.item, tt.prefer, key, ok, tt.want, tt.ok)
		}
	}
}
