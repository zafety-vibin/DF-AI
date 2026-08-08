// dfhack-plugin/work_details.cpp
//
// Work Detail (labor-group) commands: assign/unassign a unit, change a
// detail's mode (Default/EverybodyDoesThis/NobodyDoesThis/
// OnlySelectedDoesThis), and create a new custom work detail. The
// authoritative store is df::global::plotinfo->labor_info.work_details
// (df.plotinfo.xml: labor_infost, since v0.50.01, a std::vector<work_detail*>
// -- struct-type 'work_detail' original-name 'work_detailst') --
// unit->status.labors (see applySetLabor in df_ai_protocol.cpp) is a
// DERIVED CACHE the game recomputes FROM these work details via
// Units::setAutomaticProfessions(unit) (dfhack-build library/modules/
// Units.cpp:965-973, wrapping DF's own bay12 entry point) whenever a
// detail's membership or mode changes -- mirrors scripts/gui/
// manipulator.lua:641-658, DFHack's own work-details editor, which does
// the exact same toggle-then-recompute per unit.
//
// NobodyDoesThis semantics (does the game actively veto the labor for
// members, or just stop auto-assigning it) are NOT verified anywhere in
// this checkout -- the functions below stay truthful about what byte was
// WRITTEN to the struct, never a guess about what DF's job-assignment AI
// does with it afterward.
//
// All three mutate global game state (same class of operation as
// zones.cpp's applyDesignateZone / burrows.cpp's applyDesignateBurrow) and
// must run on the executeCommand path (try/catch guard + socket-thread
// drain), never a raw thread.

#include "Core.h"
#include "Console.h"
#include "modules/Units.h"

#include "df/world.h"
#include "df/plotinfost.h"
#include "df/labor_infost.h"
#include "df/work_detail.h"
#include "df/work_detail_flags.h"
#include "df/work_detail_mode.h"
#include "df/work_detail_icon_type.h"
#include "df/unit.h"
#include "df/unit_labor.h"

#include "protocol.h"
#include "MiscUtils.h"

#include <string>
#include <vector>

using namespace DFHack;

// applyAssignWorkDetail adds (add=true) or removes (add=false) unitID
// from the assigned_units of work_details[detailIndex]. Mirrors
// manipulator.lua's toggle_sorted_vec + setAutomaticProfessions pair
// exactly: assigned_units is DF's own binary-sorted vector, kept sorted
// the same way DF's own toggle does via insert_into_vector/
// erase_from_vector (MiscUtils.h) instead of an unsorted push_back+erase.
bool applyAssignWorkDetail(uint16_t detailIndex, int32_t unitID, bool add, std::string &error)
{
    if (!df::global::plotinfo) {
        error = "plotinfo is null";
        return false;
    }

    auto &details = df::global::plotinfo->labor_info.work_details;
    if (detailIndex >= details.size() || !details[detailIndex]) {
        error = "no work detail at index " + std::to_string(detailIndex);
        return false;
    }

    df::unit *u = df::unit::find(unitID);
    if (!u) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }

    df::work_detail *wd = details[detailIndex];
    bool changed;
    if (add) {
        insert_into_vector(wd->assigned_units, (int32_t)unitID, &changed);
    } else {
        changed = erase_from_vector(wd->assigned_units, (int32_t)unitID);
    }

    // Recompute this unit's derived status.labors from ALL its work
    // detail memberships -- exactly what manipulator.lua's toggle does
    // after every membership edit.
    Units::setAutomaticProfessions(u);

    error = "unit#" + std::to_string(unitID) + (add ? " added to" : " removed from") +
            " work detail '" + wd->name + "' (index " + std::to_string(detailIndex) + ")" +
            (changed ? "" : std::string(" (already ") + (add ? "assigned" : "unassigned") + ")");
    return true;
}

// applySetWorkDetailMode changes work_details[detailIndex]'s mode
// (work_detail_flags.bits.mode -- a 2-bit subfield of the flags bitfield,
// df.plotinfo.xml) and recomputes derived labors for EVERY current fort
// citizen via Units::setAutomaticProfessions -- a mode change reshuffles
// who does what FORT-WIDE (e.g. EverybodyDoesThis pulls in citizens never
// explicitly added to assigned_units), not just the previously-assigned
// membership list, so a narrower "only touch assigned_units" recompute
// would miss exactly the citizens a mode change is meant to affect.
bool applySetWorkDetailMode(uint16_t detailIndex, uint8_t mode, std::string &error)
{
    if (!df::global::plotinfo) {
        error = "plotinfo is null";
        return false;
    }

    auto &details = df::global::plotinfo->labor_info.work_details;
    if (detailIndex >= details.size() || !details[detailIndex]) {
        error = "no work detail at index " + std::to_string(detailIndex);
        return false;
    }
    if (mode > (uint8_t)df::work_detail_mode::OnlySelectedDoesThis) {
        error = "invalid work detail mode " + std::to_string((unsigned)mode);
        return false;
    }

    df::work_detail *wd = details[detailIndex];
    wd->flags.bits.mode = (df::work_detail_mode)mode;

    int recomputed = 0;
    for (auto *unit : df::global::world->units.active) {
        if (!unit || !Units::isCitizen(unit)) continue;
        Units::setAutomaticProfessions(unit);
        recomputed++;
    }

    error = "work detail '" + wd->name + "' (index " + std::to_string(detailIndex) +
            ") mode set to " + ENUM_KEY_STR(work_detail_mode, wd->flags.bits.mode) +
            "; recomputed labors for " + std::to_string(recomputed) + " citizens";
    return true;
}

// applyCreateWorkDetail allocates a new custom work detail into the first
// free CUSTOM_1..CUSTOM_8 icon slot (df::work_detail_icon_type -- the only
// icons legal for a NEW detail; the ten vanilla-category icons and
// SIEGE_OPERATORS belong to details DF itself creates at fort founding)
// and appends it to plotinfo->labor_info.work_details. No script or
// plugin in this checkout creates a work detail from scratch
// (manipulator.lua only edits ones DF already created) -- allocation here
// follows this codebase's own df::allocate<T>() convention instead
// (Buildings.cpp, Items.cpp, Military.cpp all construct new DF-owned
// structs this way), since no bay12-original creation path exists to
// port.
bool applyCreateWorkDetail(const std::string &name, uint8_t mode,
                            const std::vector<uint8_t> &laborIDs, std::string &error)
{
    if (!df::global::plotinfo) {
        error = "plotinfo is null";
        return false;
    }
    if (name.empty()) {
        error = "work detail name must not be empty";
        return false;
    }
    if (mode > (uint8_t)df::work_detail_mode::OnlySelectedDoesThis) {
        error = "invalid work detail mode " + std::to_string((unsigned)mode);
        return false;
    }
    // Validate every labor id BEFORE allocating anything -- mirrors
    // applySetLabor's bounds-check-before-write guard (df_ai_protocol.cpp)
    // and avoids allocating a work_detail we'd otherwise discard on a
    // rejected labor id.
    for (uint8_t laborID : laborIDs) {
        if (laborID > LABOR_MAX_INDEX) {
            error = "labor id " + std::to_string((unsigned)laborID) +
                    " out of range (0-" + std::to_string((unsigned)LABOR_MAX_INDEX) + ")";
            return false;
        }
    }

    auto &details = df::global::plotinfo->labor_info.work_details;

    bool iconUsed[8] = {false, false, false, false, false, false, false, false};
    for (auto *wd : details) {
        if (!wd) continue;
        int slot = (int)wd->icon - (int)df::work_detail_icon_type::CUSTOM_1;
        if (slot >= 0 && slot < 8) iconUsed[slot] = true;
    }
    int freeSlot = -1;
    for (int i = 0; i < 8; i++) {
        if (!iconUsed[i]) { freeSlot = i; break; }
    }
    if (freeSlot < 0) {
        error = "no free custom work detail slot (all 8 CUSTOM_1..CUSTOM_8 in use)";
        return false;
    }

    df::work_detail *wd = df::allocate<df::work_detail>();
    if (!wd) {
        error = "failed to allocate work_detail";
        return false;
    }
    wd->name = name;
    wd->flags.bits.mode = (df::work_detail_mode)mode;
    wd->icon = (df::work_detail_icon_type)((int)df::work_detail_icon_type::CUSTOM_1 + freeSlot);
    for (uint8_t laborID : laborIDs) {
        wd->allowed_labors[laborID] = true;
    }

    details.push_back(wd);
    int index = (int)details.size() - 1;

    // Recompute derived labors for EVERY citizen, same as
    // applySetWorkDetailMode above and for the same reason: a brand-new
    // detail is a fort-wide membership/mode event. An EverybodyDoesThis
    // detail pulls in citizens who are in nobody's assigned_units, and even a
    // Default one changes which labors DF's automatic-profession pass may
    // hand out -- without this, every citizen's status.labors cache still
    // reflects the pre-creation world, and the manager_orders missing-labor
    // rung (queries.cpp handleManagerOrders, which reads status.labors) would
    // keep reporting the labor as held by nobody immediately after a model
    // created the very detail that fixes it.
    int recomputed = 0;
    if (df::global::world) {
        for (auto *unit : df::global::world->units.active) {
            if (!unit || !Units::isCitizen(unit)) continue;
            Units::setAutomaticProfessions(unit);
            recomputed++;
        }
    }

    error = "work detail '" + name + "' created at index " + std::to_string(index) +
            " (icon " + ENUM_KEY_STR(work_detail_icon_type, wd->icon) + ")" +
            "; recomputed labors for " + std::to_string(recomputed) + " citizens";
    return true;
}
