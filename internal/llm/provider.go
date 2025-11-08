package llm

import "context"

// Provider is the interface that all LLM providers must implement
type Provider interface {
	// SendPrompt sends prompt to LLM and returns response
	// Context for timeout/cancellation
	// Returns response with parsed data and token usage
	SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error)

	// GetModelName returns the model identifier (e.g., "claude-sonnet-4", "llama-3-70b")
	GetModelName() string

	// GetProviderType returns provider category ("claude", "openai_compatible")
	GetProviderType() string

	// SupportsStreaming indicates if provider can stream responses (future feature)
	SupportsStreaming() bool
}
