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
	ViewportActiveZMargin      uint16 `yaml:"viewport_active_z_margin"`       // Z-levels above/below active
	ViewportHazardMarginTiles  uint16 `yaml:"viewport_hazard_margin_tiles"`   // Tile margin around hazards
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
}
