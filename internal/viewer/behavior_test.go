package viewer

import (
	"image"
	"image/color"
	"math"
	"path/filepath"
	"reflect"
	"testing"

	"gopdf/internal/config"
	"gopdf/internal/mupdf"

	"github.com/jupiterrider/purego-sdl3/sdl"
	"golang.org/x/image/font/basicfont"
)

func TestSearchPageOrderWrapsFromStartPage(t *testing.T) {
	tests := []struct {
		name  string
		start int
		count int
		want  []int
	}{
		{name: "empty", start: 0, count: 0, want: []int{}},
		{name: "from middle", start: 2, count: 5, want: []int{2, 3, 4, 0, 1}},
		{name: "negative start clamps", start: -4, count: 3, want: []int{0, 1, 2}},
		{name: "too large start clamps", start: 9, count: 4, want: []int{3, 0, 1, 2}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := searchPageOrder(tt.start, tt.count); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("searchPageOrder(%d, %d) = %v, want %v", tt.start, tt.count, got, tt.want)
			}
		})
	}
}

func TestSearchStatusStrings(t *testing.T) {
	app := &App{}
	if got := app.searchStatusMessage(); got != "" {
		t.Fatalf("expected empty message without query, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "" {
		t.Fatalf("expected empty counter without query, got %q", got)
	}

	app.search = searchState{query: "needle", current: -1}
	if got := app.searchStatusMessage(); got != "search /needle" {
		t.Fatalf("expected pending search message, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "" {
		t.Fatalf("expected empty counter before current match, got %q", got)
	}

	app.search = searchState{query: "needle", current: 1, order: []searchHitRef{{page: 0}, {page: 2}, {page: 4}}}
	if got := app.searchStatusMessage(); got != "match 2/3 /needle" {
		t.Fatalf("expected active match message, got %q", got)
	}
	if got := app.searchStatusCounter(); got != "[2/3]" {
		t.Fatalf("expected active match counter, got %q", got)
	}
}

func TestFormatStatusBarReplacesTemplateTokens(t *testing.T) {
	app := &App{
		config:     config.Default(),
		message:    "ready",
		page:       2,
		pageCount:  9,
		renderMode: "continuous",
		fitMode:    "width",
		rotation:   90,
		zoom:       1.25,
		docName:    "paper.pdf",
		search:     searchState{query: "term", current: 0, order: []searchHitRef{{page: 2}}},
	}

	got := app.formatStatusBar("{document} {message} {page}/{total} {mode} {fit} {rot} {zoom} {dual} {cover} {search} $$")
	want := "paper.pdf ready 3/9 continuous width 90 125% single flat [1/1] $"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestFormatStatusBarUsesInputModeMessage(t *testing.T) {
	tests := []struct {
		name  string
		mode  mode
		input string
		back  bool
		tmpl  string
		want  string
	}{
		{name: "command", mode: modeCommand, input: "open file.pdf", tmpl: "{message}|{input}", want: ":open file.pdf|open file.pdf"},
		{name: "goto", mode: modeGotoPage, input: "12", tmpl: "{message}|{input}", want: " GOTO 12|12"},
		{name: "forward search", mode: modeSearch, input: "abc", tmpl: "{message}|{input}|{prompt}", want: "/abc|abc|/"},
		{name: "backward search", mode: modeSearch, input: "abc", back: true, tmpl: "{message}|{input}|{prompt}", want: "?abc|abc|?"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := &App{mode: tt.mode, input: tt.input}
			if tt.back {
				app.searchInput = searchModeBackward
			}
			if got := app.formatStatusBar(tt.tmpl); got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}

func TestFormatStatusBarShowsDualPageSpread(t *testing.T) {
	app := &App{
		page:      1,
		pageCount: 4,
		dualPage:  true,
		rows: []rowLayout{
			{pages: []int{0}},
			{pages: []int{1, 2}},
			{pages: []int{3}},
		},
		pageToRow: []int{0, 1, 1, 2},
	}

	if got := app.formatStatusBar("{page}/{total}"); got != "2-3/4" {
		t.Fatalf("expected dual-page spread in status, got %q", got)
	}
}

func TestNormalizeAndTokenizeBindings(t *testing.T) {
	tests := []struct {
		binding string
		want    string
	}{
		{binding: "gg", want: "g g"},
		{binding: "<Space>", want: " "},
		{binding: "<Return>", want: "<cr>"},
		{binding: "< C-S-Tab >x", want: "<c-s-tab> x"},
	}

	for _, tt := range tests {
		if got := normalizeBinding(tt.binding); got != tt.want {
			t.Fatalf("normalizeBinding(%q) = %q, want %q", tt.binding, got, tt.want)
		}
	}
}

func TestKeyTokenIncludesModifiedAndPrintableKeys(t *testing.T) {
	tests := []struct {
		name string
		key  sdl.Keycode
		mod  sdl.Keymod
		want string
	}{
		{name: "ctrl letter", key: sdl.KeycodeJ, mod: sdl.KeymodCtrl, want: "<c-j>"},
		{name: "ctrl shift special", key: sdl.KeycodeTab, mod: sdl.KeymodCtrl | sdl.KeymodShift, want: "<c-s-tab>"},
		{name: "shift slash", key: sdl.KeycodeSlash, mod: sdl.KeymodShift, want: "?"},
		{name: "shift return", key: sdl.KeycodeReturn, mod: sdl.KeymodShift, want: "<s-cr>"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := keyToken(tt.key, tt.mod)
			if !ok || got != tt.want {
				t.Fatalf("keyToken(%v, %v) = %q, %v; want %q, true", tt.key, tt.mod, got, ok, tt.want)
			}
		})
	}
}

func TestMouseButtonHelpers(t *testing.T) {
	if got, ok := mouseButtonEvent(uint8(sdl.ButtonX1), sdl.EventMouseButtonDown); !ok || got != "x1_down" {
		t.Fatalf("expected x1_down event, got %q, %v", got, ok)
	}
	if got, ok := mouseButtonEvent(uint8(sdl.ButtonRight), sdl.EventMouseButtonUp); !ok || got != "right_up" {
		t.Fatalf("expected right_up event, got %q, %v", got, ok)
	}
	if got, ok := mouseButtonEvent(99, sdl.EventMouseButtonDown); ok || got != "" {
		t.Fatalf("expected unknown button to fail, got %q, %v", got, ok)
	}
	if got := buttonMask(99); got != 0 {
		t.Fatalf("expected unknown button mask 0, got %d", got)
	}
}

func TestVisibleOutlineIndicesRespectExpandedAncestors(t *testing.T) {
	app := &App{
		outline: []mupdf.OutlineItem{
			{Title: "Chapter 1", Page: 0, Parent: -1, HasChildren: true},
			{Title: "Section 1.1", Page: 1, Parent: 0, HasChildren: true},
			{Title: "Topic 1.1.1", Page: 2, Parent: 1},
			{Title: "Chapter 2", Page: 5, Parent: -1},
		},
		outlineMenu: outlineMenuState{expanded: map[int]bool{}},
	}

	if got, want := app.visibleOutlineIndices(), []int{0, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected collapsed top-level outline %v, got %v", want, got)
	}
	app.outlineMenu.expanded[0] = true
	if got, want := app.visibleOutlineIndices(), []int{0, 1, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected first level expanded outline %v, got %v", want, got)
	}
	app.outlineMenu.expanded[1] = true
	if got, want := app.visibleOutlineIndices(), []int{0, 1, 2, 3}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected nested expanded outline %v, got %v", want, got)
	}
}

func TestOutlineIndexForPageUsesNearestPreviousDestination(t *testing.T) {
	app := &App{outline: []mupdf.OutlineItem{
		{Title: "Intro", Page: 0},
		{Title: "Chapter", Page: 5},
		{Title: "Appendix", Page: 10},
	}}

	tests := []struct {
		page int
		want int
	}{
		{page: 0, want: 0},
		{page: 4, want: 0},
		{page: 5, want: 1},
		{page: 9, want: 1},
		{page: 99, want: 2},
	}
	for _, tt := range tests {
		if got := app.outlineIndexForPage(tt.page); got != tt.want {
			t.Fatalf("outlineIndexForPage(%d) = %d, want %d", tt.page, got, tt.want)
		}
	}
}

func TestRunCommandValidationPaths(t *testing.T) {
	app := testLayoutApp(3)
	app.runCommand(":page")
	if app.message != "usage: :page <n>" {
		t.Fatalf("expected page usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":page nope")
	if app.message != "invalid page: nope" {
		t.Fatalf("expected invalid page message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":mode")
	if app.message != "usage: :mode continuous|single" {
		t.Fatalf("expected mode usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":colors")
	if app.message != "usage: :colors normal|alt" {
		t.Fatalf("expected colors usage message, got %q", app.message)
	}

	app.message = ""
	app.runCommand(":wat")
	if app.message != "unknown command: wat" {
		t.Fatalf("expected unknown command message, got %q", app.message)
	}

	app.runCommand(":quit")
	if !app.quit {
		t.Fatal("expected quit command to set quit flag")
	}
}

func TestRunCommandAppliesViewerSettings(t *testing.T) {
	app := testLayoutApp(4)
	app.winW = 800
	app.winH = 600
	app.recomputeLayout(app.viewportSize())
	app.page = 2

	app.runCommand(":mode single")
	if app.renderMode != "single" || app.page != 2 {
		t.Fatalf("expected :mode single to preserve page 2, mode=%q page=%d", app.renderMode, app.page)
	}

	app.runCommand(":mode sideways")
	if app.renderMode != "continuous" {
		t.Fatalf("expected invalid render mode to fall back to continuous, got %q", app.renderMode)
	}

	app.runCommand(":fit width")
	if app.fitMode != "width" {
		t.Fatalf("expected :fit width to set fit mode, got %q", app.fitMode)
	}

	app.runCommand(":fit unknown")
	if app.fitMode != "page" {
		t.Fatalf("expected invalid fit mode to fall back to page, got %q", app.fitMode)
	}

	app.runCommand(":colors alt")
	if !app.altColors {
		t.Fatal("expected :colors alt to enable alternate colors")
	}

	app.runCommand(":colors normal")
	if app.altColors {
		t.Fatal("expected :colors normal to disable alternate colors")
	}
}

func TestRunCommandSearchAndOpenMessages(t *testing.T) {
	app := &App{}
	app.runCommand(":search needle")
	if app.message != "no document open" {
		t.Fatalf("expected search without document message, got %q", app.message)
	}

	app.runCommand(":open")
	if app.message != "usage: :open <filename>" {
		t.Fatalf("expected open usage message, got %q", app.message)
	}

	app.runCommand(":help")
	if app.message != commandHelpMessage() {
		t.Fatalf("expected help command to show command help, got %q", app.message)
	}
}

func TestResolveOpenPathReturnsAbsolutePath(t *testing.T) {
	want, err := filepath.Abs("test.txt")
	if err != nil {
		t.Fatal(err)
	}

	app := &App{}
	if got := app.resolveOpenPath("test.txt"); got != want {
		t.Fatalf("expected absolute path %q, got %q", want, got)
	}
}

func TestResolveOpenPathUsesCurrentDocumentDirectory(t *testing.T) {
	dir := t.TempDir()
	app := &App{docPath: filepath.Join(dir, "paper.pdf")}
	want := filepath.Join(dir, "other.pdf")

	if got := app.resolveOpenPath("other.pdf"); got != want {
		t.Fatalf("expected path relative to document directory %q, got %q", want, got)
	}
}

func TestHandleDroppedFileQueuesOpenWithoutRuntime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "dropped.pdf")
	app := &App{}

	app.handleDroppedFile(path)

	if app.pendingOpen != path {
		t.Fatalf("expected dropped path %q to be queued, got %q", path, app.pendingOpen)
	}
	if !app.quit {
		t.Fatal("expected drop open without runtime to quit for restart")
	}
}

func TestMoveSearchReportsExpectedState(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(800, 600)

	app.moveSearch(1)
	if app.message != "no active search" {
		t.Fatalf("expected no active search message, got %q", app.message)
	}

	app.search = searchState{query: "needle", running: true}
	app.moveSearch(1)
	if app.message != "searching /needle" {
		t.Fatalf("expected running search message, got %q", app.message)
	}

	app.search = searchState{query: "needle"}
	app.moveSearch(1)
	if app.message != "no matches for /needle" {
		t.Fatalf("expected no matches message, got %q", app.message)
	}

	app.search = searchState{query: "needle", current: -1, order: []searchHitRef{{page: 0}, {page: 1}, {page: 2}}}
	app.moveSearch(-1)
	if app.search.current != 2 || app.message != "match 3/3 /needle" {
		t.Fatalf("expected backward move to wrap to last match, current=%d message=%q", app.search.current, app.message)
	}

	app.moveSearch(1)
	if app.search.current != 0 || app.message != "match 1/3 /needle" {
		t.Fatalf("expected forward move to wrap to first match, current=%d message=%q", app.search.current, app.message)
	}
}

func TestRepeatSearchHonorsOriginalSearchDirection(t *testing.T) {
	app := testLayoutApp(3)
	app.recomputeLayout(800, 600)
	app.search = searchState{query: "needle", mode: searchModeBackward, current: 1, order: []searchHitRef{{page: 0}, {page: 1}, {page: 2}}}

	app.repeatSearch(true)
	if app.search.current != 0 {
		t.Fatalf("expected repeating backward search to move backward, got current=%d", app.search.current)
	}

	app.repeatSearch(false)
	if app.search.current != 1 {
		t.Fatalf("expected reversing backward search to move forward, got current=%d", app.search.current)
	}
}

func TestClearSearchResetsSearchState(t *testing.T) {
	app := &App{message: "match 1/2 /needle"}
	app.search = searchState{
		query:      "needle",
		matches:    map[int][]mupdf.SearchHit{0: {{}}},
		order:      []searchHitRef{{page: 0}},
		current:    0,
		running:    true,
		generation: 10,
		mode:       searchModeBackward,
	}

	app.clearSearch()

	if app.search.query != "" || len(app.search.matches) != 0 || len(app.search.order) != 0 || app.search.current != -1 || app.search.running {
		t.Fatalf("expected clear search to reset state, got %+v", app.search)
	}
	if app.search.generation != 11 || app.search.mode != searchModeForward || app.message != "" {
		t.Fatalf("expected generation increment, forward mode, and empty message; search=%+v message=%q", app.search, app.message)
	}
}

func TestOutlineSelectionMovementAndExpandCollapse(t *testing.T) {
	app := &App{
		winW:     800,
		winH:     600,
		fontFace: basicfont.Face7x13,
		config:   config.Default(),
		outline: []mupdf.OutlineItem{
			{Title: "Chapter 1", Page: 0, Parent: -1, HasChildren: true},
			{Title: "Section 1.1", Page: 1, Parent: 0},
			{Title: "Chapter 2", Page: 5, Parent: -1},
		},
		outlineMenu: outlineMenuState{selected: 0, expanded: map[int]bool{}},
	}

	app.moveOutlineSelection(1)
	if app.outlineMenu.selected != 2 {
		t.Fatalf("expected collapsed outline to move to next visible top-level item, got %d", app.outlineMenu.selected)
	}

	app.outlineMenu.selected = 0
	app.expandSelectedOutline()
	if !app.outlineMenu.expanded[0] {
		t.Fatal("expected expanding selected parent to reveal children")
	}

	app.moveOutlineSelection(1)
	if app.outlineMenu.selected != 1 {
		t.Fatalf("expected expanded outline to move into child item, got %d", app.outlineMenu.selected)
	}

	app.collapseSelectedOutline()
	if app.outlineMenu.selected != 0 {
		t.Fatalf("expected collapsing child to select its parent, got %d", app.outlineMenu.selected)
	}

	app.collapseSelectedOutline()
	if app.outlineMenu.expanded[0] {
		t.Fatal("expected collapsing expanded parent to hide children")
	}
}

func TestRenderScalePolicy(t *testing.T) {
	if validRenderScale(0) || validRenderScale(math.NaN()) || validRenderScale(math.Inf(1)) {
		t.Fatal("expected zero, NaN, and infinity to be invalid render scales")
	}
	if !validRenderScale(0.5) {
		t.Fatal("expected positive finite scale to be valid")
	}

	app := &App{config: config.Config{RenderOversample: math.NaN()}, minRenderBaseScale: math.NaN()}
	assertClose(t, app.renderScaleFloor(), defaultMinRenderBaseScale)
	assertClose(t, app.renderOversampleFactor(), defaultRenderOversample)
	assertClose(t, app.oversampledRenderScale(math.NaN()), 1)

	app = &App{scale: 1, zoom: 1, fitMode: "manual", config: config.Config{RenderOversample: 1}, minRenderBaseScale: 0.25, renderBaseScale: 2, renderPending: map[string]renderRequest{"old": {page: 1}}}
	if !app.maybeUpgradeRenderScale(4) {
		t.Fatal("expected target above tolerance to upgrade render base scale")
	}
	assertClose(t, app.renderBaseScale, 4.5)
	if app.renderGeneration != 1 || len(app.renderPending) != 0 {
		t.Fatalf("expected upgrade to invalidate render requests, generation=%d pending=%d", app.renderGeneration, len(app.renderPending))
	}

	app.maybeDowngradeRenderScale()
	assertClose(t, app.renderBaseScale, 3)
	if app.renderGeneration != 2 {
		t.Fatalf("expected downgrade to invalidate render requests, generation=%d", app.renderGeneration)
	}
}

func TestRenderScaleForAllowsLowZoomUndersampling(t *testing.T) {
	app := &App{config: config.Config{RenderOversample: 1}, minRenderBaseScale: 0.25, renderBaseScale: 2}

	assertClose(t, app.renderScaleFor(1), 2)
	assertClose(t, app.renderScaleFor(0.2), 0.4)
	assertClose(t, app.renderScaleFor(0.05), 0.25)
}

func TestViewportAndContentOffsets(t *testing.T) {
	app := &App{winW: 800, winH: 600, config: config.Config{StatusBarHeight: 28}, statusBarShown: true}
	w, h := app.viewportSize()
	if w != 800 || h != 572 {
		t.Fatalf("expected status bar to reduce viewport height, got %dx%d", w, h)
	}

	app.statusBarShown = false
	app.mode = modeCommand
	w, h = app.viewportSize()
	if w != 800 || h != 572 {
		t.Fatalf("expected input mode to reserve status bar height, got %dx%d", w, h)
	}

	app.mode = modeNormal
	app.contentW = 400
	app.contentH = 200
	app.renderMode = "single"
	x, y := app.contentViewportOffset()
	assertClose(t, x, 200)
	assertClose(t, y, 200)

	app.renderMode = "continuous"
	x, y = app.contentViewportOffset()
	assertClose(t, x, 200)
	assertClose(t, y, 0)
}

func TestModalListRowAtUsesRowsBelowHeaderOnly(t *testing.T) {
	app := &App{}
	rect := sdl.FRect{X: 10, Y: 20, W: 200, H: 160}

	if _, ok := app.modalListRowAt(rect, 3, 30, 50, 25); ok {
		t.Fatal("expected click in modal header to miss rows")
	}
	if got, ok := app.modalListRowAt(rect, 3, 30, 50, 55); !ok || got != 0 {
		t.Fatalf("expected first row hit, got row=%d ok=%v", got, ok)
	}
	if got, ok := app.modalListRowAt(rect, 3, 30, 50, 115); !ok || got != 2 {
		t.Fatalf("expected third row hit, got row=%d ok=%v", got, ok)
	}
	if _, ok := app.modalListRowAt(rect, 3, 30, 50, 145); ok {
		t.Fatal("expected click below configured rows to miss")
	}
	if _, ok := app.modalListRowAt(rect, 3, 30, 500, 55); ok {
		t.Fatal("expected click outside modal bounds to miss")
	}
}

func TestTransformAndInverseTransformRoundTrip(t *testing.T) {
	for _, rotation := range []float64{0, 45, 90, 180, 270, -90} {
		x, y := transformPoint(12, 34, 1.5, rotation)
		gotX, gotY := inverseTransformPoint(x, y, 1.5, rotation)
		assertClose(t, gotX, 12)
		assertClose(t, gotY, 34)
	}
}

func TestBuiltinPromptActionsEnterExpectedModes(t *testing.T) {
	tests := []struct {
		action     string
		wantMode   mode
		wantSearch searchMode
	}{
		{action: "command_mode", wantMode: modeCommand},
		{action: "goto_page_prompt", wantMode: modeGotoPage},
		{action: "search_prompt", wantMode: modeSearch, wantSearch: searchModeForward},
		{action: "search_prompt_backward", wantMode: modeSearch, wantSearch: searchModeBackward},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			app := &App{input: "stale", inputCursor: 5, searchInput: searchModeBackward}
			if err := app.runBuiltinAction(tt.action); err != nil {
				t.Fatal(err)
			}
			if app.mode != tt.wantMode || app.input != "" || app.inputCursor != 0 {
				t.Fatalf("expected clean input mode %v, got mode=%v input=%q cursor=%d", tt.wantMode, app.mode, app.input, app.inputCursor)
			}
			if tt.wantMode == modeSearch && app.searchInput != tt.wantSearch {
				t.Fatalf("expected search mode %v, got %v", tt.wantSearch, app.searchInput)
			}
		})
	}

	app := &App{}
	if err := app.runBuiltinAction("quit"); err != nil {
		t.Fatal(err)
	}
	if !app.quit {
		t.Fatal("expected builtin quit action to set quit flag")
	}
	if err := app.runBuiltinAction("not_an_action"); err == nil {
		t.Fatal("expected unknown builtin action to return an error")
	}
}

func TestInputEditingKeepsRuneCursorPositions(t *testing.T) {
	app := &App{input: "ab", inputCursor: 1}
	app.insertInputRune('界')
	if app.input != "a界b" || app.inputCursor != 2 {
		t.Fatalf("expected rune inserted at cursor, input=%q cursor=%d", app.input, app.inputCursor)
	}

	app.moveInputCursor(10)
	if app.inputCursor != 3 {
		t.Fatalf("expected cursor to clamp to rune length, got %d", app.inputCursor)
	}
	app.moveInputCursor(-2)
	if app.inputCursor != 1 {
		t.Fatalf("expected cursor to move left by runes, got %d", app.inputCursor)
	}

	app.backspaceInput()
	if app.input != "界b" || app.inputCursor != 0 {
		t.Fatalf("expected backspace to remove previous rune, input=%q cursor=%d", app.input, app.inputCursor)
	}

	app.backspaceInput()
	if app.input != "界b" || app.inputCursor != 0 {
		t.Fatalf("expected backspace at start to be a no-op, input=%q cursor=%d", app.input, app.inputCursor)
	}
}

func TestCommitInputModeExecutesModeSpecificBehavior(t *testing.T) {
	app := testLayoutApp(5)
	app.recomputeLayout(800, 600)
	app.mode = modeGotoPage
	app.input = "3"
	app.inputCursor = 1
	app.commitInputMode()
	if app.mode != modeNormal || app.input != "" || app.inputCursor != 0 || app.page != 2 {
		t.Fatalf("expected goto input to navigate and clear input, mode=%v input=%q cursor=%d page=%d", app.mode, app.input, app.inputCursor, app.page)
	}

	app = &App{mode: modeCommand, input: "quit", inputCursor: 4}
	app.commitInputMode()
	if app.mode != modeNormal || !app.quit {
		t.Fatalf("expected command input to run quit and return to normal, mode=%v quit=%v", app.mode, app.quit)
	}

	app = &App{mode: modeSearch, input: "needle", inputCursor: 6, searchInput: searchModeBackward}
	app.commitInputMode()
	if app.mode != modeNormal || app.search.query != "needle" || app.search.mode != searchModeBackward || app.message != "no document open" {
		t.Fatalf("expected search input to start backward search, mode=%v search=%+v message=%q", app.mode, app.search, app.message)
	}
}

func TestJumpHistoryMovesBackAndForwardBetweenRecordedPositions(t *testing.T) {
	app := testLayoutApp(5)
	app.winW = 100
	app.winH = 100
	app.recomputeLayout(app.viewportSize())

	app.page = 0
	app.scrollX = 0
	app.scrollY = app.rows[0].y
	app.recordJump()
	app.recordJump()
	if len(app.jumpBack) != 1 {
		t.Fatalf("expected duplicate jump positions to be coalesced, got %d", len(app.jumpBack))
	}

	app.page = 2
	app.scrollX = 0
	app.scrollY = app.rows[2].y
	app.recordJump()
	app.page = 4
	app.scrollX = 0
	app.scrollY = app.rows[4].y

	app.jumpBackward()
	if app.page != 2 || app.scrollX != 0 || app.scrollY != app.rows[2].y || len(app.jumpAhead) != 1 {
		t.Fatalf("expected jump back to restore previous position, page=%d scroll=(%.1f,%.1f) ahead=%d", app.page, app.scrollX, app.scrollY, len(app.jumpAhead))
	}

	app.jumpForward()
	if app.page != 4 || app.scrollX != 0 || app.scrollY != app.rows[4].y {
		t.Fatalf("expected jump forward to restore original position, page=%d scroll=(%.1f,%.1f)", app.page, app.scrollX, app.scrollY)
	}
}

func TestLayoutGapAndRotationEdgeCases(t *testing.T) {
	app := &App{config: config.Config{PageGap: 12, PageGapVertical: -1, SpreadGap: 34, PageGapHorizontal: -1}}
	if app.verticalGap() != 12 || app.horizontalGap() != 34 {
		t.Fatalf("expected fallback gaps from page/spread gap, got vertical=%d horizontal=%d", app.verticalGap(), app.horizontalGap())
	}
	app.config.PageGapVertical = 7
	app.config.PageGapHorizontal = 9
	if app.verticalGap() != 7 || app.horizontalGap() != 9 {
		t.Fatalf("expected explicit gaps to win, got vertical=%d horizontal=%d", app.verticalGap(), app.horizontalGap())
	}

	for _, tt := range []struct {
		input float64
		want  float64
	}{
		{input: -90, want: 270},
		{input: 450, want: 90},
		{input: 720, want: 0},
	} {
		assertClose(t, normalizeRotation(tt.input), tt.want)
	}
}

func TestCurrentRowIndexAndCurrentPageFollowScroll(t *testing.T) {
	app := testLayoutApp(4)
	app.winW = 100
	app.winH = 100
	app.config.PageGapVertical = 10
	app.recomputeLayout(app.viewportSize())

	app.renderMode = "continuous"
	app.scrollY = app.rows[2].y
	if got := app.currentRowIndex(); got != 2 {
		t.Fatalf("expected scroll marker in row 2, got row %d", got)
	}
	app.updateCurrentPageFromScroll()
	if app.page != app.rows[2].pages[0] {
		t.Fatalf("expected current page to follow row 2, got page %d", app.page)
	}

	app.renderMode = "single"
	app.page = 3
	if got := app.currentRowIndex(); got != app.pageToRow[3] {
		t.Fatalf("expected single-page row from current page, got %d want %d", got, app.pageToRow[3])
	}
}

func TestCompletionAcceptCloseAndVisibleRows(t *testing.T) {
	app := &App{
		input:       "open par",
		inputCursor: 8,
		config:      config.Config{CompletionMaxItems: 3},
		completion: completionState{
			visible:  true,
			selected: 1,
			start:    5,
			end:      8,
			items: []completionItem{
				{display: "one", value: "one"},
				{display: "paper.pdf", value: "paper.pdf"},
			},
		},
	}
	app.acceptCompletion()
	if app.input != "open paper.pdf" || app.inputCursor != len([]rune("open paper.pdf")) || app.completion.visible || !app.pendingRedraw {
		t.Fatalf("expected selected completion to replace range and close menu, input=%q cursor=%d completion=%+v redraw=%v", app.input, app.inputCursor, app.completion, app.pendingRedraw)
	}

	app.completion = completionState{visible: true, items: []completionItem{{display: "a", value: "a"}}}
	app.pendingRedraw = false
	app.closeCompletion()
	if app.completion.visible || len(app.completion.items) != 0 || !app.pendingRedraw {
		t.Fatalf("expected closeCompletion to clear menu and request redraw, completion=%+v redraw=%v", app.completion, app.pendingRedraw)
	}

	app.completion = completionState{selected: 3, items: []completionItem{{display: "a"}, {display: "b"}, {display: "c"}, {display: "d"}, {display: "e"}}}
	rows := app.visibleCompletionRows()
	if got, want := completionRowTexts(rows), []string{"...", "d", "e"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("expected truncated visible completion rows %v, got %v", want, got)
	}
	if !rows[1].selected {
		t.Fatalf("expected selected completion row to remain marked, rows=%+v", rows)
	}
}

func TestColorHelpersUseNormalAndAltPalettes(t *testing.T) {
	app := &App{config: config.Config{
		Background:        [3]uint8{1, 2, 3},
		PageBackground:    [3]uint8{4, 5, 6},
		Foreground:        [3]uint8{7, 8, 9},
		StatusBarColor:    [3]uint8{10, 11, 12},
		AltBackground:     [3]uint8{13, 14, 15},
		AltPageBackground: [3]uint8{16, 17, 18},
		AltForeground:     [3]uint8{19, 20, 21},
		AltStatusBarColor: [3]uint8{22, 23, 24},
	}}

	if app.statusVisible() {
		t.Fatal("expected status bar hidden in normal mode unless explicitly shown")
	}
	app.mode = modeCommand
	if !app.statusVisible() {
		t.Fatal("expected input mode to show status bar")
	}
	app.mode = modeNormal

	assertColor(t, app.backgroundColor(), color.RGBA{R: 1, G: 2, B: 3, A: 0xff})
	assertColor(t, app.pageBackgroundColor(), color.RGBA{R: 4, G: 5, B: 6, A: 0xff})
	assertColor(t, app.foregroundColor(), color.RGBA{R: 7, G: 8, B: 9, A: 0xff})
	assertColor(t, app.statusBarColor(), color.RGBA{R: 10, G: 11, B: 12, A: 0xff})

	app.altColors = true
	assertColor(t, app.backgroundColor(), color.RGBA{R: 13, G: 14, B: 15, A: 0xff})
	assertColor(t, app.pageBackgroundColor(), color.RGBA{R: 16, G: 17, B: 18, A: 0xff})
	assertColor(t, app.foregroundColor(), color.RGBA{R: 19, G: 20, B: 21, A: 0xff})
	assertColor(t, app.statusBarColor(), color.RGBA{R: 22, G: 23, B: 24, A: 0xff})
}

func TestRemapPageColorsPreservesAlphaAndMapsLuminance(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 3, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 0, G: 0, B: 0, A: 0xff})
	img.SetRGBA(1, 0, color.RGBA{R: 255, G: 255, B: 255, A: 0xff})
	img.SetRGBA(2, 0, color.RGBA{R: 30, G: 40, B: 50, A: 0})

	remapPageColors(img, [3]uint8{200, 210, 220}, [3]uint8{10, 20, 30})

	assertColor(t, img.RGBAAt(0, 0), color.RGBA{R: 10, G: 20, B: 30, A: 0xff})
	assertColor(t, img.RGBAAt(1, 0), color.RGBA{R: 200, G: 210, B: 220, A: 0xff})
	assertColor(t, img.RGBAAt(2, 0), color.RGBA{R: 30, G: 40, B: 50, A: 0})
}

func completionRowTexts(rows []completionRow) []string {
	texts := make([]string, len(rows))
	for i, row := range rows {
		texts[i] = row.text
	}
	return texts
}

func assertColor(t *testing.T, got, want color.RGBA) {
	t.Helper()
	if got != want {
		t.Fatalf("got color %+v, want %+v", got, want)
	}
}
