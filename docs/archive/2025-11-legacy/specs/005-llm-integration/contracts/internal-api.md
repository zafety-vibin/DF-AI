# Internal API Contracts: LLM Integration

**Package**: `internal/modifications`, `internal/context`, `internal/llm`, `internal/commands`, `internal/autonomous`
**Feature**: 005-llm-integration
**Date**: 2025-11-07

## Package: internal/modifications

### ModificationOverlay

```go
func NewModificationOverlay(bounds Bounds) *ModificationOverlay
```

Creates modification tracker. Bounds from map dimensions.

---

```go
func (m *ModificationOverlay) DetectModifications(tiles []protocol.TileState)
```

Scans TILE_UPDATE tiles, compares against first-seen state, detects modifications. Adds ModificationInfo for changed tiles.

---

```go
func (m *ModificationOverlay) GetBounds() Region
```

Returns min/max XYZ of all modifications. Used for viewport active area.

---

```go
func (m *ModificationOverlay) GetCount() int
```

Returns total modification count. For metrics/logging.

---

```go
func (m *ModificationOverlay) ExtractChambers() []Chamber
```

Runs 6-connected flood-fill on modifications. Returns connected chambers with bounds and tile counts.

**Performance**: O(n) where n = modification count. Target: <200ms for 1000 modifications.

---

```go
func (m *ModificationOverlay) GetModificationsInRegion(region Region, since time.Time) []ModificationInfo
```

Filters modifications within bounding box and after timestamp. Used for feedback generation (what changed after command?).

---

```go
func (m *ModificationOverlay) LinkCommandToModifications(commandID uint32, region Region, timestamp time.Time)
```

Updates CommandID field for modifications in region detected after timestamp. Links AI decisions to outcomes.

---

## Package: internal/context

### Context Assembly

```go
func AssembleContext(level ViewportLevel, mods *modifications.ModificationOverlay,
                      hazards *hazards.HazardManager, entities []protocol.EntityInfo) (*ViewportContext, error)
```

Generates fort state context at specified detail level (0-3). Enforces size budgets.

**Returns**: ViewportContext with chambers, hazards, dwarves, and size tracking.

---

```go
func GenerateTextOverview(mods *modifications.ModificationOverlay, dwarfCount int) string
```

Creates Level 0 text summary. Target: <500 bytes.

Example output: "Embark year 1, surface Z=95-98, 7 dwarves, 3 chambers (48 tiles), food adequate, drink low"

---

```go
func ExtractChamberFeatures(chambers []Chamber) []ChamberFeature
```

Converts flood-filled chambers to natural language descriptions.

Example: `{"bounds": "(10,20,95) to (15,25,96)", "dimensions": "5×8×2", "description": "entrance hall, 48 tiles"}`

---

### Query API

```go
func ExecuteQuery(queryType QueryType, params map[string]interface{}) (*QueryResponse, error)
```

Processes AI data requests. Returns filtered overlays or computed results.

**Supported queries**:
- `topology_slice`: Returns bool array or ASCII for specified region
- `hazard_list`: Filters hazard overlay to Z-range
- `pathfind`: Simple flood-fill pathfinding (not A*)
- `modifications_in_region`: Lists tile changes in area

---

## Package: internal/llm

### Provider Interface

```go
type Provider interface {
    SendPrompt(ctx context.Context, prompt *LLMPrompt) (*LLMResponse, error)
    GetModelName() string
    GetProviderType() string  // "claude" or "openai_compatible"
}
```

---

### ClaudeProvider

```go
func NewClaudeProvider(apiKey string, model string) *ClaudeProvider
```

Initializes Claude API client. Model: "claude-sonnet-4", "claude-opus-4", etc.

---

```go
func (c *ClaudeProvider) SendPrompt(ctx context.Context, prompt *LLMPrompt) (*LLMResponse, error)
```

HTTP POST to `https://api.anthropic.com/v1/messages`. Returns parsed response with token counts.

**Timeout**: 30 seconds (configurable).

---

### OpenAIProvider

```go
func NewOpenAIProvider(endpoint string, apiKey string, model string) *OpenAIProvider
```

Initializes OpenAI-compatible client. Endpoint: "https://api.openai.com/v1", "http://localhost:8000", etc.

---

```go
func (o *OpenAIProvider) SendPrompt(ctx context.Context, prompt *LLMPrompt) (*LLMResponse, error)
```

HTTP POST to `/v1/chat/completions`. Compatible with OpenAI, vLLM, llama.cpp, Ollama.

---

### Parser

```go
func ParseResponse(rawText string) (reasoning string, commands []CommandSpec, error)
```

Attempts JSON parse first, falls back to regex extraction.

**JSON format expected**:
```json
{
  "reasoning": "The fort needs bedrooms for dwarves to sleep...",
  "commands": [
    {"type": "dig", "region": {"x1": 10, "y1": 20, "z1": 95, "x2": 15, "y2": 25, "z2": 95}}
  ]
}
```

**Regex patterns** (fallback):
- Dig: `dig.*\((\d+),(\d+),(\d+)\)\s*to\s*\((\d+),(\d+),(\d+)\)`
- Build: `build.*wall.*at\s*\((\d+),(\d+),(\d+)\)`

---

## Package: internal/commands

### Protocol Messages

```go
type CommandMessage struct {
    ID      uint32
    Type    CommandType
    Payload []byte  // Type-specific data
}

func (c *CommandMessage) Serialize() ([]byte, error)
func DeserializeCommand(data []byte) (*CommandMessage, error)
```

Implements binary protocol message 0x09.

---

```go
type CommandAckMessage struct {
    CommandID uint32
    Status    AckStatus
    ErrorMsg  string
}

func (a *CommandAckMessage) Serialize() ([]byte, error)
func DeserializeCommandAck(data []byte) (*CommandAckMessage, error)
```

Implements binary protocol message 0x0A.

---

### Executor

```go
func SendCommand(cmd *Command, conn *protocol.Connection) error
```

Serializes Command to COMMAND message, sends to DFHack, tracks in pending map.

---

```go
func (e *CommandExecutor) WaitForAck(commandID uint32, timeout time.Duration) (*CommandAcknowledgment, error)
```

Blocks until COMMAND_ACK received for specified ID or timeout (default 5s).

---

### Tracker

```go
func (t *CommandTracker) TrackCommand(cmd *Command)
func (t *CommandTracker) HandleAck(ack *CommandAcknowledgment)
func (t *CommandTracker) GetPendingCommands() []*PendingCommand
```

Maps command ID → execution state. Used for feedback generation.

---

### Feedback Generator

```go
func GenerateFeedback(cmd *Command, mods *modifications.ModificationOverlay,
                       hazards *hazards.HazardManager, ack *CommandAcknowledgment) string
```

Analyzes command outcome. Returns natural language feedback for AI.

**Outcome detection**:
- Count modifications in region after command timestamp
- Check for hazards revealed (water/aquifer appeared)
- Compare expected vs actual tile counts

**Output examples**: See R12 in research.md

---

## Package: internal/autonomous

### Decision Loop

```go
func (loop *AutonomousLoop) Start(ctx context.Context) error
```

Main autonomous cycle. Runs every 100 seconds or on-demand trigger.

**Cycle steps**:
1. Assemble context
2. Send to LLM
3. Parse response
4. Execute commands
5. Wait for outcome
6. Generate feedback
7. Update history
8. Repeat

---

```go
func (loop *AutonomousLoop) TriggerImmediate()
```

Signals immediate update (AI request or external trigger). Interrupts timer.

---

```go
func (loop *AutonomousLoop) GetHistory() []Turn
```

Returns recent conversation turns for debugging/analysis.

---

## Performance Contracts

| Operation | Target Latency | Notes |
|-----------|----------------|-------|
| ModificationOverlay.DetectModifications | <10ms | For 100 tiles |
| ExtractChambers | <200ms | For 1000 modifications |
| AssembleContext Level 0 | <100ms | Text generation |
| AssembleContext Level 1 | <500ms | Chamber extraction + filtering |
| AssembleContext Level 2 | <2s | Cavern data aggregation |
| AssembleContext Level 3 | <5s | Full topology compression |
| LLM API call (Claude) | <10s | Network + model latency |
| LLM API call (local) | <5s | Depends on hardware |
| SendCommand to DFHack | <50ms | TCP send |
| WaitForAck | <1s | Command execution + ACK send |
| GenerateFeedback | <100ms | Analyze modifications |
| Full autonomous cycle | ~100s | Mostly LLM wait time |

---

## Thread Safety

All operations concurrent-safe:
- ModificationOverlay: sync.RWMutex (many readers, rare writes)
- CommandTracker: sync.Mutex (serialize ID generation and ACK handling)
- Context assembly: Read-only from overlays (no writes)
- LLM calls: Independent per turn (no shared state)
- Autonomous loop: Single goroutine (no concurrent turns)
