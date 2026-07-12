// DFHack Plugin — Query Handlers
//
// Implements the QUERY/QUERY_RESPONSE protocol. The orchestrator's
// deliberator can issue read-only queries (list_orders, manager_orders,
// dwarf_detail, building_status, workshop_jobs, stockpile_inventory)
// and get structured JSON back.
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

#include "df/world.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/manager_order.h"
#include "df/manager_order_status.h"
#include "df/unit.h"
#include "df/unit_soul.h"
#include "df/unit_skill.h"
#include "df/job_skill.h"
#include "df/building.h"
#include "df/building_type.h"
#include "df/item.h"
#include "df/item_type.h"

#include "protocol.h"

#include <atomic>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>
#include <sstream>
#include <map>

using namespace DFHack;

extern void sendQueryResponse(uint32_t queryID, uint8_t status, const std::string &dataJSON);
extern std::atomic<int64_t> g_step_target_frame;  // defined in df_ai_protocol.cpp

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

static std::string handleListOrders(const std::string &args, uint8_t &status) {
    // Walks df::job_type enum. Returns name + a coarse category for
    // filtering. Categories are heuristic — when DFHack adds
    // job_type::is_designation_job() or similar helpers, swap to those.
    std::string filter = jsonGetString(args, "filter");

    std::ostringstream os;
    os << "{\"orders\":[";
    bool first = true;

    // VERIFY: number of job_type values may differ in DF 53.12.
    // Constant ENUM_LAST_ITEM(job_type) traditionally exposes count.
    // If unavailable, hard-cap at 300 to be safe.
    int maxJob = 300;
    for (int i = 0; i < maxJob; i++) {
        const char *name = ENUM_KEY_STR(job_type, (df::job_type)i).c_str();
        if (!name || strlen(name) == 0) continue;

        std::string category = "other";
        std::string lname = name;
        for (auto &c : lname) c = tolower(c);
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

        if (!filter.empty() && category != filter) continue;
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

static std::string handleStockpileInventory(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    std::string category = jsonGetString(args, "category");

    // Aggregate by item_type. Walks world->items.all which can be
    // expensive on a large fort; the LLM should use this sparingly.
    std::map<int, int> counts; // item_type → count

    // VERIFY: world->items.all field name (may be world->items.other.IN_PLAY).
    auto &items = df::global::world->items.all;
    for (auto *it : items) {
        if (!it) continue;
        // VERIFY: filter for items in stockpiles only — in some versions
        // this requires checking item.flags.bits.in_inventory == 0 etc.
        counts[(int)it->getType()]++;
    }

    std::ostringstream os;
    os << "{\"items\":[";
    bool first = true;
    for (auto &kv : counts) {
        std::string typeName = ENUM_KEY_STR(item_type, (df::item_type)kv.first);
        if (!category.empty() && typeName.find(category) == std::string::npos) continue;
        if (!first) os << ",";
        first = false;
        os << "{\"item_type\":" << jsonStr(typeName)
           << ",\"count\":" << jsonInt(kv.second) << "}";
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
    out += "}";
    status = QUERY_STATUS_SUCCESS;
    return out;
}

// ---------------------------------------------------------------------------
// Public entry: executeQuery — dispatcher called from main thread.
// ---------------------------------------------------------------------------

void executeQuery(uint32_t queryID, const std::string &name, const std::string &args)
{
    uint8_t status = QUERY_STATUS_UNKNOWN;
    std::string data;

    if (name == "list_orders") {
        data = handleListOrders(args, status);
    } else if (name == "manager_orders") {
        data = handleManagerOrders(args, status);
    } else if (name == "dwarf_detail") {
        data = handleDwarfDetail(args, status);
    } else if (name == "building_status") {
        data = handleBuildingStatus(args, status);
    } else if (name == "workshop_jobs") {
        data = handleWorkshopJobs(args, status);
    } else if (name == "stockpile_inventory") {
        data = handleStockpileInventory(args, status);
    } else if (name == "sim_status") {
        data = handleSimStatus(args, status);
    } else {
        status = QUERY_STATUS_UNKNOWN;
        data = jsonError("unknown query name: " + name);
    }

    sendQueryResponse(queryID, status, data);
}
