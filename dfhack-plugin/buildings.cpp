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
#include "LuaTools.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_stockpilest.h"
#include "df/building_farmplotst.h"
#include "df/building_bridgest.h"
#include "df/building_type.h"
#include "df/workshop_type.h"
#include "df/furnace_type.h"
#include "df/trap_type.h"
#include "df/construction_type.h"
#include "df/coord.h"
#include "df/stockpile_settings.h"
#include "df/job_item.h"
#include "df/job_item_vector_id.h"
#include "df/item_type.h"
#include "df/plant_raw.h"
#include "df/plant_raw_flags.h"
#include "df/world.h"

#include "protocol.h"

#include <vector>
#include <string>
#include <cstdio>
#include <cctype>
#include <filesystem>

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
        case BUILD_TYPE_WS_METALSMITH:   return df::workshop_type::MetalsmithsForge;
        default: return -1;
    }
}

// Map protocol furnace ID to df::furnace_type. Returns -1 if not
// recognized. Furnaces are df::building_type::Furnace, a top-level type
// distinct from Workshop (df.building.xml) -- see placeFurnace below.
static int protocolToFurnaceType(uint8_t buildType) {
    switch (buildType) {
        case BUILD_TYPE_FURNACE_SMELTER: return df::furnace_type::Smelter;
        case BUILD_TYPE_FURNACE_WOOD:    return df::furnace_type::WoodFurnace;
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

// Fire-safe "one building-material item" filter: same as makeBuildMatFilter
// plus flags2.fire_safe, per buildings.lua's MetalsmithsForge build-material
// reagent (workshop_inputs:227) and every non-magma furnace_inputs entry
// (buildings.lua:203-206, WoodFurnace/Smelter/GlassFurnace/Kiln). Without
// fire_safe DF would happily accept a wood boulder... except boulders are
// stone, so in practice this mostly excludes non-fire-safe stone types
// (e.g. raw green glass, some ores) exactly as vanilla does.
static df::job_item *makeFireSafeBuildMatFilter() {
    df::job_item *ji = new df::job_item();
    ji->flags2.bits.building_material = true;
    ji->flags2.bits.fire_safe = true;
    ji->flags2.bits.non_economic = true;
    return ji;
}

// Anvil filter for MetalsmithsForge, per buildings.lua workshop_inputs
// MetalsmithsForge's first reagent (buildings.lua:220-227): a specific
// ANVIL item, itself required to be fire-safe (it's cast metal, so this
// never actually excludes anything, but mirrors the reference exactly).
static df::job_item *makeAnvilFilter() {
    df::job_item *ji = new df::job_item();
    ji->item_type = df::item_type::ANVIL;
    ji->vector_id = df::job_item_vector_id::ANVIL;
    ji->flags2.bits.fire_safe = true;
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
    std::vector<df::job_item*> filters;
    if (wsType == df::workshop_type::MetalsmithsForge) {
        // MetalsmithsForge is the one workshop type with two reagents: an
        // ANVIL item plus a fire-safe building material (buildings.lua
        // workshop_inputs:220-228). Order matches the reference table —
        // the anvil is attached first.
        filters.push_back(makeAnvilFilter());
        filters.push_back(makeFireSafeBuildMatFilter());
    } else {
        // Every other workshop type we place takes exactly one generic
        // building-material item (buildings.lua workshop_inputs —
        // Carpenters:215, Farmers:216, Masons:217, Craftsdwarfs:218,
        // Mechanics:239, Butchers:241, Fishery:245, Still:246, Kitchen:250).
        filters.push_back(makeBuildMatFilter());
    }
    return placeBuilding(df::coord(x, y, z), df::building_type::Workshop,
                         wsType, -1, 3, 3, filters, error);
}

bool placeFurnace(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    int fType = protocolToFurnaceType(buildType);
    if (fType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown furnace type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    // Every furnace type we place takes exactly one fire-safe
    // building-material item (buildings.lua furnace_inputs:203-204,
    // WoodFurnace/Smelter — magma variants are not exposed here).
    std::vector<df::job_item*> filters;
    filters.push_back(makeFireSafeBuildMatFilter());
    return placeBuilding(df::coord(x, y, z), df::building_type::Furnace,
                         fType, -1, 3, 3, filters, error);
}

bool placeTradeDepot(int16_t x, int16_t y, int16_t z, std::string &error) {
    // TradeDepot has no subtype (-1). getCorrectSize forces 5x5 regardless
    // of the width/height we pass (Buildings.cpp:610-614), but we pass 5x5
    // for clarity. Filter per buildings.lua building_inputs TradeDepot
    // entry (buildings.lua:40): 3x generic building material.
    std::vector<df::job_item*> filters;
    df::job_item *ji = makeBuildMatFilter();
    ji->quantity = 3;
    filters.push_back(ji);
    return placeBuilding(df::coord(x, y, z), df::building_type::TradeDepot,
                         -1, -1, 5, 5, filters, error);
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

// applyStockpilePreset populates a freshly-placed stockpile's raws-indexed
// settings vectors (stone.mats, wood.mats, food.*, ammo.*, refuse.*, ...)
// by importing DFHack's bundled "everything" stockpile preset through the
// `stockpiles` plugin's Lua API. Without this, DF v50's own invariant —
// every raws-indexed vector inside df::stockpile_settings is ALWAYS sized
// to match the corresponding raws collection (see DFHack's own
// plugins/stockpiles/StockpileSerializer.cpp:627-660
// unserialize_list_material, "that's how the memory is in DF before we
// muck with it") — is violated: our freshly alloc'd settings leave every
// such vector at size 0, and the v50 settings UI indexes them unchecked,
// crashing DF the moment a player opens that stockpile's settings screen.
//
// We ALWAYS import everything.dfstock — every one of the 17 category
// vectors gets sized and filled, regardless of which categories the
// caller actually requested. This is deliberate, not wasteful: DFHack's
// StockpileSettingsSerializer::read (StockpileSerializer.cpp:844) runs
// all 17 category readers on any single import, and in "set" mode
// read_category (StockpileSerializer.cpp:909-913) clears every category
// NOT present in the file before applying the ones that are — so
// per-category imports done one after another clobber each other's
// vectors and flag bits (a groupMask spanning multiple categories would
// silently lose all but the last-imported one). Importing the single
// "everything" preset sizes and enables all 17 categories in one shot, so
// there is nothing left to clobber. The caller (placeStockpile) then
// re-applies sp->settings.flags.whole = groupMask afterward to hide the
// tabs the caller didn't ask for — flags only, the now-sized vectors are
// untouched, exactly mirroring a vanilla stockpile with unchecked tabs
// (all materials allowed underneath, tab just not shown/used).
//
// stockpiles_import(fname, id, mode, filter) is exported via
// DFHACK_LUA_FUNCTION (plugins/stockpiles/stockpiles.cpp:202-206) and
// wraps StockpileSerializer::unserialize_from_file. We pass "set" since
// there is nothing pre-existing on a freshly allocated stockpile worth
// merging with.
//
// THREADING: called only from executeCommand, which — per
// drain_pending_work_impl's contract two functions below — always runs
// with the DF core suspended: either implicitly (plugin_onupdate fires
// from inside Core::onUpdate, itself invoked while Core::Update holds
// CoreSuspendMutex — see PluginManager.cpp:550's "protected by the
// suspend lock" comment on Plugin::on_update) or explicitly
// (drain_from_socket_thread's ConditionalCoreSuspender). Lua::CallLuaModuleFunction
// only does a direct lua_pcall against the core Lua state
// (LuaTools.cpp:836-859) — no additional suspend or thread hop — so
// calling it here is exactly the pattern every other DFHack plugin uses
// from its own command handler (aquifer.cpp, autobutcher.cpp,
// blueprint.cpp, buildingplan.cpp, ...), all of which likewise run under
// a suspend acquired by their caller. This is NOT the
// applyBlueprintDesignation situation above (disabled): that route used
// Core::runCommand, which hops through the console command dispatcher and
// deadlocked against our already-held suspend; a direct Lua module
// function call does not.
//
// Degrades truthfully: a missing/unloaded stockpiles plugin or an
// unreadable preset file makes CallLuaModuleFunction return false (it
// never throws for that), which is folded into the caller's PARTIAL ack
// rather than a false SUCCESS.
static bool applyStockpilePreset(color_ostream &out, df::building_stockpilest *sp,
                                  uint32_t groupMask, std::string &warning)
{
    (void)groupMask; // kept in the signature: documents that flags narrowing
                      // happens in the caller, not here — see comment above.
    std::filesystem::path presetPath =
        Core::getInstance().getHackPath() / "data" / "stockpiles" / "everything.dfstock";

    bool ok = false;
    bool called = Lua::CallLuaModuleFunction(out, "plugins.stockpiles", "stockpiles_import",
        std::make_tuple(presetPath.string(), sp->id, std::string("set"), std::string("")),
        1, [&](lua_State *L) { ok = lua_toboolean(L, -1); });

    if (!called || !ok) {
        warning = "stockpile placed but its setting preset failed to import "
                  "(stockpiles plugin not loaded, or everything.dfstock is missing/unreadable) "
                  "— do NOT open this stockpile's settings screen until it is reconfigured, "
                  "DF will crash on the unresized category vector(s)";
        return false;
    }
    return true;
}

// placeStockpile designates a rectangular stockpile zone with the
// requested top-level group flags enabled, then populates every
// raws-indexed settings vector for those categories via
// applyStockpilePreset — see that function's doc comment for why this is
// mandatory (not cosmetic): DF's settings UI crashes on unsized vectors.
//
// `partial` is an out-param the caller (executeCommand's STOCKPILE case)
// uses to pick ACK_STATUS_PARTIAL over ACK_STATUS_SUCCESS: the stockpile
// itself is real and usable for hauling even when the preset import
// fails, so the return value stays true (placement happened), but the
// caller must not report clean success when the settings screen is still
// crash-unsafe. Always written (false on every path except the preset
// import failure below) so callers never read an uninitialized flag.
//
// NOTE: stockpiles are ABSTRACT buildings (isActual() == false), so
// constructAbstract is the correct call here — it exists precisely for
// stockpiles/civzones (its type switch handles exactly those two,
// Buildings.cpp:1094-1113) and gets the stockpile a number. Do NOT copy
// this call into paths that place real buildings; those must use
// constructWithFilters (see placeBuilding above).
bool placeStockpile(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, uint32_t groupMask, std::string &error, bool &partial)
{
    partial = false;
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

    // Building exists now — a settings-population failure downgrades to a
    // truthful PARTIAL ack (`error` set, return true) rather than undoing
    // the placement; the stockpile itself is real and usable for hauling,
    // only its settings screen is unsafe until reconfigured.
    if (sp) {
        color_ostream_proxy preset_out(Core::getInstance().getConsole());
        std::string warning;
        bool presetOk = applyStockpilePreset(preset_out, sp, groupMask, warning);
        // applyStockpilePreset always imports the "everything" preset, which
        // sets flags.whole to all 17 categories (whether or not it fully
        // succeeds — a partial import can still flip flag bits). Re-narrow
        // to exactly what the caller asked for; the underlying vectors stay
        // sized either way, so this is flags-only and safe even on failure.
        sp->settings.flags.whole = groupMask;
        if (!presetOk) {
            error = warning;
            partial = true;
        }
    }
    return true;
}

// placeFarmPlot designates a rectangular farm plot at (x1,y1)-(x2,y2) on
// level z. FarmPlot is unusual among ACTUAL buildings (isActual()==true):
// it is the ONLY such type besides RoadDirt with needsItems()==false
// (Buildings.cpp:1154-1168), so it must go through
// Buildings::constructWithFilters with an explicitly EMPTY filter vector
// -- NOT constructAbstract (that throws for anything with
// isActual()==true, Buildings.cpp:1089) and NOT the placeBuilding() helper
// above (which rejects empty filters outright, since every OTHER type it
// places needs items). getCorrectSize groups FarmPlot with
// Stockpile/Civzone/Bridge/RoadDirt/RoadPaved -- rectangle-sized, no
// forced footprint (Buildings.cpp:601-608) -- so this mirrors
// placeStockpile's alloc->setSize sequence above, just finishing with
// constructWithFilters instead of constructAbstract.
//
// A freshly placed plot grows NOTHING until applySetFarmCrop assigns a
// crop to at least one season slot.
//
// Tile validation happens AFTER setSize, not before: Buildings::setSize
// (Buildings.cpp:922-992) only returns false when checkFreeTiles finds
// ZERO usable tiles in the whole footprint (the "found_any" return,
// Buildings.cpp:762-825). If even one requested tile is blocked --
// occupied by another building, not HighPassable (FarmPlot has
// allow_wall=false, unlike Civzone), or under flow_size>1 water/magma
// (FarmPlot has allow_flow=false, being an isActual() building) --
// checkFreeTiles instead marks that single tile's extent
// building_extents_type::None and setSize still reports success. That
// tile's occupancy.bits.building is then never set, at placement or ever
// after (see applyDesignateZone in zones.cpp for the identical
// extent-shaped pattern) -- exactly the "assign_crop can never find this
// building" bug this file exists to fix. A pre-flight tile-shape walk
// here would only approximate checkFreeTiles' real gate (HighPassable +
// occupancy + flow_size are DF-internal, not reproducible from
// tiletype_shape alone -- e.g. FORTIFICATION is walkable-shaped but not
// HighPassable, TRUNK_BRANCH is HighPassable but not walkable-shaped), so
// instead we ask DFHack's own bookkeeping after the fact via
// Buildings::countExtentTiles/containsTile: the single source of truth
// for which tiles checkFreeTiles actually accepted.
bool placeFarmPlot(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error)
{
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1) {
        error = "Invalid region (x2<x1 or y2<y1)";
        return false;
    }

    df::coord pos(x1, y1, z);
    df::building *bld = Buildings::allocInstance(pos, df::building_type::FarmPlot, -1, -1);
    if (!bld) {
        error = "Buildings::allocInstance returned null for farm plot";
        return false;
    }

    int width  = (x2 - x1) + 1;
    int height = (y2 - y1) + 1;
    df::coord2d size((int16_t)width, (int16_t)height);
    // setSize validates the tiles itself (Buildings.cpp:986) and only
    // fails outright when NO tile in the footprint is usable.
    if (!Buildings::setSize(bld, size)) {
        error = "Buildings::setSize failed for farm plot (no usable tiles -- "
                "farm plots need open, non-aquatic soil or mud floor)";
        destroyUnlinked(bld);
        return false;
    }

    // Partial-footprint case: some tiles passed, at least one did not, and
    // that one is now a permanently-excluded extent (see file comment
    // above). Reject the whole placement rather than build over a hole --
    // silently handing back a smaller-than-requested plot is not a
    // truthful ACK of what the caller asked for.
    int requestedTiles = width * height;
    if (Buildings::countExtentTiles(bld, requestedTiles) != requestedTiles) {
        std::string badTiles;
        for (int16_t yy = y1; yy <= y2; yy++) {
            for (int16_t xx = x1; xx <= x2; xx++) {
                if (Buildings::containsTile(bld, df::coord2d(xx, yy)))
                    continue;
                if (!badTiles.empty())
                    badTiles += ", ";
                badTiles += "(" + std::to_string(xx) + "," + std::to_string(yy) + ")";
            }
        }
        destroyUnlinked(bld);
        error = "farm plot footprint excludes tile(s) " + badTiles +
                " -- still undug/blocked, occupied by another building, or "
                "under deep water/magma; clear it and retry";
        return false;
    }

    // FarmPlot needsItems()==false: constructWithFilters REQUIRES the
    // filters vector be empty here (CHECK_INVALID_ARGUMENT(!items.empty()
    // == needsItems(bld)), Buildings.cpp:1214) -- passing any filter would
    // throw. This is the one case in this file where an empty vector is
    // correct, not a bug.
    std::vector<df::job_item*> noFilters;
    if (!Buildings::constructWithFilters(bld, noFilters)) {
        destroyUnlinked(bld);
        error = "constructWithFilters failed for farm plot (tiles no longer free)";
        return false;
    }
    return true;
}

// ---------------------------------------------------------------------------
// Farm crop assignment (SET_FARM_CROP).
// ---------------------------------------------------------------------------

// resolvePlantRaw looks up a plant raw by its token (df::plant_raw::id,
// e.g. "PLUMP_HELMET") or its display name (df::plant_raw::name, e.g.
// "plump helmet"), case-insensitively -- either form round-trips back from
// the list_crops query's "token"/"name" fields. Returns the raw's index
// (the same value autofarm.cpp stores directly into
// building_farmplotst::plant_id, see plugins/autofarm.cpp set_farms), or
// -1 with a truthful, example-bearing error if nothing matches.
static int32_t resolvePlantRaw(const std::string &name, std::string &error)
{
    std::string lname = name;
    for (auto &c : lname) c = (char)tolower((unsigned char)c);

    for (df::plant_raw *raw : df::global::world->raws.plants.all) {
        if (!raw) continue;
        std::string ltoken = raw->id;
        for (auto &c : ltoken) c = (char)tolower((unsigned char)c);
        if (ltoken == lname)
            return raw->index;
        std::string ldisplay = raw->name;
        for (auto &c : ldisplay) c = (char)tolower((unsigned char)c);
        if (ldisplay == lname)
            return raw->index;
    }

    std::string examples;
    int shown = 0;
    for (df::plant_raw *raw : df::global::world->raws.plants.all) {
        if (!raw) continue;
        if (raw->underground_depth_max <= 0) continue;
        if (!raw->flags.is_set(df::plant_raw_flags::SEED)) continue;
        if (shown > 0) examples += ", ";
        examples += raw->id;
        if (++shown >= 5) break;
    }
    error = "unknown crop \"" + name + "\" (not a plant raw token or display name; use list_crops to discover valid names)";
    if (!examples.empty())
        error += " -- some subterranean crops available: " + examples;
    return -1;
}

// applySetFarmCrop programs one (season != SEASON_ALL) or all four
// (season == SEASON_ALL) season slots of the farm plot at (x,y,z) to grow
// cropName, or clears them (-1, "fallow"). Precedent: DFHack's autofarm
// plugin set_farm (plugins/autofarm.cpp:197-201) assigns
// farm->plant_id[season] directly the same way.
bool applySetFarmCrop(int16_t x, int16_t y, int16_t z, uint8_t season, const std::string &cropName, std::string &error)
{
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (season > 3 && season != SEASON_ALL) {
        error = "invalid season byte (want 0-3 for spring/summer/autumn/winter, or 0xFF for all four)";
        return false;
    }
    if (cropName.empty()) {
        error = "crop name required (a plant raw token/display name, or \"fallow\")";
        return false;
    }

    df::building *bld = Buildings::findAtTile(df::coord(x, y, z));
    if (!bld) {
        error = "no building at that tile";
        return false;
    }
    df::building_farmplotst *farm = strict_virtual_cast<df::building_farmplotst>(bld);
    if (!farm) {
        error = "building at that tile is not a farm plot";
        return false;
    }

    // A plot under construction has no hookable "construction finished"
    // callback: DF's own (non-DFHack) completion code fires
    // building_actual::initFarmSeasons() ("setdefaults") exactly once,
    // the moment flags.bits.exists first becomes true, and that call
    // resets plant_id[] to the biome default -- clobbering anything
    // written here beforehand. Writing now would ACK truthfully-in-the-
    // moment but silently get stomped on completion, violating the
    // truthful-ACK house rule; refuse instead and name the exact stage so
    // the caller knows to re-issue once construction finishes (same
    // build_stage/getMaxBuildStage "done" definition queries.cpp uses).
    if (bld->getBuildStage() < bld->getMaxBuildStage()) {
        error = "farm plot still under construction (stage " +
                std::to_string(bld->getBuildStage()) + "/" +
                std::to_string(bld->getMaxBuildStage()) +
                ") -- assign crop after construction completes";
        return false;
    }

    std::string lname = cropName;
    for (auto &c : lname) c = (char)tolower((unsigned char)c);

    int32_t plantID = -1;
    if (lname != "fallow") {
        plantID = resolvePlantRaw(cropName, error);
        if (plantID < 0)
            return false;

        // Simple season-legality check, mirrored from DFHack's autofarm
        // plugin (plugins/autofarm.cpp is_plantable): a crop's raw flags
        // record which of the 4 seasons it grows in. SEASON_ALL writes the
        // same crop into every slot regardless -- that mode is a wire-level
        // convenience with no DF equivalent, not something to gate on any
        // one season's flag.
        if (season != SEASON_ALL) {
            df::plant_raw *raw = df::plant_raw::find(plantID);
            static const df::plant_raw_flags seasonFlags[4] = {
                df::plant_raw_flags::SPRING, df::plant_raw_flags::SUMMER,
                df::plant_raw_flags::AUTUMN, df::plant_raw_flags::WINTER
            };
            if (raw && !raw->flags.is_set(seasonFlags[season])) {
                static const char *seasonNames[4] = {"spring", "summer", "autumn", "winter"};
                error = cropName + std::string(" does not grow in ") + seasonNames[season] +
                        " (check its season raw flags via list_crops, or pass season=all)";
                return false;
            }
        }
    }

    if (season == SEASON_ALL) {
        for (int s = 0; s < 4; s++)
            farm->plant_id[s] = (int16_t)plantID;
    } else {
        farm->plant_id[season] = (int16_t)plantID;
    }
    return true;
}

// placeDoor handles every BuildType in the door/hatch wire range
// (isBuildTypeDoor, 0x50-0x6F): Door, Hatch, Lever, Floodgate. All four
// are 1x1 ACTUAL buildings (getCorrectSize has no case for any of them,
// so all fall to the default: forced 1x1, center (0,0) -- Buildings.cpp:
// 736-739) taking exactly one specific pre-made item, so they share the
// same placeBuilding() shape. Filters per buildings.lua building_inputs:
// Door:41, Hatch:133-138, Floodgate:42-47; Lever's filter is
// buildings.lua trap_inputs[Lever] (317-323), a different table since
// Lever is a Trap subtype, not a top-level building_type.
bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error) {
    std::vector<df::job_item*> filters;
    switch (buildType) {
        case BUILD_TYPE_DOOR:
            filters.push_back(makeItemFilter(df::item_type::DOOR,
                                             df::job_item_vector_id::DOOR));
            return placeBuilding(df::coord(x, y, z), df::building_type::Door,
                                 -1, -1, 1, 1, filters, error);
        case BUILD_TYPE_HATCH:
            filters.push_back(makeItemFilter(df::item_type::HATCH_COVER,
                                             df::job_item_vector_id::HATCH_COVER));
            return placeBuilding(df::coord(x, y, z), df::building_type::Hatch,
                                 -1, -1, 1, 1, filters, error);
        case BUILD_TYPE_LEVER:
            filters.push_back(makeItemFilter(df::item_type::TRAPPARTS,
                                             df::job_item_vector_id::TRAPPARTS));
            return placeBuilding(df::coord(x, y, z), df::building_type::Trap,
                                 df::trap_type::Lever, -1, 1, 1, filters, error);
        case BUILD_TYPE_FLOODGATE:
            filters.push_back(makeItemFilter(df::item_type::FLOODGATE,
                                             df::job_item_vector_id::FLOODGATE));
            return placeBuilding(df::coord(x, y, z), df::building_type::Floodgate,
                                 -1, -1, 1, 1, filters, error);
        default: {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown door-range build type: 0x%02X", buildType);
            error = buf;
            return false;
        }
    }
}

// Translates the wire BRIDGE_DIR_* byte (protocol.h) to DF's own
// df::building_bridgest::T_direction (Retracting=-1, Left=0, Right=1,
// Up=2, Down=3). Returns false for an unrecognized byte.
static bool bridgeDirectionFromWire(uint8_t wireDir, df::building_bridgest::T_direction &out) {
    switch (wireDir) {
        case BRIDGE_DIR_RETRACT: out = df::building_bridgest::Retracting; return true;
        case BRIDGE_DIR_RAISE_N: out = df::building_bridgest::Up;         return true;
        case BRIDGE_DIR_RAISE_S: out = df::building_bridgest::Down;       return true;
        case BRIDGE_DIR_RAISE_E: out = df::building_bridgest::Right;      return true;
        case BRIDGE_DIR_RAISE_W: out = df::building_bridgest::Left;       return true;
        default: return false;
    }
}

// placeBridge designates a rectangular bridge at (x1,y1)-(x2,y2) on level
// z, oriented per wireDirection. Bridge is rectangle-shaped like
// FarmPlot/Stockpile (getCorrectSize center=size/2, Buildings.cpp:
// 601-608), but unlike every other type placeBuilding() handles, its
// orientation is set INSIDE Buildings::setSize's 3-arg overload
// (Buildings.cpp:974-981: `obj->direction = (T_direction)direction`).
// placeBuilding() above always calls the 2-arg setSize (direction
// defaults to 0 = Left = "raises to West" for every caller), so reusing
// it here would silently build every bridge facing the wrong way -- this
// is a bespoke placer that calls the 3-arg setSize directly instead.
//
// Material filter: {building_material=true, non_economic=true,
// quantity=-1} (buildings.lua:101). quantity=-1 is a real DFHack
// sentinel: constructWithFilters (Buildings.cpp:1230-1233) replaces any
// filter with quantity<0 by computeMaterialAmount(bld) =
// floor(tileCount/4)+1 BEFORE attaching it to the job -- no manual
// tile-count scaling needed here.
bool placeBridge(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2,
                 uint8_t wireDirection, std::string &error)
{
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1) {
        error = "Invalid region (x2<x1 or y2<y1)";
        return false;
    }
    // DF hard-caps bridges at 31x31 (own UI; DFHack's quickfort enforces
    // the same limit: scripts/internal/quickfort/build.lua max_width/
    // max_height=31). Raise/retract machinery and gate_flags.has_support
    // are built around that bound -- reject oversized rectangles rather
    // than build into untested out-of-domain state.
    if ((x2 - x1 + 1) > 31 || (y2 - y1 + 1) > 31) {
        error = "bridge exceeds DF's maximum size of 31x31 tiles";
        return false;
    }
    df::building_bridgest::T_direction direction;
    if (!bridgeDirectionFromWire(wireDirection, direction)) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown bridge direction byte: 0x%02X", wireDirection);
        error = buf;
        return false;
    }

    df::coord pos(x1, y1, z);
    df::building *bld = Buildings::allocInstance(pos, df::building_type::Bridge, -1, -1);
    if (!bld) {
        error = "Buildings::allocInstance returned null for bridge";
        return false;
    }

    int width  = (x2 - x1) + 1;
    int height = (y2 - y1) + 1;
    df::coord2d size((int16_t)width, (int16_t)height);
    if (!Buildings::setSize(bld, size, (int)direction)) {
        error = "cannot place bridge here (tiles blocked, occupied, or unsuitable)";
        destroyUnlinked(bld);
        return false;
    }

    std::vector<df::job_item*> filters;
    df::job_item *ji = makeBuildMatFilter();
    ji->quantity = -1; // DFHack sentinel -- computeMaterialAmount() fills in the real count
    filters.push_back(ji);

    if (!Buildings::constructWithFilters(bld, filters)) {
        destroyUnlinked(bld);
        error = "constructWithFilters failed for bridge (tiles no longer free)";
        return false;
    }
    return true;
}
