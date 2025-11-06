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
}
