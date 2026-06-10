package viewer

type commandSpec struct {
	Name           string
	ArgCompletions []string
	Help           string
}

var commandSpecs = []commandSpec{
	{Name: "colors", ArgCompletions: []string{"alt", "normal"}, Help: ":colors normal|alt - Set color mode"},
	{Name: "fit", ArgCompletions: []string{"manual", "page", "width"}, Help: ":fit width|page|manual - Set fit mode"},
	{Name: "help", Help: ":help - Show this command help window"},
	{Name: "keybinds", Help: ":keybinds - Toggle the keybinds menu"},
	{Name: "lua", Help: ":lua <code> - Execute Lua code inline"},
	{Name: "mode", ArgCompletions: []string{"continuous", "single"}, Help: ":mode continuous|single - Set render mode"},
	{Name: "open", Help: ":open <filename> - Open another PDF relative to the current document"},
	{Name: "page", Help: ":page N, :p N, :N - Jump to page N"},
	{Name: "quit", Help: ":quit, :q - Exit"},
	{Name: "reload-config", Help: ":reload-config - Reload the config file"},
	{Name: "search", Help: ":search <text> - Search document text"},
	{Name: "set", ArgCompletions: []string{"alt_colors!", "dual_page!", "first_page_offset!", "render_mode!", "status_bar!"}, Help: ":set dual_page!|alt_colors!|render_mode!|first_page_offset!|status_bar! - Toggle setting"},
}

func commandArgCompletionValues(name string) []string {
	for _, spec := range commandSpecs {
		if spec.Name == name {
			return spec.ArgCompletions
		}
	}
	return nil
}

func commandHelpRows() []string {
	rows := make([]string, 0, len(commandSpecs)+1)
	for _, spec := range commandSpecs {
		if spec.Help != "" {
			rows = append(rows, spec.Help)
		}
	}
	rows = append(rows, ":search re:<pattern> - Search with a Go regular expression")
	return rows
}
