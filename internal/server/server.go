package server

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"strings"
	"sync"

	"github.com/hanpama/wb/internal/browser"
	"github.com/hanpama/wb/internal/browser/cdp"
	"github.com/hanpama/wb/internal/logging"
	"github.com/hanpama/wb/internal/renderer"
	"github.com/hanpama/wb/internal/service"
	"github.com/hanpama/wb/pkg/diff"
	"github.com/hanpama/wb/pkg/ir"
	"github.com/hanpama/wb/pkg/protocol"
)

const (
	DefaultPort       = "62066"
	DefaultLinesLimit = 100
)

// ServerState manages browser backend and application state
type ServerState struct {
	mu      sync.RWMutex
	backend browser.BrowserBackend

	activeTabID browser.TabID

	// Last snapshots shown to the user (for diff calculation)
	// Updated when user runs show/click/input/forward/back
	lastViewedSnapshots map[browser.TabID]*ir.PageSnapshot
}

// NewServerState initializes browser backend using Chrome DevTools Protocol
func NewServerState() (*ServerState, error) {
	// Use CDP backend
	backend := cdp.NewBackend()

	ctx := context.Background()
	if err := backend.Start(ctx); err != nil {
		return nil, fmt.Errorf("failed to start CDP backend: %w", err)
	}

	// Create initial tab
	tabID, err := backend.CreateTab(ctx, "about:blank")
	if err != nil {
		backend.Close()
		return nil, fmt.Errorf("failed to create initial tab: %w", err)
	}

	state := &ServerState{
		backend:             backend,
		lastViewedSnapshots: make(map[browser.TabID]*ir.PageSnapshot),
		activeTabID:         tabID,
	}

	return state, nil
}

// Close cleans up all resources
func (s *ServerState) Close() error {
	return s.backend.Close()
}

// RPCReceiver provides RPC methods for browser control
// It delegates business logic to the service layer
type RPCReceiver struct {
	state   *ServerState
	service *service.BrowserService
}

// Helper functions

// convertTabInfo converts service.TabInfo to protocol.TabInfo
func convertTabInfo(tabs []service.TabInfo) []protocol.TabInfo {
	result := make([]protocol.TabInfo, len(tabs))
	for i, tab := range tabs {
		result[i] = protocol.TabInfo{
			TabID:    tab.TabID,
			Title:    tab.Title,
			URL:      tab.URL,
			IsActive: tab.IsActive,
		}
	}
	return result
}

// convertPendingDialog converts service.PendingDialogInfo to protocol.PendingDialog slice
func convertPendingDialog(dialog *service.PendingDialogInfo) []protocol.PendingDialog {
	if dialog == nil {
		return nil
	}
	return []protocol.PendingDialog{
		{
			Type:    dialog.Type,
			Message: dialog.Message,
		},
	}
}

func paginateMarkdown(snapshot *ir.PageSnapshot, offset, limit int) (md string, totalLines, actualOffset, actualLimit int) {
	fullMarkdown := renderer.RenderBody(snapshot)
	lines := strings.Split(fullMarkdown, "\n")
	totalLines = len(lines)

	if limit == 0 {
		limit = DefaultLinesLimit
	}
	if offset < 0 {
		offset = 0
	}
	if offset >= totalLines {
		offset = totalLines - limit
		if offset < 0 {
			offset = 0
		}
	}

	endLine := offset + limit
	if endLine > totalLines {
		endLine = totalLines
	}

	paginatedLines := lines[offset:endLine]
	return strings.Join(paginatedLines, "\n"), totalLines, offset, limit
}


// RPC Methods

// Ping is a simple test method
func (r *RPCReceiver) Ping(args *protocol.PingArgs, reply *protocol.PingReply) error {
	reply.Message = "pong"
	return nil
}

// NewTab opens a new tab
func (r *RPCReceiver) NewTab(args *protocol.NewTabArgs, reply *protocol.NewTabReply) error {
	ctx := context.Background()

	// Use service layer to create tab and wait
	result, err := r.service.CreateTabAndWait(ctx, args.URL)
	if err != nil {
		return err
	}

	// Update state
	r.state.mu.Lock()
	r.state.activeTabID = result.TabID
	r.state.lastViewedSnapshots[result.TabID] = result.NewSnapshot
	r.state.mu.Unlock()

	// Paginate
	md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)

	reply.TabID = string(result.TabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	return nil
}

// Show refreshes the current view (no waiting, immediate snapshot)
func (r *RPCReceiver) Show(args *protocol.ShowArgs, reply *protocol.ShowReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// Get current snapshot immediately without waiting
	snapshot, err := r.service.GetSnapshot(ctx, tabID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = snapshot
	r.state.mu.Unlock()

	// Paginate
	md, totalLines, offset, limit := paginateMarkdown(snapshot, args.Offset, args.Limit)

	// Get metadata
	metadata := r.service.GetMetadata(ctx, tabID)

	reply.TabID = string(tabID)
	reply.Title = snapshot.Title
	reply.URL = snapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = snapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(metadata.PendingDialog)

	return nil
}

// Click clicks an element by hash
func (r *RPCReceiver) Click(args *protocol.ClickArgs, reply *protocol.ClickReply) error {
	logging.RPCRequestReceived("Click", map[string]interface{}{"hash": args.Hash})
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	lastViewed := r.state.lastViewedSnapshots[tabID]
	r.state.mu.RUnlock()

	// Use service layer to perform click and wait
	result, err := r.service.ClickAndWait(ctx, tabID, args.Hash)
	if err != nil {
		logging.RPCResponseSent("Click", false, map[string]interface{}{"error": err.Error()})
		return err
	}

	// Fill reply with result
	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	if result.DocumentChanged {
		// Full render for new document
		reply.URLChanged = true
		md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)
		reply.Markdown = md
		reply.TotalLines = totalLines
		reply.Offset = offset
		reply.Limit = limit
	} else {
		// Diff render for same document
		reply.URLChanged = false
		if lastViewed != nil {
			oldMd := renderer.RenderBody(lastViewed)
			newMd := renderer.RenderBody(result.NewSnapshot)
			reply.Diff = diff.RenderDiff(oldMd, newMd)
		}
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	logging.RPCResponseSent("Click", true, map[string]interface{}{"url_changed": result.DocumentChanged})
	return nil
}

// Type types text into the currently focused element
func (r *RPCReceiver) Type(args *protocol.TypeArgs, reply *protocol.TypeReply) error {
	logging.RPCRequestReceived("Type", map[string]interface{}{"text": args.Text})
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	lastViewed := r.state.lastViewedSnapshots[tabID]
	r.state.mu.RUnlock()

	// Use service layer to perform type and wait
	result, err := r.service.TypeAndWait(ctx, tabID, args.Text)
	if err != nil {
		logging.RPCResponseSent("Type", false, map[string]interface{}{"error": err.Error()})
		return err
	}

	// Fill reply with result
	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	if result.DocumentChanged {
		// Full render for new document
		reply.URLChanged = true
		md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)
		reply.Markdown = md
		reply.TotalLines = totalLines
		reply.Offset = offset
		reply.Limit = limit
	} else {
		// Diff render for same document
		reply.URLChanged = false
		if lastViewed != nil {
			oldMd := renderer.RenderBody(lastViewed)
			newMd := renderer.RenderBody(result.NewSnapshot)
			reply.Diff = diff.RenderDiff(oldMd, newMd)
		}
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	logging.RPCResponseSent("Type", true, map[string]interface{}{"url_changed": result.DocumentChanged})
	return nil
}

// Enter presses Enter on the currently focused element
func (r *RPCReceiver) Enter(args *protocol.EnterArgs, reply *protocol.EnterReply) error {
	logging.RPCRequestReceived("Enter", map[string]interface{}{})
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	lastViewed := r.state.lastViewedSnapshots[tabID]
	r.state.mu.RUnlock()

	// Use service layer to perform enter and wait
	result, err := r.service.EnterAndWait(ctx, tabID)
	if err != nil {
		logging.RPCResponseSent("Enter", false, map[string]interface{}{"error": err.Error()})
		return err
	}

	// Fill reply with result
	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	if result.DocumentChanged {
		// Full render for new document
		reply.URLChanged = true
		md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)
		reply.Markdown = md
		reply.TotalLines = totalLines
		reply.Offset = offset
		reply.Limit = limit
	} else {
		// Diff render for same document
		reply.URLChanged = false
		if lastViewed != nil {
			oldMd := renderer.RenderBody(lastViewed)
			newMd := renderer.RenderBody(result.NewSnapshot)
			reply.Diff = diff.RenderDiff(oldMd, newMd)
		}
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	logging.RPCResponseSent("Enter", true, map[string]interface{}{"url_changed": result.DocumentChanged})
	return nil
}

// GetStatus returns current browser status
func (r *RPCReceiver) GetStatus(args *protocol.GetStatusArgs, reply *protocol.GetStatusReply) error {
	ctx := context.Background()

	tabs, err := r.service.ListTabs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tabs: %w", err)
	}

	r.state.mu.RLock()
	activeTabID := r.state.activeTabID
	r.state.mu.RUnlock()

	var activeTitle string
	for _, tab := range tabs {
		if tab.ID == activeTabID {
			activeTitle = tab.Title
			break
		}
	}

	reply.TabCount = len(tabs)
	reply.ActiveTabID = string(activeTabID)
	reply.ActiveTabTitle = activeTitle

	return nil
}

// Open navigates to a URL in current tab
func (r *RPCReceiver) Open(args *protocol.OpenArgs, reply *protocol.OpenReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// If no active tab, create a new one
	if tabID == "" {
		newTabID, err := r.service.CreateTab(ctx, args.URL)
		if err != nil {
			return fmt.Errorf("failed to create tab: %w", err)
		}

		r.state.mu.Lock()
		r.state.activeTabID = newTabID
		tabID = newTabID
		r.state.mu.Unlock()
	}

	// Use service layer to navigate and wait
	result, err := r.service.NavigateAndWait(ctx, tabID, args.URL)
	if err != nil {
		return err
	}

	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)

	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	return nil
}

// Back navigates back in history (always full render)
func (r *RPCReceiver) Back(args *protocol.BackArgs, reply *protocol.BackReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// Use service layer to navigate back and wait
	result, err := r.service.BackAndWait(ctx, tabID)
	if err != nil {
		return err
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	// Full render (navigation always changes document)
	md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)

	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	return nil
}

// Forward navigates forward in history (always full render)
func (r *RPCReceiver) Forward(args *protocol.ForwardArgs, reply *protocol.ForwardReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// Use service layer to navigate forward and wait
	result, err := r.service.ForwardAndWait(ctx, tabID)
	if err != nil {
		return err
	}

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	// Full render (navigation always changes document)
	md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)

	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	return nil
}

// List returns all tabs
func (r *RPCReceiver) List(args *protocol.ListArgs, reply *protocol.ListReply) error {
	ctx := context.Background()

	tabs, err := r.service.ListTabs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tabs: %w", err)
	}

	r.state.mu.RLock()
	activeTabID := r.state.activeTabID
	r.state.mu.RUnlock()

	reply.Tabs = make([]protocol.TabInfo, len(tabs))
	for i, tab := range tabs {
		reply.Tabs[i] = protocol.TabInfo{
			TabID:    string(tab.ID),
			Title:    tab.Title,
			URL:      tab.URL,
			IsActive: tab.ID == activeTabID,
		}
	}

	return nil
}

// Switch switches to a different tab
func (r *RPCReceiver) Switch(args *protocol.SwitchArgs, reply *protocol.SwitchReply) error {
	ctx := context.Background()

	tabID := browser.TabID(args.TabID)

	// Verify tab exists
	tabs, err := r.service.ListTabs(ctx)
	if err != nil {
		return fmt.Errorf("failed to list tabs: %w", err)
	}

	found := false
	for _, tab := range tabs {
		if tab.ID == tabID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("tab not found: %s", tabID)
	}

	// Switch active tab
	r.state.mu.Lock()
	r.state.activeTabID = tabID
	r.state.mu.Unlock()

	// Get snapshot
	snapshot, err := r.service.GetSnapshot(ctx, tabID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = snapshot
	r.state.mu.Unlock()

	md, totalLines, offset, limit := paginateMarkdown(snapshot, 0, DefaultLinesLimit)

	// Get metadata
	metadata := r.service.GetMetadata(ctx, tabID)

	reply.TabID = string(tabID)
	reply.Title = snapshot.Title
	reply.URL = snapshot.URL
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit
	reply.FocusedHash = snapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(metadata.PendingDialog)

	return nil
}

// Close closes a tab
func (r *RPCReceiver) Close(args *protocol.CloseArgs, reply *protocol.CloseReply) error {
	ctx := context.Background()

	tabID := browser.TabID(args.TabID)
	if tabID == "" {
		r.state.mu.RLock()
		tabID = r.state.activeTabID
		r.state.mu.RUnlock()
	}

	if err := r.service.CloseTab(ctx, tabID); err != nil {
		return fmt.Errorf("failed to close tab: %w", err)
	}

	r.state.mu.Lock()
	delete(r.state.lastViewedSnapshots, tabID)
	if r.state.activeTabID == tabID {
		// Switch to another tab if available
		tabs, _ := r.service.ListTabs(ctx)
		if len(tabs) > 0 {
			r.state.activeTabID = tabs[0].ID
		}
	}
	r.state.mu.Unlock()

	tabs, _ := r.service.ListTabs(ctx)
	reply.Success = true
	reply.RemainingTabCount = len(tabs)

	return nil
}

// RespondToDialog responds to a pending JavaScript dialog
func (r *RPCReceiver) RespondToDialog(args *protocol.RespondToDialogArgs, reply *protocol.RespondToDialogReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// Create dialog response
	response := service.DialogResponse{
		Accept: args.Accept,
		Input:  args.Input,
	}

	// Use service layer to respond to dialog
	result, err := r.service.RespondToDialog(ctx, tabID, response)
	if err != nil {
		return err
	}

	// Fill reply
	reply.TabID = string(tabID)
	reply.Title = result.NewSnapshot.Title
	reply.URL = result.NewSnapshot.URL
	reply.FocusedHash = result.NewSnapshot.FocusedHash

	// Use metadata from service
	reply.ActiveRequestCount = result.Metadata.ActiveRequestCount
	reply.AllTabs = convertTabInfo(result.Metadata.AllTabs)
	reply.PendingDialogs = convertPendingDialog(result.Metadata.PendingDialog)

	// Render full page
	md, totalLines, offset, limit := paginateMarkdown(result.NewSnapshot, 0, DefaultLinesLimit)
	reply.Markdown = md
	reply.TotalLines = totalLines
	reply.Offset = offset
	reply.Limit = limit

	// Update last viewed snapshot
	r.state.mu.Lock()
	r.state.lastViewedSnapshots[tabID] = result.NewSnapshot
	r.state.mu.Unlock()

	return nil
}

// Describe describes an interactive element
func (r *RPCReceiver) Describe(args *protocol.DescribeArgs, reply *protocol.DescribeReply) error {
	r.state.mu.RLock()
	tabID := r.state.activeTabID
	snapshot := r.state.lastViewedSnapshots[tabID]
	r.state.mu.RUnlock()

	if snapshot == nil {
		return fmt.Errorf("no snapshot available")
	}

	// Find element by hash
	node, found := snapshot.InteractiveMap[args.Hash]
	if !found {
		reply.Found = false
		return nil
	}

	reply.Found = true
	reply.Hash = args.Hash
	reply.Tag = node.Tag
	reply.Selector = node.Selector
	reply.Attributes = node.Attributes // Include all HTML attributes

	if node.Interactive != nil {
		reply.Type = string(node.Interactive.Type)
		reply.Href = node.Interactive.Href
		reply.Placeholder = node.Interactive.Placeholder
		reply.Value = node.Interactive.Value
		reply.InputType = node.Interactive.InputType
	}

	// Get text content
	if len(node.Children) > 0 && node.Children[0].Type == ir.NodeTypeText {
		reply.Text = node.Children[0].Text
	}

	// Handle images
	if node.Tag == "img" {
		reply.ImgSrc = node.Attributes["src"]
		reply.ImgAlt = node.Attributes["alt"]
	}

	return nil
}

// DumpIR dumps the raw IR JSON structure for debugging
func (r *RPCReceiver) DumpIR(args *protocol.DumpIRArgs, reply *protocol.DumpIRReply) error {
	ctx := context.Background()

	r.state.mu.RLock()
	tabID := r.state.activeTabID
	r.state.mu.RUnlock()

	// Get current snapshot
	snapshot, err := r.service.GetSnapshot(ctx, tabID)
	if err != nil {
		return fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Marshal snapshot to JSON with indentation
	jsonBytes, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal snapshot to JSON: %w", err)
	}

	reply.JSON = string(jsonBytes)
	return nil
}

// Run starts the RPC server
func Run() {
	port := os.Getenv("WB_PORT")
	if port == "" {
		port = DefaultPort
	}

	log.Printf("[Server] Starting on port %s...", port)

	// Initialize server state
	state, err := NewServerState()
	if err != nil {
		log.Fatalf("[Server] Failed to initialize: %v", err)
	}
	defer state.Close()

	// Create service layer
	browserService := service.NewBrowserService(state.backend)

	// Create RPC receiver
	receiver := &RPCReceiver{
		state:   state,
		service: browserService,
	}
	// Register with explicit name "BrowserService" to maintain backward compatibility
	rpc.RegisterName("BrowserService", receiver)

	// Start listening
	listener, err := net.Listen("tcp", ":"+port)
	if err != nil {
		log.Fatalf("[Server] Failed to listen: %v", err)
	}
	defer listener.Close()

	log.Printf("[Server] Ready and listening on port %s", port)

	// Accept connections
	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[Server] Accept error: %v", err)
			continue
		}

		go jsonrpc.ServeConn(conn)
	}
}
