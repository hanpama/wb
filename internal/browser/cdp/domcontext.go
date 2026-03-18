package cdp

import (
	"context"
	"fmt"
	"strings"

	"github.com/hanpama/wb/internal/browser"
)

// DOMContextElement represents an element shown in DOM context
type DOMContextElement struct {
	Tag           string
	BackendNodeID int
	Hash          string
	IsSelf        bool
	TextPreview   string
}

// DOMContextResult holds the DOM context around an element
type DOMContextResult struct {
	Ancestors []DOMContextElement
	ParentTag string
	SelfTag   string
	SelfHTML  string
	Siblings  []DOMContextElement
	Children  []DOMContextElement
}

// GetDOMContext retrieves DOM context around a given backendNodeID
func (b *Backend) GetDOMContext(ctx context.Context, tabID browser.TabID, backendNodeID int) (*DOMContextResult, error) {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	// 1. Resolve to objectId
	resolveResult, err := tab.Client.SendCommand(ctx, "DOM.resolveNode", map[string]any{
		"backendNodeId": backendNodeID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to resolve node: %w", err)
	}
	object, ok := resolveResult["object"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid resolve result")
	}
	objectID, _ := object["objectId"].(string)

	// 2. Get context data via JavaScript
	jsResult, err := tab.Client.SendCommand(ctx, "Runtime.callFunctionOn", map[string]any{
		"objectId": objectID,
		"functionDeclaration": `function() {
			function openTag(el) {
				var s = '<' + el.tagName.toLowerCase();
				if (el.id) s += ' id="' + el.id + '"';
				var cls = (el.className && typeof el.className === 'string') ? el.className.trim() : '';
				if (cls) {
					if (cls.length > 60) cls = cls.substring(0, 57) + '...';
					s += ' class="' + cls + '"';
				}
				var attrs = ['type','name','href','src','for','role','value','placeholder','action','method','hx-get','aria-label'];
				for (var i = 0; i < attrs.length; i++) {
					var a = attrs[i];
					var v = el.getAttribute(a);
					if (v !== null) {
						if (v.length > 60) v = v.substring(0, 57) + '...';
						s += ' ' + a + '="' + v + '"';
					}
				}
				s += '>';
				return s;
			}
			function breadcrumb(el) {
				var tag = el.tagName.toLowerCase();
				if (el.id) return tag + '#' + el.id;
				return tag;
			}

			// Collect ancestors (skip html)
			var ancestorEls = [];
			var p = this.parentElement;
			while (p && p.tagName !== 'HTML') {
				ancestorEls.unshift(p);
				p = p.parentElement;
			}
			var ancestors = [];
			for (var i = 0; i < ancestorEls.length; i++) {
				ancestors.push({label: breadcrumb(ancestorEls[i])});
			}

			var selfTag = openTag(this);
			var selfHTML = this.outerHTML;
			if (selfHTML.length > 1000) selfHTML = selfHTML.substring(0, 1000) + '\n...truncated';
			var parentTag = this.parentElement ? openTag(this.parentElement) : '';

			// Siblings
			var siblingTags = [];
			var parent = this.parentElement;
			var selfIdx = -1;
			if (parent) {
				var ch = parent.children;
				for (var i = 0; i < ch.length; i++) {
					if (ch[i] === this) { selfIdx = i; break; }
				}
				var start = Math.max(0, selfIdx - 2);
				var end = Math.min(ch.length, selfIdx + 3);
				for (var i = start; i < end; i++) {
					var txt = ch[i].textContent || '';
					if (txt.length > 60) txt = txt.substring(0, 57) + '...';
					siblingTags.push({
						tag: openTag(ch[i]),
						isSelf: i === selfIdx,
						text: txt.replace(/\n/g, ' ').trim()
					});
				}
			}

			// Children
			var childTags = [];
			var childEls = this.children;
			var childLimit = Math.min(childEls.length, 10);
			for (var i = 0; i < childLimit; i++) {
				var txt = childEls[i].textContent || '';
				if (txt.length > 60) txt = txt.substring(0, 57) + '...';
				childTags.push({
					tag: openTag(childEls[i]),
					text: txt.replace(/\n/g, ' ').trim()
				});
			}

			// Build refs array: ancestors, then non-self siblings, then children
			window.__wb_refs = [];
			for (var i = 0; i < ancestorEls.length; i++) {
				window.__wb_refs.push(ancestorEls[i]);
			}
			if (parent) {
				var ch2 = parent.children;
				var start2 = Math.max(0, selfIdx - 2);
				var end2 = Math.min(ch2.length, selfIdx + 3);
				for (var i = start2; i < end2; i++) {
					if (i !== selfIdx) window.__wb_refs.push(ch2[i]);
				}
			}
			for (var i = 0; i < childLimit; i++) {
				window.__wb_refs.push(childEls[i]);
			}

			return {
				ancestors: ancestors,
				ancestorCount: ancestorEls.length,
				parentTag: parentTag,
				selfTag: selfTag,
				selfHTML: selfHTML,
				siblings: siblingTags,
				children: childTags,
				refCount: window.__wb_refs.length
			};
		}`,
		"returnByValue": true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get DOM context: %w", err)
	}

	resultObj, ok := jsResult["result"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid JS result")
	}
	data, ok := resultObj["value"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("invalid JS return value")
	}

	result := &DOMContextResult{
		ParentTag: strVal(data, "parentTag"),
		SelfTag:   strVal(data, "selfTag"),
		SelfHTML:  strVal(data, "selfHTML"),
	}

	// Parse ancestors
	if ancestors, ok := data["ancestors"].([]any); ok {
		for _, a := range ancestors {
			am, ok := a.(map[string]any)
			if !ok {
				continue
			}
			result.Ancestors = append(result.Ancestors, DOMContextElement{
				Tag: strVal(am, "label"),
			})
		}
	}

	// Parse siblings
	if siblings, ok := data["siblings"].([]any); ok {
		for _, s := range siblings {
			sm, ok := s.(map[string]any)
			if !ok {
				continue
			}
			isSelf, _ := sm["isSelf"].(bool)
			result.Siblings = append(result.Siblings, DOMContextElement{
				Tag:         strVal(sm, "tag"),
				IsSelf:      isSelf,
				TextPreview: strVal(sm, "text"),
			})
		}
	}

	// Parse children
	if children, ok := data["children"].([]any); ok {
		for _, c := range children {
			cm, ok := c.(map[string]any)
			if !ok {
				continue
			}
			result.Children = append(result.Children, DOMContextElement{
				Tag:         strVal(cm, "tag"),
				TextPreview: strVal(cm, "text"),
			})
		}
	}

	// 3. Resolve backendNodeIds for all refs: ancestors, siblings, children
	refCount := 0
	if rc, ok := data["refCount"].(float64); ok {
		refCount = int(rc)
	}

	if refCount > 0 {
		backendNodeIDs := b.resolveRefs(ctx, tab, refCount)
		refIdx := 0

		// Assign to ancestors
		for i := range result.Ancestors {
			if refIdx < len(backendNodeIDs) {
				bid := backendNodeIDs[refIdx]
				result.Ancestors[i].BackendNodeID = bid
				if bid > 0 {
					tag := extractTagName(result.Ancestors[i].Tag)
					if tag == "" {
						tag = result.Ancestors[i].Tag
					}
					result.Ancestors[i].Hash = generateAXHash(bid, tag)
				}
				refIdx++
			}
		}
		// Assign to siblings (non-self)
		for i := range result.Siblings {
			if result.Siblings[i].IsSelf {
				continue
			}
			if refIdx < len(backendNodeIDs) {
				bid := backendNodeIDs[refIdx]
				result.Siblings[i].BackendNodeID = bid
				if bid > 0 {
					tag := extractTagName(result.Siblings[i].Tag)
					result.Siblings[i].Hash = generateAXHash(bid, tag)
				}
				refIdx++
			}
		}
		// Assign to children
		for i := range result.Children {
			if refIdx < len(backendNodeIDs) {
				bid := backendNodeIDs[refIdx]
				result.Children[i].BackendNodeID = bid
				if bid > 0 {
					tag := extractTagName(result.Children[i].Tag)
					result.Children[i].Hash = generateAXHash(bid, tag)
				}
				refIdx++
			}
		}
	}

	// 4. Register ALL hashes for subsequent describe calls
	b.mu.Lock()
	if b.interactiveElements[tabID] == nil {
		b.interactiveElements[tabID] = make(map[string]int)
	}
	for _, a := range result.Ancestors {
		if a.Hash != "" && a.BackendNodeID > 0 {
			b.interactiveElements[tabID][a.Hash] = a.BackendNodeID
		}
	}
	for _, s := range result.Siblings {
		if s.Hash != "" && s.BackendNodeID > 0 {
			b.interactiveElements[tabID][s.Hash] = s.BackendNodeID
		}
	}
	for _, c := range result.Children {
		if c.Hash != "" && c.BackendNodeID > 0 {
			b.interactiveElements[tabID][c.Hash] = c.BackendNodeID
		}
	}
	b.mu.Unlock()

	return result, nil
}

// resolveRefs gets backendNodeIds for elements stored in window.__wb_refs
func (b *Backend) resolveRefs(ctx context.Context, tab *Tab, count int) []int {
	result := make([]int, count)

	evalResult, err := tab.Client.SendCommand(ctx, "Runtime.evaluate", map[string]any{
		"expression":    "window.__wb_refs",
		"returnByValue": false,
	})
	if err != nil {
		return result
	}
	evalObj, ok := evalResult["result"].(map[string]any)
	if !ok {
		return result
	}
	refsObjID, ok := evalObj["objectId"].(string)
	if !ok {
		return result
	}

	propsResult, err := tab.Client.SendCommand(ctx, "Runtime.getProperties", map[string]any{
		"objectId":      refsObjID,
		"ownProperties": true,
	})
	if err != nil {
		return result
	}
	props, ok := propsResult["result"].([]any)
	if !ok {
		return result
	}

	for _, p := range props {
		pm, ok := p.(map[string]any)
		if !ok {
			continue
		}
		name, _ := pm["name"].(string)
		idx := -1
		if _, err := fmt.Sscanf(name, "%d", &idx); err != nil || idx < 0 || idx >= count {
			continue
		}
		valObj, ok := pm["value"].(map[string]any)
		if !ok {
			continue
		}
		elemObjID, ok := valObj["objectId"].(string)
		if !ok {
			continue
		}
		descResult, err := tab.Client.SendCommand(ctx, "DOM.describeNode", map[string]any{
			"objectId": elemObjID,
			"depth":    0,
		})
		if err != nil {
			continue
		}
		node, ok := descResult["node"].(map[string]any)
		if !ok {
			continue
		}
		if bid, ok := node["backendNodeId"].(float64); ok {
			result[idx] = int(bid)
		}
	}

	return result
}

func strVal(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func extractTagName(openingTag string) string {
	s := strings.TrimPrefix(openingTag, "<")
	if idx := strings.IndexAny(s, " >"); idx > 0 {
		return s[:idx]
	}
	s = strings.TrimSuffix(s, ">")
	// Handle breadcrumb format "tag#id"
	if idx := strings.Index(s, "#"); idx > 0 {
		return s[:idx]
	}
	return s
}

// FormatDOMContext formats the DOM context result as a readable HTML view
func FormatDOMContext(result *DOMContextResult) string {
	var sb strings.Builder

	// Ancestor breadcrumb with hashes
	for i, a := range result.Ancestors {
		if i > 0 {
			sb.WriteString(" > ")
		}
		sb.WriteString(a.Tag)
		if a.Hash != "" {
			sb.WriteString(" {" + a.Hash + "}")
		}
	}
	sb.WriteString("\n\n")

	// Parent with siblings
	sb.WriteString(result.ParentTag)
	sb.WriteString("\n")

	for _, s := range result.Siblings {
		if s.IsSelf {
			lines := strings.Split(result.SelfHTML, "\n")
			for i, line := range lines {
				if i == 0 {
					sb.WriteString("  ★ " + line + "\n")
				} else {
					sb.WriteString("    " + line + "\n")
				}
			}
		} else {
			preview := s.TextPreview
			if len(preview) > 40 {
				preview = preview[:37] + "..."
			}
			hashStr := ""
			if s.Hash != "" {
				hashStr = " {" + s.Hash + "}"
			}
			if preview != "" {
				sb.WriteString(fmt.Sprintf("    %s %s ...%s\n", s.Tag, preview, hashStr))
			} else {
				sb.WriteString(fmt.Sprintf("    %s ...%s\n", s.Tag, hashStr))
			}
		}
	}

	if len(result.Children) > 0 {
		sb.WriteString("\nChildren:\n")
		for _, c := range result.Children {
			hashStr := ""
			if c.Hash != "" {
				hashStr = " {" + c.Hash + "}"
			}
			preview := c.TextPreview
			if len(preview) > 50 {
				preview = preview[:47] + "..."
			}
			if preview != "" {
				sb.WriteString(fmt.Sprintf("  %s %s%s\n", c.Tag, preview, hashStr))
			} else {
				sb.WriteString(fmt.Sprintf("  %s%s\n", c.Tag, hashStr))
			}
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}
