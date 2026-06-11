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
	if handled, dirty, err := a.runtime.RunAction(action); handled {
		if err != nil {
			a.message = err.Error()
			return
		}
		if dirty {
			a.applyConfig(a.runtime.Config())
		}
		return
	}
	if err := a.runBuiltinAction(action); err != nil {
		a.message = err.Error()
	}
}

func (a *App) runBuiltinAction(action string) error {
	switch action {
	case "next_page":
		a.nextPage()
	case "prev_page":
		a.prevPage()
	case "scroll_down":
		a.scrollBy(0, a.pageStep)
	case "scroll_up":
		a.scrollBy(0, -a.pageStep)
	case "scroll_left":
		a.scrollBy(-a.pageStep, 0)
	case "scroll_right":
		a.scrollBy(a.pageStep, 0)
	case "pan":
		if a.actionKey != "" {
			a.panning = true
			a.panKey = a.actionKey
			a.panButton = 0
			return nil
		}
		if a.mouseButton != 0 {
			a.panning = true
			a.panButton = a.mouseButton
			a.panKey = ""
		}
	case "next_spread":
		a.nextSpread()
	case "prev_spread":
		a.prevSpread()
	case "first_page":
		a.alignPageToAnchor(0)
	case "last_page":
		a.alignPageToAnchor(a.pageCount - 1)
	case "command_mode":
		a.closeAllUI()
		a.mode = modeCommand
		a.input.Reset()
	case "search_prompt":
		a.closeAllUI()
		a.mode = modeSearch
		a.searchInput = searchModeForward
		a.input.Reset()
	case "search_prompt_backward":
		a.closeAllUI()
		a.mode = modeSearch
		a.searchInput = searchModeBackward
		a.input.Reset()
	case "goto_page_prompt":
		a.closeAllUI()
		a.mode = modeGotoPage
		a.input.Reset()
	case "search_next":
		a.repeatSearch(true)
	case "search_prev":
		a.repeatSearch(false)
	case "toggle_dual_page":
		a.relayoutWithViewportAnchor(func() { a.dualPage = !a.dualPage })
		a.message = boolWord(a.dualPage, "dual-page on", "dual-page off")
	case "toggle_render_mode":
		a.relayoutWithViewportAnchor(func() {
			if a.renderMode == "single" {
				a.renderMode = "continuous"
			} else {
				a.renderMode = "single"
			}
		})
		a.message = "render mode " + a.renderMode
	case "toggle_alt_colors":
		a.setAltColors(!a.altColors)
		a.message = boolWord(a.altColors, "alt colors on", "alt colors off")
	case "toggle_first_page_offset":
		a.relayoutWithViewportAnchor(func() { a.firstPageOffset = !a.firstPageOffset })
		a.message = boolWord(a.firstPageOffset, "first-page offset on", "first-page offset off")
	case "toggle_status_bar":
		a.relayoutWithViewportAnchor(func() { a.statusBarShown = !a.statusBarShown })
	case "toggle_fullscreen":
		a.fullscreen = !a.fullscreen
		a.SetFullscreen(a.fullscreen)
	case "outline":
		a.toggleOutlineMenu()
	case "keybinds":
		a.toggleKeybindMenu()
	case "confirm":
		if a.completion.visible {
			a.acceptCompletion()
		} else if a.outlineMenu.visible {
			a.activateSelectedOutline()
		} else if a.mode != modeNormal {
			a.commitInputMode()
		}
	case "zoom_in":
		a.setManualZoom(1.15)
	case "zoom_out":
		a.setManualZoom(1 / 1.15)
	case "reset_zoom":
		a.zoom = 1
		a.setFitMode("manual")
	case "fit_width":
		a.setFitMode("width")
	case "fit_page":
		a.setFitMode("page")
	case "reload_config":
		a.reloadConfig()
	case "rotate_cw":
		a.relayoutWithViewportAnchor(func() {
			a.rotation = normalizeRotation(a.rotation + 90)
			a.updatePageMetricSizes()
		})
	case "rotate_ccw":
		a.relayoutWithViewportAnchor(func() {
			a.rotation = normalizeRotation(a.rotation + 270)
			a.updatePageMetricSizes()
		})
	case "quit":
		a.quit = true
	case "close":
		a.closeActiveUI()
	case "show_completion":
		a.showCompletion()
	case "next_completion":
		a.moveCompletion(1)
	case "prev_completion":
		a.moveCompletion(-1)
	case "jump_forward":
		a.jumpForward()
	case "jump_backward":
		a.jumpBackward()
	case "open_file_picker":
		path, err := filepicker.PickPDF()
		if err != nil {
			return err
		}
		if path != "" {
			return a.Open(path)
		}
	case "clear_search":
		a.clearSearch()
	default:
		return fmt.Errorf("unknown action: %s", action)
	}
	return nil
}

func (a *App) closeAllUI() {
	a.luaUI.visible = false
	a.keybindMenu.visible = false
	a.outlineMenu.visible = false
	if a.mode != modeNormal {
		a.closeCompletion()
		if a.mode == modePassword {
			a.passwordPrompt = pendingPasswordPrompt{}
		}
		a.mode = modeNormal
		a.input.Reset()
		a.ignoreText = ""
	}
	if a.search.query != "" || len(a.search.order) > 0 || a.search.running {
		a.clearSearch()
	}
}

func (a *App) closeActiveUI() {
	if a.luaUI.visible {
		a.closeLuaUI(true)
		return
	}
	if a.keybindMenu.visible {
		a.keybindMenu = keybindMenuState{}
		return
	}
	if a.outlineMenu.visible {
		a.outlineMenu.visible = false
		return
	}
	if a.mode != modeNormal {
		if a.completion.visible {
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
	a.sequence = nil
	a.pendingCount = ""
}

func (a *App) ExecuteAction(action string) error { return a.runBuiltinAction(action) }

func (a *App) Page() int { return a.page + 1 }

func (a *App) PageCount() int { return a.pageCount }

func (a *App) GotoPage(page int) error {
	if a.pageCount == 0 {
		return nil
	}
	a.alignPageToAnchor(clampInt(page-1, 0, a.pageCount-1))
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
	a.fitMode = sanitizeFitMode(cfg.FitMode)
	if preserveManualFit && currentFitMode == "manual" {
		a.fitMode = currentFitMode
	}
	a.renderMode = sanitizeRenderMode(cfg.RenderMode)
	a.cacheLimit = pageCacheLimit(cfg, a.pageCount)
	a.altColors = cfg.AltColors
	a.dualPage = cfg.DualPage
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
	a.enforceRenderCacheLimit()
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
	a.relayoutWithViewportAnchor(func() {
		a.fitMode = "manual"
		a.zoom = clampFloat(zoom, 0.05, 8.0)
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

func (a *App) CacheEntries() int { return len(a.renderCache) }

func (a *App) CachePending() int { return len(a.renderPending) }

func (a *App) CacheLimit() int { return a.cacheLimit }

func (a *App) SetCacheLimit(limit int) error {
	if limit < 1 {
		return fmt.Errorf("cache limit must be at least 1")
	}
	a.cacheLimit = limit
	a.enforceRenderCacheLimit()
	return nil
}

func (a *App) ClearCache() { a.clearCache() }

func (a *App) gotoPageInput(input string) {
	n, err := strconv.Atoi(input)
	if err != nil {
		a.message = fmt.Sprintf("invalid page: %s", input)
		return
	}
	a.alignPageToAnchor(clampInt(n-1, 0, a.pageCount-1))
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
	if name == "" {
		return
	}
	switch name {
	case "q", "quit":
		a.quit = true
	case "page", "p":
		if len(fields) < 1 {
			a.message = "usage: :page <n>"
			return
		}
		a.gotoPageInput(fields[0])
	case "set":
		if len(fields) < 1 {
			return
		}
		a.runSet(fields[0])
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
			a.applyConfig(a.runtime.Config())
		}
	case "help":
		a.showCommandHelp()
	default:
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
	if err := a.runtime.Reload(); err != nil {
		a.message = err.Error()
		return
	}
	cfg := a.runtime.Config()
	a.applyConfig(cfg)
	a.message = boolWord(cfg.ConfigPath != "", "config reloaded", "defaults reloaded")
}

func (a *App) showCommandHelp() {
	a.closeAllUI()
	a.luaUI = luaUIState{
		visible: true,
		title:   "Commands",
		rows:    commandHelpRows(),
	}
	a.pendingRedraw = true
}

func (a *App) runSet(setting string) {
	if action, ok := setActionForSetting(setting); ok {
		a.runAction(action)
	} else {
		a.message = "unknown setting: " + setting
	}
}

func (a *App) runMouseBinding(event string) bool {
	if action, ok := a.mouseBindings[event]; ok {
		a.runAction(action)
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
