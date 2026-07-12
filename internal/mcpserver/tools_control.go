package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/predicate"
)

func renderGoals(results []predicate.Result) string {
	var sb strings.Builder
	sb.WriteString("GOALS (predicates checked against live state):\n")
	for _, h := range []predicate.Horizon{predicate.HorizonNow, predicate.HorizonSoon, predicate.HorizonEventual} {
		filtered := predicate.FilterByHorizon(results, h)
		if len(filtered) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s:\n", strings.ToUpper(h.String()))
		for _, r := range filtered {
			mark := "[ ]"
			if r.Satisfied {
				mark = "[x]"
			}
			fmt.Fprintf(&sb, "  %s %s (confidence %.2f)\n", mark, r.Name, r.Confidence)
			for _, ev := range r.Evidence {
				fmt.Fprintf(&sb, "      - %s\n", ev)
			}
		}
	}
	return sb.String()
}

func simPaused(raw []byte) (paused, stepping bool) {
	var s struct {
		Paused   bool `json:"paused"`
		Stepping bool `json:"stepping"`
	}
	if json.Unmarshal(raw, &s) == nil {
		return s.Paused, s.Stepping
	}
	return false, false
}

func registerControlTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pause",
		Description: "Pause the DF simulation. Think while paused; nothing moves.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendPauseCommand(true)
		return withDash(b, ctx, ackText(res, err, "pause")), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unpause",
		Description: "Unpause DF and let it run free (prefer step for turn-based play).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendPauseCommand(false)
		return withDash(b, ctx, ackText(res, err, "unpause")), nil, nil
	})

	type stepIn struct {
		Ticks int `json:"ticks" jsonschema:"game ticks to run before auto-pausing. 1200 = one fortress day. 600 is a good default turn."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "step",
		Description: "Run the simulation for N ticks then auto-pause, and report what happened (new alerts, arrivals). This is your end-of-turn: act, then step, then observe.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stepIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		if in.Ticks <= 0 {
			in.Ticks = 600
		}
		if in.Ticks > 14400 {
			in.Ticks = 14400 // cap: ~12 game days per step
		}
		before := b.Snapshot()
		beforeAlerts := map[uint32]bool{}
		for _, a := range before.ActiveAlerts {
			beforeAlerts[a.ID] = true
		}
		res, err := b.Exec.SendStepCommand(uint32(in.Ticks))
		if err != nil || res == nil || !res.Success {
			return withDash(b, ctx, ackText(res, err, "step")), nil, nil
		}
		// Poll until the plugin re-pauses.
		timeout := time.Duration(in.Ticks/10+30) * time.Second
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			time.Sleep(500 * time.Millisecond)
			if !b.Connected() {
				return withDash(b, ctx, fmt.Sprintf(
					"step started (%d ticks) but connection to the plugin was lost mid-step; simulation state unknown — reconnect and check sim_status", in.Ticks)), nil, nil
			}
			raw, qerr := b.Query(ctx, "sim_status", "{}")
			if qerr != nil {
				continue
			}
			if paused, stepping := simPaused(raw); paused && !stepping {
				break
			}
		}
		after := b.Snapshot()
		var sb strings.Builder
		fmt.Fprintf(&sb, "stepped %d ticks (tick %d -> %d)\n", in.Ticks, before.Tick, after.Tick)
		if d := len(after.Entities.Dwarves) - len(before.Entities.Dwarves); d != 0 {
			fmt.Fprintf(&sb, "dwarf count change: %+d\n", d)
		}
		newAlerts := 0
		for _, a := range after.ActiveAlerts {
			if !beforeAlerts[a.ID] {
				newAlerts++
				if newAlerts <= 15 {
					fmt.Fprintf(&sb, "NEW ALERT [%d] %s\n", a.ID, a.Text)
				}
			}
		}
		if newAlerts == 0 {
			sb.WriteString("no new alerts\n")
		} else if newAlerts > 15 {
			fmt.Fprintf(&sb, "... and %d more new alerts (use alerts tool)\n", newAlerts-15)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_goals",
		Description: "Evaluate the survival/headroom/trajectory predicates against live state, with evidence. Your verification critic — call after milestones and before claiming success.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || b.Preds == nil {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console."), nil, nil
		}
		return withDash(b, ctx, renderGoals(b.Preds.CheckAll(b.WM))), nil, nil
	})
}
