// Package renderer provides display model for rendering browser content.
package renderer

import "strconv"

// InlineItem represents an inline element within text flow
type InlineItem interface {
	RenderInline() string
}

// BlockItem represents a block-level element that starts on a new line
type BlockItem interface {
	RenderBlock() string
}

// === Inline Items ===

// InlineText represents plain text content
type InlineText struct {
	Content string
}

func (t InlineText) RenderInline() string {
	return t.Content
}

// InlineLink represents a clickable link
type InlineLink struct {
	Hash string
	Text string
	URL  string
}

func (l InlineLink) RenderInline() string {
	return "[" + l.Text + "]{" + l.Hash + "}"
}

// InlineButton represents a clickable button
type InlineButton struct {
	Hash string
	Text string
}

func (b InlineButton) RenderInline() string {
	return "[" + b.Text + "]{" + b.Hash + "}"
}

// InlineImage represents an image
type InlineImage struct {
	Hash string
	Alt  string
	Src  string
}

func (img InlineImage) RenderInline() string {
	src := img.Src
	if len(src) > 50 {
		src = shortenURL(src, 50)
	}

	result := "![" + img.Alt + "](" + src + ")"
	if img.Hash != "" {
		result += "{" + img.Hash + "}"
	}
	return result
}

// InlineStrong represents bold text
type InlineStrong struct {
	Content []InlineItem
}

func (s InlineStrong) RenderInline() string {
	return "**" + renderInlineItems(s.Content) + "**"
}

// InlineEmphasis represents italic text
type InlineEmphasis struct {
	Content []InlineItem
}

func (e InlineEmphasis) RenderInline() string {
	return "*" + renderInlineItems(e.Content) + "*"
}

// InlineCode represents inline code
type InlineCode struct {
	Content string
}

func (c InlineCode) RenderInline() string {
	return "`" + c.Content + "`"
}

// InlineCheckbox represents a checkbox
type InlineCheckbox struct {
	Hash    string
	Checked bool
}

func (cb InlineCheckbox) RenderInline() string {
	checkbox := "[ ]"
	if cb.Checked {
		checkbox = "[✓]"
	}
	return checkbox + "{" + cb.Hash + "}"
}

// InlineRadio represents a radio button
type InlineRadio struct {
	Hash    string
	Checked bool
}

func (r InlineRadio) RenderInline() string {
	radio := "( )"
	if r.Checked {
		radio = "(•)"
	}
	return radio + "{" + r.Hash + "}"
}

// InlineInput represents a text input field
type InlineInput struct {
	Hash        string
	Type        string // text, email, password, etc.
	Value       string
	Placeholder string
}

func (ti InlineInput) RenderInline() string {
	inputType := ti.Type
	if inputType == "" {
		inputType = "text"
	}

	valueDisplay := ""
	if ti.Value != "" {
		valueDisplay = `"` + ti.Value + `"`
	} else if ti.Placeholder != "" {
		valueDisplay = "(" + ti.Placeholder + ")"
	} else {
		valueDisplay = "(empty)"
	}

	return "[Input/" + inputType + ": " + valueDisplay + "]{" + ti.Hash + "}"
}

// === Block Items ===

// Heading represents a markdown heading (h1-h6)
type Heading struct {
	Level   int // 1-6
	Content []InlineItem
}

func (h Heading) RenderBlock() string {
	prefix := ""
	for i := 0; i < h.Level; i++ {
		prefix += "#"
	}
	return "\n\n" + prefix + " " + renderInlineItems(h.Content)
}

// Paragraph represents a paragraph
type Paragraph struct {
	Content []InlineItem
}

func (p Paragraph) RenderBlock() string {
	return "\n\n" + renderInlineItems(p.Content)
}

// UnorderedListItem represents a list item in an unordered list
type UnorderedListItem struct {
	Content []InlineItem
}

func (li UnorderedListItem) RenderBlock() string {
	return "\n- " + renderInlineItems(li.Content)
}

// OrderedListItem represents a list item in an ordered list
type OrderedListItem struct {
	Index   int
	Content []InlineItem
}

func (li OrderedListItem) RenderBlock() string {
	return "\n" + strconv.Itoa(li.Index) + ". " + renderInlineItems(li.Content)
}

// LineBreak represents a line break
type LineBreak struct{}

func (lb LineBreak) RenderBlock() string {
	return "\n"
}

// === Helper functions ===

// renderInlineItems renders a slice of inline items to string
func renderInlineItems(items []InlineItem) string {
	if len(items) == 0 {
		return ""
	}

	var result string
	for i, item := range items {
		rendered := item.RenderInline()
		result += rendered

		// Add space between items unless it's the last item or already has trailing space
		if i < len(items)-1 && rendered != "" && rendered[len(rendered)-1] != ' ' {
			// Check if next item starts with punctuation or space
			if i+1 < len(items) {
				nextRendered := items[i+1].RenderInline()
				if len(nextRendered) > 0 {
					firstChar := nextRendered[0]
					// Don't add space before punctuation
					if firstChar != ',' && firstChar != '.' && firstChar != '!' && firstChar != '?' && firstChar != ' ' {
						result += " "
					}
				}
			}
		}
	}
	return result
}
