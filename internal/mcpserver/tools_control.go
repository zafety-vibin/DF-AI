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
// the world model's message counter. Tripwire fields are optional (absent
// from older plugins): set when the plugin ended a step early on its own.
type simStatus struct {
	Paused         bool   `json:"paused"`
	Stepping       bool   `json:"stepping"`
	Frame          int64  `json:"frame"`
	Tripwire       bool   `json:"tripwire"`
	TripwireReason string `json:"tripwire_reason"`
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
// from sim_status), tile-delta count, population delta, new alerts, and
// repeated warnings. pushed=false means the world model never received a
// state push after the step completed — the deltas below would then be
// computed against frozen data, so say so.
//
// "tile deltas this step" diffs Snapshot.TileDeltaCount (before vs after) --
// a separate counter from EventSeq (ENTITY_UPDATE only, checked by pushed
// above) so a dead TILE_UPDATE stream is visible even when entity pushes
// keep arriving normally.
//
// beforeAlerts maps an alert ID present BEFORE the step to its RepeatCount
// at that time (DF report.repeat_count — see worldmodel.Alert). An ID
// missing from this map is treated as brand-new this step (reported via the
// NEW ALERT lines below, never double-counted as a repeat). An ID present
// in the map whose RepeatCount grew during the step means DF re-announced
// the SAME cancellation again without a new report id (the plugin resends
// the entry in place) — that's the "silent damp-cancel blindness" this
// summation exists to kill: a job that keeps failing must be visible even
// though its alert ID never changes.
func stepReport(ticks int, beforeFrame, afterFrame int64, pushed bool, before, after worldmodel.Snapshot, beforeAlerts map[uint32]uint32, tripwire bool, tripwireReason string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "stepped %d ticks (sim frame %s -> %s)\n", ticks, frameStr(beforeFrame), frameStr(afterFrame))
	if tripwire {
		reason := tripwireReason
		if reason == "" {
			reason = "unspecified"
		}
		fmt.Fprintf(&sb, "step ended EARLY at frame %s — tripwire: %s\n", frameStr(afterFrame), reason)
	}
	if !pushed {
		sb.WriteString("WARNING: no state push received after the step — deltas below may be stale; check the dashboard data age\n")
	}
	fmt.Fprintf(&sb, "tile deltas this step: %d\n", after.TileDeltaCount-before.TileDeltaCount)
	if d := len(after.Entities.Dwarves) - len(before.Entities.Dwarves); d != 0 {
		fmt.Fprintf(&sb, "dwarf count change: %+d\n", d)
	}
	newAlerts := 0
	type repeatedWarning struct {
		id    uint32
		text  string
		delta uint32
	}
	var repeated []repeatedWarning
	for _, a := range after.ActiveAlerts {
		beforeCount, existed := beforeAlerts[a.ID]
		if !existed {
			newAlerts++
			if newAlerts <= 15 {
				fmt.Fprintf(&sb, "NEW ALERT [%d] %s\n", a.ID, a.Text)
			}
			continue
		}
		if a.Severity >= 1 && a.RepeatCount > beforeCount {
			repeated = append(repeated, repeatedWarning{id: a.ID, text: a.Text, delta: a.RepeatCount - beforeCount})
		}
	}
	if newAlerts == 0 {
		sb.WriteString("no new alerts\n")
	} else if newAlerts > 15 {
		fmt.Fprintf(&sb, "... and %d more new alerts (use alerts tool)\n", newAlerts-15)
	}
	if len(repeated) == 0 {
		sb.WriteString("no repeated warnings this step\n")
	} else {
		for i, r := range repeated {
			if i >= 15 {
				fmt.Fprintf(&sb, "... and %d more repeated warnings this step (use alerts tool)\n", len(repeated)-15)
				break
			}
			fmt.Fprintf(&sb, "repeated warnings this step: '%s' x%d\n", r.text, r.delta)
		}
	}
	return sb.String()
}

func registerControlTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pause",
		Meta:        mcp.Meta{"anthropic/alwaysLoad": true},
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
		Meta:        mcp.Meta{"anthropic/alwaysLoad": true},
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
		Meta:        mcp.Meta{"anthropic/alwaysLoad": true},
		Description: "Run the simulation for N ticks then auto-pause, and report what happened (new alerts, arrivals). The plugin may trip an early auto-pause (a tripwire: e.g. a threat or flooding) — the report says so and why. This is your end-of-turn: act, then step, then observe.",
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
		beforeAlerts := map[uint32]uint32{}
		for _, a := range before.ActiveAlerts {
			beforeAlerts[a.ID] = a.RepeatCount
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
		// Poll until the plugin re-pauses. The budget scales with ticks, but
		// a slow sim (fluid churn, FPS dips) can be alive and still miss a
		// fixed deadline — observed frame progress extends the deadline by a
		// grace window, capped at 3x the original budget. Give up only when
		// frames stall or the cap is hit, and say which case occurred.
		const progressGrace = 30 * time.Second
		budget := time.Duration(in.Ticks/10+30) * time.Second
		start := time.Now()
		hardDeadline := start.Add(3 * budget)
		deadline := start.Add(budget)
		completed := false
		afterFrame := beforeFrame
		lastStatus := simStatus{Frame: -1}
		lastProgress := start
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
			lastStatus = s
			if s.Frame > afterFrame {
				// The sim advanced: it's alive, keep waiting.
				lastProgress = time.Now()
				if ext := lastProgress.Add(progressGrace); ext.After(deadline) {
					deadline = ext
					if deadline.After(hardDeadline) {
						deadline = hardDeadline
					}
				}
			}
			afterFrame = s.Frame
			if s.Paused && !s.Stepping {
				completed = true
				break
			}
		}
		if !completed {
			elapsed := time.Since(start).Round(time.Second)
			switch {
			case time.Since(lastProgress) < progressGrace:
				// Frames were still advancing when the 3x cap hit: the game
				// is running slowly, not stuck.
				return withDash(b, ctx, fmt.Sprintf(
					"step of %d ticks DID NOT complete within %s (3x budget cap) but the sim is STILL ADVANCING (sim frame %s) — the game is running slowly, not stuck; wait and check sim_status before acting",
					in.Ticks, elapsed, frameStr(afterFrame))), nil, nil
			case lastStatus.Frame < 0:
				// Not one sim_status response landed: we know nothing.
				return withDash(b, ctx, fmt.Sprintf(
					"step of %d ticks DID NOT complete within %s and NO sim_status responses were observed — connection may be degraded; check sim_status before acting",
					in.Ticks, elapsed)), nil, nil
			case lastStatus.Paused:
				return withDash(b, ctx, fmt.Sprintf(
					"step of %d ticks DID NOT complete within %s — the game is PAUSED mid-step (sim frame %s), possibly paused manually; check sim_status before acting",
					in.Ticks, elapsed, frameStr(afterFrame))), nil, nil
			default:
				return withDash(b, ctx, fmt.Sprintf(
					"step of %d ticks DID NOT complete within %s — sim frames STALLED while unpaused (sim frame %s); the game may be wedged, check sim_status before acting",
					in.Ticks, elapsed, frameStr(afterFrame))), nil, nil
			}
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
		return withDash(b, ctx, stepReport(in.Ticks, beforeFrame, afterFrame, pushed, before, after, beforeAlerts, lastStatus.Tripwire, lastStatus.TripwireReason)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_goals",
		Description: "Evaluate the survival/headroom/trajectory predicates against live state, with evidence. Your verification critic — call after milestones and before claiming success.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || b.Preds == nil {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console."), nil, nil
		}
		// Pre-fetch a live dug-tile count (region_scan over the fort's
		// current-activity z-level, see liveDugTileCount) and install it on
		// the world model just for this CheckAll pass, so
		// HasModifiedAnything/HasShelter can prefer it over the session-delta
		// Modifications overlay without every predicate's Check(wm) signature
		// needing to change. Cleared immediately after so a stale count never
		// leaks into an unrelated later check.
		if count, source, ok := liveDugTileCount(ctx, b); ok {
			b.WM.SetLiveDugTiles(count, source)
			defer b.WM.ClearLiveDugTiles()
		}
		return withDash(b, ctx, renderGoals(b.Preds.CheckAll(b.WM))), nil, nil
	})
}
