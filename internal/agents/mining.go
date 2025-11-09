package agents

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// MiningAgent monitors mining activity and ore discovery
type MiningAgent struct {
	enabled  bool
	priority int
	targets  map[string]interface{}
}

// NewMiningAgent creates a new mining agent from config
func NewMiningAgent(cfg config.AgentConfig) *MiningAgent {
	return &MiningAgent{
		enabled:  cfg.Enabled,
		priority: cfg.Priority,
		targets:  cfg.Targets,
	}
}

// Name returns the agent identifier
func (a *MiningAgent) Name() string {
	return "Mining"
}

// Priority returns the base priority level
func (a *MiningAgent) Priority() int {
	return a.priority
}

// Enabled returns whether the agent is active
func (a *MiningAgent) Enabled() bool {
	return a.enabled
}

// Analyze examines fort metrics and proposes mining_shaft or exploratory_tunnel nodes
func (a *MiningAgent) Analyze(metrics *FortMetrics) []ModificationNode {
	// Get targets from config
	tilesPerCycle := getIntTarget(a.targets, "tiles_per_cycle", 50)
	noStrikeLimit := getIntTarget(a.targets, "no_strike_cycles", 20)

	// Check mining activity rate
	activityLow := metrics.MiningTilesPerCycle < tilesPerCycle
	noRecentStrikes := metrics.NoStrikeCycles > noStrikeLimit

	if !activityLow && !noRecentStrikes {
		return nil // Mining is productive
	}

	proposals := make([]ModificationNode, 0, 1)

	// If no ore strikes in many cycles, propose exploratory mining
	if noRecentStrikes {
		urgency := float64(metrics.NoStrikeCycles) / float64(noStrikeLimit*2) // Increases with time
		if urgency > 1.0 {
			urgency = 1.0
		}

		node := ModificationNode{
			ID:           fmt.Sprintf("mining_%d", metrics.FortAge),
			AgentName:    a.Name(),
			Type:         NodeTypeMiningShaft,
			Region:       modifications.Region{XMin: -40, YMin: -40, ZMin: 110, XMax: -35, YMax: -35, ZMax: 120},
			Dependencies: []DependencyType{DependencyStairs},
			Conflicts:    []string{},
			Priority:     a.priority,
			Urgency:      urgency,
			Rationale:    fmt.Sprintf("No ore strikes in %d cycles (limit: %d)", metrics.NoStrikeCycles, noStrikeLimit),
			Metadata: map[string]interface{}{
				"cycles_since_strike": metrics.NoStrikeCycles,
				"depth_range":         "110-120",
			},
		}
		proposals = append(proposals, node)
	}

	return proposals
}
