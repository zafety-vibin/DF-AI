// Command df-mcp serves the Dwarf Fortress bridge as an MCP stdio server.
// A Claude Code session is the player; this binary is its senses and hands.
//
// IMPORTANT: stdout carries the MCP protocol. All logging goes to stderr.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mcpserver"
)

func main() {
	ctx := context.Background()
	bridge, err := mcpserver.NewBridge("config/orchestrator.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: bridge: %v\n", err)
		os.Exit(1)
	}
	if err := bridge.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: start: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Stop()
	srv := mcpserver.New(bridge)
	fmt.Fprintln(os.Stderr, "df-mcp: serving on stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: %v\n", err)
		os.Exit(1)
	}
}
