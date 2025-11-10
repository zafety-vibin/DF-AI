package zones

import (
	"time"

	"github.com/yourusername/df-ai/internal/protocol"
)

// ZoneType represents the category of a DF zone
type ZoneType int

const (
	ZoneTypeBedroom ZoneType = iota
	ZoneTypeDining
	ZoneTypeDormitory
	ZoneTypeOffice
	ZoneTypeBarracks
	ZoneTypeWorkshop
	ZoneTypeStockpile
)

// ZoneInfo represents a DF zone extracted from game state
type ZoneInfo struct {
	ZoneID     uint32          // Unique identifier from DF
	ZoneType   ZoneType        // Zone category
	Region     protocol.Region // Bounding box (x1,y1,z1,x2,y2,z2)
	AssignedTo int32           // Dwarf ID if assigned, -1 if unassigned
	SizeX      uint16          // Width in tiles
	SizeY      uint16          // Height in tiles
	CreatedAt  time.Time       // When zone was first detected
}

// IsAssigned returns true if zone has owner
func (z *ZoneInfo) IsAssigned() bool {
	return z.AssignedTo != -1
}

// GetArea returns tile count
func (z *ZoneInfo) GetArea() int {
	return int(z.SizeX) * int(z.SizeY)
}
