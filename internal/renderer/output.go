package renderer

import (
	"fmt"
	"strings"

	"github.com/hanpama/wb/pkg/protocol"
)

const separator = "────────────────────────────────────────────────────────────────"

// OutputMode determines how the body content is rendered
type OutputMode int

const (
	// ModeFull renders the complete page content
	ModeFull OutputMode = iota
	// ModeDiff renders only the differences from previous state
	ModeDiff
)

// OutputOptions contains all information needed to render complete output
type OutputOptions struct {
	// Header
	Title string

	// Body
	Mode         OutputMode
	BodyContent  string // For ModeFull: full markdown; For ModeDiff: diff content
	DiffNoChange bool   // True if diff mode but no changes detected

	// Footer
	FooterOpts FooterOptions
}

// RenderOutput generates the complete output with header, body, and footer
func RenderOutput(opts OutputOptions) string {
	var parts []string

	// Header: Page title
	parts = append(parts, opts.Title)
	parts = append(parts, separator)

	// Body
	if opts.Mode == ModeDiff {
		parts = append(parts, "(No URL change - showing diff)")
		if opts.DiffNoChange {
			parts = append(parts, "(No changes detected)")
		} else if opts.BodyContent != "" {
			parts = append(parts, "")
			parts = append(parts, opts.BodyContent)
		}
	} else {
		// ModeFull
		parts = append(parts, opts.BodyContent)
	}

	// Footer
	parts = append(parts, separator)
	footer := RenderFooter(opts.FooterOpts)
	if footer != "" {
		parts = append(parts, footer)
	}

	return strings.Join(parts, "\n")
}

// RenderShowOutput is a convenience function for rendering show command output
func RenderShowOutput(reply *protocol.ShowReply) string {
	return RenderOutput(OutputOptions{
		Title:       reply.Title,
		Mode:        ModeFull,
		BodyContent: reply.Markdown,
		FooterOpts: FooterOptions{
			TotalLines:         reply.TotalLines,
			Offset:             reply.Offset,
			Limit:              reply.Limit,
			AllTabs:            reply.AllTabs,
			ActiveRequestCount: reply.ActiveRequestCount,
			FocusedHash:        reply.FocusedHash,
			PendingDialogs:     reply.PendingDialogs,
		},
	})
}

// RenderClickOutput renders click command output (conditional rendering)
func RenderClickOutput(reply *protocol.ClickReply) string {
	if reply.URLChanged {
		// Full render for URL change
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: reply.Markdown,
			FooterOpts: FooterOptions{
				TotalLines:         reply.TotalLines,
				Offset:             reply.Offset,
				Limit:              reply.Limit,
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           buildNewTabMessages(reply.NewTabs),
				PendingDialogs:     reply.PendingDialogs,
			},
		})
	}

	// Diff render
	messages := buildNewTabMessages(reply.NewTabs)

	// If dialog is present, show dialog in body instead of diff
	if len(reply.PendingDialogs) > 0 {
		dialogContent := buildDialogContent(reply.PendingDialogs)
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: dialogContent,
			FooterOpts: FooterOptions{
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           messages,
				// PendingDialogs removed - dialog is now in body
			},
		})
	}

	// Normal diff render (no dialog)
	return RenderOutput(OutputOptions{
		Title:        reply.Title,
		Mode:         ModeDiff,
		BodyContent:  reply.Diff,
		DiffNoChange: reply.Diff == "",
		FooterOpts: FooterOptions{
			AllTabs:            reply.AllTabs,
			ActiveRequestCount: reply.ActiveRequestCount,
			FocusedHash:        reply.FocusedHash,
			Messages:           messages,
			PendingDialogs:     reply.PendingDialogs,
		},
	})
}

// buildMessages converts FooterMessage string to messages slice
func buildMessages(footerMessage string) []string {
	if footerMessage == "" {
		return nil
	}
	return []string{footerMessage}
}

// buildNewTabMessages builds footer messages from NewTabs data
func buildNewTabMessages(newTabs []protocol.NewTabInfo) []string {
	if len(newTabs) == 0 {
		return nil
	}

	messages := make([]string, 0, len(newTabs))
	for _, tab := range newTabs {
		msg := fmt.Sprintf("[New tab opened: %s | %s]\nUse 'wb switch %s' to view the new tab.", tab.TabID, tab.URL, tab.TabID)
		messages = append(messages, msg)
	}
	return messages
}

// buildDialogContent builds body content from dialog information
func buildDialogContent(dialogs []protocol.PendingDialog) string {
	if len(dialogs) == 0 {
		return ""
	}

	var parts []string
	for _, dialog := range dialogs {
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
	return strings.Join(parts, "\n")
}

// RenderTypeOutput renders type command output (conditional rendering)
func RenderTypeOutput(reply *protocol.TypeReply) string {
	if reply.URLChanged {
		// Full render for URL change
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: reply.Markdown,
			FooterOpts: FooterOptions{
				TotalLines:         reply.TotalLines,
				Offset:             reply.Offset,
				Limit:              reply.Limit,
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           buildNewTabMessages(reply.NewTabs),
				PendingDialogs:     reply.PendingDialogs,
			},
		})
	}

	// Diff render
	messages := buildNewTabMessages(reply.NewTabs)

	// If dialog is present, show dialog in body instead of diff
	if len(reply.PendingDialogs) > 0 {
		dialogContent := buildDialogContent(reply.PendingDialogs)
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: dialogContent,
			FooterOpts: FooterOptions{
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           messages,
				// PendingDialogs removed - dialog is now in body
			},
		})
	}

	// Normal diff render (no dialog)
	return RenderOutput(OutputOptions{
		Title:        reply.Title,
		Mode:         ModeDiff,
		BodyContent:  reply.Diff,
		DiffNoChange: reply.Diff == "",
		FooterOpts: FooterOptions{
			AllTabs:            reply.AllTabs,
			ActiveRequestCount: reply.ActiveRequestCount,
			FocusedHash:        reply.FocusedHash,
			Messages:           messages,
			PendingDialogs:     reply.PendingDialogs,
		},
	})
}

// RenderEnterOutput renders enter command output (conditional rendering)
func RenderEnterOutput(reply *protocol.EnterReply) string {
	if reply.URLChanged {
		// Full render for URL change
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: reply.Markdown,
			FooterOpts: FooterOptions{
				TotalLines:         reply.TotalLines,
				Offset:             reply.Offset,
				Limit:              reply.Limit,
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           buildNewTabMessages(reply.NewTabs),
				PendingDialogs:     reply.PendingDialogs,
			},
		})
	}

	// Diff render
	messages := buildNewTabMessages(reply.NewTabs)

	// If dialog is present, show dialog in body instead of diff
	if len(reply.PendingDialogs) > 0 {
		dialogContent := buildDialogContent(reply.PendingDialogs)
		return RenderOutput(OutputOptions{
			Title:       reply.Title,
			Mode:        ModeFull,
			BodyContent: dialogContent,
			FooterOpts: FooterOptions{
				AllTabs:            reply.AllTabs,
				ActiveRequestCount: reply.ActiveRequestCount,
				FocusedHash:        reply.FocusedHash,
				Messages:           messages,
				// PendingDialogs removed - dialog is now in body
			},
		})
	}

	// Normal diff render (no dialog)
	return RenderOutput(OutputOptions{
		Title:        reply.Title,
		Mode:         ModeDiff,
		BodyContent:  reply.Diff,
		DiffNoChange: reply.Diff == "",
		FooterOpts: FooterOptions{
			AllTabs:            reply.AllTabs,
			ActiveRequestCount: reply.ActiveRequestCount,
			FocusedHash:        reply.FocusedHash,
			Messages:           messages,
			PendingDialogs:     reply.PendingDialogs,
		},
	})
}

// FormatNewTabMessage formats a "new tab opened" notification message
// Deprecated: Use buildNewTabMessages with NewTabs data instead
func FormatNewTabMessage(tabID, url string) string {
	return fmt.Sprintf("[New tab opened: %s | %s]\nUse 'wb switch %s' to view the new tab.", tabID, url, tabID)
}
