package agents

import (
	"github.com/df-ai/orchestrator/internal/phases"
)

// GoalAgent interface for specialized fort monitoring agents
type GoalAgent interface {
	// Name returns agent identifier (e.g., "FoodSecurity", "Housing")
	Name() string

	// Priority returns base priority level (1-10)
	Priority() int

	// Analyze examines fort metrics and returns modification node proposals
	// Returns empty slice if all targets met
	Analyze(metrics *FortMetrics) []ModificationNode

	// Enabled returns whether agent is active
	Enabled() bool
}

// FortMetrics contains all metrics needed by goal agents
type FortMetrics struct {
	// Population
	DwarfCount int

	// Resources
	FoodPerDwarf  float64
	DrinkPerDwarf float64

	// Infrastructure
	BedroomCount int

	// Production
	MiningTilesPerCycle int // Tiles dug in recent cycles
	NoStrikeCycles      int // Cycles since last ore discovery

	// Economy
	WealthGrowthRate float64 // Percentage growth over last 10 cycles
	TotalWealth      uint64

	// Threats
	EnemyCount int

	// Fort State
	FortAge int          // Days elapsed
	Phase   phases.FortPhase // Embark/Establish/Expand/Fortify

	// Feature 007: Zone Counts (real data from DF)
	BedroomZoneCount     int // Actual bedroom zones extracted from DF
	DiningZoneCount      int // Dining hall zones
	DormitoryZoneCount   int // Dormitory zones
	OfficeZoneCount      int // Office zones
	UnassignedBedroomCount int // Bedrooms without owners
	HousingDeficit       int // DwarfCount - BedroomZoneCount

	// Feature 007: SVP Designations
	SVPHousingZ  int   // SVP-designated housing Z-level
	SVPWorkshopZ int   // SVP-designated workshop Z-level
	SVPFarmZ     []int // SVP-designated farm Z-levels (soil layers)
}

// AgentRegistry manages available goal agents
type AgentRegistry struct {
	agents map[string]GoalAgent
}

// NewAgentRegistry creates a new registry
func NewAgentRegistry() *AgentRegistry {
	return &AgentRegistry{
		agents: make(map[string]GoalAgent),
	}
}

// Register adds an agent to the registry
func (r *AgentRegistry) Register(agent GoalAgent) {
	r.agents[agent.Name()] = agent
}

// Get retrieves an agent by name
func (r *AgentRegistry) Get(name string) (GoalAgent, bool) {
	agent, ok := r.agents[name]
	return agent, ok
}

// List returns all registered agent names
func (r *AgentRegistry) List() []string {
	names := make([]string, 0, len(r.agents))
	for name := range r.agents {
		names = append(names, name)
	}
	return names
}

// GetAll returns all registered agents
func (r *AgentRegistry) GetAll() []GoalAgent {
	agents := make([]GoalAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		agents = append(agents, agent)
	}
	return agents
}

// GetEnabled returns only enabled agents
func (r *AgentRegistry) GetEnabled() []GoalAgent {
	enabled := make([]GoalAgent, 0, len(r.agents))
	for _, agent := range r.agents {
		if agent.Enabled() {
			enabled = append(enabled, agent)
		}
	}
	return enabled
}
