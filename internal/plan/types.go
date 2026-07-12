package plan

import "time"

// Status is the lifecycle of a PlanNode.
type Status uint8

const (
	// StatusPending means the node has been added but at least one of its
	// dependencies is still un-done.
	StatusPending Status = iota

	// StatusReady means all dependencies are Done. The executor will pick
	// the node up on its next pass.
	StatusReady

	// StatusActive means the executor has dispatched the action to the
	// plugin (ACK received, predictions written). The reconciler is now
	// watching the predictions.
	StatusActive

	// StatusDone means the reconciler confirmed enough predictions to
	// consider the node satisfied. Successor nodes may now become Ready.
	StatusDone

	// StatusFailed means the action was rejected by the plugin OR enough
	// predictions diverged that we can't reasonably claim success. The
	// node's Result holds the failure reason.
	StatusFailed
)

func (s Status) String() string {
	switch s {
	case StatusPending:
		return "pending"
	case StatusReady:
		return "ready"
	case StatusActive:
		return "active"
	case StatusDone:
		return "done"
	case StatusFailed:
		return "failed"
	default:
		return "unknown"
	}
}

// Region is a 3D rectangular range, inclusive on both ends.
type Region struct {
	X1, Y1, Z1 int16
	X2, Y2, Z2 int16
}

// TileCount returns the number of tiles this region covers, treating Z as a
// range. For single-Z regions Z1 == Z2.
func (r Region) TileCount() int {
	dx := int(r.X2-r.X1) + 1
	dy := int(r.Y2-r.Y1) + 1
	dz := int(r.Z2-r.Z1) + 1
	if dx < 0 || dy < 0 || dz < 0 {
		return 0
	}
	return dx * dy * dz
}

// IsVerticalShaft is true when the region has a non-trivial Z-range and a
// single (X, Y) column. Used by the executor to fan out per-Z dig commands.
func (r Region) IsVerticalShaft() bool {
	return r.Z1 != r.Z2 && r.X1 == r.X2 && r.Y1 == r.Y2
}

// Action is a single concrete operation a PlanNode commits to. The Type and
// DigType strings match the LLM parser's output so the deliberator can wrap
// llm.CommandSpec into Action with no extra translation.
type Action struct {
	Type            string // "dig", "build", "chop", "gather", "zone", "stockpile", "smooth", "unsuspend", "order", "wait", "dismiss"
	DigType         string // for dig: "default", "stairs", "channel", "ramp", "upstair", "downstair"
	BuildType       uint8  // for build: protocol BuildType byte
	ZoneType        uint8  // for zone: protocol ZoneType byte
	OrderType       uint8  // for order: protocol OrderType byte
	Quantity        uint16 // for order: number to produce
	StockpileGroups uint32 // for stockpile: GroupMask bitfield (protocol.StockpileGroup*)
	SmoothType      uint8  // for smooth: protocol.SmoothTypeSmooth or SmoothTypeEngrave
	AlertID         uint32 // for dismiss: the alert ID; 0 = dismiss all active alerts
	Region          Region // for build/unsuspend, X1=X2,Y1=Y2,Z1=Z2 (single tile)
	Description     string // human-readable summary for prompts and logs
}

// Node is a single committed step in the plan DAG.
type Node struct {
	ID           string
	Action       Action
	Dependencies []string

	Status Status

	CreatedAt time.Time
	StartedAt time.Time // when executor marked Active
	DoneAt    time.Time // when status transitioned to Done or Failed

	// LastProgressAt is the most recent time the reconciler observed
	// progress on this node — i.e., the last time a TilePrediction owned
	// by this node transitioned to Confirmed. Initially equal to
	// StartedAt to give workers a grace period to walk to the site. Used
	// for stall detection.
	LastProgressAt time.Time

	// StallNotedAt is when the reconciler first emitted a stall event for
	// this node since the last progress. Zero when not currently stalled.
	// Used to dedupe stall events so the divergence buffer doesn't fill
	// with the same node every scan.
	StallNotedAt time.Time

	// Result is a short string summarising the outcome. ACK error message
	// when Status == StatusFailed; "ok (N predictions)" or similar when
	// Status == StatusActive or StatusDone.
	Result string

	// PredictionCount is how many TilePredictions this node owns. The
	// reconciler updates Confirmed/Diverged as observations arrive.
	PredictionCount      int
	PredictionsConfirmed int
	PredictionsDiverged  int

	// CommittingTurn is the deliberator turn number that produced this
	// node, useful for tracing dataset rows back to a specific decision.
	CommittingTurn uint64
}

// IsTerminal returns true when no more state transitions are expected.
func (n *Node) IsTerminal() bool {
	return n.Status == StatusDone || n.Status == StatusFailed
}

// Stats summarises a DAG by status counts. Cheap to compute, suitable for
// embedding in the deliberator prompt.
type Stats struct {
	Pending int
	Ready   int
	Active  int
	Done    int
	Failed  int
}
