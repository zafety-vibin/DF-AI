package modifications

import (
	"sync"
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// Detector tracks tile modifications by comparing against baseline state
// Stores first-seen tile type per coordinate to detect player/AI changes
type Detector struct {
	mu              sync.RWMutex
	baseline        map[Coordinate]uint16 // First-seen tile type per coordinate
	overlay         *ModificationOverlay
	modificationsMu sync.Mutex // Separate lock for modification processing
}

// NewDetector creates a new modification detector
func NewDetector(overlay *ModificationOverlay) *Detector {
	return &Detector{
		baseline: make(map[Coordinate]uint16),
		overlay:  overlay,
	}
}

// InferModificationType determines the type of modification based on tile type changes
// Uses heuristics to classify common Dwarf Fortress modifications
func InferModificationType(oldType, newType uint16) ModificationType {
	// This is a simplified heuristic - actual DF tile types are complex
	// We use ranges to classify tile types:
	// 0-349: Natural walls (stone, soil, etc.)
	// 350-399: Natural floors
	// 400-449: Constructed walls
	// 450-499: Constructed floors
	// 500-549: Ramps
	// 550-599: Open space

	oldIsWall := oldType < 350
	oldIsFloor := oldType >= 350 && oldType < 400
	oldIsBuiltWall := oldType >= 400 && oldType < 450
	oldIsBuiltFloor := oldType >= 450 && oldType < 500
	oldIsSpace := oldType >= 550

	newIsWall := newType < 350
	newIsFloor := newType >= 350 && newType < 400
	newIsBuiltWall := newType >= 400 && newType < 450
	newIsBuiltFloor := newType >= 450 && newType < 500
	newIsRamp := newType >= 500 && newType < 550
	newIsSpace := newType >= 550

	// Detect digging: Wall/Rock -> Floor
	if oldIsWall && newIsFloor {
		return ModificationDug
	}

	// Detect wall construction: Floor -> Wall
	if oldIsFloor && (newIsBuiltWall || newIsWall) {
		return ModificationBuiltWall
	}

	// Detect floor construction: Space -> Floor
	if oldIsSpace && (newIsBuiltFloor || newIsFloor) {
		return ModificationBuiltFloor
	}

	// Detect ramp construction: Floor -> Ramp
	if oldIsFloor && newIsRamp {
		return ModificationBuiltRamp
	}

	// Detect channeling: Floor -> Space
	if oldIsFloor && newIsSpace {
		return ModificationChanneled
	}

	// Detect destruction: Built -> anything else
	if (oldIsBuiltWall || oldIsBuiltFloor) && !(newIsBuiltWall || newIsBuiltFloor) {
		return ModificationDestroyed
	}

	// Additional heuristics based on tile type changes
	// Smoothing and engraving are typically small increments within same material
	if oldType > 0 && newType > 0 {
		diff := int(newType) - int(oldType)
		// If same category but higher type number, might be smoothing/engraving
		if diff > 0 && diff < 10 {
			if oldIsWall && newIsWall {
				return ModificationSmoothed
			}
			if oldIsFloor && newIsFloor {
				// Check if it's a significant upgrade (engraving)
				if diff > 5 {
					return ModificationEngraved
				}
				return ModificationSmoothed
			}
		}
	}

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
		baselineType, hasBaseline := d.baseline[coord]
		if !hasBaseline {
			// First time seeing this tile - record as baseline
			d.baseline[coord] = tile.TileType
			d.mu.Unlock()
			continue
		}
		d.mu.Unlock()

		// Check if tile type has changed from baseline
		if tile.TileType == baselineType {
			continue // No modification
		}

		// Infer modification type
		modType := InferModificationType(baselineType, tile.TileType)
		if modType == ModificationUnknown {
			continue // Ignore unknown modifications
		}

		// Check if we already tracked this modification
		existingInfo, exists := d.overlay.Get(coord)
		if exists {
			// Update existing modification if tile changed again
			if existingInfo.NewTileType != tile.TileType {
				updatedInfo := existingInfo
				updatedInfo.NewTileType = tile.TileType
				updatedInfo.LastVerified = now
				// Re-infer type based on original baseline and new tile type
				updatedInfo.Type = InferModificationType(existingInfo.OldTileType, tile.TileType)
				d.overlay.Add(coord, updatedInfo)
			}
			continue
		}

		// Record new modification
		modInfo := ModificationInfo{
			Type:         modType,
			OldTileType:  baselineType,
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
	d.baseline = make(map[Coordinate]uint16)

	// Record all tiles as baseline
	for _, tile := range tiles {
		coord := Coordinate{X: tile.X, Y: tile.Y, Z: tile.Z}
		d.baseline[coord] = tile.TileType
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
