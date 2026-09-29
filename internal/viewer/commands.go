package viewer

import (
	"fmt"
	"maps"
	"strconv"
	"strings"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/filepicker"

	"github.com/jupiterrider/purego-sdl3/sdl"
)

func (a *App) pushToken(token string) {
	a.sequence = append(a.sequence, token)
	a.sequenceAt = time.Now()
	a.wakeAfter(time.Duration(a.config.SequenceTimeoutMS) * time.Millisecond)
	for len(a.sequence) > 0 {
		joined := strings.Join(a.sequence, " ")
		cmd, exact := a.sequenceLookup[joined]
		prefix := a.hasPrefix(joined)
		if exact && !prefix {
			a.sequence = nil
			a.runAction(cmd)
			return
		}
		if exact && prefix {
			return
		}
		if prefix {
			return
		}
		if len(a.sequence) == 1 {
			a.sequence = nil
			return
		}
		a.sequence = a.sequence[1:]
	}
}

func (a *App) expireSequence() {
	if len(a.sequence) == 0 {
		return
	}
	if time.Since(a.sequenceAt) < time.Duration(a.config.SequenceTimeoutMS)*time.Millisecond {
		return
	}
	joined := strings.Join(a.sequence, " ")
	if cmd, ok := a.sequenceLookup[joined]; ok {
		a.sequence = nil
		a.runAction(cmd)
		return
	}
	a.sequence = nil
}

func (a *App) hasPrefix(joined string) bool {
	for key := range a.sequenceLookup {
		if key != joined && strings.HasPrefix(key, joined+" ") {
			return true
		}
	}
	return false
}

func (a *App) runAction(action string) {
	// The overview and presentation modes reinterpret actions only while
	// they have the keyboard, not while a prompt or menu opened over them.
	inFront := a.mode == modeNormal && a.activeModalUIView() == nil
	if inFront && a.overview != nil && a.runOverviewAction(action) {
		return
	}
	if inFront && a.presentation != nil && a.runPresentationAction(action) {
		return
	}
	if handled, dirty, err := a.runtime.RunAction(action); handled {
		if err != nil {
			a.message = err.Error()
			return
		}
		if dirty {
			a.applyRuntimeChanges("action")
		}
		return
	}
	if err := a.runBuiltinAction(action); err != nil {
		a.message = err.Error()
	}
}

func (a *App) runActionFrom(source smoothInputSource, action string) {
	previous := a.inputSource
	a.inputSource = source
	defer func() { a.inputSource = previous }()
	a.runAction(action)
}

func (a *App) closeAllUI() {
	a.closeAllUIWithCallbacks(true)
}

func (a *App) closeAllUIWithCallbacks(callCallbacks bool) {
	a.closeAllUIViews(callCallbacks)
	if a.mode != modeNormal {
		a.closeCompletion()
		if a.mode == modePassword {
			a.passwordPrompt = pendingPasswordPrompt{}
		}
		a.mode = modeNormal
		a.input.Reset()
		a.ignoreText = ""
	}
}

func (a *App) closeActiveUI() {
	if view := a.activeModalUIView(); view != nil {
		a.closeUIView(view, true)
		return
	}
	if a.mode != modeNormal {
		if a.completion.view != nil && a.completion.view.visible {
			a.closeCompletion()
			return
		}
		if a.mode == modePassword {
			a.passwordPrompt = pendingPasswordPrompt{}
		}
		a.mode = modeNormal
		a.input.Reset()
		a.ignoreText = ""
		return
	}
	if a.search.query != "" || len(a.search.order) > 0 || a.search.running {
		a.clearSearch()
		return
	}
	if !a.selection.empty() {
		a.clearSelection()
		return
	}
	a.sequence = nil
	a.pendingCount = ""
}

func (a *App) ExecuteAction(action string) error { return a.runBuiltinAction(action) }

func (a *App) PickDirectory() (string, error) {
	return filepicker.PickDirectory()
}

func (a *App) Page() int { return a.page + 1 }

func (a *App) PageCount() int { return a.pageCount }

func (a *App) GotoPage(page int) error {
	if a.pageCount == 0 {
		return nil
	}
	a.alignPageToAnchor(clampInt(page-1, 0, a.pageCount-1))
	return nil
}

func (a *App) GotoDocumentPoint(page int, x, y float64) error {
	if a.pageCount == 0 {
		return nil
	}
	if page < 1 || page > a.pageCount {
		return fmt.Errorf("page %d out of range", page)
	}
	a.alignPageToDocumentPoint(page-1, x, y)
	a.pendingRedraw = true
	return nil
}

func (a *App) Message() string { return a.message }

func (a *App) SetMessage(message string) { a.message = message }

func (a *App) RunCommand(command string) error {
	a.runCommand(command)
	return nil
}

func (a *App) applyConfigState(cfg config.Config, preserveManualFit bool) {
	currentFitMode := a.fitMode
	a.config = cfg
	a.cancelSmoothZoom()
	a.cancelSmoothScroll()
	a.fitMode = sanitizeFitMode(cfg.FitMode)
	if preserveManualFit && currentFitMode == "manual" {
		a.fitMode = currentFitMode
	}
	a.renderMode = sanitizeRenderMode(cfg.RenderMode)
	a.zoom = a.clampZoom(a.zoom)
	a.cache.byteLimit = pageCacheByteLimit(cfg)
	a.altColors = cfg.AltColors
	a.dualPage = cfg.DualPage
	a.trimMargins = cfg.TrimMargins
	a.firstPageOffset = cfg.FirstPageOffset
	a.statusBarShown = cfg.StatusBarVisible
	a.sequenceLookup = map[string]string{}
	a.mouseBindings = map[string]string{}
	for k, v := range cfg.KeyBindings {
		a.sequenceLookup[normalizeBinding(k)] = v
	}
	maps.Copy(a.mouseBindings, cfg.MouseBindings)
	a.pageStep = float64(cfg.ScrollStep)
	oldFontFace := a.fontFace
	a.fontFace = loadFont(cfg.UIFontPath, cfg.UIFontSize)
	a.clearTextTextureCache()
	a.cache.evict()
	closeFontFace(oldFontFace)
}

func (a *App) Mode() string {
	switch a.mode {
	case modeCommand:
		return "command"
	case modeGotoPage:
		return "goto"
	case modeSearch:
		return "search"
	default:
		return "normal"
	}
}

func (a *App) Search(query string, backward bool) error {
	mode := searchModeForward
	if backward {
		mode = searchModeBackward
	}
	a.startSearch(query, mode)
	return nil
}

func (a *App) SearchQuery() string { return a.search.query }

func (a *App) SearchMatchCount() int { return len(a.search.order) }

func (a *App) SearchMatchIndex() int {
	if a.search.current < 0 || a.search.current >= len(a.search.order) {
		return 0
	}
	return a.search.current + 1
}

func (a *App) CurrentCount() string { return a.pendingCount }

func (a *App) PendingKeys() []string { return append([]string(nil), a.sequence...) }

func (a *App) ClearPendingKeys() {
	a.sequence = nil
	a.pendingMark = ""
	a.pendingCount = ""
	if a.mode == modeNormal {
		a.message = ""
	}
}

func (a *App) FitMode() string { return a.fitMode }

func (a *App) SetFitMode(mode string) error {
	a.setFitMode(sanitizeFitMode(mode))
	return nil
}

func (a *App) RenderMode() string { return a.renderMode }

func (a *App) SetRenderMode(mode string) error {
	mode = sanitizeRenderMode(mode)
	if a.renderMode == mode {
		return nil
	}
	a.relayoutWithViewportAnchor(func() { a.renderMode = mode })
	return nil
}

func (a *App) Zoom() float64 { return a.scale }

func (a *App) SetZoom(zoom float64) error {
	if zoom <= 0 {
		return fmt.Errorf("zoom must be positive")
	}
	a.cancelSmoothZoom()
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = "manual"
		a.zoom = a.clampZoom(zoom)
		a.scheduleRenderScaleTarget(a.zoom)
	})
	return nil
}

func (a *App) Rotation() float64 { return normalizeRotation(a.rotation) }

func (a *App) SetRotation(rotation float64) error {
	a.relayoutWithViewportAnchor(func() {
		a.rotation = normalizeRotation(rotation)
		a.updatePageMetricSizes()
	})
	return nil
}

func (a *App) Fullscreen() bool { return a.fullscreen }

func (a *App) SetFullscreen(fullscreen bool) error {
	a.fullscreen = fullscreen
	if a.window == nil {
		return nil
	}
	if fullscreen {
		return renderBool(sdl.SetWindowFullscreen(a.window, true), "set fullscreen")
	}
	return renderBool(sdl.SetWindowFullscreen(a.window, false), "set fullscreen")
}

func (a *App) StatusBarVisible() bool { return a.statusBarShown }

func (a *App) SetStatusBarVisible(visible bool) error {
	if a.statusBarShown == visible {
		return nil
	}
	a.relayoutWithViewportAnchor(func() { a.statusBarShown = visible })
	return nil
}

func (a *App) CacheEntries() int { return len(a.cache.entries) }

func (a *App) CachePending() int { return len(a.renderPending) }

// CacheLimit is the render cache memory limit in MiB; 0 means unlimited.
func (a *App) CacheLimit() int { return int(a.cache.byteLimit >> 20) }

func (a *App) SetCacheLimit(mib int) error {
	if mib < 0 {
		return fmt.Errorf("cache limit must not be negative")
	}
	a.cache.byteLimit = int64(mib) << 20
	a.cache.evict()
	return nil
}

func (a *App) ClearCache() { a.clearCache() }

func (a *App) gotoPageInput(input string) {
	page, ok := a.resolvePageInput(input)
	if !ok {
		a.message = fmt.Sprintf("invalid page or label: %s", strings.TrimSpace(input))
		return
	}
	a.alignPageToAnchor(page)
}

func (a *App) resolvePageInput(input string) (int, bool) {
	input = strings.TrimSpace(input)
	for page, metric := range a.pageMetrics {
		if metric.label != "" && strings.EqualFold(metric.label, input) {
			return page, true
		}
	}
	n, err := strconv.Atoi(input)
	if err != nil {
		return 0, false
	}
	return clampInt(n-1, 0, a.pageCount-1), true
}

func (a *App) runCommand(input string) {
	command := strings.TrimPrefix(strings.TrimSpace(input), ":")
	command = strings.TrimSpace(command)
	if _, err := strconv.Atoi(command); err == nil {
		a.gotoPageInput(command)
		return
	}
	name, args, _ := strings.Cut(command, " ")
	args = strings.TrimSpace(args)
	fields := strings.Fields(args)
	if name == "" || a.runActionCommand(name, args) {
		return
	}
	switch name {
	case "q", "quit":
		a.quit = a.confirmDiscard(":q")
	case "q!", "quit!":
		a.quit = true
	case "print":
		a.printDocument(args)
	case "undo":
		a.undoEdit(false)
	case "redo":
		a.undoEdit(true)
	case "highlight":
		a.highlightSelection(a.highlightColor)
	case "w", "write":
		a.writeDocument(args)
	case "wq":
		a.writeDocument(args)
		a.quit = !a.unsaved
	case "page", "p":
		if len(fields) < 1 {
			a.message = "usage: :page <n>"
			return
		}
		a.gotoPageInput(fields[0])
	case "set":
		a.runSet(args)
	case "mode":
		if len(fields) < 1 {
			a.message = "usage: :mode continuous|single"
			return
		}
		if err := a.SetRenderMode(fields[0]); err != nil {
			a.message = err.Error()
		}
	case "colors":
		if len(fields) < 1 {
			a.message = "usage: :colors normal|alt"
			return
		}
		a.setAltColors(strings.EqualFold(fields[0], "alt"))
	case "fit":
		if len(fields) < 1 {
			return
		}
		if err := a.SetFitMode(fields[0]); err != nil {
			a.message = err.Error()
		}
	case "reload-config":
		a.reloadConfig()
	case "keybinds":
		a.toggleKeybindMenu()
	case "matches":
		a.showSearchMatches()
	case "search":
		a.startSearch(args, searchModeForward)
	case "open":
		if args == "" {
			a.message = "usage: :open <filename>"
			return
		}
		if err := a.Open(unescapeCommandArg(args)); err != nil {
			a.message = err.Error()
		}
	case "open_file_picker":
		if err := a.runBuiltinAction("open_file_picker"); err != nil {
			a.message = err.Error()
		}
	case "recent":
		a.showRecentFiles()
	case "lua":
		if a.runtime == nil {
			a.message = "no Lua runtime"
			return
		}
		dirty, err := a.runtime.Eval(args)
		if err != nil {
			a.message = err.Error()
			return
		}
		if dirty {
			a.applyRuntimeChanges("command")
		}
	case "help":
		a.toggleHelp()
	default:
		if a.runtime != nil {
			if handled, err := a.runtime.RunPluginCommand(name, args); handled {
				if err != nil {
					a.message = err.Error()
				}
				a.applyRuntimeChanges("command")
				return
			}
		}
		a.message = "unknown command: " + name
	}
}

func unescapeCommandArg(arg string) string {
	if !strings.Contains(arg, `\`) {
		return arg
	}
	var b strings.Builder
	b.Grow(len(arg))
	for i := 0; i < len(arg); i++ {
		if arg[i] == '\\' && i+1 < len(arg) && (arg[i+1] == ' ' || arg[i+1] == '\\') {
			b.WriteByte(arg[i+1])
			i++
			continue
		}
		b.WriteByte(arg[i])
	}
	return b.String()
}

func (a *App) reloadConfig() {
	oldActive := a.activeUIView()
	err := a.runtime.Reload()
	a.removeStaleLuaViews(a.runtime.Generation())
	if err != nil {
		if oldActive != nil && a.views.views[oldActive.id] == oldActive {
			a.showUIView(oldActive)
		}
		a.message = err.Error()
		return
	}
	cfg := a.runtime.Config()
	a.applyConfig(cfg)
	a.emitPluginEvent("config_reloaded", a.documentEventPayload())
	a.message = boolWord(cfg.ConfigPath != "", "config reloaded", "defaults reloaded")
}

func (a *App) showRecentFiles() {
	if !a.config.SessionDatabase {
		a.message = "session database disabled"
		return
	}
	paths := config.RecentFiles(a.config.RecentFilesMax)
	if len(paths) == 0 {
		a.message = "no recent files"
		return
	}
	a.closeAllUI()
	a.showCoreList("recent-files", "Recent Files", paths, func(path string) {
		a.CloseUI("recent-files")
		if err := a.Open(path); err != nil {
			a.message = err.Error()
		}
	})
}

func (a *App) runSet(input string) {
	if a.runtime == nil {
		a.message = "no config runtime"
		return
	}
	input = strings.TrimSpace(input)
	if input == "" {
		names := config.OptionNames()
		if a.runtime != nil {
			names = a.runtime.OptionNames()
		}
		rows := make([]string, 0, len(names))
		for _, name := range names {
			value, _ := a.runtime.OptionValue(name)
			rows = append(rows, name+"="+value)
		}
		a.closeAllUI()
		a.showCoreList("options", "Options", rows, nil)
		return
	}
	if name, value, ok := strings.Cut(input, "="); ok {
		if err := a.runtime.SetOption(name, value); err != nil {
			a.message = err.Error()
			return
		}
		a.applyRuntimeChanges("set")
		name = strings.TrimSpace(name)
		current, _ := a.runtime.OptionValue(name)
		a.message = name + "=" + current
		return
	}
	if name, ok := strings.CutSuffix(input, "!"); ok {
		if err := a.runtime.ToggleOption(name); err != nil {
			a.message = err.Error()
			return
		}
		a.applyRuntimeChanges("set")
		name = strings.TrimSpace(name)
		current, _ := a.runtime.OptionValue(name)
		a.message = name + "=" + current
		return
	}
	name := strings.TrimSuffix(input, "?")
	value, err := a.runtime.OptionValue(name)
	if err != nil {
		a.message = err.Error()
		return
	}
	a.message = strings.TrimSpace(name) + "=" + value
}

func (a *App) runMouseBinding(event string) bool {
	if action, ok := a.mouseBindings[event]; ok {
		source := a.inputSource
		if source == smoothInputSourceUnset {
			source = smoothInputSourceMouse
		}
		a.runActionFrom(source, action)
		return true
	}
	return false
}

func (a *App) applyConfig(cfg config.Config) {
	a.relayoutWithViewportAnchor(func() {
		a.applyConfigState(cfg, true)
		a.clearCache()
	})
}

func sanitizeFitMode(mode string) string { return config.NormalizeFitMode(mode) }

func sanitizeRenderMode(mode string) string { return config.NormalizeRenderMode(mode) }
