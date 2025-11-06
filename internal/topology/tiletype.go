package topology

// IsPathable determines if a DF tiletype allows movement
// This is a simplified heuristic for MVP - may need refinement based on
// actual DF tiletype enum structure from DFHack
//
// Classification:
//   - 0: Undigged rock (closed)
//   - 1-299: Floors, ramps, stairs (open)
//   - 300+: Walls, fortifications (closed)
//
// TODO: Replace with proper DF tiletype enum parsing if heuristic proves inaccurate
func IsPathable(tileType uint16) bool {
	// Undigged rock
	if tileType == 0 {
		return false
	}

	// Floors, ramps, stairs (typical range)
	if tileType < 300 {
		return true
	}

	// Walls, fortifications
	return false
}