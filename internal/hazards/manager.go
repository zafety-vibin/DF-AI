package hazards

import (
	"time"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// HazardManager coordinates all hazard overlays
type HazardManager struct {
	aquifer *AquiferOverlay
	water   *WaterOverlay
	lava    *LavaOverlay
	caverns *CavernOverlay
	enemies *EnemyOverlay
	dwarves *DwarfOverlay

	mapBounds Bounds
}

// NewHazardManager creates a new hazard manager with all overlay types initialized
func NewHazardManager(width, height, depth uint16) *HazardManager {
	bounds := Bounds{
		Width:  width,
		Height: height,
		Depth:  depth,
	}

	return &HazardManager{
		aquifer:   NewAquiferOverlay(bounds),
		water:     NewWaterOverlay(bounds),
		lava:      NewLavaOverlay(bounds),
		caverns:   NewCavernOverlay(bounds),
		enemies:   NewEnemyOverlay(bounds),
		dwarves:   NewDwarfOverlay(bounds),
		mapBounds: bounds,
	}
}

// BuildFromTiles builds all environmental hazard overlays from FULL_STATE tile data
func (m *HazardManager) BuildFromTiles(tiles []protocol.TileState) {
	// Clear all overlays
	m.aquifer.Clear()
	m.water.Clear()
	m.lava.Clear()
	m.caverns.Clear()

	// Scan all tiles and detect hazards
	for _, tile := range tiles {
		coord := Coordinate{X: tile.X, Y: tile.Y, Z: tile.Z}

		// Detect aquifer
		if IsAquifer(tile.TileType, tile.Flags) {
			info := HazardInfo{
				Severity:   255, // Max severity for aquifer (always dangerous)
				Flags:      tile.Flags,
				DetectedAt: time.Now(),
			}
			m.aquifer.Add(coord, info) // Ignore error - bounds already validated
		}

		// Detect water
		if isWater, depth, flowFlags := IsWater(tile.TileType); isWater {
			info := HazardInfo{
				Severity:   depth,
				Flags:      flowFlags,
				DetectedAt: time.Now(),
			}
			m.water.Add(coord, info)
		}

		// Detect lava
		if isLava, depth, flowFlags := IsLava(tile.TileType); isLava {
			info := HazardInfo{
				Severity:   depth,
				Flags:      flowFlags,
				DetectedAt: time.Now(),
			}
			m.lava.Add(coord, info)
		}

		// Detect caverns (only in cavern layer Z-levels)
		// Caverns exist between surface (~10-11 Z below) and magma sea
		// Typical range: Z=10 (above magma) to Z=90 (below surface)
		if tile.Z >= 10 && tile.Z < 90 && IsCavern(tile.TileType) {
			info := HazardInfo{
				Severity:   1, // Default cavern layer (could be refined with Z-level info)
				Flags:      0,
				DetectedAt: time.Now(),
			}
			m.caverns.Add(coord, info)
		}
	}
}

// UpdateFromTiles updates hazard overlays incrementally from TILE_UPDATE message
func (m *HazardManager) UpdateFromTiles(tiles []protocol.TileState) {
	for _, tile := range tiles {
		coord := Coordinate{X: tile.X, Y: tile.Y, Z: tile.Z}

		// Update aquifer overlay
		isAquifer := IsAquifer(tile.TileType, tile.Flags)
		wasAquifer := m.aquifer.Contains(tile.X, tile.Y, tile.Z)

		if isAquifer && !wasAquifer {
			// New aquifer detected
			info := HazardInfo{
				Severity:   255,
				Flags:      tile.Flags,
				DetectedAt: time.Now(),
			}
			m.aquifer.Add(coord, info)
		} else if !isAquifer && wasAquifer {
			// Aquifer removed (drained/sealed)
			m.aquifer.Remove(coord)
		}

		// Update water overlay
		isWater, depth, flowFlags := IsWater(tile.TileType)
		wasWater := m.water.Contains(tile.X, tile.Y, tile.Z)

		if isWater && !wasWater {
			// New water detected
			info := HazardInfo{
				Severity:   depth,
				Flags:      flowFlags,
				DetectedAt: time.Now(),
			}
			m.water.Add(coord, info)
		} else if isWater && wasWater {
			// Water properties changed (depth/flow)
			info := HazardInfo{
				Severity:   depth,
				Flags:      flowFlags,
				DetectedAt: time.Now(),
			}
			m.water.Add(coord, info) // Update
		} else if !isWater && wasWater {
			// Water removed (drained/evaporated)
			m.water.Remove(coord)
		}

		// Update lava overlay
		isLava, lavaDepth, lavaFlowFlags := IsLava(tile.TileType)
		wasLava := m.lava.Contains(tile.X, tile.Y, tile.Z)

		if isLava && !wasLava {
			// New lava detected
			info := HazardInfo{
				Severity:   lavaDepth,
				Flags:      lavaFlowFlags,
				DetectedAt: time.Now(),
			}
			m.lava.Add(coord, info)
		} else if isLava && wasLava {
			// Lava properties changed (depth/flow)
			info := HazardInfo{
				Severity:   lavaDepth,
				Flags:      lavaFlowFlags,
				DetectedAt: time.Now(),
			}
			m.lava.Add(coord, info) // Update
		} else if !isLava && wasLava {
			// Lava removed (drained/solidified)
			m.lava.Remove(coord)
		}

		// Update cavern overlay (only in cavern layer Z-levels)
		isCavern := tile.Z >= 10 && tile.Z < 90 && IsCavern(tile.TileType)
		wasCavern := m.caverns.Contains(tile.X, tile.Y, tile.Z)

		if isCavern && !wasCavern {
			// New cavern detected (revealed)
			info := HazardInfo{
				Severity:   1,
				Flags:      0,
				DetectedAt: time.Now(),
			}
			m.caverns.Add(coord, info)
		} else if !isCavern && wasCavern {
			// Cavern filled/constructed over
			m.caverns.Remove(coord)
		}
	}
}

// BuildFromEntities builds entity overlays from ENTITY_UPDATE message
func (m *HazardManager) BuildFromEntities(entities []protocol.EntityInfo) {
	// Clear entity overlays
	m.enemies.ClearEntities()
	m.dwarves.ClearEntities()

	// Classify and add entities
	for _, entity := range entities {
		switch entity.Type {
		case protocol.EntityTypeDwarf:
			m.dwarves.AddEntity(entity)
		case protocol.EntityTypeEnemy:
			m.enemies.AddEntity(entity)
			// Ignore animals and other types for now
		}
	}
}

// GetOverlay returns a specific overlay by type name
func (m *HazardManager) GetOverlay(hazardType string) *HazardOverlay {
	switch hazardType {
	case "aquifer":
		return m.aquifer.HazardOverlay
	case "water":
		return m.water.HazardOverlay
	case "lava":
		return m.lava.HazardOverlay
	case "caverns":
		return m.caverns.HazardOverlay
	case "enemies":
		return m.enemies.HazardOverlay
	case "dwarves":
		return m.dwarves.HazardOverlay
	default:
		return nil
	}
}

// GetAllCounts returns hazard counts for all overlay types
func (m *HazardManager) GetAllCounts() map[string]uint32 {
	return map[string]uint32{
		"aquifer": m.aquifer.GetCount(),
		"water":   m.water.GetCount(),
		"lava":    m.lava.GetCount(),
		"caverns": m.caverns.GetCount(),
		"enemies": m.enemies.GetCount(),
		"dwarves": m.dwarves.GetCount(),
	}
}

// GetMemoryUsage estimates total memory usage across all overlays (in bytes)
func (m *HazardManager) GetMemoryUsage() uint64 {
	// Estimate: 32 bytes per hazard (Coordinate + HazardInfo + map overhead)
	// Entities use slightly more (EntityHazardInfo) but use same estimate
	totalHazards := m.aquifer.GetCount() + m.water.GetCount() + m.lava.GetCount() + m.caverns.GetCount() +
		m.enemies.GetCount() + m.dwarves.GetCount()
	return uint64(totalHazards) * 32
}
