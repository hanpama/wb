// Package protocol defines RPC types for client-server communication.
// It depends only on standard library.
package protocol

// PingArgs represents the arguments for Ping RPC call
type PingArgs struct{}

// PingReply represents the reply for Ping RPC call
type PingReply struct {
	Message string
}

// NewTabArgs represents the arguments for NewTab RPC call
type NewTabArgs struct {
	URL string
}

// NewTabReply represents the reply for NewTab RPC call
type NewTabReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int       // Total number of lines in full content
	Offset             int       // Current offset
	Limit              int       // Current limit
	FocusedHash        string    // Hash of currently focused element
	ActiveRequestCount int       // Number of active network requests
	AllTabs            []TabInfo // List of all tabs
}

// ShowArgs represents the arguments for Show RPC call
type ShowArgs struct {
	Offset int // Line offset (0-based)
	Limit  int // Number of lines to show (0 = show all)
}

// PendingDialog represents a dialog waiting for user response
type PendingDialog struct {
	Type    string // "alert", "confirm", "prompt"
	Message string
	Hash    string
}

// ShowReply represents the reply for Show RPC call
type ShowReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int        // Total number of lines in full content
	Offset             int        // Current offset
	Limit              int        // Current limit
	FocusedHash        string     // Hash of currently focused element
	ActiveRequestCount int        // Number of active network requests
	AllTabs            []TabInfo  // List of all tabs
}

// ClickArgs represents the arguments for Click RPC call
type ClickArgs struct {
	Hash string
}

// ClickReply represents the reply for Click RPC call
type ClickReply struct {
	URLChanged         bool
	TabID              string
	Title              string
	URL                string
	Markdown           string
	Diff               string
	PendingDialogs     []PendingDialog
	TotalLines         int          // Total number of lines in full content (when URLChanged)
	Offset             int          // Current offset (when URLChanged)
	Limit              int          // Current limit (when URLChanged)
	NewTabs            []NewTabInfo // Newly opened tabs (e.g., from target="_blank" links)
	FocusedHash        string       // Hash of currently focused element
	ActiveRequestCount int          // Number of active network requests
	AllTabs            []TabInfo    // List of all tabs
}

// GetStatusArgs represents the arguments for GetStatus RPC call
type GetStatusArgs struct{}

// GetStatusReply represents the reply for GetStatus RPC call
type GetStatusReply struct {
	TabCount       int
	ActiveTabID    string
	ActiveTabTitle string
}

// OpenArgs represents the arguments for Open RPC call
type OpenArgs struct {
	URL string
}

// OpenReply represents the reply for Open RPC call
type OpenReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int       // Total number of lines in full content
	Offset             int       // Current offset
	Limit              int       // Current limit
	FocusedHash        string    // Hash of currently focused element
	ActiveRequestCount int       // Number of active network requests
	AllTabs            []TabInfo // List of all tabs
}

// BackArgs represents the arguments for Back RPC call
type BackArgs struct{}

// BackReply represents the reply for Back RPC call
type BackReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int       // Total number of lines in full content
	Offset             int       // Current offset
	Limit              int       // Current limit
	FocusedHash        string    // Hash of currently focused element
	ActiveRequestCount int       // Number of active network requests
	AllTabs            []TabInfo // List of all tabs
}

// ForwardArgs represents the arguments for Forward RPC call
type ForwardArgs struct{}

// ForwardReply represents the reply for Forward RPC call
type ForwardReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int       // Total number of lines in full content
	Offset             int       // Current offset
	Limit              int       // Current limit
	FocusedHash        string    // Hash of currently focused element
	ActiveRequestCount int       // Number of active network requests
	AllTabs            []TabInfo // List of all tabs
}

// TabInfo represents information about a single tab
type TabInfo struct {
	TabID    string
	Title    string
	URL      string
	IsActive bool
}

// NewTabInfo represents information about a newly opened tab
type NewTabInfo struct {
	TabID string `json:"tabId"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// ListArgs represents the arguments for List RPC call
type ListArgs struct{}

// ListReply represents the reply for List RPC call
type ListReply struct {
	Tabs []TabInfo
}

// SwitchArgs represents the arguments for Switch RPC call
type SwitchArgs struct {
	TabID string
}

// SwitchReply represents the reply for Switch RPC call
type SwitchReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int       // Total number of lines in full content
	Offset             int       // Current offset
	Limit              int       // Current limit
	FocusedHash        string    // Hash of currently focused element
	ActiveRequestCount int       // Number of active network requests
	AllTabs            []TabInfo // List of all tabs
}

// CloseArgs represents the arguments for Close RPC call
type CloseArgs struct {
	TabID string // optional, empty string means current tab
}

// CloseReply represents the reply for Close RPC call
type CloseReply struct {
	Success           bool
	RemainingTabCount int
}

// DescribeArgs represents the arguments for Describe RPC call
type DescribeArgs struct {
	Hash string
}

// DescribeReply represents the reply for Describe RPC call
type DescribeReply struct {
	Hash    string
	Role    string // AX role: "link", "button", "textbox", etc.
	Name    string // Computed accessible name
	Value   string
	URL     string
	Checked string
	Found   bool

	// DOM context (pre-formatted HTML view for DOM inspection)
	DOMContext string
}

// EvalArgs represents the arguments for Eval RPC call
type EvalArgs struct {
	Expression string
}

// EvalReply represents the reply for Eval RPC call
type EvalReply struct {
	Result string
}

// ScreenshotArgs represents the arguments for Screenshot RPC call
type ScreenshotArgs struct {
	Full bool // Capture full page beyond viewport
}

// ScreenshotReply represents the reply for Screenshot RPC call
type ScreenshotReply struct {
	Data []byte // PNG image data
}

// DumpAXArgs represents the arguments for DumpAX RPC call
type DumpAXArgs struct{}

// DumpAXReply represents the reply for DumpAX RPC call
type DumpAXReply struct {
	JSON string // Raw AX tree JSON structure
}

// RespondToDialogArgs represents the arguments for RespondToDialog RPC call
type RespondToDialogArgs struct {
	Accept bool   // true for OK/Yes, false for Cancel/No
	Input  string // For prompt dialogs (optional)
}

// RespondToDialogReply represents the reply for RespondToDialog RPC call
type RespondToDialogReply struct {
	TabID              string
	Title              string
	URL                string
	Markdown           string
	PendingDialogs     []PendingDialog
	TotalLines         int
	Offset             int
	Limit              int
	FocusedHash        string
	ActiveRequestCount int
	AllTabs            []TabInfo
}

// TypeArgs represents the arguments for Type RPC call
type TypeArgs struct {
	Text string
}

// TypeReply represents the reply for Type RPC call
type TypeReply struct {
	URLChanged         bool
	TabID              string
	Title              string
	URL                string
	Markdown           string
	Diff               string
	PendingDialogs     []PendingDialog
	TotalLines         int
	Offset             int
	Limit              int
	NewTabs            []NewTabInfo
	FocusedHash        string
	ActiveRequestCount int
	AllTabs            []TabInfo
}

// EnterArgs represents the arguments for Enter RPC call
type EnterArgs struct{}

// EnterReply represents the reply for Enter RPC call
type EnterReply struct {
	URLChanged         bool
	TabID              string
	Title              string
	URL                string
	Markdown           string
	Diff               string
	PendingDialogs     []PendingDialog
	TotalLines         int
	Offset             int
	Limit              int
	NewTabs            []NewTabInfo
	FocusedHash        string
	ActiveRequestCount int
	AllTabs            []TabInfo
}
