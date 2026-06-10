package actions

type Action struct {
	Name      string
	Countable bool
	Keys      []string
}

var registry = []Action{
	{Name: "next_page", Countable: true, Keys: []string{"J", " ", "<PgDn>"}},
	{Name: "prev_page", Countable: true, Keys: []string{"K", "<PgUp>"}},
	{Name: "scroll_down", Countable: true, Keys: []string{"j", "<Down>"}},
	{Name: "scroll_up", Countable: true, Keys: []string{"k", "<Up>"}},
	{Name: "scroll_left", Countable: true, Keys: []string{"h", "<Left>"}},
	{Name: "scroll_right", Countable: true, Keys: []string{"l", "<Right>"}},
	{Name: "next_spread", Countable: true},
	{Name: "prev_spread", Countable: true},
	{Name: "first_page", Keys: []string{"gg"}},
	{Name: "last_page", Keys: []string{"G"}},
	{Name: "command_mode", Keys: []string{":"}},
	{Name: "search_prompt", Keys: []string{"/"}},
	{Name: "search_prompt_backward", Keys: []string{"?"}},
	{Name: "search_next", Countable: true, Keys: []string{"n"}},
	{Name: "search_prev", Countable: true, Keys: []string{"N"}},
	{Name: "toggle_dual_page", Keys: []string{"d"}},
	{Name: "toggle_render_mode", Keys: []string{"m"}},
	{Name: "toggle_alt_colors", Keys: []string{"<C-r>"}},
	{Name: "toggle_first_page_offset", Keys: []string{"co"}},
	{Name: "toggle_status_bar", Keys: []string{"<C-n>"}},
	{Name: "toggle_fullscreen", Keys: []string{"f"}},
	{Name: "outline", Keys: []string{"o"}},
	{Name: "confirm", Keys: []string{"<CR>"}},
	{Name: "zoom_in", Countable: true, Keys: []string{"+", "="}},
	{Name: "zoom_out", Countable: true, Keys: []string{"-"}},
	{Name: "reset_zoom", Keys: []string{"0"}},
	{Name: "fit_width", Keys: []string{"w"}},
	{Name: "fit_page", Keys: []string{"z"}},
	{Name: "reload_config", Keys: []string{"<C-S-r>"}},
	{Name: "rotate_cw", Keys: []string{"r"}},
	{Name: "rotate_ccw", Keys: []string{"R"}},
	{Name: "goto_page_prompt", Keys: []string{"<C-g>"}},
	{Name: "clear_search"},
	{Name: "show_completion", Keys: []string{"<Tab>"}},
	{Name: "next_completion"},
	{Name: "prev_completion", Keys: []string{"<S-Tab>"}},
	{Name: "close", Keys: []string{"<Esc>"}},
	{Name: "jump_forward", Keys: []string{"<C-i>"}},
	{Name: "jump_backward", Keys: []string{"<C-o>"}},
	{Name: "open_file_picker", Keys: []string{"<C-S-o>"}},
	{Name: "keybinds", Keys: []string{"<F1>"}},
	{Name: "pan"},
	{Name: "quit", Keys: []string{"q"}},
}

var byName = func() map[string]Action {
	m := make(map[string]Action, len(registry))
	for _, action := range registry {
		m[action.Name] = action
	}
	return m
}()

func All() []Action {
	return append([]Action(nil), registry...)
}

func Names() []string {
	names := make([]string, 0, len(registry))
	for _, action := range registry {
		names = append(names, action.Name)
	}
	return names
}

func DefaultBindings() map[string]string {
	bindings := map[string]string{}
	for _, action := range registry {
		for _, key := range action.Keys {
			bindings[key] = action.Name
		}
	}
	return bindings
}

func IsBuiltin(name string) bool {
	_, ok := byName[name]
	return ok
}

func IsCountable(name string) bool {
	action, ok := byName[name]
	return ok && action.Countable
}
