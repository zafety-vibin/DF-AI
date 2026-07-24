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
//   3c. FURNITURE with a requested quality tier ONLY:
//      Buildings::constructWithItems(bld, {item}) instead of step 3a.
//      Vanilla DF has no way to request quality at craft time (no
//      manager_order/job_item field constrains output quality) — the only
//      real lever is SELECTING among already-existing items of different
//      quality at placement time. Same preconditions as constructWithFilters
//      (Buildings.cpp:1170-1175) but attaches specific df::item* pointers
//      (Job::attachJobItem, Hauled role) instead of job_item filters DF
//      searches for later. See placeFurniture/pickFurnitureItem below.
//
// Filter contents are copied from DFHack's own per-type input tables in
// library/lua/dfhack/buildings.lua (workshop_inputs / building_inputs /
// the Construction branch of get_inputs_by_type) — exact lines cited at
// each placer below.

#include "Core.h"
#include "Console.h"
#include "LuaTools.h"
#include "modules/Buildings.h"
#include "modules/Items.h"
#include "modules/Maps.h"

#include "df/building.h"
#include "df/building_stockpilest.h"
#include "df/building_farmplotst.h"
#include "df/building_bridgest.h"
#include "df/building_type.h"
#include "df/workshop_type.h"
#include "df/workshop_profile.h"
#include "df/furnace_type.h"
#include "df/trap_type.h"
#include "df/construction_type.h"
#include "df/coord.h"
#include "df/stockpile_settings.h"
#include "df/job_item.h"
#include "df/job_item_vector_id.h"
#include "df/item.h"
#include "df/item_type.h"
#include "df/item_quality.h"
#include "df/items_other_id.h"
#include "df/item_flags.h"
#include "df/general_ref_type.h"
#include "df/plant_raw.h"
#include "df/plant_raw_flags.h"
#include "df/unit.h"
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
        case BUILD_TYPE_WS_MAGMA_FORGE:  return df::workshop_type::MagmaForge;
        case BUILD_TYPE_WS_JEWELERS:     return df::workshop_type::Jewelers;
        case BUILD_TYPE_WS_BOWYERS:      return df::workshop_type::Bowyers;
        case BUILD_TYPE_WS_SIEGE:        return df::workshop_type::Siege;
        case BUILD_TYPE_WS_LEATHERWORKS: return df::workshop_type::Leatherworks;
        case BUILD_TYPE_WS_TANNERS:      return df::workshop_type::Tanners;
        case BUILD_TYPE_WS_CLOTHIERS:    return df::workshop_type::Clothiers;
        case BUILD_TYPE_WS_LOOM:         return df::workshop_type::Loom;
        case BUILD_TYPE_WS_KENNELS:      return df::workshop_type::Kennels;
        case BUILD_TYPE_WS_ASHERY:       return df::workshop_type::Ashery;
        case BUILD_TYPE_WS_DYERS:        return df::workshop_type::Dyers;
        default: return -1;
    }
}

// Map protocol furnace ID to df::furnace_type. Returns -1 if not
// recognized. Furnaces are df::building_type::Furnace, a top-level type
// distinct from Workshop (df.building.xml) -- see placeFurnace below.
// MagmaForge is deliberately NOT here -- it is a df::workshop_type value
// (see protocolToWorkshopType above and BUILD_TYPE_WS_MAGMA_FORGE's doc
// comment in protocol.h), not a furnace_type, despite the name.
static int protocolToFurnaceType(uint8_t buildType) {
    switch (buildType) {
        case BUILD_TYPE_FURNACE_SMELTER:       return df::furnace_type::Smelter;
        case BUILD_TYPE_FURNACE_WOOD:          return df::furnace_type::WoodFurnace;
        case BUILD_TYPE_FURNACE_KILN:          return df::furnace_type::Kiln;
        case BUILD_TYPE_FURNACE_GLASS:         return df::furnace_type::GlassFurnace;
        case BUILD_TYPE_FURNACE_MAGMA_SMELTER: return df::furnace_type::MagmaSmelter;
        case BUILD_TYPE_FURNACE_MAGMA_GLASS:   return df::furnace_type::MagmaGlassFurnace;
        case BUILD_TYPE_FURNACE_MAGMA_KILN:    return df::furnace_type::MagmaKiln;
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
        case BUILD_TYPE_COFFIN:  return df::building_type::Coffin;
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
// Generalized by-name build-type resolution (BUILD_TYPE_BY_NAME, see
// protocol.h). This is the BUILD-side counterpart to work_orders.cpp's
// resolveJobTypeByName -- same mechanism (DFHack's find_enum_item<T>,
// library/include/DataDefs.h:842-855 in the 53.15-r1 checkout: a plain
// linear scan over an enum's key_table, never throws, leaves the output
// unwritten on no match), tried against FOUR candidate enums in turn
// instead of job_type's one, since a buildable "thing" in DF is a
// (building_type, subtype) PAIR rather than a single flat enum.
// ---------------------------------------------------------------------------

// resolveBuildTypeByName resolves `name` against DFHack's own
// building-related enums, in priority order:
//   (a) df::building_type key name directly -- covers standalone types with
//       no further subtype (e.g. "Well", "Support", "Statue", "Bookcase").
//   (b) df::workshop_type key name -- resolves to
//       {building_type::Workshop, that workshop_type}.
//   (c) df::furnace_type key name -- resolves to
//       {building_type::Furnace, that furnace_type}.
//   (d) df::trap_type key name -- resolves to
//       {building_type::Trap, that trap_type}.
// outSubtype is -1 ("no subtype", DF's own allocInstance convention) for a
// path-(a) match, and the resolved subtype enum value (as int) for
// (b)/(c)/(d). Returns false with `error` populated (naming the attempted
// name and pointing at the building_types tool) if none of the four match.
//
// This function is PURELY a name -> (building_type, subtype) translation --
// it says nothing about whether this plugin actually knows how to PLACE the
// resolved type. Every subsequent building type this plugin learns to place
// should call this rather than duplicating the find_enum_item scan; see
// resolveCuratedBuildTypeByte below (and its KNOWN LIMITATION comment) and
// designations.cpp's applyBuildDesignation for how BUILD_TYPE_BY_NAME itself
// consumes the result today.
bool resolveBuildTypeByName(const std::string &name, int &outBuildingType, int &outSubtype, std::string &error) {
    df::building_type bt;
    if (find_enum_item(&bt, name)) {
        outBuildingType = (int)bt;
        outSubtype = -1;
        return true;
    }
    df::workshop_type wt;
    if (find_enum_item(&wt, name)) {
        outBuildingType = (int)df::building_type::Workshop;
        outSubtype = (int)wt;
        return true;
    }
    df::furnace_type ft;
    if (find_enum_item(&ft, name)) {
        outBuildingType = (int)df::building_type::Furnace;
        outSubtype = (int)ft;
        return true;
    }
    df::trap_type tt;
    if (find_enum_item(&tt, name)) {
        outBuildingType = (int)df::building_type::Trap;
        outSubtype = (int)tt;
        return true;
    }
    error = "unrecognized building type name: '" + name +
            "' (not a df::building_type, workshop_type, furnace_type, or trap_type key name -- "
            "use the building_types tool to look up valid names)";
    return false;
}

// resolveCuratedBuildTypeByte finds the existing curated BUILD_TYPE_* byte
// (if any) matching a resolved (buildingType, subtype) pair -- lets a
// by-name resolution reuse an ALREADY-VERIFIED placement recipe
// byte-for-byte instead of guessing a job_item filter for a type this
// plugin doesn't build yet. On a match, returns that byte; on no match,
// returns -1 with `error` populated (naming the resolved building_type/
// subtype via ENUM_KEY_STR and pointing at the building_types tool).
//
// KNOWN LIMITATION (mirrors resolveJobTypeByName/ORDER_TYPE_BY_NAME in
// work_orders.cpp): resolving a name to a real DFHack building_type/
// workshop_type/furnace_type/trap_type does NOT by itself mean this plugin
// knows how to place it -- job_item filter recipes (buildings.lua
// workshop_inputs/furnace_inputs/trap_inputs, confirmed against the
// 53.15-r1 checkout) are per-type domain knowledge, hand-ported one
// building type at a time, same as jobTypeAllowedAtWorkshop is for
// queue_job. Only the curated set below has a recipe today (identical to
// the numeric BUILD_TYPE_* path already handled by applyBuildDesignation);
// every other resolvable name (Statue, Bookcase, the ~15 uncurated
// workshop_type values, non-Lever trap_type values, ...) is a future
// task's addition -- see the building_types tool for what's ACTUALLY
// buildable now vs. merely nameable.
int resolveCuratedBuildTypeByte(int buildingType, int subtype, const std::string &name, std::string &error) {
    int curatedByte = -1;
    if (buildingType == df::building_type::Workshop) {
        switch ((df::workshop_type)subtype) {
            case df::workshop_type::Carpenters:      curatedByte = BUILD_TYPE_WS_CARPENTER;   break;
            case df::workshop_type::Masons:          curatedByte = BUILD_TYPE_WS_MASON;       break;
            case df::workshop_type::Still:           curatedByte = BUILD_TYPE_WS_STILL;       break;
            case df::workshop_type::Farmers:         curatedByte = BUILD_TYPE_WS_FARMER;      break;
            case df::workshop_type::Craftsdwarfs:    curatedByte = BUILD_TYPE_WS_CRAFTSDWARF; break;
            case df::workshop_type::Mechanics:       curatedByte = BUILD_TYPE_WS_MECHANIC;    break;
            case df::workshop_type::Butchers:        curatedByte = BUILD_TYPE_WS_BUTCHER;     break;
            case df::workshop_type::Kitchen:         curatedByte = BUILD_TYPE_WS_KITCHEN;     break;
            case df::workshop_type::Fishery:         curatedByte = BUILD_TYPE_WS_FISHERY;     break;
            case df::workshop_type::MetalsmithsForge:curatedByte = BUILD_TYPE_WS_METALSMITH;  break;
            case df::workshop_type::MagmaForge:      curatedByte = BUILD_TYPE_WS_MAGMA_FORGE; break;
            case df::workshop_type::Jewelers:        curatedByte = BUILD_TYPE_WS_JEWELERS;    break;
            case df::workshop_type::Bowyers:         curatedByte = BUILD_TYPE_WS_BOWYERS;     break;
            case df::workshop_type::Siege:           curatedByte = BUILD_TYPE_WS_SIEGE;       break;
            case df::workshop_type::Leatherworks:    curatedByte = BUILD_TYPE_WS_LEATHERWORKS;break;
            case df::workshop_type::Tanners:         curatedByte = BUILD_TYPE_WS_TANNERS;     break;
            case df::workshop_type::Clothiers:       curatedByte = BUILD_TYPE_WS_CLOTHIERS;   break;
            case df::workshop_type::Loom:            curatedByte = BUILD_TYPE_WS_LOOM;        break;
            case df::workshop_type::Kennels:         curatedByte = BUILD_TYPE_WS_KENNELS;     break;
            case df::workshop_type::Ashery:          curatedByte = BUILD_TYPE_WS_ASHERY;      break;
            case df::workshop_type::Dyers:           curatedByte = BUILD_TYPE_WS_DYERS;       break;
            default: break; // Tool/Custom deliberately excluded -- see BUILD_TYPE_WS_JEWELERS's doc comment block (protocol.h) for why neither has a universal recipe
        }
    } else if (buildingType == df::building_type::Furnace) {
        switch ((df::furnace_type)subtype) {
            case df::furnace_type::Smelter:          curatedByte = BUILD_TYPE_FURNACE_SMELTER;       break;
            case df::furnace_type::WoodFurnace:      curatedByte = BUILD_TYPE_FURNACE_WOOD;          break;
            case df::furnace_type::Kiln:              curatedByte = BUILD_TYPE_FURNACE_KILN;          break;
            case df::furnace_type::GlassFurnace:      curatedByte = BUILD_TYPE_FURNACE_GLASS;         break;
            case df::furnace_type::MagmaSmelter:      curatedByte = BUILD_TYPE_FURNACE_MAGMA_SMELTER; break;
            case df::furnace_type::MagmaGlassFurnace: curatedByte = BUILD_TYPE_FURNACE_MAGMA_GLASS;   break;
            case df::furnace_type::MagmaKiln:         curatedByte = BUILD_TYPE_FURNACE_MAGMA_KILN;    break;
            default: break;
        }
    } else if (buildingType == df::building_type::Trap) {
        switch ((df::trap_type)subtype) {
            case df::trap_type::Lever:         curatedByte = BUILD_TYPE_LEVER;          break;
            case df::trap_type::PressurePlate: curatedByte = BUILD_TYPE_PRESSURE_PLATE; break;
            case df::trap_type::StoneFallTrap: curatedByte = BUILD_TYPE_STONE_FALL_TRAP;break;
            case df::trap_type::WeaponTrap:    curatedByte = BUILD_TYPE_WEAPON_TRAP;    break;
            case df::trap_type::TrackStop:     curatedByte = BUILD_TYPE_TRACK_STOP;     break;
            default: break; // CageTrap deliberately excluded -- see file/task scope note
        }
    } else {
        switch ((df::building_type)buildingType) {
            case df::building_type::Bed:        curatedByte = BUILD_TYPE_BED;      break;
            case df::building_type::Table:      curatedByte = BUILD_TYPE_TABLE;    break;
            case df::building_type::Chair:      curatedByte = BUILD_TYPE_CHAIR;    break;
            case df::building_type::Cabinet:    curatedByte = BUILD_TYPE_CABINET;  break;
            case df::building_type::Box:         curatedByte = BUILD_TYPE_COFFER;   break;
            case df::building_type::Coffin:     curatedByte = BUILD_TYPE_COFFIN;   break;
            case df::building_type::Door:        curatedByte = BUILD_TYPE_DOOR;     break;
            case df::building_type::Hatch:       curatedByte = BUILD_TYPE_HATCH;    break;
            case df::building_type::Floodgate:   curatedByte = BUILD_TYPE_FLOODGATE;break;
            case df::building_type::TradeDepot:  curatedByte = BUILD_TYPE_TRADE_DEPOT; break;
            case df::building_type::Well:        curatedByte = BUILD_TYPE_WELL;     break;
            case df::building_type::Support:     curatedByte = BUILD_TYPE_SUPPORT;  break;
            case df::building_type::Statue:           curatedByte = BUILD_TYPE_STATUE;            break;
            case df::building_type::Slab:             curatedByte = BUILD_TYPE_SLAB;              break;
            case df::building_type::WindowGlass:      curatedByte = BUILD_TYPE_WINDOW_GLASS;      break;
            case df::building_type::WindowGem:        curatedByte = BUILD_TYPE_WINDOW_GEM;        break;
            case df::building_type::Bookcase:         curatedByte = BUILD_TYPE_BOOKCASE;          break;
            case df::building_type::DisplayFurniture: curatedByte = BUILD_TYPE_DISPLAY_FURNITURE; break;
            case df::building_type::OfferingPlace:    curatedByte = BUILD_TYPE_OFFERING_PLACE;    break;
            case df::building_type::Instrument:       curatedByte = BUILD_TYPE_INSTRUMENT;        break;
            case df::building_type::ScrewPump:        curatedByte = BUILD_TYPE_SCREW_PUMP;        break;
            case df::building_type::GearAssembly:     curatedByte = BUILD_TYPE_GEAR_ASSEMBLY;     break;
            case df::building_type::AxleHorizontal:   curatedByte = BUILD_TYPE_AXLE_HORIZONTAL;   break;
            case df::building_type::AxleVertical:     curatedByte = BUILD_TYPE_AXLE_VERTICAL;     break;
            case df::building_type::WaterWheel:       curatedByte = BUILD_TYPE_WATER_WHEEL;       break;
            case df::building_type::Windmill:         curatedByte = BUILD_TYPE_WINDMILL;          break;
            case df::building_type::Rollers:          curatedByte = BUILD_TYPE_ROLLERS;           break;
            case df::building_type::ArcheryTarget:    curatedByte = BUILD_TYPE_ARCHERY_TARGET;    break;
            case df::building_type::TractionBench:    curatedByte = BUILD_TYPE_TRACTION_BENCH;    break;
            case df::building_type::NestBox:          curatedByte = BUILD_TYPE_NEST_BOX;          break;
            case df::building_type::Hive:             curatedByte = BUILD_TYPE_HIVE;              break;
            default: break;
        }
    }

    if (curatedByte < 0) {
        std::string subtypeStr = (subtype >= 0) ? (" subtype=" + std::to_string(subtype)) : "";
        char buf[400];
        snprintf(buf, sizeof(buf),
                 "'%s' resolved to a real DFHack building type (building_type=%s%s) but this plugin has no "
                 "placement recipe for it yet -- only the building types listed by the building_types tool are "
                 "actually buildable today",
                 name.c_str(), ENUM_KEY_STR(building_type, (df::building_type)buildingType).c_str(), subtypeStr.c_str());
        error = buf;
        return -1;
    }
    return curatedByte;
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

// Narrows a generic building-material job_item filter (flags2.bits.
// building_material already set by the caller) to one specific raw
// material CLASS -- MATERIAL_CLASS_WOOD/STONE/BLOCKS (protocol.h). Setting
// item_type + vector_id to the class's own master vector (WOOD/BOULDER/
// BLOCKS) is exactly the mechanism DF's own job-item matching uses to pick
// a candidate item (ItemTypeInfo::matches, library/modules/Items.cpp:
// 222-237: an item's type must match the filter's vector_id's master
// item-type, df.item.xml items_other_id WOOD/BOULDER/BLOCKS entries all
// declare `item` == their own name) -- the SAME job_item_vector_id values
// queue_job's own WOOD/BOULDER material-class filter already uses for
// workshop reactions (work_orders.cpp applyQueueJob). MATERIAL_CLASS_ANY
// leaves item_type/vector_id at their df.job.xml init-values (NONE/
// IN_PLAY), reproducing the pre-existing unconstrained behavior exactly.
static void applyMaterialClass(df::job_item *ji, uint8_t materialClass) {
    switch (materialClass) {
        case MATERIAL_CLASS_WOOD:
            ji->item_type = df::item_type::WOOD;
            ji->vector_id = df::job_item_vector_id::WOOD;
            break;
        case MATERIAL_CLASS_STONE:
            ji->item_type = df::item_type::BOULDER;
            ji->vector_id = df::job_item_vector_id::BOULDER;
            break;
        case MATERIAL_CLASS_BLOCKS:
            ji->item_type = df::item_type::BLOCKS;
            ji->vector_id = df::job_item_vector_id::BLOCKS;
            break;
        default:
            break; // MATERIAL_CLASS_ANY: no constraint beyond building_material
    }
}

// Generic "one building-material item" filter. This is what DFHack's own
// tables specify for every workshop type we place (buildings.lua
// workshop_inputs:215-250) and for plain constructions (buildings.lua:407).
// materialClass optionally narrows it to one raw material class -- see
// applyMaterialClass above.
static df::job_item *makeBuildMatFilter(uint8_t materialClass = MATERIAL_CLASS_ANY) {
    df::job_item *ji = new df::job_item();
    ji->flags2.bits.building_material = true;
    ji->flags2.bits.non_economic = true;
    applyMaterialClass(ji, materialClass);
    return ji;
}

// Fire-safe "one building-material item" filter: same as makeBuildMatFilter
// plus flags2.fire_safe, per buildings.lua's MetalsmithsForge build-material
// reagent (workshop_inputs:227) and every non-magma furnace_inputs entry
// (buildings.lua:203-206, WoodFurnace/Smelter/GlassFurnace/Kiln). Without
// fire_safe DF would happily accept a wood boulder... except boulders are
// stone, so in practice this mostly excludes non-fire-safe stone types
// (e.g. raw green glass, some ores) exactly as vanilla does. Requesting
// MATERIAL_CLASS_WOOD here is a legal but self-defeating combination (logs
// are never fire-safe) -- DF simply leaves the job unsatisfied forever,
// same as any other filter nothing on the map can match, not a plugin bug.
static df::job_item *makeFireSafeBuildMatFilter(uint8_t materialClass = MATERIAL_CLASS_ANY) {
    df::job_item *ji = new df::job_item();
    ji->flags2.bits.building_material = true;
    ji->flags2.bits.fire_safe = true;
    ji->flags2.bits.non_economic = true;
    applyMaterialClass(ji, materialClass);
    return ji;
}

// Magma-safe "one building-material item" filter: the magma-fueled
// counterpart to makeFireSafeBuildMatFilter above. Per buildings.lua
// furnace_inputs (203-209) the THREE magma furnace variants
// (MagmaSmelter/MagmaGlassFurnace/MagmaKiln) set flags2.magma_safe
// INSTEAD OF flags2.fire_safe -- fire-safe materials are a strict subset
// of magma-safe ones in vanilla DF (magma is hotter than any ordinary
// fire), so this is a genuinely narrower filter, not a synonym.
// workshop_type::MagmaForge's building-material reagent (buildings.lua:
// 229-237) uses this exact same flag combination, so placeWorkshop reuses
// it too.
static df::job_item *makeMagmaSafeBuildMatFilter(uint8_t materialClass = MATERIAL_CLASS_ANY) {
    df::job_item *ji = new df::job_item();
    ji->flags2.bits.building_material = true;
    ji->flags2.bits.magma_safe = true;
    ji->flags2.bits.non_economic = true;
    applyMaterialClass(ji, materialClass);
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

// Anvil filter for MagmaForge, per buildings.lua workshop_inputs
// MagmaForge's first reagent (buildings.lua:229-237): identical to
// makeAnvilFilter above except magma_safe instead of fire_safe, exactly
// mirroring the fire-safe/magma-safe distinction in the two build-material
// filters (makeFireSafeBuildMatFilter/makeMagmaSafeBuildMatFilter).
static df::job_item *makeMagmaAnvilFilter() {
    df::job_item *ji = new df::job_item();
    ji->item_type = df::item_type::ANVIL;
    ji->vector_id = df::job_item_vector_id::ANVIL;
    ji->flags2.bits.magma_safe = true;
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

// Tool-use job_item filter for furniture whose buildings.lua recipe selects
// by FUNCTION (has_tool_use) rather than by a specific item_type+vector_id
// pair -- Bookcase (buildings.lua:183), DisplayFurniture (:184), and
// OfferingPlace (:181) all specify item_type=TOOL plus a has_tool_use value,
// no vector_id (left at its df.job.xml init-value IN_PLAY, same reasoning as
// the Slab case in placeRoomValueFurniture below). DF's own item-job
// matching checks has_tool_use against the candidate TOOL item's itemdef
// tool_uses flags when the filter sets it -- that is what actually
// discriminates a bookcase-capable tool item from a display-pedestal-capable
// one; item_type=TOOL alone cannot (every craftable tool shares it).
static df::job_item *makeToolUseFilter(df::tool_uses use) {
    df::job_item *ji = new df::job_item();
    ji->item_type = df::item_type::TOOL;
    ji->has_tool_use = use;
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
// Furniture item/vector mapping, shared by the any-matching-item filter
// path (placeFurniture's default) and the quality-aware specific-item path
// (pickFurnitureItem below). itemType/jobVectorId are the same pair
// makeItemFilter already used per building_type, sourced from buildings.lua
// building_inputs (Chair/Bed/Table/Coffin:35-38, Box:48-54). otherId is the
// SEPARATE items_other_id enum value (df.item.xml, NOT job_item_vector_id
// despite sharing the same names) used to index world->items.other[] --
// items_other_id's own item-attr confirms each is the item type's single
// master vector (df.item.xml:1783-2151, e.g. WOOD/BOULDER/BLOCKS/BED/
// COFFIN entries all declare `item` == their own name).
// ---------------------------------------------------------------------------
struct FurnitureItemSpec {
    df::item_type itemType;
    df::job_item_vector_id jobVectorId;
    df::items_other_id otherId;
    bool requireEmpty;
};

static bool furnitureItemSpec(int buildingType, FurnitureItemSpec &spec) {
    spec.requireEmpty = false;
    switch (buildingType) {
        case df::building_type::Bed:
            spec.itemType = df::item_type::BED;
            spec.jobVectorId = df::job_item_vector_id::BED;
            spec.otherId = df::items_other_id::BED;
            return true;
        case df::building_type::Table:
            spec.itemType = df::item_type::TABLE;
            spec.jobVectorId = df::job_item_vector_id::TABLE;
            spec.otherId = df::items_other_id::TABLE;
            return true;
        case df::building_type::Chair:
            spec.itemType = df::item_type::CHAIR;
            spec.jobVectorId = df::job_item_vector_id::CHAIR;
            spec.otherId = df::items_other_id::CHAIR;
            return true;
        case df::building_type::Cabinet:
            spec.itemType = df::item_type::CABINET;
            spec.jobVectorId = df::job_item_vector_id::CABINET;
            spec.otherId = df::items_other_id::CABINET;
            return true;
        case df::building_type::Box:
            spec.itemType = df::item_type::BOX;
            spec.jobVectorId = df::job_item_vector_id::BOX;
            spec.otherId = df::items_other_id::BOX;
            spec.requireEmpty = true; // buildings.lua:48-54 -- Coffer must be empty
            return true;
        case df::building_type::Coffin:
            spec.itemType = df::item_type::COFFIN;
            spec.jobVectorId = df::job_item_vector_id::COFFIN;
            spec.otherId = df::items_other_id::COFFIN;
            return true;
        default:
            return false;
    }
}

// pickFurnitureItem scans the furniture item type's master vector
// (world->items.other[otherId]) for the best available candidate item at
// or above qualityTier -- "best available" means the LOWEST quality that
// still satisfies the threshold, so a "well_crafted or better" request
// doesn't consume a fort's only masterwork bed. requireEmpty additionally
// skips a non-empty Box/Coffer, mirroring makeItemFilter's flags1.bits.empty
// via the same CONTAINS_ITEM general_ref check DFHack's own buildingplan
// plugin uses for the identical empty-filter branch (plugins/buildingplan/
// buildingplan_cycle.cpp matchesFilters).
//
// The bad-flags screen mirrors buildingplan_cycle.cpp's BadFlags struct
// (plugins/buildingplan/buildingplan_cycle.cpp:34-47) -- the closest
// DFHack-verified precedent for "is this item eligible to be claimed by a
// building-placement job": dump/forbid/garbage_collect/hostile/on_fire/
// rotten/trader/in_building/construction/in_job/owned/removed/encased/
// spider_web all disqualify a candidate.
static df::item *pickFurnitureItem(df::item_type itemType, df::items_other_id otherId,
                                    bool requireEmpty, uint8_t qualityTier)
{
    df::item_flags bad;
    bad.bits.dump = true;
    bad.bits.forbid = true;
    bad.bits.garbage_collect = true;
    bad.bits.hostile = true;
    bad.bits.on_fire = true;
    bad.bits.rotten = true;
    bad.bits.trader = true;
    bad.bits.in_building = true;
    bad.bits.construction = true;
    bad.bits.in_job = true;
    bad.bits.owned = true;
    bad.bits.removed = true;
    bad.bits.encased = true;
    bad.bits.spider_web = true;

    df::item *best = nullptr;
    int16_t bestQuality = 32767;
    for (df::item *it : df::global::world->items.other[otherId]) {
        if (!it || it->getType() != itemType)
            continue;
        if (it->flags.whole & bad.whole)
            continue;
        if (requireEmpty && Items::getGeneralRef(it, df::general_ref_type::CONTAINS_ITEM))
            continue;
        int16_t q = it->getQuality();
        if (q < (int16_t)qualityTier)
            continue;
        if (q < bestQuality) {
            best = it;
            bestQuality = q;
        }
    }
    return best;
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

bool placeWorkshop(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
    int wsType = protocolToWorkshopType(buildType);
    if (wsType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown workshop type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    // Ashery and Dyers take ONLY specific-item reagents (buildings.lua
    // workshop_inputs:251-282) -- no generic building-material reagent at
    // all, unlike every other workshop this plugin places -- so
    // materialClass has nothing to narrow. Reject truthfully up front,
    // same reasoning as placeWell/placeDoor for their all-specific-item
    // reagent lists.
    bool specificItemOnly = (wsType == df::workshop_type::Ashery ||
                             wsType == df::workshop_type::Dyers);
    if (specificItemOnly && materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to this workshop "
                "(every reagent is already a specific finished item, not a raw building-material class)";
        return false;
    }

    std::vector<df::job_item*> filters;
    if (wsType == df::workshop_type::MetalsmithsForge) {
        // MetalsmithsForge is the one workshop type with two reagents: an
        // ANVIL item plus a fire-safe building material (buildings.lua
        // workshop_inputs:220-228). Order matches the reference table —
        // the anvil is attached first. materialClass narrows only the
        // second (generic) reagent -- the anvil is always a specific item.
        filters.push_back(makeAnvilFilter());
        filters.push_back(makeFireSafeBuildMatFilter(materialClass));
    } else if (wsType == df::workshop_type::MagmaForge) {
        // MagmaForge is workshop_type, NOT furnace_type (see protocol.h's
        // BUILD_TYPE_WS_MAGMA_FORGE doc comment) -- it is the magma-fueled
        // twin of MetalsmithsForge just above, same two-reagent shape but
        // magma_safe instead of fire_safe on both reagents (buildings.lua
        // workshop_inputs:229-237).
        filters.push_back(makeMagmaAnvilFilter());
        filters.push_back(makeMagmaSafeBuildMatFilter(materialClass));
    } else if (wsType == df::workshop_type::Siege) {
        // Siege Workshop (ammunition prep -- NOT SiegeEngine, the
        // catapult/ballista building, out of scope) takes 3x the generic
        // building material instead of 1x (buildings.lua workshop_inputs:
        // 240, quantity=3) -- otherwise the same shape as every plain
        // workshop below.
        df::job_item *ji = makeBuildMatFilter(materialClass);
        ji->quantity = 3;
        filters.push_back(ji);
    } else if (wsType == df::workshop_type::Ashery) {
        // Ashery: THREE specific-item reagents, no generic building
        // material at all (buildings.lua workshop_inputs:251-268) --
        // BLOCKS/BLOCKS with no flags, an EMPTY BARREL/BARREL, and a
        // lye_milk_free BUCKET/BUCKET (identical bucket shape to
        // placeWell's bucket reagent above).
        filters.push_back(makeItemFilter(df::item_type::BLOCKS, df::job_item_vector_id::BLOCKS));
        filters.push_back(makeItemFilter(df::item_type::BARREL, df::job_item_vector_id::BARREL, /*requireEmpty=*/true));
        df::job_item *bucket = makeItemFilter(df::item_type::BUCKET, df::job_item_vector_id::BUCKET);
        bucket->flags2.bits.lye_milk_free = true;
        filters.push_back(bucket);
    } else if (wsType == df::workshop_type::Dyers) {
        // Dyers: TWO specific-item reagents, no generic building material
        // (buildings.lua workshop_inputs:269-282) -- an EMPTY BARREL/BARREL
        // and a lye_milk_free BUCKET/BUCKET, same two reagents as Ashery
        // minus the blocks.
        filters.push_back(makeItemFilter(df::item_type::BARREL, df::job_item_vector_id::BARREL, /*requireEmpty=*/true));
        df::job_item *bucket = makeItemFilter(df::item_type::BUCKET, df::job_item_vector_id::BUCKET);
        bucket->flags2.bits.lye_milk_free = true;
        filters.push_back(bucket);
    } else {
        // Every other workshop type we place takes exactly one generic
        // building-material item (buildings.lua workshop_inputs —
        // Carpenters:215, Farmers:216, Masons:217, Craftsdwarfs:218,
        // Jewelers:219, Bowyers:238, Mechanics:239, Butchers:241,
        // Leatherworks:242, Tanners:243, Clothiers:244, Fishery:245,
        // Still:246, Loom:247, Kennels:249, Kitchen:250).
        filters.push_back(makeBuildMatFilter(materialClass));
    }
    return placeBuilding(df::coord(x, y, z), df::building_type::Workshop,
                         wsType, -1, 3, 3, filters, error);
}

// placeFurnace places any of the seven df::furnace_type values (buildings.lua
// furnace_inputs:202-210): WoodFurnace/Smelter/GlassFurnace/Kiln each take
// one FIRE-safe building-material item; MagmaSmelter/MagmaGlassFurnace/
// MagmaKiln instead take one MAGMA-safe item (magma_safe flag, no fire_safe)
// -- see makeFireSafeBuildMatFilter/makeMagmaSafeBuildMatFilter's doc
// comments above for why these are genuinely different filters, not a
// renamed synonym. All seven are forced 3x3 by DF regardless of the
// width/height passed (Buildings::getCorrectSize's Furnace case has no
// per-subtype branch beyond Custom, dfhack-build library/modules/
// Buildings.cpp:688-708), matching the (3,3) call below.
//
// MAGMA-PLACEMENT VALIDATION -- what DFHack does and does NOT check here:
// the only liquid-related gate in the construction path this plugin calls
// (Buildings::checkFreeTiles, via setSize -> checkBuildingTiles) rejects a
// tile when des.bits.flow_size > 1, or flow_size >= 1 with liquid_type ==
// Magma (dfhack-build library/modules/Buildings.cpp:797-800) -- i.e. it
// refuses to build ANY actual building (this applies identically to every
// OTHER building type in this file, not something special to magma
// furnaces) directly into currently-flowing liquid. There is no SEPARATE,
// magma-furnace-specific "is this tile magma-safe / does it have real
// magma access" check anywhere in DFHack's Buildings module -- getCorrectSize
// (quoted above) has no magma-adjacency branch, and vanilla DF's own
// build-menu-side validation for that (the tile highlighting that gates
// where a magma workshop/furnace can be placed in the game's UI) lives in
// DF's own closed-source game code, not in any DFHack-exposed structure or
// function this plugin can call into or reproduce. Concretely: this command
// will happily CONSTRUCT a MagmaSmelter/MagmaGlassFurnace/MagmaKiln (or
// MagmaForge, see placeWorkshop) on ordinary dry floor with no magma access
// at all -- DFHack does not reject that placement, and this plugin does not
// add speculative validation of its own to catch it (this project's house
// rule: never invent DF domain knowledge that can't be confirmed against
// source). Whether the finished building can actually be WORKED (smelt/cast
// jobs run) once magma access is missing is a live-game runtime question,
// not a placement-time one -- same "not modeled, not verifiable short of an
// in-game check" carve-out as placeWaterPowerBuilding's adjacency KNOWN
// LIMITATION above.
bool placeFurnace(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
    int fType = protocolToFurnaceType(buildType);
    if (fType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown furnace type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    bool magmaFueled = (fType == df::furnace_type::MagmaSmelter ||
                        fType == df::furnace_type::MagmaGlassFurnace ||
                        fType == df::furnace_type::MagmaKiln);
    std::vector<df::job_item*> filters;
    filters.push_back(magmaFueled ? makeMagmaSafeBuildMatFilter(materialClass)
                                   : makeFireSafeBuildMatFilter(materialClass));
    return placeBuilding(df::coord(x, y, z), df::building_type::Furnace,
                         fType, -1, 3, 3, filters, error);
}

bool placeTradeDepot(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error) {
    // TradeDepot has no subtype (-1). getCorrectSize forces 5x5 regardless
    // of the width/height we pass (Buildings.cpp:610-614), but we pass 5x5
    // for clarity. Filter per buildings.lua building_inputs TradeDepot
    // entry (buildings.lua:40): 3x generic building material.
    std::vector<df::job_item*> filters;
    df::job_item *ji = makeBuildMatFilter(materialClass);
    ji->quantity = 3;
    filters.push_back(ji);
    return placeBuilding(df::coord(x, y, z), df::building_type::TradeDepot,
                         -1, -1, 5, 5, filters, error);
}

// placeFurniture places a furniture BuildType either via the pre-existing
// any-matching-item path (Buildings::constructWithFilters, when
// qualityTier == QUALITY_TIER_ANY) or, when a quality tier is requested,
// by searching for one already-existing item of at least that quality and
// placing it directly via Buildings::constructWithItems -- vanilla DF has
// no way to request quality at craft time (no manager_order/job_item field
// constrains output quality), so selecting among what already exists is
// the only real lever (see pickFurnitureItem's doc comment above).
// materialClass never applies to furniture (its filter is already a
// specific finished item type, not a raw material class) -- rejected
// truthfully rather than silently ignored.
bool placeFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, uint8_t qualityTier, std::string &error) {
    int fType = protocolToFurnitureType(buildType);
    if (fType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown furniture type: 0x%02X", buildType);
        error = buf;
        return false;
    }
    if (materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to furniture "
                "(already a specific finished item, not a raw building-material reagent) -- omit material, or use quality instead";
        return false;
    }

    FurnitureItemSpec spec;
    if (!furnitureItemSpec(fType, spec)) {
        char buf[64];
        snprintf(buf, sizeof(buf), "No filter mapping for furniture type: 0x%02X", buildType);
        error = buf;
        return false;
    }

    if (qualityTier != QUALITY_TIER_ANY) {
        df::item *chosen = pickFurnitureItem(spec.itemType, spec.otherId, spec.requireEmpty, qualityTier);
        if (!chosen) {
            char buf[320];
            snprintf(buf, sizeof(buf),
                     "no existing %s of quality >= %s is currently available (craft one first -- e.g. set_workshop_profile "
                     "a skilled worker to a shop, then queue_job/order -- or lower the requested quality)",
                     ENUM_KEY_STR(item_type, spec.itemType).c_str(),
                     ENUM_KEY_STR(item_quality, (df::item_quality)qualityTier).c_str());
            error = buf;
            return false;
        }

        df::coord pos(x, y, z);
        df::building *bld = Buildings::allocInstance(pos, (df::building_type)fType, -1, -1);
        if (!bld) {
            error = "Buildings::allocInstance returned null (unknown building class)";
            return false;
        }
        df::coord2d size(1, 1);
        if (!Buildings::setSize(bld, size)) {
            destroyUnlinked(bld);
            error = "cannot place building here (tiles blocked, occupied, or unsuitable)";
            return false;
        }
        std::vector<df::item*> items;
        items.push_back(chosen);
        if (!Buildings::constructWithItems(bld, items)) {
            destroyUnlinked(bld);
            error = "constructWithItems failed (tiles no longer free, or the chosen item became unavailable)";
            return false;
        }

        // Truthful ACK: report the SPECIFIC item actually claimed and
        // placed (material + quality), not just an echo of the request.
        MaterialInfo minfo(chosen);
        char buf[320];
        snprintf(buf, sizeof(buf), "placed existing %s (quality %s, material %s)",
                 ENUM_KEY_STR(item_type, spec.itemType).c_str(),
                 ENUM_KEY_STR(item_quality, (df::item_quality)chosen->getQuality()).c_str(),
                 minfo.isValid() ? minfo.toString().c_str() : "unknown");
        error = buf;
        return true;
    }

    // No quality constraint: the pre-existing any-matching-item path.
    // Filters per buildings.lua building_inputs: Chair:35, Bed:36,
    // Table:37, Box:48-54 (must be empty), Cabinet:67-69, Coffin:38.
    std::vector<df::job_item*> filters;
    filters.push_back(makeItemFilter(spec.itemType, spec.jobVectorId, spec.requireEmpty));
    return placeBuilding(df::coord(x, y, z), (df::building_type)fType,
                         -1, -1, 1, 1, filters, error);
}

bool placeConstruction(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
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
    filters.push_back(makeBuildMatFilter(materialClass));
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
bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
    if (materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to doors/hatches/levers/floodgates "
                "(already a specific finished item, not a raw building-material reagent)";
        return false;
    }
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

// placeTrap handles four MORE df::trap_type subtypes (building_type::Trap)
// beyond Lever, which stays in placeDoor above (BUILD_TYPE_LEVER, doors/
// hatches range) sharing its single-mechanism-item shape. All four are 1x1
// ACTUAL buildings (getCorrectSize has no case for building_type::Trap, so
// every subtype falls to its default branch, Buildings.cpp:736-739 -- same
// as Lever/Door/Hatch/Floodgate/Well/Support above).
//
// Filters per buildings.lua trap_inputs (dfhack-build library/lua/dfhack/
// buildings.lua:298-338):
//   PressurePlate  :324-330  1x mechanism (TRAPPARTS/TRAPPARTS) -- identical
//                             shape to Lever. PressurePlate can ALSO serve as
//                             a link_building SOURCE (not just a target) --
//                             confirmed via building_trapst's own struct
//                             layout (df/building_trapst.h): linked_mechanisms
//                             is a plain field on the shared building_trapst
//                             type, not something only a Lever subtype has,
//                             so mechanisms.cpp's applyLinkBuilding accepts
//                             either trap_type as the source (see that file).
//                             NOTE: this command does NOT configure plate_info
//                             (df::pressure_plate_info) -- DF's own struct
//                             constructor default (df.building.xml:1251-1262)
//                             leaves EVERY detection category (units/water/
//                             magma/citizens/track) OFF (flags init-value
//                             0x10 = only the unrelated "resets" bit), so a
//                             plate built by this command will not actually
//                             fire until something enables at least one
//                             category -- a real, separate gap (a future
//                             command would need to expose plate_info) not
//                             solved by this pass.
//   StoneFallTrap  :299-305  1x mechanism (TRAPPARTS/TRAPPARTS) -- same
//                             shape again. NOTE: this is the CONSTRUCTION
//                             filter only -- a freshly built stone-fall trap
//                             is UNARMED (no boulder loaded) until a separate
//                             df::job_type::LoadStoneTrap job runs, exactly
//                             mirroring vanilla DF's own two-step UI (build
//                             with a mechanism, then Load separately via the
//                             building's own menu) -- this command builds the
//                             fixture; arming it is a distinct, unimplemented
//                             follow-on job this pass does not add (same
//                             carve-out shape as NestBox/Hive's TOOL-crafting
//                             gap in placeRoomValueFurniture above).
//   WeaponTrap     :306-316  2x reagents: mechanism (TRAPPARTS/TRAPPARTS)
//                             plus a weapon (item_type left at its NONE
//                             init-value, vector_id=ANY_WEAPON) -- UNLIKE
//                             StoneFallTrap, DF's own build menu for a
//                             weapon trap takes the weapon reagent AT
//                             CONSTRUCTION time (buildings.lua's own table
//                             confirms this), so this one IS armed the
//                             moment it finishes building. ANY_WEAPON's
//                             master vector covers both actual WEAPON items
//                             and TRAPCOMP (trap component) items
//                             (df.item.xml items_other_id::ANY_WEAPON:
//                             generic_item=WEAPON,TRAPCOMP, no explicit
//                             `item` attr) -- either is valid ammunition,
//                             matching vanilla DF, and this fort already has
//                             weapons in stock from every embark.
//   TrackStop      :338      1x generic building-material item (buildings.lua
//                             -- identical shape to placeSupport/
//                             placeArcheryTarget's makeBuildMatFilter) -- the
//                             ONE trap subtype here that accepts materialClass.
//                             Track stops anchor minecart track infrastructure;
//                             this is basic placement only -- track-piece
//                             linkage, friction/dump-menu settings, and the
//                             rest of DF's minecart system are a separate,
//                             more niche system this pass does not chase.
//
// materialClass is rejected for PressurePlate/StoneFallTrap/WeaponTrap (each
// already a specific finished item, not a raw material class DF could
// narrow) but honored for TrackStop, mirroring placeSupport/
// placeArcheryTarget's acceptance further up this file.
//
// Deliberately NOT here: df::trap_type::CageTrap. Its ammunition is a cage
// item (MakeCage job_type), excluded this project alongside the Cage/Chain
// BUILDING types for justice/restraint reasons -- adding CageTrap would
// repeat the exact "craftable but can never be armed" anti-pattern this
// project already fixed once (ConstructCoffin).
bool placeTrap(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
    df::trap_type trapType;
    std::vector<df::job_item*> filters;
    switch (buildType) {
        case BUILD_TYPE_PRESSURE_PLATE:
            if (materialClass != MATERIAL_CLASS_ANY) {
                error = "material class constraint does not apply to a pressure plate (a specific mechanism item, not a raw building-material reagent)";
                return false;
            }
            trapType = df::trap_type::PressurePlate;
            filters.push_back(makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS));
            break;
        case BUILD_TYPE_STONE_FALL_TRAP:
            if (materialClass != MATERIAL_CLASS_ANY) {
                error = "material class constraint does not apply to a stone-fall trap (a specific mechanism item, not a raw building-material reagent)";
                return false;
            }
            trapType = df::trap_type::StoneFallTrap;
            filters.push_back(makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS));
            break;
        case BUILD_TYPE_WEAPON_TRAP:
            if (materialClass != MATERIAL_CLASS_ANY) {
                error = "material class constraint does not apply to a weapon trap (specific mechanism/weapon items, not a raw building-material reagent)";
                return false;
            }
            trapType = df::trap_type::WeaponTrap;
            filters.push_back(makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS));
            filters.push_back(makeItemFilter(df::item_type::NONE, df::job_item_vector_id::ANY_WEAPON));
            break;
        case BUILD_TYPE_TRACK_STOP:
            trapType = df::trap_type::TrackStop;
            filters.push_back(makeBuildMatFilter(materialClass));
            break;
        default: {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown trap build type: 0x%02X", buildType);
            error = buf;
            return false;
        }
    }
    return placeBuilding(df::coord(x, y, z), df::building_type::Trap, trapType, -1, 1, 1, filters, error);
}

// ---------------------------------------------------------------------------
// Misc/infrastructure placers (BUILD_TYPE range 0x90-0x9F). Both types below
// are 1x1 ACTUAL buildings -- getCorrectSize (Buildings.cpp) has no case for
// either Well or Support, so both fall to its default branch: forced 1x1,
// center (0,0) -- exactly like placeDoor's four types above.
// ---------------------------------------------------------------------------

// placeWell places a Well (df::building_type::Well) -- DF's water-drawing
// building, the missing piece needed to turn a sealed aquifer pierce into a
// deliberate water source instead of just a sealed-off hazard. Filter per
// buildings.lua building_inputs Well entry (buildings.lua:79-100): FOUR
// distinct item reagents, each a specific finished item -- no generic
// building-material class applies to any of them:
//   1. BLOCKS/BLOCKS, no flags at all (buildings.lua:80-83) -- makeItemFilter
//      with requireEmpty=false covers this exactly.
//   2. bucket: BUCKET/BUCKET plus flags2.lye_milk_free (buildings.lua:84-89 --
//      original-name NOT_CONTAIN_BARREL_ITEM, confirmed df.d_basics.xml:2869
//      / job_item_flags2.h) -- built directly below since makeItemFilter has
//      no flags2 parameter.
//   3. chain: CHAIN/CHAIN, no flags (buildings.lua:90-94) -- makeItemFilter.
//   4. mechanism: TRAPPARTS/TRAPPARTS, no flags (buildings.lua:95-99) --
//      identical to placeDoor's Lever mechanism filter above.
// materialClass is rejected, same as placeDoor/placeFurniture: every
// reagent here is already a specific finished item type, not a raw
// building-material class DF could narrow.
bool placeWell(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error) {
    if (materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to a well "
                "(all four reagents are specific finished items, not a raw building-material reagent)";
        return false;
    }
    std::vector<df::job_item*> filters;
    filters.push_back(makeItemFilter(df::item_type::BLOCKS, df::job_item_vector_id::BLOCKS));
    df::job_item *bucket = makeItemFilter(df::item_type::BUCKET, df::job_item_vector_id::BUCKET);
    bucket->flags2.bits.lye_milk_free = true;
    filters.push_back(bucket);
    filters.push_back(makeItemFilter(df::item_type::CHAIN, df::job_item_vector_id::CHAIN));
    filters.push_back(makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS));
    return placeBuilding(df::coord(x, y, z), df::building_type::Well,
                         -1, -1, 1, 1, filters, error);
}

// placeSupport places a Support (df::building_type::Support) -- a
// cave-in/collapse-trigger fixture, confirmed real and entirely unrelated to
// justice/restraint despite sharing this file's chain/mechanism vocabulary.
// Filter per buildings.lua building_inputs Support entry (buildings.lua:111):
// one generic building-material item, the identical shape placeConstruction
// already uses -- makeBuildMatFilter(materialClass) covers it exactly.
bool placeSupport(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error) {
    std::vector<df::job_item*> filters;
    filters.push_back(makeBuildMatFilter(materialClass));
    return placeBuilding(df::coord(x, y, z), df::building_type::Support,
                         -1, -1, 1, 1, filters, error);
}

// placeArcheryTarget places an ArcheryTarget (df::building_type::ArcheryTarget)
// -- a single-tile marksman-dwarf training target, entirely unrelated to
// justice/restraint despite sharing this file's mechanism/chain vocabulary
// elsewhere. Filter per buildings.lua building_inputs ArcheryTarget entry
// (buildings.lua:112): one generic building-material item, the identical
// shape placeConstruction/placeSupport already use -- makeBuildMatFilter
// (materialClass) covers it exactly.
bool placeArcheryTarget(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error) {
    std::vector<df::job_item*> filters;
    filters.push_back(makeBuildMatFilter(materialClass));
    return placeBuilding(df::coord(x, y, z), df::building_type::ArcheryTarget,
                         -1, -1, 1, 1, filters, error);
}

// placeRoomValueFurniture places any of the eight "room-value furniture"
// building types (BUILD_TYPE_STATUE..BUILD_TYPE_INSTRUMENT, protocol.h) plus
// three unrelated but same-shaped types (BUILD_TYPE_TRACTION_BENCH/NEST_BOX/
// HIVE) --
// each a 1x1 ACTUAL building (getCorrectSize has no case for any of them, so
// all fall to its default branch, exactly like placeWell/placeSupport
// above), placed from ONE already-crafted/existing item (or, for the three
// has_tool_use types, any TOOL item serving that function). This closes the
// "craftable but not placeable" gap for Statue and Slab specifically
// (ConstructStatue/ConstructSlab were already whitelisted for
// Carpenters/Masons in a prior wave -- see queue_job's
// jobTypeAllowedAtWorkshop) the same way an earlier wave closed it for
// Coffin.
//
// Filters per buildings.lua building_inputs (dfhack-build library/lua/
// dfhack/buildings.lua):
//   Statue           :70      item_type=STATUE,  vector_id=STATUE
//   Slab             :178     item_type=SLAB,    NO vector_id -- deliberately
//                              left at the df.job.xml init-value IN_PLAY,
//                              mirroring the reference table exactly rather
//                              than guessing a narrower vector
//   WindowGlass      :71      item_type=WINDOW,  vector_id=WINDOW
//   WindowGem        :72-78   item_type=SMALLGEM, vector_id=SMALLGEM, quantity=3
//   Bookcase         :183     has_tool_use=BOOKCASE,       item_type=TOOL (makeToolUseFilter)
//   DisplayFurniture :184     has_tool_use=DISPLAY_OBJECT, item_type=TOOL (makeToolUseFilter)
//   OfferingPlace    :181     has_tool_use=PLACE_OFFERING, item_type=TOOL (makeToolUseFilter)
//   Instrument       :182     item_type=INSTRUMENT, vector_id=INSTRUMENT_STATIONARY
//
// Also handles three more BuildType bytes that share this exact shape purely
// as CODE REUSE, not because they are DF's own "room-value" furniture (none
// of the three raises a bedroom's furnishing-value score):
//   TractionBench    :172-177 item_type=TRACTION_BENCH, vector_id=TRACTION_BENCH
//                              -- hospital splint-traction furniture; the
//                              TRACTION_BENCH item itself is crafted via
//                              ConstructTractionBench at a Mechanic's
//                              workshop (see work_orders.cpp)
//   NestBox          :179     has_tool_use=NEST_BOX,  item_type=TOOL (makeToolUseFilter)
//   Hive             :180     has_tool_use=HIVE,      item_type=TOOL (makeToolUseFilter)
// NestBox/Hive placement itself works the same as Bookcase/DisplayFurniture/
// OfferingPlace above (any existing TOOL item with the right has_tool_use
// function); crafting THAT tool item in the first place is a separate,
// pre-existing gap -- MakeTool job_items need an item_subtype (which
// itemdef_toolst the tool should be), and queue_job's generic job_item
// construction has no parameter to supply one. Out of scope for this pass.
//
// materialClass is rejected for all eleven, same as placeDoor/placeWell:
// every reagent here is already a specific finished item (or a tool-use
// FUNCTION filter), never a raw building-material class DF could narrow.
//
// No quality-tier support (unlike Bed/Table/.../Coffin's placeFurniture
// path): pickFurnitureItem's scan matches on item_type+vector_id alone and
// has no notion of has_tool_use, so it cannot discriminate a
// bookcase-capable TOOL item from a display-pedestal-capable one -- reusing
// that path here would silently accept ANY loose tool regardless of
// function. A future task can extend FurnitureItemSpec/pickFurnitureItem
// with a has_tool_use-aware scan if quality selection is ever wanted for
// these three; Statue/Slab/WindowGlass/WindowGem/Instrument have no such
// blocker and could gain it more cheaply, but are kept uniform with the
// other three here rather than special-cased.
bool placeRoomValueFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error) {
    if (materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to this building type "
                "(already a specific finished item or tool-use function, not a raw building-material reagent)";
        return false;
    }

    std::vector<df::job_item*> filters;
    df::building_type bt;
    switch (buildType) {
        case BUILD_TYPE_STATUE:
            filters.push_back(makeItemFilter(df::item_type::STATUE, df::job_item_vector_id::STATUE));
            bt = df::building_type::Statue;
            break;
        case BUILD_TYPE_SLAB:
            filters.push_back(makeItemFilter(df::item_type::SLAB, df::job_item_vector_id::IN_PLAY));
            bt = df::building_type::Slab;
            break;
        case BUILD_TYPE_WINDOW_GLASS:
            filters.push_back(makeItemFilter(df::item_type::WINDOW, df::job_item_vector_id::WINDOW));
            bt = df::building_type::WindowGlass;
            break;
        case BUILD_TYPE_WINDOW_GEM: {
            df::job_item *ji = makeItemFilter(df::item_type::SMALLGEM, df::job_item_vector_id::SMALLGEM);
            ji->quantity = 3;
            filters.push_back(ji);
            bt = df::building_type::WindowGem;
            break;
        }
        case BUILD_TYPE_BOOKCASE:
            filters.push_back(makeToolUseFilter(df::tool_uses::BOOKCASE));
            bt = df::building_type::Bookcase;
            break;
        case BUILD_TYPE_DISPLAY_FURNITURE:
            filters.push_back(makeToolUseFilter(df::tool_uses::DISPLAY_OBJECT));
            bt = df::building_type::DisplayFurniture;
            break;
        case BUILD_TYPE_OFFERING_PLACE:
            filters.push_back(makeToolUseFilter(df::tool_uses::PLACE_OFFERING));
            bt = df::building_type::OfferingPlace;
            break;
        case BUILD_TYPE_INSTRUMENT:
            // Instruments split into two distinct DF item categories that
            // both map to item_type INSTRUMENT (df.item.xml's
            // items_other_id/job_item_vector_id enums each carry an
            // INSTRUMENT entry AND a separate INSTRUMENT_STATIONARY entry):
            // INSTRUMENT_STATIONARY is buildable furniture (what this
            // building type needs); plain INSTRUMENT is handheld, carried
            // and played by musicians from a stockpile, and can never
            // satisfy this filter. constructWithFilters only validates tile
            // placement, not filter satisfiability -- without this
            // pre-check a caravan-bought handheld instrument ACKs SUCCESS
            // here and only fails much later, live, when DF's own
            // job-assignment gives up and cancels with a generic "unable to
            // complete" (no fort-side hint why). Reuses pickFurnitureItem's
            // existing scan/bad-flags screen as a pure existence check.
            // MUST pass Ordinary (0), not QUALITY_TIER_ANY (0xFF):
            // pickFurnitureItem's `q < (int16_t)qualityTier` threshold
            // comparison has no ANY special case, so passing the ANY
            // sentinel here would make it reject every real item (quality
            // only ranges 0-6) and always report "none in stock".
            if (!pickFurnitureItem(df::item_type::INSTRUMENT, df::items_other_id::INSTRUMENT_STATIONARY,
                                    false, (uint8_t)df::item_quality::Ordinary)) {
                error = "no stationary instrument in stock (handheld instruments are not "
                        "placeable -- performers use them from stockpiles, not this building type)";
                return false;
            }
            filters.push_back(makeItemFilter(df::item_type::INSTRUMENT, df::job_item_vector_id::INSTRUMENT_STATIONARY));
            bt = df::building_type::Instrument;
            break;
        case BUILD_TYPE_TRACTION_BENCH:
            filters.push_back(makeItemFilter(df::item_type::TRACTION_BENCH, df::job_item_vector_id::TRACTION_BENCH));
            bt = df::building_type::TractionBench;
            break;
        case BUILD_TYPE_NEST_BOX:
            filters.push_back(makeToolUseFilter(df::tool_uses::NEST_BOX));
            bt = df::building_type::NestBox;
            break;
        case BUILD_TYPE_HIVE:
            filters.push_back(makeToolUseFilter(df::tool_uses::HIVE));
            bt = df::building_type::Hive;
            break;
        default: {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown room-value-furniture build type: 0x%02X", buildType);
            error = buf;
            return false;
        }
    }
    return placeBuilding(df::coord(x, y, z), bt, -1, -1, 1, 1, filters, error);
}

// placeWaterPowerBuilding places any of the seven water/power-transmission
// building types (BUILD_TYPE_SCREW_PUMP..BUILD_TYPE_ROLLERS, protocol.h) --
// each an ACTUAL building whose footprint DF computes from the building
// type PLUS the orientation byte (Buildings::getCorrectSize, Buildings.cpp:
// 634-651,710-734), never from a caller-chosen width/height the way Bridge
// works. This is why it is NOT built on top of placeBuilding() above (that
// helper always calls the 2-arg setSize, direction fixed at 0) -- it needs
// the 3-arg overload, same reason placeBridge is bespoke. Unlike Bridge,
// though, the caller supplies only a single anchor tile (x,y,z): the
// initial size passed to setSize below is a placeholder DF overwrites for
// every one of these seven types, exactly like placeWell/placeSupport's
// forced-1x1 case above, just with more shapes than "always 1x1".
//
// wireOrient is BUILD_ORIENT_* (protocol.h) -- passed straight through as
// the raw `direction` int argument to Buildings::setSize's 3-arg overload,
// which DF itself casts two different ways depending on building type (see
// BUILD_ORIENT_*'s doc comment): a boolean (AxleHorizontal/WaterWheel) or
// a 4-way df::screw_pump_direction (ScrewPump/Rollers). GearAssembly/
// AxleVertical ignore it entirely (getCorrectSize has no case for either,
// so any value is harmless). BUILD_ORIENT_ANY (the wire's "unspecified"
// sentinel) maps to 0 -- a legal default for every one of these types.
//
// materialClass is rejected, same as placeDoor/placeWell/
// placeRoomValueFurniture: every reagent below is either a specific
// finished item or a hardcoded WOOD filter (buildings.lua's own reagent
// table names item_type=WOOD directly, not a flags2.building_material
// filter a caller's material CLASS could narrow) -- axles/water wheels/
// windmills are always wood in vanilla DF, not a caller choice.
//
// KNOWN LIMITATION 1 -- adjacency: see BUILD_TYPE_SCREW_PUMP et al's doc
// comment in protocol.h -- which tile touches which is entirely DF's own
// engine-level concern once pieces exist, not something this plugin sets
// or verifies at placement time.
//
// KNOWN LIMITATION 2 -- AxleHorizontal/Rollers are always built ONE TILE
// long: unlike WaterWheel/ScrewPump/Windmill (whose getCorrectSize cases
// OVERWRITE size unconditionally, Buildings.cpp:610-614,634-638,710-734)
// and GearAssembly/AxleVertical (always 1x1, no case at all), AxleHorizontal
// and Rollers instead PASS THROUGH whatever size the caller requested on
// the non-flattened axis (Buildings.cpp:640-642,644-646 -- `makeOneDim`
// only forces ONE axis to 1; the other keeps its input value), the SAME
// "caller chooses the footprint" family Bridge/FarmPlot/Stockpile belong
// to (return true, not false). This plugin has no wire parameter carrying
// a caller-chosen length for this family today -- BUILD_TYPE_* commands
// here are a single anchor tile (x,y,z), not a two-corner rectangle like
// BUILD_BRIDGE -- so the size passed to setSize below is always (1,1),
// which DF happily honors as a valid (if minimal) one-tile axle/roller
// segment. A vanilla-DF-equivalent longer line can still be built by
// placing multiple adjacent one-tile segments end to end (DF's own
// engine-level adjacency links them into one working machine, same as
// limitation 1 above) -- strictly more wood/mechanism cost than DF's
// native multi-tile single-building drag (that scales via quantity=-1's
// floor(N/4)+1, one segment per tile does not), but mechanically
// equivalent. A future wave could add a caller-chosen length (mirroring
// Bridge's width/height) if this cost difference matters in practice.
bool placeWaterPowerBuilding(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, uint8_t wireOrient, std::string &error) {
    if (materialClass != MATERIAL_CLASS_ANY) {
        error = "material class constraint does not apply to water/power infrastructure "
                "(every reagent is already a specific finished item or a fixed wood requirement, not a raw building-material class)";
        return false;
    }

    int direction = 0;
    if (wireOrient != BUILD_ORIENT_ANY) {
        if (wireOrient > BUILD_ORIENT_WEST) {
            char buf[96];
            snprintf(buf, sizeof(buf), "invalid orientation byte: 0x%02X (want 0-3, or 0xFF for default)", wireOrient);
            error = buf;
            return false;
        }
        direction = (int)wireOrient;
    }

    df::building_type bt;
    switch (buildType) {
        case BUILD_TYPE_SCREW_PUMP:      bt = df::building_type::ScrewPump;      break;
        case BUILD_TYPE_GEAR_ASSEMBLY:   bt = df::building_type::GearAssembly;   break;
        case BUILD_TYPE_AXLE_HORIZONTAL: bt = df::building_type::AxleHorizontal; break;
        case BUILD_TYPE_AXLE_VERTICAL:   bt = df::building_type::AxleVertical;   break;
        case BUILD_TYPE_WATER_WHEEL:     bt = df::building_type::WaterWheel;    break;
        case BUILD_TYPE_WINDMILL:        bt = df::building_type::Windmill;      break;
        case BUILD_TYPE_ROLLERS:         bt = df::building_type::Rollers;       break;
        default: {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown water/power build type: 0x%02X", buildType);
            error = buf;
            return false;
        }
    }

    df::building *bld = Buildings::allocInstance(df::coord(x, y, z), bt, -1, -1);
    if (!bld) {
        error = "Buildings::allocInstance returned null (unknown building class)";
        return false;
    }

    // Placeholder size -- getCorrectSize overwrites it per-type/direction
    // for every one of these seven types (forced 1x1, forced 3x3, or a
    // computed one-tile-wide line); the caller never chooses the footprint
    // here, unlike Bridge.
    df::coord2d size(1, 1);
    if (!Buildings::setSize(bld, size, direction)) {
        destroyUnlinked(bld);
        error = "cannot place building here (tiles blocked, occupied, or unsuitable -- "
                "or the requested orientation leaves no room for the resulting footprint)";
        return false;
    }

    // Filters per buildings.lua building_inputs (dfhack-build library/lua/
    // dfhack/buildings.lua):
    //   ScrewPump       :116-132  BLOCKS/BLOCKS; screw: flags2.screw + TRAPCOMP/ANY_WEAPON; pipe: PIPE_SECTION/PIPE_SECTION
    //   GearAssembly    :147-153  mechanism: TRAPPARTS/TRAPPARTS
    //   AxleHorizontal  :154-156  WOOD/WOOD, quantity=-1 (scales with length, same computeMaterialAmount sentinel as Bridge)
    //   AxleVertical    :157      WOOD/WOOD
    //   WaterWheel      :158-164  WOOD/WOOD, quantity=3
    //   Windmill        :165-171  WOOD/WOOD, quantity=4
    //   Rollers         :185-197  mechanism: TRAPPARTS/TRAPPARTS quantity=-1 (scales with length); chain: CHAIN/CHAIN
    std::vector<df::job_item*> filters;
    switch (buildType) {
        case BUILD_TYPE_SCREW_PUMP: {
            filters.push_back(makeItemFilter(df::item_type::BLOCKS, df::job_item_vector_id::BLOCKS));
            df::job_item *screw = makeItemFilter(df::item_type::TRAPCOMP, df::job_item_vector_id::ANY_WEAPON);
            screw->flags2.bits.screw = true;
            filters.push_back(screw);
            filters.push_back(makeItemFilter(df::item_type::PIPE_SECTION, df::job_item_vector_id::PIPE_SECTION));
            break;
        }
        case BUILD_TYPE_GEAR_ASSEMBLY:
            filters.push_back(makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS));
            break;
        case BUILD_TYPE_AXLE_HORIZONTAL: {
            df::job_item *ji = makeItemFilter(df::item_type::WOOD, df::job_item_vector_id::WOOD);
            // DFHack sentinel -- computeMaterialAmount() fills in the real
            // count from the ACTUAL footprint (floor(tileCount/4)+1); with
            // this plugin's current always-1x1 footprint (see KNOWN
            // LIMITATION 2 above) that's always 1 log, same as omitting
            // quantity entirely -- kept as -1 anyway to match buildings.lua
            // buildings.lua:155 exactly and to do the right thing for free
            // if a future wave adds a caller-chosen length.
            ji->quantity = -1;
            filters.push_back(ji);
            break;
        }
        case BUILD_TYPE_AXLE_VERTICAL:
            filters.push_back(makeItemFilter(df::item_type::WOOD, df::job_item_vector_id::WOOD));
            break;
        case BUILD_TYPE_WATER_WHEEL: {
            df::job_item *ji = makeItemFilter(df::item_type::WOOD, df::job_item_vector_id::WOOD);
            ji->quantity = 3;
            filters.push_back(ji);
            break;
        }
        case BUILD_TYPE_WINDMILL: {
            df::job_item *ji = makeItemFilter(df::item_type::WOOD, df::job_item_vector_id::WOOD);
            ji->quantity = 4;
            filters.push_back(ji);
            break;
        }
        case BUILD_TYPE_ROLLERS: {
            df::job_item *mech = makeItemFilter(df::item_type::TRAPPARTS, df::job_item_vector_id::TRAPPARTS);
            mech->quantity = -1; // DFHack sentinel -- see AxleHorizontal's identical comment above (KNOWN LIMITATION 2: always 1 tile long today)
            filters.push_back(mech);
            filters.push_back(makeItemFilter(df::item_type::CHAIN, df::job_item_vector_id::CHAIN));
            break;
        }
    }

    if (!Buildings::constructWithFilters(bld, filters)) {
        destroyUnlinked(bld);
        error = "constructWithFilters failed (tiles no longer free)";
        return false;
    }
    return true;
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

// applySetWorkshopProfile writes DF's real skill-gating mechanism
// (df::workshop_profile -- df.building.xml struct-type workshop_profile,
// fields permitted_workers/min_level/max_level, confirmed source-side)
// onto the ACTUAL BUILT workshop, furnace, or mechanism trap (Lever)
// occupying (x,y,z). df::building::getWorkshopProfile() (df.building.xml:
// 414-416) is a virtual method on the base building class returning null
// for any type without a profile -- using it instead of casting to a
// specific subtype means this works uniformly across all three
// profile-bearing building classes (building_workshopst, building_furnacest,
// building_trapst all declare a `profile` member, df.building.xml:
// 1300/1592/1669), the same pattern DFHack's own autogems.cpp uses
// (workshop->getWorkshopProfile()).
//
// This is the complement to placeFurniture's quality-tier path above:
// that one SELECTS an existing item of a given quality at placement time;
// this one raises the ODDS of a shop producing higher-quality output in
// the first place by restricting who may work there and how skilled they
// must be -- DF has no way to request quality directly at craft time.
//
// maxSkillLevel == -1 is the wire "uncapped" sentinel, translated to DF's
// own upper-cap value (3000 -- dfhack-build/plugins/lua/orders.lua
// MAX_SKILL_RATINGS[#MAX_SKILL_RATINGS], matching workshop_profile's own
// max_level init-value in df.building.xml). workerUnitID == -1 means
// "leave permitted_workers untouched"; otherwise the unit is appended to
// the whitelist if not already present -- a non-empty permitted_workers
// list overrides the skill range entirely for listed workers (DF's own
// vanilla semantics, dfhack-build/plugins/lua/orders.lua
// can_set_skill_level: the skill-restriction UI itself only appears when
// permitted_workers is empty).
bool applySetWorkshopProfile(int16_t x, int16_t y, int16_t z,
                              int32_t minSkillLevel, int32_t maxSkillLevel,
                              int32_t workerUnitID, std::string &error)
{
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (minSkillLevel < 0 || minSkillLevel > 20) {
        error = "invalid min_skill_level (want 0-20)";
        return false;
    }
    if (maxSkillLevel != -1 && (maxSkillLevel < 0 || maxSkillLevel > 20)) {
        error = "invalid max_skill_level (want 0-20, or -1 for uncapped)";
        return false;
    }
    if (maxSkillLevel != -1 && maxSkillLevel < minSkillLevel) {
        error = "max_skill_level must be >= min_skill_level";
        return false;
    }

    df::building *bld = Buildings::findAtTile(df::coord(x, y, z));
    if (!bld) {
        error = "no building at that tile";
        return false;
    }
    // Mirrors applySetFarmCrop's construction-stage guard: writing a
    // profile onto a still-under-construction building is meaningless (no
    // worker can be assigned to a shop that doesn't functionally exist
    // yet), so refuse with the exact stage rather than silently no-op.
    if (bld->getBuildStage() < bld->getMaxBuildStage()) {
        error = "building still under construction (stage " +
                std::to_string(bld->getBuildStage()) + "/" +
                std::to_string(bld->getMaxBuildStage()) +
                ") -- set its profile after construction completes";
        return false;
    }

    df::workshop_profile *profile = bld->getWorkshopProfile();
    if (!profile) {
        error = "building at that tile has no workshop profile (not a workshop, furnace, or mechanism trap)";
        return false;
    }

    profile->min_level = minSkillLevel;
    profile->max_level = (maxSkillLevel == -1) ? 3000 : maxSkillLevel;

    std::string workerNote;
    if (workerUnitID >= 0) {
        df::unit *unit = df::unit::find(workerUnitID);
        if (!unit) {
            error = "no unit with that id";
            return false;
        }
        bool already = false;
        for (int32_t id : profile->permitted_workers) {
            if (id == workerUnitID) { already = true; break; }
        }
        if (!already) {
            profile->permitted_workers.push_back(workerUnitID);
            workerNote = ", worker added to permitted-workers whitelist";
        } else {
            workerNote = ", worker already on the permitted-workers whitelist";
        }
    }

    char buf[192];
    snprintf(buf, sizeof(buf), "min_level=%d max_level=%d, %zu permitted worker(s)%s",
             profile->min_level, profile->max_level, profile->permitted_workers.size(),
             workerNote.c_str());
    error = buf;
    return true;
}
