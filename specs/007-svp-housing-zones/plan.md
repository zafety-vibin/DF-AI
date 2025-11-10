# Implementation Plan: Spatial Validator Planner and Housing Zones

**Branch**: `007-svp-housing-zones` | **Date**: 2025-11-09 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/007-svp-housing-zones/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command. See `.specify/templates/commands/plan.md` for the execution workflow.

## Summary

Feature 007 adds strategic spatial planning to DF-AI through the Spatial Validator Planner (SVP) component, which analyzes embark terrain to establish organized vertical fort structure (housing at Z-5 below embark, workshops below housing, farms at soil layers). Enhances HousingAgent to use real bedroom zone counts extracted from DF instead of placeholder data, enabling accurate housing deficit detection. Integrates blueprint system into arbiter decision-making with zone CSV companion files (blueprint.csv for dig patterns, blueprint_zones.csv for zone designations), allowing AI to select proven human designs. Implements zone command execution with async queue for handling multi-turn dig completion, ensuring proposed bedrooms become assignable DF zones.

## Technical Context

**Language/Version**: Go 1.21+ (orchestrator), C++17 (DFHack plugin)
**Primary Dependencies**: Existing DF-AI packages (agents, topology, hazards, modifications, protocol), DF building module API for zone extraction
**Storage**: JSON persistence files (saves/{fortname}/svp_layout.json for SVP designations), in-memory zone queue (optional persistence for restart resilience)
**Testing**: go test for SVP logic, zone extraction unit tests, integration tests with mock DF zone data
**Target Platform**: Windows/Linux (matches DF platform support)
**Project Type**: Single codebase with plugin component (Go orchestrator + C++ DFHack plugin)
**Performance Goals**: SVP analysis <200ms one-time cost, zone extraction <50ms per entity update, zone queue processing <10ms per autonomous cycle
**Constraints**: Zone CSV parsing must handle community quickfort format, SVP must integrate with existing hazard manager, blueprint metadata must fit in arbiter system prompt (<500 tokens)
**Scale/Scope**: 200 Z-levels max (DF limit), ~50-500 zones per fort, 5-10 blueprints in library, zone queue holds max 50 pending zones

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### I. Research-First Development ✅

**Hypothesis**: Feature 006 agents are non-functional due to placeholder metrics. SVP + zone extraction will provide real data enabling HousingAgent to make accurate decisions. Blueprint integration will improve fort quality by encoding human expertise.

**Experiment**: Compare HousingAgent behavior before (placeholder bedroom_count=0) vs after (real zone extraction). Measure: proposal accuracy, bedroom deficit resolution time, fort organization quality.

**Rollback Plan**: SVP and zone extraction are additive layers. Can disable via config (use_svp: false) to revert to Feature 006 behavior. Zone queue is optional (immediate ZONE execution works for pre-dug spaces).

### II. Comprehensive Logging & Observability ✅

**Logging Coverage**:
- SVP analysis: terrain detection, Z-level designations, hazard conflicts, save/load operations (DEBUG)
- Zone extraction: DF API calls, zone counts by type, extraction failures (INFO)
- HousingAgent: metric values, deficit calculations, SVP queries, proposal decisions (INFO)
- Arbiter: blueprint selection reasoning, available blueprints, adaptation decisions (INFO)
- Zone queue: ZONE commands queued, retry attempts, completion/failure status (DEBUG)
- Performance: SVP analysis timing, zone extraction latency, queue processing time (INFO)

**LLM Interactions**: Arbiter receives blueprint metadata in system prompt, logs blueprint selection in response.

**Metrics**: bedroom_zone_count, unassigned_dwarf_count, housing_deficit, svp_designation_count, zone_queue_length, zone_success_rate.

### III. Modularity & Pluggability ✅

**Component Boundaries**:
- SVP: Independent spatial planning module with clear interface (AnalyzeTerrain, GetHousingZ, ValidateProposal)
- ZoneExtractor: Standalone component querying DF API, produces ZoneInfo structs
- ZoneQueue: Separate async execution manager, not tightly coupled to executor
- Blueprint integration: Arbiter system prompt enhancement, doesn't modify existing arbiter logic

**Configuration**: SVP toggleable (use_svp: true/false), zone extraction optional (fallback to chamber count), blueprint library path configurable.

**Testing**: SVP testable with mock topology data, zone extraction with mock DF zones, queue with simulated async timing.

### IV. Lean & Efficient (Local LLM Compatible) ✅

**Context Budget**:
- SVP adds ~200 bytes to metrics (housing_z, workshop_z, farm_z values)
- Zone counts add ~100 bytes (7 zone types × ~15 bytes per metric)
- Blueprint metadata in arbiter prompt: ~400 tokens (5 blueprints × 80 tokens: name, dimensions, description)
- Total context increase: <1KB for metrics, <500 tokens for arbiter

**Memory Efficiency**:
- SVP layout: 50 bytes (embark_z, housing_z, workshop_z, 3 farm z-levels, hazards)
- Zone queue: 100 bytes per queued zone × max 50 = 5KB
- No per-tile overhead (soil detection uses Z-level bool array: 200 bytes)

**LLM Calls**: No additional LLM calls. Blueprint metadata enhances existing arbiter prompt.

### V. Iterative Experimentation ✅

**Phased Implementation**:
- Phase 1: SVP terrain analysis (foundation for spatial planning)
- Phase 2: Zone extraction (real data for HousingAgent)
- Phase 3: Blueprint integration (improve fort design quality)
- Phase 4: Zone queue (handle async execution)

**Measurement Points**:
- After Phase 1: SVP designation accuracy (correct housing Z?)
- After Phase 2: HousingAgent proposal accuracy (triggers when deficit exists, silent when met?)
- After Phase 3: Blueprint usage rate (arbiter selects blueprints vs arbitrary rects?)
- After Phase 4: Zone success rate (queued zones eventually placed?)

**A/B Testing**: Compare runs with SVP vs without, blueprint selection vs geometric generation.

### VI. Metrics-Driven Evaluation ✅

**Success Metrics** (from spec SC-001 to SC-010):
- SVP designation success: 100% of connections
- Agent respect for SVP: 95% proposals at correct Z-level
- HousingAgent accuracy: 95% (vs 0% with placeholders)
- Bedroom coverage: All dwarves housed within 40 days
- Blueprint usage: 70% of housing proposals
- Zone queue success: 90% queued zones placed
- SVP persistence: 100% save/load correctness
- Fort organization: 80% forts show clear vertical structure
- Zone extraction accuracy: 100% match with DF UI
- Zero lost zone commands

**Game-Level Metrics**: Dwarf happiness (better housing), fort efficiency (organized layout), survival time (not directly impacted, but organized forts may be more resilient).

### VII. Simplicity & Clarity ✅

**Simple Approaches**:
- SVP uses heuristic (embark - 5 = housing) instead of complex terrain analysis
- Soil detection via bool per Z-level, not per-tile material types
- Zone extraction via DF API, no reverse engineering game state
- Blueprint integration via system prompt enhancement, not arbiter rewrite
- Zone queue uses simple retry counter and timeout, not sophisticated scheduling

**Avoiding Premature Complexity**:
- No pathfinding for SVP (just Z-level designation)
- No blueprint auto-generation (use human-created library)
- No multi-Z bedroom expansion strategy (keep bedrooms on same level)
- No dynamic SVP re-analysis (one-time on connection)

## Project Structure

### Documentation (this feature)

```text
specs/007-svp-housing-zones/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
│   └── README.md        # Internal Go APIs (no external contracts)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/
├── spatial/             # NEW - SVP component
│   ├── planner.go       # SpatialValidatorPlanner implementation
│   ├── layout.go        # ZLevelDesignation, persistence
│   └── validation.go    # Spatial validation logic
├── zones/               # NEW - Zone extraction
│   ├── extractor.go     # ZoneExtractor for DF API queries
│   ├── types.go         # ZoneInfo struct definitions
│   └── queue.go         # ZoneQueue for async execution
├── agents/              # ENHANCED - Existing agent package
│   ├── housing.go       # Enhanced with SVP integration, real zone data
│   ├── executor.go      # Enhanced with ZONE command support
│   └── agent.go         # Enhanced metrics with zone counts
├── blueprints/          # ENHANCED - Existing blueprint package
│   ├── metadata.go      # NEW - BlueprintMetadata for arbiter
│   └── loader.go        # Enhanced to support zone CSV companions
├── autonomous/          # ENHANCED - Existing loop
│   └── loop.go          # Enhanced metrics computation with zone extraction
└── config/              # ENHANCED - Configuration
    └── config.go        # New SVP and zone extraction settings

cmd/df-orchestrator/
└── main.go              # Wire SVP, zone extractor into startup

plugin/                  # ENHANCED - DFHack plugin
└── df-ai-plugin.cpp     # BLUEPRINT command, zone CSV parsing, is_soil_layer extraction

blueprints/              # ENHANCED - Blueprint library
├── bedroom_3x3.csv           # Existing dig pattern
├── bedroom_3x3_zones.csv     # NEW - Zone companion
├── bedroom_cluster_10.csv    # Existing dig pattern
└── bedroom_cluster_10_zones.csv  # NEW - Zone companion

saves/                   # NEW - Fort-specific persistence
└── {fortname}/
    └── svp_layout.json  # SVP Z-level designations

tests/
├── spatial/             # NEW - SVP tests
│   ├── planner_test.go
│   └── validation_test.go
├── zones/               # NEW - Zone extraction tests
│   ├── extractor_test.go
│   └── queue_test.go
└── agents/              # ENHANCED - Agent tests
    └── housing_test.go  # Test with real zone data
```

**Structure Decision**: Single Go project with plugin component. New packages (spatial, zones) are independent modules with clear interfaces. Existing packages (agents, blueprints, autonomous) enhanced with new functionality. DFHack plugin extended with BLUEPRINT command and soil layer extraction. Blueprint directory gets zone CSV companions. Fort-specific persistence in saves/ directory.

## Complexity Tracking

No constitution violations. All design decisions align with research-first, lean, modular principles.
