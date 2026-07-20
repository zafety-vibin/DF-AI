package phases

import (
	"fmt"
	"time"
)

// FortPhase represents the current development stage of the fort
type FortPhase int

const (
	PhaseEmbark    FortPhase = iota // Days 0-7: Initial survival
	PhaseEstablish                  // Days 8-30: Basic infrastructure
	PhaseExpand                     // Days 31-100: Growth and production
	PhaseFortify                    // Days 100+: Defense and optimization
)

// String returns human-readable phase name
func (p FortPhase) String() string {
	switch p {
	case PhaseEmbark:
		return "embark"
	case PhaseEstablish:
		return "establish"
	case PhaseExpand:
		return "expand"
	case PhaseFortify:
		return "fortify"
	default:
		return "unknown"
	}
}

// PhaseConfig holds phase transition thresholds
type PhaseConfig struct {
	EmbarkDays    int // Days before transitioning to Establish
	EstablishDays int // Days before transitioning to Expand
	ExpandDays    int // Days before transitioning to Fortify
}

// PhaseContext contains phase-specific information for AI decisions
type PhaseContext struct {
	Phase       FortPhase
	DaysElapsed int
	DaysInPhase int
	Description string
	Goals       []string
	Priorities  []string
}

// PhaseManager tracks fort development phase
type PhaseManager struct {
	startTime time.Time
	config    PhaseConfig
	enabled   bool

	// Real DF days (from FortInfo)
	currentDays int
	lastUpdate  time.Time
}

// NewPhaseManager creates a new phase manager
func NewPhaseManager(config PhaseConfig, enabled bool) *PhaseManager {
	return &PhaseManager{
		startTime: time.Now(),
		config:    config,
		enabled:   enabled,
	}
}

// GetPhase determines current phase based on elapsed time
func (pm *PhaseManager) GetPhase() FortPhase {
	if !pm.enabled {
		return PhaseEmbark // Default if disabled
	}

	days := pm.GetDaysElapsed()

	if days < pm.config.EmbarkDays {
		return PhaseEmbark
	} else if days < pm.config.EstablishDays {
		return PhaseEstablish
	} else if days < pm.config.ExpandDays {
		return PhaseExpand
	}
	return PhaseFortify
}

// UpdateFromFortInfo updates phase manager with real DF days
func (pm *PhaseManager) UpdateFromFortInfo(days int) {
	pm.currentDays = days
	pm.lastUpdate = time.Now()
}

// GetDaysElapsed returns days since fort started
func (pm *PhaseManager) GetDaysElapsed() int {
	if pm.enabled && pm.currentDays > 0 {
		return pm.currentDays // Use REAL DF days from FortInfo
	}

	// Fallback: Estimate from real-time (less accurate)
	// DF day = 1200 ticks, ~28.8 seconds real-time at normal speed
	elapsed := time.Since(pm.startTime)
	estimatedDays := int(elapsed.Seconds() / 30)
	return estimatedDays
}

// GetPhaseContext returns detailed phase information for AI
func (pm *PhaseManager) GetPhaseContext() *PhaseContext {
	phase := pm.GetPhase()
	days := pm.GetDaysElapsed()
	daysInPhase := pm.getDaysInCurrentPhase(days)

	return &PhaseContext{
		Phase:       phase,
		DaysElapsed: days,
		DaysInPhase: daysInPhase,
		Description: pm.getPhaseDescription(phase),
		Goals:       pm.getPhaseGoals(phase),
		Priorities:  pm.getPhasePriorities(phase),
	}
}

// getDaysInCurrentPhase calculates how many days into current phase
func (pm *PhaseManager) getDaysInCurrentPhase(totalDays int) int {
	phase := pm.GetPhase()

	switch phase {
	case PhaseEmbark:
		return totalDays
	case PhaseEstablish:
		return totalDays - pm.config.EmbarkDays
	case PhaseExpand:
		return totalDays - pm.config.EstablishDays
	case PhaseFortify:
		return totalDays - pm.config.ExpandDays
	}
	return 0
}

// getPhaseDescription returns natural language phase description
func (pm *PhaseManager) getPhaseDescription(phase FortPhase) string {
	switch phase {
	case PhaseEmbark:
		return "Embark Phase: Focus on immediate survival - shelter, food, water"
	case PhaseEstablish:
		return "Establish Phase: Build basic infrastructure - workshops, bedrooms, dining hall"
	case PhaseExpand:
		return "Expand Phase: Grow production and population - mining, trade, complex industries"
	case PhaseFortify:
		return "Fortify Phase: Optimize defenses and efficiency - military, traps, legendary dining halls"
	default:
		return "Unknown phase"
	}
}

// getPhaseGoals returns goals for current phase
func (pm *PhaseManager) getPhaseGoals(phase FortPhase) []string {
	switch phase {
	case PhaseEmbark:
		return []string{
			"Create basic shelter (5×10 entrance hall)",
			"Secure food source (gather plants)",
			"Establish water access",
			"Dig temporary storage area",
		}
	case PhaseEstablish:
		return []string{
			"Build carpenter workshop",
			"Dig bedrooms (1 per dwarf)",
			"Create dining hall (10×10 minimum)",
			"Establish food stockpile",
			"Dig stairs to underground",
		}
	case PhaseExpand:
		return []string{
			"Expand mining operations",
			"Build multiple workshops (mason, smelter, forge)",
			"Create specialized storage (ores, gems, weapons)",
			"Dig deeper for valuable ores",
			"Establish trade depot area",
		}
	case PhaseFortify:
		return []string{
			"Build defensive walls and gates",
			"Create trap corridors",
			"Establish military training grounds",
			"Optimize production chains",
			"Build legendary dining hall (value >100k)",
		}
	default:
		return []string{}
	}
}

// getPhasePriorities returns prioritized actions for phase
func (pm *PhaseManager) getPhasePriorities(phase FortPhase) []string {
	switch phase {
	case PhaseEmbark:
		return []string{"shelter", "food", "water"}
	case PhaseEstablish:
		return []string{"workshops", "bedrooms", "storage"}
	case PhaseExpand:
		return []string{"mining", "production", "trade"}
	case PhaseFortify:
		return []string{"defense", "military", "optimization"}
	default:
		return []string{}
	}
}

// GetPromptAddition returns phase-specific text to add to system prompt
func (pm *PhaseManager) GetPromptAddition() string {
	ctx := pm.GetPhaseContext()

	return fmt.Sprintf(`
Current Fort Phase: %s (Day %d, %d days in phase)
Phase Description: %s

Phase Goals:
%s

Phase Priorities: %v

Adjust your strategy to match the current phase. Early phases focus on survival and basics,
later phases on expansion and optimization.
`,
		ctx.Phase.String(),
		ctx.DaysElapsed,
		ctx.DaysInPhase,
		ctx.Description,
		formatGoalList(ctx.Goals),
		ctx.Priorities,
	)
}

// formatGoalList converts goals to bullet list
func formatGoalList(goals []string) string {
	result := ""
	for i, goal := range goals {
		result += fmt.Sprintf("%d. %s\n", i+1, goal)
	}
	return result
}

// Reset resets the phase manager (for new fort)
func (pm *PhaseManager) Reset() {
	pm.startTime = time.Now()
}
