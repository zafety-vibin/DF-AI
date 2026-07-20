package worldmodel

import (
	"sync"
	"sync/atomic"
	"time"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// Coord is the package-local 3D coordinate type. Defined here (rather than
// reusing one of the existing per-package coord types) so plan, trigger, and
// reconcile packages can share a stable shape without circular imports.
type Coord struct {
	X int16
	Y int16
	Z int16
}

// CoordFromTile builds a Coord from a protocol.TileState.
func CoordFromTile(t protocol.TileState) Coord { return Coord{X: t.X, Y: t.Y, Z: t.Z} }

// CoordFromEntity builds a Coord from a protocol.EntityInfo.
func CoordFromEntity(e protocol.EntityInfo) Coord { return Coord{X: e.X, Y: e.Y, Z: e.Z} }

// WorldModel holds the three-layer state shared by perception, deliberator,
// executor, and reconciler. Its methods are safe for concurrent use.
type WorldModel struct {
	mu sync.RWMutex

	Observed  ObservedState
	Predicted *PredictedState

	plan PlanReader // optional; nil until plan package is wired

	logger *logging.Logger

	tick       atomic.Uint64
	eventSeq   atomic.Uint64
	lastUpdate atomic.Int64 // unix nanos
	bornAt     time.Time
}

// ObservedState mirrors what DF currently reports. The overlay pointers are
// borrowed: WorldModel does NOT construct or own them. Snapshots (Entities,
// Zones, Fort) are owned by WorldModel because they aren't stored anywhere
// else in canonical form.
type ObservedState struct {
	Topology      *topology.TopologyOverlay
	Hazards       *hazards.HazardManager
	Modifications *modifications.ModificationOverlay

	Entities EntitySnapshot
	Zones    ZoneSnapshot
	Fort     FortSnapshot
	World    WorldSnapshot

	// Alerts is the rolling buffer of DF announcements (cancellations,
	// ambushes, migrant arrivals, etc.). Populator.OnAnnouncementUpdate
	// adds to it; the deliberator reads it to react to in-game events
	// the way a player would.
	Alerts *AlertStore
}

// EntitySnapshot is a point-in-time copy of entity positions, partitioned by
// type for cheap downstream filtering.
type EntitySnapshot struct {
	All       []protocol.EntityInfo
	Dwarves   []protocol.EntityInfo
	Enemies   []protocol.EntityInfo
	Animals   []protocol.EntityInfo
	UpdatedAt time.Time
	EventSeq  uint64
}

// ZoneSnapshot is the latest set of declared zones.
type ZoneSnapshot struct {
	All       []protocol.ZoneData
	UpdatedAt time.Time
	EventSeq  uint64
}

// FortSnapshot is the latest fort-level info, if the plugin shipped any.
// Valid is false until at least one ENTITY_UPDATE arrived with a non-nil
// FortInfo payload.
type FortSnapshot struct {
	DaysElapsed   uint32
	CreatedWealth uint64
	Season        uint8
	Year          uint32
	UpdatedAt     time.Time
	Valid         bool
}

// WorldSnapshot is the plugin's most recently reported save identity
// (protocol.WorldIdentity), plus Switches: a count of how many times a
// DIFFERENT identity has been observed since this Go process started. A
// model reading the dashboard sees Switches change between two calls the
// same way it already watches the "events=" tick counter — the persistent,
// always-visible signal that a save-swap happened underneath it, so it
// never mistakes the previous world's entity roster for the current one
// (the live incident this closes: stale census across a save-swap). Known
// is false until the plugin ever reports an identity at all (an older peer,
// or before the first ENTITY_UPDATE arrives).
type WorldSnapshot struct {
	SaveDir  string
	ID1, ID2 uint32
	Known    bool
	Switches int
}

// Changed reports whether other is a genuinely different world identity
// than w (both must be Known — an unknown snapshot never "changes" into
// anything, it's simply the absence of data).
func (w WorldSnapshot) Changed(other WorldSnapshot) bool {
	if !w.Known || !other.Known {
		return false
	}
	return w.SaveDir != other.SaveDir || w.ID1 != other.ID1 || w.ID2 != other.ID2
}

// PlanReader is the read side of the plan package. The WorldModel exposes
// pending and active plan nodes through this interface so other consumers
// don't need to import the plan package directly. It is intentionally minimal
// for now; the plan package will satisfy and extend it later.
type PlanReader interface {
	PendingNodes() []PlanNodeView
	ActiveNodes() []PlanNodeView
}

// PlanNodeView is the slice of a plan node that consumers of WorldModel care
// about. The plan package will define a richer node type internally.
type PlanNodeView struct {
	ID         string
	Action     string
	TargetTile *Coord
	Priority   int
	Status     string
}

// New builds a WorldModel that wraps the given observation overlays. Any of
// the overlay arguments may be nil during early startup; the populator will
// not deference them, and consumers should check before reading.
func New(
	topo *topology.TopologyOverlay,
	hzd *hazards.HazardManager,
	mods *modifications.ModificationOverlay,
	logger *logging.Logger,
) *WorldModel {
	wm := &WorldModel{
		Predicted: newPredictedState(),
		logger:    logger,
		bornAt:    time.Now(),
	}
	wm.Observed.Topology = topo
	wm.Observed.Hazards = hzd
	wm.Observed.Modifications = mods
	wm.Observed.Alerts = NewAlertStore(0)
	return wm
}

// SetOverlays installs (or replaces) the observation overlays. Callers use
// this to wire WorldModel after the FULL_STATE handshake completes, since
// those overlays are constructed lazily on first FULL_STATE receipt.
func (w *WorldModel) SetOverlays(
	topo *topology.TopologyOverlay,
	hzd *hazards.HazardManager,
	mods *modifications.ModificationOverlay,
) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.Observed.Topology = topo
	w.Observed.Hazards = hzd
	w.Observed.Modifications = mods
}

// SetPlanReader installs the plan package's view interface. Safe to call once
// during startup or whenever the plan package wires in.
func (w *WorldModel) SetPlanReader(p PlanReader) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.plan = p
}

// PlannedNodes returns pending+active plan nodes through the registered
// PlanReader. Returns nil if no plan reader has been registered yet.
func (w *WorldModel) PlannedNodes() (pending, active []PlanNodeView) {
	w.mu.RLock()
	p := w.plan
	w.mu.RUnlock()
	if p == nil {
		return nil, nil
	}
	return p.PendingNodes(), p.ActiveNodes()
}

// Tick returns the current heartbeat tick. The tick advances on every
// perception event the populator processes (FullState, TileUpdate,
// EntityUpdate). It is the canonical clock for prediction deadlines.
func (w *WorldModel) Tick() uint64 { return w.tick.Load() }

// AdvanceTick increments the tick by one and returns the new value. The
// populator calls this on every perception event; external callers should
// not.
func (w *WorldModel) AdvanceTick() uint64 { return w.tick.Add(1) }

// NextEventID returns a monotonic event ID. Used by snapshots and divergence
// events so the reconciler can order them.
func (w *WorldModel) NextEventID() uint64 { return w.eventSeq.Add(1) }

// LastUpdate returns the time of the most recent perception event the
// populator processed.
func (w *WorldModel) LastUpdate() time.Time {
	ns := w.lastUpdate.Load()
	if ns == 0 {
		return time.Time{}
	}
	return time.Unix(0, ns)
}

// markUpdated stamps the last-update timestamp. Internal helper for the
// populator.
func (w *WorldModel) markUpdated() {
	w.lastUpdate.Store(time.Now().UnixNano())
}

// Snapshot is an immutable point-in-time view suitable for handing to a
// deliberator (LLM) call without worrying about concurrent writes invalidating
// the data mid-render.
type Snapshot struct {
	Tick     uint64
	EventSeq uint64
	TakenAt  time.Time
	// LastUpdate is the time of the most recent perception event the
	// populator ingested (zero until the first event). TakenAt-LastUpdate
	// is the honest age of everything else in this snapshot.
	LastUpdate  time.Time
	Entities    EntitySnapshot
	Zones       ZoneSnapshot
	Fort        FortSnapshot
	World       WorldSnapshot
	Predictions PredictedSummary

	// ActiveAlerts is the current set of un-dismissed DF announcements,
	// newest first, capped at SnapshotMaxAlerts.
	ActiveAlerts []Alert
	// ActiveAlertCount is the TRUE undismissed count, uncapped — use this
	// for the dashboard, not len(ActiveAlerts), which pins at
	// SnapshotMaxAlerts and stops carrying signal once alerts accumulate
	// past it (see docs/archive/2026-07-12-token-scaling-research.md).
	ActiveAlertCount int
}

// SnapshotMaxAlerts is the cap on alerts surfaced in a single snapshot.
// Beyond this, the LLM should rely on dismissal + the rolling buffer.
const SnapshotMaxAlerts = 25

// PredictedSummary is the snapshot's view of the predicted layer. It carries
// only the counts plus the outstanding predictions; confirmed/diverged are
// not included because the reconciler is the consumer for those.
type PredictedSummary struct {
	OutstandingCount int
	ConfirmedCount   int
	DivergedCount    int
	Outstanding      []TilePrediction
}

// Snapshot produces an immutable view of the world. The overlays
// (Topology, Hazards, Modifications) are NOT copied because they are large
// and have their own concurrency control; callers reading them through the
// snapshot should treat them as live references.
func (w *WorldModel) Snapshot() Snapshot {
	w.mu.RLock()
	defer w.mu.RUnlock()

	var alerts []Alert
	var activeCount int
	if w.Observed.Alerts != nil {
		alerts = w.Observed.Alerts.Active(SnapshotMaxAlerts)
		activeCount, _, _ = w.Observed.Alerts.Counts()
	}

	return Snapshot{
		Tick:             w.tick.Load(),
		EventSeq:         w.eventSeq.Load(),
		TakenAt:          time.Now(),
		LastUpdate:       w.LastUpdate(),
		Entities:         w.Observed.Entities,
		Zones:            w.Observed.Zones,
		Fort:             w.Observed.Fort,
		World:            w.Observed.World,
		Predictions:      w.Predicted.summary(),
		ActiveAlerts:     alerts,
		ActiveAlertCount: activeCount,
	}
}
