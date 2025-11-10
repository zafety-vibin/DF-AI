# Research & Technical Decisions: Spatial Validator Planner and Housing Zones

**Feature**: 007-svp-housing-zones
**Date**: 2025-11-09
**Status**: Complete

## Overview

This document records all technical decisions made during the design phase of Feature 007. Each decision is documented with the chosen approach, rationale, alternatives considered, and implications for implementation.

## Decision 1: SVP Z-Level Designation Algorithm

**Question**: How should SVP calculate appropriate Z-levels for different fort functions?

**Decision**: Heuristic-based designation using embark Z-level as reference point.

**Algorithm**:
```
1. Detect embark Z from dwarf initial positions (mode of Z coordinates)
2. Housing layer = embark_z - 5 (accounts for terrain slope)
3. Workshop layer = housing_z - 1 (immediately below housing)
4. Farm layer = first Z-level with is_soil_layer = true (prefer surface)
5. Query hazard manager to avoid aquifer/lava/cavern Z-levels
```

**Rationale**:
- Simple, predictable behavior for players to understand
- Embark - 5 reaches consistent horizontal layer below surface slope
- Stacking functions vertically (housing → workshops → farms) follows DF best practices
- Hazard avoidance prevents designating unusable layers

**Alternatives Considered**:
- **A. ML-based terrain analysis**: Would require training data, complex feature extraction. Overkill for initial version.
- **B. Pathfinding-based optimization**: Would analyze accessibility, traffic flow. Too computationally expensive for one-time analysis.
- **C. User-configured layers**: Would give control but require UI. Contradicts autonomous AI goal.

**Implications**:
- SVP requires topology data (for embark detection) and entity positions (for dwarf Z-levels)
- Must wait for both RESYNC and ENTITY_UPDATE before analysis
- Hazard manager integration needed (query API for Z-level hazards)
- Fails gracefully on flat embarks (embark_z - 5 may not reach uniform layer, but still provides organization)

---

## Decision 2: Soil Layer Detection Method

**Question**: How should SVP identify soil layers suitable for farm placement?

**Decision**: Per Z-level boolean flag (is_soil_layer) in topology data.

**Implementation**:
- DFHack plugin extracts material type flags per Z-level during RESYNC
- Array of 200 bools (1 per Z-level) sent in topology message
- Orchestrator queries array: `if topology.IsSoilLayer[z] { farm_z = z }`

**Rationale**:
- Lean: 200 bytes total (vs 36KB+ for per-tile material types)
- Sufficient: Farm placement just needs "this Z has soil" not per-tile data
- Accurate: Plugin has direct access to DF material flags
- Efficient: Sent once during RESYNC, no per-cycle overhead

**Alternatives Considered**:
- **A. Heuristic (assume surface has soil)**: Simple but fails on all-rock embarks, leads to failed farm proposals
- **B. Full material type extraction**: Accurate but 870 tiles/Z × 200 Z × 2 bytes = 348KB overhead
- **C. Request-based (query when needed)**: Would require new protocol message type, more complex

**Implications**:
- Plugin changes: Extract `tiletype_material` flags, aggregate per Z-level
- Protocol changes: Add `is_soil_layer[200]` field to RESYNC message (200 bytes)
- SVP may find no soil layers (all rock embark) → marks farm_z as -1 (none available)
- FoodAgent should check farm_z validity before proposing farms

---

## Decision 3: Zone Extraction API

**Question**: How should orchestrator extract zone data from Dwarf Fortress?

**Decision**: DF building module API via DFHack plugin.

**Approach**:
- Plugin iterates `df.global.world.buildings.other.ZONE` vector
- For each zone: Extract type, coordinates (x1,y1,z, x2,y2,z), assignment (owner dwarf ID or -1)
- Send zone list in ENTITY_UPDATE message (reuse existing entity sync)
- Orchestrator parses, counts by type, adds to fort metrics

**Rationale**:
- Direct access to DF's internal zone data structures (no reverse engineering)
- Zones already tracked by DF (bedrooms, dining halls, offices, etc.)
- Assignment status available (which dwarf owns bedroom)
- ENTITY_UPDATE is already sent every few seconds (no new message type)

**Alternatives Considered**:
- **A. Chamber detection heuristic**: Count dug rooms as proxy for bedrooms. Inaccurate (storage rooms counted as bedrooms).
- **B. Parse DF UI state**: Fragile, breaks on DF UI changes, requires screen scraping.
- **C. New protocol message (ZONE_UPDATE)**: Would batch zone data separately. Adds complexity, ENTITY_UPDATE sufficient.

**Implications**:
- Plugin changes: Add zone iteration logic to ENTITY_UPDATE handler
- Protocol changes: Extend EntityUpdate message with zone list (optional field, backward compatible)
- Zones sent every entity update (5-10 second interval) → fresh data for agents
- Zone count may lag behind reality (dwarf digs bedroom, takes 1 cycle to appear in metrics)

---

## Decision 4: Blueprint Metadata Format for Arbiter

**Question**: How should arbiter receive blueprint information for selection decisions?

**Decision**: Structured metadata in arbiter system prompt.

**Format**:
```
Available Blueprints:
- bedroom_3x3: Compact single bedroom (3×3 tiles, 1 dwarf capacity)
- bedroom_cluster_10: Noble cluster with central dining (20×15 tiles, 10 dwarf capacity, includes dining hall)
- dormitory_large: Emergency housing (15×10 tiles, 20 dwarf capacity, temporary solution)
```

**Token Budget**: ~400 tokens (5 blueprints × 80 tokens each)

**Rationale**:
- Arbiter needs name, dimensions, capacity to make informed decisions
- System prompt is ideal location (persistent across all cycles, no per-turn overhead)
- Structured format enables LLM to parse and reference blueprints
- Fits within local LLM context budget (<500 tokens)

**Alternatives Considered**:
- **A. JSON schema in user message**: Would repeat every cycle (200+ tokens/turn waste)
- **B. Full blueprint CSV in prompt**: Would exceed token budget (bedroom_cluster_10 is 300+ cells)
- **C. Arbiter requests blueprint on-demand**: Would require additional LLM call (latency, cost)

**Implications**:
- Blueprint loader must generate metadata (parse CSV, count tiles, infer capacity)
- Metadata generation happens at orchestrator startup (one-time cost)
- New blueprints require restart to appear in arbiter prompt (acceptable for research phase)
- Arbiter response includes blueprint name: `{"blueprint": "bedroom_3x3", "instances": 4}`

---

## Decision 5: Blueprint Application Method

**Question**: When arbiter selects blueprint, how does executor apply it?

**Decision**: BLUEPRINT command sent to DFHack plugin.

**Protocol**:
```
CommandType: BLUEPRINT
Parameters:
  - name: "bedroom_cluster_10"
  - origin_x, origin_y, origin_z: Placement coordinates
  - rotation: 0/90/180/270 (future enhancement, v1 = 0 only)
```

**Plugin Behavior**:
1. Load `blueprints/{name}.csv` (dig pattern, quickfort format)
2. Load `blueprints/{name}_zones.csv` (zone designations)
3. Queue dig designations from blueprint CSV
4. Queue zone commands from zones CSV
5. Execute digs, wait for completion, then execute zones
6. Report success/failure per zone

**Rationale**:
- Reuses community-standard quickfort CSV format (no custom parsing)
- Plugin has direct access to DF designation and zone APIs
- Single command vs hundreds of DIG+ZONE commands (cleaner protocol)
- Plugin can handle async timing (dig → wait → zone)

**Alternatives Considered**:
- **A. Orchestrator parses CSV, sends DIG+ZONE commands**: Would work but hundreds of commands per blueprint, complex tracking
- **B. Arbiter outputs explicit dig pattern in JSON**: Would require arbiter to understand CSV format, larger response tokens
- **C. Pre-compile blueprints to binary format**: Optimization not needed, CSV parsing is fast (<1ms)

**Implications**:
- Plugin changes: Implement BLUEPRINT command handler, CSV parsing, async zone execution
- Blueprint CSV format must follow quickfort standard (`,` delimited, `d` = dig, `#` = wall, etc.)
- Zone CSV format: Custom (x,y,z,zone_type,width,height)
- Executor just sends single BLUEPRINT command (simpler than Feature 006 multi-command approach)

---

## Decision 6: Zone CSV Companion File Format

**Question**: How should zone designations be represented alongside dig blueprints?

**Decision**: Separate CSV file with zone metadata.

**Format**:
```csv
x,y,z,zone_type,width,height
0,0,0,bedroom,3,3
4,0,0,bedroom,3,3
8,0,0,bedroom,3,3
12,4,0,dining,6,6
```

**Interpretation**:
- x,y,z: Zone origin relative to blueprint placement point
- zone_type: bedroom, dining, dormitory, office, barracks, etc.
- width,height: Zone dimensions (depth always 1 for single Z-level zones)

**Rationale**:
- Separates structure (what to dig) from semantics (what it's for)
- Same dig pattern can have different zone layouts (bedroom cluster vs dormitory)
- CSV format easy to hand-author (no complex syntax)
- Plugin validates zones fit within blueprint bounds

**Alternatives Considered**:
- **A. Embed zones in dig CSV as metadata**: Would break quickfort compatibility
- **B. Separate JSON file for zones**: Would require JSON parser in plugin (C++ complexity)
- **C. Hardcode zone rules (all 3×3 rooms = bedrooms)**: Too rigid, fails for dining halls, offices

**Implications**:
- Blueprint creators must author both files (bedroom_3x3.csv + bedroom_3x3_zones.csv)
- Plugin must validate zone coordinates within blueprint bounds
- Zone types must match DF enum (bedroom=0, dining=1, etc.) → document mapping
- Corridors have no zones (just dug, no zone CSV entry)

---

## Decision 7: Zone Queue Implementation

**Question**: How should system handle zone commands when space not yet dug?

**Decision**: In-memory queue with retry logic and timeout.

**Data Structure**:
```go
type QueuedZone struct {
    ZoneType     protocol.ZoneType
    Region       protocol.Region
    CommandID    uint32  // Associated dig command
    RetryCount   int
    MaxRetries   int     // Default: 3
    TimeoutCycles int    // Default: 10
    Status       string  // "waiting_for_dig", "retrying", "failed"
    CreatedAt    time.Time
}
```

**Behavior**:
1. Executor attempts ZONE command
2. If DFHack returns "space not available" → queue zone
3. Each autonomous cycle: Check modification overlay for dig completion
4. If tiles now dug → retry ZONE command
5. If success → remove from queue, log completion
6. If still failing after 3 retries or 10 cycles → log failure, remove from queue

**Rationale**:
- Handles realistic timing (dwarves take 5-30 seconds to dig)
- Prevents lost zone commands (explicit tracking until completion)
- Timeout prevents infinite queue growth (stuck dwarves, interrupted digs)
- Retry limit prevents spam (3 attempts sufficient to handle transient failures)

**Alternatives Considered**:
- **A. Polling (retry every cycle until success)**: Would spam DFHack with failed commands, log noise
- **B. Event-based (dig completion triggers zone)**: Would require plugin to track dig tasks, complex state management
- **C. No queue (accept zone failures)**: Would lose zone designations, bedrooms never created

**Implications**:
- Zone queue persisted to disk (survive orchestrator restarts) → saves/{fortname}/zone_queue.json
- Modification overlay queried each cycle (check dig completion) → existing functionality
- Zone success rate metric: `zones_placed / (zones_placed + zones_failed)`
- User may see delay between dig designation and zone appearance (expected, logged)

---

## Decision 8: HousingAgent Enhancement Strategy

**Question**: How should HousingAgent integrate SVP and zone data?

**Decision**: Enhance existing agent with new data sources, preserve existing logic structure.

**Changes**:
1. **Metrics Input**: Use `metrics.BedroomZoneCount` instead of placeholder 0
2. **SVP Query**: Call `svp.GetHousingZ()` for proposal Z-level instead of hardcoded
3. **Deficit Calculation**: `deficit = dwarf_count - bedroom_zone_count` (was: dwarf_count - 0)
4. **Urgency Adjustment**: `urgency = float64(deficit) / float64(dwarf_count)` (percentage unhoused)

**Backward Compatibility**:
- If SVP not initialized (use_svp: false) → use hardcoded Z-level (embark - 5)
- If zone extraction fails → fall back to chamber count estimation
- Agent works in degraded mode without new data

**Rationale**:
- Minimal changes to existing agent (just data sources, not logic)
- Preserves Feature 006 graph-based proposal structure
- Enables A/B testing (with vs without SVP/zones)
- Graceful degradation maintains functionality

**Alternatives Considered**:
- **A. Rewrite HousingAgent from scratch**: Would risk breaking existing graph integration
- **B. Create separate HousingAgentV2**: Would duplicate logic, increase maintenance burden
- **C. Hardcode SVP requirement**: Would break backward compatibility with Feature 006

**Implications**:
- Agent requires SVP and ZoneExtractor dependencies (injected via constructor)
- Unit tests need mock SVP (returns test Z-levels) and mock zones (returns test counts)
- Configuration controls new features (use_svp: true, extract_zones: true)
- Logs show data source: "Using SVP housing Z=120" vs "Using hardcoded Z=120"

---

## Decision 9: SVP Persistence Strategy

**Question**: How should SVP Z-level designations persist across sessions?

**Decision**: JSON file per fort in saves/{fortname}/ directory.

**File Format** (svp_layout.json):
```json
{
  "fort_name": "Doomfortress",
  "embark_z": 125,
  "housing_z": 120,
  "workshop_z": 119,
  "farm_z": [124, 125],
  "hazard_z": [115, 100],
  "analyzed_at": "2025-11-09T14:32:00Z",
  "version": "1.0"
}
```

**Behavior**:
- SVP saves after initial terrain analysis (one-time write)
- On reconnection to same fort: Load saved layout, skip re-analysis
- On connection to new fort: Analyze terrain, save new layout
- Fort name from DF world data (unique identifier)

**Rationale**:
- Fort organization should be stable (housing layer doesn't move mid-game)
- Persistence enables continuity across orchestrator restarts
- Per-fort files prevent conflicts (different forts have different layouts)
- JSON format human-readable for debugging

**Alternatives Considered**:
- **A. In-memory only (re-analyze on restart)**: Would waste computation, risk inconsistent designations
- **B. SQLite database**: Overkill for simple key-value data, adds dependency
- **C. Global config file (all forts in one JSON)**: Would risk corruption, harder to debug

**Implications**:
- saves/ directory must exist and be writable
- Fort name collision handling (user renames fort) → log warning, create new file
- Version field enables future migration (layout format changes)
- Manual editing supported (users can override SVP designations)

---

## Decision 10: SVP Timing and Trigger Logic

**Question**: When exactly should SVP perform terrain analysis?

**Decision**: After both RESYNC and ENTITY_UPDATE received on first connection.

**State Machine**:
```
State: WAITING_FOR_TOPOLOGY
  On RESYNC received:
    - Load topology data
    - Transition to WAITING_FOR_ENTITIES

State: WAITING_FOR_ENTITIES
  On ENTITY_UPDATE received:
    - Extract dwarf positions
    - Check if SVP layout exists for this fort
      - If exists: Load from disk
      - If not: Trigger SVP analysis
    - Transition to READY

State: READY
  - SVP available for agent queries
  - No further analysis unless manually requested
```

**Rationale**:
- SVP needs complete data: topology (for embark terrain) AND entity positions (for dwarf Z-levels)
- Messages may arrive in either order (DF timing varies)
- State tracking ensures both messages received before analysis
- One-time analysis is sufficient (fort layout stable)

**Alternatives Considered**:
- **A. Analyze on first ENTITY_UPDATE**: May not have topology data yet (RESYNC delayed)
- **B. Analyze on first RESYNC**: May not have dwarf positions yet (ENTITY_UPDATE delayed)
- **C. Re-analyze every N cycles**: Wastes computation, fort organization doesn't change

**Implications**:
- Autonomous loop tracks SVP state (enum: WAITING_FOR_TOPOLOGY/ENTITIES/READY)
- Agents must check SVP ready state before querying (graceful degradation if not ready)
- Logs show SVP initialization sequence: "Topology received, waiting for entities..." → "Entities received, analyzing terrain..."
- Manual re-analysis command could be added later (for testing, debugging)

---

## Decision 11: Arbiter Blueprint Selection Capability

**Question**: How should arbiter decide when to use blueprints vs geometric generation?

**Decision**: Arbiter autonomy with blueprint metadata guidance.

**System Prompt Addition**:
```
When coordinating housing proposals, you may select from available blueprints or use geometric patterns.

Blueprints (proven designs):
- bedroom_3x3: Compact efficiency (1 dwarf)
- bedroom_cluster_10: Noble quarters with dining (10 dwarves)

Use blueprints when:
- Space constraints match blueprint dimensions
- Dwarf count matches blueprint capacity
- Housing quality important (nobles, established fort)

Use geometric patterns when:
- No blueprint fits available space
- Emergency housing needed (speed over aesthetics)
- Experimental layout desired
```

**Response Format**:
```json
{
  "execution_sequence": [...],
  "blueprint_used": "bedroom_cluster_10",
  "instances": 2,
  "rationale": "Fort has 18 dwarves, 2 clusters provide 20 capacity with dining halls"
}
```

**Rationale**:
- LLM can reason about spatial constraints and dwarf needs
- Blueprints are suggestions, not requirements (arbiter has choice)
- Rationale logging enables analysis of blueprint selection patterns
- Flexibility supports experimentation

**Alternatives Considered**:
- **A. Hardcode blueprint selection rules**: "If deficit > 10, use cluster blueprint". Too rigid, doesn't adapt to space constraints.
- **B. Always use blueprints**: Would fail when no blueprint fits available space
- **C. Never use blueprints (always geometric)**: Would miss opportunity to encode human expertise

**Implications**:
- Arbiter must understand blueprint dimensions and constraints
- Blueprint metadata in system prompt enables spatial reasoning
- Success metric: Blueprint usage rate (target 70%)
- Can experiment with different blueprints (add new designs, measure arbiter preferences)

---

## Decision 12: Plugin BLUEPRINT Command Protocol

**Question**: What information does plugin need to apply blueprint?

**Decision**: Minimal command with name and placement coordinates.

**Protocol Message**:
```protobuf
message CommandMessage {
  CommandType type = 1;  // BLUEPRINT
  uint32 command_id = 2;

  // For BLUEPRINT type:
  string blueprint_name = 10;
  uint32 origin_x = 11;
  uint32 origin_y = 12;
  uint32 origin_z = 13;
  // Rotation future: uint32 rotation = 14;
}
```

**Plugin Behavior**:
```cpp
1. Parse blueprint_name ("bedroom_cluster_10")
2. Load blueprints/bedroom_cluster_10.csv
3. Load blueprints/bedroom_cluster_10_zones.csv
4. Parse blueprint CSV (quickfort format)
5. For each cell with 'd': Queue dig designation at (origin + offset)
6. Parse zones CSV
7. For each zone: Queue zone command at (origin + zone coords)
8. Return success/failure with error details
```

**Rationale**:
- Simple protocol (just name + placement)
- Plugin handles file I/O and parsing (C++ has filesystem access)
- Blueprint files remain on plugin side (no network transfer)
- Extensible (rotation, mirroring can be added later)

**Alternatives Considered**:
- **A. Send full blueprint data in command**: Would require large protobuf messages (300+ cells), network overhead
- **B. Pre-load blueprints at plugin startup**: Would work but complicates plugin initialization
- **C. Executor sends individual DIG commands**: Would work but hundreds of commands per blueprint

**Implications**:
- Plugin must have access to blueprints/ directory
- Blueprint files must be present on DF machine (not orchestrator machine if separate)
- Plugin returns detailed error: "Blueprint not found", "Invalid CSV format", "Zone out of bounds"
- Command ID tracking enables async completion monitoring (dig finishes later)

---

## Summary of Key Technologies

**Go Packages**:
- internal/spatial: SVP implementation (terrain analysis, Z-level designation, validation)
- internal/zones: Zone extraction (DF API queries, queue management)
- Enhanced agents, autonomous, config packages

**DFHack Plugin (C++)**:
- BLUEPRINT command handler (CSV parsing, dig/zone execution)
- Zone extraction from df.global.world.buildings.other.ZONE
- Soil layer detection from tiletype_material flags

**Data Formats**:
- Blueprint CSV: Community quickfort format (`,` delimited, `d`/`#` symbols)
- Zone CSV: Custom format (x,y,z,zone_type,width,height)
- SVP persistence: JSON (saves/{fortname}/svp_layout.json)

**Performance Targets**:
- SVP analysis: <200ms (one-time)
- Zone extraction: <50ms (per entity update)
- Zone queue processing: <10ms (per autonomous cycle)
- Blueprint metadata generation: <5ms (startup)

**Integration Points**:
- HousingAgent → SVP (GetHousingZ query)
- Autonomous loop → ZoneExtractor (fort metrics computation)
- Executor → ZoneQueue (ZONE command retry)
- Arbiter → Blueprint metadata (system prompt)
- Plugin → DF building module (zone API)
- Plugin → DF designation API (BLUEPRINT execution)
