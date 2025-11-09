# Feature 006 Architecture: What Changed and Why

**Purpose**: Clarify the full pipeline, what data lives where, and how agents work



---

## 

## The Confusion: Agents Are Not Python



**Original spec said**: "5 specialized Python agents"
**What we actually built**: 5 specialized **Go agents** (deterministic code, not ML models)



**Why Go, not Python?**

* Agents are simple metric checks (if food < 20/dwarf, propose farm)
* No ML needed - just if/else logic
* Go matches codebase (no Python dependency)
* <1ms per agent (would be slower in Python subprocess)



**What agents actually are**:

* Go structs implementing GoalAgent interface
* Pure functions: `Analyze(metrics) → proposals`
* No state between cycles
* Run in orchestrator process (not separate processes)



---

## 

## Full Pipeline: Reality → Agents → Graph → Arbiter → Commands → Reality

### 

### Step 1: Reality State (Existing Systems - Unchanged)



**Source**: DFHack plugin extracts from DF game state

**Messages Sent**:



```
RESYNC (first message only):
  - topology\_slice: 60×60 bool array (passable tiles at dwarf Z-level)

ENTITY\_UPDATE (every frame):
  - entities\[]: position (x,y,z), type (dwarf/enemy/animal), ID
  - fort\_info: days\_elapsed, wealth (if available)
```



**Stored In Orchestrator**:

* `TopologyOverlay`: Bit array of open tiles (from first RESYNC)
* `EntityCache`: Dwarf positions, enemy positions (from ENTITY\_UPDATE)
* `ModificationOverlay`: Sparse map of dug tiles (detected from RESYNC diffs)
* `HazardManager`: Water, lava, cavern locations
* `PhaseManager`: Fort age → phase (embark/establish/expand/fortify)



**This is "reality"**: What actually exists in the game RIGHT NOW.



---

### 

### Step 2: Agent Analysis (NEW - Feature 006)

**When**: Every autonomous cycle (100 seconds)

**Process**:



```go
// 1. Compute metrics from reality state
metrics := FortMetrics{
    DwarfCount: countDwarves(entityCache),  // From ENTITY\_UPDATE
    EnemyCount: countEnemies(entityCache),   // From ENTITY\_UPDATE
    FortAge: phaseManager.GetDaysElapsed(), // From fort\_info
    // TODO: Food from game state (not yet extracted)
    // TODO: Bedrooms from modification overlay chambers
}

// 2. Each agent analyzes metrics
FoodAgent.Analyze(metrics):
    if metrics.FoodPerDwarf < 20:
        return ModificationNode{
            Type: "farm\_plot",
            Region: {X: 40, Y: 30, Z: 125},  // Proposed location
            Priority: 10,
            Rationale: "Food at 11.4/dwarf, below 20 target"
        }
```



**Key Point**: Agents analyze **reality metrics** (dwarf count, food, enemies) and output **proposed future changes** (farm\_plot node at XYZ).



**Agents DO NOT**:

* Parse the node graph (they CREATE it)
* Analyze tile types directly (they use high-level metrics)
* Run ML models (they're if/else logic)



**Agents DO**:

* Read reality state (entity count, fort age, food stocks)
* Decide what needs to be done (if food low, need farm)
* Output proposals (ModificationNode: farm\_plot at these coordinates)



---

### 

### Step 3: Proposal Graph Assembly (NEW - Feature 006)



**Process**:



```go
graph := NewProposalGraph()

// Add proposals from all agents
for each agent proposal:
    graph.AddNode(proposal)

// Detect spatial conflicts (AABB intersection)
graph.DetectSpatialConflicts()

// Result:
// graph.Nodes = \[food\_1: farm\_plot, housing\_1: bedroom\_cluster]
// graph.ConflictEdges = {} (no overlaps)
```



**What the graph IS**:

* **Temporary planning structure** (exists for one cycle only)
* **Proposed future changes**, not current reality
* **Agent intentions**, not executed commands



**What the graph IS NOT**:

* NOT a representation of game state
* NOT a persistent data structure
* NOT the modification overlay



**Think of it as**: A to-do list for this cycle. "Food agent wants farm here, housing agent wants bedrooms there."



---

### 

### Step 4: Arbiter Coordination (NEW - Feature 006)



**Input**: ProposalGraph serialized to JSON

**Example JSON sent to LLM**:



```json
{
  "nodes": \[
    {
      "id": "food\_1",
      "agent": "FoodSecurity",
      "type": "farm\_plot",
      "region": {"xmin": 40, "ymin": 30, "z": 125, "xmax": 50, "ymax": 40},
      "dependencies": \["requires\_access\_from"],
      "conflicts": \[],
      "priority": 10,
      "urgency": 0.8,
      "rationale": "Food at 11.4/dwarf, below 20 target"
    },
    {
      "id": "housing\_1",
      "agent": "Housing",
      "type": "bedroom\_cluster",
      "region": {"xmin": 60, "ymin": 30, "z": 125, "xmax": 70, "ymax": 40},
      "dependencies": \["requires\_access\_from"],
      "conflicts": \[],
      "priority": 9,
      "urgency": 0.6,
      "rationale": "7 dwarves, only 3 bedrooms"
    }
  ]
}
```



**LLM Arbiter Task**:

* Recognize both need corridor access
* Inject corridor\_connector node linking them
* Return execution sequence: \[corridor\_1, food\_1, housing\_1]



**Arbiter Output** (JSON):



```json
{
  "sequence": \[
    {"id": "corridor\_1", "type": "corridor\_connector", "region": {...}, "rationale": "Shared access for farm and bedrooms"},
    {"id": "food\_1", "rationale": "Higher priority, depends on corridor"},
    {"id": "housing\_1", "rationale": "Parallel with food, shares corridor"}
  ],
  "synergies": \["corridor\_1"],
  "deferred": \[]
}
```



---

### 

### Step 5: GraphExecutor (NEW - Feature 006)



**Process**:



```go
for each node in arbiter.sequence:
    switch node.Type:
        case "farm\_plot":
            commands = \[DIG at region with DigTypeDefault]
        case "bedroom\_cluster":
            commands = \[DIG at region with DigTypeDefault]
        case "corridor\_connector":
            commands = \[DIG at region with DigTypeDefault]

    send commands to DFHack
```



**Result**: 3 DIG commands sent to plugin for execution



---

### 

### Step 6: Reality Updates (Existing System - Unchanged)



**DFHack Plugin**:

* Receives DIG commands
* Creates designations in DF
* Dwarves dig tiles
* Next RESYNC shows new dug tiles



**Modification Overlay**:

* Detects new dug tiles (diff from topology)
* Adds to modification sparse map
* Next cycle, chambers detected from connected dug tiles



**Next Cycle**:

* FoodAgent sees farm was dug (from modifications)
* HousingAgent sees bedrooms exist (chamber count increased)
* Agents don't propose same things again (targets met)



---

## 

## Key Architectural Points

### 

### 1\. Two Separate Data Structures



**Modification Overlay** (REALITY):

* Persistent sparse map: Coordinate → ModificationInfo
* Represents what WAS dug (past tense)
* Saved to disk (modifications.json)
* Used for tracking fort changes over time



**Proposal Graph** (PLANNING):

* Temporary graph: ModificationNode with dependencies/conflicts
* Represents what SHOULD BE dug (future tense)
* Exists for one cycle only (not saved)
* Used for coordination between agents



**Analogy**:

* Modification overlay = your bank account (reality)
* Proposal graph = your shopping list (intentions)

### 

### 2\. Agents Analyze Reality, Create Plans



**Agents DO NOT need**:

* Tile type data (rock vs floor vs wall)
* Items on tiles
* Individual entity details



**Agents DO need** (high-level metrics):

* Dwarf count (from entity cache)
* Food stocks (TODO: needs extraction from DF)
* Bedroom count (from modification overlay chambers)
* Mining activity (TODO: track tiles dug per cycle)
* Enemy count (from entity cache)



**Why**: Agents make high-level decisions ("need more food"), not micro-decisions ("dig this specific rock tile").



**Example**:



```go
// FoodAgent doesn't care about tile types
// It just knows: "7 dwarves, 80 food units = 11.4/dwarf"
// Decision: "Need farm. Propose farm\_plot node at reasonable location."
```

### 

### 3\. The Graph Is NOT a Game State Representation



**What you originally thought** (I think):

* Node graph represents entire fort state (every tile, every entity)
* Agents parse this mega-structure
* Agents modify graph nodes to make changes



**What it actually is**:

* Node graph is just **agent proposals for THIS cycle**
* Typically 0-5 nodes total (food\_1, housing\_1, mining\_1)
* Each node = one planned action (dig farm, dig bedrooms)
* Graph destroyed after cycle completes



**Why this is better**:

* Lean (5 nodes vs 60×60×10 tile grid)
* Fast (agents don't parse huge structure)
* Token efficient (<1k tokens vs 4k+ tokens)

### 

### 4\. What Needs More Data?



**Your question**: "Do we need to add tile type, entities on tiles, items to check-updates?"

**Answer**: Depends on what agents need to decide



**Currently Missing**:

* ✅ **Food stocks** - Need this for FoodAgent to work

  * Add to ENTITY\_UPDATE or new message
  * Or extract from DF stocks screen data

* ✅ **Bedroom count** - Need this for HousingAgent

  * Can extract from modification overlay chambers
  * Or track zone designations

* ✅ **Mining activity** - Need this for MiningAgent

  * Track tiles dug per cycle (from modification overlay diffs)
  * Track ore discoveries (from material type in tiles)



**NOT Needed**:

* ❌ Tile types at every coordinate (too much data)
* ❌ Items on every tile (not relevant for high-level planning)
* ❌ Individual entity details (just counts are fine)



**Philosophy**: Agents make strategic decisions (where to farm, where to dig), not tactical decisions (which specific tile to mine).



---

## 

## What Needs to Be Removed? (L0-L3 Context)



**Answer**: NOTHING needs removal. The old system still works!

**Two Modes**:

### 

### Mode 1: Direct-LLM (Feature 005 - Old Way)



```yaml
enable\_goal\_agents: false  # Default
```



**Pipeline**:



```
Reality → L1 JSON Context → LLM (full fort state) → Parse commands → Execute
```



**Context**: 4k+ tokens (all dwarves, all chambers, all hazards)
**LLM sees**: Everything
**LLM decides**: What to dig, where to dig

### 

### Mode 2: Graph-Based Agents (Feature 006 - New Way)



```yaml
enable\_goal\_agents: true
```



**Pipeline**:



```
Reality → Metrics → Agents → Proposal Graph → Arbiter (graph JSON) → Executor → Execute
```



**Arbiter sees**: Just proposals (~1k tokens)
**Arbiter decides**: Which proposals to approve, what synergies to add
**Agents decide**: What needs to be done (farm, bedrooms, mine)



**Both modes use the same reality data** (entities, topology, modifications). The difference is:

* Direct-LLM: Send ALL data to LLM, let it figure out what to do
* Graph-based: Agents compress data to proposals, arbiter coordinates



---

## 

## Detailed Pipeline Walkthrough

### 

### Cycle Start (t=0)



**State**:

* 7 dwarves at (various XYZ)
* 80 food units (11.4/dwarf)
* 3 bedrooms exist (from previous digs)
* 0 enemies
* Fort age: 15 days

### 

### Agent Analysis (t=0 to t=50ms)



**FoodAgent**:



```go
metrics := {DwarfCount: 7, FoodPerDwarf: 11.4}
if 11.4 < 20:  // Below target
    return farm\_plot node at (40,30,125)
```



**HousingAgent**:



```go
metrics := {DwarfCount: 7, BedroomCount: 3}
deficit = 7 - 3 = 4 bedrooms needed
return bedroom\_cluster node at (60,30,125)
```



**MiningAgent, WealthAgent, DefenseAgent**:



```go
// All targets met or no threats
return nil (no proposals)
```



**Result**: 2 proposals (food\_1, housing\_1)

### 

### Graph Assembly (t=50ms to t=70ms)



```go
graph = NewProposalGraph()
graph.AddNode(food\_1)
graph.AddNode(housing\_1)
graph.DetectSpatialConflicts()

// No conflicts (different XY coordinates)
```



**Graph State**:



```json
{
  "nodes": \[
    {"id": "food\_1", "type": "farm\_plot", "region": {40,30,125 to 50,40,125}},
    {"id": "housing\_1", "type": "bedroom\_cluster", "region": {60,30,125 to 70,40,125}}
  ],
  "conflicts": {}
}
```

### 

### Arbiter Call (t=70ms to t=570ms)



**Request to LM Studio**:



```
POST http://localhost:1234/v1/chat/completions
System: "You are arbiter, coordinate proposals, recognize synergies..."
User: {graph JSON above}
```



**LLM thinks**:

* Both nodes need corridor access
* Both at Z=125, Y=30 (same horizontal level)
* Can share corridor at Y=30 connecting X=50 to X=60



**LLM responds**:



```json
{
  "sequence": \[
    {"id": "corridor\_1", "type": "corridor\_connector", "region": {50,30,125 to 60,30,125}},
    {"id": "food\_1"},
    {"id": "housing\_1"}
  ],
  "synergies": \["corridor\_1"]
}
```

### 

### Executor Converts Nodes → Commands (t=570ms to t=600ms)



```go
for each node in sequence:
    corridor\_1 (corridor\_connector):
        → DIG command: (50,30,125) to (60,30,125), DigTypeDefault

    food\_1 (farm\_plot):
        → DIG command: (40,30,125) to (50,40,125), DigTypeDefault

    housing\_1 (bedroom\_cluster):
        → DIG command: (60,30,125) to (70,40,125), DigTypeDefault

Total: 3 DIG commands
```

### 

### Commands Sent to DFHack (t=600ms)



```
CommandMessage{CommandType: DIG, DigType: Default, Region: {50,30,125 to 60,30,125}}
CommandMessage{CommandType: DIG, DigType: Default, Region: {40,30,125 to 50,40,125}}
CommandMessage{CommandType: DIG, DigType: Default, Region: {60,30,125 to 70,40,125}}
```

### 

### DFHack Executes (t=600ms+)

* Plugin receives 3 commands
* Creates dig designations in DF
* Sends ACK back to orchestrator
* Dwarves start mining

### 

### Reality Updates (next RESYNC)

* Next RESYNC shows newly dug tiles
* Modification overlay detects them (diff from topology)
* Chamber detection finds connected rooms
* Next cycle, HousingAgent sees bedroom count increased



---

## 

## What Data Do Agents Need? (Your Question)

### 

### Current Data Available to Agents



**From Entity Cache** (ENTITY\_UPDATE):

* ✅ Dwarf count
* ✅ Enemy count
* ✅ Dwarf positions (could calculate centroid for Z-level)



**From Phase Manager** (fort\_info):

* ✅ Fort age (days)
* ✅ Current phase (embark/establish/expand)



**From Modification Overlay**:

* ✅ Total tiles dug (modification count)
* ✅ Chambers (connected dug areas)
* ❌ Bedroom count (need to track zones or infer from chamber count)



**NOT AVAILABLE YET** (but needed):

* ❌ Food stocks (need DF extraction)
* ❌ Drink stocks (need DF extraction)
* ❌ Wealth value (need DF extraction)
* ❌ Ore strike tracking (need material type tracking)

### 

### What Should We Add?



**HIGH PRIORITY** (makes agents functional):

1. **Food/drink stocks** - Add to ENTITY\_UPDATE or new STOCKS\_UPDATE message
2. **Bedroom tracking** - Count zone designations or use chamber count as proxy
3. **Mining tile tracking** - Count modifications added per cycle
4. **Wealth tracking** - Fort value from DF (if available in API)



**LOW PRIORITY** (nice-to-have):

* Tile material types (for ore detection)
* Individual entity attributes (for dwarf skills)
* Item locations (for logistics optimization)



**NOT NEEDED**:

* Full tile grid (60×60×200 array) - too large
* Every item in fort - irrelevant for strategic planning



---

## 

## The Graph Philosophy: Lean Semantic Layer



**Your question**: "Is it meant to fully represent game state or remain lean?"

**Answer**: **LEAN** - It's a semantic layer, not a mirror of reality.



**Design Goals**:

1. **Token efficiency**: Graph JSON <1k tokens vs full fort state 4k+ tokens
2. **Strategic focus**: Nodes represent goals (farm, bedrooms), not tiles
3. **Spatial reasoning**: Dependencies/conflicts encode relationships commands can't
4. **Temporal planning**: Dependencies enable multi-phase plans (shaft → tunnel → rooms)



**What nodes represent**:

* "I want a farm plot in this general area" (farm\_plot node)
* NOT "Dig tile (40,30,125), then (41,30,125), then (42,30,125)..."



**Why this is better**:

* Arbiter can reason about "farm" and "bedrooms" sharing corridor
* Can't do this with raw coordinate commands
* Graph structure makes relationships explicit



---

## 

## Old vs New: Side-by-Side

### 

### Feature 005 (Direct-LLM Mode)



**Pipeline**:



```
ENTITY\_UPDATE → Entity cache
RESYNC → Topology, modifications
          ↓
   Assemble L1 JSON context (all dwarves, all chambers, all hazards)
          ↓
   Send to LLM: "Here's the fort state, what should I do?"
          ↓
   LLM: "DIG (40,30,125) to (50,40,125)"
          ↓
   Parse response → Execute DIG command
```



**Pros**: Simple, LLM sees everything


**Cons**: 4k+ tokens, LLM must figure out EVERYTHING, expensive

### 

### Feature 006 (Graph-Based Agents)



**Pipeline**:



```
ENTITY\_UPDATE → Entity cache
RESYNC → Topology, modifications
          ↓
   Compute FortMetrics (dwarf count, food/dwarf, bedrooms, fort age)
          ↓
   Run 5 agents in parallel (Go code, <100ms)
   Each agent: if metrics bad, propose node
          ↓
   Assemble ProposalGraph (food\_1, housing\_1 nodes)
   Detect conflicts (spatial overlaps)
          ↓
   Send graph JSON to arbiter: "Here are proposals, coordinate them"
          ↓
   Arbiter: "Add corridor\_1, sequence: \[corridor, food, housing]"
          ↓
   GraphExecutor: Convert 3 nodes → 3 DIG commands
          ↓
   Execute commands
```



**Pros**: <1k tokens, agents compress analysis, arbiter just coordinates, cheap


**Cons**: More complex, agents need accurate metrics, new code to maintain



---

## 

## What Systems Can Be Removed?



**Short answer**: NOTHING should be removed.



**L0-L3 Context System**:

* Still used when `enable\_goal\_agents: false`
* Provides backward compatibility
* Allows A/B testing (graph vs direct-LLM)



**Entity Updates, Topology, Modifications**:

* ALL still needed
* Agents read from these same structures
* Just compressed differently (metrics vs full JSON)



**What's ADDED**:

* internal/agents/ package (new)
* ProposalGraph (temporary, not persisted)
* LocalLLMProvider (alternative to Claude)
* GraphExecutor (node → command converter)



---

## 

## Data Flow Diagram



```
┌─────────────────────────────────────────────────────────────┐
│ DWARF FORTRESS (Reality)                                    │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ ENTITY\_UPDATE, RESYNC
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ Orchestrator Memory (Reality Cache)                         │
│ - EntityCache: \[{x,y,z, type:dwarf}, ...]                   │
│ - TopologyOverlay: bit\[60]\[60] (passable tiles)              │
│ - ModificationOverlay: map\[Coord]ModificationInfo (dug tiles)│
│ - PhaseManager: fort age, current phase                      │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Every 100 seconds (autonomous cycle)
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ computeFortMetrics() - Extract High-Level Metrics           │
│ - DwarfCount: 7 (from EntityCache)                          │
│ - FoodPerDwarf: 11.4 (TODO: from DF stocks)                 │
│ - BedroomCount: 3 (from chambers in ModificationOverlay)    │
│ - EnemyCount: 0 (from EntityCache)                          │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Pass metrics to all agents
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ Goal Agents (Parallel Go Functions)                         │
│                                                              │
│ FoodAgent.Analyze(metrics):                                 │
│   if FoodPerDwarf < 20:                                      │
│       return farm\_plot node                                  │
│                                                              │
│ HousingAgent.Analyze(metrics):                              │
│   if BedroomCount < DwarfCount:                              │
│       return bedroom\_cluster node                            │
│                                                              │
│ \[Mining, Wealth, Defense agents...]                         │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Collect proposals from all agents
                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│ ProposalGraph (Temporary Planning Structure)                        │
│                                                                     │
│ Nodes: \[food\_1: farm\_plot, housing\_1: bedroom\_cluster]              │
│ Conflicts: {} (spatial AABB check)                                  │
│ Dependencies: \[food\_1: requires\_access, housing\_1: requires\_access] │
└────────────────┬────────────────────────────────────────────────────┘
                 │
                 │ Serialize to JSON (~500 tokens)
                 ▼
┌───────────────────────────────────────────────────────────────┐
│ Arbiter (Local LLM via LM Studio)                             │
│                                                               │
│ Input: Graph JSON                                             │
│ Task: Topological sort, recognize synergies, resolve conflicts│
│ Output: Execution sequence JSON                               │
│                                                               │
│ Decision: "Both need corridor → inject corridor\_connector"    │
│ Sequence: \[corridor\_1, food\_1, housing\_1]                     │
└────────────────┬──────────────────────────────────────────────┘
                 │
                 │ Parse JSON response
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ GraphExecutor (Node → Command Converter)                    │
│                                                              │
│ corridor\_1 (corridor\_connector) → DIG (50,30,125 to 60,30,125)│
│ food\_1 (farm\_plot) → DIG (40,30,125 to 50,40,125)            │
│ housing\_1 (bedroom\_cluster) → DIG (60,30,125 to 70,40,125)   │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Send CommandMessage\[] to DFHack
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ DFHack Plugin                                                │
│ - Creates dig designations in DF                             │
│ - Sends ACK                                                  │
└────────────────┬────────────────────────────────────────────┘
                 │
                 │ Dwarves mine, next RESYNC
                 ▼
┌─────────────────────────────────────────────────────────────┐
│ Reality Updated                                              │
│ - Modification overlay detects new dug tiles                 │
│ - Chamber count increases (bedrooms detected)                │
│ - Next cycle: HousingAgent sees bedroom count = 7 ✅         │
└─────────────────────────────────────────────────────────────┘
```



---

## 

## Summary: What You Implemented



✅ **Reality State**: Same as before (entity cache, topology, modifications)
✅ **Agent Layer**: New Go code that analyzes metrics, creates proposals
✅ **Proposal Graph**: Temporary planning structure (0-10 nodes per cycle)
✅ **Arbiter**: Local LLM coordinates proposals via graph JSON
✅ **Executor**: Converts approved nodes → DFHack commands
✅ **Configuration**: Toggle between graph mode and direct-LLM mode



**Data Flow**:

* Reality (DF) → Cache (orchestrator) → Metrics (computed) → Agents (analyze) → Graph (proposals) → Arbiter (coordinate) → Executor (convert) → Commands (to DF)



**Not Implemented Yet**:

* Food/drink stock extraction from DF
* Bedroom zone tracking
* Mining activity tracking
* Camera repositioning command



**Key Insight**: The graph is NOT a game state mirror. It's a proposal queue. Agents don't "parse" it - they CREATE it. Arbiter doesn't "analyze fort state" - it coordinates PROPOSALS.



---



Does this clarify the architecture? Should I add more data extraction (food stocks, bedroom zones) or implement the camera repositioning feature first?

