# API Contracts

**Feature**: 006-local-llm-agents

## Note

This feature does not expose new HTTP API endpoints or contracts. It extends the existing internal architecture with:

1. **Internal Go interfaces**:
   - `GoalAgent` interface (internal/agents/agent.go)
   - `Provider` interface extension (internal/llm/provider.go)
   - Graph data structures (internal/agents/graph.go)

2. **Existing HTTP endpoints** (unchanged):
   - `GET /health` - Health check
   - `POST /ai/save` - Manual save trigger
   - WebSocket `/ws` - Monitoring (if enabled)

3. **Binary protocol** (unchanged):
   - DFHack plugin ↔ Orchestrator communication via TCP
   - Protocol defined in internal/protocol/

## LM Studio Integration

**External HTTP API** (consumed by LocalLLMProvider):

- **Endpoint**: `POST http://localhost:1234/v1/chat/completions`
- **Protocol**: OpenAI-compatible API
- **Request**:
  ```json
  {
    "model": "qwen2.5-7b-instruct-q4_k_m",
    "messages": [
      {"role": "system", "content": "Arbiter instructions..."},
      {"role": "user", "content": "Graph JSON..."}
    ],
    "temperature": 0.7,
    "max_tokens": 500
  }
  ```
- **Response**:
  ```json
  {
    "id": "chatcmpl-...",
    "choices": [{
      "message": {
        "role": "assistant",
        "content": "Execution sequence JSON..."
      }
    }],
    "usage": {
      "prompt_tokens": 890,
      "completion_tokens": 150,
      "total_tokens": 1040
    }
  }
  ```

This is an **external dependency**, not a contract we define. LM Studio provides this API.

---

**Conclusion**: No new API contracts to generate for this feature. All interfaces are internal Go code.
