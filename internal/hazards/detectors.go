package hazards

// Detector functions for identifying hazards from tile data
//
// NOTE: These are heuristic-based detectors using TileType enum ranges.
// Detection accuracy may improve with protocol enhancement (Material, LiquidDepth, etc.)

// IsAquifer detects if a tile is an aquifer tile
// TileType heuristic: Aquifer tiles typically have specific wall types with wet/damp properties
// Flags bit 0x01 may indicate aquifer designation
func IsAquifer(tileType uint16, flags uint8) bool {
	// Aquifer flag check (bit 0)
	if flags&0x01 != 0 {
		return true
	}

	// TileType ranges for aquifer walls (heuristic based on DF tiletype enum)
	// These are approximate ranges - may need refinement based on actual DF data
	//
	// Typical aquifer tiletypes:
	// - Damp stone walls: 300-399 range (walls)
	// - Wet stone: specific subtypes in wall range
	//
	// This is a conservative heuristic - refine based on testing
	if tileType >= 300 && tileType < 500 {
		// Wall type - check if it's a "wet" variant
		// Without material data, we use flags as primary indicator
		return flags&0x01 != 0
	}

	return false
}

// IsWater detects if a tile contains water and returns depth and flow information
// Returns: (isWater, depth 0-7, flowFlags)
// FlowFlags: bits 0-2 = direction (N/S/E/W/Up/Down), bit 3 = standing(0)/flowing(1)
func IsWater(tileType uint16) (bool, uint8, uint8) {
	// Water floor tiles in DF tiletype enum
	// Typical water tiletypes:
	// - Floor water (shallow): ~90-99 range
	// - Floor water (deep): 100-109 range
	// - Flowing water: 110-119 range
	//
	// Without LiquidDepth field in protocol, we estimate depth from tiletype

	// Check if tiletype is in water range
	if tileType >= 90 && tileType < 120 {
		// Estimate depth based on tiletype (heuristic)
		depth := uint8(7) // Default to full depth
		flowFlags := uint8(0)

		if tileType >= 90 && tileType < 100 {
			depth = 3 // Shallow water
		} else if tileType >= 100 && tileType < 110 {
			depth = 7 // Deep water
		} else if tileType >= 110 && tileType < 120 {
			depth = 5          // Flowing water (medium depth)
			flowFlags = 0x08   // Set flowing bit
			flowFlags |= 0x01  // Default flow direction (North as placeholder)
		}

		return true, depth, flowFlags
	}

	return false, 0, 0
}

// IsLava detects if a tile contains lava/magma and returns depth and flow information
// Returns: (isLava, depth 0-7, flowFlags)
// FlowFlags: bits 0-2 = direction, bit 3 = standing(0)/flowing(1)
func IsLava(tileType uint16) (bool, uint8, uint8) {
	// Lava/magma floor tiles in DF tiletype enum
	// Typical magma tiletypes:
	// - Floor magma (shallow): ~120-129 range
	// - Floor magma (deep): 130-139 range
	// - Flowing magma: 140-149 range
	//
	// Without LiquidDepth field in protocol, we estimate depth from tiletype

	// Check if tiletype is in magma range
	if tileType >= 120 && tileType < 150 {
		// Estimate depth based on tiletype (heuristic)
		depth := uint8(7) // Default to full depth
		flowFlags := uint8(0)

		if tileType >= 120 && tileType < 130 {
			depth = 3 // Shallow magma
		} else if tileType >= 130 && tileType < 140 {
			depth = 7 // Deep magma
		} else if tileType >= 140 && tileType < 150 {
			depth = 5          // Flowing magma (medium depth)
			flowFlags = 0x08   // Set flowing bit
			flowFlags |= 0x01  // Default flow direction (North as placeholder)
		}

		return true, depth, flowFlags
	}

	return false, 0, 0
}

// IsCavern detects if a tile is part of a natural cavern (vs mined/constructed)
// Heuristic: Natural stone floors and open space in cavern Z-levels
func IsCavern(tileType uint16) bool {
	// Cavern detection heuristic:
	// - Natural stone floors: typically in 50-89 range
	// - Open/natural space: 0-50 range for natural floors
	// - Differentiate from constructed/mined floors
	//
	// Without material/biome data, this is approximate
	// Better detection requires knowing if tile is in cavern layer Z-levels

	// Natural floor tiletypes (heuristic)
	if tileType >= 50 && tileType < 90 {
		// Natural stone floor - likely cavern if in cavern Z-levels
		// This detector is conservative - will be refined with Z-level filtering
		return true
	}

	// Open space in caverns (no floor, no wall)
	if tileType >= 1 && tileType < 50 {
		// Natural open space - possible cavern
		return true
	}

	return false
}
