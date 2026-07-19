// DFHack Plugin — Manager Work Orders
//
// Implements WORK_ORDER commands by adding entries to the fortress's
// manager order queue. The manager dispatches each order to whichever
// workshop can handle it, fetching reagents from stockpiles automatically.
//
// IMPORTANT: The manager_order struct and df::global::world->manager_orders
// field have shifted across DF 53.x versions. This implementation targets
// DFHack 53.12 conventions. When upgrading, verify against the SDK headers:
//
//   df/manager_order.h          — manager_order struct fields
//   df/job_type.h               — JobType enum (ConstructBed etc.)
//   df/world.h                  — manager_orders.all field name & type
//   modules/Job.h               — any helper for allocating manager_order
//
// On a fresh DFHack version, the most likely issue is the JobType enum
// value names (e.g., ConstructBed vs MakeBed) or the manager_order
// allocation pattern (some versions use Job::createManagerOrder helper,
// others require direct struct construction and push_back).

#include "Core.h"
#include "Console.h"
#include "modules/Job.h"
#include "modules/Buildings.h"
#include "modules/Maps.h"
#include "modules/Materials.h"

#include "df/job_type.h"
#include "df/job.h"
#include "df/job_item.h"
#include "df/job_item_vector_id.h"
#include "df/manager_order.h"
#include "df/job_material_category.h"
#include "df/workquota_frequency_type.h"
#include "df/world.h"
#include "df/building.h"
#include "df/building_type.h"
#include "df/building_workshopst.h"
#include "df/workshop_type.h"
#include "df/building_furnacest.h"
#include "df/furnace_type.h"
#include "df/general_ref_type.h"
#include "df/reaction.h"
#include "df/reaction_reagent.h"
#include "df/reaction_reagent_itemst.h"
#include "df/reaction_reagent_type.h"
#include "df/reaction_flags.h"
#include "df/plotinfost.h"
#include "df/historical_entity.h"
#include "df/entity_raw.h"
#include "df/builtin_mats.h"

#include "protocol.h"

#include <vector>
#include <string>
#include <sstream>
#include <cstdio>

using namespace DFHack;

// Map protocol order type → df::job_type. Returns -1 if unrecognized.
//
// VERIFY: Each JobType name against df/job_type.h in DFHack 53.12. Some
// names may differ (e.g., ConstructTable vs MakeTable). Item-production
// jobs typically include ConstructBed/Table/Chair/Door/Cabinet/Coffer
// (Box) plus BrewDrink, PrepareMeal, MakeCrafts.
static int protocolToJobType(uint8_t orderType) {
    switch (orderType) {
        case ORDER_TYPE_MAKE_BED:     return df::job_type::ConstructBed;
        case ORDER_TYPE_MAKE_TABLE:   return df::job_type::ConstructTable;
        case ORDER_TYPE_MAKE_CHAIR:   return df::job_type::ConstructThrone;   // "Throne" in code = chair
        case ORDER_TYPE_MAKE_DOOR:    return df::job_type::ConstructDoor;
        case ORDER_TYPE_MAKE_BARREL:  return df::job_type::MakeBarrel;
        case ORDER_TYPE_MAKE_BUCKET:  return df::job_type::MakeBucket;
        case ORDER_TYPE_MAKE_CABINET: return df::job_type::ConstructCabinet;
        case ORDER_TYPE_MAKE_COFFER:  return df::job_type::ConstructChest;     // 53.12 renamed Box → Chest
        case ORDER_TYPE_BREW_DRINK:   return -1;                                // BrewDrink not in 53.12 job_type; needs reagent-based reaction lookup
        case ORDER_TYPE_PREPARE_MEAL: return df::job_type::PrepareMeal;        // VERIFY: in some versions takes a meal-size param
        case ORDER_TYPE_MAKE_BLOCKS:  return df::job_type::ConstructBlocks;
        case ORDER_TYPE_MAKE_CRAFTS:  return df::job_type::MakeCrafts;
        default: return -1;
    }
}

// resolveJobTypeByName looks up a df::job_type by its DFHack enum KEY name
// (e.g. "ConstructBed", "ConstructHatchCover" — the C++ enum-item `name`
// attr in df.job.xml, NOT the raw bay12 CONSTRUCT_BED token) using
// DFHack's find_enum_item<T> (library/include/DataDefs.h:842-855 in the
// 53.15-r1 checkout) — a plain linear scan over df::job_type's key_table,
// never throws, returns false and leaves *var unwritten on no match. This
// is the exact mechanism DFHack's own built-in `orders` plugin uses to turn
// a manager-order JSON's "job" string into a job_type
// (plugins/orders.cpp:531, orders_import) — reused here so this plugin
// never needs a new protocolToJobType switch case for a job_type DFHack
// already knows about. ORDER_TYPE_BY_NAME (protocol.h) is the wire
// sentinel that routes callers here instead of protocolToJobType above.
//
// Reachable from both applyQueueJob (the direct-to-workshop path) and, as
// of a later pass, applyWorkOrder (the manager-queue path) below — see
// ORDER_TYPE_BY_NAME's comment in protocol.h for how each path's own wire
// shape carries the trailing name. applyWorkOrder needs no job_item/
// workshop-compatibility table at all to make use of this (unlike
// applyQueueJob, which still gates every resolved job_type through
// jobTypeAllowedAtWorkshop/jobTypeAllowedAtFurnace plus a material-class
// job_item filter): a manager_order only ever carries a job_type, and DF's
// own manager fills in job_items and picks a compatible workshop once a
// Manager noble with an office scans the queue -- this function's return
// value is already everything applyWorkOrder needs.
static int resolveJobTypeByName(const std::string &name, std::string &error) {
    df::job_type jt;
    if (!find_enum_item(&jt, name)) {
        error = "unrecognized job type name: '" + name +
                "' (use the job_types query/tool to look up valid DFHack job_type names)";
        return -1;
    }
    return (int)jt;
}

// setOrderMaterial resolves a caller-supplied material token onto a
// manager_order's material selector -- the fix for this project's own
// root-cause finding (2026-07-19 manager-work-order research report): a
// manager_order needs ONE of two selectors set correctly before DF's
// manager can resolve a workshop/reagent set for any job_type with real
// material ambiguity (a MakeFigurine's wood/stone/metal choice, which also
// picks the workshop family), and applyWorkOrder previously set neither.
//
// Two mechanisms, tried in order:
//   1. A job_material_category keyword (df.d_basics.xml's job_material_
//      category bitfield -- library/xml/df.d_basics.xml:2902-2917) --
//      curated to the 8 bits DFHack's own reference order library
//      (data/orders/basic.json) actually uses for reachable job types
//      (ConstructBed/ConstructBin/MakeBucket etc. all key off "wood").
//      wood2/soap/tooth/horn/pearl/strand have no such reference use and
//      are left out of this keyword table (still reachable via mechanism 2
//      below through their raw material token if ever needed).
//   2. MaterialInfo::find(token) -- the exact mechanism DFHack's own
//      `orders` plugin uses to resolve a manager order's "material" JSON
//      field (dfhack-build/plugins/orders.cpp:590-600, orders_import) --
//      resolves an exact DFHack material token (e.g. "INORGANIC",
//      "INORGANIC:LIMONITE", "COAL") onto mat_type/mat_index.
//
// An empty material is a no-op (order->mat_type/mat_index/material_category
// stay at their struct defaults set by the caller) -- the correct
// generic-INORGANIC default for stone-safe job types like ConstructBlocks,
// confirmed by this project's own live observation. A non-empty token that
// resolves via NEITHER mechanism is a truthful FAILED ack, never a silent
// fallback to generic (matches the truthful-ACK house rule).
static bool setOrderMaterial(df::manager_order *order, const std::string &material, std::string &error) {
    if (material.empty()) return true;

    std::string lower = material;
    for (auto &c : lower) c = tolower(c);

    if (lower == "plant")   { order->material_category.bits.plant   = true; return true; }
    if (lower == "wood")    { order->material_category.bits.wood    = true; return true; }
    if (lower == "cloth")   { order->material_category.bits.cloth   = true; return true; }
    if (lower == "silk")    { order->material_category.bits.silk    = true; return true; }
    if (lower == "leather") { order->material_category.bits.leather = true; return true; }
    if (lower == "bone")    { order->material_category.bits.bone    = true; return true; }
    if (lower == "shell")   { order->material_category.bits.shell   = true; return true; }
    if (lower == "yarn")    { order->material_category.bits.yarn    = true; return true; }

    MaterialInfo mat;
    if (!mat.find(material)) {
        error = "unrecognized material '" + material +
                "' -- try a job_material_category keyword (wood, bone, shell, "
                "leather, silk, plant, cloth, yarn) or an exact DFHack material "
                "token (e.g. INORGANIC, INORGANIC:LIMONITE, COAL)";
        return false;
    }
    order->mat_type = mat.type;
    order->mat_index = mat.index;
    return true;
}

// jobTypeName is the ORDER_TYPE_BY_NAME payload's trailing name
// (protocol.h) — empty and ignored unless orderType == ORDER_TYPE_BY_NAME,
// in which case it takes priority over orderType and is resolved via
// resolveJobTypeByName exactly like applyQueueJob's own by-name path does.
// Added in a later pass than the rest of this function: previously
// applyWorkOrder only accepted the hand-maintained ORDER_TYPE_* bytes
// above. Unlike applyQueueJob, no hand-maintained workshop-compatibility
// TABLE is needed for a resolved job_type here — DF's manager picks a
// compatible workshop itself once a Manager noble with an office scans the
// queue, so any job_type find_enum_item resolves is immediately usable, no
// per-job_type whitelist required. NOTE: this generalizes job_type
// resolution only, same scope boundary as queue_job — ORDER_TYPE_CUSTOM_
// REACTION (the reaction-code path) is deliberately NOT accepted here;
// ORDER_TYPE_BREW_DRINK below remains the one hand-coded reaction special
// case this path supports.
//
// material and frequencyByte are this project's 2026-07-19 manager-work-
// order fix: material is a raw caller token resolved via setOrderMaterial
// above (job_material_category keyword or exact DFHack material token,
// "" is a legal no-op); frequencyByte is a WORK_ORDER_FREQUENCY_* constant
// selecting order->frequency's recurring cadence.
bool applyWorkOrder(uint8_t orderType, uint16_t quantity, const std::string &jobTypeName,
                     const std::string &material, uint8_t frequencyByte, std::string &error)
{
    // ORDER_TYPE_BREW_DRINK is the one hand-maintained order type with no
    // direct df::job_type mapping at all (protocolToJobType above returns
    // -1 for it) -- vanilla DF itself queues brewing as a
    // df::job_type::CustomReaction manager order with reaction_name=
    // "BREW_DRINK_FROM_PLANT", NOT a dedicated BrewDrink job_type (there
    // is none in this DFHack version's df.job.xml). Confirmed against
    // DFHack's own shipped sample manager-order file, data/orders/
    // basic.json:77-78 ({"job":"CustomReaction","reaction":
    // "BREW_DRINK_FROM_PLANT"}) — the exact job_type/reaction_name pairing
    // its own orders_import (plugins/orders.cpp:540-552) builds from that
    // same JSON shape. Handled as a special case here rather than a new
    // protocolToJobType entry, since this is the only ORDER_TYPE_* byte
    // that needs a reaction_name at all. No wire/protocol.h change is
    // needed for this — ORDER_TYPE_BREW_DRINK already exists and already
    // only reaches applyWorkOrder (ORDER_TYPE_CUSTOM_REACTION, protocol.h,
    // is the equivalent sentinel for queue_job and is explicitly NOT
    // accepted here).
    bool isReactionOrder = (orderType == ORDER_TYPE_BREW_DRINK);
    std::string reactionCode;
    int jobType;
    if (isReactionOrder) {
        reactionCode = "BREW_DRINK_FROM_PLANT";
        jobType = (int)df::job_type::CustomReaction;
    } else if (orderType == ORDER_TYPE_BY_NAME) {
        jobType = resolveJobTypeByName(jobTypeName, error);
        if (jobType < 0) {
            return false;
        }
    } else {
        jobType = protocolToJobType(orderType);
        if (jobType < 0) {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown work order type: 0x%02X", orderType);
            error = buf;
            return false;
        }
    }

    if (!df::global::world) {
        error = "world is null";
        return false;
    }

    // Build a manager_order entry. The simple-and-safe path is to
    // allocate, populate, and push onto world->manager_orders.all.
    df::manager_order* order = new df::manager_order();
    if (!order) {
        error = "manager_order allocation failed";
        return false;
    }

    // VERIFY: the field names below against df/manager_order.h. Common
    // shape: job_type, item_type, item_subtype, mat_type, mat_index,
    // amount_total, amount_left, status. Some versions split status into
    // a flags struct.
    // Assign a unique order id from DF's own counter (53.15:
    // world->manager_orders is a workquota_handlerst carrying the .all
    // vector and manager_order_next_id). Without this every order gets
    // id 0, which breaks order lookup/removal and the manager UI.
    order->id = df::global::world->manager_orders.manager_order_next_id++;
    order->job_type = (df::job_type) jobType;
    if (isReactionOrder) {
        order->reaction_name = reactionCode;
    }
    order->item_type = df::item_type::NONE;       // let manager pick item subtype
    order->item_subtype = -1;
    // mat_type: do NOT set this to -1. df.workquota.xml gives manager_order's
    // mat_type field no init-value (defaults to 0), while mat_index
    // explicitly defaults to -1 — the same 0/-1 pairing df::job and
    // df::job_item use elsewhere for "not yet assigned." MaterialInfo::decode
    // (Materials.cpp) treats any mat_type<0 as invalid ("no material"), not
    // "any material" — the real "unrestricted material" mechanism for a
    // manager_order is the separate material_category bitfield (left at its
    // zero default here). This line used to set mat_type=-1, which marks the
    // order's own material as invalid the moment DF's manager tries to
    // resolve it — a latent bug independent of the missing-manager-noble
    // gap that currently masks it. Leave mat_type at its struct default (0).
    order->mat_index = -1;
    order->amount_total = quantity;
    order->amount_left = quantity;
    // DFHack 53.12: manager_order::status is a bitfield union, not a flag.
    // Set the validated bit so the manager dispatches the order.
    //
    // DIVERGES from DFHack's own reference order library (data/orders/*.json
    // -- every shipped entry has both "is_validated" and "is_active" false)
    // -- flagged by this project's own manager-work-order research report as
    // a real, confirmed divergence from convention, but NOT proven causal to
    // any observed bug (PrepareMeal dispatches fine despite this same
    // preset). Left as-is rather than flipped: this line touches the one
    // currently-confirmed-working path (PrepareMeal) and the report
    // explicitly recommends an isolated live A/B test before changing it,
    // which this pass cannot perform (see repo's manager-work-order fix
    // wave notes). Revisit with a live test, not a speculative flip.
    order->status.bits.validated = 1;

    if (!setOrderMaterial(order, material, error)) {
        delete order;
        return false;
    }

    switch (frequencyByte) {
        case WORK_ORDER_FREQUENCY_DAILY:      order->frequency = df::workquota_frequency_type::Daily;      break;
        case WORK_ORDER_FREQUENCY_MONTHLY:    order->frequency = df::workquota_frequency_type::Monthly;    break;
        case WORK_ORDER_FREQUENCY_SEASONALLY: order->frequency = df::workquota_frequency_type::Seasonally; break;
        case WORK_ORDER_FREQUENCY_YEARLY:     order->frequency = df::workquota_frequency_type::Yearly;     break;
        case WORK_ORDER_FREQUENCY_ONE_TIME:
        default:                              order->frequency = df::workquota_frequency_type::OneTime;    break;
    }

    // VERIFY: the world->manager_orders field name. Some versions
    // (manager_orders), some (manager_order_count), some store a struct
    // with an .all vector. If the build complains, check world.h.
    df::global::world->manager_orders.all.push_back(order);

    return true;
}

// Which workshop type(s) can run each job_type. Vanilla DF: furniture jobs
// (bed/table/chair/door/cabinet/coffer) run at EITHER Carpenters (wood) or
// Masons (stone); barrels/buckets are Carpenters-only; blocks and crafts run
// at their dedicated workshop; BrewDrink has no job_type mapping (see
// protocolToJobType above) and is rejected before reaching this table.
// VERIFY: this table is DF domain knowledge, not sourced from DFHack code —
// cross-check against a live game the first time each job type is used.
//
// SCOPE BOUNDARY (generalized item construction, see resolveJobTypeByName
// above): job-TYPE resolution is fully generalized — any df::job_type
// DFHack knows about can be named and resolved with no C++ change, via
// ORDER_TYPE_BY_NAME. Workshop COMPATIBILITY (this table) and the
// WOOD/BOULDER material-class filter in applyQueueJob below are NOT
// generalized — DFHack exposes no per-job_type "which workshop(s)" or
// "which material class" metadata to derive them from (df.job.xml's
// job_type schema has no `workshop` attribute at all; researched and
// confirmed against the DFHack checkout). They remain hand-maintained DF
// domain knowledge, same as before this change.
//
// KNOWN LIMITATION: resolving a name via ORDER_TYPE_BY_NAME does NOT imply
// it will pass this whitelist — a resolvable job type still needs an entry
// below (and, if it needs a specific material class, a case in the
// WOOD/BOULDER filter further down) before applyQueueJob accepts it. This
// whitelist is applyQueueJob-only: applyWorkOrder's own ORDER_TYPE_BY_NAME
// path (added in a later pass, see that function above) never consults it
// at all — a manager_order needs no workshop/material filter on this side,
// so any job_type find_enum_item resolves is immediately queueable there,
// whitelisted here or not. Add entries below as each job type's real
// workshop/material requirement is confirmed for the queue_job path.
// (ConstructHatchCover and ConstructFloodgate were both grouped
// with the door/furniture set on later passes — same Carpenters/Masons
// pair, same per-workshop WOOD/BOULDER material class as ConstructDoor;
// ConstructFloodgate was live-verified missing from this switch entirely
// 2026-07-17 — ACK read "job type not supported at this workshop type" at
// Mechanics, Carpenters, AND Masons, since the default case rejects
// anything absent here regardless of which workshop is correct in DF.
// ConstructCoffin found missing the same way, same session, same fix
// shape: live-tested at both Carpenters and Masons post-build with the
// identical rejection text; confirmed in source before patching, not
// just live-guessed — same Carpenters/Masons pair as the other
// furniture-class construction jobs above.
//
// 2026-07-17 pass: added ConstructStatue/ConstructWeaponRack/
// ConstructArmorStand/ConstructGrate (both Masons and Carpenters lists in
// workshops.lua — ConstructStatue really is listed at both, lines 185+301,
// not a lua duplication mistake), ConstructSlab/ConstructQuern/
// ConstructMillstone (Masons list only, lines 187-211, no wood variant),
// and ConstructBin/ConstructSplint/ConstructCrutch/MakeAnimalTrap
// (Carpenters list only, lines 228-321). Also moved ConstructBed OUT of
// the Carpenters/Masons group into Carpenters-only: Masons' list has no
// bed entry at all (DF has no stone beds) — a bed queued at a Masons
// workshop got a BOULDER job_item it could never satisfy, a real, if
// minor, live bug. MakePipeSection is NOT in workshops.lua at all (that
// lua module never defines a Metalsmith's Forge job table, and its
// Carpenters section omits pipe sections too) — confirmed Carpenters/wood
// instead via plugins/lua/stockflow.lua:598-609's materials.wood reaction
// list, which groups it with MakeCage/MakeAnimalTrap/MakeBarrel/
// MakeBucket/ConstructBin, all independently confirmed Carpenters-only
// above; the metal variant at Metalsmith's Forge (stockflow.lua:526) is
// NOT wired up since it would need a BAR/metal job_item filter this pass
// doesn't add. MakeCage deliberately excluded (justice-adjacent, out of
// scope this pass) despite appearing in the same Carpenters list.
//
// 2026-07-18 pass: added MakeChain, MetalsmithsForge-only — needed for the
// Well building's chain reagent (buildings.cpp placeWell). Confirmed
// metal-only, NOT the wood/Carpenters shape MakePipeSection got above:
// plugins/lua/buildingplan/planneroverlay.lua's JOB_DEFAULTS lists
// MakeChain='iron' alongside ForgeAnvil='iron'/MakeFlask='iron' (all three
// distinct from the wood-defaulted ConstructBed/MakeBarrel/MakeBucket/
// MakeAnimalTrap/MakeCage group in that same table) — a DFHack-maintained
// table that exists specifically to pick a sensible default material per
// job_type, so its wood/iron split is authoritative. stockflow.lua's
// collect_reactions independently corroborates: its ONE building-relevant
// job_types.MakeChain emission (line 531, verb="Forge") sits inside the
// `if material.flags.IS_METAL then` branch (line 443) of the metal-forging
// section; the file's separate wood/rock/glass material_reactions blocks
// (materials.wood/materials.rock/glasses, lines ~575-679) never mention
// MakeChain at all — unlike MakeCage/MakeBarrel/MakeBucket, which appear in
// both the metal AND the wood tables. (A second, unrelated MakeChain
// emission at line 726 reuses the same item/job_type to name a cloth/silk/
// yarn "Rope" — cosmetic renaming of the identical item_type::CHAIN for a
// different display string, not a second building context, and is not
// wired up here.)
//
// 2026-07-18 Tier-2 pass: added CutGems/EncrustWithGems (Jewelers),
// PrepareRawFish/ExtractFromRawFish (Fishery), MakeTotem (Craftsdwarfs —
// MakeCrafts was already here), WeaveCloth/CollectWebs (Loom),
// DyeThread/DyeCloth (Dyers), MakeBackpack/MakeQuiver (Leatherworks), and
// ForgeAnvil (MetalsmithsForge, joining MakeChain). Each of these needs a
// job_item shape the generic WOOD/BOULDER/BAR fallthrough in applyQueueJob
// cannot produce, EXCEPT ForgeAnvil, which reuses MakeChain's existing BAR
// filter unmodified (both are single-reagent "any metal bar" jobs per
// plugins/lua/buildingplan/planneroverlay.lua's JOB_DEFAULTS, which lists
// ForgeAnvil='iron' right alongside MakeChain='iron') — see applyQueueJob's
// new per-job_type switch for the other ten's dedicated job_item vectors,
// each cited against its own workshops.lua/idle-crafting.lua source at its
// case label. Confirmed per-workshop against library/lua/dfhack/
// workshops.lua's jobs_workshop[Jewelers]/[Fishery]/[Loom]/[Dyers]/
// [Leatherworks] tables (lines 90-111, 112-128, 373-399, 428-441, 400-421
// respectively) and, for MakeTotem/rock MakeCrafts, scripts/idle-
// crafting.lua's makeTotem/makeRockCraft. ButcherAnimal (Butchers) was
// investigated and deliberately NOT added: its one workshops.lua job_item
// (flags1={butcherable,unrotten,nearby}) targets a live nearby creature,
// not a stockpile-sourced item — vanilla DF auto-generates this job the
// moment a unit is flagged for slaughter (a mechanism this project has no
// tool for yet), not via a manually queued workshop job the way every other
// entry in this switch works; adding it here would let queue_job accept an
// order that can never be satisfied through this command's own dispatch
// path, the same truthful-ACK problem MakeCrafts/PrepareMeal's reject block
// already guards against below. TanHide (Tanners) was also investigated and
// skipped: unlike every other job_type in this switch, it has ZERO
// precedent anywhere in this DFHack checkout (grepped workshops.lua,
// stockflow.lua, and the whole tree for "TanHide" — no hits at all), so its
// job_item shape cannot be confirmed against source; left as an open
// question/follow-up rather than guessed. Leatherworks' "construct leather
// bag" (ConstructChest, workshops.lua line 405) was also skipped — wiring
// it would mean branching ConstructChest's existing Carpenters/Masons
// WOOD/BOULDER filter on workshop type to add a THIRD (SKIN_TANNED) shape
// to an already-shared job_type, more risk than MakeBackpack/MakeQuiver
// below, which are exclusively Leatherworks and need no such branching.
static bool jobTypeAllowedAtWorkshop(df::job_type jobType, df::workshop_type wsType) {
    switch (jobType) {
        case df::job_type::ConstructTable:
        case df::job_type::ConstructThrone:
        case df::job_type::ConstructDoor:
        case df::job_type::ConstructHatchCover:
        case df::job_type::ConstructFloodgate:
        case df::job_type::ConstructCoffin:
        case df::job_type::ConstructCabinet:
        case df::job_type::ConstructChest:
        case df::job_type::ConstructStatue:
        case df::job_type::ConstructWeaponRack:
        case df::job_type::ConstructArmorStand:
        case df::job_type::ConstructGrate:
            return wsType == df::workshop_type::Carpenters || wsType == df::workshop_type::Masons;
        case df::job_type::ConstructSlab:
        case df::job_type::ConstructQuern:
        case df::job_type::ConstructMillstone:
            return wsType == df::workshop_type::Masons;
        case df::job_type::MakeBarrel:
        case df::job_type::MakeBucket:
        case df::job_type::MakeAnimalTrap:
        case df::job_type::ConstructBed:
        case df::job_type::ConstructBin:
        case df::job_type::ConstructSplint:
        case df::job_type::ConstructCrutch:
        case df::job_type::MakePipeSection:
            return wsType == df::workshop_type::Carpenters;
        case df::job_type::ConstructBlocks:
            return wsType == df::workshop_type::Masons || wsType == df::workshop_type::Carpenters;
        case df::job_type::MakeCrafts:
        case df::job_type::MakeTotem:
            return wsType == df::workshop_type::Craftsdwarfs;
        case df::job_type::PrepareMeal:
            return wsType == df::workshop_type::Kitchen;
        case df::job_type::ConstructMechanisms:
            // Confirmed via library/lua/dfhack/workshops.lua:360-366 and
            // stockflow.lua:692 (reaction_entry under materials.rock.
            // management) -- Mechanic's workshop only, 1x BOULDER in,
            // TRAPPARTS ("mechanism") out.
            return wsType == df::workshop_type::Mechanics;
        case df::job_type::MakeChain:
        case df::job_type::ForgeAnvil:
            // Metal-only -- see the 2026-07-18 doc notes above this switch
            // for the confirming sources. Needs its own BAR/metal job_item
            // shape below (not the WOOD/BOULDER fallthrough every other
            // case here uses) -- both share the exact same shape, so
            // ForgeAnvil needed no new applyQueueJob code, just this entry.
            return wsType == df::workshop_type::MetalsmithsForge;
        case df::job_type::CutGems:
        case df::job_type::EncrustWithGems:
            return wsType == df::workshop_type::Jewelers;
        case df::job_type::PrepareRawFish:
        case df::job_type::ExtractFromRawFish:
            return wsType == df::workshop_type::Fishery;
        case df::job_type::WeaveCloth:
        case df::job_type::CollectWebs:
            return wsType == df::workshop_type::Loom;
        case df::job_type::DyeThread:
        case df::job_type::DyeCloth:
            return wsType == df::workshop_type::Dyers;
        case df::job_type::MakeBackpack:
        case df::job_type::MakeQuiver:
            return wsType == df::workshop_type::Leatherworks;
        case df::job_type::ConstructTractionBench:
            // Mechanics-only -- workshops.lua:360-371 jobs_workshop[Mechanics]
            // ("construct traction bench", job_type=ConstructTractionBench),
            // the ONLY workshop table listing this job_type anywhere in the
            // checkout. Needs its own 3-job_item (TABLE+mechanism+CHAIN)
            // shape below (not the single-filter WOOD/BOULDER/BAR fallthrough
            // every other case here uses) -- see applyQueueJob's special
            // case for it, right before the generic single-filter block.
            return wsType == df::workshop_type::Mechanics;
        default:
            return false;
    }
    // NOTE: PrepareMeal passing this workshop check does NOT mean
    // applyQueueJob will accept it — the Carpenters/Masons WOOD/BOULDER
    // job_item filter further down is provably wrong for it (see the reject
    // block in applyQueueJob, right after this function's call site) and
    // there is no verified replacement filter yet. The workshop mapping
    // above is correct DF domain knowledge on its own; only the downstream
    // material filter is the problem. (MakeCrafts used to share this same
    // caveat; the 2026-07-18 Tier-2 pass gave it a verified rock-only
    // job_item in applyQueueJob's per-job_type switch, so it no longer
    // applies there.)
}

// Furnace counterpart to jobTypeAllowedAtWorkshop above, added 2026-07-17
// so applyQueueJob's direct job_type path can reach furnace-hosted jobs at
// all (previously its strict_virtual_cast only ever matched
// building_workshopst, so ANY furnace job_type -- including these two --
// was rejected before even reaching this whitelist).
//
// Deliberately NOT exhaustive. Only MakeCharcoal/MakeAsh are listed here,
// because they are the only furnace job_types confirmed (workshops.lua:
// 67-78, jobs_furnace[df.furnace_type.WoodFurnace] -- defaults=
// {item_type=WOOD,...}) to use the exact same plain item_type=WOOD
// job_item shape this file already builds for Carpenters below -- i.e.
// the ones that genuinely "fall out of" the existing WOOD/BOULDER filter
// with no new logic.
//
// SmeltOre and MeltMetalObject are real Smelter job_types (workshops.lua:
// 519-532 addSmeltJobs, jobs_furnace[df.furnace_type.Smelter]) that used to
// be deliberately left out of this whitelist entirely -- neither fits the
// plain-item_type-filter shape every other case here uses:
//   - SmeltOre needs job->mat_type/mat_index pinned to ONE specific ore
//     raw index (workshops.lua's job_fields sets these at the JOB level,
//     not just the job_item). 2026-07-19 manager-work-order fix wave: NOW
//     WIRED -- queue_job's new Material param (an exact ore token, e.g.
//     "INORGANIC:LIMONITE", resolved via MaterialInfo::find) supplies
//     that pin. See applyQueueJob's dedicated SmeltOre case (own job/
//     job_item construction, same shape as ConstructTractionBench below)
//     for the full job_item vector this unlocks.
//   - MeltMetalObject needs a flags2.bits.allow_melt_dump job_item
//     (matching items already player-flagged for melting) plus a
//     separate fuel job_item -- a fundamentally different shape from
//     SmeltOre's BOULDER-by-material_type filter, not a material-class
//     variant of it, and no wire parameter exists to identify a specific
//     already-flagged item. Still NOT wired -- left as documented future
//     work (same carve-out shape as the MakeCrafts/PrepareMeal reject
//     block in applyQueueJob below), to avoid a truthful-ACK violation
//     (queuing a job that can never be satisfied while still reporting
//     SUCCESS). Raw-defined smelter reactions (pig iron, steel, and the
// rest of the alloy/smelting reaction set) are unaffected by either of the
// above -- they go through applyQueueReactionJob (CustomReaction), which a
// prior pass extended to reach furnaces via the reaction's own
// building.type/subtype arrays, with no whitelist table needed there at
// all.
static bool jobTypeAllowedAtFurnace(df::job_type jobType, df::furnace_type fType) {
    switch (jobType) {
        case df::job_type::MakeCharcoal:
        case df::job_type::MakeAsh:
            return fType == df::furnace_type::WoodFurnace;
        case df::job_type::SmeltOre:
            // Both furnace_types run SmeltOre (workshops.lua's getJobs
            // dispatches addSmeltJobs for either Smelter or MagmaSmelter);
            // the only difference is the fuel job_item, added conditionally
            // in applyQueueJob's SmeltOre case (a MagmaSmelter needs no coal).
            return fType == df::furnace_type::Smelter || fType == df::furnace_type::MagmaSmelter;
        default:
            return false;
    }
}

// Job::assignToWorkshop (modules/Job.cpp:529-541 in the DFHack 53.15-r1
// checkout) is hardcoded to take df::building_workshopst* even though its
// entire body only touches fields that live on the common df::building
// base (centerx, centery, z, id, jobs) -- there is no DFHack-provided
// overload for df::building_furnacest, and reinterpret_cast-ing a
// building_furnacest* through a building_workshopst*-typed call would be
// unsafe (the two are unrelated sibling types below their shared
// building_actual/building base; their own field layouts differ). This
// local helper duplicates that same four-line body against a generic
// df::building* instead, so both workshop and furnace targets can share
// one call site without an unsafe cast. Behavior is unchanged for the
// existing workshop path -- this is the identical logic Job::
// assignToWorkshop already ran, just against the base-class fields
// directly.
static bool assignJobToBuilding(df::job *job, df::building *bld)
{
    if (bld->jobs.size() >= 10) {
        return false;
    }
    job->pos = df::coord(bld->centerx, bld->centery, bld->z);
    Job::addGeneralRef(job, df::general_ref_type::BUILDING_HOLDER, bld->id);
    bld->jobs.push_back(job);
    return true;
}

// isReactionPermittedForCiv reports whether reactionCode is in this fort's
// controlling civ's permitted-reaction allow-list, plus any game-generated
// reactions attached to the civ itself (reaction.source_enid == civ id).
//
// Reads entity_raw.workshops.permitted_reaction_id — the RESOLVED index
// vector (ref-target='reaction', df.entity.xml:705) — never
// permitted_reaction_str. The _str vector is raw-load staging that is
// empty in a live game (checking it made every reaction look unavailable,
// live-confirmed 2026-07-15), just as an earlier revision's
// df::reaction_flags::FORTRESS_MODE_ENABLED bitfield was dead. DFHack's
// own consumer (plugins/lua/stockflow.lua:418-428) iterates
// permitted_reaction_id and the source_enid loop exactly as done here.
//
// Not static: shared with handleListReactions in queries.cpp so the
// discovery tool (list_reactions) and the execution gate (queue_job's
// reaction path) always agree on what's runnable.
bool isReactionPermittedForCiv(const std::string &reactionCode)
{
    if (!df::global::world || !df::global::plotinfo) return false;
    df::historical_entity *civ = df::historical_entity::find(df::global::plotinfo->civ_id);
    if (!civ) return false;
    auto &reactions = df::global::world->raws.reactions.reactions;
    if (df::entity_raw *er = civ->entity_raw) {
        for (int32_t id : er->workshops.permitted_reaction_id) {
            if (id >= 0 && (size_t)id < reactions.size() && reactions[id]
                && reactions[id]->code == reactionCode)
                return true;
        }
    }
    for (auto *r : reactions) {
        if (r && r->source_enid == civ->id && r->code == reactionCode)
            return true;
    }
    return false;
}

// applyQueueReactionJob queues a job for a raw-defined df::reaction
// directly at the workshop occupying (x,y,z), keyed by the reaction's CODE
// (e.g. "BREW_DRINK_FROM_PLANT" — df::reaction.code, NOT the display name)
// rather than a df::job_type. This is the path ORDER_TYPE_CUSTOM_REACTION
// routes to from applyQueueJob below — the only way to reach reactions
// like brewing that have no job_type mapping at all
// (protocolToJobType(ORDER_TYPE_BREW_DRINK) returns -1 above).
//
// Recipe confirmed against DFHack's own library/lua/dfhack/workshops.lua
// (addReactionJobs/reagentToJobItem, scanRawsReaction/matchIds): no by-code
// finder exists for df::reaction (only find(int id), by raw index), so
// this does the same linear scan scanRawsReaction does in Lua, then builds
// one df::job_item per reaction_reagent using the same field-overlay
// convention that function uses.
//
// SCOPE BOUNDARY: workshop/reaction compatibility is fully derived from
// the reaction's own building.type/subtype/custom arrays (df/reaction.h)
// — unlike jobTypeAllowedAtWorkshop above, no hand-maintained workshop
// table is needed for this path. Only reaction_reagent_itemst (getType()
// == item) reagents are supported; this DFHack version's
// reaction_reagent_type enum has no other concrete reagent subclass to
// handle (confirmed against df/reaction_reagent_type.h — "item" is the
// only non-NONE value), so failing loudly on anything else costs nothing
// today but keeps the door open if a future DFHack version adds one.
//
// 2026-07-17: also accepts df::building_furnacest targets (was
// building_workshopst only, which silently rejected every furnace
// reaction -- e.g. pig iron, steel, and the rest of the alloy/smelting
// reaction set -- with "not a workshop", even though those reactions
// already declare building.type=Furnace in their raws and DFHack's own
// getJobs dispatcher (library/lua/dfhack/workshops.lua:533-547) treats
// building_type::Workshop and building_type::Furnace as parallel, equally
// valid reaction hosts). No new compatibility table needed for this --
// the matching loop below already reads whatever building.type/subtype
// the reaction raws specify; it just needed to stop assuming the found
// building was always a workshop.
static bool applyQueueReactionJob(int16_t x, int16_t y, int16_t z, const std::string &reactionCode, std::string &error)
{
    if (!df::global::world) {
        error = "world is null";
        return false;
    }
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::coord pos(x, y, z);
    df::building *bld = Buildings::findAtTile(pos);
    if (!bld) {
        std::ostringstream os;
        os << "no building at (" << x << "," << y << "," << z << ")";
        error = os.str();
        return false;
    }
    df::building_workshopst *ws = strict_virtual_cast<df::building_workshopst>(bld);
    df::building_furnacest *furn = ws ? nullptr : strict_virtual_cast<df::building_furnacest>(bld);
    if (!ws && !furn) {
        error = "building at that location is not a workshop or furnace";
        return false;
    }
    // The building_type/subtype pair this specific building actually is --
    // compared against each alternative the reaction's raws list below.
    int32_t actualBuildingType = ws ? (int32_t)df::building_type::Workshop : (int32_t)df::building_type::Furnace;
    int32_t actualSubtype = ws ? (int32_t)ws->type : (int32_t)furn->type;

    // No by-code finder exists on df::reaction -- linear scan over the raws
    // vector, same lookup DFHack's own scanRawsReaction does in Lua.
    df::reaction *reaction = nullptr;
    for (auto *r : df::global::world->raws.reactions.reactions) {
        if (r && r->code == reactionCode) {
            reaction = r;
            break;
        }
    }
    if (!reaction) {
        error = "unrecognized reaction code: '" + reactionCode +
                "' (use the list_reactions query/tool to look up valid codes)";
        return false;
    }

    if (!isReactionPermittedForCiv(reactionCode)) {
        error = "reaction '" + reactionCode + "' is not enabled in fortress mode";
        return false;
    }

    // Workshop/furnace compatibility: reaction->building.type/subtype/custom
    // are PARALLEL ARRAYS of alternative buildings the reaction can run at
    // (df/reaction.h T_building) -- a value of -1 in the type or subtype
    // slot means "any" (DF's own wildcard convention for these enums, see
    // df/building_type.h / df/workshop_type.h / df/furnace_type.h NONE=-1).
    // Mirrors DFHack's own matchIds/scanRawsReaction (library/lua/dfhack/
    // workshops.lua) -- custom is not checked here, same scope as that
    // scout note. Compares against actualBuildingType/actualSubtype (this
    // building's own type/subtype, workshop or furnace) rather than
    // hardcoding Workshop, so a furnace-hosted reaction alternative matches
    // exactly the same way a workshop-hosted one already did.
    bool compatible = false;
    std::ostringstream wants;
    size_t nAlt = reaction->building.type.size();
    for (size_t k = 0; k < nAlt; k++) {
        int32_t altBuildingType = (int32_t)reaction->building.type[k];
        int32_t altSubtype = (k < reaction->building.subtype.size()) ? reaction->building.subtype[k] : -1;
        if (k > 0) wants << ", ";
        if (altBuildingType == (int32_t)df::building_type::Workshop) {
            wants << (altSubtype == -1 ? "any workshop" : ENUM_KEY_STR(workshop_type, (df::workshop_type)altSubtype));
        } else if (altBuildingType == (int32_t)df::building_type::Furnace) {
            wants << (altSubtype == -1 ? "any furnace" : ENUM_KEY_STR(furnace_type, (df::furnace_type)altSubtype));
        } else {
            wants << ENUM_KEY_STR(building_type, (df::building_type)altBuildingType);
        }
        if (altBuildingType != -1 && altBuildingType != actualBuildingType) continue;
        if (altSubtype != -1 && altSubtype != actualSubtype) continue;
        compatible = true;
    }
    if (!compatible) {
        std::string atName = ws ? ENUM_KEY_STR(workshop_type, ws->type) : ENUM_KEY_STR(furnace_type, furn->type);
        error = "reaction '" + reactionCode + "' does not run at this workshop ("
                + atName + ") -- it wants: " + wants.str();
        return false;
    }

    if (bld->jobs.size() >= 10) {
        error = "workshop/furnace job queue is full (10 jobs)";
        return false;
    }

    // Build one job_item per reagent BEFORE allocating the df::job itself,
    // so a mid-loop failure only needs to clean up loose job_items, never a
    // half-built (not yet linked into world) job.
    std::vector<df::job_item*> jobItems;
    for (size_t reagentIdx = 0; reagentIdx < reaction->reagents.size(); reagentIdx++) {
        df::reaction_reagent *reagent = reaction->reagents[reagentIdx];
        if (!reagent) continue;
        if (reagent->getType() != df::reaction_reagent_type::item) {
            std::ostringstream os;
            os << "reaction '" << reactionCode << "' reagent '" << reagent->code
               << "' is not an item reagent -- not yet supported by queue_job";
            error = os.str();
            for (auto *ji : jobItems) delete ji;
            return false;
        }
        df::reaction_reagent_itemst *ri = (df::reaction_reagent_itemst*)reagent;

        df::job_item *ji = new df::job_item();
        // Defaults per DFHack's own dfhack/workshops.lua
        // input_filter_defaults (library/lua/dfhack/workshops.lua:5-25),
        // then overlaid with the reagent's own matching fields below --
        // reaction_reagent_itemst shares these field names 1:1 with
        // job_item.
        ji->item_type = df::item_type::NONE;
        ji->item_subtype = -1;
        ji->mat_type = -1;
        ji->mat_index = -1;
        ji->flags2.bits.allow_artifact = true;
        ji->reaction_class = "";
        ji->has_material_reaction_product = "";
        ji->metal_ore = -1;
        ji->min_dimension = -1;
        ji->has_tool_use = df::tool_uses::NONE;
        ji->quantity = 1;

        ji->item_type = ri->item_type;
        ji->item_subtype = ri->item_subtype;
        ji->mat_type = ri->mat_type;
        ji->mat_index = ri->mat_index;
        ji->reaction_class = ri->reaction_class;
        ji->has_material_reaction_product = ri->has_material_reaction_product;
        ji->metal_ore = ri->metal_ore;
        ji->min_dimension = ri->min_dimension;
        ji->has_tool_use = ri->has_tool_use;
        ji->quantity = ri->quantity;
        // OR the reagent's own filter flags onto the job_item's -- these are
        // additive match requirements (e.g. BREW_DRINK_FROM_PLANT's plant
        // reagent sets flags1.unrotten, its barrel reagent sets
        // flags1.empty + flags3.food_storage). OR-ing rather than assigning
        // preserves the allow_artifact=true default set above, since that
        // bit stays 1 regardless of what bits ri->flags2 contributes.
        ji->flags1.whole |= ri->flags1.whole;
        ji->flags2.whole |= ri->flags2.whole;
        ji->flags3.whole |= ri->flags3.whole;
        ji->flags4 |= ri->flags4;
        ji->flags5 |= ri->flags5;

        ji->reaction_id = reaction->index;
        ji->reagent_index = (int32_t)reagentIdx;

        jobItems.push_back(ji);
    }

    if (reaction->flags.is_set(df::reaction_flags::FUEL)) {
        df::job_item *fuelItem = new df::job_item();
        fuelItem->item_type = df::item_type::BAR;
        fuelItem->mat_type = df::builtin_mats::COAL;
        jobItems.push_back(fuelItem);
    }

    df::job *job = new df::job();
    job->job_type = df::job_type::CustomReaction;
    job->reaction_name = reaction->code;
    for (auto *ji : jobItems) {
        job->job_items.elements.push_back(ji);
    }

    Job::linkIntoWorld(job, true);
    if (!assignJobToBuilding(job, bld)) {
        Job::removeJob(job);
        error = "failed to attach job to workshop/furnace (queue full or invalid building)";
        return false;
    }

    return true;
}

// newFilterJobItem returns a freshly allocated df::job_item pre-populated
// with the same "no restriction" baseline library/lua/dfhack/workshops.lua's
// input_filter_defaults table (that file, lines 5-25) stamps onto every
// reagent it builds: item_type/item_subtype/mat_type/mat_index unrestricted
// (NONE/-1/-1/-1), quantity 1, artifacts allowed, no reaction-class/
// material-reaction-product string requirement. This mirrors exactly what
// applyQueueReactionJob's per-reagent loop above already does inline
// (lines ~600-610) for CustomReaction job_items built from raws data;
// factored out here as a helper because applyQueueJob's new 2026-07-18
// Tier-2 per-job_type switch (CutGems/EncrustWithGems/PrepareRawFish/
// ExtractFromRawFish/MakeTotem/rock MakeCrafts/WeaveCloth/CollectWebs/
// DyeThread/DyeCloth/MakeBackpack/MakeQuiver) builds twelve more job_items
// this same way and repeating that ten-line baseline by hand each time
// would be pure duplication. Explicitly zeroing mat_type/mat_index here is
// NOT optional: df.job.xml gives job_item's mat_type field no init-value at
// all, so a bare `new df::job_item()` zero-initializes it to 0 (a specific
// stone/material index), not "any material" — the same 0-vs-(-1) landmine
// already documented on manager_order::mat_type in applyWorkOrder above.
static df::job_item *newFilterJobItem()
{
    df::job_item *ji = new df::job_item();
    ji->item_type = df::item_type::NONE;
    ji->item_subtype = -1;
    ji->mat_type = -1;
    ji->mat_index = -1;
    ji->flags2.bits.allow_artifact = true;
    ji->reaction_class = "";
    ji->has_material_reaction_product = "";
    ji->metal_ore = -1;
    ji->min_dimension = -1;
    ji->has_tool_use = df::tool_uses::NONE;
    ji->quantity = 1;
    return ji;
}

// applyQueueJob queues a job DIRECTLY at an existing workshop — the same
// underlying mechanism DF uses when a player right-clicks a workshop and
// picks a task from its build menu. Unlike applyWorkOrder above (which adds
// a df::manager_order that sits inert until a Manager noble with an office
// scans the queue), this needs no noble or office: Job::linkIntoWorld +
// Job::assignToWorkshop (library/modules/Job.cpp:493-541 in the DFHack
// 53.15-r1 checkout) attach the job straight to the workshop's own job list.
// Keep both paths: this one is for an immediate, one-off need; applyWorkOrder
// is for standing/bulk production once a manager exists (009's planned
// homeostasis executors will want the manager path).
//
// jobTypeName is the ORDER_TYPE_BY_NAME payload's trailing name (protocol.h)
// — empty and ignored unless orderType == ORDER_TYPE_BY_NAME, in which case
// it takes priority over orderType and is resolved via
// resolveJobTypeByName instead of the protocolToJobType switch. This is
// the one generalized half of this function; jobTypeAllowedAtWorkshop and
// the material filter further down are not (see the SCOPE BOUNDARY comment
// above jobTypeAllowedAtWorkshop).
//
// material is a raw caller token, currently consumed by exactly one
// job_type: SmeltOre (an exact ore raw, e.g. "INORGANIC:LIMONITE",
// resolved via MaterialInfo::find -- see this function's dedicated SmeltOre
// case below). Ignored for every other job_type.
bool applyQueueJob(int16_t x, int16_t y, int16_t z, uint8_t orderType, const std::string &jobTypeName,
                    const std::string &material, std::string &error)
{
    // ORDER_TYPE_CUSTOM_REACTION bypasses job-type resolution entirely --
    // it's keyed by reaction code, not df::job_type -- and its
    // workshop-compatibility check is derived from the reaction's own raws
    // data (see applyQueueReactionJob above), so jobTypeAllowedAtWorkshop
    // and the WOOD/BOULDER material filter further down do not apply to
    // this path. jobTypeName doubles as the trailing wire field for both
    // sentinels (df_ai_protocol.cpp decodes one trailing string regardless
    // of which sentinel triggered it); here it holds the reaction code.
    if (orderType == ORDER_TYPE_CUSTOM_REACTION) {
        return applyQueueReactionJob(x, y, z, jobTypeName, error);
    }

    int jobTypeInt;
    if (orderType == ORDER_TYPE_BY_NAME) {
        jobTypeInt = resolveJobTypeByName(jobTypeName, error);
    } else {
        jobTypeInt = protocolToJobType(orderType);
        if (jobTypeInt < 0) {
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown work order type: 0x%02X", orderType);
            error = buf;
        }
    }
    if (jobTypeInt < 0) {
        return false;
    }
    df::job_type jobType = (df::job_type)jobTypeInt;

    if (!df::global::world) {
        error = "world is null";
        return false;
    }
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::coord pos(x, y, z);
    df::building *bld = Buildings::findAtTile(pos);
    if (!bld) {
        std::ostringstream os;
        os << "no building at (" << x << "," << y << "," << z << ")";
        error = os.str();
        return false;
    }

    // 2026-07-17: also accepts df::building_furnacest targets -- this cast
    // used to be workshop-only, which silently rejected every furnace
    // job_type (MakeCharcoal/MakeAsh at WoodFurnace, SmeltOre/
    // MeltMetalObject at Smelter) even though this plugin already builds
    // smelter/wood_furnace furnaces. See jobTypeAllowedAtFurnace above for
    // exactly which furnace job_types this unlocks -- SmeltOre was wired in
    // the 2026-07-19 manager-work-order fix wave (see this function's
    // dedicated SmeltOre case below); MeltMetalObject remains deliberately
    // excluded (documented future work, same doc comment).
    df::building_workshopst *ws = strict_virtual_cast<df::building_workshopst>(bld);
    df::building_furnacest *furn = ws ? nullptr : strict_virtual_cast<df::building_furnacest>(bld);
    if (!ws && !furn) {
        error = "building at that location is not a workshop or furnace";
        return false;
    }

    if (ws) {
        if (!jobTypeAllowedAtWorkshop(jobType, ws->type)) {
            std::ostringstream os;
            os << "job type not supported at this workshop type ("
               << ENUM_KEY_STR(workshop_type, ws->type) << ")";
            error = os.str();
            return false;
        }
    } else {
        if (!jobTypeAllowedAtFurnace(jobType, furn->type)) {
            std::ostringstream os;
            os << "job type not supported at this furnace type ("
               << ENUM_KEY_STR(furnace_type, furn->type) << ")";
            error = os.str();
            return false;
        }
    }

    // PrepareMeal is workshop-compatible (jobTypeAllowedAtWorkshop above is
    // correct for it) but is rejected here, BEFORE the Carpenters/Masons
    // WOOD/BOULDER job_item filter further down, because that filter is
    // provably wrong for it and this command has no verified replacement.
    // PrepareMeal has no job_item precedent at all in DFHack's own code; its
    // only sourced reference (scripts/internal/quickfort/stockflow.lua,
    // plugins/lua/stockflow.lua reaction_entry(job_types.PrepareMeal,
    // {mat_type=2/3/4})) queues it as a manager_order (applyWorkOrder's
    // path, not this one) with no job_item at all. Stamping item_type::WOOD
    // onto it — what the fallthrough below does for every job_type not
    // special-cased above it — would queue a job that can never be worked
    // (no wood-log reagent PrepareMeal should ever require) while still
    // ACKing SUCCESS, a truthful-ACK violation. Fail loudly instead. Use
    // `order` (applyWorkOrder) for PrepareMeal until a verified job_item
    // filter is added here — future work, not solved by this pass.
    //
    // MakeCrafts used to be rejected here too (same reasoning: DFHack's own
    // scripts/idle-crafting.lua shows it needs item_type::NONE + a
    // material_category flag (bone/horn/shell) OR item_type::BOULDER +
    // vector_id::BOULDER, chosen per material, not the WOOD/BOULDER
    // fallthrough). The 2026-07-18 Tier-2 pass added a verified rock-only
    // job_item (idle-crafting.lua's makeRockCraft) in the switch below, so
    // MakeCrafts now falls through to that special case instead of being
    // rejected here; the bone/horn/shell variants remain unwired (no
    // parameter exists yet to choose between them) — same "not solved by
    // this pass" status PrepareMeal still has.
    if (jobType == df::job_type::PrepareMeal) {
        std::ostringstream os;
        os << "queue_job does not yet support " << ENUM_KEY_STR(job_type, jobType)
           << " — no verified job_item material filter for it (the WOOD/BOULDER"
              " filter below is furniture-job domain knowledge and wrong here);"
              " use order (manager work order) for this item type instead";
        error = os.str();
        return false;
    }

    if (bld->jobs.size() >= 10) {
        error = "workshop/furnace job queue is full (10 jobs)";
        return false;
    }

    // 2026-07-18 Tier-2 pass: job_types that need a hand-built job_item
    // vector (or vectors) rather than the single generic WOOD/BOULDER/BAR
    // filter the fallthrough further below builds. Each case is confirmed
    // against library/lua/dfhack/workshops.lua's per-workshop job tables
    // (jobs_workshop[Jewelers]/[Fishery]/[Loom]/[Dyers]/[Leatherworks]) or,
    // for MakeTotem and MakeCrafts' rock variant, scripts/idle-crafting.lua
    // (makeTotem/makeRockCraft) — see the doc comment above
    // jobTypeAllowedAtWorkshop for the full per-job_type source citations
    // and the follow-ups (ButcherAnimal, TanHide, leather ConstructChest)
    // deliberately left unwired. newFilterJobItem() (below) pre-populates
    // each job_item with the same "no restriction" baseline library/lua/
    // dfhack/workshops.lua's input_filter_defaults uses (item_type=NONE,
    // mat_type/mat_index=-1, quantity=1, artifacts allowed) — required, not
    // optional: df.job.xml gives job_item's mat_type field no init-value,
    // so a bare `new df::job_item()` zero-initializes it to 0 (a specific
    // stone/material index), not "any material" — the same 0-vs-(-1)
    // landmine already documented on manager_order::mat_type in
    // applyWorkOrder above. Handled as one switch building an `items`
    // vector (and, for MakeTotem/MakeCrafts, a job-level mat_type) rather
    // than N separate copy-pasted special-case blocks, then a single
    // shared link/assign at the end -- this file's existing per-job special
    // cases (ConstructTractionBench below, applyQueueReactionJob above)
    // each inline their own link/assign instead, but twelve more copies of
    // that boilerplate here would be pure repetition with no offsetting
    // clarity benefit.
    {
        std::vector<df::job_item*> tier2Items;
        // tier2SetJobMatType/tier2JobMatType: most job_types below don't
        // touch job->mat_type at all (leave df::job's own zero-default
        // alone, same as every other job_type this file already queues).
        // MakeTotem is the one case that explicitly needs -1 (not 0) --
        // using -1 alone as a "don't set it" sentinel would silently skip
        // MakeTotem's own intended -1 assignment, so this uses an explicit
        // bool instead of overloading a sentinel value.
        bool tier2SetJobMatType = false;
        int tier2JobMatType = 0;
        bool tier2Handled = true;
        switch (jobType) {
            case df::job_type::CutGems: {
                // workshops.lua:90-95 "cut gems": items={{item_type=ROUGH,
                // flags1={unrotten=true}}}.
                df::job_item *gem = newFilterJobItem();
                gem->item_type = df::item_type::ROUGH;
                gem->flags1.bits.unrotten = true;
                tier2Items.push_back(gem);
                break;
            }
            case df::job_type::EncrustWithGems: {
                // workshops.lua:96-110 lists three target-class variants
                // (finished_goods/ammo/furniture) sharing one SMALLGEM
                // reagent; only the furniture variant is wired here (no
                // command parameter exists to pick between them) —
                // finished_goods/ammo are a follow-up, not solved by this
                // pass.
                df::job_item *gem = newFilterJobItem();
                gem->item_type = df::item_type::SMALLGEM;
                tier2Items.push_back(gem);
                df::job_item *target = newFilterJobItem();
                target->flags1.bits.improvable = true;
                target->flags1.bits.furniture = true;
                tier2Items.push_back(target);
                break;
            }
            case df::job_type::PrepareRawFish: {
                // workshops.lua:112-117: items={{item_type=FISH_RAW,
                // flags1={unrotten=true}}}.
                df::job_item *fish = newFilterJobItem();
                fish->item_type = df::item_type::FISH_RAW;
                fish->flags1.bits.unrotten = true;
                tier2Items.push_back(fish);
                break;
            }
            case df::job_type::ExtractFromRawFish: {
                // workshops.lua:118-122: reagent 1 has no item_type override
                // (matches any unrotten extract-bearing fish already in
                // hand/nearby), reagent 2 is an empty glass flask to fill.
                df::job_item *fish = newFilterJobItem();
                fish->flags1.bits.unrotten = true;
                fish->flags1.bits.extract_bearing_fish = true;
                tier2Items.push_back(fish);
                df::job_item *flask = newFilterJobItem();
                flask->item_type = df::item_type::FLASK;
                flask->flags1.bits.empty = true;
                flask->flags1.bits.glass = true;
                tier2Items.push_back(flask);
                break;
            }
            case df::job_type::MakeTotem: {
                // scripts/idle-crafting.lua makeTotem: job.mat_type = -1;
                // one job_item, item_type left at NONE, vector_id
                // ANY_REFUSE, flags1.unrotten + flags2.totemable +
                // flags2.body_part (a corpse/corpse-piece body part, not a
                // stockpiled manufactured item).
                tier2SetJobMatType = true;
                tier2JobMatType = -1;
                df::job_item *raw = newFilterJobItem();
                raw->vector_id = df::job_item_vector_id::ANY_REFUSE;
                raw->flags1.bits.unrotten = true;
                raw->flags2.bits.totemable = true;
                raw->flags2.bits.body_part = true;
                tier2Items.push_back(raw);
                break;
            }
            case df::job_type::MakeCrafts: {
                // scripts/idle-crafting.lua makeRockCraft: job.mat_type = 0;
                // one job_item, item_type=BOULDER, mat_type=0,
                // vector_id=BOULDER, flags2.non_economic + flags3.hard. The
                // bone/horn/shell variants (makeBoneCraft/makeHornCrafts/
                // makeShellCraft in the same file) each need a different
                // job.material_category bit instead and are NOT wired here —
                // this command has no parameter to choose a material, so
                // queuing MakeCrafts always produces the rock/BOULDER job.
                tier2SetJobMatType = true;
                tier2JobMatType = 0;
                df::job_item *rock = newFilterJobItem();
                rock->item_type = df::item_type::BOULDER;
                rock->mat_type = 0;
                rock->vector_id = df::job_item_vector_id::BOULDER;
                rock->flags2.bits.non_economic = true;
                rock->flags3.bits.hard = true;
                tier2Items.push_back(rock);
                break;
            }
            case df::job_type::WeaveCloth: {
                // workshops.lua:373-383 lists four thread-source variants
                // (plant/silk/yarn/inorganic); only the plant-thread variant
                // is wired here (no command parameter exists to pick
                // between them) — the others are a follow-up.
                df::job_item *thread = newFilterJobItem();
                thread->item_type = df::item_type::THREAD;
                thread->quantity = 15000;
                thread->min_dimension = 15000;
                thread->flags1.bits.collected = true;
                thread->flags2.bits.plant = true;
                tier2Items.push_back(thread);
                break;
            }
            case df::job_type::CollectWebs: {
                // workshops.lua:394-398: item_type=THREAD, quantity/
                // min_dimension=10, flags1.undisturbed (an intact spider
                // web, not yet collected into thread).
                df::job_item *web = newFilterJobItem();
                web->item_type = df::item_type::THREAD;
                web->quantity = 10;
                web->min_dimension = 10;
                web->flags1.bits.undisturbed = true;
                tier2Items.push_back(web);
                break;
            }
            case df::job_type::DyeThread: {
                // workshops.lua:429-434 "dye thread": reagent 1 is
                // dyeable, uncollected... collected THREAD; reagent 2 is
                // the unrotten dye plant/material itself.
                df::job_item *thread = newFilterJobItem();
                thread->item_type = df::item_type::THREAD;
                thread->quantity = 15000;
                thread->min_dimension = 15000;
                thread->flags1.bits.collected = true;
                thread->flags2.bits.dyeable = true;
                tier2Items.push_back(thread);
                df::job_item *dye = newFilterJobItem();
                dye->flags1.bits.unrotten = true;
                dye->flags2.bits.dye = true;
                tier2Items.push_back(dye);
                break;
            }
            case df::job_type::DyeCloth: {
                // workshops.lua:435-440 "dye cloth" item list matches this
                // exactly (CLOTH, quantity/min_dimension=10000,
                // flags2.dyeable + a dye reagent) but that entry's own
                // job_fields sets job_type=DyeThread, not DyeCloth — almost
                // certainly a copy-paste slip in that lua table, since
                // df::job_type has a distinct DyeCloth value (df/job_type.h)
                // and plugins/lua/stockflow.lua's collect_reactions
                // (line 358-359) lists DyeThread and DyeCloth as two
                // independent reaction_entry calls, not one. Assigning the
                // CLOTH-shaped item list to the DyeCloth job_type (rather
                // than reusing DyeThread for both, as the lua table
                // literally says) is a deliberate correction, not a blind
                // copy — flagged here in case live testing proves otherwise.
                df::job_item *cloth = newFilterJobItem();
                cloth->item_type = df::item_type::CLOTH;
                cloth->quantity = 10000;
                cloth->min_dimension = 10000;
                cloth->flags2.bits.dyeable = true;
                tier2Items.push_back(cloth);
                df::job_item *dye = newFilterJobItem();
                dye->flags1.bits.unrotten = true;
                dye->flags2.bits.dye = true;
                tier2Items.push_back(dye);
                break;
            }
            case df::job_type::MakeBackpack:
            case df::job_type::MakeQuiver: {
                // workshops.lua:400-421 Leatherworks defaults={item_type=
                // SKIN_TANNED} with items={{}} (no per-job override) for
                // both "construct backpack" and "construct quiver" — a bare
                // tanned-leather reagent, no additional flags.
                df::job_item *leather = newFilterJobItem();
                leather->item_type = df::item_type::SKIN_TANNED;
                tier2Items.push_back(leather);
                break;
            }
            default:
                tier2Handled = false;
                break;
        }
        if (tier2Handled) {
            df::job *tier2Job = new df::job();
            tier2Job->job_type = jobType;
            if (tier2SetJobMatType) {
                tier2Job->mat_type = (int16_t)tier2JobMatType;
            }
            for (auto *ji : tier2Items) {
                tier2Job->job_items.elements.push_back(ji);
            }

            Job::linkIntoWorld(tier2Job, true);
            if (!assignJobToBuilding(tier2Job, bld)) {
                Job::removeJob(tier2Job);
                error = "failed to attach job to workshop (queue full or invalid building)";
                return false;
            }
            return true;
        }
    }

    // ConstructTractionBench needs a full 3-item job_item vector (a table, a
    // mechanism, and a chain) rather than the single generic filter every
    // other job_type below uses -- confirmed via workshops.lua
    // jobs_workshop[Mechanics]'s "construct traction bench" entry:
    // items={{item_type=TABLE},{item_type=MECHANISM},{item_type=CHAIN}}.
    // MECHANISM there names a df::item_type value that does NOT exist in
    // this DFHack build (df/item_type.h has no MECHANISM entry -- confirmed
    // absent by grep) -- resolved instead to TRAPPARTS, the SAME item_type/
    // vector_id every other "mechanism" reagent in this codebase already
    // uses for the identical role (buildings.cpp's GearAssembly/Rollers/Well
    // placers, and this very file's ConstructMechanisms OUTPUT a few cases
    // up in jobTypeAllowedAtWorkshop's doc comment) -- TRAPPARTS is DF's
    // actual master vector for mechanism items; there is no other item type
    // a "mechanism" reagent could plausibly mean. Handled as an early
    // special case (own job_item vector, own linkIntoWorld/assign, own
    // return) rather than folding into the generic single-filter path below,
    // which has no way to express more than one job_item.
    if (jobType == df::job_type::ConstructTractionBench) {
        df::job *tbJob = new df::job();
        tbJob->job_type = jobType;

        df::job_item *table = new df::job_item();
        table->item_type = df::item_type::TABLE;
        table->vector_id = df::job_item_vector_id::TABLE;
        tbJob->job_items.elements.push_back(table);

        df::job_item *mechanism = new df::job_item();
        mechanism->item_type = df::item_type::TRAPPARTS;
        mechanism->vector_id = df::job_item_vector_id::TRAPPARTS;
        tbJob->job_items.elements.push_back(mechanism);

        df::job_item *chain = new df::job_item();
        chain->item_type = df::item_type::CHAIN;
        chain->vector_id = df::job_item_vector_id::CHAIN;
        tbJob->job_items.elements.push_back(chain);

        Job::linkIntoWorld(tbJob, true);
        if (!assignJobToBuilding(tbJob, bld)) {
            Job::removeJob(tbJob);
            error = "failed to attach job to workshop (queue full or invalid building)";
            return false;
        }
        return true;
    }

    // SmeltOre unlock (2026-07-19 manager-work-order fix wave): needs
    // job-level mat_type/mat_index pinned to ONE specific ore raw PLUS a
    // BOULDER job_item of that same material -- a fundamentally different
    // shape from the WOOD/BOULDER/BAR generic filter below, and (like
    // ConstructTractionBench above) needs its own job/job_item construction
    // and early return. Confirmed against library/lua/dfhack/workshops.lua's
    // addSmeltJobs (lines 519-532): job_fields={job_type=SmeltOre,
    // mat_type=INORGANIC, mat_index=<ore raw idx>}, one BOULDER job_item of
    // the same (mat_type, mat_index), plus a {item_type=BAR,mat_type=COAL}
    // fuel job_item -- but ONLY when use_fuel is true, and getJobs' one call
    // site (workshops.lua line 543) passes use_fuel=(workshopId==Smelter),
    // i.e. a MagmaSmelter needs no coal. jobTypeAllowedAtFurnace above
    // already gates entry to Smelter/MagmaSmelter only, so `furn` is always
    // non-null here.
    if (jobType == df::job_type::SmeltOre) {
        if (material.empty()) {
            error = "SmeltOre needs material=<exact ore token>, e.g. "
                    "material=INORGANIC:LIMONITE (a bare job_type name has no "
                    "way to pick which ore to smelt)";
            return false;
        }
        MaterialInfo ore;
        if (!ore.find(material)) {
            error = "unrecognized material '" + material +
                    "' for SmeltOre -- expected an exact ore token, e.g. "
                    "INORGANIC:LIMONITE";
            return false;
        }

        df::job *smeltJob = new df::job();
        smeltJob->job_type = jobType;
        smeltJob->mat_type = ore.type;
        smeltJob->mat_index = ore.index;

        df::job_item *boulder = new df::job_item();
        boulder->item_type = df::item_type::BOULDER;
        boulder->mat_type = ore.type;
        boulder->mat_index = ore.index;
        boulder->vector_id = df::job_item_vector_id::BOULDER;
        smeltJob->job_items.elements.push_back(boulder);

        if (furn->type == df::furnace_type::Smelter) {
            // Same item_type=BAR/mat_type=COAL fuel shape
            // applyQueueReactionJob already uses for FUEL-flagged reactions.
            df::job_item *fuelItem = new df::job_item();
            fuelItem->item_type = df::item_type::BAR;
            fuelItem->mat_type = df::builtin_mats::COAL;
            smeltJob->job_items.elements.push_back(fuelItem);
        }

        Job::linkIntoWorld(smeltJob, true);
        if (!assignJobToBuilding(smeltJob, bld)) {
            Job::removeJob(smeltJob);
            error = "failed to attach job to furnace (queue full or invalid building)";
            return false;
        }
        return true;
    }

    df::job *job = new df::job();
    job->job_type = jobType;
    // flags.bits.repeat defaults to false on a fresh job (df.job.xml has no
    // init-value for it) — a single one-off task, matching this command's
    // "queue one job" model; call again to queue more.

    // Material filter: this job CONSUMES a raw item (a log at Carpenters, a
    // boulder at Masons/Mechanics, a metal bar at Metalsmith's Forge) to
    // produce its output — a different DF mechanism from placeBuilding's
    // makeBuildMatFilter() in buildings.cpp, which constrains what item
    // class gets bound to a BUILDING PLACEMENT job (job_item flags2.bits.
    // building_material), not a workshop reaction's raw-material input.
    // Filter by item_type instead, matching the workshop's material class.
    // Mechanics joins Masons on the BOULDER side for ConstructMechanisms
    // (1x BOULDER -> 1x TRAPPARTS, workshops.lua:363-364) — every other
    // job_type reaching the WOOD/BOULDER branches is still furniture-domain
    // (WOOD at Carpenters, BOULDER at Masons). non_economic mirrors
    // DFHack's own default for unrestricted stone requests (Constructions::
    // designateNew, Constructions.cpp:102-103) — harmless for non-boulder
    // item types. The furnace branch (ws==null) only ever reaches here for
    // MakeCharcoal/MakeAsh (see jobTypeAllowedAtFurnace) which are
    // WoodFurnace/WOOD jobs -- same WOOD job_item shape as Carpenters, no
    // new material class added.
    //
    // MetalsmithsForge (MakeChain only, see jobTypeAllowedAtWorkshop above)
    // needs a THIRD shape: 1x metal bar, not wood or boulder. item_type=BAR
    // + vector_id=BAR + flags3.bits.metal=true is DFHack's own confirmed
    // "any metal bar" job_item filter -- the exact shape buildings.lua uses
    // for ReinforcedWall's second reagent (buildings.lua:405, the only other
    // metal-bar-by-class filter in this checkout), not a guess.
    df::job_item *ji = new df::job_item();
    if (ws && ws->type == df::workshop_type::MetalsmithsForge) {
        ji->item_type = df::item_type::BAR;
        ji->vector_id = df::job_item_vector_id::BAR;
        ji->flags3.bits.metal = true;
    } else {
        if (ws) {
            ji->item_type = (ws->type == df::workshop_type::Masons || ws->type == df::workshop_type::Mechanics)
                                ? df::item_type::BOULDER
                                : df::item_type::WOOD;
        } else {
            ji->item_type = df::item_type::WOOD;
        }
        ji->flags2.bits.non_economic = true;
    }
    job->job_items.elements.push_back(ji);

    Job::linkIntoWorld(job, true);
    if (!assignJobToBuilding(job, bld)) {
        Job::removeJob(job);
        error = "failed to attach job to workshop/furnace (queue full or invalid building)";
        return false;
    }

    return true;
}
