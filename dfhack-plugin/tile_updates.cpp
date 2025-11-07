// Tile change detection and update batching
// Feature: 001-binary-protocol User Story 2

#include "Core.h"
#include "modules/MapCache.h"
#include "df/map_block.h"
#include "df/world.h"

#include "protocol.h"

#include <vector>
#include <unordered_map>
#include <cstdint>

using namespace DFHack;

// Cache of previously seen tile types for change detection
static std::unordered_map<uint64_t, uint16_t> g_tile_cache;

// Helper: Create cache key from coordinates
static uint64_t make_tile_key(int32_t x, int32_t y, int32_t z) {
    return ((uint64_t)x << 32) | ((uint64_t)y << 16) | (uint64_t)(z & 0xFFFF);
}

// Helper: Write int16 big-endian
extern void write_int16_be(std::vector<uint8_t> &buf, int16_t value);

// Detect changed tiles by comparing current state to cache
// Returns binary tile data for changed tiles only (9 bytes per tile)
std::vector<uint8_t> detect_tile_changes()
{
    std::vector<uint8_t> result;

    CoreSuspender suspend;

    if (!df::global::world || !df::global::world->map.x_count) {
        return result;  // No map loaded
    }

    int32_t x_max = df::global::world->map.x_count;
    int32_t y_max = df::global::world->map.y_count;
    int32_t z_max = df::global::world->map.z_count;

    MapExtras::MapCache map_cache;

    // Scan all tiles and compare to cache
    for (int32_t z = 0; z < z_max; z++) {
        for (int32_t y = 0; y < y_max; y++) {
            for (int32_t x = 0; x < x_max; x++) {
                df::coord pos(x, y, z);
                df::tiletype current_type = map_cache.tiletypeAt(pos);

                uint64_t key = make_tile_key(x, y, z);

                // Check if changed
                auto it = g_tile_cache.find(key);
                if (it == g_tile_cache.end() || it->second != current_type) {
                    // Tile changed or new - add to result
                    uint8_t flags = 0x02;  // Assume discovered

                    // Serialize tile: [2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
                    write_int16_be(result, static_cast<int16_t>(x));
                    write_int16_be(result, static_cast<int16_t>(y));
                    write_int16_be(result, static_cast<int16_t>(z));
                    write_int16_be(result, static_cast<uint16_t>(current_type));
                    result.push_back(flags);

                    // Update cache
                    g_tile_cache[key] = current_type;
                }
            }
        }
    }

    return result;
}

// Clear the tile cache (useful for testing or resync)
void clear_tile_cache()
{
    g_tile_cache.clear();
}
