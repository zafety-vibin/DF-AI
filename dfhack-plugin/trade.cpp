// DFHack Plugin — Trade/Caravan Actions
//
// Implements the safe subset of trade tooling identified by the 2026-07-19
// trade/caravan research pass (docs/decisions.md): bring_goods_to_depot
// marks fort-owned items for hauling to a built trade depot, mirroring
// DFHack's own eligibility filter (scripts/internal/caravan/movegoods.lua
// is_tradeable_item) and its commit mechanism (Items::markForTrade,
// library/modules/Items.cpp:1912) — a pure data-layer operation with no
// viewscreen involved. DF's own hauling AI then carries the item exactly
// like any other stockpile-hauling job; no further dispatch is needed here.
//
// OUT OF SCOPE, deliberately: executing the actual trade (the Offer/Trade
// commit at the depot) is native, non-exported DF viewscreen input-handling
// code that DFHack itself never calls — scripts/internal/caravan/trade.lua
// only ever toggles selection-state flags and leaves the commit to the
// vanilla screen's own button handlers. No CHECK_*-free API for it exists
// in Items.h/Buildings.h/Job.h. See the research report's section (d) for
// the full justification; that step stays a human-in-the-client action.

#include "Core.h"
#include "modules/Buildings.h"
#include "modules/Items.h"
#include "modules/Job.h"
#include "modules/Materials.h"
#include "modules/Maps.h"

#include "df/world.h"
#include "df/building_tradedepotst.h"
#include "df/building_item_role_type.h"
#include "df/buildingitemst.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/job_item_ref.h"
#include "df/item.h"
#include "df/item_binst.h"
#include "df/item_type.h"
#include "df/item_flags.h"
#include "df/items_other_id.h"

#include "protocol.h"

#include <cctype>
#include <cstdio>
#include <vector>

using namespace DFHack;

// kNotTradeableFlags mirrors movegoods.lua's is_tradeable_item hard rejects
// (dfhack-build scripts/internal/caravan/movegoods.lua:354-368) — hostile,
// removed, a dead dwarf's remains, spider web, construction material,
// encased, murder evidence, caravan-owned, dwarf-owned, garbage-collected,
// or on-fire items are never legitimate trade goods regardless of filter
// match. `forbid` is deliberately NOT in this mask: movegoods.lua clears it
// unconditionally when committing a mark (onDismiss, line 755:
// `item.flags.forbid = false`) rather than skipping forbidden items —
// applyBringGoodsToDepot does the same below.
static const uint32_t kNotTradeableFlags =
    (uint32_t)df::item_flags::Mask::mask_hostile |
    (uint32_t)df::item_flags::Mask::mask_removed |
    (uint32_t)df::item_flags::Mask::mask_dead_dwarf |
    (uint32_t)df::item_flags::Mask::mask_spider_web |
    (uint32_t)df::item_flags::Mask::mask_construction |
    (uint32_t)df::item_flags::Mask::mask_encased |
    (uint32_t)df::item_flags::Mask::mask_murder |
    (uint32_t)df::item_flags::Mask::mask_trader |
    (uint32_t)df::item_flags::Mask::mask_owned |
    (uint32_t)df::item_flags::Mask::mask_garbage_collect |
    (uint32_t)df::item_flags::Mask::mask_on_fire;

// itemClassMatches reports whether itype belongs to the curated
// ITEM_CLASS_* bucket itemClass names (protocol.h). ITEM_CLASS_ANY matches
// everything (no class gate — the caller's itemTypeFilter/materialFilter
// alone decide, exactly as before this field existed). ITEM_CLASS_CRAFTS
// mirrors job_type::MakeCrafts's own possible_item list (df.job.xml:
// FIGURINE/AMULET/RING/EARRING/CROWN/BRACELET/SCEPTER) plus TOTEM
// (job_type::MakeTotem — a separate job producing the same kind of small
// trinket/finished good, folded into the same trade bucket) — DF 53.15 has
// no top-level item_type::CRAFTS value at all, so this bucket is the only
// way to select "finished goods" as a class rather than a single item_type
// substring.
static bool itemClassMatches(uint8_t itemClass, df::item_type itype) {
    if (itemClass == ITEM_CLASS_ANY) return true;
    if (itemClass == ITEM_CLASS_CRAFTS) {
        switch (itype) {
            case df::item_type::FIGURINE:
            case df::item_type::AMULET:
            case df::item_type::RING:
            case df::item_type::EARRING:
            case df::item_type::CROWN:
            case df::item_type::BRACELET:
            case df::item_type::SCEPTER:
            case df::item_type::TOTEM:
                return true;
            default:
                return false;
        }
    }
    return false;
}

// applyBringGoodsToDepot marks up to maxCount free fort items for hauling
// to the built trade depot at (x,y,z), filtered by itemTypeFilter (a
// substring match against the plugin's decoded item_type name, e.g.
// "CRAFTS" — same convention as queries.cpp handleStockpileInventory's
// category filter), materialFilter (a case-insensitive substring match
// against the item's decoded material name, e.g. "silver"), and itemClass
// (an ITEM_CLASS_* constant, protocol.h — see itemClassMatches above).
// Either string filter empty matches everything; itemClass ==
// ITEM_CLASS_ANY applies no class gate. Stops once the running total
// estimated value would exceed maxTotalValue (<= 0 means no value cap).
//
// Eligibility, beyond kNotTradeableFlags above:
//   - Items already attached to any job (flags.in_job) are skipped, whether
//     or not that job is already a BringItemToDepot for this same depot —
//     avoids ever risking a second hauling job on one item.
//   - Items already part of some building (flags.in_building — construction
//     material, installed furniture, or already staged at a depot) are
//     skipped. KNOWN LIMITATION: unlike movegoods.lua, this function does
//     NOT special-case an item already sitting in THIS depot's own
//     contained_items with a TEMP role (a rare leftover-staged-good state) —
//     it is simply left alone rather than re-flagged in_building=true.
//   - Maps::canWalkBetween(item position, depot position) must hold — the
//     one check Items::markForTrade itself does NOT perform (it only
//     validates the depot's own build stage), matching what movegoods.lua's
//     UI filters out before ever letting a caller select an item.
//   - KNOWN LIMITATION: items nested inside a bin/food-storage container
//     (flags.in_inventory) are reached ONLY when itemClass != ITEM_CLASS_ANY
//     (see the whole-bin branch below) — a container-reaching item_class
//     filter is the trade-pack wave's fix for the root cause DF-AI's own
//     Fort #6 hit (a bin of shell crafts was completely unreachable: bins
//     always have flags.in_inventory==false themselves, but every content
//     inside one does, and the OLD code skipped any in_inventory item
//     unconditionally regardless of filter). itemClass == ITEM_CLASS_ANY
//     (the default) leaves bins opaque exactly as before this field
//     existed — trade the container itself instead of reaching into it.
//     Creature/workshop inventories are never reached either way.
bool applyBringGoodsToDepot(int16_t x, int16_t y, int16_t z,
                             const std::string &itemTypeFilter,
                             const std::string &materialFilter,
                             uint8_t itemClass,
                             int32_t maxCount, int64_t maxTotalValue,
                             std::string &error)
{
    if (!df::global::world) {
        error = "world is null";
        return false;
    }
    if (maxCount < 1) {
        error = "max_count must be >= 1";
        return false;
    }

    df::building *b = Buildings::findAtTile(df::coord(x, y, z));
    if (!b) {
        error = "no building at depot coordinates";
        return false;
    }
    df::building_tradedepotst *depot = strict_virtual_cast<df::building_tradedepotst>(b);
    if (!depot) {
        error = "building at coordinates is not a trade depot";
        return false;
    }
    // Same preconditions Items::markForTrade itself checks (Items.cpp:1916)
    // -- checked here too so a rejected depot produces one clear error
    // instead of N per-item silent skips further down.
    if (depot->getBuildStage() < depot->getMaxBuildStage()) {
        error = "trade depot is not yet fully built";
        return false;
    }
    if (!depot->jobs.empty() && depot->jobs[0]->job_type == df::job_type::DestroyBuilding) {
        error = "trade depot is queued for removal";
        return false;
    }

    std::string lowerMaterialFilter = materialFilter;
    for (auto &c : lowerMaterialFilter) c = (char)tolower((unsigned char)c);

    df::coord depotPos(depot->centerx, depot->centery, depot->z);

    int32_t marked = 0;
    int32_t binsMarked = 0;
    int64_t totalValue = 0;
    for (auto *item : df::global::world->items.other[df::items_other_id::IN_PLAY]) {
        if (!item) continue;
        if (marked >= maxCount) break;
        if (item->flags.whole & kNotTradeableFlags) continue;
        if (item->flags.bits.in_inventory) continue;
        if (item->flags.bits.in_job) continue;
        if (item->flags.bits.in_building) continue;

        bool isContainer = strict_virtual_cast<df::item_binst>(item) != nullptr || item->isFoodStorage();

        if (itemClass != ITEM_CLASS_ANY && isContainer) {
            // Whole-bin class-filtered staging: itemTypeFilter/
            // materialFilter apply to each CONTAINED item, not the bin's
            // own type/material — if any content matches, the whole bin
            // (and everything else physically inside it, matching or not)
            // is marked in one hauling job, mirroring DFHack's own
            // container-hauling behavior (Items::markForTrade attached to
            // the bin carries its contents along via the bin's own
            // CONTAINED_IN_ITEM general_refs — ordinary DF mechanics, no
            // new job machinery here).
            std::vector<df::item*> contents;
            Items::getContainedItems(item, &contents);
            bool anyMatch = false;
            int64_t candidateValue = Items::getValue(item);
            for (auto *ci : contents) {
                if (!ci) continue;
                candidateValue += Items::getValue(ci);
                if (anyMatch) continue; // still need to sum every content's value below
                if (ci->flags.whole & kNotTradeableFlags) continue;
                if (!itemClassMatches(itemClass, ci->getType())) continue;
                std::string ciTypeName = ENUM_KEY_STR(item_type, ci->getType());
                if (!itemTypeFilter.empty() && ciTypeName.find(itemTypeFilter) == std::string::npos) continue;
                if (!lowerMaterialFilter.empty()) {
                    MaterialInfo cmi((int16_t)ci->getActualMaterial(), (int32_t)ci->getActualMaterialIndex());
                    std::string cMatName = cmi.isValid() ? cmi.toString() : "";
                    for (auto &c : cMatName) c = (char)tolower((unsigned char)c);
                    if (cMatName.find(lowerMaterialFilter) == std::string::npos) continue;
                }
                anyMatch = true;
            }
            if (!anyMatch) continue;
            if (maxTotalValue > 0 && totalValue + candidateValue > maxTotalValue) continue;
            if (!Maps::canWalkBetween(Items::getPosition(item), depotPos)) continue;
            if (!Items::markForTrade(item, depot)) continue;
            item->flags.bits.forbid = false;
            marked++;
            binsMarked++;
            totalValue += candidateValue;
            continue;
        }

        df::item_type itype = item->getType();
        if (itemClass != ITEM_CLASS_ANY && !itemClassMatches(itemClass, itype)) continue;

        std::string typeName = ENUM_KEY_STR(item_type, itype);
        if (!itemTypeFilter.empty() && typeName.find(itemTypeFilter) == std::string::npos) continue;

        MaterialInfo mi((int16_t)item->getActualMaterial(), (int32_t)item->getActualMaterialIndex());
        if (!lowerMaterialFilter.empty()) {
            std::string matName = mi.isValid() ? mi.toString() : "";
            for (auto &c : matName) c = (char)tolower((unsigned char)c);
            if (matName.find(lowerMaterialFilter) == std::string::npos) continue;
        }

        int32_t value = Items::getValue(item);
        if (maxTotalValue > 0 && totalValue + value > maxTotalValue) continue;

        if (!Maps::canWalkBetween(Items::getPosition(item), depotPos)) continue;

        if (!Items::markForTrade(item, depot)) continue;

        // Committing the mark clears forbid, mirroring movegoods.lua's own
        // onDismiss (line 755) — not a separate step from the caller's view.
        item->flags.bits.forbid = false;

        marked++;
        totalValue += value;
    }

    char buf[256];
    if (binsMarked > 0) {
        snprintf(buf, sizeof(buf),
                 "marked %d item(s) (%d whole bin(s), marked for their matching contents) total value %lld, "
                 "for trade at depot (%d,%d,%d)",
                 (int)marked, (int)binsMarked, (long long)totalValue,
                 (int)depot->centerx, (int)depot->centery, (int)depot->z);
    } else {
        snprintf(buf, sizeof(buf),
                 "marked %d item(s), total value %lld, for trade at depot (%d,%d,%d)",
                 (int)marked, (long long)totalValue,
                 (int)depot->centerx, (int)depot->centery, (int)depot->z);
    }
    error = buf;
    return true;
}

// applyUnmarkTradeGoods reverses applyBringGoodsToDepot's marking at the
// trade depot at (x,y,z), filtered by the same itemTypeFilter/
// materialFilter/itemClass surface (no maxTotalValue — nothing to cap when
// releasing goods). Two populations, both scanned up to maxCount total
// matches, mirroring scripts/internal/caravan/movegoods.lua's own
// onDismiss unmark paths exactly:
//   - PENDING: a queued BringItemToDepot job at this depot (depot->jobs,
//     same loop shape queries.cpp's handleDepotGoods already uses) whose
//     job_item(s) include a matching item — cancelled outright via
//     DFHack's own Job::removeJob, the exact primitive movegoods.lua's own
//     unmark (dfhack.job.removeJob on the item's JOB specific_ref) uses.
//   - STAGED: an already-arrived item sitting in the depot's
//     contained_items with use_mode==TEMP — released by clearing
//     flags.bits.in_building, movegoods.lua's own unmark for that case
//     (`item.flags.in_building = false`). A stale buildingitemst entry
//     left behind (no DFHack API removes one — Items.cpp's moveToBuilding
//     push has no documented inverse) is tolerated here exactly like
//     movegoods.lua tolerates it; queries.cpp's handleDepotGoods staged
//     view now also gates on flags.bits.in_building so it stops rendering
//     that stale entry as still staged.
//
// SAFETY INVARIANT: never touches an item with flags.bits.trader==true
// (merchant stock is never ours to unmark) — the same mask_trader guard
// applyBringGoodsToDepot's own kNotTradeableFlags applies at mark time.
bool applyUnmarkTradeGoods(int16_t x, int16_t y, int16_t z,
                            const std::string &itemTypeFilter,
                            const std::string &materialFilter,
                            uint8_t itemClass,
                            int32_t maxCount, std::string &error)
{
    if (!df::global::world) {
        error = "world is null";
        return false;
    }
    if (maxCount < 1) {
        error = "max_count must be >= 1";
        return false;
    }

    df::building *b = Buildings::findAtTile(df::coord(x, y, z));
    if (!b) {
        error = "no building at depot coordinates";
        return false;
    }
    df::building_tradedepotst *depot = strict_virtual_cast<df::building_tradedepotst>(b);
    if (!depot) {
        error = "building at coordinates is not a trade depot";
        return false;
    }

    std::string lowerMaterialFilter = materialFilter;
    for (auto &c : lowerMaterialFilter) c = (char)tolower((unsigned char)c);

    auto matchesFilters = [&](df::item *item) -> bool {
        if (!item) return false;
        if (item->flags.bits.trader) return false; // never touch merchant stock
        df::item_type itype = item->getType();
        if (itemClass != ITEM_CLASS_ANY && !itemClassMatches(itemClass, itype)) return false;
        std::string typeName = ENUM_KEY_STR(item_type, itype);
        if (!itemTypeFilter.empty() && typeName.find(itemTypeFilter) == std::string::npos) return false;
        if (!lowerMaterialFilter.empty()) {
            MaterialInfo mi((int16_t)item->getActualMaterial(), (int32_t)item->getActualMaterialIndex());
            std::string matName = mi.isValid() ? mi.toString() : "";
            for (auto &c : matName) c = (char)tolower((unsigned char)c);
            if (matName.find(lowerMaterialFilter) == std::string::npos) return false;
        }
        return true;
    };

    int32_t unmarked = 0;
    int32_t pendingCleared = 0;
    int32_t stagedCleared = 0;

    // PENDING: queued BringItemToDepot jobs at this depot -- collected
    // before cancelling any (Job::removeJob unlinks from depot->jobs;
    // mutating that vector while walking it would be unsafe, same
    // collect-then-act precedent work_orders.cpp's applyCancelOrder uses).
    std::vector<df::job*> jobsToCancel;
    for (auto *job : depot->jobs) {
        if ((int32_t)jobsToCancel.size() >= maxCount) break;
        if (!job || job->job_type != df::job_type::BringItemToDepot) continue;
        bool jobMatches = false;
        for (auto *ji : job->items) {
            if (ji && matchesFilters(ji->item)) { jobMatches = true; break; }
        }
        if (!jobMatches) continue;
        jobsToCancel.push_back(job);
    }
    for (auto *job : jobsToCancel) {
        Job::removeJob(job);
        pendingCleared++;
    }
    unmarked += pendingCleared;

    // STAGED: already-arrived items at the depot, not already stale from a
    // prior unmark (flags.bits.in_building -- see handleDepotGoods' own
    // matching gate, queries.cpp).
    for (auto *bi : depot->contained_items) {
        if (unmarked >= maxCount) break;
        if (!bi || !bi->item) continue;
        if (bi->use_mode != df::building_item_role_type::TEMP) continue;
        if (!bi->item->flags.bits.in_building) continue;
        if (!matchesFilters(bi->item)) continue;
        bi->item->flags.bits.in_building = false;
        unmarked++;
        stagedCleared++;
    }

    char buf[256];
    snprintf(buf, sizeof(buf),
             "unmarked %d item(s) at depot (%d,%d,%d) (%d pending job(s) cancelled, %d staged item(s) released)",
             (int)unmarked, (int)depot->centerx, (int)depot->centery, (int)depot->z,
             (int)pendingCleared, (int)stagedCleared);
    error = buf;
    return true;
}

// applySetDepotTradeFlags writes the trade depot at (x,y,z)'s trade_flags
// bitfield (building_tradedepot_flag.h -- a bare, validation-free 2-bit
// field, df.building.xml:1683) directly. Both traderRequested and
// anyoneCanTrade are the WHOLE desired final state, not a delta -- the
// caller is expected to have read current values via caravan_status first.
//
// SIDE EFFECT (matches DFHack's own scripts/caravan.lua:106-115 'leave'
// command exactly): when traderRequested transitions true -> false, ALSO
// cancel any pending TradeAtDepot job at this depot -- clearing the flag
// alone would leave a stale "come trade" job dangling. caravan.lua's own
// sequence is: clear trade_flags.trader_requested, then walk depot->jobs
// for a job_type::TradeAtDepot and dfhack.job.removeJob it (at most one
// such job is ever expected per depot, matching caravan.lua's own `break`
// after the first match).
bool applySetDepotTradeFlags(int16_t x, int16_t y, int16_t z,
                              bool traderRequested, bool anyoneCanTrade,
                              std::string &error)
{
    if (!df::global::world) {
        error = "world is null";
        return false;
    }

    df::building *b = Buildings::findAtTile(df::coord(x, y, z));
    if (!b) {
        error = "no building at depot coordinates";
        return false;
    }
    df::building_tradedepotst *depot = strict_virtual_cast<df::building_tradedepotst>(b);
    if (!depot) {
        error = "building at coordinates is not a trade depot";
        return false;
    }

    bool wasTraderRequested = depot->trade_flags.bits.trader_requested;
    depot->trade_flags.bits.trader_requested = traderRequested;
    depot->trade_flags.bits.anyone_can_trade = anyoneCanTrade;

    bool jobCancelled = false;
    if (wasTraderRequested && !traderRequested) {
        for (auto *job : depot->jobs) {
            if (job && job->job_type == df::job_type::TradeAtDepot) {
                Job::removeJob(job);
                jobCancelled = true;
                break;
            }
        }
    }

    char buf[256];
    snprintf(buf, sizeof(buf),
             "set depot (%d,%d,%d) trade_flags: trader_requested=%s anyone_can_trade=%s%s",
             (int)depot->centerx, (int)depot->centery, (int)depot->z,
             traderRequested ? "true" : "false", anyoneCanTrade ? "true" : "false",
             jobCancelled ? " (cancelled pending TradeAtDepot job)" : "");
    error = buf;
    return true;
}
