package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// maxDwarfList caps the dwarves listing; beyond it the tool summarizes.
const maxDwarfList = 50

// rawJSONCap bounds raw plugin-query payloads (stocks, orders) so one huge
// response cannot blow the MCP output budget (~10k-token cap per response).
const rawJSONCap = 8 * 1024

// capRawJSON truncates an oversized raw payload at a rune boundary and says
// so, instead of silently flooding the response.
func capRawJSON(raw string) string {
	if len(raw) <= rawJSONCap {
		return raw
	}
	cut := rawJSONCap
	for cut > 0 && !utf8.RuneStart(raw[cut]) {
		cut--
	}
	return raw[:cut] + "\n...truncated, refine your query"
}

// renderDwarfList renders the id/position roster, capped at maxDwarfList.
func renderDwarfList(dwarves []protocol.EntityInfo) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d dwarves:\n", len(dwarves))
	for i, d := range dwarves {
		if i >= maxDwarfList {
			fmt.Fprintf(&sb, "... and %d more (use dwarf_detail by id)\n", len(dwarves)-maxDwarfList)
			break
		}
		fmt.Fprintf(&sb, "- id=%d @(%d,%d,%d)\n", d.ID, d.X, d.Y, d.Z)
	}
	return sb.String()
}

func registerStateTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "status",
		Description: "Connection and simulation status: is the DFHack plugin connected, is DF paused, current tick. Call this first if anything seems wrong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || !b.Connected() {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console. The MCP server listens on the port in config/orchestrator.yaml."), nil, nil
		}
		return withDash(b, ctx, "Connected. The ground-truth header above is live game state."), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "alerts",
		Description: "DF's own announcement stream (job cancellations with reasons, sieges, moods, migrants). READ THIS when work isn't progressing — DF usually says exactly why.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		if len(snap.ActiveAlerts) == 0 {
			return withDash(b, ctx, "No active alerts."), nil, nil
		}
		var sb strings.Builder
		for i, a := range snap.ActiveAlerts {
			if i >= 30 {
				fmt.Fprintf(&sb, "... and %d more\n", len(snap.ActiveAlerts)-30)
				break
			}
			fmt.Fprintf(&sb, "- [%d] sev=%d %s", a.ID, a.Severity, a.Text)
			if a.HasPosition() {
				fmt.Fprintf(&sb, " @(%d,%d,%d)", a.X, a.Y, a.Z)
			}
			sb.WriteString("\n")
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarves",
		Description: "List all dwarves with id and position. Use dwarf_detail for skills/mood/job of one dwarf.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		return withDash(b, ctx, renderDwarfList(b.Snapshot().Entities.Dwarves)), nil, nil
	})

	type dwarfDetailIn struct {
		ID int `json:"id" jsonschema:"the dwarf's id from the dwarves tool"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarf_detail",
		Description: "One dwarf's full record: skills, labors, mood, current job.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dwarfDetailIn) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "dwarf_detail", fmt.Sprintf(`{"id":%d}`, in.ID))
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stocks",
		Description: "Stockpile inventory by item type and material — what the fort actually has.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "stockpile_inventory", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, capRawJSON(string(raw))), nil, nil
	})

	// queries.cpp handleWorkshopJobs rejects x<0 || y<0, so the workshop
	// tile is required (no plugin-side default).
	type jobsIn struct {
		X int `json:"x" jsonschema:"workshop tile x"`
		Y int `json:"y" jsonschema:"workshop tile y"`
		Z int `json:"z" jsonschema:"workshop tile z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "jobs",
		Description: "Jobs queued at one workshop, targeted by its tile (find workshops via building queries).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobsIn) (*mcp.CallToolResult, any, error) {
		args := fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, in.X, in.Y, in.Z)
		raw, err := b.Query(ctx, "workshop_jobs", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "orders",
		Description: "Manager work orders and general job list — what's queued and its status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		var sb strings.Builder
		for _, q := range []string{"manager_orders", "list_orders"} {
			raw, err := b.Query(ctx, q, "{}")
			if err != nil {
				fmt.Fprintf(&sb, "%s failed: %v\n", q, err)
				continue
			}
			fmt.Fprintf(&sb, "%s: %s\n", q, capRawJSON(string(raw)))
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
}
