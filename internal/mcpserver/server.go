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
	// Instructions load unconditionally into every session (unlike deferred
	// tool schemas) — keep this under ~300 bytes; it exists to steer tool
	// discovery, not to teach gameplay (skills own that).
	srv := mcp.NewServer(&mcp.Implementation{Name: "df-fortress", Version: Version}, &mcp.ServerOptions{
		Instructions: "Turn-based Dwarf Fortress control. Core loop: status/pause -> observe (look, alerts) -> act -> step. " +
			"Discovery tools carry per-value facts: building_types (build), job_types (queue_job/order), list_reactions, list_crops. " +
			"Every response's first line is the live ground-truth dashboard — never act on remembered state.",
	})
	registerStateTools(srv, bridge)
	registerPerceptTools(srv, bridge)
	registerActionTools(srv, bridge)
	registerControlTools(srv, bridge)
	registerPlaceTools(srv, bridge)
	registerZoneTools(srv, bridge)
	registerLocationTools(srv, bridge)
	registerFarmTools(srv, bridge)
	registerDefenseTools(srv, bridge)
	registerMilitaryTools(srv, bridge)
	return srv
}
