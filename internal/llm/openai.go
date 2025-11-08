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

// OpenAIProvider implements the Provider interface for OpenAI-compatible APIs
// Supports OpenAI API, vLLM, llama.cpp, Ollama, LM Studio, etc.
type OpenAIProvider struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

// OpenAIConfig holds configuration for OpenAI-compatible providers
type OpenAIConfig struct {
	Endpoint string        // "https://api.openai.com/v1", "http://localhost:8000", etc.
	APIKey   string        // OpenAI API key or local server auth token (may be empty for local)
	Model    string        // "gpt-4", "meta-llama/Llama-3-70b-hf", etc.
	Timeout  time.Duration // Default: 30 seconds
}

// openaiRequest matches OpenAI API request format
type openaiRequest struct {
	Model       string          `json:"model"`
	Messages    []openaiMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature"`
}

// openaiMessage represents a message in OpenAI API format
type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// openaiResponse matches OpenAI API response format
type openaiResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
}

// openaiError represents OpenAI API error response
type openaiError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// NewOpenAIProvider creates a new OpenAI-compatible provider
func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider {
	if cfg.Timeout == 0 {
		cfg.Timeout = 30 * time.Second
	}

	return &OpenAIProvider{
		endpoint: cfg.Endpoint,
		apiKey:   cfg.APIKey,
		model:    cfg.Model,
		client: &http.Client{
			Timeout: cfg.Timeout,
		},
	}
}

// SendPrompt sends a prompt to OpenAI-compatible API and returns the response
func (o *OpenAIProvider) SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error) {
	start := time.Now()

	// Build messages array from system prompt, history, and current message
	messages := make([]openaiMessage, 0, len(prompt.History)+2)

	// Add system prompt if provided
	if prompt.SystemPrompt != "" {
		messages = append(messages, openaiMessage{
			Role:    "system",
			Content: prompt.SystemPrompt,
		})
	}

	// Add history
	for _, msg := range prompt.History {
		messages = append(messages, openaiMessage{
			Role:    msg.Role,
			Content: msg.Content,
		})
	}

	// Add current user message
	messages = append(messages, openaiMessage{
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
	reqBody := openaiRequest{
		Model:       o.model,
		Messages:    messages,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}

	// Marshal to JSON
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Create HTTP request
	url := o.endpoint
	if url[len(url)-1] != '/' {
		url += "/"
	}
	url += "chat/completions"

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	if o.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+o.apiKey)
	}

	// Send request
	resp, err := o.client.Do(req)
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
		var apiErr openaiError
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
	var apiResp openaiResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Extract text from first choice
	if len(apiResp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	text := apiResp.Choices[0].Message.Content
	finishReason := apiResp.Choices[0].FinishReason
	latency := time.Since(start)

	return &Response{
		Text:             text,
		TokensPrompt:     apiResp.Usage.PromptTokens,
		TokensCompletion: apiResp.Usage.CompletionTokens,
		FinishReason:     finishReason,
		Latency:          latency,
		ProviderMetadata: map[string]interface{}{
			"id":     apiResp.ID,
			"object": apiResp.Object,
			"model":  apiResp.Model,
		},
	}, nil
}

// GetModelName returns the model identifier
func (o *OpenAIProvider) GetModelName() string {
	return o.model
}

// GetProviderType returns "openai_compatible"
func (o *OpenAIProvider) GetProviderType() string {
	return "openai_compatible"
}

// SupportsStreaming returns false (streaming not yet implemented)
func (o *OpenAIProvider) SupportsStreaming() bool {
	return false
}
