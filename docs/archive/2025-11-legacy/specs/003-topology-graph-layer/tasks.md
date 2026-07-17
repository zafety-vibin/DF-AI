# Tasks: Topology Overlay (First Spatial Layer)

**Input**: Design documents from `/specs/003-topology-graph-layer/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Not requested in feature specification - NO test tasks included (benchmarks will be added in polish phase)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a single Go project with structure:
- `cmd/df-orchestrator/` - Entry point
- `internal/topology/` - NEW: First overlay package
- `tests/benchmark/` - Benchmark suite
- `config/` - Configuration files

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and directory creation

- [ ] T001 Create `internal/topology/` directory for overlay package
- [ ] T002 [P] Create `tests/benchmark/` directory if it doesn't exist (already created in Feature 2)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T003 Add `TopologyCompressionMode` string field to Config struct in `internal/config/config.go`
- [ ] T004 [P] Add `TopologyCenterZ` uint16 field to Config struct in `internal/config/config.go`
- [ ] T005 [P] Add `TopologyZRadius` uint16 field to Config struct in `internal/config/config.go`
- [ ] T006 Update `setDefaults()` method in `internal/config/config.go` to set TopologyCompressionMode="full" and TopologyZRadius=3 if unset
- [ ] T007 Update `config/orchestrator.yaml` to include topology compression settings (topology_compression_mode, topology_center_z, topology_z_radius)
- [ ] T008 [P] Create `internal/topology/tiletype.go` with `IsPathable(tileType uint16) bool` function using simplified heuristic (0=closed, 1-299=open, 300+=closed)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Spatial State Persistence (Priority: P1) 🎯 MVP

**Goal**: Extract and store open/closed state from tile data in bit-packed array (~870 KB for 6.9M tiles), enable fast coordinate queries (<1 μs).

**Independent Test**: Receive FULL_STATE with 6.9M tiles. Build topology overlay. Query random coordinates and verify responses match tile data. Measure memory usage confirms ~870 KB (1 bit per tile).

### Implementation for User Story 1

- [ ] T009 [US1] Create `internal/topology/overlay.go` with TopologyOverlay struct (data []byte, mu sync.RWMutex, width uint16, height uint16, depth uint16, totalTiles uint32, openTileCount uint32, buildTime time.Time, memoryBytes uint64)
- [ ] T010 [US1] Implement `NewTopologyOverlay(width, height, depth uint16) *TopologyOverlay` constructor in `internal/topology/overlay.go` to allocate bit array with Z-major byte-aligned layout
- [ ] T011 [US1] Calculate bit array size in `NewTopologyOverlay`: `bytes needed = (width * height * depth + 7) / 8` to round up to nearest byte
- [ ] T012 [US1] Implement `BuildFromTiles(tiles []protocol.TileState) error` in `internal/topology/overlay.go` to populate bit array from tile data using IsPathable() for each tile
- [ ] T013 [US1] Add bit-setting logic in BuildFromTiles: calculate `bitIndex = z * width * height + y * width + x`, then `data[bitIndex/8] |= (1 << (bitIndex%8))` if tile is open
- [ ] T014 [US1] Track openTileCount during BuildFromTiles by incrementing counter for each open tile
- [ ] T015 [US1] Set buildTime and memoryBytes fields after successful BuildFromTiles completion
- [ ] T016 [P] [US1] Create `internal/topology/query.go` with `IsOpen(x, y, z int16) (bool, error)` method implementation
- [ ] T017 [US1] Implement bounds validation in IsOpen: verify 0 ≤ x < width, 0 ≤ y < height, 0 ≤ z < depth, return error if out of bounds
- [ ] T018 [US1] Implement bit extraction in IsOpen: calculate bitIndex, extract bit from data array using `(data[bitIndex/8] >> (bitIndex%8)) & 1`, return as boolean
- [ ] T019 [US1] Add RLock/RUnlock around data access in IsOpen for thread-safe concurrent reads
- [ ] T020 [US1] Implement `GetOpenTileCount(xMin, xMax, yMin, yMax, zMin, zMax int16) (uint32, error)` in `internal/topology/query.go` for region queries
- [ ] T021 [US1] Add bounds validation for region query: validate all min ≤ max and within map dimensions
- [ ] T022 [US1] Implement region scan logic: iterate tiles in bounding box, call IsOpen for each, count open tiles
- [ ] T023 [P] [US1] Implement `GetDimensions() (uint16, uint16, uint16)` method in `internal/topology/overlay.go` returning width, height, depth
- [ ] T024 [P] [US1] Implement `GetMemoryUsage() uint64` method returning `len(data)` in bytes
- [ ] T025 [P] [US1] Implement `GetOpenPercentage() float64` method returning `float64(openTileCount) / float64(totalTiles) * 100`

**Checkpoint**: At this point, User Story 1 should be fully functional - overlay stores data, queries work, memory usage validated

---

## Phase 4: User Story 2 - LLM Context Compression (Priority: P2)

**Goal**: Implement RLE compression to reduce topology from ~870 KB to <125 KB for LLM context inclusion, with <10ms compression time and lossless round-trip.

**Independent Test**: Build topology for typical fort. Compress using RLE. Verify compressed size under 125 KB. Decompress and verify exact match with original.

### Implementation for User Story 2

- [ ] T026 [P] [US2] Create `internal/topology/compression.go` with CompressedTopology struct (Data []byte, OriginalWidth uint16, OriginalHeight uint16, OriginalDepth uint16, CompressedSize uint32, UncompressedSize uint32, CompressionRatio float64, Mode string, ZLevelsIncluded []uint16, CompressedAt time.Time)
- [ ] T027 [P] [US2] Create CompressionConfig struct in `internal/topology/compression.go` (Mode string, CenterZ uint16, ZRadius uint16)
- [ ] T028 [US2] Implement `Compress(overlay *TopologyOverlay, config CompressionConfig) (*CompressedTopology, error)` method in `internal/topology/compression.go`
- [ ] T029 [US2] Add RLE compression algorithm in Compress(): iterate overlay.data bytes, count consecutive identical bytes, emit (varint count, byte value) pairs
- [ ] T030 [US2] Implement varint encoding helper: write run counts as 1 byte (0-127) or 2 bytes (128+) with MSB flag
- [ ] T031 [US2] Calculate and store compression metadata: CompressedSize, UncompressedSize, CompressionRatio in CompressedTopology
- [ ] T032 [US2] Add 12-byte header to compressed output: [2:width][2:height][2:depth][1:mode][1:zLevelCount][4:dataLength]
- [ ] T033 [US2] Set CompressedAt timestamp and Mode field in CompressedTopology result
- [ ] T034 [US2] Implement `Decompress() ([]byte, error)` method in `internal/topology/compression.go` to reconstruct original bit array from RLE data
- [ ] T035 [US2] Add varint decoding in Decompress(): read 1 or 2 byte run counts based on MSB flag
- [ ] T036 [US2] Implement RLE expansion in Decompress(): for each (count, value) pair, write value count times to output byte array
- [ ] T037 [US2] Implement `Validate(original *TopologyOverlay) error` method to perform round-trip test: decompress and compare byte-by-byte with original.data
- [ ] T038 [US2] Add `GetSize() uint32` and `GetRatio() float64` getter methods to CompressedTopology in `internal/topology/compression.go`
- [ ] T039 [US2] Add compression ratio logging in Compress() - log warning if compressed size exceeds 125 KB target

**Checkpoint**: At this point, User Stories 1 AND 2 should both work - overlay queries functional, RLE compression working with lossless round-trip

---

## Phase 5: User Story 3 - Configurable Compression Modes (Priority: P3)

**Goal**: Support three compression modes (full, active_z_levels, custom_bounds) configurable via YAML to optimize LLM context usage for different AI focus areas.

**Independent Test**: Configure "active_z_levels" mode with center Z=50, radius=3. Compress and verify only Z-levels 47-53 included. Verify compressed size ~4% of full map.

### Implementation for User Story 3

- [ ] T040 [US3] Add mode validation in Compress(): check config.Mode is "full", "active_z_levels", or "custom_bounds", return error if invalid
- [ ] T041 [US3] Implement "full" mode logic in Compress(): include all Z-levels (0 to depth-1) in compression
- [ ] T042 [US3] Implement "active_z_levels" mode logic in Compress(): calculate Z range as [max(0, centerZ - radius), min(depth-1, centerZ + radius)]
- [ ] T043 [US3] Add Z-level slicing for filtered modes: extract only relevant Z-levels from overlay.data before RLE compression
- [ ] T044 [US3] Calculate byte offsets for Z-level filtering: `zStartByte = z * (width * height / 8)`, extract `width * height / 8` bytes per level
- [ ] T045 [US3] Populate ZLevelsIncluded array in CompressedTopology with list of Z-levels included in compressed output
- [ ] T046 [US3] Add Z-level list to compressed output header: write [2 * zLevelCount: included Z-levels] after header, before RLE data
- [ ] T047 [US3] Update Decompress() to handle filtered modes: reconstruct only included Z-levels, fill omitted levels with zeros
- [ ] T048 [US3] Add "custom_bounds" mode placeholder in Compress() with TODO comment (implement when bounding box config structure added)
- [ ] T049 [US3] Add compression mode to logs: include mode, Z-levels included, and context budget percentage in compression success log

**Checkpoint**: All user stories should now be independently functional - full compression, Z-filtered compression, and custom bounds stub ready

---

## Phase 6: Integration & Benchmarks

**Purpose**: Integrate overlay into main orchestrator and validate performance

- [ ] T050 Update `cmd/df-orchestrator/main.go` to import `internal/topology` package
- [ ] T051 Add global topology overlay variable in main.go: `var topologyOverlay *topology.TopologyOverlay`
- [ ] T052 Update FULL_STATE receive handler in main.go to call `topology.NewTopologyOverlay()` and `BuildFromTiles()`
- [ ] T053 Add topology build logging in main.go: log memory usage, open percentage, build time after BuildFromTiles
- [ ] T054 [P] Create `tests/benchmark/topology_memory_test.go` with BenchmarkTopologyMemory for three map sizes (small 48x48x20, medium 100x100x100, large 192x192x189)
- [ ] T055 [US1] Implement memory benchmark logic: create overlay, build from generated tiles, report B/op and verify ~1 bit per tile
- [ ] T056 [P] Create `tests/benchmark/topology_query_test.go` with BenchmarkTopologyQuery for IsOpen and region queries
- [ ] T057 [US1] Implement query benchmarks: IsOpen with random coordinates, GetOpenTileCount for 1000-tile regions, report ns/op
- [ ] T058 [P] Create `tests/benchmark/topology_compression_test.go` with BenchmarkTopologyCompression for full and filtered modes
- [ ] T059 [US2] Implement compression benchmarks: compress overlay with both modes, report compression time and output size
- [ ] T060 [US2] Add round-trip validation benchmark: compress → decompress → compare, report total time
- [ ] T061 Update `internal/http/metrics.go` to add topology metrics to MetricsSnapshot (memory, open percentage, compression stats)
- [ ] T062 Add TopologyMetrics struct in metrics.go: MemoryBytes uint64, OpenPercentage float64, LastBuildTime, LastCompression (size, ratio, mode)
- [ ] T063 Query topology overlay in metricsHandler and populate TopologyMetrics if overlay exists

**Checkpoint**: Integration complete - overlay builds automatically, benchmarks validate performance, metrics exposed

---

## Phase 7: Polish & Validation

**Purpose**: Final validation, documentation verification, and performance testing

- [ ] T064 [P] Add race detector test: `go test -race ./internal/topology/...` to verify no data races during concurrent access
- [ ] T065 [P] Add bounds validation unit tests in `internal/topology/overlay_test.go`: test out-of-bounds queries return errors
- [ ] T066 Add worst-case compression test: create checkerboard pattern overlay, verify compression still completes in <10ms
- [ ] T067 Run all benchmarks and verify thresholds: memory ~870 KB, query <1 μs, compression <10ms, compressed size <125 KB
- [ ] T068 Test config hot-reload for topology settings: modify topology_compression_mode in YAML, verify next compression uses new mode
- [ ] T069 [P] Verify quickstart.md code examples match actual API (BuildFromTiles, IsOpen, Compress usage)
- [ ] T070 Final integration test: Start server, connect DFHack, verify topology built, compress with all 3 modes, query metrics endpoint

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-5)**: All depend on Foundational phase completion
  - User Story 1 (Storage): Can start after Foundational - No dependencies on other stories
  - User Story 2 (Compression): Can start after Foundational - No dependencies (uses TopologyOverlay interface)
  - User Story 3 (Config Modes): Can start after Foundational - No dependencies
- **Integration (Phase 6)**: Depends on at least US1 and US2 being complete
- **Polish (Phase 7)**: Depends on all user stories and integration

### User Story Dependencies

- **User Story 1 (P1)**: Independent - can implement and test alone (storage + queries)
- **User Story 2 (P2)**: Independent - can implement and test alone (compression + decompression)
- **User Story 3 (P3)**: Independent - enhances US2 but testable separately (mode configuration)

**All user stories are independently testable per spec requirements.**

### Within Each User Story

- US1: TopologyOverlay struct → NewTopologyOverlay → BuildFromTiles → query methods (IsOpen, GetOpenTileCount)
- US2: CompressedTopology struct → Compress (RLE) → Decompress → Validate
- US3: Mode validation → Z-level filtering → mode-specific compression logic

### Parallel Opportunities

**Phase 2 (Foundational)**:
- T004, T005 can run in parallel with T003 (different fields in same struct)
- T008 can run in parallel with T003-T006 (different file)

**Phase 3 (US1)**:
- T016 (query.go) can start in parallel with T009-T015 (overlay.go) if interface is clear
- T023, T024, T025 can all run in parallel (simple getter methods)

**Phase 4 (US2)**:
- T026, T027 can run in parallel (struct definitions in same file but independent)

**Phase 6 (Integration)**:
- T054, T056, T058 can all run in parallel (different benchmark files)

**Phase 7 (Polish)**:
- T064, T065, T069 can run in parallel (different concerns)

**User Stories (after Foundational)**:
- All 3 user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch overlay.go and query.go creation together:
Task: "Create internal/topology/overlay.go with TopologyOverlay struct"
Task: "Create internal/topology/query.go with IsOpen method"

# Later, launch all getter methods together:
Task: "Implement GetDimensions in overlay.go"
Task: "Implement GetMemoryUsage in overlay.go"
Task: "Implement GetOpenPercentage in overlay.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (create directories)
2. Complete Phase 2: Foundational (extend config, add IsPathable helper)
3. Complete Phase 3: User Story 1 (bit array storage + queries)
4. **STOP and VALIDATE**: Build overlay from real DFHack data, query coordinates, verify memory usage
5. Deploy/demo - spatial queries working, foundation for pathfinding

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test overlay storage independently → Queries working! (MVP)
3. Add User Story 2 → Test compression independently → LLM context ready!
4. Add User Story 3 → Test filtered modes independently → Context optimization!
5. Each story adds value without breaking previous functionality

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001-T008)
2. Once Foundational is done (T008 complete):
   - Developer A: User Story 1 (T009-T025) - Storage and queries
   - Developer B: User Story 2 (T026-T039) - RLE compression
   - Developer C: User Story 3 (T040-T049) - Compression modes
3. Stories complete independently
4. Team reconvenes for Integration (T050-T063) and Polish (T064-T070)

---

## Notes

- [P] tasks = different files or independent operations - can run concurrently
- [Story] label (US1, US2, US3) maps task to specific user story
- Each user story independently testable per spec acceptance scenarios
- Tests NOT included per spec (not explicitly requested) - benchmarks added in integration phase
- Bit array uses Z-major ordering for efficient Z-level filtering (research.md decision)
- RLE uses byte-oriented encoding with varint run counts (research.md decision)
- IsPathable uses simplified heuristic (0=closed, 1-299=open, 300+=closed) - may need refinement
- Compression modes enable context budget experiments (full vs filtered)
- Total: 70 tasks across 7 phases