package spatial

// DesignationPurpose represents the intended use of a Z-level
type DesignationPurpose int

const (
	PurposeHousing DesignationPurpose = iota
	PurposeWorkshop
	PurposeFarm
	PurposeStorage
	PurposeIndustrial
	PurposeUnassigned
)

// CapacityStatus represents space availability on a Z-level
type CapacityStatus int

const (
	CapacityAvailable CapacityStatus = iota
	CapacityPartial
	CapacityFull
)

// ZLevelDesignation represents a designated purpose for a Z-level with capacity and hazard metadata
type ZLevelDesignation struct {
	ZLevel      int                // Z-coordinate of this level (0-199)
	Purpose     DesignationPurpose // Intended use (housing, workshop, farm, etc.)
	Capacity    CapacityStatus     // Space availability
	HasSoil     bool               // Whether this Z-level contains soil
	HasAquifer  bool               // Whether this Z-level has aquifer
	HasLava     bool               // Whether this Z-level has lava
	IsCavern    bool               // Whether this Z-level is cavern layer
	Notes       string             // Human-readable rationale for designation (max 200 chars)
}

// IsSafe returns true if no hazards present
func (d *ZLevelDesignation) IsSafe() bool {
	return !d.HasAquifer && !d.HasLava && !d.IsCavern
}

// CanAccommodate checks if this Z-level suitable for given purpose
func (d *ZLevelDesignation) CanAccommodate(purpose DesignationPurpose) bool {
	// Can't accommodate if not safe
	if !d.IsSafe() {
		return false
	}

	// Farm requires soil
	if purpose == PurposeFarm && !d.HasSoil {
		return false
	}

	// Can't accommodate if full
	if d.Capacity == CapacityFull {
		return false
	}

	return true
}
