package viewer

import (
	"strings"

	"gopdf/internal/config"
)

type commandSpec struct {
	Name           string
	ArgCompletions []string
	Help           string
}

type CommandReferenceEntry struct {
	Command     string
	Description string
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
	{Name: "search", Help: ":search [-r] [-i] [-w] [-p] <text> - Search document text"},
	{Name: "set", ArgCompletions: config.OptionNames(), Help: ":set [option[?]|option!|option=value] - Inspect or change options"},
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
	rows = append(rows, "Search flags: -r regex, -i ignore case, -w whole word, -p current page")
	return rows
}

func CommandReferences() []CommandReferenceEntry {
	refs := make([]CommandReferenceEntry, 0, len(commandSpecs))
	for _, spec := range commandSpecs {
		command, description, _ := strings.Cut(spec.Help, " - ")
		refs = append(refs, CommandReferenceEntry{Command: command, Description: description})
	}
	return refs
}
