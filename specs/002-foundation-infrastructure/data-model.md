# Data Model: Foundation Infrastructure

**Feature**: 002-foundation-infrastructure
**Date**: 2025-11-06

## Overview

This document defines the key data structures for configuration management, health monitoring, and performance benchmarking.

---

## Configuration (Enhanced from existing)

### Config

**Purpose**: Holds all runtime configuration for the orchestrator server.

**Fields**:
- `ListenPort` (uint16): TCP port for DFHack protocol connections
- `HttpPort` (uint16): **NEW** - HTTP port for monitoring API
- `EnableHttpApi` (bool): **NEW** - Feature flag to enable/disable HTTP server
- `ReadTimeout` (duration): TCP read timeout
- `WriteTimeout` (duration): TCP write timeout
- `HeartbeatInterval` (duration): How often to send heartbeats
- `HeartbeatTimeout` (duration): When to consider connection dead
- `MaxMessageSize` (uint32): Maximum protocol message size
- `ReconnectDelay` (duration): Initial reconnection delay
- `ReconnectMaxDelay` (duration): Max reconnection delay (exponential backoff cap)
- `ReconnectBackoffFactor` (float64): Backoff multiplier
- `LogLevel` (string): Logging verbosity (debug|info|warn|error)
- `LogFormat` (string): Log output format (json|text)

**Validation Rules**:
- `ListenPort` must be 1024-65535 (non-privileged ports)
- `HttpPort` must be 1024-65535 and different from `ListenPort`
- `LogLevel` must be one of: debug, info, warn, error
- `LogFormat` must be: json or text
- `HeartbeatInterval` must be > 0
- `HeartbeatTimeout` must be >= `HeartbeatInterval`

**Relationships**:
- Used by ConfigManager to track current configuration
- Referenced by HTTP server, DFHack client, logger

**State Transitions**:
- Loaded from file → Validated → Active → (on file change) → Reloaded → Validated → Active

---

### ConfigManager

**Purpose**: Manages configuration lifecycle including hot-reload.

**Fields**:
- `config` (*Config): Current active configuration (protected by RWMutex)
- `mu` (sync.RWMutex): Protects config pointer during reload
- `watcher` (*fsnotify.Watcher): File system watcher for config file changes
- `configPath` (string): Path to config file being watched
- `logger` (*logging.Logger): For logging reload events
- `reloadCh` (chan *Config): Channel for validated config reload

**Methods**:
- `Get() *Config`: Returns current config (read-locked)
- `Watch(ctx context.Context)`: Starts file watcher goroutine
- `reload(newConfig *Config)`: Atomically swaps to new config (write-locked)

**Lifecycle**:
1. Initialize: Load config from file
2. Start watching: Watch() goroutine monitors file
3. On file change: Load → Validate → Send to reloadCh
4. Reload: Atomic swap under write lock
5. Shutdown: Stop watcher, close channels

---

## Health Monitoring

### HealthStatus

**Purpose**: Represents current operational state of the orchestrator.

**Fields**:
- `Status` (string): Overall status - "healthy", "degraded", or "unhealthy"
- `Uptime` (duration): Time since server started
- `DFHackConnected` (bool): Whether DFHack connection is active
- `DFHackConnectionTime` (time.Time): When connection was established (zero if not connected)
- `LastHeartbeat` (time.Time): Last heartbeat received from DFHack
- `ConfigLoaded` (bool): Whether configuration successfully loaded
- `HttpApiEnabled` (bool): Whether HTTP API is running
- `Timestamp` (time.Time): When this status was captured

**Status Determination Rules**:
- "healthy": Server running, config loaded, DFHack connected, heartbeat recent (<30s)
- "degraded": Server running, config loaded, but DFHack not connected OR heartbeat stale
- "unhealthy": Configuration load failed or critical error

**JSON Representation** (for /health endpoint):
```json
{
  "status": "healthy",
  "uptime_seconds": 3600,
  "dfhack_connected": true,
  "dfhack_connection_time": "2025-11-06T10:30:00Z",
  "last_heartbeat": "2025-11-06T11:29:55Z",
  "timestamp": "2025-11-06T11:30:00Z"
}
```

---

### ReadinessStatus

**Purpose**: Indicates whether server is ready to accept requests.

**Fields**:
- `Ready` (bool): True if server is fully initialized
- `Checks` (map[string]bool): Individual readiness checks
- `Message` (string): Human-readable reason if not ready

**Readiness Criteria**:
- Config loaded successfully
- TCP listener started (DFHack protocol)
- HTTP server started (if enabled)
- Logger initialized

**JSON Representation** (for /ready endpoint):
```json
{
  "ready": true,
  "checks": {
    "config_loaded": true,
    "tcp_listener": true,
    "http_server": true,
    "logger": true
  }
}
```

---

### MetricsSnapshot

**Purpose**: Aggregated operational metrics for /metrics endpoint.

**Fields**:
- `ServerUptime` (duration): Total server uptime
- `DFHackConnected` (bool): Current connection status
- `SessionMetrics` (SessionMetrics): Current session stats (from dfhack.Client)
  - `ConnectionTime` (time.Time)
  - `MessagesSent` (uint64)
  - `MessagesReceived` (uint64)
  - `ErrorCount` (uint64)
  - `LastError` (error)
  - `LastErrorTime` (time.Time)
  - `HeartbeatsSent` (uint64)
  - `HeartbeatsReceived` (uint64)
- `ConfigReloads` (uint64): Count of successful config reloads
- `HttpRequests` (uint64): Total HTTP requests received
- `Timestamp` (time.Time): When metrics were captured

**JSON Representation** (for /metrics endpoint):
```json
{
  "server_uptime_seconds": 3600,
  "dfhack_connected": true,
  "session": {
    "connection_time": "2025-11-06T10:30:00Z",
    "messages_sent": 150,
    "messages_received": 148,
    "error_count": 2,
    "heartbeats_sent": 360,
    "heartbeats_received": 360
  },
  "config_reloads": 3,
  "http_requests": 1250,
  "timestamp": "2025-11-06T11:30:00Z"
}
```

---

## Performance Benchmarking

### BenchmarkResult

**Purpose**: Captures output from performance benchmark runs.

**Fields**:
- `Name` (string): Benchmark name (e.g., "TileMemory/Medium_100x100x100")
- `Iterations` (int): Number of iterations run
- `NsPerOp` (float64): Nanoseconds per operation
- `AllocsPerOp` (uint64): Memory allocations per operation
- `BytesPerOp` (uint64): Bytes allocated per operation
- `TotalBytes` (uint64): Total memory used
- `Passed` (bool): Whether benchmark passed thresholds

**Threshold Validation**:
- Tile storage: `BytesPerOp` ≤ 64 bytes per tile
- Tile access: `NsPerOp` ≤ 500 nanoseconds

**Output Format** (from `go test -bench`):
```
BenchmarkTileMemory/Small_48x48x20-8              1000    1250000 ns/op    45 B/op    1 allocs/op
BenchmarkTileMemory/Medium_100x100x100-8           100   11500000 ns/op    58 B/op    1 allocs/op
BenchmarkTileAccess/Medium_100x100x100-8      5000000         310 ns/op     0 B/op    0 allocs/op
```

---

## Relationships Between Entities

```
ConfigManager
  │
  ├─ owns ──> Config (current)
  │
  └─ watches ──> config/orchestrator.yaml (file)

HTTP Server
  │
  ├─ GET /health ──> HealthStatus
  │                    │
  │                    └─ queries ──> DFHack Client (connection state)
  │
  ├─ GET /ready ──> ReadinessStatus
  │
  └─ GET /metrics ──> MetricsSnapshot
                       │
                       └─ aggregates ──> SessionMetrics (from DFHack Client)

Benchmark Suite
  │
  └─ produces ──> BenchmarkResult[] (one per test case)
```

---

## Data Validation Summary

| Entity | Validation Trigger | Validation Rules |
|--------|-------------------|------------------|
| Config | On load, on reload | Port ranges, enum values, timeout ordering |
| HealthStatus | On query | Timestamp freshness, status enum validity |
| ReadinessStatus | On query | All checks must be boolean, ready derived correctly |
| MetricsSnapshot | On query | Counters must be monotonic (never decrease) |
| BenchmarkResult | On test completion | Threshold validation (bytes/op, ns/op) |
