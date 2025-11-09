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

// LocalLLMProvider implements Provider for local LM Studio inference
type LocalLLMProvider struct {
	endpoint          string
	model             string
	client            *http.Client
	systemPromptCache string
	timeout           time.Duration
	maxRetries        int
}

// NewLocalLLMProvider creates a new local LLM provider
func NewLocalLLMProvider(endpoint, model string, timeoutMS int) *LocalLLMProvider {
	return &LocalLLMProvider{
		endpoint:   endpoint,
		model:      model,
		client:     &http.Client{},
		timeout:    time.Duration(timeoutMS) * time.Millisecond,
		maxRetries: 1,
	}
}

// GetProviderType returns the provider type identifier
func (p *LocalLLMProvider) GetProviderType() string {
	return "local"
}

// GetModelName returns the model identifier
func (p *LocalLLMProvider) GetModelName() string {
	return p.model
}

// SupportsStreaming indicates if provider can stream responses
func (p *LocalLLMProvider) SupportsStreaming() bool {
	return false // Streaming not yet implemented for local provider
}

// UpdateSystemPrompt caches arbiter instructions for reuse
func (p *LocalLLMProvider) UpdateSystemPrompt(prompt string) {
	p.systemPromptCache = prompt
}

// SendPrompt sends a prompt to local LM Studio and returns the response
func (p *LocalLLMProvider) SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error) {
	start := time.Now()

	// Use cached system prompt if available, otherwise use provided
	systemPrompt := p.systemPromptCache
	if systemPrompt == "" {
		systemPrompt = prompt.SystemPrompt
	}

	// Build OpenAI-compatible request
	reqBody := map[string]interface{}{
		"model": p.model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt.UserMessage},
		},
		"temperature": prompt.Temperature,
		"max_tokens":  prompt.MaxTokens,
	}

	reqJSON, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Send request with retry logic
	var resp *http.Response
	var lastErr error

	for attempt := 0; attempt <= p.maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(2 * time.Second) // Wait before retry
		}

		// Create request with timeout
		reqCtx, cancel := context.WithTimeout(ctx, p.timeout)
		defer cancel()

		req, err := http.NewRequestWithContext(reqCtx, "POST", p.endpoint+"/chat/completions", bytes.NewReader(reqJSON))
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)
			continue
		}

		req.Header.Set("Content-Type", "application/json")

		// Send request
		resp, err = p.client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("HTTP request failed: %w", err)
			continue
		}

		// Check status code
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("LM Studio error (status %d): %s", resp.StatusCode, string(body))
			continue
		}

		// Success
		break
	}

	if lastErr != nil {
		return nil, fmt.Errorf("all retry attempts failed: %w", lastErr)
	}

	// Parse response
	defer resp.Body.Close()
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Usage struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		} `json:"usage"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(result.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	latency := time.Since(start)

	return &Response{
		Text:             result.Choices[0].Message.Content,
		TokensPrompt:     result.Usage.PromptTokens,
		TokensCompletion: result.Usage.CompletionTokens,
		Latency:          latency,
		FinishReason:     "stop",
	}, nil
}

// HealthCheck verifies connectivity to LM Studio
func (p *LocalLLMProvider) HealthCheck() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", p.endpoint+"/models", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}

	return nil
}
