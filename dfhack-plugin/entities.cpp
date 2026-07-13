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

#include <algorithm>
#include <vector>
#include <cstring>

using namespace DFHack;

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

// Extract all active entities from DF
std::vector<EntityInfo> extract_entities()
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
    }

    console.print("Entity summary: %d total, %d citizens, %d fort-controlled, %d hostile, %d animals, %d other\n",
        total, citizens, fort_controlled, hostile, animals, other);

    return entities;
}

// Serialize entities to binary format
// Message structure:
// [4: Length] [1: Version] [1: Type=0x08] [4: Count] [N: Entities]
// [1: HasFortInfo] [17: FortInfo if present]
std::vector<uint8_t> serialize_entity_update(const std::vector<EntityInfo> &entities)
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

    // Fill in length header (total message size)
    uint32_t length = buffer.size();
    buffer[0] = (length >> 24) & 0xFF;
    buffer[1] = (length >> 16) & 0xFF;
    buffer[2] = (length >> 8) & 0xFF;
    buffer[3] = length & 0xFF;

    return buffer;
}
