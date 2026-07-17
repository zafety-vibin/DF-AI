# Locations Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Let the model convert an existing MeetingHall civzone into a real Location (Tavern/Temple/Library/Guildhall), list Locations, and — as its own separately-verified capability — assign a Bedroom civzone as guest lodging inside a Tavern.

**Architecture:** A new, file-disjoint plugin module (`dfhack-plugin/locations.cpp`) replicates DFHack's own quickfort reference implementation (`set_location()`) in C++: allocate an `abstract_building_*st`, link it to the founding civzone, register it via the game's own `categorize()` virtual method. Lodging assignment constructs a `df::rental_roomst` — a real struct with zero existing DFHack precedent, shipped as its own task with its own live-verification gate. Four new Go MCP tools (`create_location`, `list_locations`, `assign_lodging`, `unassign_lodging`) expose this, kept in entirely separate files from the Zones sub-project.

**Tech Stack:** Go 1.25 (`internal/protocol`, `internal/commands`, `internal/mcpserver`), C++20 DFHack plugin (`dfhack-plugin/`), existing binary TCP protocol.

## Global Constraints

- **Sequencing (read first)**: this plan assumes the Zones plan (`docs/superpowers/plans/2026-07-13-zones.md`) has fully landed and merged on `008-mcp-server` before this plan's implementation begins. Do not start Task 1 until that's true. This plan's `COMMAND_TYPE_*` numbering assumes `COMMAND_TYPE_ASSIGN_ZONE=0x10`/`COMMAND_TYPE_UNASSIGN_ZONE=0x11` are the highest currently allocated in `dfhack-plugin/protocol.h` — **verify this against the actual file before allocating new values**; if Zones' landed state differs (e.g. a review round changed a value), use the next real available bytes instead and note the deviation in your commit.
- Design of record: `specs/009-culture-and-learning/design-locations.md` — every task below implements a section of it; do not deviate without updating that doc first.
- C++: DFHack `CHECK_*` macros THROW — every new DF-touching path must be reachable only inside `executeCommand`'s or `executeQuery`'s existing try/catch.
- Go: stdout is the MCP transport in `cmd/df-mcp` paths — logging only via `logging.NewStderrTextLogger`. Every MCP tool must return readable text on a nil/disconnected bridge (`TestEveryToolNilBridge` in `internal/mcpserver/server_test.go`; tools with required inputs need a `minToolArgs` entry).
- Wire protocol changes in this plan are entirely additive-optional — no existing field's meaning changes, only new command types, a new query, and new structs.
- Ship a unit test for every pure helper. This project has no C++ unit-test harness — plugin-side verification is compile-check plus the live-verification checkpoints in the final task.
- Repo has `core.autocrlf=true` and no `.gitattributes` — avoid bulk/sed-style rewrites.
- Plugin rebuild command: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`. Artifact: `C:/Users/zmanl/Projects/dfhack-build/build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll`. Deploy requires DF closed — never deploy from within this plan.
- DFHack 53.15-r1 reference checkout: `C:\Users\zmanl\Projects\dfhack-build` (read-only reference; `plugins/df_ai_protocol` there is a junction to this repo's `dfhack-plugin/` — never edit through the junction).
- Do not touch any file the Zones plan owns (`dfhack-plugin/zones.cpp`, `dfhack-plugin/entities.cpp`, `internal/mcpserver/tools_zone.go`, `internal/predicate/library.go`, `internal/mcpserver/lenses.go`) — this plan's new plugin code lives in `dfhack-plugin/locations.cpp`, and its new Go tools live in `internal/mcpserver/tools_location.go`.
- Component C (lodging) is NOT "done" on a green build/compile alone — it requires the live-verification checkpoint in the final task showing an actual observable in-game effect, per the design doc's explicit completion bar.

---

## The wire-level location-type table (reference for every task below)

| Wire byte | Type name | `df::abstract_building_type` | Extra requirement |
|---|---|---|---|
| 0x01 | Tavern | INN_TAVERN | none |
| 0x02 | Temple | TEMPLE | none (Religion defaults to -1/none) |
| 0x03 | Library | LIBRARY | none |
| 0x04 | Guildhall | GUILDHALL | requires a `profession` name |

---

## File structure

New files:
- `dfhack-plugin/locations.cpp` — `applyCreateLocation`, `applyAssignLodging`, `applyUnassignLodging`, `wireFromAbstractBuildingType`/`abstractBuildingTypeFromWire`.
- `internal/mcpserver/tools_location.go` — `renderLocations`, `registerLocationTools`.
- `internal/mcpserver/tools_location_test.go`.

Modified files:
- `dfhack-plugin/protocol.h` — `LOCATION_TYPE_*` wire enum; `COMMAND_TYPE_CREATE_LOCATION`, `COMMAND_TYPE_ASSIGN_LODGING`, `COMMAND_TYPE_UNASSIGN_LODGING`.
- `dfhack-plugin/df_ai_protocol.cpp` — forward declarations, three new `case` blocks in `executeCommand`.
- `dfhack-plugin/queries.cpp` — new `handleListLocations`, wired into `executeQuery`.
- `internal/protocol/message.go` — `LocationType*` constants, `CommandTypeCreateLocation`/`CommandTypeAssignLodging`/`CommandTypeUnassignLodging`, `CreateLocationDesignation`/`AssignLodgingDesignation`/`UnassignLodgingDesignation` structs.
- `internal/protocol/codec.go` — encode/decode for the three new commands.
- `internal/commands/executor.go` — `SendCreateLocation`, `SendAssignLodging`, `SendUnassignLodging`.
- `internal/mcpserver/server.go` — `registerLocationTools(srv, bridge)` added to `New`.
- `internal/mcpserver/server_test.go` — `minToolArgs` entries for the 4 new tools.

---

## Task 1: Plugin — Location creation

**Files:**
- Create: `dfhack-plugin/locations.cpp`
- Modify: `dfhack-plugin/protocol.h` (new enum + command type), `dfhack-plugin/df_ai_protocol.cpp` (dispatch), `dfhack-plugin/CMakeLists.custom.txt` (add `locations.cpp` to the source list, same way `zones.cpp` was added by the Zones plan — check that file for the exact current pattern)

**Interfaces:**
- Consumes: `Buildings::findCivzonesAt` (already used by `zones.cpp`'s `applyAssignZone`), `df::global::plotinfo`, `df::world_site::find`, `df::find_enum_item<df::profession>`.
- Produces: `bool applyCreateLocation(int16_t x, int16_t y, int16_t z, uint8_t locationType, const std::string &profession, std::string &error)`.

- [ ] **Step 1: Add the wire enum and command type to `protocol.h`**

First, run `grep -n "COMMAND_TYPE_ASSIGN_ZONE\|COMMAND_TYPE_UNASSIGN_ZONE" dfhack-plugin/protocol.h` (use the Grep tool) to confirm their actual current values match this plan's assumption (`0x10`/`0x11`) — if not, adjust the values below to the next real available bytes and note it in your commit message.

Add, after the last `COMMAND_TYPE_*` constant:
```cpp
constexpr uint8_t COMMAND_TYPE_CREATE_LOCATION   = 0x12;
constexpr uint8_t COMMAND_TYPE_ASSIGN_LODGING    = 0x13;
constexpr uint8_t COMMAND_TYPE_UNASSIGN_LODGING  = 0x14;
```

Add a new enum block:
```cpp
// Location types -- DF-AI's own wire values for df::abstract_building_type's
// INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL. A Location is created FROM an
// existing MeetingHall civzone (see designate_zone), not designated
// directly -- these values only ever appear as create_location's `type`
// param. Guildhall additionally requires a profession name (see
// applyCreateLocation) -- confirmed via DFHack's own quickfort reference
// (scripts/internal/quickfort/zone.lua's set_location(), which refuses
// to create a guildhall without one); the other three need no extra input.
constexpr uint8_t LOCATION_TYPE_TAVERN    = 0x01;
constexpr uint8_t LOCATION_TYPE_TEMPLE    = 0x02;
constexpr uint8_t LOCATION_TYPE_LIBRARY   = 0x03;
constexpr uint8_t LOCATION_TYPE_GUILDHALL = 0x04;
```

- [ ] **Step 2: Write `locations.cpp` — Location creation**

```cpp
// dfhack-plugin/locations.cpp
//
// Location (Tavern/Temple/Library/Guildhall) creation and lodging
// assignment. A Location is a df::abstract_building linked to an
// existing MeetingHall civzone -- a completely separate DFHack
// subsystem from civzone ownership/roster assignment (see zones.cpp).
//
// Creation sequence confirmed against scripts/internal/quickfort/
// zone.lua's set_location() (DFHack 53.15-r1 checkout) -- this is a
// direct, verified C++ translation of a working, current DFHack
// reference implementation, not a guess.

#include "Core.h"
#include "Console.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_civzonest.h"
#include "df/world_site.h"
#include "df/plotinfost.h"
#include "df/global_objects.h"
#include "df/abstract_building.h"
#include "df/abstract_building_inn_tavernst.h"
#include "df/abstract_building_templest.h"
#include "df/abstract_building_libraryst.h"
#include "df/abstract_building_guildhallst.h"
#include "df/profession.h"

#include "protocol.h"

#include <string>

using namespace DFHack;

df::abstract_building_type abstractBuildingTypeFromWire(uint8_t wire) {
    switch (wire) {
        case LOCATION_TYPE_TAVERN:    return df::abstract_building_type::INN_TAVERN;
        case LOCATION_TYPE_TEMPLE:    return df::abstract_building_type::TEMPLE;
        case LOCATION_TYPE_LIBRARY:   return df::abstract_building_type::LIBRARY;
        case LOCATION_TYPE_GUILDHALL: return df::abstract_building_type::GUILDHALL;
        default:                       return df::abstract_building_type::NONE;
    }
}

uint8_t wireFromAbstractBuildingType(df::abstract_building_type t) {
    switch (t) {
        case df::abstract_building_type::INN_TAVERN: return LOCATION_TYPE_TAVERN;
        case df::abstract_building_type::TEMPLE:      return LOCATION_TYPE_TEMPLE;
        case df::abstract_building_type::LIBRARY:     return LOCATION_TYPE_LIBRARY;
        case df::abstract_building_type::GUILDHALL:   return LOCATION_TYPE_GUILDHALL;
        default:                                        return 0; // not one of the 4 supported types
    }
}

// applyCreateLocation converts an existing MeetingHall civzone into a
// Tavern/Temple/Library/Guildhall Location, mirroring set_location()
// (zone.lua:300-354) step for step.
bool applyCreateLocation(int16_t x, int16_t y, int16_t z, uint8_t locationType, const std::string &profession, std::string &error)
{
    df::abstract_building_type abType = abstractBuildingTypeFromWire(locationType);
    if (abType == df::abstract_building_type::NONE) {
        error = "unknown location type byte " + std::to_string((int)locationType);
        return false;
    }

    if (!Maps::isValidTilePos(x, y, z)) {
        error = "coordinates out of map bounds";
        return false;
    }
    std::vector<df::building_civzonest*> zones;
    Buildings::findCivzonesAt(&zones, df::coord(x, y, z));
    if (zones.empty()) {
        error = "no zone at (" + std::to_string(x) + "," + std::to_string(y) + "," + std::to_string(z) + ")";
        return false;
    }
    df::building_civzonest *zone = zones[0];
    if (zone->type != df::civzone_type::MeetingHall) {
        error = "zone at that tile is " + std::string(ENUM_KEY_STR(civzone_type, zone->type)) + ", not MeetingHall -- only a MeetingHall can become a Location";
        return false;
    }
    if (zone->location_id != -1) {
        error = "this zone already has a location (location_id=" + std::to_string(zone->location_id) + ") -- use list_locations to see it";
        return false;
    }

    df::profession prof = df::profession::NONE;
    if (abType == df::abstract_building_type::GUILDHALL) {
        if (profession.empty()) {
            error = "guildhall requires a profession name";
            return false;
        }
        if (!df::find_enum_item(&prof, profession)) {
            error = "unknown profession name " + profession;
            return false;
        }
    }

    // Step 1: current site.
    int32_t siteID = df::global::plotinfo->site_id;
    df::world_site *site = df::world_site::find(siteID);
    if (!site) {
        error = "could not resolve the current site";
        return false;
    }

    // Step 2: allocate the right subtype, apply defaults matching
    // zone.lua's valid_locations table verbatim.
    df::abstract_building *bld = nullptr;
    switch (abType) {
        case df::abstract_building_type::INN_TAVERN: {
            auto *tavern = new df::abstract_building_inn_tavernst();
            tavern->contents.desired_goblets = 10;
            tavern->contents.desired_instruments = 5;
            bld = tavern;
            break;
        }
        case df::abstract_building_type::TEMPLE: {
            auto *temple = new df::abstract_building_templest();
            temple->contents.desired_instruments = 5;
            // deity left at its default (no deity) -- matches zone.lua's Religion=-1
            bld = temple;
            break;
        }
        case df::abstract_building_type::LIBRARY: {
            auto *library = new df::abstract_building_libraryst();
            library->contents.desired_paper = 10;
            bld = library;
            break;
        }
        case df::abstract_building_type::GUILDHALL: {
            auto *guildhall = new df::abstract_building_guildhallst();
            guildhall->contents.profession = prof;
            bld = guildhall;
            break;
        }
        default:
            error = "unreachable: unhandled location type";
            return false;
    }

    bld->id = site->next_building_id;
    bld->site_id = site->id;
    bld->pos = site->pos;
    site->buildings.push_back(bld);
    site->next_building_id++;

    // Step 3: link the founding civzone.
    bld->contents.building_ids.push_back(zone->id);

    // Step 4: link back from the civzone.
    zone->site_id = site->id;
    zone->location_id = bld->id;

    // Step 5: register with the game's own building arrays (confirmed
    // these are df::building's own virtual methods, NOT a DFHack module
    // call -- see the design doc's correction of an earlier guess).
    zone->uncategorize();
    zone->categorize(true);

    return true;
}
```

(Step note for the implementer: verify `df::find_enum_item` is called with this exact signature in this codebase already — `internal/` history references it being used for job-type name resolution in `queue_job`; confirm the exact overload/usage pattern from that existing call site in `dfhack-plugin/work_orders.cpp` or wherever it lives, and match it rather than guessing at the signature shown above. Also verify `abstract_building_contents`'s exact field names — `desired_goblets`/`desired_instruments`/`desired_paper` — against `library/include/df/abstract_building_contents.h` in the checkout before compiling, since this plan's citations for those exact names come from the design doc's earlier research pass, not a fresh read in this task.)

- [ ] **Step 3: Wire the new command into `executeCommand` and add `locations.cpp` to the build**

Add a forward declaration in `df_ai_protocol.cpp` near the other command-handler declarations:
```cpp
bool applyCreateLocation(int16_t x, int16_t y, int16_t z, uint8_t locationType, const std::string &profession, std::string &error);
```

Add a new `case` block, following `CREATE_LOCATION`'s payload shape `[2:X][2:Y][2:Z][1:LocationType][2:ProfessionLen][N:ProfessionName]` (the trailing length-prefixed name mirrors `QUEUE_JOB`'s existing by-name trailing field):
```cpp
        case COMMAND_TYPE_CREATE_LOCATION: {
            if (payload.size() < 12) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_LOCATION payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            uint8_t locationType = payload[11];
            std::string profession;
            if (payload.size() >= 14) {
                uint16_t nameLen = ((uint16_t)payload[12] << 8) | payload[13];
                if (payload.size() >= 14 + nameLen) {
                    profession.assign(payload.begin() + 14, payload.begin() + 14 + nameLen);
                }
            }
            success = applyCreateLocation(x, y, z, locationType, profession, error);
            break;
        }
```

Add `locations.cpp` to the plugin's source file list, using the same mechanism the Zones plan used to add `zones.cpp` (check `dfhack-plugin/CMakeLists.custom.txt`).

- [ ] **Step 4: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0. Retry once if it fails near `generate_headers` with exit `-1073741819` before treating it as a real failure.

- [ ] **Step 5: Commit**

```bash
git add dfhack-plugin/locations.cpp dfhack-plugin/protocol.h dfhack-plugin/df_ai_protocol.cpp dfhack-plugin/CMakeLists.custom.txt
git commit -m "feat(plugin): create_location -- Tavern/Temple/Library/Guildhall from a MeetingHall

Direct C++ translation of DFHack's own quickfort set_location()
reference implementation, verified line-by-line against the DFHack
53.15-r1 checkout during planning research: current-site accessor via
plotinfo->site_id, manual abstract_building_*st allocation (no DFHack
helper exists for this), and categorize()/uncategorize() called
directly as df::building's own virtual methods -- not
Buildings::notifyCivzoneModified, an earlier design draft's incorrect
guess. Guildhall's profession requirement resolved via the same
find_enum_item pattern queue_job already uses for job-type names."
```

---

## Task 2: Plugin — `list_locations` query

**Files:**
- Modify: `dfhack-plugin/queries.cpp`

**Interfaces:**
- Consumes: `wireFromAbstractBuildingType` (Task 1), `jsonStr`/`jsonInt` (existing helpers).
- Produces (JSON): `{"locations":[{"id":5,"type":"Tavern","x1":10,"y1":10,"x2":11,"y2":11,"z":90,"lodging":[{"civzone_id":42,"x":15,"y":15,"z":90}]}],"truncated":false}`.

- [ ] **Step 1: Write `handleListLocations`**

```cpp
// dfhack-plugin/queries.cpp -- add near handleListZones
#include "df/world_site.h"
#include "df/plotinfost.h"
#include "df/abstract_building.h"
#include "df/abstract_building_inn_tavernst.h"

// Forward declaration -- implemented in locations.cpp (Task 1).
uint8_t wireFromAbstractBuildingType(df::abstract_building_type t);

static std::string handleListLocations(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    df::world_site *site = df::world_site::find(df::global::plotinfo->site_id);
    if (!site) {
        status = QUERY_STATUS_ERROR;
        return jsonError("could not resolve the current site");
    }

    std::ostringstream os;
    os << "{\"locations\":[";
    int count = 0;
    bool truncated = false;
    for (auto *bld : site->buildings) {
        if (!bld) continue;
        uint8_t wireKind = wireFromAbstractBuildingType(bld->getType());
        if (wireKind == 0) continue; // not one of the 4 supported types
        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        os << "{\"id\":" << jsonInt(bld->id)
           << ",\"type\":" << jsonStr(ENUM_KEY_STR(abstract_building_type, bld->getType()));

        // Founding civzone's extents, if resolvable.
        if (!bld->contents.building_ids.empty()) {
            int32_t zoneID = bld->contents.building_ids[0];
            df::building *zone = df::building::find(zoneID); // verify this exists -- see step note
            if (zone) {
                os << ",\"x1\":" << jsonInt(zone->x1) << ",\"y1\":" << jsonInt(zone->y1)
                   << ",\"x2\":" << jsonInt(zone->x2) << ",\"y2\":" << jsonInt(zone->y2)
                   << ",\"z\":" << jsonInt(zone->z);
            }
        }

        // Lodging roster -- Tavern only.
        os << ",\"lodging\":[";
        if (bld->getType() == df::abstract_building_type::INN_TAVERN) {
            auto *tavern = strict_virtual_cast<df::abstract_building_inn_tavernst>(bld);
            if (tavern) {
                for (size_t i = 0; i < tavern->room_info.size(); i++) {
                    if (i) os << ",";
                    auto *room = tavern->room_info[i];
                    os << "{\"civzone_id\":" << jsonInt(room->civzone)
                       << ",\"x\":" << jsonInt(room->world_x)
                       << ",\"y\":" << jsonInt(room->world_y)
                       << ",\"z\":" << jsonInt(room->world_z) << "}";
                }
            }
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

(Step note for the implementer: `df::building::find(id)` is used above by analogy with `df::unit::find`/`df::world_site::find`, both confirmed to exist in this checkout during planning research — but `df::building::find` itself was NOT directly confirmed. Check for it first (`grep -rn "building::find" ` in the checkout's `library/include/df/building.h`); if it doesn't exist, fall back to a linear scan of `df::global::world->buildings.all` matching `->id`, the same fallback pattern `handleListBuildings` itself already demonstrates for building iteration.)

- [ ] **Step 2: Wire into `executeQuery`'s dispatch chain**

Add one more branch to the existing `if/else if` chain, after the `list_zones` branch the Zones plan added:
```cpp
        } else if (name == "list_locations") {
            data = handleListLocations(args, status);
```

- [ ] **Step 3: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add dfhack-plugin/queries.cpp
git commit -m "feat(plugin): list_locations query

Iterates the current site's abstract_building list, filtered to the 4
supported Location types. Reports founding-civzone extents and, for
taverns, the lodging roster (room_info)."
```

---

## Task 3: Plugin — lodging assignment (Component C — separately verified)

**Files:**
- Modify: `dfhack-plugin/locations.cpp` (add `applyAssignLodging`/`applyUnassignLodging`)
- Modify: `dfhack-plugin/protocol.h` (already has the two command types from Task 1's Step 1)
- Modify: `dfhack-plugin/df_ai_protocol.cpp` (two new `case` blocks)

**Interfaces:**
- Consumes: `Buildings::findCivzonesAt` (Task 1's pattern), `abstractBuildingTypeFromWire`.
- Produces: `bool applyAssignLodging(int16_t tavernX, int16_t tavernY, int16_t tavernZ, int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error)`, `bool applyUnassignLodging(int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error)`.

- [ ] **Step 1: Append lodging assignment to `locations.cpp`**

```cpp
// dfhack-plugin/locations.cpp -- append
#include "df/rental_roomst.h"

// findTavernAt resolves the Tavern Location whose founding civzone sits
// at (x,y,z). Returns null with error set if the tile isn't a
// Location-linked civzone, or the linked Location isn't a Tavern.
static df::abstract_building_inn_tavernst* findTavernAt(int16_t x, int16_t y, int16_t z, std::string &error) {
    std::vector<df::building_civzonest*> zones;
    Buildings::findCivzonesAt(&zones, df::coord(x, y, z));
    if (zones.empty()) {
        error = "no zone at (" + std::to_string(x) + "," + std::to_string(y) + "," + std::to_string(z) + ")";
        return nullptr;
    }
    df::building_civzonest *zone = zones[0];
    if (zone->location_id == -1) {
        error = "this zone has no location -- use create_location first";
        return nullptr;
    }
    df::world_site *site = df::world_site::find(zone->site_id);
    if (!site) {
        error = "could not resolve the zone's site";
        return nullptr;
    }
    for (auto *bld : site->buildings) {
        if (bld && bld->id == zone->location_id) {
            if (bld->getType() != df::abstract_building_type::INN_TAVERN) {
                error = "the location at that tile is " + std::string(ENUM_KEY_STR(abstract_building_type, bld->getType())) + ", not a Tavern -- only taverns take lodging";
                return nullptr;
            }
            return strict_virtual_cast<df::abstract_building_inn_tavernst>(bld);
        }
    }
    error = "could not resolve the zone's linked location";
    return nullptr;
}

// findBedroomZoneAt resolves the Bedroom civzone at (x,y,z).
static df::building_civzonest* findBedroomZoneAt(int16_t x, int16_t y, int16_t z, std::string &error) {
    std::vector<df::building_civzonest*> zones;
    Buildings::findCivzonesAt(&zones, df::coord(x, y, z));
    if (zones.empty()) {
        error = "no zone at (" + std::to_string(x) + "," + std::to_string(y) + "," + std::to_string(z) + ")";
        return nullptr;
    }
    if (zones[0]->type != df::civzone_type::Bedroom) {
        error = "zone at that tile is " + std::string(ENUM_KEY_STR(civzone_type, zones[0]->type)) + ", not Bedroom";
        return nullptr;
    }
    return zones[0];
}

// applyAssignLodging is genuinely first-of-its-kind: no DFHack script or
// plugin anywhere in the 53.15-r1 checkout reads or writes
// rental_roomst/room_info. The struct layout is confirmed
// (abstract_building_inn_tavernst.room_info, rental_roomst.civzone as a
// building-id reference), but whether this write actually produces
// observable in-game lodging behavior is UNVERIFIED until a live DF
// session confirms it -- see the plan's final task.
bool applyAssignLodging(int16_t tavernX, int16_t tavernY, int16_t tavernZ, int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error)
{
    auto *tavern = findTavernAt(tavernX, tavernY, tavernZ, error);
    if (!tavern) return false;
    df::building_civzonest *bedroom = findBedroomZoneAt(bedroomX, bedroomY, bedroomZ, error);
    if (!bedroom) return false;

    for (auto *room : tavern->room_info) {
        if (room->civzone == bedroom->id) {
            error = "this bedroom is already lodging for this tavern";
            return false;
        }
    }

    auto *room = new df::rental_roomst();
    room->id = tavern->next_room_info_id++;
    room->civzone = bedroom->id;
    room->world_x = bedroomX;
    room->world_y = bedroomY;
    room->world_z = bedroomZ;
    tavern->room_info.push_back(room);
    return true;
}

bool applyUnassignLodging(int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error)
{
    df::building_civzonest *bedroom = findBedroomZoneAt(bedroomX, bedroomY, bedroomZ, error);
    if (!bedroom) return false;

    if (bedroom->location_id == -1) {
        error = "this bedroom is not lodging for any tavern";
        return false;
    }
    df::world_site *site = df::world_site::find(bedroom->site_id);
    if (!site) {
        error = "could not resolve the bedroom's site";
        return false;
    }
    for (auto *bld : site->buildings) {
        if (!bld || bld->getType() != df::abstract_building_type::INN_TAVERN) continue;
        auto *tavern = strict_virtual_cast<df::abstract_building_inn_tavernst>(bld);
        if (!tavern) continue;
        for (size_t i = 0; i < tavern->room_info.size(); i++) {
            if (tavern->room_info[i]->civzone == bedroom->id) {
                delete tavern->room_info[i];
                tavern->room_info.erase(tavern->room_info.begin() + i);
                return true;
            }
        }
    }
    error = "this bedroom is not assigned as lodging for any tavern";
    return false;
}
```

(Step note: `applyUnassignLodging` scans by `bedroom->site_id`, but a bedroom that was never linked to a location at all has `location_id == -1` and is rejected before that scan — however, a bedroom whose `location_id` was set some other way without an actual `room_info` entry existing would fall through to the "not assigned" error at the end, which is correct/intended behavior, not a bug — the explicit early check is just a fast path for the common case.)

- [ ] **Step 2: Wire the two new commands into `executeCommand`**

Forward declarations:
```cpp
bool applyAssignLodging(int16_t tavernX, int16_t tavernY, int16_t tavernZ, int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error);
bool applyUnassignLodging(int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error);
```

```cpp
        case COMMAND_TYPE_ASSIGN_LODGING: {
            // Payload: [4: cmdID] [1: cmdType] [2: TavernX] [2: TavernY] [2: TavernZ] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
            if (payload.size() < 17) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_LODGING payload");
                return;
            }
            int16_t tx = ((int16_t)payload[5] << 8) | payload[6];
            int16_t ty = ((int16_t)payload[7] << 8) | payload[8];
            int16_t tz = ((int16_t)payload[9] << 8) | payload[10];
            int16_t bx = ((int16_t)payload[11] << 8) | payload[12];
            int16_t by = ((int16_t)payload[13] << 8) | payload[14];
            int16_t bz = ((int16_t)payload[15] << 8) | payload[16];
            success = applyAssignLodging(tx, ty, tz, bx, by, bz, error);
            break;
        }
        case COMMAND_TYPE_UNASSIGN_LODGING: {
            // Payload: [4: cmdID] [1: cmdType] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNASSIGN_LODGING payload");
                return;
            }
            int16_t bx = ((int16_t)payload[5] << 8) | payload[6];
            int16_t by = ((int16_t)payload[7] << 8) | payload[8];
            int16_t bz = ((int16_t)payload[9] << 8) | payload[10];
            success = applyUnassignLodging(bx, by, bz, error);
            break;
        }
```

- [ ] **Step 3: Rebuild and check for compile errors**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release`
Expected: exit 0.

- [ ] **Step 4: Commit**

```bash
git add dfhack-plugin/locations.cpp dfhack-plugin/df_ai_protocol.cpp
git commit -m "feat(plugin): assign_lodging/unassign_lodging -- bedroom-as-tavern-guest-room

First-of-its-kind: no DFHack script or plugin anywhere in the
53.15-r1 checkout exercises rental_roomst/room_info. Struct layout is
confirmed (abstract_building_inn_tavernst.room_info ->
rental_roomst.civzone, a building-id reference), the write is a plain
vector push with no complex invariants, but whether it produces an
observable in-game effect is unverified until the live-verification
checkpoint in this plan's final task."
```

---

## Task 4: Go — wire protocol

**Files:**
- Modify: `internal/protocol/message.go`, `internal/protocol/codec.go`
- Test: `internal/protocol/codec_test.go` (already exists per the Zones plan; append)

**Interfaces:**
- Produces: `protocol.LocationTypeTavern/Temple/Library/Guildhall` (uint8, matching the plan's wire table), `protocol.CommandTypeCreateLocation/AssignLodging/UnassignLodging`, `protocol.CreateLocationDesignation{X,Y,Z int16, LocationType uint8, Profession string}`, `protocol.AssignLodgingDesignation{TavernX,TavernY,TavernZ,BedroomX,BedroomY,BedroomZ int16}`, `protocol.UnassignLodgingDesignation{BedroomX,BedroomY,BedroomZ int16}`.

- [ ] **Step 1: Write the failing tests**

```go
// internal/protocol/codec_test.go -- append
func TestLocationTypeConstants_MatchThePlanTable(t *testing.T) {
	cases := map[uint8]uint8{
		LocationTypeTavern: 0x01, LocationTypeTemple: 0x02,
		LocationTypeLibrary: 0x03, LocationTypeGuildhall: 0x04,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("constant value mismatch: got 0x%02X, want 0x%02X", got, want)
		}
	}
}

func TestEncodeDecodeCreateLocationCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:      1,
		CommandType:    CommandTypeCreateLocation,
		CreateLocation: CreateLocationDesignation{X: 10, Y: 20, Z: 90, LocationType: LocationTypeGuildhall, Profession: "CARPENTER"},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.CreateLocation != msg.CreateLocation {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.CreateLocation, msg.CreateLocation)
	}
}

func TestEncodeDecodeCreateLocationCommand_NoProfession(t *testing.T) {
	msg := &CommandMessage{
		CommandID:      2,
		CommandType:    CommandTypeCreateLocation,
		CreateLocation: CreateLocationDesignation{X: 5, Y: 5, Z: 90, LocationType: LocationTypeTavern, Profession: ""},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.CreateLocation.Profession != "" {
		t.Fatalf("expected empty profession, got %q", decoded.CreateLocation.Profession)
	}
}

func TestEncodeDecodeAssignLodgingCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:     3,
		CommandType:   CommandTypeAssignLodging,
		AssignLodging: AssignLodgingDesignation{TavernX: 1, TavernY: 2, TavernZ: 3, BedroomX: 4, BedroomY: 5, BedroomZ: 6},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.AssignLodging != msg.AssignLodging {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.AssignLodging, msg.AssignLodging)
	}
}

func TestEncodeDecodeUnassignLodgingCommand(t *testing.T) {
	msg := &CommandMessage{
		CommandID:       4,
		CommandType:     CommandTypeUnassignLodging,
		UnassignLodging: UnassignLodgingDesignation{BedroomX: 4, BedroomY: 5, BedroomZ: 6},
	}
	encoded, err := EncodeCommand(msg)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeCommand(encoded)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded.UnassignLodging != msg.UnassignLodging {
		t.Fatalf("round-trip mismatch: got %+v, want %+v", decoded.UnassignLodging, msg.UnassignLodging)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `go test ./internal/protocol/... -run "TestLocationTypeConstants|TestEncodeDecodeCreateLocation|TestEncodeDecodeAssignLodging|TestEncodeDecodeUnassignLodging" -v`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Add the constants and structs to `message.go`**

```go
// internal/protocol/message.go -- add near the ZoneType* block
// LocationType constants -- DF-AI's own wire values for
// df::abstract_building_type's INN_TAVERN/TEMPLE/LIBRARY/GUILDHALL.
// A Location is created FROM an existing MeetingHall civzone via
// create_location, not designated directly.
const (
	LocationTypeTavern    uint8 = 0x01
	LocationTypeTemple    uint8 = 0x02
	LocationTypeLibrary   uint8 = 0x03
	LocationTypeGuildhall uint8 = 0x04 // requires CreateLocationDesignation.Profession
)

// CreateLocationDesignation targets the MeetingHall civzone at (X,Y,Z)
// and converts it into a Location of LocationType. Profession is
// required only when LocationType is LocationTypeGuildhall.
type CreateLocationDesignation struct {
	X, Y, Z      int16
	LocationType uint8
	Profession   string
}

// AssignLodgingDesignation links the Bedroom civzone at
// (BedroomX,BedroomY,BedroomZ) as guest lodging inside the Tavern
// Location founded by the civzone at (TavernX,TavernY,TavernZ).
type AssignLodgingDesignation struct {
	TavernX, TavernY, TavernZ    int16
	BedroomX, BedroomY, BedroomZ int16
}

// UnassignLodgingDesignation removes the Bedroom civzone at
// (BedroomX,BedroomY,BedroomZ) from whichever tavern it's lodging for.
type UnassignLodgingDesignation struct {
	BedroomX, BedroomY, BedroomZ int16
}
```

Add the three new command types after the `CommandTypeUnassignZone` constant added by the Zones plan (check the actual current value first — this plan assumes `0x11` is the last one used, per Global Constraints):
```go
	CommandTypeCreateLocation  uint8 = 0x12
	CommandTypeAssignLodging   uint8 = 0x13
	CommandTypeUnassignLodging uint8 = 0x14
```

Add the three new fields to `CommandMessage` (alongside the `AssignZone`/`UnassignZone` fields the Zones plan added):
```go
	CreateLocation  CreateLocationDesignation
	AssignLodging   AssignLodgingDesignation
	UnassignLodging UnassignLodgingDesignation
```

- [ ] **Step 4: Add codec encode/decode**

In `codec.go`, add three new `case` blocks to the command-encoding switch (alongside `CommandTypeAssignZone`/`CommandTypeUnassignZone`):
```go
	case CommandTypeCreateLocation:
		// [2: X] [2: Y] [2: Z] [1: LocationType] [2: ProfessionLen] [N: Profession]
		for _, v := range []int16{msg.CreateLocation.X, msg.CreateLocation.Y, msg.CreateLocation.Z} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
		if err := binary.Write(w, binary.BigEndian, msg.CreateLocation.LocationType); err != nil {
			return err
		}
		profBytes := []byte(msg.CreateLocation.Profession)
		if err := binary.Write(w, binary.BigEndian, uint16(len(profBytes))); err != nil {
			return err
		}
		if _, err := w.Write(profBytes); err != nil {
			return err
		}
	case CommandTypeAssignLodging:
		// [2: TavernX] [2: TavernY] [2: TavernZ] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
		for _, v := range []int16{
			msg.AssignLodging.TavernX, msg.AssignLodging.TavernY, msg.AssignLodging.TavernZ,
			msg.AssignLodging.BedroomX, msg.AssignLodging.BedroomY, msg.AssignLodging.BedroomZ,
		} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
	case CommandTypeUnassignLodging:
		// [2: BedroomX] [2: BedroomY] [2: BedroomZ]
		for _, v := range []int16{msg.UnassignLodging.BedroomX, msg.UnassignLodging.BedroomY, msg.UnassignLodging.BedroomZ} {
			if err := binary.Write(w, binary.BigEndian, v); err != nil {
				return err
			}
		}
```

And the matching decode cases:
```go
	case CommandTypeCreateLocation:
		for _, p := range []*int16{&msg.CreateLocation.X, &msg.CreateLocation.Y, &msg.CreateLocation.Z} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.CreateLocation.LocationType); err != nil {
			return nil, err
		}
		var profLen uint16
		if err := binary.Read(buf, binary.BigEndian, &profLen); err != nil {
			return nil, err
		}
		if profLen > 0 {
			profBytes := make([]byte, profLen)
			if _, err := io.ReadFull(buf, profBytes); err != nil {
				return nil, err
			}
			msg.CreateLocation.Profession = string(profBytes)
		}
	case CommandTypeAssignLodging:
		for _, p := range []*int16{
			&msg.AssignLodging.TavernX, &msg.AssignLodging.TavernY, &msg.AssignLodging.TavernZ,
			&msg.AssignLodging.BedroomX, &msg.AssignLodging.BedroomY, &msg.AssignLodging.BedroomZ,
		} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
	case CommandTypeUnassignLodging:
		for _, p := range []*int16{&msg.UnassignLodging.BedroomX, &msg.UnassignLodging.BedroomY, &msg.UnassignLodging.BedroomZ} {
			if err := binary.Read(buf, binary.BigEndian, p); err != nil {
				return nil, err
			}
		}
```
(`io` must already be imported in `codec.go` for `io.ReadFull` — check first; it's very likely already imported given the file's existing use of `io.Writer`/`bytes.Reader`.)

- [ ] **Step 5: Run tests to verify they pass**

Run: `go test ./internal/protocol/... -v`
Expected: PASS — all new tests plus full existing suite.

- [ ] **Step 6: Commit**

```bash
git add internal/protocol/message.go internal/protocol/codec.go internal/protocol/codec_test.go
git commit -m "feat(protocol): Location wire types and create/assign/unassign commands"
```

---

## Task 5: Go — MCP tools

**Files:**
- Create: `internal/mcpserver/tools_location.go`
- Create: `internal/mcpserver/tools_location_test.go`
- Modify: `internal/commands/executor.go` (add `SendCreateLocation`, `SendAssignLodging`, `SendUnassignLodging`)
- Modify: `internal/mcpserver/server.go` (add `registerLocationTools(srv, bridge)`)
- Modify: `internal/mcpserver/server_test.go` (add `minToolArgs` entries)

**Interfaces:**
- Consumes: `protocol.LocationType*`, `protocol.CreateLocationDesignation`/`AssignLodgingDesignation`/`UnassignLodgingDesignation` (Task 4), `ackText`/`withDash`/`noExec` (existing).
- Produces: `func renderLocations(raw []byte) string`, `func registerLocationTools(srv *mcp.Server, b *Bridge)`.

- [ ] **Step 1: Add the executor methods**

```go
// internal/commands/executor.go -- add near SendAssignZone/SendUnassignZone (added by the Zones plan)
func (e *CommandExecutor) SendCreateLocation(x, y, z int16, locationType uint8, profession string) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeCreateLocation,
		CreateLocation: protocol.CreateLocationDesignation{
			X: x, Y: y, Z: z, LocationType: locationType, Profession: profession,
		},
	}
	return e.SendCommand(cmd)
}

func (e *CommandExecutor) SendAssignLodging(tavernX, tavernY, tavernZ, bedroomX, bedroomY, bedroomZ int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeAssignLodging,
		AssignLodging: protocol.AssignLodgingDesignation{
			TavernX: tavernX, TavernY: tavernY, TavernZ: tavernZ,
			BedroomX: bedroomX, BedroomY: bedroomY, BedroomZ: bedroomZ,
		},
	}
	return e.SendCommand(cmd)
}

func (e *CommandExecutor) SendUnassignLodging(bedroomX, bedroomY, bedroomZ int16) (*CommandResult, error) {
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypeUnassignLodging,
		UnassignLodging: protocol.UnassignLodgingDesignation{
			BedroomX: bedroomX, BedroomY: bedroomY, BedroomZ: bedroomZ,
		},
	}
	return e.SendCommand(cmd)
}
```

- [ ] **Step 2: Write the failing tests for `renderLocations`**

```go
// internal/mcpserver/tools_location_test.go
package mcpserver

import (
	"strings"
	"testing"
)

func TestRenderLocations_Basic(t *testing.T) {
	raw := []byte(`{"locations":[
		{"id":5,"type":"INN_TAVERN","x1":10,"y1":10,"x2":11,"y2":11,"z":90,"lodging":[{"civzone_id":42,"x":15,"y":15,"z":90}]},
		{"id":6,"type":"TEMPLE","x1":20,"y1":20,"x2":21,"y2":21,"z":90,"lodging":[]}
	]}`)
	out := renderLocations(raw)
	if !strings.Contains(out, "Tavern") || !strings.Contains(out, "(10,10)") {
		t.Fatalf("expected the tavern's type and extents, got:\n%s", out)
	}
	if !strings.Contains(out, "1 lodging room") {
		t.Fatalf("expected a lodging count for the tavern, got:\n%s", out)
	}
	if !strings.Contains(out, "Temple") {
		t.Fatalf("expected the temple, got:\n%s", out)
	}
}

func TestRenderLocations_NoLocations(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[]}`))
	if out != "No locations." {
		t.Fatalf("expected 'No locations.', got %q", out)
	}
}

func TestRenderLocations_Truncated(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[{"id":1,"type":"LIBRARY","x1":1,"y1":1,"x2":1,"y2":1,"z":1,"lodging":[]}],"truncated":true}`))
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `go test ./internal/mcpserver/... -run TestRenderLocations -v`
Expected: FAIL — `renderLocations` undefined.

- [ ] **Step 4: Implement `tools_location.go`**

```go
// internal/mcpserver/tools_location.go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/protocol"
)

var locationWireTypes = map[string]uint8{
	"tavern": protocol.LocationTypeTavern, "temple": protocol.LocationTypeTemple,
	"library": protocol.LocationTypeLibrary, "guildhall": protocol.LocationTypeGuildhall,
}

// locationTypeDisplayNames maps the plugin's ENUM_KEY_STR(abstract_building_type, ...)
// names to the friendlier names this tool surfaces.
var locationTypeDisplayNames = map[string]string{
	"INN_TAVERN": "Tavern", "TEMPLE": "Temple", "LIBRARY": "Library", "GUILDHALL": "Guildhall",
}

type lodgingEntry struct {
	CivzoneID int `json:"civzone_id"`
	X         int `json:"x"`
	Y         int `json:"y"`
	Z         int `json:"z"`
}

type locationListEntry struct {
	ID      int            `json:"id"`
	Type    string         `json:"type"`
	X1      int            `json:"x1"`
	Y1      int            `json:"y1"`
	X2      int            `json:"x2"`
	Y2      int            `json:"y2"`
	Z       int            `json:"z"`
	Lodging []lodgingEntry `json:"lodging"`
}

func renderLocations(raw []byte) string {
	var resp struct {
		Locations []locationListEntry `json:"locations"`
		Truncated bool                `json:"truncated"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return fmt.Sprintf("unparseable list_locations response: %v\nraw: %s", err, capRawJSON(string(raw)))
	}
	if len(resp.Locations) == 0 {
		return "No locations."
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "%d locations:\n", len(resp.Locations))
	for _, loc := range resp.Locations {
		name := locationTypeDisplayNames[loc.Type]
		if name == "" {
			name = loc.Type
		}
		fmt.Fprintf(&sb, "- %s at (%d,%d)-(%d,%d) z=%d", name, loc.X1, loc.Y1, loc.X2, loc.Y2, loc.Z)
		if len(loc.Lodging) > 0 {
			fmt.Fprintf(&sb, " — %d lodging room(s)", len(loc.Lodging))
		}
		sb.WriteString("\n")
	}
	if resp.Truncated {
		sb.WriteString("... list truncated at the plugin's cap\n")
	}
	return sb.String()
}

func registerLocationTools(srv *mcp.Server, b *Bridge) {
	type createLocationIn struct {
		X          int    `json:"x" jsonschema:"a tile inside the founding MeetingHall zone"`
		Y          int    `json:"y"`
		Z          int    `json:"z"`
		Type       string `json:"type" jsonschema:"tavern|temple|library|guildhall"`
		Profession string `json:"profession,omitempty" jsonschema:"required for guildhall only — the profession this guild serves, e.g. CARPENTER, MASON"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_location",
		Description: "Convert an existing MeetingHall zone (see designate_zone) into a real Location — a Tavern (social/entertainment, and can house lodging via assign_lodging), Temple, Library, or Guildhall (requires a profession). The MeetingHall itself must already exist and not already have a location.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createLocationIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		lt, ok := locationWireTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown location type %q", in.Type)), nil, nil
		}
		res, err := b.Exec.SendCreateLocation(int16(in.X), int16(in.Y), int16(in.Z), lt, in.Profession)
		what := fmt.Sprintf("create_location %s at (%d,%d,%d)", in.Type, in.X, in.Y, in.Z)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_locations",
		Description: "List every Tavern/Temple/Library/Guildhall Location, with the founding zone's extents and (for taverns) the lodging roster.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "list_locations", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, renderLocations(raw)), nil, nil
	})

	type assignLodgingIn struct {
		TavernX  int `json:"tavern_x" jsonschema:"a tile inside the tavern's founding zone"`
		TavernY  int `json:"tavern_y"`
		TavernZ  int `json:"tavern_z"`
		BedroomX int `json:"bedroom_x" jsonschema:"a tile inside the bedroom zone to make into a guest room"`
		BedroomY int `json:"bedroom_y"`
		BedroomZ int `json:"bedroom_z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "assign_lodging",
		Description: "Link a Bedroom zone as guest lodging inside a Tavern. First-of-its-kind capability for this project — verify with list_locations and in-game observation that it behaves as expected, don't assume it silently works just because the ACK is SUCCESS.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in assignLodgingIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendAssignLodging(int16(in.TavernX), int16(in.TavernY), int16(in.TavernZ), int16(in.BedroomX), int16(in.BedroomY), int16(in.BedroomZ))
		what := fmt.Sprintf("assign_lodging tavern(%d,%d,%d) bedroom(%d,%d,%d)", in.TavernX, in.TavernY, in.TavernZ, in.BedroomX, in.BedroomY, in.BedroomZ)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type unassignLodgingIn struct {
		BedroomX int `json:"bedroom_x"`
		BedroomY int `json:"bedroom_y"`
		BedroomZ int `json:"bedroom_z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unassign_lodging",
		Description: "Remove a bedroom's tavern-lodging assignment.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in unassignLodgingIn) (*mcp.CallToolResult, any, error) {
		if r := noExec(b); r != nil {
			return r, nil, nil
		}
		res, err := b.Exec.SendUnassignLodging(int16(in.BedroomX), int16(in.BedroomY), int16(in.BedroomZ))
		what := fmt.Sprintf("unassign_lodging bedroom(%d,%d,%d)", in.BedroomX, in.BedroomY, in.BedroomZ)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})
}
```

- [ ] **Step 5: Wire `registerLocationTools` into `server.go`**

```go
// internal/mcpserver/server.go -- New(), add alongside the other register*Tools calls
	registerLocationTools(srv, bridge)
```

- [ ] **Step 6: Add `minToolArgs` entries**

```go
// internal/mcpserver/server_test.go -- minToolArgs map, add
	"create_location":  {"x": 1, "y": 1, "z": 1, "type": "tavern"},
	"assign_lodging":   {"tavern_x": 1, "tavern_y": 1, "tavern_z": 1, "bedroom_x": 2, "bedroom_y": 2, "bedroom_z": 2},
	"unassign_lodging": {"bedroom_x": 2, "bedroom_y": 2, "bedroom_z": 2},
	// list_locations has no required fields — {} default is fine, no entry needed.
```

- [ ] **Step 7: Run the tests**

Run: `go test ./internal/mcpserver/... -run "TestRenderLocations|TestEveryToolNilBridge" -v`
Expected: PASS.

- [ ] **Step 8: Run the full test suite**

Run: `go build ./... && go test ./... > /path/to/scratchpad/task5.log 2>&1; echo exit=$?; cat /path/to/scratchpad/task5.log`
Expected: exit=0.

- [ ] **Step 9: Commit**

```bash
git add internal/mcpserver/tools_location.go internal/mcpserver/tools_location_test.go internal/commands/executor.go internal/mcpserver/server.go internal/mcpserver/server_test.go
git commit -m "feat(mcpserver): create_location / list_locations / assign_lodging / unassign_lodging"
```

---

## Task 6: Plugin rebuild, full regression sweep, and live-verification checkpoints

**Files:** none new — this task verifies Tasks 1-5's combined state.

- [ ] **Step 1: Full Go build and test**

Run: `go build ./... > /path/to/scratchpad/final-build.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-build.log`
Expected: exit=0.

Run: `go test ./... > /path/to/scratchpad/final-test.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-test.log`
Expected: exit=0.

- [ ] **Step 2: Plugin rebuild**

Run: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release > /path/to/scratchpad/final-plugin.log 2>&1; echo exit=$?; cat /path/to/scratchpad/final-plugin.log`
Expected: exit=0.

- [ ] **Step 3: Verify artifact freshness**

Compare the mtime of `df_ai_protocol.plug.dll` against the newest of `dfhack-plugin/*.cpp`/`*.h` touched across Tasks 1-3. Force a relink if stale.

- [ ] **Step 4: Do NOT deploy**

Report the artifact path for the user to deploy when ready.

- [ ] **Step 5: Live-verification checkpoint A — Location creation (coordinate with the user; requires a deployed plugin and running DF)**

1. `designate_zone` a MeetingHall (if one doesn't already exist).
2. `create_location` it into a Tavern.
3. `list_locations` — confirm it appears with the correct type and extents.
4. Report this as the basic completion bar for Component A/B.

- [ ] **Step 6: Live-verification checkpoint B — lodging (Component C's stricter bar, do NOT skip)**

1. `designate_zone` a Bedroom near the Tavern (or reuse an existing one).
2. `assign_lodging` linking them.
3. `list_locations` — confirm the bedroom's civzone id appears in the tavern's lodging roster.
4. **Critically**: observe actual in-game behavior over a few `step()` calls and/or DF's own Locations UI (the user may need to check this manually in-client) to see whether the lodging assignment has any observable effect (a visitor/traveler using the room, the room appearing correctly in DF's own room list, etc.) — a clean ACK and a correct `list_locations` report are NOT sufficient to call this done, per the design doc's explicit completion bar. Report the actual observed outcome, including "no observable effect found" as a legitimate, reportable result rather than something to paper over.

- [ ] **Step 7: Final commit (if anything was added while closing gaps)**

```bash
git add -A
git status --short
git commit -m "test: close any remaining design-doc testing-section gaps"
```

(Skip if nothing to add.)

---

## Self-review notes (from the writing-plans process)

- **Spec coverage**: all 3 design-doc components have tasks — creation (1), listing (2), lodging (3), wire protocol (4), MCP tools (5), integration + both live-verification tiers (6).
- **Type consistency checked**: the wire-value table matches identically across Task 1 (C++ enum), Task 2 (`ENUM_KEY_STR` display), Task 4 (Go constants), and Task 5 (`locationWireTypes` map). Zones are targeted by tile coordinate throughout — no synthetic ID anywhere, consistent with the Zones sub-project's own established convention.
- **No placeholders**: every step has complete, concrete code. Three explicitly-flagged verify-during-implementation items remain (`df::find_enum_item`'s exact call signature, `abstract_building_contents`'s exact field names, `df::building::find`'s existence) — each is a small, bounded check against source with a stated fallback, not an open-ended guess.
