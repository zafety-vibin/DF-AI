# Research: Foundation Infrastructure

**Feature**: 002-foundation-infrastructure
**Date**: 2025-11-06

## Research Questions

This document resolves technical unknowns identified during planning.

---

## R1: Configuration Hot-Reload Strategy

**Question**: How should config hot-reload be implemented in Go without service downtime?

**Decision**: File watching with atomic config swap using `fsnotify` library and sync.RWMutex

**Rationale**:
- `fsnotify` is the standard Go library for cross-platform file watching (Linux inotify, Windows ReadDirectoryChangesW, macOS FSEvents)
- Atomic swap pattern: watch file → detect change → load new config → validate → swap pointer under write lock
- RWMutex allows concurrent reads of current config while reload happens
- Minimal overhead: file watcher runs in single goroutine, only triggers on actual file changes

**Alternatives considered**:
1. **Signal-based reload (SIGHUP)**: Rejected because Windows doesn't support Unix signals reliably
2. **HTTP endpoint for reload**: Rejected to keep HTTP API read-only (monitoring only, not control plane)
3. **Polling**: Rejected due to unnecessary overhead (checking file mtime every N seconds vs event-driven)

**Implementation pattern**:
```go
type ConfigManager struct {
    mu     sync.RWMutex
    config *Config
    watcher *fsnotify.Watcher
}

func (cm *ConfigManager) Get() *Config {
    cm.mu.RLock()
    defer cm.mu.RUnlock()
    return cm.config
}

func (cm *ConfigManager) reload(newConfig *Config) {
    cm.mu.Lock()
    defer cm.mu.Unlock()
    cm.config = newConfig
}
```

---

## R2: HTTP Server Patterns for Health Endpoints

**Question**: What's the simplest pattern for health/ready/metrics endpoints in Go?

**Decision**: Go stdlib `net/http` with individual HandlerFunc registrations

**Rationale**:
- stdlib `net/http` is sufficient for 3 simple endpoints (no routing complexity)
- No framework overhead (Gin, Echo, etc. add unnecessary dependencies for monitoring endpoints)
- Standard Go idiom: `http.HandleFunc("/health", healthHandler)`
- Graceful shutdown via `http.Server.Shutdown(context)` for clean lifecycle management

**Alternatives considered**:
1. **HTTP framework (Gin/Echo)**: Rejected - adds 5+MB to binary for features we don't need (routing, middleware, templating)
2. **Separate port for each endpoint**: Rejected - wasteful, monitoring systems expect single port
3. **gRPC health check protocol**: Rejected - requires protobuf dependency, overkill for simple JSON endpoints

**Endpoint patterns**:
- `/health`: Always returns 200 if server is running, includes status: "healthy"/"degraded" based on DFHack connection
- `/ready`: Returns 200 only when server is fully initialized (config loaded, listeners started)
- `/metrics`: Returns JSON with current operational metrics (Prometheus exposition format out of scope for now)

---

## R3: Benchmark Test Structure

**Question**: How should Go benchmarks be structured for tile data structures?

**Decision**: Use standard `testing.B` framework with table-driven tests for different map sizes

**Rationale**:
- Go's built-in `testing.B` provides accurate benchmarking with automatic iteration counting
- Table-driven approach lets us test 3 map sizes: small (48x48x20), medium (100x100x100), large (200x200x100)
- `b.ReportAllocs()` tracks memory allocations automatically
- `b.ResetTimer()` excludes setup time from measurements

**Alternatives considered**:
1. **External benchmark tool (criterion-like)**: Rejected - Go's testing package is sufficient and idiomatic
2. **Manual timing with time.Now()**: Rejected - less accurate, doesn't handle iteration optimization
3. **Profiling instead of benchmarks**: Rejected - both are useful, benchmarks are easier to automate in CI

**Benchmark test pattern**:
```go
func BenchmarkTileMemory(b *testing.B) {
    testCases := []struct{
        name string
        width, height, depth uint16
    }{
        {"Small_48x48x20", 48, 48, 20},
        {"Medium_100x100x100", 100, 100, 100},
        {"Large_200x200x100", 200, 200, 100},
    }

    for _, tc := range testCases {
        b.Run(tc.name, func(b *testing.B) {
            b.ReportAllocs()
            // ... benchmark logic
        })
    }
}
```

---

## R4: Graceful Shutdown Coordination

**Question**: How to coordinate graceful shutdown between TCP and HTTP servers?

**Decision**: errgroup.Group with context cancellation for coordinated shutdown

**Rationale**:
- `golang.org/x/sync/errgroup` provides clean pattern for managing multiple goroutines
- Single context cancellation triggers shutdown for both servers
- `http.Server.Shutdown(ctx)` waits for in-flight requests to complete
- DFHack client `Stop()` already handles TCP cleanup

**Alternatives considered**:
1. **Manual sync.WaitGroup**: Rejected - errgroup handles error propagation better
2. **OS signal handling only**: Rejected - need programmatic shutdown for tests
3. **No graceful shutdown**: Rejected - violates functional requirement FR-013

**Shutdown sequence**:
1. Catch SIGINT/SIGTERM
2. Cancel context → triggers both servers to stop accepting new connections
3. Wait for in-flight HTTP requests (up to 30s timeout)
4. Close DFHack TCP connection cleanly
5. Exit process

---

## R5: Metrics Collection Strategy

**Question**: How should metrics be collected and exposed?

**Decision**: In-memory counters with mutex protection, JSON format on `/metrics` endpoint

**Rationale**:
- Simple atomic counters (uint64) with mutex for complex metrics like SessionMetrics
- JSON format is human-readable and easy to parse (Prometheus exposition format is future work)
- Metrics collected where events occur (e.g., dfhack.Client tracks message counts)
- HTTP endpoint aggregates metrics from various components

**Alternatives considered**:
1. **Prometheus client library**: Rejected - adds dependency, can add later if needed
2. **Metrics in log stream only**: Rejected - logs are for events, metrics need real-time query
3. **StatsD/external metrics service**: Rejected - adds deployment complexity

**Metrics to expose**:
- Server uptime
- DFHack connection status (connected/disconnected)
- Message counts (sent/received) from SessionMetrics
- Error counts
- Heartbeat stats (sent/received)

---

## Summary

All technical unknowns resolved. Key decisions:
- **Config reload**: fsnotify + RWMutex for zero-downtime updates
- **HTTP server**: stdlib net/http with 3 HandlerFunc endpoints
- **Benchmarks**: table-driven testing.B tests with ReportAllocs
- **Shutdown**: errgroup.Group with context cancellation
- **Metrics**: in-memory counters + JSON exposure

Ready to proceed to Phase 1 (Design & Contracts).
