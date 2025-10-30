package service

import (
	"context"
	"fmt"
	"time"

	"github.com/hanpama/wb/internal/browser"
	"github.com/hanpama/wb/internal/browser/cdp"
	"github.com/hanpama/wb/internal/logging"
	"github.com/hanpama/wb/pkg/ir"
)

// BrowserService provides high-level browser operations
// by orchestrating backend calls
type BrowserService struct {
	backend browser.BrowserBackend
}

// NewBrowserService creates a new browser service
func NewBrowserService(backend browser.BrowserBackend) *BrowserService {
	return &BrowserService{
		backend: backend,
	}
}

// checkDialogAndReturn checks for pending dialog and returns early if found
// Returns true if dialog exists, false otherwise
// This is a lightweight check that doesn't call GetSnapshot
func (s *BrowserService) checkDialogAndReturn(ctx context.Context, tabID browser.TabID) bool {
	dialog, _ := s.backend.GetPendingDialog(ctx, tabID)
	if dialog == nil {
		return false
	}
	logging.Debug("Dialog found in check", map[string]interface{}{"tab.id": tabID, "dialog.message": dialog.Message})
	return true
}

// getSnapshotForDialog gets a snapshot when a dialog is present
// This returns an empty snapshot when a dialog is active
// Dialog blocks page rendering, so we don't show any content
func (s *BrowserService) getSnapshotForDialog(ctx context.Context, tabID browser.TabID) *ir.PageSnapshot {
	// Return empty snapshot - dialog blocks the page
	return &ir.PageSnapshot{
		URL:   "",
		Title: "",
	}
}

// TabInfo contains information about a tab for display
type TabInfo struct {
	TabID    string
	Title    string
	URL      string
	IsActive bool
}

// PendingDialogInfo contains information about a pending JavaScript dialog
type PendingDialogInfo struct {
	Type    string
	Message string
}

// Metadata contains common metadata for all operations
type Metadata struct {
	ActiveRequestCount int
	AllTabs            []TabInfo
	PendingDialog      *PendingDialogInfo
}

// ClickResult contains the result of a click operation
type ClickResult struct {
	NewSnapshot     *ir.PageSnapshot
	DocumentChanged bool
	Metadata        Metadata
}

// ClickAndWait performs a click and waits for the page to stabilize
func (s *BrowserService) ClickAndWait(ctx context.Context, tabID browser.TabID, hash string) (*ClickResult, error) {
	// BEFORE check: If dialog already exists, don't proceed with click
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &ClickResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get selector for hash
	selector, err := s.backend.GetSelector(ctx, tabID, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to get selector for hash %s: %w", hash, err)
	}

	// Get current URL before click
	oldSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot before click: %w", err)
	}

	// Perform click
	if err := s.backend.Click(ctx, tabID, selector); err != nil {
		return nil, fmt.Errorf("failed to click: %w", err)
	}

	// Wait for stable state (includes dialog detection)
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 5*time.Second)
	}

	// Check for dialog after WaitForStable (may have arrived in the gap)
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &ClickResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get new snapshot after stability
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// FINAL check: Dialog may have arrived between checkDialogAndReturn and GetSnapshot completion
	// If so, return the snapshot we got (which won't have dialog info yet) with dialog metadata
	if s.checkDialogAndReturn(ctx, tabID) {
		logging.Debug("Dialog detected after GetSnapshot in ClickAndWait", map[string]interface{}{"tab.id": tabID})
		metadata := s.GetMetadata(ctx, tabID)
		return &ClickResult{
			NewSnapshot:     newSnapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Check if document changed
	documentChanged := oldSnapshot.URL != newSnapshot.URL

	// Collect metadata (includes new tabs automatically tracked in background)
	metadata := s.GetMetadata(ctx, tabID)

	return &ClickResult{
		NewSnapshot:     newSnapshot,
		DocumentChanged: documentChanged,
		Metadata:        metadata,
	}, nil
}

// NavigateResult contains the result of a navigation operation
type NavigateResult struct {
	NewSnapshot *ir.PageSnapshot
	Metadata    Metadata
}

// NavigateAndWait performs navigation and waits for the page to load
func (s *BrowserService) NavigateAndWait(ctx context.Context, tabID browser.TabID, url string) (*NavigateResult, error) {
	// BEFORE check: If dialog exists, cannot navigate - show dialog instead
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &NavigateResult{
			NewSnapshot: snapshot,
			Metadata:    metadata,
		}, nil
	}

	// No dialog - proceed with navigation
	if err := s.backend.Navigate(ctx, tabID, url); err != nil {
		return nil, fmt.Errorf("failed to navigate: %w", err)
	}

	// Wait for page to stabilize
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 30*time.Second)
	}

	// Get snapshot
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &NavigateResult{
		NewSnapshot: newSnapshot,
		Metadata:    metadata,
	}, nil
}

// BackAndWait navigates back and waits for the page to load
func (s *BrowserService) BackAndWait(ctx context.Context, tabID browser.TabID) (*NavigateResult, error) {
	// BEFORE check: If dialog exists, cannot navigate - show dialog instead
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &NavigateResult{
			NewSnapshot: snapshot,
			Metadata:    metadata,
		}, nil
	}

	// No dialog - perform back navigation
	if err := s.backend.NavigateBack(ctx, tabID); err != nil {
		return nil, fmt.Errorf("failed to navigate back: %w", err)
	}

	// Wait for page to stabilize
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 30*time.Second)
	}

	// Get snapshot
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &NavigateResult{
		NewSnapshot: newSnapshot,
		Metadata:    metadata,
	}, nil
}

// ForwardAndWait navigates forward and waits for the page to load
func (s *BrowserService) ForwardAndWait(ctx context.Context, tabID browser.TabID) (*NavigateResult, error) {
	// BEFORE check: If dialog exists, cannot navigate - show dialog instead
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &NavigateResult{
			NewSnapshot: snapshot,
			Metadata:    metadata,
		}, nil
	}

	// No dialog - perform forward navigation
	if err := s.backend.NavigateForward(ctx, tabID); err != nil {
		return nil, fmt.Errorf("failed to navigate forward: %w", err)
	}

	// Wait for page to stabilize
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 30*time.Second)
	}

	// Get snapshot
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &NavigateResult{
		NewSnapshot: newSnapshot,
		Metadata:    metadata,
	}, nil
}

// GetSnapshot is a passthrough to backend.GetSnapshot
func (s *BrowserService) GetSnapshot(ctx context.Context, tabID browser.TabID) (*ir.PageSnapshot, error) {
	return s.backend.GetSnapshot(ctx, tabID)
}

// GetMetadata collects metadata for a tab
func (s *BrowserService) GetMetadata(ctx context.Context, tabID browser.TabID) Metadata {
	metadata := Metadata{}

	// Get active request count
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		metadata.ActiveRequestCount = cdpBackend.GetActiveRequestCount(tabID)
	}

	// Get all tabs
	tabs, err := s.backend.ListTabs(ctx)
	if err == nil {
		metadata.AllTabs = make([]TabInfo, len(tabs))
		for i, tab := range tabs {
			metadata.AllTabs[i] = TabInfo{
				TabID:    string(tab.ID),
				Title:    tab.Title,
				URL:      tab.URL,
				IsActive: tab.ID == tabID,
			}
		}
	}

	// Get pending dialog
	dialog, err := s.backend.GetPendingDialog(ctx, tabID)
	if err == nil && dialog != nil {
		metadata.PendingDialog = &PendingDialogInfo{
			Type:    dialog.Type,
			Message: dialog.Message,
		}
	}

	return metadata
}

// CreateTabResult contains the result of creating a new tab
type CreateTabResult struct {
	TabID       browser.TabID
	NewSnapshot *ir.PageSnapshot
	Metadata    Metadata
}

// CreateTab creates a new tab and navigates to the URL
func (s *BrowserService) CreateTab(ctx context.Context, url string) (browser.TabID, error) {
	return s.backend.CreateTab(ctx, url)
}

// CreateTabAndWait creates a new tab, navigates to URL, and waits for stability
func (s *BrowserService) CreateTabAndWait(ctx context.Context, url string) (*CreateTabResult, error) {
	tabID, err := s.backend.CreateTab(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("failed to create tab: %w", err)
	}

	// Wait for page to stabilize
	time.Sleep(100 * time.Millisecond)
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 5*time.Second)
	}

	// Get snapshot
	snapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// FINAL check: Dialog may have arrived during page load
	if s.checkDialogAndReturn(ctx, tabID) {
		logging.Debug("Dialog detected after GetSnapshot in CreateTabAndWait", map[string]interface{}{"tab.id": tabID})
	}

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &CreateTabResult{
		TabID:       tabID,
		NewSnapshot: snapshot,
		Metadata:    metadata,
	}, nil
}

// ListTabs returns all tabs
func (s *BrowserService) ListTabs(ctx context.Context) ([]browser.TabInfo, error) {
	return s.backend.ListTabs(ctx)
}

// CloseTab closes a tab
func (s *BrowserService) CloseTab(ctx context.Context, tabID browser.TabID) error {
	return s.backend.CloseTab(ctx, tabID)
}

// DialogResponse represents a user's response to a dialog
type DialogResponse struct {
	Accept bool   // true for OK/Yes, false for Cancel/No
	Input  string // For prompt dialogs
}

// RespondToDialogResult contains the result after responding to a dialog
type RespondToDialogResult struct {
	NewSnapshot *ir.PageSnapshot
	Metadata    Metadata
}

// RespondToDialog responds to a pending dialog and waits for stability
func (s *BrowserService) RespondToDialog(ctx context.Context, tabID browser.TabID, response DialogResponse) (*RespondToDialogResult, error) {
	// Verify dialog exists
	dialog, err := s.backend.GetPendingDialog(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get pending dialog: %w", err)
	}
	if dialog == nil {
		return nil, fmt.Errorf("no pending dialog")
	}

	// Handle the dialog
	if err := s.backend.HandleDialog(ctx, tabID, response.Accept, response.Input); err != nil {
		return nil, fmt.Errorf("failed to handle dialog: %w", err)
	}

	// Wait for page to stabilize (dialog response may trigger navigation or JS execution)
	time.Sleep(100 * time.Millisecond)
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 5*time.Second)
	}

	// Get new snapshot
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// Collect metadata (dialog should be gone now)
	metadata := s.GetMetadata(ctx, tabID)

	return &RespondToDialogResult{
		NewSnapshot: newSnapshot,
		Metadata:    metadata,
	}, nil
}

// TypeResult contains the result of a type operation
type TypeResult struct {
	NewSnapshot     *ir.PageSnapshot
	DocumentChanged bool
	Metadata        Metadata
}

// TypeAndWait types text into the currently focused element and waits for stability
func (s *BrowserService) TypeAndWait(ctx context.Context, tabID browser.TabID, text string) (*TypeResult, error) {
	// BEFORE check: If dialog already exists, don't proceed with type
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &TypeResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get current URL before typing
	oldSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot before type: %w", err)
	}

	// Perform type
	if err := s.backend.Type(ctx, tabID, text); err != nil {
		return nil, fmt.Errorf("failed to type: %w", err)
	}

	// Wait for stable state (includes dialog detection)
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 5*time.Second)
	}

	// Check for dialog after WaitForStable (may have arrived in the gap)
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &TypeResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get new snapshot after stability
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// FINAL check: Dialog may have arrived between checkDialogAndReturn and GetSnapshot completion
	if s.checkDialogAndReturn(ctx, tabID) {
		logging.Debug("Dialog detected after GetSnapshot in TypeAndWait", map[string]interface{}{"tab.id": tabID})
		metadata := s.GetMetadata(ctx, tabID)
		return &TypeResult{
			NewSnapshot:     newSnapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Check if document changed
	documentChanged := oldSnapshot.URL != newSnapshot.URL

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &TypeResult{
		NewSnapshot:     newSnapshot,
		DocumentChanged: documentChanged,
		Metadata:        metadata,
	}, nil
}

// EnterResult contains the result of an enter operation
type EnterResult struct {
	NewSnapshot     *ir.PageSnapshot
	DocumentChanged bool
	Metadata        Metadata
}

// EnterAndWait presses Enter on the currently focused element and waits for stability
func (s *BrowserService) EnterAndWait(ctx context.Context, tabID browser.TabID) (*EnterResult, error) {
	// BEFORE check: If dialog already exists, don't proceed with enter
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &EnterResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get current URL before pressing enter
	oldSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot before enter: %w", err)
	}

	// Perform enter
	if err := s.backend.Enter(ctx, tabID); err != nil {
		return nil, fmt.Errorf("failed to press enter: %w", err)
	}

	// Wait for stable state (includes dialog detection)
	if cdpBackend, ok := s.backend.(*cdp.Backend); ok {
		cdpBackend.WaitForStable(ctx, tabID, 5*time.Second)
	}

	// Check for dialog after WaitForStable (may have arrived in the gap)
	if s.checkDialogAndReturn(ctx, tabID) {
		snapshot := s.getSnapshotForDialog(ctx, tabID)
		metadata := s.GetMetadata(ctx, tabID)
		return &EnterResult{
			NewSnapshot:     snapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Get new snapshot after stability
	newSnapshot, err := s.backend.GetSnapshot(ctx, tabID)
	if err != nil {
		return nil, fmt.Errorf("failed to get snapshot: %w", err)
	}

	// FINAL check: Dialog may have arrived between checkDialogAndReturn and GetSnapshot completion
	if s.checkDialogAndReturn(ctx, tabID) {
		logging.Debug("Dialog detected after GetSnapshot in EnterAndWait", map[string]interface{}{"tab.id": tabID})
		metadata := s.GetMetadata(ctx, tabID)
		return &EnterResult{
			NewSnapshot:     newSnapshot,
			DocumentChanged: false,
			Metadata:        metadata,
		}, nil
	}

	// Check if document changed
	documentChanged := oldSnapshot.URL != newSnapshot.URL

	// Collect metadata
	metadata := s.GetMetadata(ctx, tabID)

	return &EnterResult{
		NewSnapshot:     newSnapshot,
		DocumentChanged: documentChanged,
		Metadata:        metadata,
	}, nil
}
