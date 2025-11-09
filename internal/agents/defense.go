package agents

import (
	"fmt"

	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/modifications"
)

// DefenseAgent monitors enemy presence and threats
type DefenseAgent struct {
	enabled       bool
	priorityBase  int
	priorityThreat int
}

// NewDefenseAgent creates a new defense agent from config
func NewDefenseAgent(cfg config.DefenseAgentConfig) *DefenseAgent {
	return &DefenseAgent{
		enabled:        cfg.Enabled,
		priorityBase:   cfg.PriorityBase,
		priorityThreat: cfg.PriorityThreat,
	}
}

// Name returns the agent identifier
func (a *DefenseAgent) Name() string {
	return "Defense"
}

// Priority returns the current priority level (variable based on threat status)
func (a *DefenseAgent) Priority() int {
	// Note: This returns base priority. Actual priority set in proposals based on threat detection.
	return a.priorityBase
}

// Enabled returns whether the agent is active
func (a *DefenseAgent) Enabled() bool {
	return a.enabled
}

// Analyze examines fort metrics and proposes seal_entrance or defensive_wall nodes
func (a *DefenseAgent) Analyze(metrics *FortMetrics) []ModificationNode {
	// Check for enemy threats
	if metrics.EnemyCount == 0 {
		return nil // No threats, no defensive actions needed
	}

	// Threats detected - escalate to maximum priority
	urgency := 1.0 // Always critical when enemies present

	// Propose defensive measures
	proposals := make([]ModificationNode, 0, 2)

	// Seal entrance to slow enemy advance
	sealNode := ModificationNode{
		ID:           fmt.Sprintf("defense_seal_%d", metrics.FortAge),
		AgentName:    a.Name(),
		Type:         NodeTypeSealEntrance,
		Region:       modifications.Region{XMin: 0, YMin: 0, ZMin: 125, XMax: 5, YMax: 5, ZMax: 125},
		Dependencies: []DependencyType{},
		Conflicts:    []string{},
		Priority:     a.priorityThreat, // Use threat priority (10)
		Urgency:      urgency,
		Rationale:    fmt.Sprintf("%d enemies detected - seal entrance for safety", metrics.EnemyCount),
		Metadata: map[string]interface{}{
			"enemy_count": metrics.EnemyCount,
			"action":      "emergency_seal",
		},
	}
	proposals = append(proposals, sealNode)

	// If multiple enemies, add defensive wall
	if metrics.EnemyCount > 3 {
		wallNode := ModificationNode{
			ID:           fmt.Sprintf("defense_wall_%d", metrics.FortAge),
			AgentName:    a.Name(),
			Type:         NodeTypeDefensive,
			Region:       modifications.Region{XMin: 10, YMin: 10, ZMin: 125, XMax: 15, YMax: 15, ZMax: 125},
			Dependencies: []DependencyType{},
			Conflicts:    []string{},
			Priority:     a.priorityThreat,
			Urgency:      urgency,
			Rationale:    fmt.Sprintf("%d enemies detected - fortify perimeter", metrics.EnemyCount),
			Metadata: map[string]interface{}{
				"enemy_count": metrics.EnemyCount,
				"action":      "build_walls",
			},
		}
		proposals = append(proposals, wallNode)
	}

	return proposals
}
