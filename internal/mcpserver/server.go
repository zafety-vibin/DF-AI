// Package mcpserver exposes the DF bridge as MCP tools. Thin layer: all
// game logic lives in the packages it wraps (mapview, commands, topology,
// worldmodel, predicate).
package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "0.1.0"

// TextResult wraps a plain string as an MCP text tool result.
func TextResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

// New builds the MCP server and registers every tool. bridge may be nil
// during scaffolding (tools then report the missing connection).
func New(bridge *Bridge) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "df-fortress", Version: Version}, nil)
	registerStateTools(srv, bridge)
	registerPerceptTools(srv, bridge)
	registerActionTools(srv, bridge)
	registerControlTools(srv, bridge)
	registerPlaceTools(srv, bridge)
	registerZoneTools(srv, bridge)
	registerLocationTools(srv, bridge)
	return srv
}
