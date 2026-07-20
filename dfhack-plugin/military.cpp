// dfhack-plugin/military.cpp
//
// Minimal DF v50 military capability: create a squad, staff it, and give
// it one of two direct orders (or clear its orders). See docs/decisions.md
// (2026-07-19 military research pass) for the full data-model citation
// this file implements against. Every risky step below repeats its
// UNVERIFIED flag inline, per this project's house rule of documenting
// real limitations instead of papering over them:
//
//   1. applyCreateSquad's assignment-minting path: no DFHack code
//      anywhere in this checkout mints a brand-new
//      entity_position_assignment. The recipe here is inferred from the
//      struct shape and df::create_squad_interfacest's own candidate-list
//      field (proving the closed DF binary treats "mint a fresh
//      assignment from this position template" as a real, distinct
//      step) -- NOT confirmed against any known-working DFHack script.
//      Needs a live check (DF's own Squads/Nobles screen) before this
//      path is fully trusted; the success ACK says so explicitly.
//   2. applySquadOrder's Station case (squad_order_movest): the
//      pos/point_id field pairing with no saved Notes-screen waypoint is
//      inferred from field shape only -- no code in this checkout ever
//      constructs one.
//   3. squad->orders' multi-entry processing order/lifecycle is unknown
//      (no code here reads or writes that vector outside construction)
//      -- mitigated by always clearing the queue before pushing exactly
//      one new order, never depending on queue semantics.
//
// Everything else here -- Military::addToSquad/removeFromSquad, the
// defend-burrow order's burrows list, and (per Military.cpp's own source)
// makeSquad itself -- mirrors proven-safe DFHack patterns 1:1 (real
// callers in scripts/autotraining.lua for the first two; Military.cpp is
// DFHack's own module implementation for the third).
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

// applyCreateSquad fills (or mints -- see file comment, risk item 1) a
// vacant entity_position_assignment for positionCode ("" defaults to
// "MILITIA_CAPTAIN", the position vanilla DF's own [SQUAD:...] raw token
// attaches to -- confirmed against the live Steam install's
// data/vanilla/vanilla_entities/objects/entity_default.txt [ENTITY:MOUNTAIN]
// block), then calls Military::makeSquad on it.
bool applyCreateSquad(const std::string &positionCodeIn, std::string &error)
{
    if (!df::global::world || !df::global::plotinfo) {
        error = "world or plotinfo is null";
        return false;
    }
    std::string positionCode = positionCodeIn.empty() ? "MILITIA_CAPTAIN" : positionCodeIn;

    df::historical_entity *fort = df::historical_entity::find(df::global::plotinfo->group_id);
    if (!fort) {
        error = "fort entity (plotinfo->group_id) not found";
        return false;
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
            " total slots, leader vacant";
    if (minted) {
        error += " -- NOTE: this position had no assignment slot yet, so one was newly minted; "
                 "this exact write has no DFHack precedent in this checkout and is UNVERIFIED -- "
                 "confirm the squad looks correct on DF's own Squads screen before relying on it";
    }
    return true;
}

// applyAssignSquad adds (add=true) or removes (add=false) unitID from
// squadID's membership via DFHack's own Military::addToSquad/
// removeFromSquad -- both proven-safe (real callers in
// scripts/autotraining.lua at this exact tag).
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
        if (!Military::addToSquad(unitID, squadID)) {
            error = "Military::addToSquad failed for unit#" + std::to_string(unitID) +
                    " -> squad #" + std::to_string(squadID) +
                    " (squad may be full, or the unit has no historical figure)";
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

// handleListSquads reports every squad in world->squads.all: id, DF's own
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
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return "{\"error\":\"world is null\"}";
    }

    std::map<int32_t, int32_t> histfigToUnit;
    for (auto *u : df::global::world->units.active) {
        if (u && u->hist_figure_id >= 0) histfigToUnit[u->hist_figure_id] = u->id;
    }

    std::string out = "{\"squads\":[";
    bool first = true;
    for (auto *squad : df::global::world->squads.all) {
        if (!squad) continue;
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
