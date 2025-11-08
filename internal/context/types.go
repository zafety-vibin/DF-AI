package context

import (
	"time"

	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// ViewportLevel represents the detail level for context assembly
// Each level has different size budgets and content targets
type ViewportLevel uint8

const (
	// Level0 - Text overview only (500 bytes target)
	// "Embark year 1, surface Z=95-98, 7 dwarves, 3 chambers (48 tiles), food adequate, drink low"
	Level0 ViewportLevel = 0

	// Level1 - Active area (10 KB target)
	// Chambers + hazards + dwarves in modification region
	Level1 ViewportLevel = 1

	// Level2 - Deep planning (50 KB target)
	// Caverns + water/lava + modification area expanded
	Level2 ViewportLevel = 2

	// Level3 - Full context (200 KB target)
	// Compressed topology + all overlays + complete state
	Level3 ViewportLevel = 3
)

// String returns human-readable level name
func (v ViewportLevel) String() string {
	switch v {
	case Level0:
		return "Level0-TextOverview"
	case Level1:
		return "Level1-ActiveArea"
	case Level2:
		return "Level2-DeepPlanning"
	case Level3:
		return "Level3-FullContext"
	default:
		return "Unknown"
	}
}

// ViewportContext contains assembled fort state at a specific detail level
type ViewportContext struct {
	Level        ViewportLevel `json:"level"`
	AssembledAt  time.Time     `json:"assembled_at"`
	SizeBytes    uint32        `json:"size_bytes"`
	BudgetBytes  uint32        `json:"budget_bytes"`
	WithinBudget bool          `json:"within_budget"`

	// Level 0 (text only)
	TextOverview string `json:"text_overview,omitempty"`

	// Level 1+ (structured data)
	EmbarkPoint   *modifications.Coordinate `json:"embark_point,omitempty"` // Cached starting position (set once, never updated)
	Chambers      []ChamberFeature      `json:"chambers,omitempty"`
	Hazards       HazardData            `json:"hazards,omitempty"`
	Dwarves       []EntityPosition      `json:"dwarves,omitempty"`
	ActiveRegion  *modifications.Region `json:"active_region,omitempty"`
	TopologySlice *TopologySliceData    `json:"topology_slice,omitempty"` // Terrain map (first turn only)

	// Level 2+ (expanded data)
	CavernData   *CavernData      `json:"cavern_data,omitempty"`
	WaterHazards []HazardPosition `json:"water_hazards,omitempty"`
	LavaHazards  []HazardPosition `json:"lava_hazards,omitempty"`

	// Level 3 (compressed topology)
	TopologyData *TopologyData `json:"topology_data,omitempty"`
}

// ChamberFeature represents a chamber in natural language format
type ChamberFeature struct {
	ID          uint32 `json:"id"`
	Description string `json:"description"`
	BoundsMin   string `json:"bounds_min"` // "(x,y,z)"
	BoundsMax   string `json:"bounds_max"` // "(x,y,z)"
	Dimensions  string `json:"dimensions"` // "WxHxD"
	TileCount   uint32 `json:"tile_count"`
}

// HazardData contains aggregated hazard information
type HazardData struct {
	AquiferCount uint32 `json:"aquifer_count"`
	WaterCount   uint32 `json:"water_count"`
	LavaCount    uint32 `json:"lava_count"`
	CavernCount  uint32 `json:"cavern_count"`
	EnemyCount   uint32 `json:"enemy_count"`
}

// EntityPosition represents an entity's location
type EntityPosition struct {
	ID   uint32 `json:"id"`
	X    int16  `json:"x"`
	Y    int16  `json:"y"`
	Z    int16  `json:"z"`
	Type string `json:"type,omitempty"` // "dwarf", "enemy", etc.
}

// HazardPosition represents a hazard's location with severity
type HazardPosition struct {
	X        int16 `json:"x"`
	Y        int16 `json:"y"`
	Z        int16 `json:"z"`
	Severity uint8 `json:"severity"`
	Flags    uint8 `json:"flags,omitempty"`
}

// CavernData contains cavern layer information
type CavernData struct {
	TotalTiles uint32           `json:"total_tiles"`
	ZMin       int16            `json:"z_min"`
	ZMax       int16            `json:"z_max"`
	Samples    []HazardPosition `json:"samples,omitempty"` // Sampled positions, not all
}

// TopologyData contains compressed topology information
type TopologyData struct {
	Mode             string  `json:"mode"` // "full", "z_range", "single_z"
	CompressedSizeKB uint32  `json:"compressed_size_kb"`
	ZLevelsIncluded  []int16 `json:"z_levels_included"`
	OpenPercentage   float64 `json:"open_percentage"`
}

// TopologySliceData contains terrain map for a single Z-level region
type TopologySliceData struct {
	Z          int16    `json:"z"`           // Z-level
	XMin       int16    `json:"x_min"`       // Region bounds
	YMin       int16    `json:"y_min"`
	XMax       int16    `json:"x_max"`
	YMax       int16    `json:"y_max"`
	Width      int16    `json:"width"`       // Dimensions
	Height     int16    `json:"height"`
	OpenTiles  []string `json:"open_tiles"`  // Array of "X,Y" for open tiles
	Description string  `json:"description"` // Human-readable terrain summary
}

// QueryType represents the type of query to execute
type QueryType string

const (
	QueryTopologySlice         QueryType = "topology_slice"
	QueryHazardList            QueryType = "hazard_list"
	QueryModificationsInRegion QueryType = "modifications_in_region"
	QueryPathfind              QueryType = "pathfind" // Future extension
)

// QueryRequest represents a data query from the AI
type QueryRequest struct {
	Type   QueryType              `json:"type"`
	Params map[string]interface{} `json:"params"`
}

// QueryResponse contains the result of a query
type QueryResponse struct {
	Type         QueryType   `json:"type"`
	Success      bool        `json:"success"`
	Data         interface{} `json:"data,omitempty"`
	ErrorMessage string      `json:"error,omitempty"`
	SizeBytes    uint32      `json:"size_bytes"`
}

// EntityInfoSlice is a helper type for passing entity data
type EntityInfoSlice []protocol.EntityInfo
