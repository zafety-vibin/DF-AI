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
//
// Allocation note: df::abstract_building_* subtypes are virtual_class
// types with a PROTECTED constructor (only df::allocator_fn<T> -- an
// unrelated friend template -- can call it). A plain `new
// df::abstract_building_inn_tavernst()` therefore does not compile from
// here. DFHack's own built-in plugins/zone.cpp hits the identical
// problem for a different virtual_class type and documents the fix
// inline ("!! calling new() doesn't work, need _identity.instantiate()
// instead !!", plugins/zone.cpp:240) -- allocate via the type's own
// virtual_identity::instantiate() and cast the returned virtual_ptr,
// exactly as that reference call site does.

#include "Core.h"
#include "Console.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_civzonest.h"
#include "df/civzone_type.h"
#include "df/coord.h"
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
        if (!find_enum_item(&prof, profession)) {
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
    // zone.lua's valid_locations table verbatim. Allocation goes through
    // each type's own virtual_identity::instantiate() (see file-header
    // note) rather than `new`, which the protected constructor forbids.
    df::abstract_building *bld = nullptr;
    switch (abType) {
        case df::abstract_building_type::INN_TAVERN: {
            auto *tavern = (df::abstract_building_inn_tavernst*)
                df::abstract_building_inn_tavernst::_identity.instantiate();
            if (!tavern) {
                error = "failed to allocate tavern building";
                return false;
            }
            tavern->contents.desired_goblets = 10;
            tavern->contents.desired_instruments = 5;
            bld = tavern;
            break;
        }
        case df::abstract_building_type::TEMPLE: {
            auto *temple = (df::abstract_building_templest*)
                df::abstract_building_templest::_identity.instantiate();
            if (!temple) {
                error = "failed to allocate temple building";
                return false;
            }
            temple->contents.desired_instruments = 5;
            // deity left at its default (no deity) -- matches zone.lua's Religion=-1
            bld = temple;
            break;
        }
        case df::abstract_building_type::LIBRARY: {
            auto *library = (df::abstract_building_libraryst*)
                df::abstract_building_libraryst::_identity.instantiate();
            if (!library) {
                error = "failed to allocate library building";
                return false;
            }
            library->contents.desired_paper = 10;
            bld = library;
            break;
        }
        case df::abstract_building_type::GUILDHALL: {
            auto *guildhall = (df::abstract_building_guildhallst*)
                df::abstract_building_guildhallst::_identity.instantiate();
            if (!guildhall) {
                error = "failed to allocate guildhall building";
                return false;
            }
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

    // Step 3: link the founding civzone. `contents` only exists on the
    // derived abstract_building_*st types, not the base df::abstract_building
    // that `bld` is typed as here -- go through the base class's own
    // getContents() virtual accessor (overridden per-subtype by DFHack's
    // codegen) instead of a `bld->contents` field access that would not
    // compile against the base pointer.
    bld->getContents()->building_ids.push_back(zone->id);

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
