package ir

import "time"

// PageSnapshot represents a snapshot of a web page at a point in time.
// This is a pure data structure with no business logic.
type PageSnapshot struct {
	// Page metadata
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`

	// Pre-rendered content (aria snapshot format)
	Content string `json:"content,omitempty"`

	// Interactive elements mapping: hash -> element info
	InteractiveMap map[string]*ElementInfo `json:"-"`

	// Currently focused element hash (empty if no focus)
	FocusedHash string `json:"focusedHash,omitempty"`

	// Pending JavaScript dialog (alert, confirm, prompt)
	PendingDialog *DialogInfo `json:"pendingDialog,omitempty"`
}

// ElementInfo represents an interactive element identified from the accessibility tree
type ElementInfo struct {
	Role             string // AX role: "link", "button", "textbox", etc.
	Name             string // Computed accessible name
	Value            string // Current value (input/select)
	URL              string // For links
	Checked          string // "true", "false", "mixed"
	Focused          bool
	BackendDOMNodeID int // For interaction (click/type)
}

// DialogInfo represents a JavaScript dialog (alert, confirm, or prompt)
type DialogInfo struct {
	Type         string `json:"type"`         // "alert", "confirm", "prompt"
	Message      string `json:"message"`      // Dialog message text
	DefaultValue string `json:"defaultValue"` // Default value for prompt (empty for alert/confirm)
}
