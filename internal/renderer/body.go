// Package renderer provides functions to render browser output in terminal format.
package renderer

import (
	"strings"

	"github.com/hanpama/wb/pkg/ir"
)

// shortenURL truncates long URLs to make them more readable
func shortenURL(url string, maxLen int) string {
	if len(url) <= maxLen {
		return url
	}
	prefixLen := maxLen - 15
	if prefixLen < 10 {
		prefixLen = 10
	}
	return url[:prefixLen] + "[...]" + url[len(url)-10:]
}

// hasValidBoundingBox checks if a node has a valid bounding box
func hasValidBoundingBox(node *ir.Node) bool {
	if node.BoundingBox == nil {
		return false
	}
	return node.BoundingBox.Width > 0 && node.BoundingBox.Height > 0
}

// RenderBody converts a PageSnapshot to Markdown representation
// This uses a two-stage process:
// 1. Convert IR tree to list of BlockItems (view model)
// 2. Render BlockItems to text
func RenderBody(snapshot *ir.PageSnapshot) string {
	// Stage 1: Convert IR to Block Items
	blocks := ConvertToBlockItems(snapshot.Root)

	// Stage 2: Render Block Items to text
	var parts []string
	for _, block := range blocks {
		rendered := block.RenderBlock()
		if rendered != "" {
			parts = append(parts, rendered)
		}
	}

	result := strings.Join(parts, "")

	// Clean up: trim whitespace and normalize excessive newlines
	result = strings.TrimSpace(result)

	// Remove excessive consecutive newlines (more than 2)
	for strings.Contains(result, "\n\n\n") {
		result = strings.ReplaceAll(result, "\n\n\n", "\n\n")
	}

	return result
}
