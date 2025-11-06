# Implementation Plan: DFHack Binary Protocol

**Branch**: `001-binary-protocol` | **Date**: 2025-11-04 | **Spec**: [spec.md](spec.md)
**Input**: Feature specification from `/specs/001-binary-protocol/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Establish bidirectional binary communication protocol between DFHack C++ plugin (running in Dwarf Fortress) and Go orchestrator server. Primary requirement is efficient transmission of fort tile data (up to 1M tiles) with sub-2-second full state sync and sub-1-second incremental updates. Protocol must support automatic reconnection, comprehensive logging, and graceful failure handling to enable reliable AI fort management research.

## Technical Context

**Language/Version**: Go 1.21+ (server), C++17 (DFHack plugin)
**Primary Dependencies**:
- Go: stdlib only (net, encoding/binary, log/slog)
- DFHack: 53.02-r1 SDK for DF 53.02 (MapCache, SimpleSockets/clsocket, df::coord, df::map_block)
- Build: CMake 3.10+, MSVC 2022 (Windows) or GCC 7+ (Linux)
**Storage**: In-memory state only (no persistence at protocol layer)
**Testing**: Go testing package, mock DFHack client for integration tests
**Target Platform**: Windows 10+ (primary), Linux (secondary) - localhost communication only
**Project Type**: Dual codebase - C++ plugin + Go server (separate compilation)
**Performance Goals**:
- Full state transfer: 1M tiles in <2 seconds (<500KB/s minimum)
- Incremental updates: <1 second latency from DF event to Go receipt
- Message overhead: <5% CPU impact on DF gameplay
**Constraints**:
- Binary protocol must be platform-endianness aware
- Zero external dependencies on Go side (stdlib only per constitution)
- Must handle DFHack restart without Go server restart
- Logging cannot be disabled (constitution requirement)
**Scale/Scope**:
- Support maps up to 200x200x100 (4M tiles)
- Handle up to 1000 tile changes per second during heavy gameplay
- Protocol version negotiation for future extensibility

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Research-First Development
- ✅ **PASS**: This is foundation infrastructure - necessary for any AI experiments
- ✅ Feature enables hypothesis testing: "Can binary protocol achieve <2s sync for 1M tiles?"
- ✅ Easy rollback: protocol is isolated, can swap implementations without affecting other layers

### II. Comprehensive Logging & Observability
- ✅ **PASS**: FR-008 requires all protocol events logged
- ✅ Structured logging planned (JSON format)
- ✅ Performance metrics tracked: message size, transmission time, throughput
- ✅ All connection events, errors, and state changes logged

### III. Modularity & Pluggability
- ✅ **PASS**: Protocol layer isolated from graph layers and LLM client
- ⚠️  **WARN**: C++ plugin tightly coupled to DFHack SDK (unavoidable)
- ✅ Go side uses interfaces - can mock DFHack for testing
- ✅ Could swap TCP for other transports without changing protocol semantics

### IV. Lean & Efficient (Local LLM Compatible)
- ✅ **PASS**: Zero external Go dependencies (stdlib only)
- ✅ Binary format minimizes data size vs JSON/XML
- ✅ Performance targets align: <2s for 1M tiles = <500KB/s (achievable on localhost)
- ✅ No heavyweight frameworks

### V. Iterative Experimentation
- ✅ **PASS**: This is Phase 1 (Foundation) per constitution
- ✅ MVP is P1 (Initial State Sync) - can test immediately
- ✅ P2-P4 add incrementally: updates, reconnection, enhanced logging
- ✅ Could A/B test: binary vs JSON, TCP vs HTTP

### VI. Metrics-Driven Evaluation
- ✅ **PASS**: Success criteria defined (SC-001 through SC-008)
- ✅ Measurable: sync time, latency, FPS impact, crash rate
- ✅ Benchmarks planned for 1M and 4M tile maps
- ✅ Metrics feed into future RL reward signals (protocol reliability = positive reward)

### VII. Simplicity & Clarity
- ✅ **PASS**: Binary protocol is well-understood pattern
- ✅ No clever optimizations planned initially - simple length-prefixed messages
- ✅ Avoid premature compression/batching - add only if benchmarks show need
- ✅ Clear contracts defined in Phase 1

**Gate Result**: ✅ **PASSED** - No violations, proceed to Phase 0 research

## Project Structure

### Documentation (this feature)

```text
specs/[###-feature]/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
# Go Server
cmd/
└── df-orchestrator/
    └── main.go              # Server entry point

internal/
├── protocol/
│   ├── connection.go        # Connection management, state machine
│   ├── message.go           # Message types and serialization
│   ├── codec.go             # Binary encoding/decoding
│   └── version.go           # Protocol versioning
├── dfhack/
│   ├── client.go            # DFHack client interface
│   ├── mock_client.go       # Mock for testing
│   └── types.go             # Shared tile/entity types
└── logging/
    └── logger.go            # Structured JSON logger

tests/
├── protocol/
│   ├── connection_test.go
│   ├── codec_test.go
│   └── integration_test.go  # End-to-end with mock DFHack
└── testdata/
    └── fixtures/            # Sample tile data for tests

# C++ DFHack Plugin
dfhack-plugin/
├── CMakeLists.txt           # Build configuration
├── df_ai_protocol.cpp       # Main plugin logic
├── protocol.h               # C++ protocol implementation
├── tile_extractor.cpp       # DF tile data extraction
└── README.md                # Build and install instructions

# Configuration
config/
└── orchestrator.yaml        # Server config (port, log level, etc.)
```

**Structure Decision**: Dual codebase architecture required - Go for orchestrator (aligns with project goals), C++ for DFHack plugin (required by DFHack SDK). Go code follows standard layout (cmd/, internal/). C++ plugin is separate directory with own build system (CMake per DFHack conventions).

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations - Constitution Check passed cleanly. No complexity justifications required.

---

## Constitution Check Re-evaluation (Post Phase 1)

*Re-checked after Phase 1 design completion*

### Design Review Against Principles

**I. Research-First Development** - ✅ STILL PASSING
- Binary protocol enables testable hypothesis: "Can we transmit 1M tiles in <2s?"
- Simple enough to implement quickly and measure
- Can easily A/B test: binary vs JSON, TCP vs other transports

**II. Comprehensive Logging & Observability** - ✅ STILL PASSING
- data-model.md defines structured JSON logging schema
- All protocol events logged (connections, messages, errors)
- Metrics exposed for monitoring (message sizes, latency, throughput)
- go-server-interface.md specifies Prometheus metrics

**III. Modularity & Pluggability** - ✅ STILL PASSING
- Clean interface separation (Connection, Message, Client interfaces)
- Mock implementations defined for testing
- Could swap out SimpleSockets for different transport
- Protocol versioning enables future extensions

**IV. Lean & Efficient (Local LLM Compatible)** - ✅ STILL PASSING
- Go stdlib only (net, encoding/binary, log/slog)
- Binary format minimizes overhead vs JSON
- Size estimates calculated: 100K tiles = 880KB (well within budget)
- No heavyweight dependencies added

**V. Iterative Experimentation** - ✅ STILL PASSING
- MVP clearly defined (P1: Initial State Sync)
- P2-P4 add incrementally (updates, reconnection, logging)
- Polling approach chosen for simplicity (can optimize later if needed)
- Version 1.0 protocol with extensibility for v2

**VI. Metrics-Driven Evaluation** - ✅ STILL PASSING
- Performance benchmarks defined in quickstart.md
- Success criteria still measurable (SC-001 through SC-008)
- Logging enables post-analysis of all protocol events
- Can track metrics over time for improvements

**VII. Simplicity & Clarity** - ✅ STILL PASSING
- Length-prefixed messages (simple, well-understood pattern)
- No compression in v1 (can add later if benchmarks show need)
- Clear contracts and interfaces documented
- Polling for tile changes (simpler than event hooks)

**Re-evaluation Result**: ✅ **STILL PASSING** - Design maintains all constitutional principles

---

## Phase Summary

### Phase 0: Research ✅ Complete
**Output**: [research.md](research.md)

**Key Findings**:
- DFHack 53.02-r1 SDK for DF 53.02 researched and documented
- SimpleSockets library identified for network communication
- Polling approach chosen over event hooks (simplicity)
- Plugin-as-client architecture decided (Go server, C++ client)

**Resolved**:
- DFHack version: 53.02-r1 for latest DF 53.02 (was NEEDS CLARIFICATION)
- Build requirements (CMake 3.10+, MSVC 2022/GCC 7+)
- Network communication patterns
- Tile access methods (MapCache with block iteration)

---

### Phase 1: Design ✅ Complete

**Outputs**:
- [data-model.md](data-model.md) - Binary message formats and protocol structures
- [contracts/protocol-spec.md](contracts/protocol-spec.md) - Detailed protocol specification with examples
- [contracts/go-server-interface.md](contracts/go-server-interface.md) - Go implementation interfaces
- [quickstart.md](quickstart.md) - Build and test instructions

**Key Decisions**:
- 7 message types defined (HANDSHAKE through DISCONNECT)
- Big-endian encoding for cross-platform compatibility
- Length-prefixed messages (4-byte header)
- 9-byte tile state structure (coordinates, type, flags)
- Heartbeat every 10s, timeout after 5s
- Exponential backoff reconnection (1s → 30s max)

**Size Calculations**:
- 100×100×10 map: ~880KB full state
- 200×200×100 map: ~34MB full state
- Tile updates: ~916 bytes per 100 tiles

---

## Next Steps

### For Implementation (`/speckit.tasks`)

When ready to implement, run `/speckit.tasks` to generate task breakdown based on:
- User stories from spec.md (P1-P4 priorities)
- Technical design from data-model.md
- Contracts from contracts/
- Project structure from this plan

**Estimated Implementation Order**:
1. **Foundation**: Go protocol message types and serialization
2. **P1 (MVP)**: Connection management, HANDSHAKE, FULL_STATE
3. **C++ Plugin**: DFHack integration, tile extraction, TCP client
4. **P2**: TILE_UPDATE messages and incremental updates
5. **P3**: Reconnection logic and RESYNC_REQUEST
6. **P4**: Enhanced logging and metrics
7. **Testing**: Integration tests with mock plugin
8. **Documentation**: Update quickstart with real build steps

---

## Artifacts Generated

| File | Purpose | Status |
|------|---------|--------|
| plan.md | This file - implementation plan | ✅ Complete |
| research.md | DFHack research findings | ✅ Complete |
| data-model.md | Binary protocol data structures | ✅ Complete |
| contracts/protocol-spec.md | Protocol message specifications | ✅ Complete |
| contracts/go-server-interface.md | Go implementation contracts | ✅ Complete |
| quickstart.md | Build and test guide | ✅ Complete |

**Ready for**: Task generation (`/speckit.tasks`) and implementation

---

## References

- [Feature Specification](spec.md)
- [Project Constitution](../../.specify/memory/constitution.md)
- [Feature Breakdown](../../assets/docs/feature-breakdown.md)
- [DFHack Documentation](https://docs.dfhack.org/)
- [Go stdlib Documentation](https://pkg.go.dev/std)
