// DFHack Plugin — Lever/Trigger Mechanisms
//
// Implements PULL_LEVER and LINK_BUILDING: pulling a built lever, and
// wiring a lever to a trigger target (bridge/floodgate/door/hatch) so DF's
// own trigger machinery treats them as linked.
//
// PULL_LEVER — verified against DFHack's own scripts/lever.lua
// (leverPullJob): a lever pull is NOT a workshop reaction, so it does not
// go through Job::assignToWorkshop like work_orders.cpp's applyQueueJob.
// The job attaches straight to the lever building's own `jobs` vector via
// a BUILDING_HOLDER general_ref, then Job::linkIntoWorld +
// Job::checkBuildingsNow make DF notice it.
//
// LINK_BUILDING — df::job_type::LinkBuildingToTrigger exists but DFHack
// provides no C++ or Lua path anywhere in its own codebase that CREATES
// that job (it is entirely player-UI-driven in vanilla DF; every DFHack
// script that touches it only reads or cancels an existing link). The
// recipe below is DFHack's own acknowledged workaround for this gap:
// scripts/gui/advfort.lua's `fake_linking` (used in adventure mode, which
// has no dwarf labor to complete a real job) — direct struct wiring that
// installs two mechanisms (one in the lever, one in the target) and links
// them via general_refs, exactly mirroring what a completed
// LinkBuildingToTrigger job would leave behind. Every API below is a real,
// exported DFHack call; advfort.lua's own comments (and this project's
// research pass) note this exact sequence has not been visually
// live-verified against DF's "Linked Buildings" UI tab in a fortress-mode
// game — recommended before leaning on it in a real fort.

#include "Core.h"
#include "modules/Maps.h"
#include "modules/Buildings.h"
#include "modules/Job.h"
#include "modules/Items.h"

#include "df/building.h"
#include "df/building_trapst.h"
#include "df/building_bridgest.h"
#include "df/building_floodgatest.h"
#include "df/building_doorst.h"
#include "df/building_hatchst.h"
#include "df/general_ref_building_holderst.h"
#include "df/general_ref_building_triggerst.h"
#include "df/general_ref_building_triggertargetst.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/item.h"
#include "df/item_type.h"
#include "df/world.h"

#include "protocol.h"

#include <string>
#include <vector>

using namespace DFHack;

// applyPullLever queues DF's real PullLever job against the lever building
// occupying (x,y,z). Recipe confirmed C++-reachable via
// scripts/lever.lua's leverPullJob: allocate the job, attach a
// BUILDING_HOLDER general_ref pointing at the lever, push it onto the
// lever's OWN jobs vector (not a workshop's), then Job::linkIntoWorld +
// Job::checkBuildingsNow so DF picks it up.
bool applyPullLever(int16_t x, int16_t y, int16_t z, std::string &error)
{
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::building *bld = Buildings::findAtTile(df::coord(x, y, z));
    if (!bld) {
        error = "no building at that tile";
        return false;
    }
    df::building_trapst *lever = strict_virtual_cast<df::building_trapst>(bld);
    if (!lever || lever->trap_type != df::trap_type::Lever) {
        error = "building at that tile is not a lever";
        return false;
    }
    if (lever->getBuildStage() < lever->getMaxBuildStage()) {
        error = "lever still under construction -- pull after construction completes";
        return false;
    }

    df::job *job = new df::job();
    job->job_type = df::job_type::PullLever;
    job->pos = df::coord(lever->centerx, lever->centery, lever->z);

    // These general_ref_* subtypes have a protected constructor (DFHack
    // codegen convention for polymorphic df:: types) -- df::allocate<T>()
    // is the correct way to instantiate one, mirroring Items.cpp's own
    // moveToBuilding (df::allocate<general_ref_building_holderst>()).
    df::general_ref_building_holderst *ref = df::allocate<df::general_ref_building_holderst>();
    ref->building_id = lever->id;
    job->general_refs.push_back(ref);

    lever->jobs.push_back(job);
    Job::linkIntoWorld(job, true);
    Job::checkBuildingsNow();

    return true;
}

// findFreeMechanism scans world->items.all for an unclaimed TRAPPARTS item
// (DF's internal name for a built "mechanism") -- not already consumed by
// a job, not already installed in a building, not forbidden. excludeID
// lets the second lookup in a pair skip the item the first lookup just
// selected: DF's flags.bits.in_building isn't set on item1 until
// Items::moveToBuilding actually runs further down in applyLinkBuilding,
// so a second unguarded scan before that call would return the SAME item.
static df::item *findFreeMechanism(int32_t excludeID = -1)
{
    if (!df::global::world) return nullptr;
    for (df::item *it : df::global::world->items.all) {
        if (!it) continue;
        if (it->getType() != df::item_type::TRAPPARTS) continue;
        if (it->id == excludeID) continue;
        if (it->flags.bits.in_job || it->flags.bits.in_building || it->flags.bits.forbid) continue;
        // Skip caravan-owned/dumped/garbage items -- advfort routes this
        // through a suitability picker; without this a trader's mechanism
        // could be silently confiscated into a fort building.
        if (it->flags.bits.trader || it->flags.bits.dump || it->flags.bits.garbage_collect) continue;
        return it;
    }
    return nullptr;
}

// applyLinkBuilding wires the lever at (leverX,leverY,leverZ) to the
// trigger target at (targetX,targetY,targetZ) using DFHack's own
// direct-struct-wiring recipe (see file comment above for provenance).
// Consumes two free mechanisms from the fort's stockpiles -- distinct from
// the one mechanism a freshly-built lever already consumed at construction
// (that one sits unlinked in the lever's own contained_items until this
// call installs the two new ones).
bool applyLinkBuilding(int16_t leverX, int16_t leverY, int16_t leverZ,
                        int16_t targetX, int16_t targetY, int16_t targetZ,
                        std::string &error)
{
    if (!Maps::isValidTilePos(leverX, leverY, leverZ) ||
        !Maps::isValidTilePos(targetX, targetY, targetZ)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::building *leverBld = Buildings::findAtTile(df::coord(leverX, leverY, leverZ));
    if (!leverBld) {
        error = "no building at lever tile";
        return false;
    }
    df::building_trapst *lever = strict_virtual_cast<df::building_trapst>(leverBld);
    if (!lever || lever->trap_type != df::trap_type::Lever) {
        error = "building at lever tile is not a lever";
        return false;
    }
    if (lever->getBuildStage() < lever->getMaxBuildStage()) {
        error = "lever still under construction -- link after construction completes";
        return false;
    }

    df::building *target = Buildings::findAtTile(df::coord(targetX, targetY, targetZ));
    if (!target) {
        error = "no building at target tile";
        return false;
    }
    // Supported trigger targets for this pass: bridge, floodgate, door,
    // hatch -- the four DF-domain-verified cases (see file comment).
    // door/hatch additionally get operated_by_mechanisms flipped below,
    // mirroring advfort.lua's fake_linking; bridge/floodgate have no such
    // flag and need none (their gate machinery keys off linked_mechanisms
    // alone, same as leverPullInstant's walk).
    df::building_bridgest *bridge = strict_virtual_cast<df::building_bridgest>(target);
    df::building_floodgatest *floodgate = strict_virtual_cast<df::building_floodgatest>(target);
    df::building_doorst *door = strict_virtual_cast<df::building_doorst>(target);
    df::building_hatchst *hatch = strict_virtual_cast<df::building_hatchst>(target);
    if (!bridge && !floodgate && !door && !hatch) {
        error = "target building type is not a supported trigger target (bridge, floodgate, door, or hatch)";
        return false;
    }
    if (target->getBuildStage() < target->getMaxBuildStage()) {
        error = "target building still under construction -- link after construction completes";
        return false;
    }

    df::item *item1 = findFreeMechanism();
    if (!item1) {
        error = "no free mechanism (TRAPPARTS) available -- craft one at a Mechanic's workshop first (queue_job item=ConstructMechanisms)";
        return false;
    }
    df::item *item2 = findFreeMechanism(item1->id);
    if (!item2) {
        error = "only one free mechanism (TRAPPARTS) available -- linking consumes two; craft another at a Mechanic's workshop";
        return false;
    }

    // moveToBuilding is genuinely fallible (Items.cpp detachItem): it
    // returns false if the item has leftover specific_refs, an unexpected
    // world_data_id, or a stale BUILDING_TRIGGER/BUILDING_TRIGGERTARGET
    // general_ref -- none of which findFreeMechanism's in_job/in_building/
    // forbid checks catch (e.g. a mechanism salvaged from a previously
    // linked, deconstructed building). Mirrors advfort.lua's fake_linking,
    // which qerrors on exactly these two calls -- proceeding past a false
    // return would wire refs onto an item that was never installed.
    if (!Items::moveToBuilding(item1, (df::building_actual*)lever, df::building_item_role_type::PERM)) {
        error = "failed to install mechanism into lever (item not detachable -- check for stale refs)";
        return false;
    }
    if (!Items::moveToBuilding(item2, (df::building_actual*)target, df::building_item_role_type::PERM)) {
        // Roll item1 back out so the lever isn't left holding a mechanism
        // with no corresponding link -- avoid leaving a half-linked state.
        Items::moveToGround(item1, df::coord(lever->centerx, lever->centery, lever->z));
        error = "failed to install mechanism into target building (item not detachable -- check for stale refs)";
        return false;
    }

    // The mechanism now sitting in the TARGET "is triggered by" the lever.
    // df::allocate<T>() -- see the PULL_LEVER comment above on why plain
    // `new` doesn't work for these general_ref_* subtypes.
    df::general_ref_building_triggerst *trigRef = df::allocate<df::general_ref_building_triggerst>();
    trigRef->building_id = lever->id;
    item2->general_refs.push_back(trigRef);

    // The mechanism now sitting in the LEVER "targets" the target building.
    df::general_ref_building_triggertargetst *targetRef = df::allocate<df::general_ref_building_triggertargetst>();
    targetRef->building_id = target->id;
    item1->general_refs.push_back(targetRef);

    // Lever's own bookkeeping: leverPullInstant (scripts/lever.lua) walks
    // linked_mechanisms to find every building this lever controls.
    lever->linked_mechanisms.push_back(item2);

    if (door) {
        door->door_flags.bits.operated_by_mechanisms = true;
    } else if (hatch) {
        hatch->door_flags.bits.operated_by_mechanisms = true;
    }

    return true;
}
