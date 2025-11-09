package agents

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// FoodSecurityAgent monitors food and drink stocks
type FoodSecurityAgent struct {
	enabled  bool
	priority int
	targets  map[string]interface{}
}

// NewFoodSecurityAgent creates a new food security agent from config
func NewFoodSecurityAgent(cfg config.AgentConfig) *FoodSecurityAgent {
	return &FoodSecurityAgent{
		enabled:  cfg.Enabled,
		priority: cfg.Priority,
		targets:  cfg.Targets,
	}
}

// Name returns the agent identifier
func (a *FoodSecurityAgent) Name() string {
	return "FoodSecurity"
}

// Priority returns the base priority level
func (a *FoodSecurityAgent) Priority() int {
	return a.priority
}

// Enabled returns whether the agent is active
func (a *FoodSecurityAgent) Enabled() bool {
	return a.enabled
}

// Analyze examines fort metrics and proposes farm_plot or gather_zone nodes
func (a *FoodSecurityAgent) Analyze(metrics *FortMetrics) []ModificationNode {
	if metrics.DwarfCount == 0 {
		return nil // No dwarves, nothing to do
	}

	// Get target thresholds from config
	foodTarget := getFloatTarget(a.targets, "food_per_dwarf", 20.0)
	drinkTarget := getFloatTarget(a.targets, "drink_per_dwarf", 15.0)

	// Check if food/drink below target
	foodDeficit := metrics.FoodPerDwarf < foodTarget
	drinkDeficit := metrics.DrinkPerDwarf < drinkTarget

	if !foodDeficit && !drinkDeficit {
		return nil // All targets met
	}

	proposals := make([]ModificationNode, 0, 2)

	// Calculate urgency based on how far below target
	foodUrgency := calculateUrgency(metrics.FoodPerDwarf, foodTarget, 5.0) // Critical if < 5/dwarf
	drinkUrgency := calculateUrgency(metrics.DrinkPerDwarf, drinkTarget, 5.0)

	// Propose farm plot if food deficit
	if foodDeficit {
		// TODO: Better coordinate selection based on topology
		// For now, propose near embark point at appropriate Z-level
		node := ModificationNode{
			ID:           fmt.Sprintf("food_%d", metrics.FortAge),
			AgentName:    a.Name(),
			Type:         NodeTypeFarmPlot,
			Region:       modifications.Region{XMin: 40, YMin: 30, ZMin: 125, XMax: 50, YMax: 40, ZMax: 125},
			Dependencies: []DependencyType{DependencyAccess},
			Conflicts:    []string{},
			Priority:     a.priority,
			Urgency:      foodUrgency,
			Rationale:    fmt.Sprintf("Food at %.1f/dwarf, below %.0f target", metrics.FoodPerDwarf, foodTarget),
			Metadata: map[string]interface{}{
				"deficit_per_dwarf": foodTarget - metrics.FoodPerDwarf,
			},
		}
		proposals = append(proposals, node)
	}

	// Propose gather zone if drink deficit (early game, before brewery)
	if drinkDeficit && metrics.FortAge < 30 {
		node := ModificationNode{
			ID:           fmt.Sprintf("gather_%d", metrics.FortAge),
			AgentName:    a.Name(),
			Type:         NodeTypeGatherZone,
			Region:       modifications.Region{XMin: 50, YMin: 40, ZMin: 125, XMax: 60, YMax: 50, ZMax: 125},
			Dependencies: []DependencyType{},
			Conflicts:    []string{},
			Priority:     a.priority,
			Urgency:      drinkUrgency,
			Rationale:    fmt.Sprintf("Drink at %.1f/dwarf, below %.0f target", metrics.DrinkPerDwarf, drinkTarget),
			Metadata: map[string]interface{}{
				"deficit_per_dwarf": drinkTarget - metrics.DrinkPerDwarf,
			},
		}
		proposals = append(proposals, node)
	}

	return proposals
}

// getFloatTarget safely retrieves a float target from config map
func getFloatTarget(targets map[string]interface{}, key string, defaultVal float64) float64 {
	if val, ok := targets[key]; ok {
		switch v := val.(type) {
		case float64:
			return v
		case int:
			return float64(v)
		}
	}
	return defaultVal
}

// getIntTarget safely retrieves an int target from config map
func getIntTarget(targets map[string]interface{}, key string, defaultVal int) int {
	if val, ok := targets[key]; ok {
		switch v := val.(type) {
		case int:
			return v
		case float64:
			return int(v)
		}
	}
	return defaultVal
}

// calculateUrgency computes urgency score based on how far below target
// Returns 0.0-1.0, where 1.0 is critical (below criticalThreshold)
func calculateUrgency(current, target, criticalThreshold float64) float64 {
	if current >= target {
		return 0.0 // No urgency if above target
	}

	deficit := target - current
	if current < criticalThreshold {
		return 1.0 // Critical urgency if below critical threshold
	}

	// Linear scale: 0.5 urgency at target, 1.0 at critical
	fraction := deficit / (target - criticalThreshold)
	if fraction > 1.0 {
		return 1.0
	}
	return 0.5 + (fraction * 0.5)
}
