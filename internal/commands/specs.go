package commands

import "strings"

type Spec struct {
	Name           string
	ArgCompletions []string
	Help           string
}

type CommandReferenceEntry struct {
	Command     string
	Description string
}

var specs = []Spec{
	{Name: "colors", ArgCompletions: []string{"alt", "normal"}, Help: ":colors normal|alt - Set color mode"},
	{Name: "fit", ArgCompletions: []string{"height", "manual", "page", "width"}, Help: ":fit width|height|page|manual - Set fit mode"},
	{Name: "fullscreen", ArgCompletions: []string{"on", "off"}, Help: ":fullscreen [on|off] - Toggle or set fullscreen"},
	{Name: "help", Help: ":help - Show commands, key bindings, search flags and options"},
	{Name: "highlight", Help: ":highlight - Highlight the selection in the last colour used"},
	{Name: "keybinds", Help: ":keybinds - Toggle the keybinds menu"},
	{Name: "lua", Help: ":lua <code> - Execute Lua code inline"},
	{Name: "matches", Help: ":matches - List every match of the current search"},
	{Name: "mode", ArgCompletions: []string{"continuous", "single"}, Help: ":mode continuous|single - Set render mode"},
	{Name: "open", Help: ":open <filename> - Open another PDF relative to the current document"},
	{Name: "open_file_picker", Help: ":open_file_picker - Open the PDF file picker"},
	{Name: "outline", Help: ":outline - Open the document outline"},
	{Name: "overview", Help: ":overview - Toggle the page overview grid"},
	{Name: "page", Help: ":page PAGE, :p PAGE, :N - Jump to a page number or label"},
	{Name: "present", Help: ":present - Toggle presentation mode"},
	{Name: "print", Help: ":print [lp options] - Print the saved document from a dialog, or directly with lp options such as -d office -P 1-3"},
	{Name: "quit", Help: ":quit, :q - Exit; :q! discards unsaved edits"},
	{Name: "recent", Help: ":recent - Open the recent-files menu"},
	{Name: "redo", Help: ":redo - Reapply the last undone edit"},
	{Name: "reload-config", Help: ":reload-config - Reload the config file"},
	{Name: "search", Help: ":search [-r] [-i] [-w] [-p] <text> - Search document text"},
	{Name: "set", Help: ":set [option[?]|option!|option=value] - Inspect or change options"},
	{Name: "undo", Help: ":undo - Undo the last edit"},
	{Name: "write", Help: ":write [path], :w - Save edits, or a copy to path; :wq saves and exits"},
}

func All() []Spec {
	result := make([]Spec, len(specs))
	copy(result, specs)
	return result
}

// Names lists the built-in commands, which plugins may not reuse.
func Names() []string {
	names := make([]string, len(specs))
	for i, spec := range specs {
		names[i] = spec.Name
	}
	return names
}

func ArgCompletionValues(name string) []string {
	for _, spec := range specs {
		if spec.Name == name {
			return spec.ArgCompletions
		}
	}
	return nil
}

type SearchFlag struct {
	Flag        string
	Description string
}

// SearchFlags are the flags a search query may start with, such as -ri.
func SearchFlags() []SearchFlag {
	return []SearchFlag{
		{Flag: "r", Description: "regular expression"},
		{Flag: "i", Description: "ignore case"},
		{Flag: "w", Description: "whole word"},
		{Flag: "p", Description: "current page only"},
	}
}

func CommandReferences() []CommandReferenceEntry {
	refs := make([]CommandReferenceEntry, 0, len(specs))
	for _, spec := range specs {
		command, description, _ := strings.Cut(spec.Help, " - ")
		refs = append(refs, CommandReferenceEntry{Command: command, Description: description})
	}
	return refs
}
