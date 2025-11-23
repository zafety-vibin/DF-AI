package spatial

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

// SpatialValidatorPlanner analyzes embark terrain and designates Z-levels for fort functions
type SpatialValidatorPlanner struct {
	embarkZ         int       // Z-level where fortress embark occurred
	housingZ        int       // Designated Z-level for bedrooms/dormitories
	workshopZ       int       // Designated Z-level for workshops/stockpiles
	farmZ           []int     // Array of Z-levels suitable for farming (has soil)
	hazardZ         []int     // Z-levels to avoid (aquifer, lava, caverns)
	fortName        string    // Name of fortress (unique identifier for persistence)
	analyzedAt      time.Time // When terrain analysis was performed
	layoutVersion   string    // Schema version for migration support
	persistencePath string    // File path for saving layout
	logger          *logging.Logger
	ready           bool // Whether analysis has been completed

	// HRM Architecture: Rich spatial context for arbiter (R001)
	strategicLayout *StrategicLayout // Structured layout with regions and infrastructure counts
}

// NewSpatialValidatorPlanner creates a new SVP instance
func NewSpatialValidatorPlanner(fortName string, persistenceDir string, logger *logging.Logger) *SpatialValidatorPlanner {
	return &SpatialValidatorPlanner{
		fortName:        fortName,
		persistencePath: filepath.Join(persistenceDir, fortName, "svp_layout.json"),
		layoutVersion:   "1.0",
		logger:          logger,
		ready:           false,
		farmZ:           make([]int, 0),
		hazardZ:         make([]int, 0),
	}
}

// AnalyzeTerrain performs initial terrain analysis using topology and entity data
func (svp *SpatialValidatorPlanner) AnalyzeTerrain(
	topologyOverlay *topology.TopologyOverlay,
	entities []*protocol.EntityInfo,
	hazardMgr *hazards.HazardManager,
) error {
	svp.logger.Info("SVP: Starting terrain analysis",
		logging.Field{Key: "fort", Value: svp.fortName})

	// Detect embark Z from dwarf positions
	embarkZ, err := svp.detectEmbarkZ(entities)
	if err != nil {
		return fmt.Errorf("failed to detect embark Z: %w", err)
	}
	svp.embarkZ = embarkZ
	svp.logger.Debug("SVP: Detected embark Z",
		logging.Field{Key: "z", Value: embarkZ})

	// Calculate housing layer (5 levels below embark to reach uniform horizontal layer)
	svp.housingZ = embarkZ - 5
	svp.logger.Debug("SVP: Designated housing Z",
		logging.Field{Key: "z", Value: svp.housingZ})

	// Calculate workshop layer (immediately below housing)
	svp.workshopZ = svp.housingZ - 1
	svp.logger.Debug("SVP: Designated workshop Z",
		logging.Field{Key: "z", Value: svp.workshopZ})

	// Detect soil layers suitable for farming
	svp.farmZ = svp.detectSoilLayers(topologyOverlay)
	svp.logger.Debug("SVP: Detected soil layers for farming",
		logging.Field{Key: "count", Value: len(svp.farmZ)},
		logging.Field{Key: "z_levels", Value: svp.farmZ})

	// Query hazard manager for dangerous Z-levels
	if hazardMgr != nil {
		svp.hazardZ = svp.detectHazardLayers(hazardMgr)
		svp.logger.Debug("SVP: Detected hazard layers",
			logging.Field{Key: "count", Value: len(svp.hazardZ)},
			logging.Field{Key: "z_levels", Value: svp.hazardZ})

		// Validate housing/workshop don't overlap with hazards
		if svp.isHazardZLevel(svp.housingZ) {
			svp.logger.Warn("SVP: Housing Z conflicts with hazard, adjusting",
				logging.Field{Key: "original_z", Value: svp.housingZ})
			svp.housingZ = svp.findSafeZLevel(svp.housingZ, -1, hazardMgr)
			svp.workshopZ = svp.housingZ - 1
		}
		if svp.isHazardZLevel(svp.workshopZ) {
			svp.logger.Warn("SVP: Workshop Z conflicts with hazard, adjusting",
				logging.Field{Key: "original_z", Value: svp.workshopZ})
			svp.workshopZ = svp.findSafeZLevel(svp.workshopZ, -1, hazardMgr)
		}
	}

	svp.analyzedAt = time.Now()
	svp.ready = true

	// HRM Architecture: Build StrategicLayout with empty zone counts (R004)
	// Zone counts will be populated later via UpdateStrategicLayoutWithZones()
	emptyZones := make(map[int]map[string]int)
	svp.strategicLayout = svp.buildStrategicLayout(topologyOverlay, hazardMgr, emptyZones)

	svp.logger.Info("SVP: Terrain analysis complete",
		logging.Field{Key: "embark_z", Value: svp.embarkZ},
		logging.Field{Key: "housing_z", Value: svp.housingZ},
		logging.Field{Key: "workshop_z", Value: svp.workshopZ},
		logging.Field{Key: "farm_z_count", Value: len(svp.farmZ)},
		logging.Field{Key: "layout_layers", Value: len(svp.strategicLayout.Layers)})

	return nil
}

// detectEmbarkZ calculates embark Z from dwarf initial positions (mode of Z coordinates)
func (svp *SpatialValidatorPlanner) detectEmbarkZ(entities []*protocol.EntityInfo) (int, error) {
	if len(entities) == 0 {
		return 0, fmt.Errorf("no entities provided for embark detection")
	}

	// Count dwarf Z-levels
	zCounts := make(map[int]int)
	dwarfCount := 0

	for _, entity := range entities {
		if entity.Type == protocol.EntityTypeDwarf {
			zCounts[int(entity.Z)]++
			dwarfCount++
		}
	}

	if dwarfCount == 0 {
		return 0, fmt.Errorf("no dwarves found in entity list")
	}

	// Find mode (most common Z-level)
	maxCount := 0
	embarkZ := 0
	for z, count := range zCounts {
		if count > maxCount {
			maxCount = count
			embarkZ = z
		}
	}

	svp.logger.Debug("SVP: Embark Z detection",
		logging.Field{Key: "dwarf_count", Value: dwarfCount},
		logging.Field{Key: "unique_z_levels", Value: len(zCounts)},
		logging.Field{Key: "mode_z", Value: embarkZ})

	return embarkZ, nil
}

// detectSoilLayers queries topology for Z-levels with soil
func (svp *SpatialValidatorPlanner) detectSoilLayers(topologyOverlay *topology.TopologyOverlay) []int {
	soilLayers := make([]int, 0)

	if topologyOverlay == nil {
		svp.logger.Warn("SVP: Topology overlay not available, cannot detect soil")
		return soilLayers
	}

	// Query IsSoilLayer array from topology
	// Note: This assumes topology has been extended with is_soil_layer field
	// For now, use heuristic: embark surface and embark-1 likely have soil
	// TODO: Replace with actual topology.IsSoilLayer query once plugin implements it
	if svp.embarkZ > 0 {
		soilLayers = append(soilLayers, svp.embarkZ)
		soilLayers = append(soilLayers, svp.embarkZ-1)
	}

	return soilLayers
}

// detectHazardLayers queries hazard manager for dangerous Z-levels
func (svp *SpatialValidatorPlanner) detectHazardLayers(hazardMgr *hazards.HazardManager) []int {
	hazardLayers := make([]int, 0)

	// Query hazard manager for aquifer, lava, cavern Z-levels
	// Note: This depends on hazard manager API
	// For now, return empty - will be populated when hazard manager integration is complete
	// TODO: Implement hazard manager query once API is available

	return hazardLayers
}

// isHazardZLevel checks if a Z-level is in the hazard list
func (svp *SpatialValidatorPlanner) isHazardZLevel(z int) bool {
	for _, hz := range svp.hazardZ {
		if hz == z {
			return true
		}
	}
	return false
}

// findSafeZLevel finds a nearby Z-level that's not hazardous
func (svp *SpatialValidatorPlanner) findSafeZLevel(startZ int, direction int, hazardMgr *hazards.HazardManager) int {
	// Search in the given direction for a safe Z-level
	safeZ := startZ
	for i := 0; i < 10; i++ {
		safeZ += direction
		if safeZ < 0 || safeZ > 199 {
			break
		}
		if !svp.isHazardZLevel(safeZ) {
			return safeZ
		}
	}
	// If no safe Z found, return original (with warning already logged)
	return startZ
}

// GetHousingZ returns designated Z-level for housing
func (svp *SpatialValidatorPlanner) GetHousingZ() int {
	return svp.housingZ
}

// GetWorkshopZ returns designated Z-level for workshops
func (svp *SpatialValidatorPlanner) GetWorkshopZ() int {
	return svp.workshopZ
}

// GetFarmZ returns array of Z-levels suitable for farming
func (svp *SpatialValidatorPlanner) GetFarmZ() []int {
	return svp.farmZ
}

// IsReady returns true if SVP has completed analysis
func (svp *SpatialValidatorPlanner) IsReady() bool {
	return svp.ready
}

// ValidateProposal checks if proposed region complies with Z-level designations
func (svp *SpatialValidatorPlanner) ValidateProposal(
	nodeType string, // agents.NodeType would create circular dependency
	region modifications.Region,
) (bool, string) {
	if !svp.ready {
		return true, "" // SVP not ready, allow all proposals
	}

	// Check if proposal is in hazard zone
	proposalZ := int(region.ZMin)
	if svp.isHazardZLevel(proposalZ) {
		return false, fmt.Sprintf("Proposal at Z=%d conflicts with hazard zone", proposalZ)
	}

	// Type-specific validation
	switch nodeType {
	case "bedroom_cluster":
		if proposalZ != svp.housingZ {
			return false, fmt.Sprintf("Bedroom proposal should be at housing Z=%d, not Z=%d", svp.housingZ, proposalZ)
		}
	case "farm_plot":
		validFarmZ := false
		for _, fz := range svp.farmZ {
			if proposalZ == fz {
				validFarmZ = true
				break
			}
		}
		if !validFarmZ {
			return false, fmt.Sprintf("Farm proposal at Z=%d, but soil only at Z=%v", proposalZ, svp.farmZ)
		}
	}

	return true, ""
}

// Save persists layout to disk
func (svp *SpatialValidatorPlanner) Save() error {
	if !svp.ready {
		return fmt.Errorf("cannot save SVP layout: analysis not complete")
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(svp.persistencePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create persistence directory: %w", err)
	}

	// Marshal to JSON
	layout := map[string]interface{}{
		"fort_name":      svp.fortName,
		"embark_z":       svp.embarkZ,
		"housing_z":      svp.housingZ,
		"workshop_z":     svp.workshopZ,
		"farm_z":         svp.farmZ,
		"hazard_z":       svp.hazardZ,
		"analyzed_at":    svp.analyzedAt.Format(time.RFC3339),
		"version":        svp.layoutVersion,
	}

	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal SVP layout: %w", err)
	}

	// Write to file
	if err := os.WriteFile(svp.persistencePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write SVP layout: %w", err)
	}

	svp.logger.Info("SVP: Saved layout to disk",
		logging.Field{Key: "path", Value: svp.persistencePath})
	return nil
}

// Load restores layout from disk
func (svp *SpatialValidatorPlanner) Load(fortName string) error {
	data, err := os.ReadFile(svp.persistencePath)
	if err != nil {
		return fmt.Errorf("failed to read SVP layout: %w", err)
	}

	var layout map[string]interface{}
	if err := json.Unmarshal(data, &layout); err != nil {
		return fmt.Errorf("failed to unmarshal SVP layout: %w", err)
	}

	// Validate fort name matches
	if layout["fort_name"] != fortName {
		svp.logger.Warn("SVP: Fort name mismatch in loaded layout",
			logging.Field{Key: "expected", Value: fortName},
			logging.Field{Key: "found", Value: layout["fort_name"]})
	}

	// Restore fields
	svp.fortName = fortName
	svp.embarkZ = int(layout["embark_z"].(float64))
	svp.housingZ = int(layout["housing_z"].(float64))
	svp.workshopZ = int(layout["workshop_z"].(float64))

	// Restore farm Z array
	if farmZInterface, ok := layout["farm_z"].([]interface{}); ok {
		svp.farmZ = make([]int, len(farmZInterface))
		for i, v := range farmZInterface {
			svp.farmZ[i] = int(v.(float64))
		}
	}

	// Restore hazard Z array
	if hazardZInterface, ok := layout["hazard_z"].([]interface{}); ok {
		svp.hazardZ = make([]int, len(hazardZInterface))
		for i, v := range hazardZInterface {
			svp.hazardZ[i] = int(v.(float64))
		}
	}

	// Parse timestamp
	if analyzedAtStr, ok := layout["analyzed_at"].(string); ok {
		if t, err := time.Parse(time.RFC3339, analyzedAtStr); err == nil {
			svp.analyzedAt = t
		}
	}

	svp.layoutVersion = layout["version"].(string)
	svp.ready = true

	svp.logger.Info("SVP: Loaded layout from disk",
		logging.Field{Key: "path", Value: svp.persistencePath},
		logging.Field{Key: "housing_z", Value: svp.housingZ},
		logging.Field{Key: "workshop_z", Value: svp.workshopZ},
		logging.Field{Key: "farm_z", Value: svp.farmZ})

	return nil
}

// ===== HRM Architecture: StrategicLayout Generation (R003-R006) =====

// GetStrategicLayout returns the rich spatial context for arbiter (R003)
// Returns nil if SVP analysis not complete
func (svp *SpatialValidatorPlanner) GetStrategicLayout() *StrategicLayout {
	return svp.strategicLayout
}

// buildStrategicLayout constructs StrategicLayout from SVP analysis (R004)
// This is called at the end of AnalyzeTerrain()
func (svp *SpatialValidatorPlanner) buildStrategicLayout(
	topologyOverlay *topology.TopologyOverlay,
	hazardMgr *hazards.HazardManager,
	existingZones map[int]map[string]int, // Z-level → zone type → count
) *StrategicLayout {
	layout := &StrategicLayout{
		FortName:   svp.fortName,
		AnalyzedAt: svp.analyzedAt.Format(time.RFC3339),
		Version:    svp.layoutVersion,
		Layers:     make(map[string]*ZoneLayer),
	}

	// Analyze housing layer
	housingZones := make(map[string]int)
	if zones, ok := existingZones[svp.housingZ]; ok {
		housingZones = zones
	}
	layout.Layers["housing"] = svp.analyzeLayer(
		svp.housingZ,
		"housing",
		topologyOverlay,
		hazardMgr,
		housingZones,
	)

	// Analyze workshop layer
	workshopZones := make(map[string]int)
	if zones, ok := existingZones[svp.workshopZ]; ok {
		workshopZones = zones
	}
	layout.Layers["workshop"] = svp.analyzeLayer(
		svp.workshopZ,
		"workshop",
		topologyOverlay,
		hazardMgr,
		workshopZones,
	)

	// Analyze farm layers
	for i, farmZ := range svp.farmZ {
		farmZones := make(map[string]int)
		if zones, ok := existingZones[farmZ]; ok {
			farmZones = zones
		}
		layerName := fmt.Sprintf("farm_%d", i+1)
		layout.Layers[layerName] = svp.analyzeLayer(
			farmZ,
			"farm",
			topologyOverlay,
			hazardMgr,
			farmZones,
		)
	}

	return layout
}

// analyzeLayer scans a Z-level and identifies available construction regions (R005)
func (svp *SpatialValidatorPlanner) analyzeLayer(
	z int,
	purpose string,
	topologyOverlay *topology.TopologyOverlay,
	hazardMgr *hazards.HazardManager,
	existingZones map[string]int,
) *ZoneLayer {
	layer := &ZoneLayer{
		ZLevel:             z,
		Purpose:            purpose,
		Regions:            make([]*SpatialRegion, 0),
		ExistingZones:      existingZones,
		ExistingWorkshops:  make(map[string]int),
		ExistingStockpiles: make(map[string]int),
		ExistingStairs:     make([]*StaircaseLocation, 0),
		OrphanedStairs:     make([]*StaircaseLocation, 0),
		ConnectsToZAbove:   false,
		ConnectsToZBelow:   false,
		Constraints:        make([]string, 0),
	}

	// Add constraints based on purpose and hazards
	if purpose == "farm" {
		layer.Constraints = append(layer.Constraints, "requires_soil")
	}
	if svp.isHazardZLevel(z) {
		layer.Constraints = append(layer.Constraints, "hazard_zone")
	}

	// Find available construction regions on this Z-level (R006)
	regions := svp.findAvailableRegions(z, topologyOverlay, hazardMgr)
	layer.Regions = regions

	svp.logger.Debug("SVP: Analyzed layer",
		logging.Field{Key: "z", Value: z},
		logging.Field{Key: "purpose", Value: purpose},
		logging.Field{Key: "region_count", Value: len(regions)},
		logging.Field{Key: "existing_zones", Value: len(existingZones)})

	return layer
}

// findAvailableRegions scans topology for contiguous open/dug areas (R006)
// This is a placeholder - full implementation would use flood-fill/connected-components
func (svp *SpatialValidatorPlanner) findAvailableRegions(
	z int,
	topologyOverlay *topology.TopologyOverlay,
	hazardMgr *hazards.HazardManager,
) []*SpatialRegion {
	regions := make([]*SpatialRegion, 0)

	// Placeholder: Create one large available region per layer
	// Real implementation would:
	// 1. Scan all tiles at Z-level from topology
	// 2. Find contiguous "open" (dug, walkable) areas via flood-fill
	// 3. Generate bounding boxes for each contiguous region
	// 4. Filter out hazard zones, too-small regions (<9 tiles)

	// For now, assume embark has a large central area available
	width := 200  // Map width (from topology)
	height := 200 // Map height (from topology)
	if topologyOverlay != nil {
		w, h, _ := topologyOverlay.GetDimensions()
		width = int(w)
		height = int(h)
	}

	// Create one large region representing "somewhere on this layer can be built"
	// This gives arbiter freedom to place blueprints anywhere
	region := &SpatialRegion{
		ID:     fmt.Sprintf("%s_region_1", svp.getPurposePrefix(z)),
		BBox:   [6]int{10, 10, z, width - 10, height - 10, z}, // Leave 10-tile margin
		Status: "available",
		Area:   (width - 20) * (height - 20), // Available area
		Features: []string{"open_space"},
	}

	regions = append(regions, region)
	return regions
}

// getPurposePrefix returns a short prefix for region IDs
func (svp *SpatialValidatorPlanner) getPurposePrefix(z int) string {
	if z == svp.housingZ {
		return "housing"
	} else if z == svp.workshopZ {
		return "workshop"
	}
	for _, fz := range svp.farmZ {
		if z == fz {
			return "farm"
		}
	}
	return "misc"
}

// UpdateStrategicLayoutWithZones updates existing zone counts in StrategicLayout (R006)
// Called periodically when zone extraction data is available
func (svp *SpatialValidatorPlanner) UpdateStrategicLayoutWithZones(zonesByZ map[int]map[string]int) {
	if svp.strategicLayout == nil {
		return
	}

	// Update housing layer
	if layer := svp.strategicLayout.Layers["housing"]; layer != nil {
		if zones, ok := zonesByZ[svp.housingZ]; ok {
			layer.ExistingZones = zones
		}
	}

	// Update workshop layer
	if layer := svp.strategicLayout.Layers["workshop"]; layer != nil {
		if zones, ok := zonesByZ[svp.workshopZ]; ok {
			layer.ExistingZones = zones
		}
	}

	// Update farm layers
	for i, farmZ := range svp.farmZ {
		layerName := fmt.Sprintf("farm_%d", i+1)
		if layer := svp.strategicLayout.Layers[layerName]; layer != nil {
			if zones, ok := zonesByZ[farmZ]; ok {
				layer.ExistingZones = zones
			}
		}
	}

	svp.logger.Debug("SVP: Updated StrategicLayout with zone counts",
		logging.Field{Key: "z_levels_updated", Value: len(zonesByZ)})
}

// RebuildStrategicLayout rebuilds the StrategicLayout after loading from disk
// This is called when topology becomes available after loading saved Z-levels
func (svp *SpatialValidatorPlanner) RebuildStrategicLayout(
	topologyOverlay *topology.TopologyOverlay,
	hazardMgr *hazards.HazardManager,
) {
	if !svp.ready {
		return
	}
	if svp.strategicLayout != nil {
		return
	}
	emptyZones := make(map[int]map[string]int)
	svp.strategicLayout = svp.buildStrategicLayout(topologyOverlay, hazardMgr, emptyZones)
	svp.logger.Info("SVP: Rebuilt StrategicLayout from loaded data",
		logging.Field{Key: "layout_layers", Value: len(svp.strategicLayout.Layers)})
}
