package topology

// IsPathable determines if a DF tiletype allows movement
// INVERTED HEURISTIC: Testing revealed original logic was backwards
//
// Classification (CORRECTED):
//   - 0: Undigged rock/walls (closed - CAN DIG)
//   - 1-299: More walls, solid tiles (closed - CAN DIG)
//   - 300+: Floors, open space, ramps, stairs (open - DO NOT DIG)
//
// TODO: Replace with proper DF tiletype enum from DFHack once confirmed
func IsPathable(tileType uint16) bool {
	// Inverted: High values (300+) are open/passable (floors, air, ramps)
	// Low values (0-299) are solid/closed (walls, rock)
	return tileType >= 300
}