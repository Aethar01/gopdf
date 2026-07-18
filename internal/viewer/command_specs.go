package viewer

import "strings"

type commandSpec struct {
	Name           string
	ArgCompletions []string
	Help           string
}

type setSettingSpec struct {
	Name   string
	Action string
}

var setSettingSpecs = []setSettingSpec{
	{Name: "alt_colors!", Action: "toggle_alt_colors"},
	{Name: "dual_page!", Action: "toggle_dual_page"},
	{Name: "first_page_offset!", Action: "toggle_first_page_offset"},
	{Name: "render_mode!", Action: "toggle_render_mode"},
	{Name: "status_bar!", Action: "toggle_status_bar"},
}

var commandSpecs = []commandSpec{
	{Name: "colors", ArgCompletions: []string{"alt", "normal"}, Help: ":colors normal|alt - Set color mode"},
	{Name: "fit", ArgCompletions: []string{"manual", "page", "width"}, Help: ":fit width|page|manual - Set fit mode"},
	{Name: "help", Help: ":help - Show this command help window"},
	{Name: "keybinds", Help: ":keybinds - Toggle the keybinds menu"},
	{Name: "lua", Help: ":lua <code> - Execute Lua code inline"},
	{Name: "mode", ArgCompletions: []string{"continuous", "single"}, Help: ":mode continuous|single - Set render mode"},
	{Name: "open", Help: ":open <filename> - Open another PDF relative to the current document"},
	{Name: "open_file_picker", Help: ":open_file_picker - Open the PDF file picker"},
	{Name: "page", Help: ":page PAGE, :p PAGE, :N - Jump to a page number or label"},
	{Name: "quit", Help: ":quit, :q - Exit"},
	{Name: "reload-config", Help: ":reload-config - Reload the config file"},
	{Name: "recent", Help: ":recent - Open the recent-files menu"},
	{Name: "search", Help: ":search <text> - Search document text"},
	{Name: "set", ArgCompletions: setSettingNames(), Help: ":set " + strings.Join(setSettingNames(), "|") + " - Toggle setting"},
}

func setActionForSetting(name string) (string, bool) {
	for _, spec := range setSettingSpecs {
		if spec.Name == name {
			return spec.Action, true
		}
	}
	return "", false
}

func setSettingNames() []string {
	names := make([]string, 0, len(setSettingSpecs))
	for _, spec := range setSettingSpecs {
		names = append(names, spec.Name)
	}
	return names
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
