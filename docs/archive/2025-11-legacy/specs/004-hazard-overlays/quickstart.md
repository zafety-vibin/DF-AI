# Quickstart: Hazard Overlays

**Feature**: 004-hazard-overlays | **Date**: 2025-11-06

## What Are Hazard Overlays?

Hazard overlays are **sparse spatial maps** that track dangerous tiles (aquifer, water, lava, caverns) and entity positions (enemies, dwarves) in a Dwarf Fortress fort. They enable the AI to make safety-aware decisions without checking all 6.9M tiles.

**Key Characteristics**:
- **Sparse storage**: Only hazard coordinates stored (~2000 typical vs 6.9M total tiles)
- **O(1) queries**: `Contains(x,y,z)` checks if coordinate is hazardous in <10μs
- **Thread-safe**: Concurrent reads using `sync.RWMutex`
- **Memory efficient**: <50 KB total for all 6 overlays combined
- **Independent**: Each hazard type (aquifer, water, lava, etc.) is separate overlay

---

## 5-Minute Integration

### Step 1: Create HazardManager

```go
package main

import (
    "github.com/df-ai/orchestrator/internal/hazards"
    "github.com/df-ai/orchestrator/internal/protocol"
)

var hazardManager *hazards.HazardManager

func main() {
    // Initialize with map dimensions from config or HANDSHAKE
    hazardManager = hazards.NewHazardManager(256, 256, 189)
}
```

---

### Step 2: Build Overlays from FULL_STATE

```go
// In your DFHack client setup
client.SetOnFullState(func(state *protocol.FullStateMessage) {
    // Build hazard overlays from tile data
    hazardManager.BuildFromTiles(state.Tiles)

    // Log detected hazards
    counts := hazardManager.GetAllCounts()
    memoryKB := hazardManager.GetMemoryUsage() / 1024

    log.WithFields(log.Fields{
        "aquifer":  counts["aquifer"],
        "water":    counts["water"],
        "lava":     counts["lava"],
        "caverns":  counts["caverns"],
        "enemies":  counts["enemies"],
        "dwarves":  counts["dwarves"],
        "memory_kb": memoryKB,
    }).Info("hazard overlays built")
})
```

---

### Step 3: Update Overlays Incrementally

```go
// Subscribe to tile updates
updates := client.SubscribeTileUpdates()

go func() {
    for update := range updates {
        // Update hazard overlays from changed tiles
        hazardManager.UpdateFromTiles(update.Tiles)
    }
}()
```

---

### Step 4: Query Hazards

```go
// Check if specific coordinate is hazardous
func isSafeToDig(x, y, z int16) bool {
    aquifer := hazardManager.GetOverlay("aquifer")
    water := hazardManager.GetOverlay("water")
    lava := hazardManager.GetOverlay("lava")

    // Check all critical hazards
    if aquifer.Contains(x, y, z) {
        log.Warnf("aquifer at (%d,%d,%d) - do not dig", x, y, z)
        return false
    }

    if water.Contains(x, y, z) {
        log.Warnf("water at (%d,%d,%d) - flooding risk", x, y, z)
        return false
    }

    if lava.Contains(x, y, z) {
        log.Warnf("lava at (%d,%d,%d) - incineration risk", x, y, z)
        return false
    }

    return true
}
```

---

### Step 5: Regional Queries

```go
// Get all hazards in a region (e.g., for planning mining area)
func getHazardsInMiningArea(xMin, xMax, yMin, yMax, zMin, zMax int16) map[string][]hazards.Coordinate {
    region := hazards.Region{
        XMin: xMin, XMax: xMax,
        YMin: yMin, YMax: yMax,
        ZMin: zMin, ZMax: zMax,
    }

    result := make(map[string][]hazards.Coordinate)

    // Check each hazard type
    for _, hazardType := range []string{"aquifer", "water", "lava", "caverns"} {
        overlay := hazardManager.GetOverlay(hazardType)
        coords := overlay.GetInRegion(region)
        if len(coords) > 0 {
            result[hazardType] = coords
        }
    }

    return result
}
```

---

## Common Use Cases

### Use Case 1: Pre-Dig Safety Check

```go
func canDigDesignation(designation Designation) (bool, string) {
    for _, coord := range designation.Tiles {
        // Check aquifer (critical - instant fort failure if breached)
        if hazardManager.GetOverlay("aquifer").Contains(coord.X, coord.Y, coord.Z) {
            return false, fmt.Sprintf("aquifer at %v - catastrophic breach risk", coord)
        }

        // Check water (high risk - flooding)
        if water := hazardManager.GetOverlay("water"); water.Contains(coord.X, coord.Y, coord.Z) {
            if info, _ := water.Get(coord.X, coord.Y, coord.Z); info.Severity >= 7 {
                return false, fmt.Sprintf("deep water at %v - flooding risk", coord)
            }
        }

        // Check lava (high risk - incineration)
        if hazardManager.GetOverlay("lava").Contains(coord.X, coord.Y, coord.Z) {
            return false, fmt.Sprintf("lava at %v - incineration risk", coord)
        }
    }

    return true, "safe to dig"
}
```

---

### Use Case 2: Entity Collision Avoidance

```go
func isDwarfPresentAt(x, y, z int16) bool {
    dwarves := hazardManager.GetOverlay("dwarves")
    return dwarves.Contains(x, y, z)
}

func getEnemyCount() uint32 {
    enemies := hazardManager.GetOverlay("enemies")
    return enemies.GetCount()
}
```

---

### Use Case 3: Hazard Metrics for HTTP API

```go
// In internal/http/metrics.go
func (s *Server) metricsHandler(w http.ResponseWriter, r *http.Request) {
    counts := hazardManager.GetAllCounts()
    memoryBytes := hazardManager.GetMemoryUsage()

    metrics := map[string]interface{}{
        "hazards": map[string]interface{}{
            "aquifer_tiles":  counts["aquifer"],
            "water_tiles":    counts["water"],
            "lava_tiles":     counts["lava"],
            "cavern_tiles":   counts["caverns"],
            "enemy_count":    counts["enemies"],
            "dwarf_count":    counts["dwarves"],
            "memory_bytes":   memoryBytes,
            "memory_kb":      memoryBytes / 1024,
        },
    }

    json.NewEncoder(w).Write(metrics)
}
```

---

## Testing

### Unit Test Example

```go
package hazards

import (
    "testing"
    "github.com/stretchr/testify/assert"
)

func TestHazardOverlay_AddAndContains(t *testing.T) {
    bounds := Bounds{Width: 100, Height: 100, Depth: 50}
    overlay := NewHazardOverlay("test", bounds)

    // Add hazard
    coord := Coordinate{X: 10, Y: 20, Z: 30}
    info := HazardInfo{Severity: 5, Flags: 0, DetectedAt: time.Now()}
    err := overlay.Add(coord, info)
    assert.NoError(t, err)

    // Check contains
    assert.True(t, overlay.Contains(10, 20, 30))
    assert.False(t, overlay.Contains(11, 21, 31))

    // Check count
    assert.Equal(t, uint32(1), overlay.GetCount())
}

func TestHazardOverlay_OutOfBounds(t *testing.T) {
    bounds := Bounds{Width: 10, Height: 10, Depth: 10}
    overlay := NewHazardOverlay("test", bounds)

    coord := Coordinate{X: 100, Y: 100, Z: 100} // Out of bounds
    info := HazardInfo{Severity: 1, Flags: 0, DetectedAt: time.Now()}
    err := overlay.Add(coord, info)

    assert.Error(t, err)
    assert.Equal(t, ErrOutOfBounds, err)
}
```

---

### Benchmark Example

```go
func BenchmarkHazardOverlay_Contains(b *testing.B) {
    bounds := Bounds{Width: 256, Height: 256, Depth: 189}
    overlay := NewHazardOverlay("benchmark", bounds)

    // Add 1000 hazards
    for i := 0; i < 1000; i++ {
        coord := Coordinate{X: int16(i % 256), Y: int16(i / 256), Z: 0}
        overlay.Add(coord, HazardInfo{Severity: 1, Flags: 0, DetectedAt: time.Now()})
    }

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        overlay.Contains(10, 20, 0) // Should be <10μs
    }
}
```

**Run benchmarks**:
```bash
go test -bench=. -benchmem ./tests/hazards/
```

**Expected results**:
- `BenchmarkHazardOverlay_Contains`: <10μs per op
- `BenchmarkHazardManager_BuildFromTiles`: <100ms for 6.9M tiles
- `BenchmarkHazardManager_UpdateFromTiles`: <10ms for 100 tiles

---

## Debugging

### Enable Debug Logging

```go
// In main.go or config
log.SetLevel(log.DebugLevel)

// Logs will show:
// - "hazard detected" (type, coordinate, severity)
// - "hazard removed" (type, coordinate)
// - "overlay built" (type, count, memory, duration)
// - "overlay updated" (type, added, removed, duration)
```

---

### Check Memory Usage

```go
func logHazardMemory() {
    counts := hazardManager.GetAllCounts()
    memoryKB := hazardManager.GetMemoryUsage() / 1024

    log.WithFields(log.Fields{
        "total_hazards": counts["aquifer"] + counts["water"] + counts["lava"] +
                         counts["caverns"] + counts["enemies"] + counts["dwarves"],
        "memory_kb":     memoryKB,
    }).Debug("hazard memory usage")

    // Alert if over budget
    if memoryKB > 50 {
        log.Warnf("hazard overlays exceed 50 KB budget: %d KB", memoryKB)
    }
}
```

---

### Validate Hazard Counts

```go
func validateHazardCounts(tiles []protocol.TileState) {
    // Rebuild overlays
    hazardManager.BuildFromTiles(tiles)
    counts := hazardManager.GetAllCounts()

    // Manual count (for validation)
    manualAquiferCount := 0
    for _, tile := range tiles {
        if IsAquifer(tile.TileType, tile.Flags) {
            manualAquiferCount++
        }
    }

    // Compare
    if counts["aquifer"] != uint32(manualAquiferCount) {
        log.Errorf("aquifer count mismatch: overlay=%d, manual=%d",
            counts["aquifer"], manualAquiferCount)
    } else {
        log.Info("aquifer detection 100% accurate")
    }
}
```

---

## FAQ

**Q: When should I use hazard overlays vs querying tile data directly?**

A: Use overlays when:
- Querying many coordinates (overlays are O(1) sparse lookups vs O(n) tile array search)
- Checking safety before commands (aquifer/lava detection)
- Tracking entity positions (dwarves/enemies not in tile data)

Query tile data directly when:
- You need full tile details (material, designation, etc.)
- You're already iterating all tiles for another reason

---

**Q: How often are overlays updated?**

A:
- **Environmental hazards** (aquifer, water, lava, caverns): Updated on TILE_UPDATE (rare - only when tiles change)
- **Entities** (enemies, dwarves): Updated on ENTITY_UPDATE (frequent - every tick or every N ticks)

---

**Q: What happens if I query a coordinate outside map bounds?**

A: `Contains()` and `Get()` validate coordinates. Out-of-bounds queries return `false` or `error` respectively (no panic).

---

**Q: Can I add custom hazard types?**

A: Yes! Create a new `HazardOverlay` with `NewHazardOverlay("my_custom_type", bounds)` and manage it alongside the standard overlays. Add custom detector function and wire up updates.

---

**Q: Why are entity overlays empty?**

A: Entity overlays (enemies, dwarves) require `ENTITY_UPDATE` protocol message, which is **not implemented in MVP**. Environmental hazards (aquifer, water, lava, caverns) work with current `TILE_UPDATE` messages. Entity tracking will be added after protocol enhancement.

---

## Next Steps

1. **Implement detector functions** (`IsAquifer`, `IsWater`, etc.) based on DF tiletype enum research
2. **Wire up in main.go** following integration example above
3. **Test with live DFHack** connection - verify hazard counts and memory usage
4. **Add entity support** (requires ENTITY_UPDATE protocol message design)
5. **Benchmark performance** - ensure queries meet <10μs target

See [tasks.md](./tasks.md) (after running `/speckit.tasks`) for detailed implementation checklist.
