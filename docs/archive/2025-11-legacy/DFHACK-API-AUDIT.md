# DFHack API Usage Audit

**Purpose**: Document what data we're ACTUALLY extracting from DF vs placeholder/TODO fields
**Last Updated**: 2025-11-08

---

## ✅ VERIFIED - Actually Extracted from DF

### Entity Data (`extract_entities()`)
**Source**: `df::global::world->units.active`

| Field | DFHack API | Status |
|-------|------------|--------|
| Entity ID | `unit->id` | ✅ REAL |
| Position X | `unit->pos.x` | ✅ REAL |
| Position Y | `unit->pos.y` | ✅ REAL |
| Position Z | `unit->pos.z` | ✅ REAL |
| Race/Species | `unit->race` | ✅ REAL |
| Type Classification | `unit->flags1.bits.tame`, `unit->flags2.bits.resident`, `unit->flags1.bits.marauder` | ✅ REAL |

**Verified Working**: Entity tracking tested Nov 8, shows "18 dwarves" correctly

### Topology Data (`extract_topology()`)
**Source**: `Maps::getTileBlock()` + `Maps::getTileType()`

| Field | DFHack API | Status |
|-------|------------|--------|
| Tile Type | `Maps::getTileType(block, x%16, y%16)` | ✅ REAL |
| Wall vs Open | Converted to 0/1 bit | ✅ REAL |
| Map Dimensions | `world->map.x_count`, `world->map.y_count`, `world->map.z_count` | ✅ REAL |

**Verified Working**: Topology overlay built, 172 KB, 67.5% open tiles

### Hazard Detection (Inferred from Topology)
**Source**: TileType analysis

| Hazard Type | Detection Method | Status |
|-------------|------------------|--------|
| Water | `tiletype == Brook`, `RiverSource`, `MurkyPool` | ✅ REAL |
| Lava | `tiletype == Magma*` | ✅ REAL |
| Aquifer | `tiletype == *AquiferLight/Heavy` | ✅ REAL |
| Caverns | `tiletype == CavernFloor` + Z-filter (10-90) | ✅ REAL |

**Verified Working**: Hazard counts match DF game state

### Dig Designation Execution
**Source**: `MapExtras::MapCache` + ` df::tile_designation`

| Operation | DFHack API | Status |
|-----------|------------|--------|
| Read designation | `cache.designationAt(pos)` | ✅ REAL |
| Set dig type | `des.bits.dig = tile_dig_designation::*` | ✅ REAL |
| Write to DF | `cache.setDesignationAt(pos, des)` + `cache.WriteAll()` | ✅ REAL |

**Verified Working**: Tested Nov 8, dwarves mine designated tiles, "inappropriate dig square" shows DF received designation

---

## ⚠️ TODO - Placeholder Fields (Not Yet Implemented)

### Enhanced Dwarf Data (DwarfDetails)

| Field | Required DFHack API | Implementation Effort |
|-------|---------------------|----------------------|
| Name | `unit->name.first_name` + `unit->name.nickname` | **Easy** - direct field access |
| Profession | `unit->profession` or `Units::getProfessionName(unit)` | **Easy** - helper exists |
| Skills | `unit->status.current_soul->skills` | **Medium** - iterate skill list |
| Mood | `unit->status.happiness` | **Medium** - need to normalize to -10/+10 |
| Current Task | `unit->job.current_job` | **Hard** - complex job system |
| Idle Time | Track via position changes | **Hard** - needs state tracking |

**Recommendation**: Implement Name + Profession first (easy wins), defer Skills/Mood until needed.

### Fort Statistics (FortStatistics)

| Field | Required DFHack API | Implementation Effort |
|-------|---------------------|----------------------|
| Wealth | `world->status.wealth` (created_wealth) | **Easy** - direct field |
| Food Stocks | Count items in food stockpiles | **Medium** - iterate stockpiles + items |
| Drink Stocks | Count barrels in drink stockpiles | **Medium** - same as food |
| Production Rate | Track item creation over time | **Hard** - needs historical tracking |

**Recommendation**: Implement Wealth first (single field read), defer inventory tracking.

### Days Elapsed (Fort Age)

| Field | Required DFHack API | Implementation Effort |
|-------|---------------------|----------------------|
| In-Game Days | `world->cur_year_tick / 1200` + `world->cur_year * 336` | **Easy** - simple calculation |
| Season | `world->cur_season` | **Easy** - enum value |
| Year | `world->cur_year` | **Easy** - direct field |

**Current**: Using real-time estimate (30s ≈ 1 day)
**Recommendation**: **Replace immediately** - this is critical for phase system accuracy!

---

## 🚨 CRITICAL FIX NEEDED

### Phase System is Using WRONG Time Source

**Current Code** (`phases/phase_manager.go:68`):
```go
func (pm *PhaseManager) GetDaysElapsed() int {
    elapsed := time.Since(pm.startTime)
    estimatedDays := int(elapsed.Seconds() / 30)  // WRONG!
    return estimatedDays
}
```

**Problem**: Uses real-time, not in-game time!
- Player pauses game → AI thinks fort is aging
- Player plays at 100 FPS → AI thinks days pass too slowly

**Fix Required**:
```cpp
// Add to df_ai_protocol.cpp
uint32_t get_fort_age_days() {
    if (!df::global::world) return 0;

    // DF ticks: 1200 ticks/day, 336 days/year
    uint32_t total_ticks =
        (df::global::world->cur_year * 336 * 1200) +
        df::global::world->cur_year_tick;

    return total_ticks / 1200;
}

// Add to ENTITY_UPDATE message or create new FORT_INFO message
struct FortInfo {
    uint32_t days_elapsed;
    uint32_t created_wealth;
    uint16_t dwarf_count;
    uint16_t enemy_count;
};
```

**Priority**: **HIGH** - Phase system is foundation for new architecture

---

## 📋 Implementation Plan for Missing APIs

### Phase 1: Critical (Do First)
1. **Fort Age** - Add to ENTITY_UPDATE or create FORT_INFO message
2. **Dwarf Names** - Easy win, helps debugging
3. **Wealth** - Single field read, useful for metrics

### Phase 2: Quality of Life
4. **Profession** - Helps understand dwarf roles
5. **Food/Drink Stocks** - Survival metrics
6. **Season/Year** - Context for AI decisions

### Phase 3: Advanced (Later)
7. **Skills** - Complex but useful for labor optimization
8. **Mood** - Tantrum detection
9. **Current Task** - Job system integration

---

## Code Changes Needed

### 1. Add Fort Info to Protocol (`protocol/message.go`)

```go
// Add to ENTITY_UPDATE or create separate message
type FortInfo struct {
    DaysElapsed    uint32
    CreatedWealth  uint64
    Season         uint8  // 0=Spring, 1=Summer, 2=Autumn, 3=Winter
    Year           uint32
}

type EntityUpdateMessage struct {
    Count    uint32
    Entities []EntityInfo
    FortInfo *FortInfo  // NEW
}
```

### 2. Extract in Plugin (`df_ai_protocol.cpp`)

```cpp
FortInfo extract_fort_info() {
    FortInfo info = {0};

    if (df::global::world) {
        // Calculate days
        uint32_t total_ticks =
            (df::global::world->cur_year * 336 * 1200) +
            df::global::world->cur_year_tick;
        info.days_elapsed = total_ticks / 1200;

        // Get wealth
        if (df::global::world->status.created_wealth) {
            info.created_wealth = *df::global::world->status.created_wealth;
        }

        // Get season/year
        info.season = df::global::world->cur_season;
        info.year = df::global::world->cur_year;
    }

    return info;
}

// Update serialize_entity_update() to include fort info
```

### 3. Update Phase Manager

```go
type PhaseManager struct {
    startTime time.Time
    config    PhaseConfig
    enabled   bool

    // NEW: Track actual DF days
    currentDays int
    lastUpdate  time.Time
}

// Update from fort info instead of estimating
func (pm *PhaseManager) UpdateFromFortInfo(days int) {
    pm.currentDays = days
    pm.lastUpdate = time.Now()
}

func (pm *PhaseManager) GetDaysElapsed() int {
    return pm.currentDays  // Use actual DF days, not estimate
}
```

---

## Recommendation

**Before continuing with refactoring**, fix the fort age extraction:

1. ✅ Add `FortInfo` to `ENTITY_UPDATE` message (expand existing message)
2. ✅ Implement `extract_fort_info()` in plugin
3. ✅ Update serialization to include fort info
4. ✅ Update `PhaseManager` to use real days
5. ✅ Test phase transitions with actual DF time

This ensures the phase system works correctly before building on top of it.

**Alternative**: If adding to ENTITY_UPDATE is complex, create separate `FORT_INFO (0x0B)` message sent every 100s.
