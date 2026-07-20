// dfhack-plugin/nobles.cpp
//
// Noble/administrator appointment: fills (or replaces the holder of) one
// entity_position_assignment slot, and writes the Bookkeeper's goal
// precision setting. See docs/decisions.md (2026-07-19 nobles research
// pass) for the full data-model writeup this file implements against, and
// queries.cpp's handlePositionVacancies (the position_vacancies query) for
// the read-side discovery surface this command is meant to be called
// after.
//
// applyAppointPosition follows the EXACT mutation sequence DFHack's own
// scripts/make-monarch.lua uses -- the closest thing DFHack ships to an
// appointment script. There is no C++ module helper for this write:
// modules/Units.h/Units.cpp and modules/Military.cpp only ever READ
// entity_position_assignment (Units::getNoblePositions, the read path
// queries.cpp's noble_demands query already uses, walks a UNIT's
// historical figure's entity_links fresh every call -- no stale cache to
// worry about on the read side once the write below is done):
//   1. Find the position by `code` in entity->positions.own -- BOTH
//      df::global::plotinfo->group_id (the fort's own historical_entity --
//      home to Manager/Bookkeeper/Broker/Sheriff/Captain of the
//      Guard/militia-squad-leader positions) and plotinfo->civ_id (the
//      parent civilization -- home to Monarch/Baron/Count/Duke succession
//      positions) are searched. Confirmed as the correct pair via
//      Units::get_units_by_noble_role (Units.cpp), which searches exactly
//      these two entities, and Military::makeSquad (Military.cpp), which
//      names plotinfo->group_id literally "fort" and reads/writes its
//      positions.assignments/positions.own directly for squad-leader
//      positions.
//   2. Find the assignment slot: linear-scan entity->positions.assignments
//      for position_id == position->id, capturing the LIVE vector index
//      (assignment_vector_idx) at write time -- never cached from an
//      earlier read, per make-monarch.lua's own comment on this point.
//   3. If already held, unlink the old holder's matching
//      histfig_entity_link_positionst from THEIR entity_links (erase +
//      delete -- entity_links is a pointer-owning vector; the erase-then-
//      delete pairing mirrors Military.cpp's remove_soldier_entity_link).
//   4. Set assignment->histfig = newfig->id.
//   5. Insert a new histfig_entity_link_positionst on the new holder's
//      entity_links (entity_vector_idx deliberately left at its -1
//      default, per make-monarch.lua precedent -- untested against this
//      checkout whether DF's own bookkeeping needs it populated).
//
// Only ever FILLS an assignment slot DF itself already created (vacant
// histfig==-1, or currently held for a replacement) -- a requested position
// code with no assignment record at all means the position hasn't unlocked
// yet (population/market threshold unmet per entity_position.
// requires_population / the REQUIRES_MARKET flag), and the command fails
// truthfully rather than fabricate one (untested whether DF accepts an
// externally-created slot's next_assignment_id/position_vector_idx
// bookkeeping correctly). Discover live vacant/appointable position codes
// via the position_vacancies query first.
//
// Gated to current citizens only (Units::isCitizen) as a safety net
// independent of raw-defined caste/allowed_creature/rejected_creature
// eligibility rules (no raw files are bundled in this checkout to verify
// those against). Justice-mechanic consequences of Sheriff/Captain of the
// Guard appointment (jailing, patrol, hammerings) are DELIBERATELY out of
// scope here -- only the appointment write itself.

#include "Core.h"
#include "Console.h"
#include "modules/Units.h"

#include "df/world.h"
#include "df/plotinfost.h"
#include "df/unit.h"
#include "df/historical_entity.h"
#include "df/historical_figure.h"
#include "df/entity_position.h"
#include "df/entity_position_assignment.h"
#include "df/histfig_entity_link.h"
#include "df/histfig_entity_link_positionst.h"
#include "df/record_precision_level_type.h"
#include "df/global_objects.h"

#include "protocol.h"

#include <string>
#include <vector>

using namespace DFHack;

bool applyAppointPosition(int32_t unitID, const std::string &positionCode, std::string &error)
{
    if (!df::global::world || !df::global::plotinfo) {
        error = "world or plotinfo is null";
        return false;
    }
    if (positionCode.empty()) {
        error = "position code must not be empty";
        return false;
    }

    df::unit *u = df::unit::find(unitID);
    if (!u) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }
    if (!Units::isCitizen(u)) {
        error = "unit#" + std::to_string(unitID) + " is not a current citizen (appointment is limited to citizens)";
        return false;
    }
    if (u->hist_figure_id < 0) {
        error = "unit#" + std::to_string(unitID) + " has no historical figure";
        return false;
    }
    df::historical_figure *newfig = df::historical_figure::find(u->hist_figure_id);
    if (!newfig) {
        error = "no historical figure #" + std::to_string(u->hist_figure_id) +
                " for unit#" + std::to_string(unitID);
        return false;
    }

    // Search both the fort's own entity (group_id) and the parent
    // civilization (civ_id) for a position with this code -- see the doc
    // comment above for why both are searched.
    int32_t entityIDs[2] = { df::global::plotinfo->group_id, df::global::plotinfo->civ_id };
    df::historical_entity *entity = nullptr;
    df::entity_position *position = nullptr;
    for (int32_t eid : entityIDs) {
        df::historical_entity *candidate = df::historical_entity::find(eid);
        if (!candidate) continue;
        for (auto *p : candidate->positions.own) {
            if (p && p->code == positionCode) {
                entity = candidate;
                position = p;
                break;
            }
        }
        if (position) break;
    }
    if (!position) {
        error = "no position with code '" + positionCode + "' found on the fort or civilization entity "
                "(discover valid codes via the position_vacancies query)";
        return false;
    }

    df::entity_position_assignment *assignment = nullptr;
    int32_t assignmentVectorIdx = -1;
    auto &assignments = entity->positions.assignments;
    for (size_t i = 0; i < assignments.size(); i++) {
        if (assignments[i] && assignments[i]->position_id == position->id) {
            assignment = assignments[i];
            assignmentVectorIdx = (int32_t)i;
            break;
        }
    }
    if (!assignment) {
        error = "position '" + positionCode + "' (" + position->name[0] + ") has no assignment slot yet -- "
                "not unlocked (population/market requirement unmet); nothing to appoint into";
        return false;
    }

    if (assignment->histfig == newfig->id) {
        error = "unit#" + std::to_string(unitID) + " already holds position '" + positionCode + "'";
        return true;
    }

    int32_t oldHistfigID = assignment->histfig;
    if (oldHistfigID >= 0) {
        df::historical_figure *oldfig = df::historical_figure::find(oldHistfigID);
        if (oldfig) {
            for (size_t i = 0; i < oldfig->entity_links.size(); i++) {
                auto *link = strict_virtual_cast<df::histfig_entity_link_positionst>(oldfig->entity_links[i]);
                if (link && link->assignment_id == assignment->id && link->entity_id == entity->id) {
                    oldfig->entity_links.erase(oldfig->entity_links.begin() + i);
                    delete link;
                    break;
                }
            }
        }
    }

    assignment->histfig = newfig->id;

    auto *newLink = df::allocate<df::histfig_entity_link_positionst>();
    newLink->entity_id = entity->id;
    newLink->assignment_id = assignment->id;
    newLink->assignment_vector_idx = assignmentVectorIdx;
    newLink->link_strength = 100;
    newLink->start_year = *df::global::cur_year;
    newfig->entity_links.push_back(newLink);

    error = "unit#" + std::to_string(unitID) + " appointed to '" + positionCode + "' (" + position->name[0] + ")" +
            (oldHistfigID >= 0
                 ? " -- replaced previous holder (histfig#" + std::to_string(oldHistfigID) + ")"
                 : "");
    return true;
}

// applySetBookkeeperPrecision writes plotinfo->nobles.bookkeeper_settings
// (df::record_precision_level_type) directly -- the same field
// plugins/stockflow.cpp already mutates (a sibling field,
// bookkeeper_precision) with no recompute call, in this exact checkout.
// UNVERIFIED: whether vanilla DF clamps/ignores a precision beyond what the
// current Bookkeeper's Appraisal skill supports (the in-game Nobles screen
// grays out unearned options; this direct write bypasses that UI gate) --
// stays truthful about what byte was written, never a guess about whether
// DF actually honors it.
bool applySetBookkeeperPrecision(uint8_t precisionByte, std::string &error)
{
    if (!df::global::plotinfo) {
        error = "plotinfo is null";
        return false;
    }
    if (precisionByte > BOOKKEEPER_PRECISION_ALL_ACCURATE) {
        error = "invalid bookkeeper precision byte " + std::to_string((unsigned)precisionByte);
        return false;
    }

    df::global::plotinfo->nobles.bookkeeper_settings = (df::record_precision_level_type)precisionByte;

    error = "bookkeeper goal precision set to " +
            std::string(ENUM_KEY_STR(record_precision_level_type, df::global::plotinfo->nobles.bookkeeper_settings));
    return true;
}
