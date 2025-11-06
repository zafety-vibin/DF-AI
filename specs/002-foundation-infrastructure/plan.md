# Implementation Plan: Foundation Infrastructure Completion

**Branch**: `002-foundation-infrastructure` | **Date**: 2025-11-06 | **Spec**: [spec.md](./spec.md)

## Summary

Add HTTP API server for health/metrics endpoints, enhance configuration management with hot-reload support, and create performance benchmarks for tile data structures. This completes the foundational server infrastructure needed before building graph layers.

**Primary Requirement**: Operators need externalized configuration, health monitoring endpoints, and validated performance characteristics for tile storage before implementing memory-intensive graph layers.

**Technical Approach**: Extend existing YAML config system with hot-reload via file watching, add lightweight HTTP server for monitoring endpoints (using Go stdlib `net/http`), and implement Go benchmark tests for tile data structure performance validation.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**:
- `gopkg.in/yaml.v3` (already in use for config)
- `github.com/fsnotify/fsnotify` (file watching for hot-reload)
- Go stdlib `net/http` (HTTP server - no external framework needed)
**Storage**: Filesystem (config files), in-memory (metrics, health state)
**Testing**: Go testing framework (`go test`, `go test -bench`)
**Target Platform**: Windows/Linux server (cross-platform Go binary)
**Project Type**: Single project (server daemon)
**Performance Goals**:
- Config hot-reload: <5s to take effect
- Health endpoint latency: <50ms response time
- Tile storage: <64 bytes per tile average
- Tile access: <500ns per coordinate lookup
**Constraints**:
- No service downtime during config reload
- HTTP server must not interfere with existing TCP protocol server
- Benchmarks must be reproducible and automated
**Scale/Scope**:
- Config: ~20 fields across network, protocol, logging sections
- HTTP API: 3 endpoints (/health, /ready, /metrics)
- Benchmarks: 2 test suites (memory usage, access speed)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle II: Comprehensive Logging & Observability ✅

**Requirement**: Log EVERYTHING with structured format, track metrics

**How this feature supports it**:
- HTTP `/metrics` endpoint exposes operational metrics (connections, uptime, message counts, errors)
- Config hot-reload logs old/new values for audit trail
- Health endpoint provides structured JSON status for monitoring systems

**Status**: PASS - Feature directly enables observability goals

---

### Principle III: Modularity & Pluggability ✅

**Requirement**: Configuration-driven component selection, clear interfaces

**How this feature supports it**:
- YAML config already supports modular settings (network, protocol, logging)
- Hot-reload enables changing behavior without code changes
- HTTP server runs independently from TCP protocol server (separate goroutines)

**Status**: PASS - Enhances modularity

---

### Principle IV: Lean & Efficient (Local LLM Compatible) ✅

**Requirement**: Resource-efficient, no unnecessary dependencies

**How this feature supports it**:
- Uses Go stdlib `net/http` (no heavy frameworks)
- Minimal dependency addition (`fsnotify` for file watching is ~10KB)
- Benchmarks validate memory efficiency (<64 bytes per tile target)
- Performance validation ensures no resource regressions

**Status**: PASS - Maintains lean architecture

---

### Principle VII: Simplicity & Clarity ✅

**Requirement**: Simple, readable code. Avoid abstractions until needed in 3+ places.

**How this feature supports it**:
- HTTP server uses standard Go patterns (http.HandleFunc)
- Config reload: simple file watch → reload → swap pattern
- Benchmarks use standard `testing.B` framework
- No premature abstractions (direct implementation of 3 endpoints)

**Status**: PASS - Straightforward implementation

---

**Constitution Check Result**: ✅ ALL PRINCIPLES ALIGNED

No violations detected. This feature strengthens the foundation for research by improving observability and validating performance assumptions.

## Project Structure

### Documentation (this feature)

```text
specs/002-foundation-infrastructure/
├── plan.md              # This file
├── research.md          # Phase 0 output (decisions on HTTP patterns, hot-reload strategy)
├── data-model.md        # Phase 1 output (Config, HealthStatus, BenchmarkResult structures)
├── quickstart.md        # Phase 1 output (how to use config, query endpoints, run benchmarks)
├── contracts/           # Phase 1 output (HTTP API spec)
│   └── http-api.yaml    # OpenAPI spec for /health, /ready, /metrics
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT YET CREATED)
```

### Source Code (repository root)

```text
# Existing structure (from 001-binary-protocol)
cmd/
└── df-orchestrator/
    └── main.go                    # Entry point (MODIFY: add HTTP server startup)

internal/
├── config/
│   ├── config.go                  # MODIFY: add hot-reload support
│   └── watcher.go                 # NEW: file watching for config changes
├── logging/
│   └── logger.go                  # EXISTS: already has structured logging
├── protocol/
│   ├── message.go                 # EXISTS: protocol types
│   ├── codec.go                   # EXISTS: binary serialization
│   └── connection.go              # EXISTS: TCP connection management
├── dfhack/
│   ├── client.go                  # EXISTS: DFHack connection client
│   └── types.go                   # MODIFY: enhance TileData if needed for benchmarks
└── http/                          # NEW: HTTP API server
    ├── server.go                  # NEW: HTTP server setup and lifecycle
    ├── health.go                  # NEW: /health endpoint handler
    ├── ready.go                   # NEW: /ready endpoint handler
    └── metrics.go                 # NEW: /metrics endpoint handler

tests/
├── benchmark/                     # NEW: performance benchmarks
│   ├── tile_memory_test.go        # NEW: tile storage memory benchmarks
│   └── tile_access_test.go        # NEW: tile access speed benchmarks
└── integration/                   # EXISTS: integration tests
    └── http_endpoints_test.go     # NEW: HTTP API integration tests

config/
└── orchestrator.yaml              # MODIFY: add http_port, enable_http_api fields
```

**Structure Decision**: Single project structure maintained. HTTP server added as new `internal/http` package. Benchmark tests follow Go convention in `/tests/benchmark`. No web frontend needed (monitoring endpoints only).

## Complexity Tracking

> **Note**: No Constitution violations detected. This section left empty as no complexity justifications needed.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |
