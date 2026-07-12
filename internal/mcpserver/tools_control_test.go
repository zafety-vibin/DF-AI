package mcpserver

import (
	"strings"
	"testing"

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
