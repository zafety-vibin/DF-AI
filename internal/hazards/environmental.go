package hazards

// Environmental hazard overlay types
// Each embeds HazardOverlay for sparse storage and provides type-specific semantics

// AquiferOverlay tracks aquifer tiles (critical safety hazard)
type AquiferOverlay struct {
	*HazardOverlay
}

// NewAquiferOverlay creates a new aquifer hazard overlay
func NewAquiferOverlay(bounds Bounds) *AquiferOverlay {
	return &AquiferOverlay{
		HazardOverlay: NewHazardOverlay("aquifer", bounds),
	}
}

// WaterOverlay tracks water tiles (standing and flowing)
type WaterOverlay struct {
	*HazardOverlay
}

// NewWaterOverlay creates a new water hazard overlay
func NewWaterOverlay(bounds Bounds) *WaterOverlay {
	return &WaterOverlay{
		HazardOverlay: NewHazardOverlay("water", bounds),
	}
}

// LavaOverlay tracks lava/magma tiles (critical safety hazard)
type LavaOverlay struct {
	*HazardOverlay
}

// NewLavaOverlay creates a new lava hazard overlay
func NewLavaOverlay(bounds Bounds) *LavaOverlay {
	return &LavaOverlay{
		HazardOverlay: NewHazardOverlay("lava", bounds),
	}
}

// CavernOverlay tracks natural cavern tiles (underground open space)
type CavernOverlay struct {
	*HazardOverlay
}

// NewCavernOverlay creates a new cavern hazard overlay
func NewCavernOverlay(bounds Bounds) *CavernOverlay {
	return &CavernOverlay{
		HazardOverlay: NewHazardOverlay("caverns", bounds),
	}
}
