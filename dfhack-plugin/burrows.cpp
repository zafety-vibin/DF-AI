// dfhack-plugin/burrows.cpp
//
// Burrow creation/removal/tile-paint/unit-assignment and the v50 civilian
// alert. DFHack's own Burrows module (library/modules/Burrows.cpp) covers
// naming, unit assign/clear, and tile assign/clear/mask-list -- but has NO
// burrow-creation or burrow-deletion function at all (confirmed by a full
// read of Burrows.h/.cpp in the 53.15-r2 checkout). Creation/deletion here
// mirror scripts/internal/quickfort/burrow.lua's create_burrow()/deletion
// path exactly -- the only place in this checkout that does either, and the
// pattern DFHack's own quickfort burrow feature ships in production. The
// civilian alert (applySetAlert) has no Burrows-module function either --
// it lives entirely in plotinfo->alerts, mirroring
// scripts/gui/civ-alert.lua's get_civ_alert()/sound_alarm()/clear_alarm()/
// add_civalert_burrow()/remove_civalert_burrow().
//
// All four operations mutate global game state (same class of operation as
// zones.cpp's applyDesignateZone) and must run on the executeCommand path
// (try/catch guard + socket-thread drain), never a raw thread.

#include "Core.h"
#include "Console.h"
#include "modules/Maps.h"
#include "modules/Units.h"
#include "modules/Burrows.h"

#include "df/world.h"
#include "df/plotinfost.h"
#include "df/burrow.h"
#include "df/burrow_infost.h"
#include "df/burrow_flag.h"
#include "df/block_burrow.h"
#include "df/tile_bitmask.h"
#include "df/map_block.h"
#include "df/alert_statest.h"
#include "df/alert_state_infost.h"
#include "df/unit.h"
#include "df/coord.h"

#include "protocol.h"

#include <algorithm>
#include <string>
#include <vector>

using namespace DFHack;

// applyDesignateBurrow creates the burrow named `name` if it doesn't exist
// yet (Burrows::findByName(name) here is exact/case-sensitive -- note this
// differs from scripts/internal/quickfort/burrow.lua's do_burrow(), which
// calls findByName(name, true) to ignore a trailing '+'; a burrow named
// with a trailing '+' via quickfort will NOT be found by this exact match),
// then paints the tile rect (x1,y1,z1)-(x2,y2,z2) into it. Repeatable:
// calling again with the same name adds more tiles to the same burrow
// instead of creating a second one.
//
// No floor/walkability check (unlike zones.cpp's applyDesignateZone) --
// burrows paint through hidden tiles by design, same house rule as dig
// designations, and DF's own burrow UI does this too (confirmed against
// Burrows::setAssignedTile, which does no such validation).
bool applyDesignateBurrow(const std::string &name, int16_t x1, int16_t y1, int16_t z1,
                          int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (name.empty()) {
        error = "burrow name must not be empty";
        return false;
    }
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1 || z2 < z1) {
        error = "invalid region (x2<x1, y2<y1, or z2<z1)";
        return false;
    }

    df::burrow *b = Burrows::findByName(name);
    bool created = false;
    if (!b) {
        // create_burrow() (burrow.lua:60-76) -- next_id is a plain
        // allocator the caller must post-increment; no CHECK_* macro
        // involved, pure heap alloc + vector push, same as the shipped
        // quickfort burrow-designation feature.
        b = new df::burrow();
        b->id = df::global::plotinfo->burrows.next_id++;
        b->name = name;
        df::global::plotinfo->burrows.list.push_back(b);
        created = true;
    }

    // DF map bounds are a single rectangular prism: once both corners are
    // validated in-bounds above, every enclosed tile is automatically
    // in-bounds too -- no per-tile bounds check needed inside the loop
    // (mirrors zones.cpp's applyDesignateZone, which relies on the same
    // property).
    int painted = 0, alreadyIn = 0;
    for (int16_t z = z1; z <= z2; z++) {
        for (int16_t y = y1; y <= y2; y++) {
            for (int16_t x = x1; x <= x2; x++) {
                df::coord pos(x, y, z);
                if (Burrows::isAssignedTile(b, pos)) {
                    alreadyIn++;
                    continue;
                }
                Burrows::setAssignedTile(b, pos, true);
                painted++;
            }
        }
    }

    error = (created ? "burrow '" + name + "' created; " : "burrow '" + name + "': ") +
            std::to_string(painted) + " tiles added, " + std::to_string(alreadyIn) + " already in burrow";
    return true;
}

// applyRemoveBurrow deletes the named burrow entirely -- clears its tiles
// and unit assignments first (mirrors burrow.lua's deletion path, which
// only ever deletes empty burrows; this does it unconditionally and
// defensively per this plugin's house style), detaches it from the
// civilian alert's burrow set if it was a member (civ-alert.lua's
// remove_civalert_burrow behavior, so the alert never carries a dangling
// burrow id), then frees the struct.
bool applyRemoveBurrow(const std::string &name, std::string &error)
{
    df::burrow *b = Burrows::findByName(name);
    if (!b) {
        error = "no burrow named '" + name + "'";
        return false;
    }

    Burrows::clearTiles(b);
    Burrows::clearUnits(b);

    auto &alerts = df::global::plotinfo->alerts;
    if (alerts.list.size() > 1) {
        df::alert_statest *civAlert = alerts.list[1];
        auto &members = civAlert->burrows;
        auto pos = std::lower_bound(members.begin(), members.end(), b->id);
        if (pos != members.end() && *pos == b->id) {
            members.erase(pos);
            if (members.empty()) {
                alerts.civ_alert_idx = 0;
            }
        }
    }

    auto &list = df::global::plotinfo->burrows.list;
    auto it = std::find(list.begin(), list.end(), b);
    if (it != list.end()) list.erase(it);
    delete b;

    error = "burrow '" + name + "' removed";
    return true;
}

// applyAssignBurrow assigns (assign=true) or unassigns (assign=false)
// units to/from the named burrow via Burrows::setAssignedUnit, which keeps
// unit->burrows (or unit->inactive_burrows for a suspended burrow) and
// burrow->units in sync, both binary-sorted. allCitizens=true targets
// every current citizen (Units::isCitizen -- sane, non-dead, current-fort)
// in one call; unitID is ignored in that case.
bool applyAssignBurrow(const std::string &name, bool assign, bool allCitizens, int32_t unitID, std::string &error)
{
    df::burrow *b = Burrows::findByName(name);
    if (!b) {
        error = "no burrow named '" + name + "'";
        return false;
    }

    if (allCitizens) {
        // setAssignedUnit is idempotent, so count only actual state changes
        // (isAssignedUnit pre-check) -- a repeat call over an already
        // assigned population should truthfully report 0 changed, not
        // overstate work done by counting every citizen processed.
        int changed = 0, total = 0;
        for (auto *unit : df::global::world->units.active) {
            if (!unit || !Units::isCitizen(unit)) continue;
            total++;
            if (Burrows::isAssignedUnit(b, unit) == assign) continue;
            Burrows::setAssignedUnit(b, unit, assign);
            changed++;
        }
        error = std::to_string(changed) + " of " + std::to_string(total) + " citizens now " +
                (assign ? "assigned to" : "unassigned from") + " burrow '" + name + "'";
        return true;
    }

    df::unit *unit = df::unit::find(unitID);
    if (!unit) {
        error = "no unit with id " + std::to_string(unitID);
        return false;
    }
    Burrows::setAssignedUnit(b, unit, assign);
    error = std::string(assign ? "assigned unit#" : "unassigned unit#") + std::to_string(unitID) +
            (assign ? " to" : " from") + " burrow '" + name + "'";
    return true;
}

// getOrCreateCivAlert mirrors civ-alert.lua's get_civ_alert(): bootstraps
// plotinfo->alerts.list to at least 2 entries -- index 0 is a reserved/
// unused placeholder enforced only by script convention, not the struct
// itself, so a save this plugin's own command reaches before gui/civ-alert
// or quickfort has touched plotinfo->alerts needs this same bootstrap or
// list[1] would be an out-of-bounds access -- and returns the usable slot
// at index 1.
static df::alert_statest *getOrCreateCivAlert()
{
    auto &alerts = df::global::plotinfo->alerts;
    while (alerts.list.size() < 2) {
        auto *a = new df::alert_statest();
        a->id = alerts.next_id++;
        a->name = "civ-alert";
        alerts.list.push_back(a);
    }
    return alerts.list[1];
}

// applySetAlert sounds (active=true) or clears (active=false) DF's v50
// civilian alert against the named burrow. Per DFHack's own docs for this
// exact vanilla mechanism (scripts/docs/gui/civ-alert.rst): while active,
// the game engine rushes ALL non-military citizens to a member burrow and
// keeps them confined there -- unlike ordinary burrow assignment (a
// "suggestion"), this is a hard behavioral override that does NOT require
// any unit to have been assigned to the burrow first (gui/civ-alert's own
// workflow never assigns units; it only paints tiles and flips this alert).
// Leaving the alert active after the danger passes leaves every civilian
// confined and they can grow unhappy or starve, per the same doc -- callers
// should clear it (active=false) promptly.
bool applySetAlert(const std::string &name, bool active, std::string &error)
{
    df::burrow *b = Burrows::findByName(name);
    if (!b) {
        error = "no burrow named '" + name + "'";
        return false;
    }

    df::alert_statest *civAlert = getOrCreateCivAlert();
    auto &members = civAlert->burrows;
    auto pos = std::lower_bound(members.begin(), members.end(), b->id);
    bool present = (pos != members.end() && *pos == b->id);

    if (active) {
        if (!present) members.insert(pos, b->id);
        // can_sound_alarm() gate (civ-alert.lua:22-24): idx==0 && burrows
        // non-empty. members is guaranteed non-empty here (b->id was just
        // ensured a member), so this only needs the idx==0 half.
        if (df::global::plotinfo->alerts.civ_alert_idx == 0) {
            df::global::plotinfo->alerts.civ_alert_idx = 1;
        }
        int assignedUnits = (int)b->units.size();
        error = "civilian alert ACTIVE via burrow '" + name + "' -- ALL non-military citizens are now "
                "rushing to and confined to this burrow regardless of burrow assignment (" +
                std::to_string(assignedUnits) + " units separately assigned to it via assign_burrow, "
                "which is not required for this). Call set_alert active=false promptly once the danger "
                "passes, or citizens stay confined and may grow unhappy or starve.";
        return true;
    }

    if (present) {
        members.erase(pos);
        // remove_civalert_burrow() (civ-alert.lua:45-51): auto-clear the
        // alarm once its burrow set becomes empty, so no dangling active
        // index with nothing left to restrict.
        if (members.empty()) {
            df::global::plotinfo->alerts.civ_alert_idx = 0;
        }
    }
    bool stillSounding = df::global::plotinfo->alerts.civ_alert_idx != 0;
    error = "burrow '" + name + "' removed from the civilian alert" +
            (stillSounding ? " (alarm still sounding for other burrows)" : " (alarm now clear)");
    return true;
}

// countBurrowTiles sums the tile_bitmask popcount across every block that
// has a mask for this burrow -- Burrows::listBlocks + getBlockMask(create=
// false) is the same read pattern plugins/burrow.cpp's own paint/report
// code uses, just read-only here. Cheap: bounded by the burrow's own
// block_x.size(), not the whole map.
static int countBurrowTiles(df::burrow *b)
{
    std::vector<df::map_block*> blocks;
    Burrows::listBlocks(&blocks, b);
    int total = 0;
    for (auto *block : blocks) {
        auto *mask = Burrows::getBlockMask(b, block, false);
        if (!mask) continue;
        for (int i = 0; i < 16; i++) {
            uint16_t bits = mask->tile_bitmask.bits[i];
            while (bits) {
                total += (bits & 1);
                bits >>= 1;
            }
        }
    }
    return total;
}

// handleListBurrows is declared here (uses Burrows/df::burrow types this
// file already includes) but dispatched from queries.cpp's executeQuery,
// same forward-declared-elsewhere pattern as locations.cpp's
// wireFromAbstractBuildingType. jsonStr/jsonInt/jsonError mirror the
// private helpers in queries.cpp exactly (same tiny hand-rolled JSON
// writer, no shared header for it in this plugin).
static std::string jsonEscapeBurrow(const std::string &s) {
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
static std::string jsonStrBurrow(const std::string &s) { return "\"" + jsonEscapeBurrow(s) + "\""; }
static std::string jsonIntBurrow(int64_t v) { char buf[32]; snprintf(buf, sizeof(buf), "%lld", (long long)v); return std::string(buf); }

std::string handleListBurrows(const std::string & /*args*/, uint8_t &status)
{
    if (!df::global::plotinfo || !df::global::world) {
        status = QUERY_STATUS_ERROR;
        return "{\"error\":\"plotinfo or world is null\"}";
    }

    auto &alerts = df::global::plotinfo->alerts;
    bool alertSounding = alerts.civ_alert_idx != 0;
    std::vector<int32_t> activeAlertBurrows;
    if (alertSounding && (size_t)alerts.civ_alert_idx < alerts.list.size()) {
        activeAlertBurrows = alerts.list[alerts.civ_alert_idx]->burrows;
    }

    std::string out = "{\"burrows\":[";
    bool first = true;
    for (auto *b : df::global::plotinfo->burrows.list) {
        if (!b) continue;
        bool inAlert = std::binary_search(activeAlertBurrows.begin(), activeAlertBurrows.end(), b->id);
        if (!first) out += ",";
        first = false;
        out += "{\"name\":" + jsonStrBurrow(b->name) +
               ",\"id\":" + jsonIntBurrow(b->id) +
               ",\"tile_count\":" + jsonIntBurrow(countBurrowTiles(b)) +
               ",\"assigned_units\":" + jsonIntBurrow((int64_t)b->units.size()) +
               ",\"in_active_alert\":" + (inAlert ? "true" : "false") + "}";
    }
    out += "],\"alert_sounding\":";
    out += (alertSounding ? "true" : "false");
    out += "}";
    status = QUERY_STATUS_SUCCESS;
    return out;
}
