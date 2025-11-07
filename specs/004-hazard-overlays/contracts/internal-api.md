# Internal API Contracts: Hazard Overlays

**Package**: `internal/hazards`
**Feature**: 004-hazard-overlays
**Date**: 2025-11-06

## Package Overview

The `hazards` package provides sparse overlay storage for six hazard types: aquifer, water, lava, caverns, enemies, and dwarves. Each overlay uses `map[Coordinate]HazardInfo` for O(1) queries and efficient memory usage.

**Thread Safety**: All overlay operations are thread-safe using `sync.RWMutex`.

---

## Core Types

### Coordinate

```go
type Coordinate struct {
    X int16
    Y int16
    Z int16
}
```

**Used as map key** - must be comparable type.

---

### HazardInfo

```go
type HazardInfo struct {
    Severity   uint8
    Flags      uint8
    DetectedAt time.Time
}
```

**Interpretation varies by hazard type** (see data-model.md).

---

### Bounds

```go
type Bounds struct {
    Width  uint16
    Height uint16
    Depth  uint16
}
```

**Map dimensions for coordinate validation.**

---

### Region

```go
type Region struct {
    XMin, XMax int16
    YMin, YMax int16
    ZMin, ZMax int16
}
```

**Bounding box for regional queries.**

---

## HazardOverlay API

### Constructor

```go
func NewHazardOverlay(hazardType string, bounds Bounds) *HazardOverlay
```

**Parameters**:
- `hazardType`: Human-readable name ("aquifer", "water", "lava", etc.) for logging
- `bounds`: Map dimensions for coordinate validation

**Returns**: Initialized empty overlay

**Example**:
```go
aquifer := hazards.NewHazardOverlay("aquifer", hazards.Bounds{
    Width: 256, Height: 256, Depth: 189,
})
```

---

### Query Operations (Thread-Safe Reads)

#### Contains

```go
func (h *HazardOverlay) Contains(x, y, z int16) bool
```

**Purpose**: Check if coordinate contains a hazard

**Parameters**: x, y, z coordinates (validated against bounds)

**Returns**: `true` if hazard exists at coordinate, `false` otherwise

**Performance**: O(1) average, <10μs

**Thread Safety**: Uses RLock (concurrent reads allowed)

**Example**:
```go
if aquifer.Contains(10, 20, 30) {
    log.Info("aquifer detected at (10, 20, 30)")
}
```

---

#### Get

```go
func (h *HazardOverlay) Get(x, y, z int16) (HazardInfo, bool)
```

**Purpose**: Retrieve hazard metadata if exists

**Parameters**: x, y, z coordinates (validated against bounds)

**Returns**:
- `HazardInfo`: Hazard metadata (severity, flags, detected time)
- `bool`: `true` if hazard exists, `false` if not found

**Performance**: O(1) average, <10μs

**Thread Safety**: Uses RLock (concurrent reads allowed)

**Example**:
```go
if info, exists := water.Get(50, 60, 70); exists {
    depth := info.Severity // Water depth 0-7
    isFlowing := (info.Flags & 0x08) != 0
    log.Infof("water depth=%d, flowing=%v", depth, isFlowing)
}
```

---

#### GetInRegion

```go
func (h *HazardOverlay) GetInRegion(region Region) []Coordinate
```

**Purpose**: Get all hazard coordinates within bounding box

**Parameters**: `region` - bounding box (XMin/Max, YMin/Max, ZMin/Max)

**Returns**: Slice of coordinates (unsorted)

**Performance**: O(n) where n = total hazard count (iterates all hazards, filters by bounds)

**Thread Safety**: Uses RLock (concurrent reads allowed)

**Example**:
```go
region := hazards.Region{
    XMin: 0, XMax: 50,
    YMin: 0, YMax: 50,
    ZMin: 80, ZMax: 90,
}
lavaCoords := lava.GetInRegion(region)
log.Infof("found %d lava tiles in region", len(lavaCoords))
```

---

#### GetCount

```go
func (h *HazardOverlay) GetCount() uint32
```

**Purpose**: Get total hazard count (cached, no iteration)

**Returns**: Number of hazards in overlay

**Performance**: O(1)

**Thread Safety**: Uses RLock

**Example**:
```go
count := enemies.GetCount()
log.Infof("tracking %d hostile creatures", count)
```

---

### Mutation Operations (Thread-Safe Writes)

#### Add

```go
func (h *HazardOverlay) Add(coord Coordinate, info HazardInfo) error
```

**Purpose**: Add or update hazard at coordinate

**Parameters**:
- `coord`: Coordinate to add (validated against bounds)
- `info`: Hazard metadata

**Returns**: `error` if coordinate out of bounds

**Side Effects**: Increments `totalCount` if new coordinate

**Performance**: O(1) average, <1μs

**Thread Safety**: Uses Lock (exclusive write access)

**Example**:
```go
coord := hazards.Coordinate{X: 10, Y: 20, Z: 30}
info := hazards.HazardInfo{
    Severity: 7,        // Full water depth
    Flags: 0x08,        // Flowing bit set
    DetectedAt: time.Now(),
}
if err := water.Add(coord, info); err != nil {
    log.Errorf("failed to add water hazard: %v", err)
}
```

---

#### Remove

```go
func (h *HazardOverlay) Remove(coord Coordinate) error
```

**Purpose**: Remove hazard at coordinate

**Parameters**: `coord` - Coordinate to remove

**Returns**: `error` if coordinate out of bounds (silently succeeds if not found)

**Side Effects**: Decrements `totalCount` if coordinate existed

**Performance**: O(1) average, <1μs

**Thread Safety**: Uses Lock (exclusive write access)

**Example**:
```go
coord := hazards.Coordinate{X: 10, Y: 20, Z: 30}
if err := water.Remove(coord); err != nil {
    log.Errorf("failed to remove water hazard: %v", err)
}
```

---

#### Clear

```go
func (h *HazardOverlay) Clear()
```

**Purpose**: Remove all hazards (for rebuild)

**Side Effects**: Resets `totalCount` to 0, creates new empty map

**Performance**: O(1) (GC handles old map cleanup)

**Thread Safety**: Uses Lock (exclusive write access)

**Example**:
```go
aquifer.Clear() // Prepare for rebuild from FULL_STATE
```

---

## HazardManager API

### Constructor

```go
func NewHazardManager(width, height, depth uint16) *HazardManager
```

**Parameters**: Map dimensions

**Returns**: HazardManager with all 6 overlays initialized

**Example**:
```go
manager := hazards.NewHazardManager(256, 256, 189)
```

---

### Build & Update Operations

#### BuildFromTiles

```go
func (m *HazardManager) BuildFromTiles(tiles []protocol.TileState)
```

**Purpose**: Initial build of all environmental overlays from FULL_STATE

**Parameters**: `tiles` - Full tile array from FULL_STATE message

**Side Effects**:
- Clears all environmental overlays (aquifer, water, lava, caverns)
- Detects hazards using TileType heuristics
- Populates overlays with detected hazards
- Logs counts for each overlay

**Performance**: O(n) where n = tile count (6.9M tiles → ~50-100ms)

**Thread Safety**: Thread-safe (calls overlay Add/Clear methods)

**Does NOT affect**: Entity overlays (enemies, dwarves) - those use BuildFromEntities

**Example**:
```go
client.SetOnFullState(func(state *protocol.FullStateMessage) {
    hazardManager.BuildFromTiles(state.Tiles)
    log.Info("hazard overlays built from full state")
})
```

---

#### UpdateFromTiles

```go
func (m *HazardManager) UpdateFromTiles(tiles []protocol.TileState)
```

**Purpose**: Incremental update of environmental overlays from TILE_UPDATE

**Parameters**: `tiles` - Changed tiles from TILE_UPDATE message

**Side Effects**:
- For each changed tile: detect hazard type
- If hazard: Add/Update in appropriate overlay
- If not hazard: Remove from overlay (if previously hazardous)
- Logs update counts

**Performance**: O(n) where n = changed tile count (typically 10-100 tiles → <10ms)

**Thread Safety**: Thread-safe (calls overlay Add/Remove methods)

**Example**:
```go
updates := client.SubscribeTileUpdates()
go func() {
    for update := range updates {
        hazardManager.UpdateFromTiles(update.Tiles)
    }
}()
```

---

#### BuildFromEntities (Future)

```go
func (m *HazardManager) BuildFromEntities(entities []protocol.EntityInfo)
```

**Purpose**: Build entity overlays (enemies, dwarves) from entity list

**Parameters**: `entities` - Entity array from ENTITY_UPDATE message

**Side Effects**:
- Clears entity overlays
- Filters entities by type (dwarf vs enemy)
- Populates entity overlays with positions

**Status**: **Not implemented in MVP** - requires ENTITY_UPDATE protocol message

**Example**:
```go
// Future implementation
client.SetOnEntityUpdate(func(entities *protocol.EntityUpdateMessage) {
    hazardManager.BuildFromEntities(entities.Entities)
})
```

---

### Accessor Operations

#### GetOverlay

```go
func (m *HazardManager) GetOverlay(hazardType string) *HazardOverlay
```

**Purpose**: Retrieve specific overlay by type name

**Parameters**: `hazardType` - "aquifer", "water", "lava", "caverns", "enemies", or "dwarves"

**Returns**: Overlay pointer, or `nil` if type unknown

**Example**:
```go
aquifer := manager.GetOverlay("aquifer")
if aquifer != nil && aquifer.Contains(x, y, z) {
    log.Warn("aquifer detected, do not dig here")
}
```

---

#### GetAllCounts

```go
func (m *HazardManager) GetAllCounts() map[string]uint32
```

**Purpose**: Get hazard counts for all overlays (for logging/metrics)

**Returns**: Map of hazard type → count

**Example**:
```go
counts := manager.GetAllCounts()
// counts = {"aquifer": 500, "water": 200, "lava": 100, "caverns": 1000, "enemies": 10, "dwarves": 50}
log.Infof("hazard counts: %+v", counts)
```

---

#### GetMemoryUsage

```go
func (m *HazardManager) GetMemoryUsage() uint64
```

**Purpose**: Estimate total memory usage across all overlays

**Returns**: Bytes (approximate - based on count × 32 bytes per hazard)

**Example**:
```go
memoryKB := manager.GetMemoryUsage() / 1024
log.Infof("hazard overlays using ~%d KB", memoryKB)
```

---

## Detector Functions (Internal)

These functions are used internally by HazardManager, but exposed for testing.

### IsAquifer

```go
func IsAquifer(tileType uint16, flags uint8) bool
```

**Purpose**: Detect if tile is aquifer from TileType/Flags

**Logic**: TileType ranges for damp/wet walls, or Flags aquifer bit

**Returns**: `true` if aquifer detected

---

### IsWater

```go
func IsWater(tileType uint16) (bool, uint8, uint8)
```

**Purpose**: Detect water tile and extract depth/flow

**Returns**:
- `bool`: Is water tile
- `uint8`: Water depth (0-7)
- `uint8`: Flow flags (0-2 bits = direction, bit 3 = standing/flowing)

---

### IsLava

```go
func IsLava(tileType uint16) (bool, uint8, uint8)
```

**Purpose**: Detect lava tile and extract depth/flow

**Returns**: Same as IsWater

---

### IsCavern

```go
func IsCavern(tileType uint16) bool
```

**Purpose**: Detect natural cavern tile (vs mined/constructed)

**Returns**: `true` if cavern detected

---

## Error Handling

**All functions return errors for validation failures**:

```go
var (
    ErrOutOfBounds = errors.New("coordinate out of bounds")
)
```

**Idempotent operations** (Remove, Clear): Do not error if hazard doesn't exist

**Thread safety**: All panics from concurrent access prevented by mutexes

---

## Integration Example

```go
package main

import (
    "github.com/df-ai/orchestrator/internal/hazards"
    "github.com/df-ai/orchestrator/internal/protocol"
)

func main() {
    // Initialize manager
    manager := hazards.NewHazardManager(256, 256, 189)

    // Build from FULL_STATE
    client.SetOnFullState(func(state *protocol.FullStateMessage) {
        manager.BuildFromTiles(state.Tiles)
        counts := manager.GetAllCounts()
        log.Infof("hazards detected: %+v", counts)
    })

    // Update from TILE_UPDATE
    updates := client.SubscribeTileUpdates()
    go func() {
        for update := range updates {
            manager.UpdateFromTiles(update.Tiles)
        }
    }()

    // Query hazards
    aquifer := manager.GetOverlay("aquifer")
    if aquifer.Contains(10, 20, 30) {
        log.Warn("do not dig at (10, 20, 30) - aquifer present")
    }
}
```

---

## Performance Guarantees

| Operation | Complexity | Target Latency |
|-----------|-----------|----------------|
| Contains(x,y,z) | O(1) | <10μs |
| Get(x,y,z) | O(1) | <10μs |
| GetInRegion(region) | O(n) | <100μs for 1000 tiles |
| Add(coord, info) | O(1) | <1μs |
| Remove(coord) | O(1) | <1μs |
| BuildFromTiles | O(n) | <100ms for 6.9M tiles |
| UpdateFromTiles | O(n) | <10ms for 100 tiles |

**Memory**: <50 KB total for <1600 total hazards across all overlays

---

## Thread Safety Guarantees

- **Concurrent reads**: Multiple goroutines can call Contains/Get/GetInRegion simultaneously
- **Exclusive writes**: Add/Remove/Clear acquire exclusive lock
- **No data races**: All fields protected by RWMutex
- **Safe for testing**: Use `-race` flag to verify thread safety
