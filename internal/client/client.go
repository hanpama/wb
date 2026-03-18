package client

import (
	"fmt"
	"log"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"os/exec"
	"time"

	"github.com/hanpama/wb/internal/renderer"
	"github.com/hanpama/wb/pkg/protocol"
	"github.com/spf13/cobra"
)

const (
	ServerAddr        = "localhost:62066"
	DefaultLinesLimit = 100 // Default number of lines to show per page
)

// ensureServerRunning checks if server is running and starts it if needed
func ensureServerRunning() error {
	// Try to ping the server
	conn, err := net.DialTimeout("tcp", ServerAddr, 500*time.Millisecond)
	if err == nil {
		// Server is running
		conn.Close()
		return nil
	}

	// Server is not running, start it silently
	// Get the path to current executable
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Start server in background with WB_INTERNAL_DAEMON=1
	// Preserve WB_HEADLESS environment variable if set
	cmd := exec.Command(exePath)
	cmd.Env = append(os.Environ(), "WB_INTERNAL_DAEMON=1")

	// Create log file for server output
	logFile, err := os.Create("/tmp/wb-server.log")
	if err != nil {
		return fmt.Errorf("failed to create log file: %w", err)
	}
	cmd.Stdout = logFile
	cmd.Stderr = logFile

	err = cmd.Start()
	if err != nil {
		logFile.Close()
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Wait for server to be ready
	for i := 0; i < 10; i++ {
		time.Sleep(300 * time.Millisecond)
		conn, err := net.DialTimeout("tcp", ServerAddr, 500*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
	}

	return fmt.Errorf("server failed to start after 3 seconds")
}

// connectToServer creates a connection to the server
func connectToServer() (*rpc.Client, net.Conn, error) {
	// Ensure server is running
	if err := ensureServerRunning(); err != nil {
		return nil, nil, err
	}

	// Connect to server
	conn, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to server: %w", err)
	}

	client := jsonrpc.NewClient(conn)
	return client, conn, nil
}

var rootCmd = &cobra.Command{
	Use:   "wb",
	Short: "The TUI Web Browser",
	Long:  "wb :: The TUI Web Browser",
	Run: func(cmd *cobra.Command, args []string) {
		// When called without arguments, show status and help
		showStatusAndHelp(cmd)
	},
}

func showStatusAndHelp(cmd *cobra.Command) {
	// Try to get server status
	client, conn, err := connectToServer()
	if err != nil {
		fmt.Println("[Status: Server not available]")
		fmt.Println(" Use 'wb open <url>' to start browsing.")
	} else {
		defer conn.Close()
		defer client.Close()

		// Call GetStatus to get tab information
		var statusArgs protocol.GetStatusArgs
		var statusReply protocol.GetStatusReply
		err = client.Call("BrowserService.GetStatus", &statusArgs, &statusReply)

		if err != nil || statusReply.TabCount == 0 {
			fmt.Println("[Status: No tabs open]")
			fmt.Println(" Start a new session with the command below.")
			fmt.Println("> wb open https://example.com")
		} else {
			fmt.Printf("[Status: %d tabs open]\n", statusReply.TabCount)
			fmt.Printf("🌐 The active tab is [%s: \"%s\"].\n", statusReply.ActiveTabID, statusReply.ActiveTabTitle)
			fmt.Println("Use 'wb show' to view its content or 'wb list' to see all tabs.")
		}
	}

	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────────")

	// Show quick start guide
	fmt.Println()
	fmt.Println("Quick Start:")
	fmt.Println("  1. Open a website:")
	fmt.Println("     wb open https://example.com")
	fmt.Println()
	fmt.Println("  2. Interactive elements have {hash} identifiers:")
	fmt.Println("     [Click here]{a1b2}  [Input/text: (empty)]{c3d4}")
	fmt.Println()
	fmt.Println("  3. Inspect an element to see what it does:")
	fmt.Println("     wb describe a1b2")
	fmt.Println()
	fmt.Println("  4. Click links and buttons:")
	fmt.Println("     wb click a1b2")
	fmt.Println()
	fmt.Println("  5. Fill in forms:")
	fmt.Println("     wb input c3d4 \"your text here\"")
	fmt.Println()
	fmt.Println("────────────────────────────────────────────────────────────────")

	// Use Cobra's built-in help
	cmd.Help()
}

var newCmd = &cobra.Command{
	Use:   "new [url]",
	Short: "Open a new tab with the specified URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call NewTab method
		newTabArgs := protocol.NewTabArgs{URL: url}
		var newTabReply protocol.NewTabReply

		err = client.Call("BrowserService.NewTab", &newTabArgs, &newTabReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render full output
		showReply := &protocol.ShowReply{
			TabID:              newTabReply.TabID,
			Title:              newTabReply.Title,
			URL:                newTabReply.URL,
			Markdown:           newTabReply.Markdown,
			PendingDialogs:     newTabReply.PendingDialogs,
			TotalLines:         newTabReply.TotalLines,
			Offset:             newTabReply.Offset,
			Limit:              newTabReply.Limit,
			FocusedHash:        newTabReply.FocusedHash,
			ActiveRequestCount: newTabReply.ActiveRequestCount,
			AllTabs:            newTabReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

var showCmd = &cobra.Command{
	Use:   "show",
	Short: "Show the current tab as markdown",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Get flags
		offset, _ := cmd.Flags().GetInt("offset")
		limit, _ := cmd.Flags().GetInt("limit")

		if limit == 0 {
			limit = DefaultLinesLimit
		}

		// Call Show method
		showArgs := protocol.ShowArgs{
			Offset: offset,
			Limit:  limit,
		}
		var showReply protocol.ShowReply

		err = client.Call("BrowserService.Show", &showArgs, &showReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render output with status bar and footer
		renderShowOutput(&showReply)
	},
}

func renderShowOutput(reply *protocol.ShowReply) {
	output := renderer.RenderShowOutput(reply)
	fmt.Println(output)
}

var clickCmd = &cobra.Command{
	Use:   "click [hash]",
	Short: "Click an interactive element by its hash",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Click method
		clickArgs := protocol.ClickArgs{Hash: hash}
		var clickReply protocol.ClickReply

		err = client.Call("BrowserService.Click", &clickArgs, &clickReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render output using new renderer
		output := renderer.RenderClickOutput(&clickReply)
		fmt.Println(output)
	},
}

var typeCmd = &cobra.Command{
	Use:   "type [text]",
	Short: "Type text into the currently focused element",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		text := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Type method
		typeArgs := protocol.TypeArgs{Text: text}
		var typeReply protocol.TypeReply

		err = client.Call("BrowserService.Type", &typeArgs, &typeReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render output using renderer
		output := renderer.RenderTypeOutput(&typeReply)
		fmt.Println(output)
	},
}

var enterCmd = &cobra.Command{
	Use:   "enter",
	Short: "Press Enter on the currently focused element",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Enter method
		enterArgs := protocol.EnterArgs{}
		var enterReply protocol.EnterReply

		err = client.Call("BrowserService.Enter", &enterArgs, &enterReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render output using renderer
		output := renderer.RenderEnterOutput(&enterReply)
		fmt.Println(output)
	},
}

var openCmd = &cobra.Command{
	Use:   "open [url]",
	Short: "Navigate the current tab to a different URL",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		url := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Open method
		openArgs := protocol.OpenArgs{URL: url}
		var openReply protocol.OpenReply

		err = client.Call("BrowserService.Open", &openArgs, &openReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render full output
		showReply := &protocol.ShowReply{
			TabID:              openReply.TabID,
			Title:              openReply.Title,
			URL:                openReply.URL,
			Markdown:           openReply.Markdown,
			PendingDialogs:     openReply.PendingDialogs,
			TotalLines:         openReply.TotalLines,
			Offset:             openReply.Offset,
			Limit:              openReply.Limit,
			FocusedHash:        openReply.FocusedHash,
			ActiveRequestCount: openReply.ActiveRequestCount,
			AllTabs:            openReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

var backCmd = &cobra.Command{
	Use:   "back",
	Short: "Navigate back in the current tab's history",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Back method
		var backArgs protocol.BackArgs
		var backReply protocol.BackReply

		err = client.Call("BrowserService.Back", &backArgs, &backReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render full output
		showReply := &protocol.ShowReply{
			TabID:              backReply.TabID,
			Title:              backReply.Title,
			URL:                backReply.URL,
			Markdown:           backReply.Markdown,
			PendingDialogs:     backReply.PendingDialogs,
			TotalLines:         backReply.TotalLines,
			Offset:             backReply.Offset,
			Limit:              backReply.Limit,
			FocusedHash:        backReply.FocusedHash,
			ActiveRequestCount: backReply.ActiveRequestCount,
			AllTabs:            backReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

var forwardCmd = &cobra.Command{
	Use:   "forward",
	Short: "Navigate forward in the current tab's history",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Forward method
		var forwardArgs protocol.ForwardArgs
		var forwardReply protocol.ForwardReply

		err = client.Call("BrowserService.Forward", &forwardArgs, &forwardReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render full output
		showReply := &protocol.ShowReply{
			TabID:              forwardReply.TabID,
			Title:              forwardReply.Title,
			URL:                forwardReply.URL,
			Markdown:           forwardReply.Markdown,
			PendingDialogs:     forwardReply.PendingDialogs,
			TotalLines:         forwardReply.TotalLines,
			Offset:             forwardReply.Offset,
			Limit:              forwardReply.Limit,
			FocusedHash:        forwardReply.FocusedHash,
			ActiveRequestCount: forwardReply.ActiveRequestCount,
			AllTabs:            forwardReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all open tabs",
	Run: func(cmd *cobra.Command, args []string) {
		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call List method
		var listArgs protocol.ListArgs
		var listReply protocol.ListReply

		err = client.Call("BrowserService.List", &listArgs, &listReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Display tabs
		for _, tab := range listReply.Tabs {
			prefix := " "
			if tab.IsActive {
				prefix = "*"
			}
			fmt.Printf("%s %s: %s (%s)\n", prefix, tab.TabID, tab.Title, tab.URL)
		}
	},
}

var switchCmd = &cobra.Command{
	Use:   "switch [tab-id]",
	Short: "Switch focus to a specific tab",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tabID := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Switch method
		switchArgs := protocol.SwitchArgs{TabID: tabID}
		var switchReply protocol.SwitchReply

		err = client.Call("BrowserService.Switch", &switchArgs, &switchReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Convert to ShowReply and render
		showReply := &protocol.ShowReply{
			TabID:              switchReply.TabID,
			Title:              switchReply.Title,
			URL:                switchReply.URL,
			Markdown:           switchReply.Markdown,
			PendingDialogs:     switchReply.PendingDialogs,
			TotalLines:         switchReply.TotalLines,
			Offset:             switchReply.Offset,
			Limit:              switchReply.Limit,
			FocusedHash:        switchReply.FocusedHash,
			ActiveRequestCount: switchReply.ActiveRequestCount,
			AllTabs:            switchReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

var closeCmd = &cobra.Command{
	Use:   "close [tab-id]",
	Short: "Close the current or a specific tab",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		tabID := ""
		if len(args) > 0 {
			tabID = args[0]
		}

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Close method
		closeArgs := protocol.CloseArgs{TabID: tabID}
		var closeReply protocol.CloseReply

		err = client.Call("BrowserService.Close", &closeArgs, &closeReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		if tabID == "" {
			fmt.Println("Closed current tab")
		} else {
			fmt.Printf("Closed tab %s\n", tabID)
		}
		fmt.Printf("%d tabs remaining\n", closeReply.RemainingTabCount)
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe [hash]",
	Short: "Describe an interactive element by its hash",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		hash := args[0]

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call Describe method
		describeArgs := protocol.DescribeArgs{Hash: hash}
		var describeReply protocol.DescribeReply

		err = client.Call("BrowserService.Describe", &describeArgs, &describeReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		if !describeReply.Found {
			fmt.Printf("Element with hash {%s} not found\n", hash)
			fmt.Println("Run 'wb show' to refresh the page content")
			return
		}

		fmt.Printf("Element {%s}\n", describeReply.Hash)
		fmt.Println("────────────────────────────────────────────────────────────────")
		if describeReply.Role != "" {
			fmt.Printf("  Role:     %s\n", describeReply.Role)
		}
		if describeReply.Name != "" {
			fmt.Printf("  Name:     %s\n", describeReply.Name)
		}
		if describeReply.Value != "" {
			fmt.Printf("  Value:    %s\n", describeReply.Value)
		}
		if describeReply.URL != "" {
			fmt.Printf("  URL:      %s\n", describeReply.URL)
		}
		if describeReply.Checked != "" {
			fmt.Printf("  Checked:  %s\n", describeReply.Checked)
		}

		if describeReply.DOMContext != "" {
			fmt.Println("────────────────────────────────────────────────────────────────")
			fmt.Println(describeReply.DOMContext)
		}
		fmt.Println("────────────────────────────────────────────────────────────────")
	},
}

var respondCmd = &cobra.Command{
	Use:   "respond [ok|cancel] [input]",
	Short: "Respond to a pending JavaScript dialog",
	Long: `Respond to a pending JavaScript dialog (alert, confirm, or prompt).

Examples:
  wb respond ok               # Click OK on alert or confirm
  wb respond cancel           # Click Cancel on confirm or prompt
  wb respond ok "text"        # Enter text and click OK on prompt`,
	Args: cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		action := args[0]
		input := ""
		if len(args) > 1 {
			input = args[1]
		}

		accept := false
		if action == "accept" || action == "ok" || action == "yes" {
			accept = true
		} else if action == "reject" || action == "cancel" || action == "no" {
			accept = false
		} else {
			log.Fatalf("Invalid action: %s. Use 'ok' or 'cancel'", action)
		}

		client, conn, err := connectToServer()
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		defer client.Close()

		// Call RespondToDialog method
		respondArgs := protocol.RespondToDialogArgs{
			Accept: accept,
			Input:  input,
		}
		var respondReply protocol.RespondToDialogReply

		err = client.Call("BrowserService.RespondToDialog", &respondArgs, &respondReply)
		if err != nil {
			log.Fatal("RPC call failed:", err)
		}

		// Render output as show reply
		showReply := &protocol.ShowReply{
			TabID:              respondReply.TabID,
			Title:              respondReply.Title,
			URL:                respondReply.URL,
			Markdown:           respondReply.Markdown,
			PendingDialogs:     respondReply.PendingDialogs,
			TotalLines:         respondReply.TotalLines,
			Offset:             respondReply.Offset,
			Limit:              respondReply.Limit,
			FocusedHash:        respondReply.FocusedHash,
			ActiveRequestCount: respondReply.ActiveRequestCount,
			AllTabs:            respondReply.AllTabs,
		}
		renderShowOutput(showReply)
	},
}

func init() {
	// Add flags to show command
	showCmd.Flags().Int("offset", 0, "Line offset to start from (0-based)")
	showCmd.Flags().Int("limit", 0, "Number of lines to show (default: 100)")

	rootCmd.AddCommand(newCmd)
	rootCmd.AddCommand(openCmd)
	rootCmd.AddCommand(backCmd)
	rootCmd.AddCommand(forwardCmd)
	rootCmd.AddCommand(showCmd)
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(switchCmd)
	rootCmd.AddCommand(closeCmd)
	rootCmd.AddCommand(clickCmd)
	rootCmd.AddCommand(typeCmd)
	rootCmd.AddCommand(enterCmd)
	rootCmd.AddCommand(describeCmd)
	rootCmd.AddCommand(respondCmd)
}

// Run starts the client
func Run() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
