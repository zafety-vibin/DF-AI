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

// ===== HRM Architecture: StrategicLayout JSON Output (R001) =====

// StrategicLayout is the rich spatial context for arbiter (HRM H-module input)
// Replaces simple int fields with structured region data and infrastructure counts
// Note: Different from FortLayout in types.go (room detection) - this is for strategic planning
type StrategicLayout struct {
	FortName   string                `json:"fort_name"`
	AnalyzedAt string                `json:"analyzed_at"` // RFC3339 format
	Version    string                `json:"version"`
	Layers     map[string]*ZoneLayer `json:"layers"` // "housing", "workshop", "farm"
}

// ZoneLayer represents a functional Z-level with available regions and existing infrastructure
type ZoneLayer struct {
	ZLevel  int              `json:"z_level"`
	Purpose string           `json:"purpose"` // "housing", "workshop", "farm", "storage"
	Regions []*SpatialRegion `json:"regions"` // Available construction areas

	// Existing infrastructure counts (from zone extraction)
	ExistingZones      map[string]int `json:"existing_zones"`      // "bedroom": 5, "dining": 1
	ExistingWorkshops  map[string]int `json:"existing_workshops"`  // "craftsdwarf": 2 (future)
	ExistingStockpiles map[string]int `json:"existing_stockpiles"` // "stone": 3 (future)

	Constraints []string `json:"constraints"` // "avoid_aquifer", "requires_soil", "hazard_zone"
}

// SpatialRegion is an available construction area within a layer
type SpatialRegion struct {
	ID       string   `json:"id"`       // "housing_1", "workshop_2"
	BBox     [6]int   `json:"bbox"`     // [x1, y1, z1, x2, y2, z2]
	Status   string   `json:"status"`   // "available", "partial", "occupied"
	Area     int      `json:"area"`     // Tile count (for blueprint fitting)
	Features []string `json:"features"` // "smooth_walls", "no_hazards", "near_stairs"
}

// GetLayer returns a specific layer by purpose, or nil if not found
func (sl *StrategicLayout) GetLayer(purpose string) *ZoneLayer {
	return sl.Layers[purpose]
}

// GetAvailableArea returns total available area on a layer
func (zl *ZoneLayer) GetAvailableArea() int {
	total := 0
	for _, region := range zl.Regions {
		if region.Status == "available" {
			total += region.Area
		}
	}
	return total
}

// HasConstraint checks if a constraint is present
func (zl *ZoneLayer) HasConstraint(constraint string) bool {
	for _, c := range zl.Constraints {
		if c == constraint {
			return true
		}
	}
	return false
}
