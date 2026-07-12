package worldmodel

import (
	"sync"
	"time"
)

// PredictionStatus is the lifecycle state of a single TilePrediction.
type PredictionStatus uint8

const (
	// PredictionOutstanding means the prediction has been recorded but
	// observation has not yet caught up to it.
	PredictionOutstanding PredictionStatus = iota

	// PredictionConfirmed means an observation arrived that matched the
	// prediction. The reconciler retires confirmed predictions on a cadence;
	// they are not removed eagerly so curious tooling can inspect recent
	// confirmations.
	PredictionConfirmed

	// PredictionDiverged means the prediction's deadline passed (or an
	// observation explicitly contradicted it) without the prediction landing.
	// Divergence is the dataset signal: every Diverged prediction is a row of
	// "command language failed to produce intended outcome."
	PredictionDiverged
)

// String returns a human-readable label for log lines and JSON.
func (s PredictionStatus) String() string {
	switch s {
	case PredictionOutstanding:
		return "outstanding"
	case PredictionConfirmed:
		return "confirmed"
	case PredictionDiverged:
		return "diverged"
	default:
		return "unknown"
	}
}

// TilePrediction is a single per-tile expectation stamped with the plan node
// that produced it. The reconciler watches Outstanding predictions for
// observed-vs-predicted match against the topology overlay.
//
// There is intentionally no deadline. Predictions stay Outstanding until
// observation catches up (Confirmed), the plugin reports an explicit
// blocker (Diverged with a specific reason), or the parent plan node is
// retired. Stall detection lives at the plan-node level (LastProgressAt),
// not at the per-tile level — because "this specific tile hasn't been
// touched yet" is meaningless when work is happening on adjacent tiles.
//
// Estimate is carried for telemetry (so the LLM can see "this work is
// roughly ~100 minutes of dwarf-time") but is NOT used to time-out
// predictions.
type TilePrediction struct {
	Coord           Coord
	Action          string // "dig", "channel", "stair_up", "stair_down", "stair_updown", "build_floor", ...
	PredictedFlags  uint8  // expected protocol.Flag* bits after the action lands
	PredictedTile   uint16 // expected DF tiletype enum value (0 if don't care)
	PlanNodeID      string
	PredictedAtTick uint64
	PredictedAt     time.Time    // wall-clock time the prediction was committed
	Estimate        WorkEstimate // work model carried for telemetry; NOT a deadline
	Status          PredictionStatus
	UpdatedAt       time.Time
	DivergedReason  string // populated when Status == PredictionDiverged
}

// PredictedState is the sparse overlay of in-flight predictions. It is
// accessed concurrently by the executor (writes), the reconciler (writes),
// and snapshot/render code (reads).
type PredictedState struct {
	mu sync.RWMutex

	tiles      map[Coord]*TilePrediction
	byPlanNode map[string]map[Coord]struct{}

	outstandingCount int
	confirmedCount   int
	divergedCount    int
}

func newPredictedState() *PredictedState {
	return &PredictedState{
		tiles:      make(map[Coord]*TilePrediction),
		byPlanNode: make(map[string]map[Coord]struct{}),
	}
}

// Predict records (or replaces) a tile prediction. PredictedAt defaults to
// now if zero. UpdatedAt, Status, and DivergedReason are always overwritten.
// The caller is responsible for setting PredictedAtTick (the heartbeat
// sequence). Estimate is stored for telemetry but does NOT establish a
// deadline.
func (p *PredictedState) Predict(pred TilePrediction) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if existing, ok := p.tiles[pred.Coord]; ok {
		// Replacing an existing prediction. Decrement the old status counter,
		// then drop the old plan-node back-reference if it differs.
		p.adjustCount(existing.Status, -1)
		if existing.PlanNodeID != pred.PlanNodeID {
			if set, ok := p.byPlanNode[existing.PlanNodeID]; ok {
				delete(set, existing.Coord)
				if len(set) == 0 {
					delete(p.byPlanNode, existing.PlanNodeID)
				}
			}
		}
	}

	now := time.Now()
	if pred.PredictedAt.IsZero() {
		pred.PredictedAt = now
	}

	pred.Status = PredictionOutstanding
	pred.UpdatedAt = now
	pred.DivergedReason = ""

	stored := pred
	p.tiles[pred.Coord] = &stored

	if pred.PlanNodeID != "" {
		set, ok := p.byPlanNode[pred.PlanNodeID]
		if !ok {
			set = make(map[Coord]struct{})
			p.byPlanNode[pred.PlanNodeID] = set
		}
		set[pred.Coord] = struct{}{}
	}

	p.outstandingCount++
}

// Get returns a copy of the prediction at the given coord and a found-flag.
func (p *PredictedState) Get(c Coord) (TilePrediction, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if pred, ok := p.tiles[c]; ok {
		return *pred, true
	}
	return TilePrediction{}, false
}

// Confirm marks the prediction at coord as confirmed. Returns true if the
// transition actually happened (i.e., a prediction existed and was previously
// Outstanding). Confirmations on already-confirmed or diverged predictions
// return false.
func (p *PredictedState) Confirm(c Coord) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	pred, ok := p.tiles[c]
	if !ok || pred.Status != PredictionOutstanding {
		return false
	}
	p.adjustCount(pred.Status, -1)
	pred.Status = PredictionConfirmed
	pred.UpdatedAt = time.Now()
	p.adjustCount(pred.Status, +1)
	return true
}

// MarkDiverged transitions the prediction to Diverged with a reason string.
// Returns true only if there was a prediction to transition. The reconciler
// calls this when DeadlineTick has passed without a matching observation.
func (p *PredictedState) MarkDiverged(c Coord, reason string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	pred, ok := p.tiles[c]
	if !ok || pred.Status != PredictionOutstanding {
		return false
	}
	p.adjustCount(pred.Status, -1)
	pred.Status = PredictionDiverged
	pred.DivergedReason = reason
	pred.UpdatedAt = time.Now()
	p.adjustCount(pred.Status, +1)
	return true
}

// RemoveByPlanNode removes every prediction associated with planNodeID,
// regardless of status. Returns the number of entries removed. Callers use
// this when a plan node is cancelled before completion.
func (p *PredictedState) RemoveByPlanNode(planNodeID string) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	set, ok := p.byPlanNode[planNodeID]
	if !ok {
		return 0
	}
	removed := 0
	for c := range set {
		if pred, ok := p.tiles[c]; ok {
			p.adjustCount(pred.Status, -1)
			delete(p.tiles, c)
			removed++
		}
	}
	delete(p.byPlanNode, planNodeID)
	return removed
}

// RetireConfirmedOlderThan deletes Confirmed predictions whose UpdatedAt is
// before cutoff. Returns the number of entries retired. The reconciler calls
// this on a slow cadence to bound memory.
func (p *PredictedState) RetireConfirmedOlderThan(cutoff time.Time) int {
	p.mu.Lock()
	defer p.mu.Unlock()

	retired := 0
	for c, pred := range p.tiles {
		if pred.Status != PredictionConfirmed {
			continue
		}
		if pred.UpdatedAt.After(cutoff) {
			continue
		}
		p.adjustCount(pred.Status, -1)
		delete(p.tiles, c)
		retired++

		if set, ok := p.byPlanNode[pred.PlanNodeID]; ok {
			delete(set, c)
			if len(set) == 0 {
				delete(p.byPlanNode, pred.PlanNodeID)
			}
		}
	}
	return retired
}

// Outstanding returns a snapshot copy of every currently outstanding
// prediction. Suitable for rendering into a deliberator prompt.
func (p *PredictedState) Outstanding() []TilePrediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]TilePrediction, 0, p.outstandingCount)
	for _, pred := range p.tiles {
		if pred.Status == PredictionOutstanding {
			out = append(out, *pred)
		}
	}
	return out
}

// Diverged returns a snapshot copy of every prediction currently in the
// Diverged state. The persistence pipeline reads these to build training
// rows.
func (p *PredictedState) Diverged() []TilePrediction {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := make([]TilePrediction, 0, p.divergedCount)
	for _, pred := range p.tiles {
		if pred.Status == PredictionDiverged {
			out = append(out, *pred)
		}
	}
	return out
}

// summary returns the counters for embedding in a Snapshot. Caller must hold
// the WorldModel's read lock; this method takes its own read lock on
// PredictedState.
func (p *PredictedState) summary() PredictedSummary {
	p.mu.RLock()
	defer p.mu.RUnlock()

	out := PredictedSummary{
		OutstandingCount: p.outstandingCount,
		ConfirmedCount:   p.confirmedCount,
		DivergedCount:    p.divergedCount,
	}
	if p.outstandingCount > 0 {
		out.Outstanding = make([]TilePrediction, 0, p.outstandingCount)
		for _, pred := range p.tiles {
			if pred.Status == PredictionOutstanding {
				out.Outstanding = append(out.Outstanding, *pred)
			}
		}
	}
	return out
}

// adjustCount nudges the per-status counters. Caller must hold the write
// lock.
func (p *PredictedState) adjustCount(s PredictionStatus, delta int) {
	switch s {
	case PredictionOutstanding:
		p.outstandingCount += delta
	case PredictionConfirmed:
		p.confirmedCount += delta
	case PredictionDiverged:
		p.divergedCount += delta
	}
}
