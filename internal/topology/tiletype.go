package topology

import "github.com/df-ai/orchestrator/internal/protocol"

// TileClass represents the 4 fundamental tile classifications
type TileClass uint8

const (
	TileClassWall   TileClass = 0 // Solid rock/wall - can dig
	TileClassFloor  TileClass = 1 // Walkable surface - can walk/channel
	TileClassVoid   TileClass = 2 // Open air - fall hazard
	TileClassLiquid TileClass = 3 // Water/magma 7/7 - disaster if breached
)

// ClassifyTile determines tile type from protocol flags (from DFHack classification)
func ClassifyTile(flags uint8) TileClass {
	// Check in priority order: Liquid > Void > Floor > Wall
	if flags&protocol.FlagLiquid7 != 0 {
		return TileClassLiquid
	}
	if flags&protocol.FlagVoid != 0 {
		return TileClassVoid
	}
	if flags&protocol.FlagFloor != 0 {
		return TileClassFloor
	}
	if flags&protocol.FlagWall != 0 {
		return TileClassWall
	}
	// Default: treat as wall if unclassified (conservative)
	return TileClassWall
}

// IsPathable determines if a tile allows movement (for backwards compatibility)
// Now uses DFHack classification flags instead of tiletype heuristic
func IsPathable(flags uint8) bool {
	class := ClassifyTile(flags)
	return class == TileClassFloor
}

// IsWall checks if tile is solid wall/rock (can dig)
func IsWall(flags uint8) bool {
	return ClassifyTile(flags) == TileClassWall
}

// IsFloor checks if tile is walkable surface
func IsFloor(flags uint8) bool {
	return ClassifyTile(flags) == TileClassFloor
}

// IsVoid checks if tile is open air (fall hazard)
func IsVoid(flags uint8) bool {
	return ClassifyTile(flags) == TileClassVoid
}

// IsLiquid checks if tile is dangerous liquid (7/7 water/magma)
func IsLiquid(flags uint8) bool {
	return ClassifyTile(flags) == TileClassLiquid
}