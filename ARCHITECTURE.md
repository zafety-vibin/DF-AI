# DF-AI System Architecture

**Version**: 0.1.0 (Feature 005 Complete)
**Last Updated**: 2025-11-08
**Status**: Claude API validated, local LLM integration planned

---

## Table of Contents

1. [System Overview](#system-overview)
2. [Architecture Diagram](#architecture-diagram)
3. [Component Details](#component-details)
4. [Data Flow](#data-flow)
5. [Protocol Specification](#protocol-specification)
6. [Current Capabilities](#current-capabilities)
7. [Current Limitations](#current-limitations)
8. [Future Features](#future-features)
9. [How to Modify](#how-to-modify)

---

## System Overview

DF-AI is an autonomous agent that plays Dwarf Fortress by:
1. Reading game state from DFHack plugin (topology, entities, hazards)
2. Assembling structured context for LLM consumption
3. Getting decisions from LLM (Claude API or local model)
4. Sending commands back to DFHack to execute in-game
5. Tracking task completion and providing feedback

**Key Innovation**: The modification overlay tracks only player/AI changes (not the entire 6.9M tile map), creating a "fort signature" that grows as the fort develops. This makes context efficient and focuses the AI on what matters.

### Technology Stack

- **Server**: Go 1.21+ (orchestrator)
- **Plugin**: C++17 (DFHack plugin)
- **Protocol**: Binary TCP with big-endian serialization
- **LLM**: Claude API (validated), OpenAI-compatible local endpoints (planned)
- **Storage**: In-memory overlays (topology 172 KB, hazards ~10 KB, modifications <1 MB)

### Design Principles

1. **Sparse Data Structures**: Only store what changed/matters (overlays vs full map)
2. **Progressive Context**: 4 viewport levels (500B → 200KB) for token efficiency
3. **Learn by Experience**: No safety blocks - AI learns from outcomes (flood, breach, etc.)
4. **Session-Scoped**: Modifications reset on load (persistence planned for Feature 10-11)

---

## Architecture Diagram

```
┌─────────────────────────────────────────────────────────────┐
│                     Dwarf Fortress Game                     │
│  ┌────────────────────────────────────────────────────┐    │
│  │  DFHack Plugin (C++)                                │    │
│  │  - Reads map tiles, entities, designations          │    │
│  │  - Sends state via TCP                              │    │
│  │  - Receives commands, executes designations         │    │
│  └──────────────┬──────────────────────────▲───────────┘    │
└─────────────────┼──────────────────────────┼────────────────┘
                  │ Binary Protocol          │
                  │ (TCP 5001)              │
                  ▼                          │
┌─────────────────────────────────────────────────────────────┐
│              Go Orchestrator Server                         │
│                                                              │
│  ┌──────────────────────────────────────────────────┐      │
│  │  Protocol Layer                                   │      │
│  │  - Connection management                          │      │
│  │  - Message serialization/deserialization          │      │
│  └────────────┬──────────────────────────────────────┘      │
│               │                                              │
│  ┌────────────▼─────────────────────────────────────┐      │
│  │  Data Layers (In-Memory)                          │      │
│  │  ┌─────────────────────────────────────────────┐ │      │
│  │  │ Topology Overlay                             │ │      │
│  │  │ - 172 KB bit array (67.5% open)             │ │      │
│  │  │ - IsOpen(x,y,z) queries                     │ │      │
│  │  │ - Compressed for Level 3 context            │ │      │
│  │  └─────────────────────────────────────────────┘ │      │
│  │  ┌─────────────────────────────────────────────┐ │      │
│  │  │ Hazard Overlays (6 types)                    │ │      │
│  │  │ - Aquifer: 4,737 tiles                      │ │      │
│  │  │ - Water: 14,573 tiles                       │ │      │
│  │  │ - Lava: 335 tiles                           │ │      │
│  │  │ - Caverns: 47,591 tiles (Z-filtered)        │ │      │
│  │  │ - Enemies: Dynamic                          │ │      │
│  │  │ - Sparse map[Coordinate]HazardInfo          │ │      │
│  │  └─────────────────────────────────────────────┘ │      │
│  │  ┌─────────────────────────────────────────────┐ │      │
│  │  │ Modification Overlay                         │ │      │
│  │  │ - Tracks AI/player changes only             │ │      │
│  │  │ - Chamber extraction (flood-fill)           │ │      │
│  │  │ - Session-scoped (resets on load)           │ │      │
│  │  │ - Infers action: Rock→Floor=DUG             │ │      │
│  │  └─────────────────────────────────────────────┘ │      │
│  │  ┌─────────────────────────────────────────────┐ │      │
│  │  │ Entity Cache                                 │ │      │
│  │  │ - 18 dwarves (tame/resident flags)          │ │      │
│  │  │ - Updated via ENTITY_UPDATE (0x08)          │ │      │
│  │  │ - Dwarf positions for centroid/Z-detection  │ │      │
│  │  └─────────────────────────────────────────────┘ │      │
│  └────────────┬──────────────────────────────────────┘      │
│               │                                              │
│  ┌────────────▼─────────────────────────────────────┐      │
│  │  Context Assembly                                 │      │
│  │  - Level 0: Text (500B) - overview               │      │
│  │  - Level 1: Active area (10KB) - chambers/dwarves│      │
│  │  - Level 2: Deep planning (50KB) - hazards       │      │
│  │  - Level 3: Full (200KB) - compressed topology   │      │
│  │  - Embark point (cached centroid, never updates) │      │
│  │  - Topology slice (60×60 first turn, mode Z)     │      │
│  └────────────┬──────────────────────────────────────┘      │
│               │                                              │
│  ┌────────────▼─────────────────────────────────────┐      │
│  │  Autonomous Loop (100s cycle)                     │      │
│  │  - Assembles context                              │      │
│  │  - Sends to LLM with conversation history (5 turn)│      │
│  │  - Parses commands from response                  │      │
│  │  - Executes via CommandExecutor                   │      │
│  │  - Tracks task completion                         │      │
│  │  - Generates feedback for next cycle              │      │
│  └────────────┬──────────────────────────────────────┘      │
│               │                                              │
│  ┌────────────▼─────────────────────────────────────┐      │
│  │  LLM Integration                                  │      │
│  │  - Provider interface (Claude, OpenAI-compatible) │      │
│  │  - System prompt (constant per session)           │      │
│  │  - Response parsing (regex + structured)          │      │
│  │  - JSONL logging for fine-tuning datasets         │      │
│  └────────────┬──────────────────────────────────────┘      │
│               │                                              │
│  ┌────────────▼─────────────────────────────────────┐      │
│  │  Command Executor                                 │      │
│  │  - DIG (6 types): Default, Stairs, Channel, Ramp │      │
│  │  - CHOP (trees), GATHER (plants)                  │      │
│  │  - Task tracking (CommandID → feedback)           │      │
│  │  - Timeout detection (5s ACK wait)                │      │
│  └──────────────────────────────────────────────────┘      │
│                                                              │
│  HTTP API (8081): /health, /ai/history, /metrics           │
└─────────────────────────────────────────────────────────────┘
```

---

## Component Details

### 1. DFHack Plugin (`df_ai_protocol.cpp`)

**Location**: `dfhack-build/plugins/df_ai_protocol.cpp`
**Language**: C++17
**Built for**: DFHack 53.02-r2

#### Responsibilities
- Extract game state (tiles, entities, designations)
- Send binary messages to orchestrator
- Receive and execute commands (dig, build, etc.)
- Apply designations using MapCache pattern

#### Key Functions
```cpp
void extract_topology(std::vector<uint8_t> &tiles)
  - Iterates all map blocks
  - Converts df::tiletype to 0 (wall) or 1 (open)
  - Sends via FULL_STATE (0x02) on connect

void extract_entities(std::vector<EntityInfo> &entities)
  - Iterates world.units.active
  - Classifies: tame/resident = DWARF
  - Sends via ENTITY_UPDATE (0x08) every 100s

bool applyDigDesignation(uint8_t digType, int16_t x1..., std::string &error)
  - Uses MapCache for proper read/modify/write
  - Sets designation: des.bits.dig = static_cast<df::tile_dig_designation>(digType)
  - Flushes with cache.WriteAll()
```

#### Protocol Messages Sent
- `HANDSHAKE (0x01)`: Version negotiation
- `FULL_STATE (0x02)`: Complete map (1.4M tiles)
- `TILE_UPDATE (0x03)`: Incremental changes
- `HEARTBEAT (0x04)`: Keep-alive (10s interval)
- `ENTITY_UPDATE (0x08)`: Dwarf/enemy positions
- `COMMAND_ACK (0x0A)`: Command execution result

#### Protocol Messages Received
- `RESYNC_REQUEST (0x07)`: Server requests full state
- `COMMAND (0x09)`: Dig/build/cancel commands

### 2. Protocol Layer (`internal/protocol/`)

**Binary format**: Big-endian, length-prefixed messages

#### Message Structure
```
[4 bytes: Total Length (includes header)]
[1 byte:  Message Type]
[N bytes: Payload (type-specific)]
```

#### Message Types

| Type | Name | Direction | Payload |
|------|------|-----------|---------|
| 0x01 | HANDSHAKE | Both | version (uint8) |
| 0x02 | FULL_STATE | Plugin→Server | width, height, depth, tiles |
| 0x03 | TILE_UPDATE | Plugin→Server | count, changed tiles |
| 0x04 | HEARTBEAT | Both | timestamp (uint64) |
| 0x07 | RESYNC_REQUEST | Server→Plugin | reason (uint8) |
| 0x08 | ENTITY_UPDATE | Plugin→Server | count, entities |
| 0x09 | COMMAND | Server→Plugin | cmdID, cmdType, digType, region/build |
| 0x0A | COMMAND_ACK | Plugin→Server | cmdID, status, error |

#### Command Payload (0x09)
```
[4 bytes: CommandID]
[1 byte:  CommandType] (0x01=DIG, 0x02=BUILD, 0x03=CANCEL)
[1 byte:  DigType]     (0x01=Default, 0x02=UpDownStair, 0x03=Channel, 0x04=Ramp, 0x05=DownStair, 0x06=UpStair)
[12 bytes: Region]     (X1, Y1, Z1, X2, Y2, Z2 as int16)
```

### 3. Topology Overlay (`internal/topology/`)

**Memory**: 172 KB bit array
**Purpose**: Fast spatial queries without re-scanning map

#### Features
- `IsOpen(x, y, z) bool`: Wall vs passable tile check
- `GetOpenPercentage() float64`: Fort expansion metric
- `Compress(config) *CompressedTopology`: Reduce context size
  - Mode "full": All Z-levels, run-length encoding
  - Mode "active_z_levels": ±5 Z-levels around modifications

#### Compression Stats
- Uncompressed: 172 KB
- Compressed (full): 189 KB (1.1x ratio due to high entropy)
- Compressed (active): ~50 KB (3.4x ratio, focused on work area)

### 4. Hazard Overlays (`internal/hazards/`)

**Memory**: ~10 KB total (sparse maps)
**Purpose**: Danger detection for AI planning

#### Hazard Types
```go
type HazardInfo struct {
    Severity   uint8     // 1-5 danger level
    Flags      uint8     // Type-specific metadata
    DetectedAt time.Time // First seen timestamp
}
```

| Type | Tile Matchers | Z-Level Filter | Count |
|------|---------------|----------------|-------|
| Aquifer | AquiferLight, AquiferHeavy | None | 4,737 |
| Water | RiverSource, Brook, Murky Pool | None | 14,573 |
| Lava | Magma | None | 335 |
| Caverns | Cavern floors (open space) | 10-90 only | 47,591 |
| Enemies | Entity flags (marauder, invader) | Dynamic | Variable |

**Cavern Z-Filter**: Prevents sky detection (Z>90) and magma sea (Z<10)

### 5. Modification Overlay (`internal/modifications/`)

**Memory**: <1 MB (grows with fort)
**Purpose**: Track AI/player changes, extract chambers

#### Key Insight
Natural DF maps have 6.9M tiles. The AI doesn't need to know about unchanged natural terrain - only what's been modified. This creates a "fort signature" that:
- Starts empty (embark)
- Grows as fort is built
- Focuses AI attention on work areas
- Efficient chamber extraction via flood-fill

#### Detection Logic
```go
// Baseline: First FULL_STATE = natural state
// On TILE_UPDATE: Compare new tiletype to baseline

if oldType == Rock && newType == Floor {
    action = DUG  // AI dug this
} else if oldType == Floor && newType == Wall {
    action = BUILT_WALL  // AI built this
}
```

#### Chamber Extraction
- Flood-fill on DUG/BUILT tiles (6-connected)
- Groups contiguous spaces into chambers
- Generates natural language descriptions
- Example: "corridor 15×3×1 (45 tiles) on level 130"

**Limitation**: Session-scoped. Loaded saves show no modifications until AI/player acts again.

### 6. Context Assembly (`internal/context/`)

**Progressive Detail System**: Send only what AI needs at each decision stage

#### Level 0: Text Overview (500B target)
```
"18 dwarves, 3 chambers (48 tiles), hazards: 14,573 water, 335 lava"
```
- Used for: Quick status checks
- Contents: Dwarf count, chamber summary, hazard totals

#### Level 1: Active Area (10 KB target)
```json
{
  "level": 1,
  "embark_point": {"x": 46, "y": 38, "z": 120},
  "active_region": {"x_min": 16, "x_max": 76, ...},
  "chambers": [...],
  "dwarves": [...],
  "topology_slice": {  // First turn only
    "z": 130,
    "open_tiles": ["48,46", "48,47", ...],  // 60×60 area
    "description": "Embark area 60×60 at Z=130: 1800/3600 tiles open (50% passable)"
  }
}
```
- Used for: Standard AI decisions
- Contents: Chambers, dwarf positions, hazard counts, embark reference
- Special: Topology slice on first turn (mode Z-level, 60×60 around dwarves)

#### Level 2: Deep Planning (50 KB target)
- Adds: Cavern data, water/lava hazard positions in active region
- Used for: Strategic depth exploration planning

#### Level 3: Full Context (200 KB target)
- Adds: Compressed topology (active Z-levels or full map)
- Used for: Large-scale fort redesign, pathfinding planning

#### Embark Point Detection
```go
// Calculated ONCE on first cycle (mods.GetCount() == 0)
// Never updated afterwards
func calculateDwarfCentroid(entities) Coordinate {
    // Average X,Y,Z of all DWARF entities
    return Coordinate{X: sumX/count, Y: sumY/count, Z: sumZ/count}
}

// Topology slice uses MODE (most common) Z, not average
func findMostCommonDwarfZ(entities) int16 {
    // Returns Z-level where most dwarves are standing
}
```

**Why separate?** Dwarves split across Z-levels (Z=105 and Z=130) → centroid averages to Z=120 (wrong for both groups). Mode Z (130) shows actual terrain where majority of dwarves are.

### 7. Autonomous Loop (`internal/autonomous/`)

**Cycle**: 100 seconds (configurable)
**Trigger**: Immediate on first entity data, then timer-based

#### Decision Cycle Flow

```
1. Check connection (skip if disconnected - don't waste API calls)
2. Assemble context (Level 0 + Level 1 by default)
3. Build conversation history (last 5 turns)
4. Send to LLM with system prompt
5. Parse response (regex + structured JSON fallback)
6. Execute commands (dig/chop/gather/wait)
7. Track task completion
8. Generate feedback for next cycle
9. Log interaction to JSONL (fine-tuning dataset)
10. Sleep until next cycle
```

#### Task Tracking
```go
type PendingCommand struct {
    CommandID      uint32
    Command        *protocol.CommandMessage
    StartTime      time.Time
    ExpectedTiles  int  // Region size
    CompletedTiles int  // From modification tracking
}
```

**Completion Detection**: Compares modification count in command region to expected tiles.

**Status States**: not_started, in_progress, completed, stalled (no progress 60s), failed

#### Feedback Generation
```go
if task.Status == Completed {
    feedback = "Task completed: 48/48 tiles dug in 2m30s"
} else if task.Status == Stalled {
    feedback = "Task stalled: 12/48 tiles dug, no progress for 60s. Dwarves may be busy or path blocked."
}
```

### 8. LLM Integration (`internal/llm/`)

**Provider Interface**:
```go
type LLMProvider interface {
    SendPrompt(prompt *Prompt) (*Response, error)
}
```

#### Implemented Providers
- **Claude** (`claude.go`): Anthropic API, Messages API format
- **OpenAI** (`openai.go`): OpenAI-compatible endpoints (GPT, local models)

#### Response Parsing
1. Try structured JSON first (future: tool use)
2. Fallback to regex pattern matching:
   - `dig [type] from (x1,y1,z) to (x2,y2,z)`
   - `chop from (x1,y1,z) to (x2,y2,z)`
   - `gather from (x1,y1,z) to (x2,y2,z)`
   - `wait`

#### Conversation History
- Sliding window: Last 5 turns
- Format: `[{role: "user", content: "..."}, {role: "assistant", content: "..."}]`
- System prompt sent every turn (caching planned)

#### JSONL Logging
```jsonl
{"timestamp":"2025-11-08T08:54:37-06:00","turn_number":1,"provider":"claude","model":"claude-haiku-4-5-20251001","tokens_prompt":15236,"tokens_completion":499,"latency_ms":7465,"action":"dig from (40,35,120) to (45,42,120)","command_id":2,"command_sent":true}
```
- Purpose: Generate fine-tuning datasets from successful forts
- Location: `logs/llm-interactions.jsonl`

### 9. Command Executor (`internal/commands/`)

**Timeout**: 5s for ACK response

#### Dig Command Types
| Name | Protocol Constant | DF Designation | Use Case |
|------|------------------|----------------|----------|
| Default | DigTypeDefault (0x01) | Default | Standard mining, remove walls |
| Stairs | DigTypeUpDownStair (0x02) | UpDownStair | Connect Z-levels vertically |
| Channel | DigTypeChannel (0x03) | Channel | Dig down, create holes |
| Ramp | DigTypeRamp (0x04) | Ramp | Sloped access for wagons |
| Down Stair | DigTypeDownStair (0x05) | DownStair | One-way down |
| Up Stair | DigTypeUpStair (0x06) | UpStair | One-way up |

#### Command Tracking
- Generates unique CommandID per command
- Waits for ACK (0x0A) with status
- Maps CommandID → PendingCommand for feedback loop

---

## Data Flow

### Startup Sequence

```
1. Orchestrator starts, listens on TCP 5001
2. DFHack plugin loads, connects to 5001
3. HANDSHAKE exchange (protocol version check)
4. Plugin sends FULL_STATE (1.4M tiles)
5. Server builds topology overlay (172 KB)
6. Plugin sends initial ENTITY_UPDATE
7. Server detects dwarves, triggers first AI cycle
8. Autonomous loop begins 100s cycles
```

### Steady State (Per Cycle)

```
Plugin (every 10s):
  └─> HEARTBEAT → Server

Plugin (every 100s OR player unpause):
  └─> ENTITY_UPDATE → Server
      └─> Updates entity cache
      └─> Triggers autonomous cycle if first data

Plugin (on designation completion):
  └─> TILE_UPDATE → Server
      └─> Updates topology overlay
      └─> Updates modification overlay
      └─> Updates task completion

Autonomous Loop (every 100s):
  1. Assemble context from overlays
  2. Send prompt to LLM
  3. Parse response → commands
  4. Send COMMAND → Plugin
  5. Wait for COMMAND_ACK
  6. Update task tracker
  7. Generate feedback
  8. Log interaction
```

### Command Execution Flow

```
AI Decision:
  "dig stairs from (50,50,120) to (52,52,120)"
      ↓
Parser:
  CommandSpec{Type: "dig", DigType: "stairs", Region: {...}}
      ↓
Executor:
  CommandMessage{CommandID: 123, CommandType: 0x01, DigType: 0x02, Region: {...}}
      ↓
Protocol:
  Binary: [00 00 00 13] [09] [00 00 00 7B] [01] [02] [00 32 00 32 00 78 00 34 00 34 00 78]
      ↓
Plugin:
  Deserialize → applyDigDesignation(digType=2, x1=50, y1=50, z=120, ...)
      ↓
DFHack:
  for each tile in region:
    des.bits.dig = df::tile_dig_designation::UpDownStair
    cache.setDesignationAt(pos, des)
  cache.WriteAll()  // Flush to DF
      ↓
Plugin:
  COMMAND_ACK{CommandID: 123, Status: 0 (success)}
      ↓
Executor:
  Mark command as acknowledged, start tracking
      ↓
Dwarves:
  Execute designation over next 30-180 seconds
      ↓
Plugin:
  TILE_UPDATE → Server (each tile mined)
      ↓
Modification Overlay:
  Increment completedTiles for CommandID 123
      ↓
Next Cycle:
  Generate feedback: "Stairwell 3×3 completed (9/9 tiles)"
```

---

## Current Capabilities

### ✅ What Works

#### Data Collection
- [x] Full map topology (96×96×153, 1.4M tiles)
- [x] Incremental tile updates
- [x] Entity tracking (18 dwarves, positions updated every 100s)
- [x] Hazard detection (6 types: aquifer, water, lava, caverns, enemies)
- [x] Modification tracking (player/AI changes only)
- [x] Chamber extraction (flood-fill contiguous spaces)
- [x] Auto-update (100s intervals)

#### AI Integration
- [x] Context assembly (4 levels, 500B → 200KB)
- [x] Embark point detection (dwarf centroid, cached)
- [x] Topology slice (60×60 terrain map, first turn only, mode Z-level)
- [x] LLM integration (Claude Haiku: $0.02/hour validated)
- [x] Conversation history (5-turn sliding window)
- [x] Command parsing (dig/chop/gather with types)
- [x] Task tracking (expected vs completed tiles)
- [x] Feedback generation (progress, status, stalled detection)
- [x] JSONL logging (fine-tuning datasets)

#### Commands
- [x] DIG (6 types): Default, Stairs, Channel, Ramp, UpStair, DownStair
- [x] CHOP (tree designation)
- [x] GATHER (plant designation)
- [x] WAIT (observe without acting)

#### Protocol
- [x] Binary TCP (big-endian, length-prefixed)
- [x] Bidirectional communication
- [x] Command acknowledgment (success/failure feedback)
- [x] Connection state management (skip cycles when disconnected)
- [x] Timeout handling (5s ACK wait)

### 🎮 Validated Behavior

From testing sessions (Nov 8, 2025):
- ✅ AI sees dwarves correctly (18 dwarves detected)
- ✅ Dig commands execute in DF (blue 'd' markers appear)
- ✅ Dwarves mine designated tiles
- ✅ Topology slice provides spatial awareness (walls vs open)
- ✅ Embark point detection works (dwarf Z-level clustering)
- ✅ Modification tracking captures AI changes
- ✅ Chamber extraction identifies dug spaces

**Known Issue**: AI initially dug at wrong Z-level (averaged centroid vs mode Z). Fixed by using mode Z-level for topology slice.

**Validation Cost**: ~$0.15 in Claude API calls over 4 hours of debugging

---

## Current Limitations

### 🚧 Architecture Limitations

#### 1. Session-Scoped Modifications
**Issue**: Modification overlay resets when save is loaded
**Impact**: AI has no memory of pre-existing fort on reload
**Workaround**: Only works for new forts started with AI active
**Fix**: Feature 10-11 will add persistence (`saves/{fortName}/modifications.json`)

#### 2. No Blueprint System
**Issue**: Cannot load pre-designed fortress layouts
**Impact**: AI reinvents wheel each embark, no knowledge transfer
**Workaround**: None yet
**Fix**: Feature 7 (blueprint CSV import/export)

#### 3. Entity Classification Fragility
**Issue**: Uses `tame/resident` flags (may miss edge cases)
**Impact**: Foreign merchants might be classified as dwarves
**Workaround**: Works for standard embarks
**Fix**: More robust classification (race checks, profession)

#### 4. No Staircase Awareness Yet
**Issue**: AI can't verify stairs connect vertically
**Impact**: May place stairs that don't align between Z-levels
**Workaround**: Parser supports stair commands, execution works
**Fix**: Post-execution verification (check connectivity graph)

#### 5. No System Prompt Caching
**Issue**: System prompt sent every turn (~1000 tokens/turn wasted)
**Impact**: Higher API costs, slower responses
**Workaround**: Use cheap models (Haiku: $0.80/MTok vs Sonnet: $3/MTok)
**Fix**: Feature 6 local LLM will cache system prompt

### 🔧 Implementation Gaps

#### Commands Not Yet Implemented
- [ ] BUILD (construct walls, floors, furniture)
- [ ] STOCKPILE (designate storage zones)
- [ ] WORKSHOP (place buildings)
- [ ] ZONE (activity areas, meeting halls, temples)
- [ ] ORDER (manager work orders)

#### Features Not Yet Available
- [ ] Dwarf labor assignment
- [ ] Military squad management
- [ ] Trade depot automation
- [ ] Danger response (sieges, forgotten beasts)
- [ ] Tantrum spiral detection
- [ ] Food/drink shortage alerts

### 📊 Observability Gaps

- [ ] No heatmap visualization (planned)
- [ ] No performance metrics (API latency, decision quality)
- [ ] No reward signal (fort value, dwarf happiness, survival time)
- [ ] No A/B testing framework (compare AI strategies)

### 🧠 AI Capability Limitations

#### Spatial Reasoning
- **Current**: AI receives text description of chambers + coordinate lists
- **Limitation**: Claude struggles with ASCII terrain maps (Brendan Long article)
- **Impact**: May make suboptimal spatial decisions
- **Fix**: Rich context (chamber descriptions, topology slice) compensates

#### Multi-Step Planning
- **Current**: AI decides every 100s, no intermediate planning
- **Limitation**: Can't plan multi-phase builds (dig → build → furnish)
- **Impact**: May issue conflicting commands or miss dependencies
- **Fix**: Task queue system (Feature planned)

#### Learning Across Forts
- **Current**: Each embark starts from scratch
- **Limitation**: No knowledge transfer between games
- **Impact**: AI doesn't improve over time, no meta-strategy
- **Fix**: Fine-tuning on JSONL logs (see Future Features)

---

## Future Features

### 🎯 Near-Term (Next 3 Features)

#### Feature 006: Local LLM Provider
**Goal**: Eliminate API costs, enable experimentation

**Architecture Changes**:
```go
// Add provider abstraction
type LLMProvider interface {
    SendPrompt(prompt *Prompt) (*Response, error)
}

// Implement local endpoint
type LocalProvider struct {
    endpoint string // http://localhost:1234/v1
    model    string // qwen2.5-coder-14b-instruct
}
```

**Config**:
```yaml
llm_provider_type: local  # claude | local
local_endpoint: http://localhost:1234/v1
local_model: qwen2.5-coder-14b-instruct
local_context_length: 32768
```

**System Prompt Caching**:
- Send system prompt once on session start
- Reuse cached prompt for subsequent turns
- Reduces tokens: ~1000/turn → ~50/turn (20x savings)

**Model Candidates**:
- **Qwen2.5-Coder 14B** (32K context, strong reasoning)
- **Deepseek-Coder 33B** (16K context, excellent code understanding)
- **Llama 3.1 70B** (128K context, expensive but capable)

**Infrastructure**: LM Studio (GUI), Ollama (CLI), llama.cpp (direct)

#### Feature 007: Blueprint System
**Goal**: Enable AI to load/save fortress designs

**Format**: CSV (human-readable, version control friendly)
```csv
x,y,z,type,subtype
50,50,120,dig,stairs
51,50,120,dig,stairs
52,50,120,dig,stairs
50,45,119,dig,default
51,45,119,build,wall
```

**Commands**:
```
load_blueprint "entrance_hall.csv" at (50,50,120)
save_blueprint from (40,40,120) to (60,60,130) as "my_design.csv"
```

**Use Cases**:
- AI learns from successful fort designs
- Human architects provide templates
- Rapid fort iteration (test different layouts)

#### Feature 008: Build Commands
**Goal**: AI can construct buildings, not just dig

**New Protocol Messages**:
- BUILD_WALL, BUILD_FLOOR, BUILD_DOOR
- BUILD_BED, BUILD_TABLE, BUILD_CHAIR
- BUILD_WORKSHOP (carpenter, mason, smelter, forge)

**DFHack Integration**:
```cpp
bool applyBuildDesignation(BuildType type, int16_t x, y, z) {
    // Use DF building API
    df::building* bld = Buildings::allocInstance(df::coord(x,y,z), type);
    Buildings::setSize(bld, width, height, dir);
    Buildings::checkFreeTiles(bld);
    Buildings::allocate(bld);
}
```

### 📈 Learning & Optimization Features

#### Reward Signals

**Proposed Metrics**:
```go
type FortScore struct {
    // Survival
    DaysSurvived      int     // Primary metric
    DwarvesAlive      int     // Current population
    DwarvesDied       int     // Casualties (negative)

    // Economic
    CreatedWealth     int64   // Total fort value
    TradedWealth      int64   // Exports - imports

    // Efficiency
    FoodStockpile     int     // Sustainability
    ProductionRate    float64 // Items crafted/day

    // Complexity
    ChamberCount      int     // Fort size
    DepthReached      int     // Deepest Z-level
    WorkshopsBuilt    int     // Infrastructure

    // Catastrophes (penalties)
    Floods            int     // Water breaches
    MagmaBreaches     int     // Lava disasters
    Sieges            int     // Enemy attacks
    TantrumSpirals    int     // Morale failures
}
```

**Collection**:
- Poll DF stats every cycle via DFHack
- Store as time-series: `fort_metrics.jsonl`
- Correlate with AI decisions

**Optimization**:
- **Supervised**: Train on human-played forts (target = expert score)
- **Reinforcement**: Reward survival days + created wealth
- **Curriculum**: Start with easy embarks → add aquifers → add goblins

#### Heatmap Visualization

**Goal**: Show AI activity patterns on map

**Data Collection**:
```go
type ActivityHeatmap struct {
    DesignationCount  map[Coordinate]int  // How often AI designated tile
    CompletionTime    map[Coordinate]time.Duration  // How long to complete
    FailureCount      map[Coordinate]int  // Tasks that stalled/failed
}
```

**Visualizations**:
1. **Attention Map**: Where does AI focus? (dark = high activity)
2. **Efficiency Map**: Which areas complete fastest? (green = fast, red = slow)
3. **Problem Map**: Where do tasks stall? (hotspots = pathing issues)

**Output Formats**:
- PNG heatmap overlay (tile colors)
- Interactive HTML (hover for details)
- DF in-game overlay (via DFHack rendering)

**Use Cases**:
- Debug AI decision-making (why did it dig there?)
- Identify inefficient patterns (too much backtracking?)
- Visualize fort growth over time (timelapse)

#### Fine-Tuning Pipeline

**Goal**: Improve AI from successful forts

**Dataset Format**:
```jsonl
{"prompt": "Fort state: 18 dwarves, 0 chambers...", "completion": "dig from (40,35,120) to (45,42,120)", "score": 85}
{"prompt": "Fort state: 18 dwarves, 3 chambers...", "completion": "dig stairs from (42,38,120) to (44,40,120)", "score": 92}
```

**Filtering**:
- Only include forts that survived >100 days
- Weight by fort score (high-value forts = more training weight)
- Exclude disasters (floods, total wipeouts)

**Training**:
1. **Supervised Fine-Tuning (SFT)**: Learn from expert decisions
2. **Direct Preference Optimization (DPO)**: Prefer high-score actions
3. **Reinforcement Learning from Fort Feedback (RLFF)**: Custom RL loop

**Models**:
- Qwen2.5-Coder (good baseline, efficient)
- Fine-tuned LoRA adapters (cheap, fast iteration)

### 🔮 Long-Term Vision

#### Multi-Fort Meta-Learning
- AI plays 1000 embarks → learns meta-strategies
- Adapt to biome (desert → cisterns, glacier → magma pumps)
- Adapt to threats (no sieges → expand, sieges → defenses)

#### Hierarchical Planning
- **Strategic layer**: "Build bedroom level, then dining hall"
- **Tactical layer**: "Dig 3×3 stairwell at (50,50,120)"
- **Execution layer**: "Designate tiles, track completion"

#### Multi-Agent Collaboration
- **Architect AI**: Designs fort layout
- **Engineer AI**: Handles water/magma systems
- **Defense AI**: Manages military
- **Economy AI**: Optimizes production chains

#### Human-AI Co-Play
- AI handles tedious tasks (digging, hauling)
- Human handles creative decisions (art, stories)
- Shared control: AI suggests, human approves

---

## How to Modify

### Adding a New Command

**Example**: Add STOCKPILE command

#### 1. Update Protocol (`protocol/message.go`)
```go
const (
    CommandTypeDig       uint8 = 0x01
    CommandTypeBuild     uint8 = 0x02
    CommandTypeCancel    uint8 = 0x03
    CommandTypeStockpile uint8 = 0x06  // NEW
)

type StockpileDesignation struct {
    X, Y, Z       int16
    Width, Height int16
    StockpileType uint8  // Food, Furniture, Weapons, etc.
}

// Add to CommandMessage
type CommandMessage struct {
    CommandID   uint32
    CommandType uint8
    DigType     uint8
    Region      Region
    Build       BuildDesignation
    Stockpile   StockpileDesignation  // NEW
}
```

#### 2. Update Serialization (`protocol/codec.go`)
```go
case CommandTypeStockpile:
    // Write stockpile fields
    binary.Write(w, binary.BigEndian, msg.Stockpile.X)
    binary.Write(w, binary.BigEndian, msg.Stockpile.Y)
    // ... etc
```

#### 3. Update Plugin (`df_ai_protocol.cpp`)
```cpp
case 0x06: {  // STOCKPILE
    // Read payload
    int16_t x = ...;
    int16_t y = ...;
    uint8_t stockpileType = ...;

    // Execute in DF
    bool success = applyStockpileDesignation(x, y, z, width, height, stockpileType, error);
    break;
}

bool applyStockpileDesignation(int16_t x, int16_t y, int16_t z, ...) {
    // Use DFHack stockpiles API
    df::building_stockpilest* stockpile = ...;
    Buildings::setSize(stockpile, width, height);
    Buildings::allocate(stockpile);
    return true;
}
```

#### 4. Update Parser (`llm/parser.go`)
```go
// Add regex pattern
stockpilePattern := regexp.MustCompile(`(?i)stockpile\s+(\w+)\s+at\s+\((\d+),\s*(\d+),\s*(\d+)\)\s+size\s+(\d+)x(\d+)`)

// Parse matches
cmd := CommandSpec{
    Type: "stockpile",
    Params: map[string]interface{}{
        "stockpile_type": match[1],
        "width": width,
        "height": height,
    },
    Region: &RegionSpec{...},
}
```

#### 5. Update Executor (`commands/executor.go`)
```go
func (e *CommandExecutor) SendStockpileCommand(x, y, z int16, width, height int16, stockpileType uint8) (*CommandResult, error) {
    cmd := &protocol.CommandMessage{
        CommandID:   e.tracker.GenerateCommandID(),
        CommandType: protocol.CommandTypeStockpile,
        Stockpile: protocol.StockpileDesignation{
            X: x, Y: y, Z: z,
            Width: width, Height: height,
            StockpileType: stockpileType,
        },
    }
    return e.SendCommand(cmd)
}
```

#### 6. Update Autonomous Loop (`autonomous/loop.go`)
```go
case "stockpile":
    return al.executeStockpile(spec)
```

#### 7. Update System Prompt (`cmd/df-orchestrator/main.go`)
```
Available Commands:
...
5. stockpile [type] at (x, y, z) size WxH
   Types: food, furniture, weapons, armor, gems, finished_goods
   Example: "stockpile food at (50,50,120) size 5x5"
```

#### 8. Test
```bash
go build -o bin/df-orchestrator.exe ./cmd/df-orchestrator
# Rebuild plugin
cd dfhack-build && cmake --build . --target df_ai_protocol
```

### Adding a New Overlay

**Example**: Add VEGETATION overlay (trees, shrubs)

#### 1. Create Overlay (`internal/vegetation/overlay.go`)
```go
package vegetation

type VegetationType uint8

const (
    VegetationTree  VegetationType = 0x01
    VegetationShrub VegetationType = 0x02
    VegetationGrass VegetationType = 0x03
)

type VegetationInfo struct {
    Type       VegetationType
    Species    uint16  // Tree species ID
    DetectedAt time.Time
}

type VegetationOverlay struct {
    mu         sync.RWMutex
    vegetation map[Coordinate]VegetationInfo
}

func NewVegetationOverlay() *VegetationOverlay {
    return &VegetationOverlay{
        vegetation: make(map[Coordinate]VegetationInfo),
    }
}

func (v *VegetationOverlay) Add(coord Coordinate, info VegetationInfo) {
    v.mu.Lock()
    defer v.mu.Unlock()
    v.vegetation[coord] = info
}
```

#### 2. Detect from Topology (`topology/overlay.go`)
```go
func (t *TopologyOverlay) ExtractVegetation() *vegetation.VegetationOverlay {
    overlay := vegetation.NewVegetationOverlay()

    for z := 0; z < t.depth; z++ {
        for y := 0; y < t.height; y++ {
            for x := 0; x < t.width; x++ {
                tiletype := t.getTileType(x, y, z)  // Need to cache this
                if IsTree(tiletype) {
                    overlay.Add(Coordinate{x, y, z}, vegetation.VegetationInfo{
                        Type: vegetation.VegetationTree,
                    })
                }
            }
        }
    }

    return overlay
}
```

#### 3. Add to Context (`context/assembly.go`)
```go
type ViewportContext struct {
    // ... existing fields
    Vegetation *VegetationData `json:"vegetation,omitempty"`
}

type VegetationData struct {
    TreeCount   int                         `json:"tree_count"`
    ShrubCount  int                         `json:"shrub_count"`
    Trees       []vegetation.VegetationInfo `json:"trees,omitempty"`  // Sampled
}
```

#### 4. Update Main (`cmd/df-orchestrator/main.go`)
```go
// After topology overlay built
vegetationOverlay := topologyOverlay.ExtractVegetation()

// Pass to context assembler
ctx, _ := contextAssembler.AssembleContext(
    level,
    mods,
    hazardMgr,
    entities,
    topoOverlay,
    vegetationOverlay,  // NEW
)
```

### Adding a New LLM Provider

**Example**: Add Ollama local provider

#### 1. Implement Interface (`llm/ollama.go`)
```go
package llm

import (
    "bytes"
    "encoding/json"
    "fmt"
    "net/http"
)

type OllamaProvider struct {
    endpoint string  // http://localhost:11434
    model    string  // qwen2.5-coder:14b
    client   *http.Client
}

func NewOllamaProvider(endpoint, model string) *OllamaProvider {
    return &OllamaProvider{
        endpoint: endpoint,
        model:    model,
        client:   &http.Client{Timeout: 120 * time.Second},
    }
}

func (o *OllamaProvider) SendPrompt(prompt *Prompt) (*Response, error) {
    // Build Ollama API request
    reqBody := map[string]interface{}{
        "model": o.model,
        "messages": []map[string]string{
            {"role": "system", "content": prompt.SystemPrompt},
            {"role": "user", "content": prompt.UserMessage},
        },
        "options": map[string]interface{}{
            "temperature": prompt.Temperature,
            "num_predict": prompt.MaxTokens,
        },
    }

    // Send request to Ollama
    resp, err := o.client.Post(
        o.endpoint+"/api/chat",
        "application/json",
        bytes.NewBuffer(jsonData),
    )

    // Parse response
    var ollamaResp struct {
        Message struct {
            Content string `json:"content"`
        } `json:"message"`
    }
    json.NewDecoder(resp.Body).Decode(&ollamaResp)

    return &Response{
        Text: ollamaResp.Message.Content,
        TokensPrompt: 0,  // Ollama doesn't return token counts
        TokensCompletion: 0,
        FinishReason: "stop",
    }, nil
}
```

#### 2. Add to Factory (`llm/factory.go`)
```go
func NewLLMProvider(providerType, model, apiKey, endpoint string) (LLMProvider, error) {
    switch providerType {
    case "claude":
        return NewClaudeProvider(apiKey, model), nil
    case "openai":
        return NewOpenAIProvider(apiKey, model, endpoint), nil
    case "ollama":  // NEW
        return NewOllamaProvider(endpoint, model), nil
    default:
        return nil, fmt.Errorf("unknown provider: %s", providerType)
    }
}
```

#### 3. Add Config Support (`config/config.go`)
```go
type Config struct {
    // ... existing fields

    // Ollama settings
    OllamaEndpoint string `yaml:"ollama_endpoint"`  // http://localhost:11434
    OllamaModel    string `yaml:"ollama_model"`     // qwen2.5-coder:14b
}
```

#### 4. Update orchestrator.yaml
```yaml
llm_provider_type: ollama
ollama_endpoint: http://localhost:11434
ollama_model: qwen2.5-coder:14b
llm_temperature: 0.7
llm_max_tokens: 4096
```

### Modifying Context Levels

**Example**: Add Level 4 for super-detailed planning

#### 1. Add Level Constant (`context/types.go`)
```go
const (
    Level0 ViewportLevel = 0
    Level1 ViewportLevel = 1
    Level2 ViewportLevel = 2
    Level3 ViewportLevel = 3
    Level4 ViewportLevel = 4  // NEW: Ultra-detailed (500KB)
)
```

#### 2. Add Assembler Method (`context/assembly.go`)
```go
func (a *Assembler) assembleLevel4(
    mods *modifications.ModificationOverlay,
    hazardMgr *hazards.HazardManager,
    entities EntityInfoSlice,
    topoOverlay *topology.TopologyOverlay,
) (*ViewportContext, error) {
    // Start with Level 3
    ctx, err := a.assembleLevel3(mods, hazardMgr, entities, topoOverlay)
    if err != nil {
        return nil, err
    }

    // Update to Level 4
    ctx.Level = Level4
    ctx.BudgetBytes = a.level4Budget  // 500 KB

    // Add ultra-detailed data
    // - Full uncompressed topology
    // - All hazard positions (not sampled)
    // - Detailed dwarf stats (skills, labors, needs)
    // - Workshop production chains
    // - Trade goods inventory

    return ctx, nil
}
```

#### 3. Update Dispatcher (`context/assembly.go`)
```go
func (a *Assembler) AssembleContext(...) (*ViewportContext, error) {
    switch level {
    case Level0:
        return a.assembleLevel0(...)
    case Level1:
        return a.assembleLevel1(...)
    case Level2:
        return a.assembleLevel2(...)
    case Level3:
        return a.assembleLevel3(...)
    case Level4:  // NEW
        return a.assembleLevel4(...)
    default:
        return nil, fmt.Errorf("invalid viewport level: %d", level)
    }
}
```

---

## Conclusion

This architecture enables:
- **Efficient AI**: Sparse overlays + progressive context = low token cost
- **Real-time Feedback**: Task tracking + modification detection = learn from outcomes
- **Extensibility**: Clean interfaces for new commands, overlays, providers
- **Observability**: JSONL logs + HTTP API = debugging and analysis
- **Scalability**: Session-scoped state = stateless restarts, easy deployment

**Next Steps**:
1. Spec Feature 006 (local LLM provider)
2. Test staircase commands with local model
3. Build blueprint system (Feature 007)
4. Add reward signals and heatmaps
5. Fine-tune on successful forts

**Philosophy**: Let the AI learn by doing. No hard-coded rules, no safety rails. If it breaches an aquifer, floods the fort, and kills everyone - that's a learning moment. The modification overlay preserves the evidence, the JSONL log captures the decision, and the next iteration avoids the mistake.

---

**Document Status**: Living document, update as features are added
**Maintainer**: Generated from implementation analysis Nov 8, 2025
**Related Docs**: `CURRENT-STATUS.md`, `US*-IMPLEMENTATION-SUMMARY.md`, `spec.md`
