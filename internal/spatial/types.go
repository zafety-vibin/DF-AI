package spatial

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/modifications"
)

// RoomType represents the functional purpose of a room
type RoomType int

const (
	RoomTypeUnknown RoomType = iota
	RoomTypeCorridor
	RoomTypeBedroom
	RoomTypeDining
	RoomTypeWorkshop
	RoomTypeStorage
	RoomTypeEntrance
	RoomTypeStairwell
	RoomTypeFarm
	RoomTypeBarracks
)

// String returns human-readable room type
func (rt RoomType) String() string {
	names := []string{
		"unknown", "corridor", "bedroom", "dining", "workshop",
		"storage", "entrance", "stairwell", "farm", "barracks",
	}
	if int(rt) < len(names) {
		return names[rt]
	}
	return "unknown"
}

// Room represents a functional space in the fort
type Room struct {
	ID        string                  // Unique identifier
	Type      RoomType                // Functional purpose
	Bounds    modifications.Region    // 3D bounding box
	Tiles     []modifications.Coordinate // All tiles in room
	Entrances []modifications.Coordinate // Entrance points

	// Connectivity
	ConnectedRooms []string // IDs of adjacent rooms
	ParentArea     string   // Area ID this room belongs to

	// Metadata
	Purpose     string                 // AI-assigned purpose
	CreatedTurn int                    // Which AI turn created this
	LastUsed    int                    // Last turn with activity
	Metadata    map[string]interface{} // Custom properties
}

// GetDimensions returns width x height x depth
func (r *Room) GetDimensions() (int16, int16, int16) {
	return r.Bounds.XMax - r.Bounds.XMin + 1,
		r.Bounds.YMax - r.Bounds.YMin + 1,
		r.Bounds.ZMax - r.Bounds.ZMin + 1
}

// GetCenter returns approximate center coordinate
func (r *Room) GetCenter() modifications.Coordinate {
	return modifications.Coordinate{
		X: (r.Bounds.XMin + r.Bounds.XMax) / 2,
		Y: (r.Bounds.YMin + r.Bounds.YMax) / 2,
		Z: (r.Bounds.ZMin + r.Bounds.ZMax) / 2,
	}
}

// GetNaturalLanguageDescription returns human-readable description
func (r *Room) GetNaturalLanguageDescription() string {
	w, h, d := r.GetDimensions()
	return fmt.Sprintf("%s %dx%dx%d (%d tiles) on level %d",
		r.Type.String(), w, h, d, len(r.Tiles), r.Bounds.ZMin)
}

// Area represents a collection of related rooms
type Area struct {
	ID      string   // Unique identifier
	Name    string   // "Living Quarters", "Industrial Zone"
	RoomIDs []string // Rooms in this area
	ZLevels []int16  // Z-levels spanned

	// Purpose and function
	Purpose     string // "residential", "industrial", "military"
	Description string // Natural language summary
}

// GetRoomCount returns number of rooms in area
func (a *Area) GetRoomCount() int {
	return len(a.RoomIDs)
}

// FortLayout tracks spatial organization of the fort
type FortLayout struct {
	Rooms map[string]*Room // Room ID → Room
	Areas map[string]*Area // Area ID → Area

	// Indexing for fast queries
	roomsByType  map[RoomType][]*Room
	roomsByZLevel map[int16][]*Room
}

// NewFortLayout creates an empty fort layout
func NewFortLayout() *FortLayout {
	return &FortLayout{
		Rooms:         make(map[string]*Room),
		Areas:         make(map[string]*Area),
		roomsByType:   make(map[RoomType][]*Room),
		roomsByZLevel: make(map[int16][]*Room),
	}
}

// AddRoom adds a room to the layout
func (fl *FortLayout) AddRoom(room *Room) {
	fl.Rooms[room.ID] = room
	fl.roomsByType[room.Type] = append(fl.roomsByType[room.Type], room)
	for z := room.Bounds.ZMin; z <= room.Bounds.ZMax; z++ {
		fl.roomsByZLevel[z] = append(fl.roomsByZLevel[z], room)
	}
}

// GetRoomsByType returns all rooms of a specific type
func (fl *FortLayout) GetRoomsByType(roomType RoomType) []*Room {
	return fl.roomsByType[roomType]
}

// GetRoomsByZLevel returns all rooms on a specific Z-level
func (fl *FortLayout) GetRoomsByZLevel(z int16) []*Room {
	return fl.roomsByZLevel[z]
}

// GetNaturalLanguageSummary returns fort layout as text
func (fl *FortLayout) GetNaturalLanguageSummary() string {
	summary := fmt.Sprintf("Fort contains %d rooms across %d areas:\n",
		len(fl.Rooms), len(fl.Areas))

	// Group by type
	for roomType, rooms := range fl.roomsByType {
		if len(rooms) > 0 {
			summary += fmt.Sprintf("- %d %s(s)\n", len(rooms), roomType.String())
		}
	}

	// List areas
	for _, area := range fl.Areas {
		summary += fmt.Sprintf("- %s: %d rooms across Z-levels %v\n",
			area.Name, len(area.RoomIDs), area.ZLevels)
	}

	return summary
}
