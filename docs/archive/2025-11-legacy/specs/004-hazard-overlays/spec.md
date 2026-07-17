# Feature Specification: Hazard Overlays

**Feature Branch**: `004-hazard-overlays`
**Created**: 2025-11-06
**Status**: Draft
**Input**: User description: "Hazard Overlays: Implement sparse storage and detection for multiple hazard types as independent XYZ overlays. Track aquifer tiles, water (standing/flowing), lava, caverns, enemies, and dwarves. Each hazard type stored separately as sparse map (only hazard coordinates tracked, not full 6.9M tiles). Detect hazards from tile data and entity lists. Update overlays incrementally from TILE_UPDATE. Memory target: <50 KB total for all hazard overlays combined (sparse storage scales with hazard count, not total tiles)."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Environmental Hazard Detection (Priority: P1)

When the AI analyzes the fort for planning decisions, it needs to know which tiles contain environmental hazards (aquifer, water, lava, caverns) to avoid catastrophic mistakes like breaching into water sources or digging into magma. Each hazard type should be stored as a separate sparse overlay that only tracks hazardous coordinates, not the entire 6.9M tile space.

**Why this priority**: Environmental hazards are immediately dangerous and can destroy a fort in seconds (aquifer breach, magma flooding). The AI must know where these hazards are before making ANY digging or construction decisions. This is the critical safety foundation.

**Independent Test**: Load a map with aquifers, underground water, magma sea, and cavern layers. Build hazard overlays from tile data. Query known hazard coordinates and verify correct hazard types detected. Measure total memory usage for all environmental hazard overlays combined, confirm under 30 KB.

**Acceptance Scenarios**:

1. **Given** a map with 500 aquifer tiles scattered across 6.9M tiles, **When** building the aquifer overlay, **Then** memory usage is proportional to hazard count (not total tiles), approximately 20-30 bytes per hazard coordinate
2. **Given** tile data with water tiles (standing and flowing), **When** detecting water hazards, **Then** both water types are identified and stored in the water overlay with flow state tracked
3. **Given** a map with magma sea at Z-levels 0-10, **When** building the lava overlay, **Then** all lava tiles are detected and memory usage scales with lava tile count (approximately 10-20 KB for typical magma sea)

---

### User Story 2 - Entity Hazard Tracking (Priority: P2)

When the AI makes decisions, it needs real-time awareness of dynamic hazards: hostile creatures and friendly dwarves. Unlike environmental hazards (mostly static), entity positions change constantly as creatures move and dwarves work. These overlays must track current entity positions for collision avoidance and dwarf safety.

**Why this priority**: Entity tracking prevents the AI from issuing commands that endanger dwarves or ignore enemy positions. While less immediately catastrophic than aquifer breaches, entity awareness is essential for practical fort management and dwarf safety.

**Independent Test**: Load a fort with 50 dwarves and 10 hostile creatures. Build entity overlays from entity position data. Verify all dwarves and enemies are tracked with current coordinates. Update overlays when entities move, verify positions stay synchronized.

**Acceptance Scenarios**:

1. **Given** a fort with 50 dwarves at various positions, **When** building the dwarf overlay, **Then** all 50 dwarf positions are stored with memory usage under 5 KB (sparse storage for ~50 coordinates)
2. **Given** 10 hostile creatures in the fort, **When** building the enemy overlay, **Then** all enemy positions and types are tracked separately from dwarves
3. **Given** entities that move between updates, **When** receiving position updates, **Then** the overlays update entity coordinates to reflect current positions

---

### User Story 3 - Incremental Hazard Updates (Priority: P3)

When tiles change in DF (dwarves dig into aquifer, water flows into new tiles, magma breaches a wall), the hazard overlays must update to reflect the new hazard state. Updates should be incremental (only changed tiles processed) rather than rebuilding entire overlays.

**Why this priority**: Incremental updates keep overlays synchronized with minimal overhead. While overlays could be rebuilt from scratch on each FULL_STATE, incremental updates from TILE_UPDATE messages are more efficient and keep the AI's view current as the game progresses.

**Independent Test**: Start with hazard overlays built from initial state. Trigger tile changes (dig into aquifer, breach magma chamber). Send TILE_UPDATE messages. Verify hazard overlays update to add new hazards or remove resolved ones. Measure update latency for 100 changed tiles.

**Acceptance Scenarios**:

1. **Given** an aquifer overlay built from initial state, **When** receiving TILE_UPDATE showing a tile changed from rock to aquifer (due to digging), **Then** the aquifer overlay adds that coordinate to the hazard list
2. **Given** a water overlay tracking 200 water tiles, **When** receiving TILE_UPDATE showing water dried up on 10 tiles, **Then** those 10 coordinates are removed from the water overlay
3. **Given** 100 tiles changing hazard state, **When** processing TILE_UPDATE incrementally, **Then** all overlay updates complete in under 10 milliseconds

---

### Edge Cases

- What happens when the same tile has multiple hazard types (water + cavern edge)? (Store in both overlays independently, queries can check multiple overlays)
- How does the system handle hazards disappearing (water evaporates, aquifer drained)? (Remove from overlay when tile changes to non-hazardous type)
- What if entity positions are unavailable or incomplete? (Log warning, build overlay with available data, gracefully handle missing entities)
- How do entity overlays handle entity death/spawning? (Add on spawn, remove on death, update on movement)
- What happens when hazard count exceeds memory budget (>10k aquifer tiles)? (Log warning, store all hazards anyway, track actual memory usage for monitoring)
- How are flowing vs standing water differentiated? (Use DF liquid level and flow flags from tile data, store water type in overlay metadata)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST implement sparse storage for each hazard type where only hazardous coordinates are stored (not full tile array)
- **FR-002**: System MUST provide separate overlays for six hazard types: aquifer, water, lava, caverns, enemies, dwarves
- **FR-003**: System MUST detect aquifer tiles from tile data flags or material properties
- **FR-004**: System MUST detect water tiles and differentiate between standing water (depth > 0, no flow) and flowing water (flow flags set)
- **FR-005**: System MUST detect lava/magma tiles from tile material type
- **FR-006**: System MUST detect cavern tiles (underground open space with specific material or designation flags)
- **FR-007**: System MUST track enemy entity positions from entity list data (coordinates, entity type, unit ID)
- **FR-008**: System MUST track dwarf entity positions from entity list data (coordinates, unit ID, current task if available)
- **FR-009**: Each hazard overlay MUST support query operation: Contains(x, y, z) returning boolean in under 10 microseconds
- **FR-010**: System MUST support GetHazardsInRegion(xMin, xMax, yMin, yMax, zMin, zMax) for bounding box queries returning all hazards in region
- **FR-011**: System MUST update hazard overlays incrementally from TILE_UPDATE messages (add/remove hazards as tiles change)
- **FR-012**: Entity overlays (enemies, dwarves) MUST support position updates when entities move
- **FR-013**: Total memory usage for all six hazard overlays combined MUST be under 50 KB for typical forts (assumes <5000 total hazard coordinates)
- **FR-014**: System MUST log hazard counts for each type (aquifer: 500, water: 200, etc.) for monitoring and debugging
- **FR-015**: Hazard detection MUST be thread-safe to support concurrent queries from topology, pathfinding, and other systems

### Key Entities

- **HazardOverlay**: Generic sparse storage for a single hazard type. Stores only coordinates where hazards exist (not full 6.9M tiles). Supports fast Contains(x,y,z) lookups and region queries. Each hazard type (aquifer, water, lava, etc.) has its own HazardOverlay instance.

- **AquiferHazard**: Overlay tracking aquifer tiles. Detected from tile flags or material properties. Static (doesn't change unless tiles are mined). Critical for preventing catastrophic water breaches.

- **WaterHazard**: Overlay tracking water tiles with flow state (standing vs flowing). Detected from liquid level and flow flags. Updates as water spreads/drains. Includes depth information for flood risk assessment.

- **LavaHazard**: Overlay tracking lava/magma tiles. Detected from material type. Critical for preventing magma flooding and dwarf incineration.

- **CavernHazard**: Overlay tracking cavern tiles (underground open space). Detected from material or biome flags. Helps AI understand natural cave systems vs mined areas.

- **EnemyHazard**: Overlay tracking hostile creature positions. Detected from entity list. Updates as creatures move. Includes entity type and ID for threat assessment.

- **DwarfPosition**: Overlay tracking friendly dwarf positions. Technically not a "hazard" but tracked similarly for collision avoidance and dwarf safety. Updates as dwarves move. Includes unit ID and current task if available.

- **Coordinate**: 3D position (x, y, z) used as key in sparse storage. Validated against map bounds.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Total memory usage for all six hazard overlays combined is under 50 KB for maps with up to 5000 total hazard coordinates
- **SC-002**: Aquifer overlay stores 500 aquifer tiles using less than 15 KB of memory
- **SC-003**: Water overlay stores 200 water tiles with flow state using less than 10 KB of memory
- **SC-004**: Entity overlays (enemies + dwarves) store up to 100 entities using less than 5 KB combined
- **SC-005**: Hazard query Contains(x,y,z) completes in under 10 microseconds per query
- **SC-006**: Region query GetHazardsInRegion for 1000-tile bounding box completes in under 100 microseconds
- **SC-007**: Incremental updates from TILE_UPDATE processing 100 changed tiles complete in under 10 milliseconds
- **SC-008**: All hazard overlays support concurrent read access without data races
- **SC-009**: Hazard detection accuracy: 100% of aquifer tiles correctly identified, 95%+ of water/lava tiles detected

### Assumptions

- Aquifer tiles are identifiable from DF tile flags or material properties (may require DFHack API documentation)
- Water vs lava differentiated by liquid type flag in tile data
- Standing vs flowing water determined by flow vector flags in tile data
- Cavern tiles have specific material or biome markers distinguishing them from mined areas
- Entity position data is available from DFHack (may require additional protocol messages beyond tile data)
- Typical forts have <5000 total hazard coordinates (sparse relative to 6.9M total tiles)
- Entity positions change more frequently than environmental hazards (entities move every tick, environment is mostly static)
- Hazard overlays are read-heavy (many queries from pathfinding, few writes from updates)
- This is overlay layer #2-7 (six overlays) sharing XYZ coordinate space with topology overlay

### Out of Scope

- Hazard prediction or simulation (flow dynamics, aquifer breach radius) - that's Feature 9 (Safety Validation) if needed later
- Pathfinding around hazards (that's Feature 12 - Traffic/Pathfinding which will query these overlays)
- Hazard visualization or debugging tools (may add later)
- Historical hazard data (only current state maintained, not hazard change history)
- Complex entity AI behavior tracking (just positions, not creature intentions or pathfinding)
- Dwarf task management or job assignment (just positions for collision avoidance)
- Network transmission of hazard data (Feature 18 - Context Assembly handles LLM integration)
