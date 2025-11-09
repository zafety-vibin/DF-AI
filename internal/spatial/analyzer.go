package spatial

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/modifications"
)

// RoomAnalyzer infers room types from chamber characteristics
type RoomAnalyzer struct {
	minRoomSize int
	maxRoomSize int
}

// NewRoomAnalyzer creates a new room analyzer
func NewRoomAnalyzer(minSize, maxSize int) *RoomAnalyzer {
	return &RoomAnalyzer{
		minRoomSize: minSize,
		maxRoomSize: maxSize,
	}
}

// AnalyzeChambers converts raw chambers into typed rooms
func (ra *RoomAnalyzer) AnalyzeChambers(chambers []*modifications.Chamber) []*Room {
	rooms := make([]*Room, 0, len(chambers))

	for i, chamber := range chambers {
		room := ra.chamberToRoom(chamber, i)
		rooms = append(rooms, room)
	}

	return rooms
}

// chamberToRoom converts a chamber to a typed room
func (ra *RoomAnalyzer) chamberToRoom(chamber *modifications.Chamber, index int) *Room {
	bounds := chamber.Bounds
	tileCount := len(chamber.Tiles)

	w := bounds.XMax - bounds.XMin + 1
	h := bounds.YMax - bounds.YMin + 1
	d := bounds.ZMax - bounds.ZMin + 1

	room := &Room{
		ID:              fmt.Sprintf("room_%d", index),
		Type:            ra.inferRoomType(w, h, d, tileCount),
		Bounds:          bounds,
		Tiles:           chamber.Tiles,
		Entrances:       []modifications.Coordinate{}, // TODO: Detect entrances
		ConnectedRooms:  []string{},
		Purpose:         "",
		CreatedTurn:     0,
		LastUsed:        0,
		Metadata:        make(map[string]interface{}),
	}

	return room
}

// inferRoomType guesses room purpose from dimensions
func (ra *RoomAnalyzer) inferRoomType(width, height, depth int16, tileCount int) RoomType {
	// Single-tile thick = corridor
	if (width == 1 && height > 3) || (height == 1 && width > 3) {
		return RoomTypeCorridor
	}

	// Very small chamber (1-8 tiles) = likely passageway
	if tileCount <= 8 {
		return RoomTypeCorridor
	}

	// Small square room (9-20 tiles, roughly square) = bedroom
	if tileCount >= 9 && tileCount <= 20 {
		ratio := float64(width) / float64(height)
		if ratio > 0.7 && ratio < 1.3 { // Roughly square
			return RoomTypeBedroom
		}
	}

	// Medium room (20-80 tiles) = workshop or storage
	if tileCount >= 20 && tileCount <= 80 {
		// Wide rooms = storage (easier to access)
		if float64(width) > float64(height)*1.5 || float64(height) > float64(width)*1.5 {
			return RoomTypeStorage
		}
		return RoomTypeWorkshop
	}

	// Large room (80+ tiles) = dining hall or barracks
	if tileCount >= 80 {
		return RoomTypeDining
	}

	// Vertical structure (multi-Z) = stairwell
	if depth > 1 {
		return RoomTypeStairwell
	}

	return RoomTypeUnknown
}

// ClusterIntoAreas groups rooms into logical areas
func (ra *RoomAnalyzer) ClusterIntoAreas(rooms []*Room) []*Area {
	areas := []*Area{}

	// Group by Z-level proximity and type similarity
	grouped := ra.groupRoomsByProximity(rooms)

	for i, group := range grouped {
		area := &Area{
			ID:      fmt.Sprintf("area_%d", i),
			Name:    ra.inferAreaName(group),
			RoomIDs: ra.getRoomIDs(group),
			ZLevels: ra.getZLevels(group),
			Purpose: ra.inferAreaPurpose(group),
		}
		area.Description = ra.getAreaDescription(area, group)
		areas = append(areas, area)
	}

	return areas
}

// groupRoomsByProximity clusters nearby rooms
func (ra *RoomAnalyzer) groupRoomsByProximity(rooms []*Room) [][]*Room {
	// Simple Z-level clustering for now
	byZ := make(map[int16][]*Room)

	for _, room := range rooms {
		z := room.Bounds.ZMin
		byZ[z] = append(byZ[z], room)
	}

	groups := make([][]*Room, 0, len(byZ))
	for _, group := range byZ {
		if len(group) >= 2 { // At least 2 rooms to form an area
			groups = append(groups, group)
		}
	}

	return groups
}

// inferAreaName generates a name based on dominant room types
func (ra *RoomAnalyzer) inferAreaName(rooms []*Room) string {
	typeCounts := make(map[RoomType]int)
	for _, room := range rooms {
		typeCounts[room.Type]++
	}

	// Find dominant type
	var dominantType RoomType
	maxCount := 0
	for roomType, count := range typeCounts {
		if count > maxCount {
			maxCount = count
			dominantType = roomType
		}
	}

	switch dominantType {
	case RoomTypeBedroom:
		return "Living Quarters"
	case RoomTypeWorkshop:
		return "Industrial Zone"
	case RoomTypeDining:
		return "Social Area"
	case RoomTypeStorage:
		return "Storage District"
	case RoomTypeBarracks:
		return "Military Quarter"
	case RoomTypeFarm:
		return "Agricultural Area"
	default:
		z := rooms[0].Bounds.ZMin
		return fmt.Sprintf("Level %d Complex", z)
	}
}

// inferAreaPurpose determines functional purpose
func (ra *RoomAnalyzer) inferAreaPurpose(rooms []*Room) string {
	typeCounts := make(map[RoomType]int)
	for _, room := range rooms {
		typeCounts[room.Type]++
	}

	if typeCounts[RoomTypeBedroom] > len(rooms)/2 {
		return "residential"
	}
	if typeCounts[RoomTypeWorkshop] > len(rooms)/2 {
		return "industrial"
	}
	if typeCounts[RoomTypeBarracks] > 0 {
		return "military"
	}
	if typeCounts[RoomTypeFarm] > 0 {
		return "agricultural"
	}

	return "mixed_use"
}

// getRoomIDs extracts IDs from room slice
func (ra *RoomAnalyzer) getRoomIDs(rooms []*Room) []string {
	ids := make([]string, len(rooms))
	for i, room := range rooms {
		ids[i] = room.ID
	}
	return ids
}

// getZLevels extracts unique Z-levels
func (ra *RoomAnalyzer) getZLevels(rooms []*Room) []int16 {
	zSet := make(map[int16]bool)
	for _, room := range rooms {
		for z := room.Bounds.ZMin; z <= room.Bounds.ZMax; z++ {
			zSet[z] = true
		}
	}

	levels := make([]int16, 0, len(zSet))
	for z := range zSet {
		levels = append(levels, z)
	}
	return levels
}

// getAreaDescription generates natural language summary
func (ra *RoomAnalyzer) getAreaDescription(area *Area, rooms []*Room) string {
	return fmt.Sprintf("%s contains %d rooms (%s purpose) on Z-levels %v",
		area.Name, len(rooms), area.Purpose, area.ZLevels)
}
