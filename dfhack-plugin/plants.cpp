// DFHack Command Handling - Plant Designations (CHOP / GATHER)
// Marks trees for felling and shrubs for gathering by writing the same
// designation DF's own UI does, via DFHack::Designations::markPlant
// (tile_dig_designation::Default on the plant's designation tile).
//
// Replaces the old console shell-outs: "chop-designate" never existed
// (CHOP always failed), and "getplants all" ignored the requested region
// (GATHER designated the whole map). Both were proven broken in live play.

#include "Core.h"

// DataDefs.h must precede modules/Designations.h — that header names
// df::coord in a signature without including it, and df/coord.h cannot be
// included standalone (it and DataDefs.h are mutually dependent).
#include "DataDefs.h"

#include "modules/Designations.h"
#include "modules/Maps.h"

#include "df/map_block.h"
#include "df/plant.h"
#include "df/tiletype.h"
#include "df/world.h"

#include <algorithm>
#include <cstdio>
#include <string>

using namespace DFHack;

namespace {

// Per-region tallies for one marking pass. Every candidate that is not
// marked lands in exactly one bucket — the ACK text must add up truthfully.
struct MarkTally {
    int marked = 0;    // newly designated this call
    int already = 0;   // designation or fell/gather job already present
    int hidden = 0;    // undiscovered tiles (cavern flora) — see note below
    int dead = 0;      // withered shrubs, nothing to gather (GATHER only)
    int invalid = 0;   // no map block / markPlant refused
};

// appendCount adds "N label" to out, comma-separated after the first.
void appendCount(std::string &out, int n, const char *label)
{
    if (n <= 0)
        return;
    char buf[64];
    snprintf(buf, sizeof(buf), "%s%d %s", out.empty() ? "" : ", ", n, label);
    out += buf;
}

// markPlantsInRegion walks the world plant vector once and designates every
// matching plant whose designation tile falls inside [x1..x2]x[y1..y2] with
// z in [z1..z2]. wantTrees selects the CHOP filter (mature trees:
// tree_info != NULL — a plant with NULL tree_info is a sapling or shrub and
// cannot be felled) vs the GATHER filter (shrubs: plant_type is_shrub with
// a live Shrub tile).
//
// Multi-tile trees: Designations::getPlantDesignationTile picks the SE
// trunk tile (dfhack library/modules/Designations.cpp) — the exact tile
// DF's UI marks — so the region test uses that tile, not the raw
// plant->pos, and marking a canopy-overlapping region never double-counts.
//
// HIDDEN plants are skipped and counted. This is deliberately different
// from dig designations (which punch through fog-of-war by design): plants
// only sit on hidden tiles in undiscovered caverns, DF's own designation
// UI cannot mark them, and dwarves cannot path to them. Mirrors dfhack's
// getplants plugin (plugins/getplants.cpp hidden check), not the dig rule.
//
// THREAD SAFETY: callers run under executeCommand on the socket thread
// (CoreSuspender held) or plugin_onupdate; Designations::* use DFHack
// CHECK_* macros that THROW, so this must stay reachable only under
// executeCommand's try/catch. df::global::world is verified non-null by
// executeCommand before dispatch.
MarkTally markPlantsInRegion(bool wantTrees,
                             int16_t x1, int16_t y1, int16_t z1,
                             int16_t x2, int16_t y2, int16_t z2)
{
    MarkTally t;
    for (df::plant *plant : df::global::world->plants.all) {
        if (!plant)
            continue;

        bool isTree = plant->tree_info != nullptr;
        if (wantTrees != isTree)
            continue;
        // Defensive: non-shrub flora without tree_info (tree saplings) is
        // neither fellable nor gatherable.
        if (!wantTrees && !ENUM_ATTR(plant_type, is_shrub, plant->type))
            continue;

        df::coord des = Designations::getPlantDesignationTile(plant);
        if (des.z < z1 || des.z > z2 ||
            des.x < x1 || des.x > x2 ||
            des.y < y1 || des.y > y2)
            continue;

        df::map_block *block = Maps::getTileBlock(des);
        if (!block) {
            t.invalid++;
            continue;
        }
        if (block->designation[des.x % 16][des.y % 16].bits.hidden) {
            t.hidden++;
            continue;
        }
        // Withered shrubs keep their plant object but the tile decays to
        // ShrubDead — gathering yields nothing, so tally them separately.
        if (!wantTrees &&
            block->tiletype[des.x % 16][des.y % 16] != df::tiletype::Shrub) {
            t.dead++;
            continue;
        }
        if (Designations::isPlantMarked(plant)) {
            t.already++;
            continue;
        }
        if (Designations::markPlant(plant))
            t.marked++;
        else
            t.invalid++;
    }
    return t;
}

// tallyMessage renders the truthful ACK line. Zero matches is an honest
// success ("no trees found in region"), never an error — the region was
// searched and the answer is zero; retrying the same command cannot help.
std::string tallyMessage(const MarkTally &t, const char *noun,
                         const char *verb, const char *emptyMsg)
{
    std::string skips;
    appendCount(skips, t.already, "already marked");
    appendCount(skips, t.dead, "dead/withered");
    appendCount(skips, t.hidden, "hidden (undiscovered)");
    appendCount(skips, t.invalid, "invalid");

    char buf[256];
    if (t.marked > 0) {
        if (skips.empty())
            snprintf(buf, sizeof(buf), "designated %d %s for %s",
                     t.marked, noun, verb);
        else
            snprintf(buf, sizeof(buf), "designated %d %s for %s (skipped: %s)",
                     t.marked, noun, verb, skips.c_str());
    } else if (!skips.empty()) {
        snprintf(buf, sizeof(buf), "no new %s designated (skipped: %s)",
                 noun, skips.c_str());
    } else {
        snprintf(buf, sizeof(buf), "%s", emptyMsg);
    }
    return std::string(buf);
}

// validateRegion normalizes corner order and bounds-checks against the map.
bool validateRegion(int16_t &x1, int16_t &y1, int16_t &z1,
                    int16_t &x2, int16_t &y2, int16_t &z2, std::string &error)
{
    if (x1 > x2) std::swap(x1, x2);
    if (y1 > y2) std::swap(y1, y2);
    if (z1 > z2) std::swap(z1, z2);
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    return true;
}

} // anonymous namespace

// Apply chop designation: mark mature trees in the region for felling.
// On success, `error` carries the truthful count summary — the Go side
// forwards it to the model verbatim.
bool applyChopDesignation(int16_t x1, int16_t y1, int16_t z1,
                          int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (!validateRegion(x1, y1, z1, x2, y2, z2, error))
        return false;

    MarkTally t = markPlantsInRegion(true, x1, y1, z1, x2, y2, z2);
    error = tallyMessage(t, "trees", "felling",
        "no trees found in region (saplings and shrubs cannot be felled)");
    return true;
}

// Apply gather designation: mark live shrubs in the region for gathering.
// Honors the requested rectangle (the old implementation designated the
// entire map). Same count-summary ACK convention as applyChopDesignation.
bool applyGatherDesignation(int16_t x1, int16_t y1, int16_t z1,
                            int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (!validateRegion(x1, y1, z1, x2, y2, z2, error))
        return false;

    MarkTally t = markPlantsInRegion(false, x1, y1, z1, x2, y2, z2);
    error = tallyMessage(t, "shrubs", "gathering",
        "no gatherable shrubs found in region");
    return true;
}
