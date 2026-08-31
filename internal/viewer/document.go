package viewer

import (
	"fmt"
	"path/filepath"
	"time"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"
)

type openDocumentOptions struct {
	startPage         int
	startPageExplicit bool
	reloadConfig      bool
	preserveView      *viewState
}

func (a *App) Open(path string) error {
	if path == "" {
		return fmt.Errorf("open: empty path")
	}
	path = a.resolveOpenPath(path)
	a.logf("open requested path=%q", path)
	if a.runtime == nil {
		a.pendingOpen = path
		a.quit = true
		return nil
	}
	state := a.captureViewState().atDocumentStart()
	return a.openDocument(path, openDocumentOptions{reloadConfig: true, preserveView: &state})
}

func (a *App) resolveOpenPath(path string) string {
	path = expandHomePath(path)
	if !filepath.IsAbs(path) {
		if a.docPath != "" {
			dir := filepath.Dir(a.docPath)
			path = filepath.Join(dir, path)
		}
	}
	return config.AbsoluteDocumentPath(path)
}

func (a *App) initMetricLoader(pageCount int, startPage int) {
	a.logf("start metric loader pages=%d startPage=%d", pageCount, startPage+1)
	l := &metricLoader{
		workerLifecycle: newWorkerLifecycle(),
		updates:         make(chan pageMetricUpdate, 128),
	}
	a.metricLoader = l
	go l.run(a.doc, pageCount, startPage)
}

func (a *App) startPendingMetricLoader() {
	if a.metricLoader != nil || !a.pendingLoad || a.pendingPages <= 1 {
		return
	}
	pages := a.pendingPages
	start := a.pendingStart
	a.pendingLoad = false
	a.pendingPages = 0
	a.pendingStart = 0
	a.initMetricLoader(pages, start)
}

func (a *App) pollMetricUpdates() {
	if a.metricLoader == nil {
		return
	}
	changed := false
	anchor := a.captureViewportAnchor()
	for {
		select {
		case update, ok := <-a.metricLoader.updates:
			if !ok {
				return
			}
			if update.err != nil {
				a.message = update.err.Error()
				a.pendingRedraw = true
				continue
			}
			if update.page >= len(a.pageMetrics) {
				continue
			}
			a.pageMetrics[update.page].bounds = update.bounds
			a.pageMetrics[update.page].width = update.width
			a.pageMetrics[update.page].height = update.height
			a.pageMetrics[update.page].label = update.label
			a.pageMetrics[update.page].loaded = true
			changed = true
		default:
			if changed {
				a.recomputeLayout(a.viewportSize())
				a.restoreViewportAnchor(anchor)
				a.pendingRedraw = true
			}
			return
		}
	}
}

func (a *App) openDocument(path string, opts openDocumentOptions) error {
	return a.openDocumentWithPassword(path, opts, "")
}

func (a *App) openDocumentWithPassword(path string, opts openDocumentOptions, password string) error {
	a.message = "opening " + path

	path = config.AbsoluteDocumentPath(path)
	a.logf("opening document path=%q startPage=%d reloadConfig=%t", path, opts.startPage+1, opts.reloadConfig)
	a.emitPluginEvent("document_open_pre", map[string]any{"document": map[string]any{"path": path, "name": filepath.Base(path)}})
	doc, err := mupdf.Open(path, password)
	if err != nil {
		a.logf("open document failed path=%q err=%v", path, err)
		if mupdf.IsPasswordError(err) {
			a.promptDocumentPassword(path, opts)
			return nil
		}
		return err
	}
	pages := doc.CachedPageCount()
	startPage := opts.startPage
	if startPage < 0 {
		startPage = 0
	}
	if pages > 0 && startPage >= pages {
		startPage = pages - 1
	}
	savedState, hasSavedState := a.documentSessionViewState(path)
	if !opts.startPageExplicit && hasSavedState {
		startPage = clampInt(savedState.page, 0, max(0, pages-1))
	}

	oldDocumentPayload := a.documentEventPayload()
	hadDocument := a.doc != nil
	if hadDocument && a.runtime != nil {
		a.emitPluginEvent("document_close_pre", oldDocumentPayload)
	}
	a.saveDocumentSession()
	a.closeDocumentResources()
	if hadDocument && a.runtime != nil {
		a.emitPluginEvent("document_closed", oldDocumentPayload)
	}
	a.runtime.SetPageCount(pages)

	a.document.record(path)
	a.installDocument(doc, path, pages, startPage)
	a.resetForNewDocument(password)

	a.initDocumentMetrics(doc, pages, startPage)
	a.logf("opened document path=%q pages=%d page=%d", path, pages, startPage+1)

	var configErr error
	if opts.reloadConfig {
		configErr = a.runtime.SetDocument(path, pages)
		a.removeStaleLuaViews(a.runtime.Generation())
	}
	a.applyConfigState(a.runtime.Config(), false)
	a.message = a.config.NormalMessage

	a.setWindowTitle()
	a.initRenderWorker()
	a.initSearch()
	a.recomputeLayout(a.viewportSize())
	a.ensureRenderBaseScale()
	if opts.preserveView != nil {
		a.restoreViewState(*opts.preserveView)
	} else if !opts.startPageExplicit && hasSavedState {
		a.restoreViewState(savedState)
	} else {
		a.alignPageToAnchor(startPage)
	}
	a.pendingRedraw = true
	if a.runtime != nil {
		a.emitPluginEvent("document_opened", a.documentEventPayload())
	}
	if configErr != nil {
		a.logf("document config reload failed err=%v", configErr)
		a.message = configErr.Error()
		return configErr
	}
	return nil
}

func (a *App) resetForNewDocument(password string) {
	a.docPassword = password
	a.rotation = 0
	a.zoom = a.clampZoom(1)
	a.scale = 1
	a.scrollX = 0
	a.scrollY = 0
	a.search = searchState{}
	a.outlineMenu = outlineMenuState{}
	a.keybindMenu = keybindMenuState{}
	a.closeAllUIViews(true)
	a.completion = completionState{}
	a.mode = modeNormal
	a.input.Reset()
	a.ignoreText = ""
	a.sequence = nil
	a.pendingCount = ""
	a.jumpBack = nil
	a.jumpAhead = nil
	a.pendingOpen = ""
}

func (a *App) promptDocumentPassword(path string, opts openDocumentOptions) {
	a.closeAllUI()
	a.mode = modePassword
	a.input.Reset()
	a.passwordPrompt = pendingPasswordPrompt{path: path, opts: opts}
	a.message = "password required: " + filepath.Base(path)
	a.pendingRedraw = true
}

func (a *App) submitDocumentPassword(password string) {
	prompt := a.passwordPrompt
	a.passwordPrompt = pendingPasswordPrompt{}
	if prompt.path == "" {
		return
	}
	if err := a.openDocumentWithPassword(prompt.path, prompt.opts, password); err != nil {
		a.message = err.Error()
	}
}

func (a *App) initDocumentMetrics(doc *mupdf.Document, pages int, startPage int) {
	defaultW, defaultH := 612.0, 792.0
	if pages > 0 {
		if bounds, err := doc.Bounds(startPage); err == nil {
			w, h := rotatedBoundsSize(bounds, 0)
			label, _ := doc.PageLabel(startPage)
			a.pageMetrics[startPage] = pageMetrics{bounds: bounds, width: w, height: h, label: label, loaded: true}
			defaultW, defaultH = w, h
		}
	}
	if defaultW == 0 || defaultH == 0 {
		defaultW, defaultH = 612.0, 792.0
	}
	for i := range a.pageMetrics {
		if !a.pageMetrics[i].loaded {
			a.pageMetrics[i].width = defaultW
			a.pageMetrics[i].height = defaultH
		}
	}

	if pages > 1 {
		a.logf("queue metric loader pages=%d startPage=%d", pages, startPage+1)
		a.pendingLoad = true
		a.pendingPages = pages
		a.pendingStart = startPage
	}
}

func (a *App) installDocument(doc *mupdf.Document, path string, pages, startPage int) {
	a.generation++
	a.docPath = path
	a.docName = filepath.Base(path)
	a.recordRecentFile(path)
	a.doc = doc
	a.pageCount = pages
	a.page = startPage
	a.pageMetrics = make([]pageMetrics, pages)
	a.rows = nil
	a.pageToRow = nil
	a.contentW = 0
	a.contentH = 0
	a.cacheLimit = pageCacheLimit(a.config, pages)
	a.renderBaseScale = 0
	a.pageLinks = map[int][]mupdf.Link{}
	a.outline = nil
	a.selection = textSelection{}
}

func (a *App) pollDocumentUpdate() {
	change, ok := a.document.poll(time.Now())
	if !ok {
		return
	}
	a.logf("document changed size=%d mod=%s", change.size, change.mod.Format(time.RFC3339Nano))
	if err := a.reloadUpdatedDocument(change); err != nil {
		a.logf("document reload failed err=%v", err)
		a.message = err.Error()
	}
}

func (a *App) reloadUpdatedDocument(change documentChange) error {
	path := a.docPath
	state := a.captureViewState()
	if err := a.softReloadDocument(path, state); err != nil {
		return err
	}
	a.document.commit(change)
	a.message = "reloaded " + a.docName
	a.pendingRedraw = true
	return nil
}

func (a *App) softReloadDocument(path string, state viewState) error {
	path = config.AbsoluteDocumentPath(path)
	a.logf("soft reload document path=%q", path)
	doc, err := mupdf.Open(path, a.docPassword)
	if err != nil {
		return err
	}
	pages := doc.CachedPageCount()
	startPage := clampInt(state.page, 0, max(0, pages-1))

	if a.runtime != nil {
		a.runtime.SetPageCount(pages)
	}
	a.closeDocumentResources()

	a.installDocument(doc, path, pages, startPage)

	a.initDocumentMetrics(doc, pages, startPage)
	a.logf("soft reloaded document path=%q pages=%d page=%d", path, pages, startPage+1)
	a.setWindowTitle()
	a.initRenderWorker()
	a.initSearch()
	a.recomputeLayout(a.viewportSize())
	a.ensureRenderBaseScale()
	a.restoreViewState(state)
	a.pendingRedraw = true
	if a.runtime != nil {
		a.emitPluginEvent("document_reloaded", a.documentEventPayload())
	}
	return nil
}

func (a *App) documentEventPayload() map[string]any {
	return map[string]any{
		"document": map[string]any{
			"path":       a.docPath,
			"name":       a.docName,
			"page_count": a.pageCount,
		},
		"generation": a.generation,
	}
}
