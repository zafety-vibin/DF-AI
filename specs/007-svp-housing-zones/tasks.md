# Tasks: Spatial Validator Planner and Housing Zones

**Feature**: 007-svp-housing-zones
**Input**: Design documents from `/specs/007-svp-housing-zones/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Not explicitly requested in spec - focusing on implementation and integration tests per quickstart.md

**Organization**: Tasks grouped by user story (5 stories, all P1 priority) to enable independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (US1-US5)
- Include exact file paths in descriptions

## Path Conventions

- Go project: `internal/`, `cmd/`, `plugin/` at repository root
- Tests: `tests/` at repository root
- Config: `config/` at repository root
- Blueprints: `blueprints/` at repository root

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and package structure for SVP and zones

- [X] T001 Create internal/spatial/ package directory for SVP component
- [X] T002 Create internal/zones/ package directory for zone extraction and queue
- [X] T003 [P] Create tests/spatial/ directory for SVP tests
- [X] T004 [P] Create tests/zones/ directory for zone extraction tests
- [X] T005 [P] Create saves/ directory for fort-specific persistence (svp_layout.json, zone_queue.json)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [X] T006 Define ZLevelDesignation struct in internal/spatial/layout.go (z_level, purpose enum, capacity enum, hazard flags, soil flag)
- [X] T007 Define ZoneInfo struct in internal/zones/types.go (zone_id, zone_type enum, region, assigned_to, size, created_at)
- [X] T008 Define QueuedZone struct in internal/zones/queue.go (zone_type, region, command_id, retry_count, timeout_cycles, status enum, timestamps)
- [X] T009 Define BlueprintMetadata struct in internal/blueprints/metadata.go (name, display_name, dimensions, tile_count, dwarf_capacity, description, tags, suitability_criteria)
- [X] T010 [P] Add Config fields for SVP in internal/config/config.go (use_svp, svp_persistence_dir)
- [X] T011 [P] Add Config fields for zone extraction in internal/config/config.go (extract_zones, zone_extraction_interval_ms)
- [X] T012 [P] Add Config fields for blueprints in internal/config/config.go (blueprint_directory, include_blueprint_metadata)
- [X] T013 [P] Add Config fields for zone queue in internal/config/config.go (zone_queue_max_size, zone_queue_max_retries, zone_queue_timeout_cycles)
- [X] T014 Extend FortMetrics struct in internal/agents/agent.go with zone counts (BedroomZoneCount, DiningZoneCount, DormitoryZoneCount, OfficeZoneCount, UnassignedBedroomCount, HousingDeficit)
- [X] T015 Extend FortMetrics struct in internal/agents/agent.go with SVP designations (SVPHousingZ, SVPWorkshopZ, SVPFarmZ []int)
- [X] T016 Add BLUEPRINT command type to protocol.CommandType enum in internal/protocol/message.go
- [X] T017 Extend CommandMessage in internal/protocol/message.go with blueprint fields (blueprint_name, origin_x, origin_y, origin_z)
- [X] T018 Define ZoneType enum in internal/protocol/message.go (ZONE_BEDROOM, ZONE_DINING, ZONE_DORMITORY, ZONE_OFFICE, ZONE_BARRACKS, ZONE_WORKSHOP, ZONE_STOCKPILE)
- [X] T019 Extend EntityUpdate message in internal/protocol/message.go with zones array (repeated ZoneData)
- [X] T020 Define ZoneData struct in internal/protocol/message.go (zone_id, zone_type, coordinates x1-z2, assigned_to)
- [X] T021 Extend TopologyMessage in internal/protocol/message.go with is_soil_layer array (200 bools)

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - SVP Establishes Fort Z-Level Organization (Priority: P1) 🎯 MVP

**Goal**: SVP analyzes embark terrain on first connection and designates Z-levels for housing, workshops, and farms, persisting designations across sessions.

**Independent Test**: Connect to new embark, verify SVP logs show terrain analysis (embark Z detected, housing/workshop/farm layers designated), check saves/{fortname}/svp_layout.json created. Restart orchestrator, reconnect, verify SVP loads saved layout without re-analysis. (Quickstart Test 1 & 2)

### Implementation for User Story 1

- [ ] T022 [P] [US1] Implement SpatialValidatorPlanner struct in internal/spatial/planner.go (embark_z, housing_z, workshop_z, farm_z, hazard_z, fort_name, analyzed_at, layout_version, persistence_path fields)
- [ ] T023 [P] [US1] Implement NewSpatialValidatorPlanner constructor in internal/spatial/planner.go
- [ ] T024 [US1] Implement AnalyzeTerrain method in internal/spatial/planner.go (detect embark Z from dwarf positions, calculate housing Z = embark - 5, workshop Z = housing - 1, query topology for soil layers, query hazard manager for hazards)
- [ ] T025 [US1] Implement detectEmbarkZ helper in internal/spatial/planner.go (mode of dwarf Z coordinates from entity list)
- [ ] T026 [US1] Implement detectSoilLayers helper in internal/spatial/planner.go (query topology.is_soil_layer array, return Z-levels with soil)
- [ ] T027 [US1] Implement GetHousingZ method in internal/spatial/planner.go (returns housing_z field)
- [ ] T028 [P] [US1] Implement GetWorkshopZ method in internal/spatial/planner.go (returns workshop_z field)
- [ ] T029 [P] [US1] Implement GetFarmZ method in internal/spatial/planner.go (returns farm_z array)
- [ ] T030 [P] [US1] Implement IsReady method in internal/spatial/planner.go (returns true if analyzed_at not zero)
- [ ] T031 [US1] Implement Save method in internal/spatial/planner.go (marshal to JSON, write to saves/{fortname}/svp_layout.json)
- [ ] T032 [US1] Implement Load method in internal/spatial/planner.go (read saves/{fortname}/svp_layout.json, unmarshal, restore fields)
- [ ] T033 [US1] Implement ValidateProposal method in internal/spatial/planner.go (check node type matches Z-level purpose, verify not in hazard zones)
- [ ] T034 [US1] Add SVP state tracking to AutonomousLoop in internal/autonomous/loop.go (WAITING_FOR_TOPOLOGY, WAITING_FOR_ENTITIES, READY states)
- [ ] T035 [US1] Update RESYNC handler in internal/autonomous/loop.go to transition state to WAITING_FOR_ENTITIES after topology loaded
- [ ] T036 [US1] Update ENTITY_UPDATE handler in internal/autonomous/loop.go to trigger SVP analysis when both topology and entities available
- [ ] T037 [US1] Add SVP initialization in cmd/df-orchestrator/main.go (create SVP instance, inject into autonomous loop)
- [ ] T038 [US1] Update computeFortMetrics in internal/autonomous/loop.go to add SVP designations to metrics (SVPHousingZ, SVPWorkshopZ, SVPFarmZ)
- [ ] T039 [US1] Add logging for SVP analysis in internal/spatial/planner.go (DEBUG: embark Z detected, housing/workshop/farm designations, hazards avoided; INFO: analysis complete, saved to disk)
- [ ] T040 [US1] Add logging for SVP load in internal/spatial/planner.go (INFO: loaded existing layout, DEBUG: designation values)

**Checkpoint**: SVP analyzes terrain, designates Z-levels, persists to disk, loads on reconnection. Test independently per quickstart.md Test 1 & 2.

---

## Phase 4: User Story 2 - HousingAgent Uses Real Bedroom Zone Data (Priority: P1)

**Goal**: Extract actual bedroom zone counts from DF, enabling HousingAgent to detect real housing deficits and propose appropriately.

**Independent Test**: Create 3 bedroom zones in DF manually, verify logs show BedroomZoneCount=3 in metrics. With 7 dwarves, verify HousingAgent proposes 4 bedrooms (deficit = 4). (Quickstart Test 3 & 4)

### Implementation for User Story 2

- [ ] T041 [P] [US2] Implement ZoneExtractor struct in internal/zones/extractor.go (plugin_client, logger, last_extract_time, zone_cache, extraction_enabled fields)
- [ ] T042 [P] [US2] Implement NewZoneExtractor constructor in internal/zones/extractor.go
- [ ] T043 [US2] Implement ExtractZones method in internal/zones/extractor.go (parse EntityUpdate.zones array, create ZoneInfo structs, update cache)
- [ ] T044 [US2] Implement CountByType method in internal/zones/extractor.go (iterate zones, count by zone_type, return map[ZoneType]int)
- [ ] T045 [US2] Implement GetUnassignedCount method in internal/zones/extractor.go (filter zones by type and assigned_to == -1, return count)
- [ ] T046 [P] [US2] Implement IsEnabled method in internal/zones/extractor.go (returns extraction_enabled field)
- [ ] T047 [US2] Update computeFortMetrics in internal/autonomous/loop.go to call zone extractor (extract zones, count by type, add to metrics: BedroomZoneCount, DiningZoneCount, etc.)
- [ ] T048 [US2] Update computeFortMetrics in internal/autonomous/loop.go to calculate HousingDeficit (DwarfCount - BedroomZoneCount)
- [ ] T049 [US2] Update computeFortMetrics in internal/autonomous/loop.go to add fallback logic (if zone extraction fails, use chamber count estimation, log warning)
- [ ] T050 [US2] Update HousingAgent.Analyze in internal/agents/housing.go to use metrics.BedroomZoneCount instead of placeholder 0
- [ ] T051 [US2] Update HousingAgent.Analyze in internal/agents/housing.go to calculate deficit from bedroom zone count (deficit = dwarf_count - bedroom_zone_count)
- [ ] T052 [US2] Update HousingAgent.Analyze in internal/agents/housing.go to adjust urgency based on deficit percentage (urgency = float64(deficit) / float64(dwarf_count))
- [ ] T053 [US2] Update HousingAgent.Analyze in internal/agents/housing.go to query SVP for housing Z-level (if SVP ready, use GetHousingZ(), else use hardcoded embark - 5)
- [ ] T054 [US2] Add ZoneExtractor initialization in cmd/df-orchestrator/main.go (create extractor, inject into autonomous loop)
- [ ] T055 [US2] Add logging for zone extraction in internal/zones/extractor.go (DEBUG: querying DF API, INFO: extracted N zones with type counts, WARN: extraction failed fallback)
- [ ] T056 [US2] Add logging for HousingAgent in internal/agents/housing.go (INFO: analyzing metrics with real zone data, DEBUG: deficit calculation, SVP query result)
- [ ] T057 [US2] Implement plugin zone extraction in plugin/df-ai-plugin.cpp (iterate df.global.world.buildings.other.ZONE, extract zone data, add to EntityUpdate message)
- [ ] T058 [US2] Implement MapZoneType helper in plugin/df-ai-plugin.cpp (convert DF zone type enum to protocol ZoneType enum)
- [ ] T059 [US2] Implement getAssignedDwarfID helper in plugin/df-ai-plugin.cpp (extract owner dwarf ID from zone, return -1 if unassigned)
- [ ] T060 [US2] Implement is_soil_layer extraction in plugin/df-ai-plugin.cpp (query tiletype_material per Z-level, set bool array in TopologyMessage)

**Checkpoint**: Zone extraction works, HousingAgent uses real bedroom counts, detects deficits accurately, proposes appropriate housing. Test independently per quickstart.md Test 3 & 4.

---

## Phase 5: User Story 3 - Arbiter Selects Blueprints for Proven Designs (Priority: P1)

**Goal**: Arbiter receives blueprint metadata in system prompt and selects appropriate blueprints for housing proposals instead of arbitrary rectangles.

**Independent Test**: Trigger HousingAgent proposal (deficit exists), verify arbiter logs show blueprint library available, arbiter selects blueprint (e.g., bedroom_3x3), executor sends BLUEPRINT command. (Quickstart Test 5 & 6)

### Implementation for User Story 3

- [ ] T061 [P] [US3] Implement LoadMetadata function in internal/blueprints/metadata.go (scan blueprint directory, parse CSV files, generate BlueprintMetadata structs)
- [ ] T062 [P] [US3] Implement parseBlueprint helper in internal/blueprints/metadata.go (read CSV, count tiles, infer dimensions and capacity)
- [ ] T063 [US3] Implement ToPromptString method on BlueprintMetadata in internal/blueprints/metadata.go (format as "name: description (WxH tiles, N dwarf capacity)")
- [ ] T064 [US3] Implement FitsInSpace method on BlueprintMetadata in internal/blueprints/metadata.go (check if width/height fit in available region)
- [ ] T065 [US3] Implement GetMetadataPrompt function in internal/blueprints/metadata.go (format all blueprints for arbiter system prompt, ~400 token budget)
- [ ] T066 [US3] Update arbiter system prompt builder in internal/autonomous/loop.go to include blueprint metadata (load metadata at startup, append to system prompt)
- [ ] T067 [US3] Update arbiter prompt in internal/autonomous/loop.go to add blueprint selection guidance ("Use blueprints when space matches, geometric patterns for emergency")
- [ ] T068 [US3] Parse arbiter response in internal/autonomous/loop.go to extract blueprint_used field (JSON: {"blueprint_used": "bedroom_3x3", "instances": 4})
- [ ] T069 [US3] Update GraphExecutor.convertNodeToCommands in internal/agents/executor.go to check if arbiter specified blueprint
- [ ] T070 [US3] Implement sendBlueprintCommand helper in internal/agents/executor.go (if blueprint selected, send BLUEPRINT command instead of DIG+ZONE)
- [ ] T071 [US3] Create bedroom_3x3_zones.csv in blueprints/ directory (format: x,y,z,zone_type,width,height with single bedroom zone at 0,0,0 size 3x3)
- [ ] T072 [P] [US3] Create bedroom_cluster_10_zones.csv in blueprints/ directory (format: 10 bedroom zones + 1 dining zone with relative coordinates)
- [ ] T073 [US3] Add logging for blueprint metadata loading in internal/blueprints/metadata.go (INFO: loaded N blueprints, DEBUG: blueprint details)
- [ ] T074 [US3] Add logging for arbiter blueprint selection in internal/autonomous/loop.go (INFO: arbiter selected blueprint X for N dwarves, DEBUG: rationale)
- [ ] T075 [US3] Add logging for executor blueprint command in internal/agents/executor.go (INFO: sending BLUEPRINT command, DEBUG: blueprint name and placement)

**Checkpoint**: Arbiter receives blueprint metadata, selects blueprints when appropriate, executor sends BLUEPRINT commands. Test independently per quickstart.md Test 5 & 6.

---

## Phase 6: User Story 4 - Executor Creates Assignable Bedroom Zones (Priority: P1)

**Goal**: Executor sends both DIG and ZONE commands (or BLUEPRINT command), creating DF zones that dwarves can be assigned to.

**Independent Test**: Trigger bedroom proposal, verify executor logs show BLUEPRINT command sent (or DIG + ZONE pair), check DF zones menu (z) shows new bedroom zones created, confirm dwarves auto-assign. (Quickstart Test 6 & 7)

### Implementation for User Story 4

- [ ] T076 [P] [US4] Implement BLUEPRINT command handler in plugin/df-ai-plugin.cpp (parse blueprint_name, origin coordinates from CommandMessage)
- [ ] T077 [US4] Implement ParseBlueprintCSV function in plugin/df-ai-plugin.cpp (read blueprints/{name}.csv, parse quickfort format, return dig pattern)
- [ ] T078 [US4] Implement ParseZoneCSV function in plugin/df-ai-plugin.cpp (read blueprints/{name}_zones.csv, parse x,y,z,zone_type,width,height, return zone designations)
- [ ] T079 [US4] Implement ValidateZone function in plugin/df-ai-plugin.cpp (check zone coordinates within blueprint bounds, validate zone type enum)
- [ ] T080 [US4] Implement ApplyDigPattern function in plugin/df-ai-plugin.cpp (queue dig designations via DF API at origin + offsets from blueprint)
- [ ] T081 [US4] Implement QueueZones function in plugin/df-ai-plugin.cpp (store zone commands for execution after dig completion)
- [ ] T082 [US4] Implement ApplyZone function in plugin/df-ai-plugin.cpp (create DF zone via building API, set zone type, set coordinates)
- [ ] T083 [US4] Update GraphExecutor.handleBedroomCluster in internal/agents/executor.go to send ZONE commands for each bedroom region
- [ ] T084 [US4] Update GraphExecutor.convertNodeToCommands in internal/agents/executor.go to generate DIG + ZONE command pairs when not using blueprints
- [ ] T085 [US4] Implement ZONE command sending in internal/protocol/client.go (send CommandMessage with type=ZONE, zone_type, region)
- [ ] T086 [US4] Implement BLUEPRINT command sending in internal/protocol/client.go (send CommandMessage with type=BLUEPRINT, blueprint_name, origin)
- [ ] T087 [US4] Add error handling in plugin BLUEPRINT handler for missing blueprint files (return error message "Blueprint not found: {name}")
- [ ] T088 [US4] Add error handling in plugin BLUEPRINT handler for invalid CSV format (return error message "Invalid CSV format in {file}")
- [ ] T089 [US4] Add logging for BLUEPRINT command in plugin/df-ai-plugin.cpp (INFO: applying blueprint {name}, DEBUG: loaded N dig entries and M zones)
- [ ] T090 [US4] Add logging for ZONE command in plugin/df-ai-plugin.cpp (INFO: zone created at coordinates, DEBUG: zone type and assignment status)

**Checkpoint**: Executor sends BLUEPRINT or DIG+ZONE commands, plugin creates assignable bedroom zones in DF. Test independently per quickstart.md Test 6 & 7.

---

## Phase 7: User Story 5 - Deferred Zone Queue Handles Async Execution (Priority: P2)

**Goal**: Zone queue manages ZONE commands that cannot be placed immediately (space not yet dug), retrying after dig completion.

**Independent Test**: Issue bedroom DIG command, verify ZONE queued when space unavailable, wait for dwarf to dig, observe ZONE automatically placed on next cycle. (Quickstart Test 7 & 8)

### Implementation for User Story 5

- [ ] T091 [P] [US5] Implement ZoneQueue struct in internal/zones/queue.go (queued_zones array, modification_overlay, logger, persistence_path fields)
- [ ] T092 [P] [US5] Implement NewZoneQueue constructor in internal/zones/queue.go
- [ ] T093 [US5] Implement Enqueue method in internal/zones/queue.go (add QueuedZone to array, enforce max queue size 50, log enqueue)
- [ ] T094 [US5] Implement ProcessQueue method in internal/zones/queue.go (iterate queued zones, check dig completion, retry ZONE commands, update status)
- [ ] T095 [US5] Implement checkDigCompletion helper in internal/zones/queue.go (query modification overlay for region, verify all tiles dug)
- [ ] T096 [US5] Implement retryZoneCommand helper in internal/zones/queue.go (send ZONE command to DFHack client, handle success/failure)
- [ ] T097 [US5] Implement ShouldRetry method on QueuedZone in internal/zones/queue.go (check retry_count < max_retries and timeout_cycles > 0)
- [ ] T098 [US5] Implement IncrementRetry method on QueuedZone in internal/zones/queue.go (increment retry_count, update last_retry_at)
- [ ] T099 [US5] Implement DecrementTimeout method on QueuedZone in internal/zones/queue.go (decrement timeout_cycles)
- [ ] T100 [US5] Implement IsTimedOut method on QueuedZone in internal/zones/queue.go (return true if timeout_cycles <= 0)
- [ ] T101 [US5] Implement GetPendingCount method in internal/zones/queue.go (return length of queued_zones array)
- [ ] T102 [US5] Implement Save method in internal/zones/queue.go (marshal queue to JSON, write to saves/{fortname}/zone_queue.json)
- [ ] T103 [US5] Implement Load method in internal/zones/queue.go (read saves/{fortname}/zone_queue.json, unmarshal, restore queue)
- [ ] T104 [US5] Update GraphExecutor.Execute in internal/agents/executor.go to enqueue ZONE on failure (if DFHack returns "space not available", create QueuedZone, enqueue)
- [ ] T105 [US5] Update AutonomousLoop.runCycle in internal/autonomous/loop.go to call ProcessQueue each cycle (process queued zones, log results)
- [ ] T106 [US5] Add ZoneQueue initialization in cmd/df-orchestrator/main.go (create queue, load from disk if exists, inject into autonomous loop and executor)
- [ ] T107 [US5] Add logging for zone queue in internal/zones/queue.go (DEBUG: processing queue with N pending, INFO: zone completed/failed, WARN: zone timed out)
- [ ] T108 [US5] Add metrics for zone queue in internal/autonomous/loop.go (zone_queue_length, zone_success_rate)

**Checkpoint**: Zone queue enqueues failed ZONE commands, retries after dig completion, handles timeout. Test independently per quickstart.md Test 7 & 8.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final validation

- [ ] T109 [P] Update config/orchestrator.yaml with SVP, zone extraction, blueprint, and queue settings (use_svp: true, extract_zones: true, etc.)
- [ ] T110 [P] Add unit tests for SVP terrain analysis in tests/spatial/planner_test.go (test embark Z detection, housing/workshop/farm designation, hazard avoidance)
- [ ] T111 [P] Add unit tests for SVP persistence in tests/spatial/planner_test.go (test Save/Load, fort name handling, version migration)
- [ ] T112 [P] Add unit tests for zone extraction in tests/zones/extractor_test.go (test CountByType, GetUnassignedCount, fallback logic)
- [ ] T113 [P] Add unit tests for zone queue in tests/zones/queue_test.go (test Enqueue, ProcessQueue, retry logic, timeout handling)
- [ ] T114 [P] Add mock SVP for agent tests in tests/agents/housing_test.go (mock GetHousingZ, test HousingAgent with real zone data)
- [ ] T115 [P] Add mock ZoneExtractor for metrics tests in tests/autonomous/loop_test.go (mock ExtractZones, test metrics computation)
- [ ] T116 Run quickstart.md validation (execute all 8 test scenarios, verify all checkboxes pass)
- [ ] T117 Performance profiling (measure SVP analysis latency, zone extraction latency, queue processing latency, verify targets met)
- [ ] T118 Documentation updates in specs/007-svp-housing-zones/IMPLEMENTATION-STATUS.md (record task completion, phase progress, blockers encountered)
- [ ] T119 [P] Code cleanup (remove debug comments, ensure consistent error handling, verify all TODOs addressed)
- [ ] T120 [P] Add comprehensive logging review (verify all INFO/DEBUG/WARN logs present per constitution principle II)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-7)**: All depend on Foundational phase completion
  - US1 (SVP) can start immediately after Foundational
  - US2 (Zone Extraction) can start immediately after Foundational (independent of US1, but US1 provides SVP queries)
  - US3 (Blueprint Integration) depends on US2 completion (needs zone data in arbiter context)
  - US4 (Zone Commands) depends on US3 completion (needs BLUEPRINT command support)
  - US5 (Zone Queue) depends on US4 completion (needs ZONE command execution to queue)
- **Polish (Phase 8)**: Depends on desired user stories being complete (minimum US1-US2 for MVP)

### User Story Dependencies

- **User Story 1 (SVP)**: Can start after Foundational - No dependencies on other stories
- **User Story 2 (Zone Extraction)**: Can start after Foundational - Works better with US1 (SVP provides housing Z for HousingAgent) but independently testable
- **User Story 3 (Blueprint Integration)**: Depends on US2 (needs real zone data in metrics for arbiter context)
- **User Story 4 (Zone Commands)**: Depends on US3 (needs BLUEPRINT command infrastructure)
- **User Story 5 (Zone Queue)**: Depends on US4 (needs ZONE command execution to have something to queue)

### Within Each User Story

- Foundational types and structs before implementation
- Core logic before integration
- Plugin changes alongside orchestrator changes (T057-T060 with US2, T076-T082 with US4)
- Logging after implementation
- Story complete before moving to next dependency

### Parallel Opportunities

- **Setup Phase**: All T001-T005 can run in parallel
- **Foundational Phase**:
  - T006-T009 (struct definitions) can run in parallel
  - T010-T013 (config fields) can run in parallel
  - T014-T015 (metrics extension) sequential (same file)
  - T016-T021 (protocol extensions) can run in parallel
- **US1 (SVP)**: T022-T023 parallel, T027-T029 parallel, T031-T033 parallel, T039-T040 parallel
- **US2 (Zone Extraction)**: T041-T042 parallel, T044-T046 parallel, T055-T056 parallel
- **US3 (Blueprint Integration)**: T061-T062 parallel, T063-T065 sequential (same file), T071-T072 parallel, T073-T075 parallel
- **US4 (Zone Commands)**: T076-T082 sequential (plugin logic), T083-T086 sequential (executor logic), T087-T090 parallel (error handling/logging)
- **US5 (Zone Queue)**: T091-T092 parallel, T097-T100 parallel (QueuedZone methods), T110-T115 parallel (tests)
- **Polish Phase**: All T109-T115 and T119-T120 can run in parallel

---

## Parallel Example: User Story 1 (SVP)

```bash
# Launch all parallel struct and method tasks together:
Task: "Implement SpatialValidatorPlanner struct in internal/spatial/planner.go"
Task: "Implement NewSpatialValidatorPlanner constructor in internal/spatial/planner.go"

# After sequential AnalyzeTerrain, launch getters in parallel:
Task: "Implement GetHousingZ method in internal/spatial/planner.go"
Task: "Implement GetWorkshopZ method in internal/spatial/planner.go"
Task: "Implement GetFarmZ method in internal/spatial/planner.go"
Task: "Implement IsReady method in internal/spatial/planner.go"

# Launch logging tasks in parallel:
Task: "Add logging for SVP analysis in internal/spatial/planner.go"
Task: "Add logging for SVP load in internal/spatial/planner.go"
```

---

## Parallel Example: User Story 2 (Zone Extraction)

```bash
# Launch struct and constructor in parallel:
Task: "Implement ZoneExtractor struct in internal/zones/extractor.go"
Task: "Implement NewZoneExtractor constructor in internal/zones/extractor.go"

# Launch utility methods in parallel:
Task: "Implement CountByType method in internal/zones/extractor.go"
Task: "Implement GetUnassignedCount method in internal/zones/extractor.go"
Task: "Implement IsEnabled method in internal/zones/extractor.go"

# Launch logging in parallel:
Task: "Add logging for zone extraction in internal/zones/extractor.go"
Task: "Add logging for HousingAgent in internal/agents/housing.go"
```

---

## Implementation Strategy

### MVP First (User Stories 1 + 2)

1. Complete Phase 1: Setup (T001-T005)
2. Complete Phase 2: Foundational (T006-T021) - CRITICAL, blocks all stories
3. Complete Phase 3: User Story 1 (T022-T040) - SVP analyzes terrain, designates Z-levels
4. Complete Phase 4: User Story 2 (T041-T060) - Zone extraction, HousingAgent uses real data
5. **STOP and VALIDATE**: Test US1 + US2 independently (quickstart.md Test 1-4)
6. Deploy/demo if ready (SVP + zone extraction functional MVP)

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready (T001-T021)
2. Add User Story 1 → Test independently → SVP working (T022-T040, quickstart Test 1-2)
3. Add User Story 2 → Test independently → Zone extraction working (T041-T060, quickstart Test 3-4)
4. Add User Story 3 → Test independently → Blueprint integration working (T061-T075, quickstart Test 5-6)
5. Add User Story 4 → Test independently → Zone commands working (T076-T090, quickstart Test 6-7)
6. Add User Story 5 → Test independently → Zone queue working (T091-T108, quickstart Test 7-8)
7. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001-T021)
2. Once Foundational is done:
   - Developer A: User Story 1 (SVP) - T022-T040
   - Developer B: User Story 2 (Zone Extraction) - T041-T060
   - Wait for A & B to complete
   - Developer A: User Story 3 (Blueprint Integration) - T061-T075
   - Wait for A to complete
   - Developer A or B: User Story 4 (Zone Commands) - T076-T090
   - Wait for US4 to complete
   - Developer A or B: User Story 5 (Zone Queue) - T091-T108
3. Stories complete in dependency order, integrate incrementally

---

## Notes

- [P] tasks = different files, no dependencies within phase
- [Story] label maps task to specific user story for traceability
- User stories have dependencies: US1 → US2 → US3 → US4 → US5 (sequential implementation recommended)
- Each user story should be independently testable via quickstart.md scenarios
- Plugin tasks (T057-T060, T076-T082) run alongside orchestrator tasks (coordinate with plugin developer)
- Commit after each task or logical group (e.g., after each user story phase)
- Stop at any checkpoint to validate story independently
- Performance targets from research.md: SVP <200ms, zone extraction <50ms, queue <10ms
- All logging follows constitution principle II (comprehensive, structured, DEBUG/INFO/WARN levels)
