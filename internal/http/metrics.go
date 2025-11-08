package http

import (
	"net/http"
	"time"

	"github.com/df-ai/orchestrator/internal/dfhack"
)

// HazardMetrics captures hazard overlay statistics
type HazardMetrics struct {
	AquiferCount uint32 `json:"aquifer_count"`
	WaterCount   uint32 `json:"water_count"`
	LavaCount    uint32 `json:"lava_count"`
	CavernCount  uint32 `json:"cavern_count"`
	EnemyCount   uint32 `json:"enemy_count"`
	DwarfCount   uint32 `json:"dwarf_count"`
	MemoryKB     uint64 `json:"memory_kb"`
}

// ModificationMetrics captures modification overlay statistics
type ModificationMetrics struct {
	TotalCount  int    `json:"total_count"`
	ChamberCount int   `json:"chamber_count"`
	BoundsX     string `json:"bounds_x,omitempty"`
	BoundsY     string `json:"bounds_y,omitempty"`
	BoundsZ     string `json:"bounds_z,omitempty"`
	MemoryKB    uint64 `json:"memory_kb"`
}

// ContextMetrics captures context assembly statistics
type ContextMetrics struct {
	Level0SizeBytes int `json:"level0_size_bytes"`
	Level1SizeBytes int `json:"level1_size_bytes"`
	BudgetKB        int `json:"budget_kb"`
}

// LLMMetrics captures LLM interaction statistics
type LLMMetrics struct {
	TotalTurns       int     `json:"total_turns"`
	TotalTokensPrompt int    `json:"total_tokens_prompt"`
	TotalTokensCompletion int `json:"total_tokens_completion"`
	AverageLatencyMS float64 `json:"average_latency_ms"`
	ProviderType     string  `json:"provider_type"`
	ModelName        string  `json:"model_name"`
}

// CommandMetrics captures command execution statistics
type CommandMetrics struct {
	TotalCommands    int     `json:"total_commands"`
	PendingCommands  int     `json:"pending_commands"`
	CompletedCommands int    `json:"completed_commands"`
	FailedCommands   int     `json:"failed_commands"`
	AverageAckMS     float64 `json:"average_ack_ms"`
}

// MetricsSnapshot aggregates operational metrics
type MetricsSnapshot struct {
	ServerUptimeSeconds int64                   `json:"server_uptime_seconds"`
	DFHackConnected     bool                    `json:"dfhack_connected"`
	Session             *dfhack.SessionMetrics  `json:"session,omitempty"`
	Hazards             *HazardMetrics          `json:"hazards,omitempty"`
	Modifications       *ModificationMetrics    `json:"modifications,omitempty"`
	Context             *ContextMetrics         `json:"context,omitempty"`
	LLM                 *LLMMetrics             `json:"llm,omitempty"`
	Commands            *CommandMetrics         `json:"commands,omitempty"`
	ConfigReloads       uint64                  `json:"config_reloads"`
	HttpRequests        uint64                  `json:"http_requests"`
	Timestamp           time.Time               `json:"timestamp"`
}

// metricsHandler implements GET /metrics endpoint
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
	now := time.Now()
	uptime := now.Sub(s.startTime)

	// Get connection status
	connected := s.dfhackClient.IsConnected()

	// Get session metrics from DFHack client
	var sessionMetrics *dfhack.SessionMetrics
	if connected {
		metrics := s.dfhackClient.GetSessionMetrics()
		sessionMetrics = &metrics
	}

	// Get HTTP request count
	s.mu.Lock()
	httpReqs := s.httpRequests
	s.mu.Unlock()

	// Get config reload count from ConfigManager
	configReloads := s.configMgr.GetConfigReloads()

	// Get all metrics callbacks
	s.mu.Lock()
	getHazardMetrics := s.getHazardMetricsFunc
	getModificationMetrics := s.getModificationMetricsFunc
	getContextMetrics := s.getContextMetricsFunc
	getLLMMetrics := s.getLLMMetricsFunc
	getCommandMetrics := s.getCommandMetricsFunc
	s.mu.Unlock()

	// Call all metric functions
	var hazardMetrics *HazardMetrics
	if getHazardMetrics != nil {
		hazardMetrics = getHazardMetrics()
	}

	var modificationMetrics *ModificationMetrics
	if getModificationMetrics != nil {
		modificationMetrics = getModificationMetrics()
	}

	var contextMetrics *ContextMetrics
	if getContextMetrics != nil {
		contextMetrics = getContextMetrics()
	}

	var llmMetrics *LLMMetrics
	if getLLMMetrics != nil {
		llmMetrics = getLLMMetrics()
	}

	var commandMetrics *CommandMetrics
	if getCommandMetrics != nil {
		commandMetrics = getCommandMetrics()
	}

	snapshot := MetricsSnapshot{
		ServerUptimeSeconds: int64(uptime.Seconds()),
		DFHackConnected:     connected,
		Session:             sessionMetrics,
		Hazards:             hazardMetrics,
		Modifications:       modificationMetrics,
		Context:             contextMetrics,
		LLM:                 llmMetrics,
		Commands:            commandMetrics,
		ConfigReloads:       configReloads,
		HttpRequests:        httpReqs,
		Timestamp:           now,
	}

	s.writeJSON(w, http.StatusOK, snapshot)
}
