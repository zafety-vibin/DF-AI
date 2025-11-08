package context

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// ExtractChamberFeatures converts Chamber structs to ChamberFeature JSON objects
// Includes natural language descriptions and formatted coordinates
func ExtractChamberFeatures(chambers []modifications.Chamber) []ChamberFeature {
	if len(chambers) == 0 {
		return []ChamberFeature{}
	}

	features := make([]ChamberFeature, 0, len(chambers))
	for _, chamber := range chambers {
		// Generate natural language description
		description := modifications.GenerateChamberDescription(chamber)

		// Format bounds
		boundsMin := fmt.Sprintf("(%d,%d,%d)",
			chamber.Bounds.XMin, chamber.Bounds.YMin, chamber.Bounds.ZMin)
		boundsMax := fmt.Sprintf("(%d,%d,%d)",
			chamber.Bounds.XMax, chamber.Bounds.YMax, chamber.Bounds.ZMax)

		// Calculate dimensions
		width := chamber.Bounds.XMax - chamber.Bounds.XMin + 1
		height := chamber.Bounds.YMax - chamber.Bounds.YMin + 1
		depth := chamber.Bounds.ZMax - chamber.Bounds.ZMin + 1
		dimensions := fmt.Sprintf("%dx%dx%d", width, height, depth)

		feature := ChamberFeature{
			ID:          chamber.ID,
			Description: description,
			BoundsMin:   boundsMin,
			BoundsMax:   boundsMax,
			Dimensions:  dimensions,
			TileCount:   chamber.TileCount,
		}

		features = append(features, feature)
	}

	return features
}

// FormatHazardsAsJSON filters hazards to a region and outputs coordinates
// Returns list of hazard positions with severity/flags
func FormatHazardsAsJSON(
	overlay *hazards.HazardOverlay,
	region modifications.Region,
	limit uint32,
) []HazardPosition {
	if overlay == nil {
		return []HazardPosition{}
	}

	// Get hazards in region
	hazardRegion := hazards.Region{
		XMin: region.XMin, XMax: region.XMax,
		YMin: region.YMin, YMax: region.YMax,
		ZMin: region.ZMin, ZMax: region.ZMax,
	}

	coords := overlay.GetInRegion(hazardRegion)

	// Apply limit
	if uint32(len(coords)) > limit {
		coords = coords[:limit]
	}

	// Convert to HazardPosition format
	positions := make([]HazardPosition, 0, len(coords))
	for _, coord := range coords {
		info, exists := overlay.Get(coord.X, coord.Y, coord.Z)
		if !exists {
			continue
		}

		position := HazardPosition{
			X:        coord.X,
			Y:        coord.Y,
			Z:        coord.Z,
			Severity: info.Severity,
			Flags:    info.Flags,
		}
		positions = append(positions, position)
	}

	return positions
}

// FormatDwarfsAsJSON converts entity positions to JSON-serializable format
// Filters entities to only include dwarves and outputs as coordinate vectors
func FormatDwarfsAsJSON(entities EntityInfoSlice) []EntityPosition {
	if len(entities) == 0 {
		return []EntityPosition{}
	}

	positions := make([]EntityPosition, 0)
	for _, entity := range entities {
		// Only include dwarves
		if entity.Type != protocol.EntityTypeDwarf {
			continue
		}

		position := EntityPosition{
			ID:   entity.ID,
			X:    entity.X,
			Y:    entity.Y,
			Z:    entity.Z,
			Type: "dwarf",
		}
		positions = append(positions, position)
	}

	return positions
}

// FormatEnemiesAsJSON converts enemy entities to JSON-serializable format
func FormatEnemiesAsJSON(entities EntityInfoSlice) []EntityPosition {
	if len(entities) == 0 {
		return []EntityPosition{}
	}

	positions := make([]EntityPosition, 0)
	for _, entity := range entities {
		// Only include enemies
		if entity.Type != protocol.EntityTypeEnemy {
			continue
		}

		position := EntityPosition{
			ID:   entity.ID,
			X:    entity.X,
			Y:    entity.Y,
			Z:    entity.Z,
			Type: "enemy",
		}
		positions = append(positions, position)
	}

	return positions
}

// FormatAllEntitiesAsJSON converts all entities to JSON-serializable format
func FormatAllEntitiesAsJSON(entities EntityInfoSlice) []EntityPosition {
	if len(entities) == 0 {
		return []EntityPosition{}
	}

	positions := make([]EntityPosition, 0, len(entities))
	for _, entity := range entities {
		entityType := "unknown"
		switch entity.Type {
		case protocol.EntityTypeDwarf:
			entityType = "dwarf"
		case protocol.EntityTypeEnemy:
			entityType = "enemy"
		case protocol.EntityTypeAnimal:
			entityType = "animal"
		case protocol.EntityTypeOther:
			entityType = "other"
		}

		position := EntityPosition{
			ID:   entity.ID,
			X:    entity.X,
			Y:    entity.Y,
			Z:    entity.Z,
			Type: entityType,
		}
		positions = append(positions, position)
	}

	return positions
}
