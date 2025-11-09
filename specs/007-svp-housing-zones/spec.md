# Feature Specification: Spatial Validator Planner and Housing Zones

**Feature Branch**: `007-svp-housing-zones`
**Created**: 2025-11-09
**Status**: Draft
**Input**: User description: "Spatial Validator Planner model with HousingAgent zone support and blueprint integration"

## User Scenarios & Testing

### User Story 1 - SVP Establishes Fort Z-Level Organization (Priority: P1)

When fort first loads or AI connects, the Spatial Validator and Planner analyzes embark terrain slope, dwarf positions, and soil distribution to designate appropriate Z-levels for different fort functions (housing at Z-5 below embark, workshops at Z-6, farms at surface soil layer). These Z-level designations persist across sessions and guide agent proposal placement decisions, ensuring organized vertical fort structure instead of chaotic mixed-purpose levels.

**Why this priority**: Critical foundation for intelligent fort planning. Without Z-level organization, agents propose bedrooms, workshops, and farms on random levels creating disorganized, inefficient forts. SVP establishes coherent vertical structure that all agents respect.

**Independent Test**: Can be fully tested by connecting to new embark, observing SVP analyze terrain (logs show embark Z, slope detection, soil layer identification), verify SVP saves Z-level designations (housing_z: 120, workshop_z: 119, farm_z: 125), and confirm future agent proposals respect these levels. Delivers organized fort structure.

**Acceptance Scenarios**:

1. **Given** new fort embark at Z=125 with slope terrain and dwarves scattered at Z=123-125, **When** SVP analyzes terrain on first connection, **Then** SVP designates housing layer at Z=120 (5 levels below embark), workshop layer at Z=119, and detects soil at Z=124 for farming
2. **Given** SVP has established Z-level organization (housing: Z=120, workshops: Z=119), **When** HousingAgent proposes bedroom_cluster, **Then** proposal uses Z=120 (housing layer) instead of arbitrary Z-level
3. **Given** fort has filled housing layer Z=120 with bedrooms, **When** arbiter cannot fit more bedrooms without conflicts, **Then** arbiter requests SVP to designate additional housing layer at Z=118
4. **Given** SVP-designated layers saved to persistence, **When** orchestrator restarts and reconnects, **Then** SVP loads saved designations and continues enforcing Z-level organization
5. **Given** SVP detects aquifer at Z=115, **When** MiningAgent proposes exploratory shaft, **Then** SVP validates proposal avoids hazard Z-level and suggests safe alternative depth

---

### User Story 2 - HousingAgent Uses Real Bedroom Zone Data (Priority: P1)

System extracts actual DF bedroom zone counts from game state instead of using placeholder values, enabling HousingAgent to accurately detect bedroom deficits and propose appropriate housing expansions when dwarves lack assigned bedrooms.

**Why this priority**: Essential for HousingAgent to function correctly. Current implementation uses hardcoded bedroom count of zero, causing agent to always propose bedrooms (spam) or never propose (if placeholder is high). Real zone data makes agent responsive to actual housing needs.

**Independent Test**: Can be fully tested by creating 3 bedroom zones in DF manually, verifying HousingAgent sees BedroomCount=3 in metrics (logs show extracted zone count), triggering cycle with 7 dwarves, observing agent proposes 4 more bedrooms to cover deficit. Delivers functional housing monitoring.

**Acceptance Scenarios**:

1. **Given** fort has 3 designated bedroom zones and 7 dwarves, **When** zone data extracted from DF building module, **Then** system reports bedroom_zone_count: 3 and calculates deficit of 4 bedrooms needed
2. **Given** bedroom zone count is 5 and dwarf count is 5, **When** HousingAgent analyzes metrics, **Then** agent returns no proposals (housing target met, no deficit)
3. **Given** new migrant wave increases dwarf count from 7 to 12, **When** HousingAgent analyzes updated metrics, **Then** agent proposes bedroom expansion for 5 additional dwarves
4. **Given** zone extraction fails or returns error, **When** system attempts to compute fort metrics, **Then** logs warning and falls back to chamber count estimation (degraded but functional)

---

### User Story 3 - Arbiter Selects Blueprints for Proven Designs (Priority: P1)

Arbiter has access to blueprint library (bedroom_3x3.csv, bedroom_cluster_10.csv) in system prompt, enabling intelligent design selection when coordinating housing proposals. Instead of arbitrary rectangles, arbiter chooses appropriate pre-designed layouts that follow DF best practices for bedroom placement, corridor connectivity, and spatial efficiency.

**Why this priority**: Dramatically improves fort quality. Without blueprints, arbiter creates arbitrary rectangular digs. Blueprints encode human expertise (compact 3×3 bedrooms, noble clusters with dining halls, efficient corridor patterns). Essential for AI to build aesthetically pleasing, functionally sound forts.

**Independent Test**: Can be fully tested by triggering HousingAgent proposal (deficit exists), verifying arbiter system prompt includes available blueprints, observing arbiter response references blueprint choice ("using bedroom_3x3 layout"), and confirming executor applies blueprint pattern instead of simple rectangle. Delivers human-quality fort designs.

**Acceptance Scenarios**:

1. **Given** HousingAgent proposes bedroom_cluster for 4 dwarves, **When** arbiter evaluates proposal with blueprint library available, **Then** arbiter selects bedroom_3x3.csv blueprint and applies 4 instances at appropriate spacing
2. **Given** HousingAgent proposes noble bedrooms for 10 dwarves, **When** arbiter sees bedroom_cluster_10.csv blueprint (includes central dining hall), **Then** arbiter chooses cluster blueprint over individual bedrooms for superior design
3. **Given** available space constrained at housing Z-level, **When** arbiter evaluates bedroom proposal, **Then** arbiter adapts blueprint scale or suggests alternative compact blueprint that fits available space
4. **Given** blueprint requires 20×15 area but only 15×10 available, **When** arbiter attempts to apply blueprint, **Then** arbiter recognizes size mismatch and either scales blueprint proportionally or uses smaller alternative (bedroom_3x3 fallback)

---

### User Story 4 - Executor Creates Assignable Bedroom Zones (Priority: P1)

When executing bedroom_cluster nodes, executor sends both DIG commands (create room space) and ZONE commands (designate as bedroom) to DFHack plugin, creating rooms that dwarves can actually be assigned to instead of just empty dug chambers.

**Why this priority**: Core requirement for functional housing. Dug rooms without zone designations are useless - dwarves cannot be assigned, remain unhappy. Zone command execution converts abstract proposals into actual DF bedrooms that fulfill dwarf needs.

**Independent Test**: Can be fully tested by triggering bedroom proposal, observing executor logs show DIG + ZONE commands sent for each bedroom, verifying DF zones menu (z) shows new bedroom zones created, and confirming dwarves auto-assign to new bedrooms. Delivers complete housing functionality.

**Acceptance Scenarios**:

1. **Given** arbiter approves bedroom_cluster node at region (60,30,120), **When** executor processes node, **Then** executor sends DIG command followed by ZONE command (type: bedroom) for same region
2. **Given** DIG command creates room but ZONE command fails (invalid region), **When** executor detects ZONE failure, **Then** executor logs error, queues ZONE retry for next cycle when space confirmed available
3. **Given** blueprint specifies 4 bedrooms in cluster pattern, **When** executor applies blueprint, **Then** executor sends 4 DIG commands + 4 ZONE commands (one pair per bedroom), creating 4 assignable zones
4. **Given** ZONE command sent but space not yet dug (dwarf still mining), **When** zone designation attempted, **Then** DFHack returns error, executor queues ZONE for retry after dig completion verification

---

### User Story 5 - Deferred Zone Queue Handles Async Execution (Priority: P2)

When ZONE commands cannot be placed immediately (space not yet dug by dwarves), system queues zone designations and retries on subsequent cycles after verifying dig completion, ensuring zone assignments are not lost during multi-turn excavation.

**Why this priority**: Handles realistic timing - dwarves take time to dig, zones should be designated automatically once space available. Prevents lost zone commands and ensures housing proposals eventually complete. Important for robustness but not critical for initial functionality.

**Independent Test**: Can be tested by issuing bedroom DIG command, verifying ZONE queued when space unavailable, waiting for dwarf to dig tiles, observing ZONE automatically placed on next cycle after dig verified. Delivers reliable async zone placement.

**Acceptance Scenarios**:

1. **Given** DIG command sent for bedroom but dwarves still mining, **When** executor attempts ZONE designation, **Then** ZONE queued with status "waiting_for_dig", retry scheduled for next cycle
2. **Given** ZONE queued with "waiting_for_dig" status, **When** next cycle detects tiles now dug (modification overlay shows completion), **Then** executor resends ZONE command successfully, updates status to "completed"
3. **Given** queued ZONE fails 3 times over 3 cycles, **When** retry limit exceeded, **Then** executor logs failure, removes from queue, notifies that housing proposal incomplete
4. **Given** multiple ZONE commands queued (4 bedrooms), **When** space becomes available for 2 bedrooms only, **Then** executor places 2 zones successfully, keeps 2 queued for future cycles

---

### Edge Cases

- **What happens when SVP-designated housing layer conflicts with existing aquifer?**
  - SVP queries hazard manager before designation
  - Avoids water, lava, cavern Z-levels
  - Logs hazard conflict, selects alternative safe Z-level
  - If no safe levels available, warns user and defaults to dwarf Z-level

- **What happens when blueprint requires 20×15 space but only 10×10 available at housing Z?**
  - Arbiter recognizes size constraint from SVP spatial limits
  - Selects smaller blueprint alternative (bedroom_3x3 instead of cluster)
  - Or scales blueprint proportionally if possible
  - Logs blueprint adaptation decision

- **What happens when zone extraction from DF fails or returns corrupted data?**
  - System logs error with DF API details
  - Falls back to chamber count estimation (dug rooms as bedroom proxy)
  - Marks metrics as degraded quality
  - Continues operation with reduced accuracy until zone data restored

- **What happens when arbiter requests new Z-level designation but all levels occupied?**
  - SVP rejects request, logs "no available Z-levels for housing expansion"
  - Arbiter receives rejection, falls back to compact existing levels
  - May propose vertical expansion (dig down) to create new layer
  - Logs constraint to help debug fort space exhaustion

- **What happens when ZONE command queued but dwarf never completes dig (interrupted, killed)?**
  - Queue has 10-cycle timeout per ZONE
  - After timeout, executor checks dig status
  - If still incomplete, logs "dig abandoned", removes ZONE from queue
  - Housing deficit remains, agent proposes alternative location next cycle

- **What happens when blueprint file (bedroom_3x3.csv) is missing or corrupted?**
  - Arbiter sees blueprint unavailable in library
  - Falls back to geometric bedroom generation (3×3 grid pattern)
  - Logs missing blueprint warning
  - Continues operation with reduced design quality

---

## Requirements

### Functional Requirements

**Spatial Validator and Planner**:

- **FR-001**: System MUST implement SVP component that analyzes embark terrain on initial connection to determine optimal Z-level organization
- **FR-002**: SVP MUST detect embark Z-level from dwarf initial positions (mode of Z coordinates)
- **FR-003**: SVP MUST calculate housing layer as 5 Z-levels below embark Z (accounts for terrain slope to reach uniform horizontal layer)
- **FR-004**: SVP MUST designate workshop/stockpile layer immediately below housing layer (housing Z - 1)
- **FR-005**: SVP MUST detect soil layers suitable for farming via is_soil_layer bool flag per Z-level in topology data (200 bytes total, accurate without per-tile overhead)
- **FR-006**: SVP MUST query hazard manager to avoid designating housing/workshop layers at aquifer, lava, or cavern Z-levels
- **FR-007**: SVP MUST persist Z-level designations to disk (saved with fort name, restored on reconnection)
- **FR-008**: SVP MUST enforce Z-level organization when agents propose nodes (housing proposals must target housing Z-level)
- **FR-009**: SVP MUST support dynamic Z-level expansion when arbiter requests additional layers (housing overflow to new Z-level)
- **FR-010**: SVP MUST provide spatial validation for agent proposals (reject proposals in hazard zones, wrong Z-levels, or outside map bounds)

**Zone Extraction and Tracking**:

- **FR-011**: System MUST extract zone list from DF building module on each entity update
- **FR-012**: System MUST count zones by type (bedroom, dining hall, dormitory, office, barracks, workshop, stockpile)
- **FR-013**: System MUST include zone counts in fort metrics available to agents (bedroom_zones: 5, dining_zones: 1, etc.)
- **FR-014**: Zone extraction MUST distinguish between zone types accurately (bedroom ≠ dormitory ≠ dining hall)
- **FR-015**: Zone extraction MUST handle corrupted or unavailable zone data gracefully (fallback to chamber count estimation)
- **FR-016**: System MUST track zone assignment status (assigned to dwarf vs unassigned)

**HousingAgent Enhancement**:

- **FR-017**: HousingAgent MUST use real bedroom zone count from fort metrics instead of placeholder value
- **FR-018**: HousingAgent MUST query SVP for housing Z-level designation when proposing bedroom_cluster nodes
- **FR-019**: HousingAgent MUST calculate bedroom deficit as (dwarf_count - bedroom_zone_count), not (dwarf_count - chamber_count)
- **FR-020**: HousingAgent MUST propose bedroom layouts that account for walls and corridors (not just dwarf count × 3×3)
- **FR-021**: HousingAgent MUST have access to blueprint templates for sizing calculations (knows bedroom_3x3 requires 9 tiles, bedroom_cluster_10 requires 300 tiles)
- **FR-022**: HousingAgent MUST adjust urgency based on unassigned dwarf percentage (all dwarves unhoused = 1.0 urgency, half unhoused = 0.5)

**Blueprint Integration**:

- **FR-023**: System MUST load all blueprints from blueprints/ directory at startup (existing Feature 005 functionality)
- **FR-024**: Arbiter system prompt MUST include blueprint library with names, dimensions, and descriptions
- **FR-025**: Arbiter MUST have capability to select blueprints based on space constraints and dwarf count requirements
- **FR-026**: Arbiter MUST be able to scale or adapt blueprints when exact size unavailable (15×10 space gets scaled 20×15 blueprint)
- **FR-027**: System MUST support blueprint companion files: blueprint.csv (dig pattern using community-standard format) and blueprint_zones.csv (zone designations for each room in pattern)
- **FR-028**: Arbiter MUST send BLUEPRINT command to DFHack plugin with blueprint name when blueprint selected (uses proper quickfort-compatible CSV files)
- **FR-029**: DFHack plugin MUST load both blueprint.csv (dig pattern) and blueprint_zones.csv (zone designations), apply dig pattern first, then zones after dig completion
- **FR-030**: Plugin MUST parse zones CSV format: x,y,z,zone_type,width,height (defines zone region and type for each room in blueprint)
- **FR-031**: System MUST create zone CSV companion files for existing blueprints (bedroom_3x3.csv → bedroom_3x3_zones.csv with bedroom zone at 0,0,0 size 3×3)
- **FR-032**: Zone CSV MUST distinguish room purposes: bedroom zones for sleeping, dining zones for eating, corridor regions unmarked (no zone needed)
- **FR-033**: Plugin MUST validate blueprint and zones CSV match (all zone coordinates within blueprint dig bounds)

**Zone Command Execution** (when not using blueprints):

- **FR-034**: Executor MUST send both DIG and ZONE commands when processing bedroom_cluster nodes without blueprint (create space + designate simple rooms)
- **FR-035**: Executor MUST send ZONE commands with correct type parameter (bedroom, dining, dormitory, etc.)
- **FR-036**: Executor MUST queue ZONE commands for retry when space not yet available (dig in progress)
- **FR-037**: Zone queue MUST track retry count and timeout (maximum 10 cycles, 3 retry attempts)
- **FR-038**: Executor MUST verify dig completion before retrying queued ZONE commands (check modification overlay)
- **FR-039**: Executor MUST log ZONE command success/failure with zone type, coordinates, and assignment outcome

**Data Persistence**:

- **FR-044**: SVP Z-level designations MUST be saved to disk in saves/{fortname}/svp_layout.json
- **FR-045**: Queued ZONE commands MUST be saved to disk to survive orchestrator restarts
- **FR-046**: SVP MUST load previous designations on reconnection to same fort (maintain continuity)
- **FR-047**: System MUST handle fort name changes gracefully (save new layout, warn about previous layout orphaned)

### Key Entities

- **SpatialValidatorPlanner**: Represents the strategic Z-level organization component. Attributes: embark Z-level, housing Z-level, workshop Z-level, farm Z-levels (array), hazard Z-levels (avoid), soil regions (bounding boxes), water source locations, designation persistence path, last updated timestamp.

- **ZLevelDesignation**: Represents a designated purpose for a Z-level. Attributes: Z-level number, purpose type (housing/workshop/farm/storage/industrial), capacity status (space available/full), hazards (aquifer/lava/cavern flags), soil availability (boolean), notes (rationale for designation).

- **ZoneInfo**: Represents a DF zone extracted from game state. Attributes: zone type (bedroom/dining/dormitory/office/barracks), coordinates (bounding box), assignment status (assigned to which dwarf or unassigned), zone size (tile count), creation time.

- **ZoneQueue**: Represents queued zone designations awaiting dig completion. Attributes: zone type, target coordinates, associated dig command ID, retry count, max retries, timeout cycles, status (waiting_for_dig/retrying/failed), created timestamp.

- **BlueprintMetadata**: Represents blueprint information for arbiter context. Attributes: name (bedroom_3x3), dimensions (width, height, depth), tile count, description, tags (bedroom/compact/nobles), suitability criteria (min space required, dwarf capacity).

## Success Criteria

### Measurable Outcomes

- **SC-001**: SVP successfully designates Z-levels on 100% of new fort connections (housing, workshop, farm layers identified)
- **SC-002**: Agent proposals respect SVP Z-level organization in 95%+ of cases (bedrooms proposed at housing Z, not random levels)
- **SC-003**: HousingAgent accuracy improves from 0% (placeholder data) to 95%+ (proposes when deficit exists, silent when target met)
- **SC-004**: All dwarves receive assignable bedroom zones within 40 in-game days (measured from zone count reaching dwarf count)
- **SC-005**: Arbiter selects blueprints in 70%+ of housing proposals when appropriate blueprints available (vs arbitrary rectangles)
- **SC-006**: Zone queue success rate above 90% (queued zones eventually placed after dig completion)
- **SC-007**: SVP Z-level designations persist correctly across 100% of orchestrator restarts (save/load verification)
- **SC-008**: Fort organization shows clear vertical structure (housing layer distinct from workshop layer) in 80%+ of test forts
- **SC-009**: Bedroom zone extraction accuracy matches DF UI zone count in 100% of test cases (no false counts)
- **SC-010**: Zero lost zone commands (all queued zones eventually placed or explicitly failed with logged reason)

## Assumptions

- Feature 006 (graph-based agents, arbiter, executor) is complete and functional
- DFHack plugin has zone designation capability (CommandTypeZone protocol support exists)
- DF building module provides accessible zone list via API
- Blueprint CSV files exist in blueprints/ directory (Feature 005 legacy)
- ModificationOverlay chamber detection works correctly (existing Feature 005 functionality)
- Hazard manager provides Z-level hazard queries (existing Feature 004 functionality)
- Fort embark terrain has detectable slope (not perfectly flat surface embark)
- Soil layers exist at embark site (not all rock embarks) - if no soil, SVP marks farm_z as "none"
- SVP operates on first connection only for initial designation (not re-analyzed every cycle unless requested)
- Zone assignment is automatic in DF once zone designated (dwarves claim bedrooms)
- Executor has access to DFHack client for sending ZONE commands
- Persistence layer (saves/ directory) is writable and persistent across sessions

## Dependencies

**From Feature 006**:
- Graph-based agent infrastructure (ModificationNode, ProposalGraph)
- HousingAgent implementation (needs enhancement, not replacement)
- Arbiter system prompt and coordination logic
- GraphExecutor framework (needs ZONE command support)
- AgentRegistry and fort metrics computation
- Local LLM provider (for arbiter)

**From Feature 005**:
- Blueprint loading system (blueprints/ directory)
- Blueprint CSV parsing (LoadFromCSV)
- ModificationOverlay with chamber detection

**From Feature 004**:
- HazardManager (aquifer, lava, cavern detection)

**From Feature 003**:
- TopologyOverlay (embark terrain data)

**From Feature 001**:
- DFHack protocol (ZONE command type)

**External Dependencies**:
- DF building module API (for zone extraction)
- DF zone designation capability (create bedroom zones)

## Out of Scope

**Not Included in This Feature**:
- Other agent enhancements (FoodAgent, MiningAgent, WealthAgent remain placeholder-based)
- Food stock extraction from DF
- Comprehensive soil region extraction (SVP uses heuristic: embark Z-1 likely has soil)
- Water source extraction (wells, aquifer access points)
- Mining activity tracking (tiles dug per cycle)
- Wealth tracking from DF
- Material type extraction per tile (ore detection)
- Advanced blueprint generation (AI learning from successful designs)
- Sophisticated blueprint auto-scaling algorithms (just simple proportional scaling or fallback to smaller)
- Multi-Z-level bedroom expansion strategy (overflow bedrooms stay on same Z, just denser packing)
- Dynamic SVP re-analysis (once designated, Z-levels fixed unless manually requested via new command)
- Camera repositioning commands (separate feature)
- Full spatial classifier with pathfinding and traffic flow analysis

## Open Questions

**1. Soil Detection Strategy**: How should SVP detect soil layers for farm planning?

**Options**:

| Option | Answer | Implications |
|--------|--------|--------------|
| A | Heuristic: Assume embark surface and Z-1 have soil | Simple, no plugin changes, works for most embarks, fails on all-rock sites |
| B | Add is_soil_layer bool per Z-level to topology data | Lean (200 bytes), accurate, requires plugin change to extract material flags |
| C | Full material type extraction per tile | Accurate, heavy (36k+ tiles × material byte), massive protocol overhead |

**Suggested Answer**: Option B - Add is_soil_layer flag per Z-level. Provides accuracy for farm planning without per-tile overhead.

---

**2. SVP Analysis Timing**: When exactly does SVP perform initial terrain analysis?

**Options**:

| Option | Answer | Implications |
|--------|--------|--------------|
| A | On first ENTITY_UPDATE after connection | Automatic, has dwarf positions, may lack full topology if RESYNC delayed |
| B | On first RESYNC after topology loaded | Has complete terrain, may lack dwarf positions if ENTITY_UPDATE delayed |
| C | After both RESYNC and ENTITY_UPDATE received | Most data, requires state tracking (which arrived first), slight complexity |

**Suggested Answer**: Option C - Wait for both topology and entity data. Ensures SVP has complete information for analysis. Worth minor complexity.

---

**3. Blueprint Application Method**: How does executor apply blueprint when arbiter selects it?

**Options**:

| Option | Answer | Implications |
|--------|--------|--------------|
| A | Executor loads CSV, parses dig entries, sends multiple DIG+ZONE commands | Reuses existing code, explicit tracking, many commands per blueprint |
| B | Send single BLUEPRINT command with name, plugin handles application | Lean protocol, requires plugin enhancement, less command tracking |
| C | Arbiter outputs explicit dig pattern in JSON, executor just sends commands | No blueprint loading, arbiter does conversion, larger arbiter response tokens |

**Suggested Answer**: Option A - Executor loads and applies CSV. Reuses Feature 005 blueprint loading, no plugin changes, maintains explicit command tracking for debugging.

## Notes

**Architectural Rationale**:

SVP adds strategic planning layer missing from Feature 006:
- **Problem**: Agents propose locations arbitrarily (HousingAgent: "bedrooms at 60,30,125" - why 125?)
- **Solution**: SVP designates Z-levels by purpose, agents query SVP for appropriate Z
- **Benefit**: Organized vertical fort structure (all bedrooms on one level, workshops below)

**Why SVP Runs Once**:
- Fort organization should be stable (housing layer doesn't move)
- Re-analyzing every cycle wastes computation
- Dynamic expansion handled via arbiter requests (new layer on overflow)
- Simplifies implementation (one-time analysis vs continuous re-evaluation)

**Blueprint Integration Philosophy**:
- Blueprints are **teaching tools** for AI (show good designs)
- Arbiter has **choice** (blueprint vs custom)
- Not rigid templates (can adapt, scale, or ignore)
- Human designs encode best practices (compact, efficient, aesthetic)

**Zone Queue Rationale**:
- Dwarves take 5-30 seconds to dig room
- Zone must be designated AFTER dig complete
- Queue handles async timing without lost commands
- Alternative (polling) would spam failed ZONE commands

**Backward Compatibility**:
- SVP is optional new layer (doesn't break existing agents)
- HousingAgent still works without SVP (just uses hardcoded Z)
- Zone queue is optional (immediate ZONE works for pre-dug rooms)
- Existing chamber-based metrics remain as fallback

**From Session Context**:
- Feature 006 implemented graph infrastructure but agents use placeholder data
- HousingAgent currently proposes arbitrary rectangles with bedroom count always 0
- Blueprint system exists from Feature 005 but not accessible to arbiter
- Zone command protocol defined but executor doesn't use it
- User identified spatial awareness and zone support as critical gaps for agent functionality

## Design Decisions (Questions Resolved)

**1. Soil Detection**: is_soil_layer bool per Z-level (Option B)
- Lean: 200 bytes total (1 byte × 200 Z-levels)
- Accurate: Plugin extracts material flags per Z
- Sufficient: Farm placement just needs "this Z has soil" not per-tile data

**2. SVP Timing**: After both RESYNC and ENTITY_UPDATE received (Option C)
- Complete data: Has topology AND dwarf positions
- State tracking: Orchestrator waits for both messages before SVP analysis
- One-time cost: Slight complexity worth having full context

**3. Blueprint Application**: Plugin-based BLUEPRINT command (Option B)
- Uses community-standard quickfort CSV format
- Plugin handles dig pattern + zone designation
- Companion files: blueprint.csv (dig) + blueprint_zones.csv (zones)
- No orchestrator CSV parsing (plugin responsibility)

## Zone CSV Companion File Format

**Design Pattern**:
```
blueprints/
├── bedroom_3x3.csv          # Dig pattern (community standard)
├── bedroom_3x3_zones.csv    # Zone designations (new)
├── bedroom_cluster_10.csv   # Dig pattern
└── bedroom_cluster_10_zones.csv  # Zone designations (bedrooms + dining)
```

**Zones CSV Format**:
```csv
x,y,z,zone_type,width,height
0,0,0,bedroom,3,3
4,0,0,bedroom,3,3
8,0,0,bedroom,3,3
12,4,0,dining,6,6
```

**Interpretation**:
- x,y,z: Zone origin (relative to blueprint placement point)
- zone_type: bedroom, dining, dormitory, office, etc.
- width,height: Zone dimensions (depth always 1 for single Z-level zones)

**Plugin Behavior**:
1. Load blueprint.csv → Queue dig designations
2. Load blueprint_zones.csv → Queue zone commands
3. Execute digs first
4. Wait for dig completion (poll modification updates)
5. Execute zones when space available
6. Report completion or errors per zone

**Benefits**:
- Separates structure (dig) from semantics (zones)
- Same dig pattern can have different zone layouts
- Follows community quickfort standards for dig CSVs
- Extensible: Add metadata CSV for furniture placement later
