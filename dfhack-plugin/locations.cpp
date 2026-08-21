// dfhack-plugin/locations.cpp
//
// Location (Tavern/Temple/Library/Guildhall/Hospital) creation and lodging
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
#include "df/abstract_building_hospitalst.h"
#include "df/profession.h"
#include "df/world.h"
#include "df/historical_entity.h"
#include "df/entity_site_link.h"
#include "df/historical_figure.h"
#include "df/histfig_flags.h"
#include "df/religious_practice_type.h"
#include "df/religious_practice_data.h"

#include "protocol.h"

#include <cstdlib>
#include <ctime>
#include <string>

using namespace DFHack;

df::abstract_building_type abstractBuildingTypeFromWire(uint8_t wire) {
    switch (wire) {
        case LOCATION_TYPE_TAVERN:    return df::abstract_building_type::INN_TAVERN;
        case LOCATION_TYPE_TEMPLE:    return df::abstract_building_type::TEMPLE;
        case LOCATION_TYPE_LIBRARY:   return df::abstract_building_type::LIBRARY;
        case LOCATION_TYPE_GUILDHALL: return df::abstract_building_type::GUILDHALL;
        case LOCATION_TYPE_HOSPITAL:  return df::abstract_building_type::HOSPITAL;
        default:                       return df::abstract_building_type::NONE;
    }
}

uint8_t wireFromAbstractBuildingType(df::abstract_building_type t) {
    switch (t) {
        case df::abstract_building_type::INN_TAVERN: return LOCATION_TYPE_TAVERN;
        case df::abstract_building_type::TEMPLE:      return LOCATION_TYPE_TEMPLE;
        case df::abstract_building_type::LIBRARY:     return LOCATION_TYPE_LIBRARY;
        case df::abstract_building_type::GUILDHALL:   return LOCATION_TYPE_GUILDHALL;
        case df::abstract_building_type::HOSPITAL:    return LOCATION_TYPE_HOSPITAL;
        default:                                        return 0; // not one of the 5 supported types
    }
}

// applyCreateLocation converts an existing MeetingHall civzone into a
// Tavern/Temple/Library/Guildhall/Hospital Location, mirroring set_location()
// (zone.lua:300-354) step for step.
//
// hasDeity/deityHfId carry create_location's OPTIONAL temple dedication (see
// the TEMPLE case below). A presence flag rather than a -1 sentinel because
// historical-figure id 0 is a valid deity id -- "absent" and "id 0" must stay
// distinguishable. Ignored for every non-Temple type.
bool applyCreateLocation(int16_t x, int16_t y, int16_t z, uint8_t locationType, const std::string &profession,
                         bool hasDeity, int32_t deityHfId, std::string &error)
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

    // Deity dedication is a TEMPLE-only field. Reject it elsewhere instead of
    // silently dropping it -- a caller that asked for a dedication and got a
    // plain SUCCESS would reasonably believe it landed.
    if (hasDeity && abType != df::abstract_building_type::TEMPLE) {
        error = "deity dedication applies only to a temple, not "
                + std::string(ENUM_KEY_STR(abstract_building_type, abType));
        return false;
    }
    if (hasDeity) {
        df::historical_figure *deityHf = df::historical_figure::find(deityHfId);
        if (!deityHf) {
            error = "no historical figure with id " + std::to_string(deityHfId)
                    + " -- deity ids come from fort_story mode=social kind=worship (the b_id of a relation=deity edge)";
            return false;
        }
        // "The id resolves" is NOT "the id is a god". Every mortal dwarf is a
        // historical figure too, and writing deity_type=WORSHIP_HFID with a
        // mortal's hf id produces a temple state DF's own UI cannot create --
        // on a first-of-its-kind write with no DFHack reference to fall back
        // on (see the TEMPLE case below). df::histfig_flags
        // (df.history_figure.xml:998-1015) carries the real answer, and
        // BitArray::is_set (library/include/BitArray.h:156) reads it for free.
        //
        // FORCE COUNTS. DF dedicates temples to forces as well as to gods, and
        // the skeletal/rotting deity flags are deity variants -- a
        // deity-only check would over-reject legitimate dedications.
        const auto &hfFlags = deityHf->flags;
        if (!hfFlags.is_set(df::histfig_flags::deity) &&
            !hfFlags.is_set(df::histfig_flags::force) &&
            !hfFlags.is_set(df::histfig_flags::skeletal_deity) &&
            !hfFlags.is_set(df::histfig_flags::rotting_deity)) {
            error = "historical figure " + std::to_string(deityHfId)
                    + " exists but is not a deity or force -- a temple dedication requires an hf"
                      " carrying one of histfig_flags deity / force / skeletal_deity /"
                      " rotting_deity, and this one carries none of them (an ordinary mortal's"
                      " hf id looks exactly like a deity id from the outside); deity ids come"
                      " from fort_story mode=social kind=worship (the b_id of a relation=deity edge)";
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

    // Step 1b: generate a real name, mirroring zone.lua's generate_name()
    // (word_table[0][ArtImage] is the same generic adjective+noun bucket the
    // reference draws from -- confirmed by counting language_name_category,
    // ArtImage is index 35). Every Location gets a name regardless of type,
    // so this is computed once, ahead of the per-type switch below.
    static bool s_locationNameRngSeeded = false;
    if (!s_locationNameRngSeeded) {
        std::srand(static_cast<unsigned>(std::time(nullptr)));
        s_locationNameRngSeeded = true;
    }
    int32_t nameAdjWord = -1;
    int32_t nameTheXWord = -1;
    {
        df::language_word_table &wordTable =
            df::global::world->raws.language.word_table[0][df::language_name_category::ArtImage];
        auto &adjectives = wordTable.words[df::language_word_table_index::Adjectives];
        auto &theXWords = wordTable.words[df::language_word_table_index::TheX];
        if (!adjectives.empty())
            nameAdjWord = adjectives[std::rand() % adjectives.size()];
        if (!theXWords.empty())
            nameTheXWord = theXWords[std::rand() % theXWords.size()];
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
            tavern->contents.need_more.bits.goblets = true;
            tavern->contents.need_more.bits.instruments = true;
            tavern->name.has_name = true;
            tavern->name.type = df::language_name_type::FoodStore;
            tavern->name.parts_of_speech[df::language_name_component::FirstAdjective] = df::part_of_speech::Adjective;
            tavern->name.words[df::language_name_component::FirstAdjective] = nameAdjWord;
            tavern->name.words[df::language_name_component::TheX] = nameTheXWord;
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
            temple->contents.need_more.bits.instruments = true;
            // Dedication. df::abstract_building_templest carries a
            // religious_practice_type `deity_type` plus a
            // religious_practice_data UNION `deity_data`
            // (practice_id/Deity[historical_figure]/Religion[historical_entity],
            // all the same 4 bytes) -- df.abstract_building.xml:235-246,
            // df.d_basics.xml:343-347 (NONE=-1, WORSHIP_HFID=0,
            // RELIGION_ENID=1).
            //
            // Both branches WRITE both fields rather than leaning on the
            // allocator's defaults. (Those defaults are in fact benign --
            // DFHack's generated ctor is
            // `deity_type(ENUM_FIRST_ITEM(religious_practice_type))` = NONE,
            // static.ctors.inc:2188, and religious_practice_data() zeroes the
            // union -- but an explicit write is a statement instead of an
            // assumption, and it also lets the generic branch match zone.lua's
            // own Religion=-1 exactly, which the bare default did not.)
            //
            // UNVERIFIED IN A LIVE FORT: no DFHack reference implementation
            // anywhere in the 53.16-r1 checkout ever sets WORSHIP_HFID with a
            // real deity id -- quickfort's zone.lua only ever writes the
            // generic Religion=-1 case (zone.lua:155-158). This is a
            // first-of-its-kind write for this project. The figure is checked
            // to exist AND to carry a deity/force histfig flag (see the guard
            // above), but DF's remaining runtime expectations (must the deity
            // belong to this civ's pantheon? to a worshipper's religion?) are
            // not modeled. The ACK says so, and list_locations reads the two
            // written fields back so the caller can at least confirm the write
            // landed without in-game eyes.
            if (hasDeity) {
                temple->deity_type = df::religious_practice_type::WORSHIP_HFID;
                temple->deity_data.Deity = deityHfId;
            } else {
                temple->deity_type = df::religious_practice_type::NONE;
                temple->deity_data.Religion = -1; // matches zone.lua's generic temple
            }
            temple->name.has_name = true;
            temple->name.type = df::language_name_type::Temple;
            temple->name.parts_of_speech[df::language_name_component::FirstAdjective] = df::part_of_speech::Adjective;
            temple->name.words[df::language_name_component::FirstAdjective] = nameAdjWord;
            temple->name.words[df::language_name_component::TheX] = nameTheXWord;
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
            library->contents.need_more.bits.paper = true;
            library->name.has_name = true;
            library->name.type = df::language_name_type::Library;
            library->name.parts_of_speech[df::language_name_component::FirstAdjective] = df::part_of_speech::Adjective;
            library->name.words[df::language_name_component::FirstAdjective] = nameAdjWord;
            library->name.words[df::language_name_component::TheX] = nameTheXWord;
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
            // no desired_*/need_more entry for guildhall in zone.lua's
            // valid_locations table -- matches (guilds don't stock demands).
            guildhall->name.has_name = true;
            guildhall->name.type = df::language_name_type::Guildhall;
            guildhall->name.parts_of_speech[df::language_name_component::FirstAdjective] = df::part_of_speech::Adjective;
            guildhall->name.words[df::language_name_component::FirstAdjective] = nameAdjWord;
            guildhall->name.words[df::language_name_component::TheX] = nameTheXWord;
            bld = guildhall;
            break;
        }
        case df::abstract_building_type::HOSPITAL: {
            auto *hospital = (df::abstract_building_hospitalst*)
                df::abstract_building_hospitalst::_identity.instantiate();
            if (!hospital) {
                error = "failed to allocate hospital building";
                return false;
            }
            // Defaults match zone.lua's valid_locations.hospital table
            // verbatim (thread/cloth/powder/soap are pre-multiplied there --
            // see abstract_building_contents's comments for the per-unit
            // factors -- so these are the final stored values, not raw
            // item counts).
            hospital->contents.desired_splints = 5;
            hospital->contents.desired_thread = 75000;
            hospital->contents.desired_cloth = 50000;
            hospital->contents.desired_crutches = 5;
            hospital->contents.desired_powder = 750;
            hospital->contents.desired_buckets = 2;
            hospital->contents.desired_soap = 750;
            hospital->contents.need_more.bits.splints = true;
            hospital->contents.need_more.bits.thread = true;
            hospital->contents.need_more.bits.cloth = true;
            hospital->contents.need_more.bits.crutches = true;
            hospital->contents.need_more.bits.powder = true;
            hospital->contents.need_more.bits.buckets = true;
            hospital->contents.need_more.bits.soap = true;
            hospital->name.has_name = true;
            hospital->name.type = df::language_name_type::Hospital;
            hospital->name.parts_of_speech[df::language_name_component::FirstAdjective] = df::part_of_speech::Adjective;
            hospital->name.words[df::language_name_component::FirstAdjective] = nameAdjWord;
            hospital->name.words[df::language_name_component::TheX] = nameTheXWord;
            bld = hospital;
            break;
        }
        default:
            error = "unreachable: unhandled location type";
            return false;
    }

    // Step 2b: resolve site ownership (zone.lua:323-329 -- walk the site's
    // entity_links for the SiteGovernment historical_entity) and fix up the
    // base-class access-restriction flags. zone.lua's own comment calls this
    // fixup step out explicitly ("fix up BitArray flags... which don't seem
    // to get set by the insert above") -- our direct field writes here don't
    // share that Lua-marshaling quirk, but the flags themselves are the same
    // real, load-bearing state either way. Default matches zone.lua's
    // valid_restrictions.visitors preset (the reference's own default when
    // no `allow=` override is given): visitors and non-citizens welcome,
    // members-only off.
    for (auto *link : site->entity_links) {
        df::historical_entity *he = df::historical_entity::find(link->entity_id);
        if (he && he->type == df::historical_entity_type::SiteGovernment) {
            bld->site_owner_id = he->id;
            break;
        }
    }
    bld->flags.set(df::abstract_building_flags::VISITORS_ALLOWED, true);
    bld->flags.set(df::abstract_building_flags::NON_CITIZENS_ALLOWED, true);
    bld->flags.set(df::abstract_building_flags::MEMBERS_ONLY, false);

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

    if (hasDeity) {
        // Truthful ACK: the write went in, but nothing here has confirmed DF
        // itself accepts/renders this dedication (see the TEMPLE case's
        // UNVERIFIED note). Say so rather than implying a verified capability.
        error = "temple dedicated to historical figure id " + std::to_string(deityHfId)
              + " (deity_type=WORSHIP_HFID) -- UNVERIFIED: this is the first deity dedication this plugin has "
                "ever written and DFHack ships no reference implementation for it; list_locations reads the "
                "dedication back (proving the fields landed), but only in-game observation proves DF honors it";
    }
    return true;
}

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

    // Check every tavern at the current site, not just the target one --
    // assign_lodging never marks the bedroom civzone itself (a bedroom is
    // never converted into its own Location, so its location_id/site_id
    // stay unset), so the only way to detect "already lodging elsewhere"
    // is to scan every tavern's own room_info roster.
    int32_t siteID = df::global::plotinfo->site_id;
    df::world_site *site = df::world_site::find(siteID);
    if (site) {
        for (auto *bld : site->buildings) {
            if (!bld || bld->getType() != df::abstract_building_type::INN_TAVERN) continue;
            auto *otherTavern = strict_virtual_cast<df::abstract_building_inn_tavernst>(bld);
            if (!otherTavern) continue;
            for (auto *room : otherTavern->room_info) {
                if (room->civzone == bedroom->id) {
                    if (otherTavern == tavern) {
                        error = "this bedroom is already lodging for this tavern";
                    } else {
                        error = "this bedroom is already lodging for a different tavern -- unassign_lodging it first";
                    }
                    return false;
                }
            }
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

    // Fixed 2026-07-13 (final whole-branch review, Critical finding):
    // assign_lodging never sets bedroom->location_id/site_id (those fields
    // mean "this civzone IS a location's founding civzone", not "this
    // civzone is registered as someone else's lodging room"), so the
    // former fast-path guard here (bedroom->location_id == -1) was always
    // true and made unassign_lodging permanently non-functional. Resolve
    // the current site directly (same accessor applyCreateLocation uses)
    // and scan every tavern's room_info -- the only real source of truth.
    int32_t siteID = df::global::plotinfo->site_id;
    df::world_site *site = df::world_site::find(siteID);
    if (!site) {
        error = "could not resolve the current site";
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
