package renderer

import (
	"fmt"
	"strings"

	"github.com/hanpama/wb/pkg/protocol"
)

// FooterOptions contains all information needed to render the footer
type FooterOptions struct {
	// Pagination info
	TotalLines int
	Offset     int
	Limit      int

	// Tab list
	AllTabs []protocol.TabInfo

	// Network status
	ActiveRequestCount int

	// Focus info
	FocusedHash string

	// Alert/notification messages
	Messages []string

	// Pending dialogs
	PendingDialogs []protocol.PendingDialog
}

// RenderFooter generates the footer section with all metadata
func RenderFooter(opts FooterOptions) string {
	var parts []string

	// 1. Pagination info (if applicable)
	if opts.TotalLines > 0 {
		endLine := opts.Offset + opts.Limit
		if endLine > opts.TotalLines {
			endLine = opts.TotalLines
		}

		line := fmt.Sprintf("[Lines %d-%d / %d]", opts.Offset+1, endLine, opts.TotalLines)

		// Add navigation hints
		var hints []string
		if endLine < opts.TotalLines {
			nextOffset := opts.Offset + opts.Limit
			hints = append(hints, fmt.Sprintf("Next: wb show --offset %d", nextOffset))
		}
		if opts.Offset > 0 {
			prevOffset := opts.Offset - opts.Limit
			if prevOffset < 0 {
				prevOffset = 0
			}
			hints = append(hints, fmt.Sprintf("Prev: wb show --offset %d", prevOffset))
		}

		if len(hints) > 0 {
			line += " • " + strings.Join(hints, " • ")
		}

		parts = append(parts, line)
	}

	// 2. Tab list (always shown)
	if len(opts.AllTabs) > 0 {
		for _, tab := range opts.AllTabs {
			prefix := "  "
			if tab.IsActive {
				prefix = "• "
			}
			parts = append(parts, fmt.Sprintf("%s%s | %s", prefix, tab.TabID, tab.URL))
		}
	}

	// 3. Network status (if active requests)
	if opts.ActiveRequestCount > 0 {
		parts = append(parts, fmt.Sprintf("[⏳ %d network requests in progress...]", opts.ActiveRequestCount))
	}

	// 4. Focus info (if focused element)
	if opts.FocusedHash != "" {
		parts = append(parts, fmt.Sprintf("Focused: {%s}", opts.FocusedHash))
	}

	// 5. Alert/notification messages
	for _, msg := range opts.Messages {
		parts = append(parts, "")
		parts = append(parts, msg)
	}

	// 6. Pending dialogs
	if len(opts.PendingDialogs) > 0 {
		parts = append(parts, "")
		for _, dialog := range opts.PendingDialogs {
			switch dialog.Type {
			case "alert":
				parts = append(parts, fmt.Sprintf("[Alert] \"%s\"", dialog.Message))
				parts = append(parts, "  Use: wb respond ok")
			case "confirm":
				parts = append(parts, fmt.Sprintf("[Confirm] \"%s\"", dialog.Message))
				parts = append(parts, "  Use: wb respond ok  (or: wb respond cancel)")
			case "prompt":
				parts = append(parts, fmt.Sprintf("[Prompt] \"%s\"", dialog.Message))
				parts = append(parts, "  Use: wb respond ok \"your text\"  (or: wb respond cancel)")
			}
		}
	}

	return strings.Join(parts, "\n")
}
