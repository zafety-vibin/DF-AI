// dfhack-plugin/zones.cpp
//
// Civzone (DF "zone") creation and the wire<->DFHack type translation.
// Assignment (applyAssignZone/applyUnassignZone) is a separate task --
// see the plan's Task 2 -- added to this same file.
//
// Wire-value table confirmed against library/include/df/civzone_type.h
// in the DFHack 53.15-r1 checkout (C:\Users\zmanl\Projects\dfhack-build).
// The previous stub's comment claimed DFHack 53.12's civzone_type had
// "NO direct Bedroom/Dining/Barracks values" -- that claim does not hold
// for 53.15-r1 (all three exist, at civzone_type values 92/80/95
// respectively); this file's table was independently re-verified against
// the actual checkout, not carried forward from that comment.
//
// Civzone-subtype wiring confirmed against
// scripts/internal/quickfort/zone.lua (create_zone, ~line 366) in the
// same checkout: DFHack's own canonical zone creator passes the
// civzone_type value as `subtype` to dfhack.buildings.constructBuilding,
// which is Lua's wrapper around the same allocInstance(pos, type,
// subtype) this file calls directly -- subtype IS how DF's real (game
// binary) virtual setSubtype() sets building_civzonest::type; DFHack's
// generated headers only declare the vtable slot (building.h:109-110),
// the actual field write happens on the other side of that virtual call.
//
// Floor/walkability check deliberately does NOT use a hypothetical
// Maps::isTileVoid/Maps::isWalkable helper (neither exists in this
// checkout's modules/Maps.h). It instead follows this codebase's own
// established pattern for tile-shape classification -- MapCache +
// tileShape() switched against the same walkable-shape set
// tile_extractor.cpp's compute_tile_flags uses for FLAG_FLOOR -- because
// that file's own comment documents that the alternative DFHack helpers
// (isWalkable/isWallTerrain) "have produced unexpected results across
// versions" and the shape-enum switch is the stable, checkable choice.

#include "Core.h"
#include "Console.h"
#include "TileTypes.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"
#include "modules/MapCache.h"

#include "df/building.h"
#include "df/building_civzonest.h"
#include "df/civzone_type.h"
#include "df/coord.h"
#include "df/tiletype_shape.h"

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

// isWalkableFloorShape mirrors tile_extractor.cpp's compute_tile_flags
// FLAG_FLOOR classification exactly, so "is this carved floor" agrees
// everywhere in the plugin.
static bool isWalkableFloorShape(df::tiletype_shape shape) {
    switch (shape) {
        case df::tiletype_shape::FLOOR:
        case df::tiletype_shape::BOULDER:
        case df::tiletype_shape::PEBBLES:
        case df::tiletype_shape::FORTIFICATION:
        case df::tiletype_shape::STAIR_UP:
        case df::tiletype_shape::STAIR_DOWN:
        case df::tiletype_shape::STAIR_UPDOWN:
        case df::tiletype_shape::RAMP:
        case df::tiletype_shape::RAMP_TOP:
        case df::tiletype_shape::BROOK_TOP:
        case df::tiletype_shape::SAPLING:
        case df::tiletype_shape::SHRUB:
        case df::tiletype_shape::TWIG:
        case df::tiletype_shape::BRANCH:
            return true;
        default:
            return false;
    }
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
    MapExtras::MapCache cache;
    for (int16_t yy = y1; yy <= y2; yy++) {
        for (int16_t xx = x1; xx <= x2; xx++) {
            df::coord pos(xx, yy, z);
            df::tiletype tt = cache.tiletypeAt(pos);
            if (!isWalkableFloorShape(tileShape(tt))) {
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

// ---------------------------------------------------------------------
// Zone assignment / unassignment (Task 2; refined 2026-07-13 -- see
// docs/decisions.md).
//
// DFHack's own source confirms exactly two assignment mechanisms:
//   - Owner (Bedroom/Office/Tomb/DiningHall): Buildings::setOwner, which
//     writes building_civzonest::assigned_unit_id and maintains the
//     unit's owned_buildings back-reference (Buildings.cpp:325-366).
//     Buildings::setOwner(zone, nullptr) is that same function's own
//     path for clearing an owner -- unit_id becomes -1, the previous
//     owner (if any) is pulled out of owned_buildings, and no new
//     owned_buildings entry is pushed since unit is null. Verified
//     directly against Buildings.cpp in the checkout; no deviation from
//     the plan needed.
//   - Roster (Pen/Pond/Dormitory): a general_ref_building_civzone_assignedst
//     on the unit plus a matching entry in the zone's own assigned_units
//     vector (building_civzonest.h:21) -- the pattern DFHack's
//     pen-assignment UI code uses for roster-style zones. Neither
//     Buildings::setOwner nor this roster write path carries any type
//     restriction anywhere in DFHack's own code (re-confirmed
//     2026-07-13) -- the Pen/Pond-only gate in plugins/zone.cpp's
//     assignUnitToZone is that function's OWN external guard clause, not
//     a DFHack API or struct-level limitation. Dormitory is wired to
//     this same mechanism because a single assigned_unit_id (Owner)
//     can't represent multiple simultaneous occupants, and
//     assigned_units is a plain vector field, the same shape as
//     Pen/Pond's. The WRITE is proven-safe (identical code path already
//     shipped and compiled for Pen/Pond); whether DF's closed-source
//     game-engine simulation actually reads assigned_units for Dormitory
//     the way it does for Pen/Pond is NOT confirmed -- same confidence
//     tier as assign_lodging
//     (specs/009-culture-and-learning/design-locations.md).
//
// Four more types have a real, now-understood reason they're not a
// unit-ownership/roster operation at all -- actively something else,
// not merely "unconfirmed":
//   - MeetingHall: not civzone ownership/roster -- its real per-fort
//     relationship goes through DF's Location system
//     (df::abstract_building / world_site), a separate subsystem from
//     building_civzonest assignment entirely.
//   - ArcheryRange/Dungeon: squad-keyed, same as Barracks --
//     building_civzonest.squad_room_info via
//     Military::updateRoomAssignments(squad_id, ...) takes a squad_id,
//     not a unit_id.
//   - AnimalTraining: labor-driven -- any dwarf with the Animal Training
//     labor uses the zone automatically when a training job comes up;
//     there is no zone-ownership/roster record for this tool to write.
// Barracks itself stays squad-based (explicitly out of scope). The
// remaining 6 zone types (WaterSource, Dump, SandCollection,
// FishingArea, ClayCollection, PlantGathering) genuinely have no
// mechanism DFHack's source confirms -- see design-zones.md Component 3.
// assign_zone/unassign_zone return a specific named error for all of the
// above rather than guessing at an unverified struct write.
// ---------------------------------------------------------------------

#include "df/unit.h"
#include "df/general_ref_building_civzone_assignedst.h"
#include <algorithm>
#include <vector>

// zoneMechanism classifies a civzone_type into how (if at all) a unit
// can be assigned to it -- see the plan's wire-table comment and
// design-zones.md Component 3 for the DFHack-source evidence behind
// each bucket. SquadRoom/Location/Labor are split out from Unconfirmed
// (2026-07-13) because those four types have a real, now-understood
// reason they're not ownership/roster operations, distinct from the
// remaining 6 types that are genuinely unconfirmed -- see the comment
// block above.
enum class ZoneMechanism { Owner, Roster, Squad, SquadRoom, Location, Labor, Unconfirmed };

static ZoneMechanism mechanismFor(df::civzone_type t) {
    switch (t) {
        case df::civzone_type::Bedroom:
        case df::civzone_type::Office:
        case df::civzone_type::Tomb:
        case df::civzone_type::DiningHall:
            return ZoneMechanism::Owner;
        case df::civzone_type::Pen:
        case df::civzone_type::Pond:
        case df::civzone_type::Dormitory:
            return ZoneMechanism::Roster;
        case df::civzone_type::Barracks:
            return ZoneMechanism::Squad;
        case df::civzone_type::ArcheryRange:
        case df::civzone_type::Dungeon:
            return ZoneMechanism::SquadRoom;
        case df::civzone_type::MeetingHall:
            return ZoneMechanism::Location;
        case df::civzone_type::AnimalTraining:
            return ZoneMechanism::Labor;
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
            // general_ref_building_civzone_assignedst's constructor is
            // protected (generated-class convention) -- `new` cannot call
            // it directly. DFHack's own plugins/zone.cpp (createCivzoneRef,
            // dead code in this checkout but the documented pattern) uses
            // virtual_identity::instantiate() instead, which is a friend
            // of the generated allocator and is the supported way to
            // construct these ref types dynamically.
            auto *ref = (df::general_ref_building_civzone_assignedst*)
                df::general_ref_building_civzone_assignedst::_identity.instantiate();
            if (!ref) {
                error = "failed to instantiate general_ref_building_civzone_assignedst";
                return false;
            }
            ref->building_id = zone->id;
            unit->general_refs.push_back(ref);
            zone->assigned_units.push_back(unitID);
            return true;
        }
        case ZoneMechanism::Squad:
            error = "Barracks assignment uses squads, not units -- not supported by this tool; see the future military/squad workstream";
            return false;
        case ZoneMechanism::SquadRoom: {
            const char *name = (zone->type == df::civzone_type::ArcheryRange) ? "ArcheryRange" : "Dungeon";
            error = std::string(name) + " assignment uses squads, not units, same as Barracks (building_civzonest.squad_room_info via Military::updateRoomAssignments) -- not supported by this tool; see the future military/squad workstream";
            return false;
        }
        case ZoneMechanism::Location:
            error = "MeetingHall assignment isn't civzone ownership or roster at all -- its real per-fort relationship goes through DF's Location system (abstract_building), a separate subsystem; use the Locations feature to manage it instead";
            return false;
        case ZoneMechanism::Labor:
            error = "AnimalTraining has no zone-ownership or roster record to assign -- it is labor-driven: any dwarf with the Animal Training labor uses the zone automatically when a training job comes up";
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
        case ZoneMechanism::SquadRoom: {
            const char *name = (zone->type == df::civzone_type::ArcheryRange) ? "ArcheryRange" : "Dungeon";
            error = std::string(name) + " assignment uses squads, not units, same as Barracks (building_civzonest.squad_room_info via Military::updateRoomAssignments) -- not supported by this tool";
            return false;
        }
        case ZoneMechanism::Location:
            error = "MeetingHall assignment isn't civzone ownership or roster at all -- its real per-fort relationship goes through DF's Location system (abstract_building), a separate subsystem; use the Locations feature to manage it instead";
            return false;
        case ZoneMechanism::Labor:
            error = "AnimalTraining has no zone-ownership or roster record to unassign -- it is labor-driven: any dwarf with the Animal Training labor uses the zone automatically when a training job comes up";
            return false;
        case ZoneMechanism::Unconfirmed:
        default:
            error = "assignment mechanism for this zone type is not confirmed against the DFHack API";
            return false;
    }
}

// Deconstruct the civzone at (x,y,z) immediately. Civzones take DFHack's
// on_civzone_delete branch inside Buildings::deconstruct (confirmed against
// library/modules/Buildings.cpp:1307-1348 in the 53.15-r1 checkout) -- no
// dwarf labor queued, unlike a constructed building (applyRemoveBuilding).
//
// Rejected if the zone founds a Location (location_id != -1): what happens
// to the Location side of that relationship when its founding civzone
// disappears out from under it is not confirmed against this checkout, so
// per house style this is an explicit named error rather than a guess --
// unassign/retire the Location first (see locations.cpp), then remove the
// zone.
bool applyRemoveZone(int16_t x, int16_t y, int16_t z, std::string &error)
{
    df::building_civzonest *zone = findZoneAt(x, y, z, error);
    if (!zone) return false;

    if (zone->location_id != -1) {
        error = "this zone founds a Location (location_id=" + std::to_string(zone->location_id) +
                ") -- removing located zones is not supported yet; unassign/retire the location first";
        return false;
    }

    if (!Buildings::deconstruct(zone)) {
        error = "Buildings::deconstruct failed to remove the zone";
        return false;
    }

    error = "zone removed immediately";
    return true;
}
