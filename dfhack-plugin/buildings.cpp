// DFHack Plugin — Building Placement
//
// Implements workshop, furniture, construction, and door placement in
// response to BUILD command messages from the orchestrator.
//
// IMPORTANT: This file targets the DFHack 53.x Buildings module API. The
// API has historically been stable across DFHack 5x releases, but enum
// values (workshop_type, building_type, construction_type) MAY have
// shifted between 53.02 and 53.12. When upgrading the plugin to a new
// DFHack version, verify these against the SDK headers:
//
//   df/workshop_type.h     — workshop_type enum values
//   df/building_type.h     — building_type enum values (Workshop, Bed, Door, etc.)
//   df/construction_type.h — construction_type enum values (Wall, Floor, Stair, Ramp)
//   modules/Buildings.h    — allocInstance / setSize / checkFreeTiles / allocate signatures
//
// Common 53.12 API points to verify:
//   - Buildings::allocInstance signature (some versions take 4 args, some 3+optional)
//   - Buildings::setSize return type (bool vs void in some branches)
//   - Buildings::checkFreeTiles signature (df::coord vs df::coord2d in different builds)
//   - Cleanup path on partial-construction failure (Buildings::deallocate vs delete)
//
// If the user hits compile errors, they should grep DFHack 53.12's
// modules/Buildings.h for the canonical signatures and adjust.

#include "Core.h"
#include "Console.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_stockpilest.h"
#include "df/building_type.h"
#include "df/workshop_type.h"
#include "df/construction_type.h"
#include "df/coord.h"
#include "df/stockpile_settings.h"

#include "protocol.h"

#include <vector>
#include <string>
#include <cstdio>

using namespace DFHack;

extern int16_t read_int16_be(const std::vector<uint8_t> &data, size_t offset);

// ---------------------------------------------------------------------------
// Type translation: protocol BuildType byte → DFHack enums.
// ---------------------------------------------------------------------------

// Map protocol workshop ID to df::workshop_type. Returns -1 if not a
// recognized workshop.
static int protocolToWorkshopType(uint8_t buildType) {
    switch (buildType) {
        case BUILD_TYPE_WS_CARPENTER:    return df::workshop_type::Carpenters;
        case BUILD_TYPE_WS_MASON:        return df::workshop_type::Masons;
        case BUILD_TYPE_WS_STILL:        return df::workshop_type::Still;
        case BUILD_TYPE_WS_FARMER:       return df::workshop_type::Farmers;
        case BUILD_TYPE_WS_CRAFTSDWARF:  return df::workshop_type::Craftsdwarfs;
        case BUILD_TYPE_WS_MECHANIC:     return df::workshop_type::Mechanics;
        case BUILD_TYPE_WS_BUTCHER:      return df::workshop_type::Butchers;
        case BUILD_TYPE_WS_KITCHEN:      return df::workshop_type::Kitchen;
        case BUILD_TYPE_WS_FISHERY:      return df::workshop_type::Fishery;
        default: return -1;
    }
}

// Map protocol furniture ID to df::building_type. Returns -1 if not
// recognized.
static int protocolToFurnitureType(uint8_t buildType) {
    switch (buildType) {
        case BUILD_TYPE_BED:     return df::building_type::Bed;
        case BUILD_TYPE_TABLE:   return df::building_type::Table;
        case BUILD_TYPE_CHAIR:   return df::building_type::Chair;
        case BUILD_TYPE_CABINET: return df::building_type::Cabinet;
        case BUILD_TYPE_COFFER:  return df::building_type::Box; // "Coffer" in UI = Box internally
        default: return -1;
    }
}

// Map protocol construction ID to df::construction_type. Returns -1 if
// not recognized.
static int protocolToConstructionType(uint8_t buildType) {
    switch (buildType) {
        case BUILD_TYPE_WALL:         return df::construction_type::Wall;
        case BUILD_TYPE_FLOOR:        return df::construction_type::Floor;
        case BUILD_TYPE_UP_STAIR:     return df::construction_type::UpStair;
        case BUILD_TYPE_DOWN_STAIR:   return df::construction_type::DownStair;
        case BUILD_TYPE_UPDOWN_STAIR: return df::construction_type::UpDownStair;
        case BUILD_TYPE_RAMP:         return df::construction_type::Ramp;
        default: return -1;
    }
}

// ---------------------------------------------------------------------------
// Generic placement helper: allocate, size, check, allocate. Single point of
// truth so all categories share the same error handling.
// ---------------------------------------------------------------------------
static bool placeBuilding(
    df::coord pos,
    df::building_type type,
    int subtype,
    int custom,
    int width,
    int height,
    std::string &error)
{
    df::building* bld = Buildings::allocInstance(pos, type, subtype, custom);
    if (!bld) {
        error = "Buildings::allocInstance returned null (tile occupied or invalid type combination)";
        return false;
    }

    df::coord2d size((int16_t)width, (int16_t)height);
    if (!Buildings::setSize(bld, size)) {
        error = "Buildings::setSize failed";
        delete bld;
        return false;
    }

    if (!Buildings::checkFreeTiles(pos, size)) {
        error = "Required tiles are not free for this building";
        delete bld;
        return false;
    }

    // DFHack 53.x renamed Buildings::allocate → Buildings::constructAbstract.
    if (!Buildings::constructAbstract(bld)) {
        error = "Buildings::constructAbstract failed (likely missing build material in stockpile)";
        delete bld;
        return false;
    }
    return true;
}

// ---------------------------------------------------------------------------
// Category placers. Each maps the protocol BuildType byte to the right
// df_type / size combination and delegates to placeBuilding.
// ---------------------------------------------------------------------------

bool placeWorkshop(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int wsType = protocolToWorkshopType(buildType);
    if (wsType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown workshop type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    return placeBuilding(df::coord(x, y, z), df::building_type::Workshop, wsType, -1, 3, 3, error);
}

bool placeFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int fType = protocolToFurnitureType(buildType);
    if (fType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown furniture type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    return placeBuilding(df::coord(x, y, z), (df::building_type)fType, -1, -1, 1, 1, error);
}

bool placeConstruction(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int cType = protocolToConstructionType(buildType);
    if (cType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown construction type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    return placeBuilding(df::coord(x, y, z), df::building_type::Construction, cType, -1, 1, 1, error);
}

// placeStockpile designates a rectangular stockpile zone with the
// requested top-level group flags enabled. Per-material sub-flags
// (which woods, which stones, which food types) are NOT set — those
// require either DFHack's stockpiles plugin Lua API or manual UI work
// in DF after the stockpile is placed. The category TABS will be
// enabled though, so the player only needs to click "all" within each
// category once.
bool placeStockpile(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, uint32_t groupMask, std::string &error)
{
    using namespace DFHack;
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1) {
        error = "Invalid region (x2<x1 or y2<y1)";
        return false;
    }

    df::coord pos(x1, y1, z);
    df::building* bld = Buildings::allocInstance(pos, df::building_type::Stockpile, -1, -1);
    if (!bld) {
        error = "Buildings::allocInstance returned null for stockpile";
        return false;
    }

    int width  = (x2 - x1) + 1;
    int height = (y2 - y1) + 1;
    df::coord2d size((int16_t)width, (int16_t)height);
    if (!Buildings::setSize(bld, size)) {
        error = "Buildings::setSize failed for stockpile";
        delete bld;
        return false;
    }

    // Set top-level group flags so the requested category tabs show in DF.
    df::building_stockpilest* sp = strict_virtual_cast<df::building_stockpilest>(bld);
    if (sp) {
        sp->settings.flags.whole = groupMask;
    }

    if (!Buildings::checkFreeTiles(pos, size)) {
        error = "Required tiles are not free for stockpile";
        delete bld;
        return false;
    }

    if (!Buildings::constructAbstract(bld)) {
        error = "Buildings::constructAbstract failed for stockpile";
        delete bld;
        return false;
    }
    return true;
}

bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int dType = (buildType == BUILD_TYPE_DOOR)
        ? df::building_type::Door
        : df::building_type::Hatch;
    return placeBuilding(df::coord(x, y, z), (df::building_type)dType, -1, -1, 1, 1, error);
}
