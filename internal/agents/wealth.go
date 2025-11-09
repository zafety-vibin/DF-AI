package agents

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// WealthAgent monitors fort value growth
type WealthAgent struct {
	enabled  bool
	priority int
	targets  map[string]interface{}
}

// NewWealthAgent creates a new wealth agent from config
func NewWealthAgent(cfg config.AgentConfig) *WealthAgent {
	return &WealthAgent{
		enabled:  cfg.Enabled,
		priority: cfg.Priority,
		targets:  cfg.Targets,
	}
}

// Name returns the agent identifier
func (a *WealthAgent) Name() string {
	return "Wealth"
}

// Priority returns the base priority level
func (a *WealthAgent) Priority() int {
	return a.priority
}

// Enabled returns whether the agent is active
func (a *WealthAgent) Enabled() bool {
	return a.enabled
}

// Analyze examines fort metrics and proposes workshop_zone or production_area nodes
func (a *WealthAgent) Analyze(metrics *FortMetrics) []ModificationNode {
	// Get growth threshold from config
	growthThreshold := getFloatTarget(a.targets, "growth_threshold", 0.10) // 10% per 10 cycles

	// Check if wealth growth is stagnating
	if metrics.WealthGrowthRate >= growthThreshold {
		return nil // Growth is healthy
	}

	// Calculate urgency based on how far below threshold
	urgency := (growthThreshold - metrics.WealthGrowthRate) / growthThreshold
	if urgency > 1.0 {
		urgency = 1.0
	}
	if urgency < 0.3 {
		urgency = 0.3 // Minimum urgency for economic issues
	}

	// Propose workshop zone to boost production
	node := ModificationNode{
		ID:           fmt.Sprintf("wealth_%d", metrics.FortAge),
		AgentName:    a.Name(),
		Type:         NodeTypeWorkshop,
		Region:       modifications.Region{XMin: 20, YMin: 20, ZMin: 125, XMax: 40, YMax: 40, ZMax: 125},
		Dependencies: []DependencyType{DependencyAccess},
		Conflicts:    []string{},
		Priority:     a.priority,
		Urgency:      urgency,
		Rationale:    fmt.Sprintf("Wealth growth at %.1f%%, below %.0f%% target", metrics.WealthGrowthRate*100, growthThreshold*100),
		Metadata: map[string]interface{}{
			"current_growth": metrics.WealthGrowthRate,
			"target_growth":  growthThreshold,
		},
	}

	return []ModificationNode{node}
}
