package modifications

import (
	"fmt"
	"sort"
	"time"
)

// Chamber represents a connected group of modifications (e.g., a mined room)
type Chamber struct {
	ID          uint32       // Unique chamber ID
	Bounds      Region       // Bounding box of the chamber
	TileCount   uint32       // Number of tiles in the chamber
	Tiles       []Coordinate // All coordinates in the chamber
	Connections []uint32     // IDs of connected chambers (for multi-room analysis)
}

// ChamberExtractor extracts chambers from modifications using flood-fill
type ChamberExtractor struct {
	overlay *ModificationOverlay
}

// NewChamberExtractor creates a new chamber extractor
func NewChamberExtractor(overlay *ModificationOverlay) *ChamberExtractor {
	return &ChamberExtractor{
		overlay: overlay,
	}
}

// ExtractChambers finds all connected chambers in the modification overlay
// Uses 6-connected flood-fill (N/S/E/W/Up/Down)
func (c *ChamberExtractor) ExtractChambers() []Chamber {
	// Get all modifications to process
	allBounds := c.overlay.GetBounds()
	if allBounds.XMin == 0 && allBounds.XMax == 0 && c.overlay.GetCount() == 0 {
		return []Chamber{} // No modifications
	}

	// Get all modification coordinates
	allMods := c.overlay.GetModificationsInRegion(allBounds, c.overlay.GetLastUpdateTime().Add(-24*365*100*time.Hour)) // Far past to get all
	if len(allMods) == 0 {
		return []Chamber{}
	}

	// Track visited coordinates
	visited := make(map[Coordinate]bool)
	chambers := []Chamber{}
	nextChamberID := uint32(1)

	// Process each unvisited modification
	for coord := range allMods {
		if visited[coord] {
			continue
		}

		// Start flood-fill from this coordinate
		chamberTiles := c.floodFill(coord, allMods, visited)
		if len(chamberTiles) == 0 {
			continue
		}

		// Create chamber from flood-fill results
		chamber := c.createChamber(nextChamberID, chamberTiles)
		chambers = append(chambers, chamber)
		nextChamberID++
	}

	return chambers
}

// floodFill performs 6-connected flood-fill from a starting coordinate
// Returns all coordinates in the connected region
func (c *ChamberExtractor) floodFill(start Coordinate, modifications map[Coordinate]ModificationInfo, visited map[Coordinate]bool) []Coordinate {
	// Use queue-based BFS for flood-fill
	queue := []Coordinate{start}
	result := []Coordinate{}
	visited[start] = true

	// 6-connected neighbors: N, S, E, W, Up, Down
	directions := []Coordinate{
		{X: 0, Y: -1, Z: 0}, // North
		{X: 0, Y: 1, Z: 0},  // South
		{X: 1, Y: 0, Z: 0},  // East
		{X: -1, Y: 0, Z: 0}, // West
		{X: 0, Y: 0, Z: 1},  // Up
		{X: 0, Y: 0, Z: -1}, // Down
	}

	for len(queue) > 0 {
		// Pop from queue
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		// Check all 6 neighbors
		for _, dir := range directions {
			neighbor := Coordinate{
				X: current.X + dir.X,
				Y: current.Y + dir.Y,
				Z: current.Z + dir.Z,
			}

			// Skip if already visited
			if visited[neighbor] {
				continue
			}

			// Skip if not a modification
			if _, exists := modifications[neighbor]; !exists {
				continue
			}

			// Mark as visited and add to queue
			visited[neighbor] = true
			queue = append(queue, neighbor)
		}
	}

	return result
}

// createChamber creates a Chamber struct from a list of coordinates
func (c *ChamberExtractor) createChamber(id uint32, tiles []Coordinate) Chamber {
	if len(tiles) == 0 {
		return Chamber{ID: id}
	}

	// Calculate bounding box
	bounds := Region{
		XMin: tiles[0].X, XMax: tiles[0].X,
		YMin: tiles[0].Y, YMax: tiles[0].Y,
		ZMin: tiles[0].Z, ZMax: tiles[0].Z,
	}

	for _, coord := range tiles {
		if coord.X < bounds.XMin {
			bounds.XMin = coord.X
		}
		if coord.X > bounds.XMax {
			bounds.XMax = coord.X
		}
		if coord.Y < bounds.YMin {
			bounds.YMin = coord.Y
		}
		if coord.Y > bounds.YMax {
			bounds.YMax = coord.Y
		}
		if coord.Z < bounds.ZMin {
			bounds.ZMin = coord.Z
		}
		if coord.Z > bounds.ZMax {
			bounds.ZMax = coord.Z
		}
	}

	return Chamber{
		ID:        id,
		Bounds:    bounds,
		TileCount: uint32(len(tiles)),
		Tiles:     tiles,
	}
}

// GenerateChamberDescription creates a natural language description of a chamber
// Example: "bedroom area 5x8x2 (80 tiles) on level -15"
func GenerateChamberDescription(chamber Chamber) string {
	if chamber.TileCount == 0 {
		return "empty chamber"
	}

	// Calculate dimensions
	width := chamber.Bounds.XMax - chamber.Bounds.XMin + 1
	height := chamber.Bounds.YMax - chamber.Bounds.YMin + 1
	depth := chamber.Bounds.ZMax - chamber.Bounds.ZMin + 1

	// Determine chamber type based on dimensions
	chamberType := classifyChamber(int(width), int(height), int(depth), int(chamber.TileCount))

	// Format description
	desc := fmt.Sprintf("%s %dx%dx%d (%d tiles)",
		chamberType,
		width, height, depth,
		chamber.TileCount)

	// Add level information
	if chamber.Bounds.ZMin == chamber.Bounds.ZMax {
		desc += fmt.Sprintf(" on level %d", chamber.Bounds.ZMin)
	} else {
		desc += fmt.Sprintf(" spanning levels %d to %d", chamber.Bounds.ZMin, chamber.Bounds.ZMax)
	}

	return desc
}

// classifyChamber attempts to classify a chamber based on its dimensions
func classifyChamber(width, height, depth, tileCount int) string {
	// Single level chambers
	if depth == 1 {
		area := width * height
		fillRatio := float64(tileCount) / float64(area)

		// Corridor: long and narrow
		if (width >= 10 && height <= 3) || (height >= 10 && width <= 3) {
			return "corridor"
		}

		// Small room (bedroom, office)
		if area <= 40 && fillRatio > 0.5 {
			return "small room"
		}

		// Medium room (dining hall, workshop)
		if area <= 120 && fillRatio > 0.5 {
			return "medium room"
		}

		// Large room (great hall, barracks)
		if area > 120 && fillRatio > 0.5 {
			return "large hall"
		}

		// Partially filled area
		if fillRatio <= 0.5 {
			return "excavated area"
		}

		return "room"
	}

	// Multi-level chambers
	if depth > 1 {
		// Shaft: small footprint, multiple levels
		if width <= 3 && height <= 3 {
			return "vertical shaft"
		}

		// Stairwell
		if width <= 5 && height <= 5 && depth > 2 {
			return "stairwell"
		}

		// Multi-level chamber
		return "multi-level chamber"
	}

	return "chamber"
}

// GetChambersBySize returns chambers sorted by tile count (largest first)
func GetChambersBySize(chambers []Chamber) []Chamber {
	sorted := make([]Chamber, len(chambers))
	copy(sorted, chambers)

	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].TileCount > sorted[j].TileCount
	})

	return sorted
}

// GetChambersByLevel returns chambers on a specific Z-level
func GetChambersByLevel(chambers []Chamber, z int16) []Chamber {
	result := []Chamber{}
	for _, chamber := range chambers {
		// Chamber intersects this Z-level
		if z >= chamber.Bounds.ZMin && z <= chamber.Bounds.ZMax {
			result = append(result, chamber)
		}
	}
	return result
}
