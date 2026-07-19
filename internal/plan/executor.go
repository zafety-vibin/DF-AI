package plan

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Executor drains Ready nodes from a DAG, dispatches their actions to the
// dfhack plugin, and writes per-tile predictions into the world model's
// predicted overlay so the reconciler can confirm or diverge them.
//
// The executor is a single goroutine. If any single dispatch is slow (the
// plugin's ACK wait is up to 5 s), other Ready nodes wait. That is fine
// for now — at this stage we expect 1–3 nodes per deliberator turn and the
// reconciler doesn't depend on dispatch ordering.
type Executor struct {
	dag       *DAG
	cmdExec   *commands.CommandExecutor
	wm        *worldmodel.WorldModel
	logger    *logging.Logger
	pollEvery time.Duration

	// WorkConstants tunes the deadline formula for dig predictions. The
	// executor uses these to compute DeadlineAt on each prediction it
	// writes; the reconciler reads the deadline.
	WorkConstants worldmodel.WorkConstants
}

// NewExecutor wires the dependencies. pollEvery <= 0 falls back to 500ms.
func NewExecutor(dag *DAG, cmdExec *commands.CommandExecutor, wm *worldmodel.WorldModel, logger *logging.Logger, pollEvery time.Duration) *Executor {
	if pollEvery <= 0 {
		pollEvery = 500 * time.Millisecond
	}
	return &Executor{
		dag:           dag,
		cmdExec:       cmdExec,
		wm:            wm,
		logger:        logger,
		pollEvery:     pollEvery,
		WorkConstants: worldmodel.DefaultWorkConstants(),
	}
}

// Run drives the executor loop until ctx cancels.
func (e *Executor) Run(ctx context.Context) {
	if e == nil {
		return
	}
	t := time.NewTicker(e.pollEvery)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			e.drain()
		}
	}
}

// drain dispatches every currently-Ready node. Errors are logged and the
// node is marked Failed; the deliberator decides whether to retry next
// turn.
func (e *Executor) drain() {
	ready := e.dag.Ready()
	if len(ready) == 0 {
		return
	}
	for _, n := range ready {
		e.executeNode(n)
	}
}

func (e *Executor) executeNode(n Node) {
	if !e.dag.MarkActive(n.ID) {
		return // someone else picked it up, or it's no longer Ready
	}

	switch n.Action.Type {
	case "dig":
		e.executeDig(n)
	case "build":
		e.executeBuild(n)
	case "zone":
		e.executeZone(n)
	case "stockpile":
		e.executeStockpile(n)
	case "smooth":
		e.executeSmooth(n)
	case "unsuspend":
		e.executeUnsuspend(n)
	case "order":
		e.executeOrder(n)
	case "chop":
		e.executeSimple(n, "chop")
	case "gather":
		e.executeSimple(n, "gather")
	case "dismiss":
		e.executeDismiss(n)
	case "wait":
		e.markDone(n.ID, "wait completed (no-op)")
	default:
		e.markFailed(n.ID, fmt.Sprintf("unknown action type %q", n.Action.Type))
	}
}

// executeZone designates a region as a civzone (bedroom / dining / etc.).
// Zones have no per-tile prediction — the plugin either accepts the
// designation or rejects it. ACK success → Done; the zone shows up in the
// next ENTITY_UPDATE's ZoneData. Predicates can then count zones via
// WorldModel.Observed.Zones.
func (e *Executor) executeZone(n Node) {
	r := n.Action.Region
	res, err := e.cmdExec.SendZoneCommand(n.Action.ZoneType, r.X1, r.Y1, r.Z1, r.X2, r.Y2)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("zone: %s", msg))
		return
	}
	e.markDone(n.ID, fmt.Sprintf("zone ack accepted (type 0x%02x)", n.Action.ZoneType))
}

// executeStockpile designates a rectangular region as a stockpile zone
// accepting the named category groups. ACK success → Done; the stockpile
// shows up at its position in DF. NOTE: per-material sub-flags are not
// set in this build — the stockpile's category tabs will be enabled but
// individual materials may need manual click-through in DF the first
// time. Future plugin work will fill sub-params via DFHack helpers.
func (e *Executor) executeStockpile(n Node) {
	r := n.Action.Region
	res, err := e.cmdExec.SendStockpileCommand(r.X1, r.Y1, r.Z1, r.X2, r.Y2, n.Action.StockpileGroups)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("stockpile: %s", msg))
		return
	}
	e.markDone(n.ID, fmt.Sprintf("stockpile ack accepted (groups 0x%05x)", n.Action.StockpileGroups))
}

// executeDismiss marks one or all DF announcements as acknowledged in the
// AlertStore. No plugin round-trip — dismissal is an orchestrator-side
// state change that controls what the next snapshot shows.
func (e *Executor) executeDismiss(n Node) {
	if e.wm == nil || e.wm.Observed.Alerts == nil {
		e.markDone(n.ID, "dismiss: no alert store")
		return
	}
	if n.Action.AlertID == 0 {
		count := e.wm.Observed.Alerts.DismissAll()
		e.markDone(n.ID, fmt.Sprintf("dismissed %d alerts", count))
		return
	}
	if e.wm.Observed.Alerts.Dismiss(n.Action.AlertID) {
		e.markDone(n.ID, fmt.Sprintf("dismissed alert %d", n.Action.AlertID))
	} else {
		e.markFailed(n.ID, fmt.Sprintf("alert %d not found or already dismissed", n.Action.AlertID))
	}
}

// executeSmooth designates a region for smoothing (or engraving). Only
// natural stone walls/floors are valid targets; the plugin sets the
// designation regardless and DF's labor system ignores invalid tiles. Used
// for sealing light aquifer leaks on stone layers.
func (e *Executor) executeSmooth(n Node) {
	r := n.Action.Region
	st := n.Action.SmoothType
	if st == 0 {
		st = protocol.SmoothTypeSmooth
	}
	res, err := e.cmdExec.SendSmoothCommand(st, r.X1, r.Y1, r.Z1, r.X2, r.Y2)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("smooth: %s", msg))
		return
	}
	e.markDone(n.ID, fmt.Sprintf("smooth ack accepted (type 0x%02x)", st))
}

// executeUnsuspend clears suspend on jobs at the given tile. Used to resume
// auto-suspended constructions.
func (e *Executor) executeUnsuspend(n Node) {
	r := n.Action.Region
	res, err := e.cmdExec.SendUnsuspendCommand(r.X1, r.Y1, r.Z1)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("unsuspend: %s", msg))
		return
	}
	e.markDone(n.ID, "unsuspend ack accepted")
}

// executeOrder adds a manager work order. Manager assigns to whichever
// workshop fits. Without a manager dwarf and a manager office, DF queues
// the order without dispatching — predicate logic should warn the LLM.
func (e *Executor) executeOrder(n Node) {
	res, err := e.cmdExec.SendWorkOrderCommand(n.Action.OrderType, n.Action.Quantity, "", "", protocol.WorkOrderFrequencyOneTime)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("order: %s", msg))
		return
	}
	e.markDone(n.ID, fmt.Sprintf("order ack accepted (type 0x%02x x%d)", n.Action.OrderType, n.Action.Quantity))
}

// executeBuild dispatches a build command (workshop / furniture /
// construction / door). Builds don't currently produce TilePredictions —
// the topology overlay tracks open/closed only, not building presence.
// Once CONSTRUCTION_UPDATE is added to the protocol, builds gain
// reconciler-driven confirmation; for now, ACK success → Done, ACK
// rejection → Failed.
func (e *Executor) executeBuild(n Node) {
	r := n.Action.Region
	res, err := e.cmdExec.SendBuildCommand(r.X1, r.Y1, r.Z1, n.Action.BuildType)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("build: %s", msg))
		return
	}
	e.markDone(n.ID, fmt.Sprintf("build ack accepted (type 0x%02x)", n.Action.BuildType))
	if e.logger != nil {
		e.logger.Info("plan: build dispatched",
			logging.Field{Key: "node", Value: n.ID},
			logging.Field{Key: "build_type", Value: fmt.Sprintf("0x%02x", n.Action.BuildType)},
			logging.Field{Key: "pos", Value: fmt.Sprintf("(%d,%d,%d)", r.X1, r.Y1, r.Z1)})
	}
}

// executeDig dispatches a single dig command to the plugin. The plugin
// handles 3D rectangles natively — for stair shafts (digType=stairs/
// upstair/downstair with Z1 != Z2), the plugin auto-assigns UpStair on
// the bottom Z, DownStair on the top, UpDownStair on middles. We send
// ONE command for the whole region (no per-Z fan-out).
func (e *Executor) executeDig(n Node) {
	r := n.Action.Region
	digType := mapDigType(n.Action.DigType)

	res, err := e.cmdExec.SendDigRegion(digType, r.X1, r.Y1, r.Z1, r.X2, r.Y2, r.Z2)
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "ack rejected"
		if res != nil && res.ErrorMsg != "" {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, msg)
		return
	}

	predictions := e.writeDigPredictionsForRegion(n)

	e.dag.Update(n.ID, func(n *Node) {
		n.PredictionCount = predictions
		n.Result = fmt.Sprintf("ack accepted, %d predictions", predictions)
	})

	if e.logger != nil {
		e.logger.Info("plan: dig dispatched",
			logging.Field{Key: "node", Value: n.ID},
			logging.Field{Key: "predictions", Value: predictions},
			logging.Field{Key: "region", Value: fmt.Sprintf("(%d,%d,%d)-(%d,%d,%d)", r.X1, r.Y1, r.Z1, r.X2, r.Y2, r.Z2)})
	}
}


// writeDigPredictionsForRegion writes one TilePrediction per (x, y, z) in
// the action's region, expecting each tile to become walkable. Stair-type
// digs across Z1 != Z2 still produce one prediction per tile (the whole
// shaft), since DF designates each tile of the shaft independently.
func (e *Executor) writeDigPredictionsForRegion(n Node) int {
	if e.wm == nil || e.wm.Predicted == nil {
		return 0
	}
	r := n.Action.Region
	tilesInRegion := r.TileCount()
	isStair := n.Action.DigType == "stairs" || n.Action.DigType == "updownstairs" ||
		n.Action.DigType == "upstair" || n.Action.DigType == "downstair"

	count := 0
	for z := r.Z1; z <= r.Z2; z++ {
		for x := r.X1; x <= r.X2; x++ {
			for y := r.Y1; y <= r.Y2; y++ {
				label := digActionLabel(n.Action.DigType)
				if isStair {
					label = stairActionLabel(n.Action.DigType)
				}
				pred := worldmodel.TilePrediction{
					Coord:           worldmodel.Coord{X: x, Y: y, Z: z},
					Action:          label,
					PredictedFlags:  protocol.FlagFloor, // stair / dug tiles become walkable
					PlanNodeID:      n.ID,
					PredictedAtTick: e.wm.Tick(),
					Estimate: worldmodel.WorkEstimate{
						TileCount:   tilesInRegion,
						Material:    worldmodel.MaterialStone, // worst-case
						Parallelism: 2,
					},
				}
				e.wm.Predicted.Predict(pred)
				count++
			}
		}
	}
	return count
}

func (e *Executor) executeSimple(n Node, kind string) {
	r := n.Action.Region
	var (
		res *commands.CommandResult
		err error
	)
	switch kind {
	case "chop":
		res, err = e.cmdExec.SendChopCommand(r.X1, r.Y1, r.Z1, r.X2, r.Y2)
	case "gather":
		res, err = e.cmdExec.SendGatherCommand(r.X1, r.Y1, r.Z1, r.X2, r.Y2)
	default:
		e.markFailed(n.ID, fmt.Sprintf("unknown simple kind %q", kind))
		return
	}
	if err != nil {
		e.markFailed(n.ID, err.Error())
		return
	}
	if res == nil || !res.Success {
		msg := "rejected"
		if res != nil {
			msg = res.ErrorMsg
		}
		e.markFailed(n.ID, fmt.Sprintf("%s: %s", kind, msg))
		return
	}
	// Chop and gather don't currently produce TilePredictions — there's no
	// observable per-tile state change to predict against. Mark Done
	// immediately on ACK; the deliberator will see the action accepted and
	// move on. (When we add tree-cleared / plant-removed observations,
	// these become full reconciled predictions.)
	e.markDone(n.ID, fmt.Sprintf("%s ack accepted", kind))
}

func (e *Executor) markDone(id, result string) {
	e.dag.Update(id, func(n *Node) {
		if n.IsTerminal() {
			return
		}
		n.Status = StatusDone
		n.DoneAt = time.Now()
		n.Result = result
	})
}

func (e *Executor) markFailed(id, reason string) {
	e.dag.Update(id, func(n *Node) {
		if n.IsTerminal() {
			return
		}
		n.Status = StatusFailed
		n.DoneAt = time.Now()
		n.Result = reason
	})
	if e.logger != nil {
		e.logger.Warn("plan: node failed",
			logging.Field{Key: "node", Value: id},
			logging.Field{Key: "reason", Value: reason})
	}
}

func mapDigType(digType string) uint8 {
	switch digType {
	case "stairs", "updownstairs":
		return protocol.DigTypeUpDownStair
	case "channel":
		return protocol.DigTypeChannel
	case "ramp":
		return protocol.DigTypeRamp
	case "downstair":
		return protocol.DigTypeDownStair
	case "upstair":
		return protocol.DigTypeUpStair
	default:
		return protocol.DigTypeDefault
	}
}

func digActionLabel(digType string) string {
	if digType == "" {
		return "dig"
	}
	return "dig:" + digType
}

func stairActionLabel(digType string) string {
	if digType == "" {
		return "stair"
	}
	return "stair:" + digType
}
