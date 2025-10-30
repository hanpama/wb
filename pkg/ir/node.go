// Package ir provides Intermediate Representation of web pages.
// This package contains pure data structures with zero external dependencies.
package ir

// NodeType represents the type of a DOM node
type NodeType string

const (
	NodeTypeElement NodeType = "element"
	NodeTypeText    NodeType = "text"
)

// InteractiveType represents the type of interactive element
type InteractiveType string

const (
	InteractiveLink     InteractiveType = "link"
	InteractiveButton   InteractiveType = "button"
	InteractiveInput    InteractiveType = "textinput"
	InteractiveTextarea InteractiveType = "textarea"
	InteractiveCheckbox InteractiveType = "checkbox"
	InteractiveRadio    InteractiveType = "radio"
	InteractiveSelect   InteractiveType = "select"
	InteractiveImage    InteractiveType = "image"
)

// Node represents a DOM node
type Node struct {
	Type NodeType `json:"type"`
	Tag  string   `json:"tag,omitempty"`
	Text string   `json:"text,omitempty"`

	// Identification
	ID            string            `json:"id,omitempty"`
	Classes       []string          `json:"classes,omitempty"`
	DOMPath       string            `json:"domPath,omitempty"`
	Selector      string            `json:"selector,omitempty"`
	Attributes    map[string]string `json:"attributes,omitempty"`
	BackendNodeID int               `json:"backendNodeId,omitempty"` // CDP backend node ID

	// Layout (from CDP)
	BoundingBox *BoundingBox `json:"boundingBox,omitempty"`

	// Interactive metadata
	Interactive *InteractiveInfo `json:"interactive,omitempty"`

	// Tree structure
	Children []*Node `json:"children,omitempty"`
}

// BoundingBox represents element layout position and size
type BoundingBox struct {
	X      float64 `json:"x"`
	Y      float64 `json:"y"`
	Width  float64 `json:"width"`
	Height float64 `json:"height"`
}

// InteractiveInfo contains information about interactive elements
type InteractiveInfo struct {
	Hash string          `json:"hash"`
	Type InteractiveType `json:"type"`

	// Event listeners
	HasClickListener  bool `json:"hasClickListener,omitempty"`
	HasInputListener  bool `json:"hasInputListener,omitempty"`
	HasSubmitListener bool `json:"hasSubmitListener,omitempty"`

	// Input properties
	InputType   string `json:"inputType,omitempty"`
	Placeholder string `json:"placeholder,omitempty"`
	Value       string `json:"value,omitempty"`

	// Link properties
	Href   string `json:"href,omitempty"`
	Target string `json:"target,omitempty"`

	// Accessibility
	Role      string `json:"role,omitempty"`
	AriaLabel string `json:"ariaLabel,omitempty"`

	// State
	Disabled bool `json:"disabled,omitempty"`
	Readonly bool `json:"readonly,omitempty"`
	Checked  bool `json:"checked,omitempty"`
	Selected bool `json:"selected,omitempty"`

	// Select options
	Options []SelectOption `json:"options,omitempty"`
}

// SelectOption represents an option in a select element
type SelectOption struct {
	Value string `json:"value"`
	Text  string `json:"text"`
}
