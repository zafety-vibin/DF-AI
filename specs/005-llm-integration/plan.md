# Implementation Plan: LLM Integration with Modification Tracking and Command Execution

**Branch**: `005-llm-integration` | **Date**: 2025-11-07 | **Spec**: [spec.md](./spec.md)

## Summary

Enable AI to observe fort through modification tracking overlay and take autonomous actions via bidirectional command protocol. Modification overlay tracks only player/AI tile changes (not 6.9M natural tiles), providing compact fort signature. Context assembly uses viewport system (4 detail levels: 500B → 10KB → 50KB → 200KB) optimized for LLM token efficiency. AI sends commands to DFHack (DIG/BUILD/CANCEL), receives acknowledgments and feedback, learns from outcomes including hazard interactions. Support Claude API and local LLMs with modular provider interface.

**Primary Requirement**: AI autonomously manages fort by observing modifications, reasoning about state, and issuing commands while learning from experience.

**Technical Approach**: Sparse modification overlay (map[Coordinate]ModificationInfo), flood-fill chamber extraction, staged viewport assembly, HTTP LLM client (Claude/OpenAI-compatible), bidirectional protocol messages (COMMAND + COMMAND_ACK), command ID lineage tracking modification → AI decision.

## Technical Context

**Language/Version**: Go 1.21+ (server), C++17 (DFHack plugin for command execution)
**Primary Dependencies**: Go stdlib + net/http (LLM HTTP client), existing protocol/hazards/topology packages
**Storage**: In-memory sparse maps (modification overlay <1 MB), no persistence (research experiment)
**Testing**: Go testing framework, live DFHack integration testing
**Target Platform**: Windows/Linux server
**Project Type**: Single project (server + plugin components)
**Performance Goals**: Context assembly <5s for Level 3, LLM response <30s, command ack <1s, autonomous loop 100s cycle time
**Constraints**: 200 KB context budget, 1 MB modification overlay memory, 30s LLM timeout, no safety blocks (AI learns from failures)
**Scale/Scope**: 6 user stories, ~1500 LOC (modifications ~200, context ~300, LLM client ~200, commands ~300, loop ~200, config ~100, protocol ~200)

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

✅ **Aligned with all constitution principles**:
- ✅ Research-First: Modification overlay + LLM feedback loop enables hypothesis testing (does AI learn from hazard failures?)
- ✅ Comprehensive Logging: All LLM interactions logged in fine-tuning format (context, response, tokens, outcomes)
- ✅ Modularity: LLM client interface allows swapping providers (Claude ↔ local models), context assembly modular by detail level
- ✅ Lean & Efficient: Viewport system enforces 200 KB budget, modification overlay sparse (only changes, not 6.9M tiles)
- ✅ Iterative: 6 user stories enable incremental delivery (US1 modification tracking → US2 context → US3 first LLM call → US4 commands → US5 loop → US6 config)
- ✅ Metrics-Driven: Success measured by AI decision quality, hazard learning, context efficiency (90% reduction target)
- ✅ Simplicity: No hard-coded safety rules, no complex pathfinding, flood-fill is straightforward, HTTP client is stdlib

**No violations** - feature designed for research experimentation with clear measurement.

## Project Structure

### Documentation (this feature)

```text
specs/005-llm-integration/
├── plan.md              # This file
├── research.md          # Phase 0 research decisions
├── data-model.md        # Phase 1 entity design
├── quickstart.md        # Phase 1 integration guide
├── contracts/           # Phase 1 API contracts
│   ├── internal-api.md      # Go internal packages
│   ├── protocol.md          # Binary protocol messages
│   └── llm-provider.md      # LLM client interface
└── tasks.md             # Phase 2 (/speckit.tasks - not created by plan)
```

### Source Code (repository root)

```text
internal/modifications/
├── overlay.go           # ModificationOverlay sparse map
├── detector.go          # Detect modifications from TILE_UPDATE
├── chambers.go          # Flood-fill chamber extraction
└── baseline.go          # Natural tile baseline detection

internal/context/
├── assembly.go          # Viewport-based context assembly
├── levels.go            # Level 0-3 detail formatters
├── queries.go           # Query API (topology slice, hazard filter, etc.)
└── features.go          # Chamber feature extraction to natural language

internal/llm/
├── client.go            # LLM client interface
├── claude.go            # Claude API provider implementation
├── openai.go            # OpenAI-compatible provider (local models)
├── parser.go            # Parse LLM responses for commands
└── logger.go            # JSONL logging for fine-tuning dataset

internal/commands/
├── protocol.go          # COMMAND and COMMAND_ACK message types
├── executor.go          # Send commands to DFHack, track acknowledgments
├── tracker.go           # Map command ID → modification changes
└── feedback.go          # Generate feedback descriptions for AI

internal/autonomous/
├── loop.go              # Main decision loop (observe → LLM → command → feedback)
├── timer.go             # 100s cycle timer with immediate update trigger
└── state.go             # Track conversation history and command lineage

cmd/df-orchestrator/
└── main.go              # Wire up modifications, context, LLM, commands, loop

dfhack-plugin/
├── df_ai_protocol.cpp   # Add COMMAND message handlers
└── designations.cpp     # NEW: Execute dig/build/cancel via df.designation API

config/
└── orchestrator.yaml    # Add LLM provider config (API key, model, endpoint, etc.)

tests/
├── modifications/       # Modification overlay tests
├── context/             # Context assembly and viewport tests
├── llm/                 # LLM client and parser tests
└── integration/         # End-to-end: modification → context → LLM → command → ack
```

**Structure Decision**: New packages for modifications (overlay + chamber extraction), context (viewport assembly + queries), llm (provider clients + parser), commands (bidirectional protocol + tracking), and autonomous (execution loop). Plugin extended with designation execution. Estimated ~1500 LOC total across 20+ files.

## Complexity Tracking

No violations - all constitution principles aligned.
