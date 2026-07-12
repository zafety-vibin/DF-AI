package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerStateTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "status",
		Description: "Connection and simulation status: is the DFHack plugin connected, is DF paused, current tick. Call this first if anything seems wrong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || !b.Connected() {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console. The MCP server listens on the port in config/orchestrator.yaml."), nil, nil
		}
		return TextResult(b.StatusLine(ctx)), nil, nil
	})
}
