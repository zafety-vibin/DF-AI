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
	srv := mcpserver.New(nil) // bridge wired in the next task
	fmt.Fprintln(os.Stderr, "df-mcp: serving on stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: %v\n", err)
		os.Exit(1)
	}
}
