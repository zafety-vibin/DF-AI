// Tile extraction utilities for DFHack binary protocol
// Feature: 001-binary-protocol
// Target: DFHack 53.02-r1, DF 53.02

#include "Core.h"
#include "modules/MapCache.h"
#include "TileTypes.h"
#include "df/map_block.h"
#include "df/world.h"
#include "df/tiletype.h"
#include "df/tiletype_material.h"
#include "df/job.h"
#include "df/job_list_link.h"

#include "protocol.h"

#include <vector>
#include <unordered_set>
#include <cstdint>
#include <cstdio>

using namespace DFHack;
using namespace df::enums;

// Whether a job_type represents an in-flight dig-designation job — a unit
// has claimed the designation and is walking to/working the tile. DF clears
// (or stops reflecting) designation.bits.dig once a job claims it, so a
// tile mid-dig looks indistinguishable from an undesignated one if we only
// read the designation bit. DFHack's own dig-now.cpp (plugins/dig-now.cpp,
// DesignationJobs::load) treats exactly this set of job types as
// designation-equivalent; mirrored here for the same reason.
bool is_dig_job_type(df::job_type type)
{
    switch (type) {
        case df::job_type::Dig:
        case df::job_type::DigChannel:
        case df::job_type::CarveRamp:
        case df::job_type::CarveUpwardStaircase:
        case df::job_type::CarveDownwardStaircase:
        case df::job_type::CarveUpDownStaircase:
            return true;
        default:
            return false;
    }
}

// Build the set of tile coordinates with an in-flight dig-designation job
// (see is_dig_job_type). Call once per extraction pass and pass the result
// into compute_tile_flags for every tile in that pass -- walking the global
// job list per-tile would be O(tiles * jobs).
std::unordered_set<df::coord> collect_dig_job_targets()
{
    std::unordered_set<df::coord> targets;

    if (!df::global::world) {
        return targets;
    }

    for (df::job_list_link *node = df::global::world->jobs.list.next; node; node = node->next) {
        df::job *job = node->item;
        if (!job) {
            continue;
        }
        if (is_dig_job_type(job->job_type)) {
            targets.insert(job->pos);
        }
    }

    return targets;
}

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

// Compute the full protocol flag byte for one tile. This is the SINGLE
// source of truth for tile flags — called by both the full-state path
// (extract_full_map_state below) and the delta path (detect_tile_changes
// in tile_updates.cpp) so the two can never drift.
//
// `dig_job_targets` must be built ONCE per extraction pass by both callers
// (via collect_dig_job_targets above) and threaded through here as a
// parameter -- an earlier version of this fix computed the flag byte
// straight from the designation bit in each path independently, which is
// exactly the asymmetry that caused the 062455f regression: one path
// picked up a follow-on tweak and the other didn't, and the two silently
// diverged again. A shared parameter makes that class of bug impossible --
// there's only one signature to change.
uint8_t compute_tile_flags(MapExtras::MapCache &map_cache, const df::coord &pos, df::tiletype tile_type,
                            const std::unordered_set<df::coord> &dig_job_targets)
{
    uint8_t flags = 0;

    // If the block isn't loaded OR DF returned no tiletype, mark the tile
    // as HIDDEN so the Go side's topology overlay treats it as "unknown /
    // unexplored" rather than implicitly "wall." DF lazily allocates
    // blocks; on a fresh embark only a small fraction of the 192×192×129
    // volume has populated tile data. Without this flag the Go side sees
    // ~99% of tiles with no FLAG_FLOOR/WALL/VOID and defaults them to
    // closed — the source of the "agent thinks the world is sealed in
    // walls" symptom.
    if (static_cast<uint16_t>(tile_type) == 0) {
        return FLAG_HIDDEN;
    }

    MapExtras::Block *block = map_cache.BlockAt(pos);
    if (!block) {
        return flags;
    }

    // Get raw block for designation access
    df::map_block *raw_block = block->getRaw();
    if (!raw_block) {
        return flags;
    }

    // Calculate position within block
    int block_x = pos.x & 15;
    int block_y = pos.y & 15;

    // Check designation flags
    df::tile_designation des = raw_block->designation[block_x][block_y];

    if (des.bits.hidden) {
        flags |= FLAG_HIDDEN;
    }
    // Note: discovered status not directly available in all DF versions
    // For now, assume discovered if not hidden
    if (!des.bits.hidden) {
        flags |= FLAG_DISCOVERED;
    }

    // Check if designated for digging. DF clears (or stops reflecting)
    // designation.bits.dig once a unit claims the dig job, so the raw bit
    // alone goes cold mid-dig -- a tile with a unit actively digging it
    // would otherwise report as "not designated" for the whole job
    // duration. Treat an in-flight job on this tile (see
    // collect_dig_job_targets/is_dig_job_type above) as equally
    // authoritative. NOTE: this is deliberately NOT the same thing as
    // marker-mode digging (dig_marked/DES_MARKER_ONLY) -- marker mode is a
    // player UI concept for staged designation review, unrelated to
    // whether a job has been claimed.
    if (des.bits.dig != df::tile_dig_designation::No || dig_job_targets.count(pos)) {
        flags |= FLAG_DESIGNATED;
    }

    // Classify tile using DFHack API
    // Bit 4: Wall (solid rock/constructed wall)
    // Bit 5: Floor (walkable surface including ramps/stairs)
    // Bit 6: Void (open air, missing floor, fall hazard)
    // Bit 7: Liquid (water/magma at 7/7 depth)

    // Classify by tile_shape directly. The DFHack helpers
    // isWalkable / isWallTerrain go through the
    // tiletype_shape `walkable` and `basic_shape` attribute
    // table, which has produced unexpected results across
    // versions. Switching on the shape enum is unambiguous
    // and stable: each value here is a documented DF tile
    // shape, so the mapping is checkable against the
    // df/tiletype_shape.h enum at any version.
    df::tiletype_shape shape = tileShape(tile_type);

    switch (shape) {
        // Walkable surfaces — dwarves can stand here.
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
            flags |= FLAG_FLOOR;
            break;

        // Solid walls — diggable.
        case df::tiletype_shape::WALL:
        case df::tiletype_shape::TRUNK_BRANCH:
        case df::tiletype_shape::BROOK_BED:
            flags |= FLAG_WALL;
            break;

        // Open / void — fall hazard, not diggable.
        case df::tiletype_shape::EMPTY:
        case df::tiletype_shape::ENDLESS_PIT:
            flags |= FLAG_VOID;
            break;

        // Default: unclassified — leave flags untouched.
        // NONE / unknown shapes shouldn't be common at
        // runtime; if they show up, the orchestrator
        // will treat them as closed.
        default:
            break;
    }

    // Check for dangerous liquids (7/7 depth only) —
    // independent of shape classification.
    if (des.bits.flow_size == 7) {
        df::tiletype_material mat = tileMaterial(tile_type);
        if (mat == df::tiletype_material::POOL ||   // Water
            mat == df::tiletype_material::MAGMA) {  // Magma
            flags |= FLAG_LIQUID_7_7;
        }
    }

    return flags;
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

    // Diagnostic: histogram of tile_shape values seen during extraction.
    // Written to dfhack-ai-shape-histogram.txt at the end so we can see
    // exactly what shapes the plugin encounters on this map. If surface
    // grass should be FLOOR but is classified as something unexpected,
    // we'll see it here.
    int shape_hist[32] = {0};            // shape int → count (-1..18 expected)
    int classified_floor = 0;
    int classified_wall  = 0;
    int classified_void  = 0;
    int classified_none  = 0;

    // Estimate size: (x * y * z) tiles * 9 bytes per tile
    size_t estimated_tiles = x_max * y_max * z_max;
    result.reserve(estimated_tiles * 9);

    // Use MapCache for efficient tile access
    MapExtras::MapCache map_cache;

    // Job-target coords with an in-flight dig job, built once for this
    // whole pass (see compute_tile_flags's comment on why this must be
    // shared/threaded rather than recomputed per path).
    std::unordered_set<df::coord> dig_job_targets = collect_dig_job_targets();

    // Iterate in row-major order: Z outermost, then Y, then X innermost
    for (int32_t z = 0; z < z_max; z++) {
        for (int32_t y = 0; y < y_max; y++) {
            for (int32_t x = 0; x < x_max; x++) {
                df::coord pos(x, y, z);

                // Get tile type
                df::tiletype tile_type = map_cache.tiletypeAt(pos);

                // Full flag byte — shared with the delta path
                // (see compute_tile_flags above).
                uint8_t flags = compute_tile_flags(map_cache, pos, tile_type, dig_job_targets);

                // Histogram + classification counts for diagnostics,
                // derived from the computed flags. Shape values are -1..18
                // for the standard enum; offset by +1 so index 0 = NONE.
                if (static_cast<uint16_t>(tile_type) > 0) {
                    df::tiletype_shape shape = tileShape(tile_type);
                    int shape_idx = (int)shape + 1;
                    if (shape_idx >= 0 && shape_idx < 32) {
                        shape_hist[shape_idx]++;
                    }

                    if (flags & FLAG_FLOOR) {
                        classified_floor++;
                    } else if (flags & FLAG_WALL) {
                        classified_wall++;
                    } else if (flags & FLAG_VOID) {
                        classified_void++;
                    } else {
                        classified_none++;
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

    // Diagnostic dump: shape histogram + classification counts. Written
    // to <DF root>/dfhack-ai-shape-histogram.txt so we can see what the
    // plugin actually classified. Append-only so multiple FULL_STATE
    // calls accumulate.
    {
        const char* shape_names[] = {
            "NONE", "EMPTY", "FLOOR", "BOULDER", "PEBBLES", "WALL",
            "FORTIFICATION", "STAIR_UP", "STAIR_DOWN", "STAIR_UPDOWN",
            "RAMP", "RAMP_TOP", "BROOK_BED", "BROOK_TOP", "BRANCH",
            "TRUNK_BRANCH", "TWIG", "SAPLING", "SHRUB", "ENDLESS_PIT",
        };
        std::FILE* f = std::fopen("dfhack-ai-shape-histogram.txt", "a");
        if (f) {
            std::fprintf(f, "=== FULL_STATE extracted (map %dx%dx%d) ===\n",
                         x_max, y_max, z_max);
            std::fprintf(f, "Classification totals: floor=%d wall=%d void=%d unclassified=%d\n",
                         classified_floor, classified_wall, classified_void, classified_none);
            std::fprintf(f, "Shape histogram:\n");
            for (int i = 0; i < 32; i++) {
                if (shape_hist[i] == 0) continue;
                const char* name = (i < 20) ? shape_names[i] : "(out of range)";
                std::fprintf(f, "  shape %2d (%s): %d tiles\n", i - 1, name, shape_hist[i]);
            }
            std::fprintf(f, "\n");
            std::fclose(f);
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
