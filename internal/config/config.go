package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds the orchestrator server configuration
type Config struct {
	// Network settings
	ListenPort    uint16        `yaml:"listen_port"`
	HttpPort      uint16        `yaml:"http_port"`
	EnableHttpApi bool          `yaml:"enable_http_api"`
	ReadTimeout   time.Duration `yaml:"read_timeout"`
	WriteTimeout  time.Duration `yaml:"write_timeout"`

	// Protocol settings
	HeartbeatInterval time.Duration `yaml:"heartbeat_interval"`
	HeartbeatTimeout  time.Duration `yaml:"heartbeat_timeout"`
	MaxMessageSize    uint32        `yaml:"max_message_size"`

	// Reconnection settings
	ReconnectDelay         time.Duration `yaml:"reconnect_delay"`
	ReconnectMaxDelay      time.Duration `yaml:"reconnect_max_delay"`
	ReconnectBackoffFactor float64       `yaml:"reconnect_backoff_factor"`

	// Logging settings
	LogLevel  string `yaml:"log_level"`  // debug, info, warn, error
	LogFormat string `yaml:"log_format"` // json, text

	// Topology overlay compression settings
	TopologyCompressionMode string `yaml:"topology_compression_mode"` // full, active_z_levels, custom_bounds
	TopologyCenterZ         uint16 `yaml:"topology_center_z"`
	TopologyZRadius         uint16 `yaml:"topology_z_radius"`

	// LLM Provider Configuration
	LLMProviderType string `yaml:"llm_provider_type"` // claude, openai_compatible, multi_model

	// Claude API settings
	ClaudeAPIKey string `yaml:"claude_api_key"` // From environment or config
	ClaudeModel  string `yaml:"claude_model"`   // claude-sonnet-4, claude-opus-4, etc.

	// OpenAI-compatible endpoint settings
	LLMEndpoint string `yaml:"llm_endpoint"` // Local LLM server or OpenAI-compatible API
	LLMModel    string `yaml:"llm_model"`    // Model name for OpenAI endpoint
	LLMAPIKey   string `yaml:"llm_api_key"`  // API key if required

	// LLM Request tuning
	LLMTemperature   float64       `yaml:"llm_temperature"`    // Randomness (0.0-1.0)
	LLMMaxTokens     int           `yaml:"llm_max_tokens"`     // Max response length
	LLMTimeoutSeconds time.Duration `yaml:"llm_timeout_seconds"` // Request timeout

	// Context assembly settings
	ContextBudgetKB              int `yaml:"context_budget_kb"`               // Max context size in KB
	ContextUpdateFrequencySeconds int `yaml:"context_update_frequency_seconds"` // Update frequency

	// Viewport settings
	ViewportActiveZMargin     uint16 `yaml:"viewport_active_z_margin"`      // Z-levels above/below active
	ViewportHazardMarginTiles uint16 `yaml:"viewport_hazard_margin_tiles"`  // Tile margin around hazards

	// Fort phase configuration
	PhaseEmbarkDays    int `yaml:"phase_embark_days"`     // Days in embark phase
	PhaseEstablishDays int `yaml:"phase_establish_days"`  // Days in establish phase
	PhaseExpandDays    int `yaml:"phase_expand_days"`     // Days before fortify phase
	EnablePhaseSystem  bool `yaml:"enable_phase_system"`   // Enable phase-based strategies

	// Task queue settings
	EnableTaskQueue       bool `yaml:"enable_task_queue"`        // Enable multi-step planning
	MaxQueuedTasks        int  `yaml:"max_queued_tasks"`         // Max tasks in queue
	TaskTimeoutSeconds    int  `yaml:"task_timeout_seconds"`     // Task execution timeout
	EnableTaskDependencies bool `yaml:"enable_task_dependencies"` // Allow dependent tasks

	// Persistence settings
	EnablePersistence    bool   `yaml:"enable_persistence"`      // Save modifications to disk
	PersistencePath      string `yaml:"persistence_path"`        // Where to save state
	AutoSaveIntervalSec  int    `yaml:"autosave_interval_sec"`   // Auto-save frequency
	LoadOnStartup        bool   `yaml:"load_on_startup"`         // Load saved state on boot

	// Spatial reasoning settings
	EnableRoomDetection   bool `yaml:"enable_room_detection"`    // Detect room types
	EnableAreaClustering  bool `yaml:"enable_area_clustering"`   // Cluster rooms into areas
	RoomMinSize           int  `yaml:"room_min_size"`            // Minimum tiles for room
	RoomMaxSize           int  `yaml:"room_max_size"`            // Maximum tiles for room

	// Dynamic context settings
	EnableDynamicContext  bool `yaml:"enable_dynamic_context"`   // Use query-based context
	ContextAlwaysInclude  []string `yaml:"context_always_include"` // Always send these fields
	ContextMaxSizeBytes   int  `yaml:"context_max_size_bytes"`   // Hard limit on context
}

// Load reads and parses the configuration file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	// Set defaults for any missing values
	cfg.setDefaults()

	return &cfg, nil
}

// setDefaults fills in default values for unset configuration
func (c *Config) setDefaults() {
	if c.ListenPort == 0 {
		c.ListenPort = 5001
	}
	if c.HttpPort == 0 {
		c.HttpPort = 8080
	}
	// EnableHttpApi defaults to true (no check needed, false is default)
	if c.ReadTimeout == 0 {
		c.ReadTimeout = 30 * time.Second
	}
	if c.WriteTimeout == 0 {
		c.WriteTimeout = 10 * time.Second
	}
	if c.HeartbeatInterval == 0 {
		c.HeartbeatInterval = 10 * time.Second
	}
	if c.HeartbeatTimeout == 0 {
		c.HeartbeatTimeout = 5 * time.Second
	}
	if c.MaxMessageSize == 0 {
		c.MaxMessageSize = 10 * 1024 * 1024 // 10 MB
	}
	if c.ReconnectDelay == 0 {
		c.ReconnectDelay = 1 * time.Second
	}
	if c.ReconnectMaxDelay == 0 {
		c.ReconnectMaxDelay = 30 * time.Second
	}
	if c.ReconnectBackoffFactor == 0 {
		c.ReconnectBackoffFactor = 2.0
	}
	if c.LogLevel == "" {
		c.LogLevel = "info"
	}
	if c.LogFormat == "" {
		c.LogFormat = "json"
	}
	if c.TopologyCompressionMode == "" {
		c.TopologyCompressionMode = "full"
	}
	if c.TopologyZRadius == 0 {
		c.TopologyZRadius = 3
	}

	// LLM Provider Configuration defaults
	if c.LLMProviderType == "" {
		c.LLMProviderType = "claude"
	}
	if c.ClaudeModel == "" {
		c.ClaudeModel = "claude-opus-4-1"
	}
	if c.LLMEndpoint == "" {
		c.LLMEndpoint = "http://localhost:8000"
	}
	if c.LLMModel == "" {
		c.LLMModel = "llama-3-70b"
	}
	if c.LLMTemperature == 0 {
		c.LLMTemperature = 0.7
	}
	if c.LLMMaxTokens == 0 {
		c.LLMMaxTokens = 4096
	}
	if c.LLMTimeoutSeconds == 0 {
		c.LLMTimeoutSeconds = 60 * time.Second
	}

	// Context assembly defaults
	if c.ContextBudgetKB == 0 {
		c.ContextBudgetKB = 200 // 200 KB
	}
	if c.ContextUpdateFrequencySeconds == 0 {
		c.ContextUpdateFrequencySeconds = 100
	}

	// Viewport defaults
	if c.ViewportActiveZMargin == 0 {
		c.ViewportActiveZMargin = 3
	}
	if c.ViewportHazardMarginTiles == 0 {
		c.ViewportHazardMarginTiles = 5
	}

	// Fort phase defaults
	if c.PhaseEmbarkDays == 0 {
		c.PhaseEmbarkDays = 7
	}
	if c.PhaseEstablishDays == 0 {
		c.PhaseEstablishDays = 30
	}
	if c.PhaseExpandDays == 0 {
		c.PhaseExpandDays = 100
	}

	// Task queue defaults
	if c.MaxQueuedTasks == 0 {
		c.MaxQueuedTasks = 20
	}
	if c.TaskTimeoutSeconds == 0 {
		c.TaskTimeoutSeconds = 300 // 5 minutes
	}

	// Persistence defaults
	if c.PersistencePath == "" {
		c.PersistencePath = "saves"
	}
	if c.AutoSaveIntervalSec == 0 {
		c.AutoSaveIntervalSec = 300 // 5 minutes
	}

	// Spatial reasoning defaults
	if c.RoomMinSize == 0 {
		c.RoomMinSize = 9 // 3x3 minimum
	}
	if c.RoomMaxSize == 0 {
		c.RoomMaxSize = 400 // 20x20 maximum
	}

	// Dynamic context defaults
	if len(c.ContextAlwaysInclude) == 0 {
		c.ContextAlwaysInclude = []string{"dwarf_count", "alerts", "phase"}
	}
	if c.ContextMaxSizeBytes == 0 {
		c.ContextMaxSizeBytes = 250 * 1024 // 250 KB hard limit
	}
}
