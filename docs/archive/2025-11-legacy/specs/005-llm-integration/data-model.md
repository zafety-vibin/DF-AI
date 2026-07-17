# Data Model: LLM Integration with Modification Tracking and Command Execution

**Feature**: 005-llm-integration | **Date**: 2025-11-07

## Core Entities

### Coordinate

**Purpose**: 3D spatial position (shared across all overlays)

**Fields**:
```go
type Coordinate struct {
    X int16
    Y int16
    Z int16
}
```

**Usage**: Map key for sparse overlays, region boundaries

---

### ModificationInfo

**Purpose**: Metadata for a single player/AI tile change

**Fields**:
```go
type ModificationInfo struct {
    Action    ModificationType  // DUG, BUILT_WALL, BUILT_FLOOR, etc.
    Timestamp time.Time          // When change occurred
    CommandID *uint32            // Nil for player actions, set for AI commands
}

type ModificationType uint8
const (
    ModificationDug ModificationType = iota
    ModificationBuiltWall
    ModificationBuiltFloor
    ModificationDesignationDig
    ModificationDesignationBuild
    ModificationCancelled
)
```

**Relationships**:
- Links to Command via CommandID (optional - player actions have nil)
- Stored in ModificationOverlay sparse map

**Validation**:
- Timestamp must be non-zero
- Action must be valid ModificationType
- CommandID if set must reference existing command

---

### Chamber

**Purpose**: Connected space of modified tiles extracted via flood-fill

**Fields**:
```go
type Chamber struct {
    ID           uint32
    Bounds       Region           // Min/max XYZ bounding box
    TileCount    int              // Number of tiles in chamber
    Connections  []uint32         // IDs of adjacent chambers
    Description  string           // Natural language (e.g., "bedroom area 5×8")
}

type Region struct {
    XMin, XMax int16
    YMin, YMax int16
    ZMin, ZMax int16
}
```

**Relationships**:
- Extracted from ModificationOverlay
- Connections link to other Chamber IDs (passages)

**Derivation**:
- Computed on-demand via flood-fill algorithm
- Not persisted - regenerated each context assembly

**Description Generation**:
- Bounds: "(X1,Y1,Z1) to (X2,Y2,Z2)"
- Dimensions: W × H × D
- Type: Generic "chamber" (semantic classification deferred)

---

### ViewportContext

**Purpose**: Fort state formatted for LLM at specific detail level

**Fields**:
```go
type ViewportContext struct {
    Level            ViewportLevel  // 0, 1, 2, or 3
    TextOverview     string         // Level 0: Text summary
    Chambers         []Chamber      // Level 1+: Modified spaces
    HazardsNearby    []HazardEntry  // Level 1+: Hazards in active area
    DwarfPositions   []DwarfInfo    // Level 1+: Dwarf locations + tasks
    CavernData       *CavernInfo    // Level 2+: Cavern exploration data
    TopologyData     []byte         // Level 3: Full compressed topology
    QueryResults     []QueryResult  // Any query responses included
    SizeBytes        int            // Total context size
}

type HazardEntry struct {
    Type     string       // "aquifer", "water", "lava"
    Location Coordinate
    Severity uint8
    Distance int16        // Tiles from nearest modification
}

type DwarfInfo struct {
    ID       uint32
    Position Coordinate
    Task     string  // "mining", "hauling", "idle" (from entity overlay if available)
}

type CavernInfo struct {
    WaterSources []Coordinate
    LavaSources  []Coordinate
    CavernTiles  []Coordinate  // Natural open spaces in Z=10-90
}
```

**Lifecycle**:
- Created per AI turn
- Assembled from overlays (modifications, hazards, entities, topology)
- Serialized to JSON for LLM prompt
- Discarded after turn (not cached)

---

### LLMPrompt

**Purpose**: Request sent to language model

**Fields**:
```go
type LLMPrompt struct {
    SystemPrompt string          // Role instructions (constant per session)
    Context      *ViewportContext // Current fort state
    Query        string          // Question for AI
    History      []Turn          // Previous conversation turns
}
```

**Serialization**:
- Claude: System as separate field, context + query + history in messages
- OpenAI: System as first message with role="system", context + query + history in user messages

**Example**:
```
System: "You are a Dwarf Fortress AI managing an early embark. Observe modifications, hazards, dwarves. Suggest safe actions. Learn from outcomes."

Context: {
  "overview": "Embark year 1, Z=95-98 surface, 7 dwarves, food OK, drink low",
  "chambers": [{"bounds": "(10,20,95) to (15,25,96)", "tiles": 48, "description": "entrance hall 5×8"}],
  "hazards": [{"type": "aquifer", "location": "(12,30,94)", "distance": 8}],
  "dwarves": [{"id": 1, "pos": "(12,22,95)", "task": "idle"}]
}

Query: "Describe the fort state and suggest one safe action for early game."
```

---

### LLMResponse

**Purpose**: Model output with extracted data

**Fields**:
```go
type LLMResponse struct {
    RawText          string           // Full LLM response text
    Reasoning        string           // Extracted reasoning/description
    Commands         []CommandSpec    // Parsed command proposals (may be empty)
    TokensPrompt     int              // Input tokens used
    TokensCompletion int              // Output tokens generated
    FinishReason     string           // "stop", "length", "error"
    Latency          time.Duration    // API call duration
    ParsedAt         time.Time
}

type CommandSpec struct {
    Type   CommandType  // DIG, BUILD, CANCEL
    Region Region       // Target coordinates
}
```

**State Transitions**:
```
API Call → Raw Response → Parse → CommandSpec extraction → Command creation
```

**Parsing Logic**:
- Try JSON: `{"reasoning": "...", "commands": [...]}`
- Fallback regex: Extract coordinates from "dig (10,20,95) to (15,25,95)"
- If no commands found: Reasoning-only (AI observing, not acting)

---

### Command

**Purpose**: Action instruction from Server to DFHack

**Fields**:
```go
type Command struct {
    ID        uint32
    Type      CommandType
    Region    Region         // For DIG/CANCEL (bounding box)
    BuildType uint8          // For BUILD (wall, floor, stairs, etc.)
    Position  *Coordinate    // For BUILD (single tile)
    Timestamp time.Time      // When command created
}

type CommandType uint8
const (
    CommandTypeDig CommandType = 0x01
    CommandTypeBuild = 0x02
    CommandTypeCancel = 0x03
)
```

**Serialization** (Binary protocol message 0x09):
```
[4: Length] [1: Version] [1: Type=0x09] [4: CommandID] [1: CommandType] [Payload]

DIG payload: [2:X1][2:Y1][2:Z1][2:X2][2:Y2][2:Z2]
BUILD payload: [2:X][2:Y][2:Z][1:BuildType]
CANCEL payload: [2:X1][2:Y1][2:Z1][2:X2][2:Y2][2:Z2]
```

**Validation**:
- Coordinates must be within map bounds
- Region must be non-empty (X2 >= X1, etc.)
- BuildType must be valid DF building type

---

### CommandAcknowledgment

**Purpose**: DFHack response confirming command execution

**Fields**:
```go
type CommandAcknowledgment struct {
    CommandID    uint32
    Status       AckStatus
    ErrorMessage string       // Empty if success
    Timestamp    time.Time
}

type AckStatus uint8
const (
    AckSuccess AckStatus = 0x00
    AckPartialSuccess = 0x01  // Some tiles designated, others blocked
    AckFailure = 0x02
)
```

**Serialization** (Binary protocol message 0x0A):
```
[4: Length] [1: Version] [1: Type=0x0A] [4: CommandID] [1: Status] [2: ErrorMsgLen] [N: ErrorMsg]
```

**Lifecycle**:
```
Server sends COMMAND → DFHack receives → Executes → Sends ACK → Server receives → Updates tracker
```

**Timeout**: If ACK not received within 5 seconds, assume command lost/failed

---

### QueryRequest

**Purpose**: AI request for specific spatial data

**Fields**:
```go
type QueryRequest struct {
    Type       QueryType
    Parameters map[string]interface{}  // Query-specific params
}

type QueryType string
const (
    QueryTopologySlice QueryType = "topology_slice"     // Params: z, x_min, x_max, y_min, y_max
    QueryHazardList    = "hazard_list"                  // Params: type (water/aquifer/lava), z_min, z_max
    QueryPathFind      = "pathfind"                     // Params: start (x,y,z), end (x,y,z)
    QueryModifications = "modifications_in_region"      // Params: region
)
```

**Example queries**:
- "Show topology slice Z=95, X=10-30, Y=20-40"
- "List water sources in Z=80-100"
- "Find path from (10,20,95) to (45,60,85)"

---

### QueryResponse

**Purpose**: Filtered data matching query parameters

**Fields**:
```go
type QueryResponse struct {
    QueryType  QueryType
    Data       interface{}  // Type varies by query
    Format     string       // "json", "ascii", "coordinates"
    SizeBytes  int
}
```

**Data formats by query type**:
- **topology_slice**: `[][]bool` (2D slice) or ASCII art string
- **hazard_list**: `[]Coordinate` array
- **pathfind**: `[]Coordinate` path or "no path found"
- **modifications_in_region**: `[]ModificationInfo`

---

## Entity Relationships

```
ModificationOverlay
  └─ contains ───> ModificationInfo (sparse map)
                      └─ links to ───> Command (via CommandID)

Chamber
  └─ extracted from ───> ModificationOverlay (flood-fill)
  └─ connects to ───> Chamber (via Connections[])

ViewportContext
  ├─ includes ───> Chamber[] (feature list)
  ├─ includes ───> HazardEntry[] (from HazardManager)
  ├─ includes ───> DwarfInfo[] (from EntityData)
  └─ includes ───> QueryResponse[] (on-demand data)

LLMPrompt
  ├─ contains ───> ViewportContext
  └─ includes ───> Turn[] (conversation history)

LLMResponse
  └─ parsed into ───> CommandSpec[] (proposed actions)

Command
  ├─ created from ───> CommandSpec (LLM proposal)
  ├─ tracked by ───> CommandTracker (pending acknowledgments)
  └─ acknowledged via ───> CommandAcknowledgment

CommandAcknowledgment
  └─ links to ───> Command (via CommandID)

Turn (in conversation history)
  ├─ includes ───> ViewportContext (what AI saw)
  ├─ includes ───> LLMResponse (what AI said)
  ├─ includes ───> Command (what AI did)
  └─ includes ───> Feedback (what happened)
```

---

## State Transitions

### Modification Lifecycle

```
Natural Baseline (on FULL_STATE)
    ↓
Player digs tile (TILE_UPDATE received)
    ↓
DetectModification: TileType changed from Rock → Floor
    ↓
Infer action: DUG
    ↓
ModificationOverlay stores: {Action: DUG, Timestamp: now, CommandID: nil}
    ↓
Modification persists until server restart
```

### AI Command Lifecycle

```
AI Turn starts (timer triggers)
    ↓
Assemble ViewportContext (Level 0+1)
    ↓
Send LLMPrompt to provider
    ↓
Receive LLMResponse
    ↓
Parse CommandSpec from response
    ↓
Create Command with generated ID
    ↓
Send COMMAND message to DFHack (binary protocol)
    ↓
DFHack executes designation
    ↓
Send COMMAND_ACK back to Server
    ↓
Server receives ACK, updates CommandTracker
    ↓
Wait for TILE_UPDATE showing actual modifications
    ↓
Link modifications to Command via ID
    ↓
Generate Feedback describing outcome
    ↓
Append to conversation history
    ↓
Next turn: Include feedback in context
```

### Chamber Extraction Flow

```
ModificationOverlay with N scattered modifications
    ↓
Flood-fill starts from first unvisited modification
    ↓
Expand to 6-connected neighbors (N/S/E/W/Up/Down)
    ↓
Mark all connected tiles as Chamber #1
    ↓
Repeat for next unvisited modification → Chamber #2
    ↓
Continue until all modifications visited
    ↓
For each chamber: Calculate bounds (min/max XYZ), count tiles
    ↓
Detect adjacencies between chambers → Connections[]
    ↓
Generate natural language description
    ↓
Return []Chamber for context assembly
```

---

## Memory Budget Analysis

**Modification Overlay** (sparse):
- Early fort (1000 mods): 1000 × 32 bytes = 32 KB ✅
- Large fort (10k mods): 10k × 32 bytes = 320 KB ✅ (under 1 MB target)
- Baseline cache: 6.9M × 2 bytes (TileType only) = 13.8 MB ⚠️

**Optimization for baseline**:
- Don't cache entire baseline - too large!
- Instead: Assume baseline TileType = first time we see coordinate
- Or: Heuristic rules - Rock types = natural, Floor in open area = natural
- Accept: Loaded saves can't detect pre-existing modifications

**Revised approach**: No baseline cache. Detect modifications incrementally from first FULL_STATE received.

**ViewportContext sizes**:
- Level 0: ~500 bytes (text only)
- Level 1: ~10 KB (chambers, hazards, dwarves as JSON)
- Level 2: ~50 KB (+ cavern data)
- Level 3: ~200 KB (+ compressed topology)

**Conversation History**:
- 5 turns × ~15 KB per turn = 75 KB
- Sliding window: Drop oldest turn when > 5

**Total memory** (in-flight):
- Modification overlay: 320 KB max
- Current context: 200 KB max
- Conversation history: 75 KB
- Command tracker: 10 KB (100 pending commands × 100 bytes)
- **Total: ~600 KB** ✅ Acceptable for research phase

---

## Data Flow Summary

```
DF Plugin sends TILE_UPDATE
    ↓
ModificationOverlay.DetectModifications()
    ↓
Store modified coordinates in sparse map
    ↓
                    ┌──────────────────┐
                    │  AI Turn Cycle   │
                    │  (every 100s)    │
                    └──────────────────┘
                            ↓
    Extract Chambers via flood-fill
                            ↓
    Assemble ViewportContext (Level 0+1)
                            ↓
    Create LLMPrompt (context + history)
                            ↓
    Send to LLM Provider (Claude or local)
                            ↓
    Receive LLMResponse
                            ↓
    Parse CommandSpec from response
                            ↓
    Create Command with generated ID
                            ↓
    Send COMMAND binary message to DFHack
                            ↓
    Receive COMMAND_ACK from DFHack
                            ↓
    Wait for TILE_UPDATE (actual tile changes)
                            ↓
    Link modifications to Command.ID
                            ↓
    Generate Feedback (success/hazard/failure)
                            ↓
    Append Turn to history (context + response + outcome)
                            ↓
    Repeat cycle
```

---

## Query API Data Flow

```
AI response includes query request: "show water sources in Z=80-100"
    ↓
Parse QueryRequest from AI text
    ↓
Execute query against HazardManager water overlay
    ↓
Filter to Z range: 80-100
    ↓
Return QueryResponse with coordinate array
    ↓
Include QueryResponse in next ViewportContext
    ↓
AI sees query results in next turn
```
