package viewer

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopdf/internal/config"

	"github.com/jupiterrider/purego-sdl3/sdl"
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
	a.input.ReplaceRange(a.completion.start, a.completion.end, item.value)
	a.closeCompletion()
}

func (a *App) closeCompletion() {
	if a.completion.visible || len(a.completion.items) > 0 {
		a.completion = completionState{}
		a.pendingRedraw = true
	}
}

func (a *App) commandCompletions() ([]completionItem, int, int) {
	left := a.input.Left()
	cmdStart := firstNonSpaceRune(left)
	cmdEnd := commandNameEndRune(a.input.Value, cmdStart)
	if a.input.Cursor <= cmdEnd {
		prefix := strings.TrimSpace(sliceRunes(a.input.Value, cmdStart, a.input.Cursor))
		return prefixedCommandCompletions(prefix), cmdStart, cmdEnd
	}
	cmd := strings.TrimSpace(sliceRunes(a.input.Value, cmdStart, cmdEnd))
	argStart := nextNonSpaceRune(a.input.Value, cmdEnd)
	if a.input.Cursor < argStart {
		argStart = a.input.Cursor
	}
	argEnd := nextSpaceRune(a.input.Value, argStart)
	arg := sliceRunes(a.input.Value, argStart, a.input.Cursor)
	if cmd == "open" {
		return a.openPathCompletions(arg), argStart, argEnd
	}
	if validArgs := commandArgCompletionValues(cmd); len(validArgs) > 0 {
		items := []completionItem{}
		for _, v := range validArgs {
			if strings.HasPrefix(v, arg) {
				items = append(items, completionItem{value: v, display: v})
			}
		}
		if len(items) > 0 {
			return items, argStart, argEnd
		}
	}
	return nil, 0, 0
}

func prefixedCommandCompletions(prefix string) []completionItem {
	items := []completionItem{}
	for _, spec := range commandSpecs {
		if strings.HasPrefix(spec.Name, prefix) {
			items = append(items, completionItem{value: spec.Name, display: spec.Name})
		}
	}
	return items
}

func (a *App) openPathCompletions(arg string) []completionItem {
	arg = unescapeCommandArg(arg)
	recent := a.recentFileCompletions(arg)
	base, prefix, typedBase := splitCompletionPath(arg)
	if arg == "." {
		return append(recent, completionItem{value: "." + pathSeparator(), display: "." + pathSeparator()})
	}
	if arg == ".." {
		return append(recent, completionItem{value: ".." + pathSeparator(), display: ".." + pathSeparator()})
	}
	readDir := base
	if strings.HasPrefix(base, "~") {
		readDir = expandHomePath(base)
	} else if !filepath.IsAbs(readDir) && a.docPath != "" {
		readDir = filepath.Join(filepath.Dir(a.docPath), readDir)
	}
	entries, err := os.ReadDir(readDir)
	if err != nil {
		return recent
	}
	items := recent
	for _, entry := range entries {
		name := entry.Name()
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		value := escapeCompletionPath(typedBase + name)
		display := name
		if entry.IsDir() {
			value += pathSeparator()
			display += pathSeparator()
		}
		items = append(items, completionItem{value: value, display: display})
	}
	sort.Slice(items, func(i, j int) bool {
		iDir := strings.HasSuffix(items[i].value, pathSeparator())
		jDir := strings.HasSuffix(items[j].value, pathSeparator())
		if iDir != jDir {
			return iDir
		}
		return strings.ToLower(items[i].display) < strings.ToLower(items[j].display)
	})
	return items
}

func (a *App) recentFileCompletions(arg string) []completionItem {
	if !a.config.SessionDatabase {
		return nil
	}
	argLower := strings.ToLower(arg)
	items := []completionItem{}
	seen := map[string]bool{}
	for _, path := range config.RecentFiles(a.config.RecentFilesMax) {
		if path == "" || seen[path] {
			continue
		}
		seen[path] = true
		base := filepath.Base(path)
		if argLower != "" && !strings.Contains(strings.ToLower(path), argLower) && !strings.Contains(strings.ToLower(base), argLower) {
			continue
		}
		items = append(items, completionItem{value: escapeCompletionPath(path), display: "recent: " + base})
	}
	return items
}

func escapeCompletionPath(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == ' ' || (path[i] == '\\' && filepath.Separator != '\\') {
			b.WriteByte('\\')
		}
		b.WriteByte(path[i])
	}
	return b.String()
}

func splitCompletionPath(arg string) (base, prefix, typedBase string) {
	arg = filepath.FromSlash(arg)
	sep := pathSeparator()
	if arg == "" {
		return ".", "", ""
	}
	if arg == "~" {
		return "~", "", "~" + sep
	}
	if hasHomePathPrefix(arg) {
		base, prefix = filepath.Split(arg)
		return strings.TrimSuffix(base, sep), prefix, base
	}
	if strings.HasSuffix(arg, sep) {
		return filepath.Clean(arg), "", arg
	}
	base, prefix = filepath.Split(arg)
	if base != "" {
		return strings.TrimSuffix(base, sep), prefix, base
	}
	return ".", arg, ""
}

func pathSeparator() string {
	return string(filepath.Separator)
}

func expandHomePath(path string) string {
	path = filepath.FromSlash(path)
	if path != "~" && !hasHomePathPrefix(path) {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if path == "~" {
		return home
	}
	rest := strings.TrimPrefix(path, "~"+pathSeparator())
	return filepath.Join(home, rest)
}

func hasHomePathPrefix(path string) bool {
	path = filepath.FromSlash(path)
	return strings.HasPrefix(path, "~"+pathSeparator())
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
	width = clampInt(width+24, 120, max(120, a.winW-16))
	left := a.input.Left()
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
		if err := a.drawText(renderer, a.truncateModalListText(row.text, width-20), x+10, rowY+baseline, clr); err != nil {
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
	return len([]rune(s))
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
