# Implementation Plan: Topology Overlay (First Spatial Layer)

**Branch**: `003-topology-graph-layer` | **Date**: 2025-11-06 | **Spec**: [spec.md](./spec.md)

## Summary

Build the first XYZ overlay by extracting open/closed state from tile data and storing in a bit-packed array (~870 KB for 6.9M tiles). Implement RLE compression with configurable modes (full map, Z-level filtering, custom bounds) to reduce to <125 KB for LLM context. Enable concurrent spatial queries for pathfinding and placement decisions.

**Primary Requirement**: AI needs queryable spatial data ("Is tile open?") without parsing full 62 MB tile dataset. Compressed representation must fit in LLM context budget alongside other overlays.

**Technical Approach**: Bit-packed byte array organized by Z-level (1 bit per tile), sync.RWMutex for concurrent reads, RLE compression with configurable filtering modes, benchmark tests to validate <1 μs queries and <10ms compression.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**:
- Go stdlib only (math/bits for bit manipulation, sync for RWMutex)
- No external dependencies needed (RLE is simple algorithm)
**Storage**: In-memory bit array (~870 KB), no disk persistence
**Testing**: Go testing framework (`go test`, `go test -bench`) with race detector
**Target Platform**: Windows/Linux server (cross-platform Go binary)
**Project Type**: Single project (server daemon)
**Performance Goals**:
- Storage: 1 bit per tile = ~870 KB for 6.9M tiles
- Query: <1 μs per coordinate lookup
- Region query: <100 μs for 1000 tiles
- Compression: <10ms for full map
- Compressed size: ≤125 KB (full mode), ~5 KB (filtered mode)
**Constraints**:
- Must support concurrent reads (pathfinding, safety checks, room detection all query simultaneously)
- Compression must be lossless (round-trip validation required)
- Z-level filtering must clamp to valid range (no negative Z-levels)
- Memory overhead <1 MB (keeps total overhead reasonable given 6.9M tile scale)
**Scale/Scope**:
- Support maps up to 256x256x256 (16.7M tiles max)
- Current typical: 192x192x189 (6.9M tiles)
- 3 compression modes (full, filtered, custom)
- ~500 lines of code (bit array + RLE + config integration)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Principle I: Research-First Development ✅

**Requirement**: Experimental project, prioritize learning and iteration

**How this feature supports it**:
- First overlay in multi-layer architecture - testing spatial reasoning hypothesis
- Compression modes allow experimenting with context budget tradeoffs
- Logging compression ratios to understand spatial patterns in real forts
- Benchmark-driven validation of memory and performance assumptions

**Status**: PASS - Enables spatial reasoning experiments

---

### Principle II: Comprehensive Logging & Observability ✅

**Requirement**: Log everything, track metrics for analysis

**How this feature supports it**:
- Log overlay build time and memory usage
- Log compression ratio for each mode (full vs filtered)
- Track query counts and latencies
- Log when compression exceeds 125KB budget (learn which forts are problematic)

**Status**: PASS - Provides observability into spatial data patterns

---

### Principle IV: Lean & Efficient (Local LLM Compatible) ✅

**Requirement**: ≤200KB context budget, memory efficient, local LLM compatible

**How this feature supports it**:
- Bit-packed storage: 1 bit per tile = 6.4x better than initial target
- RLE compression: 870 KB → <125 KB (enables fitting in LLM context)
- Z-level filtering: Can reduce to ~5 KB when focused on active area
- No external dependencies (stdlib only)
- Leaves 75 KB budget for other overlays (hazards, traffic, semantics)

**Status**: PASS - Critical for context budget management

---

### Principle VII: Simplicity & Clarity ✅

**Requirement**: Simple, readable code. Avoid abstractions until needed 3+ places.

**How this feature supports it**:
- Bit array is straightforward: `[]byte` with bit manipulation helpers
- RLE is simple algorithm: count consecutive bits, emit (count, value) pairs
- No premature abstraction: direct implementation of storage + compression
- Clear separation: TopologyOverlay (storage), CompressedTopology (RLE output)

**Status**: PASS - Minimal, clear implementation

---

**Constitution Check Result**: ✅ ALL PRINCIPLES ALIGNED

No violations. This feature is the first step in the multi-overlay architecture for spatial reasoning.

## Project Structure

### Documentation (this feature)

```text
specs/003-topology-graph-layer/
├── plan.md              # This file
├── research.md          # Phase 0 output (RLE algorithm, bit packing strategy)
├── data-model.md        # Phase 1 output (TopologyOverlay, CompressedTopology, CompressionConfig)
├── quickstart.md        # Phase 1 output (how to query overlay, compress for LLM, configure modes)
├── contracts/           # Phase 1 output
│   └── internal-api.md  # Internal API specification (not REST - internal Go API)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT YET CREATED)
```

### Source Code (repository root)

```text
# Existing structure (from 001-binary-protocol, 002-foundation-infrastructure)
cmd/
└── df-orchestrator/
    └── main.go                    # MODIFY: integrate topology overlay

internal/
├── config/
│   ├── config.go                  # MODIFY: add topology compression config fields
│   └── watcher.go                 # EXISTS: hot-reload support
├── topology/                      # NEW: first overlay package
│   ├── overlay.go                 # NEW: TopologyOverlay struct and bit array storage
│   ├── compression.go             # NEW: RLE compression with mode support
│   ├── query.go                   # NEW: IsOpen, GetOpenTiles region queries
│   └── tiletype.go                # NEW: helper to determine if DF tiletype is pathable
├── protocol/
│   └── message.go                 # EXISTS: TileState struct already defined
├── dfhack/
│   └── client.go                  # EXISTS: receives tile data
└── http/
    └── metrics.go                 # MODIFY: add topology metrics (memory, compression ratio)

tests/
└── benchmark/                     # EXISTS: benchmark suite
    ├── topology_memory_test.go    # NEW: bit array memory benchmarks
    ├── topology_query_test.go     # NEW: query latency benchmarks
    └── topology_compression_test.go # NEW: RLE compression benchmarks

config/
└── orchestrator.yaml              # MODIFY: add topology compression mode config
```

**Structure Decision**: Single project structure maintained. Topology overlay added as new `internal/topology` package (first of future overlay packages: topology, hazards, traffic, semantics). Benchmarks added to existing `tests/benchmark/` directory.

## Complexity Tracking

> **Note**: No Constitution violations detected. This section left empty as no complexity justifications needed.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |
