package context

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/phases"
	"github.com/df-ai/orchestrator/internal/spatial"
	"github.com/df-ai/orchestrator/internal/topology"
)

// ContextQueryType represents what kind of information to include
type ContextQueryType string

const (
	QueryTypeSpatial    ContextQueryType = "spatial"    // Topology, terrain
	QueryTypeEntities   ContextQueryType = "entities"   // Dwarves, animals, enemies
	QueryTypeHazards    ContextQueryType = "hazards"    // Water, lava, aquifers
	QueryTypeHistory    ContextQueryType = "history"    // Past modifications
	QueryTypeStatistics ContextQueryType = "statistics" // Fort-level metrics
	QueryTypePhase      ContextQueryType = "phase"      // Current development phase
	QueryTypeLayout     ContextQueryType = "layout"     // Rooms and areas
)

// ContextQuery specifies what information to include
type ContextQuery struct {
	Type        ContextQueryType
	Focus       *modifications.Coordinate // Optional center point
	Radius      int16                     // Tile radius around focus
	ZLevels     []int16                   // Specific Z-levels
	DetailLevel string                    // "summary", "detailed", "full"
}

// DynamicContext holds flexibly-assembled fort information
type DynamicContext struct {
	// Always included
	Meta     *MetaInfo `json:"meta"`
	BaseInfo *BaseInfo `json:"base_info"`

	// Query-driven sections
	Spatial    *SpatialInfo         `json:"spatial,omitempty"`
	Entities   *EntityInfo          `json:"entities,omitempty"`
	Hazards    *HazardInfo          `json:"hazards,omitempty"`
	History    *HistoryInfo         `json:"history,omitempty"`
	Statistics *StatisticsInfo      `json:"statistics,omitempty"`
	Phase      *phases.PhaseContext `json:"phase,omitempty"`
	Layout     *LayoutInfo          `json:"layout,omitempty"`

	// Size tracking
	SizeBytes   uint32 `json:"size_bytes"`
	BudgetBytes uint32 `json:"budget_bytes"`
}

// MetaInfo contains context assembly metadata
type MetaInfo struct {
	AssembledAt time.Time          `json:"assembled_at"`
	QueriesUsed []ContextQueryType `json:"queries_used"`
}

// BaseInfo contains always-included minimal information
type BaseInfo struct {
	DwarfCount     int      `json:"dwarf_count"`
	CriticalAlerts []string `json:"critical_alerts,omitempty"`
	DaysElapsed    int      `json:"days_elapsed,omitempty"`
}

// SpatialInfo contains terrain and topology data
type SpatialInfo struct {
	TopologySlice *TopologySliceData    `json:"topology_slice,omitempty"`
	OpenPercent   float64               `json:"open_percent,omitempty"`
	ActiveRegion  *modifications.Region `json:"active_region,omitempty"`
}

// EntityInfo contains dwarf/animal/enemy data
type EntityInfo struct {
	Dwarves []EntityPosition `json:"dwarves"`
	Enemies []EntityPosition `json:"enemies,omitempty"`
	Animals []EntityPosition `json:"animals,omitempty"`
}

// HazardInfo contains danger data
type HazardInfo struct {
	Counts    HazardData       `json:"counts"`
	Positions []HazardPosition `json:"positions,omitempty"` // Detailed if requested
}

// HistoryInfo contains fort development history
type HistoryInfo struct {
	Modifications int              `json:"total_modifications"`
	Chambers      []ChamberFeature `json:"chambers"`
	RecentActions []string         `json:"recent_actions,omitempty"`
}

// StatisticsInfo contains fort-level stats
type StatisticsInfo struct {
	Wealth      int64   `json:"wealth,omitempty"`
	FoodStocks  int     `json:"food_stocks,omitempty"`
	DrinkStocks int     `json:"drink_stocks,omitempty"`
	MoodAverage float64 `json:"mood_average,omitempty"`
}

// LayoutInfo contains room and area information
type LayoutInfo struct {
	Rooms   []*spatial.Room `json:"rooms"`
	Areas   []*spatial.Area `json:"areas,omitempty"`
	Summary string          `json:"summary"`
}

// DynamicContextAssembler builds context from queries
type DynamicContextAssembler struct {
	topology      *topology.TopologyOverlay
	modifications *modifications.ModificationOverlay
	hazards       *hazards.HazardManager
	phaseManager  *phases.PhaseManager
	fortLayout    *spatial.FortLayout

	maxSizeBytes  uint32
	alwaysInclude []string
}

// NewDynamicContextAssembler creates a new dynamic assembler
func NewDynamicContextAssembler(
	topo *topology.TopologyOverlay,
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	phaseMgr *phases.PhaseManager,
	layout *spatial.FortLayout,
	maxSize uint32,
	alwaysInclude []string,
) *DynamicContextAssembler {
	return &DynamicContextAssembler{
		topology:      topo,
		modifications: mods,
		hazards:       hazardMgr,
		phaseManager:  phaseMgr,
		fortLayout:    layout,
		maxSizeBytes:  maxSize,
		alwaysInclude: alwaysInclude,
	}
}

// AssembleContext builds context from queries
func (dca *DynamicContextAssembler) AssembleContext(
	queries []ContextQuery,
	entities EntityInfoSlice,
) (*DynamicContext, error) {

	ctx := &DynamicContext{
		Meta: &MetaInfo{
			AssembledAt: time.Now(),
			QueriesUsed: []ContextQueryType{},
		},
		BaseInfo:    dca.getBaseInfo(entities),
		BudgetBytes: dca.maxSizeBytes,
	}

	// Always include configured sections
	for _, include := range dca.alwaysInclude {
		switch include {
		case "phase":
			if dca.phaseManager != nil {
				ctx.Phase = dca.phaseManager.GetPhaseContext()
				ctx.Meta.QueriesUsed = append(ctx.Meta.QueriesUsed, QueryTypePhase)
			}
		}
	}

	// Process queries
	for _, query := range queries {
		switch query.Type {
		case QueryTypeSpatial:
			ctx.Spatial = dca.getSpatialInfo(query, entities)
		case QueryTypeEntities:
			ctx.Entities = dca.getEntityInfo(query, entities)
		case QueryTypeHazards:
			ctx.Hazards = dca.getHazardInfo(query)
		case QueryTypeHistory:
			ctx.History = dca.getHistoryInfo(query)
		case QueryTypeStatistics:
			ctx.Statistics = dca.getStatisticsInfo()
		case QueryTypeLayout:
			ctx.Layout = dca.getLayoutInfo()
		}
		ctx.Meta.QueriesUsed = append(ctx.Meta.QueriesUsed, query.Type)
	}

	// Calculate size
	size, err := calculateDynamicContextSize(ctx)
	if err != nil {
		return nil, err
	}
	ctx.SizeBytes = size

	return ctx, nil
}

// getBaseInfo returns always-included minimal information
func (dca *DynamicContextAssembler) getBaseInfo(entities EntityInfoSlice) *BaseInfo {
	dwarfCount := 0
	enemyCount := 0

	for _, entity := range entities {
		switch entity.Type {
		case 0x01: // DWARF
			dwarfCount++
		case 0x02: // ENEMY
			enemyCount++
		}
	}

	baseInfo := &BaseInfo{
		DwarfCount:     dwarfCount,
		CriticalAlerts: []string{},
	}

	// Generate alerts
	if enemyCount > 0 {
		baseInfo.CriticalAlerts = append(baseInfo.CriticalAlerts,
			fmt.Sprintf("ALERT: %d enemies detected!", enemyCount))
	}

	if dca.phaseManager != nil {
		baseInfo.DaysElapsed = dca.phaseManager.GetDaysElapsed()
	}

	return baseInfo
}

// getSpatialInfo returns topology and terrain data
func (dca *DynamicContextAssembler) getSpatialInfo(query ContextQuery, entities EntityInfoSlice) *SpatialInfo {
	info := &SpatialInfo{}

	// Add topology slice if detailed
	if query.DetailLevel == "detailed" || query.DetailLevel == "full" {
		// Get most common dwarf Z-level
		dwarfZ := findMostCommonDwarfZ(entities)
		if dwarfZ == 0 && query.Focus != nil {
			dwarfZ = query.Focus.Z
		}

		if dwarfZ != 0 && dca.topology != nil {
			region := modifications.Region{
				XMin: query.Focus.X - query.Radius,
				XMax: query.Focus.X + query.Radius,
				YMin: query.Focus.Y - query.Radius,
				YMax: query.Focus.Y + query.Radius,
				ZMin: dwarfZ,
				ZMax: dwarfZ,
			}
			info.TopologySlice = extractTopologySlice(dca.topology, region)
		}
	}

	if dca.topology != nil {
		info.OpenPercent = dca.topology.GetOpenPercentage()
	}

	if dca.modifications != nil && dca.modifications.GetCount() > 0 {
		bounds := dca.modifications.GetBounds()
		info.ActiveRegion = &bounds
	}

	return info
}

// getEntityInfo returns entity positions
func (dca *DynamicContextAssembler) getEntityInfo(query ContextQuery, entities EntityInfoSlice) *EntityInfo {
	dwarves := FormatDwarfsAsJSON(entities)

	info := &EntityInfo{
		Dwarves: dwarves,
	}

	// Add enemies if detail level warrants
	if query.DetailLevel == "full" {
		// TODO: Format enemies, animals
	}

	return info
}

// getHazardInfo returns hazard data
func (dca *DynamicContextAssembler) getHazardInfo(query ContextQuery) *HazardInfo {
	if dca.hazards == nil {
		return &HazardInfo{}
	}

	counts := dca.hazards.GetAllCounts()
	info := &HazardInfo{
		Counts: HazardData{
			AquiferCount: counts["aquifer"],
			WaterCount:   counts["water"],
			LavaCount:    counts["lava"],
			CavernCount:  counts["caverns"],
			EnemyCount:   counts["enemies"],
		},
	}

	// Add detailed positions if requested
	if query.DetailLevel == "full" && query.Focus != nil {
		region := modifications.Region{
			XMin: query.Focus.X - query.Radius,
			XMax: query.Focus.X + query.Radius,
			YMin: query.Focus.Y - query.Radius,
			YMax: query.Focus.Y + query.Radius,
			ZMin: query.Focus.Z - 3,
			ZMax: query.Focus.Z + 3,
		}

		// Extract hazard positions in region
		waterOverlay := dca.hazards.GetOverlay("water")
		if waterOverlay != nil {
			info.Positions = extractHazardsInRegion(waterOverlay, region, 100)
		}
	}

	return info
}

// getHistoryInfo returns modification history
func (dca *DynamicContextAssembler) getHistoryInfo(query ContextQuery) *HistoryInfo {
	if dca.modifications == nil {
		return &HistoryInfo{}
	}

	extractor := modifications.NewChamberExtractor(dca.modifications)
	chambers := extractor.ExtractChambers()
	chamberFeatures := ExtractChamberFeatures(chambers)

	return &HistoryInfo{
		Modifications: int(dca.modifications.GetCount()),
		Chambers:      chamberFeatures,
	}
}

// getStatisticsInfo returns fort-level stats
func (dca *DynamicContextAssembler) getStatisticsInfo() *StatisticsInfo {
	// TODO: Get actual stats from DF
	return &StatisticsInfo{}
}

// getLayoutInfo returns room and area structure
func (dca *DynamicContextAssembler) getLayoutInfo() *LayoutInfo {
	if dca.fortLayout == nil {
		return &LayoutInfo{}
	}

	rooms := make([]*spatial.Room, 0, len(dca.fortLayout.Rooms))
	for _, room := range dca.fortLayout.Rooms {
		rooms = append(rooms, room)
	}

	areas := make([]*spatial.Area, 0, len(dca.fortLayout.Areas))
	for _, area := range dca.fortLayout.Areas {
		areas = append(areas, area)
	}

	return &LayoutInfo{
		Rooms:   rooms,
		Areas:   areas,
		Summary: dca.fortLayout.GetNaturalLanguageSummary(),
	}
}

// calculateDynamicContextSize estimates JSON size
func calculateDynamicContextSize(ctx *DynamicContext) (uint32, error) {
	data, err := json.Marshal(ctx)
	if err != nil {
		return 0, err
	}
	return uint32(len(data)), nil
}

// CreateStandardQueries returns default query set for autonomous mode
func CreateStandardQueries(focus *modifications.Coordinate) []ContextQuery {
	queries := []ContextQuery{
		{
			Type:        QueryTypeSpatial,
			Focus:       focus,
			Radius:      30,
			DetailLevel: "detailed",
		},
		{
			Type:        QueryTypeEntities,
			DetailLevel: "summary",
		},
		{
			Type:        QueryTypeHazards,
			Focus:       focus,
			Radius:      40,
			DetailLevel: "summary",
		},
		{
			Type:        QueryTypeHistory,
			DetailLevel: "detailed",
		},
	}

	return queries
}
