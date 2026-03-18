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
		if bid, ok := m["backendDOMNodeId"].(float64); ok {
			node.BackendDOMNodeID = int(bid)
		}

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

// ============================================================================
// Phase 1: Normalize — clean the AX tree before rendering
// ============================================================================

// NormalizeAXTree produces a clean tree ready for rendering:
//   - Removes: InlineTextBox, LineBreak, decorative img (no name)
//   - Flattens: generic, none, Ignored, RootWebArea, WebArea (children promoted to parent)
//   - Merges: adjacent text-like siblings into a single text node
func NormalizeAXTree(node *AXNode) *AXNode {
	if node == nil {
		return nil
	}
	// Normalize all children recursively
	var normalized []*AXNode
	for _, child := range node.Children {
		normalized = appendNormalized(normalized, child)
	}
	normalized = mergeAdjacentText(normalized)

	// If root itself is transparent, wrap children in a synthetic root
	switch node.Role {
	case "RootWebArea", "WebArea":
		node.Role = "RootWebArea"
		node.Children = normalized
		return node
	}

	node.Children = normalized
	return node
}

// appendNormalized adds a child to the list, flattening transparent nodes
// and skipping removed nodes.
func appendNormalized(dst []*AXNode, node *AXNode) []*AXNode {
	if node == nil {
		return dst
	}

	switch node.Role {
	// Remove: produce no output, no children worth keeping
	case "InlineTextBox", "LineBreak":
		return dst

	// Remove: decorative images (no alt text)
	case "img", "image":
		if node.Name == "" {
			return dst
		}
		node.Children = nil // img has no meaningful children
		return append(dst, node)

	// Flatten: promote children to parent level
	case "generic", "none", "Ignored", "IgnoredRole", "RootWebArea", "WebArea":
		for _, child := range node.Children {
			dst = appendNormalized(dst, child)
		}
		return dst
	}

	// Keep: normalize children recursively
	var normalized []*AXNode
	for _, child := range node.Children {
		normalized = appendNormalized(normalized, child)
	}
	normalized = mergeAdjacentText(normalized)
	node.Children = normalized

	return append(dst, node)
}

// isTextLike returns true if a node is purely textual and can be merged
// with adjacent text. Only StaticText/text qualifies — all other roles
// (even non-interactive ones like time, emphasis, strong) preserve their
// structure and break merging.
func isTextLike(node *AXNode) bool {
	if node == nil {
		return false
	}
	return node.Role == "StaticText" || node.Role == "text"
}

// mergeAdjacentText merges consecutive text-like siblings into one text node.
// Non-text nodes break the run. This is lossless: StaticText/text nodes
// contain nothing but their Name string, so merging preserves all content.
func mergeAdjacentText(children []*AXNode) []*AXNode {
	if len(children) == 0 {
		return nil
	}

	var result []*AXNode
	var textParts []string

	flush := func() {
		if len(textParts) > 0 {
			merged := strings.TrimSpace(strings.Join(textParts, " "))
			if merged != "" {
				result = append(result, &AXNode{
					Role: "StaticText",
					Name: merged,
				})
			}
			textParts = nil
		}
	}

	for _, child := range children {
		if isTextLike(child) && child.Name != "" {
			textParts = append(textParts, child.Name)
		} else if isTextLike(child) {
			// empty text node — skip silently
			continue
		} else {
			flush()
			result = append(result, child)
		}
	}
	flush()

	return result
}

// ============================================================================
// Phase 2: Render — convert normalized tree to text + register hashes
// ============================================================================

// RenderAXSnapshot normalizes then renders an AX tree.
// Returns the rendered content, interactive element map, and focused hash.
func RenderAXSnapshot(root *AXNode) (string, map[string]*ir.ElementInfo, string) {
	if root == nil {
		return "", nil, ""
	}

	root = NormalizeAXTree(root)

	elements := make(map[string]*ir.ElementInfo)
	var focusedHash string
	var sb strings.Builder

	renderNode(&sb, root, 0, elements, &focusedHash)

	content := strings.TrimSpace(sb.String())
	for strings.Contains(content, "\n\n\n") {
		content = strings.ReplaceAll(content, "\n\n\n", "\n\n")
	}

	return content, elements, focusedHash
}

func renderNode(sb *strings.Builder, node *AXNode, depth int, elements map[string]*ir.ElementInfo, focusedHash *string) {
	if node == nil {
		return
	}

	// Root container — render children only, no self
	if node.Role == "RootWebArea" || node.Role == "WebArea" {
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

	// === Render self ===
	switch node.Role {
	case "StaticText", "text":
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- text %s", indent, quote(node.Name))
		}
		return // leaf

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
		fmt.Fprintf(sb, "\n%s- %s %s [value=%s] {%s}", indent, node.Role, quote(node.Name), quote(node.Value), hash)

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
		fmt.Fprintf(sb, "\n%s- %s %s [value=%s] {%s}", indent, node.Role, quote(node.Name), quote(node.Value), hash)

	case "option":
		selected := ""
		if node.Checked == "true" {
			selected = " [selected]"
		}
		if hash != "" {
			fmt.Fprintf(sb, "\n%s- option %s%s {%s}", indent, quote(node.Name), selected, hash)
		} else {
			fmt.Fprintf(sb, "\n%s- option %s%s", indent, quote(node.Name), selected)
		}

	case "menuitem", "menuitemcheckbox", "menuitemradio":
		fmt.Fprintf(sb, "\n%s- %s %s {%s}", indent, node.Role, quote(node.Name), hash)

	case "tab":
		selected := ""
		if node.Checked == "true" {
			selected = " [selected]"
		}
		fmt.Fprintf(sb, "\n%s- tab %s%s {%s}", indent, quote(node.Name), selected, hash)

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

	case "img", "image":
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- img %s", indent, quote(node.Name))
		}

	case "separator":
		fmt.Fprintf(sb, "\n%s- separator", indent)

	default:
		if node.Name != "" {
			fmt.Fprintf(sb, "\n%s- %s %s", indent, node.Role, quote(node.Name))
		} else if len(node.Children) > 0 {
			sb.WriteString("\n" + indent + "- " + node.Role)
		}
	}

	// === Always render children ===
	for _, child := range node.Children {
		renderNode(sb, child, depth+1, elements, focusedHash)
	}
}

// ============================================================================
// Helpers
// ============================================================================

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
	if node.Role == "StaticText" || node.Role == "text" {
		if node.Name != "" {
			return node.Name
		}
		return ""
	}
	for _, child := range node.Children {
		if t := collectText(child); t != "" {
			parts = append(parts, t)
		}
	}
	return strings.Join(parts, " ")
}

func quote(s string) string {
	return fmt.Sprintf("%q", s)
}

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
