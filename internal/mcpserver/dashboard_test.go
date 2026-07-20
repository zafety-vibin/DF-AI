package mcpserver

import (
	"strings"
	"testing"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestRenderDashboard(t *testing.T) {
	now := time.Now()
	snap := worldmodel.Snapshot{Tick: 48210, TakenAt: now, LastUpdate: now.Add(-3 * time.Second)}
	snap.Fort.Valid = true
	snap.Fort.Year = 125
	snap.Fort.Season = 0
	snap.Fort.DaysElapsed = 32
	snap.Entities.Dwarves = make([]protocol.EntityInfo, 7)
	out := renderDashboard(snap, true, `{"paused":true,"frame":48210,"stepping":false}`)
	// events= is the world model MESSAGE counter — never label it tick.
	for _, want := range []string{"events=48210", "dwarves=7", "paused=true", "year=125", "data 3s old"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dashboard missing %q in %q", want, out)
		}
	}
	if strings.Contains(out, "tick=") {
		t.Fatalf("dashboard must not mislabel the message counter as tick: %q", out)
	}
	if strings.Count(out, "\n") > 1 {
		t.Fatalf("dashboard must be one line, got %q", out)
	}
}

// A failed sim_status fetch (empty simJSON sentinel) must report
// paused=unknown, never assert paused=false as ground truth.
func TestRenderDashboardPausedUnknownOnFailedFetch(t *testing.T) {
	snap := worldmodel.Snapshot{Tick: 100}
	snap.Fort.Valid = true
	out := renderDashboard(snap, true, "")
	if !strings.Contains(out, "paused=unknown") {
		t.Fatalf("expected paused=unknown on failed sim_status fetch, got %q", out)
	}
	if strings.Contains(out, "paused=false") {
		t.Fatalf("dashboard asserted paused=false with unknown pause state: %q", out)
	}
}

// The data-age stamp is the frozen-world tripwire: fresh data reads as an
// age, old data escalates to STALE, and a model that never received a
// perception event says so instead of implying freshness.
func TestDataAgeStamp(t *testing.T) {
	base := time.Now()
	cases := []struct {
		name       string
		lastUpdate time.Time
		want       string
	}{
		{"fresh", base.Add(-3 * time.Second), "data 3s old"},
		{"stale", base.Add(-142 * time.Second), "data STALE (142s)"},
		{"never updated", time.Time{}, "no data yet"},
		{"clock skew clamps to zero", base.Add(2 * time.Second), "data 0s old"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := dataAgeStamp(base, tc.lastUpdate); got != tc.want {
				t.Fatalf("dataAgeStamp = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestRenderDashboardWorldIdentity covers Q5's dashboard disclosure: unknown
// identity (no ENTITY_UPDATE with World data has arrived yet) renders
// plainly, and a known identity always names the save dir plus the
// persistent switches= tripwire so a save-swap is visible on every
// subsequent call, not just the one right after it happened.
func TestRenderDashboardWorldIdentity(t *testing.T) {
	var unknown worldmodel.Snapshot
	if out := renderDashboard(unknown, true, ""); !strings.Contains(out, "world=unknown") {
		t.Fatalf("expected world=unknown before any identity is reported, got %q", out)
	}

	known := worldmodel.Snapshot{World: worldmodel.WorldSnapshot{Known: true, SaveDir: "region2", Switches: 1}}
	out := renderDashboard(known, true, "")
	if !strings.Contains(out, "world=region2") {
		t.Fatalf("expected the current save dir named in the dashboard, got %q", out)
	}
	if !strings.Contains(out, "switches=1") {
		t.Fatalf("expected the persistent switch counter in the dashboard, got %q", out)
	}
}

func TestRenderDashboardStaleData(t *testing.T) {
	now := time.Now()
	snap := worldmodel.Snapshot{Tick: 7, TakenAt: now, LastUpdate: now.Add(-142 * time.Second)}
	snap.Fort.Valid = true
	out := renderDashboard(snap, true, `{"paused":true,"frame":10,"stepping":false}`)
	if !strings.Contains(out, "data STALE (142s)") {
		t.Fatalf("expected STALE stamp on old data, got %q", out)
	}
}
