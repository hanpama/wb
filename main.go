package main

import (
	"os"

	"github.com/hanpama/wb/internal/client"
	"github.com/hanpama/wb/internal/server"
)

func main() {
	// Check if running in daemon mode
	if os.Getenv("WB_INTERNAL_DAEMON") == "1" {
		// Run as server
		server.Run()
	} else {
		// Run as client
		client.Run()
	}
}
