package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestRenderGoals(t *testing.T) {
	results := []predicate.Result{
		{Name: "has_shelter", Horizon: predicate.HorizonNow, Satisfied: false, Confidence: 0.9,
			Evidence: []string{"0 dug tiles"}},
		{Name: "has_min_dwarves", Horizon: predicate.HorizonNow, Satisfied: true, Confidence: 1.0},
	}
	out := renderGoals(results)
	if !strings.Contains(out, "[ ] has_shelter") || !strings.Contains(out, "[x] has_min_dwarves") {
		t.Fatalf("goal markers wrong:\n%s", out)
	}
	if !strings.Contains(out, "0 dug tiles") {
		t.Fatalf("evidence missing:\n%s", out)
	}
}

func TestRenderGoalsHorizonSections(t *testing.T) {
	results := []predicate.Result{
		{Name: "now_goal", Horizon: predicate.HorizonNow, Satisfied: true, Confidence: 1.0},
		{Name: "eventual_goal", Horizon: predicate.HorizonEventual, Satisfied: false, Confidence: 0.5},
	}
	out := renderGoals(results)
	if !strings.Contains(out, "NOW:") || !strings.Contains(out, "EVENTUAL:") {
		t.Fatalf("horizon headers missing:\n%s", out)
	}
	if strings.Contains(out, "SOON:") {
		t.Fatalf("empty horizon should be omitted:\n%s", out)
	}
}

// TestControlToolsNilBridge drives every control tool over an in-memory
// MCP session against a nil bridge: no panics, each reports NOT CONNECTED.
func TestControlToolsNilBridge(t *testing.T) {
	ctx := context.Background()
	srv := New(nil)

	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	serverSession, err := srv.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatalf("server connect: %v", err)
	}
	defer serverSession.Close()

	client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0.0.1"}, nil)
	clientSession, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer clientSession.Close()

	for _, name := range []string{"pause", "unpause", "check_goals"} {
		res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{Name: name})
		if err != nil {
			t.Fatalf("call %s: %v", name, err)
		}
		tc, ok := res.Content[0].(*mcp.TextContent)
		if !ok {
			t.Fatalf("%s: expected *mcp.TextContent, got %T", name, res.Content[0])
		}
		if !strings.Contains(tc.Text, "NOT CONNECTED") {
			t.Errorf("%s: expected NOT CONNECTED, got %q", name, tc.Text)
		}
	}

	res, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "step", Arguments: map[string]any{"ticks": 600},
	})
	if err != nil {
		t.Fatalf("call step: %v", err)
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("step: expected *mcp.TextContent, got %T", res.Content[0])
	}
	if !strings.Contains(tc.Text, "NOT CONNECTED") {
		t.Errorf("step: expected NOT CONNECTED, got %q", tc.Text)
	}
}

func TestSimPaused(t *testing.T) {
	cases := []struct {
		name         string
		raw          string
		wantPaused   bool
		wantStepping bool
	}{
		{"paused done stepping", `{"paused":true,"frame":1234,"stepping":false}`, true, false},
		{"mid-step", `{"paused":false,"frame":1000,"stepping":true}`, false, true},
		{"paused but still stepping", `{"paused":true,"frame":1100,"stepping":true}`, true, true},
		{"garbage", `not json`, false, false},
		{"empty", ``, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			paused, stepping := simPaused([]byte(tc.raw))
			if paused != tc.wantPaused || stepping != tc.wantStepping {
				t.Fatalf("simPaused(%q) = (%v, %v), want (%v, %v)",
					tc.raw, paused, stepping, tc.wantPaused, tc.wantStepping)
			}
		})
	}
}

func TestParseSimStatusFrame(t *testing.T) {
	s, ok := parseSimStatus([]byte(`{"paused":true,"frame":48210,"stepping":false}`))
	if !ok || s.Frame != 48210 || !s.Paused || s.Stepping {
		t.Fatalf("parseSimStatus = (%+v, %v), want frame 48210 paused", s, ok)
	}
	if _, ok := parseSimStatus([]byte(`not json`)); ok {
		t.Fatal("garbage must not parse")
	}
}

func TestFrameStr(t *testing.T) {
	if got := frameStr(-1); got != "?" {
		t.Fatalf("frameStr(-1) = %q, want ?", got)
	}
	if got := frameStr(1234); got != "1234" {
		t.Fatalf("frameStr(1234) = %q, want 1234", got)
	}
}

// TestStepReport checks the post-step summary speaks in SIM FRAMES (game
// time), reports population deltas and new alerts, and warns when no state
// push arrived (deltas would be computed against frozen data).
func TestStepReport(t *testing.T) {
	var before, after worldmodel.Snapshot
	before.Entities.Dwarves = make([]protocol.EntityInfo, 7)
	after.Entities.Dwarves = make([]protocol.EntityInfo, 9)
	after.ActiveAlerts = []worldmodel.Alert{
		{ID: 3, Text: "migrants have arrived"},
		{ID: 1, Text: "old alert"},
	}
	out := stepReport(600, 1000, 1600, true, before, after, map[uint32]bool{1: true}, false, "")
	for _, want := range []string{
		"stepped 600 ticks (sim frame 1000 -> 1600)",
		"dwarf count change: +2",
		"NEW ALERT [3] migrants have arrived",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("step report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "old alert") || strings.Contains(out, "WARNING") ||
		strings.Contains(out, "tripwire") {
		t.Fatalf("unexpected content in report:\n%s", out)
	}

	// Unknown before-frame renders "?", and a missing state push warns.
	out = stepReport(600, -1, 1600, false, before, before, map[uint32]bool{}, false, "")
	if !strings.Contains(out, "(sim frame ? -> 1600)") {
		t.Fatalf("expected ? for unknown before frame:\n%s", out)
	}
	if !strings.Contains(out, "WARNING: no state push received") {
		t.Fatalf("expected stale-delta warning without a push:\n%s", out)
	}
	if !strings.Contains(out, "no new alerts") {
		t.Fatalf("expected no-new-alerts line:\n%s", out)
	}
}

// TestStepReportTripwire: a plugin-initiated early stop is surfaced with
// its reason (and a fallback when the reason is empty).
func TestStepReportTripwire(t *testing.T) {
	var snap worldmodel.Snapshot
	out := stepReport(1200, 1000, 1300, true, snap, snap, map[uint32]bool{}, true, "water rising in the fort")
	if !strings.Contains(out, "step ended EARLY at frame 1300 — tripwire: water rising in the fort") {
		t.Fatalf("tripwire line missing:\n%s", out)
	}
	out = stepReport(1200, 1000, 1300, true, snap, snap, map[uint32]bool{}, true, "")
	if !strings.Contains(out, "tripwire: unspecified") {
		t.Fatalf("empty tripwire reason must fall back to 'unspecified':\n%s", out)
	}
}

// TestParseSimStatusTripwire: tripwire fields decode when present and stay
// zero-valued when absent (older plugin).
func TestParseSimStatusTripwire(t *testing.T) {
	s, ok := parseSimStatus([]byte(`{"paused":true,"frame":10,"stepping":false,"tripwire":true,"tripwire_reason":"flooding"}`))
	if !ok || !s.Tripwire || s.TripwireReason != "flooding" {
		t.Fatalf("tripwire fields not decoded: %+v", s)
	}
	s, ok = parseSimStatus([]byte(`{"paused":true,"frame":10,"stepping":false}`))
	if !ok || s.Tripwire || s.TripwireReason != "" {
		t.Fatalf("absent tripwire fields must decode to zero values: %+v", s)
	}
}
