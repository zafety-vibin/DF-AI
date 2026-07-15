// DFHack Plugin — Query Handlers
//
// Implements the QUERY/QUERY_RESPONSE protocol. The orchestrator's
// deliberator can issue read-only queries (list_orders, manager_orders,
// dwarf_detail, building_status, workshop_jobs, stockpile_inventory,
// sim_status, map_slice, column_profile, list_buildings) and get
// structured JSON back.
//
// Threading: executeQuery runs on the main DF thread (called from
// plugin_onupdate after queue dispatch). Safe to read df::global state
// without CoreSuspender per the existing convention in this plugin.
//
// IMPORTANT: This file targets DFHack 53.12 conventions. Verify against
// the SDK headers when upgrading. Most-likely breakage points:
//   df/job_type.h            — JobType enum names + count
//   df/manager_order.h       — fields (job_type, item_type, mat_type, amount_total/left, status)
//   df/world.h               — manager_orders.all field name
//   df/unit.h                — skills, labors, current_job, mood, name access
//   df/unit_skill.h          — skill list shape
//   df/job_skill.h           — skill enum names
//   df/building.h            — building.flags, getType, jobs field
//   df/item.h                — item type, material refs
//   modules/Buildings.h      — findAtTile

#include "Core.h"
#include "Console.h"
#include "modules/Buildings.h"
#include "modules/Job.h"
#include "modules/Items.h"
#include "modules/Materials.h"
#include "modules/World.h"
#include "modules/Maps.h"
#include "modules/MapCache.h"
#include "TileTypes.h"
#include "MiscUtils.h"

#include "df/world.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/manager_order.h"
#include "df/manager_order_status.h"
#include "df/unit.h"
#include "df/unit_labor.h"
#include "df/unit_soul.h"
#include "df/unit_skill.h"
#include "df/job_skill.h"
#include "df/building.h"
#include "df/building_type.h"
#include "df/item.h"
#include "df/item_type.h"
#include "df/tiletype.h"
#include "df/tile_designation.h"
#include "df/plotinfost.h"
#include "df/building_civzonest.h"
#include "df/world_site.h"
#include "df/abstract_building.h"
#include "df/abstract_building_inn_tavernst.h"
#include "df/rental_roomst.h"
#include "df/reaction.h"
#include "df/reaction_reagent.h"
#include "df/reaction_reagent_itemst.h"
#include "df/reaction_reagent_type.h"
#include "df/reaction_flags.h"
#include "df/workshop_type.h"

#include "protocol.h"

#include <atomic>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>
#include <sstream>
#include <map>
#include <tuple>

using namespace DFHack;

extern void sendQueryResponse(uint32_t queryID, uint8_t status, const std::string &dataJSON);
extern std::atomic<int64_t> g_step_target_frame;   // defined in df_ai_protocol.cpp
extern std::atomic<bool> g_step_tripwire;          // defined in df_ai_protocol.cpp
extern std::string get_step_tripwire_reason();     // defined in df_ai_protocol.cpp

// ---------------------------------------------------------------------------
// JSON construction helpers — minimal, no external library.
// ---------------------------------------------------------------------------

static std::string jsonEscape(const std::string &s) {
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

static std::string jsonStr(const std::string &s) {
    return "\"" + jsonEscape(s) + "\"";
}
static std::string jsonInt(int64_t v) {
    char buf[32]; snprintf(buf, sizeof(buf), "%lld", (long long)v); return std::string(buf);
}
static std::string jsonError(const std::string &msg) {
    return "{\"error\":" + jsonStr(msg) + "}";
}

// Very small naive JSON object reader — extracts a top-level key's string
// or int value. Sufficient for the few simple args we pass in queries.
// Returns empty string if not found.
static std::string jsonGetString(const std::string &args, const std::string &key) {
    std::string needle = "\"" + key + "\"";
    auto pos = args.find(needle);
    if (pos == std::string::npos) return "";
    pos = args.find(':', pos);
    if (pos == std::string::npos) return "";
    pos++;
    while (pos < args.size() && (args[pos] == ' ' || args[pos] == '\t')) pos++;
    if (pos >= args.size()) return "";
    if (args[pos] == '"') {
        auto end = args.find('"', pos + 1);
        if (end == std::string::npos) return "";
        return args.substr(pos + 1, end - pos - 1);
    }
    // bare value — read until comma or }
    size_t end = pos;
    while (end < args.size() && args[end] != ',' && args[end] != '}') end++;
    return args.substr(pos, end - pos);
}

static int64_t jsonGetInt(const std::string &args, const std::string &key, int64_t def) {
    std::string s = jsonGetString(args, key);
    if (s.empty()) return def;
    try { return std::stoll(s); } catch (...) { return def; }
}

// ---------------------------------------------------------------------------
// Handlers. Each takes the args JSON string, returns the response JSON
// string and sets `status` (QUERY_STATUS_*).
// ---------------------------------------------------------------------------

// handleListOrders is the discovery half of generalized item construction:
// queue_job's ORDER_TYPE_BY_NAME path (work_orders.cpp:
// resolveJobTypeByName, via DFHack's find_enum_item<df::job_type>) accepts
// ANY of the names this enumerates, verbatim, with no C++ whitelist — this
// query is how the model finds a name it doesn't already know without a
// plugin rebuild. Exposed on the Go side as the standalone `job_types` tool
// (NOT folded into order/queue_job/orders — those stay cheap on every
// call; see internal/mcpserver/tools_state.go).
static std::string handleListOrders(const std::string &args, uint8_t &status) {
    // filter is an optional case-insensitive SUBSTRING match against the
    // job type NAME (e.g. filter="hatch" finds ConstructHatchCover among
    // ~240 entries) — mirrors the stocks tool's category param
    // (handleStockpileInventory above: substring match on item type name).
    // category below is a separate, heuristic string-matching
    // classification kept for readability in the response; it is NOT used
    // for filtering.
    std::string filter = jsonGetString(args, "filter");
    std::string lfilter = filter;
    for (auto &c : lfilter) c = tolower(c);

    std::ostringstream os;
    os << "{\"orders\":[";
    bool first = true;

    // VERIFY: number of job_type values may differ in DF 53.12.
    // Constant ENUM_LAST_ITEM(job_type) traditionally exposes count.
    // If unavailable, hard-cap at 300 to be safe.
    int maxJob = 300;
    for (int i = 0; i < maxJob; i++) {
        // Own the string (not a .c_str() pointer into it): ENUM_KEY_STR
        // returns a temporary std::string, and a raw pointer into that
        // temporary dangles the instant this statement ends — every use
        // below (strlen/lname copy/jsonStr) would be undefined behavior.
        std::string name = ENUM_KEY_STR(job_type, (df::job_type)i);
        if (name.empty()) continue;

        std::string lname = name;
        for (auto &c : lname) c = tolower(c);

        if (!lfilter.empty() && lname.find(lfilter) == std::string::npos) continue;

        std::string category = "other";
        if (lname.find("construct") != std::string::npos &&
            (lname.find("bed") != std::string::npos ||
             lname.find("table") != std::string::npos ||
             lname.find("throne") != std::string::npos ||
             lname.find("door") != std::string::npos ||
             lname.find("cabinet") != std::string::npos ||
             lname.find("box") != std::string::npos)) {
            category = "furniture";
        } else if (lname.find("brew") != std::string::npos ||
                   lname.find("preparemeal") != std::string::npos ||
                   lname.find("butcher") != std::string::npos ||
                   lname.find("cook") != std::string::npos ||
                   lname.find("mill") != std::string::npos) {
            category = "food";
        } else if (lname.find("makeweapon") != std::string::npos ||
                   lname.find("makearmor") != std::string::npos ||
                   lname.find("makeshield") != std::string::npos ||
                   lname.find("forge") != std::string::npos) {
            category = "military";
        } else if (lname.find("make") == 0 || lname.find("smelt") != std::string::npos ||
                   lname.find("decorate") != std::string::npos) {
            category = "raw";
        }

        if (!first) os << ",";
        first = false;
        os << "{\"id\":" << i
           << ",\"name\":" << jsonStr(name)
           << ",\"category\":" << jsonStr(category) << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleListReactions is the discovery half of queue_job's reaction-based
// path (ORDER_TYPE_CUSTOM_REACTION, work_orders.cpp applyQueueReactionJob)
// -- enumerates every FORTRESS_MODE_ENABLED df::reaction from the raws, the
// same vector applyQueueReactionJob linear-scans by code. Every code shown
// here round-trips into queue_job's reaction path verbatim. Exposed on the
// Go side as the standalone `list_reactions` tool (see internal/mcpserver/
// tools_state.go), same pattern as job_types/list_orders above -- kept out
// of the cheap-per-call tools (orders/queue_job) on purpose.
static std::string handleListReactions(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    // filter is an optional case-insensitive SUBSTRING match against either
    // the reaction's code or its display name -- mirrors job_types' filter
    // param (handleListOrders below).
    std::string filter = jsonGetString(args, "filter");
    std::string lfilter = filter;
    for (auto &c : lfilter) c = tolower(c);

    std::ostringstream os;
    os << "{\"reactions\":[";
    int count = 0;
    bool truncated = false;

    for (auto *reaction : df::global::world->raws.reactions.reactions) {
        if (!reaction) continue;
        if (!reaction->flags.is_set(df::reaction_flags::FORTRESS_MODE_ENABLED)) continue;

        std::string lcode = reaction->code;
        for (auto &c : lcode) c = tolower(c);
        std::string lname = reaction->name;
        for (auto &c : lname) c = tolower(c);
        if (!lfilter.empty() &&
            lcode.find(lfilter) == std::string::npos &&
            lname.find(lfilter) == std::string::npos) {
            continue;
        }

        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        count++;

        os << "{\"code\":" << jsonStr(reaction->code)
           << ",\"name\":" << jsonStr(reaction->name)
           << ",\"buildings\":[";
        size_t nAlt = reaction->building.type.size();
        for (size_t k = 0; k < nAlt; k++) {
            if (k) os << ",";
            int32_t altBuildingType = (int32_t)reaction->building.type[k];
            int32_t altSubtype = (k < reaction->building.subtype.size()) ? reaction->building.subtype[k] : -1;
            if (altBuildingType == (int32_t)df::building_type::Workshop) {
                os << jsonStr(altSubtype == -1 ? "any workshop" : ENUM_KEY_STR(workshop_type, (df::workshop_type)altSubtype));
            } else if (altBuildingType == -1) {
                os << jsonStr("any building");
            } else {
                os << jsonStr(ENUM_KEY_STR(building_type, (df::building_type)altBuildingType));
            }
        }
        os << "],\"reagents\":[";
        int reagentsEmitted = 0;
        for (size_t r = 0; r < reaction->reagents.size(); r++) {
            df::reaction_reagent *reagent = reaction->reagents[r];
            if (!reagent) continue;
            if (reagentsEmitted) os << ",";
            reagentsEmitted++;
            os << "{\"code\":" << jsonStr(reagent->code)
               << ",\"quantity\":" << jsonInt(reagent->quantity);
            if (reagent->getType() == df::reaction_reagent_type::item) {
                df::reaction_reagent_itemst *ri = (df::reaction_reagent_itemst*)reagent;
                os << ",\"item_type\":" << jsonStr(ri->item_type == df::item_type::NONE
                                                        ? "any" : ENUM_KEY_STR(item_type, ri->item_type));
            }
            os << "}";
        }
        os << "]}";
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleManagerOrders(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::ostringstream os;
    os << "{\"orders\":[";
    bool first = true;

    // VERIFY: world->manager_orders.all field name. Some 53.x have
    // manager_orders directly as a vector; some wrap in a struct.
    auto &orders = df::global::world->manager_orders.all;
    for (auto *o : orders) {
        if (!o) continue;
        if (!first) os << ",";
        first = false;
        os << "{"
           << "\"id\":" << jsonInt(o->id)
           << ",\"job_type\":" << jsonStr(ENUM_KEY_STR(job_type, o->job_type))
           << ",\"amount_total\":" << jsonInt(o->amount_total)
           << ",\"amount_left\":" << jsonInt(o->amount_left)
           << ",\"item_type\":" << jsonInt((int)o->item_type)
           << ",\"mat_type\":" << jsonInt(o->mat_type)
           << ",\"mat_index\":" << jsonInt(o->mat_index)
           << ",\"validated\":" << (o->status.bits.validated ? "true" : "false")
           << ",\"active\":" << (o->status.bits.active ? "true" : "false")
           << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleDwarfDetail(const std::string &args, uint8_t &status) {
    int64_t id = jsonGetInt(args, "id", -1);
    if (id < 0) {
        status = QUERY_STATUS_ERROR;
        return jsonError("missing or invalid id");
    }
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    df::unit *u = nullptr;
    for (auto *unit : df::global::world->units.active) {
        if (unit && (int64_t)unit->id == id) { u = unit; break; }
    }
    if (!u) {
        status = QUERY_STATUS_ERROR;
        return jsonError("unit not found");
    }

    std::ostringstream os;
    os << "{"
       << "\"id\":" << jsonInt(u->id)
       << ",\"position\":{\"x\":" << jsonInt(u->pos.x)
       << ",\"y\":" << jsonInt(u->pos.y)
       << ",\"z\":" << jsonInt(u->pos.z) << "}";

    // VERIFY: df::unit name field. In recent versions it's name.first_name +
    // a translated name. For now, expose the first-name token.
    os << ",\"first_name\":" << jsonStr(u->name.first_name);

    // Top 5 skills by experience.
    if (u->status.current_soul) {
        auto &skills = u->status.current_soul->skills;
        // crude sort: copy pointers, sort by rating
        std::vector<df::unit_skill *> sorted(skills.begin(), skills.end());
        std::sort(sorted.begin(), sorted.end(), [](df::unit_skill *a, df::unit_skill *b) {
            return a->rating > b->rating;
        });
        os << ",\"top_skills\":[";
        int n = 0;
        for (auto *s : sorted) {
            if (!s || n >= 5) break;
            if (s->rating == 0) break;
            if (n > 0) os << ",";
            os << "{\"skill\":" << jsonStr(ENUM_KEY_STR(job_skill, s->id))
               << ",\"level\":" << jsonInt(s->rating)
               << ",\"experience\":" << jsonInt(s->experience) << "}";
            n++;
        }
        os << "]";
    } else {
        os << ",\"top_skills\":[]";
    }

    // Current job, if any.
    if (u->job.current_job) {
        os << ",\"current_job\":" << jsonStr(ENUM_KEY_STR(job_type, u->job.current_job->job_type));
    } else {
        os << ",\"current_job\":null";
    }

    // Mood — very coarse. VERIFY: u->mood is an enum in recent versions.
    os << ",\"mood\":" << jsonInt((int)u->mood);

    // Currently-enabled labors — status.labors is a fixed C array of bool
    // indexed 0..LABOR_MAX_INDEX by the unit_labor enum (df/unit.h:374-386,
    // df/unit_labor.h:122). Surfaced so set_labor callers can see current
    // state before changing it (see protocol.h LABOR_* for the same
    // indices the set_labor command accepts).
    os << ",\"labors\":[";
    bool firstLabor = true;
    for (int i = 0; i <= LABOR_MAX_INDEX; i++) {
        if (!u->status.labors[i]) continue;
        if (!firstLabor) os << ",";
        firstLabor = false;
        os << jsonStr(ENUM_KEY_STR(unit_labor, (df::unit_labor)i));
    }
    os << "]";

    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleBuildingStatus(const std::string &args, uint8_t &status) {
    int64_t x = jsonGetInt(args, "x", -1);
    int64_t y = jsonGetInt(args, "y", -1);
    int64_t z = jsonGetInt(args, "z", -1);
    if (x < 0 || y < 0) {
        status = QUERY_STATUS_ERROR;
        return jsonError("missing or invalid coordinates");
    }

    df::building *b = Buildings::findAtTile(df::coord((int16_t)x, (int16_t)y, (int16_t)z));
    if (!b) {
        status = QUERY_STATUS_ERROR;
        return jsonError("no building at coordinates");
    }

    std::ostringstream os;
    os << "{"
       << "\"id\":" << jsonInt(b->id)
       << ",\"type\":" << jsonStr(ENUM_KEY_STR(building_type, b->getType()))
       << ",\"subtype\":" << jsonInt(b->getSubtype())
       << ",\"position\":{\"x\":" << jsonInt(b->centerx)
       << ",\"y\":" << jsonInt(b->centery)
       << ",\"z\":" << jsonInt(b->z) << "}";

    // VERIFY: building.getBuildStage() returns 0=unbuilt, 1=being built, ...
    os << ",\"build_stage\":" << jsonInt(b->getBuildStage());
    os << ",\"job_count\":" << jsonInt((int)b->jobs.size());
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleWorkshopJobs(const std::string &args, uint8_t &status) {
    int64_t x = jsonGetInt(args, "x", -1);
    int64_t y = jsonGetInt(args, "y", -1);
    int64_t z = jsonGetInt(args, "z", -1);
    if (x < 0 || y < 0) {
        status = QUERY_STATUS_ERROR;
        return jsonError("missing or invalid coordinates");
    }

    df::building *b = Buildings::findAtTile(df::coord((int16_t)x, (int16_t)y, (int16_t)z));
    if (!b) {
        status = QUERY_STATUS_ERROR;
        return jsonError("no building at coordinates");
    }

    std::ostringstream os;
    os << "{\"jobs\":[";
    bool first = true;
    for (auto *j : b->jobs) {
        if (!j) continue;
        if (!first) os << ",";
        first = false;
        os << "{\"id\":" << jsonInt(j->id)
           << ",\"job_type\":" << jsonStr(ENUM_KEY_STR(job_type, j->job_type))
           << ",\"suspend\":" << (j->flags.bits.suspend ? "true" : "false") << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleListBuildings(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    // Optional {"z": int} arg — absent means all levels.
    std::string zArg = jsonGetString(args, "z");
    bool hasZ = !zArg.empty();
    int64_t zFilter = hasZ ? jsonGetInt(args, "z", 0) : 0;

    std::ostringstream os;
    os << "{\"buildings\":[";
    int count = 0;
    bool truncated = false;
    for (auto *b : df::global::world->buildings.all) {
        if (!b) continue;
        if (hasZ && (int64_t)b->z != zFilter) continue;
        if (count >= 200) { truncated = true; break; }
        // getBuildStage counts construction progress; a building is done
        // when it reaches getMaxBuildStage (0/0 for instant buildings).
        int32_t stage = b->getBuildStage();
        int32_t maxStage = b->getMaxBuildStage();
        if (count) os << ",";
        os << "{\"type\":" << jsonStr(ENUM_KEY_STR(building_type, b->getType()))
           << ",\"x\":" << jsonInt(b->centerx)
           << ",\"y\":" << jsonInt(b->centery)
           << ",\"z\":" << jsonInt(b->z)
           << ",\"x1\":" << jsonInt(b->x1)
           << ",\"y1\":" << jsonInt(b->y1)
           << ",\"x2\":" << jsonInt(b->x2)
           << ",\"y2\":" << jsonInt(b->y2)
           << ",\"stage\":" << jsonInt(stage)
           << ",\"max_stage\":" << jsonInt(maxStage)
           << ",\"done\":" << (stage == maxStage ? "true" : "false")
           << "}";
        count++;
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// Forward declaration -- implemented in zones.cpp.
uint8_t wireFromCivzoneType(df::civzone_type t);

static std::string handleListZones(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    // Optional {"type": int} and {"z": int} filters -- absent means all.
    std::string typeArg = jsonGetString(args, "type");
    bool hasType = !typeArg.empty();
    int64_t typeFilter = hasType ? jsonGetInt(args, "type", 0) : 0;
    std::string zArg = jsonGetString(args, "z");
    bool hasZ = !zArg.empty();
    int64_t zFilter = hasZ ? jsonGetInt(args, "z", 0) : 0;

    std::ostringstream os;
    os << "{\"zones\":[";
    int count = 0;
    bool truncated = false;
    for (auto *b : df::global::world->buildings.all) {
        if (!b || b->getType() != df::building_type::Civzone) continue;
        auto *cz = strict_virtual_cast<df::building_civzonest>(b);
        if (!cz) continue;
        uint8_t wireKind = wireFromCivzoneType(cz->type);
        if (wireKind == 0) continue; // not one of the 18 fortress-relevant types
        if (hasType && (int64_t)wireKind != typeFilter) continue;
        if (hasZ && (int64_t)b->z != zFilter) continue;
        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        os << "{\"kind\":" << jsonInt(wireKind)
           << ",\"type_name\":" << jsonStr(ENUM_KEY_STR(civzone_type, cz->type))
           << ",\"x1\":" << jsonInt(b->x1)
           << ",\"y1\":" << jsonInt(b->y1)
           << ",\"x2\":" << jsonInt(b->x2)
           << ",\"y2\":" << jsonInt(b->y2)
           << ",\"z\":" << jsonInt(b->z)
           << ",\"owner_unit_id\":" << jsonInt(cz->assigned_unit_id)
           << ",\"assigned_units\":[";
        for (size_t i = 0; i < cz->assigned_units.size(); i++) {
            if (i) os << ",";
            os << jsonInt(cz->assigned_units[i]);
        }
        os << "]}";
        count++;
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// Forward declaration -- implemented in locations.cpp (Task 1).
uint8_t wireFromAbstractBuildingType(df::abstract_building_type t);

static std::string handleListLocations(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    df::world_site *site = df::world_site::find(df::global::plotinfo->site_id);
    if (!site) {
        status = QUERY_STATUS_ERROR;
        return jsonError("could not resolve the current site");
    }

    std::ostringstream os;
    os << "{\"locations\":[";
    int count = 0;
    bool truncated = false;
    for (auto *bld : site->buildings) {
        if (!bld) continue;
        uint8_t wireKind = wireFromAbstractBuildingType(bld->getType());
        if (wireKind == 0) continue; // not one of the 4 supported types
        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        os << "{\"id\":" << jsonInt(bld->id)
           << ",\"type\":" << jsonStr(ENUM_KEY_STR(abstract_building_type, bld->getType()));

        // Founding civzone's extents, if resolvable. getContents() is the
        // base class's virtual accessor -- `bld` here is the base
        // df::abstract_building* type (site->buildings' element type),
        // which has no `contents` field of its own (only the derived
        // abstract_building_*st subtypes do; see locations.cpp's
        // applyCreateLocation, which goes through this same accessor for
        // the identical reason).
        df::abstract_building_contents *contents = bld->getContents();
        if (contents && !contents->building_ids.empty()) {
            int32_t zoneID = contents->building_ids[0];
            df::building *zone = df::building::find(zoneID);
            if (zone) {
                os << ",\"x1\":" << jsonInt(zone->x1) << ",\"y1\":" << jsonInt(zone->y1)
                   << ",\"x2\":" << jsonInt(zone->x2) << ",\"y2\":" << jsonInt(zone->y2)
                   << ",\"z\":" << jsonInt(zone->z);
            }
        }

        // Lodging roster -- Tavern only.
        os << ",\"lodging\":[";
        if (bld->getType() == df::abstract_building_type::INN_TAVERN) {
            auto *tavern = strict_virtual_cast<df::abstract_building_inn_tavernst>(bld);
            if (tavern) {
                for (size_t i = 0; i < tavern->room_info.size(); i++) {
                    if (i) os << ",";
                    auto *room = tavern->room_info[i];
                    os << "{\"civzone_id\":" << jsonInt(room->civzone)
                       << ",\"x\":" << jsonInt(room->world_x)
                       << ",\"y\":" << jsonInt(room->world_y)
                       << ",\"z\":" << jsonInt(room->world_z) << "}";
                }
            }
        }
        os << "]}";
        count++;
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleStockpileInventory(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    std::string category = jsonGetString(args, "category");

    // Aggregate by (item_type, actual material). Walks world->items.all
    // which can be expensive on a large fort; the LLM should use this
    // sparingly. The material key is the same (type, index) pair
    // MaterialInfo::decode(item) reads, so Shale and Chalk boulders count
    // as separate entries instead of one anonymous BOULDER pile.
    std::map<std::tuple<int, int, int>, int> counts; // (item_type, mat_type, mat_index) → count

    // VERIFY: world->items.all field name (may be world->items.other.IN_PLAY).
    auto &items = df::global::world->items.all;
    for (auto *it : items) {
        if (!it) continue;
        // VERIFY: filter for items in stockpiles only — in some versions
        // this requires checking item.flags.bits.in_inventory == 0 etc.
        counts[std::make_tuple((int)it->getType(),
                               (int)it->getActualMaterial(),
                               (int)it->getActualMaterialIndex())]++;
    }

    std::ostringstream os;
    os << "{\"items\":[";
    bool first = true;
    for (auto &kv : counts) {
        df::item_type itype = (df::item_type)std::get<0>(kv.first);
        std::string typeName = ENUM_KEY_STR(item_type, itype);
        if (!category.empty() && typeName.find(category) == std::string::npos) continue;
        // Human-readable material name via MaterialInfo (state_name at room
        // temperature, e.g. "shale"). Empty string when the pair doesn't
        // decode (e.g. materialless items).
        MaterialInfo mi((int16_t)std::get<1>(kv.first), (int32_t)std::get<2>(kv.first));
        std::string matName = mi.isValid() ? mi.toString() : "";
        if (!first) os << ",";
        first = false;
        os << "{\"item_type\":" << jsonStr(typeName)
           << ",\"material\":" << jsonStr(matName)
           << ",\"count\":" << jsonInt(kv.second);
        if (itype == df::item_type::BOULDER) {
            // Economic stones (flux, ore-adjacent, etc.) are reserved by the
            // stone-use screen and masons won't take them by default — the
            // model needs to know a 40-boulder pile might be all off-limits.
            // economic_stone is indexed by inorganic raw index.
            bool economic = false;
            if (df::global::plotinfo && mi.isInorganic() && mi.index >= 0)
                economic = vector_get(df::global::plotinfo->economic_stone,
                                      (unsigned)mi.index, (char)0) != 0;
            os << ",\"economic\":" << (economic ? "true" : "false");
        }
        os << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

static std::string handleSimStatus(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    std::string out = "{";
    out += "\"paused\":";
    out += World::ReadPauseState() ? "true" : "false";
    out += ",\"frame\":" + jsonInt((int64_t)df::global::world->frame_counter);
    out += ",\"stepping\":";
    out += (g_step_target_frame >= 0) ? "true" : "false";
    // Tripwire: a critical announcement ended the last step early. The
    // record persists until the next step starts, so the Go step tool's
    // completion poll cannot miss it. Reason only present when tripped.
    if (g_step_tripwire.load()) {
        out += ",\"tripwire\":true";
        out += ",\"tripwire_reason\":" + jsonStr(get_step_tripwire_reason());
    } else {
        out += ",\"tripwire\":false";
    }
    out += "}";
    status = QUERY_STATUS_SUCCESS;
    return out;
}

// ---------------------------------------------------------------------------
// Classified map queries: map_slice + column_profile.
// ---------------------------------------------------------------------------

// classifyTileRevealed maps a tiletype + liquid state to one model-facing
// glyph, IGNORING the hidden (fog-of-war) bit. The HIDDEN designation bit
// only controls what the *dwarves/UI* have revealed; the real tiletype
// under fog is present in the map block and MapCache::tiletypeAt returns
// it regardless. column_profile uses this directly so surveys report true
// stratigraphy (soil/stone/mineral/liquid) for unexplored tiles — that is
// exactly what a survey is for, and the overseer in DF can see site
// stratigraphy via pre-embark/tools anyway. Mapping hidden tiles to
// "unknown" made column_profile useless on a fresh embark.
// Legend (keep in sync with internal/mapview and the look tool):
//   ? hidden  # stone wall  % soil wall  = mineral wall  . floor  , grass
//   T tree  t sapling/shrub  _ open air  < > X stairs  ^ ramp  ~ water
//   L magma  F fortification
static char classifyTileRevealed(df::tiletype tt, const df::tile_designation &des) {
    if (des.bits.flow_size > 0)
        return des.bits.liquid_type == df::tile_liquid::Magma ? 'L' : '~';
    using S = df::tiletype_shape;
    using M = df::tiletype_material;
    S shape = tileShape(tt);
    M mat = tileMaterial(tt);
    switch (shape) {
        case S::WALL:
            // DF 50.x+ tree trunks are multi-tile plants that read as
            // WALL shape with TREE material.
            if (mat == M::TREE) return 'T';
            if (mat == M::SOIL) return '%';
            if (mat == M::MINERAL) return '=';
            return '#';
        case S::FLOOR: case S::BOULDER: case S::PEBBLES:
            if (mat == M::GRASS_LIGHT || mat == M::GRASS_DARK ||
                mat == M::GRASS_DRY || mat == M::GRASS_DEAD) return ',';
            return '.';
        case S::STAIR_UP: return '<';
        case S::STAIR_DOWN: return '>';
        case S::STAIR_UPDOWN: return 'X';
        case S::RAMP: return '^';
        case S::RAMP_TOP: return '_';
        case S::SAPLING: case S::SHRUB: return 't';
        case S::TRUNK_BRANCH: case S::BRANCH: case S::TWIG: return 'T';
        case S::FORTIFICATION: return 'F';
        case S::EMPTY: case S::NONE: default: return '_';
    }
}

// classifyTile: the fog-respecting variant used for map_slice rows — the
// grid keeps '?' for hidden tiles so the model can still distinguish
// explored from unexplored terrain at a glance.
static char classifyTile(df::tiletype tt, const df::tile_designation &des) {
    if (des.bits.hidden) return '?';
    return classifyTileRevealed(tt, des);
}

// wetAt: designation-level wetness test for the damp computation. Wet =
// standing/flowing water (flow_size>=1, Water) or the aquifer bit — the
// same inputs dig.cpp's is_wet/is_aquifer use for DF's own damp-dig
// warnings. Returns false off-map (getTileDesignation is NULL past edges).
static bool wetAt(int32_t x, int32_t y, int32_t z) {
    df::tile_designation *des = Maps::getTileDesignation(x, y, z);
    if (!des) return false;
    if (des->bits.flow_size >= 1 && des->bits.liquid_type == df::tile_liquid::Water)
        return true;
    return des->bits.water_table;
}

// dampAt: mirrors dig.cpp's is_damp — a tile is damp when any of its 8
// horizontal neighbors or the tile directly above is wet. This is what DF
// itself checks when it cancels a dig with "damp stone located".
static bool dampAt(int32_t x, int32_t y, int32_t z) {
    for (int dy = -1; dy <= 1; dy++)
        for (int dx = -1; dx <= 1; dx++) {
            if (dx == 0 && dy == 0) continue;
            if (wetAt(x + dx, y + dy, z)) return true;
        }
    return wetAt(x, y, z + 1);
}

static std::string queryMapSlice(const std::string &args, uint8_t &status) {
    int64_t x1 = jsonGetInt(args, "x1", -1), y1 = jsonGetInt(args, "y1", -1);
    int64_t x2 = jsonGetInt(args, "x2", -1), y2 = jsonGetInt(args, "y2", -1);
    int64_t z  = jsonGetInt(args, "z", -1);
    status = QUERY_STATUS_ERROR;
    if (x1 < 0 || y1 < 0 || x2 < x1 || y2 < y1 || z < 0)
        return jsonError("map_slice needs x1,y1,z,x2,y2 with x2>=x1, y2>=y1");
    if ((x2 - x1 + 1) > 48 || (y2 - y1 + 1) > 48)
        return jsonError("map_slice region too large (max 48x48)");
    if (!Maps::isValidTilePos((int16_t)x1, (int16_t)y1, (int16_t)z) ||
        !Maps::isValidTilePos((int16_t)x2, (int16_t)y2, (int16_t)z))
        return jsonError("map_slice out of bounds");

    MapExtras::MapCache cache;
    std::string rows = "[";
    std::string designated = "[";
    std::string water = "[";
    std::string aquifer = "[";
    std::string designationKinds = "[";
    int desCount = 0, waterCount = 0, aquiferCount = 0, kindCount = 0;
    for (int16_t y = (int16_t)y1; y <= (int16_t)y2; y++) {
        std::string row;
        for (int16_t x = (int16_t)x1; x <= (int16_t)x2; x++) {
            df::coord pos(x, y, (int16_t)z);
            df::tile_designation des = cache.designationAt(pos);
            df::tiletype tt = cache.tiletypeAt(pos);
            row += classifyTile(tt, des);
            if (des.bits.dig != df::tile_dig_designation::No && desCount < 200) {
                if (desCount) designated += ",";
                designated += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                desCount++;

                // Kind detail for the designations lens. Smooth/engrave
                // (des.bits.smooth) is orthogonal to the dig-designation
                // enum, so it's checked separately and takes priority in
                // the rendered kind when both are set (a tile can be
                // marked both dig AND smooth simultaneously in DF, but
                // the "what am I about to become" question the lens
                // answers is dominated by whichever finishes first —
                // smooth only applies to already-carved floor, so a tile
                // with des.bits.dig != No hasn't been carved yet and
                // smooth wouldn't apply; this branch order is defensive,
                // not load-bearing, given that constraint).
                int kind = 0; // Default (dig)
                switch (des.bits.dig) {
                    case df::tile_dig_designation::Channel: kind = 1; break;
                    case df::tile_dig_designation::Ramp: kind = 2; break;
                    case df::tile_dig_designation::UpStair:
                    case df::tile_dig_designation::DownStair:
                    case df::tile_dig_designation::UpDownStair: kind = 3; break;
                    default: kind = 0; break;
                }
                if (kindCount < 200) {
                    if (kindCount) designationKinds += ",";
                    designationKinds += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(kind) + "]";
                    kindCount++;
                }
            } else if (des.bits.smooth && kindCount < 200) {
                if (kindCount) designationKinds += ",";
                designationKinds += "[" + jsonInt(x) + "," + jsonInt(y) + ",4]"; // smooth/engrave
                kindCount++;
            }
            // Visible water only — hidden pockets stay under fog, matching
            // the '?' the grid shows for the same tile.
            if (!des.bits.hidden && des.bits.flow_size > 0 &&
                des.bits.liquid_type == df::tile_liquid::Water) {
                if (waterCount) water += ",";
                water += "[" + jsonInt(x) + "," + jsonInt(y) + "," +
                         jsonInt(des.bits.flow_size) + "]";
                waterCount++;
            }
            // Aquifer bit INCLUDING hidden tiles — approved fog-honesty
            // exception: DF's own damp-stone dig cancellations make aquifers
            // player-knowable, so hiding them only manufactures surprises.
            if (des.bits.water_table) {
                if (aquiferCount) aquifer += ",";
                aquifer += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                aquiferCount++;
            }
        }
        if (y != (int16_t)y1) rows += ",";
        rows += jsonStr(row);
    }
    rows += "]"; designated += "]"; water += "]"; aquifer += "]"; designationKinds += "]";
    status = QUERY_STATUS_SUCCESS;
    return "{\"z\":" + jsonInt(z) + ",\"x1\":" + jsonInt(x1) + ",\"y1\":" + jsonInt(y1) +
           ",\"rows\":" + rows + ",\"designated\":" + designated +
           ",\"water\":" + water + ",\"aquifer\":" + aquifer +
           ",\"designation_kinds\":" + designationKinds + "}";
}

static std::string queryColumnProfile(const std::string &args, uint8_t &status) {
    int64_t x = jsonGetInt(args, "x", -1), y = jsonGetInt(args, "y", -1);
    int64_t zt = jsonGetInt(args, "z_top", -1), zb = jsonGetInt(args, "z_bottom", -1);
    status = QUERY_STATUS_ERROR;
    if (x < 0 || y < 0 || zt < zb || zt < 0)
        return jsonError("column_profile needs x,y,z_top>=z_bottom");
    if ((zt - zb + 1) > 60) return jsonError("column_profile too tall (max 60)");
    MapExtras::MapCache cache;
    std::string levels = "[";
    bool first = true;
    for (int16_t z = (int16_t)zt; z >= (int16_t)zb; z--) {
        if (!Maps::isValidTilePos((int16_t)x, (int16_t)y, z)) continue;
        df::coord pos((int16_t)x, (int16_t)y, z);
        df::tile_designation des = cache.designationAt(pos);
        df::tiletype tt = cache.tiletypeAt(pos);
        char g = classifyTile(tt, des);
        // Hidden tiles keep glyph '?' and hidden:true, but shape/material
        // report the TRUTH from the real tiletype under the fog (see
        // classifyTileRevealed). DF's designation bits can't reveal hidden
        // material — but DFHack reads the tiletype directly, and a survey
        // that answers "unknown" for every undug tile is no survey at all.
        char cls = des.bits.hidden ? classifyTileRevealed(tt, des) : g;
        const char *shape = "other"; const char *mat = "other";
        switch (cls) {
            case '#': shape = "wall"; mat = "stone"; break;
            case '%': shape = "wall"; mat = "soil"; break;
            case '=': shape = "wall"; mat = "mineral"; break;
            case '?': shape = "hidden"; mat = "unknown"; break; // unreachable; kept as a safe default
            case ',': shape = "floor"; mat = "grass"; break;
            case '.': shape = "floor"; mat = "rock_or_soil"; break;
            case '_': shape = "open"; mat = "air"; break;
            case '~': shape = "liquid"; mat = "water"; break;
            case 'L': shape = "liquid"; mat = "magma"; break;
            case '<': case '>': case 'X': shape = "stair"; mat = "carved"; break;
            case '^': shape = "ramp"; mat = "carved"; break;
            case 'T': case 't': shape = "plant"; mat = "wood"; break;
            case 'F': shape = "fortification"; mat = "stone"; break;
        }
        // Water/aquifer/damp report the truth under fog, same as
        // shape/material above — this is the survey path, and dampness is
        // exactly what a cautious digger must see before breaching a wet
        // layer. All three fields are omitted when falsy (additive JSON).
        std::string wetness;
        if (des.bits.flow_size > 0 && des.bits.liquid_type == df::tile_liquid::Water)
            wetness += ",\"water\":" + jsonInt(des.bits.flow_size);
        if (des.bits.water_table)
            wetness += ",\"aquifer\":true";
        if (dampAt((int32_t)x, (int32_t)y, z))
            wetness += ",\"damp\":true";
        if (!first) levels += ",";
        first = false;
        levels += "{\"z\":" + jsonInt(z) + ",\"glyph\":" + jsonStr(std::string(1, g)) +
                  ",\"shape\":" + jsonStr(shape) + ",\"material\":" + jsonStr(mat) +
                  ",\"hidden\":" + (des.bits.hidden ? "true" : "false") + wetness + "}";
    }
    levels += "]";
    status = QUERY_STATUS_SUCCESS;
    return "{\"x\":" + jsonInt(x) + ",\"y\":" + jsonInt(y) + ",\"levels\":" + levels + "}";
}

// ---------------------------------------------------------------------------
// Public entry: executeQuery — dispatcher called from main thread.
// ---------------------------------------------------------------------------

void executeQuery(uint32_t queryID, const std::string &name, const std::string &args)
{
    uint8_t status = QUERY_STATUS_UNKNOWN;
    std::string data;

    // Exception barrier: DFHack module APIs used by these handlers
    // (Buildings::findAtTile in building_status/workshop_jobs, MapCache,
    // Items/Materials helpers) validate preconditions with
    // CHECK_NULL_POINTER / CHECK_INVALID_ARGUMENT macros that THROW
    // (dfhack library/include/Error.h). An exception escaping here unwinds
    // into drain_pending_work's caller — plugin_onupdate or the socket
    // thread — and without a guard would std::terminate DF. Mirror
    // executeCommand's guard (df_ai_protocol.cpp): convert to a
    // QUERY_STATUS_ERROR response carrying the exception text.
    try {
        if (name == "list_orders") {
            data = handleListOrders(args, status);
        } else if (name == "list_reactions") {
            data = handleListReactions(args, status);
        } else if (name == "manager_orders") {
            data = handleManagerOrders(args, status);
        } else if (name == "dwarf_detail") {
            data = handleDwarfDetail(args, status);
        } else if (name == "building_status") {
            data = handleBuildingStatus(args, status);
        } else if (name == "workshop_jobs") {
            data = handleWorkshopJobs(args, status);
        } else if (name == "list_buildings") {
            data = handleListBuildings(args, status);
        } else if (name == "list_zones") {
            data = handleListZones(args, status);
        } else if (name == "list_locations") {
            data = handleListLocations(args, status);
        } else if (name == "stockpile_inventory") {
            data = handleStockpileInventory(args, status);
        } else if (name == "sim_status") {
            data = handleSimStatus(args, status);
        } else if (name == "map_slice") {
            data = queryMapSlice(args, status);
        } else if (name == "column_profile") {
            data = queryColumnProfile(args, status);
        } else {
            status = QUERY_STATUS_UNKNOWN;
            data = jsonError("unknown query name: " + name);
        }
    } catch (std::exception &e) {
        status = QUERY_STATUS_ERROR;
        data = jsonError(std::string("DFHack exception: ") + e.what());
    } catch (...) {
        status = QUERY_STATUS_ERROR;
        data = jsonError("DFHack exception: unknown non-standard exception");
    }

    sendQueryResponse(queryID, status, data);
}
