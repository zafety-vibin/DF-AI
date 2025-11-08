package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ClaudeProvider implements the Provider interface for Claude API
type ClaudeProvider struct {
	apiKey  string
	model   string
	baseURL string
	client  *http.Client
}

// ClaudeConfig holds configuration for Claude API
type ClaudeConfig struct {
	APIKey  string        // Anthropic API key (from config file or env)
	Model   string        // "claude-sonnet-4", "claude-opus-4", etc.
	BaseURL string        // Default: "https://api.anthropic.com" (override for testing)
	Timeout time.Duration // Default: 30 seconds
}

// claudeRequest matches Claude API request format
type claudeRequest struct {
	Model       string          `json:"model"`
	MaxTokens   int             `json:"max_tokens"`
	System      string          `json:"system,omitempty"`
	Messages    []claudeMessage `json:"messages"`
	Temperature float64         `json:"temperature"`
}

// claudeMessage represents a message in Claude API format
type claudeMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// claudeResponse matches Claude API response format
type claudeResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	StopReason string `json:"stop_reason"`
	Usage      struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

// claudeError represents Claude API error response
type claudeError struct {
	Type  string `json:"type"`
	Error struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// NewClaudeProvider creates a new Claude API provider
func NewClaudeProvider(cfg ClaudeConfig) *ClaudeProvider {
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://api.anthropic.com"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &ClaudeProvider{
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		baseURL: cfg.BaseURL,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// SendPrompt sends a prompt to Claude API and returns the response
func (c *ClaudeProvider) SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error) {
	start := time.Now()

	// Build messages array from history and current message
	messages := make([]claudeMessage, 0, len(prompt.History)+1)
	for _, msg := range prompt.History {
		messages = append(messages, claudeMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}
	// Add current user message
	messages = append(messages, claudeMessage{
		Role:    "user",
		Content: prompt.UserMessage,
	})

	// Set defaults
	maxTokens := prompt.MaxTokens
	if maxTokens == 0 {
		maxTokens = 4096
	}
	temperature := prompt.Temperature
	if temperature == 0 {
		temperature = 0.7
	}

	// Build request body
	reqBody := claudeRequest{
		Model:       c.model,
		MaxTokens:   maxTokens,
		System:      prompt.SystemPrompt,
		Messages:    messages,
		Temperature: temperature,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("content-type", "application/json")

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API call failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	// Handle error responses
	if resp.StatusCode != http.StatusOK {
		var apiErr claudeError
		if err := json.Unmarshal(body, &apiErr); err != nil {
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
		}

		switch resp.StatusCode {
		case 401:
			return nil, fmt.Errorf("authentication failed: invalid API key")
		case 429:
			return nil, fmt.Errorf("rate limit exceeded: %s", apiErr.Error.Message)
		case 500, 502, 503, 504:
			return nil, fmt.Errorf("server error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
		default:
			return nil, fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, apiErr.Error.Message)
		}
	}

	// Parse successful response
	var apiResp claudeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text from content array
	text := ""
	for _, block := range apiResp.Content {
		if block.Type == "text" {
			text += block.Text
		}
	}

	latency := time.Since(start)

	return &Response{
		Text:             text,
		TokensPrompt:     apiResp.Usage.InputTokens,
		TokensCompletion: apiResp.Usage.OutputTokens,
		FinishReason:     apiResp.StopReason,
		Latency:          latency,
		ProviderMetadata: map[string]interface{}{
			"id":   apiResp.ID,
			"type": apiResp.Type,
		},
	}, nil
}

// GetModelName returns the model identifier
func (c *ClaudeProvider) GetModelName() string {
	return c.model
}

// GetProviderType returns "claude"
func (c *ClaudeProvider) GetProviderType() string {
	return "claude"
}

// SupportsStreaming returns false (streaming not yet implemented)
func (c *ClaudeProvider) SupportsStreaming() bool {
	return false
}
