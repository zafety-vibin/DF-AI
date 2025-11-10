// Entity extraction for DF AI Protocol
// Scans df.global.world.units.active and extracts entity positions

#include "Core.h"
#include "Console.h"
#include "protocol.h"

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

    // Access world units
    auto &units = df::global::world->units.active;

    // Get fort civ ID for dwarf detection
    int32_t fort_civ_id = df::global::plotinfo->civ_id;

    for (auto unit : units) {
        if (!unit) continue;

        // Skip dead units
        if (unit->flags1.bits.dead) continue;

        // Get position
        if (unit->pos.x == -30000) continue; // Invalid/off-map position

        EntityInfo entity;
        entity.id = unit->id;
        entity.x = unit->pos.x;
        entity.y = unit->pos.y;
        entity.z = unit->pos.z;
        entity.subtype = unit->race;

        // Classify entity type (permissive: treat allied/owned as dwarves)
        // This includes fort citizens, pets, livestock - anything under player control
        bool is_hostile = unit->flags1.bits.marauder || unit->flags1.bits.invader_origin ||
                         unit->flags2.bits.underworld || unit->flags2.bits.visitor_uninvited;

        if (is_hostile) {
            // Definitely hostile
            entity.type = ENTITY_TYPE_ENEMY;
        } else if (unit->civ_id == fort_civ_id ||
                   unit->flags1.bits.tame ||
                   unit->flags2.bits.resident ||
                   unit->flags1.bits.fortress_guard) {
            // Allied/owned by fort (includes dwarves, pets, livestock, guards)
            // Permissive approach: if it's ours and not hostile, count it
            entity.type = ENTITY_TYPE_DWARF;
        } else if (unit->flags1.bits.merchant || unit->flags2.bits.visitor) {
            // Merchants and visitors
            entity.type = ENTITY_TYPE_ANIMAL;
        } else {
            // Unknown/other
            entity.type = ENTITY_TYPE_OTHER;
        }

        entities.push_back(entity);
    }

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
