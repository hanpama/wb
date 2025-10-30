package browser

import (
	"context"

	"github.com/hanpama/wb/pkg/ir"
)

// BrowserBackend abstracts browser automation operations.
// Single implementation: WebDriverBackend (WebDriver BiDi only).
// This interface is completely vendor-agnostic.
type BrowserBackend interface {
	// Tab lifecycle
	CreateTab(ctx context.Context, url string) (TabID, error)
	CloseTab(ctx context.Context, tabID TabID) error
	ListTabs(ctx context.Context) ([]TabInfo, error)

	// Navigation
	Navigate(ctx context.Context, tabID TabID, url string) error
	NavigateBack(ctx context.Context, tabID TabID) error
	NavigateForward(ctx context.Context, tabID TabID) error

	// Interaction
	Click(ctx context.Context, tabID TabID, selector string) error
	Type(ctx context.Context, tabID TabID, text string) error
	Enter(ctx context.Context, tabID TabID) error
	Input(ctx context.Context, tabID TabID, selector string, value string) error // Deprecated: use Click + Type instead

	// Page inspection
	GetSnapshot(ctx context.Context, tabID TabID) (*ir.PageSnapshot, error)
	GetSelector(ctx context.Context, tabID TabID, hash string) (string, error)

	// Dialogs
	HandleDialog(ctx context.Context, tabID TabID, accept bool, input string) error
	GetPendingDialog(ctx context.Context, tabID TabID) (*DialogInfo, error)

	// Cleanup
	Close() error
}
