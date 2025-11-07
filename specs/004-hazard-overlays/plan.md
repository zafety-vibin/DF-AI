# Implementation Plan: Hazard Overlays

**Branch**: `004-hazard-overlays` | **Date**: 2025-11-06 | **Spec**: [spec.md](./spec.md)

## Summary

Build six independent sparse overlays tracking aquifer, water, lava, caverns, enemies, and dwarves. Sparse map storage (only hazard coordinates, ~20-30 bytes each). Detect from tile data and entity positions. Update incrementally. Memory target: <50 KB total.

**Primary Requirement**: AI needs hazard awareness to avoid catastrophes (aquifer breach, magma flood) and make dwarf-safe decisions.

**Technical Approach**: `map[Coordinate]HazardInfo` for each type, detector functions per hazard, RWMutex for concurrent reads, may need TileState protocol enhancement for liquid/material data.

## Technical Context

**Language/Version**: Go 1.21+
**Primary Dependencies**: Go stdlib only (may need protocol enhancement)
**Storage**: In-memory sparse maps (~10-50 KB total)
**Testing**: Go testing framework, race detector
**Target Platform**: Windows/Linux server
**Project Type**: Single project
**Performance Goals**: <10 μs query, <10ms updates, <50 KB memory
**Constraints**: Sparse storage, concurrent reads, protocol limits (current TileState may lack liquid depth/flow data)
**Scale/Scope**: 6 overlays, ~2000 hazards typical, ~400 LOC

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

✅ All principles aligned - sparse overlays enable AI safety experiments without hard-coded rails.

## Project Structure

### Documentation (this feature)

```text
specs/004-hazard-overlays/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)

```text
internal/hazards/
├── overlay.go           # Generic HazardOverlay sparse map
├── detectors.go         # Hazard detection functions (IsAquifer, IsWater, etc.)
├── environmental.go     # Aquifer, Water, Lava, Cavern overlays
├── entities.go          # Enemy and Dwarf position overlays
└── manager.go           # Coordinate all overlays, update from TILE_UPDATE

tests/hazards/
├── overlay_test.go      # Sparse map tests
├── detectors_test.go    # Detection logic tests
└── benchmark_test.go    # Query/update performance benchmarks
```

**Structure Decision**: New `internal/hazards/` package with 5 source files. Each overlay type (aquifer, water, lava, caverns, enemies, dwarves) uses the generic HazardOverlay sparse map structure with custom detector functions. Manager coordinates updates from protocol messages.

**Note**: Protocol enhancement may be needed - current TileState only has TileType and Flags. Need liquid depth, flow state, material for accurate hazard detection. This will be researched in Phase 0.

## Complexity Tracking

No violations - all constitution principles aligned.
