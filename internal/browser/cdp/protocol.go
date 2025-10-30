package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// CDPClient manages WebSocket connection to Chrome DevTools Protocol
type CDPClient struct {
	conn      *websocket.Conn
	mu        sync.Mutex
	commandID atomic.Int64

	// Response handling
	responses map[int64]chan *CDPResponse
	respMu    sync.RWMutex

	// Event handling
	events chan *CDPEvent
	done   chan struct{}
}

type CDPCommand struct {
	ID     int64          `json:"id"`
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

type CDPResponse struct {
	ID     int64          `json:"id"`
	Result map[string]any `json:"result,omitempty"`
	Error  *CDPError      `json:"error,omitempty"`
}

type CDPError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type CDPEvent struct {
	Method string         `json:"method"`
	Params map[string]any `json:"params,omitempty"`
}

// NewCDPClient creates a new CDP client and connects to the WebSocket
func NewCDPClient(ctx context.Context, wsURL string) (*CDPClient, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to dial websocket: %w", err)
	}

	client := &CDPClient{
		conn:      conn,
		responses: make(map[int64]chan *CDPResponse),
		events:    make(chan *CDPEvent, 100),
		done:      make(chan struct{}),
	}

	// Start message receiver goroutine
	go client.receiveLoop()

	return client, nil
}

// SendCommand sends a CDP command and waits for response
func (c *CDPClient) SendCommand(ctx context.Context, method string, params map[string]any) (map[string]any, error) {
	id := c.commandID.Add(1)

	cmd := CDPCommand{
		ID:     id,
		Method: method,
		Params: params,
	}

	// Create response channel
	respChan := make(chan *CDPResponse, 1)
	c.respMu.Lock()
	c.responses[id] = respChan
	c.respMu.Unlock()

	// Ensure cleanup
	defer func() {
		c.respMu.Lock()
		delete(c.responses, id)
		c.respMu.Unlock()
		close(respChan)
	}()

	// Send command
	c.mu.Lock()
	err := c.conn.WriteJSON(cmd)
	c.mu.Unlock()

	if err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for response or timeout
	select {
	case resp := <-respChan:
		if resp.Error != nil {
			return nil, fmt.Errorf("CDP error %d: %s", resp.Error.Code, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-c.done:
		return nil, fmt.Errorf("connection closed")
	}
}

// WaitForEvent waits for a specific CDP event
func (c *CDPClient) WaitForEvent(ctx context.Context, eventMethod string) error {
	timeout := time.After(30 * time.Second)

	for {
		select {
		case event := <-c.events:
			if event.Method == eventMethod {
				return nil
			}
		case <-timeout:
			return fmt.Errorf("timeout waiting for event %s", eventMethod)
		case <-ctx.Done():
			return ctx.Err()
		case <-c.done:
			return fmt.Errorf("connection closed")
		}
	}
}

// receiveLoop continuously receives messages from WebSocket
func (c *CDPClient) receiveLoop() {
	defer close(c.done)

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			return
		}

		// Try to parse as response first
		var resp CDPResponse
		if err := json.Unmarshal(message, &resp); err == nil && resp.ID > 0 {
			c.respMu.RLock()
			if ch, ok := c.responses[resp.ID]; ok {
				select {
				case ch <- &resp:
				default:
				}
			}
			c.respMu.RUnlock()
			continue
		}

		// Parse as event
		var event CDPEvent
		if err := json.Unmarshal(message, &event); err == nil && event.Method != "" {
			select {
			case c.events <- &event:
			default:
				// Drop event if buffer full
			}
		}
	}
}

// Close closes the WebSocket connection
func (c *CDPClient) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}
