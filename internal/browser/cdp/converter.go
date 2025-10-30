package cdp

import (
	"fmt"
	"strings"
	"time"

	"github.com/hanpama/wb/pkg/ir"
)

// ConvertCDPToSnapshot converts CDP DOMSnapshot.captureSnapshot result to IR
func ConvertCDPToSnapshot(result map[string]any, url, title string) (*ir.PageSnapshot, error) {
	documents, ok := result["documents"].([]any)
	if !ok || len(documents) == 0 {
		return nil, fmt.Errorf("no documents in CDP response")
	}

	doc, ok := documents[0].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid document format")
	}

	cdpStrings, ok := result["strings"].([]any)
	if !ok {
		return nil, fmt.Errorf("no strings table in CDP response")
	}

	stringTable := make([]string, len(cdpStrings))
	for i, s := range cdpStrings {
		stringTable[i], _ = s.(string)
	}

	// Convert nodes
	root, err := convertNodes(doc, stringTable)
	if err != nil {
		return nil, fmt.Errorf("failed to convert nodes: %w", err)
	}

	snapshot := &ir.PageSnapshot{
		URL:       url,
		Title:     title,
		Timestamp: time.Now(),
		Root:      root,
	}

	return snapshot, nil
}

// convertNodes converts CDP node structure to IR node tree
func convertNodes(doc map[string]any, stringTable []string) (*ir.Node, error) {
	nodesData, ok := doc["nodes"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("no nodes in document")
	}

	// Extract node arrays
	nodeTypes := getIntArray(nodesData, "nodeType")
	nodeNames := getIntArray(nodesData, "nodeName")
	nodeValues := getIntArray(nodesData, "nodeValue")
	parentIndexes := getIntArray(nodesData, "parentIndex")
	attributes := getIntArrayArray(nodesData, "attributes")
	backendNodeIDs := getIntArray(nodesData, "backendNodeId")

	if len(nodeTypes) == 0 {
		return nil, fmt.Errorf("empty node arrays")
	}

	// Get layout information if available
	var layoutBounds [][]float64
	var layoutNodeIndexes []int
	if layoutData, ok := doc["layout"].(map[string]any); ok {
		layoutBounds = getFloatArrayArray(layoutData, "bounds")
		layoutNodeIndexes = getIntArray(layoutData, "nodeIndex")
	}

	// Create IR nodes
	nodes := make([]*ir.Node, len(nodeTypes))
	for i := range nodeTypes {
		node := &ir.Node{
			Type:       convertNodeType(nodeTypes[i]),
			Attributes: make(map[string]string),
		}

		// Set tag name
		if nodeNames[i] >= 0 && nodeNames[i] < len(stringTable) {
			node.Tag = stringTable[nodeNames[i]]
		}

		// Set text content
		if nodeValues[i] >= 0 && nodeValues[i] < len(stringTable) {
			node.Text = stringTable[nodeValues[i]]
		}

		// Set backend node ID
		if i < len(backendNodeIDs) {
			node.BackendNodeID = backendNodeIDs[i]
		}

		// Parse attributes
		if i < len(attributes) {
			attrs := attributes[i]
			for j := 0; j < len(attrs)-1; j += 2 {
				keyIdx := attrs[j]
				valIdx := attrs[j+1]

				if keyIdx >= 0 && keyIdx < len(stringTable) && valIdx >= 0 && valIdx < len(stringTable) {
					key := stringTable[keyIdx]
					val := stringTable[valIdx]
					node.Attributes[key] = val

					// Extract ID and classes
					if key == "id" {
						node.ID = val
					} else if key == "class" {
						node.Classes = strings.Split(val, " ")
					}
				}
			}
		}

		// Add layout information
		for li, layoutNodeIdx := range layoutNodeIndexes {
			if layoutNodeIdx == i && li < len(layoutBounds) {
				bounds := layoutBounds[li]
				if len(bounds) == 4 {
					node.BoundingBox = &ir.BoundingBox{
						X:      bounds[0],
						Y:      bounds[1],
						Width:  bounds[2],
						Height: bounds[3],
					}
				}
				break
			}
		}

		nodes[i] = node
	}

	// Build tree structure
	for i, parentIdx := range parentIndexes {
		if parentIdx >= 0 && parentIdx < len(nodes) {
			nodes[parentIdx].Children = append(nodes[parentIdx].Children, nodes[i])
		}
	}

	// Return root node (first node with no parent)
	if len(nodes) > 0 {
		return nodes[0], nil
	}

	return nil, fmt.Errorf("no root node found")
}

// convertNodeType converts CDP node type (int) to IR NodeType
func convertNodeType(nodeType int) ir.NodeType {
	switch nodeType {
	case 1: // ELEMENT_NODE
		return ir.NodeTypeElement
	case 3: // TEXT_NODE
		return ir.NodeTypeText
	default:
		return ir.NodeTypeElement
	}
}

// Helper functions to extract arrays from CDP response

func getIntArray(data map[string]any, key string) []int {
	arr, ok := data[key].([]any)
	if !ok {
		return nil
	}

	result := make([]int, len(arr))
	for i, v := range arr {
		if num, ok := v.(float64); ok {
			result[i] = int(num)
		}
	}
	return result
}

func getIntArrayArray(data map[string]any, key string) [][]int {
	arr, ok := data[key].([]any)
	if !ok {
		return nil
	}

	result := make([][]int, len(arr))
	for i, v := range arr {
		if subArr, ok := v.([]any); ok {
			result[i] = make([]int, len(subArr))
			for j, sv := range subArr {
				if num, ok := sv.(float64); ok {
					result[i][j] = int(num)
				}
			}
		}
	}
	return result
}

func getFloatArrayArray(data map[string]any, key string) [][]float64 {
	arr, ok := data[key].([]any)
	if !ok {
		return nil
	}

	result := make([][]float64, len(arr))
	for i, v := range arr {
		if subArr, ok := v.([]any); ok {
			result[i] = make([]float64, len(subArr))
			for j, sv := range subArr {
				if num, ok := sv.(float64); ok {
					result[i][j] = num
				}
			}
		}
	}
	return result
}
