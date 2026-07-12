package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/predicate"
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
