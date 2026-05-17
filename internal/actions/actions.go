package actions

type Action struct {
	Name      string
	Countable bool
}

var registry = []Action{
	{Name: "next_page", Countable: true},
	{Name: "prev_page", Countable: true},
	{Name: "scroll_down", Countable: true},
	{Name: "scroll_up", Countable: true},
	{Name: "scroll_left", Countable: true},
	{Name: "scroll_right", Countable: true},
	{Name: "next_spread", Countable: true},
	{Name: "prev_spread", Countable: true},
	{Name: "first_page"},
	{Name: "last_page"},
	{Name: "command_mode"},
	{Name: "search_prompt"},
	{Name: "search_prompt_backward"},
	{Name: "search_next", Countable: true},
	{Name: "search_prev", Countable: true},
	{Name: "toggle_dual_page"},
	{Name: "toggle_render_mode"},
	{Name: "toggle_alt_colors"},
	{Name: "toggle_first_page_offset"},
	{Name: "toggle_status_bar"},
	{Name: "toggle_fullscreen"},
	{Name: "outline"},
	{Name: "confirm"},
	{Name: "zoom_in", Countable: true},
	{Name: "zoom_out", Countable: true},
	{Name: "reset_zoom"},
	{Name: "fit_width"},
	{Name: "fit_page"},
	{Name: "reload_config"},
	{Name: "rotate_cw"},
	{Name: "rotate_ccw"},
	{Name: "goto_page_prompt"},
	{Name: "clear_search"},
	{Name: "show_completion"},
	{Name: "next_completion"},
	{Name: "prev_completion"},
	{Name: "close"},
	{Name: "jump_forward"},
	{Name: "jump_backward"},
	{Name: "open_file_picker"},
	{Name: "keybinds"},
	{Name: "pan"},
	{Name: "quit"},
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

func IsBuiltin(name string) bool {
	_, ok := byName[name]
	return ok
}

func IsCountable(name string) bool {
	action, ok := byName[name]
	return ok && action.Countable
}
