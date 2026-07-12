package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// renderDashboard is the one-line ground-truth header. Injected into every
// tool response so the model never acts on remembered state (prior-art
// lesson: models trust stale beliefs for hours).
func renderDashboard(snap worldmodel.Snapshot, connected bool, simJSON string) string {
	if !connected {
		return "[GROUND TRUTH: no game connection — run ai-connect in the DFHack console]"
	}
	paused := "unknown"
	var sim struct {
		Paused   bool `json:"paused"`
		Stepping bool `json:"stepping"`
	}
	if json.Unmarshal([]byte(simJSON), &sim) == nil {
		if sim.Stepping {
			paused = "stepping"
		} else if sim.Paused {
			paused = "true"
		} else {
			paused = "false"
		}
	}
	seasons := [4]string{"spring", "summer", "autumn", "winter"}
	season := "?"
	if snap.Fort.Valid && snap.Fort.Season < 4 {
		season = seasons[snap.Fort.Season]
	}
	nAlerts := len(snap.ActiveAlerts)
	return fmt.Sprintf("[GROUND TRUTH tick=%d year=%d season=%s day=%d | dwarves=%d enemies=%d | paused=%s | alerts=%d active]",
		snap.Tick, snap.Fort.Year, season, snap.Fort.DaysElapsed,
		len(snap.Entities.Dwarves), len(snap.Entities.Enemies), paused, nAlerts)
}

// withDash prepends the dashboard to a tool body.
func withDash(b *Bridge, ctx context.Context, body string) *mcp.CallToolResult {
	dash := "[GROUND TRUTH unavailable]"
	if b != nil {
		sim := "{}"
		if raw, err := b.Query(ctx, "sim_status", "{}"); err == nil {
			sim = string(raw)
		}
		dash = renderDashboard(b.Snapshot(), b.Connected(), sim)
	}
	return TextResult(dash + "\n\n" + body)
}
