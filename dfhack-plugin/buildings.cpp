// DFHack Plugin — Building Placement
//
// Implements workshop, furniture, construction, door, and stockpile
// placement in response to BUILD/STOCKPILE command messages from the
// orchestrator.
//
// CONSTRUCTION SEQUENCE — verified against DFHack 53.15-r1 source
// (library/modules/Buildings.cpp) and DFHack's own canonical consumer
// dfhack.buildings.constructBuilding (library/lua/dfhack/buildings.lua,
// lines 481-549). Line numbers cited below refer to those files.
//
//   1. Buildings::allocInstance(pos, type, subtype, custom)
//      Allocates the instance through DF's virtual_identity (DF vtable, so
//      DF virtual methods like isActual() work). Leaves bld->id == -1
//      (init-value in library/xml/df.building.xml:351) — later steps
//      REQUIRE id == -1. pos becomes the NW corner: x1=x2=centerx=pos.x
//      (Buildings.cpp:476-478); setSize then expands x2/y2.
//
//   2. Buildings::setSize(bld, size)
//      Requires bld->id == -1 (throws otherwise, Buildings.cpp:925).
//      Corrects size/center per building type via getCorrectSize
//      (Buildings.cpp:930 — a 3x3 request for a regular workshop stays
//      3x3, center (1,1)), then VALIDATES every tile via
//      checkBuildingTiles and returns the result (Buildings.cpp:986).
//      A separate checkFreeTiles call is redundant — the Lua reference
//      path relies solely on setSize's return (buildings.lua:525-530).
//
//   3a. ACTUAL buildings (workshops, furniture, doors, constructions):
//      Buildings::constructWithFilters(bld, filters).
//      Preconditions — each one THROWS DFHack::Error::InvalidArgument
//      (Error.h:83-84) if violated (Buildings.cpp:1211-1214):
//        * bld != NULL
//        * bld->id == -1            (fresh, unlinked instance)
//        * bld->isActual()          (true for everything this file builds)
//        * !filters.empty() == needsItems(bld) — needsItems is true for
//          every actual building except FarmPlot/RoadDirt
//          (Buildings.cpp:1154-1168), so we MUST pass >= 1 job_item.
//      On success it links the building into the world, sets occupancy,
//      creates the ConstructBuilding job and attaches the filters
//      (linkForConstruct, Buildings.cpp:1126-1152; filter attach + design,
//      1228-1253). On failure it deletes the filters itself
//      (Buildings.cpp:1220-1226).
//
//   3b. ABSTRACT buildings (stockpiles, civzones) ONLY:
//      Buildings::constructAbstract(bld). It THROWS Error::InvalidArgument
//      for any building with isActual() == true (Buildings.cpp:1089) —
//      calling it for a carpenter's workshop is what crashed DF on
//      2026-07-12 (unhandled exception on the socket thread ->
//      std::terminate). NEVER call it for real buildings.
//
// Filter contents are copied from DFHack's own per-type input tables in
// library/lua/dfhack/buildings.lua (workshop_inputs / building_inputs /
// the Construction branch of get_inputs_by_type) — exact lines cited at
// each placer below.

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
#include "df/job_item.h"
#include "df/job_item_vector_id.h"
#include "df/item_type.h"

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
// job_item filter factories.
//
// In-tree C++ precedent for `new df::job_item()` + direct field assignment
// on objects handed to DF jobs: plugins/autogems.cpp:58,
// plugins/strangemood.cpp:603. The generated df::job_item constructor
// applies the df.job.xml init-values (quantity=1, mat_index=-1,
// vector_id=IN_PLAY, min_dimension=-1, ...), which is byte-for-byte what
// the known-good Lua construction path starts from before assigning its
// table fields — so setting only the fields the Lua tables set reproduces
// DFHack's own behavior exactly.
// ---------------------------------------------------------------------------

// Generic "one building-material item" filter. This is what DFHack's own
// tables specify for every workshop type we place (buildings.lua
// workshop_inputs:215-250) and for plain constructions (buildings.lua:407).
static df::job_item *makeBuildMatFilter() {
    df::job_item *ji = new df::job_item();
    ji->flags2.bits.building_material = true;
    ji->flags2.bits.non_economic = true;
    return ji;
}

// Specific-item filter for furniture and doors: an item_type plus the
// matching job_item_vector_id, per buildings.lua building_inputs.
// requireEmpty sets flags1.empty (needed only for Box, buildings.lua:48-54).
static df::job_item *makeItemFilter(df::item_type itype,
                                    df::job_item_vector_id vecId,
                                    bool requireEmpty = false) {
    df::job_item *ji = new df::job_item();
    ji->item_type = itype;
    ji->vector_id = vecId;
    if (requireEmpty)
        ji->flags1.bits.empty = true;
    return ji;
}

// Free a building instance that was never linked into the world.
// Mirrors the finalizer of dfhack.buildings.constructBuilding
// (buildings.lua:512-519): room.extents may have been allocated during
// tile validation (init_extents, Buildings.cpp:743-753, uses new[]) and
// the destructor does NOT free it — the Lua reference explicitly deletes
// it first ("ensure we don't leak extents created by
// Buildings::checkFreeTiles").
static void destroyUnlinked(df::building *bld) {
    if (bld->room.extents) {
        delete[] bld->room.extents;
        bld->room.extents = NULL;
    }
    delete bld;
}

// ---------------------------------------------------------------------------
// Generic placement helper for ACTUAL buildings: allocate → setSize →
// constructWithFilters. Single point of truth so all categories share the
// same, source-verified call sequence and error handling. Takes ownership
// of `filters` (df::job_item* the caller allocated).
// ---------------------------------------------------------------------------
static bool placeBuilding(
    df::coord pos,
    df::building_type type,
    int subtype,
    int custom,
    int width,
    int height,
    std::vector<df::job_item*> filters,
    std::string &error)
{
    // constructWithFilters THROWS on an empty vector for buildings that
    // need items (Buildings.cpp:1214), and every type placed through this
    // helper needs them (needsItems, Buildings.cpp:1154-1168).
    if (filters.empty()) {
        error = "internal error: no job_item filters for actual building";
        return false;
    }

    df::building *bld = Buildings::allocInstance(pos, type, subtype, custom);
    if (!bld) {
        for (df::job_item *f : filters)
            delete f;
        error = "Buildings::allocInstance returned null (unknown building class)";
        return false;
    }
    // bld->id is -1 here (df.building.xml:351 init-value) — required by
    // the CHECK_INVALID_ARGUMENT preconditions of setSize (Buildings.cpp:925)
    // and constructWithFilters (Buildings.cpp:1212).

    // setSize corrects size/center for the type and validates every tile
    // (checkBuildingTiles via Buildings.cpp:986). False = tiles blocked,
    // occupied, or unsuitable terrain.
    df::coord2d size((int16_t)width, (int16_t)height);
    if (!Buildings::setSize(bld, size)) {
        for (df::job_item *f : filters)
            delete f;
        destroyUnlinked(bld);
        error = "cannot place building here (tiles blocked, occupied, or unsuitable)";
        return false;
    }

    // Links the building into the world (id assignment, world vectors,
    // occupancy) and creates the ConstructBuilding job with our filters
    // attached (Buildings.cpp:1209-1254). Dwarves then haul matching items
    // and build it. NOTE: on failure this call deletes the filters itself
    // (Buildings.cpp:1220-1226), so only the instance is ours to free.
    if (!Buildings::constructWithFilters(bld, filters)) {
        destroyUnlinked(bld);
        error = "constructWithFilters failed (tiles no longer free)";
        return false;
    }
    return true;
}

// ---------------------------------------------------------------------------
// Category placers. Each maps the protocol BuildType byte to the right
// df type / size / filter combination and delegates to placeBuilding.
// ---------------------------------------------------------------------------

bool placeWorkshop(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int wsType = protocolToWorkshopType(buildType);
    if (wsType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown workshop type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    // Every workshop type we place takes exactly one generic
    // building-material item (buildings.lua workshop_inputs —
    // Carpenters:215, Farmers:216, Masons:217, Craftsdwarfs:218,
    // Mechanics:239, Butchers:241, Fishery:245, Still:246, Kitchen:250).
    std::vector<df::job_item*> filters;
    filters.push_back(makeBuildMatFilter());
    return placeBuilding(df::coord(x, y, z), df::building_type::Workshop,
                         wsType, -1, 3, 3, filters, error);
}

bool placeFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int fType = protocolToFurnitureType(buildType);
    if (fType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown furniture type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    // Filters per buildings.lua building_inputs: Chair:35, Bed:36,
    // Table:37, Box:48-54 (must be empty), Cabinet:67-69. Furniture is
    // built from an already-made item of the same type.
    std::vector<df::job_item*> filters;
    switch (fType) {
        case df::building_type::Bed:
            filters.push_back(makeItemFilter(df::item_type::BED,
                                             df::job_item_vector_id::BED));
            break;
        case df::building_type::Table:
            filters.push_back(makeItemFilter(df::item_type::TABLE,
                                             df::job_item_vector_id::TABLE));
            break;
        case df::building_type::Chair:
            filters.push_back(makeItemFilter(df::item_type::CHAIR,
                                             df::job_item_vector_id::CHAIR));
            break;
        case df::building_type::Cabinet:
            filters.push_back(makeItemFilter(df::item_type::CABINET,
                                             df::job_item_vector_id::CABINET));
            break;
        case df::building_type::Box:
            filters.push_back(makeItemFilter(df::item_type::BOX,
                                             df::job_item_vector_id::BOX,
                                             /*requireEmpty=*/true));
            break;
        default: {
            char buf[64];
            snprintf(buf, sizeof(buf), "No filter mapping for furniture type: 0x%02X", buildType);
            error = buf;
            return false;
        }
    }
    return placeBuilding(df::coord(x, y, z), (df::building_type)fType,
                         -1, -1, 1, 1, filters, error);
}

bool placeConstruction(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int cType = protocolToConstructionType(buildType);
    if (cType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown construction type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    // Constructed wall/floor/stairs/ramp take one generic building-material
    // item (buildings.lua get_inputs_by_type Construction branch:403-407;
    // only ReinforcedWall differs, which we do not place).
    std::vector<df::job_item*> filters;
    filters.push_back(makeBuildMatFilter());
    return placeBuilding(df::coord(x, y, z), df::building_type::Construction,
                         cType, -1, 1, 1, filters, error);
}

// placeStockpile designates a rectangular stockpile zone with the
// requested top-level group flags enabled. Per-material sub-flags
// (which woods, which stones, which food types) are NOT set — those
// require either DFHack's stockpiles plugin Lua API or manual UI work
// in DF after the stockpile is placed. The category TABS will be
// enabled though, so the player only needs to click "all" within each
// category once.
//
// NOTE: stockpiles are ABSTRACT buildings (isActual() == false), so
// constructAbstract is the correct call here — it exists precisely for
// stockpiles/civzones (its type switch handles exactly those two,
// Buildings.cpp:1094-1113) and gets the stockpile a number. Do NOT copy
// this call into paths that place real buildings; those must use
// constructWithFilters (see placeBuilding above).
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
    // setSize validates the tiles itself (Buildings.cpp:986); stockpiles
    // are extent-shaped, so blocked tiles are excluded from the extents
    // rather than failing the whole placement.
    if (!Buildings::setSize(bld, size)) {
        error = "Buildings::setSize failed for stockpile (no usable tiles)";
        destroyUnlinked(bld);
        return false;
    }

    // Set top-level group flags so the requested category tabs show in DF.
    df::building_stockpilest* sp = strict_virtual_cast<df::building_stockpilest>(bld);
    if (sp) {
        sp->settings.flags.whole = groupMask;
    }

    // Abstract construction: assigns stockpile_number, links the building
    // into the world and sets flags.exists (Buildings.cpp:1085-1123).
    // Never throws for a stockpile: id == -1 (fresh instance) and
    // isActual() == false both hold.
    if (!Buildings::constructAbstract(bld)) {
        error = "Buildings::constructAbstract failed for stockpile";
        destroyUnlinked(bld);
        return false;
    }
    return true;
}

bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    // Filters per buildings.lua building_inputs: Door:41, Hatch:133-138.
    std::vector<df::job_item*> filters;
    if (buildType == BUILD_TYPE_DOOR) {
        filters.push_back(makeItemFilter(df::item_type::DOOR,
                                         df::job_item_vector_id::DOOR));
        return placeBuilding(df::coord(x, y, z), df::building_type::Door,
                             -1, -1, 1, 1, filters, error);
    }
    filters.push_back(makeItemFilter(df::item_type::HATCH_COVER,
                                     df::job_item_vector_id::HATCH_COVER));
    return placeBuilding(df::coord(x, y, z), df::building_type::Hatch,
                         -1, -1, 1, 1, filters, error);
}
