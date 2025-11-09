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

	// Check if bedroom deficit exists
	deficit := requiredBedrooms - metrics.BedroomCount
	if deficit <= 0 {
		return nil // All dwarves have bedrooms
	}

	// Calculate urgency based on deficit percentage
	deficitRatio := float64(deficit) / float64(metrics.DwarfCount)
	urgency := deficitRatio // 1.0 if all dwarves lack bedrooms, 0.5 if half do

	// Propose bedroom cluster
	// TODO: Better coordinate selection based on topology and existing rooms
	node := ModificationNode{
		ID:           fmt.Sprintf("housing_%d", metrics.FortAge),
		AgentName:    a.Name(),
		Type:         NodeTypeBedroom,
		Region:       modifications.Region{XMin: 60, YMin: 30, ZMin: 125, XMax: 70, YMax: 40, ZMax: 125},
		Dependencies: []DependencyType{DependencyAccess},
		Conflicts:    []string{},
		Priority:     a.priority,
		Urgency:      urgency,
		Rationale:    fmt.Sprintf("%d dwarves, only %d bedrooms (%d deficit)", metrics.DwarfCount, metrics.BedroomCount, deficit),
		Metadata: map[string]interface{}{
			"deficit":          deficit,
			"bedrooms_needed":  requiredBedrooms,
			"current_bedrooms": metrics.BedroomCount,
		},
	}

	return []ModificationNode{node}
}
