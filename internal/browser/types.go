package browser

// TabID is a unique identifier for a browser tab/context
type TabID string

// TabInfo represents information about a browser tab
type TabInfo struct {
	ID    TabID
	URL   string
	Title string
}

// DialogInfo represents a pending dialog (alert, confirm, prompt)
type DialogInfo struct {
	Type    string // "alert", "confirm", "prompt"
	Message string
}
