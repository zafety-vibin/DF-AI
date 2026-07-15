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

#include "df/job_type.h"
#include "df/job.h"
#include "df/job_item.h"
#include "df/manager_order.h"
#include "df/world.h"
#include "df/building.h"
#include "df/building_type.h"
#include "df/building_workshopst.h"
#include "df/workshop_type.h"
#include "df/reaction.h"
#include "df/reaction_reagent.h"
#include "df/reaction_reagent_itemst.h"
#include "df/reaction_reagent_type.h"
#include "df/reaction_flags.h"
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
// Only reachable from applyQueueJob (see NOTE on ORDER_TYPE_BY_NAME in
// protocol.h) — applyWorkOrder/the manager-queue path still only accepts
// the hand-maintained ORDER_TYPE_* bytes.
static int resolveJobTypeByName(const std::string &name, std::string &error) {
    df::job_type jt;
    if (!find_enum_item(&jt, name)) {
        error = "unrecognized job type name: '" + name +
                "' (use the job_types query/tool to look up valid DFHack job_type names)";
        return -1;
    }
    return (int)jt;
}

bool applyWorkOrder(uint8_t orderType, uint16_t quantity, std::string &error)
{
    int jobType = protocolToJobType(orderType);
    if (jobType < 0) {
        char buf[64];
        snprintf(buf, sizeof(buf), "Unknown work order type: 0x%02X", orderType);
        error = buf;
        return false;
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
    order->status.bits.validated = 1;

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
// WOOD/BOULDER filter further down) before applyQueueJob accepts it. Add
// entries as each job type's real workshop/material requirement is
// confirmed. (ConstructHatchCover was the first such addition: grouped
// with the door/furniture set — same Carpenters/Masons pair, same
// per-workshop WOOD/BOULDER material class as ConstructDoor.)
static bool jobTypeAllowedAtWorkshop(df::job_type jobType, df::workshop_type wsType) {
    switch (jobType) {
        case df::job_type::ConstructBed:
        case df::job_type::ConstructTable:
        case df::job_type::ConstructThrone:
        case df::job_type::ConstructDoor:
        case df::job_type::ConstructHatchCover:
        case df::job_type::ConstructCabinet:
        case df::job_type::ConstructChest:
            return wsType == df::workshop_type::Carpenters || wsType == df::workshop_type::Masons;
        case df::job_type::MakeBarrel:
        case df::job_type::MakeBucket:
            return wsType == df::workshop_type::Carpenters;
        case df::job_type::ConstructBlocks:
            return wsType == df::workshop_type::Masons || wsType == df::workshop_type::Carpenters;
        case df::job_type::MakeCrafts:
            return wsType == df::workshop_type::Craftsdwarfs;
        case df::job_type::PrepareMeal:
            return wsType == df::workshop_type::Kitchen;
        default:
            return false;
    }
    // NOTE: MakeCrafts/PrepareMeal passing this workshop check does NOT mean
    // applyQueueJob will accept them — the Carpenters/Masons WOOD/BOULDER
    // job_item filter further down is provably wrong for both (see the
    // reject block in applyQueueJob, right after this function's call site)
    // and there is no verified replacement filter yet. The workshop mapping
    // above is correct DF domain knowledge on its own; only the downstream
    // material filter is the problem.
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
    if (!ws) {
        error = "building at that location is not a workshop";
        return false;
    }

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

    if (!reaction->flags.is_set(df::reaction_flags::FORTRESS_MODE_ENABLED)) {
        error = "reaction '" + reactionCode + "' is not enabled in fortress mode";
        return false;
    }

    // Workshop compatibility: reaction->building.type/subtype/custom are
    // PARALLEL ARRAYS of alternative buildings the reaction can run at
    // (df/reaction.h T_building) -- a value of -1 in the type or subtype
    // slot means "any" (DF's own wildcard convention for these enums, see
    // df/building_type.h / df/workshop_type.h NONE=-1). Mirrors DFHack's
    // own matchIds/scanRawsReaction (library/lua/dfhack/workshops.lua) --
    // custom is not checked here, same scope as that scout note.
    bool compatible = false;
    std::ostringstream wants;
    size_t nAlt = reaction->building.type.size();
    for (size_t k = 0; k < nAlt; k++) {
        int32_t altBuildingType = (int32_t)reaction->building.type[k];
        int32_t altSubtype = (k < reaction->building.subtype.size()) ? reaction->building.subtype[k] : -1;
        if (k > 0) wants << ", ";
        if (altBuildingType == (int32_t)df::building_type::Workshop) {
            wants << (altSubtype == -1 ? "any workshop" : ENUM_KEY_STR(workshop_type, (df::workshop_type)altSubtype));
        } else {
            wants << ENUM_KEY_STR(building_type, (df::building_type)altBuildingType);
        }
        if (altBuildingType != -1 && altBuildingType != (int32_t)df::building_type::Workshop) continue;
        if (altSubtype != -1 && altSubtype != (int32_t)ws->type) continue;
        compatible = true;
    }
    if (!compatible) {
        error = "reaction '" + reactionCode + "' does not run at this workshop ("
                + ENUM_KEY_STR(workshop_type, ws->type) + ") -- it wants: " + wants.str();
        return false;
    }

    if (ws->jobs.size() >= 10) {
        error = "workshop job queue is full (10 jobs)";
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
    if (!Job::assignToWorkshop(job, ws)) {
        Job::removeJob(job);
        error = "failed to attach job to workshop (queue full or invalid workshop)";
        return false;
    }

    return true;
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
bool applyQueueJob(int16_t x, int16_t y, int16_t z, uint8_t orderType, const std::string &jobTypeName, std::string &error)
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

    df::building_workshopst *ws = strict_virtual_cast<df::building_workshopst>(bld);
    if (!ws) {
        error = "building at that location is not a workshop";
        return false;
    }

    if (!jobTypeAllowedAtWorkshop(jobType, ws->type)) {
        std::ostringstream os;
        os << "job type not supported at this workshop type ("
           << ENUM_KEY_STR(workshop_type, ws->type) << ")";
        error = os.str();
        return false;
    }

    // MakeCrafts and PrepareMeal are workshop-compatible (jobTypeAllowedAtWorkshop
    // above is correct for both) but are rejected here, BEFORE the
    // Carpenters/Masons WOOD/BOULDER job_item filter further down, because that
    // filter is provably wrong for them and this command has no verified
    // replacement. DFHack's own scripts/idle-crafting.lua (makeBoneCraft/
    // makeHornCrafts/makeShellCraft/makeRockCraft) shows MakeCrafts actually
    // needs item_type::NONE + a material_category flag (bone/horn/shell) OR
    // item_type::BOULDER + vector_id::BOULDER — chosen per material the fort
    // actually has, a choice this command takes no input for. PrepareMeal has
    // no job_item precedent at all in DFHack's own code; its only sourced
    // reference (scripts/internal/quickfort/stockflow.lua, plugins/lua/
    // stockflow.lua reaction_entry(job_types.PrepareMeal, {mat_type=2/3/4}))
    // queues it as a manager_order (applyWorkOrder's path, not this one) with
    // no job_item at all. Stamping item_type::WOOD onto either job — what the
    // fallthrough below does for every job_type not special-cased as Masons —
    // would queue a job that can never be worked (no wood-log reagent either
    // job should ever require) while still ACKing SUCCESS, a truthful-ACK
    // violation. Fail loudly instead. Use `order` (applyWorkOrder) for these
    // two until a verified job_item filter is added here — future work, not
    // solved by this pass.
    if (jobType == df::job_type::MakeCrafts || jobType == df::job_type::PrepareMeal) {
        std::ostringstream os;
        os << "queue_job does not yet support " << ENUM_KEY_STR(job_type, jobType)
           << " — no verified job_item material filter for it (the WOOD/BOULDER"
              " filter below is furniture-job domain knowledge and wrong here);"
              " use order (manager work order) for this item type instead";
        error = os.str();
        return false;
    }

    if (ws->jobs.size() >= 10) {
        error = "workshop job queue is full (10 jobs)";
        return false;
    }

    df::job *job = new df::job();
    job->job_type = jobType;
    // flags.bits.repeat defaults to false on a fresh job (df.job.xml has no
    // init-value for it) — a single one-off task, matching this command's
    // "queue one job" model; call again to queue more.

    // Material filter: this job CONSUMES a raw item (a log at Carpenters, a
    // boulder at Masons) to produce its output — a different DF mechanism
    // from placeBuilding's makeBuildMatFilter() in buildings.cpp, which
    // constrains what item class gets bound to a BUILDING PLACEMENT job
    // (job_item flags2.bits.building_material), not a workshop reaction's
    // raw-material input. Filter by item_type instead, matching the
    // workshop's material class. non_economic mirrors DFHack's own default
    // for unrestricted stone requests (Constructions::designateNew,
    // Constructions.cpp:102-103) — harmless for non-boulder item types.
    df::job_item *ji = new df::job_item();
    ji->item_type = (ws->type == df::workshop_type::Masons)
                        ? df::item_type::BOULDER
                        : df::item_type::WOOD;
    ji->flags2.bits.non_economic = true;
    job->job_items.elements.push_back(ji);

    Job::linkIntoWorld(job, true);
    if (!Job::assignToWorkshop(job, ws)) {
        Job::removeJob(job);
        error = "failed to attach job to workshop (queue full or invalid workshop)";
        return false;
    }

    return true;
}
