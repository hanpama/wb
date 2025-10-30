// Package renderer converts IR to display items
package renderer

import (
	"strings"

	"github.com/hanpama/wb/pkg/ir"
)

// ConvertToBlockItems converts IR tree to list of block items using DFS traversal
// Interactive elements are extracted as independent blocks regardless of parent context
func ConvertToBlockItems(root *ir.Node) []BlockItem {
	var blocks []BlockItem
	extractBlocks(root, &blocks)
	return blocks
}

// extractBlocks recursively extracts blocks via depth-first traversal
// Priority: 1) Interactive elements (own block), 2) Structural blocks, 3) Containers
func extractBlocks(node *ir.Node, blocks *[]BlockItem) {
	if node == nil {
		return
	}

	// Skip nodes without valid bounding box
	if !hasValidBoundingBox(node) {
		// For invisible elements, children might still be visible
		if node.Type == ir.NodeTypeElement {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}
		return
	}

	// Text nodes at block level - wrap in paragraph
	if node.Type == ir.NodeTypeText {
		text := strings.TrimSpace(node.Text)
		if text != "" {
			*blocks = append(*blocks, Paragraph{
				Content: []InlineItem{InlineText{Content: text}},
			})
		}
		return
	}

	// Element nodes
	if node.Type != ir.NodeTypeElement {
		return
	}

	tag := strings.ToLower(node.Tag)

	// PRIORITY 1: Interactive elements - special handling for nesting
	if isInteractiveTag(tag) && node.Interactive != nil {
		// Extract child blocks first (inner interactive elements)
		var childBlocks []BlockItem
		for _, child := range node.Children {
			if child.Type == ir.NodeTypeElement {
				extractBlocks(child, &childBlocks)
			} else if child.Type == ir.NodeTypeText {
				text := strings.TrimSpace(child.Text)
				if text != "" && hasValidBoundingBox(child) {
					childBlocks = append(childBlocks, Paragraph{
						Content: []InlineItem{InlineText{Content: text}},
					})
				}
			}
		}

		// If we have child blocks, wrap the first one with this interactive
		if len(childBlocks) > 0 {
			wrapped := wrapBlockWithInteractive(childBlocks[0], node)
			*blocks = append(*blocks, wrapped)

			// Add remaining child blocks as-is
			for i := 1; i < len(childBlocks); i++ {
				*blocks = append(*blocks, childBlocks[i])
			}
		} else {
			// No children - create simple interactive block
			*blocks = append(*blocks, createInteractiveBlock(node))
		}
		return
	}

	// PRIORITY 2: Structural block elements - extract inline content only
	switch tag {
	case "h1":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 1, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "h2":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 2, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "h3":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 3, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "h4":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 4, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "h5":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 5, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "h6":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Heading{Level: 6, Content: directText})
		} else {
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "p":
		directText := extractDirectInlineContent(node)
		if len(directText) > 0 {
			*blocks = append(*blocks, Paragraph{Content: directText})
		} else {
			// No direct inline content - recurse into children (handles malformed HTML)
			for _, child := range node.Children {
				extractBlocks(child, blocks)
			}
		}

	case "ul":
		for _, child := range node.Children {
			if strings.ToLower(child.Tag) == "li" && hasValidBoundingBox(child) {
				// Extract all blocks from LI children recursively
				var liBlocks []BlockItem
				for _, liChild := range child.Children {
					extractBlocks(liChild, &liBlocks)
				}

				// Convert first block to list item, keep rest as-is
				if len(liBlocks) > 0 {
					first := liBlocks[0]
					switch f := first.(type) {
					case Paragraph:
						*blocks = append(*blocks, UnorderedListItem{Content: f.Content})
					default:
						// Non-paragraph blocks: add as-is
						*blocks = append(*blocks, first)
					}
					// Add remaining blocks
					*blocks = append(*blocks, liBlocks[1:]...)
				}
			}
		}

	case "ol":
		index := 1
		for _, child := range node.Children {
			if strings.ToLower(child.Tag) == "li" && hasValidBoundingBox(child) {
				// Extract all blocks from LI children recursively
				var liBlocks []BlockItem
				for _, liChild := range child.Children {
					extractBlocks(liChild, &liBlocks)
				}

				// Convert first block to ordered list item, keep rest as-is
				if len(liBlocks) > 0 {
					first := liBlocks[0]
					switch f := first.(type) {
					case Paragraph:
						*blocks = append(*blocks, OrderedListItem{
							Index:   index,
							Content: f.Content,
						})
						index++
					default:
						// Non-paragraph blocks: add as-is
						*blocks = append(*blocks, first)
						index++
					}
					// Add remaining blocks
					*blocks = append(*blocks, liBlocks[1:]...)
				}
			}
		}

	case "br":
		*blocks = append(*blocks, LineBreak{})

	// Default: Unknown elements are treated as transparent containers (fallback behavior)
	// This ensures no content is lost and supports custom elements/web components
	default:
		for _, child := range node.Children {
			extractBlocks(child, blocks)
		}
	}
}

// extractDirectInlineContent extracts inline content from direct children
// Only processes text and inline formatting elements, skips interactive and block elements
func extractDirectInlineContent(node *ir.Node) []InlineItem {
	var items []InlineItem

	for _, child := range node.Children {
		if !hasValidBoundingBox(child) {
			continue
		}

		if child.Type == ir.NodeTypeText {
			text := strings.TrimSpace(child.Text)
			if text != "" {
				items = append(items, InlineText{Content: text})
			}
			continue
		}

		if child.Type == ir.NodeTypeElement {
			tag := strings.ToLower(child.Tag)

			// Skip block elements (they'll be extracted as separate blocks)
			if isBlockTag(tag) {
				continue
			}

			// Process interactive elements as inline items
			if isInteractiveTag(tag) && child.Interactive != nil {
				switch tag {
				case "a":
					items = append(items, InlineLink{
						Hash: child.Interactive.Hash,
						Text: getInteractiveText(child),
						URL:  child.Interactive.Href,
					})
				case "button":
					items = append(items, InlineButton{
						Hash: child.Interactive.Hash,
						Text: getInteractiveText(child),
					})
				case "input":
					switch child.Interactive.Type {
					case ir.InteractiveCheckbox:
						items = append(items, InlineCheckbox{
							Hash:    child.Interactive.Hash,
							Checked: child.Interactive.Checked,
						})
					case ir.InteractiveRadio:
						items = append(items, InlineRadio{
							Hash:    child.Interactive.Hash,
							Checked: child.Interactive.Checked,
						})
					default:
						inputType := child.Interactive.InputType
						if inputType == "" {
							inputType = "text"
						}
						items = append(items, InlineInput{
							Hash:        child.Interactive.Hash,
							Type:        inputType,
							Value:       child.Interactive.Value,
							Placeholder: child.Interactive.Placeholder,
						})
					}
				case "textarea":
					items = append(items, InlineInput{
						Hash:        child.Interactive.Hash,
						Type:        "textarea",
						Value:       child.Interactive.Value,
						Placeholder: child.Interactive.Placeholder,
					})
				}
				continue
			}

			// Process inline formatting elements
			switch tag {
			case "strong", "b":
				nested := extractDirectInlineContent(child)
				if len(nested) > 0 {
					items = append(items, InlineStrong{Content: nested})
				}

			case "em", "i":
				nested := extractDirectInlineContent(child)
				if len(nested) > 0 {
					items = append(items, InlineEmphasis{Content: nested})
				}

			case "code":
				items = append(items, InlineCode{Content: extractPlainText(child)})

			case "br":
				items = append(items, InlineText{Content: " "})

			// Other elements - recurse into children
			default:
				items = append(items, extractDirectInlineContent(child)...)
			}
		}
	}

	return items
}

// isInteractiveTag checks if a tag represents an interactive element
func isInteractiveTag(tag string) bool {
	switch tag {
	case "a", "button", "input", "textarea", "select":
		return true
	}
	return false
}

// isBlockTag checks if a tag represents a block-level element
func isBlockTag(tag string) bool {
	switch tag {
	case "div", "p", "h1", "h2", "h3", "h4", "h5", "h6",
		"ul", "ol", "li", "table", "section", "article",
		"nav", "header", "footer", "main", "aside", "form":
		return true
	}
	return false
}

// createInteractiveBlock creates a block item for an interactive element
func createInteractiveBlock(node *ir.Node) BlockItem {
	tag := strings.ToLower(node.Tag)

	switch tag {
	case "a":
		return Paragraph{
			Content: []InlineItem{
				InlineLink{
					Hash: node.Interactive.Hash,
					Text: getInteractiveText(node),
					URL:  node.Interactive.Href,
				},
			},
		}

	case "button":
		return Paragraph{
			Content: []InlineItem{
				InlineButton{
					Hash: node.Interactive.Hash,
					Text: getInteractiveText(node),
				},
			},
		}

	case "input":
		switch node.Interactive.Type {
		case ir.InteractiveCheckbox:
			return Paragraph{
				Content: []InlineItem{
					InlineCheckbox{
						Hash:    node.Interactive.Hash,
						Checked: node.Interactive.Checked,
					},
				},
			}

		case ir.InteractiveRadio:
			return Paragraph{
				Content: []InlineItem{
					InlineRadio{
						Hash:    node.Interactive.Hash,
						Checked: node.Interactive.Checked,
					},
				},
			}

		default:
			inputType := node.Interactive.InputType
			if inputType == "" {
				inputType = "text"
			}
			return Paragraph{
				Content: []InlineItem{
					InlineInput{
						Hash:        node.Interactive.Hash,
						Type:        inputType,
						Value:       node.Interactive.Value,
						Placeholder: node.Interactive.Placeholder,
					},
				},
			}
		}

	case "textarea":
		return Paragraph{
			Content: []InlineItem{
				InlineInput{
					Hash:        node.Interactive.Hash,
					Type:        "textarea",
					Value:       node.Interactive.Value,
					Placeholder: node.Interactive.Placeholder,
				},
			},
		}
	}

	// Fallback
	return Paragraph{Content: []InlineItem{InlineText{Content: getInteractiveText(node)}}}
}

// extractPlainText extracts only text content, ignoring structure
func extractPlainText(node *ir.Node) string {
	if node == nil {
		return ""
	}

	var parts []string
	extractTextRecursive(node, &parts)
	return strings.Join(parts, " ")
}

// getInteractiveText gets display text for interactive element, with accessibility fallbacks
// This renders child images as markdown syntax
func getInteractiveText(node *ir.Node) string {
	// 1. Try to render children as markdown (includes images)
	markdown := extractMarkdownContent(node)
	if markdown != "" {
		return markdown
	}

	// 2. Try accessibility attributes
	if node.Interactive != nil {
		if node.Interactive.AriaLabel != "" {
			return node.Interactive.AriaLabel
		}
	}

	// 3. Try title attribute
	if title, ok := node.Attributes["title"]; ok && title != "" {
		return title
	}

	// 4. Try alt attribute (for images)
	if alt, ok := node.Attributes["alt"]; ok && alt != "" {
		return alt
	}

	// 5. For inputs, use placeholder as last resort
	if node.Interactive != nil && node.Interactive.Placeholder != "" {
		return "(" + node.Interactive.Placeholder + ")"
	}

	// 6. Fallback to tag name
	return "[" + strings.ToUpper(node.Tag) + "]"
}

// extractMarkdownContent extracts content as markdown (text + images)
func extractMarkdownContent(node *ir.Node) string {
	if node == nil {
		return ""
	}

	var parts []string
	extractMarkdownRecursive(node, &parts)
	result := strings.Join(parts, "")
	return strings.TrimSpace(result)
}

func extractMarkdownRecursive(node *ir.Node, parts *[]string) {
	if node == nil {
		return
	}

	// Extract text nodes
	if node.Type == ir.NodeTypeText {
		text := strings.TrimSpace(node.Text)
		if text != "" {
			*parts = append(*parts, text)
		}
	}

	// Convert img tags to markdown image syntax
	if node.Type == ir.NodeTypeElement && strings.ToLower(node.Tag) == "img" {
		alt := ""
		if a, ok := node.Attributes["alt"]; ok {
			alt = a
		}

		src := ""
		if s, ok := node.Attributes["src"]; ok {
			src = s
			// Shorten long URLs
			if len(src) > 50 {
				src = shortenURL(src, 50)
			}
		}

		*parts = append(*parts, "!["+alt+"]("+src+")")
		return // Don't recurse into img children
	}

	// Recurse into children
	for _, child := range node.Children {
		extractMarkdownRecursive(child, parts)
	}
}

func extractTextRecursive(node *ir.Node, parts *[]string) {
	if node == nil {
		return
	}

	if node.Type == ir.NodeTypeText {
		text := strings.TrimSpace(node.Text)
		if text != "" {
			*parts = append(*parts, text)
		}
	}

	// Extract alt text from img tags
	if node.Type == ir.NodeTypeElement && strings.ToLower(node.Tag) == "img" {
		if alt, ok := node.Attributes["alt"]; ok && alt != "" {
			*parts = append(*parts, alt)
		}
	}

	for _, child := range node.Children {
		extractTextRecursive(child, parts)
	}
}

// wrapBlockWithInteractive wraps a block's content with an interactive element
func wrapBlockWithInteractive(block BlockItem, interactiveNode *ir.Node) BlockItem {
	tag := strings.ToLower(interactiveNode.Tag)

	// Create the inline interactive wrapper
	var wrapper InlineItem

	switch tag {
	case "a":
		wrapper = InlineLink{
			Hash: interactiveNode.Interactive.Hash,
			Text: getBlockText(block),
			URL:  interactiveNode.Interactive.Href,
		}
	case "button":
		wrapper = InlineButton{
			Hash: interactiveNode.Interactive.Hash,
			Text: getBlockText(block),
		}
	default:
		// For other interactive types, just return the block unchanged
		return block
	}

	// Wrap the block's content with the interactive element
	switch b := block.(type) {
	case Heading:
		return Heading{
			Level:   b.Level,
			Content: []InlineItem{wrapper},
		}
	case Paragraph:
		return Paragraph{
			Content: []InlineItem{wrapper},
		}
	case UnorderedListItem:
		return UnorderedListItem{
			Content: []InlineItem{wrapper},
		}
	case OrderedListItem:
		return OrderedListItem{
			Index:   b.Index,
			Content: []InlineItem{wrapper},
		}
	default:
		// For blocks we can't wrap, return unchanged
		return block
	}
}

// getBlockText extracts text from a block item for wrapping
func getBlockText(block BlockItem) string {
	switch b := block.(type) {
	case Heading:
		var parts []string
		for _, item := range b.Content {
			rendered := item.RenderInline()
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}
		return strings.Join(parts, " ")
	case Paragraph:
		var parts []string
		for _, item := range b.Content {
			rendered := item.RenderInline()
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}
		return strings.Join(parts, " ")
	case UnorderedListItem:
		var parts []string
		for _, item := range b.Content {
			rendered := item.RenderInline()
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}
		return strings.Join(parts, " ")
	case OrderedListItem:
		var parts []string
		for _, item := range b.Content {
			rendered := item.RenderInline()
			if rendered != "" {
				parts = append(parts, rendered)
			}
		}
		return strings.Join(parts, " ")
	default:
		return ""
	}
}
