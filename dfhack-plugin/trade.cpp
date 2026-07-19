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
#include "modules/Materials.h"
#include "modules/Maps.h"

#include "df/world.h"
#include "df/building_tradedepotst.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/item.h"
#include "df/item_type.h"
#include "df/item_flags.h"
#include "df/items_other_id.h"

#include <cctype>
#include <cstdio>

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

// applyBringGoodsToDepot marks up to maxCount free fort items for hauling
// to the built trade depot at (x,y,z), filtered by itemTypeFilter (a
// substring match against the plugin's decoded item_type name, e.g.
// "CRAFTS" — same convention as queries.cpp handleStockpileInventory's
// category filter) and materialFilter (a case-insensitive substring match
// against the item's decoded material name, e.g. "silver"). Either filter
// empty matches everything. Stops once the running total estimated value
// would exceed maxTotalValue (<= 0 means no value cap).
//
// Eligibility, beyond kNotTradeableFlags above:
//   - KNOWN LIMITATION: items nested inside a container/creature/workshop
//     inventory (flags.in_inventory) are not reached — trade the container
//     itself instead of reaching into it. Matches movegoods.lua's own
//     is_container gate in spirit, simplified: this function never looks
//     inside a bin, movegoods.lua sometimes does.
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
bool applyBringGoodsToDepot(int16_t x, int16_t y, int16_t z,
                             const std::string &itemTypeFilter,
                             const std::string &materialFilter,
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
    int64_t totalValue = 0;
    for (auto *item : df::global::world->items.other[df::items_other_id::IN_PLAY]) {
        if (!item) continue;
        if (marked >= maxCount) break;
        if (item->flags.whole & kNotTradeableFlags) continue;
        if (item->flags.bits.in_inventory) continue;
        if (item->flags.bits.in_job) continue;
        if (item->flags.bits.in_building) continue;

        std::string typeName = ENUM_KEY_STR(item_type, item->getType());
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
    snprintf(buf, sizeof(buf),
             "marked %d item(s), total value %lld, for trade at depot (%d,%d,%d)",
             (int)marked, (long long)totalValue,
             (int)depot->centerx, (int)depot->centery, (int)depot->z);
    error = buf;
    return true;
}
