# Feature Specification: Foundation Infrastructure Completion

**Feature Branch**: `002-foundation-infrastructure`
**Created**: 2025-11-06
**Status**: Draft
**Input**: User description: "Foundation Completion: Add HTTP API server, YAML/JSON configuration file loading with hot-reload support, health check endpoints, and enhanced tile data structures with memory benchmarks. This completes the basic server infrastructure to support graph layers."

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Configuration Management (Priority: P1)

When operators deploy the orchestrator to different environments (development, testing, production), they need to configure server settings without recompiling the binary. Configuration changes like port numbers, log levels, and feature flags should be externalized and easily modified.

**Why this priority**: Configuration management is foundational infrastructure. Without it, every configuration change requires rebuilding and redeploying the binary, which blocks all other development and testing workflows.

**Independent Test**: Deploy orchestrator with a YAML config file specifying port 6000 and debug logging. Start the server and verify it binds to port 6000 and outputs debug-level logs. Change config to port 7000 and info logging, reload config, verify changes take effect without restart.

**Acceptance Scenarios**:

1. **Given** a config file specifying server port 5001 and log level "info", **When** the orchestrator starts, **Then** the server binds to port 5001 and logs at info level
2. **Given** the orchestrator is running, **When** the operator modifies the config file and triggers a reload, **Then** new configuration values take effect without restarting the process
3. **Given** an invalid config file (malformed YAML or missing required fields), **When** the orchestrator attempts to load it, **Then** it logs a clear error message identifying the problem and continues using the previous valid configuration

---

### User Story 2 - Service Health Monitoring (Priority: P2)

When operating the orchestrator in production, monitoring systems (Prometheus, load balancers, Kubernetes probes) need to check if the service is healthy and ready to accept connections. Operators need visibility into connection status and system health.

**Why this priority**: Health endpoints are essential for production deployments but not strictly required for development. They enable automated monitoring, alerting, and orchestration (like Kubernetes readiness probes) that are critical for reliability.

**Independent Test**: Start the orchestrator and query the `/health` HTTP endpoint. Verify it returns HTTP 200 with JSON indicating healthy status and connection state. Disconnect the DFHack plugin and verify the health endpoint reflects degraded state.

**Acceptance Scenarios**:

1. **Given** the orchestrator is running and connected to DFHack, **When** a monitoring system queries the `/health` endpoint, **Then** it receives HTTP 200 with JSON showing status "healthy" and "dfhack_connected: true"
2. **Given** the orchestrator is running but not connected to DFHack, **When** querying `/health`, **Then** it receives HTTP 200 with status "degraded" and "dfhack_connected: false"
3. **Given** a load balancer checking readiness, **When** it queries `/ready`, **Then** it receives HTTP 200 only when the server is fully initialized and ready to process requests

---

### User Story 3 - Tile Data Structure Validation (Priority: P3)

When implementing graph layers that process millions of tiles, developers need confidence that the tile data structures are memory-efficient and performant. Benchmarks should validate that the current tile representation meets the target memory footprint and access speed requirements.

**Why this priority**: Performance validation is important for ensuring the architecture scales, but the system can function without formal benchmarks during initial development. This becomes critical before deploying with large maps or multiple graph layers.

**Independent Test**: Run memory benchmark suite on a 200x200x100 map (4M tiles). Verify that tile storage uses no more than 50 bytes per tile on average and that random access completes in under 100 nanoseconds per tile.

**Acceptance Scenarios**:

1. **Given** a benchmark testing tile storage for a 100x100x100 map, **When** the benchmark runs, **Then** it reports total memory usage and average bytes per tile
2. **Given** a benchmark testing tile access speed, **When** the benchmark runs, **Then** it reports average access time for random coordinate lookups
3. **Given** benchmark results, **When** they exceed defined thresholds (memory or speed), **Then** the benchmarks fail with a clear error indicating which metric exceeded limits

---

### Edge Cases

- What happens when a config file is modified while being read? (Use atomic file operations or file locks to prevent partial reads)
- How does the system handle a config file that exists but is empty? (Treat as error, log clearly, use defaults or previous config)
- What if hot-reload is triggered during active processing (e.g., mid-graph update)? (Queue reload until safe checkpoint, log pending reload)
- How does the system handle HTTP requests during shutdown? (Graceful shutdown: stop accepting new requests, finish in-flight requests, then close)
- What happens if benchmarks are run on a system with insufficient memory? (Benchmarks should detect and report environment constraints)

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: System MUST load configuration from a file in YAML or JSON format at startup
- **FR-002**: Configuration MUST support at minimum: server port, log level, DFHack connection settings, and feature flags
- **FR-003**: System MUST validate configuration schema on load and reject invalid configs with clear error messages
- **FR-004**: System MUST support hot-reload of configuration without restarting the process
- **FR-005**: System MUST expose an HTTP server on a configurable port separate from the TCP connection to DFHack
- **FR-006**: HTTP server MUST provide a `/health` endpoint returning JSON with system status and connection state
- **FR-007**: HTTP server MUST provide a `/ready` endpoint for readiness probes (returns 200 only when fully initialized)
- **FR-008**: HTTP server MUST provide a `/metrics` endpoint exposing basic operational metrics (connections, uptime, message counts)
- **FR-009**: System MUST include benchmarks for tile data structure memory usage (bytes per tile)
- **FR-010**: System MUST include benchmarks for tile data structure access speed (nanoseconds per lookup)
- **FR-011**: Benchmarks MUST be runnable via standard test framework (e.g., `go test -bench`)
- **FR-012**: Configuration changes via hot-reload MUST be logged with old and new values
- **FR-013**: HTTP server MUST support graceful shutdown (finish in-flight requests before closing)

### Key Entities

- **Configuration**: Represents all externalized settings for the orchestrator, including server ports, logging configuration, connection parameters, and feature flags. Loaded from YAML/JSON file and reloadable at runtime.
- **HealthStatus**: Represents the current operational state of the orchestrator, including whether it's healthy, degraded, or failing, along with details like DFHack connection status and system resource state.
- **BenchmarkResult**: Represents the outcome of performance benchmarks, including memory usage metrics (total bytes, bytes per tile) and access speed metrics (average latency, p95/p99 latencies).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Operators can deploy the orchestrator to a new environment by providing a config file, with no code changes or recompilation required
- **SC-002**: Configuration changes take effect within 5 seconds of hot-reload trigger without service downtime
- **SC-003**: Monitoring systems can determine service health by querying a single HTTP endpoint, with response time under 50ms
- **SC-004**: Health endpoint provides sufficient detail to diagnose common issues (connection status, initialization state, recent errors)
- **SC-005**: Tile data structures use less than 64 bytes per tile on average for typical maps (100x100x100)
- **SC-006**: Tile coordinate lookups complete in under 500 nanoseconds per access on standard hardware
- **SC-007**: Benchmarks run in under 10 seconds and clearly report pass/fail against defined thresholds
- **SC-008**: Invalid configuration files are rejected with error messages that allow operators to fix the issue within 2 minutes

### Assumptions

- Configuration files are stored on local filesystem accessible to the orchestrator process
- Hot-reload is triggered via signal (SIGHUP) or filesystem watch, not via HTTP API (HTTP config reload API is out of scope)
- Operators have basic familiarity with YAML/JSON syntax
- Health endpoint format follows standard conventions (JSON with status codes 200/503)
- Benchmarks run on development hardware representative of deployment targets
- HTTP API is used for monitoring only, not for control plane operations (command execution is out of scope)
- Configuration schema is defined in code, not externalized (e.g., no JSON Schema file validation)

### Out of Scope

- Authentication/authorization for HTTP endpoints (assume trusted network or add later)
- HTTPS/TLS support for HTTP server (can be added via reverse proxy if needed)
- Configuration management UI or web console (operators use text editors)
- Distributed configuration (etcd, Consul) - only local file-based config
- Historical metrics or time-series data (Prometheus scraping is supported, but storage is external)
- Advanced benchmark visualizations (results are text output only)
- Configuration encryption or secrets management (assume secure deployment environment)
