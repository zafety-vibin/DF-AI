# Research: LLM Integration with Modification Tracking and Command Execution

**Feature**: 005-llm-integration | **Date**: 2025-11-07

## R1: Modification Detection Strategy

**Decision**: Compare TILE_UPDATE tile states against cached baseline to detect player/AI modifications

**Rationale**:
- Cannot distinguish "natural cave" from "mined cave" using TileType alone
- Need baseline of initial embark state to know what changed
- TILE_UPDATE already provides before/after TileType - compare against baseline
- Cache natural baseline on FULL_STATE: `map[Coordinate]uint16` storing original TileType
- Modification detected when: current TileType != baseline TileType
- Modification type inferred from transition: Rock → Floor = DUG, Floor → Wall = BUILT_WALL

**Alternatives Considered**:
- **Track all DF designations directly**: Requires accessing df.designation API from Go (not available, DFHack-only)
- **Heuristic-only detection**: TileType ranges for "mined" vs "natural" (too unreliable without material data)
- **Player-reported modifications**: Ask player to mark changes (defeats autonomous AI purpose)

**Implementation**:
```go
type ModificationOverlay struct {
    baseline     map[Coordinate]uint16      // Natural state from FULL_STATE
    modifications map[Coordinate]ModificationInfo
    mu           sync.RWMutex
}

func (m *ModificationOverlay) DetectModifications(tiles []TileState) {
    for _, tile := range tiles {
        coord := Coordinate{tile.X, tile.Y, tile.Z}
        baselineType := m.baseline[coord]

        if tile.TileType != baselineType {
            // Modification detected - infer type from transition
            modType := inferModificationType(baselineType, tile.TileType)
            m.modifications[coord] = ModificationInfo{
                Action: modType,
                Timestamp: time.Now(),
                CommandID: nil, // Set later if linked to AI command
            }
        }
    }
}
```

**Loaded save handling**: For saves loaded mid-game, baseline = current state on first FULL_STATE. Only track new changes going forward.

---

## R2: Flood-Fill Chamber Extraction

**Decision**: 6-connected flood-fill to identify contiguous modified spaces as chambers

**Rationale**:
- Modified tiles form connected rooms/passages - flood-fill naturally groups them
- 6-connected (N/S/E/W/Up/Down, not diagonal) matches DF pathability
- Output: list of chambers with bounding box, tile count, entry/exit points
- Complexity: O(n) where n = modification count (~1000 typical), runs in ~50ms

**Alternatives Considered**:
- **Semantic classification**: Detect "bedroom" vs "workshop" by analyzing furniture (too complex, deferred to Feature 16)
- **Graph-based clustering**: DBSCAN or similar (overkill for simple connected components)
- **Manual annotation**: Require player to mark room types (defeats autonomous purpose)

**Implementation**:
```go
type Chamber struct {
    ID      uint32
    Bounds  Region  // Min/max XYZ
    Tiles   []Coordinate
    Connections []uint32  // Connected chamber IDs
}

func (m *ModificationOverlay) ExtractChambers() []Chamber {
    visited := make(map[Coordinate]bool)
    chambers := []Chamber{}

    for coord := range m.modifications {
        if visited[coord] { continue }

        // Flood-fill from this tile
        chamber := floodFill(coord, m.modifications, visited)
        chambers = append(chambers, chamber)
    }

    return chambers
}
```

**Natural language description**: "3 chambers: Bedroom area 5×8×2 at Z=95, Mining hall 10×3×1 at Z=96, Entrance passage 2×15×1 connecting surface to chambers"

---

## R3: Viewport Staging System

**Decision**: 4 detail levels with cumulative data and explicit size budgets

**Rationale**:
- Level 0 (500B): Text only - for quick summary without context bloat
- Level 1 (10KB): Active work area - sufficient for early game decisions
- Level 2 (50KB): Cavern exploration - when AI needs resource scouting
- Level 3 (200KB): Emergency full context - when AI explicitly requests

Each level includes all previous levels + additional data:
- L0 → L1: Add modification features + local hazards + dwarves
- L1 → L2: Add cavern Z-levels (10-90) with water/lava sources
- L2 → L3: Add complete topology RLE compressed + all overlays

**Alternatives Considered**:
- **Single adaptive context**: Dynamically adjust detail to fit budget (complex priority logic, harder to debug)
- **Fixed context**: Always send same data (wastes tokens, doesn't scale with fort growth)
- **Delta-only updates**: Only send changes since last turn (AI loses context over time, requires state management)

**Implementation**:
```go
type ViewportLevel int
const (
    Level0_Overview ViewportLevel = 0
    Level1_ActiveArea = 1
    Level2_DeepPlanning = 2
    Level3_FullContext = 3
)

func AssembleContext(level ViewportLevel, mods *ModificationOverlay,
                      hazards *HazardManager, entities *EntityData) ([]byte, error) {
    ctx := &ContextBuilder{}

    // Level 0: Text summary
    ctx.AddOverview(generateTextSummary(mods, entities))
    if level == Level0_Overview { return ctx.Build() }

    // Level 1: Active area features
    chambers := mods.ExtractChambers()
    ctx.AddChambers(chambers)
    ctx.AddHazardsInRegion(mods.GetBounds().Expand(10))
    ctx.AddDwarfPositions(entities.GetDwarves())
    if level == Level1_ActiveArea { return ctx.Build() }

    // Level 2: Cavern layers
    ctx.AddCavernData(hazards.GetOverlay("caverns"), ZRange{10, 90})
    ctx.AddWaterSources(hazards.GetOverlay("water"), ZRange{10, 90})
    if level == Level2_DeepPlanning { return ctx.Build() }

    // Level 3: Full compressed topology
    ctx.AddCompressedTopology(topologyOverlay.Compress(CompressionConfig{Mode: "full"}))
    return ctx.Build()
}
```

---

## R4: LLM Provider Interface

**Decision**: Modular Provider interface with Claude and OpenAI implementations

**Rationale**:
- Research requires testing multiple models (Claude Sonnet, local Llama, fine-tuned models)
- Provider interface allows swapping without changing calling code
- Claude API: Official SDK or direct HTTP (use HTTP for lighter dependency)
- OpenAI-compatible: Works with vLLM, llama.cpp server, Ollama
- Common interface: `SendPrompt(context, systemPrompt) → (response, tokens, error)`

**Alternatives Considered**:
- **Single provider (Claude only)**: Locks into paid API, prevents local experimentation
- **LangChain/LlamaIndex**: Heavy dependencies, complex abstractions, violates lean principle
- **Direct API calls scattered**: Not modular, hard to swap providers

**Implementation**:
```go
type Provider interface {
    SendPrompt(ctx context.Context, prompt string, systemPrompt string) (*Response, error)
    GetModelName() string
    SupportsStreaming() bool
}

type Response struct {
    Text              string
    TokensPrompt      int
    TokensCompletion  int
    FinishReason      string
    Latency           time.Duration
}

type ClaudeProvider struct {
    apiKey  string
    model   string
    baseURL string
    client  *http.Client
}

func (c *ClaudeProvider) SendPrompt(ctx context.Context, prompt string, system string) (*Response, error) {
    // Build request to Claude Messages API
    // POST /v1/messages with anthropic-version header
    // Parse response, extract usage.input_tokens, usage.output_tokens
}

type OpenAIProvider struct {
    endpoint string  // localhost:8000 or OpenAI API
    apiKey   string
    model    string
    client   *http.Client
}

func (o *OpenAIProvider) SendPrompt(ctx context.Context, prompt string, system string) (*Response, error) {
    // Build request to /v1/chat/completions
    // Works with OpenAI, vLLM, llama.cpp, Ollama
}
```

**Multi-model pipeline**: Chain providers - Feature extractor (local fast model) → Planner (Claude/fine-tuned)

---

## R5: Command Protocol Design

**Decision**: Add COMMAND (0x09) and COMMAND_ACK (0x0A) message types to existing binary protocol

**Rationale**:
- Current protocol is one-way: DFHack → Server
- Need bidirectional: Server → DFHack for AI commands
- Reuse existing serialization patterns (big-endian, length-prefixed)
- Command types: 0x01=DIG, 0x02=BUILD, 0x03=CANCEL
- Include command ID (uint32) for tracking acknowledgments

**Alternatives Considered**:
- **Separate command channel**: New TCP connection for commands (complexity, two connections to manage)
- **HTTP REST API from DFHack**: DFHack can't host HTTP server easily, polling is inefficient
- **Shared memory/IPC**: Platform-specific, violates cross-platform goal

**Message Formats**:

```
COMMAND (0x09):
[4: Length] [1: Version] [1: Type=0x09] [4: CommandID] [1: CommandType] [Payload]

CommandType 0x01 (DIG):
  [2: X1] [2: Y1] [2: Z1] [2: X2] [2: Y2] [2: Z2]  // Bounding box

CommandType 0x02 (BUILD):
  [2: X] [2: Y] [2: Z] [1: BuildType]  // Single tile construction

CommandType 0x03 (CANCEL):
  [2: X1] [2: Y1] [2: Z1] [2: X2] [2: Y2] [2: Z2]  // Cancel designations in region

COMMAND_ACK (0x0A):
[4: Length] [1: Version] [1: Type=0x0A] [4: CommandID] [1: Status] [2: ErrorCodeLength] [N: ErrorCode]

Status: 0x00=success, 0x01=partial_success, 0x02=failure
```

**DFHack execution**: Use `df::global::world->map.designation` to set dig flags, `df::global::world->jobs` for constructions

---

## R6: Baseline Detection for Loaded Saves

**Decision**: First FULL_STATE becomes baseline, regardless of when loaded - accept session-scoped tracking

**Rationale**:
- For new embark: FULL_STATE received immediately = true natural baseline ✅
- For loaded save: FULL_STATE shows current state (already modified) = imperfect baseline
- Accept limitation: Can only track NEW modifications after load
- Alternative would require shipping baseline snapshots with saves (violates simplicity)

**Mitigation for loaded saves**:
- Heuristic: Assume "natural-looking" tiletypes are baseline (floors with no walls nearby = natural)
- Or: First 10 seconds after load, assume no changes (let fort settle)
- Or: Accept that loaded saves start with "empty" modification overlay

**Decision**: Accept session-scoped modification tracking for research phase. Document in logs: "Loaded save - modification tracking starts from current state"

**Future Enhancement** (Feature 10-11 - Modification History):
- Persist modification overlay to `saves/{fortName}/modifications.json`
- Store baseline map (8 MB) + modifications map
- On load, restore both baseline and modification history
- Enables AI to maintain memory across sessions

**Alternative Approach** (AI inference):
- Even without persistence, AI can infer fort structure from loaded saves
- Constructed walls (specific TileType ranges) vs natural stone
- DF zones (bedrooms, dining halls) provide semantic labels
- AI Level 3 context includes full topology - can detect patterns
- Document: "For long-running experiments, start from fresh embark"

**Testing Validation**: Tested with loaded save - pre-existing mining not tracked, only new changes after connection detected ✅

---

## R7: Response Parsing Strategy

**Decision**: Flexible parser supporting both structured (JSON/XML) and unstructured (natural language) LLM responses

**Rationale**:
- Cannot guarantee LLM always outputs perfect JSON (especially smaller local models)
- Prompt LLM to use structured format, but fallback to regex extraction if needed
- Structured: `{"reasoning": "...", "commands": [{"type": "dig", "region": {...}}]}`
- Unstructured: Extract from text - "I recommend digging bedrooms at coordinates (10,20,95) to (15,25,95)"

**Alternatives Considered**:
- **Strict JSON-only**: Fails if LLM doesn't comply perfectly (local models vary in JSON adherence)
- **Natural language only**: Hard to parse coordinates reliably
- **Function calling API**: Claude supports tools, but limits provider flexibility (OpenAI function calling format differs)

**Implementation**:
```go
type CommandProposal struct {
    Reasoning   string
    Commands    []CommandSpec  // May be empty if no clear action
}

type CommandSpec struct {
    Type   string  // "dig", "build", "cancel"
    Region Region  // Coordinates
}

func ParseResponse(llmText string) (*CommandProposal, error) {
    // Try JSON parse first
    if proposal, err := parseJSON(llmText); err == nil {
        return proposal, nil
    }

    // Fallback: regex extraction
    // Look for patterns like "dig (10,20,95) to (15,25,95)"
    // Extract reasoning from first paragraph
    return parseNaturalLanguage(llmText)
}
```

---

## R8: Conversation History Management

**Decision**: Append-only conversation log with sliding window for context assembly

**Rationale**:
- AI needs previous decisions + outcomes to learn (can't have amnesia each turn)
- Full history grows unbounded (turn 100 = 100 context+response pairs = way over budget)
- Sliding window: Keep last N turns (e.g., 5 turns = ~50 KB history) + initial prompt
- Summarize older turns: "Previous sessions: dug 3 bedrooms, breached aquifer on turn 8, sealed with wall"

**Alternatives Considered**:
- **Stateless AI**: Each turn sees only current state (can't learn from past mistakes)
- **Full history**: Send all turns (context explodes, hits budget by turn 10)
- **External vector DB**: RAG retrieval of relevant past turns (complex, violates simplicity for MVP)

**Implementation**:
```go
type ConversationHistory struct {
    turns []Turn
    maxTurns int  // Default 5
}

type Turn struct {
    TurnNumber  int
    Context     string  // What AI saw
    Response    string  // What AI said
    Command     *Command  // What AI did
    Outcome     string  // What happened
    Timestamp   time.Time
}

func (h *ConversationHistory) GetContext() string {
    // Include last N turns
    // Summarize older turns if total > context budget
}
```

---

## R9: Claude API vs OpenAI Client Format

**Decision**: Use OpenAI-compatible format as lowest common denominator, adapt Claude API

**Rationale**:
- OpenAI format: `{model, messages: [{role, content}]}`
- Claude format: `{model, messages: [{role, content}], system: "..."}`
- Most local models (vLLM, llama.cpp, Ollama) support OpenAI format
- Claude API requires anthropic-specific headers and format
- Solution: Provider interface abstracts differences

**Claude API specifics**:
- Endpoint: `https://api.anthropic.com/v1/messages`
- Headers: `anthropic-version: 2023-06-01`, `x-api-key: ...`
- Request: `{model: "claude-sonnet-4", messages: [...], system: "...", max_tokens: 4096}`
- Response: `{id, content: [{type: "text", text: "..."}], usage: {input_tokens, output_tokens}}`

**OpenAI-compatible format**:
- Endpoint: Configurable (localhost:8000, api.openai.com, etc.)
- Headers: `Authorization: Bearer ...`
- Request: `{model: "gpt-4", messages: [{role: "system", content: "..."}, {role: "user", content: "..."}]}`
- Response: `{choices: [{message: {content: "..."}}], usage: {prompt_tokens, completion_tokens}}`

**Implementation**: Separate ClaudeProvider and OpenAIProvider, both implement Provider interface

---

## R10: Command ID Generation and Tracking

**Decision**: Monotonically increasing uint32 command IDs, reset per session

**Rationale**:
- Need unique ID to match COMMAND → COMMAND_ACK → modifications
- Sequence: Server generates ID → sends COMMAND → DFHack executes → sends ACK with same ID → Server maps ID to outcome
- uint32 allows 4 billion commands (way more than needed for research)
- Reset on server restart (not persisted - experimental system)

**Alternatives Considered**:
- **UUID**: 16 bytes (too large for binary protocol), overkill for session scope
- **Timestamp-based**: Collision risk if multiple commands in same millisecond
- **Random**: Collision probability, harder to debug

**Implementation**:
```go
type CommandTracker struct {
    nextID     uint32
    pending    map[uint32]*PendingCommand
    mu         sync.Mutex
}

type PendingCommand struct {
    ID          uint32
    SentAt      time.Time
    Type        CommandType
    Region      Region
    AckReceived bool
    Success     bool
    ErrorMsg    string
}

func (t *CommandTracker) GenerateID() uint32 {
    t.mu.Lock()
    defer t.mu.Unlock()
    id := t.nextID
    t.nextID++
    return id
}

func (t *CommandTracker) TrackCommand(cmd *Command) {
    t.pending[cmd.ID] = &PendingCommand{
        ID: cmd.ID,
        SentAt: time.Now(),
        Type: cmd.Type,
        Region: cmd.Region,
    }
}

func (t *CommandTracker) HandleAck(ack *CommandAck) {
    if pending, exists := t.pending[ack.CommandID]; exists {
        pending.AckReceived = true
        pending.Success = (ack.Status == 0x00)
        pending.ErrorMsg = ack.ErrorMessage
    }
}
```

**Timeout handling**: If ACK not received within 5 seconds, mark command as failed

---

## R11: DFHack Designation API Integration

**Decision**: Use `df::global::world->map.designation[z][x/16][y/16]` for dig, `Jobs::linkIntoWorld` for constructions

**Rationale**:
- DFHack exposes DF's designation system via `df::map_block->designation`
- Each block is 16×16 tiles, indexed: `block = world->map.block_index[z][x/16][y/16]`
- Designation is bitfield: `designation[x%16][y%16].bits.dig = df::tile_dig_designation::Default`
- For constructions: Create job, set building type, link into world

**Code needed in DFHack plugin**:
```cpp
// New message handler in df_ai_protocol.cpp
case MSG_TYPE_COMMAND:
    handleCommand(payload);
    break;

void handleCommand(const std::vector<uint8_t> &payload) {
    // Parse command ID, type, coordinates
    uint32_t cmdID = read_uint32(payload, 0);
    uint8_t cmdType = payload[4];

    if (cmdType == 0x01) {  // DIG
        applyDigDesignation(x1, y1, z1, x2, y2, z2);
        sendCommandAck(cmdID, 0x00);  // Success
    } else if (cmdType == 0x02) {  // BUILD
        applyBuildDesignation(x, y, z, buildType);
        sendCommandAck(cmdID, 0x00);
    }
}

void applyDigDesignation(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2) {
    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::map_block *block = Maps::getTileBlock(x, y, z);
            if (block) {
                block->designation[x%16][y%16].bits.dig = df::tile_dig_designation::Default;
            }
        }
    }
}
```

**Testing**: Apply designation, verify DF UI shows tiles marked, dwarves path to mine

---

## R12: Task Completion Tracking

**Decision**: Track command progress from acknowledgment through actual tile modifications to completion

**Rationale**:
- ACK only means "designation placed", not "task done"
- Dwarves take time to mine (30s to 5min depending on distance/skill)
- Dwarves work non-linearly (don't follow command order, get distracted)
- AI needs to know: "8 of 15 tiles mined (53% complete)" vs "all done"

**Implementation**:
```go
type TaskStatus string
const (
    TaskAcknowledged = "acknowledged"  // DFHack received, designation set
    TaskInProgress   = "in_progress"   // Dwarves actively working
    TaskCompleted    = "completed"     // All expected tiles changed
    TaskStalled      = "stalled"       // No progress for 60s
    TaskFailed       = "failed"        // Hit hazard or impossible
)

type PendingCommand struct {
    // ... existing fields ...
    ExpectedTiles  int        // How many tiles should change
    CompletedTiles int        // How many actually changed
    LastProgress   time.Time  // Last modification detected
    Status         TaskStatus
}

func (t *CommandTracker) CheckProgress(mods *ModificationOverlay) {
    for _, cmd := range t.pending {
        modsInRegion := mods.GetModificationsInRegion(cmd.Region, cmd.SentAt)
        cmd.CompletedTiles = len(modsInRegion)

        if cmd.CompletedTiles >= cmd.ExpectedTiles {
            cmd.Status = TaskCompleted
        } else if time.Since(cmd.LastProgress) > 60*time.Second {
            cmd.Status = TaskStalled
        } else if cmd.CompletedTiles > 0 {
            cmd.Status = TaskInProgress
        }
    }
}
```

**Called by autonomous loop** every cycle to update AI on task progress

**Feedback includes status**:
- "Command #1001: Bedroom dig (8/15 tiles, 53% complete, in progress)"
- "Command #1002: Passage complete (10/10 tiles mined)"
- "Command #1003: Stalled (3/20 tiles, no progress 90s - dwarves busy hauling)"

---

## R13: Feedback Generation

**Decision**: Template-based feedback with outcome classification (success, hazard, failure) and task progress

**Rationale**:
- AI needs to understand what happened after command
- Feedback quality determines learning effectiveness
- Compare expected vs actual modifications: Did AI get what it wanted?
- Detect hazards revealed: Water/aquifer appeared after dig?

**Feedback templates**:

**Success**:
```
"Command #{id} completed successfully. Dug 15 tiles from (10,20,95) to (15,25,95).
Chamber expanded as planned. Dwarves finished mining in 45 seconds.
No hazards encountered."
```

**Hazard Encounter**:
```
"Command #{id} partially completed. Dug 8 of 15 tiles before breaching light aquifer
at (12,22,94). Water flowing into chamber at depth 3/7. Dwarves stopped mining.
Chamber now contains water hazard. Consider: seal with wall or drain to cistern."
```

**Failure**:
```
"Command #{id} failed. Target region (10,20,80) is inaccessible - no path from
current fort location. Dwarves cannot reach designation. Consider: dig stairway
down from Z=95 to Z=80 first."
```

**Implementation**:
```go
func GenerateFeedback(cmd *Command, mods *ModificationOverlay, hazards *HazardManager) string {
    // Count actual modifications in command region after execution
    actualMods := mods.GetModificationsInRegion(cmd.Region, cmd.Timestamp)

    // Check for new hazards revealed
    newHazards := hazards.GetNewHazardsInRegion(cmd.Region, cmd.Timestamp)

    if len(actualMods) == 0 {
        return generateFailureFeedback(cmd)  // No changes = failed
    }

    if len(newHazards) > 0 {
        return generateHazardFeedback(cmd, actualMods, newHazards)
    }

    return generateSuccessFeedback(cmd, actualMods)
}
```

---

## Summary

**Key Research Decisions**:

1. **Modification Detection**: Baseline comparison from FULL_STATE, transitions infer action types
2. **Chamber Extraction**: 6-connected flood-fill, O(n) where n = modifications
3. **Viewport System**: 4 detail levels (500B/10KB/50KB/200KB) with cumulative data
4. **LLM Providers**: Modular interface, Claude + OpenAI-compatible implementations
5. **Command Protocol**: Binary messages 0x09 (COMMAND) and 0x0A (COMMAND_ACK) on existing TCP connection
6. **Baseline for Saves**: Accept limitation - loaded saves start with empty modification overlay
7. **Response Parsing**: Try JSON, fallback to regex natural language extraction
8. **Conversation History**: Sliding window (last 5 turns) to maintain learning context
9. **API Formats**: Abstract Claude vs OpenAI differences in provider implementations
10. **Command Tracking**: uint32 IDs, pending map, 5s timeout for ACK
11. **DFHack Integration**: `df::map_block->designation[x][y].bits.dig` for mining
12. **Feedback Templates**: Success/hazard/failure with specific outcome descriptions

**All NEEDS CLARIFICATION resolved** - ready for Phase 1 design.
