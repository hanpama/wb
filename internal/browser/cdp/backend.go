package cdp

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/hanpama/wb/internal/browser"
	"github.com/hanpama/wb/internal/logging"
	"github.com/hanpama/wb/pkg/ir"
)

// Backend implements browser.BrowserBackend using Chrome DevTools Protocol
type Backend struct {
	mu sync.RWMutex

	// Chrome process and connection
	chromeCmd *ChromeProcess
	wsURL     string
	client    *CDPClient

	// Tab management
	tabs      map[browser.TabID]*Tab
	nextTabID int

	// Interactive element tracking: tabID -> (hash -> BackendDOMNodeID)
	interactiveElements map[browser.TabID]map[string]int
}

type Tab struct {
	ID           browser.TabID
	TargetID     string
	WebSocketURL string
	Client       *CDPClient

	// Network request tracking
	mu                    sync.RWMutex
	inFlightRequests      map[string]*RequestInfo // requestId -> info
	currentLoaderID       string
	currentFrameID        string // main frame ID
	currentURL            string // current page URL (updated from Page.frameNavigated)
	navigationInProgress  bool   // true if main frame navigation in progress

	// Dialog tracking
	pendingDialog *ir.DialogInfo
}

// RequestInfo tracks an in-flight network request
type RequestInfo struct {
	RequestID string
	FrameID   string
	LoaderID  string
	URL       string
	StartTime time.Time
}

// NewBackend creates a new CDP backend
func NewBackend() *Backend {
	return &Backend{
		tabs:                make(map[browser.TabID]*Tab),
		interactiveElements: make(map[browser.TabID]map[string]int),
	}
}

// Start launches Chrome and establishes connection
func (b *Backend) Start(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Launch Chrome process
	chrome, err := LaunchChrome(ctx)
	if err != nil {
		return fmt.Errorf("failed to launch chrome: %w", err)
	}
	b.chromeCmd = chrome

	// Get browser-level WebSocket URL
	wsURL, err := chrome.GetWebSocketURL()
	if err != nil {
		chrome.Close()
		return fmt.Errorf("failed to get websocket url: %w", err)
	}
	b.wsURL = wsURL

	// Connect to browser-level WebSocket (for creating tabs)
	client, err := NewCDPClient(ctx, wsURL)
	if err != nil {
		chrome.Close()
		return fmt.Errorf("failed to connect to chrome: %w", err)
	}
	b.client = client

	return nil
}

// Stop closes all connections and terminates Chrome
func (b *Backend) Stop(ctx context.Context) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	// Close all tab connections
	for _, tab := range b.tabs {
		if tab.Client != nil {
			tab.Client.Close()
		}
	}

	// Close browser connection
	if b.client != nil {
		b.client.Close()
	}

	// Kill Chrome process
	if b.chromeCmd != nil {
		return b.chromeCmd.Close()
	}

	return nil
}

// CreateTab creates a new browser tab
func (b *Backend) CreateTab(ctx context.Context, url string) (browser.TabID, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	if url == "" {
		url = "about:blank"
	}

	// Create new target via CDP
	result, err := b.client.SendCommand(ctx, "Target.createTarget", map[string]any{
		"url": url,
	})
	if err != nil {
		return "", fmt.Errorf("failed to create target: %w", err)
	}

	targetID, ok := result["targetId"].(string)
	if !ok {
		return "", fmt.Errorf("invalid targetId in response")
	}

	// Get page list to find WebSocket URL for this target
	pages, err := b.chromeCmd.GetPageList()
	if err != nil {
		return "", fmt.Errorf("failed to get page list: %w", err)
	}

	var pageWSURL string
	for _, page := range pages {
		if page.ID == targetID {
			pageWSURL = page.WebSocketDebuggerURL
			break
		}
	}

	if pageWSURL == "" {
		return "", fmt.Errorf("could not find websocket url for target %s", targetID)
	}

	// Connect to page-level WebSocket
	pageClient, err := NewCDPClient(ctx, pageWSURL)
	if err != nil {
		return "", fmt.Errorf("failed to connect to page: %w", err)
	}

	// Enable necessary domains
	if _, err := pageClient.SendCommand(ctx, "Page.enable", nil); err != nil {
		pageClient.Close()
		return "", fmt.Errorf("failed to enable Page domain: %w", err)
	}

	if _, err := pageClient.SendCommand(ctx, "DOM.enable", nil); err != nil {
		pageClient.Close()
		return "", fmt.Errorf("failed to enable DOM domain: %w", err)
	}

	if _, err := pageClient.SendCommand(ctx, "Network.enable", nil); err != nil {
		pageClient.Close()
		return "", fmt.Errorf("failed to enable Network domain: %w", err)
	}

	if _, err := pageClient.SendCommand(ctx, "Accessibility.enable", nil); err != nil {
		pageClient.Close()
		return "", fmt.Errorf("failed to enable Accessibility domain: %w", err)
	}

	// Create tab entry
	b.nextTabID++
	tabID := browser.TabID(fmt.Sprintf("tab-%d", b.nextTabID))

	tab := &Tab{
		ID:               tabID,
		TargetID:         targetID,
		WebSocketURL:     pageWSURL,
		Client:           pageClient,
		inFlightRequests: make(map[string]*RequestInfo),
	}

	b.tabs[tabID] = tab

	// Start event listener for this tab
	go b.handleTabEvents(tabID)

	// Start periodic cleanup for this tab
	go b.startRequestCleanup(tabID)

	return tabID, nil
}

// CloseTab closes a browser tab
func (b *Backend) CloseTab(ctx context.Context, tabID browser.TabID) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	tab, ok := b.tabs[tabID]
	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Clean up in-flight requests map to prevent memory leak
	tab.mu.Lock()
	tab.inFlightRequests = nil
	tab.mu.Unlock()

	// Close WebSocket connection
	if tab.Client != nil {
		tab.Client.Close()
	}

	// Close target via CDP
	if _, err := b.client.SendCommand(ctx, "Target.closeTarget", map[string]any{
		"targetId": tab.TargetID,
	}); err != nil {
		return fmt.Errorf("failed to close target: %w", err)
	}

	delete(b.tabs, tabID)
	return nil
}

// Navigate navigates to a URL
func (b *Backend) Navigate(ctx context.Context, tabID browser.TabID, url string) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	_, err := tab.Client.SendCommand(ctx, "Page.navigate", map[string]any{
		"url": url,
	})
	if err != nil {
		return fmt.Errorf("failed to navigate: %w", err)
	}

	// Don't wait for Page.loadEventFired here - let the caller use WaitForStable instead
	// This is more reliable as it waits for actual network stability rather than a single event

	return nil
}

// GetSnapshot captures the current page snapshot using the accessibility tree
func (b *Backend) GetSnapshot(ctx context.Context, tabID browser.TabID) (*ir.PageSnapshot, error) {
	logging.BackendMethodStart("GetSnapshot", map[string]interface{}{"tab.id": tabID})

	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		logging.BackendMethodEnd("GetSnapshot", false, map[string]interface{}{"tab.id": tabID, "error": "tab not found"})
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	// Get current URL from tab.currentURL (updated by Page.frameNavigated event)
	tab.mu.RLock()
	url := tab.currentURL
	tab.mu.RUnlock()

	// Get title from Target.getTargetInfo
	var title string
	if url == "" {
		result, err := tab.Client.SendCommand(ctx, "Target.getTargetInfo", map[string]any{
			"targetId": tab.TargetID,
		})
		if err != nil {
			logging.BackendMethodEnd("GetSnapshot", false, map[string]interface{}{"tab.id": tabID, "error": "failed to get target info"})
			return nil, fmt.Errorf("failed to get target info: %w", err)
		}
		if targetInfo, ok := result["targetInfo"].(map[string]any); ok {
			url, _ = targetInfo["url"].(string)
			title, _ = targetInfo["title"].(string)
		}
	} else {
		result, err := tab.Client.SendCommand(ctx, "Target.getTargetInfo", map[string]any{
			"targetId": tab.TargetID,
		})
		if err == nil {
			if targetInfo, ok := result["targetInfo"].(map[string]any); ok {
				title, _ = targetInfo["title"].(string)
			}
		}
	}

	// Get accessibility tree with timeout
	captureCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	axResult, err := tab.Client.SendCommand(captureCtx, "Accessibility.getFullAXTree", map[string]any{})
	if err != nil {
		// Check if timeout due to dialog
		if errors.Is(err, context.DeadlineExceeded) {
			tab.mu.RLock()
			hasDialog := tab.pendingDialog != nil
			tab.mu.RUnlock()

			if hasDialog {
				logging.Debug("AX tree timeout due to dialog", map[string]interface{}{"tab.id": tabID})
				logging.BackendMethodEnd("GetSnapshot", true, map[string]interface{}{"tab.id": tabID, "has_dialog": true})
				return &ir.PageSnapshot{
					URL:   url,
					Title: title,
				}, nil
			}
		}
		logging.BackendMethodEnd("GetSnapshot", false, map[string]interface{}{"tab.id": tabID, "error": "failed to get AX tree"})
		return nil, fmt.Errorf("failed to get AX tree: %w", err)
	}

	// Parse and render AX tree
	root := ParseAXTree(axResult)
	content, elementInfoMap, focusedHash := RenderAXSnapshot(root)

	logging.Debug("AX snapshot rendered", map[string]interface{}{"tab.id": tabID, "element.count": len(elementInfoMap)})

	snapshot := &ir.PageSnapshot{
		URL:            url,
		Title:          title,
		Content:        content,
		InteractiveMap: elementInfoMap,
		FocusedHash:    focusedHash,
	}

	// Store hash -> backendDOMNodeID mapping for this tab
	b.mu.Lock()
	hashMap := make(map[string]int)
	for hash, elem := range elementInfoMap {
		hashMap[hash] = elem.BackendDOMNodeID
	}
	b.interactiveElements[tabID] = hashMap
	b.mu.Unlock()

	// Include pending dialog info
	tab.mu.RLock()
	if tab.pendingDialog != nil {
		snapshot.PendingDialog = tab.pendingDialog
		logging.Debug("Pending dialog included in snapshot", map[string]interface{}{"tab.id": tabID, "dialog.type": tab.pendingDialog.Type})
	}
	tab.mu.RUnlock()

	logging.BackendMethodEnd("GetSnapshot", true, map[string]interface{}{"tab.id": tabID, "url": url})
	return snapshot, nil
}


// SetViewport sets the viewport size for the given tab
func (b *Backend) SetViewport(ctx context.Context, tabID browser.TabID, width, height int) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()
	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	_, err := tab.Client.SendCommand(ctx, "Emulation.setDeviceMetricsOverride", map[string]any{
		"width":             width,
		"height":            height,
		"deviceScaleFactor": 1,
		"mobile":            false,
	})
	if err != nil {
		return fmt.Errorf("failed to set viewport: %w", err)
	}
	return nil
}

// CaptureScreenshot captures a PNG screenshot of the current tab
func (b *Backend) CaptureScreenshot(ctx context.Context, tabID browser.TabID, fullPage bool) ([]byte, error) {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	params := map[string]any{
		"format": "png",
	}

	if fullPage {
		// Get full document size
		metricsResult, err := tab.Client.SendCommand(ctx, "Page.getLayoutMetrics", nil)
		if err != nil {
			return nil, fmt.Errorf("failed to get layout metrics: %w", err)
		}
		contentSize, ok := metricsResult["contentSize"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("invalid contentSize in layout metrics")
		}
		width, _ := contentSize["width"].(float64)
		height, _ := contentSize["height"].(float64)
		if width > 0 && height > 0 {
			params["captureBeyondViewport"] = true
			params["clip"] = map[string]any{
				"x":      0,
				"y":      0,
				"width":  width,
				"height": height,
				"scale":  1,
			}
		}
	}

	result, err := tab.Client.SendCommand(ctx, "Page.captureScreenshot", params)
	if err != nil {
		return nil, fmt.Errorf("failed to capture screenshot: %w", err)
	}

	dataStr, ok := result["data"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid screenshot data")
	}

	data, err := base64.StdEncoding.DecodeString(dataStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode screenshot: %w", err)
	}

	return data, nil
}

// Eval executes a JavaScript expression in the current tab and returns the result as JSON
func (b *Backend) Eval(ctx context.Context, tabID browser.TabID, expression string) (string, error) {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()
	if !ok {
		return "", fmt.Errorf("tab not found: %s", tabID)
	}

	result, err := tab.Client.SendCommand(ctx, "Runtime.evaluate", map[string]any{
		"expression":    expression,
		"returnByValue": true,
	})
	if err != nil {
		return "", fmt.Errorf("eval failed: %w", err)
	}

	resultObj, ok := result["result"].(map[string]any)
	if !ok {
		return "", fmt.Errorf("invalid eval result")
	}

	// Check for exceptions
	if exDesc, ok := result["exceptionDetails"].(map[string]any); ok {
		if text, ok := exDesc["text"].(string); ok {
			return "", fmt.Errorf("JS error: %s", text)
		}
	}

	value := resultObj["value"]
	if value == nil {
		return "undefined", nil
	}

	// If it's a string, return directly
	if s, ok := value.(string); ok {
		return s, nil
	}

	// Otherwise JSON-encode
	jsonBytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Sprintf("%v", value), nil
	}
	return string(jsonBytes), nil
}

// Click clicks on an element using CDP native methods
// selector format: "backend:<backendNodeId>"
func (b *Backend) Click(ctx context.Context, tabID browser.TabID, selector string) error {
	logging.BackendMethodStart("Click", map[string]interface{}{
		"tab.id":   tabID,
		"selector": selector,
	})

	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		logging.BackendMethodEnd("Click", false, map[string]interface{}{
			"error": "tab not found",
		})
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Parse backend node ID from selector
	var backendNodeID int
	if _, err := fmt.Sscanf(selector, "backend:%d", &backendNodeID); err != nil {
		logging.BackendMethodEnd("Click", false, map[string]interface{}{
			"error": "invalid selector format",
		})
		return fmt.Errorf("invalid selector format: %s", selector)
	}

	// 1. Resolve BackendNodeId to ObjectId
	logging.Debug("Click: Calling DOM.resolveNode", map[string]interface{}{
		"tab.id":         tabID,
		"backendNodeID": backendNodeID,
	})
	cmdCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	resolveResult, err := tab.Client.SendCommand(cmdCtx, "DOM.resolveNode", map[string]any{
		"backendNodeId": backendNodeID,
	})
	cancel()
	if err != nil {
		logging.BackendMethodEnd("Click", false, map[string]interface{}{
			"error": "DOM.resolveNode failed",
		})
		return fmt.Errorf("failed to resolve node: %w", err)
	}
	logging.Debug("Click: DOM.resolveNode completed", map[string]interface{}{
		"tab.id": tabID,
	})

	object, ok := resolveResult["object"].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid object in resolve result")
	}

	objectID, ok := object["objectId"].(string)
	if !ok {
		return fmt.Errorf("invalid objectId")
	}

	// 2. Try to scroll element into view (best effort)
	// Some elements may not be scrollable (e.g., fixed position elements)
	logging.Debug("Click: Calling DOM.scrollIntoViewIfNeeded", map[string]interface{}{
		"tab.id": tabID,
	})
	scrollCtx, scrollCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	_, _ = tab.Client.SendCommand(scrollCtx, "DOM.scrollIntoViewIfNeeded", map[string]any{
		"objectId": objectID,
	})
	scrollCancel()
	logging.Debug("Click: DOM.scrollIntoViewIfNeeded completed", map[string]interface{}{
		"tab.id": tabID,
	})

	// 3. Try coordinate-based click, fall back to JS click
	clicked := false

	boxCtx, boxCancel := context.WithTimeout(ctx, 1*time.Second)
	boxResult, boxErr := tab.Client.SendCommand(boxCtx, "DOM.getBoxModel", map[string]any{
		"objectId": objectID,
	})
	boxCancel()

	if boxErr == nil {
		model, ok := boxResult["model"].(map[string]any)
		if ok {
			content, ok := model["content"].([]any)
			if ok && len(content) >= 8 {
				x1, _ := content[0].(float64)
				y1, _ := content[1].(float64)
				x3, _ := content[4].(float64)
				y3, _ := content[5].(float64)
				centerX := (x1 + x3) / 2
				centerY := (y1 + y3) / 2

				pressCtx, pressCancel := context.WithTimeout(ctx, 1*time.Second)
				_, err = tab.Client.SendCommand(pressCtx, "Input.dispatchMouseEvent", map[string]any{
					"type":       "mousePressed",
					"x":          centerX,
					"y":          centerY,
					"button":     "left",
					"clickCount": 1,
				})
				pressCancel()
				if err == nil {
					releaseCtx, releaseCancel := context.WithTimeout(ctx, 1*time.Second)
					_, _ = tab.Client.SendCommand(releaseCtx, "Input.dispatchMouseEvent", map[string]any{
						"type":       "mouseReleased",
						"x":          centerX,
						"y":          centerY,
						"button":     "left",
						"clickCount": 1,
					})
					releaseCancel()
					clicked = true
				}
			}
		}
	}

	// Fallback: JS click for elements without box model (hidden, zero-size, etc.)
	if !clicked {
		logging.Debug("Click: box model unavailable, falling back to JS click", map[string]interface{}{
			"tab.id": tabID,
		})
		clickCtx, clickCancel := context.WithTimeout(ctx, 1*time.Second)
		_, err = tab.Client.SendCommand(clickCtx, "Runtime.callFunctionOn", map[string]any{
			"objectId":            objectID,
			"functionDeclaration": "function() { this.focus(); this.click(); }",
		})
		clickCancel()
		if err != nil {
			logging.BackendMethodEnd("Click", false, map[string]interface{}{
				"error": "JS click fallback failed",
			})
			return fmt.Errorf("failed to click: %w", err)
		}
	}

	logging.BackendMethodEnd("Click", true, map[string]interface{}{
		"tab.id": tabID,
	})
	return nil
}

// Type types text into the currently focused element
// This does NOT press Enter - use Enter() for that
func (b *Backend) Type(ctx context.Context, tabID browser.TabID, text string) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Handle special characters like \t, \n
	for _, char := range text {
		switch char {
		case '\t':
			// Tab key
			_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown",
				"key":  "Tab",
				"code": "Tab",
			})
			_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp",
				"key":  "Tab",
				"code": "Tab",
			})
		case '\n':
			// Newline - insert as text (not Enter key)
			_, _ = tab.Client.SendCommand(ctx, "Input.insertText", map[string]any{
				"text": "\n",
			})
		case '\b':
			// Backspace
			_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
				"type": "keyDown",
				"key":  "Backspace",
				"code": "Backspace",
			})
			_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
				"type": "keyUp",
				"key":  "Backspace",
				"code": "Backspace",
			})
		default:
			// Regular character - send as single character
			_, err := tab.Client.SendCommand(ctx, "Input.insertText", map[string]any{
				"text": string(char),
			})
			if err != nil {
				return fmt.Errorf("failed to insert text: %w", err)
			}
		}
	}

	return nil
}

// Input is deprecated - use Type instead
// Keeping for backward compatibility
func (b *Backend) Input(ctx context.Context, tabID browser.TabID, selector string, value string) error {
	// Click the element first to focus it
	if err := b.Click(ctx, tabID, selector); err != nil {
		return err
	}

	// Select all text first (Ctrl+A)
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type":      "keyDown",
		"key":       "a",
		"code":      "KeyA",
		"modifiers": 2, // Ctrl (Windows) / Cmd (Mac)
	})
	_, _ = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type":      "keyUp",
		"key":       "a",
		"code":      "KeyA",
		"modifiers": 2,
	})

	// Type the value
	return b.Type(ctx, tabID, value)
}

// Enter presses the Enter key (for form submission, etc.)
func (b *Backend) Enter(ctx context.Context, tabID browser.TabID) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Dispatch Enter keyDown
	_, err := tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type": "keyDown",
		"key":  "Enter",
		"code": "Enter",
	})
	if err != nil {
		return fmt.Errorf("failed to dispatch Enter keyDown: %w", err)
	}

	// Dispatch Enter keyUp
	_, err = tab.Client.SendCommand(ctx, "Input.dispatchKeyEvent", map[string]any{
		"type": "keyUp",
		"key":  "Enter",
		"code": "Enter",
	})
	if err != nil {
		return fmt.Errorf("failed to dispatch Enter keyUp: %w", err)
	}

	return nil
}

// NavigateBack navigates back in history
func (b *Backend) NavigateBack(ctx context.Context, tabID browser.TabID) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Get navigation history
	historyResult, err := tab.Client.SendCommand(ctx, "Page.getNavigationHistory", nil)
	if err != nil {
		return fmt.Errorf("failed to get navigation history: %w", err)
	}

	currentIndex, ok := historyResult["currentIndex"].(float64)
	if !ok {
		return fmt.Errorf("invalid currentIndex in history")
	}

	// Check if we can go back
	if int(currentIndex) <= 0 {
		return fmt.Errorf("cannot navigate back: already at the first page")
	}

	entries, ok := historyResult["entries"].([]any)
	if !ok {
		return fmt.Errorf("invalid entries in history")
	}

	// Get previous entry
	prevIndex := int(currentIndex) - 1
	if prevIndex >= len(entries) {
		return fmt.Errorf("invalid history index")
	}

	prevEntry, ok := entries[prevIndex].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid entry at index %d", prevIndex)
	}

	entryID, ok := prevEntry["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid entry id")
	}

	// Navigate to history entry
	_, err = tab.Client.SendCommand(ctx, "Page.navigateToHistoryEntry", map[string]any{
		"entryId": int(entryID),
	})
	if err != nil {
		return fmt.Errorf("failed to navigate to history entry: %w", err)
	}

	return nil
}

// NavigateForward navigates forward in history
func (b *Backend) NavigateForward(ctx context.Context, tabID browser.TabID) error {
	b.mu.RLock()
	tab, ok := b.tabs[tabID]
	b.mu.RUnlock()

	if !ok {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Get navigation history
	historyResult, err := tab.Client.SendCommand(ctx, "Page.getNavigationHistory", nil)
	if err != nil {
		return fmt.Errorf("failed to get navigation history: %w", err)
	}

	currentIndex, ok := historyResult["currentIndex"].(float64)
	if !ok {
		return fmt.Errorf("invalid currentIndex in history")
	}

	entries, ok := historyResult["entries"].([]any)
	if !ok {
		return fmt.Errorf("invalid entries in history")
	}

	// Check if we can go forward
	if int(currentIndex) >= len(entries)-1 {
		return fmt.Errorf("cannot navigate forward: already at the last page")
	}

	// Get next entry
	nextIndex := int(currentIndex) + 1
	nextEntry, ok := entries[nextIndex].(map[string]any)
	if !ok {
		return fmt.Errorf("invalid entry at index %d", nextIndex)
	}

	entryID, ok := nextEntry["id"].(float64)
	if !ok {
		return fmt.Errorf("invalid entry id")
	}

	// Navigate to history entry
	_, err = tab.Client.SendCommand(ctx, "Page.navigateToHistoryEntry", map[string]any{
		"entryId": int(entryID),
	})
	if err != nil {
		return fmt.Errorf("failed to navigate to history entry: %w", err)
	}

	return nil
}

// GetSelector returns the CSS selector for a given hash
// For CDP backend, we actually return a special format: "backend:<backendNodeId>"
// which Click/Input methods will recognize and use directly
func (b *Backend) GetSelector(ctx context.Context, tabID browser.TabID, hash string) (string, error) {
	logging.BackendMethodStart("GetSelector", map[string]interface{}{
		"tab.id": tabID,
		"hash":   hash,
	})

	b.mu.RLock()
	hashMap, ok := b.interactiveElements[tabID]
	b.mu.RUnlock()

	if !ok {
		logging.BackendMethodEnd("GetSelector", false, map[string]interface{}{
			"error": "no interactive elements for tab",
		})
		return "", fmt.Errorf("no interactive elements for tab %s", tabID)
	}

	backendNodeID, ok := hashMap[hash]
	if !ok {
		logging.BackendMethodEnd("GetSelector", false, map[string]interface{}{
			"error": "hash not found",
		})
		return "", fmt.Errorf("hash %s not found", hash)
	}

	// Return a special format that our Click/Input methods will recognize
	selector := fmt.Sprintf("backend:%d", backendNodeID)
	logging.BackendMethodEnd("GetSelector", true, map[string]interface{}{
		"tab.id":   tabID,
		"selector": selector,
	})
	return selector, nil
}

// HandleDialog handles a pending dialog
func (b *Backend) HandleDialog(ctx context.Context, tabID browser.TabID, accept bool, input string) error {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	tab.mu.Lock()
	if tab.pendingDialog == nil {
		tab.mu.Unlock()
		return fmt.Errorf("no pending dialog")
	}
	dialogType := tab.pendingDialog.Type
	tab.mu.Unlock()

	// Send CDP Page.handleJavaScriptDialog command
	params := map[string]any{
		"accept": accept,
	}

	// For prompt dialogs, include the input text if accepting
	if dialogType == "prompt" && accept && input != "" {
		params["promptText"] = input
	}

	_, err := tab.Client.SendCommand(ctx, "Page.handleJavaScriptDialog", params)
	if err != nil {
		return fmt.Errorf("failed to handle dialog: %w", err)
	}

	return nil
}

// GetPendingDialog returns information about a pending dialog
func (b *Backend) GetPendingDialog(ctx context.Context, tabID browser.TabID) (*browser.DialogInfo, error) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return nil, fmt.Errorf("tab not found: %s", tabID)
	}

	tab.mu.RLock()
	defer tab.mu.RUnlock()

	if tab.pendingDialog == nil {
		return nil, nil
	}

	return &browser.DialogInfo{
		Type:    tab.pendingDialog.Type,
		Message: tab.pendingDialog.Message,
	}, nil
}

// ListTabs returns information about all tabs
func (b *Backend) ListTabs(ctx context.Context) ([]browser.TabInfo, error) {
	// Get tab list with minimal lock time
	b.mu.RLock()
	tabCopies := make([]struct {
		ID       browser.TabID
		TargetID string
		Client   *CDPClient
	}, 0, len(b.tabs))
	for id, tab := range b.tabs {
		tabCopies = append(tabCopies, struct {
			ID       browser.TabID
			TargetID string
			Client   *CDPClient
		}{ID: id, TargetID: tab.TargetID, Client: tab.Client})
	}
	b.mu.RUnlock()

	// Now query each tab without holding the lock
	tabs := make([]browser.TabInfo, 0, len(tabCopies))
	for _, tabCopy := range tabCopies {
		result, err := tabCopy.Client.SendCommand(ctx, "Target.getTargetInfo", map[string]any{
			"targetId": tabCopy.TargetID,
		})

		url := ""
		title := ""
		if err == nil {
			if targetInfo, ok := result["targetInfo"].(map[string]any); ok {
				url, _ = targetInfo["url"].(string)
				title, _ = targetInfo["title"].(string)
			}
		}

		tabs = append(tabs, browser.TabInfo{
			ID:    tabCopy.ID,
			Title: title,
			URL:   url,
		})
	}

	return tabs, nil
}

// DiscoverAndTrackNewTabs finds any new Chrome tabs that aren't being tracked yet
// and starts tracking them. Returns list of newly discovered tab IDs.
func (b *Backend) DiscoverAndTrackNewTabs(ctx context.Context) ([]browser.TabID, error) {
	// Get current tracked tab target IDs
	b.mu.RLock()
	trackedTargets := make(map[string]bool)
	for _, tab := range b.tabs {
		trackedTargets[tab.TargetID] = true
	}
	nextID := b.nextTabID
	b.mu.RUnlock()

	// Get all pages from Chrome
	pages, err := b.chromeCmd.GetPageList()
	if err != nil {
		return nil, fmt.Errorf("failed to get page list: %w", err)
	}

	// Find new tabs
	var newTabIDs []browser.TabID
	for _, page := range pages {
		if page.Type != "page" {
			continue
		}
		if trackedTargets[page.ID] {
			continue // Already tracking this one
		}

		// This is a new tab - start tracking it
		pageClient, err := NewCDPClient(ctx, page.WebSocketDebuggerURL)
		if err != nil {
			continue // Skip if we can't connect
		}

		// Enable necessary domains
		if _, err := pageClient.SendCommand(ctx, "Page.enable", nil); err != nil {
			pageClient.Close()
			continue
		}
		if _, err := pageClient.SendCommand(ctx, "DOM.enable", nil); err != nil {
			pageClient.Close()
			continue
		}
		if _, err := pageClient.SendCommand(ctx, "Network.enable", nil); err != nil {
			pageClient.Close()
			continue
		}
		if _, err := pageClient.SendCommand(ctx, "Accessibility.enable", nil); err != nil {
			pageClient.Close()
			continue
		}

		// Create tab entry
		nextID++
		tabID := browser.TabID(fmt.Sprintf("tab-%d", nextID))

		tab := &Tab{
			ID:               tabID,
			TargetID:         page.ID,
			WebSocketURL:     page.WebSocketDebuggerURL,
			Client:           pageClient,
			inFlightRequests: make(map[string]*RequestInfo),
		}

		b.mu.Lock()
		b.tabs[tabID] = tab
		b.nextTabID = nextID
		b.mu.Unlock()

		// Start event listener for this tab
		go b.handleTabEvents(tabID)
		go b.startRequestCleanup(tabID)

		newTabIDs = append(newTabIDs, tabID)
	}

	return newTabIDs, nil
}

// Close closes the backend and cleans up resources
func (b *Backend) Close() error {
	return b.Stop(context.Background())
}

// Network request tracking and event handling

const (
	requestTimeout  = 30 * time.Second // Requests older than this are cleaned up
	cleanupInterval = 60 * time.Second // Cleanup runs every minute
	maxTrackedRequests = 1000          // Maximum number of tracked requests per tab
)

// handleTabEvents listens to CDP events for a specific tab
func (b *Backend) handleTabEvents(tabID browser.TabID) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	for {
		select {
		case event := <-tab.Client.events:
			logging.EventReceived(event.Method, map[string]interface{}{"tab.id": tabID})
			switch event.Method {
			case "Network.requestWillBeSent":
				b.handleRequestWillBeSent(tabID, event.Params)
			case "Network.loadingFinished":
				b.handleLoadingFinished(tabID, event.Params)
			case "Network.loadingFailed":
				b.handleLoadingFailed(tabID, event.Params)
			case "Page.frameStartedNavigating":
				b.handleFrameStartedNavigating(tabID, event.Params)
			case "Page.frameNavigated":
				b.handleFrameNavigated(tabID, event.Params)
			case "Page.javascriptDialogOpening":
				b.handleDialogOpening(tabID, event.Params)
			case "Page.javascriptDialogClosed":
				b.handleDialogClosed(tabID, event.Params)
			}
		case <-tab.Client.done:
			return
		}
	}
}

// handleRequestWillBeSent tracks new network requests
func (b *Backend) handleRequestWillBeSent(tabID browser.TabID, params map[string]any) {
	requestID, _ := params["requestId"].(string)
	if requestID == "" {
		return
	}

	request, _ := params["request"].(map[string]any)
	url, _ := request["url"].(string)

	// Ignore WebSocket connections
	if strings.HasPrefix(url, "ws://") || strings.HasPrefix(url, "wss://") {
		return
	}

	frameID, _ := params["frameId"].(string)
	loaderID, _ := params["loaderId"].(string)

	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Initialize currentLoaderID if not set yet (for initial page load)
	if tab.currentLoaderID == "" && loaderID != "" {
		tab.currentLoaderID = loaderID
		tab.currentFrameID = frameID
	}

	// Enforce maximum tracked requests to prevent memory leaks
	if len(tab.inFlightRequests) >= maxTrackedRequests {
		// Find and remove oldest request
		var oldestID string
		var oldestTime time.Time
		for id, req := range tab.inFlightRequests {
			if oldestID == "" || req.StartTime.Before(oldestTime) {
				oldestID = id
				oldestTime = req.StartTime
			}
		}
		if oldestID != "" {
			delete(tab.inFlightRequests, oldestID)
		}
	}

	tab.inFlightRequests[requestID] = &RequestInfo{
		RequestID: requestID,
		FrameID:   frameID,
		LoaderID:  loaderID,
		URL:       url,
		StartTime: time.Now(),
	}
}

// handleLoadingFinished removes completed requests
func (b *Backend) handleLoadingFinished(tabID browser.TabID, params map[string]any) {
	requestID, _ := params["requestId"].(string)
	if requestID == "" {
		return
	}

	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab != nil {
		tab.mu.Lock()
		delete(tab.inFlightRequests, requestID)
		tab.mu.Unlock()
	}
}

// handleLoadingFailed removes failed requests (including ERR_ABORTED, cancellations)
func (b *Backend) handleLoadingFailed(tabID browser.TabID, params map[string]any) {
	// Same as handleLoadingFinished - just remove from tracking
	b.handleLoadingFinished(tabID, params)
}

// handleFrameStartedNavigating marks navigation as in progress
func (b *Backend) handleFrameStartedNavigating(tabID browser.TabID, params map[string]any) {
	frameID, _ := params["frameId"].(string)

	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Check if this is main frame (no parentId in the frame data)
	// For frameStartedNavigating, we don't have full frame object, so check frameID
	if frameID == tab.currentFrameID || tab.currentFrameID == "" {
		tab.navigationInProgress = true
	}
}

// handleFrameNavigated updates current loader and frame IDs, and cleans up old requests
func (b *Backend) handleFrameNavigated(tabID browser.TabID, params map[string]any) {
	frame, ok := params["frame"].(map[string]any)
	if !ok {
		return
	}

	frameID, _ := frame["id"].(string)
	loaderID, _ := frame["loaderId"].(string)
	url, _ := frame["url"].(string)

	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	tab.mu.Lock()
	defer tab.mu.Unlock()

	// Main frame is the one without a parentId
	_, hasParent := frame["parentId"]
	isMainFrame := !hasParent

	// Update current frame/loader for main frame navigation only
	if isMainFrame {
		oldLoaderID := tab.currentLoaderID
		tab.currentFrameID = frameID
		tab.currentLoaderID = loaderID
		tab.currentURL = url // Update URL from frameNavigated event
		tab.navigationInProgress = false // Navigation completed

		// Clean up requests from previous navigation (different loaderID)
		if oldLoaderID != "" && oldLoaderID != loaderID {
			for reqID, req := range tab.inFlightRequests {
				if req.LoaderID == oldLoaderID {
					delete(tab.inFlightRequests, reqID)
				}
			}
		}
	}
}

// startRequestCleanup periodically removes stale requests
func (b *Backend) startRequestCleanup(tabID browser.TabID) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	ticker := time.NewTicker(cleanupInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			b.cleanupStaleRequests(tabID)
		case <-tab.Client.done:
			return
		}
	}
}

// cleanupStaleRequests removes requests that have been pending too long
func (b *Backend) cleanupStaleRequests(tabID browser.TabID) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	now := time.Now()
	tab.mu.Lock()
	defer tab.mu.Unlock()

	for reqID, req := range tab.inFlightRequests {
		if now.Sub(req.StartTime) > requestTimeout {
			delete(tab.inFlightRequests, reqID)
		}
	}
}

// GetActiveRequestCount returns the number of in-flight requests for the current loader
func (b *Backend) GetActiveRequestCount(tabID browser.TabID) int {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return 0
	}

	tab.mu.RLock()
	defer tab.mu.RUnlock()

	// Count only requests matching current loaderID (current navigation context)
	count := 0
	for _, req := range tab.inFlightRequests {
		if req.LoaderID == tab.currentLoaderID {
			count++
		}
	}

	return count
}

// WaitForStable waits until there are no active network requests and navigation is complete
func (b *Backend) WaitForStable(ctx context.Context, tabID browser.TabID, timeout time.Duration) error {
	logging.BackendMethodStart("WaitForStable", map[string]interface{}{"tab.id": tabID, "timeout": timeout.String()})

	deadline := time.Now().Add(timeout)
	stableThreshold := 500 * time.Millisecond // Must be stable for 500ms

	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		logging.BackendMethodEnd("WaitForStable", false, map[string]interface{}{"tab.id": tabID, "error": "tab not found"})
		return fmt.Errorf("tab not found: %s", tabID)
	}

	for time.Now().Before(deadline) {
		// Check for dialog first - if dialog exists, exit immediately
		tab.mu.RLock()
		hasDialog := tab.pendingDialog != nil
		tab.mu.RUnlock()

		if hasDialog {
			logging.Debug("Dialog detected during wait, exiting early", map[string]interface{}{"tab.id": tabID})
			logging.BackendMethodEnd("WaitForStable", true, map[string]interface{}{"tab.id": tabID, "reason": "dialog"})
			return nil // Dialog detected - consider this stable (dialog blocks page)
		}

		count := b.GetActiveRequestCount(tabID)

		tab.mu.RLock()
		navInProgress := tab.navigationInProgress
		tab.mu.RUnlock()

		if count == 0 && !navInProgress {
			// Wait a bit more to ensure it stays stable
			// Split the wait into smaller intervals to detect dialogs quickly
			stableStart := time.Now()
			for time.Since(stableStart) < stableThreshold {
				// Check for dialog during stable wait
				tab.mu.RLock()
				hasDialogDuringWait := tab.pendingDialog != nil
				tab.mu.RUnlock()

				if hasDialogDuringWait {
					logging.Debug("Dialog detected during stable threshold wait", map[string]interface{}{"tab.id": tabID})
					logging.BackendMethodEnd("WaitForStable", true, map[string]interface{}{"tab.id": tabID, "reason": "dialog_during_threshold"})
					return nil
				}

				time.Sleep(50 * time.Millisecond) // Check every 50ms
			}

			// Final check after loop - dialog may have appeared right at the end
			tab.mu.RLock()
			stillNavInProgress := tab.navigationInProgress
			stillHasDialog := tab.pendingDialog != nil
			tab.mu.RUnlock()

			// Exit if dialog appeared during wait (including the final moments)
			if stillHasDialog {
				logging.Debug("Dialog detected after stable threshold", map[string]interface{}{"tab.id": tabID})
				logging.BackendMethodEnd("WaitForStable", true, map[string]interface{}{"tab.id": tabID, "reason": "dialog_after_threshold"})
				return nil
			}

			if b.GetActiveRequestCount(tabID) == 0 && !stillNavInProgress {
				logging.BackendMethodEnd("WaitForStable", true, map[string]interface{}{"tab.id": tabID, "reason": "stable"})
				return nil // Stable!
			}
		}

		// Check for context cancellation
		select {
		case <-ctx.Done():
			logging.BackendMethodEnd("WaitForStable", false, map[string]interface{}{"tab.id": tabID, "error": "context cancelled"})
			return ctx.Err()
		default:
		}

		time.Sleep(100 * time.Millisecond)
	}

	// Timeout is not an error - we proceed anyway (practical approach)
	logging.BackendMethodEnd("WaitForStable", true, map[string]interface{}{"tab.id": tabID, "reason": "timeout"})
	return nil
}

// handleDialogOpening handles Page.javascriptDialogOpening event
func (b *Backend) handleDialogOpening(tabID browser.TabID, params map[string]any) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	dialogType, _ := params["type"].(string)
	message, _ := params["message"].(string)
	defaultPrompt, _ := params["defaultPrompt"].(string)

	logging.Info("JavaScript dialog opened", map[string]interface{}{
		"tab.id":       tabID,
		"dialog.type":  dialogType,
		"dialog.message": message,
	})

	tab.mu.Lock()
	tab.pendingDialog = &ir.DialogInfo{
		Type:         dialogType,
		Message:      message,
		DefaultValue: defaultPrompt,
	}
	tab.mu.Unlock()
}

// handleDialogClosed handles Page.javascriptDialogClosed event
func (b *Backend) handleDialogClosed(tabID browser.TabID, params map[string]any) {
	b.mu.RLock()
	tab := b.tabs[tabID]
	b.mu.RUnlock()

	if tab == nil {
		return
	}

	tab.mu.Lock()
	tab.pendingDialog = nil
	tab.mu.Unlock()
}
