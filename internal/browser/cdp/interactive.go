package cdp

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hanpama/wb/pkg/ir"
)

// InteractiveElement represents an interactive element extracted from IR
type InteractiveElement struct {
	Node          *ir.Node
	Hash          string
	Type          ir.InteractiveType
	BackendNodeID int
	DisplayText   string
	Href          string
	InputType     string
	Value         string
	Placeholder   string
}

// ExtractInteractiveElements walks the IR tree and extracts interactive elements
func ExtractInteractiveElements(snapshot *ir.PageSnapshot) []*InteractiveElement {
	var elements []*InteractiveElement

	walkNodes(snapshot.Root, func(node *ir.Node) {
		if isInteractive(node) {
			elem := &InteractiveElement{
				Node:          node,
				Hash:          generateHash(node),
				Type:          determineType(node),
				BackendNodeID: node.BackendNodeID,
				DisplayText:   extractText(node),
			}

			// Extract type-specific information
			switch elem.Type {
			case ir.InteractiveLink:
				elem.Href = node.Attributes["href"]
			case ir.InteractiveInput, ir.InteractiveTextarea:
				elem.InputType = node.Attributes["type"]
				if elem.InputType == "" {
					elem.InputType = "text"
				}
				elem.Placeholder = node.Attributes["placeholder"]
				elem.Value = node.Attributes["value"]
			}

			// Set Interactive info on the node itself with all properties
			interactiveInfo := &ir.InteractiveInfo{
				Hash: elem.Hash,
				Type: elem.Type,
			}

			// Copy type-specific properties
			switch elem.Type {
			case ir.InteractiveLink:
				interactiveInfo.Href = node.Attributes["href"]
				interactiveInfo.Target = node.Attributes["target"]
			case ir.InteractiveInput, ir.InteractiveTextarea:
				interactiveInfo.InputType = elem.InputType
				interactiveInfo.Placeholder = elem.Placeholder
				interactiveInfo.Value = elem.Value
				interactiveInfo.Disabled = node.Attributes["disabled"] == "true" || node.Attributes["disabled"] == "disabled"
				interactiveInfo.Readonly = node.Attributes["readonly"] == "true" || node.Attributes["readonly"] == "readonly"
			case ir.InteractiveCheckbox, ir.InteractiveRadio:
				interactiveInfo.Checked = node.Attributes["checked"] == "true" || node.Attributes["checked"] == "checked"
				interactiveInfo.Disabled = node.Attributes["disabled"] == "true" || node.Attributes["disabled"] == "disabled"
			}

			// Copy accessibility attributes
			interactiveInfo.Role = node.Attributes["role"]
			interactiveInfo.AriaLabel = node.Attributes["aria-label"]

			node.Interactive = interactiveInfo

			elements = append(elements, elem)
		}
	})

	return elements
}

// BuildInteractiveMap creates a hash -> node mapping for quick lookup
func BuildInteractiveMap(elements []*InteractiveElement) map[string]*ir.Node {
	m := make(map[string]*ir.Node)
	for _, elem := range elements {
		m[elem.Hash] = elem.Node
	}
	return m
}

// isInteractive determines if a node is interactive based on tag and attributes
func isInteractive(node *ir.Node) bool {
	if node.Type != ir.NodeTypeElement {
		return false
	}

	tag := strings.ToLower(node.Tag)

	// Interactive tags
	interactiveTags := map[string]bool{
		"a":        true,
		"button":   true,
		"input":    true,
		"select":   true,
		"textarea": true,
	}

	if interactiveTags[tag] {
		// Exclude hidden inputs
		if tag == "input" && node.Attributes["type"] == "hidden" {
			return false
		}
		return true
	}

	// Check onclick attribute
	if _, ok := node.Attributes["onclick"]; ok {
		return true
	}

	// Check role attribute
	role := node.Attributes["role"]
	if role == "button" || role == "link" {
		return true
	}

	return false
}

// determineType determines the InteractiveType based on tag and attributes
func determineType(node *ir.Node) ir.InteractiveType {
	tag := strings.ToLower(node.Tag)

	switch tag {
	case "a":
		return ir.InteractiveLink
	case "button":
		return ir.InteractiveButton
	case "input":
		inputType := strings.ToLower(node.Attributes["type"])
		switch inputType {
		case "checkbox":
			return ir.InteractiveCheckbox
		case "radio":
			return ir.InteractiveRadio
		default:
			return ir.InteractiveInput
		}
	case "textarea":
		return ir.InteractiveTextarea
	case "select":
		return ir.InteractiveSelect
	default:
		// Element with onclick or role
		role := node.Attributes["role"]
		if role == "button" {
			return ir.InteractiveButton
		}
		if role == "link" {
			return ir.InteractiveLink
		}
		return ir.InteractiveButton // Default for onclick elements
	}
}

// extractText extracts visible text from a node
func extractText(node *ir.Node) string {
	if node == nil {
		return ""
	}

	var texts []string
	walkNodes(node, func(n *ir.Node) {
		if n.Type == ir.NodeTypeText && strings.TrimSpace(n.Text) != "" {
			texts = append(texts, strings.TrimSpace(n.Text))
		}
	})

	return strings.Join(texts, " ")
}

// generateHash generates a unique hash for an interactive element
// Uses BackendNodeID which is stable within a page snapshot
func generateHash(node *ir.Node) string {
	// BackendNodeID is unique within a page and stable across snapshots
	input := fmt.Sprintf("%d:%s", node.BackendNodeID, node.Tag)
	hash := sha256.Sum256([]byte(input))

	// Return first 8 characters of hex
	return hex.EncodeToString(hash[:])[:8]
}

// walkNodes recursively walks the IR tree
func walkNodes(node *ir.Node, fn func(*ir.Node)) {
	if node == nil {
		return
	}

	fn(node)

	for _, child := range node.Children {
		walkNodes(child, fn)
	}
}
