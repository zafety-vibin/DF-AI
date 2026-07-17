# Tasks: DFHack Binary Protocol

**Input**: Design documents from `/specs/001-binary-protocol/`
**Prerequisites**: plan.md (required), spec.md (required for user stories), data-model.md, contracts/

**Tests**: Tests are NOT explicitly requested in the feature specification. Tasks focus on implementation only.

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3, US4)
- Include exact file paths in descriptions

## Path Conventions

- **Go Server**: `cmd/df-orchestrator/`, `internal/protocol/`, `internal/dfhack/`, `internal/logging/`
- **C++ Plugin**: `dfhack-plugin/`
- **Config**: `config/`
- **Tests**: `tests/protocol/`, `tests/testdata/`

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and basic structure

- [x] T001 Create Go project structure with cmd/, internal/, tests/, config/ directories
- [x] T002 Initialize Go module with `go mod init` and set Go version to 1.21+
- [x] T003 [P] Create C++ plugin directory structure dfhack-plugin/ with CMakeLists.txt
- [x] T004 [P] Create config/orchestrator.yaml with default configuration (port 5001, timeouts, log settings)
- [x] T005 [P] Create .gitignore for Go binaries and C++ build artifacts

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [x] T006 [P] Create internal/logging/logger.go with structured JSON logging using log/slog
- [x] T007 [P] Implement log levels (DEBUG, INFO, WARN, ERROR) and format configuration in internal/logging/logger.go
- [x] T008 [P] Create internal/dfhack/types.go defining TileState struct (X, Y, Z int16, TileType uint16, Flags uint8)
- [x] T009 [P] Create internal/protocol/version.go with protocol version constant (ProtocolVersion = 1)
- [x] T010 [P] Create internal/protocol/message.go defining Message interface with Type(), Serialize(), Deserialize(), Validate() methods
- [x] T011 Define ConnectionState enum (Disconnected, Connecting, Connected, Error) in internal/protocol/connection.go
- [x] T012 [P] Create config loading utility in internal/config/config.go to parse orchestrator.yaml
- [x] T013 [P] Create dfhack-plugin/protocol.h defining C++ protocol constants and message type enums

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Initial Fort State Synchronization (Priority: P1) 🎯 MVP

**Goal**: Go server can connect to DFHack plugin and receive complete initial map state

**Independent Test**: Start DF with existing fort, start Go server, verify server receives all tile states for entire map within 2 seconds

### Implementation for User Story 1

- [x] T014 [P] [US1] Implement HandshakeMessage struct in internal/protocol/message.go (ProtocolVersion uint8, ConnectionID uint32, Capabilities string)
- [x] T015 [P] [US1] Implement FullStateMessage struct in internal/protocol/message.go (Width, Height, Depth uint16, Tiles []TileState)
- [x] T016 [P] [US1] Implement ResyncRequestMessage struct in internal/protocol/message.go (Reason uint8)
- [x] T017 [US1] Implement binary codec for HandshakeMessage serialization/deserialization in internal/protocol/codec.go (big-endian encoding)
- [x] T018 [US1] Implement binary codec for FullStateMessage serialization/deserialization in internal/protocol/codec.go (handle large tile arrays efficiently)
- [x] T019 [US1] Implement binary codec for ResyncRequestMessage serialization/deserialization in internal/protocol/codec.go
- [x] T020 [US1] Implement Connection type in internal/protocol/connection.go with Connect(host, port) method using net.Dial
- [x] T021 [US1] Implement Connection.Send(Message) method in internal/protocol/connection.go with length-prefixed framing
- [x] T022 [US1] Implement Connection.Receive() method in internal/protocol/connection.go with message length validation
- [x] T023 [US1] Implement Connection.State() and ConnectionState management in internal/protocol/connection.go
- [x] T024 [US1] Implement TCP server listener in internal/dfhack/client.go with Start(ctx, port) method
- [x] T025 [US1] Implement RequestFullState(reason) method in internal/dfhack/client.go returning channel for FullStateMessage
- [x] T026 [US1] Add handshake exchange logic to internal/dfhack/client.go (send/receive HANDSHAKE, validate protocol version)
- [x] T027 [US1] Add full state request logic to internal/dfhack/client.go (send RESYNC_REQUEST, receive FULL_STATE)
- [x] T028 [US1] Implement Go server main() in cmd/df-orchestrator/main.go (load config, start server, handle shutdown)
- [x] T029 [US1] Implement C++ plugin initialization in dfhack-plugin/df_ai_protocol.cpp (plugin_init, plugin_enable, register commands)
- [x] T030 [US1] Implement TCP client connection logic in dfhack-plugin/df_ai_protocol.cpp using SimpleSockets ActiveSocket
- [x] T031 [US1] Implement tile data extraction in dfhack-plugin/tile_extractor.cpp using MapCache and df::map_block
- [x] T032 [US1] Implement full map iteration in dfhack-plugin/tile_extractor.cpp (iterate 16x16 blocks, extract all tiles)
- [x] T033 [US1] Implement C++ handshake message send/receive in dfhack-plugin/df_ai_protocol.cpp
- [x] T034 [US1] Implement C++ full state message serialization in dfhack-plugin/protocol.h (pack tiles into binary format)
- [x] T035 [US1] Implement C++ full state message transmission in dfhack-plugin/df_ai_protocol.cpp (send on RESYNC_REQUEST)
- [x] T036 [US1] Add CMake configuration in dfhack-plugin/CMakeLists.txt (DFHACK_PLUGIN macro, link clsocket)
- [x] T037 [US1] Create dfhack-plugin/README.md with build instructions for Windows (MSVC 2022) and Linux (GCC 7+)

**Checkpoint**: At this point, User Story 1 should be fully functional and testable independently - Go server can receive initial fort state

---

## Phase 4: User Story 2 - Real-Time Change Notifications (Priority: P2)

**Goal**: Go server receives incremental tile updates as game progresses

**Independent Test**: With Go server connected, designate mining in DF, wait for dwarves to mine, verify server is notified when tiles change

### Implementation for User Story 2

- [x] T038 [P] [US2] Implement TileUpdateMessage struct in internal/protocol/message.go (Count uint32, Tiles []TileState)
- [x] T039 [US2] Implement binary codec for TileUpdateMessage in internal/protocol/codec.go with batching support
- [x] T040 [US2] Implement SubscribeTileUpdates() method in internal/dfhack/client.go returning channel for TileUpdateMessage
- [x] T041 [US2] Add message routing logic in internal/dfhack/client.go to distribute incoming messages to appropriate channels
- [x] T042 [US2] Implement tile change polling in dfhack-plugin/tile_extractor.cpp (scan blocks periodically, detect changes)
- [x] T043 [US2] Implement tile change detection logic in dfhack-plugin/tile_extractor.cpp (compare current state to cached state)
- [x] T044 [US2] Implement tile update batching in dfhack-plugin/tile_extractor.cpp (collect changes, batch up to 1000 tiles)
- [x] T045 [US2] Implement C++ TILE_UPDATE message serialization in dfhack-plugin/protocol.h
- [x] T046 [US2] Implement C++ TILE_UPDATE message transmission in dfhack-plugin/df_ai_protocol.cpp (send batched updates)
- [x] T047 [US2] Add polling timer to dfhack-plugin/df_ai_protocol.cpp (configurable poll_interval_ms, default 1000ms)
- [x] T048 [US2] Add tile update processing to cmd/df-orchestrator/main.go (subscribe to updates, log received changes)

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - Go server receives both initial state and incremental updates

---

## Phase 5: User Story 3 - Connection Recovery (Priority: P3)

**Goal**: System automatically recovers from connection failures without crashing

**Independent Test**: Forcibly kill connection, verify both sides detect failure and automatically reconnect and resync

### Implementation for User Story 3

- [x] T049 [P] [US3] Implement HeartbeatMessage struct in internal/protocol/message.go (Timestamp uint64, Sequence uint8)
- [x] T050 [P] [US3] Implement DisconnectMessage struct in internal/protocol/message.go (Reason uint8)
- [x] T051 [US3] Implement binary codec for HeartbeatMessage in internal/protocol/codec.go
- [x] T052 [US3] Implement binary codec for DisconnectMessage in internal/protocol/codec.go
- [x] T053 [US3] Implement heartbeat sender in internal/dfhack/client.go (send HEARTBEAT every 10s with timestamp and sequence)
- [x] T054 [US3] Implement heartbeat echo responder in internal/dfhack/client.go (receive HEARTBEAT, increment sequence, echo back)
- [x] T055 [US3] Implement heartbeat timeout detection in internal/dfhack/client.go (mark connection dead if no response within 5s)
- [x] T056 [US3] Implement Connection.Close() method in internal/protocol/connection.go (send DISCONNECT, flush, close socket)
- [x] T057 [US3] Implement reconnection logic in dfhack-plugin/df_ai_protocol.cpp with exponential backoff (1s, 2s, 4s, 8s, 16s, max 30s)
- [x] T058 [US3] Implement connection state monitoring in dfhack-plugin/df_ai_protocol.cpp (detect TCP errors, timeouts)
- [x] T059 [US3] Implement C++ heartbeat echo logic in dfhack-plugin/df_ai_protocol.cpp (receive, increment, send back)
- [x] T060 [US3] Implement C++ heartbeat timeout detection in dfhack-plugin/df_ai_protocol.cpp (disconnect if no heartbeat for 15s)
- [x] T061 [US3] Implement automatic resync on reconnection in internal/dfhack/client.go (send RESYNC_REQUEST with reason=0x02)
- [x] T062 [US3] Add graceful shutdown handling to cmd/df-orchestrator/main.go (send DISCONNECT on SIGINT/SIGTERM)
- [x] T063 [US3] Add graceful shutdown handling to dfhack-plugin/df_ai_protocol.cpp (plugin_disable sends DISCONNECT)

**Checkpoint**: All user stories should now be independently functional - System survives connection failures and reconnects automatically

---

## Phase 6: User Story 4 - Diagnostic Logging (Priority: P4)

**Goal**: Comprehensive logging for debugging and performance analysis

**Independent Test**: Trigger various protocol events, verify all events appear in structured logs with timestamps and details

### Implementation for User Story 4

- [x] T064 [P] [US4] Implement ErrorMessage struct in internal/protocol/message.go (Code uint16, Message string)
- [x] T065 [US4] Implement binary codec for ErrorMessage in internal/protocol/codec.go with UTF-8 string encoding
- [x] T066 [US4] Add connection event logging to internal/protocol/connection.go (connection_opened, connection_closed)
- [x] T067 [US4] Add message event logging to internal/protocol/connection.go (message_sent, message_received with type, size, duration)
- [x] T068 [US4] Add error event logging to internal/protocol/connection.go (error_occurred with error code and context)
- [x] T069 [US4] Add heartbeat event logging to internal/dfhack/client.go (heartbeat round-trip time tracking)
- [x] T070 [US4] Add resync event logging to internal/dfhack/client.go (resync_triggered with reason, duration, tile count)
- [x] T071 [US4] Implement SessionMetrics tracking in internal/dfhack/client.go (messages sent/received, bytes, errors, timestamps)
- [x] T072 [US4] Implement GetSessionMetrics() method in internal/dfhack/client.go returning current session statistics
- [x] T073 [US4] Add comprehensive logging to dfhack-plugin/df_ai_protocol.cpp (all protocol events with timestamps)
- [x] T074 [US4] Implement log file output in dfhack-plugin/df_ai_protocol.cpp (write to df_ai_protocol.log in DF folder)
- [x] T075 [US4] Add performance metrics logging to dfhack-plugin/tile_extractor.cpp (tile extraction time, serialization time)
- [x] T076 [US4] Create plugin config file config/df_ai_protocol.yaml with server_host, server_port, poll_interval_ms, log_level

**Checkpoint**: All user stories should now be independently functional with comprehensive logging for debugging

---

## Phase 7: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories

- [ ] T077 [P] Add input validation to all message Deserialize() methods in internal/protocol/codec.go (length checks, coordinate bounds)
- [ ] T078 [P] Implement message integrity validation in internal/protocol/connection.go (detect truncated messages)
- [ ] T079 [P] Add protocol version mismatch detection to internal/dfhack/client.go (send ERROR 0x0003 on version mismatch)
- [ ] T080 [P] Implement max message size enforcement in internal/protocol/connection.go (reject messages >10MB)
- [ ] T081 [P] Add connection timeout configuration in config/orchestrator.yaml (read_timeout, write_timeout)
- [ ] T082 [P] Implement MockDFHackClient in internal/dfhack/mock_client.go for testing (simulate HANDSHAKE, FULL_STATE, TILE_UPDATE)
- [ ] T083 Create integration test in tests/protocol/integration_test.go (full connection flow with mock plugin)
- [ ] T084 Create codec benchmark tests in tests/protocol/codec_test.go (benchmark serialization/deserialization of 100K tiles)
- [ ] T085 Create connection unit tests in tests/protocol/connection_test.go (test state transitions, error handling)
- [ ] T086 Create test fixtures in tests/testdata/fixtures/ (sample tile data for 100x100x10 map)
- [ ] T087 Update dfhack-plugin/README.md with installation instructions (copy plugin to hack/plugins/, edit dfhack.init)
- [ ] T088 Add performance benchmarking command to dfhack-plugin/df_ai_protocol.cpp (ai-benchmark full-state, ai-benchmark tile-updates)
- [ ] T089 Add FPS impact monitoring to dfhack-plugin/df_ai_protocol.cpp (log FPS before/after enabling plugin)
- [ ] T090 Create quickstart validation script to test end-to-end functionality per quickstart.md

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-6)**: All depend on Foundational phase completion
  - User stories can then proceed in parallel (if staffed)
  - Or sequentially in priority order (P1 → P2 → P3 → P4)
- **Polish (Phase 7)**: Depends on all desired user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Can start after Foundational (Phase 2) - No dependencies on other stories
- **User Story 2 (P2)**: Can start after Foundational (Phase 2) - Builds on US1 protocol but independently testable
- **User Story 3 (P3)**: Can start after Foundational (Phase 2) - Builds on US1 connection but independently testable
- **User Story 4 (P4)**: Can start after Foundational (Phase 2) - Enhances all stories but independently testable

### Within Each User Story

- **US1**: Logging foundation → Message types → Codec → Connection → Client → Plugin → Integration
- **US2**: Message types → Codec → Polling → Batching → Transmission
- **US3**: Heartbeat messages → Timeout detection → Reconnection → Graceful shutdown
- **US4**: Error messages → Event logging → Metrics tracking → Config

### Parallel Opportunities

- All Setup tasks marked [P] can run in parallel
- All Foundational tasks marked [P] can run in parallel (within Phase 2)
- Once Foundational phase completes, all user stories can start in parallel (if team capacity allows)
- Within each story:
  - All tasks marked [P] can run in parallel
  - US1: T014-T016 (message structs), T017-T019 (codecs), T029-T032 (C++ core) can run in parallel
  - US2: T038-T039 (message and codec) can run in parallel with T042-T044 (C++ detection logic)
  - US3: T049-T052 (message structs and codecs) can run in parallel
  - US4: T064-T065 can run in parallel, T066-T072 (logging additions) can run in parallel
- Different user stories can be worked on in parallel by different team members

---

## Parallel Example: User Story 1

```bash
# Launch all message struct definitions together:
Task T014: "Implement HandshakeMessage struct in internal/protocol/message.go"
Task T015: "Implement FullStateMessage struct in internal/protocol/message.go"
Task T016: "Implement ResyncRequestMessage struct in internal/protocol/message.go"

# Then launch all codec implementations together:
Task T017: "Implement binary codec for HandshakeMessage"
Task T018: "Implement binary codec for FullStateMessage"
Task T019: "Implement binary codec for ResyncRequestMessage"

# Launch C++ core components together:
Task T029: "Implement C++ plugin initialization"
Task T030: "Implement TCP client connection logic"
Task T031: "Implement tile data extraction"
Task T032: "Implement full map iteration"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup
2. Complete Phase 2: Foundational (CRITICAL - blocks all stories)
3. Complete Phase 3: User Story 1 (T014-T037)
4. **STOP and VALIDATE**: Test User Story 1 independently
   - Start DF with existing fort
   - Start Go server
   - Verify connection established
   - Verify full state received within 2 seconds
   - Verify tile data accuracy
5. Deploy/demo if ready

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 (T014-T037) → Test independently → Deploy/Demo (MVP!)
3. Add User Story 2 (T038-T048) → Test independently → Deploy/Demo (now with live updates)
4. Add User Story 3 (T049-T063) → Test independently → Deploy/Demo (now with resilience)
5. Add User Story 4 (T064-T076) → Test independently → Deploy/Demo (now with full observability)
6. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together
2. Once Foundational is done:
   - Developer A: User Story 1 (Go server core)
   - Developer B: User Story 1 (C++ plugin core)
   - Developer C: User Story 2 (incremental updates)
3. Stories complete and integrate independently

With single developer (recommended order):

1. Setup (T001-T005)
2. Foundational (T006-T013)
3. US1 Go server (T014-T028)
4. US1 C++ plugin (T029-T037)
5. Validate US1 end-to-end
6. US2 (T038-T048)
7. Validate US2 with US1
8. US3 (T049-T063)
9. Validate US3 with US1+US2
10. US4 (T064-T076)
11. Polish (T077-T090)

---

## Task Count Summary

- **Total Tasks**: 90
- **Setup (Phase 1)**: 5 tasks
- **Foundational (Phase 2)**: 8 tasks (BLOCKS user stories)
- **User Story 1 (MVP)**: 24 tasks (T014-T037)
- **User Story 2**: 11 tasks (T038-T048)
- **User Story 3**: 15 tasks (T049-T063)
- **User Story 4**: 13 tasks (T064-T076)
- **Polish**: 14 tasks (T077-T090)

### Parallel Opportunities

- **Setup Phase**: 3 of 5 tasks can run in parallel
- **Foundational Phase**: 6 of 8 tasks can run in parallel
- **US1**: Up to 6 tasks can run in parallel at key points
- **US2**: Up to 3 tasks can run in parallel
- **US3**: Up to 4 tasks can run in parallel
- **US4**: Up to 10 tasks can run in parallel
- **Between Stories**: All 4 user stories can proceed in parallel after Foundational phase

### Suggested MVP Scope

**Minimum Viable Product** = Setup + Foundational + User Story 1 = 37 tasks

This delivers:
- ✅ Full connection establishment
- ✅ Protocol handshake with version negotiation
- ✅ Complete fort state synchronization
- ✅ Binary protocol implementation (Go and C++)
- ✅ Basic logging
- ✅ Independent testability

Estimated effort: 37 tasks × 1-2 hours per task = 37-74 hours for a working MVP

---

## Notes

- [P] tasks = different files, no dependencies
- [Story] label maps task to specific user story for traceability
- Each user story should be independently completable and testable
- File paths are explicit for all implementation tasks
- C++ and Go tasks can proceed in parallel within each story
- Commit after each task or logical group
- Stop at any checkpoint to validate story independently
- Avoid: vague tasks, same file conflicts, cross-story dependencies that break independence