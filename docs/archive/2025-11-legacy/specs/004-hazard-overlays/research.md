# Research: Hazard Overlays

**Feature**: 004-hazard-overlays | **Date**: 2025-11-06

## R1: Sparse Storage Data Structure

**Decision**: `map[Coordinate]HazardInfo` per hazard type

**Rationale**:
- Go maps are hash tables with O(1) average lookup (aligns with <10μs query goal)
- Sparse storage: Only store hazard coordinates (~2000 typical), not all 6.9M tiles
- Memory per entry: ~16 bytes (Coordinate key: 6 bytes for X/Y/Z int16) + ~8 bytes (HazardInfo value) + ~8 bytes (map overhead) = ~32 bytes total
- For 2000 hazards: 2000 × 32 bytes = 64 KB (within <50 KB target if <1600 hazards, or compress HazardInfo)
- Thread-safe with sync.RWMutex (many concurrent readers, rare writes)

**Alternatives Considered**:
- **Quadtree/Octree**: Complex spatial partitioning, overkill for sparse data, harder to maintain
- **Sorted slice with binary search**: O(log n) lookup (slower than O(1) map), O(n) insertion
- **Bit array like topology**: Would waste memory (6.9M bits = 850 KB even at 1 bit per tile)

**Map is simplest and meets performance goals.**

---

## R2: Protocol Enhancement for Hazard Detection

**Decision**: Start with TileType-only detection (MVP), enhance protocol later if needed

**Current TileState Protocol**:
```go
type TileState struct {
    X, Y, Z    int16   // 6 bytes - coordinate
    TileType   uint16  // 2 bytes - DF tiletype enum (floor, wall, ramp, etc.)
    Flags      uint8   // 1 byte - designation flags
}
```

**MVP Detection Strategy** (TileType-based heuristics):

1. **Aquifer**:
   - TileType ranges indicating damp/wet stone walls (research DF tiletype.h enum)
   - May use Flags for aquifer designation bit
   - Detection accuracy goal: 100% (aquifer is critical safety hazard)

2. **Water**:
   - TileType for water floor tiles (standing vs flowing differentiation may require additional data)
   - Current limitation: Cannot distinguish depth or flow direction without LiquidDepth/FlowVector
   - MVP: Detect presence, log if flow state needed

3. **Lava**:
   - TileType for magma floor tiles
   - Same depth/flow limitations as water
   - MVP: Binary presence detection sufficient for safety

4. **Caverns**:
   - TileType for natural stone floors/walls (vs constructed/mined)
   - Biome/material data would improve accuracy but not critical
   - MVP: Heuristic based on TileType patterns

5. **Entities** (Enemies/Dwarves):
   - **NOT in current TILE_UPDATE messages**
   - Requires new protocol message: ENTITY_UPDATE or ENTITY_POSITIONS
   - DFHack API: `df.global.world.units.active` provides unit list with coordinates
   - Defer to Phase 1 design

**Future Protocol Enhancement** (if MVP insufficient):
```go
type TileState struct {
    // Existing fields
    X, Y, Z    int16
    TileType   uint16
    Flags      uint8

    // NEW fields for hazard detection
    Material   uint16  // Stone/soil/ore type for cavern detection
    LiquidDepth uint8  // 0-7 depth for water/magma (0 = none, 7 = full)
    LiquidType  uint8  // 0 = none, 1 = water, 2 = magma
    FlowVector  uint8  // Bitfield: flow direction (N/S/E/W/Up/Down)
}
```

**For MVP**: Use TileType heuristics, log detection accuracy during testing, enhance protocol only if gaps found.

---

## R3: Entity Position Tracking

**Decision**: Implement entity overlays in parallel with environmental hazards (both in US1 scope)

**Approach**:

1. **Data Source**: DFHack Units API
   - `df.global.world.units.active` - all active units (dwarves, enemies, animals)
   - Each unit has: `id`, `pos.x/y/z`, `race`, `civ_id`, `flags` (enemy, tame, etc.)

2. **Protocol Message** (new):
```
Message Type: ENTITY_UPDATE (0x05)
Fields:
  - EntityCount uint32
  - Entities []EntityInfo {
      ID       uint32
      X, Y, Z  int16
      Type     uint8  // 1=dwarf, 2=enemy, 3=animal, 4=other
      Subtype  uint16 // Race ID for detailed classification
    }
```

3. **Detection Logic**:
   - **Dwarves**: `unit.civ_id == fort_civ_id && unit.race == DWARF`
   - **Enemies**: `unit.flags.hostile == true` or `unit.civ_id != fort_civ_id && aggressive_race`

4. **Update Frequency**:
   - Environmental hazards: Update from TILE_UPDATE (infrequent - only when tiles change)
   - Entities: Update from ENTITY_UPDATE (frequent - every tick or every N ticks as entities move)

**Implementation Plan**:
- Phase 1: Design ENTITY_UPDATE protocol message format
- Phase 1: Add entity detection to DFHack plugin (scan units, send ENTITY_UPDATE)
- Phase 1: Implement entity overlays using same HazardOverlay sparse map structure
- Phase 2: Wire up entity updates in main.go message handler

---

## R4: Concurrent Access Patterns

**Decision**: sync.RWMutex per overlay for thread-safe concurrent reads

**Rationale**:
- **Read-heavy workload**: Pathfinding, planning, queries (many concurrent readers)
- **Write-rare**: Updates only on TILE_UPDATE (environmental) or ENTITY_UPDATE (entities)
- RWMutex allows multiple concurrent readers with exclusive writer lock
- Aligns with topology overlay pattern (proven design)

**Implementation**:
```go
type HazardOverlay struct {
    mu       sync.RWMutex
    hazards  map[Coordinate]HazardInfo
    // metadata: count, lastUpdate, etc.
}

func (h *HazardOverlay) Contains(x, y, z int16) bool {
    h.mu.RLock()
    defer h.mu.RUnlock()
    _, exists := h.hazards[Coordinate{x, y, z}]
    return exists
}

func (h *HazardOverlay) Update(coord Coordinate, info HazardInfo) {
    h.mu.Lock()
    defer h.mu.Unlock()
    h.hazards[coord] = info
}
```

---

## Summary

- **Storage**: Sparse `map[Coordinate]HazardInfo` per hazard type, ~32 bytes per entry
- **Detection**: TileType-based heuristics for MVP (environmental hazards), enhance protocol if needed
- **Entities**: New ENTITY_UPDATE protocol message, parallel implementation with environmental hazards
- **Concurrency**: sync.RWMutex per overlay for concurrent reads
- **Memory Budget**: <50 KB total (achievable with <1600 total hazards across all 6 overlays)
- **Performance**: O(1) map lookup meets <10μs query goal, RWMutex enables concurrent access
