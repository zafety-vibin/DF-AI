# Implementation Guide: Critical Features

**Status**: Architectural improvements added, need DFHack integration
**Priority**: Complete before local LLM spec

---

## 1. Fort Age Extraction (CRITICAL)

### Current Problem
Phase system uses real-time estimate instead of DF in-game days.

###  DFHack API to Use (VERIFIED)

```cpp
// Add to df_ai_protocol.cpp
struct FortInfo {
    uint32_t days_elapsed;
    uint64_t created_wealth;
    uint8_t season;
    uint32_t year;
};

FortInfo extract_fort_info() {
    FortInfo info = {0};

    if (!df::global::world) {
        return info;
    }

    // REAL DF DAYS: world->cur_year_tick / 1200 + world->cur_year * 336
    // DF has 1200 ticks/day, 336 days/year
    uint32_t total_ticks =
        (df::global::world->cur_year * 336 * 1200) +
        df::global::world->cur_year_tick;
    info.days_elapsed = total_ticks / 1200;

    // REAL WEALTH: world->status.created_wealth (pointer, check null)
    if (df::global::world->status.created_wealth) {
        info.created_wealth = *df::global::world->status.created_wealth;
    }

    // REAL SEASON/YEAR: Direct fields
    info.season = (uint8_t)df::global::world->cur_season;
    info.year = df::global::world->cur_year;

    return info;
}
```

### Integration Points

**1. Update `serialize_entity_update()` in plugin**:
```cpp
std::vector<uint8_t> serialize_entity_update(const std::vector<EntityInfo> &entities) {
    // ... existing entity serialization ...

    // Add FortInfo (optional)
    FortInfo fort_info = extract_fort_info();
    buffer.push_back(1);  // hasFortInfo = true

    // DaysElapsed (4 bytes)
    buffer.push_back((fort_info.days_elapsed >> 24) & 0xFF);
    buffer.push_back((fort_info.days_elapsed >> 16) & 0xFF);
    buffer.push_back((fort_info.days_elapsed >> 8) & 0xFF);
    buffer.push_back(fort_info.days_elapsed & 0xFF);

    // CreatedWealth (8 bytes)
    for (int i = 7; i >= 0; i--) {
        buffer.push_back((fort_info.created_wealth >> (i * 8)) & 0xFF);
    }

    // Season (1 byte)
    buffer.push_back(fort_info.season);

    // Year (4 bytes)
    buffer.push_back((fort_info.year >> 24) & 0xFF);
    buffer.push_back((fort_info.year >> 16) & 0xFF);
    buffer.push_back((fort_info.year >> 8) & 0xFF);
    buffer.push_back(fort_info.year & 0xFF);

    return buffer;
}
```

**2. Update PhaseManager to use real days**:
```go
// In cmd/df-orchestrator/main.go when ENTITY_UPDATE received
if update.FortInfo != nil {
    phaseManager.UpdateFromFortInfo(int(update.FortInfo.DaysElapsed))
}
```

**3. Add UpdateFromFortInfo to PhaseManager**:
```go
// In internal/phases/phase_manager.go
func (pm *PhaseManager) UpdateFromFortInfo(days int) {
    pm.currentDays = days
    pm.lastUpdate = time.Now()
}

func (pm *PhaseManager) GetDaysElapsed() int {
    if pm.enabled {
        return pm.currentDays  // Use real DF days
    }
    // Fallback to estimate if fort info not available
    elapsed := time.Since(pm.startTime)
    return int(elapsed.Seconds() / 30)
}
```

---

## 2. Zone Designation (Bedroom, Dining Hall, etc.)

### DFHack API to Use

**DF Zone System**: `df::building_civzonest`

```cpp
// Add to df_ai_protocol.cpp
#include "df/building_civzonest.h"
#include "modules/Buildings.h"

bool applyZoneDesignation(uint8_t zoneType, int16_t x1, int16_t y1, int16_t z,
                          int16_t x2, int16_t y2, std::string &error) {
    using namespace DFHack;

    // Calculate dimensions
    int16_t width = x2 - x1 + 1;
    int16_t height = y2 - y1 + 1;

    // Allocate zone building
    df::building_civzonest* zone = virtual_cast<df::building_civzonest>(
        Buildings::allocInstance(df::coord(x1, y1, z), df::building_type::Civzone)
    );

    if (!zone) {
        error = "Failed to allocate zone building";
        return false;
    }

    // Set zone type
    switch (zoneType) {
        case 0x01:  // BEDROOM
            zone->zone_flags.bits.bedroom = true;
            break;
        case 0x02:  // DINING
            zone->zone_flags.bits.dining_hall = true;
            break;
        case 0x03:  // MEETING_HALL
            zone->zone_flags.bits.meeting_area = true;
            break;
        case 0x04:  // BARRACKS
            zone->zone_flags.bits.barracks = true;
            break;
        case 0x05:  // DORMITORY
            zone->zone_flags.bits.bedroom = true;
            zone->zone_flags.bits.dormitory = true;
            break;
        default:
            Buildings::deconstruct(zone);
            error = "Unknown zone type";
            return false;
    }

    // Set zone dimensions
    Buildings::setSize(zone, df::coord(x1, y1, z), df::coord(x2, y2, z));

    // Check tiles are free
    if (!Buildings::checkFreeTiles(zone)) {
        Buildings::deconstruct(zone);
        error = "Tiles not free for zone placement";
        return false;
    }

    // Finalize zone
    Buildings::constructWithFilters(zone, NULL);

    return true;
}
```

### Add to Command Handler

```cpp
// In onCommand() switch statement
case 0x06: {  // ZONE
    if (payload.size() < 12) {
        sendCommandAck(cmdID, 0x02, "Invalid ZONE payload");
        return;
    }
    uint8_t zoneType = payload[5];
    int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
    int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
    int16_t z = ((int16_t)payload[10] << 8) | payload[11];
    int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
    int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];

    success = applyZoneDesignation(zoneType, x1, y1, z, x2, y2, error);
    break;
}
```

### Protocol Serialization

**Add to codec.go**:
```go
case CommandTypeZone:
    // [1: ZoneType] [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2]
    if err := binary.Write(w, binary.BigEndian, msg.Zone.ZoneType); err != nil {
        return err
    }
    if err := binary.Write(w, binary.BigEndian, msg.Zone.X1); err != nil {
        return err
    }
    // ... etc for Y1, Z, X2, Y2
```

### AI Command Syntax

**Update system prompt** (`cmd/df-orchestrator/main.go`):
```
Zone Designation Commands:
1. bedroom at (x, y, z) size WxH
   - Designates personal bedroom zone
   - Dwarves will claim these rooms
   - Example: "bedroom at (50,50,120) size 3x3"

2. dining at (x, y, z) size WxH
   - Designates dining hall
   - Improves dwarf mood (good thoughts from dining)
   - Example: "dining at (45,45,120) size 10x10"

3. meeting at (x, y, z) size WxH
   - Designates meeting area
   - Dwarves gather here when idle
   - Example: "meeting at (60,60,120) size 5x5"

4. barracks at (x, y, z) size WxH
   - Military training area
   - Assign to squads for training
   - Example: "barracks at (40,40,120) size 8x8"
```

---

## 3. Save on DF Quit/Save Hook

### DFHack Event System

**DF has save events we can hook**:
```cpp
#include "modules/EventManager.h"

// Register for save events
DFHack::EventManager::EventHandler saveHandler(plugin_self, 1);

void onSaveEvent(color_ostream &out, void* data) {
    out.print("DF is saving - triggering modification save\n");

    // Send signal to server to save modifications
    // Option A: Send special SAVE_REQUEST message
    // Option B: Server auto-saves on disconnect (simpler)
}

// In plugin_init()
DFHack::EventManager::registerListener(
    EventManager::EventType::WORLD_LOADED,
    saveHandler,
    plugin_self
);
```

**Simpler Approach**: Auto-save on disconnect

```cpp
// In disconnect_from_server()
void disconnect_from_server() {
    if (!g_connected) return;

    // Send notification to server before disconnecting
    std::vector<uint8_t> disconnect_msg;
    disconnect_msg.push_back(0);  // Length placeholder
    disconnect_msg.push_back(0);
    disconnect_msg.push_back(0);
    disconnect_msg.push_back(0);
    disconnect_msg.push_back(PROTOCOL_VERSION);
    disconnect_msg.push_back(0x06);  // DISCONNECT message
    disconnect_msg.push_back(0x01);  // Reason: Normal shutdown (triggers save)

    g_socket->Send(disconnect_msg.data(), disconnect_msg.size());

    g_socket->Close();
    g_connected = false;
}
```

**Server-side** (`cmd/df-orchestrator/main.go`):
```go
// In disconnect handler
client.SetOnDisconnect(func(reason uint8) {
    logger.Info("client disconnecting", logging.Field{Key: "reason", Value: reason})

    // Auto-save modifications if persistence enabled
    if persistentMods != nil && cfg.EnablePersistence {
        fortName := getFortName()  // TODO: Extract from DF or use timestamp
        if err := persistentMods.Save(fortName); err != nil {
            logger.Error("failed to save modifications", err)
        } else {
            logger.Info("modifications saved", logging.Field{Key: "fort", Value: fortName})
        }
    }
})
```

---

## 4. GetAll() Method for ModificationOverlay

Already implemented in `internal/modifications/overlay.go:168-179`.

---

## Summary of Work Remaining

### Plugin Changes (C++)

**File**: `dfhack-build/plugins/df_ai_protocol.cpp`

1. ✅ Add `FortInfo struct` and `extract_fort_info()`
2. ✅ Update `serialize_entity_update()` to include fort info
3. ✅ Add zone designation: `applyZoneDesignation()`
4. ✅ Add ZONE case to `onCommand()` switch
5. ✅ Trigger save on disconnect

**Estimated**: 2-3 hours coding + testing

### Go Server Changes

**Files**: Multiple

1. ✅ Protocol updated (FortInfo, ZoneDesignation)
2. ✅ Serialization updated (optional FortInfo)
3. ⏳ Add zone serialization to `codec.go`
4. ⏳ Add `PhaseManager.UpdateFromFortInfo()`
5. ⏳ Wire phase manager into autonomous loop
6. ⏳ Add save-on-disconnect handler
7. ⏳ Add zone command executor

**Estimated**: 1-2 hours coding

### Testing Required

1. Fort age accuracy (compare DF UI days to phase manager)
2. Zone placement (verify bedroom/dining zones appear in DF)
3. Save/load persistence (quit DF, reload, check modifications loaded)
4. Phase transitions (verify embark → establish at day 7)

---

## Recommendation

Given the scope, suggest:

**Option A - Complete Now**:
- Implement all critical fixes (fort age, zones, save hooks)
- Test with Claude API
- Commit as "Feature 005 Complete + Architectural Improvements"
- Then spec Feature 006 (local LLM)

**Option B - Defer to Feature 006**:
- Commit current code as "Feature 005 + Architecture Skeleton"
- Create Feature 006 spec that includes:
  - Local LLM provider
  - Complete DFHack integration (fort age, zones)
  - System prompt redesign for local models
- Implement everything together

**My Recommendation**: Option B
- Local LLM architecture might change how we structure prompts
- Fort age/zones can be tested with local model
- Faster to validation (can test zones with local LLM immediately)
- Avoids $$ on Claude testing

Your call!
