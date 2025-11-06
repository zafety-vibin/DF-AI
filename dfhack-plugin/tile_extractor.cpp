// Tile extraction utilities for DFHack binary protocol
// Feature: 001-binary-protocol
// Target: DFHack 53.02-r1, DF 53.02

#include "Core.h"
#include "modules/MapCache.h"
#include "df/map_block.h"
#include "df/world.h"
#include "df/tiletype.h"

#include <vector>
#include <cstdint>

using namespace DFHack;
using namespace df::enums;

// Helper to serialize uint16 big-endian
void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value) {
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Helper to serialize int16 big-endian
void write_int16_be(std::vector<uint8_t> &buf, int16_t value) {
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Helper to serialize uint32 big-endian
void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value) {
    buf.push_back((value >> 24) & 0xFF);
    buf.push_back((value >> 16) & 0xFF);
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Helper to serialize uint64 big-endian
void write_uint64_be(std::vector<uint8_t> &buf, uint64_t value) {
    buf.push_back((value >> 56) & 0xFF);
    buf.push_back((value >> 48) & 0xFF);
    buf.push_back((value >> 40) & 0xFF);
    buf.push_back((value >> 32) & 0xFF);
    buf.push_back((value >> 24) & 0xFF);
    buf.push_back((value >> 16) & 0xFF);
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Extract full map state as binary tile array
// Returns binary data ready to be embedded in FULL_STATE message
// Format: for each tile (Z, Y, X order): [2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
std::vector<uint8_t> extract_full_map_state()
{
    std::vector<uint8_t> result;

    CoreSuspender suspend;  // Pause DF while reading

    // Get map dimensions
    if (!df::global::world || !df::global::world->map.x_count) {
        return result;  // No map loaded
    }

    int32_t x_max = df::global::world->map.x_count;
    int32_t y_max = df::global::world->map.y_count;
    int32_t z_max = df::global::world->map.z_count;

    // Estimate size: (x * y * z) tiles * 9 bytes per tile
    size_t estimated_tiles = x_max * y_max * z_max;
    result.reserve(estimated_tiles * 9);

    // Use MapCache for efficient tile access
    MapExtras::MapCache map_cache;

    // Iterate in row-major order: Z outermost, then Y, then X innermost
    for (int32_t z = 0; z < z_max; z++) {
        for (int32_t y = 0; y < y_max; y++) {
            for (int32_t x = 0; x < x_max; x++) {
                df::coord pos(x, y, z);

                // Get tile type
                df::tiletype tile_type = map_cache.tiletypeAt(pos);

                // Determine flags
                uint8_t flags = 0;

                // Check if tile is hidden (not yet discovered)
                MapExtras::Block *block = map_cache.BlockAt(pos);
                if (block) {
                    // Calculate position within block
                    int block_x = x & 15;
                    int block_y = y & 15;

                    // Get raw block for designation access
                    df::map_block *raw_block = block->getRaw();

                    // Check designation flags
                    df::tile_designation des = raw_block->designation[block_x][block_y];

                    if (des.bits.hidden) {
                        flags |= 0x01;  // FLAG_HIDDEN
                    }
                    // Note: discovered status not directly available in all DF versions
                    // For now, assume discovered if not hidden
                    if (!des.bits.hidden) {
                        flags |= 0x02;  // FLAG_DISCOVERED
                    }

                    // Check if designated for digging
                    if (des.bits.dig != df::tile_dig_designation::No) {
                        flags |= 0x04;  // FLAG_DESIGNATED
                    }
                }

                // Serialize tile: [2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
                write_int16_be(result, static_cast<int16_t>(x));
                write_int16_be(result, static_cast<int16_t>(y));
                write_int16_be(result, static_cast<int16_t>(z));
                write_uint16_be(result, static_cast<uint16_t>(tile_type));
                result.push_back(flags);
            }
        }
    }

    return result;
}

// Get map dimensions
bool get_map_dimensions(int32_t &width, int32_t &height, int32_t &depth)
{
    CoreSuspender suspend;

    if (!df::global::world || !df::global::world->map.x_count) {
        return false;  // No map loaded
    }

    width = df::global::world->map.x_count;
    height = df::global::world->map.y_count;
    depth = df::global::world->map.z_count;

    return true;
}
