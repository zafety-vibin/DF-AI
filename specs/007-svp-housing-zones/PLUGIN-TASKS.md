# Plugin Implementation Tasks: Feature 007

**Target**: plugin/df-ai-plugin.cpp (DFHack C++ plugin)
**Status**: Orchestrator-side complete, plugin work pending
**Tasks**: T057-T060

## Overview

The orchestrator is ready to receive zone data and soil layer information from the plugin. These 4 tasks extend the plugin to extract zone data from DF and populate the protocol messages.

---

## T057: Zone Extraction from DF Building Module

**Goal**: Extract zone list from `df.global.world.buildings.other.ZONE` and add to EntityUpdate message.

**Location**: ENTITY_UPDATE message handler in plugin

**Implementation**:

```cpp
// In ENTITY_UPDATE handler (where entities are extracted)

// After extracting entities, extract zones
for (auto* zone : df::global::world->buildings.other.ZONE) {
    if (zone == nullptr) continue;

    // Create ZoneData protobuf message
    ZoneData* zoneData = entityUpdate.add_zones();

    zoneData->set_zone_id(zone->id);
    zoneData->set_zone_type(MapZoneType(zone->type)); // T058
    zoneData->set_x1(zone->x1);
    zoneData->set_y1(zone->y1);
    zoneData->set_z1(zone->z1);
    zoneData->set_x2(zone->x2);
    zoneData->set_y2(zone->y2);
    zoneData->set_z2(zone->z2);
    zoneData->set_assigned_to(getAssignedDwarfID(zone)); // T059
}
```

**Expected Output**: EntityUpdate message includes zones array with all DF zones

---

## T058: MapZoneType Helper

**Goal**: Convert DF zone type enum to protocol ZoneType uint8 constants.

**Implementation**:

```cpp
uint8_t MapZoneType(df::building_type zoneType) {
    switch (zoneType) {
        case df::building_type::Bed:
            return 0x01; // ZoneTypeBedroom
        case df::building_type::Table:
            return 0x02; // ZoneTypeDining
        case df::building_type::Chair:
            return 0x03; // ZoneTypeMeetingHall
        case df::building_type::Weapon:
        case df::building_type::Armor:
            return 0x04; // ZoneTypeBarracks
        // TODO: Map all relevant DF zone types
        default:
            return 0x01; // Default to bedroom
    }
}
```

**Protocol Constants** (from internal/protocol/message.go):
```
ZoneTypeBedroom     = 0x01
ZoneTypeDining      = 0x02
ZoneTypeMeetingHall = 0x03
ZoneTypeBarracks    = 0x04
ZoneTypeDormitory   = 0x05
ZoneTypeOffice      = 0x06
ZoneTypeWorkshop    = 0x07
ZoneTypeStockpile   = 0x08
```

**Expected Output**: Correct uint8 zone type for orchestrator

---

## T059: getAssignedDwarfID Helper

**Goal**: Extract owner dwarf ID from zone, return -1 if unassigned.

**Implementation**:

```cpp
int32_t getAssignedDwarfID(df::building* zone) {
    // Check if zone has assigned owner
    if (zone->owner != nullptr) {
        return zone->owner->id;
    }

    // Alternate: Check specific_refs for assigned unit
    for (auto& ref : zone->specific_refs) {
        if (ref->getType() == df::specific_ref_type::UNIT) {
            auto unit_ref = static_cast<df::specific_ref_unitst*>(ref);
            if (unit_ref->unit != nullptr) {
                return unit_ref->unit->id;
            }
        }
    }

    return -1; // Unassigned
}
```

**Expected Output**: Dwarf ID (positive int32) or -1 if unassigned

---

## T060: Soil Layer Extraction

**Goal**: Query tiletype_material per Z-level and populate IsSoilLayer array in FullStateMessage.

**Location**: RESYNC/FullState message handler

**Implementation**:

```cpp
// In FullStateMessage handler (after extracting tile types)

// Extract soil flags per Z-level (200 Z-levels max)
for (int z = 0; z < 200; z++) {
    bool hasSoil = CheckZLevelForSoil(z);
    fullState.add_is_soil_layer(hasSoil);
}

// Helper function
bool CheckZLevelForSoil(int z) {
    // Check if any tiles at this Z-level have soil material
    // Iterate tiles at Z-level and check tiletype_material
    for (int x = 0; x < map_width; x++) {
        for (int y = 0; y < map_height; y++) {
            df::tiletype tileType = getTileType(x, y, z);
            df::tiletype_material material = tileTypeMaterial(tileType);

            if (material == df::tiletype_material::SOIL ||
                material == df::tiletype_material::GRASS_DARK ||
                material == df::tiletype_material::GRASS_LIGHT) {
                return true; // This Z-level has soil
            }
        }
    }
    return false; // No soil at this Z-level
}
```

**Expected Output**: IsSoilLayer array with 200 bools (one per Z-level)

---

## Testing Plugin Changes

**After implementing T057-T060**:

1. Build plugin with DFHack
2. Load in DF with active fort
3. Connect orchestrator (`ai-connect` command)
4. Check orchestrator logs for:
   - "zones cached from entity update" with zone_count > 0
   - "SVP: Detected soil layers" with z_levels array
5. Verify saves/{fortname}/svp_layout.json contains farm_z with actual soil layers
6. Create bedroom zone in DF (z menu)
7. Check orchestrator logs for BedroomZoneCount = 1

---

## Protocol Message Examples

**EntityUpdate with Zones** (what plugin sends):
```protobuf
EntityUpdate {
    count: 7
    entities: [dwarf1, dwarf2, ...]
    fort_info: {days_elapsed: 10, ...}
    zones: [
        {zone_id: 1, zone_type: 0x01, x1: 60, y1: 30, z1: 120, x2: 63, y2: 33, z2: 120, assigned_to: 42},
        {zone_id: 2, zone_type: 0x01, x1: 64, y1: 30, z1: 120, x2: 67, y2: 33, z2: 120, assigned_to: -1},
    ]
}
```

**FullStateMessage with Soil** (what plugin sends):
```protobuf
FullStateMessage {
    width: 96
    height: 96
    depth: 200
    tiles: [...]
    is_soil_layer: [false, false, ..., true, true, false, ...]  // 200 bools
}
```

---

## Validation

After plugin implementation:
- ✅ Zone count matches DF UI (z menu shows N bedrooms, orchestrator logs N bedroom zones)
- ✅ Assignment status correct (assigned bedrooms show dwarf ID, unassigned show -1)
- ✅ Soil layers detected (orchestrator logs "farm_z: [124, 125]" for embarks with surface soil)
- ✅ SVP uses soil data (proposes farms only at soil Z-levels)

---

## Priority

**Critical for MVP**: T057-T060 must be complete for full Feature 007 testing
**Current Workaround**: Orchestrator works with empty zone arrays (graceful degradation)
**Impact**: Without plugin work, HousingAgent uses placeholder data (same as Feature 006)

**Recommendation**: Implement T057-T060 in plugin to enable full Feature 007 functionality
