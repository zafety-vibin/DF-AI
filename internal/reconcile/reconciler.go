package reconcile

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/plan"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Config tunes reconciler cadence and stall thresholds.
type Config struct {
	// ScanEvery is how often to walk PredictedState.Outstanding and
	// check confirmations / stalls. 1–2 seconds is a reasonable default.
	ScanEvery time.Duration

	// RetireEvery is how often to drop Confirmed predictions older than
	// RetainConfirmed.
	RetireEvery time.Duration

	// RetainConfirmed is the TTL for Confirmed predictions. Older entries
	// are removed.
	RetainConfirmed time.Duration

	// EventBufferSize is the size of the divergence ring buffer.
	EventBufferSize int

	// StallThreshold is how long a plan node can go with no observed
	// progress before the reconciler emits a stall event. Default 90s
	// — long enough that brief pauses don't trigger, short enough that
	// real blocks are detected within a couple deliberator cycles.
	StallThreshold time.Duration

	// StallReNotifyInterval is how often to re-emit stall events for a
	// node that has remained stalled. Stalls are dedupe'd via the plan
	// node's StallNotedAt field; this interval governs re-notification.
	// Default 5 minutes.
	StallReNotifyInterval time.Duration
}

// DefaultConfig returns reasonable defaults for early experimentation.
func DefaultConfig() Config {
	return Config{
		ScanEvery:             1500 * time.Millisecond,
		RetireEvery:           60 * time.Second,
		RetainConfirmed:       5 * time.Minute,
		EventBufferSize:       128,
		StallThreshold:        90 * time.Second,
		StallReNotifyInterval: 5 * time.Minute,
	}
}

// Reconciler is the background goroutine that compares predicted vs
// observed state, confirms predictions, tracks per-node progress, and
// emits stall events when work appears blocked.
type Reconciler struct {
	cfg    Config
	wm     *worldmodel.WorldModel
	dag    *plan.DAG
	events *EventBuffer
	logger *logging.Logger
}

// New wires the dependencies. cfg falls back to DefaultConfig() for any
// zero-valued fields.
func New(cfg Config, wm *worldmodel.WorldModel, dag *plan.DAG, logger *logging.Logger) *Reconciler {
	def := DefaultConfig()
	if cfg.ScanEvery <= 0 {
		cfg.ScanEvery = def.ScanEvery
	}
	if cfg.RetireEvery <= 0 {
		cfg.RetireEvery = def.RetireEvery
	}
	if cfg.RetainConfirmed <= 0 {
		cfg.RetainConfirmed = def.RetainConfirmed
	}
	if cfg.EventBufferSize <= 0 {
		cfg.EventBufferSize = def.EventBufferSize
	}
	if cfg.StallThreshold <= 0 {
		cfg.StallThreshold = def.StallThreshold
	}
	if cfg.StallReNotifyInterval <= 0 {
		cfg.StallReNotifyInterval = def.StallReNotifyInterval
	}
	return &Reconciler{
		cfg:    cfg,
		wm:     wm,
		dag:    dag,
		events: NewEventBuffer(cfg.EventBufferSize),
		logger: logger,
	}
}

// Events exposes the divergence ring buffer for the deliberator's renderer.
func (r *Reconciler) Events() *EventBuffer { return r.events }

// Run drives the reconciler loop until ctx cancels.
func (r *Reconciler) Run(ctx context.Context) {
	if r == nil {
		return
	}
	scanT := time.NewTicker(r.cfg.ScanEvery)
	retireT := time.NewTicker(r.cfg.RetireEvery)
	defer scanT.Stop()
	defer retireT.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-scanT.C:
			r.scan()
		case <-retireT.C:
			r.retire()
		}
	}
}

// scan is one pass: confirm predictions whose observed state matches,
// update per-node progress timestamps, emit stall events for nodes idle
// past StallThreshold, transition nodes to Done when fully confirmed.
//
// Stalls do NOT mark the node Failed. The plan node stays Active; the
// LLM is informed via the divergence buffer and decides what to do.
func (r *Reconciler) scan() {
	if r.wm == nil || r.wm.Predicted == nil {
		return
	}

	now := time.Now()

	// Step 1: confirm predictions whose observed state matches.
	confirmed := 0
	progressedNodes := map[string]struct{}{}
	for _, pred := range r.wm.Predicted.Outstanding() {
		if r.isConfirmed(pred) {
			if r.wm.Predicted.Confirm(pred.Coord) {
				confirmed++
				if pred.PlanNodeID != "" {
					progressedNodes[pred.PlanNodeID] = struct{}{}
				}
			}
		}
	}
	for nodeID := range progressedNodes {
		r.dag.NoteProgress(nodeID)
	}

	// Step 2: per-node bookkeeping — count confirmed/diverged, transition
	// fully-confirmed nodes to Done.
	if r.dag != nil {
		r.updateNodeCounts()
	}

	// Step 3: stall detection. Walk Active nodes; for any node whose
	// LastProgressAt is older than StallThreshold, emit a divergence
	// event (deduped via StallNotedAt + StallReNotifyInterval).
	stallEmitted := r.detectStalls(now)

	if r.logger != nil && (confirmed > 0 || stallEmitted > 0) {
		r.logger.Debug("reconcile: scan",
			logging.Field{Key: "confirmed", Value: confirmed},
			logging.Field{Key: "stalls_emitted", Value: stallEmitted})
	}
}

// retire trims old Confirmed predictions to bound memory.
func (r *Reconciler) retire() {
	if r.wm == nil || r.wm.Predicted == nil {
		return
	}
	cutoff := time.Now().Add(-r.cfg.RetainConfirmed)
	n := r.wm.Predicted.RetireConfirmedOlderThan(cutoff)
	if r.logger != nil && n > 0 {
		r.logger.Debug("reconcile: retired",
			logging.Field{Key: "count", Value: n})
	}
}

// isConfirmed reports whether the current observed state matches the
// prediction. For dig predictions this is "is the tile open now."
func (r *Reconciler) isConfirmed(pred worldmodel.TilePrediction) bool {
	if r.wm.Observed.Topology == nil {
		return false
	}
	open, err := r.wm.Observed.Topology.GetTile(pred.Coord.X, pred.Coord.Y, pred.Coord.Z)
	if err != nil {
		return false
	}
	return open
}

// detectStalls walks Active plan nodes and emits stall divergences for
// any whose LastProgressAt is older than StallThreshold. Returns the
// number of stall events emitted this pass.
func (r *Reconciler) detectStalls(now time.Time) int {
	if r.dag == nil {
		return 0
	}
	active := r.dag.Active()
	emitted := 0

	for _, n := range active {
		if n.PredictionCount == 0 {
			continue // nodes with no predictions can't stall meaningfully
		}
		// Only Outstanding predictions matter for stall — if all are
		// already Confirmed we'd be transitioning to Done elsewhere.
		if n.PredictionsConfirmed >= n.PredictionCount {
			continue
		}

		idle := now.Sub(n.LastProgressAt)
		if idle < r.cfg.StallThreshold {
			continue
		}

		// Dedupe: don't re-emit if we already noted a stall recently.
		if !n.StallNotedAt.IsZero() && now.Sub(n.StallNotedAt) < r.cfg.StallReNotifyInterval {
			continue
		}

		// Emit. Use the action's region centroid as the event coord.
		c := centroidOf(n.Action)
		note := fmt.Sprintf("no observed progress for %s; %d/%d predictions confirmed",
			idle.Round(time.Second), n.PredictionsConfirmed, n.PredictionCount)
		r.events.Append(DivergenceEvent{
			Coord:       c,
			Action:      describeAction(n.Action),
			PlanNodeID:  n.ID,
			PredictedAt: n.StartedAt,
			EmittedAt:   now,
			Reason:      ReasonStalled,
			IdleSeconds: int(idle.Seconds()),
			Note:        note,
		})
		r.dag.NoteStall(n.ID)
		emitted++

		if r.logger != nil {
			r.logger.Info("reconcile: stall noted",
				logging.Field{Key: "node", Value: n.ID},
				logging.Field{Key: "idle_s", Value: int(idle.Seconds())},
				logging.Field{Key: "confirmed", Value: n.PredictionsConfirmed},
				logging.Field{Key: "total", Value: n.PredictionCount})
		}
	}
	return emitted
}

// updateNodeCounts walks the DAG's Active nodes and tallies
// confirmed/diverged predictions per node. If all predictions for a node
// are confirmed, the node transitions to Done. Failure-by-divergence is
// NOT applied here; the only failure paths are ACK rejection at execute
// time (handled in plan/executor.go) and explicit-blocker divergences
// (future work, when the plugin starts reporting them).
func (r *Reconciler) updateNodeCounts() {
	active := r.dag.Active()
	if len(active) == 0 {
		return
	}

	// Derive confirmed = total - outstanding - diverged per node.
	divergedByNode := map[string]int{}
	outstandingByNode := map[string]int{}
	if r.wm.Predicted != nil {
		for _, pred := range r.wm.Predicted.Diverged() {
			divergedByNode[pred.PlanNodeID]++
		}
		for _, pred := range r.wm.Predicted.Outstanding() {
			outstandingByNode[pred.PlanNodeID]++
		}
	}

	for _, n := range active {
		if n.PredictionCount == 0 {
			continue
		}
		confirmed := n.PredictionCount - outstandingByNode[n.ID] - divergedByNode[n.ID]
		if confirmed < 0 {
			confirmed = 0
		}
		diverged := divergedByNode[n.ID]

		r.dag.Update(n.ID, func(node *plan.Node) {
			node.PredictionsConfirmed = confirmed
			node.PredictionsDiverged = diverged
		})

		if confirmed >= n.PredictionCount {
			r.dag.MarkDone(n.ID)
			if r.logger != nil {
				r.logger.Info("reconcile: node done",
					logging.Field{Key: "node", Value: n.ID},
					logging.Field{Key: "predictions", Value: n.PredictionCount})
			}
		}
	}
}

func centroidOf(a plan.Action) worldmodel.Coord {
	r := a.Region
	return worldmodel.Coord{
		X: (r.X1 + r.X2) / 2,
		Y: (r.Y1 + r.Y2) / 2,
		Z: (r.Z1 + r.Z2) / 2,
	}
}

func describeAction(a plan.Action) string {
	if a.Type == "dig" {
		return "dig:" + a.DigType
	}
	return a.Type
}

// observedFlagsAt is unused with the current divergence reasons (stall
// events don't carry flag deltas) but kept for future use when we wire
// contradicted/explicit-blocker reasons.
func (r *Reconciler) observedFlagsAt(c worldmodel.Coord) uint8 {
	if r.wm.Observed.Topology == nil {
		return 0
	}
	open, err := r.wm.Observed.Topology.GetTile(c.X, c.Y, c.Z)
	if err != nil {
		return 0
	}
	if open {
		return protocol.FlagFloor
	}
	return protocol.FlagWall
}
