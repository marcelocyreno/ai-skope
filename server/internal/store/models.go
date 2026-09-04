package store

// Pairing is an extension that completed the pairing handshake.
type Pairing struct {
	ID         string `json:"id"`
	Origin     string `json:"origin"`
	Label      string `json:"label"`
	CreatedAt  int64  `json:"createdAt"`
	LastSeenAt int64  `json:"lastSeenAt"`
	RevokedAt  int64  `json:"revokedAt,omitempty"`
}

// RuntimeOverride is the user's stored preference for one runtime.
type RuntimeOverride struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
	Command string `json:"command"`
}

// Provider is a model provider whose key the server holds for the runtimes.
type Provider struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	Name        string   `json:"name"`
	BaseURL     string   `json:"baseUrl,omitempty"`
	KeyRef      string   `json:"-"`
	KeyMasked   string   `json:"key"`
	AvailableTo []string `json:"availableTo"`
	CreatedAt   int64    `json:"createdAt"`
	UpdatedAt   int64    `json:"updatedAt"`
	LastTestAt  int64    `json:"lastTestAt,omitempty"`
	LastTestOK  bool     `json:"lastTestOk"`
	LastTestMsg string   `json:"lastTestMessage,omitempty"`
	Models      []Model  `json:"models,omitempty"`
}

// Model is one model discovered from a provider.
type Model struct {
	Name string `json:"model"`
	Ctx  int64  `json:"ctx,omitempty"`
}

// Access levels for an allowed folder.
const (
	AccessRead      = "read"
	AccessReadWatch = "read+watch"
)

// Folder is a directory the server is allowed to read.
type Folder struct {
	ID            string `json:"id"`
	Path          string `json:"path"`
	Access        string `json:"access"`
	FileCount     int64  `json:"fileCount"`
	LastIndexedAt int64  `json:"lastIndexedAt"`
	CreatedAt     int64  `json:"createdAt"`
}

// File is one indexed entry inside an allowed folder.
type File struct {
	Path      string `json:"path"`
	FolderID  string `json:"folderId"`
	Name      string `json:"name"`
	Ext       string `json:"ext"`
	Size      int64  `json:"size"`
	MTime     int64  `json:"mtime"`
	IsDir     bool   `json:"isDir"`
	IndexedAt int64  `json:"indexedAt,omitempty"`
	Snippet   string `json:"snippet,omitempty"`
}

// Chat is one conversation, always tied to the page (or file) it started on.
type Chat struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	URL          string `json:"url"`
	Host         string `json:"host"`
	PageTitle    string `json:"pageTitle,omitempty"`
	Favicon      string `json:"favicon,omitempty"`
	Runtime      string `json:"runtime,omitempty"`
	Variant      string `json:"variant,omitempty"`
	Provider     string `json:"provider,omitempty"`
	Model        string `json:"model,omitempty"`
	Effort       string `json:"effort,omitempty"`
	AgentSession string `json:"-"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
	DeletedAt    int64  `json:"deletedAt,omitempty"`
	MessageCount int    `json:"messageCount"`
}

// Message is one turn in a chat.
type Message struct {
	ID        string        `json:"id"`
	ChatID    string        `json:"chatId"`
	Role      string        `json:"role"` // user | assistant
	Text      string        `json:"text"`
	Tools     []ToolRecord  `json:"tools,omitempty"`
	Usage     *Usage        `json:"usage,omitempty"`
	Error     string        `json:"error,omitempty"`
	Model     string        `json:"model,omitempty"`
	CreatedAt int64         `json:"createdAt"`
	Context   []ContextItem `json:"context,omitempty"`
}

// ToolRecord is a tool line shown in the transcript ("Read table.pg-table").
type ToolRecord struct {
	Name   string `json:"name"`
	Target string `json:"target,omitempty"`
	Detail string `json:"detail,omitempty"`
	State  string `json:"state"` // running | done | failed
}

// Usage reports token counts and wall time for a turn.
type Usage struct {
	InputTokens  int64 `json:"inputTokens"`
	OutputTokens int64 `json:"outputTokens"`
	MS           int64 `json:"ms"`
}

// Context item types.
const (
	ContextElement = "element"
	ContextText    = "text"
	ContextFile    = "file"
	ContextPage    = "page"
)

// ContextItem is one piece of context attached to a user message: a picked
// element, a text selection, a local file, or the page itself.
type ContextItem struct {
	ID   string `json:"id,omitempty"`
	Type string `json:"type"`

	// element
	Selector string `json:"selector,omitempty"`
	HTML     string `json:"html,omitempty"`
	Rect     []int  `json:"rect,omitempty"`

	// text selection
	Quote  string `json:"quote,omitempty"`
	Before string `json:"before,omitempty"`
	After  string `json:"after,omitempty"`

	// file
	Path string `json:"path,omitempty"`

	// element/page/file shared
	Text  string `json:"text,omitempty"`
	URL   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

// Note is a page-linked note, optionally quoting a selection.
type Note struct {
	ID        string `json:"id"`
	URL       string `json:"url"`
	Host      string `json:"host"`
	Title     string `json:"title"`
	Favicon   string `json:"favicon,omitempty"`
	Quote     string `json:"quote,omitempty"`
	Body      string `json:"body"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}
