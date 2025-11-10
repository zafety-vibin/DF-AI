package agents

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// HousingAgent monitors bedroom availability
type HousingAgent struct {
	enabled  bool
	priority int
	targets  map[string]interface{}
}

// NewHousingAgent creates a new housing agent from config
func NewHousingAgent(cfg config.AgentConfig) *HousingAgent {
	return &HousingAgent{
		enabled:  cfg.Enabled,
		priority: cfg.Priority,
		targets:  cfg.Targets,
	}
}

// Name returns the agent identifier
func (a *HousingAgent) Name() string {
	return "Housing"
}

// Priority returns the base priority level
func (a *HousingAgent) Priority() int {
	return a.priority
}

// Enabled returns whether the agent is active
func (a *HousingAgent) Enabled() bool {
	return a.enabled
}

// Analyze examines fort metrics and proposes bedroom_cluster nodes
func (a *HousingAgent) Analyze(metrics *FortMetrics) []ModificationNode {
	if metrics.DwarfCount == 0 {
		return nil
	}

	// Get target from config
	bedroomsPerDwarf := getFloatTarget(a.targets, "bedrooms_per_dwarf", 1.0)
	requiredBedrooms := int(float64(metrics.DwarfCount) * bedroomsPerDwarf)

	// Feature 007: Use real bedroom zone count instead of placeholder
	currentBedrooms := metrics.BedroomZoneCount
	if currentBedrooms == 0 {
		// Fallback to old BedroomCount if zone extraction not available
		currentBedrooms = metrics.BedroomCount
	}

	// Feature 007: Calculate deficit from real zone data
	deficit := requiredBedrooms - currentBedrooms
	if deficit <= 0 {
		return nil // All dwarves have bedrooms
	}

	// Feature 007: Calculate urgency based on deficit percentage (unhoused dwarf ratio)
	deficitRatio := float64(deficit) / float64(metrics.DwarfCount)
	urgency := deficitRatio // 1.0 if all dwarves lack bedrooms, 0.5 if half do

	// Feature 007: Use SVP housing Z-level if available
	housingZ := int16(125) // Default: arbitrary Z-level
	if metrics.SVPHousingZ != 0 {
		housingZ = int16(metrics.SVPHousingZ)
	} else {
		// Fallback heuristic if SVP not available
		housingZ = int16(125) // Assume mid-level embark
	}

	// Propose bedroom cluster at SVP-designated housing Z
	node := ModificationNode{
		ID:           fmt.Sprintf("housing_%d", metrics.FortAge),
		AgentName:    a.Name(),
		Type:         NodeTypeBedroom,
		Region:       modifications.Region{XMin: 60, YMin: 30, ZMin: housingZ, XMax: 70, YMax: 40, ZMax: housingZ},
		Dependencies: []DependencyType{DependencyAccess},
		Conflicts:    []string{},
		Priority:     a.priority,
		Urgency:      urgency,
		Rationale: fmt.Sprintf("%d dwarves, %d bedroom zones, %d deficit (housing Z=%d)",
			metrics.DwarfCount, currentBedrooms, deficit, housingZ),
		Metadata: map[string]interface{}{
			"deficit":          deficit,
			"bedrooms_needed":  requiredBedrooms,
			"current_bedrooms": currentBedrooms,
			"housing_z":        housingZ,
			"using_svp":        metrics.SVPHousingZ != 0,
		},
	}

	return []ModificationNode{node}
}

// AnalyzeIntent examines fort metrics and proposes housing INTENT (no coordinates) - HRM Architecture (R007)
func (a *HousingAgent) AnalyzeIntent(metrics *FortMetrics) []IntentProposal {
	if metrics.DwarfCount == 0 {
		return nil
	}

	// Get target from config
	bedroomsPerDwarf := getFloatTarget(a.targets, "bedrooms_per_dwarf", 1.0)
	requiredBedrooms := int(float64(metrics.DwarfCount) * bedroomsPerDwarf)

	// Use real bedroom zone count
	currentBedrooms := metrics.BedroomZoneCount
	if currentBedrooms == 0 {
		currentBedrooms = metrics.BedroomCount // Fallback
	}

	// Calculate deficit
	deficit := requiredBedrooms - currentBedrooms
	if deficit <= 0 {
		return nil // All dwarves have bedrooms
	}

	// Calculate urgency
	deficitRatio := float64(deficit) / float64(metrics.DwarfCount)
	urgency := deficitRatio

	// Select blueprint hint based on deficit size
	blueprintHint := ""
	if deficit >= 10 {
		blueprintHint = "bedroom_cluster_10"
	} else if deficit >= 3 {
		blueprintHint = "bedroom_3x3"
	}

	// Propose intent WITHOUT coordinates - arbiter decides WHERE
	proposal := IntentProposal{
		Agent:         a.Name(),
		Intent:        "provide_housing",
		Purpose:       "housing",
		Quantity:      deficit,
		Priority:      a.priority,
		Urgency:       urgency,
		Constraints:   []string{"safe_layer", "avoid_aquifer"},
		BlueprintHint: blueprintHint,
		Rationale: fmt.Sprintf("%d dwarves, %d bedrooms, %d deficit",
			metrics.DwarfCount, currentBedrooms, deficit),
		Metadata: map[string]interface{}{
			"deficit":          deficit,
			"bedrooms_needed":  requiredBedrooms,
			"current_bedrooms": currentBedrooms,
			"using_svp":        metrics.SVPHousingZ != 0,
		},
	}

	return []IntentProposal{proposal}
}
