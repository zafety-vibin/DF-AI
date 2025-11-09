# Tasks: Local LLM with Graph-Based Goal-Oriented Agents

**Input**: Design documents from `/specs/006-local-llm-agents/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, quickstart.md

**Tests**: Tests are OPTIONAL - not included in this task list as not explicitly requested in spec

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

- **Single project**: `internal/`, `tests/` at repository root
- All paths relative to `C:\Users\zmanl\Projects\DF-AI\`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and graph agent infrastructure

- [ ] T001 Create internal/agents/ package directory structure
- [ ] T002 [P] Add agent configuration schema to internal/config/config.go (AgentConfig, DefenseAgentConfig structs)
- [ ] T003 [P] Create tests/agents/ directory for unit tests
- [ ] T004 [P] Create tests/integration/ directory for end-to-end tests

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core graph infrastructure and interfaces that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T005 Define GoalAgent interface in internal/agents/agent.go (Name, Priority, Analyze, Enabled methods)
- [ ] T006 Define FortMetrics struct in internal/agents/agent.go (dwarf count, food, bedrooms, etc.)
- [ ] T007 [P] Define ModificationNode struct in internal/agents/graph.go (ID, Type, Region, Dependencies, Conflicts, Priority, Urgency, Rationale)
- [ ] T008 [P] Define NodeType constants in internal/agents/graph.go (bedroom_cluster, mining_shaft, farm_plot, etc.)
- [ ] T009 [P] Define DependencyType constants in internal/agents/graph.go (requires_access_from, requires_water, requires_stairs)
- [ ] T010 Define ProposalGraph struct in internal/agents/graph.go (Nodes, DependencyEdges, ConflictEdges, AgentMetadata)
- [ ] T011 Implement TopologicalSort() function in internal/agents/graph.go using Kahn's algorithm with cycle detection
- [ ] T012 [P] Implement DetectSpatialConflicts() function in internal/agents/graph.go using AABB intersection
- [ ] T013 [P] Implement ToJSON() function in internal/agents/graph.go for arbiter prompt serialization
- [ ] T014 Create AgentRegistry in internal/agents/agent.go (map[string]GoalAgent with Register/Get/List methods)
- [ ] T015 Define ArbitrationDecision struct in internal/agents/graph.go (ExecutionSequence, InjectedNodes, DeferredNodes, RejectedNodes)
- [ ] T016 Update config/orchestrator.yaml with LLM provider settings (llm_provider_type, local_llm_endpoint, local_llm_model, llm_timeout_ms)
- [ ] T017 Update config/orchestrator.yaml with enable_goal_agents toggle and all 5 agent configs

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Goal Agents Maintain Fort Health (Priority: P1) 🎯 MVP

**Goal**: Implement 5 specialized agents that monitor fort metrics and propose graph-based modification nodes when targets not met

**Independent Test**: Run orchestrator with all 5 agents enabled, observe agent proposals in logs when food < 20/dwarf or bedrooms < dwarf count. Verify proposals contain node type, region, dependencies, rationale. Fort should survive 30+ in-game days with food > 15/dwarf and all dwarves assigned bedrooms.

### Implementation for User Story 1

- [ ] T018 [P] [US1] Implement FoodSecurityAgent in internal/agents/food.go (Analyze checks food/drink stocks, proposes farm_plot or gather_zone nodes)
- [ ] T019 [P] [US1] Implement HousingAgent in internal/agents/housing.go (Analyze checks bedroom count, proposes bedroom_cluster nodes)
- [ ] T020 [P] [US1] Implement MiningAgent in internal/agents/mining.go (Analyze checks mining activity rate, proposes mining_shaft or exploratory_tunnel nodes)
- [ ] T021 [P] [US1] Implement WealthAgent in internal/agents/wealth.go (Analyze checks wealth growth rate, proposes workshop_zone or production_area nodes)
- [ ] T022 [P] [US1] Implement DefenseAgent in internal/agents/defense.go (Analyze checks enemy presence, proposes seal_entrance or defensive_wall nodes with dynamic priority)
- [ ] T023 [US1] Update internal/autonomous/loop.go to compute FortMetrics from entity cache, modification overlay, phase manager
- [ ] T024 [US1] Update internal/autonomous/loop.go to initialize AgentRegistry and register all 5 agents based on config
- [ ] T025 [US1] Update internal/autonomous/loop.go to call all enabled agents in parallel during decision cycle
- [ ] T026 [US1] Update internal/autonomous/loop.go to assemble ProposalGraph from agent ModificationNode outputs
- [ ] T027 [US1] Add agent proposal logging (DEBUG level) in internal/autonomous/loop.go with JSON format
- [ ] T028 [US1] Add agent latency metrics logging in internal/autonomous/loop.go (target: <100ms for 5 agents)

**Checkpoint**: At this point, agents analyze fort state and propose modification nodes. Logs show agent proposals with node types, regions, dependencies, rationale. No LLM arbitration yet.

---

## Phase 4: User Story 2 - LLM Arbiter Coordinates Graph-Based Proposals (Priority: P1) 🎯 MVP

**Goal**: Local LLM arbiter receives ProposalGraph, performs topological sort and synergy recognition, generates coordinated execution sequence

**Independent Test**: Trigger low food + low housing simultaneously, verify arbiter recognizes bedroom_cluster and farm_plot can share corridor, logs show corridor_connector node injected. Execution sequence respects dependencies (corridor first, then farm/bedrooms). Verify arbiter completes in <500ms for 7B model, <1000ms for 14B model.

### Implementation for User Story 2

- [ ] T029 [US2] Implement LocalLLMProvider in internal/llm/local.go implementing Provider interface (GenerateResponse, HealthCheck methods)
- [ ] T030 [US2] Add OpenAI-compatible HTTP client in internal/llm/local.go (POST to endpoint/v1/chat/completions)
- [ ] T031 [US2] Implement system prompt caching in internal/llm/local.go (arbiter instructions, graph schema, synergy patterns)
- [ ] T032 [US2] Implement arbiter prompt generation in internal/autonomous/loop.go (system: cached instructions, user: graph JSON + fort metrics)
- [ ] T033 [US2] Update internal/autonomous/loop.go to call arbiter (LocalLLMProvider.GenerateResponse) with ProposalGraph JSON
- [ ] T034 [US2] Implement arbiter response parsing in internal/autonomous/loop.go (JSON execution sequence → ArbitrationDecision)
- [ ] T035 [US2] Implement synergy recognition logging in internal/autonomous/loop.go (count corridor_connector injections, shared infrastructure)
- [ ] T036 [US2] Implement conflict resolution fallback heuristic in internal/autonomous/loop.go (highest priority non-conflicting proposal if LLM fails)
- [ ] T037 [US2] Add arbiter latency and token usage logging in internal/autonomous/loop.go
- [ ] T038 [US2] Add LLM health check at startup in cmd/df-orchestrator/main.go (verify endpoint reachable, log model info)
- [ ] T039 [US2] Add retry logic in internal/llm/local.go (1 retry with 2s delay, then fallback to heuristic)
- [ ] T040 [US2] Add timeout handling in internal/llm/local.go (5s default, 10s for 14B models)

**Checkpoint**: At this point, arbiter receives graph proposals, performs topological sort, recognizes synergies, generates execution sequence. Logs show graph JSON sent to LLM, arbiter decision JSON received, synergy count, latency, token usage. Still need to convert nodes → commands.

---

## Phase 5: User Story 3 - Local LLM Inference Replaces Cloud API (Priority: P1) 🎯 MVP

**Goal**: System connects to LM Studio, uses local inference, generates zero API costs

**Independent Test**: Start LM Studio with Qwen2.5-7B Q4_K_M, start orchestrator with llm_provider_type: local, verify connection logs, run autonomous cycle, check logs show local endpoint used (http://localhost:1234/v1), no Claude API calls, latency <500ms. Monitor for 8+ hours, verify no out-of-memory crashes.

### Implementation for User Story 3

- [ ] T041 [US3] Update cmd/df-orchestrator/main.go to initialize LocalLLMProvider when llm_provider_type is "local"
- [ ] T042 [US3] Update cmd/df-orchestrator/main.go to initialize OpenAI provider (existing) when llm_provider_type is "claude"
- [ ] T043 [US3] Add provider type validation in internal/config/config.go (reject invalid provider types)
- [ ] T044 [US3] Add local endpoint validation in internal/config/config.go (must be valid HTTP/HTTPS URL)
- [ ] T045 [US3] Implement GraphExecutor in internal/agents/executor.go (Execute converts ArbitrationDecision → CommandMessage array)
- [ ] T046 [US3] Register node type handlers in internal/agents/executor.go (bedroom_cluster → DIG commands, farm_plot → ZONE commands, corridor_connector → DIG commands)
- [ ] T047 [US3] Implement bedroom_cluster handler in internal/agents/executor.go (generate 3×3 bedroom grid DIG commands)
- [ ] T048 [US3] Implement mining_shaft handler in internal/agents/executor.go (generate vertical DownStair DIG commands)
- [ ] T049 [US3] Implement farm_plot handler in internal/agents/executor.go (generate ZONE commands with farming type)
- [ ] T050 [US3] Implement corridor_connector handler in internal/agents/executor.go (calculate Manhattan path, generate 1-wide DIG commands)
- [ ] T051 [US3] Update internal/autonomous/loop.go to instantiate GraphExecutor and execute ArbitrationDecision
- [ ] T052 [US3] Update internal/autonomous/loop.go to send converted commands to DFHack client
- [ ] T053 [US3] Add command execution status tracking in internal/agents/executor.go (NodeStatus: in_progress, completed, failed)
- [ ] T054 [US3] Add GraphMetadata to modification persistence in internal/modifications/persistence.go (extend ModificationInfo struct)
- [ ] T055 [US3] Update save/load functions in internal/modifications/persistence.go to persist GraphMetadata (node type, agent name, dependencies, status, rationale)

**Checkpoint**: At this point, full pipeline works: agents → graph → arbiter (local LLM) → executor → DFHack commands. System runs autonomously with zero API costs. Verify in DF: dig designations appear, bedrooms get dug, corridors connect rooms.

---

## Phase 6: User Story 4 - Configurable Agent Behavior (Priority: P2)

**Goal**: Users adjust agent thresholds, enable/disable agents, tune priorities via config

**Independent Test**: Set food_target_per_dwarf: 30 in config, restart, verify FoodAgent triggers at 25 food/dwarf (above old 20 threshold). Disable wealth agent (enabled: false), verify no workshop proposals in logs. Override priorities (housing: 10, food: 9), verify housing proposals win conflicts.

### Implementation for User Story 4

- [ ] T056 [US4] Add target threshold loading in internal/agents/food.go (read food_per_dwarf, drink_per_dwarf from config)
- [ ] T057 [US4] Add target threshold loading in internal/agents/housing.go (read bedrooms_per_dwarf from config)
- [ ] T058 [US4] Add target threshold loading in internal/agents/mining.go (read tiles_per_cycle, no_strike_cycles from config)
- [ ] T059 [US4] Add target threshold loading in internal/agents/wealth.go (read growth_threshold from config)
- [ ] T060 [US4] Add dynamic priority calculation in internal/agents/defense.go (priority_base when no enemies, priority_threat when enemies present)
- [ ] T061 [US4] Update internal/autonomous/loop.go to check agent Enabled() before including in analysis
- [ ] T062 [US4] Update internal/autonomous/loop.go to respect agent Priority() values in conflict resolution
- [ ] T063 [US4] Add fallback to direct-LLM mode in internal/autonomous/loop.go when enable_goal_agents: false (Feature 005 behavior)
- [ ] T064 [US4] Add config validation in cmd/df-orchestrator/main.go (warn if targets are extreme values, suggest defaults)
- [ ] T065 [US4] Update quickstart.md with agent configuration examples (food target adjustment, disable wealth, priority override)

**Checkpoint**: Users can customize agent behavior via config. Different thresholds change when agents trigger. Disabling agents removes their proposals. Priority changes affect conflict resolution.

---

## Phase 7: User Story 5 - Arbiter Handles Emergencies and Novel Events (Priority: P2)

**Goal**: Arbiter applies LLM reasoning to generate emergency responses for situations outside agent coverage (forgotten beasts, strange moods, floods)

**Independent Test**: Spawn forgotten beast in DFHack (modtools/spawn), verify arbiter generates emergency seal_cavern or defensive_wall nodes (not from any agent). Breach aquifer, verify arbiter detects catastrophic water increase (50+ tiles), proposes emergency flood response. Check logs show ad-hoc node generation with high priority.

### Implementation for User Story 5

- [ ] T066 [US5] Add emergency event detection in internal/autonomous/loop.go (forgotten beast entity type, rapid water tile increase, strange mood status)
- [ ] T067 [US5] Extend arbiter system prompt in internal/llm/local.go with emergency response guidelines (seal caverns, emergency drainage, prioritize mood materials)
- [ ] T068 [US5] Add emergency context to arbiter user message in internal/autonomous/loop.go (forgotten_beast: true, aquifer_breach_detected: true, dwarf_in_mood: "craftsdwarf needs iron bar")
- [ ] T069 [US5] Implement ad-hoc node injection in internal/autonomous/loop.go (arbiter can generate nodes not from any agent)
- [ ] T070 [US5] Add emergency node type handlers in internal/agents/executor.go (seal_entrance → BUILD wall commands, defensive_wall → BUILD commands)
- [ ] T071 [US5] Add emergency priority escalation in internal/autonomous/loop.go (emergency nodes execute first regardless of dependencies)
- [ ] T072 [US5] Add emergency event logging in internal/autonomous/loop.go (WARN level: "Forgotten beast detected", "Aquifer breach", "Strange mood: X needs Y")

**Checkpoint**: System handles rare events beyond agent coverage. Arbiter generates appropriate emergency nodes. Logs show emergency detection and response. In-game: cavern entrances sealed when beasts spawn, walls built at flood points.

---

## Phase 8: User Story 6 - Fast Decision Cycles Enable Responsive Management (Priority: P2)

**Goal**: Full cycles complete in <2s, enabling frequent adjustments and rapid responses

**Independent Test**: Run 100 consecutive decision cycles, time each (logs/metrics), verify 95%+ complete within 2s target. Measure: agent analysis <100ms, graph assembly <50ms, arbiter <500ms (7B) or <1000ms (14B). Check latency_ms fields in logs.

### Implementation for User Story 6

- [ ] T073 [US6] Add cycle timer in internal/autonomous/loop.go (measure from agent analysis start to command execution complete)
- [ ] T074 [US6] Add agent analysis parallel execution in internal/autonomous/loop.go using goroutines and WaitGroup (5 agents concurrently)
- [ ] T075 [US6] Add graph assembly timer in internal/agents/graph.go (measure TopologicalSort + DetectSpatialConflicts)
- [ ] T076 [US6] Add arbiter inference timer in internal/llm/local.go (measure HTTP request → response)
- [ ] T077 [US6] Add command execution timer in internal/agents/executor.go (measure node conversion → DFHack send)
- [ ] T078 [US6] Add cycle performance metrics logging in internal/autonomous/loop.go (total_cycle_ms, agent_analysis_ms, graph_assembly_ms, arbiter_inference_ms, command_execution_ms)
- [ ] T079 [US6] Add performance warning logs when thresholds exceeded (WARN if agent >100ms, graph >50ms, arbiter >1000ms)
- [ ] T080 [US6] Implement metric caching in internal/autonomous/loop.go (compute fort metrics once, pass to all agents to avoid redundant calculations)
- [ ] T081 [US6] Add early exit optimization in agents (return empty proposals immediately if all targets met, avoid expensive computation)

**Checkpoint**: System achieves <2s cycle target consistently. Logs show detailed timing breakdown. Performance warnings trigger if components slow down. Responsive AI adjusts fort management rapidly.

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T082 [P] Add unit tests for TopologicalSort in tests/agents/graph_test.go (various dependency graphs, cycle detection, empty graph)
- [ ] T083 [P] Add unit tests for DetectSpatialConflicts in tests/agents/graph_test.go (overlapping regions, adjacent regions, disjoint regions)
- [ ] T084 [P] Add unit tests for FoodSecurityAgent in tests/agents/food_test.go (mock FortMetrics, verify proposal generation)
- [ ] T085 [P] Add unit tests for HousingAgent in tests/agents/housing_test.go (mock FortMetrics, verify bedroom_cluster proposals)
- [ ] T086 [P] Add integration test in tests/integration/arbiter_test.go (mock 5 agents, generate proposals, mock LLM response, verify command conversion)
- [ ] T087 [P] Add benchmark test in tests/agents/graph_test.go (graph ops with 20 nodes, verify <50ms)
- [ ] T088 Update README.md with Feature 006 quickstart reference
- [ ] T089 Update CLAUDE.md with graph agent usage examples (how to add new node types, modify agent thresholds)
- [ ] T090 Add error handling for invalid arbiter responses in internal/autonomous/loop.go (malformed JSON → log + fallback heuristic)
- [ ] T091 Add VRAM monitoring logs at startup in cmd/df-orchestrator/main.go (warn if using 14B model on 8GB GPU)
- [ ] T092 Run quickstart.md validation (install LM Studio, download model, configure orchestrator, verify 100-day survival)
- [ ] T093 Commit all changes with message: "feat: implement graph-based goal-oriented agents with local LLM arbitration"

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Story 1 (Phase 3)**: Depends on Foundational phase - Implements agent analysis and proposal generation
- **User Story 2 (Phase 4)**: Depends on Foundational phase - Implements arbiter coordination (can parallelize with US1 if different developers)
- **User Story 3 (Phase 5)**: Depends on US1 + US2 - Integrates agents + arbiter + executor into full pipeline
- **User Story 4 (Phase 6)**: Depends on US3 - Extends with configurable behavior
- **User Story 5 (Phase 7)**: Depends on US3 - Extends with emergency handling
- **User Story 6 (Phase 8)**: Depends on US3 - Optimizes performance
- **Polish (Phase 9)**: Depends on desired user stories being complete

### User Story Dependencies

**Critical Path** (MVP = P1 stories):
- Phase 1 (Setup) → Phase 2 (Foundational) → **US1 (Agents)** → **US2 (Arbiter)** → **US3 (Integration)** → MVP Complete

**Parallel Opportunities**:
- US1 and US2 can be implemented in parallel by different developers (US1: agents, US2: arbiter + LLM provider)
- All 5 agent implementations (T018-T022) can be developed in parallel
- All node type handlers (T047-T050) can be developed in parallel
- US4, US5, US6 can be implemented in parallel after US3 complete

### Within Each User Story

- **US1**: Agents can be implemented in parallel (T018-T022 [P]), then integration tasks (T023-T028) sequentially
- **US2**: LLM provider (T029-T031 [P]), then arbiter integration (T032-T040) sequentially
- **US3**: Executor handlers (T047-T050 [P]) in parallel, then integration (T051-T055) sequentially
- **US4**: Agent threshold loading (T056-T060 [P]) in parallel
- **US5**: Detection + handling (T066-T072) sequentially
- **US6**: Timer additions (T073-T081) mostly sequential (same files)

---

## Parallel Example: User Story 1

```bash
# Launch all agent implementations together:
Task T018: "Implement FoodSecurityAgent in internal/agents/food.go"
Task T019: "Implement HousingAgent in internal/agents/housing.go"
Task T020: "Implement MiningAgent in internal/agents/mining.go"
Task T021: "Implement WealthAgent in internal/agents/wealth.go"
Task T022: "Implement DefenseAgent in internal/agents/defense.go"

# Then integration tasks sequentially (same file: autonomous/loop.go):
Task T023: "Update loop.go to compute FortMetrics"
Task T024: "Update loop.go to initialize AgentRegistry"
Task T025: "Update loop.go to call agents in parallel"
...
```

---

## Parallel Example: User Story 3

```bash
# Launch all executor handlers together:
Task T047: "Implement bedroom_cluster handler in internal/agents/executor.go"
Task T048: "Implement mining_shaft handler in internal/agents/executor.go"
Task T049: "Implement farm_plot handler in internal/agents/executor.go"
Task T050: "Implement corridor_connector handler in internal/agents/executor.go"

# Then integration tasks (different files, can parallelize):
Task T051: "Update loop.go to instantiate GraphExecutor" (loop.go)
Task T054: "Add GraphMetadata to persistence.go" (persistence.go)
```

---

## Implementation Strategy

### MVP First (User Stories 1, 2, 3 Only - All P1)

1. Complete Phase 1: Setup (T001-T004)
2. Complete Phase 2: Foundational (T005-T017) - CRITICAL
3. Complete Phase 3: User Story 1 (T018-T028) - Agents analyze and propose
4. Complete Phase 4: User Story 2 (T029-T040) - Arbiter coordinates
5. Complete Phase 5: User Story 3 (T041-T055) - Full integration
6. **STOP and VALIDATE**: Run 100-day fort, verify:
   - Food stocks > 15/dwarf (95%+ of cycles)
   - All dwarves have bedrooms (within 30 days)
   - Zero API costs (local LLM only)
   - Cycle latency < 2s (95%+ of cycles)
   - Token usage < 1000 (average)
7. Deploy/demo if MVP successful

### Incremental Delivery

1. Setup + Foundational → Foundation ready (T001-T017)
2. Add US1 → Agents propose nodes → Test agent proposals in logs (T018-T028)
3. Add US2 → Arbiter coordinates → Test synergy recognition in logs (T029-T040)
4. Add US3 → Full pipeline → Test 100-day survival (T041-T055) **MVP!**
5. Add US4 → Configurable behavior → Test custom thresholds (T056-T065)
6. Add US5 → Emergency handling → Test forgotten beast response (T066-T072)
7. Add US6 → Performance optimized → Test <2s cycles (T073-T081)
8. Polish → Tests + docs (T082-T093)

### Parallel Team Strategy

With 2-3 developers:

1. Team completes Setup + Foundational together (T001-T017)
2. Once Foundational done:
   - Developer A: User Story 1 (agents) (T018-T028)
   - Developer B: User Story 2 (arbiter + LLM) (T029-T040)
3. Developer A + B collaborate on User Story 3 integration (T041-T055)
4. Then split P2 stories:
   - Developer A: US4 + US6 (config + performance)
   - Developer B: US5 + tests (emergencies + polish)

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- Commit after each logical task group
- Stop at checkpoints to validate story independently
- MVP = User Stories 1, 2, 3 (all P1) - provides core autonomous fort management
- P2 stories (4, 5, 6) add polish and configurability

---

## Task Summary

**Total Tasks**: 93
- Setup: 4 tasks
- Foundational: 13 tasks
- User Story 1 (P1 - Goal Agents): 11 tasks
- User Story 2 (P1 - Arbiter): 12 tasks
- User Story 3 (P1 - Integration): 15 tasks
- User Story 4 (P2 - Configurable): 10 tasks
- User Story 5 (P2 - Emergencies): 7 tasks
- User Story 6 (P2 - Performance): 9 tasks
- Polish: 12 tasks

**Parallel Opportunities**: 35 tasks marked [P]
- 5 agent implementations (US1)
- 4 executor handlers (US3)
- 5 agent threshold loaders (US4)
- 6 unit test files (Polish)
- Multiple config/struct definitions (Foundational)

**MVP Scope**: Phases 1-5 (Tasks T001-T055) = 55 tasks
- Estimated: 10-14 hours implementation + 4-6 hours testing
- Delivers: Autonomous fort management, graph-based spatial reasoning, zero API costs

**Independent Test Criteria**:
- US1: Agents propose nodes when targets not met, logs show proposals
- US2: Arbiter recognizes synergies (corridor_connector injections), <1000ms latency
- US3: Full pipeline works, DF shows dig designations, zero API costs
- US4: Config changes affect agent behavior
- US5: Emergency events trigger appropriate responses
- US6: 95%+ cycles complete in <2s

---

**Ready for Implementation**: 2025-11-09
