package config

import (
	"context"
	"fmt"
	"sync"

	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/fsnotify/fsnotify"
)

// ConfigManager manages configuration lifecycle including hot-reload
type ConfigManager struct {
	config        *Config
	mu            sync.RWMutex
	watcher       *fsnotify.Watcher
	configPath    string
	logger        *logging.Logger
	reloadCh      chan *Config
	reloadCounter uint64
	counterMu     sync.Mutex
}

// NewConfigManager creates a new config manager and loads initial configuration
func NewConfigManager(path string, logger *logging.Logger) (*ConfigManager, error) {
	// Load initial config
	cfg, err := Load(path)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	// Create file watcher
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	// Add config file to watch list
	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return nil, fmt.Errorf("failed to watch config file: %w", err)
	}

	cm := &ConfigManager{
		config:     cfg,
		watcher:    watcher,
		configPath: path,
		logger:     logger,
		reloadCh:   make(chan *Config, 1),
	}

	logger.Info("config loaded",
		logging.Field{Key: "path", Value: path},
		logging.Field{Key: "listen_port", Value: cfg.ListenPort},
		logging.Field{Key: "http_port", Value: cfg.HttpPort},
		logging.Field{Key: "log_level", Value: cfg.LogLevel})

	return cm, nil
}

// Get returns the current configuration (thread-safe read)
func (cm *ConfigManager) Get() *Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.config
}

// Watch starts watching for config file changes
func (cm *ConfigManager) Watch(ctx context.Context) {
	go func() {
		for {
			select {
			case <-ctx.Done():
				cm.watcher.Close()
				close(cm.reloadCh)
				return

			case event, ok := <-cm.watcher.Events:
				if !ok {
					return
				}

				// Handle different editor write patterns:
				// - Write: Direct file modification
				// - Remove/Rename: Safe write (VS Code, many editors)
				shouldReload := false

				if event.Op&fsnotify.Write == fsnotify.Write {
					shouldReload = true
					cm.logger.Info("config file modified (write), reloading",
						logging.Field{Key: "file", Value: event.Name})
				}

				// Many editors use "safe write": write temp, delete original, rename temp
				// This triggers Remove or Rename events
				if event.Op&fsnotify.Remove == fsnotify.Remove || event.Op&fsnotify.Rename == fsnotify.Rename {
					shouldReload = true
					cm.logger.Info("config file modified (safe write), reloading",
						logging.Field{Key: "file", Value: event.Name},
						logging.Field{Key: "operation", Value: event.Op.String()})

					// Re-add watch since file was removed/renamed and recreated
					// Ignore errors - if file doesn't exist yet, next event will trigger
					cm.watcher.Remove(cm.configPath)
					cm.watcher.Add(cm.configPath)
				}

				if shouldReload {
					// Load new config
					newCfg, err := Load(cm.configPath)
					if err != nil {
						cm.logger.Error("failed to reload config, keeping current", err,
							logging.Field{Key: "path", Value: cm.configPath})
						continue
					}

					// Validate new config
					if err := cm.validate(newCfg); err != nil {
						cm.logger.Error("config validation failed, keeping current", err)
						continue
					}

					// Get old config for change logging
					oldCfg := cm.Get()

					// Apply reload
					cm.reload(newCfg)

					// Log changes
					cm.logChanges(oldCfg, newCfg)
				}

			case err, ok := <-cm.watcher.Errors:
				if !ok {
					return
				}
				cm.logger.Error("config watcher error", err)
			}
		}
	}()
}

// reload atomically swaps to new configuration
func (cm *ConfigManager) reload(newConfig *Config) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.config = newConfig

	// Increment reload counter
	cm.counterMu.Lock()
	cm.reloadCounter++
	cm.counterMu.Unlock()
}

// GetConfigReloads returns the number of successful config reloads
func (cm *ConfigManager) GetConfigReloads() uint64 {
	cm.counterMu.Lock()
	defer cm.counterMu.Unlock()
	return cm.reloadCounter
}

// validate checks if config values are valid
func (cm *ConfigManager) validate(cfg *Config) error {
	// Validate port ranges (non-privileged ports)
	if cfg.ListenPort < 1024 || cfg.ListenPort > 65535 {
		return fmt.Errorf("listen_port must be 1024-65535, got %d", cfg.ListenPort)
	}
	if cfg.HttpPort < 1024 || cfg.HttpPort > 65535 {
		return fmt.Errorf("http_port must be 1024-65535, got %d", cfg.HttpPort)
	}

	// Ports must be different
	if cfg.HttpPort == cfg.ListenPort {
		return fmt.Errorf("http_port and listen_port must be different (both set to %d)", cfg.HttpPort)
	}

	// Validate log level enum
	validLogLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLogLevels[cfg.LogLevel] {
		return fmt.Errorf("log_level must be debug|info|warn|error, got %s", cfg.LogLevel)
	}

	// Validate log format enum
	validLogFormats := map[string]bool{"json": true, "text": true}
	if !validLogFormats[cfg.LogFormat] {
		return fmt.Errorf("log_format must be json|text, got %s", cfg.LogFormat)
	}

	// Validate heartbeat settings
	if cfg.HeartbeatInterval <= 0 {
		return fmt.Errorf("heartbeat_interval must be > 0, got %s", cfg.HeartbeatInterval)
	}
	if cfg.HeartbeatTimeout < cfg.HeartbeatInterval {
		return fmt.Errorf("heartbeat_timeout (%s) must be >= heartbeat_interval (%s)",
			cfg.HeartbeatTimeout, cfg.HeartbeatInterval)
	}

	return nil
}

// logChanges logs configuration changes with old/new values
func (cm *ConfigManager) logChanges(old, new *Config) {
	changes := make(map[string]interface{})

	// Check for changed fields
	if old.ListenPort != new.ListenPort {
		changes["listen_port"] = map[string]interface{}{
			"old": old.ListenPort,
			"new": new.ListenPort,
		}
		cm.logger.Warn("listen_port changed - requires restart to take effect",
			logging.Field{Key: "old", Value: old.ListenPort},
			logging.Field{Key: "new", Value: new.ListenPort})
	}

	if old.HttpPort != new.HttpPort {
		changes["http_port"] = map[string]interface{}{
			"old": old.HttpPort,
			"new": new.HttpPort,
		}
		cm.logger.Warn("http_port changed - requires restart to take effect",
			logging.Field{Key: "old", Value: old.HttpPort},
			logging.Field{Key: "new", Value: new.HttpPort})
	}

	if old.EnableHttpApi != new.EnableHttpApi {
		changes["enable_http_api"] = map[string]interface{}{
			"old": old.EnableHttpApi,
			"new": new.EnableHttpApi,
		}
	}

	if old.LogLevel != new.LogLevel {
		changes["log_level"] = map[string]interface{}{
			"old": old.LogLevel,
			"new": new.LogLevel,
		}
	}

	if old.LogFormat != new.LogFormat {
		changes["log_format"] = map[string]interface{}{
			"old": old.LogFormat,
			"new": new.LogFormat,
		}
	}

	if old.HeartbeatInterval != new.HeartbeatInterval {
		changes["heartbeat_interval"] = map[string]interface{}{
			"old": old.HeartbeatInterval.String(),
			"new": new.HeartbeatInterval.String(),
		}
	}

	if old.HeartbeatTimeout != new.HeartbeatTimeout {
		changes["heartbeat_timeout"] = map[string]interface{}{
			"old": old.HeartbeatTimeout.String(),
			"new": new.HeartbeatTimeout.String(),
		}
	}

	if len(changes) > 0 {
		cm.logger.Info("config reloaded successfully",
			logging.Field{Key: "changes_count", Value: len(changes)},
			logging.Field{Key: "changes", Value: changes})
	} else {
		cm.logger.Info("config reloaded (no changes detected)")
	}
}
