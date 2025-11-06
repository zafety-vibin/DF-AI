# Tasks: Foundation Infrastructure Completion

**Input**: Design documents from `/specs/002-foundation-infrastructure/`
**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/, quickstart.md

**Tests**: Not requested in feature specification - NO test tasks included

**Organization**: Tasks are grouped by user story to enable independent implementation and testing of each story.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel (different files, no dependencies)
- **[Story]**: Which user story this task belongs to (e.g., US1, US2, US3)
- Include exact file paths in descriptions

## Path Conventions

This is a single Go project with structure:
- `cmd/df-orchestrator/` - Entry point
- `internal/` - Internal packages
- `tests/` - Test suite
- `config/` - Configuration files

---

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Project initialization and dependency installation

- [ ] T001 Install `github.com/fsnotify/fsnotify` dependency via `go get github.com/fsnotify/fsnotify`
- [ ] T002 [P] Install `golang.org/x/sync/errgroup` dependency via `go get golang.org/x/sync/errgroup`
- [ ] T003 [P] Create `internal/http/` directory for HTTP server package

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Core infrastructure that MUST be complete before ANY user story can be implemented

**⚠️ CRITICAL**: No user story work can begin until this phase is complete

- [ ] T004 Add `HttpPort` uint16 field to Config struct in `internal/config/config.go`
- [ ] T005 [P] Add `EnableHttpApi` bool field to Config struct in `internal/config/config.go`
- [ ] T006 [P] Update `setDefaults()` method in `internal/config/config.go` to set HttpPort=8080 and EnableHttpApi=true if unset
- [ ] T007 Update `config/orchestrator.yaml` to include `http_port: 8080` and `enable_http_api: true`

**Checkpoint**: Foundation ready - user story implementation can now begin in parallel

---

## Phase 3: User Story 1 - Configuration Management (Priority: P1) 🎯 MVP

**Goal**: Enable operators to configure the orchestrator via YAML file with hot-reload support, allowing configuration changes without service downtime.

**Independent Test**: Start orchestrator with config file specifying port 6000 and debug logging. Verify server binds to port 6000. Modify config to port 7000 and info logging. Verify hot-reload applies changes without restart.

### Implementation for User Story 1

- [ ] T008 [US1] Create `internal/config/watcher.go` with ConfigManager struct (config *Config, mu sync.RWMutex, watcher *fsnotify.Watcher, configPath string, logger *logging.Logger, reloadCh chan *Config)
- [ ] T009 [US1] Implement `NewConfigManager(path string, logger *logging.Logger) (*ConfigManager, error)` in `internal/config/watcher.go` to initialize ConfigManager with initial config load
- [ ] T010 [US1] Implement `Get() *Config` method in `internal/config/watcher.go` to return current config with read lock
- [ ] T011 [US1] Implement `Watch(ctx context.Context)` goroutine in `internal/config/watcher.go` using fsnotify to watch config file for Write events
- [ ] T012 [US1] Implement `reload(newConfig *Config)` method in `internal/config/watcher.go` to atomically swap config pointer with write lock
- [ ] T013 [US1] Add config validation in `internal/config/watcher.go` reload path: validate port ranges (1024-65535), enum values (log_level, log_format), HttpPort != ListenPort
- [ ] T014 [US1] Add structured logging in `internal/config/watcher.go` for config reload events with old/new value changes
- [ ] T015 [US1] Update `cmd/df-orchestrator/main.go` to use ConfigManager instead of direct Config.Load() for config management
- [ ] T016 [US1] Update `cmd/df-orchestrator/main.go` to start ConfigManager.Watch() goroutine with context after loading config

**Checkpoint**: At this point, User Story 1 should be fully functional - config hot-reload working independently

---

## Phase 4: User Story 2 - Service Health Monitoring (Priority: P2)

**Goal**: Provide HTTP monitoring endpoints (/health, /ready, /metrics) for monitoring systems to check orchestrator status and connection health.

**Independent Test**: Start orchestrator and query `/health` endpoint. Verify HTTP 200 with JSON showing healthy status and DFHack connection state. Disconnect DFHack and verify `/health` reflects degraded state.

### Implementation for User Story 2

- [ ] T017 [P] [US2] Create `internal/http/server.go` with Server struct (httpServer *http.Server, logger *logging.Logger, dfhackClient *dfhack.Client, configMgr *config.ConfigManager, startTime time.Time, configReloads uint64, httpRequests uint64, mu sync.Mutex)
- [ ] T018 [P] [US2] Create `internal/http/health.go` with HealthStatus struct (Status string, UptimeSeconds int64, DFHackConnected bool, DFHackConnectionTime *time.Time, LastHeartbeat *time.Time, Timestamp time.Time)
- [ ] T019 [P] [US2] Create `internal/http/ready.go` with ReadinessStatus struct (Ready bool, Checks map[string]bool, Message string)
- [ ] T020 [P] [US2] Create `internal/http/metrics.go` with MetricsSnapshot struct (ServerUptimeSeconds int64, DFHackConnected bool, Session *dfhack.SessionMetrics, ConfigReloads uint64, HttpRequests uint64, Timestamp time.Time)
- [ ] T021 [US2] Implement `NewServer(logger *logging.Logger, dfhackClient *dfhack.Client, configMgr *config.ConfigManager) *Server` constructor in `internal/http/server.go`
- [ ] T022 [US2] Implement `Start(ctx context.Context) error` method in `internal/http/server.go` to start HTTP server on HttpPort from config with http.HandleFunc registrations
- [ ] T023 [US2] Implement `healthHandler(w http.ResponseWriter, r *http.Request)` in `internal/http/health.go` to query DFHack client connection status and return HealthStatus JSON
- [ ] T024 [US2] Add health status determination logic in `internal/http/health.go`: "healthy" if DFHack connected + heartbeat <30s, "degraded" if disconnected, "unhealthy" if config error
- [ ] T025 [US2] Implement `readyHandler(w http.ResponseWriter, r *http.Request)` in `internal/http/ready.go` to check readiness criteria and return HTTP 200/503 with ReadinessStatus JSON
- [ ] T026 [US2] Add readiness checks in `internal/http/ready.go`: config_loaded, tcp_listener (check if dfhack client listener != nil), http_server (self-check), logger
- [ ] T027 [US2] Implement `metricsHandler(w http.ResponseWriter, r *http.Request)` in `internal/http/metrics.go` to aggregate metrics from dfhack.Client.GetSessionMetrics() and server state, return MetricsSnapshot JSON
- [ ] T028 [US2] Add HTTP request counter increment in `internal/http/server.go` middleware that wraps all handlers
- [ ] T029 [US2] Implement `Stop(ctx context.Context) error` in `internal/http/server.go` using http.Server.Shutdown() for graceful shutdown
- [ ] T030 [US2] Update `cmd/df-orchestrator/main.go` to initialize HTTP Server if EnableHttpApi is true in config
- [ ] T031 [US2] Update `cmd/df-orchestrator/main.go` to start HTTP server in errgroup.Group alongside DFHack client for coordinated shutdown
- [ ] T032 [US2] Update `cmd/df-orchestrator/main.go` to handle SIGINT/SIGTERM signals and trigger context cancellation for graceful shutdown of both servers

**Checkpoint**: At this point, User Stories 1 AND 2 should both work independently - HTTP endpoints queryable, config hot-reload functional

---

## Phase 5: User Story 3 - Tile Data Structure Validation (Priority: P3)

**Goal**: Provide automated performance benchmarks to validate that tile data structures meet memory efficiency (<64 bytes/tile) and access speed (<500ns/lookup) requirements before implementing graph layers.

**Independent Test**: Run `go test -bench=. ./tests/benchmark/` and verify benchmarks report memory usage and access speed metrics. Confirm thresholds are validated automatically.

### Implementation for User Story 3

- [ ] T033 [P] [US3] Create `tests/benchmark/` directory for benchmark test files
- [ ] T034 [P] [US3] Create `tests/benchmark/tile_memory_test.go` with table-driven BenchmarkTileMemory tests for three map sizes: Small (48x48x20), Medium (100x100x100), Large (200x200x100)
- [ ] T035 [US3] Implement memory allocation benchmark in `tests/benchmark/tile_memory_test.go` using b.ReportAllocs() to track bytes per tile for each map size
- [ ] T036 [US3] Add threshold validation in `tests/benchmark/tile_memory_test.go` to fail benchmark if bytes/op exceeds 64 bytes per tile with clear error message
- [ ] T037 [P] [US3] Create `tests/benchmark/tile_access_test.go` with table-driven BenchmarkTileAccess tests for three map sizes using random coordinate lookups
- [ ] T038 [US3] Implement access speed benchmark in `tests/benchmark/tile_access_test.go` measuring nanoseconds per coordinate lookup operation
- [ ] T039 [US3] Add threshold validation in `tests/benchmark/tile_access_test.go` to fail benchmark if ns/op exceeds 500 nanoseconds with clear error message
- [ ] T040 [US3] Enhance `internal/dfhack/types.go` TileData struct with benchmark-specific helper methods if needed (e.g., NewTileMap, GetTile, SetTile)
- [ ] T041 [US3] Document benchmark execution in quickstart.md "Running Benchmarks" section (already exists - verify accuracy)

**Checkpoint**: All user stories should now be independently functional - config reload working, HTTP endpoints live, benchmarks automated

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Improvements that affect multiple user stories and final validation

- [ ] T042 [P] Add ConfigManager.GetConfigReloads() uint64 counter method in `internal/config/watcher.go` to track successful reload count
- [ ] T043 [P] Increment config reload counter in ConfigManager when reload succeeds in `internal/config/watcher.go`
- [ ] T044 [P] Update MetricsSnapshot in `internal/http/metrics.go` to query ConfigManager.GetConfigReloads() for config_reloads field
- [ ] T045 Add error logging in `internal/config/watcher.go` when config validation fails during reload (log error, keep old config)
- [ ] T046 Add "requires restart" detection in `internal/config/watcher.go` for fields like ListenPort that cannot be hot-reloaded (log warning message)
- [ ] T047 [P] Add OpenAPI contract validation: ensure `contracts/http-api.yaml` matches actual JSON response structures from health.go, ready.go, metrics.go
- [ ] T048 Run quickstart.md validation: manually test config reload, HTTP endpoints, benchmarks as documented
- [ ] T049 Final integration test: Start server, reload config, query all 3 HTTP endpoints, run benchmarks, verify all working together

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies - can start immediately
- **Foundational (Phase 2)**: Depends on Setup completion - BLOCKS all user stories
- **User Stories (Phase 3-5)**: All depend on Foundational phase completion
  - User Story 1 (Config Management): Can start after Foundational - No dependencies on other stories
  - User Story 2 (HTTP Monitoring): Can start after Foundational - No dependencies on other stories (references dfhack.Client which already exists)
  - User Story 3 (Benchmarks): Can start after Foundational - No dependencies on other stories
- **Polish (Phase 6)**: Depends on all user stories being complete

### User Story Dependencies

- **User Story 1 (P1)**: Independent - can implement and test alone
- **User Story 2 (P2)**: Independent - can implement and test alone (uses existing dfhack.Client)
- **User Story 3 (P3)**: Independent - can implement and test alone

**All user stories are independently testable per spec requirements.**

### Within Each User Story

- US1: ConfigManager struct → methods → integration in main.go
- US2: Data structures (parallel) → Server setup → Handler implementations → main.go integration
- US3: Benchmark files (parallel) → Threshold validation → Documentation verification

### Parallel Opportunities

**Phase 1 (Setup)**:
- T001, T002, T003 can all run in parallel (different operations)

**Phase 2 (Foundational)**:
- T005, T006 can run in parallel with T004 (different fields)

**Phase 3 (US1)**:
- All tasks sequential due to logical dependencies (struct → methods → integration)

**Phase 4 (US2)**:
- T017, T018, T019, T020 can all run in parallel (different files)
- T023-T027 can run in parallel (different handlers in different files)

**Phase 5 (US3)**:
- T033, T034, T037 can run in parallel (different files)

**Phase 6 (Polish)**:
- T042, T043, T044, T047 can run in parallel (different files/concerns)

**User Stories (after Foundational)**:
- All 3 user stories can be worked on in parallel by different team members since they're independent

---

## Parallel Example: User Story 2

```bash
# Launch all data structure creation tasks together:
Task: "Create internal/http/server.go with Server struct"
Task: "Create internal/http/health.go with HealthStatus struct"
Task: "Create internal/http/ready.go with ReadinessStatus struct"
Task: "Create internal/http/metrics.go with MetricsSnapshot struct"

# Later, launch all handler implementations together:
Task: "Implement healthHandler in internal/http/health.go"
Task: "Implement readyHandler in internal/http/ready.go"
Task: "Implement metricsHandler in internal/http/metrics.go"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Phase 1: Setup (install dependencies)
2. Complete Phase 2: Foundational (extend Config struct)
3. Complete Phase 3: User Story 1 (config hot-reload)
4. **STOP and VALIDATE**: Test config reload independently - modify config file, verify changes take effect without restart
5. Deploy/demo if ready - operators can now change config without downtime

### Incremental Delivery

1. Complete Setup + Foundational → Foundation ready
2. Add User Story 1 → Test config reload independently → Deploy/Demo (MVP!)
3. Add User Story 2 → Test HTTP endpoints independently → Deploy/Demo (monitoring enabled!)
4. Add User Story 3 → Run benchmarks independently → Deploy/Demo (performance validated!)
5. Each story adds value without breaking previous stories

### Parallel Team Strategy

With multiple developers:

1. Team completes Setup + Foundational together (T001-T007)
2. Once Foundational is done (T007 complete):
   - Developer A: User Story 1 (T008-T016) - Config hot-reload
   - Developer B: User Story 2 (T017-T032) - HTTP monitoring
   - Developer C: User Story 3 (T033-T041) - Benchmarks
3. Stories complete and integrate independently
4. Team reconvenes for Polish phase (T042-T049)

---

## Notes

- [P] tasks = different files, no dependencies - can run concurrently
- [Story] label (US1, US2, US3) maps task to specific user story for traceability
- Each user story should be independently completable and testable per spec acceptance scenarios
- Tests NOT included per spec (not explicitly requested)
- Commit after each task or logical group for clean git history
- Stop at any checkpoint to validate story independently before proceeding
- ConfigManager uses fsnotify for cross-platform file watching (Windows/Linux compatible)
- HTTP server uses stdlib net/http (no external framework dependencies per research.md decision)
- Benchmarks use Go testing.B framework (standard approach per research.md)
- Graceful shutdown uses errgroup for coordinated server lifecycle per research.md
