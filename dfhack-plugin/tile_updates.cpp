// Tile change detection and update batching
// Feature: 001-binary-protocol User Story 2

#include "Core.h"
#include "modules/MapCache.h"
#include "df/map_block.h"
#include "df/world.h"

#include "protocol.h"

#include <vector>
#include <unordered_map>
#include <unordered_set>
#include <cstdint>

using namespace DFHack;

// Cache of previously seen tile state for change detection. The value
// packs [16: tiletype][8: flags] so flag-only transitions — a dig
// designation added/canceled, a hidden tile revealed, liquid filling —
// produce deltas too; the tiletype alone doesn't change on those.
static std::unordered_map<uint64_t, uint32_t> g_tile_cache;

// Helper: Pack tiletype + flags into one cache value
static uint32_t make_tile_state(df::tiletype tile_type, uint8_t flags) {
    return ((uint32_t)(uint16_t)tile_type << 8) | (uint32_t)flags;
}

// Helper: Create cache key from coordinates
static uint64_t make_tile_key(int32_t x, int32_t y, int32_t z) {
    return ((uint64_t)x << 32) | ((uint64_t)y << 16) | (uint64_t)(z & 0xFFFF);
}

// Helper: Write int16 big-endian
extern void write_int16_be(std::vector<uint8_t> &buf, int16_t value);

// Shared flag computation (implemented in tile_extractor.cpp) — single
// source of truth for tile flags across the full-state and delta paths.
// `dig_job_targets` must be built once per pass via collect_dig_job_targets
// and threaded through unchanged -- see the comment on compute_tile_flags
// in tile_extractor.cpp for why (the 062455f asymmetry bug this guards
// against).
uint8_t compute_tile_flags(MapExtras::MapCache &map_cache, const df::coord &pos, df::tiletype tile_type,
                            const std::unordered_set<df::coord> &dig_job_targets);

// Builds the set of tiles with an in-flight dig-designation job (implemented
// in tile_extractor.cpp).
std::unordered_set<df::coord> collect_dig_job_targets();

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

    // Job-target coords with an in-flight dig job, built once for this pass
    // (see compute_tile_flags's declaration comment above).
    std::unordered_set<df::coord> dig_job_targets = collect_dig_job_targets();

    // Scan all tiles and compare to cache
    for (int32_t z = 0; z < z_max; z++) {
        for (int32_t y = 0; y < y_max; y++) {
            for (int32_t x = 0; x < x_max; x++) {
                df::coord pos(x, y, z);
                df::tiletype current_type = map_cache.tiletypeAt(pos);

                // Full flag byte — same computation as the full-state path
                // (compute_tile_flags in tile_extractor.cpp), so delta tiles
                // carry real hidden/designated/wall/floor/liquid knowledge.
                uint8_t flags = compute_tile_flags(map_cache, pos, current_type, dig_job_targets);

                uint64_t key = make_tile_key(x, y, z);
                uint32_t state = make_tile_state(current_type, flags);

                // Check if changed (tiletype OR flags)
                auto it = g_tile_cache.find(key);
                if (it == g_tile_cache.end() || it->second != state) {
                    // Tile changed or new - add to result

                    // Serialize tile: [2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
                    write_int16_be(result, static_cast<int16_t>(x));
                    write_int16_be(result, static_cast<int16_t>(y));
                    write_int16_be(result, static_cast<int16_t>(z));
                    write_uint16_be(result, static_cast<uint16_t>(current_type));
                    result.push_back(flags);

                    // Update cache
                    g_tile_cache[key] = state;
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
