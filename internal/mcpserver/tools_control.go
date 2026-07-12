package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/worldmodel"
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

// simStatus is the decoded sim_status query payload. Frame is DF's
// simulation frame counter (world->frame_counter) — real game time, unlike
// the world model's message counter.
type simStatus struct {
	Paused   bool  `json:"paused"`
	Stepping bool  `json:"stepping"`
	Frame    int64 `json:"frame"`
}

func parseSimStatus(raw []byte) (simStatus, bool) {
	var s simStatus
	if err := json.Unmarshal(raw, &s); err != nil {
		return simStatus{}, false
	}
	return s, true
}

func simPaused(raw []byte) (paused, stepping bool) {
	s, _ := parseSimStatus(raw)
	return s.Paused, s.Stepping
}

// frameStr renders a sim frame, using "?" for the -1 sentinel (frame never
// observed, e.g. the pre-step sim_status fetch failed).
func frameStr(f int64) string {
	if f < 0 {
		return "?"
	}
	return fmt.Sprintf("%d", f)
}

// stepReport builds the post-step summary: sim-frame progress (game time,
// from sim_status), population delta, and new alerts. pushed=false means
// the world model never received a state push after the step completed —
// the deltas below would then be computed against frozen data, so say so.
func stepReport(ticks int, beforeFrame, afterFrame int64, pushed bool, before, after worldmodel.Snapshot, beforeAlerts map[uint32]bool) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "stepped %d ticks (sim frame %s -> %s)\n", ticks, frameStr(beforeFrame), frameStr(afterFrame))
	if !pushed {
		sb.WriteString("WARNING: no state push received after the step — deltas below may be stale; check the dashboard data age\n")
	}
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
	return sb.String()
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
		// Capture the sim frame before stepping so the report speaks in
		// SIM FRAMES (game time), not the world model's message counter.
		beforeFrame := int64(-1)
		if raw, qerr := b.Query(ctx, "sim_status", "{}"); qerr == nil {
			if s, ok := parseSimStatus(raw); ok {
				beforeFrame = s.Frame
			}
		}
		res, err := b.Exec.SendStepCommand(uint32(in.Ticks))
		if err != nil || res == nil || !res.Success {
			return withDash(b, ctx, ackText(res, err, "step")), nil, nil
		}
		// Poll until the plugin re-pauses.
		timeout := time.Duration(in.Ticks/10+30) * time.Second
		deadline := time.Now().Add(timeout)
		completed := false
		afterFrame := int64(-1)
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
			s, ok := parseSimStatus(raw)
			if !ok {
				continue
			}
			afterFrame = s.Frame
			if s.Paused && !s.Stepping {
				completed = true
				break
			}
		}
		if !completed {
			return withDash(b, ctx, fmt.Sprintf(
				"step of %d ticks DID NOT complete within %s — game may still be running; check sim_status before acting (last seen sim frame %s)",
				in.Ticks, timeout.Round(time.Second), frameStr(afterFrame))), nil, nil
		}
		// The plugin pushes ENTITY_UPDATE + tile deltas at auto-repause.
		// Wait briefly for the world model to ingest that push so the
		// 'after' snapshot reflects the completed step, not frozen
		// pre-step state.
		pushed := false
		pushDeadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(pushDeadline) {
			if b.Snapshot().EventSeq > before.EventSeq {
				pushed = true
				break
			}
			time.Sleep(100 * time.Millisecond)
		}
		after := b.Snapshot()
		return withDash(b, ctx, stepReport(in.Ticks, beforeFrame, afterFrame, pushed, before, after, beforeAlerts)), nil, nil
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
