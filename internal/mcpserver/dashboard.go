package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// dataStaleAfter is the snapshot age past which the dashboard escalates the
// data stamp to STALE. The plugin pushes state at every pause/step boundary
// (and `ai-auto-update on` refreshes periodically), so a healthy session
// stays well under this.
const dataStaleAfter = 60 * time.Second

// dataAgeStamp renders the honest freshness of the world model: how long ago
// the last perception event was ingested, relative to when the snapshot was
// taken. The model must never silently trust frozen state — a step that
// completed without a state push, or a wedged populator, shows up here.
func dataAgeStamp(takenAt, lastUpdate time.Time) string {
	if lastUpdate.IsZero() {
		return "no data yet"
	}
	age := takenAt.Sub(lastUpdate)
	if age < 0 {
		age = 0
	}
	secs := int(age.Seconds())
	if age > dataStaleAfter {
		return fmt.Sprintf("data STALE (%ds)", secs)
	}
	return fmt.Sprintf("data %ds old", secs)
}

// renderDashboard is the one-line ground-truth header. Injected into every
// tool response so the model never acts on remembered state (prior-art
// lesson: models trust stale beliefs for hours).
//
// events= is the world model's MESSAGE counter (perception events ingested),
// not simulation time — sim frames live in sim_status and step reports.
func renderDashboard(snap worldmodel.Snapshot, connected bool, simJSON string) string {
	if !connected {
		return "[GROUND TRUTH: no game connection — run ai-connect in the DFHack console]"
	}
	paused := "unknown"
	var sim struct {
		Paused   bool `json:"paused"`
		Stepping bool `json:"stepping"`
	}
	// Empty simJSON means the sim_status fetch failed: keep paused=unknown
	// instead of letting a zero-value unmarshal assert paused=false.
	if simJSON != "" && json.Unmarshal([]byte(simJSON), &sim) == nil {
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
	nAlerts := snap.ActiveAlertCount
	return fmt.Sprintf("[GROUND TRUTH events=%d year=%d season=%s day=%d | dwarves=%d enemies=%d | paused=%s | alerts=%d active | %s]",
		snap.Tick, snap.Fort.Year, season, snap.Fort.DaysElapsed,
		len(snap.Entities.Dwarves), len(snap.Entities.Enemies), paused, nAlerts,
		dataAgeStamp(snap.TakenAt, snap.LastUpdate))
}

// withDash prepends the dashboard to a tool body. The sim_status fetch +
// render lives in exactly one place: Bridge.StatusLine (nil-safe).
func withDash(b *Bridge, ctx context.Context, body string) *mcp.CallToolResult {
	return TextResult(b.StatusLine(ctx) + "\n\n" + body)
}
