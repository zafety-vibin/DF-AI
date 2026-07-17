# Tasks: Hazard Overlays

**Input**: Design documents from `/specs/004-hazard-overlays/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/internal-api.md, quickstart.md

**Tests**: Tests are included where specified in spec (independent test criteria for each user story)

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

Project structure: `internal/hazards/` (new package), `tests/hazards/` (new test directory)

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [ ] T001 Create internal/hazards/ package directory
- [ ] T002 Create tests/hazards/ test directory
- [ ] T003 [P] Add hazards package import to go.mod if needed

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core data structures and interfaces that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Create Coordinate type in internal/hazards/overlay.go
- [ ] T005 [P] Create HazardInfo type in internal/hazards/overlay.go
- [ ] T006 [P] Create Bounds type in internal/hazards/overlay.go
- [ ] T007 [P] Create Region type in internal/hazards/overlay.go
- [ ] T008 Create HazardOverlay struct with RWMutex in internal/hazards/overlay.go
- [ ] T009 Implement NewHazardOverlay constructor in internal/hazards/overlay.go
- [ ] T010 [P] Implement Contains(x, y, z) query in internal/hazards/overlay.go
- [ ] T011 [P] Implement Get(x, y, z) query in internal/hazards/overlay.go
- [ ] T012 [P] Implement GetInRegion(region) query in internal/hazards/overlay.go
- [ ] T013 [P] Implement GetCount() query in internal/hazards/overlay.go
- [ ] T014 [P] Implement Add(coord, info) mutation in internal/hazards/overlay.go
- [ ] T015 [P] Implement Remove(coord) mutation in internal/hazards/overlay.go
- [ ] T016 [P] Implement Clear() mutation in internal/hazards/overlay.go
- [ ] T017 Add ErrOutOfBounds error constant in internal/hazards/overlay.go
- [ ] T018 Add coordinate bounds validation to all overlay methods in internal/hazards/overlay.go

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Environmental Hazard Detection (Priority: P1) 🎯 MVP

**Goal**: Detect and track aquifer, water, lava, and cavern tiles from tile data using sparse overlays

**Independent Test**: Load map with aquifers, water, magma sea, and caverns. Build overlays from FULL_STATE. Query known hazard coordinates. Verify correct detection. Measure memory usage <30 KB for environmental hazards.

### Implementation for User Story 1

#### Detector Functions

- [ ] T019 [P] [US1] Create internal/hazards/detectors.go file
- [ ] T020 [P] [US1] Implement IsAquifer(tileType, flags) detector in internal/hazards/detectors.go
- [ ] T021 [P] [US1] Implement IsWater(tileType) detector returning (bool, depth, flowFlags) in internal/hazards/detectors.go
- [ ] T022 [P] [US1] Implement IsLava(tileType) detector returning (bool, depth, flowFlags) in internal/hazards/detectors.go
- [ ] T023 [P] [US1] Implement IsCavern(tileType) detector in internal/hazards/detectors.go

#### Environmental Overlay Implementations

- [ ] T024 [P] [US1] Create internal/hazards/environmental.go file
- [ ] T025 [P] [US1] Create AquiferOverlay type embedding HazardOverlay in internal/hazards/environmental.go
- [ ] T026 [P] [US1] Create WaterOverlay type embedding HazardOverlay in internal/hazards/environmental.go
- [ ] T027 [P] [US1] Create LavaOverlay type embedding HazardOverlay in internal/hazards/environmental.go
- [ ] T028 [P] [US1] Create CavernOverlay type embedding HazardOverlay in internal/hazards/environmental.go

#### HazardManager Core

- [ ] T029 [US1] Create internal/hazards/manager.go file
- [ ] T030 [US1] Create HazardManager struct with aquifer, water, lava, caverns fields in internal/hazards/manager.go
- [ ] T031 [US1] Implement NewHazardManager(width, height, depth) constructor in internal/hazards/manager.go
- [ ] T032 [US1] Implement BuildFromTiles(tiles) for environmental overlays in internal/hazards/manager.go
- [ ] T033 [US1] Implement UpdateFromTiles(tiles) for environmental overlays in internal/hazards/manager.go
- [ ] T034 [P] [US1] Implement GetOverlay(hazardType) accessor in internal/hazards/manager.go
- [ ] T035 [P] [US1] Implement GetAllCounts() accessor in internal/hazards/manager.go
- [ ] T036 [P] [US1] Implement GetMemoryUsage() accessor in internal/hazards/manager.go

#### Integration with Main

- [ ] T037 [US1] Add hazardManager global variable in cmd/df-orchestrator/main.go
- [ ] T038 [US1] Initialize hazardManager in main() in cmd/df-orchestrator/main.go
- [ ] T039 [US1] Call BuildFromTiles in FULL_STATE callback in cmd/df-orchestrator/main.go
- [ ] T040 [US1] Add logging for hazard counts and memory usage in cmd/df-orchestrator/main.go
- [ ] T041 [US1] Call UpdateFromTiles in TILE_UPDATE handler in cmd/df-orchestrator/main.go

#### Tests for User Story 1

- [ ] T042 [P] [US1] Create tests/hazards/overlay_test.go with HazardOverlay unit tests
- [ ] T043 [P] [US1] Create tests/hazards/detectors_test.go with detector function tests
- [ ] T044 [P] [US1] Create tests/hazards/manager_test.go with HazardManager tests
- [ ] T045 [P] [US1] Add TestBuildFromTiles integration test in tests/hazards/manager_test.go
- [ ] T046 [P] [US1] Add TestUpdateFromTiles integration test in tests/hazards/manager_test.go

**Checkpoint**: At this point, environmental hazard overlays should be fully functional. Test by connecting DFHack, loading fort with aquifers/water/lava/caverns, verify overlays build and update correctly.

---

## Phase 4: User Story 2 - Entity Hazard Tracking (Priority: P2)

**Goal**: Track enemy and dwarf positions using sparse overlays from entity data

**Independent Test**: Load fort with 50 dwarves and 10 hostile creatures. Build entity overlays from entity position data. Verify all entities tracked. Update overlays when entities move, verify positions synchronized.

**⚠️ PROTOCOL DEPENDENCY**: Requires ENTITY_UPDATE protocol message (not in current implementation). This phase designs the interface but may not be fully functional until protocol enhancement.

### Implementation for User Story 2

#### Entity Data Structures

- [ ] T047 [P] [US2] Create internal/hazards/entities.go file
- [ ] T048 [P] [US2] Create EntityHazardInfo struct extending HazardInfo in internal/hazards/entities.go
- [ ] T049 [P] [US2] Create DwarfHazardInfo struct extending HazardInfo in internal/hazards/entities.go
- [ ] T050 [P] [US2] Create EnemyOverlay type embedding HazardOverlay in internal/hazards/entities.go
- [ ] T051 [P] [US2] Create DwarfOverlay type embedding HazardOverlay in internal/hazards/entities.go

#### Entity Detection (Protocol Design)

- [ ] T052 [P] [US2] Add EntityInfo type to internal/protocol/messages.go (ID, X, Y, Z, Type, Subtype)
- [ ] T053 [P] [US2] Add ENTITY_UPDATE message type constant in internal/protocol/messages.go
- [ ] T054 [P] [US2] Create EntityUpdateMessage struct in internal/protocol/messages.go
- [ ] T055 [US2] Implement EntityUpdateMessage encoding in internal/protocol/encoding.go
- [ ] T056 [US2] Implement EntityUpdateMessage decoding in internal/protocol/decoding.go

#### HazardManager Entity Support

- [ ] T057 [US2] Add enemies and dwarves fields to HazardManager in internal/hazards/manager.go
- [ ] T058 [US2] Initialize entity overlays in NewHazardManager in internal/hazards/manager.go
- [ ] T059 [US2] Implement BuildFromEntities(entities) in internal/hazards/manager.go
- [ ] T060 [US2] Update GetOverlay to support "enemies" and "dwarves" types in internal/hazards/manager.go
- [ ] T061 [US2] Update GetAllCounts to include entity counts in internal/hazards/manager.go
- [ ] T062 [US2] Update GetMemoryUsage to include entity overlays in internal/hazards/manager.go

#### Integration with Main (Entity Support)

- [ ] T063 [US2] Add ENTITY_UPDATE handler in cmd/df-orchestrator/main.go
- [ ] T064 [US2] Call BuildFromEntities when ENTITY_UPDATE received in cmd/df-orchestrator/main.go
- [ ] T065 [US2] Add logging for entity counts in cmd/df-orchestrator/main.go

#### Tests for User Story 2

- [ ] T066 [P] [US2] Create tests/hazards/entities_test.go with entity overlay tests
- [ ] T067 [P] [US2] Add TestBuildFromEntities integration test in tests/hazards/manager_test.go
- [ ] T068 [P] [US2] Add TestEntityPositionUpdate test in tests/hazards/entities_test.go

**Checkpoint**: At this point, entity overlays interface is complete. Full functionality requires DFHack plugin enhancement to send ENTITY_UPDATE messages.

---

## Phase 5: User Story 3 - Incremental Hazard Updates (Priority: P3)

**Goal**: Update hazard overlays incrementally from tile changes without rebuilding entire overlays

**Independent Test**: Start with overlays built from initial state. Trigger tile changes (dig into aquifer, breach magma). Send TILE_UPDATE. Verify overlays update to add new hazards or remove resolved ones. Measure update latency for 100 changed tiles <10ms.

### Implementation for User Story 3

**Note**: Core UpdateFromTiles already implemented in US1 (T033). This phase adds optimizations and validation.

#### Update Optimizations

- [ ] T069 [US3] Add change tracking to UpdateFromTiles in internal/hazards/manager.go (count added/removed per type)
- [ ] T070 [US3] Add update timing metrics to UpdateFromTiles in internal/hazards/manager.go
- [ ] T071 [US3] Add logging for incremental update statistics in internal/hazards/manager.go

#### Validation & Monitoring

- [ ] T072 [P] [US3] Add GetLastUpdateTime() to HazardOverlay in internal/hazards/overlay.go
- [ ] T073 [P] [US3] Add staleness detection (log if overlay not updated in N seconds) in internal/hazards/manager.go
- [ ] T074 [US3] Add hazard change event logging (DEBUG level) in internal/hazards/manager.go

#### Integration Testing

- [ ] T075 [P] [US3] Add TestIncrementalUpdateAccuracy test in tests/hazards/manager_test.go
- [ ] T076 [P] [US3] Add TestUpdatePerformance benchmark in tests/hazards/benchmark_test.go
- [ ] T077 [P] [US3] Add TestConcurrentUpdates race test in tests/hazards/manager_test.go

**Checkpoint**: All hazard overlays now update efficiently from incremental changes. Verify with live testing: make tile changes in DF, confirm overlays update within 10ms.

---

## Phase 6: Performance & Benchmarks

**Purpose**: Validate performance targets and memory usage

- [ ] T078 [P] Create tests/hazards/benchmark_test.go file
- [ ] T079 [P] Add BenchmarkHazardOverlay_Contains in tests/hazards/benchmark_test.go (target: <10μs)
- [ ] T080 [P] Add BenchmarkHazardOverlay_Get in tests/hazards/benchmark_test.go (target: <10μs)
- [ ] T081 [P] Add BenchmarkHazardOverlay_GetInRegion in tests/hazards/benchmark_test.go (target: <100μs for 1000 tiles)
- [ ] T082 [P] Add BenchmarkHazardOverlay_Add in tests/hazards/benchmark_test.go (target: <1μs)
- [ ] T083 [P] Add BenchmarkHazardManager_BuildFromTiles in tests/hazards/benchmark_test.go (target: <100ms for 6.9M tiles)
- [ ] T084 [P] Add BenchmarkHazardManager_UpdateFromTiles in tests/hazards/benchmark_test.go (target: <10ms for 100 tiles)
- [ ] T085 Add memory usage validation test (target: <50 KB total) in tests/hazards/manager_test.go

---

## Phase 7: HTTP API Integration

**Purpose**: Expose hazard metrics via HTTP monitoring API

- [ ] T086 [P] Add hazard metrics to metricsHandler in internal/http/metrics.go
- [ ] T087 [P] Add GetHazardCounts() helper in internal/http/metrics.go
- [ ] T088 [P] Add hazard overlay memory to metrics response in internal/http/metrics.go
- [ ] T089 Test /metrics endpoint returns hazard data in JSON format

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T090 [P] Add comprehensive logging for all hazard operations (DEBUG, INFO, WARN levels)
- [ ] T091 [P] Add race detector testing (go test -race) to CI
- [ ] T092 Code cleanup and comment documentation in internal/hazards/
- [ ] T093 [P] Update quickstart.md with live testing results
- [ ] T094 [P] Add hazard overlay section to SESSION-SUMMARY.md or equivalent docs
- [ ] T095 Run go fmt on all hazards package files
- [ ] T096 Run go vet on all hazards package files
- [ ] T097 Verify all tests pass with go test ./tests/hazards/...
- [ ] T098 Verify benchmarks meet performance targets
- [ ] T099 Test with live DFHack connection per quickstart.md validation steps

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 completion - BLOCKS all user stories
- **Phase 3 (US1)**: Depends on Phase 2 completion - Environmental hazards (MVP)
- **Phase 4 (US2)**: Depends on Phase 2 completion - Entity tracking (protocol enhancement needed)
- **Phase 5 (US3)**: Depends on Phase 3 completion - Incremental updates use US1 infrastructure
- **Phase 6 (Benchmarks)**: Depends on Phase 3 completion (can run in parallel with US2/US3)
- **Phase 7 (HTTP)**: Depends on Phase 3 completion
- **Phase 8 (Polish)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1 - Environmental)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2 - Entities)**: Can start after Foundational (Phase 2) - Independent from US1 but requires protocol enhancement
- **User Story 3 (P3 - Updates)**: Depends on US1 (builds on UpdateFromTiles infrastructure)

### Within Each User Story

**User Story 1**:
- Detector functions (T019-T023) can all run in parallel [P]
- Environmental overlay types (T025-T028) can all run in parallel [P]
- HazardManager requires detectors and overlays complete (T029 starts after T023, T028)
- Integration requires HazardManager complete (T037 starts after T036)
- Tests can run in parallel [P] after implementation complete

**User Story 2**:
- Entity data structures (T047-T051) can all run in parallel [P]
- Protocol design (T052-T054) can run in parallel [P]
- Protocol encoding/decoding must be sequential (T055, T056)
- HazardManager updates require entity types and protocol complete
- Tests can run in parallel [P] after implementation complete

**User Story 3**:
- All tasks sequential as they enhance existing US1 UpdateFromTiles
- Tests can run in parallel [P] (T075-T077)

### Parallel Opportunities

- **Setup Phase**: All [P] tasks can run together (T003)
- **Foundational Phase**: All [P] tasks can run together (T005-T007, T010-T016)
- **US1 Detector Functions**: T020-T023 can run in parallel
- **US1 Overlay Types**: T025-T028 can run in parallel
- **US1 Manager Accessors**: T034-T036 can run in parallel
- **US1 Tests**: T042-T046 can run in parallel
- **US2 Entity Structures**: T047-T051 can run in parallel
- **US2 Protocol Design**: T052-T054 can run in parallel
- **US2 Tests**: T066-T068 can run in parallel
- **US3 Tests**: T075-T077 can run in parallel
- **Benchmarks Phase**: All tasks T078-T084 can run in parallel
- **HTTP Phase**: All tasks T086-T089 can run in parallel
- **Polish Phase**: T090, T091, T093, T094 can run in parallel

---

## Parallel Example: User Story 1 (Environmental Hazards)

```bash
# Launch all detector functions together:
Task: "Implement IsAquifer detector in internal/hazards/detectors.go"
Task: "Implement IsWater detector in internal/hazards/detectors.go"
Task: "Implement IsLava detector in internal/hazards/detectors.go"
Task: "Implement IsCavern detector in internal/hazards/detectors.go"

# Launch all overlay types together:
Task: "Create AquiferOverlay in internal/hazards/environmental.go"
Task: "Create WaterOverlay in internal/hazards/environmental.go"
Task: "Create LavaOverlay in internal/hazards/environmental.go"
Task: "Create CavernOverlay in internal/hazards/environmental.go"

# Launch all tests together:
Task: "Create overlay_test.go with HazardOverlay unit tests"
Task: "Create detectors_test.go with detector function tests"
Task: "Create manager_test.go with HazardManager tests"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (T001-T003)
2. Complete Phase 2: Foundational (T004-T018) - CRITICAL foundation
3. Complete Phase 3: User Story 1 - Environmental Hazards (T019-T046)
4. Complete Phase 6: Benchmarks (T078-T085) - Validate performance
5. **STOP and VALIDATE**: Test with live DFHack connection
6. Deploy/demo environmental hazard detection

**Estimated LOC**: ~400 LOC for MVP (overlay.go ~150, detectors.go ~50, environmental.go ~50, manager.go ~100, main.go integration ~50)

### Incremental Delivery

1. **Foundation** (Phases 1-2) → Core overlay infrastructure ready
2. **MVP** (Phase 3) → Environmental hazards working, test with DFHack
3. **Entities** (Phase 4) → Entity tracking ready (requires protocol enhancement to be fully functional)
4. **Updates** (Phase 5) → Incremental update optimizations
5. **Performance** (Phase 6) → Validate all targets met
6. **HTTP** (Phase 7) → Monitoring integration
7. **Polish** (Phase 8) → Production ready

Each phase adds value and can be tested independently.

### Parallel Team Strategy

With multiple developers:

1. Team completes Phase 1-2 together (foundation)
2. Once Foundational is done:
   - Developer A: User Story 1 (Environmental hazards) - T019-T046
   - Developer B: User Story 2 (Entities) - T047-T068 (parallel, different files)
   - Developer C: Benchmarks - T078-T085 (parallel after US1 basics done)
3. Phase 5 (US3) starts after US1 complete (enhances US1 infrastructure)
4. Phases 7-8 (HTTP, Polish) after all stories complete

---

## Task Summary

**Total Tasks**: 99

**Tasks by Phase**:
- Phase 1 (Setup): 3 tasks
- Phase 2 (Foundational): 15 tasks
- Phase 3 (US1 - Environmental): 28 tasks
- Phase 4 (US2 - Entities): 22 tasks
- Phase 5 (US3 - Updates): 9 tasks
- Phase 6 (Benchmarks): 8 tasks
- Phase 7 (HTTP): 4 tasks
- Phase 8 (Polish): 10 tasks

**Parallel Opportunities**: 52 tasks marked [P] can run in parallel with other tasks in same phase

**MVP Scope**: Phases 1-3 + Phase 6 = 54 tasks for fully functional environmental hazard detection

**Protocol Enhancement Required**: User Story 2 (Phase 4) requires ENTITY_UPDATE protocol message implementation in DFHack plugin (not in current scope)

---

## Notes

- [P] tasks = different files, no dependencies on incomplete tasks
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Run `go test -race ./tests/hazards/...` regularly to ensure thread safety
- User Story 2 (entities) designs the interface but requires protocol work outside this feature
- All environmental hazards (US1) work with current TILE_UPDATE protocol
