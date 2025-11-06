# Feature Specification: Topology Overlay (First Spatial Layer)

**Feature Branch**: `003-topology-graph-layer`
**Created**: 2025-11-06
**Status**: Draft
**Input**: User description: "Topology Graph Layer: Implement bit-packed binary array storing open/closed state for all map tiles (1 bit per tile for 6M tiles) with Z-level indexed access and thread-safe concurrent reads. Add RLE compression to reduce topology for LLM context transmission with <10ms compression time. This provides the first spatial reasoning layer for AI decision-making."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Spatial State Persistence (Priority: P1)

When the orchestrator receives tile data from DFHack (6.9M tiles with XYZ coordinates, tiletype, material, etc.), it needs to extract and persist just the open/closed state in a queryable format. This first overlay enables spatial reasoning queries like "Is tile (x,y,z) walkable?" without repeatedly parsing the full 62 MB tile dataset.

**Why this priority**: This is the foundation for all spatial intelligence. The AI cannot make pathfinding or placement decisions without knowing which tiles are open versus closed. This is the absolute MVP - the first overlay that makes spatial data queryable.

**Independent Test**: Receive a FULL_STATE message with 6.9M tiles. Extract open/closed bit for each tile and store in the topology overlay. Query random coordinates and verify responses match the original tile data. Measure memory usage and confirm approximately 1 bit per tile (~870 KB for 6.9M tiles).

**Acceptance Scenarios**:

1. **Given** the orchestrator receives FULL_STATE with 6,967,296 tiles (192x192x189), **When** building the topology overlay, **Then** memory usage is under 1 MB (target: ~870 KB bit array)
2. **Given** the topology overlay is populated from tile data, **When** querying if tile (50, 50, 5) is open, **Then** the system returns correct open/closed state in under 1 microsecond
3. **Given** tiles with various TileTypes (walls, floors, open space), **When** storing in overlay, **Then** only open/pathable tiles are marked as open (1), all obstacles marked closed (0)

---

### User Story 2 - LLM Context Compression (Priority: P2)

When assembling context for LLM decision-making, the AI needs to "see" the entire fort's spatial layout. The topology overlay must be compressed from bit array form (~870 KB) to a representation under 125 KB using run-length encoding, making it suitable for inclusion in LLM context without consuming the entire context budget.

**Why this priority**: Without compression, the 870 KB topology would consume most of the 200 KB total context budget, leaving no room for hazards, traffic, or semantic layers. Compression is essential for multi-layer spatial reasoning.

**Independent Test**: Build a topology overlay for a typical fort with rooms, corridors, and mined areas. Compress using RLE. Verify compressed size under 125 KB. Decompress and verify lossless round-trip (matches original bit array).

**Acceptance Scenarios**:

1. **Given** a topology overlay for a 192x192x189 map with typical fort layout (10-30% open space), **When** compressing using RLE, **Then** compressed output is no larger than 125 KB
2. **Given** a topology overlay ready for compression, **When** measuring compression operation, **Then** it completes in under 10 milliseconds
3. **Given** compressed topology data, **When** decompressing and comparing to original, **Then** the decompressed bit array exactly matches the original (100% lossless)

---

### User Story 3 - Configurable Compression Modes (Priority: P3)

When the AI is focused on a specific area of the fort (active construction zone, current mining level), including the entire map topology in context wastes tokens. The system should support configurable compression modes: full map, active Z-levels only (±3 levels from center), or custom bounds.

**Why this priority**: Z-level filtering is an optimization that significantly reduces context usage but isn't strictly required for initial LLM integration. The system can function with full-map compression first, then add filtering to improve context efficiency.

**Independent Test**: Configure compression mode to "active_z_levels" with center Z=50, radius=3. Compress topology and verify only Z-levels 47-53 are included in output. Verify compressed size is proportionally smaller (~4% of full map for 7 of 189 levels).

**Acceptance Scenarios**:

1. **Given** compression mode configured as "full", **When** compressing topology, **Then** all Z-levels (0-189) are included in compressed output
2. **Given** compression mode configured as "active_z_levels" with center Z=50 and radius=3, **When** compressing, **Then** only Z-levels 47-53 are included, and compressed size is ~7/189 = 3.7% of full map size
3. **Given** compression mode configured as "custom_bounds" with specific XYZ range, **When** compressing, **Then** only tiles within the bounding box are included in compressed output

---

### Edge Cases

- What happens when map dimensions change between sessions? (Rebuild topology overlay with new dimensions, old data discarded)
- How does compression handle worst-case fragmentation (checkerboard pattern of open/closed)? (RLE degrades to ~full size, should still complete in <10ms, log compression ratio warning)
- What if compression exceeds 125KB target due to extreme fragmentation? (Allow overage, log warning, include in context anyway with budget tracking)
- How does system handle querying coordinates outside map bounds? (Return error with clear message, do not panic or corrupt state)
- What happens during concurrent reads while overlay is being rebuilt? (Use copy-on-write or read-write lock to prevent data races)
- How does Z-level filtering behave at map edges (center Z=2, radius=3 includes negative Z)? (Clamp to valid range: max(0, centerZ - radius) to min(maxZ, centerZ + radius))

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST extract open/closed state from received tile data and store in a bit-packed binary array (1 bit per tile)
- **FR-002**: System MUST determine "open" as any tile that is pathable (DF tiletype indicates floor, ramp, stairs, etc.) and "closed" as walls, obstacles, or undiggable rock
- **FR-003**: System MUST support map dimensions up to 256x256x256 tiles (16.7M tiles maximum)
- **FR-004**: System MUST organize bit array storage by Z-level for efficient per-level access and filtering
- **FR-005**: System MUST provide coordinate query operation: IsOpen(x, y, z) returning boolean in under 1 microsecond
- **FR-006**: System MUST provide region query operation: GetOpenTiles(xMin, xMax, yMin, yMax, zMin, zMax) returning count of open tiles
- **FR-007**: System MUST support concurrent read access from multiple goroutines without data races
- **FR-008**: System MUST implement run-length encoding (RLE) compression for the bit array
- **FR-009**: RLE compression MUST support three modes: "full" (all Z-levels), "active_z_levels" (center ± radius), "custom_bounds" (explicit XYZ range)
- **FR-010**: Compression mode MUST be configurable via YAML config file (topology_compression_mode, topology_center_z, topology_z_radius, topology_custom_bounds)
- **FR-011**: RLE compression MUST complete in under 10 milliseconds for full map (6.9M tiles)
- **FR-012**: Compressed output MUST be no larger than 125 KB for typical forts (10-30% open space) in full mode
- **FR-013**: System MUST provide decompression function for round-trip validation (lossless reconstruction)
- **FR-014**: System MUST track and log compression ratio (compressed size / uncompressed size) for monitoring
- **FR-015**: Coordinate queries MUST validate bounds and return error for out-of-range coordinates (no panic, no corruption)

### Key Entities

- **TopologyOverlay**: The first spatial overlay extracting open/closed state from raw tile data. Stores bit-packed array organized by Z-level. Each bit represents whether a tile is pathable (1) or blocked (0). Supports fast coordinate queries and region scans. This is NOT a graph with nodes/edges - it's a 3D binary array.

- **CompressedTopology**: RLE-encoded form of the topology overlay, suitable for LLM context inclusion. Contains compressed byte stream, original dimensions, compression mode used, and metadata (ratio, tile counts). Can be decompressed for validation. Size target: ≤125 KB.

- **CompressionConfig**: Configuration for how topology is compressed for LLM context. Specifies mode (full/active_z_levels/custom_bounds), center Z-level, radius, or explicit bounding box. Allows AI to focus on relevant spatial areas without wasting context on distant Z-levels.

- **Coordinate**: 3D position (x, y, z) used for all spatial queries. Validated against overlay dimensions before access to prevent out-of-bounds errors.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Topology overlay for a 192x192x189 map (6.9M tiles) uses less than 1 MB of memory (approximately 1 bit per tile)
- **SC-002**: Single tile queries (IsOpen) complete in under 1 microsecond per query
- **SC-003**: Region queries for 1000 tiles complete in under 100 microseconds
- **SC-004**: Compressed topology in "full" mode for a typical fort fits within 125 KB
- **SC-005**: Compression operation completes in under 10 milliseconds for 6.9M tile maps
- **SC-006**: Compression in "active_z_levels" mode (±3 Z) reduces context size to approximately 3-5% of full map
- **SC-007**: Round-trip test (compress → decompress → compare) completes in under 20 milliseconds with 100% match
- **SC-008**: System supports 100+ concurrent readers querying topology without performance degradation or data races
- **SC-009**: Operators can switch compression modes via config file change and hot-reload without restarting the server

### Assumptions

- Map dimensions are fixed after initialization (received from FULL_STATE, don't change during session)
- Tile open/closed state is determined by DF tiletype enum (floors, ramps, stairs = open; walls, rock = closed)
- Typical forts have 10-30% open space with spatial locality (rooms and corridors cluster, not random distribution)
- RLE compression exploits spatial coherence (consecutive open or closed tiles compress well)
- Read queries are 1000x more frequent than overlay rebuilds (optimized for read-heavy workload)
- Active construction typically happens within ±3 Z-levels of a focus area (justifies z-level filtering)
- LLM context budget is 200 KB total, topology overlay should consume ≤125 KB to leave room for other overlays
- This is the FIRST overlay - hazards, traffic, semantics will be added as additional overlays sharing the same XYZ coordinate space

### Out of Scope

- Graph data structure with nodes and edges (that's Feature 12 - Traffic/Pathfinding, not this overlay)
- Real-time updates from tile change events (that's Feature 5 - Topology Updates, separate feature)
- Storing tile attributes beyond open/closed (material, designation flags, etc. stay in raw tile data)
- Advanced compression beyond RLE (LZ4, zstd, etc. - RLE chosen for speed and simplicity)
- Persistence to disk (topology is in-memory, rebuilt from DFHack on server restart)
- Network transmission of compressed topology (Feature 18 - Context Assembly handles LLM integration)
- Neighbor/connectivity queries (A* pathfinding in Feature 12 will build actual graph from this overlay)
- Multiple concurrent writers (only one overlay rebuild at a time, many concurrent readers allowed)