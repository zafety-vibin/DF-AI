package llm

import "time"

// Message represents a single message in conversation history
type Message struct {
	Role    string // "user", "assistant", "system"
	Content string
}

// Prompt contains all information needed to send a request to an LLM
type Prompt struct {
	SystemPrompt string    // Role and instructions (constant per session)
	UserMessage  string    // Current fort context + query
	History      []Message // Previous conversation (for multi-turn)
	MaxTokens    int       // Response length limit (default 4096)
	Temperature  float64   // Randomness (0.0-1.0, default 0.7)
}

// Response contains the LLM's response and metadata
type Response struct {
	Text             string                 // Full response text
	TokensPrompt     int                    // Input tokens consumed
	TokensCompletion int                    // Output tokens generated
	FinishReason     string                 // "stop", "length", "tool_use", "error"
	Latency          time.Duration          // API call duration
	ProviderMetadata map[string]interface{} // Provider-specific data
}
