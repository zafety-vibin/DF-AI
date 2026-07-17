package modifications

import (
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// baselineTile is the first shape-bearing observation of a tile: the flag
// byte drives classification; the tiletype is kept only for the
// ModificationInfo Old/NewTileType audit trail.
type baselineTile struct {
	TileType uint16
	Flags    uint8
}

// Detector tracks tile modifications by comparing against baseline state
// (first shape-bearing observation per coordinate) to detect player/AI changes
type Detector struct {
	mu              sync.RWMutex
	baseline        map[Coordinate]baselineTile
	overlay         *ModificationOverlay
	modificationsMu sync.Mutex // Separate lock for modification processing
}

// NewDetector creates a new modification detector
func NewDetector(overlay *ModificationOverlay) *Detector {
	return &Detector{
		baseline: make(map[Coordinate]baselineTile),
		overlay:  overlay,
	}
}

// shapeMask isolates the mutually exclusive shape bits of a tile flag byte.
// The hidden/discovered/designated/liquid bits churn for reasons unrelated
// to player work (fog-of-war reveals, water depth) and must never enter
// classification.
const shapeMask = protocol.FlagWall | protocol.FlagFloor | protocol.FlagVoid

// InferModificationTypeFromFlags classifies a tile change by its shape-bit
// transition (wall/floor/void). The plugin computes these bits from
// df::tiletype_shape, which is authoritative — unlike raw tiletype values,
// where ambient simulation churn (grass dark<->light cycles, murky pools
// drying/refilling) changes the tiletype without changing the shape. Any
// same-shape transition classifies Unknown, which also means smoothing and
// engraving are deliberately NOT inferred from deltas: a smoothed wall is
// still a wall, indistinguishable here from ambient tiletype churn.
func InferModificationTypeFromFlags(oldFlags, newFlags uint8) ModificationType {
	oldShape := oldFlags & shapeMask
	newShape := newFlags & shapeMask
	switch {
	case oldShape == protocol.FlagWall && newShape == protocol.FlagFloor:
		return ModificationDug
	case oldShape == protocol.FlagWall && newShape == protocol.FlagVoid:
		return ModificationChanneled
	case oldShape == protocol.FlagFloor && newShape == protocol.FlagVoid:
		return ModificationChanneled
	case oldShape == protocol.FlagFloor && newShape == protocol.FlagWall:
		return ModificationBuiltWall
	case oldShape == protocol.FlagVoid && newShape == protocol.FlagWall:
		return ModificationBuiltWall
	case oldShape == protocol.FlagVoid && newShape == protocol.FlagFloor:
		return ModificationBuiltFloor
	}
	// Same shape, or one side has no shape bit at all (unloaded/unallocated
	// block placeholder) — no basis for attributing a player modification.
	return ModificationUnknown
}

// DetectModifications processes tile updates and detects modifications
// Compares against baseline (first-seen state) to identify player/AI changes
func (d *Detector) DetectModifications(tiles []protocol.TileState) int {
	d.modificationsMu.Lock()
	defer d.modificationsMu.Unlock()

	detectedCount := 0
	now := time.Now()

	for _, tile := range tiles {
		coord := Coordinate{X: tile.X, Y: tile.Y, Z: tile.Z}

		// Get or set baseline
		d.mu.Lock()
		base, hasBaseline := d.baseline[coord]
		wasShapeless := hasBaseline && base.Flags&shapeMask == 0
		if !hasBaseline || wasShapeless {
			// The first shape-bearing observation becomes the real
			// baseline going forward, whether this is a true first
			// sighting or the coordinate's earlier sighting(s) were all
			// shapeless (FLAG_HIDDEN only, unallocated-block placeholder).
			d.baseline[coord] = baselineTile{TileType: tile.TileType, Flags: tile.Flags}
		}
		d.mu.Unlock()

		if !hasBaseline {
			// True first sighting — nothing to compare against yet.
			continue
		}

		// DFHack's MapCache can observe a block's tiles but never forces DF
		// to generate one that doesn't exist yet (see tile_extractor.cpp
		// compute_tile_flags): a deep, never-visited block can sit as a
		// shapeless HIDDEN placeholder right through FULL_STATE, and only
		// materializes once a dig job actually touches it — at which point
		// the very first shape-bearing delta the plugin ever sends for that
		// tile can already BE the dug floor, with no intervening
		// wall-shaped observation on the wire. A never-materialized block
		// is virtually always solid rock, so treat a shapeless baseline as
		// an implied wall for classification; without this, a fort's first
		// dig into a fresh z-level is silently swallowed as "just a
		// rebaseline" and nothing is ever recorded.
		baseFlags := base.Flags
		if wasShapeless {
			baseFlags = protocol.FlagWall
		}

		// Infer modification type from the shape-bit transition
		modType := InferModificationTypeFromFlags(baseFlags, tile.Flags)
		if modType == ModificationUnknown {
			continue // Same shape (or unclassifiable) — not player work
		}

		// Check if we already tracked this modification
		existingInfo, exists := d.overlay.Get(coord)
		if exists {
			// Update existing modification if tile changed again
			if existingInfo.NewTileType != tile.TileType {
				updatedInfo := existingInfo
				updatedInfo.NewTileType = tile.TileType
				updatedInfo.LastVerified = now
				// Re-infer type against the original baseline shape
				updatedInfo.Type = modType
				d.overlay.Add(coord, updatedInfo)
			}
			continue
		}

		// Record new modification
		modInfo := ModificationInfo{
			Type:         modType,
			OldTileType:  base.TileType,
			NewTileType:  tile.TileType,
			DetectedAt:   now,
			CommandID:    0, // Will be linked later if from a command
			LastVerified: now,
		}

		if err := d.overlay.Add(coord, modInfo); err != nil {
			// Ignore out-of-bounds errors (defensive)
			continue
		}

		detectedCount++
	}

	return detectedCount
}

// InitializeBaseline sets the baseline state from initial full state
// Should be called once when FULL_STATE is received
func (d *Detector) InitializeBaseline(tiles []protocol.TileState) {
	d.mu.Lock()
	defer d.mu.Unlock()

	// Clear existing baseline
	d.baseline = make(map[Coordinate]baselineTile, len(tiles))

	// Record all tiles as baseline
	for _, tile := range tiles {
		coord := Coordinate{X: tile.X, Y: tile.Y, Z: tile.Z}
		d.baseline[coord] = baselineTile{TileType: tile.TileType, Flags: tile.Flags}
	}
}

// GetBaselineCount returns the number of tiles in the baseline
func (d *Detector) GetBaselineCount() int {
	d.mu.RLock()
	defer d.mu.RUnlock()
	return len(d.baseline)
}

// LinkCommandToModifications links modifications in a region to a specific command
// Returns the number of modifications linked
func (d *Detector) LinkCommandToModifications(commandID uint32, region Region, timestamp time.Time) int {
	return d.overlay.LinkCommandToModifications(commandID, region, timestamp)
}
