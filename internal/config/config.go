package config

import lua "github.com/yuin/gopher-lua"

type Config struct {
	ConfigPath            string
	AutogenPath           string
	StatusBarVisible      bool
	RenderMode            string
	RenderOversample      float64
	MinZoom               float64
	MaxZoom               float64
	PinchSensitivity      float64
	PageCacheSize         int
	DualPage              bool
	FirstPageOffset       bool
	FitMode               string
	AnchorPosition        string
	Background            [3]uint8
	PageBackground        [3]uint8
	Foreground            [3]uint8
	StatusBarColor        [3]uint8
	AltBackground         [3]uint8
	AltPageBackground     [3]uint8
	AltForeground         [3]uint8
	AltStatusBarColor     [3]uint8
	HighlightForeground   [3]uint8
	HighlightBackground   [3]uint8
	AltColors             bool
	PageGap               int
	SpreadGap             int
	PageGapVertical       int
	PageGapHorizontal     int
	ScrollStep            int
	ScrollOff             int
	StatusBarPadding      int
	UIFont                string
	UIFontSize            int
	UIFontStyle           string
	UIFontWeight          int
	UIFontPath            string
	UIFontPathOverride    string
	StatusBarLeft         string
	StatusBarRight        string
	SequenceTimeoutMS     int
	NormalMessage         string
	KeyBindings           map[string]string
	MouseBindings         map[string]string
	MouseTextSelect       bool
	CopyOnSelect          bool
	SmoothScroll          bool
	InvertScroll          bool
	InvertSmoothScroll    bool
	SmoothScrollDampening float64
	SessionDatabase       bool
	AntiAliasing          int
	OutlineInitialDepth   int
	OutlineWidthPercent   int
	OutlineHeightPercent  int
	CompletionMaxItems    int
	RecentFilesMax        int
}

type Runtime struct {
	explicitPath     string
	docPath          string
	docName          string
	docMeta          documentMeta
	cfg              Config
	state            *lua.LState
	host             Host
	callbacks        map[string]*lua.LFunction
	callbackSeq      int
	uiSeq            int
	luaCallDepth     int
	deferredOpen     string
	dirty            bool
	verbose          bool
	pluginCatalog    *pluginCatalog
	pluginPaths      []string
	disabledPlugins  []string
	noConfig         bool
	plugins          *pluginState
	jobs             map[int]pluginJob
	jobResults       chan pluginJobResult
	nextJobID        int
	operations       map[int]*pluginOperation
	operationResults chan pluginOperationResult
	nextOperationID  int
	pluginGeneration int
	loadingPlugin    string
	activePlugin     string
	loadingAutogen   bool
}

type UIOverlay struct {
	ID         string
	Title      string
	Rows       []UIListRow
	Selected   int
	Scroll     int
	Query      string
	Searchable bool
	OnSelect   string
	OnClose    string
	Generation int
}

type UIListRow struct {
	Text      string
	Value     string
	ID        string
	Secondary string
	Depth     int
	Disabled  bool
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
	CloseUI(id string)
	UIVisible(id string) bool
	SetUIRows(id string, rows []UIListRow)
	SetUISelected(id string, selected int)
	SetUIScroll(id string, scroll int)
	SetUIQuery(id string, query string)
	UISelected(id string) int
	UIScroll(id string) int
	UIQuery(id string) string
	Page() int
	PageCount() int
	GotoPage(page int) error
	GotoDocumentPoint(page int, x, y float64) error
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

type ClipboardGetter interface {
	GetClipboard() string
}

type ClipboardSetter interface {
	SetClipboard(text string) error
}

type ExternalOpener interface {
	OpenExternal(uri string) error
}

type DirectoryPicker interface {
	PickDirectory() (string, error)
}

// DocumentFormatHost reports which formats the document engine can open. It is
// optional so hosts without a document engine still satisfy Host.
type DocumentFormatHost interface {
	SupportedExtensions() []string
	SupportsPath(path string) bool
}
