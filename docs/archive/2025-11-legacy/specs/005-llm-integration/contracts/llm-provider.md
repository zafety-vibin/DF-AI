# LLM Provider Contract: Provider Interface Specification

**Package**: `internal/llm`
**Feature**: 005-llm-integration
**Date**: 2025-11-07

## Provider Interface

All LLM providers MUST implement this interface for modularity:

```go
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
```

---

## Prompt Structure

```go
type Prompt struct {
    SystemPrompt string   // Role and instructions (constant per session)
    UserMessage  string   // Current fort context + query
    History      []Message // Previous conversation (for multi-turn)
    MaxTokens    int      // Response length limit (default 4096)
    Temperature  float64  // Randomness (0.0-1.0, default 0.7)
}

type Message struct {
    Role    string  // "user", "assistant", "system"
    Content string
}
```

---

## Response Structure

```go
type Response struct {
    Text             string        // Full response text
    TokensPrompt     int           // Input tokens consumed
    TokensCompletion int           // Output tokens generated
    FinishReason     string        // "stop", "length", "tool_use", "error"
    Latency          time.Duration // API call duration
    ProviderMetadata map[string]interface{}  // Provider-specific data
}
```

---

## Claude Provider Implementation

### Configuration

```go
type ClaudeConfig struct {
    APIKey    string  // Anthropic API key (from config file or env)
    Model     string  // "claude-sonnet-4", "claude-opus-4", etc.
    BaseURL   string  // Default: "https://api.anthropic.com" (override for testing)
    Timeout   time.Duration  // Default: 30 seconds
}

func NewClaudeProvider(cfg ClaudeConfig) *ClaudeProvider
```

### API Request Format

**Endpoint**: `POST {BaseURL}/v1/messages`

**Headers**:
```
anthropic-version: 2023-06-01
x-api-key: {APIKey}
content-type: application/json
```

**Request Body**:
```json
{
  "model": "claude-sonnet-4",
  "max_tokens": 4096,
  "system": "You are a Dwarf Fortress AI...",
  "messages": [
    {"role": "user", "content": "{fort context JSON}"},
    {"role": "assistant", "content": "{previous AI response}"},
    {"role": "user", "content": "{feedback from outcome}"}
  ],
  "temperature": 0.7
}
```

**Response Body**:
```json
{
  "id": "msg_01...",
  "type": "message",
  "role": "assistant",
  "content": [
    {"type": "text", "text": "The fort is in early embark stage..."}
  ],
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 1250,
    "output_tokens": 380
  }
}
```

### Error Handling

**HTTP Status Codes**:
- 200: Success
- 400: Invalid request (malformed JSON, missing fields)
- 401: Invalid API key
- 429: Rate limit exceeded (retry with backoff)
- 500: Server error (retry once)

**Implementation**:
```go
func (c *ClaudeProvider) SendPrompt(ctx context.Context, prompt *Prompt) (*Response, error) {
    // Build request
    reqBody := map[string]interface{}{
        "model":       c.model,
        "max_tokens":  prompt.MaxTokens,
        "system":      prompt.SystemPrompt,
        "messages":    formatMessages(prompt),
        "temperature": prompt.Temperature,
    }

    // HTTP POST
    req, _ := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/v1/messages", jsonBody)
    req.Header.Set("anthropic-version", "2023-06-01")
    req.Header.Set("x-api-key", c.apiKey)
    req.Header.Set("content-type", "application/json")

    // Send with timeout
    resp, err := c.client.Do(req)
    if err != nil {
        return nil, fmt.Errorf("API call failed: %w", err)
    }
    defer resp.Body.Close()

    // Parse response
    var apiResp ClaudeResponse
    json.NewDecoder(resp.Body).Decode(&apiResp)

    // Extract text from content array
    text := ""
    for _, block := range apiResp.Content {
        if block.Type == "text" {
            text += block.Text
        }
    }

    return &Response{
        Text:             text,
        TokensPrompt:     apiResp.Usage.InputTokens,
        TokensCompletion: apiResp.Usage.OutputTokens,
        FinishReason:     apiResp.StopReason,
        Latency:          latency,
    }, nil
}
```

---

## OpenAI-Compatible Provider Implementation

### Configuration

```go
type OpenAIConfig struct {
    Endpoint  string  // "https://api.openai.com/v1", "http://localhost:8000", etc.
    APIKey    string  // OpenAI API key or local server auth token (may be empty for local)
    Model     string  // "gpt-4", "meta-llama/Llama-3-70b-hf", etc.
    Timeout   time.Duration
}

func NewOpenAIProvider(cfg OpenAIConfig) *OpenAIProvider
```

### API Request Format

**Endpoint**: `POST {Endpoint}/chat/completions`

**Headers**:
```
Authorization: Bearer {APIKey}  (omit for local servers without auth)
Content-Type: application/json
```

**Request Body**:
```json
{
  "model": "gpt-4",
  "messages": [
    {"role": "system", "content": "You are a Dwarf Fortress AI..."},
    {"role": "user", "content": "{fort context JSON}"},
    {"role": "assistant", "content": "{previous AI response}"},
    {"role": "user", "content": "{feedback}"}
  ],
  "max_tokens": 4096,
  "temperature": 0.7
}
```

**Response Body**:
```json
{
  "id": "chatcmpl-...",
  "object": "chat.completion",
  "model": "gpt-4",
  "choices": [
    {
      "index": 0,
      "message": {
        "role": "assistant",
        "content": "The fort is in early embark stage..."
      },
      "finish_reason": "stop"
    }
  ],
  "usage": {
    "prompt_tokens": 1250,
    "completion_tokens": 380,
    "total_tokens": 1630
  }
}
```

### Local LLM Compatibility

Tested with:
- **vLLM**: `http://localhost:8000` - Fast inference server for local models
- **llama.cpp server**: `http://localhost:8080` - Lightweight C++ server
- **Ollama**: `http://localhost:11434/v1` - User-friendly model runner
- **LM Studio**: `http://localhost:1234/v1` - GUI model server

All use OpenAI-compatible format.

---

## Multi-Model Pipeline

### Configuration

```go
type MultiModelConfig struct {
    ExtractorProvider Provider  // Fast local model for feature extraction
    PlannerProvider   Provider  // Capable model for decision-making
}
```

### Execution Flow

```
Raw overlays (modifications, hazards, entities)
    ↓
Assemble full context (Level 2 or 3)
    ↓
Send to Extractor with prompt: "Extract chambers, passages, hazards. Output JSON features only."
    ↓
Extractor returns structured features (~5 KB)
    ↓
Send features to Planner with prompt: "Given these fort features, suggest next action."
    ↓
Planner returns command proposal
    ↓
Parse and execute command
```

**Benefits**:
- Extractor runs locally (free, fast, <2s)
- Planner receives clean input (better reasoning)
- Total cost lower (planner sees 5 KB not 50 KB)

**Example extractors**: Qwen-2.5-7B, Mistral-7B, Llama-3-8B
**Example planners**: Claude Sonnet 4, GPT-4, fine-tuned Llama-70B

---

## Provider Selection Logic

```go
type ProviderFactory struct {
    config *config.Config
}

func (f *ProviderFactory) CreateProvider() (Provider, error) {
    switch f.config.LLMProviderType {
    case "claude":
        return NewClaudeProvider(ClaudeConfig{
            APIKey: f.config.ClaudeAPIKey,
            Model: f.config.ClaudeModel,
        }), nil

    case "openai_compatible":
        return NewOpenAIProvider(OpenAIConfig{
            Endpoint: f.config.LLMEndpoint,
            APIKey: f.config.LLMAPIKey,
            Model: f.config.LLMModel,
        }), nil

    case "multi_model":
        extractor := NewOpenAIProvider(...)  // Local fast model
        planner := NewClaudeProvider(...)    // Cloud capable model
        return NewMultiModelProvider(extractor, planner), nil

    default:
        return nil, fmt.Errorf("unknown provider type: %s", f.config.LLMProviderType)
    }
}
```

---

## Logging Contract

All LLM interactions MUST log to JSONL format for fine-tuning:

```json
{
  "timestamp": "2025-11-07T01:30:00Z",
  "turn_number": 5,
  "provider": "claude",
  "model": "claude-sonnet-4",
  "prompt": {
    "system": "You are a Dwarf Fortress AI...",
    "context": {...},
    "history": [...]
  },
  "response": {
    "text": "The fort needs bedrooms...",
    "reasoning": "Dwarves are sleeping in the dining hall...",
    "commands": [{"type": "dig", "region": {...}}]
  },
  "tokens": {
    "prompt": 1250,
    "completion": 380,
    "total": 1630
  },
  "latency_ms": 3420,
  "outcome": {
    "command_id": 1005,
    "success": true,
    "tiles_modified": 15,
    "hazards_encountered": []
  }
}
```

**Log file**: `logs/llm-interactions.jsonl` (append-only)

**Usage**: Offline analysis, fine-tuning dataset preparation, reward signal calculation
