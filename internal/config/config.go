package config

import lua "github.com/yuin/gopher-lua"

type Config struct {
	ConfigPath           string
	AutogenPath          string
	StatusBarVisible     bool
	RenderMode           string
	RenderOversample     float64
	DualPage             bool
	FirstPageOffset      bool
	FitMode              string
	AnchorPosition       string
	Background           [3]uint8
	PageBackground       [3]uint8
	Foreground           [3]uint8
	StatusBarColor       [3]uint8
	AltBackground        [3]uint8
	AltPageBackground    [3]uint8
	AltForeground        [3]uint8
	AltStatusBarColor    [3]uint8
	HighlightForeground  [3]uint8
	HighlightBackground  [3]uint8
	AltColors            bool
	PageGap              int
	SpreadGap            int
	PageGapVertical      int
	PageGapHorizontal    int
	ScrollStep           int
	StatusBarHeight      int
	StatusBarPadding     int
	UIFontSize           int
	UIFontPath           string
	StatusBarLeft        string
	StatusBarRight       string
	SequenceTimeoutMS    int
	NormalMessage        string
	KeyBindings          map[string]string
	MouseBindings        map[string]string
	MouseTextSelect      bool
	NaturalScroll        bool
	AntiAliasing         int
	OutlineInitialDepth  int
	OutlineWidthPercent  int
	OutlineHeightPercent int
	CompletionMaxItems   int
}

type Runtime struct {
	explicitPath string
	docPath      string
	docName      string
	docMeta      documentMeta
	cfg          Config
	state        *lua.LState
	host         Host
	callbacks    map[string]*lua.LFunction
	callbackSeq  int
	dirty        bool
}

type UIOverlay struct {
	Title    string
	Rows     []string
	Selected int
	OnSelect string
	OnClose  string
}

type documentMeta struct {
	exists    bool
	sizeBytes int64
	ext       string
	pageCount int
	hasPages  bool
}

type Host interface {
	ExecuteAction(action string) error
	Open(path string) error
	ShowUI(overlay UIOverlay) error
	CloseUI()
	UIVisible() bool
	SetUIRows(rows []string)
	SetUISelected(selected int)
	Page() int
	PageCount() int
	GotoPage(page int) error
	Message() string
	SetMessage(message string)
	RunCommand(command string) error
	Mode() string
	Search(query string, backward bool) error
	SearchQuery() string
	SearchMatchCount() int
	SearchMatchIndex() int
	CurrentCount() string
	PendingKeys() []string
	ClearPendingKeys()
	FitMode() string
	SetFitMode(mode string) error
	RenderMode() string
	SetRenderMode(mode string) error
	Zoom() float64
	SetZoom(zoom float64) error
	Rotation() float64
	SetRotation(rotation float64) error
	Fullscreen() bool
	SetFullscreen(fullscreen bool) error
	StatusBarVisible() bool
	SetStatusBarVisible(visible bool) error
	CacheEntries() int
	CachePending() int
	CacheLimit() int
	SetCacheLimit(limit int) error
	ClearCache()
}
