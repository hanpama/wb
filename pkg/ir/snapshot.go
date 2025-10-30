package ir

import "time"

// PageSnapshot represents a snapshot of a web page at a point in time.
// This is a pure data structure with no business logic.
type PageSnapshot struct {
	// Page metadata
	URL       string    `json:"url"`
	Title     string    `json:"title"`
	Timestamp time.Time `json:"timestamp"`

	// DOM tree
	Root *Node `json:"root,omitempty"`

	// Interactive elements mapping: hash -> node
	// This allows quick lookup of interactive elements by their hash
	InteractiveMap map[string]*Node `json:"-"`

	// Currently focused element hash (empty if no focus)
	FocusedHash string `json:"focusedHash,omitempty"`

	// Pending JavaScript dialog (alert, confirm, prompt)
	PendingDialog *DialogInfo `json:"pendingDialog,omitempty"`
}

// DialogInfo represents a JavaScript dialog (alert, confirm, or prompt)
type DialogInfo struct {
	Type         string `json:"type"`         // "alert", "confirm", "prompt"
	Message      string `json:"message"`      // Dialog message text
	DefaultValue string `json:"defaultValue"` // Default value for prompt (empty for alert/confirm)
}
