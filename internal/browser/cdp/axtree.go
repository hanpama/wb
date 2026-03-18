package cdp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hanpama/wb/pkg/ir"
)

// AXNode represents a node in the accessibility tree
type AXNode struct {
	NodeID           string
	Role             string
	Name             string
	Value            string
	URL              string
	Checked          string // "true", "false", "mixed"
	Focused          bool
	Level            int // For headings
	BackendDOMNodeID int
	Children         []*AXNode
}

// ParseAXTree parses the CDP Accessibility.getFullAXTree result into AXNode tree
func ParseAXTree(result map[string]any) *AXNode {
	nodesRaw, ok := result["nodes"].([]any)
	if !ok || len(nodesRaw) == 0 {
		return nil
	}

	// Build lookup map
	nodeMap := make(map[string]*AXNode, len(nodesRaw))
	var rootID string

	for i, raw := range nodesRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}

		node := &AXNode{}

		if id, ok := m["nodeId"].(string); ok {
			node.NodeID = id
		}

		if role, ok := m["role"].(map[string]any); ok {
			if v, ok := role["value"].(string); ok {
				node.Role = v
			}
		}

		if name, ok := m["name"].(map[string]any); ok {
			if v, ok := name["value"].(string); ok {
				node.Name = v
			}
		}

		if value, ok := m["value"].(map[string]any); ok {
			if v, ok := value["value"].(string); ok {
				node.Value = v
			}
		}

		// Extract backendDOMNodeId
		if bid, ok := m["backendDOMNodeId"].(float64); ok {
			node.BackendDOMNodeID = int(bid)
		}

		// Extract properties
		if props, ok := m["properties"].([]any); ok {
			for _, p := range props {
				pm, ok := p.(map[string]any)
				if !ok {
					continue
				}
				propName, _ := pm["name"].(string)
				propValue, _ := pm["value"].(map[string]any)
				if propValue == nil {
					continue
				}
				val := propValue["value"]

				switch propName {
				case "url":
					if s, ok := val.(string); ok {
						node.URL = s
					}
				case "checked":
					if s, ok := val.(string); ok {
						node.Checked = s
					}
				case "focused":
					if b, ok := val.(bool); ok {
						node.Focused = b
					}
				case "level":
					if lvl, ok := val.(float64); ok {
						node.Level = int(lvl)
					}
				}
			}
		}

		nodeMap[node.NodeID] = node
		if i == 0 {
			rootID = node.NodeID
		}
	}

	// Build tree from childIds
	for _, raw := range nodesRaw {
		m, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		id, _ := m["nodeId"].(string)
		parent := nodeMap[id]
		if parent == nil {
			continue
		}

		if childIDs, ok := m["childIds"].([]any); ok {
			for _, cid := range childIDs {
				cidStr, ok := cid.(string)
				if !ok {
					continue
				}
				if child, ok := nodeMap[cidStr]; ok {
					parent.Children = append(parent.Children, child)
				}
			}
		}
	}

	return nodeMap[rootID]
}

// RenderAXSnapshot renders an AX tree as Playwright-style aria snapshot text.
// Returns the rendered content, a map of interactive elements, and the focused element hash.
func RenderAXSnapshot(root *AXNode) (string, map[string]*ir.ElementInfo, string) {
	if root == nil {
		return "", nil, ""
	}

	elements := make(map[string]*ir.ElementInfo)
	var focusedHash string
	var sb strings.Builder

	renderNode(&sb, root, 0, elements, &focusedHash)

	content := strings.TrimSpace(sb.String())
	// Collapse excessive newlines
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return content, elements, focusedHash
}

func renderNode(sb *strings.Builder, node *AXNode, depth int, elements map[string]*ir.ElementInfo, focusedHash *string) {
	if node == nil {
		return
	}

	// Skip ignored/none roles and transparent containers
	switch node.Role {
	case "none", "Ignored", "IgnoredRole", "RootWebArea", "WebArea", "generic":
		for _, child := range node.Children {
			renderNode(sb, child, depth, elements, focusedHash)
		}
		return
	}

	indent := strings.Repeat("  ", depth)
	interactive := isInteractiveRole(node.Role)

	// Generate hash for interactive elements
	var hash string
	if interactive && node.BackendDOMNodeID > 0 {
		hash = generateAXHash(node.BackendDOMNodeID, node.Role)
		elements[hash] = &ir.ElementInfo{
			Role:             node.Role,
			Name:             node.Name,
			Value:            node.Value,
			URL:              node.URL,
			Checked:          node.Checked,
			Focused:          node.Focused,
			BackendDOMNodeID: node.BackendDOMNodeID,
		}
		if node.Focused && *focusedHash == "" {
			*focusedHash = hash
		}
	}

	switch node.Role {
	case "heading":
		level := node.Level
		if level == 0 {
			level = 2
		}
		name := node.Name
		if name == "" {
			name = collectText(node)
		}
		if name != "" {
			fmt.Fprintf(sb, "\n%s- heading %q [level=%d]", indent, name, level)
		}

	case "link":
		name := node.Name
		if name == "" {
			name = collectText(node)
		}
		if name != "" {
			sb.WriteString("\n" + indent + "- link " + quote(name))
			if node.URL != "" {
				sb.WriteString(" [url=" + shortenURL(node.URL, 80) + "]")
			}
			if hash != "" {
				sb.WriteString(" {" + hash + "}")
			}
		}

	case "button":
		name := node.Name
		if name == "" {
			name = "button"
		}
		fmt.Fprintf(sb, "\n%s- button %s {%s}", indent, quote(name), hash)

	case "textbox", "searchbox":
		name := node.Name
		value := node.Value
		fmt.Fprintf(sb, "\n%s- %s %s [value=%s] {%s}", indent, node.Role, quote(name), quote(value), hash)

	case "checkbox":
		checked := node.Checked
		if checked == "" {
			checked = "false"
		}
		fmt.Fprintf(sb, "\n%s- checkbox %s [checked=%s] {%s}", indent, quote(node.Name), checked, hash)

	case "radio":
		checked := node.Checked
		if checked == "" {
			checked = "false"
		}
		fmt.Fprintf(sb, "\n%s- radio %s [checked=%s] {%s}", indent, quote(node.Name), checked, hash)

	case "combobox", "listbox":
		name := node.Name
		value := node.Value
		fmt.Fprintf(sb, "\n%s- %s %s [value=%s] {%s}", indent, node.Role, quote(name), quote(value), hash)

	case "option":
		name := node.Name
		selected := ""
		if node.Checked == "true" {
			selected = " [selected]"
		}
		if hash != "" {
			fmt.Fprintf(sb, "\n%s- option %s%s {%s}", indent, quote(name), selected, hash)
		} else {
			fmt.Fprintf(sb, "\n%s- option %s%s", indent, quote(name), selected)
		}

	case "menuitem", "menuitemcheckbox", "menuitemradio":
		name := node.Name
		fmt.Fprintf(sb, "\n%s- %s %s {%s}", indent, node.Role, quote(name), hash)

	case "tab":
		name := node.Name
		selected := ""
		if node.Checked == "true" {
			selected = " [selected]"
		}
		fmt.Fprintf(sb, "\n%s- tab %s%s {%s}", indent, quote(name), selected, hash)

	case "switch":
		checked := node.Checked
		if checked == "" {
			checked = "false"
		}
		fmt.Fprintf(sb, "\n%s- switch %s [checked=%s] {%s}", indent, quote(node.Name), checked, hash)

	case "slider":
		fmt.Fprintf(sb, "\n%s- slider %s [value=%s] {%s}", indent, quote(node.Name), quote(node.Value), hash)

	case "spinbutton":
		fmt.Fprintf(sb, "\n%s- spinbutton %s [value=%s] {%s}", indent, quote(node.Name), quote(node.Value), hash)

	case "img", "image":
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- img %s", indent, quote(node.Name))
		}

	case "StaticText", "text":
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- text %s", indent, quote(node.Name))
		}
		return // no children

	case "InlineTextBox":
		return // skip — parent StaticText has the text

	case "LineBreak":
		return // skip — tree indentation already conveys structure

	case "paragraph":
		sb.WriteString("\n" + indent + "- paragraph")
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "list":
		sb.WriteString("\n" + indent + "- list")
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "listitem":
		sb.WriteString("\n" + indent + "- listitem")
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "table":
		sb.WriteString("\n" + indent + "- table")
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "row":
		sb.WriteString("\n" + indent + "- row")
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "cell", "columnheader", "rowheader", "gridcell":
		name := node.Name
		if name != "" {
			fmt.Fprintf(sb, "\n%s- %s %s", indent, node.Role, quote(name))
		} else {
			fmt.Fprintf(sb, "\n%s- %s", indent, node.Role)
		}
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	case "separator":
		fmt.Fprintf(sb, "\n%s- separator", indent)

	case "navigation", "banner", "contentinfo", "main", "complementary",
		"region", "article", "section", "form", "group", "generic",
		"dialog", "alertdialog", "application", "document", "figure",
		"toolbar", "menu", "menubar", "tablist", "tabpanel",
		"tree", "treeitem", "grid", "status", "alert", "log",
		"marquee", "timer", "search", "directory", "feed",
		"math", "note", "presentation", "blockquote", "caption",
		"definition", "deletion", "emphasis", "insertion", "strong",
		"subscript", "superscript", "term", "time", "code":
		// Structural/landmark roles — show role with optional name, render children
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- %s %s", indent, node.Role, quote(node.Name))
		} else {
			sb.WriteString("\n" + indent + "- " + node.Role)
		}
		for _, child := range node.Children {
			renderNode(sb, child, depth+1, elements, focusedHash)
		}
		return

	default:
		// Unknown role — render name if leaf, otherwise recurse
		if len(node.Children) == 0 {
			if node.Name != "" {
				fmt.Fprintf(sb, "\n%s- %s %s", indent, node.Role, quote(node.Name))
			}
		} else {
			if node.Name != "" {
				fmt.Fprintf(sb, "\n%s- %s %s", indent, node.Role, quote(node.Name))
			}
			for _, child := range node.Children {
				renderNode(sb, child, depth+1, elements, focusedHash)
			}
		}
		return
	}
}

func isInteractiveRole(role string) bool {
	switch role {
	case "link", "button", "textbox", "searchbox", "checkbox", "radio",
		"combobox", "listbox", "menuitem", "menuitemcheckbox", "menuitemradio",
		"tab", "switch", "slider", "spinbutton", "option":
		return true
	}
	return false
}

func generateAXHash(backendDOMNodeID int, role string) string {
	input := fmt.Sprintf("%d:%s", backendDOMNodeID, role)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])[:8]
}

func collectText(node *AXNode) string {
	if node == nil {
		return ""
	}
	var parts []string
	for _, child := range node.Children {
		if child.Role == "StaticText" || child.Role == "text" {
			if child.Name != "" {
				parts = append(parts, child.Name)
			}
		} else {
			if t := collectText(child); t != "" {
				parts = append(parts, t)
			}
		}
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

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
