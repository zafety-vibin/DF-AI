package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestRenderDashboard(t *testing.T) {
	snap := worldmodel.Snapshot{Tick: 48210}
	snap.Fort.Valid = true
	snap.Fort.Year = 125
	snap.Fort.Season = 0
	snap.Fort.DaysElapsed = 32
	snap.Entities.Dwarves = make([]protocol.EntityInfo, 7)
	out := renderDashboard(snap, true, `{"paused":true,"frame":48210,"stepping":false}`)
	for _, want := range []string{"tick=48210", "dwarves=7", "paused=true", "year=125"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dashboard missing %q in %q", want, out)
		}
	}
	if strings.Count(out, "\n") > 1 {
		t.Fatalf("dashboard must be one line, got %q", out)
	}
}
