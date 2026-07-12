package worldmodel

import "time"

// Material is a coarse classification of digging difficulty. The plan package
// will set this on each TilePrediction at commit time, derived from the
// observed tiletype at the predicted coordinate.
//
// The categories are intentionally coarse: per-tile fidelity isn't useful
// while the rest of the work model has 2-3x safety factors anyway. Soil
// covers loam/clay/sand/silt; Stone covers most rock; Obsidian covers
// hard/expensive materials (obsidian, hematite, magnetite, native metals).
type Material uint8

const (
	MaterialUnknown  Material = 0
	MaterialSoil     Material = 1
	MaterialStone    Material = 2
	MaterialObsidian Material = 3
)

func (m Material) String() string {
	switch m {
	case MaterialSoil:
		return "soil"
	case MaterialStone:
		return "stone"
	case MaterialObsidian:
		return "obsidian"
	default:
		return "unknown"
	}
}

// WorkConstants are the tunable parameters of the deadline formula. The
// deliberator and curriculum can swap constants to model harder embarks
// (slower per-tile times, fewer parallel miners) or training scenarios
// (compressed times for faster iteration).
type WorkConstants struct {
	// FixedOverheadSeconds covers pathfinding, designation propagation, and
	// general approach time before the first tile gets dug.
	FixedOverheadSeconds float64

	// SecondsPerTile is the per-tile real-seconds estimate per Material.
	// Missing entries fall back to SecondsPerTile[MaterialUnknown].
	SecondsPerTile map[Material]float64

	// SafetyFactor multiplies the final estimate. Absorbs noise from labor
	// contention, hauling distractions, and dwarf mood swings. 2.5 is a
	// reasonable empirical default; raise to 3-4 in early curriculum runs.
	SafetyFactor float64

	// MaxParallelism caps the parallelism multiplier. DF dwarves can step on
	// each other when too many converge on a small face; six is a soft
	// physical limit for most rooms.
	MaxParallelism int
}

// DefaultWorkConstants returns the package's stock numbers. Treat the
// returned value as immutable; mutate a copy if you need a variant.
func DefaultWorkConstants() WorkConstants {
	return WorkConstants{
		FixedOverheadSeconds: 30,
		SecondsPerTile: map[Material]float64{
			MaterialUnknown:  5,
			MaterialSoil:     2,
			MaterialStone:    5,
			MaterialObsidian: 30,
		},
		SafetyFactor:   2.5,
		MaxParallelism: 6,
	}
}

// WorkEstimate describes the work cost of a single planned action. The
// executor stamps one of these onto each TilePrediction at commit time and
// uses Estimate() to compute DeadlineAt.
type WorkEstimate struct {
	// TileCount is the number of tiles the action will affect. For a single-
	// tile action (one stair step, one floor build), this is 1.
	TileCount int

	// Material is the dominant material in the affected region. If a region
	// straddles soil and stone, pick the slower one.
	Material Material

	// Parallelism is the expected number of dwarves working concurrently. The
	// plan package computes this from idle miners + accessible-face geometry.
	// 0 collapses to 1.
	Parallelism int
}

// Estimate returns the predicted real-time duration for this work using the
// supplied constants. Always non-negative; degenerate inputs (TileCount<=0)
// collapse to FixedOverheadSeconds * SafetyFactor.
func (we WorkEstimate) Estimate(c WorkConstants) time.Duration {
	if we.TileCount <= 0 {
		return durationSeconds(c.FixedOverheadSeconds * safe(c.SafetyFactor, 1))
	}

	secondsPerTile := c.SecondsPerTile[we.Material]
	if secondsPerTile == 0 {
		secondsPerTile = c.SecondsPerTile[MaterialUnknown]
		if secondsPerTile == 0 {
			secondsPerTile = 5 // safety net if the map is misconfigured
		}
	}

	par := we.Parallelism
	if par <= 0 {
		par = 1
	}
	if c.MaxParallelism > 0 && par > c.MaxParallelism {
		par = c.MaxParallelism
	}

	workSec := float64(we.TileCount) / float64(par) * secondsPerTile
	total := (c.FixedOverheadSeconds + workSec) * safe(c.SafetyFactor, 1)
	return durationSeconds(total)
}

// EstimateDuration is the convenience wrapper that uses package defaults.
func (we WorkEstimate) EstimateDuration() time.Duration {
	return we.Estimate(DefaultWorkConstants())
}

// DeadlineFromNow returns a wall-clock deadline at now + EstimateDuration().
func (we WorkEstimate) DeadlineFromNow(now time.Time) time.Time {
	return now.Add(we.EstimateDuration())
}

func durationSeconds(s float64) time.Duration {
	if s <= 0 {
		return 0
	}
	return time.Duration(s * float64(time.Second))
}

func safe(v, fallback float64) float64 {
	if v <= 0 {
		return fallback
	}
	return v
}
