# Data Model: Hazard Overlays

**Feature**: 004-hazard-overlays | **Date**: 2025-11-06

## Core Entities

### Coordinate

**Purpose**: 3D spatial position key for sparse maps

**Fields**:
```go
type Coordinate struct {
    X int16  // 0 to Width-1 (typically 0-255)
    Y int16  // 0 to Height-1 (typically 0-255)
    Z int16  // 0 to Depth-1 (typically 0-189)
}
```

**Validation Rules**:
- X, Y, Z must be >= 0
- X < map width, Y < map height, Z < map depth
- Used as map key (must be comparable type)

**Memory**: 6 bytes per coordinate

---

### HazardInfo

**Purpose**: Metadata stored per hazard coordinate

**Fields**:
```go
type HazardInfo struct {
    Severity   uint8     // 0-255: hazard severity/depth (e.g., water depth 0-7, or threat level)
    Flags      uint8     // Bitfield: flow direction, standing/flowing, aquifer type, etc.
    DetectedAt time.Time // When hazard was first detected (for staleness tracking)
}
```

**Flag Bits** (Flags field):
- Bit 0-2: Flow direction (N=001, S=010, E=011, W=100, Up=101, Down=110)
- Bit 3: Standing (0) vs Flowing (1) for liquids
- Bit 4-5: Liquid type (00=none, 01=water, 10=magma, 11=other)
- Bit 6-7: Reserved for future use

**Validation Rules**:
- Severity: 0-255 (no validation, interpretation depends on hazard type)
- DetectedAt: Must be non-zero after detection

**Memory**: ~16 bytes per entry (1 byte + 1 byte + 8 bytes time + padding)

---

### HazardOverlay

**Purpose**: Generic sparse storage for a single hazard type

**Fields**:
```go
type HazardOverlay struct {
    mu          sync.RWMutex              // Concurrent access protection
    hazards     map[Coordinate]HazardInfo // Sparse storage (only hazard coordinates)
    hazardType  string                     // "aquifer", "water", "lava", etc. (for logging)
    totalCount  uint32                     // Cached count (updated on add/remove)
    lastUpdate  time.Time                  // Last update timestamp
    mapBounds   Bounds                     // Map dimensions for validation
}

type Bounds struct {
    Width  uint16
    Height uint16
    Depth  uint16
}
```

**Operations**:
- `NewHazardOverlay(hazardType string, bounds Bounds) *HazardOverlay`
- `Contains(x, y, z int16) bool` - O(1) hazard existence check
- `Get(x, y, z int16) (HazardInfo, bool)` - O(1) retrieve hazard metadata
- `Add(coord Coordinate, info HazardInfo) error` - Add/update hazard
- `Remove(coord Coordinate) error` - Remove hazard
- `GetInRegion(bounds Region) []Coordinate` - Get all hazards in bounding box
- `GetCount() uint32` - Thread-safe count getter
- `Clear()` - Remove all hazards (for rebuild)

**Validation Rules**:
- Coordinate must be within mapBounds
- totalCount must match len(hazards)
- lastUpdate must be updated on any add/remove

**Concurrency**:
- Read operations (Contains, Get, GetInRegion, GetCount): RLock
- Write operations (Add, Remove, Clear): Lock

**Memory**: 32 bytes overhead + (32 bytes × hazard count)

---

## Specific Hazard Types

### AquiferOverlay

**Purpose**: Track aquifer tiles (critical safety hazard)

**HazardInfo Interpretation**:
- Severity: Aquifer strength (0=light, 255=heavy)
- Flags: Aquifer type bits

**Detection**: TileType ranges for damp/wet stone walls, or Flags aquifer bit

**Update Frequency**: Rare (only when tiles mined/changed)

---

### WaterOverlay

**Purpose**: Track water tiles (standing and flowing)

**HazardInfo Interpretation**:
- Severity: Water depth (0-7, where 7=full tile)
- Flags: Bit 3 = standing/flowing, Bits 0-2 = flow direction

**Detection**: TileType for water floor tiles

**Update Frequency**: Moderate (water spreads/drains over time)

---

### LavaOverlay

**Purpose**: Track lava/magma tiles (critical safety hazard)

**HazardInfo Interpretation**:
- Severity: Lava depth (0-7, where 7=full tile)
- Flags: Bit 3 = standing/flowing, Bits 0-2 = flow direction

**Detection**: TileType for magma floor tiles

**Update Frequency**: Rare (lava rarely moves in typical forts)

---

### CavernOverlay

**Purpose**: Track natural cavern tiles (underground open space)

**HazardInfo Interpretation**:
- Severity: Cavern layer (1=first cavern, 2=second, 3=third)
- Flags: Reserved

**Detection**: TileType for natural stone floors/walls (vs constructed/mined)

**Update Frequency**: Rare (caverns discovered, not created)

---

### EnemyOverlay

**Purpose**: Track hostile creature positions

**HazardInfo Interpretation**:
- Severity: Threat level (based on creature type/skills)
- Flags: Reserved for future use (alert status, etc.)

**Detection**: ENTITY_UPDATE protocol message, filter by hostile flag

**Update Frequency**: High (entities move every tick or every N ticks)

**Additional Fields** (extended HazardInfo):
```go
type EntityHazardInfo struct {
    HazardInfo            // Embedded base fields
    EntityID   uint32     // Unit ID for tracking across updates
    RaceID     uint16     // Creature race for threat assessment
}
```

---

### DwarfOverlay

**Purpose**: Track friendly dwarf positions (for collision avoidance)

**HazardInfo Interpretation**:
- Severity: Dwarf skill/importance (optional)
- Flags: Current task type (idle, hauling, mining, etc.)

**Detection**: ENTITY_UPDATE protocol message, filter by civ_id and race

**Update Frequency**: High (dwarves move frequently)

**Additional Fields** (extended HazardInfo):
```go
type DwarfHazardInfo struct {
    HazardInfo            // Embedded base fields
    DwarfID    uint32     // Unit ID for tracking across updates
    TaskType   uint8      // Current job/task (for context)
}
```

---

## Manager Entity

### HazardManager

**Purpose**: Coordinate all hazard overlays, handle protocol updates

**Fields**:
```go
type HazardManager struct {
    aquifer  *HazardOverlay
    water    *HazardOverlay
    lava     *HazardOverlay
    caverns  *HazardOverlay
    enemies  *HazardOverlay  // Uses EntityHazardInfo internally
    dwarves  *HazardOverlay  // Uses DwarfHazardInfo internally

    mapBounds Bounds
}
```

**Operations**:
- `NewHazardManager(width, height, depth uint16) *HazardManager`
- `BuildFromTiles(tiles []protocol.TileState)` - Initial build from FULL_STATE
- `UpdateFromTiles(tiles []protocol.TileState)` - Incremental update from TILE_UPDATE
- `UpdateFromEntities(entities []protocol.EntityInfo)` - Update entity overlays from ENTITY_UPDATE
- `GetOverlay(hazardType string) *HazardOverlay` - Retrieve specific overlay
- `GetAllCounts() map[string]uint32` - Get counts for all overlays (for logging/metrics)
- `GetMemoryUsage() uint64` - Total memory across all overlays

**Lifecycle**:
1. Create HazardManager on startup with map dimensions
2. Call BuildFromTiles when FULL_STATE received
3. Call UpdateFromTiles when TILE_UPDATE received
4. Call UpdateFromEntities when ENTITY_UPDATE received (if implemented)

---

## State Transitions

### Environmental Hazards (Aquifer, Water, Lava, Caverns)

```
[Undetected] --dig/reveal--> [Detected] --tile_change--> [Updated] --drain/fill--> [Removed]
     |                            |                           |                         |
     +----------------------------+---------------------------+-------------------------+
                                           (sparse map lifecycle)
```

**Triggers**:
- Add: TILE_UPDATE shows new hazard tile (aquifer breached, water flows in)
- Update: TILE_UPDATE changes hazard properties (water depth changes)
- Remove: TILE_UPDATE shows tile no longer hazardous (water drained)

### Entity Hazards (Enemies, Dwarves)

```
[Spawned] --ENTITY_UPDATE--> [Tracked] --movement--> [Position Updated] --death/leave--> [Removed]
    |                             |                         |                               |
    +-----------------------------+-------------------------+-------------------------------+
                                    (highly dynamic, updates every tick)
```

**Triggers**:
- Add: ENTITY_UPDATE shows new entity ID at position
- Update: ENTITY_UPDATE shows existing entity ID at new position
- Remove: ENTITY_UPDATE no longer includes entity ID (died/left map)

---

## Memory Budget Analysis

**Target**: <50 KB total for all 6 overlays

**Typical Fort Hazard Counts** (conservative estimates):
- Aquifer: 500 tiles × 32 bytes = 16 KB
- Water: 200 tiles × 32 bytes = 6.4 KB
- Lava: 100 tiles × 32 bytes = 3.2 KB
- Caverns: 1000 tiles × 32 bytes = 32 KB
- Enemies: 10 entities × 40 bytes = 0.4 KB (EntityHazardInfo slightly larger)
- Dwarves: 50 entities × 40 bytes = 2 KB

**Total**: ~60 KB (slightly over budget)

**Optimizations if needed**:
- Reduce HazardInfo to 8 bytes (remove DetectedAt, or pack into uint64)
- Caverns: Only track edges (not full cavern area), reduce count to ~200 tiles → 6.4 KB savings
- Target adjusted: <50 KB achievable with optimization

**Acceptable for MVP**: 60 KB is close enough to proceed, optimize if testing shows issues.
