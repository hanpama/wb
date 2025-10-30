package cdp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"
)

// ChromeProcess manages a Chrome browser process
type ChromeProcess struct {
	cmd  *exec.Cmd
	port int
}

// PageInfo represents a Chrome page/tab
type PageInfo struct {
	ID                   string `json:"id"`
	Type                 string `json:"type"`
	Title                string `json:"title"`
	URL                  string `json:"url"`
	WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
}

// LaunchChrome starts a Chrome process with remote debugging enabled
func LaunchChrome(ctx context.Context) (*ChromeProcess, error) {
	port := 9222
	chromePath := getChromePath()

	if chromePath == "" {
		return nil, fmt.Errorf("chrome executable not found")
	}

	// Create temporary user data dir
	userDataDir, err := os.MkdirTemp("", "wb-chrome-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp dir: %w", err)
	}

	args := []string{
		fmt.Sprintf("--remote-debugging-port=%d", port),
		"--no-sandbox",
		"--disable-dev-shm-usage",
		fmt.Sprintf("--user-data-dir=%s", userDataDir),
	}

	// Add headless flag unless WB_HEADLESS=false
	if os.Getenv("WB_HEADLESS") != "false" {
		args = append(args, "--headless=new", "--disable-gpu")
	}

	cmd := exec.CommandContext(ctx, chromePath, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		os.RemoveAll(userDataDir)
		return nil, fmt.Errorf("failed to start chrome: %w", err)
	}

	chrome := &ChromeProcess{
		cmd:  cmd,
		port: port,
	}

	// Wait for Chrome to be ready
	if err := chrome.waitForReady(ctx); err != nil {
		chrome.Close()
		os.RemoveAll(userDataDir)
		return nil, fmt.Errorf("chrome not ready: %w", err)
	}

	return chrome, nil
}

// waitForReady waits for Chrome to start accepting connections
func (c *ChromeProcess) waitForReady(ctx context.Context) error {
	timeout := time.After(10 * time.Second)
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return fmt.Errorf("timeout waiting for chrome")
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", c.port))
			if err == nil {
				resp.Body.Close()
				return nil
			}
		}
	}
}

// GetWebSocketURL returns the browser-level WebSocket URL
func (c *ChromeProcess) GetWebSocketURL() (string, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/version", c.port))
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	defer resp.Body.Close()

	var version struct {
		WebSocketDebuggerURL string `json:"webSocketDebuggerUrl"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&version); err != nil {
		return "", fmt.Errorf("failed to decode version: %w", err)
	}

	return version.WebSocketDebuggerURL, nil
}

// GetPageList returns all open pages/tabs
func (c *ChromeProcess) GetPageList() ([]PageInfo, error) {
	resp, err := http.Get(fmt.Sprintf("http://localhost:%d/json/list", c.port))
	if err != nil {
		return nil, fmt.Errorf("failed to get page list: %w", err)
	}
	defer resp.Body.Close()

	var pages []PageInfo
	if err := json.NewDecoder(resp.Body).Decode(&pages); err != nil {
		return nil, fmt.Errorf("failed to decode page list: %w", err)
	}

	return pages, nil
}

// Close terminates the Chrome process
func (c *ChromeProcess) Close() error {
	if c.cmd != nil && c.cmd.Process != nil {
		return c.cmd.Process.Kill()
	}
	return nil
}

// getChromePath returns the path to Chrome executable
func getChromePath() string {
	// Try common Chrome paths based on OS
	var paths []string

	switch runtime.GOOS {
	case "darwin":
		paths = []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	case "linux":
		paths = []string{
			"/usr/bin/google-chrome",
			"/usr/bin/chromium-browser",
			"/usr/bin/chromium",
		}
	case "windows":
		paths = []string{
			"C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe",
			"C:\\Program Files (x86)\\Google\\Chrome\\Application\\chrome.exe",
		}
	}

	// Check PATH
	if path, err := exec.LookPath("google-chrome"); err == nil {
		paths = append(paths, path)
	}
	if path, err := exec.LookPath("chromium"); err == nil {
		paths = append(paths, path)
	}

	// Return first existing path
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}
