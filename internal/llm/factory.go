package llm

import (
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/config"
)

// CreateProvider creates an LLM provider based on configuration
// Supports: claude, openai_compatible, multi_model
// Validates that required configuration is present (e.g., API keys)
func CreateProvider(cfg *config.Config) (Provider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	switch cfg.LLMProviderType {
	case "claude":
		return createClaudeProvider(cfg)
	case "openai_compatible":
		return createOpenAIProvider(cfg)
	case "multi_model":
		return createMultiModelProvider(cfg)
	default:
		return nil, fmt.Errorf("unknown LLM provider type: %s", cfg.LLMProviderType)
	}
}

// createClaudeProvider creates a Claude API provider with validation
func createClaudeProvider(cfg *config.Config) (Provider, error) {
	// Validate required configuration
	if cfg.ClaudeAPIKey == "" {
		return nil, fmt.Errorf("claude_api_key is required for Claude provider")
	}
	if cfg.ClaudeModel == "" {
		return nil, fmt.Errorf("claude_model is required for Claude provider")
	}

	claudeConfig := ClaudeConfig{
		APIKey:  cfg.ClaudeAPIKey,
		Model:   cfg.ClaudeModel,
		BaseURL: "https://api.anthropic.com",
		Timeout: cfg.LLMTimeoutSeconds,
	}

	provider := NewClaudeProvider(claudeConfig)
	return provider, nil
}

// createOpenAIProvider creates an OpenAI-compatible provider with validation
func createOpenAIProvider(cfg *config.Config) (Provider, error) {
	// Validate required configuration
	if cfg.LLMEndpoint == "" {
		return nil, fmt.Errorf("llm_endpoint is required for OpenAI-compatible provider")
	}
	if cfg.LLMModel == "" {
		return nil, fmt.Errorf("llm_model is required for OpenAI-compatible provider")
	}

	openaiConfig := OpenAIConfig{
		Endpoint: cfg.LLMEndpoint,
		APIKey:   cfg.LLMAPIKey, // Optional for some local models
		Model:    cfg.LLMModel,
		Timeout:  cfg.LLMTimeoutSeconds,
	}

	provider := NewOpenAIProvider(openaiConfig)
	return provider, nil
}

// createMultiModelProvider creates a pipeline with two providers (extractor + planner)
// Can combine any provider types (e.g., claude extractor + openai planner)
func createMultiModelProvider(cfg *config.Config) (Provider, error) {
	// For multi_model, we use Claude as extractor and local LLM as planner
	// Or you can configure both separately via config

	// Create extractor provider
	extractorConfig := &config.Config{
		LLMProviderType:   "claude",
		ClaudeAPIKey:      cfg.ClaudeAPIKey,
		ClaudeModel:       cfg.ClaudeModel,
		LLMTemperature:    0.3, // Low temp for consistent extraction
		LLMMaxTokens:      2048,
		LLMTimeoutSeconds: cfg.LLMTimeoutSeconds,
	}

	extractorProvider, err := createClaudeProvider(extractorConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create extractor provider: %w", err)
	}

	// Create planner provider
	plannerConfig := &config.Config{
		LLMProviderType:   "openai_compatible",
		LLMEndpoint:       cfg.LLMEndpoint,
		LLMModel:          cfg.LLMModel,
		LLMAPIKey:         cfg.LLMAPIKey,
		LLMTemperature:    cfg.LLMTemperature,
		LLMMaxTokens:      cfg.LLMMaxTokens,
		LLMTimeoutSeconds: cfg.LLMTimeoutSeconds,
	}

	plannerProvider, err := createOpenAIProvider(plannerConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create planner provider: %w", err)
	}

	// Create multi-model pipeline
	pipeline, err := NewMultiModelProvider(extractorProvider, plannerProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create multi-model pipeline: %w", err)
	}

	return pipeline, nil
}

// ValidateProviderConfig checks that a provider configuration is valid
// Returns error if critical fields are missing or invalid
func ValidateProviderConfig(cfg *config.Config) error {
	if cfg == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if cfg.LLMProviderType == "" {
		return fmt.Errorf("llm_provider_type must be set")
	}

	// Timeout must be positive
	if cfg.LLMTimeoutSeconds <= 0 {
		return fmt.Errorf("llm_timeout_seconds must be positive")
	}

	// Temperature must be in valid range
	if cfg.LLMTemperature < 0.0 || cfg.LLMTemperature > 1.0 {
		return fmt.Errorf("llm_temperature must be between 0.0 and 1.0")
	}

	// Max tokens must be positive
	if cfg.LLMMaxTokens <= 0 {
		return fmt.Errorf("llm_max_tokens must be positive")
	}

	// Context budget must be positive
	if cfg.ContextBudgetKB <= 0 {
		return fmt.Errorf("context_budget_kb must be positive")
	}

	// Viewport settings must be positive or zero
	if cfg.ViewportActiveZMargin < 0 {
		return fmt.Errorf("viewport_active_z_margin must be >= 0")
	}
	if cfg.ViewportHazardMarginTiles < 0 {
		return fmt.Errorf("viewport_hazard_margin_tiles must be >= 0")
	}

	// Provider-specific validation
	switch cfg.LLMProviderType {
	case "claude":
		if cfg.ClaudeAPIKey == "" {
			return fmt.Errorf("claude_api_key required for Claude provider")
		}
		if cfg.ClaudeModel == "" {
			return fmt.Errorf("claude_model required for Claude provider")
		}
	case "openai_compatible":
		if cfg.LLMEndpoint == "" {
			return fmt.Errorf("llm_endpoint required for OpenAI-compatible provider")
		}
		if cfg.LLMModel == "" {
			return fmt.Errorf("llm_model required for OpenAI-compatible provider")
		}
	case "multi_model":
		if cfg.ClaudeAPIKey == "" {
			return fmt.Errorf("claude_api_key required for multi_model (extractor)")
		}
		if cfg.ClaudeModel == "" {
			return fmt.Errorf("claude_model required for multi_model (extractor)")
		}
		if cfg.LLMEndpoint == "" {
			return fmt.Errorf("llm_endpoint required for multi_model (planner)")
		}
		if cfg.LLMModel == "" {
			return fmt.Errorf("llm_model required for multi_model (planner)")
		}
	default:
		return fmt.Errorf("unknown llm_provider_type: %s", cfg.LLMProviderType)
	}

	return nil
}

// DefaultProviderConfig returns a configuration with sensible defaults for development
func DefaultProviderConfig() *config.Config {
	timeout := time.Duration(60) * time.Second
	return &config.Config{
		LLMProviderType:               "claude",
		ClaudeModel:                   "claude-opus-4-1",
		LLMTemperature:                0.7,
		LLMMaxTokens:                  4096,
		LLMTimeoutSeconds:             timeout,
		ContextBudgetKB:               200,
		ContextUpdateFrequencySeconds: 100,
		ViewportActiveZMargin:         3,
		ViewportHazardMarginTiles:     5,
	}
}
