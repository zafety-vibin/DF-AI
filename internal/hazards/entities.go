package hazards

import (
	"github.com/df-ai/orchestrator/internal/protocol"
)

// EntityHazardInfo extends HazardInfo with entity-specific data
type EntityHazardInfo struct {
	HazardInfo
	EntityID uint32 // Unit ID for tracking across updates
	RaceID   uint16 // Creature race for threat assessment
}

// EnemyOverlay tracks hostile creature positions
type EnemyOverlay struct {
	*HazardOverlay
	// entityData maps coordinates to extended entity info
	// Using the base HazardOverlay.hazards for positions, this adds metadata
	entityData map[Coordinate]EntityHazardInfo
}

// NewEnemyOverlay creates a new enemy hazard overlay
func NewEnemyOverlay(bounds Bounds) *EnemyOverlay {
	return &EnemyOverlay{
		HazardOverlay: NewHazardOverlay("enemies", bounds),
		entityData:    make(map[Coordinate]EntityHazardInfo),
	}
}

// AddEntity adds or updates an enemy entity
func (e *EnemyOverlay) AddEntity(entity protocol.EntityInfo) error {
	coord := Coordinate{X: entity.X, Y: entity.Y, Z: entity.Z}

	// Add to base overlay
	info := HazardInfo{
		Severity:   calculateThreatLevel(entity.Subtype), // Placeholder threat calculation
		Flags:      0,
		DetectedAt: e.HazardOverlay.lastUpdate,
	}

	if err := e.HazardOverlay.Add(coord, info); err != nil {
		return err
	}

	// Store extended entity data
	e.HazardOverlay.mu.Lock()
	e.entityData[coord] = EntityHazardInfo{
		HazardInfo: info,
		EntityID:   entity.ID,
		RaceID:     entity.Subtype,
	}
	e.HazardOverlay.mu.Unlock()

	return nil
}

// RemoveEntity removes an enemy entity by coordinate
func (e *EnemyOverlay) RemoveEntity(coord Coordinate) error {
	e.HazardOverlay.mu.Lock()
	delete(e.entityData, coord)
	e.HazardOverlay.mu.Unlock()

	return e.HazardOverlay.Remove(coord)
}

// ClearEntities removes all entities
func (e *EnemyOverlay) ClearEntities() {
	e.HazardOverlay.mu.Lock()
	e.entityData = make(map[Coordinate]EntityHazardInfo)
	e.HazardOverlay.mu.Unlock()

	e.HazardOverlay.Clear()
}

// DwarfOverlay tracks friendly dwarf positions
type DwarfOverlay struct {
	*HazardOverlay
	// entityData maps coordinates to dwarf entity info
	entityData map[Coordinate]EntityHazardInfo
}

// NewDwarfOverlay creates a new dwarf position overlay
func NewDwarfOverlay(bounds Bounds) *DwarfOverlay {
	return &DwarfOverlay{
		HazardOverlay: NewHazardOverlay("dwarves", bounds),
		entityData:    make(map[Coordinate]EntityHazardInfo),
	}
}

// AddEntity adds or updates a dwarf entity
func (d *DwarfOverlay) AddEntity(entity protocol.EntityInfo) error {
	coord := Coordinate{X: entity.X, Y: entity.Y, Z: entity.Z}

	// Add to base overlay
	info := HazardInfo{
		Severity:   1, // Dwarves all equally important
		Flags:      0, // Could store task type in future
		DetectedAt: d.HazardOverlay.lastUpdate,
	}

	if err := d.HazardOverlay.Add(coord, info); err != nil {
		return err
	}

	// Store extended entity data
	d.HazardOverlay.mu.Lock()
	d.entityData[coord] = EntityHazardInfo{
		HazardInfo: info,
		EntityID:   entity.ID,
		RaceID:     entity.Subtype,
	}
	d.HazardOverlay.mu.Unlock()

	return nil
}

// RemoveEntity removes a dwarf entity by coordinate
func (d *DwarfOverlay) RemoveEntity(coord Coordinate) error {
	d.HazardOverlay.mu.Lock()
	delete(d.entityData, coord)
	d.HazardOverlay.mu.Unlock()

	return d.HazardOverlay.Remove(coord)
}

// ClearEntities removes all entities
func (d *DwarfOverlay) ClearEntities() {
	d.HazardOverlay.mu.Lock()
	d.entityData = make(map[Coordinate]EntityHazardInfo)
	d.HazardOverlay.mu.Unlock()

	d.HazardOverlay.Clear()
}

// calculateThreatLevel estimates threat based on creature race
// This is a placeholder - would need DF race data for accurate assessment
func calculateThreatLevel(raceID uint16) uint8 {
	// Placeholder: treat all enemies as equal threat
	return 255
}
