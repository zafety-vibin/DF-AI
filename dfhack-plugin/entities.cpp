// Entity extraction for DF AI Protocol
// Scans df.global.world.units.active and extracts entity positions

#include "Core.h"
#include "Console.h"
#include "protocol.h"
#include "modules/Units.h"

#include "df/world.h"
#include "df/unit.h"
#include "df/historical_entity.h"

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

    // Access world units
    auto &units = df::global::world->units.active;

    // Get fort civ ID for dwarf detection
    int32_t fort_civ_id = df::global::plotinfo->civ_id;

    // Debug counters
    int total = 0, citizens = 0, fort_controlled = 0, hostile = 0, other = 0;

    for (auto unit : units) {
        if (!unit) continue;

        // Skip dead units
        if (unit->flags1.bits.dead) continue;

        // Get position
        if (unit->pos.x == -30000) continue; // Invalid/off-map position

        total++;

        EntityInfo entity;
        entity.id = unit->id;
        entity.x = unit->pos.x;
        entity.y = unit->pos.y;
        entity.z = unit->pos.z;
        entity.subtype = unit->race;

        // Classify with debug output for first 3 units
        bool is_citizen = Units::isCitizen(unit);
        bool is_fort_ctrl = Units::isFortControlled(unit);

        if (total <= 3) {
            console.print("Unit %d: civ_id=%d (fort=%d), citizen=%d, fort_ctrl=%d, race=%d\n",
                unit->id, unit->civ_id, fort_civ_id, is_citizen, is_fort_ctrl, unit->race);
        }

        if (is_citizen) {
            entity.type = ENTITY_TYPE_DWARF;
            citizens++;
        } else if (is_fort_ctrl) {
            entity.type = ENTITY_TYPE_DWARF;
            fort_controlled++;
        } else if (unit->flags1.bits.marauder || unit->flags1.bits.invader_origin ||
                   unit->flags2.bits.underworld || unit->flags2.bits.visitor_uninvited) {
            entity.type = ENTITY_TYPE_ENEMY;
            hostile++;
        } else if (unit->flags1.bits.merchant || unit->flags2.bits.visitor) {
            entity.type = ENTITY_TYPE_ANIMAL;
        } else {
            entity.type = ENTITY_TYPE_OTHER;
            other++;
        }

        entities.push_back(entity);
    }

    console.print("Entity summary: %d total, %d citizens, %d fort-controlled, %d hostile, %d other\n",
        total, citizens, fort_controlled, hostile, other);

    return entities;
}

// Serialize entities to binary format
// Message structure:
// [4: Length] [1: Version] [1: Type=0x08] [4: Count] [N: Entities]
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

    // Fill in length header (total message size)
    uint32_t length = buffer.size();
    buffer[0] = (length >> 24) & 0xFF;
    buffer[1] = (length >> 16) & 0xFF;
    buffer[2] = (length >> 8) & 0xFF;
    buffer[3] = length & 0xFF;

    return buffer;
}
