package context

import (
	"encoding/json"
	"fmt"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/spatial"
	"github.com/df-ai/orchestrator/internal/topology"
)

// Assembler coordinates context assembly at different detail levels
type Assembler struct {
	// Budget configuration (in bytes)
	level0Budget uint32 // Default: 500 bytes
	level1Budget uint32 // Default: 10 KB
	level2Budget uint32 // Default: 50 KB
	level3Budget uint32 // Default: 200 KB

	// Viewport configuration
	zMargin     int16  // Z-level margin above/below modifications
	hazardLimit uint32 // Maximum hazard positions to include

	// Embark point (cached from first dwarf positions, never updated)
	embarkPoint    modifications.Coordinate
	embarkPointSet bool

	// Spatial analysis (room detection)
	roomAnalyzer *spatial.RoomAnalyzer
	enableRoomDetection bool

	// Blueprint templates (optional)
	blueprints []BlueprintInfo
}

// NewAssembler creates a new context assembler with default budgets
func NewAssembler() *Assembler {
	return &Assembler{
		level0Budget: 500,        // 500 bytes for text overview
		level1Budget: 10 * 1024,  // 10 KB for active area
		level2Budget: 50 * 1024,  // 50 KB for deep planning
		level3Budget: 200 * 1024, // 200 KB for full context
		zMargin:      3,          // +/- 3 Z-levels
		hazardLimit:  1000,       // Max 1000 hazard positions
		roomAnalyzer: spatial.NewRoomAnalyzer(9, 400), // Default: 3x3 min, 20x20 max
		enableRoomDetection: false, // Disabled by default
	}
}

// EnableRoomDetection enables spatial room analysis
func (a *Assembler) EnableRoomDetection(minSize, maxSize int) {
	a.roomAnalyzer = spatial.NewRoomAnalyzer(minSize, maxSize)
	a.enableRoomDetection = true
}

// SetBlueprints sets available blueprint templates for AI
func (a *Assembler) SetBlueprints(blueprints []BlueprintInfo) {
	a.blueprints = blueprints
}

// NewAssemblerWithConfig creates an assembler with custom configuration
func NewAssemblerWithConfig(budgetKB uint32, zMargin int16, hazardLimit uint32) *Assembler {
	// Use provided budgetKB as base, scale for each level
	return &Assembler{
		level0Budget: 500,                  // Always 500B for text
		level1Budget: budgetKB * 1024,      // Use as-is for Level 1
		level2Budget: budgetKB * 1024 * 5,  // 5x for Level 2
		level3Budget: budgetKB * 1024 * 20, // 20x for Level 3
		zMargin:      zMargin,
		hazardLimit:  hazardLimit,
	}
}

// AssembleContext generates fort state context at the specified detail level
// This is the main dispatcher that routes to level-specific formatters
func (a *Assembler) AssembleContext(
	level ViewportLevel,
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	entities EntityInfoSlice,
	topoOverlay *topology.TopologyOverlay,
) (*ViewportContext, error) {
	switch level {
	case Level0:
		return a.assembleLevel0(mods, hazardMgr, entities)
	case Level1:
		return a.assembleLevel1(mods, hazardMgr, entities, topoOverlay)
	case Level2:
		return a.assembleLevel2(mods, hazardMgr, entities, topoOverlay)
	case Level3:
		return a.assembleLevel3(mods, hazardMgr, entities, topoOverlay)
	default:
		return nil, fmt.Errorf("invalid viewport level: %d", level)
	}
}

// assembleLevel0 generates text-only overview (500 bytes target)
func (a *Assembler) assembleLevel0(
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	entities EntityInfoSlice,
) (*ViewportContext, error) {
	// Count dwarves
	dwarfCount := 0
	for _, entity := range entities {
		if entity.Type == protocol.EntityTypeDwarf {
			dwarfCount++
		}
	}

	// Generate text overview
	text := GenerateTextOverview(mods, dwarfCount, hazardMgr)

	ctx := &ViewportContext{
		Level:        Level0,
		BudgetBytes:  a.level0Budget,
		TextOverview: text,
	}

	// Calculate size and check budget
	ctx.SizeBytes = uint32(len(text))
	ctx.WithinBudget = ctx.SizeBytes <= ctx.BudgetBytes

	return ctx, nil
}

// assembleLevel1 generates active area context (10 KB target)
func (a *Assembler) assembleLevel1(
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	entities EntityInfoSlice,
	topoOverlay *topology.TopologyOverlay,
) (*ViewportContext, error) {
	ctx := &ViewportContext{
		Level:       Level1,
		BudgetBytes: a.level1Budget,
	}

	// Calculate and cache embark point on first turn (never updates)
	// TODO: Future enhancement - detect when wagon/meeting area should be removed
	// Trigger: First bedrooms + tavern built, remove embark structures
	if mods != nil && mods.GetCount() == 0 && !a.embarkPointSet && len(entities) > 0 {
		a.embarkPoint = calculateDwarfCentroid(entities)
		a.embarkPointSet = true
		ctx.EmbarkPoint = &a.embarkPoint
	} else if a.embarkPointSet {
		// Include cached embark point in all contexts
		ctx.EmbarkPoint = &a.embarkPoint
	}

	// Extract chambers from modifications
	chambers := []ChamberFeature{}

	if mods != nil && mods.GetCount() > 0 {
		extractor := modifications.NewChamberExtractor(mods)
		chamberList := extractor.ExtractChambers()

		// Use room analyzer if enabled
		if a.enableRoomDetection && a.roomAnalyzer != nil {
			chambers = ExtractChamberFeaturesWithRooms(chamberList, a.roomAnalyzer)
		} else {
			chambers = ExtractChamberFeatures(chamberList)
		}

		// Set active region from modification bounds
		bounds := mods.GetBounds()
		ctx.ActiveRegion = &bounds
	} else if a.embarkPointSet {
		// No modifications yet - use embark point as focus (30-tile radius)
		embarkRegion := modifications.Region{
			XMin: a.embarkPoint.X - 30,
			XMax: a.embarkPoint.X + 30,
			YMin: a.embarkPoint.Y - 30,
			YMax: a.embarkPoint.Y + 30,
			ZMin: a.embarkPoint.Z,
			ZMax: a.embarkPoint.Z + 2, // Show 3 Z-levels at embark
		}
		ctx.ActiveRegion = &embarkRegion
	}

	// Extract hazard counts
	var hazardData HazardData
	if hazardMgr != nil {
		counts := hazardMgr.GetAllCounts()
		hazardData = HazardData{
			AquiferCount: counts["aquifer"],
			WaterCount:   counts["water"],
			LavaCount:    counts["lava"],
			CavernCount:  counts["caverns"],
			EnemyCount:   counts["enemies"],
		}
	}

	// Extract dwarf positions
	dwarves := FormatDwarfsAsJSON(entities)

	// If no modifications, include topology slice so AI can see terrain
	var topologySliceData *TopologySliceData
	if mods.GetCount() == 0 && a.embarkPointSet && topoOverlay != nil {
		// Find Z-level where MOST dwarves are (mode, not average)
		// This is more accurate than embark_point.Z which averages all positions
		dwarfZ := findMostCommonDwarfZ(entities)
		if dwarfZ == 0 {
			dwarfZ = a.embarkPoint.Z // Fallback to centroid if no dwarves
		}

		region := modifications.Region{
			XMin: a.embarkPoint.X - 30,
			XMax: a.embarkPoint.X + 30,
			YMin: a.embarkPoint.Y - 30,
			YMax: a.embarkPoint.Y + 30,
			ZMin: dwarfZ,
			ZMax: dwarfZ,
		}

		// Extract topology for this region
		topologySliceData = extractTopologySlice(topoOverlay, region)
	}

	ctx.Chambers = chambers
	ctx.Hazards = hazardData
	ctx.Dwarves = dwarves
	ctx.TopologySlice = topologySliceData
	ctx.Blueprints = a.blueprints // Available templates (if any)

	// Calculate size
	size, err := calculateContextSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate context size: %w", err)
	}
	ctx.SizeBytes = size
	ctx.WithinBudget = ctx.SizeBytes <= ctx.BudgetBytes

	return ctx, nil
}

// assembleLevel2 generates deep planning context (50 KB target)
func (a *Assembler) assembleLevel2(
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	entities EntityInfoSlice,
	topoOverlay *topology.TopologyOverlay,
) (*ViewportContext, error) {
	// Start with Level 1 context
	ctx, err := a.assembleLevel1(mods, hazardMgr, entities, topoOverlay)
	if err != nil {
		return nil, err
	}

	// Update to Level 2
	ctx.Level = Level2
	ctx.BudgetBytes = a.level2Budget

	// Add cavern data if available
	if hazardMgr != nil {
		cavernOverlay := hazardMgr.GetOverlay("caverns")
		if cavernOverlay != nil && cavernOverlay.GetCount() > 0 {
			ctx.CavernData = extractCavernData(cavernOverlay, a.hazardLimit)
		}

		// Add water hazards in active region
		waterOverlay := hazardMgr.GetOverlay("water")
		if waterOverlay != nil && ctx.ActiveRegion != nil {
			ctx.WaterHazards = extractHazardsInRegion(waterOverlay, *ctx.ActiveRegion, a.hazardLimit)
		}

		// Add lava hazards in active region
		lavaOverlay := hazardMgr.GetOverlay("lava")
		if lavaOverlay != nil && ctx.ActiveRegion != nil {
			ctx.LavaHazards = extractHazardsInRegion(lavaOverlay, *ctx.ActiveRegion, a.hazardLimit)
		}
	}

	// Recalculate size
	size, err := calculateContextSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate context size: %w", err)
	}
	ctx.SizeBytes = size
	ctx.WithinBudget = ctx.SizeBytes <= ctx.BudgetBytes

	return ctx, nil
}

// assembleLevel3 generates full context with compressed topology (200 KB target)
func (a *Assembler) assembleLevel3(
	mods *modifications.ModificationOverlay,
	hazardMgr *hazards.HazardManager,
	entities EntityInfoSlice,
	topoOverlay *topology.TopologyOverlay,
) (*ViewportContext, error) {
	// Start with Level 2 context
	ctx, err := a.assembleLevel2(mods, hazardMgr, entities, topoOverlay)
	if err != nil {
		return nil, err
	}

	// Update to Level 3
	ctx.Level = Level3
	ctx.BudgetBytes = a.level3Budget

	// Add compressed topology if available
	if topoOverlay != nil {
		// Determine compression mode based on active region
		var compConfig topology.CompressionConfig
		if ctx.ActiveRegion != nil {
			// Use Z-range compression centered on modification area
			centerZ := uint16((ctx.ActiveRegion.ZMin + ctx.ActiveRegion.ZMax) / 2)
			compConfig = topology.CompressionConfig{
				Mode:    "active_z_levels",
				CenterZ: centerZ,
				ZRadius: 5, // +/- 5 Z-levels
			}
		} else {
			// No modifications yet, use full compression
			compConfig = topology.CompressionConfig{
				Mode: "full",
			}
		}

		compressed, err := topoOverlay.Compress(compConfig)
		if err == nil {
			// Convert uint16 slice to int16 slice for JSON consistency
			zLevels := make([]int16, len(compressed.ZLevelsIncluded))
			for i, z := range compressed.ZLevelsIncluded {
				zLevels[i] = int16(z)
			}

			ctx.TopologyData = &TopologyData{
				Mode:             compressed.Mode,
				CompressedSizeKB: compressed.GetSize() / 1024,
				ZLevelsIncluded:  zLevels,
				OpenPercentage:   topoOverlay.GetOpenPercentage(),
			}
		}
	}

	// Recalculate size
	size, err := calculateContextSize(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate context size: %w", err)
	}
	ctx.SizeBytes = size
	ctx.WithinBudget = ctx.SizeBytes <= ctx.BudgetBytes

	return ctx, nil
}

// FormatAsJSON serializes a ViewportContext to JSON
func FormatAsJSON(ctx *ViewportContext) ([]byte, error) {
	return json.MarshalIndent(ctx, "", "  ")
}

// calculateContextSize estimates the JSON size of a context
func calculateContextSize(ctx *ViewportContext) (uint32, error) {
	data, err := json.Marshal(ctx)
	if err != nil {
		return 0, err
	}
	return uint32(len(data)), nil
}

// extractCavernData samples cavern overlay positions
func extractCavernData(overlay *hazards.HazardOverlay, limit uint32) *CavernData {
	// This is a simplified implementation
	// In production, would sample cavern positions intelligently
	count := overlay.GetCount()
	if count == 0 {
		return nil
	}

	return &CavernData{
		TotalTiles: count,
		ZMin:       10, // Typical cavern Z range
		ZMax:       90,
		Samples:    []HazardPosition{}, // Would add sampled positions here
	}
}

// extractHazardsInRegion gets hazard positions within a region, limited by count
func extractHazardsInRegion(overlay *hazards.HazardOverlay, region modifications.Region, limit uint32) []HazardPosition {
	coords := overlay.GetInRegion(hazards.Region{
		XMin: region.XMin, XMax: region.XMax,
		YMin: region.YMin, YMax: region.YMax,
		ZMin: region.ZMin, ZMax: region.ZMax,
	})

	// Limit results
	if uint32(len(coords)) > limit {
		coords = coords[:limit]
	}

	positions := make([]HazardPosition, 0, len(coords))
	for _, coord := range coords {
		info, exists := overlay.Get(coord.X, coord.Y, coord.Z)
		if !exists {
			continue
		}
		positions = append(positions, HazardPosition{
			X:        coord.X,
			Y:        coord.Y,
			Z:        coord.Z,
			Severity: info.Severity,
			Flags:    info.Flags,
		})
	}

	return positions
}

// findMostCommonDwarfZ finds the Z-level where most dwarves are located
// Returns the mode (most frequent) Z coordinate among all dwarves
func findMostCommonDwarfZ(entities EntityInfoSlice) int16 {
	zCounts := make(map[int16]int)

	for _, entity := range entities {
		if entity.Type == protocol.EntityTypeDwarf {
			zCounts[entity.Z]++
		}
	}

	if len(zCounts) == 0 {
		return 0 // No dwarves found
	}

	// Find Z-level with most dwarves
	var maxZ int16
	maxCount := 0
	for z, count := range zCounts {
		if count > maxCount {
			maxCount = count
			maxZ = z
		}
	}

	return maxZ
}

// calculateDwarfCentroid finds the average position of all dwarves
// Used to determine embark point on first turn (cached permanently)
func calculateDwarfCentroid(entities EntityInfoSlice) modifications.Coordinate {
	if len(entities) == 0 {
		return modifications.Coordinate{X: 0, Y: 0, Z: 0}
	}

	var sumX, sumY, sumZ int32
	count := 0

	for _, entity := range entities {
		// Only count dwarves (type 1), not animals
		if entity.Type == protocol.EntityTypeDwarf {
			sumX += int32(entity.X)
			sumY += int32(entity.Y)
			sumZ += int32(entity.Z)
			count++
		}
	}

	if count == 0 {
		// No dwarves found, use any entity
		sumX = int32(entities[0].X)
		sumY = int32(entities[0].Y)
		sumZ = int32(entities[0].Z)
		count = 1
	}

	return modifications.Coordinate{
		X: int16(sumX / int32(count)),
		Y: int16(sumY / int32(count)),
		Z: int16(sumZ / int32(count)),
	}
}

// extractTopologySlice extracts terrain map for embark area on first turn
func extractTopologySlice(topoOverlay *topology.TopologyOverlay, region modifications.Region) *TopologySliceData {
	if topoOverlay == nil {
		return nil
	}

	width := region.XMax - region.XMin + 1
	height := region.YMax - region.YMin + 1

	// Extract open tiles in region (walls vs open space)
	openTiles := []string{}
	openCount := 0

	for x := region.XMin; x <= region.XMax; x++ {
		for y := region.YMin; y <= region.YMax; y++ {
			isOpen, err := topoOverlay.IsOpen(x, y, region.ZMin)
			if err == nil && isOpen {
				openTiles = append(openTiles, fmt.Sprintf("%d,%d", x, y))
				openCount++
			}
		}
	}

	totalTiles := int(width * height)
	openPct := float64(openCount) / float64(totalTiles) * 100
	closedCount := totalTiles - openCount
	closedPct := 100 - openPct

	description := fmt.Sprintf("Embark area %dx%d at Z=%d: %d tiles ALREADY DUG/OPEN (%.1f%% - DO NOT DIG), %d tiles SOLID ROCK (%.1f%% - CAN DIG HERE)",
		width, height, region.ZMin, openCount, openPct, closedCount, closedPct)

	return &TopologySliceData{
		Z:           region.ZMin,
		XMin:        region.XMin,
		YMin:        region.YMin,
		XMax:        region.XMax,
		YMax:        region.YMax,
		Width:       width,
		Height:      height,
		OpenTiles:   openTiles,
		Description: description,
	}
}
