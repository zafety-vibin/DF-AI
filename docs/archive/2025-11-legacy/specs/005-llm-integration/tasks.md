# Tasks: LLM Integration with Modification Tracking and Command Execution

**Input**: Design documents from `/specs/005-llm-integration/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/

**Organization**: Tasks grouped by user story for independent implementation and testing.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: User story label (US1-US6)
- Include exact file paths

---

## Phase 1: Setup

**Purpose**: Project initialization

- [ ] T001 Create internal/modifications/ package directory
- [ ] T002 Create internal/context/ package directory
- [ ] T003 Create internal/llm/ package directory
- [ ] T004 Create internal/commands/ package directory
- [ ] T005 Create internal/autonomous/ package directory
- [ ] T006 [P] Create tests/modifications/ test directory
- [ ] T007 [P] Create tests/context/ test directory
- [ ] T008 [P] Create tests/llm/ test directory
- [ ] T009 [P] Create tests/integration/ test directory

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core types and protocol extensions that ALL user stories depend on

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T010 Add MessageTypeCommand (0x09) and MessageTypeCommandAck (0x0A) constants in internal/protocol/message.go
- [ ] T011 [P] Create CommandType constants (DIG=0x01, BUILD=0x02, CANCEL=0x03) in internal/protocol/message.go
- [ ] T012 [P] Create AckStatus constants (Success=0x00, Partial=0x01, Failure=0x02) in internal/protocol/message.go
- [X] T013 Create Coordinate type alias in internal/modifications/types.go (reuse from hazards if possible)
- [X] T014 [P] Create Region type in internal/modifications/types.go
- [X] T015 [P] Create ModificationType enum in internal/modifications/types.go
- [X] T016 Create ModificationInfo struct in internal/modifications/types.go
- [ ] T017 Create ViewportLevel enum (Level0-Level3) in internal/context/types.go

**Checkpoint**: Foundation ready - user story implementation can begin

---

## Phase 3: User Story 1 - Modification Tracking Overlay (P1) 🎯 MVP

**Goal**: Track only player/AI tile changes as sparse overlay, extract chambers via flood-fill

**Independent Test**: Mine 3 chambers, query modification bounds and chamber list, verify only changed tiles tracked (not 6.9M natural), memory under 100 KB

### Implementation for US1

#### Core Overlay

- [X] T018 [P] [US1] Create ModificationOverlay struct with sparse map in internal/modifications/overlay.go
- [X] T019 [P] [US1] Implement NewModificationOverlay(bounds) constructor in internal/modifications/overlay.go
- [X] T020 [P] [US1] Implement Add(coord, info) method in internal/modifications/overlay.go
- [X] T021 [P] [US1] Implement Get(coord) method in internal/modifications/overlay.go
- [X] T022 [P] [US1] Implement GetCount() method in internal/modifications/overlay.go
- [X] T023 [P] [US1] Implement GetBounds() returning min/max XYZ in internal/modifications/overlay.go
- [X] T024 [P] [US1] Implement GetModificationsInRegion(region, since) filter in internal/modifications/overlay.go

#### Modification Detection

- [X] T025 [US1] Create internal/modifications/detector.go file
- [X] T026 [US1] Implement InferModificationType(oldType, newType) heuristic in internal/modifications/detector.go
- [X] T027 [US1] Implement DetectModifications(tiles) comparing against first-seen baseline in internal/modifications/detector.go
- [X] T028 [US1] Add LinkCommandToModifications(commandID, region, timestamp) in internal/modifications/overlay.go

#### Chamber Extraction

- [X] T029 [US1] Create internal/modifications/chambers.go file
- [X] T030 [US1] Create Chamber struct with ID, Bounds, TileCount, Connections in internal/modifications/chambers.go
- [X] T031 [US1] Implement floodFill(start, modifications, visited) 6-connected algorithm in internal/modifications/chambers.go
- [X] T032 [US1] Implement ExtractChambers() running flood-fill on all modifications in internal/modifications/chambers.go
- [X] T033 [US1] Implement GenerateChamberDescription(chamber) natural language output in internal/modifications/chambers.go

#### Integration

- [X] T034 [US1] Add modificationOverlay global variable in cmd/df-orchestrator/main.go
- [X] T035 [US1] Initialize ModificationOverlay in FULL_STATE callback in cmd/df-orchestrator/main.go
- [X] T036 [US1] Call DetectModifications in TILE_UPDATE handler in cmd/df-orchestrator/main.go
- [X] T037 [US1] Add logging for modification counts and bounds in cmd/df-orchestrator/main.go

**Checkpoint**: Modification tracking working - can observe player mining and see chambers extracted

---

## Phase 4: User Story 2 - Context Assembly with Viewports (P1)

**Goal**: Format fort state into 4 detail levels (500B/10KB/50KB/200KB) optimized for LLM tokens

**Independent Test**: With 3 chambers and aquifer, generate all 4 levels, verify size budgets met, query API returns filtered data

### Implementation for US2

#### Context Assembly Core

- [X] T038 [P] [US2] Create internal/context/assembly.go file
- [X] T039 [P] [US2] Create ViewportContext struct in internal/context/types.go
- [X] T040 [P] [US2] Create Assembler struct with budget/margin config in internal/context/assembly.go
- [X] T041 [US2] Implement NewAssembler(budgetKB, zMargin, hazardMargin) in internal/context/assembly.go
- [X] T042 [US2] Implement AssembleContext(level, mods, hazards, entities) dispatcher in internal/context/assembly.go

#### Level Formatters

- [X] T043 [P] [US2] Create internal/context/levels.go file
- [X] T044 [P] [US2] Implement GenerateTextOverview(mods, dwarfCount) Level 0 formatter in internal/context/levels.go
- [X] T045 [P] [US2] Implement AssembleActiveArea(mods, hazards, entities) Level 1 formatter in internal/context/levels.go
- [X] T046 [P] [US2] Implement AssembleDeepPlanning(caverns, water, lava) Level 2 formatter in internal/context/levels.go
- [X] T047 [P] [US2] Implement AssembleFullContext(topology, allOverlays) Level 3 formatter in internal/context/levels.go

#### Feature Extraction

- [X] T048 [P] [US2] Create internal/context/features.go file
- [X] T049 [P] [US2] Implement ExtractChamberFeatures(chambers) to JSON in internal/context/features.go
- [X] T050 [P] [US2] Implement FormatHazardsAsJSON(hazards, region) in internal/context/features.go
- [X] T051 [P] [US2] Implement FormatDwarfsAsJSON(entities) in internal/context/features.go
- [X] T052 [US2] Implement FormatAsJSON(context) complete serializer in internal/context/assembly.go

#### Query API

- [X] T053 [P] [US2] Create internal/context/queries.go file
- [X] T054 [P] [US2] Create QueryRequest and QueryResponse structs in internal/context/queries.go
- [X] T055 [P] [US2] Implement ExecuteTopologySlice(z, region) query in internal/context/queries.go
- [X] T056 [P] [US2] Implement ExecuteHazardList(hazardType, zRange) query in internal/context/queries.go
- [X] T057 [P] [US2] Implement ExecuteModificationsInRegion(region) query in internal/context/queries.go
- [X] T058 [US2] Implement ExecuteQuery(queryType, params) dispatcher in internal/context/queries.go

#### Integration

- [X] T059 [US2] Add contextAssembler global in cmd/df-orchestrator/main.go
- [X] T060 [US2] Initialize Assembler with config values in cmd/df-orchestrator/main.go
- [X] T061 [US2] Add test: manually generate Level 0-3 contexts and log sizes in cmd/df-orchestrator/main.go

**Checkpoint**: Context assembly working - can generate all detail levels within budget, query API functional

---

## Phase 5: User Story 3 - First Interaction (P1)

**Goal**: Send prompt to LLM and receive first AI response

**Independent Test**: Send Level 0+1 context to Claude/local LLM, receive response with reasoning and command, verify parsing works

### Implementation for US3

#### LLM Provider Interface

- [ ] T062 [P] [US3] Create internal/llm/provider.go with Provider interface
- [ ] T063 [P] [US3] Create Prompt struct in internal/llm/types.go
- [ ] T064 [P] [US3] Create Response struct in internal/llm/types.go
- [ ] T065 [P] [US3] Create Message struct for conversation history in internal/llm/types.go

#### Claude Provider

- [ ] T066 [P] [US3] Create internal/llm/claude.go file
- [ ] T067 [P] [US3] Create ClaudeProvider struct with API config in internal/llm/claude.go
- [ ] T068 [US3] Implement NewClaudeProvider(apiKey, model) in internal/llm/claude.go
- [ ] T069 [US3] Implement ClaudeProvider.SendPrompt() with HTTP POST to /v1/messages in internal/llm/claude.go
- [ ] T070 [US3] Add Claude API response parsing (extract text from content array) in internal/llm/claude.go
- [ ] T071 [US3] Add Claude error handling (401, 429, 500) with retry logic in internal/llm/claude.go

#### OpenAI-Compatible Provider

- [ ] T072 [P] [US3] Create internal/llm/openai.go file
- [ ] T073 [P] [US3] Create OpenAIProvider struct in internal/llm/openai.go
- [ ] T074 [US3] Implement NewOpenAIProvider(endpoint, apiKey, model) in internal/llm/openai.go
- [ ] T075 [US3] Implement OpenAIProvider.SendPrompt() with HTTP POST to /chat/completions in internal/llm/openai.go
- [ ] T076 [US3] Add OpenAI response parsing (extract from choices array) in internal/llm/openai.go

#### Response Parser

- [ ] T077 [P] [US3] Create internal/llm/parser.go file
- [ ] T078 [P] [US3] Create CommandSpec struct in internal/llm/parser.go
- [ ] T079 [US3] Implement ParseResponse(text) trying JSON first in internal/llm/parser.go
- [ ] T080 [US3] Implement parseJSON(text) structured extraction in internal/llm/parser.go
- [ ] T081 [US3] Implement parseNaturalLanguage(text) regex fallback in internal/llm/parser.go

#### LLM Logger

- [ ] T082 [P] [US3] Create internal/llm/logger.go file
- [ ] T083 [US3] Implement LogInteraction(prompt, response, outcome) JSONL writer in internal/llm/logger.go
- [ ] T084 [US3] Create logs/llm-interactions.jsonl file on first write in internal/llm/logger.go

#### Provider Factory

- [ ] T085 [US3] Create internal/llm/factory.go file
- [ ] T086 [US3] Implement CreateProvider(config) selecting Claude vs OpenAI in internal/llm/factory.go

#### Integration

- [ ] T087 [US3] Add LLM config fields to internal/config/config.go (api_key, model, endpoint, temperature, etc.)
- [ ] T088 [US3] Add llmProvider global in cmd/df-orchestrator/main.go
- [ ] T089 [US3] Initialize LLM provider from config in cmd/df-orchestrator/main.go
- [ ] T090 [US3] Create test: send simple prompt, verify response, log interaction in cmd/df-orchestrator/main.go

**Checkpoint**: LLM communication working - can send prompts to Claude or local model, receive and parse responses

---

## Phase 6: User Story 4 - Command Protocol Bidirectional (P1)

**Goal**: Server sends commands to DFHack, receives acknowledgments

**Independent Test**: Send DESIGNATE_DIG command, DFHack applies designation, sends ACK, verify in DF UI

### Implementation for US4 - Protocol (Go)

#### Protocol Messages

- [ ] T091 [P] [US4] Create CommandMessage struct in internal/protocol/message.go
- [ ] T092 [P] [US4] Create CommandAckMessage struct in internal/protocol/message.go
- [ ] T093 [P] [US4] Implement CommandMessage.Serialize() in internal/protocol/codec.go
- [ ] T094 [P] [US4] Implement DeserializeCommand(data) in internal/protocol/codec.go
- [ ] T095 [P] [US4] Implement CommandAckMessage.Serialize() in internal/protocol/codec.go
- [ ] T096 [P] [US4] Implement DeserializeCommandAck(data) in internal/protocol/codec.go
- [ ] T097 [US4] Add COMMAND and COMMAND_ACK cases to SerializeMessage switch in internal/protocol/codec.go
- [ ] T098 [US4] Add COMMAND and COMMAND_ACK cases to DeserializeMessage switch in internal/protocol/codec.go

#### Command Execution (Go)

- [ ] T099 [P] [US4] Create internal/commands/executor.go file
- [ ] T100 [P] [US4] Create Command struct in internal/commands/types.go
- [ ] T101 [P] [US4] Create CommandTracker struct with pending map in internal/commands/tracker.go
- [ ] T102 [US4] Implement GenerateCommandID() monotonic uint32 in internal/commands/tracker.go
- [ ] T103 [US4] Implement SendCommand(cmd) serializing and sending via client in internal/commands/executor.go
- [ ] T104 [US4] Implement WaitForAck(commandID, timeout) blocking wait in internal/commands/executor.go
- [ ] T105 [US4] Implement HandleAck(ack) updating tracker in internal/commands/tracker.go

#### DFHack Client Extension (Go)

- [ ] T106 [US4] Add commandAckCh channel to Client struct in internal/dfhack/client.go
- [ ] T107 [US4] Add SubscribeCommandAcks() method in internal/dfhack/client.go
- [ ] T108 [US4] Add COMMAND_ACK case to message loop in internal/dfhack/client.go
- [ ] T109 [US4] Implement SendCommand(cmd) method in internal/dfhack/client.go

### Implementation for US4 - Plugin (C++)

- [ ] T110 [US4] Create dfhack-plugin/designations.cpp file for command execution
- [ ] T111 [US4] Implement handleCommand(payload) parsing COMMAND message in dfhack-plugin/df_ai_protocol.cpp
- [ ] T112 [US4] Implement applyDigDesignation(x1,y1,z,x2,y2,z2) using df::map_block->designation in dfhack-plugin/designations.cpp
- [ ] T113 [P] [US4] Implement applyBuildDesignation(x,y,z,buildType) in dfhack-plugin/designations.cpp
- [ ] T114 [P] [US4] Implement applyCancelDesignation(x1,y1,z,x2,y2,z2) in dfhack-plugin/designations.cpp
- [ ] T115 [US4] Implement sendCommandAck(cmdID, status, error) serializing COMMAND_ACK in dfhack-plugin/df_ai_protocol.cpp
- [ ] T116 [US4] Add COMMAND case (0x09) to plugin message loop in dfhack-plugin/df_ai_protocol.cpp
- [ ] T117 [US4] Add designations.cpp to CMakeLists.txt (or inline if build issues)

#### Integration

- [ ] T118 [US4] Add commandExecutor global in cmd/df-orchestrator/main.go
- [ ] T119 [US4] Initialize CommandExecutor in main() in cmd/df-orchestrator/main.go
- [ ] T120 [US4] Subscribe to command ACKs and handle in main() in cmd/df-orchestrator/main.go
- [ ] T121 [US4] Create test: manually send DIG command, wait for ACK, verify designation in DF in cmd/df-orchestrator/main.go

**Checkpoint**: Bidirectional commands working - server can issue commands, DFHack executes and acknowledges

---

## Phase 7: User Story 5 - Command Execution Loop (P2)

**Goal**: Full autonomous cycle with feedback and learning

**Independent Test**: Run loop for 10 minutes (6 turns), verify AI receives feedback about outcomes, logs show decision chains

### Implementation for US5

#### Task Completion Tracking

- [ ] T122 [P] [US5] Add TaskStatus enum to internal/commands/types.go (Acknowledged, InProgress, Completed, Stalled, Failed)
- [ ] T123 [P] [US5] Add ExpectedTiles, CompletedTiles, LastProgress, Status fields to PendingCommand in internal/commands/tracker.go
- [ ] T124 [US5] Implement CheckProgress(mods) method in internal/commands/tracker.go to update task status
- [ ] T125 [US5] Implement CalculateExpectedTiles(region) in internal/commands/tracker.go
- [ ] T126 [US5] Add periodic CheckProgress calls in autonomous loop

#### Feedback Generation

- [ ] T127 [P] [US5] Create internal/commands/feedback.go file
- [ ] T128 [US5] Implement GenerateFeedback(cmd, mods, hazards, ack) analyzing outcomes with task status in internal/commands/feedback.go
- [ ] T129 [US5] Implement generateSuccessFeedback(cmd, actualMods) template with completion % in internal/commands/feedback.go
- [ ] T130 [P] [US5] Implement generateInProgressFeedback(cmd, completedTiles, expectedTiles) in internal/commands/feedback.go
- [ ] T131 [P] [US5] Implement generateHazardFeedback(cmd, mods, newHazards) template in internal/commands/feedback.go
- [ ] T132 [P] [US5] Implement generateStalledFeedback(cmd) template in internal/commands/feedback.go
- [ ] T133 [P] [US5] Implement generateFailureFeedback(cmd, ack) template in internal/commands/feedback.go

#### Conversation History

- [ ] T134 [P] [US5] Create internal/autonomous/history.go file
- [ ] T135 [P] [US5] Create Turn struct with context/response/command/outcome in internal/autonomous/history.go
- [ ] T136 [US5] Create ConversationHistory with sliding window (5 turns) in internal/autonomous/history.go
- [ ] T137 [US5] Implement AddTurn(turn) with window management in internal/autonomous/history.go
- [ ] T138 [US5] Implement GetContext() formatting history for LLM prompt in internal/autonomous/history.go

#### Autonomous Loop

- [ ] T139 [US5] Create internal/autonomous/loop.go file
- [ ] T140 [US5] Create AutonomousLoop struct with dependencies in internal/autonomous/loop.go
- [ ] T141 [US5] Implement NewLoop(mods, topology, hazards, assembler, llm, executor, updateFreq) in internal/autonomous/loop.go
- [ ] T142 [US5] Implement Start(ctx) main cycle goroutine in internal/autonomous/loop.go
- [ ] T143 [US5] Implement runCycle() executing: assemble → LLM → parse → command → wait → CheckProgress → feedback in internal/autonomous/loop.go
- [ ] T137 [P] [US5] Implement TriggerImmediate() signaling urgent update in internal/autonomous/loop.go
- [ ] T138 [P] [US5] Implement GetHistory() accessor for debugging in internal/autonomous/loop.go

#### Timer Management

- [ ] T139 [P] [US5] Create internal/autonomous/timer.go file
- [ ] T140 [US5] Implement createTimer(frequency) with immediate trigger channel in internal/autonomous/timer.go

#### Integration

- [ ] T141 [US5] Add autonomousLoop global in cmd/df-orchestrator/main.go
- [ ] T142 [US5] Create system prompt constant in cmd/df-orchestrator/main.go
- [ ] T143 [US5] Initialize AutonomousLoop after all overlays ready in cmd/df-orchestrator/main.go
- [ ] T144 [US5] Start autonomous loop in background goroutine in cmd/df-orchestrator/main.go
- [ ] T145 [US5] Add logging for each cycle (turn number, tokens, outcome) in internal/autonomous/loop.go

**Checkpoint**: Autonomous operation - AI makes decisions every 100s, commands execute, feedback sent, history maintained

---

## Phase 8: User Story 6 - Configuration and Provider Support (P2)

**Goal**: Multi-provider support, multi-model pipeline, configuration

**Independent Test**: Configure Claude then local LLM, verify both work, configure pipeline with extractor + planner, verify chain executes

### Implementation for US6

#### Multi-Model Pipeline

- [ ] T146 [P] [US6] Create internal/llm/pipeline.go file
- [ ] T147 [P] [US6] Create MultiModelProvider struct with extractor + planner in internal/llm/pipeline.go
- [ ] T148 [US6] Implement NewMultiModelProvider(extractor, planner) in internal/llm/pipeline.go
- [ ] T149 [US6] Implement MultiModelProvider.SendPrompt() chaining models in internal/llm/pipeline.go
- [ ] T150 [US6] Add feature extraction system prompt in internal/llm/pipeline.go

#### Configuration

- [ ] T151 [P] [US6] Add llm_provider_type to config/orchestrator.yaml
- [ ] T152 [P] [US6] Add claude_api_key, claude_model to config/orchestrator.yaml
- [ ] T153 [P] [US6] Add llm_endpoint, llm_model for OpenAI-compatible in config/orchestrator.yaml
- [ ] T154 [P] [US6] Add llm_temperature, llm_max_tokens, llm_timeout_seconds in config/orchestrator.yaml
- [ ] T155 [P] [US6] Add context_budget_kb, context_update_frequency_seconds in config/orchestrator.yaml
- [ ] T156 [P] [US6] Add viewport_active_z_margin, viewport_hazard_margin_tiles in config/orchestrator.yaml
- [ ] T157 [US6] Update internal/config/config.go struct with all LLM fields
- [ ] T158 [US6] Update internal/config/config.go setDefaults() for LLM config

#### Provider Factory Updates

- [ ] T159 [US6] Update CreateProvider() to support multi_model type in internal/llm/factory.go
- [ ] T160 [US6] Add provider validation (check API key if needed) in internal/llm/factory.go

#### ASCII Template Support

- [ ] T161 [P] [US6] Create internal/context/templates.go file
- [ ] T162 [P] [US6] Implement LoadASCIITemplate(filepath) reader in internal/context/templates.go
- [ ] T163 [US6] Implement ParseASCIIBlueprint(template) to coordinate list in internal/context/templates.go

**Checkpoint**: Full configuration support - can swap providers, use pipelines, load templates

---

## Phase 9: Polish & Cross-Cutting Concerns

**Purpose**: Finalization and validation

- [ ] T164 [P] Add comprehensive logging (DEBUG, INFO) to all packages
- [ ] T165 [P] Add modification overlay metrics to internal/http/metrics.go
- [ ] T166 [P] Add context size metrics to internal/http/metrics.go
- [ ] T167 [P] Add LLM token usage metrics to internal/http/metrics.go
- [ ] T168 [P] Add command execution metrics to internal/http/metrics.go
- [ ] T169 Run go fmt on all new packages
- [ ] T170 Run go vet on all new packages
- [ ] T171 Build Go server and verify compilation
- [ ] T172 Build DFHack plugin and copy to DF
- [ ] T173 Test with live DFHack: modifications → context → LLM → command → ACK → feedback
- [ ] T174 Verify autonomous loop runs for 10 minutes without crashes
- [ ] T175 Verify logs/llm-interactions.jsonl contains valid JSONL entries
- [ ] T176 [P] Update quickstart.md with actual test results
- [ ] T177 [P] Document configuration examples in quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1 (Setup)**: No dependencies - can start immediately
- **Phase 2 (Foundational)**: Depends on Phase 1 - BLOCKS all user stories
- **Phase 3 (US1)**: Depends on Phase 2 - Modification tracking (MVP foundation)
- **Phase 4 (US2)**: Depends on Phase 3 - Context uses modification overlay
- **Phase 5 (US3)**: Depends on Phase 4 - LLM needs context
- **Phase 6 (US4)**: Depends on Phase 2 - Commands are independent (can parallel with US1-3)
- **Phase 7 (US5)**: Depends on US1-4 complete - Loop integrates everything
- **Phase 8 (US6)**: Can run in parallel with US5 - Config is orthogonal
- **Phase 9 (Polish)**: Depends on all user stories complete

### User Story Dependencies

- **US1 (Modifications)**: No dependencies - can start after Foundational
- **US2 (Context)**: Depends on US1 (uses modification overlay for chamber extraction)
- **US3 (LLM)**: Depends on US2 (needs context to send to LLM)
- **US4 (Commands)**: Depends on Foundational only - independent from US1-3
- **US5 (Loop)**: Depends on US1, US2, US3, US4 all complete
- **US6 (Config)**: No dependencies on other stories - can parallel

### Parallel Opportunities

**Phase 2 (Foundational)**: All [P] tasks can run together (T011-T012, T014-T017)

**Phase 3 (US1)**:
- Overlay methods: T018-T024 can run in parallel
- Chamber extraction: T030-T033 can run in parallel

**Phase 4 (US2)**:
- Level formatters: T044-T047 can run in parallel
- Feature extractors: T049-T051 can run in parallel
- Query implementations: T055-T057 can run in parallel

**Phase 5 (US3)**:
- Provider structs: T062-T065, T066, T072-T073 can run in parallel
- Parser components: T078, T082 can run in parallel

**Phase 6 (US4)**:
- Protocol messages: T091-T092, T093-T096 can run in parallel
- Plugin designation functions: T113-T114 can run in parallel

**Phase 7 (US5)**:
- Feedback templates: T125-T126 can run in parallel
- History structs: T127-T129 can run in parallel

**Phase 8 (US6)**:
- Config fields: T151-T156 all parallel
- Template support: T161-T162 can run in parallel

**Phase 9 (Polish)**: T164-T168, T169-T170, T176-T177 can run in parallel

**US4 can run in parallel with US1-3** since commands don't depend on modifications/context/LLM (different subsystems)

---

## Implementation Strategy

### MVP First (Just US1)

1. Phase 1-2: Setup + Foundational (T001-T017)
2. Phase 3: US1 Modification Tracking (T018-T037)
3. **STOP and VALIDATE**: Mine chambers in DF, verify modifications tracked
4. Delivers: Modification overlay working, chamber extraction functional

**Estimated**: ~200 LOC, testable with live DF

### Incremental Delivery Path

1. **Foundation** (Phases 1-2) → Core types ready
2. **US1 Modifications** (Phase 3) → Can track player work ✅ **MVP Checkpoint**
3. **US2 Context** (Phase 4) → Can generate LLM-ready context
4. **US3 LLM** (Phase 5) → AI can observe and respond (read-only)
5. **US4 Commands** (Phase 6) → AI can take actions ✅ **First Autonomous AI**
6. **US5 Loop** (Phase 7) → Fully autonomous with learning
7. **US6 Config** (Phase 8) → Multi-provider flexibility

Each checkpoint delivers incremental value and is independently testable.

### Parallel Development Strategy

With 3 developers:

1. All together: Phases 1-2 (foundation)
2. Split work after foundational complete:
   - **Dev A**: US1 Modifications (T018-T037)
   - **Dev B**: US4 Commands (T091-T121) - parallel!
   - **Dev C**: Setup US2 Context types (T038-T042)
3. Sequential after US1+US4:
   - Dev A: US2 Context completion (T043-T061)
   - Dev B: US3 LLM (T062-T090)
4. Integration: US5 Loop (all devs collaborate)

---

## Task Summary

**Total Tasks**: 177

**Tasks by Phase**:
- Phase 1 (Setup): 9 tasks
- Phase 2 (Foundational): 8 tasks
- Phase 3 (US1 - Modifications): 20 tasks
- Phase 4 (US2 - Context): 24 tasks
- Phase 5 (US3 - LLM): 29 tasks
- Phase 6 (US4 - Commands): 31 tasks
- Phase 7 (US5 - Loop): 24 tasks
- Phase 8 (US6 - Config): 18 tasks
- Phase 9 (Polish): 14 tasks

**Parallel Opportunities**: 78 tasks marked [P] can run in parallel

**MVP Scope**: Phases 1-3 (US1 only) = 37 tasks for modification tracking
**First Autonomous AI**: Phases 1-7 (US1-5) = 145 tasks
**Full Feature**: All 177 tasks

**Critical Path**: Foundation → US1 → US2 → US3 → US5 (US4 can parallel with US1-3)

---

## Notes

- [P] tasks = different files or independent implementations
- [Story] labels map to spec.md user stories
- US1 is MVP - modification tracking alone is valuable for fort analysis
- US4 (commands) can develop in parallel with US1-3 (different subsystem)
- US5 (loop) requires US1-4 all complete (integrator story)
- Tests deferred unless explicitly needed (research phase prioritizes iteration)
- Plugin tasks (T110-T117) require DF closed for rebuild/copy
- Commit after each user story phase for incremental progress
