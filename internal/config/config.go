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
	ZoomStep              float64
	PageCacheMemoryMB     int
	LinkSchemes           []string
	RenderThreads         int
	MuPDFStoreMB          int
	HintChars             string
	AltColorsKeepImages   bool
	TrimMargins           bool
	AnnotationColors      []string
	LinkPreview           bool
	DualPage              bool
	FirstPageOffset       bool
	FitMode               string
	AnchorPosition        string
	Theme                 Theme
	OverviewThumbWidth    int
	OverviewMaxColumns    int
	OverviewGap           int
	LoadingIndicator      bool
	AltColors             bool
	PageGap               int
	SpreadGap             int
	PageGapVertical       int
	PageGapHorizontal     int
	ScrollStep            int
	ScrollOff             int
	StatusBarLeft         string
	StatusBarRight        string
	SequenceTimeoutMS     int
	AnimationFrameMS      int
	NormalMessage         string
	KeyBindings           map[string]string
	MouseBindings         map[string]string
	MouseTextSelect       bool
	CopyOnSelect          bool
	SmoothScrollSources   SmoothInputSources
	SmoothZoomSources     SmoothInputSources
	InvertScroll          bool
	InvertSmoothScroll    bool
	SmoothScrollDampening float64
	SmoothZoomDampening   float64
	AutoscrollSpeedFactor float64
	AutoscrollMaxSpeed    float64
	SessionDatabase       bool
	AntiAliasing          int
	OutlineInitialDepth   int
	OutlineWidthPercent   int
	OutlineHeightPercent  int
	CompletionMaxItems    int
	RecentFilesMax        int
	AutoReload            bool
	AutoReloadDelayMS     int
	LinkPreviewDelayMS    int
	PromptHistoryMax      int
	TrimPadding           int
}

type Runtime struct {
	luaGeneration
	explicitPath     string
	docPath          string
	docName          string
	docMeta          documentMeta
	host             Host
	uiSeq            int
	luaCallDepth     int
	deferredOpen     string
	assigned         map[string]bool // built-in options assigned since ConsumeDirty
	verbose          bool
	pluginPaths      []string
	disabledPlugins  []string
	noConfig         bool
	operationResults chan pluginOperationResult
	wake             func() // wakes the viewer to poll a delivered result; may be nil
	nextOperationID  int
	loadingPlugin    string
	activePlugin     string
	loadingAutogen   bool
}

// luaGeneration is what one load of the configuration builds. Reload
// replaces it as a whole and, if loading fails, puts the old one back.
type luaGeneration struct {
	state            *lua.LState
	cfg              Config
	callbacks        map[string]*lua.LFunction
	callbackSeq      int
	dirty            bool
	pluginCatalog    *pluginCatalog
	plugins          *pluginState
	operations       map[int]*pluginOperation
	pluginGeneration int
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
