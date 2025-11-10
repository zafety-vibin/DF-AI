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
	topologyOverlay *topology.Overlay,
	entities []*protocol.Entity,
	hazardMgr *hazards.Manager,
) error {
	svp.logger.Info("SVP: Starting terrain analysis", "fort", svp.fortName)

	// Detect embark Z from dwarf positions
	embarkZ, err := svp.detectEmbarkZ(entities)
	if err != nil {
		return fmt.Errorf("failed to detect embark Z: %w", err)
	}
	svp.embarkZ = embarkZ
	svp.logger.Debug("SVP: Detected embark Z", "z", embarkZ)

	// Calculate housing layer (5 levels below embark to reach uniform horizontal layer)
	svp.housingZ = embarkZ - 5
	svp.logger.Debug("SVP: Designated housing Z", "z", svp.housingZ)

	// Calculate workshop layer (immediately below housing)
	svp.workshopZ = svp.housingZ - 1
	svp.logger.Debug("SVP: Designated workshop Z", "z", svp.workshopZ)

	// Detect soil layers suitable for farming
	svp.farmZ = svp.detectSoilLayers(topologyOverlay)
	svp.logger.Debug("SVP: Detected soil layers for farming", "count", len(svp.farmZ), "z_levels", svp.farmZ)

	// Query hazard manager for dangerous Z-levels
	if hazardMgr != nil {
		svp.hazardZ = svp.detectHazardLayers(hazardMgr)
		svp.logger.Debug("SVP: Detected hazard layers", "count", len(svp.hazardZ), "z_levels", svp.hazardZ)

		// Validate housing/workshop don't overlap with hazards
		if svp.isHazardZLevel(svp.housingZ) {
			svp.logger.Warn("SVP: Housing Z conflicts with hazard, adjusting", "original_z", svp.housingZ)
			svp.housingZ = svp.findSafeZLevel(svp.housingZ, -1, hazardMgr)
			svp.workshopZ = svp.housingZ - 1
		}
		if svp.isHazardZLevel(svp.workshopZ) {
			svp.logger.Warn("SVP: Workshop Z conflicts with hazard, adjusting", "original_z", svp.workshopZ)
			svp.workshopZ = svp.findSafeZLevel(svp.workshopZ, -1, hazardMgr)
		}
	}

	svp.analyzedAt = time.Now()
	svp.ready = true

	svp.logger.Info("SVP: Terrain analysis complete",
		"embark_z", svp.embarkZ,
		"housing_z", svp.housingZ,
		"workshop_z", svp.workshopZ,
		"farm_z_count", len(svp.farmZ))

	return nil
}

// detectEmbarkZ calculates embark Z from dwarf initial positions (mode of Z coordinates)
func (svp *SpatialValidatorPlanner) detectEmbarkZ(entities []*protocol.Entity) (int, error) {
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

	svp.logger.Debug("SVP: Embark Z detection", "dwarf_count", dwarfCount, "unique_z_levels", len(zCounts), "mode_z", embarkZ)

	return embarkZ, nil
}

// detectSoilLayers queries topology for Z-levels with soil
func (svp *SpatialValidatorPlanner) detectSoilLayers(topologyOverlay *topology.Overlay) []int {
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
func (svp *SpatialValidatorPlanner) detectHazardLayers(hazardMgr *hazards.Manager) []int {
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
func (svp *SpatialValidatorPlanner) findSafeZLevel(startZ int, direction int, hazardMgr *hazards.Manager) int {
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
	if svp.isHazardZLevel(region.ZMin) {
		return false, fmt.Sprintf("Proposal at Z=%d conflicts with hazard zone", region.ZMin)
	}

	// Type-specific validation
	switch nodeType {
	case "bedroom_cluster":
		if region.ZMin != svp.housingZ {
			return false, fmt.Sprintf("Bedroom proposal should be at housing Z=%d, not Z=%d", svp.housingZ, region.ZMin)
		}
	case "farm_plot":
		validFarmZ := false
		for _, fz := range svp.farmZ {
			if region.ZMin == fz {
				validFarmZ = true
				break
			}
		}
		if !validFarmZ {
			return false, fmt.Sprintf("Farm proposal at Z=%d, but soil only at Z=%v", region.ZMin, svp.farmZ)
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

	svp.logger.Info("SVP: Saved layout to disk", "path", svp.persistencePath)
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
			"expected", fortName,
			"found", layout["fort_name"])
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
		"path", svp.persistencePath,
		"housing_z", svp.housingZ,
		"workshop_z", svp.workshopZ,
		"farm_z", svp.farmZ)

	return nil
}
