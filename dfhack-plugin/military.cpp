// dfhack-plugin/military.cpp
//
// Minimal DF v50 military capability: create a squad, staff it, and give
// it one of two direct orders (or clear its orders). See docs/decisions.md
// (2026-07-19 military research pass) for the full data-model citation
// this file implements against. Every risky step below repeats its
// UNVERIFIED flag inline, per this project's house rule of documenting
// real limitations instead of papering over them:
//
//   1. applyCreateSquad's assignment-minting path (reachable ONLY when
//      the caller names position_code explicitly -- the default-selection
//      path never mints): no DFHack code anywhere in this checkout mints
//      a brand-new entity_position_assignment. The recipe here is
//      inferred from the struct shape and df::create_squad_interfacest's
//      own candidate-list field (proving the closed DF binary treats
//      "mint a fresh assignment from this position template" as a real,
//      distinct step) -- NOT confirmed against any known-working DFHack
//      script. Needs a live check (DF's own Squads/Nobles screen) before
//      this path is fully trusted; the success ACK says so explicitly.
//   2. applySquadOrder's Station case (squad_order_movest): the
//      pos/point_id field pairing with no saved Notes-screen waypoint is
//      inferred from field shape only -- no code in this checkout ever
//      constructs one.
//   3. squad->orders' multi-entry processing order/lifecycle is unknown
//      (no code here reads or writes that vector outside construction)
//      -- mitigated by always clearing the queue before pushing exactly
//      one new order, never depending on queue semantics.
//
// CORRECTED 2026-08-07 (this comment previously claimed addToSquad
// "mirrors proven-safe DFHack patterns 1:1" and, in message.go, that it
// "auto-picks the first free NON-commander slot" -- that misreading IS the
// root cause of assign_squad failing on every squad this plugin created):
// Military::addToSquad is NEVER called here with its default
// squad_pos == -1. That auto-pick path is broken for any freshly made
// squad. Military.cpp's loop (`for (int p = 0; p < 10; p++)`) selects
// p == 0 the instant the commander slot is vacant -- which is ALWAYS true
// on a squad Military::makeSquad just built, since makeSquad allocates
// every squad_position but never writes positions[0].occupant (the
// codegen ctor leaves it at -1) -- and the very next line then refuses
// squad_pos == 0 outright. DFHack's own docs state this failure mode in
// prose: docs/dev/Lua API.rst's addToSquad entry says it "will fail if
// squad_pos is specified as 0 or if squad_pos is specified as -1 and the
// squad leader position is currently vacant". DFHack's only real caller,
// scripts/autotraining.lua, dodges it by looping i=1..9 and ALWAYS
// passing an explicit index -- that explicit-index call, not the -1
// default, is the pattern this file now mirrors: applyAssignSquad
// computes the slot itself via findFreeNonCommanderSlot (bounded by the
// squad's real positions.size(), never a hardcoded 10) and passes it.
//
// Still accurate, and still the basis for the rest of this file:
// Military::removeFromSquad and Military::makeSquad are DFHack's own
// module implementations (makeSquad additionally pushes the new squad
// into BOTH fort->squads and world->squads.all unconditionally), and the
// defend-burrow order's burrows list carries real in-tree precedent.
//
// STILL UNSOLVED -- do not read this file as "military is fixed": nothing
// here can fill a squad's COMMANDER slot (position 0). Across the whole
// DFHack checkout exactly two writers of squad_position::occupant exist,
// addToSquad (which refuses index 0) and removeFromSquad (which clears
// it), so whether DF's own per-tick simulation binds the leader on its
// own after ticks run is UNVERIFIED. A live tick-test must answer that
// before any commander-binding write is attempted here; assign_squad's
// full-squad ACK says so out loud rather than implying the slot is
// reachable.
//
// Deliberately OUT OF SCOPE (see docs/decisions.md for the full
// reasoning):
//   - Training schedules (squad_schedulest month-by-month editing) --
//     Military::makeSquad already builds a syntactically valid default
//     (empty/idle, or templated off whatever alert routines already
//     exist) schedule on its own; a full 12-month/per-position editor is
//     a separate, much deeper workstream.
//   - Uniforms (squad_position_equipmentst) -- confirmed unremarkable
//     with none assigned by scripts/uniform-unstick.lua's own treatment
//     of an empty uniform as expected, not an error, state. Soldiers
//     fight with whatever they already carry/wear as civilians.
//   - Patrol routes (patrol_routes_interfacest) -- a whole separate
//     named-point/named-route sub-editor.
//   - Kill-list/kill-hf orders -- need pre-known hostile unit/histfig
//     ids this plugin has no discovery tool for yet.
//   - The raid/drive-off/rescue/retrieve site-leaving order family --
//     these carry their own exit_point field and drive a squad off-map
//     under an army_controller, entirely unlike chokepoint defense; see
//     df.squad.xml's squad_order subtype list.
//   - Work-detail/labor interaction -- v50 does NOT auto-toggle civilian
//     labors off when a unit joins a squad (uniform-unstick.lua warns
//     about exactly this conflict); use the existing set_labor tool.

#include "Core.h"
#include "modules/Maps.h"
#include "modules/Military.h"
#include "modules/Burrows.h"

#include "df/world.h"
#include "df/plotinfost.h"
#include "df/unit.h"
#include "df/historical_entity.h"
#include "df/entity_position.h"
#include "df/entity_position_flags.h"
#include "df/entity_position_assignment.h"
#include "df/squad.h"
#include "df/squad_position.h"
#include "df/squad_order.h"
#include "df/squad_order_movest.h"
#include "df/squad_order_defend_burrowsst.h"
#include "df/burrow.h"
#include "df/coord.h"
#include "df/global_objects.h"

#include "protocol.h"

#include <map>
#include <string>
#include <vector>

using namespace DFHack;

// findPositionAndVacantAssignment scans `entity`'s positions.own for an
// entity_position with code == positionCode, then (if found)
// positions.assignments for a slot naming that position with
// squad_id == -1 (vacant). Returns false ONLY when the position code
// itself isn't found -- a genuinely wrong code, so no mint is attempted.
// An empty *outAssignment with a non-null *outPosition means the position
// exists but has no vacant (or any) assignment slot yet -- the fresh-embark
// case applyCreateSquad's minting path exists for.
static bool findPositionAndVacantAssignment(df::historical_entity *entity, const std::string &positionCode,
                                             df::entity_position **outPosition,
                                             df::entity_position_assignment **outAssignment)
{
    *outPosition = nullptr;
    *outAssignment = nullptr;
    for (auto *p : entity->positions.own) {
        if (p && p->code == positionCode) {
            *outPosition = p;
            break;
        }
    }
    if (!*outPosition) return false;

    for (auto *a : entity->positions.assignments) {
        if (a && a->position_id == (*outPosition)->id && a->squad_id == -1) {
            *outAssignment = a;
            break;
        }
    }
    return true;
}

// selectDefaultSquadLeaderPosition picks which entity_position a new squad
// should be led by when the caller named none. It replaces an older
// hardcoded "MILITIA_CAPTAIN" default that silently fell through to the
// UNVERIFIED minting path (file comment, risk item 1) on young forts,
// where MILITIA_CAPTAIN typically has no assignment slot yet while
// MILITIA_COMMANDER already does.
//
// Preference order (both tiers require squad_size > 0, i.e. the position
// can actually lead a squad):
//   1. a position with an assignment whose holder is appointed
//      (histfig >= 0) AND which is not already leading a squad
//      (squad_id == -1) -- ready to go, nothing to mint.
//   2. a position with any assignment not already leading a squad,
//      appointed or not -- still avoids the mint path.
// Returns false when neither tier matches, filling outWhy with an
// actionable roll-call of every squad_size > 0 position seen and its
// population-gate state, so applyCreateSquad can refuse truthfully
// instead of minting behind the caller's back.
//
// UNVERIFIED preference ordering: DF's own vanilla precedence semantics
// for competing squad-leader positions are not modelled here.
// entity_position::precedence exists, but nothing in this checkout (DF-AI
// or DFHack) sorts by it, so it is deliberately NOT used as a tiebreak --
// with two simultaneously-eligible positions the pick may differ from
// what a human would choose in DF's own UI. Likewise, the observation
// motivating this helper (that MILITIA_COMMANDER unlocks before
// MILITIA_CAPTAIN on a young fort) is general DF community knowledge plus
// one live observation, NOT confirmed against this checkout's bundled
// raws -- which is exactly why this scans runtime assignment state rather
// than hardcoding any position code's unlock order.
static bool selectDefaultSquadLeaderPosition(df::historical_entity *fort,
                                             std::string &outCode,
                                             std::string &outReason,
                                             std::string &outWhy)
{
    df::entity_position *ready = nullptr;    // tier 1: appointed holder, no squad yet
    df::entity_position *unbound = nullptr;  // tier 2: assignment exists, no squad yet
    std::string rollCall;
    int leaderPositions = 0;

    for (auto *p : fort->positions.own) {
        if (!p || p->squad_size <= 0) continue;
        leaderPositions++;

        int slots = 0, freeSlots = 0, readySlots = 0;
        for (auto *a : fort->positions.assignments) {
            if (!a || a->position_id != p->id) continue;
            slots++;
            if (a->squad_id != -1) continue;
            freeSlots++;
            if (a->histfig >= 0) readySlots++;
        }

        if (!rollCall.empty()) rollCall += "; ";
        rollCall += "'" + p->code + "' (squad_size=" + std::to_string(p->squad_size) +
                    ", assignment slots=" + std::to_string(slots) +
                    ", not-yet-leading-a-squad=" + std::to_string(freeSlots) +
                    ", requires_population=" + std::to_string(p->requires_population) +
                    ", has_met_pop_req=" +
                    (p->flags.is_set(df::entity_position_flags::HAS_MET_POP_REQ) ? "true" : "false") + ")";

        if (readySlots > 0 && !ready) ready = p;
        if (freeSlots > 0 && !unbound) unbound = p;
    }

    if (ready) {
        outCode = ready->code;
        outReason = " -- position auto-selected (no position_code given): it already has an "
                    "appointed holder and an assignment slot not yet leading a squad, so no "
                    "assignment had to be minted";
        return true;
    }
    if (unbound) {
        outCode = unbound->code;
        outReason = " -- position auto-selected (no position_code given): it has an assignment "
                    "slot not yet leading a squad, but NO appointed holder yet (use "
                    "appoint_position); no assignment had to be minted";
        return true;
    }

    if (leaderPositions == 0) {
        outWhy = "this fort's entity defines no squad-leader position at all (none with "
                 "squad_size > 0) -- nothing to create a squad under; inspect position_vacancies";
        return false;
    }
    outWhy = "no squad-leader position on this fort has an assignment slot available to lead a "
             "new squad, so create_squad refuses rather than minting one behind your back "
             "(minting is UNVERIFIED -- see military.cpp risk item 1). Positions seen: " +
             rollCall +
             ". Wait for the population gate (requires_population vs has_met_pop_req above) to "
             "unlock a slot, free an existing squad's leader assignment, or pass position_code "
             "explicitly to take the UNVERIFIED mint path deliberately";
    return false;
}

// applyCreateSquad fills (or, only when positionCodeIn is explicitly
// given, mints -- see file comment, risk item 1) a vacant
// entity_position_assignment for positionCode, then calls
// Military::makeSquad on it. An empty positionCodeIn no longer defaults to
// a hardcoded "MILITIA_CAPTAIN"; it runs selectDefaultSquadLeaderPosition
// instead, which either names a position that already has a usable
// assignment slot or refuses with an actionable roll-call.
bool applyCreateSquad(const std::string &positionCodeIn, std::string &error)
{
    if (!df::global::world || !df::global::plotinfo) {
        error = "world or plotinfo is null";
        return false;
    }

    df::historical_entity *fort = df::historical_entity::find(df::global::plotinfo->group_id);
    if (!fort) {
        error = "fort entity (plotinfo->group_id) not found";
        return false;
    }

    std::string positionCode = positionCodeIn;
    std::string autoReason;
    if (positionCode.empty()) {
        std::string why;
        if (!selectDefaultSquadLeaderPosition(fort, positionCode, autoReason, why)) {
            error = why;
            return false;
        }
    }

    df::entity_position *position = nullptr;
    df::entity_position_assignment *assignment = nullptr;
    if (!findPositionAndVacantAssignment(fort, positionCode, &position, &assignment)) {
        error = "no position with code '" + positionCode + "' found on the fort entity "
                "(discover valid codes via the position_vacancies query -- a squad-leader "
                "position carries squad_size > 0)";
        return false;
    }
    if (position->squad_size <= 0) {
        error = "position '" + positionCode + "' cannot lead a squad (squad_size <= 0)";
        return false;
    }

    bool minted = false;
    if (!assignment) {
        // UNVERIFIED, highest risk -- see file comment, risk item 1. The
        // codegen default constructor already sets histfig/histfig2/
        // squad_id/position_vector_idx to -1 (confirmed against
        // static.ctors.inc), so only id and position_id need setting here
        // -- position_vector_idx is deliberately left at that -1 default,
        // mirroring nobles.cpp's identical precedent for
        // histfig_entity_link_positionst::entity_vector_idx.
        assignment = new df::entity_position_assignment();
        assignment->id = fort->positions.next_assignment_id++;
        assignment->position_id = position->id;
        fort->positions.assignments.push_back(assignment);
        minted = true;
    }

    df::squad *squad = Military::makeSquad(assignment->id);
    if (!squad) {
        error = "Military::makeSquad failed for assignment#" + std::to_string(assignment->id) +
                " (position '" + positionCode + "') -- the assignment may already lead a squad";
        return false;
    }

    error = "squad #" + std::to_string(squad->id) + " created, led by position '" + positionCode +
            "' (" + position->name[0] + "), " + std::to_string(squad->positions.size()) +
            " total slots, leader (position 0) vacant -- this plugin cannot fill the commander "
            "slot and it is UNVERIFIED whether DF's own simulation binds it on its own; staff "
            "positions 1+ with assign_squad" + autoReason;
    if (minted) {
        error += " -- NOTE: this position had no assignment slot yet, so one was newly minted; "
                 "this exact write has no DFHack precedent in this checkout and is UNVERIFIED -- "
                 "confirm the squad looks correct on DF's own Squads screen before relying on it";
    }
    return true;
}

// findFreeNonCommanderSlot returns the index of squad's first vacant
// NON-commander position, or -1 if every soldier slot is occupied (or the
// squad has no soldier slot at all). *outFilled/*outTotal report the
// occupied/total SOLDIER slot counts (index 0, the commander, is excluded
// from both) so callers can fail with a truthful fullness message.
//
// This exists to replace Military::addToSquad's own squad_pos == -1
// auto-pick, which cannot work on a squad this plugin created -- see the
// file header comment for the full mechanism and DFHack's own
// documentation of the failure.
//
// MEMORY SAFETY, deliberate divergence from upstream: this loop is bounded
// by squad->positions.size(), NOT by a hardcoded 10 the way DFHack's own
// auto-pick loop is. That upstream bound is a latent out-of-bounds risk
// for any squad whose squad_size (and therefore positions.size(), fixed
// once in makeSquad and never resized elsewhere in this checkout) is under
// 10: vector_get returns nullptr past the end, which upstream cannot
// distinguish from a legitimately vacant slot, and its follow-up
// `squad->positions[squad_pos] = ...` write then indexes past the end.
// Fixing that is upstream's business; not importing the pattern is ours.
// Every index this function returns is < positions.size() by
// construction, so the explicit squad_pos we hand addToSquad only ever
// reaches its already-safe path.
static int32_t findFreeNonCommanderSlot(df::squad *squad, int32_t *outFilled, int32_t *outTotal)
{
    int32_t filled = 0;
    int32_t freeIdx = -1;
    size_t count = squad->positions.size();
    for (size_t i = 1; i < count; i++) {
        df::squad_position *pos = squad->positions[i];
        if (pos && pos->occupant != -1) {
            filled++;
            continue;
        }
        if (freeIdx < 0) freeIdx = (int32_t)i;
    }
    if (outFilled) *outFilled = filled;
    if (outTotal) *outTotal = (count > 0) ? (int32_t)(count - 1) : 0;
    return freeIdx;
}

// applyAssignSquad adds (add=true) or removes (add=false) unitID from
// squadID's membership via DFHack's own Military::addToSquad/
// removeFromSquad. On add, the squad_pos is computed HERE
// (findFreeNonCommanderSlot) and passed explicitly -- mirroring
// scripts/autotraining.lua's real, working call pattern rather than
// addToSquad's broken -1 default. See the file header comment.
bool applyAssignSquad(int32_t squadID, int32_t unitID, bool add, std::string &error)
{
    df::squad *squad = df::squad::find(squadID);
    if (!squad) {
        error = "no squad with id " + std::to_string(squadID);
        return false;
    }
    df::unit *unit = df::unit::find(unitID);
    if (!unit) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }

    if (add) {
        if (unit->military.squad_id != -1) {
            error = "unit#" + std::to_string(unitID) + " is already in squad #" +
                    std::to_string(unit->military.squad_id) + " -- remove it first";
            return false;
        }
        int32_t filled = 0, total = 0;
        int32_t squadPos = findFreeNonCommanderSlot(squad, &filled, &total);
        if (squadPos < 0) {
            error = "squad #" + std::to_string(squadID) + " has no free soldier slot (" +
                    std::to_string(filled) + "/" + std::to_string(total) +
                    " non-commander slots filled). Position 0 is the commander slot: DFHack's "
                    "Military::addToSquad refuses it outright, and this plugin implements no "
                    "commander-binding write at all (UNVERIFIED whether DF's own simulation "
                    "fills it on its own) -- remove a member (add=false) or create another squad";
            return false;
        }
        if (!Military::addToSquad(unitID, squadID, squadPos)) {
            error = "Military::addToSquad failed for unit#" + std::to_string(unitID) +
                    " -> squad #" + std::to_string(squadID) + " at position " +
                    std::to_string(squadPos) +
                    " (the unit has no historical figure, or the squad's state changed under "
                    "this call -- the slot was vacant when this plugin picked it)";
            return false;
        }
        error = "unit#" + std::to_string(unitID) + " assigned to squad #" + std::to_string(squadID) +
                " at position " + std::to_string(unit->military.squad_position) +
                " -- civilian labors are NOT auto-disabled (v50 does not do this; "
                "use set_labor if a dedicated, non-working soldier is wanted)";
        return true;
    }

    if (unit->military.squad_id != squadID) {
        error = "unit#" + std::to_string(unitID) + " is not currently in squad #" + std::to_string(squadID);
        return false;
    }
    if (!Military::removeFromSquad(unitID)) {
        error = "Military::removeFromSquad failed for unit#" + std::to_string(unitID);
        return false;
    }
    error = "unit#" + std::to_string(unitID) + " removed from squad #" + std::to_string(squadID) +
            " (returned to civilian duty)";
    return true;
}

// clearSquadOrders deletes and clears every existing entry in
// squad->orders -- mirrors Military.cpp's own room-removal deletion idiom
// (delete each pointer, then erase/clear the vector) and sidesteps the
// UNVERIFIED question of squad->orders' multi-entry processing (see file
// comment, risk item 3) by always reducing the queue to at most one order.
static void clearSquadOrders(df::squad *squad)
{
    for (auto *order : squad->orders) {
        delete order;
    }
    squad->orders.clear();
}

// applySquadOrder replaces squadID's entire orders queue with at most one
// order (station or defend-burrow), or clears it entirely (cancel). See
// SQUAD_ORDER_* constants and this file's header comment for the
// UNVERIFIED flags on the station path.
bool applySquadOrder(int32_t squadID, uint8_t orderType, int16_t x, int16_t y, int16_t z,
                      const std::string &burrowName, std::string &error)
{
    df::squad *squad = df::squad::find(squadID);
    if (!squad) {
        error = "no squad with id " + std::to_string(squadID);
        return false;
    }

    clearSquadOrders(squad);

    if (orderType == SQUAD_ORDER_CANCEL) {
        error = "squad #" + std::to_string(squadID) +
                " orders cleared (returns to its default schedule/idle behavior)";
        return true;
    }

    if (orderType == SQUAD_ORDER_STATION) {
        if (!Maps::isValidTilePos(x, y, z)) {
            error = "coordinates out of map bounds";
            return false;
        }
        // UNVERIFIED -- see file comment, risk item 2.
        df::squad_order_movest *order = df::allocate<df::squad_order_movest>();
        order->year = *df::global::cur_year;
        order->year_tick = *df::global::cur_year_tick;
        order->pos = df::coord(x, y, z);
        order->point_id = -1; // no associated saved Notes-screen waypoint
        squad->orders.push_back(order);
        error = "squad #" + std::to_string(squadID) + " ordered to station at (" +
                std::to_string(x) + "," + std::to_string(y) + "," + std::to_string(z) +
                ") -- UNVERIFIED order type (no DFHack code in this checkout ever constructs "
                "a squad_order_movest); confirm the squad actually moves there before relying on it";
        return true;
    }

    if (orderType == SQUAD_ORDER_DEFEND_BURROW) {
        if (burrowName.empty()) {
            error = "squad_order defend_burrow requires a non-empty burrow name (see designate_burrow)";
            return false;
        }
        df::burrow *b = Burrows::findByName(burrowName);
        if (!b) {
            error = "no burrow named '" + burrowName + "' -- create one via designate_burrow first";
            return false;
        }
        df::squad_order_defend_burrowsst *order = df::allocate<df::squad_order_defend_burrowsst>();
        order->year = *df::global::cur_year;
        order->year_tick = *df::global::cur_year_tick;
        order->burrows.push_back(b->id);
        squad->orders.push_back(order);
        error = "squad #" + std::to_string(squadID) + " ordered to defend burrow '" + burrowName + "'";
        return true;
    }

    error = "unknown squad order type";
    return false;
}

// ---------------------------------------------------------------------------
// Read side: handleListSquads is declared here (uses df::squad/Military
// types this file already includes) but dispatched from queries.cpp's
// executeQuery, same forward-declared-elsewhere pattern as burrows.cpp's
// handleListBurrows. jsonStr/jsonInt mirror queries.cpp's tiny hand-rolled
// JSON writer (no shared header for it in this plugin, per burrows.cpp's
// own local-copy precedent) -- suffixed *Squad to avoid any reader
// confusion with the same-named statics in other files (internal linkage
// means no actual link collision either way).
// ---------------------------------------------------------------------------

static std::string jsonEscapeSquad(const std::string &s) {
    std::string out;
    out.reserve(s.size() + 2);
    for (char c : s) {
        switch (c) {
            case '"':  out += "\\\""; break;
            case '\\': out += "\\\\"; break;
            case '\n': out += "\\n"; break;
            case '\r': out += "\\r"; break;
            case '\t': out += "\\t"; break;
            default:
                if ((unsigned char)c < 0x20) {
                    char buf[8];
                    snprintf(buf, sizeof(buf), "\\u%04x", c);
                    out += buf;
                } else {
                    out += c;
                }
        }
    }
    return out;
}
static std::string jsonStrSquad(const std::string &s) { return "\"" + jsonEscapeSquad(s) + "\""; }
static std::string jsonIntSquad(int64_t v) { char buf[32]; snprintf(buf, sizeof(buf), "%lld", (long long)v); return std::string(buf); }

// handleListSquads reports THIS FORT'S squads -- world->squads.all is a
// WORLD-wide vector that also holds every other historical entity's squads
// (hostile civs, visiting groups), so it is filtered by
// squad->entity_id == plotinfo->group_id, exactly as DFHack's own
// scripts/autotraining.lua does before touching a squad. Without that
// filter, foreign squads' member histfigs never resolve against this
// fort's units.active map and every member renders as unit#-1. entity_id
// is still emitted per squad for debugging.
//
// Per squad it reports: id, DF's own
// display name (Military::getSquadName -- alias-or-translated-name
// lookup, a proven-safe DFHack call with real callers in
// scripts/fix/stuck-squad.lua), membership (position index, commander
// flag, vacancy, and the occupying UNIT id -- squad_position::occupant is
// a HISTORICAL FIGURE id, so this resolves it back to a live unit the same
// way queries.cpp's handlePositionVacancies does, via a linear scan of
// units.active built once into a lookup map), and a summary of the
// squad's current orders (recognizing only the two order types this
// plugin can construct -- station/defend_burrow -- everything else is
// reported generically as "other" rather than guessed at).
std::string handleListSquads(const std::string & /*args*/, uint8_t &status)
{
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return "{\"error\":\"world or plotinfo is null\"}";
    }
    const int32_t fortEntityID = df::global::plotinfo->group_id;

    std::map<int32_t, int32_t> histfigToUnit;
    for (auto *u : df::global::world->units.active) {
        if (u && u->hist_figure_id >= 0) histfigToUnit[u->hist_figure_id] = u->id;
    }

    std::string out = "{\"squads\":[";
    bool first = true;
    for (auto *squad : df::global::world->squads.all) {
        if (!squad) continue;
        if (squad->entity_id != fortEntityID) continue; // other entities' squads -- not ours
        if (!first) out += ",";
        first = false;

        out += "{\"id\":" + jsonIntSquad(squad->id) +
               ",\"name\":" + jsonStrSquad(Military::getSquadName(squad->id)) +
               ",\"entity_id\":" + jsonIntSquad(squad->entity_id);

        out += ",\"members\":[";
        for (size_t i = 0; i < squad->positions.size(); i++) {
            if (i) out += ",";
            auto *pos = squad->positions[i];
            int32_t histfig = pos ? pos->occupant : -1;
            int32_t unitID = -1;
            if (histfig >= 0) {
                auto it = histfigToUnit.find(histfig);
                if (it != histfigToUnit.end()) unitID = it->second;
            }
            out += "{\"position_index\":" + jsonIntSquad((int64_t)i) +
                   ",\"is_commander\":" + (i == 0 ? "true" : "false") +
                   ",\"vacant\":" + (histfig < 0 ? "true" : "false") +
                   ",\"unit_id\":" + jsonIntSquad(unitID) + "}";
        }
        out += "]";

        out += ",\"orders\":[";
        bool firstOrder = true;
        for (auto *order : squad->orders) {
            if (!order) continue;
            if (!firstOrder) out += ",";
            firstOrder = false;

            auto *move = strict_virtual_cast<df::squad_order_movest>(order);
            auto *defend = strict_virtual_cast<df::squad_order_defend_burrowsst>(order);
            if (move) {
                out += "{\"type\":\"station\",\"x\":" + jsonIntSquad(move->pos.x) +
                       ",\"y\":" + jsonIntSquad(move->pos.y) +
                       ",\"z\":" + jsonIntSquad(move->pos.z) + "}";
            } else if (defend) {
                out += "{\"type\":\"defend_burrow\",\"burrows\":[";
                for (size_t j = 0; j < defend->burrows.size(); j++) {
                    if (j) out += ",";
                    int32_t bid = defend->burrows[j];
                    df::burrow *b = df::burrow::find(bid);
                    out += "{\"id\":" + jsonIntSquad(bid) +
                           ",\"name\":" + jsonStrSquad(b ? b->name : "") + "}";
                }
                out += "]}";
            } else {
                out += "{\"type\":\"other\"}";
            }
        }
        out += "]}";
    }
    out += "]}";
    status = QUERY_STATUS_SUCCESS;
    return out;
}
