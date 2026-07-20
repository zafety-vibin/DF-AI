// Entity extraction for DF AI Protocol
// Scans df.global.world.units.active and extracts entity positions

#include "Core.h"
#include "Console.h"
#include "protocol.h"
#include "modules/Units.h"

#include "df/world.h"
#include "df/unit.h"
#include "df/historical_entity.h"
#include "df/global_objects.h"
#include "df/building_civzonest.h"
#include "df/building_type.h"

#include <algorithm>
#include <vector>
#include <cstring>

using namespace DFHack;

// Forward declaration -- implemented in zones.cpp.
uint8_t wireFromCivzoneType(df::civzone_type t);

// EntityInfo structure (13 bytes per entity)
// Matches protocol.EntityInfo in Go:
// [4: ID] [2: X] [2: Y] [2: Z] [1: Type] [2: Subtype]
struct EntityInfo {
    uint32_t id;       // Unit ID
    int16_t x;         // X coordinate
    int16_t y;         // Y coordinate
    int16_t z;         // Z coordinate
    uint8_t type;      // 1=dwarf, 2=enemy, 3=animal, 4=other
    uint16_t subtype;  // Race ID
};

// Entity type constants (must match Go protocol.EntityType*)
const uint8_t ENTITY_TYPE_DWARF = 0x01;
const uint8_t ENTITY_TYPE_ENEMY = 0x02;
const uint8_t ENTITY_TYPE_ANIMAL = 0x03;
const uint8_t ENTITY_TYPE_OTHER = 0x04;

// Extract all active entities from DF. If dead_ids is non-null, it is
// appended with the id of every extracted unit that is dead per DFHack's
// own Units::isDead() definition (flags2.bits.killed || flags3.bits.ghostly
// -- confirmed against this checkout's library/modules/Units.cpp, DFHack
// 53.15-r2). Death is independent of the type classification below: a dead
// dwarf is still reported as ENTITY_TYPE_DWARF (she still IS a dwarf, just
// dead) so dwarves/dwarf_detail can say so truthfully instead of reporting
// a corpse as a living idle unit at a frozen position (the bug that
// motivated this).
std::vector<EntityInfo> extract_entities(std::vector<uint32_t> *dead_ids)
{
    std::vector<EntityInfo> entities;
    auto &console = Core::getInstance().getConsole();

    // Units are read from a background thread; suspend DF for the scan.
    // CoreSuspender is reentrant/no-op when already on the core thread.
    CoreSuspender suspend;

    auto &units = df::global::world->units.active;

    // Debug counters
    int total = 0, citizens = 0, fort_controlled = 0, hostile = 0, animals = 0, other = 0;

    for (auto unit : units) {
        if (!unit) continue;

        // Get position
        if (unit->pos.x == -30000) continue; // Invalid/off-map position

        total++;

        EntityInfo entity;
        entity.id = unit->id;
        entity.x = unit->pos.x;
        entity.y = unit->pos.y;
        entity.z = unit->pos.z;
        entity.subtype = unit->race;

        // Classify with debug output for first 3 units.
        //
        // Census bug (fixed): Units::isFortControlled() returns true for
        // ANY tame unit — its own doc comment says "Similar to isCitizen,
        // but includes tame animals" (Units.h:83), and Units.cpp:182-183
        // short-circuits `else if (unit->flags1.bits.tame) return true;`
        // before ever checking isOwnGroup/isOwnCiv. Every pet, pack animal,
        // and piece of livestock at the fort is fort-controlled AND tame,
        // so without the isAnimal() check below they fell into the
        // is_fort_ctrl branch and were stamped ENTITY_TYPE_DWARF right
        // alongside real citizens — inflating dwarf counts (and anything
        // downstream that divides by them, e.g. has_shelter_N_per_dwarf)
        // by the animal population. Units::isCitizen() is the correct
        // citizen-only check (isOwnGroup walks the historical figure's
        // entity_links for a MEMBER link to plotinfo->group_id — animals
        // essentially never have one, Units.cpp:194-205); isFortControlled
        // is only for "should this unit react to the fort's alerts/orders"
        // (ambusher visibility etc.), a broader set that legitimately
        // includes tame animals. Reference: dwarfmonitor.cpp:1315,
        // stocks.cpp:1045, preserve-rooms.cpp:333 all gate on isCitizen for
        // this exact "real dwarf" question.
        bool is_citizen = Units::isCitizen(unit);
        bool is_fort_ctrl = Units::isFortControlled(unit);
        bool is_animal = Units::isAnimal(unit);

        if (total <= 3) {
            console.print("Unit %d: civ_id=%d, citizen=%d, fort_ctrl=%d, animal=%d, race=%d\n",
                unit->id, unit->civ_id, is_citizen, is_fort_ctrl, is_animal, unit->race);
        }

        if (is_citizen) {
            entity.type = ENTITY_TYPE_DWARF;
            citizens++;
        } else if (is_fort_ctrl && !is_animal) {
            entity.type = ENTITY_TYPE_DWARF;
            fort_controlled++;
        } else if (is_fort_ctrl && is_animal) {
            // Tame pet/pack animal/livestock — fort-controlled but not a
            // citizen. See the census-bug comment above.
            entity.type = ENTITY_TYPE_ANIMAL;
            animals++;
        } else if (unit->flags1.bits.marauder || unit->flags1.bits.invader_origin ||
                   unit->flags2.bits.underworld || unit->flags2.bits.visitor_uninvited) {
            entity.type = ENTITY_TYPE_ENEMY;
            hostile++;
        } else if (unit->flags1.bits.merchant || unit->flags2.bits.visitor) {
            // Visiting merchants/diplomats are humans, not animals — this
            // branch was previously (and incorrectly) stamped ANIMAL.
            entity.type = ENTITY_TYPE_OTHER;
            other++;
        } else if (is_animal) {
            // Untamed wildlife wandering the map (not fort-controlled).
            entity.type = ENTITY_TYPE_ANIMAL;
            animals++;
        } else {
            entity.type = ENTITY_TYPE_OTHER;
            other++;
        }

        entities.push_back(entity);

        if (dead_ids && Units::isDead(unit)) {
            dead_ids->push_back(entity.id);
        }
    }

    console.print("Entity summary: %d total, %d citizens, %d fort-controlled, %d hostile, %d animals, %d other\n",
        total, citizens, fort_controlled, hostile, animals, other);

    return entities;
}

// Serialize entities to binary format
// Message structure:
// [4: Length] [1: Version] [1: Type=0x08] [4: Count] [N: Entities]
// [1: HasFortInfo] [17: FortInfo if present] [1: HasZones] [...zones]
// [1: HasDeadUnits] [4: DeadCount] [4xDeadCount: UnitID]
// [1: HasWorldIdentity] [2: SaveDirLen] [N: SaveDir] [4: ID1] [4: ID2]
//
// dead_ids lists the ids (a subset of entities' ids) that are dead per
// Units::isDead() -- see extract_entities' doc comment. Passing an empty
// vector is fine; the block is always emitted (HasDeadUnits=1, DeadCount=0)
// for uniformity with the Zones block below.
std::vector<uint8_t> serialize_entity_update(const std::vector<EntityInfo> &entities,
                                              const std::vector<uint32_t> &dead_ids)
{
    std::vector<uint8_t> buffer;

    // Reserve space for length header (will fill later)
    buffer.resize(4, 0);

    // Protocol version
    buffer.push_back(PROTOCOL_VERSION);

    // Message type (0x08 = ENTITY_UPDATE)
    buffer.push_back(0x08);

    // Entity count (uint32, big-endian)
    uint32_t count = entities.size();
    buffer.push_back((count >> 24) & 0xFF);
    buffer.push_back((count >> 16) & 0xFF);
    buffer.push_back((count >> 8) & 0xFF);
    buffer.push_back(count & 0xFF);

    // Serialize each entity (13 bytes per entity)
    for (const auto &entity : entities) {
        // ID (uint32, big-endian)
        buffer.push_back((entity.id >> 24) & 0xFF);
        buffer.push_back((entity.id >> 16) & 0xFF);
        buffer.push_back((entity.id >> 8) & 0xFF);
        buffer.push_back(entity.id & 0xFF);

        // X (int16, big-endian)
        buffer.push_back((entity.x >> 8) & 0xFF);
        buffer.push_back(entity.x & 0xFF);

        // Y (int16, big-endian)
        buffer.push_back((entity.y >> 8) & 0xFF);
        buffer.push_back(entity.y & 0xFF);

        // Z (int16, big-endian)
        buffer.push_back((entity.z >> 8) & 0xFF);
        buffer.push_back(entity.z & 0xFF);

        // Type (uint8)
        buffer.push_back(entity.type);

        // Subtype (uint16, big-endian)
        buffer.push_back((entity.subtype >> 8) & 0xFF);
        buffer.push_back(entity.subtype & 0xFF);
    }

    // Optional FortInfo block — the calendar date. Field order/widths MUST
    // match deserializeEntityUpdate in internal/protocol/codec.go:
    // [1: HasFortInfo] [4: DaysElapsed] [8: CreatedWealth] [1: Season] [4: Year]
    {
        // Calendar globals are read under suspension (CoreSuspender is
        // reentrant/no-op when the caller already holds one).
        CoreSuspender suspend;

        if (df::global::cur_year && df::global::cur_year_tick) {
            // Clamp like World::ReadCurrentTick — avoids calendar math on a
            // transiently negative tick.
            int32_t tick = std::max(0, *df::global::cur_year_tick);
            uint32_t days_elapsed = tick / 1200;   // day-of-year (0-335)
            uint8_t season = static_cast<uint8_t>(
                std::min<int32_t>(tick / 100800, 3));  // 0=spring .. 3=winter
            uint64_t created_wealth = 0;           // Not extracted yet.
            uint32_t year = static_cast<uint32_t>(*df::global::cur_year);

            buffer.push_back(1);  // HasFortInfo
            buffer.push_back((days_elapsed >> 24) & 0xFF);
            buffer.push_back((days_elapsed >> 16) & 0xFF);
            buffer.push_back((days_elapsed >> 8) & 0xFF);
            buffer.push_back(days_elapsed & 0xFF);
            buffer.push_back((created_wealth >> 56) & 0xFF);
            buffer.push_back((created_wealth >> 48) & 0xFF);
            buffer.push_back((created_wealth >> 40) & 0xFF);
            buffer.push_back((created_wealth >> 32) & 0xFF);
            buffer.push_back((created_wealth >> 24) & 0xFF);
            buffer.push_back((created_wealth >> 16) & 0xFF);
            buffer.push_back((created_wealth >> 8) & 0xFF);
            buffer.push_back(created_wealth & 0xFF);
            buffer.push_back(season);
            buffer.push_back((year >> 24) & 0xFF);
            buffer.push_back((year >> 16) & 0xFF);
            buffer.push_back((year >> 8) & 0xFF);
            buffer.push_back(year & 0xFF);
        } else {
            buffer.push_back(0);  // HasFortInfo=0 — calendar globals unavailable
        }
    }

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

            // building's x1/y1/x2/y2/z are int32_t; narrow explicitly (a
            // braced-init narrowing conversion is a hard error under this
            // build's /WX, unlike the plain-assignment case).
            int16_t coords[5] = {(int16_t)b->x1, (int16_t)b->y1,
                                  (int16_t)b->x2, (int16_t)b->y2, (int16_t)b->z};
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

    // DeadUnits block -- additive, appended after Zones. Lists unit IDs
    // (from the entity array above) that are dead. An old Go decoder that
    // doesn't know about this block simply never reads these trailing
    // bytes -- message framing is length-prefixed, so this is backward
    // compatible, the same pattern the Zones block above already
    // established. Field order/widths MUST match deserializeEntityUpdate
    // in internal/protocol/codec.go.
    // [1: HasDeadUnits] [4: DeadCount] [4xDeadCount: UnitID]
    {
        buffer.push_back(1); // HasDeadUnits
        uint32_t deadCount = (uint32_t)dead_ids.size();
        buffer.push_back((deadCount >> 24) & 0xFF);
        buffer.push_back((deadCount >> 16) & 0xFF);
        buffer.push_back((deadCount >> 8) & 0xFF);
        buffer.push_back(deadCount & 0xFF);
        for (uint32_t id : dead_ids) {
            buffer.push_back((id >> 24) & 0xFF);
            buffer.push_back((id >> 16) & 0xFF);
            buffer.push_back((id >> 8) & 0xFF);
            buffer.push_back(id & 0xFF);
        }
    }

    // WorldIdentity block -- additive, appended after DeadUnits. Fingerprints
    // the currently loaded save (df::global::world->cur_savegame) so a
    // long-lived Go peer that stays connected across a save-swap (fort
    // abandoned/completed, a different save loaded into the same running
    // DF process without restarting df-mcp) can detect the switch and stop
    // serving the PREVIOUS world's entity roster as if it were current -- a
    // live incident (see internal/worldmodel's Populator.OnEntityUpdate).
    // save_dir alone is not a safe fingerprint (two save folders across
    // reinstalls could coincidentally share a name); id1/id2
    // (shared_world_headerst, df.datafile.xml -- "based on tick at start of
    // game" / "based on tick at creation time") are a numeric pair that
    // cannot collide the same way. Field order/widths MUST match
    // deserializeEntityUpdate in internal/protocol/codec.go.
    // [1: HasWorldIdentity] [2: SaveDirLen] [N: SaveDir] [4: ID1] [4: ID2]
    {
        CoreSuspender suspend;
        bool haveWorld = df::global::world != nullptr;
        if (haveWorld) {
            const std::string &saveDir = df::global::world->cur_savegame.save_dir;
            uint32_t id1 = df::global::world->cur_savegame.world_header.id1;
            uint32_t id2 = df::global::world->cur_savegame.world_header.id2;

            buffer.push_back(1); // HasWorldIdentity
            uint16_t dirLen = (uint16_t)std::min<size_t>(saveDir.size(), 65535);
            buffer.push_back((dirLen >> 8) & 0xFF);
            buffer.push_back(dirLen & 0xFF);
            buffer.insert(buffer.end(), saveDir.begin(), saveDir.begin() + dirLen);
            buffer.push_back((id1 >> 24) & 0xFF);
            buffer.push_back((id1 >> 16) & 0xFF);
            buffer.push_back((id1 >> 8) & 0xFF);
            buffer.push_back(id1 & 0xFF);
            buffer.push_back((id2 >> 24) & 0xFF);
            buffer.push_back((id2 >> 16) & 0xFF);
            buffer.push_back((id2 >> 8) & 0xFF);
            buffer.push_back(id2 & 0xFF);
        } else {
            buffer.push_back(0); // HasWorldIdentity=0 -- world unavailable
        }
    }

    // Fill in length header (total message size)
    uint32_t length = buffer.size();
    buffer[0] = (length >> 24) & 0xFF;
    buffer[1] = (length >> 16) & 0xFF;
    buffer[2] = (length >> 8) & 0xFF;
    buffer[3] = length & 0xFF;

    return buffer;
}
