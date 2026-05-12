package viewer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/veandco/go-sdl2/sdl"
)

type completionState struct {
	visible  bool
	items    []completionItem
	selected int
	start    int
	end      int
}

type completionItem struct {
	value   string
	display string
}

var commandCompletionNames = []string{
	"colors",
	"fit",
	"help",
	"mode",
	"open",
	"page",
	"quit",
	"reload-config",
	"search",
	"set",
}

func (a *App) showCompletion() {
	if a.mode != modeCommand {
		return
	}
	if a.completion.visible {
		a.moveCompletion(1)
		return
	}
	items, start, end := a.commandCompletions()
	if len(items) == 0 {
		return
	}
	a.completion = completionState{visible: len(items) > 1, items: items, start: start, end: end}
	if len(items) == 1 {
		a.acceptCompletion()
	}
}

func (a *App) moveCompletion(delta int) {
	if !a.completion.visible {
		a.showCompletion()
		return
	}
	n := len(a.completion.items)
	if n == 0 {
		a.closeCompletion()
		return
	}
	a.completion.selected = (a.completion.selected + delta + n) % n
}

func (a *App) acceptCompletion() {
	if len(a.completion.items) == 0 {
		a.closeCompletion()
		return
	}
	item := a.completion.items[clampInt(a.completion.selected, 0, len(a.completion.items)-1)]
	left, _ := splitAtRune(a.input, a.completion.start)
	_, after := splitAtRune(a.input, a.completion.end)
	a.input = left + item.value + after
	a.inputCursor = a.completion.start + utf8RuneCount(item.value)
	a.closeCompletion()
}

func (a *App) closeCompletion() {
	if a.completion.visible || len(a.completion.items) > 0 {
		a.completion = completionState{}
		a.pendingRedraw = true
	}
}

func (a *App) commandCompletions() ([]completionItem, int, int) {
	left, _ := splitAtRune(a.input, a.inputCursor)
	cmdStart := firstNonSpaceRune(left)
	cmdEnd := commandNameEndRune(a.input, cmdStart)
	if a.inputCursor <= cmdEnd {
		prefix := strings.TrimSpace(sliceRunes(a.input, cmdStart, a.inputCursor))
		return prefixedCommandCompletions(prefix), cmdStart, cmdEnd
	}
	cmd := strings.TrimSpace(sliceRunes(a.input, cmdStart, cmdEnd))
	if cmd != "open" {
		return nil, 0, 0
	}
	argStart := nextNonSpaceRune(a.input, cmdEnd)
	if a.inputCursor < argStart {
		argStart = a.inputCursor
	}
	argEnd := nextSpaceRune(a.input, argStart)
	arg := sliceRunes(a.input, argStart, a.inputCursor)
	return a.openPathCompletions(arg), argStart, argEnd
}

func prefixedCommandCompletions(prefix string) []completionItem {
	items := []completionItem{}
	for _, name := range commandCompletionNames {
		if strings.HasPrefix(name, prefix) {
			items = append(items, completionItem{value: name, display: name})
		}
	}
	return items
}

func (a *App) openPathCompletions(arg string) []completionItem {
	base, prefix, typedBase := splitCompletionPath(arg)
	if arg == "." {
		return []completionItem{{value: "." + string(os.PathSeparator), display: "." + string(os.PathSeparator)}}
	}
	if arg == ".." {
		return []completionItem{{value: ".." + string(os.PathSeparator), display: ".." + string(os.PathSeparator)}}
	}
	readDir := base
	if strings.HasPrefix(base, "~") {
		readDir = expandHomePath(base)
	} else if !filepath.IsAbs(readDir) && a.docPath != "" {
		readDir = filepath.Join(filepath.Dir(a.docPath), readDir)
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return nil
	}
	items := []completionItem{}
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		value := typedBase + name
		display := name
		if entry.IsDir() {
			value += string(os.PathSeparator)
			display += string(os.PathSeparator)
		}
		items = append(items, completionItem{value: value, display: display})
	}
	sort.Slice(items, func(i, j int) bool {
		iDir := strings.HasSuffix(items[i].value, string(os.PathSeparator))
		jDir := strings.HasSuffix(items[j].value, string(os.PathSeparator))
		if iDir != jDir {
			return iDir
		}
		return strings.ToLower(items[i].display) < strings.ToLower(items[j].display)
	})
	return items
}

func splitCompletionPath(arg string) (base, prefix, typedBase string) {
	sep := string(os.PathSeparator)
	if arg == "" {
		return ".", "", ""
	}
	if arg == "~" {
		return "~", "", "~/"
	}
	if strings.HasPrefix(arg, "~/") {
		idx := strings.LastIndex(arg, sep)
		return arg[:idx], arg[idx+1:], arg[:idx+1]
	}
	if strings.HasSuffix(arg, sep) {
		base = strings.TrimSuffix(arg, sep)
		if base == "" && filepath.IsAbs(arg) {
			base = sep
		}
		return base, "", arg
	}
	if strings.Contains(arg, sep) {
		idx := strings.LastIndex(arg, sep)
		base = arg[:idx]
		if base == "" && filepath.IsAbs(arg) {
			base = sep
		}
		return base, arg[idx+1:], arg[:idx+1]
	}
	return ".", arg, ""
}

func expandHomePath(path string) string {
	if path != "~" && !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	return filepath.Join(home, strings.TrimPrefix(path, "~/"))
}

func (a *App) drawCompletion(renderer *sdl.Renderer) error {
	rows := a.visibleCompletionRows()
	if len(rows) == 0 {
		return nil
	}
	rowHeight := a.modalListRowHeight()
	width := 0
	for _, row := range rows {
		width = max(width, measureText(a.fontFace, row.text))
	}
	width = min(max(width+24, 120), max(120, a.winW-16))
	left, _ := splitAtRune(a.input, a.inputCursor)
	x := 8 + measureText(a.fontFace, a.inputPrefix()+left)
	x = clampInt(x, 8, max(8, a.winW-width-8))
	height := len(rows) * rowHeight
	y := a.winH - a.config.StatusBarHeight - height - 4
	y = max(8, y)
	rect := sdl.FRect{X: float32(x), Y: float32(y), W: float32(width), H: float32(height)}
	if err := a.drawModalListFrame(renderer, rect); err != nil {
		return err
	}
	baseline := a.modalListBaselineOffset(rowHeight)
	for i, row := range rows {
		rowY := y + i*rowHeight
		clr := a.foregroundColor()
		if row.selected {
			if err := a.drawModalListSelection(renderer, rect, rowY, rowHeight); err != nil {
				return err
			}
			clr = a.highlightForegroundColor()
		}
		if err := drawText(renderer, a.fontFace, a.truncateModalListText(row.text, width-20), x+10, rowY+baseline, clr); err != nil {
			return err
		}
	}
	return nil
}

type completionRow struct {
	text     string
	selected bool
}

func (a *App) visibleCompletionRows() []completionRow {
	items := a.completion.items
	if len(items) == 0 {
		return nil
	}
	maxItems := max(1, a.config.CompletionMaxItems)
	if len(items) <= maxItems {
		rows := make([]completionRow, len(items))
		for i, item := range items {
			rows[i] = completionRow{text: item.display, selected: i == a.completion.selected}
		}
		return rows
	}
	selected := clampInt(a.completion.selected, 0, len(items)-1)
	start := clampInt(selected-maxItems/2, 0, len(items)-maxItems)
	end := start + maxItems
	showTop := start > 0
	showBottom := end < len(items)
	if showTop {
		start++
	}
	if showBottom {
		end--
	}
	rows := []completionRow{}
	if showTop {
		rows = append(rows, completionRow{text: "..."})
	}
	for i := start; i < end; i++ {
		rows = append(rows, completionRow{text: items[i].display, selected: i == selected})
	}
	if showBottom {
		rows = append(rows, completionRow{text: "..."})
	}
	return rows
}

func firstNonSpaceRune(s string) int {
	for i, r := range []rune(s) {
		if r != ' ' && r != '\t' {
			return i
		}
	}
	return utf8RuneCount(s)
}

func commandNameEndRune(s string, start int) int {
	runes := []rune(s)
	for i := start; i < len(runes); i++ {
		if runes[i] == ' ' || runes[i] == '\t' {
			return i
		}
	}
	return len(runes)
}

func nextNonSpaceRune(s string, start int) int {
	runes := []rune(s)
	for i := start; i < len(runes); i++ {
		if runes[i] != ' ' && runes[i] != '\t' {
			return i
		}
	}
	return len(runes)
}

func nextSpaceRune(s string, start int) int {
	runes := []rune(s)
	for i := start; i < len(runes); i++ {
		if runes[i] == ' ' || runes[i] == '\t' {
			return i
		}
	}
	return len(runes)
}

func sliceRunes(s string, start, end int) string {
	runes := []rune(s)
	start = clampInt(start, 0, len(runes))
	end = clampInt(end, start, len(runes))
	return string(runes[start:end])
}

func utf8RuneCount(s string) int {
	return len([]rune(s))
}
