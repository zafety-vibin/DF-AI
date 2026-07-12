package reconcile

import (
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// DivergenceReason classifies why the reconciler is emitting a divergence
// event. The set is intentionally small — these are the named failure
// modes the LLM can reason about. New reasons get added as new
// observability surfaces come online (e.g., when CONSTRUCTION_UPDATE
// lands, ReasonConstructionSuspended starts firing).
type DivergenceReason string

const (
	// ReasonStalled: a plan node has had no observed progress in
	// StallThreshold seconds. NOT a failure — the node stays Active.
	// The reconciler emits this so the LLM can decide whether to wait,
	// redirect workers, or investigate a blocker.
	ReasonStalled DivergenceReason = "stalled"

	// ReasonContradicted: an observation arrived that affirmatively
	// contradicts a prediction (e.g., a tile predicted to become open
	// got built on instead). This DOES mark the prediction Diverged.
	// Rare; requires action-specific observation logic.
	ReasonContradicted DivergenceReason = "contradicted"

	// ReasonExplicitBlocker: the plugin reported a specific reason the
	// work cannot proceed (construction suspended, no path, missing
	// material). Placeholder until CONSTRUCTION_UPDATE lands.
	ReasonExplicitBlocker DivergenceReason = "explicit_blocker"
)

// DivergenceEvent is one observed-vs-predicted mismatch or a stall on a
// plan node. The reconciler pushes one of these onto its ring buffer per
// detected condition.
type DivergenceEvent struct {
	// Coord is the affected tile (if applicable). For node-level stalls,
	// this is the centroid of the node's region.
	Coord worldmodel.Coord

	// Action describes what the prediction or plan was trying to do.
	Action string

	// PlanNodeID identifies the plan node the event belongs to.
	PlanNodeID string

	// PredictedAt and EmittedAt are wall-clock stamps for ordering and
	// dataset replay.
	PredictedAt time.Time
	EmittedAt   time.Time

	// Reason classifies the event. See the constants above.
	Reason DivergenceReason

	// IdleSeconds is how long since the plan node last saw progress.
	// Populated for ReasonStalled events.
	IdleSeconds int

	// PredictedFlags / ObservedFlags carry the per-flag delta when
	// available. Empty / zero for stall events.
	PredictedFlags uint8
	ObservedFlags  uint8

	// Note carries any extra reconciler-supplied context as a short
	// human-readable string. Used in dataset rows and shown to the LLM.
	Note string
}

// EventBuffer is a thread-safe bounded ring buffer of DivergenceEvents.
// The reconciler appends; the deliberator reads via Recent().
type EventBuffer struct {
	mu     sync.RWMutex
	events []DivergenceEvent
	cap    int
}

// NewEventBuffer returns a buffer holding the most recent capacity events.
// capacity <= 0 falls back to 64.
func NewEventBuffer(capacity int) *EventBuffer {
	if capacity <= 0 {
		capacity = 64
	}
	return &EventBuffer{cap: capacity}
}

// Append adds e to the buffer, evicting the oldest entry when full.
func (b *EventBuffer) Append(e DivergenceEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.events) < b.cap {
		b.events = append(b.events, e)
		return
	}
	b.events = append(b.events[1:], e)
}

// Recent returns up to n most-recent events (newest first). n <= 0 returns
// everything.
func (b *EventBuffer) Recent(n int) []DivergenceEvent {
	b.mu.RLock()
	defer b.mu.RUnlock()
	if len(b.events) == 0 {
		return nil
	}
	if n <= 0 || n > len(b.events) {
		n = len(b.events)
	}
	out := make([]DivergenceEvent, 0, n)
	for i := len(b.events) - 1; i >= len(b.events)-n; i-- {
		out = append(out, b.events[i])
	}
	return out
}

// Len returns the current buffer occupancy.
func (b *EventBuffer) Len() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.events)
}
