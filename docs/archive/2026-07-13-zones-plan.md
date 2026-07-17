# Zones Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the permanently-failing `zone` stub with real civzone tooling — designate a zone, assign/unassign a unit, list zones, see zones painted in `look` — closing Feature 009's highest-priority capability gap (First Fort built 7 beds with zero room-assignment step).

**Architecture:** A corrected wire-level zone-type enum (matching real DFHack `civzone_type` values, not the previous guessed/legacy ones) flows through a new `dfhack-plugin/zones.cpp` (creation via the same `allocInstance→setSize→constructAbstract` sequence already used for stockpiles; assignment via `Buildings::setOwner` for 4 confirmed owner-type kinds and a roster mechanism for 2 confirmed membership-type kinds, with honest "not implemented" errors for the rest) into four new Go MCP tools. Zones are targeted by tile coordinate (matching `remove_building`/`unsuspend`'s existing convention), not a synthetic ID. `WorldModel.Zones` — already fully wired on the Go side — goes live once the wire actually carries zone data in the periodic `ENTITY_UPDATE` refresh.

**Tech Stack:** Go 1.25 (`internal/protocol`, `internal/commands`, `internal/mcpserver`, `internal/predicate`), C++20 DFHack plugin (`dfhack-plugin/`), existing binary TCP protocol.

## Global Constraints

- Design of record: `specs/009-culture-and-learning/design-zones.md` — every task below implements a section of it; do not deviate without updating that doc first. Read Component 3 in full before Task 2 — it is the load-bearing correction of this whole plan (assignment only works for 6 of 18 zone types; the design doc explains exactly why).
- C++: DFHack `CHECK_*` macros THROW — every new DF-touching path must be reachable only inside `executeCommand`'s or `executeQuery`'s existing try/catch (`dfhack-plugin/df_ai_protocol.cpp:625-852`, `dfhack-plugin/queries.cpp:781-812`).
- Go: stdout is the MCP transport in `cmd/df-mcp` paths — logging only via `logging.NewStderrTextLogger`. Every MCP tool must return readable text on a nil/disconnected bridge (`TestEveryToolNilBridge` in `internal/mcpserver/server_test.go` calls every registered tool against a nil bridge; tools with required inputs need an entry in `minToolArgs`).
- Wire protocol changes are additive-optional **except** the `ZoneType*` value table itself (`internal/protocol/message.go`, `dfhack-plugin/protocol.h`) — that field has never carried real data (the tool has always been a hard-fail stub), so replacing its values wholesale is a deliberate one-time correction, not a violation of the usual additive-only rule. Every other change in this plan (new command types, new struct fields, the `Zones` block appended to `ENTITY_UPDATE`) IS additive-optional: an old decoder simply stops reading before the new bytes and is unaffected.
- Ship a unit test for every pure helper (project house rule). This project has no C++ unit-test harness — plugin-side verification is compile-check plus the live-verification checkpoint in the final task (established precedent, Perception round 2 Task 4/11).
- Repo has `core.autocrlf=true` and no `.gitattributes` — avoid bulk/sed-style rewrites that would churn line endings.
- Plugin rebuild command: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`. Artifact: `C:/Users/zmanl/Projects/dfhack-build/build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll`. Deploy requires DF closed (file lock) — never deploy from within this plan; report the artifact path for the user to deploy when ready.
- DFHack 53.15-r1 reference checkout: `C:\Users\zmanl\Projects\dfhack-build` (read-only reference; `plugins/df_ai_protocol` there is a junction to this repo's `dfhack-plugin/` — never edit through the junction).
- Do not touch `internal/zones/`, `cmd/df-orchestrator/`, or `internal/autonomous/` — all three are legacy code pending a separate Task 16 cleanup, explicitly out of scope here (design doc Component 8).
- Do not implement Barracks assignment (squad-based, a different tool shape) or assignment for the 11 unconfirmed-mechanism zone types (MeetingHall, Dormitory, ArcheryRange, PlantGathering, WaterSource, Dump, SandCollection, FishingArea, ClayCollection, Dungeon, AnimalTraining) — these get creation and listing, and `assign_zone` returns a clear, specific error naming why, never a guess.

---

## The corrected wire-level zone-type table (reference for every task below)

| Wire byte | Type name | Real `df::civzone_type` value | Assignment mechanism |
|---|---|---|---|
| 0x01 | Bedroom | 92 | Owner |
| 0x02 | Office | 93 | Owner |
| 0x03 | Tomb | 97 | Owner |
| 0x04 | DiningHall | 80 | Owner |
| 0x05 | MeetingHall | 87 | Unconfirmed |
| 0x06 | Dormitory | 79 | Unconfirmed |
| 0x07 | Barracks | 95 | Squad (out of scope) |
| 0x08 | Pen | 88 | Roster |
| 0x09 | Pond | 86 | Roster |
| 0x0A | ArcheryRange | 94 | Unconfirmed |
| 0x0B | PlantGathering | 91 | Unconfirmed |
| 0x0C | WaterSource | 82 | Unconfirmed |
| 0x0D | Dump | 83 | Unconfirmed |
| 0x0E | SandCollection | 84 | Unconfirmed |
| 0x0F | FishingArea | 85 | Unconfirmed |
| 0x10 | ClayCollection | 89 | Unconfirmed |
| 0x11 | Dungeon | 96 | Unconfirmed |
| 0x12 | AnimalTraining | 90 | Unconfirmed |

---

## File structure

New files:
- `dfhack-plugin/zones.cpp` — `civzoneTypeFromWire`, `wireFromCivzoneType`, `applyDesignateZone`, `applyAssignZone`, `applyUnassignZone`.
- `internal/mcpserver/tools_zone.go` — `renderZones`, `registerZoneTools` (the 4 zone MCP tools).
- `internal/mcpserver/tools_zone_test.go` — `renderZones` summary-grouping tests, `assign_zone`/`unassign_zone` error-text tests.
- `internal/mcpserver/lenses_zones_test.go` — is NOT created; the zones lens test is added to the existing `internal/mcpserver/lenses_test.go`.

Modified files:
- `dfhack-plugin/protocol.h` — `ZONE_TYPE_*` block replaced with the corrected table above; two new `COMMAND_TYPE_*` constants (`ASSIGN_ZONE=0x10`, `UNASSIGN_ZONE=0x11`).
- `dfhack-plugin/df_ai_protocol.cpp` — `applyZoneDesignation` forward declaration/stub removed (replaced by `zones.cpp`'s real implementation); `case 0x06`'s payload bounds-check bug fixed; two new `case` blocks for `COMMAND_TYPE_ASSIGN_ZONE`/`COMMAND_TYPE_UNASSIGN_ZONE`.
- `dfhack-plugin/queries.cpp` — new `handleListZones`, wired into `executeQuery`'s dispatch chain.
- `dfhack-plugin/entities.cpp` — `serialize_entity_update` gains a zones block appended after the existing `FortInfo` block.
- `internal/protocol/message.go` — `ZoneType*` constants corrected to the table above; `ZoneData` struct gains `OwnerUnitID int32`/`AssignedUnits []int32`, `ZoneID` field's doc comment corrected; two new `CommandType*` constants; two new command-payload structs (`AssignZoneDesignation`, `UnassignZoneDesignation`).
- `internal/protocol/codec.go` — encode/decode for the two new commands; encode/decode for `EntityUpdateMessage.Zones` (currently dead wire-wise despite the struct existing).
- `internal/commands/executor.go` — `SendAssignZone`, `SendUnassignZone` (new); `SendZoneCommand` unchanged in shape (still designate-only).
- `internal/mcpserver/tools_action.go` — the entire non-functional `zone` tool block and `zoneTypes` map removed (superseded by `tools_zone.go`).
- `internal/mcpserver/tools_action_test.go` — the `zone` entry removed from `TestActionToolsNilBridge`'s `calls` table.
- `internal/mcpserver/server.go` — `registerZoneTools(srv, bridge)` added to `New`.
- `internal/mcpserver/server_test.go` — `minToolArgs["zone"]` replaced with entries for `designate_zone`/`assign_zone`/`unassign_zone`/`list_zones`.
- `internal/predicate/library.go` — `HasBedroomZones`/`HasDiningHall` use the corrected wire values from `protocol.ZoneType*` instead of local hardcoded magic numbers.
- `internal/mcpserver/lenses.go` — new `"zones"` entry in the `lenses` map, `gatherZonesLens`, `zoneCategoryGlyph`; `lensGlyphSet` gains a `"zones"` case.

---

## Task 1: Plugin — corrected civzone enum + zone creation

**Files:**
- Create: `dfhack-plugin/zones.cpp`
- Modify: `dfhack-plugin/protocol.h:86-104` (the `ZONE_TYPE_*` block)
- Modify: `dfhack-plugin/df_ai_protocol.cpp:59` (forward declaration), `:280-306` (remove the stub — the real implementation moves to `zones.cpp`), `:669-682` (the `case 0x06` dispatch block — fix the payload-size bug while touching this code)
- Modify: `dfhack-plugin/CMakeLists.custom.txt` (or wherever `dfhack-plugin`'s source file list lives — add `zones.cpp`; check the existing file first, following the same pattern `buildings.cpp`/`entities.cpp` are already listed under)

**Interfaces:**
- Consumes: `Buildings::allocInstance`, `Buildings::setSize`, `Buildings::constructAbstract` (all already used identically in `buildings.cpp:354-387`'s `placeStockpile`), `df::civzone_type` (checkout enum), `Maps::isValidTilePos`.
- Produces: `uint8_t wireFromCivzoneType(df::civzone_type t)` (returns 0 if not in the table — used by `zones.cpp`/`entities.cpp`/`queries.cpp`), `df::civzone_type civzoneTypeFromWire(uint8_t wire)` (returns `df::civzone_type::NONE` if unknown), `bool applyDesignateZone(uint8_t zoneType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error)`.

- [ ] **Step 1: Replace the `ZONE_TYPE_*` block in `protocol.h`**

Find the current block (lines 86-104, from Task 1's research):
```cpp
// Zone types — kept in sync with internal/protocol/message.go.
// 0x06–0x08 are LEGACY values (Office, Workshop, Stockpile) from the
// Feature 007 layout system; do not emit them from new code. Real DF
// civzone categories live at 0x10+.
constexpr uint8_t ZONE_TYPE_BEDROOM        = 0x01;
constexpr uint8_t ZONE_TYPE_DINING         = 0x02;
constexpr uint8_t ZONE_TYPE_MEETING        = 0x03;
constexpr uint8_t ZONE_TYPE_BARRACKS       = 0x04;
constexpr uint8_t ZONE_TYPE_DORMITORY      = 0x05;

constexpr uint8_t ZONE_TYPE_FARM           = 0x10;
constexpr uint8_t ZONE_TYPE_PEN            = 0x11;
constexpr uint8_t ZONE_TYPE_GARBAGE_DUMP   = 0x12;
constexpr uint8_t ZONE_TYPE_PIT_POND       = 0x13;
constexpr uint8_t ZONE_TYPE_WATER_SOURCE   = 0x14;
constexpr uint8_t ZONE_TYPE_FISHING        = 0x15;
constexpr uint8_t ZONE_TYPE_HOSPITAL       = 0x16;
constexpr uint8_t ZONE_TYPE_ANIMAL_TRAIN   = 0x17;
constexpr uint8_t ZONE_TYPE_TOMB           = 0x18;
```

Replace it entirely with:
```cpp
// Zone types — kept in sync with internal/protocol/message.go. These are
// DF-AI's OWN wire values, not DFHack's df::civzone_type values directly
// (those are scattered 79-97, not a small sequential range — see
// zones.cpp's civzoneTypeFromWire/wireFromCivzoneType for the real
// translation table, confirmed against library/include/df/civzone_type.h
// in the DFHack 53.15-r1 checkout). This insulates the wire format from
// DFHack renumbering and keeps command payloads compact.
//
// Assignment support (see zones.cpp::applyAssignZone): Owner-type
// (Bedroom/Office/Tomb/DiningHall) and Roster-type (Pen/Pond) are the
// only 6 with a confirmed DFHack assignment mechanism. Barracks uses a
// separate squad-based mechanism, out of scope for assign_zone. The rest
// have no confirmed mechanism in the DFHack source at all — assign_zone
// returns an explicit "not implemented" error for them, never a guess.
constexpr uint8_t ZONE_TYPE_BEDROOM         = 0x01; // Owner
constexpr uint8_t ZONE_TYPE_OFFICE          = 0x02; // Owner
constexpr uint8_t ZONE_TYPE_TOMB            = 0x03; // Owner
constexpr uint8_t ZONE_TYPE_DINING_HALL     = 0x04; // Owner
constexpr uint8_t ZONE_TYPE_MEETING_HALL    = 0x05; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DORMITORY       = 0x06; // Unconfirmed
constexpr uint8_t ZONE_TYPE_BARRACKS        = 0x07; // Squad (out of scope)
constexpr uint8_t ZONE_TYPE_PEN             = 0x08; // Roster
constexpr uint8_t ZONE_TYPE_POND            = 0x09; // Roster
constexpr uint8_t ZONE_TYPE_ARCHERY_RANGE   = 0x0A; // Unconfirmed
constexpr uint8_t ZONE_TYPE_PLANT_GATHERING = 0x0B; // Unconfirmed
constexpr uint8_t ZONE_TYPE_WATER_SOURCE    = 0x0C; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DUMP            = 0x0D; // Unconfirmed
constexpr uint8_t ZONE_TYPE_SAND_COLLECTION = 0x0E; // Unconfirmed
constexpr uint8_t ZONE_TYPE_FISHING_AREA    = 0x0F; // Unconfirmed
constexpr uint8_t ZONE_TYPE_CLAY_COLLECTION = 0x10; // Unconfirmed
constexpr uint8_t ZONE_TYPE_DUNGEON         = 0x11; // Unconfirmed
constexpr uint8_t ZONE_TYPE_ANIMAL_TRAINING = 0x12; // Unconfirmed
```

Also add the two new command types immediately after `COMMAND_TYPE_SET_LABOR` (currently the last entry at `0x0F`, `protocol.h:75`):
```cpp
constexpr uint8_t COMMAND_TYPE_ASSIGN_ZONE   = 0x10;
constexpr uint8_t COMMAND_TYPE_UNASSIGN_ZONE = 0x11;
```

- [ ] **Step 2: Remove the old stub from `df_ai_protocol.cpp`**

Delete the forward declaration at line 59 (`bool applyZoneDesignation(...)`) and the complete stub function at lines 280-306. `zones.cpp` (Step 3 below) provides the real implementation with the same signature, so no other change is needed at the call site yet (Step 4 handles the dispatch bug fix).

- [ ] **Step 3: Write `zones.cpp` — translation functions + zone creation**

```cpp
// dfhack-plugin/zones.cpp
//
// Civzone (DF "zone") creation and the wire<->DFHack type translation.
// Assignment (applyAssignZone/applyUnassignZone) is a separate task —
// see the plan's Task 2 — added to this same file.
//
// Wire-value table confirmed against library/include/df/civzone_type.h
// in the DFHack 53.15-r1 checkout (C:\Users\zmanl\Projects\dfhack-build).
// The previous stub's comment claimed DFHack 53.12's civzone_type had
// "NO direct Bedroom/Dining/Barracks values" -- that claim does not hold
// for 53.15-r1 (all three exist, at civzone_type values 92/80/95
// respectively); this file's table was independently re-verified against
// the actual checkout, not carried forward from that comment.

#include "Core.h"
#include "Console.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_civzonest.h"
#include "df/civzone_type.h"
#include "df/coord.h"

#include "protocol.h"

#include <string>

using namespace DFHack;

// destroyUnlinked is declared static in buildings.cpp and not exported;
// zones.cpp needs its own copy of the same failed-allocation cleanup
// (room.extents may have been allocated during tile validation and the
// destructor does not free it -- see buildings.cpp:167-180 for the
// original, identical reasoning).
static void destroyUnlinkedZone(df::building *bld) {
    if (bld->room.extents) {
        delete[] bld->room.extents;
        bld->room.extents = NULL;
    }
    delete bld;
}

df::civzone_type civzoneTypeFromWire(uint8_t wire) {
    switch (wire) {
        case ZONE_TYPE_BEDROOM:         return df::civzone_type::Bedroom;
        case ZONE_TYPE_OFFICE:          return df::civzone_type::Office;
        case ZONE_TYPE_TOMB:            return df::civzone_type::Tomb;
        case ZONE_TYPE_DINING_HALL:     return df::civzone_type::DiningHall;
        case ZONE_TYPE_MEETING_HALL:    return df::civzone_type::MeetingHall;
        case ZONE_TYPE_DORMITORY:       return df::civzone_type::Dormitory;
        case ZONE_TYPE_BARRACKS:        return df::civzone_type::Barracks;
        case ZONE_TYPE_PEN:             return df::civzone_type::Pen;
        case ZONE_TYPE_POND:            return df::civzone_type::Pond;
        case ZONE_TYPE_ARCHERY_RANGE:   return df::civzone_type::ArcheryRange;
        case ZONE_TYPE_PLANT_GATHERING: return df::civzone_type::PlantGathering;
        case ZONE_TYPE_WATER_SOURCE:    return df::civzone_type::WaterSource;
        case ZONE_TYPE_DUMP:            return df::civzone_type::Dump;
        case ZONE_TYPE_SAND_COLLECTION: return df::civzone_type::SandCollection;
        case ZONE_TYPE_FISHING_AREA:    return df::civzone_type::FishingArea;
        case ZONE_TYPE_CLAY_COLLECTION: return df::civzone_type::ClayCollection;
        case ZONE_TYPE_DUNGEON:         return df::civzone_type::Dungeon;
        case ZONE_TYPE_ANIMAL_TRAINING: return df::civzone_type::AnimalTraining;
        default:                        return df::civzone_type::NONE;
    }
}

uint8_t wireFromCivzoneType(df::civzone_type t) {
    switch (t) {
        case df::civzone_type::Bedroom:         return ZONE_TYPE_BEDROOM;
        case df::civzone_type::Office:          return ZONE_TYPE_OFFICE;
        case df::civzone_type::Tomb:             return ZONE_TYPE_TOMB;
        case df::civzone_type::DiningHall:       return ZONE_TYPE_DINING_HALL;
        case df::civzone_type::MeetingHall:      return ZONE_TYPE_MEETING_HALL;
        case df::civzone_type::Dormitory:        return ZONE_TYPE_DORMITORY;
        case df::civzone_type::Barracks:         return ZONE_TYPE_BARRACKS;
        case df::civzone_type::Pen:              return ZONE_TYPE_PEN;
        case df::civzone_type::Pond:             return ZONE_TYPE_POND;
        case df::civzone_type::ArcheryRange:     return ZONE_TYPE_ARCHERY_RANGE;
        case df::civzone_type::PlantGathering:   return ZONE_TYPE_PLANT_GATHERING;
        case df::civzone_type::WaterSource:      return ZONE_TYPE_WATER_SOURCE;
        case df::civzone_type::Dump:             return ZONE_TYPE_DUMP;
        case df::civzone_type::SandCollection:   return ZONE_TYPE_SAND_COLLECTION;
        case df::civzone_type::FishingArea:      return ZONE_TYPE_FISHING_AREA;
        case df::civzone_type::ClayCollection:   return ZONE_TYPE_CLAY_COLLECTION;
        case df::civzone_type::Dungeon:          return ZONE_TYPE_DUNGEON;
        case df::civzone_type::AnimalTraining:   return ZONE_TYPE_ANIMAL_TRAINING;
        default:                                  return 0; // unmapped -- caller must treat 0 as "not representable on the wire"
    }
}

// applyDesignateZone creates a new civzone over [x1,y1]-[x2,y2] at z,
// mirroring buildings.cpp's placeStockpile exactly -- civzones and
// stockpiles are both ABSTRACT buildings and use the identical
// allocInstance -> setSize -> constructAbstract sequence (see
// buildings.cpp's file-header comment, section 3b).
bool applyDesignateZone(uint8_t zoneType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error)
{
    df::civzone_type czType = civzoneTypeFromWire(zoneType);
    if (czType == df::civzone_type::NONE) {
        error = "unknown zone type byte " + std::to_string((int)zoneType);
        return false;
    }
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z)) {
        error = "coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1) {
        error = "invalid region (x2<x1 or y2<y1)";
        return false;
    }

    // A civzone claims existing floor space -- it doesn't dig. Reject any
    // tile that isn't walkable floor, naming the first failing tile
    // rather than a generic construction failure.
    for (int16_t yy = y1; yy <= y2; yy++) {
        for (int16_t xx = x1; xx <= x2; xx++) {
            df::tiletype *tt = Maps::getTileType(xx, yy, z);
            if (!tt || DFHack::Maps::isTileVoid(*tt)) {
                error = "tile (" + std::to_string(xx) + "," + std::to_string(yy) +
                        "," + std::to_string(z) + ") is not carved floor -- zones claim existing space, they don't dig";
                return false;
            }
        }
    }

    df::coord pos(x1, y1, z);
    df::building* bld = Buildings::allocInstance(pos, df::building_type::Civzone, (int)czType);
    if (!bld) {
        error = "Buildings::allocInstance returned null for zone";
        return false;
    }

    int width  = (x2 - x1) + 1;
    int height = (y2 - y1) + 1;
    df::coord2d size((int16_t)width, (int16_t)height);
    if (!Buildings::setSize(bld, size)) {
        error = "Buildings::setSize failed for zone (no usable tiles)";
        destroyUnlinkedZone(bld);
        return false;
    }

    if (!Buildings::constructAbstract(bld)) {
        error = "Buildings::constructAbstract failed for zone";
        destroyUnlinkedZone(bld);
        return false;
    }
    return true;
}
```

(Step's note for the implementer: `Maps::isTileVoid`/`Maps::getTileType` signatures should be confirmed against the checkout's `modules/Maps.h` before compiling — if the exact helper name differs, use whatever this plugin's other floor-validation code already calls; `designations.cpp`'s dig-designation validation is the nearest precedent in this same codebase for "is this tile solid/floor/hidden.")

- [ ] **Step 4: Wire `zones.cpp` into the build and fix the `case 0x06` dispatch bug**

Add `zones.cpp` to the plugin's source file list (check `dfhack-plugin/CMakeLists.custom.txt` for exactly how `buildings.cpp`/`entities.cpp` are listed and add the new file the same way).

In `df_ai_protocol.cpp`, `case 0x06:` currently reads (lines 669-682):
```cpp
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

The bounds check (`payload.size() < 12`) is wrong — the code reads through index 15, which requires 16 bytes, not 12. Fix the check and rename the call to the new function:
```cpp
        case 0x06: {  // ZONE (designate)
            if (payload.size() < 16) {
                sendCommandAck(cmdID, 0x02, "Invalid ZONE payload");
                return;
            }
            uint8_t zoneType = payload[5];
            int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
            int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
            int16_t z = ((int16_t)payload[10] << 8) | payload[11];
            int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
            int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];
            success = applyDesignateZone(zoneType, x1, y1, z, x2, y2, error);
            break;
        }
```

- [ ] **Step 5: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
(Redirect to a log file, check the exit code directly — never pipe through `tail`/`head`. Retry once if it fails near `generate_headers` with exit `-1073741819`, a known flaky Perl-codegen issue, before treating it as a real failure.)
Expected: exit 0.

- [ ] **Step 6: Commit**

```bash
git add dfhack-plugin/zones.cpp dfhack-plugin/protocol.h dfhack-plugin/df_ai_protocol.cpp dfhack-plugin/CMakeLists.custom.txt
git commit -m "feat(plugin): real civzone creation, replacing the permanent-fail stub

Wire-level zone-type table corrected against the actual DFHack
53.15-r1 civzone_type enum (scattered 79-97, not the guessed 0x10+
range the stub's era used). Creation mirrors placeStockpile's
allocInstance->setSize->constructAbstract sequence exactly -- civzones
and stockpiles are both abstract buildings. Also fixes a pre-existing
off-by-bounds bug in the ZONE command's payload size check (checked
<12 but read through byte 15)."
```

---

## Task 2: Plugin — zone assignment/unassignment

**Files:**
- Modify: `dfhack-plugin/zones.cpp` (add `applyAssignZone`/`applyUnassignZone`)
- Modify: `dfhack-plugin/df_ai_protocol.cpp` (two new `case` blocks for `COMMAND_TYPE_ASSIGN_ZONE`/`COMMAND_TYPE_UNASSIGN_ZONE`, forward declarations)

**Interfaces:**
- Consumes: `Buildings::findCivzonesAt` (`library/modules/Buildings.h:100`, confirmed present in the checkout), `Buildings::setOwner`/`getOwner` (`Buildings.h:79,84`), `df::unit::find` (existing precedent: `applySetLabor` already resolves a wire unit ID this way).
- Produces: `bool applyAssignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error)`, `bool applyUnassignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error)`.

- [ ] **Step 1: Append `applyAssignZone`/`applyUnassignZone` to `zones.cpp`**

```cpp
// dfhack-plugin/zones.cpp -- append at end of file

#include "df/unit.h"
#include "df/general_ref_building_civzone_assignedst.h"
#include <vector>

// zoneMechanism classifies a civzone_type into how (if at all) a unit
// can be assigned to it -- see the plan's wire-table comment and
// design-zones.md Component 3 for the DFHack-source evidence behind
// each bucket.
enum class ZoneMechanism { Owner, Roster, Squad, Unconfirmed };

static ZoneMechanism mechanismFor(df::civzone_type t) {
    switch (t) {
        case df::civzone_type::Bedroom:
        case df::civzone_type::Office:
        case df::civzone_type::Tomb:
        case df::civzone_type::DiningHall:
            return ZoneMechanism::Owner;
        case df::civzone_type::Pen:
        case df::civzone_type::Pond:
            return ZoneMechanism::Roster;
        case df::civzone_type::Barracks:
            return ZoneMechanism::Squad;
        default:
            return ZoneMechanism::Unconfirmed;
    }
}

// findZoneAt resolves the civzone at (x,y,z), or returns null with error
// set. Zones are targeted by tile coordinate (matching this plugin's
// existing remove_building/unsuspend convention), not a synthetic ID.
static df::building_civzonest* findZoneAt(int16_t x, int16_t y, int16_t z, std::string &error) {
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "coordinates out of map bounds";
        return nullptr;
    }
    std::vector<df::building_civzonest*> results;
    Buildings::findCivzonesAt(&results, df::coord(x, y, z));
    if (results.empty()) {
        error = "no zone at (" + std::to_string(x) + "," + std::to_string(y) + "," + std::to_string(z) + ")";
        return nullptr;
    }
    return results[0];
}

bool applyAssignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error)
{
    df::building_civzonest *zone = findZoneAt(x, y, z, error);
    if (!zone) return false;

    df::unit *unit = df::unit::find(unitID);
    if (!unit) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }

    switch (mechanismFor(zone->type)) {
        case ZoneMechanism::Owner: {
            if (!Buildings::setOwner(zone, unit)) {
                error = "Buildings::setOwner failed";
                return false;
            }
            return true;
        }
        case ZoneMechanism::Roster: {
            auto *ref = new df::general_ref_building_civzone_assignedst();
            ref->building_id = zone->id;
            unit->general_refs.push_back(ref);
            zone->assigned_units.push_back(unitID);
            return true;
        }
        case ZoneMechanism::Squad:
            error = "Barracks assignment uses squads, not units -- not supported by this tool; see the future military/squad workstream";
            return false;
        case ZoneMechanism::Unconfirmed:
        default:
            error = "assignment mechanism for this zone type is not confirmed against the DFHack API -- creation and listing work, assignment does not yet";
            return false;
    }
}

bool applyUnassignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error)
{
    df::building_civzonest *zone = findZoneAt(x, y, z, error);
    if (!zone) return false;

    df::unit *unit = df::unit::find(unitID);
    if (!unit) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }

    switch (mechanismFor(zone->type)) {
        case ZoneMechanism::Owner: {
            if (Buildings::getOwner(zone) != unit) {
                error = "unit " + std::to_string(unitID) + " is not the owner of this zone -- not assigned";
                return false;
            }
            if (!Buildings::setOwner(zone, nullptr)) {
                error = "Buildings::setOwner(null) failed to clear owner";
                return false;
            }
            return true;
        }
        case ZoneMechanism::Roster: {
            auto &roster = zone->assigned_units;
            auto it = std::find(roster.begin(), roster.end(), unitID);
            if (it == roster.end()) {
                error = "unit " + std::to_string(unitID) + " is not assigned to this zone -- not assigned";
                return false;
            }
            roster.erase(it);
            for (size_t i = 0; i < unit->general_refs.size(); i++) {
                auto *ref = virtual_cast<df::general_ref_building_civzone_assignedst>(unit->general_refs[i]);
                if (ref && ref->building_id == zone->id) {
                    delete unit->general_refs[i];
                    unit->general_refs.erase(unit->general_refs.begin() + i);
                    break;
                }
            }
            return true;
        }
        case ZoneMechanism::Squad:
            error = "Barracks assignment uses squads, not units -- not supported by this tool";
            return false;
        case ZoneMechanism::Unconfirmed:
        default:
            error = "assignment mechanism for this zone type is not confirmed against the DFHack API";
            return false;
    }
}
```

(Step note for the implementer: verify `Buildings::setOwner(zone, nullptr)` is actually the correct way to clear an owner — re-check `Buildings.cpp:325-366`'s real implementation before compiling; if clearing needs a different call, adjust the unassign-owner branch accordingly and note the deviation in the commit message. Also verify `#include <algorithm>` is present for `std::find` — add it if the file doesn't already pull it in transitively.)

- [ ] **Step 2: Wire the two new commands into `executeCommand`**

Add forward declarations near the existing `applyDesignateZone` declaration in `df_ai_protocol.cpp`:
```cpp
bool applyAssignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error);
bool applyUnassignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error);
```

Add two new `case` blocks inside `executeCommand`'s `switch`, after the existing `case COMMAND_TYPE_SET_LABOR:` block (matches this file's payload-parsing style exactly — see `SET_LABOR`'s own block for the pattern):
```cpp
        case COMMAND_TYPE_ASSIGN_ZONE: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [4: UnitID]
            if (payload.size() < 15) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_ZONE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            int32_t unitID = (int32_t)read_uint32_be(payload, 11);
            success = applyAssignZone(x, y, z, unitID, error);
            break;
        }
        case COMMAND_TYPE_UNASSIGN_ZONE: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [4: UnitID]
            if (payload.size() < 15) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNASSIGN_ZONE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            int32_t unitID = (int32_t)read_uint32_be(payload, 11);
            success = applyUnassignZone(x, y, z, unitID, error);
            break;
        }
```
(`read_uint32_be` is the same helper `SET_LABOR`'s own block already calls at line 837 — reuse it, don't reimplement.)

- [ ] **Step 3: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add dfhack-plugin/zones.cpp dfhack-plugin/df_ai_protocol.cpp
git commit -m "feat(plugin): zone assign/unassign, honest about unconfirmed mechanisms

Owner-type (Bedroom/Office/Tomb/DiningHall) via Buildings::setOwner;
roster-type (Pen/Pond) via general_ref_building_civzone_assignedst +
assigned_units, the only two mechanisms DFHack's own source actually
confirms. Barracks (squad-based) and the 11 remaining types return a
specific, named error instead of guessing at an unverified struct
write. Zones are targeted by tile coordinate via
Buildings::findCivzonesAt, matching remove_building/unsuspend's
existing convention -- no synthetic zone ID anywhere in this plugin."
```

---

## Task 3: Plugin — `list_zones` query

**Files:**
- Modify: `dfhack-plugin/queries.cpp` (new `handleListZones`, dispatch wiring)

**Interfaces:**
- Consumes: `wireFromCivzoneType` (Task 1), `jsonStr`/`jsonInt`/`jsonGetString`/`jsonGetInt` (existing helpers, `queries.cpp:100-137`).
- Produces (JSON): `{"zones":[{"kind":1,"type_name":"Bedroom","x1":10,"y1":10,"x2":11,"y2":11,"z":90,"owner_unit_id":42,"assigned_units":[]}, ...],"truncated":false}`.

- [ ] **Step 1: Write `handleListZones`**

```cpp
// dfhack-plugin/queries.cpp -- add near handleListBuildings
#include "df/building_civzonest.h"

// Forward declaration -- implemented in zones.cpp.
uint8_t wireFromCivzoneType(df::civzone_type t);

static std::string handleListZones(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    // Optional {"type": int} and {"z": int} filters -- absent means all.
    std::string typeArg = jsonGetString(args, "type");
    bool hasType = !typeArg.empty();
    int64_t typeFilter = hasType ? jsonGetInt(args, "type", 0) : 0;
    std::string zArg = jsonGetString(args, "z");
    bool hasZ = !zArg.empty();
    int64_t zFilter = hasZ ? jsonGetInt(args, "z", 0) : 0;

    std::ostringstream os;
    os << "{\"zones\":[";
    int count = 0;
    bool truncated = false;
    for (auto *b : df::global::world->buildings.all) {
        if (!b || b->getType() != df::building_type::Civzone) continue;
        auto *cz = strict_virtual_cast<df::building_civzonest>(b);
        if (!cz) continue;
        uint8_t wireKind = wireFromCivzoneType(cz->type);
        if (wireKind == 0) continue; // not one of the 18 fortress-relevant types
        if (hasType && (int64_t)wireKind != typeFilter) continue;
        if (hasZ && (int64_t)b->z != zFilter) continue;
        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        os << "{\"kind\":" << jsonInt(wireKind)
           << ",\"type_name\":" << jsonStr(ENUM_KEY_STR(civzone_type, cz->type))
           << ",\"x1\":" << jsonInt(b->x1)
           << ",\"y1\":" << jsonInt(b->y1)
           << ",\"x2\":" << jsonInt(b->x2)
           << ",\"y2\":" << jsonInt(b->y2)
           << ",\"z\":" << jsonInt(b->z)
           << ",\"owner_unit_id\":" << jsonInt(cz->assigned_unit_id)
           << ",\"assigned_units\":[";
        for (size_t i = 0; i < cz->assigned_units.size(); i++) {
            if (i) os << ",";
            os << jsonInt(cz->assigned_units[i]);
        }
        os << "]}";
        count++;
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}
```

- [ ] **Step 2: Wire into `executeQuery`'s dispatch chain**

`executeQuery` (lines 781-805) is a linear `if (name == "...") ... else if (...)` chain. Add one more branch right after the existing `list_buildings` branch (line 793):
```cpp
        } else if (name == "list_zones") {
            data = handleListZones(args, status);
```

- [ ] **Step 3: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add dfhack-plugin/queries.cpp
git commit -m "feat(plugin): list_zones query

Mirrors list_buildings' shape and 200-entry cap+truncated pattern.
Optional type/z filters. type_name comes straight from
ENUM_KEY_STR(civzone_type, ...) -- DFHack's own enum key names already
match the Go-side type names this plan uses (Bedroom, Pen, etc.), no
separate name table needed."
```

---

## Task 4: Plugin — zone data in the periodic `ENTITY_UPDATE` refresh

**Files:**
- Modify: `dfhack-plugin/entities.cpp` (`serialize_entity_update` gains a zones block)

**Interfaces:**
- Consumes: `wireFromCivzoneType` (Task 1).
- Produces (wire, appended after the existing `FortInfo` block): `[1: HasZones][4: ZoneCount][N × ZoneEntry]` where each `ZoneEntry` = `[4: BuildingID][1: WireKind][2: X1][2: Y1][2: X2][2: Y2][2: Z][4: OwnerUnitID][2: AssignedCount][4×AssignedCount: AssignedUnitID]`. Capped at 200 zones (matching the `list_buildings`/`list_zones` cap), silently — this is a passive background refresh, not an on-demand query, so no truncation flag is needed the way `list_zones` has one; `list_zones` stays the authoritative on-demand source for anything beyond the cap.

- [ ] **Step 1: Append the zones block to `serialize_entity_update`**

The function currently ends its buffer-building at the optional `FortInfo` block (lines 188-227, reproduced in the design research) and then fills in the length header (lines 229-234). Insert the new zones block between those two points:

```cpp
// dfhack-plugin/entities.cpp -- add near the top, after the existing includes
#include "df/building_civzonest.h"
#include "df/building_type.h"

// Forward declaration -- implemented in zones.cpp.
uint8_t wireFromCivzoneType(df::civzone_type t);
```

```cpp
// dfhack-plugin/entities.cpp -- serialize_entity_update, insert this
// block AFTER the existing FortInfo block (after line 227's closing
// brace) and BEFORE the "Fill in length header" comment (line 229).

    // Zone block -- additive, appended after FortInfo. An old decoder
    // simply stops reading at the end of the FortInfo block and never
    // sees these bytes; this is the same additive-optional pattern
    // FortInfo itself already established.
    // [1: HasZones][4: ZoneCount][N x ZoneEntry], ZoneEntry =
    // [4: BuildingID][1: WireKind][2: X1][2: Y1][2: X2][2: Y2][2: Z]
    // [4: OwnerUnitID][2: AssignedCount][4xAssignedCount: AssignedUnitID]
    {
        CoreSuspender suspend;
        std::vector<df::building_civzonest*> zonesToSend;
        for (auto *b : df::global::world->buildings.all) {
            if (!b || b->getType() != df::building_type::Civzone) continue;
            auto *cz = strict_virtual_cast<df::building_civzonest>(b);
            if (!cz) continue;
            if (wireFromCivzoneType(cz->type) == 0) continue; // not fortress-relevant
            zonesToSend.push_back(cz);
            if (zonesToSend.size() >= 200) break; // matches list_zones' cap
        }

        buffer.push_back(1); // HasZones
        uint32_t zoneCount = (uint32_t)zonesToSend.size();
        buffer.push_back((zoneCount >> 24) & 0xFF);
        buffer.push_back((zoneCount >> 16) & 0xFF);
        buffer.push_back((zoneCount >> 8) & 0xFF);
        buffer.push_back(zoneCount & 0xFF);

        for (auto *cz : zonesToSend) {
            df::building *b = cz;
            uint32_t id = (uint32_t)b->id;
            buffer.push_back((id >> 24) & 0xFF);
            buffer.push_back((id >> 16) & 0xFF);
            buffer.push_back((id >> 8) & 0xFF);
            buffer.push_back(id & 0xFF);

            buffer.push_back(wireFromCivzoneType(cz->type));

            int16_t coords[5] = {b->x1, b->y1, b->x2, b->y2, (int16_t)b->z};
            for (int16_t c : coords) {
                buffer.push_back((c >> 8) & 0xFF);
                buffer.push_back(c & 0xFF);
            }

            int32_t owner = cz->assigned_unit_id;
            buffer.push_back((owner >> 24) & 0xFF);
            buffer.push_back((owner >> 16) & 0xFF);
            buffer.push_back((owner >> 8) & 0xFF);
            buffer.push_back(owner & 0xFF);

            uint16_t assignedCount = (uint16_t)cz->assigned_units.size();
            buffer.push_back((assignedCount >> 8) & 0xFF);
            buffer.push_back(assignedCount & 0xFF);
            for (int32_t uid : cz->assigned_units) {
                buffer.push_back((uid >> 24) & 0xFF);
                buffer.push_back((uid >> 16) & 0xFF);
                buffer.push_back((uid >> 8) & 0xFF);
                buffer.push_back(uid & 0xFF);
            }
        }
    }
```

- [ ] **Step 2: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0.

**Note for the implementing agent**: this task and Task 5 (the Go-side decode) must agree on the exact byte layout above before either is considered done — both were specified together in this plan for that reason. Do not deploy the rebuilt plugin until both land (deployment stays out of scope for every task in this plan regardless).

- [ ] **Step 3: Commit**

```bash
git add dfhack-plugin/entities.cpp
git commit -m "feat(plugin): zone data in the periodic ENTITY_UPDATE refresh

Additive block appended after the existing FortInfo block, same
pattern. Capped at 200 zones (list_zones stays the authoritative
on-demand source beyond that). This is what makes
WorldModel.Zones -- already fully wired on the Go side via
OnEntityUpdate -- actually receive real data for the first time."
```

---

## Task 5: Go — corrected wire protocol

**Files:**
- Modify: `internal/protocol/message.go` (`ZoneType*` values, `ZoneData` fields, 2 new `CommandType*` constants, 2 new payload structs)
- Modify: `internal/protocol/codec.go` (encode/decode for the 2 new commands; encode/decode for `EntityUpdateMessage.Zones`)
- Test: `internal/protocol/codec_test.go` (create if it doesn't exist — check first with Glob)

**Interfaces:**
- Consumes: none new (this task only touches wire-format code).
- Produces: `protocol.ZoneTypeBedroom` through `protocol.ZoneTypeAnimalTraining` (18 constants, corrected values matching the plan's wire table), `protocol.ZoneData{ZoneID uint32, ZoneType uint8, X1,Y1,Z1,X2,Y2,Z2 int16, OwnerUnitID int32, AssignedUnits []int32}`, `protocol.CommandTypeAssignZone`/`protocol.CommandTypeUnassignZone` (both `uint8`), `protocol.AssignZoneDesignation{X,Y,Z int16, UnitID int32}`, `protocol.UnassignZoneDesignation{X,Y,Z int16, UnitID int32}`.

- [ ] **Step 1: Write the failing tests for the corrected wire format**

```go
// internal/protocol/codec_test.go
package protocol

import (
	"bytes"
	"testing"
)

func TestZoneTypeConstants_MatchThePlanTable(t *testing.T) {
	cases := map[uint8]uint8{
		ZoneTypeBedroom: 0x01, ZoneTypeOffice: 0x02, ZoneTypeTomb: 0x03,
		ZoneTypeDiningHall: 0x04, ZoneTypeMeetingHall: 0x05, ZoneTypeDormitory: 0x06,
		ZoneTypeBarracks: 0x07, ZoneTypePen: 0x08, ZoneTypePond: 0x09,
		ZoneTypeArcheryRange: 0x0A, ZoneTypePlantGathering: 0x0B, ZoneTypeWaterSource: 0x0C,
		ZoneTypeDump: 0x0D, ZoneTypeSandCollection: 0x0E, ZoneTypeFishingArea: 0x0F,
		ZoneTypeClayCollection: 0x10, ZoneTypeDungeon: 0x11, ZoneTypeAnimalTraining: 0x12,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant value mismatch: got 0x%02X, want 0x%02X", got, want)
		}
	}
}

func TestEncodeDecodeAssignZoneCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:   7,
		CommandType: CommandTypeAssignZone,
		AssignZone:  AssignZoneDesignation{X: 10, Y: 20, Z: 90, UnitID: 42},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AssignZone != msg.AssignZone {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.AssignZone, msg.AssignZone)
	}
}

func TestEncodeDecodeUnassignZoneCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:    8,
		CommandType:  CommandTypeUnassignZone,
		UnassignZone: UnassignZoneDesignation{X: 10, Y: 20, Z: 90, UnitID: 42},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.UnassignZone != msg.UnassignZone {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.UnassignZone, msg.UnassignZone)
	}
}

func TestEntityUpdateMessage_ZonesRoundTrip(t *testing.T) {
	msg := &EntityUpdateMessage{
		Count:    0,
		Entities: nil,
		Zones: []ZoneData{
			{ZoneID: 99, ZoneType: ZoneTypeBedroom, X1: 1, Y1: 1, Z1: 90, X2: 2, Y2: 2, Z2: 90, OwnerUnitID: 42, AssignedUnits: nil},
			{ZoneID: 100, ZoneType: ZoneTypePen, X1: 5, Y1: 5, Z1: 90, X2: 8, Y2: 8, Z2: 90, OwnerUnitID: -1, AssignedUnits: []int32{1, 2, 3}},
		},
	}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(decoded.Zones) != 2 {
		t.Fatalf("expected 2 zones, got %d", len(decoded.Zones))
	}
	z0 := decoded.Zones[0]
	if z0.ZoneID != 99 || z0.ZoneType != ZoneTypeBedroom || z0.X1 != 1 || z0.Y1 != 1 ||
		z0.X2 != 2 || z0.Y2 != 2 || z0.Z1 != 90 || z0.Z2 != 90 || z0.OwnerUnitID != 42 {
		t.Fatalf("zone 0 mismatch: got %+v", z0)
	}
	if len(z0.AssignedUnits) != 0 {
		t.Fatalf("expected zone 0 to have no assigned units, got %v", z0.AssignedUnits)
	}
	z1 := decoded.Zones[1]
	if len(z1.AssignedUnits) != 3 || z1.AssignedUnits[0] != 1 || z1.AssignedUnits[1] != 2 || z1.AssignedUnits[2] != 3 {
		t.Fatalf("expected zone 1 assigned units [1,2,3], got %v", z1.AssignedUnits)
	}
}

func TestEntityUpdateMessage_NoZonesDecodesEmpty(t *testing.T) {
	msg := &EntityUpdateMessage{Count: 0, Entities: nil, Zones: nil}
	var buf bytes.Buffer
	if err := serializeEntityUpdate(&buf, msg); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	decoded, err := deserializeEntityUpdate(buf.Bytes())
	if err != nil {
		t.Fatalf("deserialize: %v", err)
	}
	if len(decoded.Zones) != 0 {
		t.Fatalf("expected 0 zones, got %d", len(decoded.Zones))
	}
}
```

(`ZoneData` is compared field-by-field above rather than with `!=` because it contains a slice field, `AssignedUnits`, which Go does not allow in a struct equality comparison.)

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/protocol/... -run "TestZoneTypeConstants|TestEncodeDecodeAssignZone|TestEncodeDecodeUnassignZone|TestEntityUpdateMessage_Zones" -v`
Expected: FAIL — compile errors (`ZoneTypeOffice`, `CommandTypeAssignZone`, `AssignZoneDesignation`, etc. all undefined; existing `ZoneType*` constants have the old wrong values).

- [ ] **Step 3: Correct the `ZoneType*` constants in `message.go`**

Replace the entire existing block (lines 382-408, shown in full in the plan's research) with:
```go
// ZoneType constants for zone designations. These are DF-AI's OWN wire
// values, matching dfhack-plugin/protocol.h's ZONE_TYPE_* constants —
// NOT DFHack's own df::civzone_type values directly (those are scattered
// 79-97, confirmed against the DFHack 53.15-r1 checkout). Assignment
// support: Owner (setOwner) works for Bedroom/Office/Tomb/DiningHall;
// Roster (assigned_units) works for Pen/Pond; Barracks uses a separate
// squad-based mechanism (out of scope for assign_zone); the rest have no
// confirmed DFHack assignment mechanism at all — assign_zone returns an
// explicit "not implemented" error for them.
const (
	ZoneTypeBedroom        uint8 = 0x01 // Owner
	ZoneTypeOffice         uint8 = 0x02 // Owner
	ZoneTypeTomb           uint8 = 0x03 // Owner
	ZoneTypeDiningHall     uint8 = 0x04 // Owner
	ZoneTypeMeetingHall    uint8 = 0x05 // Unconfirmed
	ZoneTypeDormitory      uint8 = 0x06 // Unconfirmed
	ZoneTypeBarracks       uint8 = 0x07 // Squad (out of scope)
	ZoneTypePen            uint8 = 0x08 // Roster
	ZoneTypePond           uint8 = 0x09 // Roster
	ZoneTypeArcheryRange   uint8 = 0x0A // Unconfirmed
	ZoneTypePlantGathering uint8 = 0x0B // Unconfirmed
	ZoneTypeWaterSource    uint8 = 0x0C // Unconfirmed
	ZoneTypeDump           uint8 = 0x0D // Unconfirmed
	ZoneTypeSandCollection uint8 = 0x0E // Unconfirmed
	ZoneTypeFishingArea    uint8 = 0x0F // Unconfirmed
	ZoneTypeClayCollection uint8 = 0x10 // Unconfirmed
	ZoneTypeDungeon        uint8 = 0x11 // Unconfirmed
	ZoneTypeAnimalTraining uint8 = 0x12 // Unconfirmed
)
```

Correct `ZoneData` (replace the existing struct at lines 256-263):
```go
// ZoneData represents a DF civzone extracted from game state. ZoneID is
// DFHack's internal building id — informational only; assign_zone and
// unassign_zone target zones by tile coordinate (matching
// remove_building/unsuspend's existing convention), never by this ID.
type ZoneData struct {
	ZoneID        uint32  // DFHack building id (informational only)
	ZoneType      uint8   // One of the ZoneType* constants above
	X1, Y1, Z1    int16   // Start coordinates
	X2, Y2, Z2    int16   // End coordinates
	OwnerUnitID   int32   // -1 if unowned or not an owner-type zone
	AssignedUnits []int32 // roster (empty for owner-type or unconfirmed-mechanism zones)
}
```

Add the two new command types after `CommandTypeSetLabor` (existing block, `message.go:283-300`):
```go
	CommandTypeAssignZone   uint8 = 0x10 // Assign a unit to the zone at a tile (owner or roster, depending on zone type)
	CommandTypeUnassignZone uint8 = 0x11 // Remove a unit's zone assignment
```

Add the two new payload structs near the existing `ZoneDesignation` struct (`message.go:526-531`):
```go
// AssignZoneDesignation targets the zone at (X,Y,Z) and assigns UnitID.
type AssignZoneDesignation struct {
	X, Y, Z int16
	UnitID  int32
}

// UnassignZoneDesignation targets the zone at (X,Y,Z) and removes UnitID's assignment.
type UnassignZoneDesignation struct {
	X, Y, Z int16
	UnitID  int32
}
```

Add the two new fields to `CommandMessage` (find the struct — it already has a `Zone ZoneDesignation` field per the `SendZoneCommand` precedent in executor.go; add alongside it):
```go
	AssignZone   AssignZoneDesignation
	UnassignZone UnassignZoneDesignation
```

- [ ] **Step 4: Add codec encode/decode for the two new commands**

In `codec.go`, add two new `case` blocks to the command-encoding switch, alongside the existing `case CommandTypeZone:` block (`codec.go:822-831`):
```go
	case CommandTypeAssignZone:
		// [2: X] [2: Y] [2: Z] [4: UnitID]
		for _, v := range []int16{msg.AssignZone.X, msg.AssignZone.Y, msg.AssignZone.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.AssignZone.UnitID); err != nil {
			return err
		}
	case CommandTypeUnassignZone:
		// [2: X] [2: Y] [2: Z] [4: UnitID]
		for _, v := range []int16{msg.UnassignZone.X, msg.UnassignZone.Y, msg.UnassignZone.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.UnassignZone.UnitID); err != nil {
			return err
		}
```

And the matching decode cases, alongside the existing `case CommandTypeZone:` decode block (`codec.go:1018-1026`):
```go
	case CommandTypeAssignZone:
		for _, p := range []*int16{&msg.AssignZone.X, &msg.AssignZone.Y, &msg.AssignZone.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.AssignZone.UnitID); err != nil {
			return nil, err
		}
	case CommandTypeUnassignZone:
		for _, p := range []*int16{&msg.UnassignZone.X, &msg.UnassignZone.Y, &msg.UnassignZone.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.UnassignZone.UnitID); err != nil {
			return nil, err
		}
```

- [ ] **Step 5: Add codec encode/decode for `EntityUpdateMessage.Zones`**

The byte layout must exactly match Task 4's plugin-side format: `[1: HasZones][4: ZoneCount][N x ZoneEntry]`, `ZoneEntry = [4: BuildingID(->ZoneID)][1: WireKind][2: X1][2: Y1][2: X2][2: Y2][2: Z][4: OwnerUnitID][2: AssignedCount][4xAssignedCount: AssignedUnitID]`.

In `serializeEntityUpdate` (`codec.go:605-645`), append after the existing `FortInfo` block (after line 642's closing `}`, before the final `return nil` at line 644):
```go
	// Zones block -- additive, appended after FortInfo. Byte layout must
	// exactly match dfhack-plugin/entities.cpp's serialize_entity_update.
	hasZones := uint8(0)
	if len(msg.Zones) > 0 {
		hasZones = 1
	}
	if err := binary.Write(w, binary.BigEndian, hasZones); err != nil {
		return err
	}
	if err := binary.Write(w, binary.BigEndian, uint32(len(msg.Zones))); err != nil {
		return err
	}
	for _, z := range msg.Zones {
		if err := binary.Write(w, binary.BigEndian, z.ZoneID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, z.ZoneType); err != nil {
			return err
		}
		for _, v := range []int16{z.X1, z.Y1, z.X2, z.Y2, z.Z1} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, z.OwnerUnitID); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, uint16(len(z.AssignedUnits))); err != nil {
			return err
		}
		for _, uid := range z.AssignedUnits {
			if err := binary.Write(w, binary.BigEndian, uid); err != nil {
				return err
			}
		}
	}
```
(Note: the plugin writes 5 coordinate fields as `X1,Y1,X2,Y2,Z` — a single Z, not `Z1`/`Z2` separately, since a civzone footprint is flat on one Z-level. `ZoneData.Z1` is used for both `Z1`/`Z2` on decode, per Step 6 below — this matches how the design's byte layout was specified in Task 4.)

In `deserializeEntityUpdate` (`codec.go:647-692`), append after the existing `FortInfo` decode block (after line 689's comment, before `return msg, nil` at line 691):
```go
	// Zones block (additive — absent on an old peer, decoded as empty).
	var hasZones uint8
	if err := binary.Read(buf, binary.BigEndian, &hasZones); err == nil && hasZones == 1 {
		var zoneCount uint32
		if err := binary.Read(buf, binary.BigEndian, &zoneCount); err != nil {
			return nil, err
		}
		msg.Zones = make([]ZoneData, zoneCount)
		for i := uint32(0); i < zoneCount; i++ {
			z := &msg.Zones[i]
			if err := binary.Read(buf, binary.BigEndian, &z.ZoneID); err != nil {
				return nil, err
			}
			if err := binary.Read(buf, binary.BigEndian, &z.ZoneType); err != nil {
				return nil, err
			}
			var x1, y1, x2, y2, zLevel int16
			for _, p := range []*int16{&x1, &y1, &x2, &y2, &zLevel} {
				if err := binary.Read(buf, binary.BigEndian, p); err != nil {
					return nil, err
				}
			}
			z.X1, z.Y1, z.X2, z.Y2 = x1, y1, x2, y2
			z.Z1, z.Z2 = zLevel, zLevel
			if err := binary.Read(buf, binary.BigEndian, &z.OwnerUnitID); err != nil {
				return nil, err
			}
			var assignedCount uint16
			if err := binary.Read(buf, binary.BigEndian, &assignedCount); err != nil {
				return nil, err
			}
			if assignedCount > 0 {
				z.AssignedUnits = make([]int32, assignedCount)
				for j := uint16(0); j < assignedCount; j++ {
					if err := binary.Read(buf, binary.BigEndian, &z.AssignedUnits[j]); err != nil {
						return nil, err
					}
				}
			}
		}
	}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `go test ./internal/protocol/... -v`
Expected: PASS — all new tests plus the full existing suite (no regressions).

- [ ] **Step 7: Run the full repo build to catch any other callers broken by the `ZoneData`/`ZoneType*` changes**

Run: `go build ./... > /path/to/scratchpad/task5-build.log 2>&1; echo exit=$?; cat /path/to/scratchpad/task5-build.log`
(Use your own scratchpad path, never pipe through `tail`/`head`.) Expected: exit=0. If `internal/zones/extractor.go` fails to compile because it references the old `ZoneData.AssignedTo` field name (now `OwnerUnitID`) or the old `ZoneType` constant values it hardcodes its own mapping against — **do not delete or restructure `internal/zones`** (Global Constraints: out of scope, Task 16's job). Instead, make the minimal compiling fix: update `internal/zones/extractor.go`'s `convertZoneType`/field references to match the renamed field, changing nothing about that package's own (separate, legacy) `zones.ZoneType` enum or behavior.

- [ ] **Step 8: Commit**

```bash
git add internal/protocol/message.go internal/protocol/codec.go internal/protocol/codec_test.go
git commit -m "feat(protocol): corrected zone wire types, assign/unassign commands, live Zones payload

ZoneType* constants replaced wholesale to match the plan's verified
civzone_type table (previous values were a mix of dead Feature-007
numbering and guessed hex values that didn't match any real DFHack
enum). ZoneData gains OwnerUnitID/AssignedUnits. Two new command types
for assign/unassign, targeted by tile coordinate. EntityUpdateMessage.
Zones finally has real encode/decode code -- it existed as a dead
struct field since before this session."
```

---

## Task 6: Go — MCP tools (`designate_zone`, `assign_zone`, `unassign_zone`, `list_zones`)

**Files:**
- Create: `internal/mcpserver/tools_zone.go`
- Create: `internal/mcpserver/tools_zone_test.go`
- Modify: `internal/commands/executor.go` (add `SendAssignZone`, `SendUnassignZone`)
- Modify: `internal/mcpserver/tools_action.go` (remove the old `zone` tool block and `zoneTypes` map)
- Modify: `internal/mcpserver/tools_action_test.go` (remove the `zone` entry from `TestActionToolsNilBridge`)
- Modify: `internal/mcpserver/server.go` (add `registerZoneTools(srv, bridge)`)
- Modify: `internal/mcpserver/server_test.go` (replace `minToolArgs["zone"]` with entries for the 4 new tools)

**Interfaces:**
- Consumes: `protocol.ZoneType*` (Task 5), `commands.CommandExecutor.SendZoneCommand` (existing, unchanged shape), `ackText`/`withDash`/`noExec` (`tools_action.go`, existing).
- Produces: `func renderZones(raw []byte) string` (the summary-by-type render, Component 5's token-scaling correction), `func registerZoneTools(srv *mcp.Server, b *Bridge)`.

- [ ] **Step 1: Add `SendAssignZone`/`SendUnassignZone` to the executor**

```go
// internal/commands/executor.go -- add near SendZoneCommand
// SendAssignZone assigns unitID to the zone at (x,y,z) -- owner or
// roster mechanism, selected by the plugin based on the zone's type.
func (e *CommandExecutor) SendAssignZone(x, y, z int16, unitID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignZone,
		AssignZone: protocol.AssignZoneDesignation{
			X: x, Y: y, Z: z, UnitID: unitID,
		},
	}
	return e.SendCommand(cmd)
}

// SendUnassignZone removes unitID's assignment from the zone at (x,y,z).
func (e *CommandExecutor) SendUnassignZone(x, y, z int16, unitID int32) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnassignZone,
		UnassignZone: protocol.UnassignZoneDesignation{
			X: x, Y: y, Z: z, UnitID: unitID,
		},
	}
	return e.SendCommand(cmd)
}
```

- [ ] **Step 2: Write the failing tests for `renderZones`**

```go
// internal/mcpserver/tools_zone_test.go
package mcpserver

import (
	"strings"
	"testing"
)

func TestRenderZones_GroupsByTypeByDefault(t *testing.T) {
	raw := []byte(`{"zones":[
		{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":5,"assigned_units":[]},
		{"kind":1,"type_name":"Bedroom","x1":2,"y1":2,"x2":2,"y2":2,"z":90,"owner_unit_id":-1,"assigned_units":[]},
		{"kind":4,"type_name":"DiningHall","x1":5,"y1":5,"x2":8,"y2":8,"z":90,"owner_unit_id":-1,"assigned_units":[]},
		{"kind":8,"type_name":"Pen","x1":10,"y1":10,"x2":15,"y2":15,"z":85,"owner_unit_id":-1,"assigned_units":[1,2,3]}
	]}`)
	out := renderZones(raw, "", -1)
	// Must collapse same-type zones to ONE line, not one line per zone.
	if strings.Count(out, "Bedroom") != 1 {
		t.Fatalf("expected exactly one Bedroom summary line, got:\n%s", out)
	}
	if !strings.Contains(out, "2 Bedroom") || !strings.Contains(out, "1 owned") {
		t.Fatalf("expected a bedroom count with an owned breakdown, got:\n%s", out)
	}
	if !strings.Contains(out, "1 DiningHall") {
		t.Fatalf("expected a dining hall summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 Pen") || !strings.Contains(out, "3 animals") {
		t.Fatalf("expected a pen summary with animal count, got:\n%s", out)
	}
}

func TestRenderZones_TypeFilterShowsFullDetail(t *testing.T) {
	raw := []byte(`{"zones":[
		{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":5,"assigned_units":[]},
		{"kind":4,"type_name":"DiningHall","x1":5,"y1":5,"x2":8,"y2":8,"z":90,"owner_unit_id":-1,"assigned_units":[]}
	]}`)
	out := renderZones(raw, "Bedroom", -1)
	if !strings.Contains(out, "(1,1)") {
		t.Fatalf("expected per-zone extents when filtered by type, got:\n%s", out)
	}
	if strings.Contains(out, "DiningHall") {
		t.Fatalf("type filter must exclude other types, got:\n%s", out)
	}
}

func TestRenderZones_NoZones(t *testing.T) {
	out := renderZones([]byte(`{"zones":[]}`), "", -1)
	if out != "No zones." {
		t.Fatalf("expected 'No zones.', got %q", out)
	}
}

func TestRenderZones_Truncated(t *testing.T) {
	out := renderZones([]byte(`{"zones":[{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":-1,"assigned_units":[]}],"truncated":true}`), "", -1)
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/mcpserver/... -run TestRenderZones -v`
Expected: FAIL — `renderZones` undefined.

- [ ] **Step 4: Implement `tools_zone.go`**

```go
// internal/mcpserver/tools_zone.go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

// zoneWireTypes maps a lowercase user-facing name to its wire byte —
// matches the plan's corrected wire table exactly.
var zoneWireTypes = map[string]uint8{
	"bedroom": protocol.ZoneTypeBedroom, "office": protocol.ZoneTypeOffice,
	"tomb": protocol.ZoneTypeTomb, "dining_hall": protocol.ZoneTypeDiningHall,
	"meeting_hall": protocol.ZoneTypeMeetingHall, "dormitory": protocol.ZoneTypeDormitory,
	"barracks": protocol.ZoneTypeBarracks, "pen": protocol.ZoneTypePen,
	"pond": protocol.ZoneTypePond, "archery_range": protocol.ZoneTypeArcheryRange,
	"plant_gathering": protocol.ZoneTypePlantGathering, "water_source": protocol.ZoneTypeWaterSource,
	"dump": protocol.ZoneTypeDump, "sand_collection": protocol.ZoneTypeSandCollection,
	"fishing_area": protocol.ZoneTypeFishingArea, "clay_collection": protocol.ZoneTypeClayCollection,
	"dungeon": protocol.ZoneTypeDungeon, "animal_training": protocol.ZoneTypeAnimalTraining,
}

type zoneListEntry struct {
	Kind          int    `json:"kind"`
	TypeName      string `json:"type_name"`
	X1            int    `json:"x1"`
	Y1            int    `json:"y1"`
	X2            int    `json:"x2"`
	Y2            int    `json:"y2"`
	Z             int    `json:"z"`
	OwnerUnitID   int    `json:"owner_unit_id"`
	AssignedUnits []int  `json:"assigned_units"`
}

// renderZones renders list_zones' response. Default (no typeFilter) is a
// summary grouped by type -- one line per type, not one line per zone --
// per the design doc's token-scaling correction (recommendation #5:
// the model needs the shape of the fort far more often than N
// coordinates). Passing typeFilter switches to full per-zone detail for
// that type only. zFilter < 0 means no z filter (informational only in
// the render; the actual filtering happens plugin-side via the query
// args — this parameter only affects the summary-vs-detail choice
// alongside typeFilter, kept simple: any explicit filter narrows to detail).
func renderZones(raw []byte, typeFilter string, zFilter int) string {
	var resp struct {
		Zones     []zoneListEntry `json:"zones"`
		Truncated bool            `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_zones response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Zones) == 0 {
		return "No zones."
	}

	var sb strings.Builder
	if typeFilter != "" || zFilter >= 0 {
		fmt.Fprintf(&sb, "%d zones:\n", len(resp.Zones))
		for _, z := range resp.Zones {
			fmt.Fprintf(&sb, "- %s at (%d,%d)-(%d,%d) z=%d", z.TypeName, z.X1, z.Y1, z.X2, z.Y2, z.Z)
			if z.OwnerUnitID >= 0 {
				fmt.Fprintf(&sb, " owner=unit#%d", z.OwnerUnitID)
			}
			if len(z.AssignedUnits) > 0 {
				fmt.Fprintf(&sb, " assigned=%v", z.AssignedUnits)
			}
			sb.WriteString("\n")
		}
	} else {
		byType := map[string][]zoneListEntry{}
		for _, z := range resp.Zones {
			byType[z.TypeName] = append(byType[z.TypeName], z)
		}
		types := make([]string, 0, len(byType))
		for t := range byType {
			types = append(types, t)
		}
		sort.Strings(types)

		fmt.Fprintf(&sb, "%d zones:\n", len(resp.Zones))
		for _, t := range types {
			zones := byType[t]
			owned, unowned, animals := 0, 0, 0
			hasOwnerData := false
			for _, z := range zones {
				if z.OwnerUnitID >= 0 {
					owned++
					hasOwnerData = true
				} else if len(z.AssignedUnits) == 0 {
					unowned++
				}
				animals += len(z.AssignedUnits)
			}
			fmt.Fprintf(&sb, "- %d %s", len(zones), t)
			switch {
			case animals > 0:
				fmt.Fprintf(&sb, " (%d animals total)", animals)
			case hasOwnerData:
				fmt.Fprintf(&sb, " (%d owned, %d unowned)", owned, unowned)
			}
			sb.WriteString("\n")
		}
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap — pass type or z to narrow it\n")
	}
	return sb.String()
}

func registerZoneTools(srv *mcp.Server, b *Bridge) {
	type designateZoneIn struct {
		Type string `json:"type" jsonschema:"bedroom|office|tomb|dining_hall|meeting_hall|dormitory|barracks|pen|pond|archery_range|plant_gathering|water_source|dump|sand_collection|fishing_area|clay_collection|dungeon|animal_training"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "designate_zone",
		Description: "Designate a civzone rectangle over EXISTING built/carved floor on one z-level — zones claim space, they don't dig it (dig with designate_dig first). Assignment support varies by type: assign_zone works for bedroom/office/tomb/dining_hall (single owner) and pen/pond (animal roster); the other types designate and list fine but cannot be assigned yet (barracks needs squad tooling; the rest have no confirmed DFHack assignment mechanism).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in designateZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		zt, ok := zoneWireTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown zone type %q", in.Type)), nil, nil
		}
		res, err := b.Exec.SendZoneCommand(zt, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		what := fmt.Sprintf("designate_zone %s (%d,%d)-(%d,%d) z=%d", in.Type, in.X1, in.Y1, in.X2, in.Y2, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type assignZoneIn struct {
		X      int `json:"x" jsonschema:"a tile inside the target zone"`
		Y      int `json:"y"`
		Z      int `json:"z"`
		UnitID int `json:"unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_zone",
		Description: "Assign a unit to the zone at a tile (any tile inside the zone's footprint — see list_zones for extents). Owner-type zones (bedroom/office/tomb/dining_hall) get a single owner; roster-type zones (pen/pond) get an animal added to the roster. Other zone types return a clear error naming why assignment isn't supported yet.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendAssignZone(int16(in.X), int16(in.Y), int16(in.Z), int32(in.UnitID))
		what := fmt.Sprintf("assign_zone (%d,%d,%d) -> unit#%d", in.X, in.Y, in.Z, in.UnitID)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type unassignZoneIn struct {
		X      int `json:"x" jsonschema:"a tile inside the target zone"`
		Y      int `json:"y"`
		Z      int `json:"z"`
		UnitID int `json:"unit_id"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unassign_zone",
		Description: "Remove a unit's assignment from the zone at a tile.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in unassignZoneIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendUnassignZone(int16(in.X), int16(in.Y), int16(in.Z), int32(in.UnitID))
		what := fmt.Sprintf("unassign_zone (%d,%d,%d) -> unit#%d", in.X, in.Y, in.Z, in.UnitID)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type listZonesIn struct {
		Type string `json:"type,omitempty" jsonschema:"optional: only list zones of this type, and show full per-zone detail instead of the type-summary default"`
		Z    *int   `json:"z,omitempty" jsonschema:"optional: only list zones on this z-level"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_zones",
		Description: "List zones. Default: summary grouped by type. Pass type (and/or z) to see full per-zone extents and owner/roster detail for that type.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listZonesIn) (*mcp.CallToolResult, any, error) {
		args := "{"
		parts := []string{}
		if in.Type != "" {
			if kind, ok := zoneWireTypes[strings.ToLower(in.Type)]; ok {
				parts = append(parts, fmt.Sprintf(`"type":%d`, kind))
			}
		}
		if in.Z != nil {
			parts = append(parts, fmt.Sprintf(`"z":%d`, *in.Z))
		}
		args += strings.Join(parts, ",") + "}"
		raw, err := b.Query(ctx, "list_zones", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		zFilter := -1
		if in.Z != nil {
			zFilter = *in.Z
		}
		return withDash(b, ctx, renderZones(raw, in.Type, zFilter)), nil, nil
	})
}
```

- [ ] **Step 5: Remove the old stub from `tools_action.go`**

Delete the `zoneTypes` map (lines 135-143) and the entire `zone` tool registration block (lines 339-360), both shown in full in this plan's research phase.

- [ ] **Step 6: Remove the `zone` case from `tools_action_test.go`**

In `TestActionToolsNilBridge`'s `calls` table (lines 71-82), delete the line:
```go
		{"zone", map[string]any{"type": "bedroom", "x1": 10, "y1": 10, "z": 90, "x2": 12, "y2": 12}},
```

- [ ] **Step 7: Wire `registerZoneTools` into `server.go`**

```go
// internal/mcpserver/server.go -- New(), add alongside the other register*Tools calls
	registerZoneTools(srv, bridge)
```

- [ ] **Step 8: Update `server_test.go`'s `minToolArgs`**

Replace the existing `"zone": {...}` line (line 54) with:
```go
	"designate_zone": {"type": "bedroom", "x1": 1, "y1": 1, "z": 1, "x2": 2, "y2": 2},
	"assign_zone":     {"x": 1, "y": 1, "z": 1, "unit_id": 1},
	"unassign_zone":   {"x": 1, "y": 1, "z": 1, "unit_id": 1},
	// list_zones has no required fields — {} default is fine, no entry needed.
```

- [ ] **Step 9: Run the tests to verify they pass**

Run: `go test ./internal/mcpserver/... -run "TestRenderZones|TestEveryToolNilBridge|TestActionToolsNilBridge" -v`
Expected: PASS.

- [ ] **Step 10: Run the full test suite**

Run: `go build ./... && go test ./... > /path/to/scratchpad/task6.log 2>&1; echo exit=$?; cat /path/to/scratchpad/task6.log`
Expected: exit=0, all packages ok.

- [ ] **Step 11: Commit**

```bash
git add internal/mcpserver/tools_zone.go internal/mcpserver/tools_zone_test.go internal/commands/executor.go internal/mcpserver/tools_action.go internal/mcpserver/tools_action_test.go internal/mcpserver/server.go internal/mcpserver/server_test.go
git commit -m "feat(mcpserver): designate_zone / assign_zone / unassign_zone / list_zones

Replaces the permanently-failing zone stub entirely. list_zones
defaults to a type-grouped summary (token-scaling correction from the
design doc) with full per-zone detail behind a type/z filter. Zones
are targeted by tile coordinate throughout, matching remove_building/
unsuspend's existing convention."
```

---

## Task 7: Go — predicate fixes

**Files:**
- Modify: `internal/predicate/library.go`

**Interfaces:**
- Consumes: `protocol.ZoneTypeBedroom`, `protocol.ZoneTypeDiningHall` (Task 5).

- [ ] **Step 1: Write the failing test**

```go
// internal/predicate/library_test.go -- add (file already exists per this session's earlier work; append)
func TestHasBedroomZones_UsesCorrectedWireValue(t *testing.T) {
	wm := worldmodel.New()
	wm.Observed.Zones = worldmodel.ZoneSnapshot{
		All: []protocol.ZoneData{
			{ZoneType: protocol.ZoneTypeBedroom, OwnerUnitID: 1},
			{ZoneType: protocol.ZoneTypeBedroom, OwnerUnitID: -1},
			{ZoneType: protocol.ZoneTypePen, OwnerUnitID: -1}, // must NOT count
		},
		UpdatedAt: time.Now(),
	}
	p := HasBedroomZones{Min: 2, H: HorizonSoon}
	r := p.Check(wm)
	if !r.Satisfied {
		t.Fatalf("expected satisfied with 2 bedroom zones, got %+v", r)
	}
}

func TestHasDiningHall_UsesCorrectedWireValue(t *testing.T) {
	wm := worldmodel.New()
	wm.Observed.Zones = worldmodel.ZoneSnapshot{
		All: []protocol.ZoneData{
			{ZoneType: protocol.ZoneTypeDiningHall},
		},
		UpdatedAt: time.Now(),
	}
	p := HasDiningHall{H: HorizonSoon}
	r := p.Check(wm)
	if !r.Satisfied {
		t.Fatalf("expected satisfied with 1 dining hall, got %+v", r)
	}
}
```

(Check the existing `internal/predicate/library_test.go` first — it already exists per this project's history — for the exact `worldmodel.New()`/construction idiom other tests in that file use, and match it exactly rather than guessing at the constructor shape shown above.)

- [ ] **Step 2: Run tests to verify current behavior**

Run: `go test ./internal/predicate/... -run "TestHasBedroomZones_UsesCorrectedWireValue|TestHasDiningHall_UsesCorrectedWireValue" -v`
Expected: PASS already, in fact — the old hardcoded local constants (`0x01`/`0x02`) happen to still match the NEW corrected `protocol.ZoneTypeBedroom`/`protocol.ZoneTypeDiningHall` values (both are still `0x01`/`0x02` in the new table — see the plan's wire table at the top of this document). This step exists to confirm that overlap is real, not assumed, before Step 3 removes the local magic numbers as a correctness/maintainability fix rather than a behavior fix.

- [ ] **Step 3: Replace the local magic-number constants with the canonical `protocol` package references**

```go
// internal/predicate/library.go — HasBedroomZones.Check, replace
	const zoneTypeBedroom = uint8(0x01)
// with
	const zoneTypeBedroom = protocol.ZoneTypeBedroom
```
(Add `"github.com/df-ai/orchestrator/internal/protocol"` to this file's imports if not already present — check first, `worldmodel.ZoneSnapshot`/`ZoneData` already come from this package so it's very likely already imported.)

```go
// internal/predicate/library.go — HasDiningHall.Check, replace
	const zoneTypeDining = uint8(0x02)
// with
	const zoneTypeDining = protocol.ZoneTypeDiningHall
```

- [ ] **Step 4: Run tests to verify they still pass**

Run: `go test ./internal/predicate/... -v`
Expected: PASS, no regressions — this step is a pure refactor (removing a coincidental-magic-number footgun before it silently drifts), not a behavior change, so nothing should flip.

- [ ] **Step 5: Commit**

```bash
git add internal/predicate/library.go internal/predicate/library_test.go
git commit -m "fix(predicate): HasBedroomZones/HasDiningHall reference protocol constants

Previously hardcoded local magic numbers (0x01/0x02) that happened to
still match the corrected wire values by coincidence, not by
reference -- a future wire renumbering would have silently broken
these predicates with no compile-time signal. Now references
protocol.ZoneTypeBedroom/ZoneTypeDiningHall directly."
```

---

## Task 8: Go — `look` zones lens

**Files:**
- Modify: `internal/mcpserver/lenses.go`
- Modify: `internal/mcpserver/lenses_test.go`

**Interfaces:**
- Consumes: `zoneListEntry` (Task 6), `mapview.Overlay` (existing, Perception round 2), `LensDef`/`lenses`/`lensGlyphSet` (existing).
- Produces: `func zoneCategoryGlyph(typeName string) rune`, `func gatherZonesLens(ctx, b, s, z) (mapview.Overlay, error)`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/mcpserver/lenses_test.go -- append
func TestZoneCategoryGlyph(t *testing.T) {
	cases := map[string]rune{
		"Bedroom": 'H', "Office": 'H', "Tomb": 'H', "DiningHall": 'H', "MeetingHall": 'H', "Dormitory": 'H',
		"Barracks": 'K',
		"Pen": 'A', "Pond": 'A', "AnimalTraining": 'A', "ArcheryRange": 'A',
		"WaterSource": 'R', "Dump": 'R', "SandCollection": 'R', "FishingArea": 'R', "ClayCollection": 'R', "PlantGathering": 'R',
		"Dungeon": 'J',
	}
	for typeName, want := range cases {
		if got := zoneCategoryGlyph(typeName); got != want {
			t.Errorf("zoneCategoryGlyph(%q) = %q, want %q", typeName, string(got), string(want))
		}
	}
}
```

Also extend the existing `TestLensGlyphsDisjointFromBaseSet` test's glyph-set coverage by adding a `"zones"` case to `lensGlyphSet` in Step 2 below — the existing table-driven test in `lenses_test.go` already walks every registered lens automatically, so no new test loop is needed for disjointness, only the new `case` in `lensGlyphSet` itself (Step 2).

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/mcpserver/... -run TestZoneCategoryGlyph -v`
Expected: FAIL — `zoneCategoryGlyph` undefined.

- [ ] **Step 3: Implement the zones lens**

```go
// internal/mcpserver/lenses.go -- add "zones" to the lenses map
	"zones": {
		Name:   "zones",
		Legend: "lens=zones: H housing (bedroom/office/tomb/dining/meeting/dormitory) K barracks A animal (pen/pond/training/archery) R resource-gather (water/dump/sand/fishing/clay/plants) J dungeon — exact owner/roster: list_zones",
		Gather: gatherZonesLens,
	},
```

```go
// internal/mcpserver/lenses.go -- lensGlyphSet, add a case
	case "zones":
		return []rune{'H', 'K', 'A', 'R', 'J'}
```

```go
// internal/mcpserver/lenses.go -- append at end of file

// zoneCategoryGlyph maps a zone type name (list_zones' "type_name" field,
// which comes straight from DFHack's own civzone_type enum key strings)
// to one of 5 category glyphs -- a handful of glyphs, not one per
// civzone_type, per the project's house rule against per-type ASCII
// budget blowout (the same philosophy buildingCategoryGlyph already
// applies to the 52 real building_type values).
func zoneCategoryGlyph(typeName string) rune {
	switch typeName {
	case "Bedroom", "Office", "Tomb", "DiningHall", "MeetingHall", "Dormitory":
		return 'H' // housing/social
	case "Barracks":
		return 'K'
	case "Pen", "Pond", "AnimalTraining", "ArcheryRange":
		return 'A' // animal/training
	case "WaterSource", "Dump", "SandCollection", "FishingArea", "ClayCollection", "PlantGathering":
		return 'R' // resource-gathering
	case "Dungeon":
		return 'J'
	default:
		return 'H'
	}
}

func gatherZonesLens(ctx context.Context, b *Bridge, s *mapview.Slice, z int16) (mapview.Overlay, error) {
	args := fmt.Sprintf(`{"z":%d}`, z)
	raw, err := b.Query(ctx, "list_zones", args)
	if err != nil {
		return mapview.Overlay{}, err
	}
	var resp struct {
		Zones []zoneListEntry `json:"zones"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return mapview.Overlay{}, fmt.Errorf("zones lens: unparseable list_zones response: %w", err)
	}
	marks := map[[2]int16]rune{}
	for _, zn := range resp.Zones {
		g := zoneCategoryGlyph(zn.TypeName)
		for x := zn.X1; x <= zn.X2; x++ {
			for y := zn.Y1; y <= zn.Y2; y++ {
				marks[[2]int16{int16(x), int16(y)}] = g
			}
		}
	}
	return mapview.Overlay{Marks: marks}, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `go test ./internal/mcpserver/... -run "TestZoneCategoryGlyph|TestLensGlyphsDisjointFromBaseSet|TestLensNamesErrorMessage" -v`
Expected: PASS.

- [ ] **Step 5: Run the full mcpserver test suite**

Run: `go test ./internal/mcpserver/... -v`
Expected: PASS, no regressions.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver/lenses.go internal/mcpserver/lenses_test.go
git commit -m "feat(mcpserver): zones look lens

Fulfils Perception round 2's explicit extensibility hook (\"the zones
lens is designed when that workstream lands\") -- one LensDef entry,
zero changes to RenderCrop or the paint pipeline. 5 category glyphs
across the 18 zone types, matching buildingCategoryGlyph's established
philosophy. Dwarf-collision footnoting applies automatically (fixed at
the look handler level in Perception round 2's post-review pass)."
```

---

## Task 9: Plugin rebuild, full regression sweep, and live-verification checkpoint

**Files:** none new — this task verifies Tasks 1-8's combined state.

- [ ] **Step 1: Full Go build and test**

Run: `go build ./... > /path/to/scratchpad/final-build.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-build.log`
Expected: exit=0, clean log.

Run: `go test ./... > /path/to/scratchpad/final-test.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-test.log`
Expected: exit=0, every package `ok` or `[no test files]`.

- [ ] **Step 2: Plugin rebuild**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release > /path/to/scratchpad/final-plugin.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-plugin.log`
Expected: exit=0. Retry once if it fails near `generate_headers` with exit `-1073741819` before treating it as a real failure.

- [ ] **Step 3: Verify artifact freshness**

Compare the mtime of `C:/Users/zmanl/Projects/dfhack-build/build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll` against the newest of `dfhack-plugin/*.cpp`/`*.h` touched across Tasks 1-4. The artifact must be newer — if not, delete it and rebuild once to force a relink.

- [ ] **Step 4: Do NOT deploy**

Leave the DLL at its build-output path only. Report the artifact path and freshness to the user for them to deploy when ready (DF must be closed — a user-facing action outside this plan's scope).

- [ ] **Step 5: Confirm the design doc's Testing section is fully covered**

Cross-check against `specs/009-culture-and-learning/design-zones.md`'s "Testing" section:
- Wire-enum translation functions — Task 5 (`TestZoneTypeConstants_MatchThePlanTable`).
- `ZoneData` decode including absent-owner/absent-roster — Task 5 (`TestEntityUpdateMessage_ZonesRoundTrip`, `TestEntityUpdateMessage_NoZonesDecodesEmpty`).
- Corrected predicates — Task 7.
- Zones lens glyph mapping + disjointness — Task 8.
- `renderZones` summary-by-type grouping + filter behavior — Task 6.
- `assign_zone` distinct error text per Component 3 bucket — covered structurally by Task 2's plugin-side branch (owner/roster/squad/unconfirmed all return distinct, named error strings); if no Go-side unit test currently exercises the actual ACK text for the squad/unconfirmed error paths (since those never reach a live DFHack call, they're pure string literals returned by the plugin, not independently testable from Go), note this explicitly as a plugin-only code path verified by code review and the live-verification checkpoint below, not a Go unit test — this is consistent with the project's established C++-has-no-unit-harness precedent.
If any gap is found beyond that one documented exception, add the missing test now rather than deferring.

- [ ] **Step 6: Live-verification checkpoint (requires the user to deploy the plugin and have DF running — coordinate with the user before this step; do not attempt it unattended)**

This is the design doc's stated completion bar, not a Go/C++ test — report readiness for it rather than executing it autonomously, since it requires a live DF session with the plugin deployed (Step 4 explicitly left undeployed). Once the user deploys and reconnects:
1. `designate_zone` a Bedroom over one of the fort's existing unassigned beds.
2. `list_zones` — confirm the new zone appears with `owner_unit_id: -1`.
3. `assign_zone` that tile to a specific dwarf's unit ID (from `dwarves`/`dwarf_detail`).
4. `list_zones` again — confirm `owner_unit_id` now matches.
5. `check_goals` — confirm `has_bedroom_zones_7` (or whichever threshold is currently registered) shows progress/`HasBedroomZones`'s evidence count increments.

- [ ] **Step 7: Final commit (if Step 5 added anything)**

```bash
git add -A
git status --short  # review before committing — confirm only expected files
git commit -m "test: close any remaining design-doc testing-section gaps"
```

(Skip this commit if Step 5 found nothing to add.)

---

## Self-review notes (from the writing-plans process)

- **Spec coverage**: all 8 design-doc components have at least one task — wire correction (1, 5), zone creation (1), zone assignment (2), list_zones (3), ENTITY_UPDATE payload (4, 5), MCP tools (6), predicate fixes (7), zones lens (8), integration (9). Component 8 (internal/zones deletion) is explicitly OUT of this plan per the design doc's own correction — captured in Global Constraints, not a task.
- **Type consistency checked**: the wire-value table at the top of this plan is referenced identically by Task 1 (C++ enum), Task 3 (list_zones type_name via `ENUM_KEY_STR`), Task 4 (ENTITY_UPDATE payload), Task 5 (Go constants), Task 6 (`zoneWireTypes` map), and Task 8 (lens glyph categories) — no task invents a different numbering. `zone_id`-based targeting was fully replaced by tile-coordinate targeting everywhere (Component 2/3's design correction) — no task references a `zone_id` parameter.
- **No placeholders**: every step has complete, concrete code. Two explicitly-flagged verify-during-implementation items remain (Task 1's floor-validation helper name, Task 2's clear-owner call signature) — both are named uncertainties with a clear resolution path (check the checkout, note the deviation), not vague TBDs, matching Perception round 2's established precedent for handling genuine implementation-time unknowns.
