// DFHack Plugin — Query Handlers
//
// Implements the QUERY/QUERY_RESPONSE protocol. The orchestrator's
// deliberator can issue read-only queries (list_orders, manager_orders,
// dwarf_detail, building_status, workshop_jobs, stockpile_inventory,
// sim_status, map_slice, column_profile, list_buildings, list_mandates) and
// get structured JSON back.
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
#include "modules/Units.h"
#include "TileTypes.h"
#include "MiscUtils.h"

#include "df/world.h"
#include "df/job.h"
#include "df/job_type.h"
#include "df/itemdef_weaponst.h"
#include "df/itemdef_armorst.h"
#include "df/itemdef_toolst.h"
#include "df/tool_uses.h"
#include "df/job_list_link.h"
#include "df/job_item.h"
#include "df/job_item_ref.h"
#include "df/general_ref.h"
#include "df/mood_type.h"
#include "df/mood_stage_type.h"
#include "df/inorganic_raw.h"
#include "df/inorganic_flags.h"
#include "df/manager_order.h"
#include "df/manager_order_status.h"
#include "df/job_material_category.h"
#include "df/workquota_frequency_type.h"
#include "df/mandate.h"
#include "df/mandate_type.h"
#include "df/mandate_flag.h"
#include "df/punishmentst.h"
#include "df/punishment_flag.h"
#include "df/unit.h"
#include "df/creature_raw.h"
#include "df/unit_labor.h"
#include "df/unit_soul.h"
#include "df/unit_skill.h"
#include "df/job_skill.h"
#include "df/unit_personality.h"
#include "df/personality_needst.h"
#include "df/need_type.h"
#include "df/personality_moodst.h"
#include "df/emotion_type.h"
#include "df/unit_thought_type.h"
#include "df/personality_valuest.h"
#include "df/value_type.h"
#include "df/personality_goalst.h"
#include "df/personality_goal_flag.h"
#include "df/goal_type.h"
#include "df/personality_facet_type.h"
#include "df/building.h"
#include "df/building_type.h"
#include "df/item.h"
#include "df/item_actual.h"
#include "df/item_type.h"
#include "df/item_quality.h"
#include "df/map_block.h"
#include "df/tiletype.h"
#include "df/tile_designation.h"
#include "df/plotinfost.h"
#include "df/labor_infost.h"
#include "df/work_detail.h"
#include "df/work_detail_flags.h"
#include "df/work_detail_mode.h"
#include "df/work_detail_icon_type.h"
#include "df/building_civzonest.h"
#include "df/building_stockpilest.h"
#include "df/stockpile_settings.h"
#include "df/stockpile_group_set.h"
#include "df/buildings_other.h"
#include "df/world_site.h"
#include "df/abstract_building.h"
#include "df/abstract_building_inn_tavernst.h"
#include "df/abstract_building_templest.h"
#include "df/religious_practice_type.h"
#include "df/religious_practice_data.h"
#include "df/rental_roomst.h"
#include "df/reaction.h"
#include "df/reaction_reagent.h"
#include "df/reaction_reagent_itemst.h"
#include "df/reaction_reagent_type.h"
#include "df/workshop_type.h"
#include "df/plant_raw.h"
#include "df/plant_raw_flags.h"
#include "df/item_seedsst.h"
#include "df/items_other_id.h"
#include "df/item_flags.h"
#include "df/entity_position.h"
#include "df/entity_position_responsibility.h"
#include "df/entity_position_assignment.h"
#include "df/entity_position_flags.h"
#include "df/unit_demand.h"
#include "df/demand_room.h"
#include "df/building_actual.h"
#include "df/buildingitemst.h"
#include "df/building_item_role_type.h"
#include "df/engraving.h"
#include "df/engraving_flags.h"
#include "df/building_bridgest.h"
#include "df/building_bridge_flag.h"
#include "df/building_coffinst.h"
#include "df/incident.h"
#include "df/caravan_state.h"
#include "df/timed_event.h"
#include "df/timed_event_type.h"
#include "df/season.h"
#include "df/meeting_diplomat_info.h"
#include "df/historical_entity.h"
#include "df/building_tradedepotst.h"
#include "df/building_tradedepot_flag.h"
#include "df/plot_merchant_flag.h"
#include "df/entity_buy_prices.h"
#include "df/entity_sell_prices.h"
#include "df/entity_buy_requests.h"
#include "df/entity_sell_requests.h"
#include "df/entity_sell_category.h"
#include "df/meeting_topic.h"
#include "df/meeting_event_type.h"
#include "df/meeting_event.h"
#include "df/historical_figure.h"
#include "modules/Translation.h"

#include "protocol.h"

#include <algorithm>
#include <array>
#include <atomic>
#include <cstdint>
#include <cstdio>
#include <string>
#include <vector>
#include <sstream>
#include <map>
#include <mutex>
#include <set>
#include <tuple>
#include <unordered_map>
#include <unordered_set>
#include <utility>

// Builds the set of tile coords with an in-flight dig-designation job
// (implemented in tile_extractor.cpp; also forward-declared the same way
// in tile_updates.cpp). DF clears -- or stops reflecting -- the raw
// des.bits.dig bit once a unit claims the dig job, so a tile mid-dig
// looks indistinguishable from an undesignated one if map_slice only
// reads that bit. queryMapSlice below ORs this set into its designated-
// tile check for exactly that reason: a tile with an active miner en
// route or working it must still show as designated in `look`, not
// silently revert to looking like plain undesignated rock.
std::unordered_set<df::coord> collect_dig_job_targets();

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
// jsonNum -- only user is wellbeing/dwarf_detail's focus_ratio (current_focus
// / undistracted_focus). %g avoids scientific notation for the small ratios
// those fields produce and never emits trailing zeros; callers only ever
// pass finite values here (both call sites guard the zero-denominator case
// before reaching this function).
static std::string jsonNum(double v) {
    char buf[64]; snprintf(buf, sizeof(buf), "%g", v); return std::string(buf);
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

// handleListJobSubtypes is job_types' second discovery mode (the
// subtype_of arg, see handleListOrders below) -- the discovery half of the
// item-SUBTYPE pinning wave (docs/decisions.md), the counterpart to
// work_orders.cpp's resolveItemSubtype (queue_job/order's execution-time
// resolver). Given a df::job_type NAME (resolved via the SAME find_enum_item
// lookup resolveJobTypeByName/resolveItemSubtype use), enumerates every raws
// itemdef `id` token valid for that job_type's produced item -- item_type
// itself is derived from the job_type's own ENUM_ATTR(item), the same way
// execution does, so this and the execution-time resolver can never
// disagree about which raws vector is authoritative for a given job_type.
//
// Scoped to exactly the three item_types this pass's queue_job whitelist
// actually needs (WEAPON/ARMOR/TOOL — MakeWeapon/MakeArmor/MakeTool) rather
// than every item_type ItemTypeInfo::find's own switch could resolve
// (Items.cpp's ITEMDEF_VECTORS macro covers 14): order's subtype path is
// deliberately fully generic and would still resolve, say, MakeShield's
// SHIELD subtype correctly even though it isn't listed here -- this is a
// documented discovery-catalog gap, not a resolution-correctness one.
static std::string handleListJobSubtypes(const std::string &jobTypeName, uint8_t &status) {
    df::job_type jt;
    std::ostringstream os;
    if (!find_enum_item(&jt, jobTypeName)) {
        os << "{\"orders\":[],\"subtype_of\":" << jsonStr(jobTypeName)
           << ",\"error\":\"unrecognized job type name\"}";
        status = QUERY_STATUS_SUCCESS;
        return os.str();
    }

    df::item_type itemType = ENUM_ATTR(job_type, item, jt);
    os << "{\"orders\":[],\"subtype_of\":" << jsonStr(jobTypeName)
       << ",\"item_type\":" << jsonStr(ENUM_KEY_STR(item_type, itemType));

    os << ",\"subtypes\":[";
    bool first = true;
    auto emit = [&](const std::string &id, const std::string &name) {
        if (!first) os << ",";
        first = false;
        os << "{\"id\":" << jsonStr(id) << ",\"name\":" << jsonStr(name) << "}";
    };
    if (df::global::world) {
        auto &defs = df::global::world->raws.itemdefs;
        switch (itemType) {
            case df::item_type::WEAPON:
                for (auto *d : defs.weapons) if (d) emit(d->id, d->name);
                break;
            case df::item_type::ARMOR:
                for (auto *d : defs.armor) if (d) emit(d->id, d->name);
                break;
            case df::item_type::TOOL:
                for (auto *d : defs.tools) if (d) emit(d->id, d->name);
                break;
            default:
                break;
        }
    }
    os << "]";

    if (first) {
        // No subtypes were emitted -- either this job type has no item at
        // all (itemType == NONE, e.g. ConstructBed) or a valid item_type
        // this discovery catalog doesn't enumerate yet (see the scope note
        // above). Distinguish the two rather than a single flat "empty".
        if (itemType == df::item_type::NONE) {
            os << ",\"note\":\"this job type has no associated item -- subtype pinning does not apply to it\"";
        } else {
            os << ",\"note\":\"no subtype catalog for item_type " << ENUM_KEY_STR(item_type, itemType)
               << " yet -- order's subtype param may still resolve a valid raws token for it directly\"";
        }
    }
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleListOrders is the discovery half of generalized item construction:
// queue_job's ORDER_TYPE_BY_NAME path (work_orders.cpp:
// resolveJobTypeByName, via DFHack's find_enum_item<df::job_type>) accepts
// ANY of the names this enumerates, verbatim, with no C++ whitelist — this
// query is how the model finds a name it doesn't already know without a
// plugin rebuild. Exposed on the Go side as the standalone `job_types` tool
// (NOT folded into order/queue_job/orders — those stay cheap on every
// call; see internal/mcpserver/tools_state.go).
//
// subtype_of (optional): when set, bypasses the name-catalog listing below
// entirely and delegates to handleListJobSubtypes above instead -- job_types'
// second discovery mode, added for the item-SUBTYPE pinning wave.
static std::string handleListOrders(const std::string &args, uint8_t &status) {
    std::string subtypeOf = jsonGetString(args, "subtype_of");
    if (!subtypeOf.empty()) {
        return handleListJobSubtypes(subtypeOf, status);
    }

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

// isReactionPermittedForCiv is implemented in work_orders.cpp (shared with
// applyQueueReactionJob) so both the discovery and execution paths agree on
// what's actually runnable -- df::reaction_flags::FORTRESS_MODE_ENABLED is
// never set on raw-loaded reactions (vanilla raws carry no such tag, and no
// DFHack code reads it); the real gate is the fort's civ entity_raw
// permitted-reaction list.
bool isReactionPermittedForCiv(const std::string &reactionCode);

// handleListReactions is the discovery half of queue_job's reaction-based
// path (ORDER_TYPE_CUSTOM_REACTION, work_orders.cpp applyQueueReactionJob)
// -- enumerates every df::reaction from the raws that this fort's civ
// permits (see isReactionPermittedForCiv), the same vector
// applyQueueReactionJob linear-scans by code. Every code shown here
// round-trips into queue_job's reaction path verbatim. Exposed on the
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
        if (!isReactionPermittedForCiv(reaction->code)) continue;

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

// resolveOrderLabor maps a manager order's job_type (plus the order's own
// pinned material, when it has one) to the single df::unit_labor a citizen
// must have ENABLED before DF will ever hand a job of that type to anyone.
// Returns unit_labor::NONE when it cannot resolve one -- callers MUST treat
// NONE as "unknown", never as "no labor needed" (see handleManagerOrders'
// labor_available emission, which degrades to true in that case rather than
// asserting a blocker it cannot prove).
//
// Three tiers, in order:
//   (a) DF's own direct job_type->labor enum-attr (df.job.xml:76). Free, but
//       DF only populates it for ~17 haul/mine/lever-class job types -- none
//       of the manufacturing ones a manager order usually carries.
//   (b) The skill chain: job_type->skill (df.job.xml:80), or, when that is
//       NONE and the order's material resolves to craft class
//       Wood/Stone/Metal, the matching job_type->skill_wood/skill_stone/
//       skill_metal (df.job.xml:81-83); then job_skill->labor
//       (df.skill_enum.xml:6). Covers the plain-skill job types
//       (mining/woodcutting/herbalism/fishing/trapping/stonecutting) and the
//       generic multi-material craft/tool/ammo/jewelry job types whenever the
//       order's own material is known -- see the craftClass block below for
//       the two DIFFERENT ways DF encodes that (pinned mat_type vs. the
//       material_category bitfield), only one of which MaterialInfo can see.
//   (c) A tiny hardcoded override for the furniture job types DF's own
//       attribute tables leave completely unmapped -- ConstructBed
//       (df.job.xml:439-443), ConstructBin (:469-473) and MakeBarrel (:782-786)
//       carry neither `labor` nor any `skill*` attr at all. ConstructBed is
//       wood-exclusive and maps unconditionally; ConstructBin/MakeBarrel are
//       NOT (metal bins and metal barrels are real forge orders) and only map
//       when the order is actually wood.
//
// Deliberately NOT extended to the material-ambiguous furniture job types
// (ConstructTable/ConstructChair/ConstructCabinet/...), which can be wood,
// stone or metal and whose actual required labor depends on which workshop DF's
// manager eventually picks -- those degrade to NONE (unresolved) rather than
// guess a labor and produce a confident wrong blocker. Metal ConstructBin /
// MakeBarrel orders land in that same unresolved bucket by design.
static df::unit_labor resolveOrderLabor(df::job_type jt, MaterialInfo &mi,
                                        df::job_material_category matCat)
{
    // (a) direct job_type -> unit_labor
    df::unit_labor direct = ENUM_ATTR(job_type, labor, jt);
    if (direct != df::unit_labor::NONE)
        return direct;

    // Effective craft class. DF encodes a manager order's material in two
    // mutually exclusive ways and MaterialInfo only sees one of them:
    //   - metal/stone orders PIN an exact material (mat_type=0,
    //     mat_index=<inorganic raw id> -- stockflow.lua:437-441), which
    //     MaterialInfo decodes and getCraftClass() classifies;
    //   - wood/bone/cloth/leather/... orders leave mat_type=-1 and set a
    //     material_category bit instead (stockflow.lua:265-272, and the whole
    //     wooden-items block at :576-609 uses materials.wood). MaterialInfo
    //     (-1,-1) has a null material, so getCraftClass() returns None
    //     (Materials.cpp:362-365) and the skill_wood branch below would be
    //     unreachable for exactly the wooden orders that motivated this.
    // Only wood has a job_material_category flag-bit that maps to a craft
    // class (df.d_basics.xml:2902-2916 -- there is no stone or metal bit), so
    // this category fallback is wood-only by construction.
    df::craft_material_class craftClass = mi.getCraftClass();
    bool isWoodOrder = (craftClass == df::craft_material_class::Wood) ||
                       matCat.bits.wood || matCat.bits.wood2;
    if (craftClass == df::craft_material_class::None && isWoodOrder)
        craftClass = df::craft_material_class::Wood;

    // (b) job_type -> job_skill -> unit_labor, with the material-class
    // fallback for the generic craft job types.
    df::job_skill skill = ENUM_ATTR(job_type, skill, jt);
    if (skill == df::job_skill::NONE) {
        switch (craftClass) {
            case df::craft_material_class::Wood:
                skill = ENUM_ATTR(job_type, skill_wood, jt);
                break;
            case df::craft_material_class::Stone:
                skill = ENUM_ATTR(job_type, skill_stone, jt);
                break;
            case df::craft_material_class::Metal:
                skill = ENUM_ATTR(job_type, skill_metal, jt);
                break;
            default:
                break;
        }
    }
    if (skill != df::job_skill::NONE) {
        df::unit_labor fromSkill = ENUM_ATTR(job_skill, labor, skill);
        if (fromSkill != df::unit_labor::NONE)
            return fromSkill;
    }

    // (c) unmapped furniture override
    switch (jt) {
        case df::job_type::ConstructBed:
            // Wood-only: stockflow.lua:681-682 spells it out ("-- Bed,
            // specified as wooden") and ConstructBed is absent from the
            // IS_METAL/ITEMS_HARD forge list. ASSUMPTION: it has been
            // wood-only for the game's whole history; if DF ever adds a
            // stone/metal bed, this entry must go.
            return df::unit_labor::CARPENTER;
        case df::job_type::ConstructBin:
        case df::job_type::MakeBarrel:
            // NOT wood-exclusive: stockflow.lua lists both inside the
            // IS_METAL / ITEMS_HARD block too ({MakeBarrel,"Forge","Barrel"}
            // at :523, {ConstructBin,"Construct","Bin"} at :525), so a forged
            // metal barrel or bin is a real, orderable job done at a
            // Metalsmith's Forge under a metal labor -- claiming CARPENTER
            // for one would be a confident lie AND would outrank the weaker
            // but true workshop_assigned rung on the Go side.
            return isWoodOrder ? df::unit_labor::CARPENTER : df::unit_labor::NONE;
        default:
            break;
    }

    return df::unit_labor::NONE;
}

static std::string handleManagerOrders(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    // Order-progression fix (2026-07-22, see task C3): the plugin
    // force-validates every order at creation time (work_orders.cpp), so
    // "validated" alone can never distinguish a dispatched order from one
    // still sitting in the manager's queue. Real progression lives
    // elsewhere: df::job::order_id (job.h) is a plain int32_t copy of the
    // spawning order's id, set on every job the manager has actually
    // dispatched (-1 on jobs not spawned from an order); manager_order's
    // own workshop_id (manager_order.h) goes non-negative once the manager
    // has picked a workshop for the order, even before a worker takes the
    // job. One pass over world->jobs.list here builds a count per order_id
    // -- O(jobs) total instead of O(orders * jobs) from re-walking the job
    // list inside the orders loop below.
    std::unordered_map<int32_t, int32_t> jobsInProgressByOrder;
    for (df::job_list_link *cur = df::global::world->jobs.list.next; cur != NULL; cur = cur->next) {
        df::job *job = cur->item;
        if (job && job->order_id >= 0) {
            jobsInProgressByOrder[job->order_id]++;
        }
    }

    // Missing-labor fix: an order whose required labor is enabled on NOBODY
    // in the fort can never be dispatched, but every status the ladder above
    // reports ("queued, awaiting manager dispatch") reads as a healthy
    // pending order -- a live fort watched wood furniture orders sit for
    // game-weeks with no work detail holding CARPENTER and the tool never
    // said so. One extra single pass over the citizen roster (Units::isCitizen
    // -- the same filter work_details.cpp and handleWellbeing use) ORs every
    // citizen's enabled labors into a fort-wide bitmap, O(citizens * labors)
    // total instead of a per-order re-walk.
    //
    // Reading status.labors here is correct per docs/decisions.md
    // (2026-07-18): it is DF's own recomputed cache
    // (Units::setAutomaticProfessions) and is what DF's job dispatcher
    // actually consults, so it reflects BOTH explicit work-detail membership
    // AND DF's automatic-profession fallback -- exactly the two mechanisms
    // that made this bug survivable for masonry but fatal for carpentry.
    std::array<bool, LABOR_MAX_INDEX + 1> laborAvailable{};
    for (auto *unit : df::global::world->units.active) {
        if (!unit || !Units::isCitizen(unit)) continue;
        for (int i = 0; i <= LABOR_MAX_INDEX; i++) {
            if (unit->status.labors[i]) laborAvailable[i] = true;
        }
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

        auto jobsIt = jobsInProgressByOrder.find(o->id);
        int32_t jobsInProgress = (jobsIt != jobsInProgressByOrder.end()) ? jobsIt->second : 0;

        // material/material_category/frequency: added in the 2026-07-19
        // manager-work-order fix wave alongside applyWorkOrder's new
        // Material/Frequency params (work_orders.cpp) -- lets a session
        // directly see whether an order actually got material-pinned
        // instead of re-deriving it from job_type name alone. Same
        // MaterialInfo decode/toString convention handleListMandates above
        // uses; empty string when mat_type/mat_index don't decode (no exact
        // material selected -- the material_category bitfield, or nothing,
        // may still apply). bitfield_to_string returns "" when no bit is
        // set (the common case: most job types carry no material_category
        // constraint at all).
        MaterialInfo mi((int16_t)o->mat_type, (int32_t)o->mat_index);

        // required_labor/labor_available: the empty-string sentinel for
        // required_labor follows the SAME convention material/
        // material_category above already use -- "" means "could not be
        // decoded", not "none needed". When the labor is unresolved,
        // labor_available is emitted as true so the Go-side status ladder
        // keeps falling through to its existing rungs instead of claiming a
        // blocker this side cannot prove (see resolveOrderLabor's doc
        // comment for exactly which job types stay unresolved by design).
        df::unit_labor requiredLabor = resolveOrderLabor(o->job_type, mi, o->material_category);
        bool laborAvail = true;
        if (requiredLabor != df::unit_labor::NONE) {
            int laborIdx = (int)requiredLabor;
            if (laborIdx >= 0 && laborIdx <= LABOR_MAX_INDEX)
                laborAvail = laborAvailable[laborIdx];
        }

        os << "{"
           << "\"id\":" << jsonInt(o->id)
           << ",\"job_type\":" << jsonStr(ENUM_KEY_STR(job_type, o->job_type))
           << ",\"amount_total\":" << jsonInt(o->amount_total)
           << ",\"amount_left\":" << jsonInt(o->amount_left)
           << ",\"item_type\":" << jsonInt((int)o->item_type)
           << ",\"mat_type\":" << jsonInt(o->mat_type)
           << ",\"mat_index\":" << jsonInt(o->mat_index)
           << ",\"material\":" << jsonStr(mi.isValid() ? mi.toString() : "")
           << ",\"material_category\":" << jsonStr(bitfield_to_string(o->material_category))
           << ",\"frequency\":" << jsonStr(ENUM_KEY_STR(workquota_frequency_type, o->frequency))
           << ",\"validated\":" << (o->status.bits.validated ? "true" : "false")
           << ",\"active\":" << (o->status.bits.active ? "true" : "false")
           << ",\"jobs_in_progress\":" << jsonInt(jobsInProgress)
           << ",\"workshop_assigned\":" << (o->workshop_id >= 0 ? "true" : "false")
           << ",\"required_labor\":" << jsonStr(requiredLabor == df::unit_labor::NONE
                                                    ? "" : ENUM_KEY_STR(unit_labor, requiredLabor))
           << ",\"labor_available\":" << (laborAvail ? "true" : "false")
           << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleListMandates enumerates active noble mandates:
// df::global::world->mandates.all, confirmed via library/xml/df.mandate.xml
// in the DFHack 53.15-r2 checkout (struct-type 'mandate' original-name
// 'mandatest', reached through df.world.xml's world.mandates field of type
// mandate_handlerst -- .all is std::vector<df::mandate*>, per
// df/mandate_handlerst.h). No args.
//
// Three modes exist (mandate_type: Export/RESTRICTION_EXPORT_ITEM,
// Make/MAKE_ITEM, Guild/PAYJOB). Export and Make are the item-quota kind a
// player actually needs to react to (ban an export, keep production
// flowing); Guild ("pay job" dues) is a distinct mandate flavor confirmed
// only by the enum's original name -- its item/material/amount fields may
// not carry the same meaning, so they're still surfaced raw rather than
// guessed at.
//
// timeout_counter/timeout_limit are DF's own deadline pair (df.mandate.xml:
// original names "activetime"/"duration", counted once per 10 frames per
// that file's own comment). ticks_remaining converts that to df-ai's own
// tick vocabulary -- df::global::world->frame_counter, the same counter
// the step-N-ticks target math in df_ai_protocol.cpp uses -- so a caller
// doesn't have to know the x10 factor itself.
//
// `punishment` (df::punishmentst: hammerstrikes/prison_time/flags) is read
// verbatim, not computed here. Whether DF populates it at mandate creation
// or only once broken is NOT confirmed by anything in this checkout (that
// logic lives in DF's closed-source engine, not DFHack's structure
// definitions) -- an all-zero punishment on an otherwise real mandate is
// DF's own state, not a decode bug, and callers should not read zeros here
// as "no punishment will occur."
static std::string handleListMandates(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::ostringstream os;
    os << "{\"mandates\":[";
    bool first = true;
    int index = 0;
    for (auto *m : df::global::world->mandates.all) {
        if (!m) { index++; continue; }
        if (!first) os << ",";
        first = false;

        os << "{\"index\":" << jsonInt(index)
           << ",\"mode\":" << jsonStr(ENUM_KEY_STR(mandate_type, m->mode));

        os << ",\"item_type\":" << jsonStr(m->item_type == df::item_type::NONE
                                                ? "ANY" : ENUM_KEY_STR(item_type, m->item_type));
        os << ",\"item_subtype\":" << jsonInt(m->item_subtype);

        // Human-readable subtype name via DFHack's generic ItemTypeInfo
        // decode/toString (modules/Items.h) -- resolves (item_type, item_subtype)
        // to a raws-defined name (e.g. "short sword") with no per-item-type table
        // of our own. Confirmed against Items.cpp: decode()'s underlying
        // Items::getSubtypeDef() bounds-checks via vector_get (idx < vec.size()),
        // so out-of-range or -1 subtypes (item types with no subtype concept, or
        // item_type NONE) can't throw or read out of bounds -- toString() falls
        // back to the item_type's own caption, then its lowercased enum name, and
        // is empty only if even that lookup fails.
        ItemTypeInfo iti(m->item_type, m->item_subtype);
        os << ",\"item_subtype_name\":" << jsonStr(iti.toString());

        // Same MaterialInfo decode handleStockpileInventory uses -- gives a
        // human-readable state_name (e.g. "steel"), empty string when the
        // (type, index) pair doesn't decode (no material specified, e.g. a
        // Guild mandate or a NONE item_type).
        MaterialInfo mi((int16_t)m->mat_type, (int32_t)m->mat_index);
        os << ",\"material\":" << jsonStr(mi.isValid() ? mi.toString() : "");

        os << ",\"amount_total\":" << jsonInt(m->amount_total)
           << ",\"amount_remaining\":" << jsonInt(m->amount_remaining);

        os << ",\"timeout_counter\":" << jsonInt(m->timeout_counter)
           << ",\"timeout_limit\":" << jsonInt(m->timeout_limit)
           << ",\"ticks_remaining\":"
           << jsonInt(10LL * ((int64_t)m->timeout_limit - (int64_t)m->timeout_counter));

        // Issuing noble, when resolvable. Same crude first-name-only
        // identification handleDwarfDetail below uses (see its own VERIFY
        // comment on df::unit's name field) -- kept consistent rather than
        // inventing a fuller name resolution just for this handler.
        if (m->unit) {
            os << ",\"issued_by\":{\"id\":" << jsonInt(m->unit->id)
               << ",\"first_name\":" << jsonStr(m->unit->name.first_name) << "}";
        } else {
            os << ",\"issued_by\":null";
        }

        os << ",\"punishment\":{"
           << "\"hammerstrikes\":" << jsonInt(m->punishment.hammerstrikes)
           << ",\"prison_months\":" << jsonInt(m->punishment.prison_time)
           << ",\"beating\":" << (m->punishment.flags.bits.beating ? "true" : "false")
           << ",\"exiled\":" << (m->punishment.flags.bits.exiled ? "true" : "false")
           << ",\"death_sentence\":" << (m->punishment.flags.bits.death_sentence ? "true" : "false")
           << ",\"no_prison_available\":" << (m->punishment.flags.bits.no_prison_available ? "true" : "false")
           << "}"
           << ",\"punish_multiple\":" << (m->punish_multiple ? "true" : "false")
           << ",\"total_exempt\":" << (m->flags.bits.mandate_total_exempt ? "true" : "false");

        os << "}";
        index++;
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleListWorkDetails enumerates every entry of
// df::global::plotinfo->labor_info.work_details (df.plotinfo.xml:
// labor_infost, since v0.50.01; struct-type 'work_detail' original-name
// 'work_detailst') -- DF's own work-details/labor-group mechanism, the
// AUTHORITATIVE store behind a unit's derived status.labors cache (see
// df_ai_protocol.cpp's applySetLabor doc comment for that derived-cache
// relationship). assign_work_detail/set_work_detail_mode/
// create_work_detail (work_details.cpp) are this query's write side; this
// is the read side, covering both DF's own ten vanilla-category details
// (Miners/Woodcutters/.../Orderlies, created at fort founding) and any
// CUSTOM_1..CUSTOM_8 detail create_work_detail has added.
//
// index is the entry's position in that vector for THIS call only --
// work_detail carries no ID field of its own, so a caller that deletes or
// adds details between calls must re-resolve by name. allowed_labors is
// decoded the same way handleDwarfDetail's labors[] is below (ENUM_KEY_STR
// over the unit_labor enum, only true entries emitted). assigned_units
// resolves each member's id to a first-name using the same crude
// first-name-only identification handleListMandates' issued_by uses.
static std::string handleListWorkDetails(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world/plotinfo is null");
    }

    std::ostringstream os;
    os << "{\"work_details\":[";
    bool first = true;
    int index = 0;
    for (auto *wd : df::global::plotinfo->labor_info.work_details) {
        if (!wd) { index++; continue; }
        if (!first) os << ",";
        first = false;

        os << "{\"index\":" << jsonInt(index)
           << ",\"name\":" << jsonStr(wd->name)
           << ",\"icon\":" << jsonStr(ENUM_KEY_STR(work_detail_icon_type, wd->icon))
           << ",\"mode\":" << jsonStr(ENUM_KEY_STR(work_detail_mode, wd->flags.bits.mode))
           << ",\"no_modify\":" << (wd->flags.bits.no_modify ? "true" : "false")
           << ",\"cannot_be_everybody\":" << (wd->flags.bits.cannot_be_everybody ? "true" : "false");

        os << ",\"allowed_labors\":[";
        bool firstLabor = true;
        for (int i = 0; i <= LABOR_MAX_INDEX; i++) {
            if (!wd->allowed_labors[i]) continue;
            if (!firstLabor) os << ",";
            firstLabor = false;
            os << jsonStr(ENUM_KEY_STR(unit_labor, (df::unit_labor)i));
        }
        os << "]";

        os << ",\"assigned_units\":[";
        bool firstUnit = true;
        for (int32_t unitID : wd->assigned_units) {
            if (!firstUnit) os << ",";
            firstUnit = false;
            df::unit *u = df::unit::find(unitID);
            os << "{\"id\":" << jsonInt(unitID)
               << ",\"first_name\":" << (u ? jsonStr(u->name.first_name) : "null")
               << "}";
        }
        os << "]";

        os << "}";
        index++;
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleListMoods enumerates active strange-mood jobs -- the direct answer
// to "who's having a mood and do we have the material." Ported from this
// exact checkout's plugins/showmood.cpp (DFHack 53.15-r2), which is a
// complete, compiling reference for every piece below, rather than
// reinvented from general familiarity:
//
//  - The eleven StrangeMood* job types are contiguous in df/job_type.h
//    (StrangeMoodCrafter=53 .. StrangeMoodMechanics=65), so the single
//    range check below catches all of them, including StrangeMoodBrooding
//    (macabre mood) and StrangeMoodFell (fell mood), which sit inside that
//    same numeric range rather than needing separate bounds.
//  - The claiming dwarf is the job's UNIT_WORKER general_ref
//    (df::general_ref_type); the claimed workshop -- null if the dwarf
//    hasn't picked one yet -- is its BUILDING_HOLDER general_ref.
//  - Needed materials come from job->job_items.elements (df::job_item);
//    "got so far" is a count of job->items entries whose job_item_idx
//    matches that element's index. showmood.cpp's own BAR=150/CLOTH=10000
//    unit-divisor quirk is copied verbatim below (DF stores those
//    quantities in smaller sub-units; dividing gives the bars/cloth-units
//    count a player actually thinks in).
//  - unit->mood (df/mood_type.h, confirmed identical to the moodNames map
//    the dwarf_detail Go renderer already carries), unit->moodstage
//    (df/mood_stage_type.h: INITIAL/WORKING), and unit->job.mood_timeout
//    (df/unit.h's own doc comment: "counts down from 50000, insanity upon
//    reaching zero") are reported as-is.
//
// OPEN QUESTION, not resolved by anything in this checkout: whether
// mood_timeout ticks only while the dwarf is unable to act on the mood
// (no claimed workshop, or claimed but missing material) or continuously
// regardless of state. The raw counter is exposed either way -- callers
// should read it as "ticks until insanity, per DF's own bookkeeping" and
// not assume a particular relationship to elapsed real time.
//
// Reports data only, by design: no claim/produce/interrupt action, and no
// "what to do" field. build/queue_job/order already exist as the levers a
// caller pulls once they see there's rare material to produce or a
// workshop to build.
static std::string handleListMoods(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::ostringstream os;
    os << "{\"moods\":[";
    bool first = true;
    for (df::job_list_link *cur = df::global::world->jobs.list.next; cur != NULL; cur = cur->next) {
        df::job *job = cur->item;
        if (!job) continue;
        if ((job->job_type < df::job_type::StrangeMoodCrafter) ||
            (job->job_type > df::job_type::StrangeMoodMechanics))
            continue;

        df::unit *unit = nullptr;
        df::building *building = nullptr;
        for (auto *ref : job->general_refs) {
            if (!ref) continue;
            if (ref->getType() == df::general_ref_type::UNIT_WORKER)
                unit = ref->getUnit();
            if (ref->getType() == df::general_ref_type::BUILDING_HOLDER)
                building = ref->getBuilding();
        }

        if (!first) os << ",";
        first = false;

        os << "{\"job_id\":" << jsonInt(job->id)
           << ",\"job_type\":" << jsonStr(ENUM_KEY_STR(job_type, job->job_type));

        if (unit) {
            os << ",\"unit_id\":" << jsonInt(unit->id)
               << ",\"first_name\":" << jsonStr(unit->name.first_name)
               << ",\"mood\":" << jsonStr(unit->mood == df::mood_type::None
                                               ? "None" : ENUM_KEY_STR(mood_type, unit->mood))
               << ",\"moodstage\":" << jsonStr(ENUM_KEY_STR(mood_stage_type, unit->moodstage))
               << ",\"mood_skill\":" << jsonStr(ENUM_KEY_STR(job_skill, unit->job.mood_skill))
               << ",\"mood_timeout\":" << jsonInt(unit->job.mood_timeout);
        } else {
            // Per showmood.cpp: a strange-mood job can momentarily lack a
            // UNIT_WORKER ref (between claim and full attachment). Surfaced
            // as explicit nulls rather than skipped, so the job's existence
            // -- and its needed-items list below -- isn't hidden.
            os << ",\"unit_id\":null,\"first_name\":null,\"mood\":null"
               << ",\"moodstage\":null,\"mood_skill\":null,\"mood_timeout\":null";
        }

        if (building) {
            std::string bname;
            building->getName(&bname);
            os << ",\"claimed_building\":{\"id\":" << jsonInt(building->id)
               << ",\"type\":" << jsonStr(ENUM_KEY_STR(building_type, building->getType()))
               << ",\"name\":" << jsonStr(bname) << "}";
        } else {
            os << ",\"claimed_building\":null";
        }

        os << ",\"needed_items\":[";
        for (size_t i = 0; i < job->job_items.elements.size(); i++) {
            df::job_item *item = job->job_items.elements[i];
            if (!item) continue;
            if (i > 0) os << ",";

            MaterialInfo matinfo(item->mat_type, item->mat_index);

            // Same unit-divisor quirk showmood.cpp applies -- copied
            // verbatim rather than approximated (see file comment above).
            int64_t divisor = 1;
            if (item->item_type == df::item_type::BAR) divisor = 150;
            else if (item->item_type == df::item_type::CLOTH) divisor = 10000;

            int countGot = 0;
            for (auto *ir : job->items) {
                if (ir && ir->job_item_idx == (int32_t)i) countGot++;
            }

            ItemTypeInfo iti(item->item_type, item->item_subtype);

            os << "{\"index\":" << jsonInt((int)i)
               << ",\"item_type\":" << jsonStr(item->item_type == df::item_type::NONE
                                                    ? "NONE" : ENUM_KEY_STR(item_type, item->item_type))
               << ",\"item_subtype\":" << jsonInt(item->item_subtype)
               << ",\"item_subtype_name\":" << jsonStr(iti.toString())
               << ",\"mat_type\":" << jsonInt(item->mat_type)
               << ",\"mat_index\":" << jsonInt(item->mat_index)
               << ",\"material\":" << jsonStr(matinfo.isValid() ? matinfo.toString() : "")
               << ",\"is_any_inorganic\":" << (matinfo.isAnyInorganic() ? "true" : "false")
               << ",\"is_inorganic_wildcard\":" << (matinfo.isInorganicWildcard() ? "true" : "false")
               << ",\"is_wafers\":" << ((matinfo.inorganic && matinfo.inorganic->flags.is_set(df::inorganic_flags::WAFERS)) ? "true" : "false")
               << ",\"is_any_silk\":" << (item->flags2.bits.silk ? "true" : "false")
               << ",\"is_any_plant_fiber\":" << (item->flags2.bits.plant ? "true" : "false")
               << ",\"is_any_yarn\":" << (item->flags2.bits.yarn ? "true" : "false")
               << ",\"is_murdered_corpse\":" << (item->flags1.bits.murdered ? "true" : "false")
               << ",\"quantity_needed\":" << jsonInt(item->quantity < divisor ? item->quantity : item->quantity / divisor)
               << ",\"quantity_got\":" << jsonInt(countGot)
               << "}";
        }
        os << "]";

        os << "}";
    }
    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleNobleDemands enumerates every unit holding a noble/administrative
// position, via Units::getNoblePositions (modules/Units.h -- confirmed
// against this checkout's library/modules/Units.cpp: it walks the unit's
// historical figure's entity_links for a histfig_entity_link_positionst,
// resolving to the (entity, entity_position_assignment, entity_position)
// triple; no hand-walked link chain needed here). This is a COMPLETELY
// SEPARATE mechanism from list_mandates above (which only covers
// Export/Make/Guild violations): unit_demand (df.unit.xml struct
// 'unit_demand', original-name 'demandst', reached via df::unit's
// status.demands -- df.unit.xml's 'status' compound, the same one
// handleDwarfDetail already reads current_soul/labors from) is the literal
// "the baron demands a cabinet in his office, with a deadline" data, which
// nothing else exposes.
//
// entity_position's required_office/required_bedroom/required_dining/
// required_tomb (df.entity.xml original names req_room_THRONE/BEDROOM/
// DINING/TOMB) are room-VALUE minimums the position expects a room to meet,
// NOT tile counts; required_boxes/required_cabinets/required_racks/
// required_stands are furniture-item-count minimums. responsibilities is a
// bool[] indexed by entity_position_responsibility -- only true entries are
// emitted, same convention as handleDwarfDetail's labors[] below. name[0] is
// the position's singular display name (confirmed against Units.cpp's own
// get_noble_title, which indexes the same array by a 0=singular/1=plural
// index).
//
// unit_demand's place (enum demand_room: Office/Bedroom/DiningRoom/Tomb,
// original names THRONE/BEDROOM/DINING/TOMB) says which room category the
// demand concerns; item_type/item_subtype/mat_type/mat_index are decoded
// the same way list_mandates decodes the analogous mandate fields above.
// timeout_counter/timeout_limit is the same "counts once per 10 frames"
// deadline pair df.mandate.xml documents for mandates (df.unit.xml's own
// comment on unit_demand's fields is identical), so ticks_remaining uses
// the same x10 conversion as list_mandates' ticks_remaining.
//
// No satisfied/violated judgment is computed here -- the raw required_*
// minimums and the raw active-demand list are reported side by side,
// unresolved against each other, for the caller to reason about, per this
// project's data-not-rules philosophy.
static std::string handleNobleDemands(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::ostringstream os;
    os << "{\"nobles\":[";
    bool firstNoble = true;

    for (auto *u : df::global::world->units.active) {
        if (!u) continue;

        std::vector<Units::NoblePosition> positions;
        if (!Units::getNoblePositions(&positions, u) || positions.empty()) continue;

        if (!firstNoble) os << ",";
        firstNoble = false;

        os << "{\"unit_id\":" << jsonInt(u->id)
           << ",\"first_name\":" << jsonStr(u->name.first_name);

        os << ",\"positions\":[";
        bool firstPos = true;
        for (size_t i = 0; i < positions.size(); i++) {
            df::entity_position *p = positions[i].position;
            if (!p) continue;
            if (!firstPos) os << ",";
            firstPos = false;

            os << "{\"code\":" << jsonStr(p->code)
               << ",\"name\":" << jsonStr(p->name[0])
               << ",\"precedence\":" << jsonInt(p->precedence);

            os << ",\"responsibilities\":[";
            bool firstResp = true;
            for (int r = 0; r <= df::enum_traits<df::entity_position_responsibility>::last_item_value; r++) {
                if (!p->responsibilities[r]) continue;
                if (!firstResp) os << ",";
                firstResp = false;
                os << jsonStr(ENUM_KEY_STR(entity_position_responsibility, (df::entity_position_responsibility)r));
            }
            os << "]";

            os << ",\"required_office\":" << jsonInt(p->required_office)
               << ",\"required_bedroom\":" << jsonInt(p->required_bedroom)
               << ",\"required_dining\":" << jsonInt(p->required_dining)
               << ",\"required_tomb\":" << jsonInt(p->required_tomb)
               << ",\"required_boxes\":" << jsonInt(p->required_boxes)
               << ",\"required_cabinets\":" << jsonInt(p->required_cabinets)
               << ",\"required_racks\":" << jsonInt(p->required_racks)
               << ",\"required_stands\":" << jsonInt(p->required_stands)
               << "}";
        }
        os << "]";

        os << ",\"demands\":[";
        auto &demands = u->status.demands;
        bool firstDemand = true;
        for (auto *d : demands) {
            if (!d) continue;
            if (!firstDemand) os << ",";
            firstDemand = false;

            os << "{\"place\":" << jsonStr(ENUM_KEY_STR(demand_room, d->place));

            os << ",\"item_type\":" << jsonStr(d->item_type == df::item_type::NONE
                                                    ? "ANY" : ENUM_KEY_STR(item_type, d->item_type));
            os << ",\"item_subtype\":" << jsonInt(d->item_subtype);
            // Same ItemTypeInfo decode as list_mandates uses -- resolves
            // (item_type, item_subtype) to a raws-defined name with no
            // per-item-type table of our own; safe on out-of-range/-1
            // subtypes per the same bounds-checked Items::getSubtypeDef
            // path list_mandates' comment documents.
            ItemTypeInfo iti(d->item_type, d->item_subtype);
            os << ",\"item_subtype_name\":" << jsonStr(iti.toString());

            MaterialInfo mi((int16_t)d->mat_type, (int32_t)d->mat_index);
            os << ",\"material\":" << jsonStr(mi.isValid() ? mi.toString() : "");

            os << ",\"timeout_counter\":" << jsonInt(d->timeout_counter)
               << ",\"timeout_limit\":" << jsonInt(d->timeout_limit)
               << ",\"ticks_remaining\":"
               << jsonInt(10LL * ((int64_t)d->timeout_limit - (int64_t)d->timeout_counter))
               << "}";
        }
        os << "]";

        os << "}";
    }

    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handlePositionVacancies enumerates every entity_position DF has defined
// on the fort's own historical_entity (df::global::plotinfo->group_id --
// Manager/Bookkeeper/Broker/Sheriff/Captain of the Guard/militia-squad-
// leader positions) and the parent civilization's (plotinfo->civ_id --
// Monarch/Baron/Count/Duke succession positions), cross-referenced against
// positions.assignments to report each slot vacant or filled. This is the
// discovery surface appoint_position needs BEFORE calling it: which
// position codes exist, which slots are open, and whether DF's own
// possible_appointable/possible_elected caches (df.entity.xml's
// T_positions compound, since v0.40.01) consider a slot directly
// assignable at all -- MAYOR carries the ELECTED flag and is periodically
// re-elected, never player-appointed, per general DF community knowledge
// NOT independently verified against this checkout's raws (none bundled
// here) -- the possible_elected/possible_appointable membership below is
// the runtime-verifiable substitute for hardcoding that.
//
// This is the analogous data to noble_demands' positions[] block, but read
// from the ENTITY side instead of a held unit's side -- required since a
// VACANT slot has no unit to walk from (Units::getNoblePositions, which
// noble_demands uses, only ever resolves FROM a historical figure).
//
// possible_appointable/possible_elected membership is checked by POINTER
// identity against entity->positions.assignments (both vectors hold
// pointers into that same underlying set of entity_position_assignment
// objects) -- valid within this single read; no pointer is cached across
// calls.
static std::string handlePositionVacancies(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world or plotinfo is null");
    }

    std::ostringstream os;
    os << "{\"entities\":[";
    bool firstEntity = true;
    std::set<int32_t> seenEntityIDs;

    struct EntityRole { int32_t id; const char *role; };
    EntityRole roles[2] = {
        {df::global::plotinfo->group_id, "group"},
        {df::global::plotinfo->civ_id, "civ"},
    };

    for (auto &er : roles) {
        df::historical_entity *entity = df::historical_entity::find(er.id);
        if (!entity) continue;
        if (!seenEntityIDs.insert(entity->id).second) continue; // already emitted (civ_id==group_id edge case)

        if (!firstEntity) os << ",";
        firstEntity = false;

        os << "{\"entity_id\":" << jsonInt(entity->id)
           << ",\"role\":" << jsonStr(er.role);

        std::set<df::entity_position_assignment*> appointableSet(
            entity->positions.possible_appointable.begin(), entity->positions.possible_appointable.end());
        std::set<df::entity_position_assignment*> electedSet(
            entity->positions.possible_elected.begin(), entity->positions.possible_elected.end());

        os << ",\"positions\":[";
        bool firstPos = true;
        for (auto *p : entity->positions.own) {
            if (!p) continue;
            if (!firstPos) os << ",";
            firstPos = false;

            os << "{\"code\":" << jsonStr(p->code)
               << ",\"name\":" << jsonStr(p->name[0])
               << ",\"precedence\":" << jsonInt(p->precedence)
               << ",\"squad_size\":" << jsonInt(p->squad_size)
               << ",\"active\":" << (p->flags.is_set(df::entity_position_flags::ACTIVE) ? "true" : "false")
               << ",\"elected\":" << (p->flags.is_set(df::entity_position_flags::ELECTED) ? "true" : "false")
               << ",\"requires_market\":" << (p->flags.is_set(df::entity_position_flags::REQUIRES_MARKET) ? "true" : "false")
               << ",\"has_met_market_req\":" << (p->flags.is_set(df::entity_position_flags::HAS_MET_MARKET_REQ) ? "true" : "false")
               << ",\"requires_population\":" << jsonInt(p->requires_population)
               << ",\"has_met_pop_req\":" << (p->flags.is_set(df::entity_position_flags::HAS_MET_POP_REQ) ? "true" : "false");

            os << ",\"responsibilities\":[";
            bool firstResp = true;
            for (int r = 0; r <= df::enum_traits<df::entity_position_responsibility>::last_item_value; r++) {
                if (!p->responsibilities[r]) continue;
                if (!firstResp) os << ",";
                firstResp = false;
                os << jsonStr(ENUM_KEY_STR(entity_position_responsibility, (df::entity_position_responsibility)r));
            }
            os << "]";

            os << ",\"required_office\":" << jsonInt(p->required_office)
               << ",\"required_bedroom\":" << jsonInt(p->required_bedroom)
               << ",\"required_dining\":" << jsonInt(p->required_dining)
               << ",\"required_tomb\":" << jsonInt(p->required_tomb)
               << ",\"required_boxes\":" << jsonInt(p->required_boxes)
               << ",\"required_cabinets\":" << jsonInt(p->required_cabinets)
               << ",\"required_racks\":" << jsonInt(p->required_racks)
               << ",\"required_stands\":" << jsonInt(p->required_stands);

            os << ",\"assignments\":[";
            bool firstAssign = true;
            for (auto *a : entity->positions.assignments) {
                if (!a || a->position_id != p->id) continue;
                if (!firstAssign) os << ",";
                firstAssign = false;

                bool vacant = (a->histfig < 0);
                os << "{\"assignment_id\":" << jsonInt(a->id)
                   << ",\"vacant\":" << (vacant ? "true" : "false")
                   << ",\"appointable\":" << (appointableSet.count(a) ? "true" : "false")
                   << ",\"in_possible_elected\":" << (electedSet.count(a) ? "true" : "false")
                   << ",\"holder_hist_figure_id\":" << jsonInt(a->histfig);

                int32_t holderUnitID = -1;
                std::string holderName;
                if (!vacant) {
                    // Resolve to a currently-active citizen unit, if any --
                    // a holder may be a historical figure with no live unit
                    // at this fort (e.g. a monarch who never physically
                    // visits), in which case these stay -1/"".
                    for (auto *u : df::global::world->units.active) {
                        if (u && u->hist_figure_id == a->histfig) {
                            holderUnitID = u->id;
                            holderName = u->name.first_name;
                            break;
                        }
                    }
                }
                os << ",\"holder_unit_id\":" << jsonInt(holderUnitID)
                   << ",\"holder_name\":" << jsonStr(holderName)
                   << "}";
            }
            os << "]";

            os << "}";
        }
        os << "]";

        os << "}";
    }

    os << "]}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleFortWealth reports overall fort wealth plus embark-relative age.
// df::global::plotinfo->tasks is df::entity_activity_statistics
// (original-name reportst, defined in df.report.xml) -- its 'wealth'
// compound (original-name prefix wealth_*) carries per-category int32
// totals. Field names/types confirmed against df.report.xml and the
// generated codegen/entity_activity_statistics.h in this checkout
// (T_wealth: total/weapons/armor/furniture/other/architecture/displayed/
// held/imported/offered/exported).
//
// fortress_age (df::global::plotinfo->fortress_age, original-name
// fortress_duration) is a separate top-level plotinfo field, not part of
// wealth. df.plotinfo.xml's own comment on it is "?; +1 per 10; used in
// first 2 migrant waves etc" -- i.e. even DFHack's structure-definition
// maintainers weren't fully certain of its tick semantics. Reported
// verbatim here, not rescaled to df-ai's own tick vocabulary the way
// list_mandates' timeout_counter/timeout_limit pair is, since that "+1
// per 10" relationship isn't confirmed the same way.
static std::string handleFortWealth(const std::string &args, uint8_t &status) {
    if (!df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("plotinfo is null");
    }

    auto &w = df::global::plotinfo->tasks.wealth;

    std::ostringstream os;
    os << "{\"wealth\":{"
       << "\"total\":" << jsonInt(w.total)
       << ",\"weapons\":" << jsonInt(w.weapons)
       << ",\"armor\":" << jsonInt(w.armor)
       << ",\"furniture\":" << jsonInt(w.furniture)
       << ",\"other\":" << jsonInt(w.other)
       << ",\"architecture\":" << jsonInt(w.architecture)
       << ",\"displayed\":" << jsonInt(w.displayed)
       << ",\"held\":" << jsonInt(w.held)
       << ",\"imported\":" << jsonInt(w.imported)
       << ",\"offered\":" << jsonInt(w.offered)
       << ",\"exported\":" << jsonInt(w.exported)
       << "}"
       << ",\"fortress_age\":" << jsonInt(df::global::plotinfo->fortress_age)
       << "}";

    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// Emotions cap for dwarf_detail's psyche.emotions -- df::unit_personality's
// emotions vector (df.personality.xml, original-name 'mood') can accumulate
// unboundedly over a long-lived dwarf's life; nothing in this checkout
// documents a size bound or a culling mechanism. 20 keeps a single
// dwarf_detail response bounded while still covering recent emotional
// history; psyche.emotions_total/emotions_cap are always reported alongside
// the (possibly truncated) emotions array, so a caller can tell there's
// more rather than silently losing that fact.
static const int DWARF_EMOTIONS_CAP = 20;

// personality_facet_type has exactly 50 named entries (LOVE_PROPENSITY..
// ART_INCLINED, confirmed by counting this checkout's df.d_basics.xml),
// numbered 0..49 (its NONE=-1 entry does not occupy a traits[] slot).
// traits[] is DFHack's static array over that same index-enum. Mirrors
// this file's existing LABOR_MAX_INDEX convention (protocol.h) -- a
// manually-confirmed inclusive bound rather than a runtime enum-count
// lookup, since neither this codebase nor the confirmed DFHack headers
// expose one for this enum.
static const int DWARF_FACET_MAX_INDEX = 49;

// How many of the 50 facets psyche.top_facets surfaces as "extremes" --
// the full array is deliberately NOT emitted (see the task this shipped
// under). A fixed cut by |deviation from 50|, not a per-facet significance
// judgment.
static const int DWARF_TOP_FACETS = 8;

// handleWellbeing enumerates every living citizen (Units::isCitizen --
// sane, non-dead, current-fort; the same filter burrows.cpp's allCitizens
// path and entities.cpp's roster-building already use) in ONE pass -- the
// single-query counterpart to dwarf_detail below, which costs one query
// per dwarf and does not scale to a large fort's citizen count. Reports,
// per citizen, all read off unit->status.current_soul->personality
// (df.personality.xml's unit_personality compound):
//   - stress (raw int32 'stress' field) and Units::getStressCategory(unit)
//     -- DFHack's own 0-6 banding over its documented stress_cutoffs table
//     (modules/Units.cpp), the same function dwarfmonitor.cpp/
//     manipulator.cpp use for their own stress coloring, reused here rather
//     than reinventing the thresholds. NOTE: this checkout's own
//     plugins/manipulator.cpp references a ".stress_level" field that does
//     NOT exist in this checkout's df.personality.xml (the real field is
//     'stress') -- that reference is stale/dead code in DFHack itself and
//     is not relied on anywhere in this file; 'stress' is confirmed instead
//     against modules/Units.cpp's own getStressCategory (which reads
//     personality.stress) and plugins/misery.cpp (a definitely-compiled
//     file that reads AND writes personality.emotions/type/thought/etc.
//     under these exact field names).
//   - current_focus/undistracted_focus (df.personality.xml: "weighted sum
//     of needs focus_level-s" over "usually number of needs multiplied by
//     4") and their ratio. Chosen over sorting each citizen's needs vector
//     for the N-most-negative entries: that per-need sort is O(needs log
//     needs) per citizen where this ratio is O(1) per citizen, and this
//     query is specifically the cheap roster-wide scan -- per-need detail
//     already exists on dwarf_detail's psyche.needs below for the
//     single-dwarf drill-down.
// A citizen can, in principle, lack a current_soul (df::unit_soul* is a
// pointer) -- reported as explicit nulls rather than skipped or guessed
// at, the same convention handleListMoods uses for a job's momentarily
// absent unit ref.
static std::string handleWellbeing(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::ostringstream os;
    os << "{\"citizens\":[";
    bool first = true;
    for (auto *unit : df::global::world->units.active) {
        if (!unit || !Units::isCitizen(unit)) continue;
        if (!first) os << ",";
        first = false;

        os << "{\"id\":" << jsonInt(unit->id)
           << ",\"first_name\":" << jsonStr(unit->name.first_name);

        if (unit->status.current_soul) {
            auto &p = unit->status.current_soul->personality;
            os << ",\"stress\":" << jsonInt(p.stress)
               << ",\"stress_category\":" << jsonInt(Units::getStressCategory(unit))
               << ",\"current_focus\":" << jsonInt(p.current_focus)
               << ",\"undistracted_focus\":" << jsonInt(p.undistracted_focus)
               << ",\"focus_ratio\":";
            if (p.undistracted_focus != 0) {
                os << jsonNum((double)p.current_focus / (double)p.undistracted_focus);
            } else {
                os << "null";
            }
        } else {
            os << ",\"stress\":null,\"stress_category\":null"
               << ",\"current_focus\":null,\"undistracted_focus\":null,\"focus_ratio\":null";
        }

        os << "}";
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

    // Death status -- DFHack's own Units::isDead() definition
    // (flags2.bits.killed || flags3.bits.ghostly; confirmed against this
    // checkout's library/modules/Units.cpp, DFHack 53.15-r2). A starved/
    // murdered/etc. dwarf's df::unit stays in units.active (still found by
    // the id lookup above) reporting a frozen last-known position -- this
    // flag is the only truthful signal that the unit is no longer alive;
    // do not infer death from position staleness or absence elsewhere.
    bool dead = Units::isDead(u);
    os << ",\"dead\":" << (dead ? "true" : "false");

    if (dead) {
        // Time of death, if this unit has a recorded death incident
        // (unit->counters.death_id -> df::incident; same field
        // scripts/entomb.lua reads for the identical purpose). Not every
        // dead unit has one (e.g. very old deaths, or civ-foreign units).
        df::incident *inc = df::incident::find(u->counters.death_id);
        if (inc) {
            os << ",\"death_year\":" << jsonInt(inc->event_year)
               << ",\"death_time\":" << jsonInt(inc->event_time);
        } else {
            os << ",\"death_year\":null,\"death_time\":null";
        }

        // Burial check -- scripts/entomb.lua's isEntombed() algorithm
        // exactly: a unit is buried only if EVERY entry in corpse_parts
        // resolves to an item currently held by a building_coffinst via a
        // BUILDING_HOLDER general_ref. corpse_parts entries survive item
        // destruction (per df.unit.xml's own comment), so a missing item
        // or a holder that isn't a coffin both mean "not (fully) buried."
        // unit->pos is never re-synced once dead (Units::getPosition has
        // no dead-unit special case) -- the coffin's own position is the
        // corpse's real current location once this is true.
        bool buried = !u->corpse_parts.empty();
        df::building *coffin = nullptr;
        for (int32_t itemId : u->corpse_parts) {
            df::item *item = df::item::find(itemId);
            df::general_ref *ref = item ? Items::getGeneralRef(item, df::general_ref_type::BUILDING_HOLDER) : nullptr;
            df::building *holder = ref ? ref->getBuilding() : nullptr;
            if (!holder || !strict_virtual_cast<df::building_coffinst>(holder)) {
                buried = false;
                break;
            }
            coffin = holder;
        }
        os << ",\"buried\":" << (buried ? "true" : "false");
        if (buried && coffin) {
            os << ",\"burial_position\":{\"x\":" << jsonInt(coffin->centerx)
               << ",\"y\":" << jsonInt(coffin->centery)
               << ",\"z\":" << jsonInt(coffin->z) << "}";
        }
    } else {
        os << ",\"death_year\":null,\"death_time\":null,\"buried\":false";
    }

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

    // Mood -- df::mood_type enum name (Fey/Secretive/.../None), same
    // ENUM_KEY_STR decode handleListMoods uses for the identical field
    // (unit->mood), kept consistent here rather than left as the bare int
    // this field used to be.
    os << ",\"mood\":" << jsonStr(u->mood == df::mood_type::None ? "None" : ENUM_KEY_STR(mood_type, u->mood));

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

    // psyche -- stress/needs/emotions/facets/values/dreams, all read off
    // unit->status.current_soul->personality (df.personality.xml). See
    // handleWellbeing's comment above for the field-name verification this
    // relies on (misery.cpp/Units.cpp confirm 'stress'/'emotions'/etc.;
    // manipulator.cpp's ".stress_level" is a stale reference not followed
    // here).
    if (u->status.current_soul) {
        auto &p = u->status.current_soul->personality;

        os << ",\"psyche\":{"
           << "\"stress\":" << jsonInt(p.stress)
           << ",\"stress_category\":" << jsonInt(Units::getStressCategory(u));

        // Full needs list -- need_type name + focus_level (climbs to 400
        // when the need is satisfied) + need_level (how fast focus_level
        // decays once negative), per df.personality.xml's own field
        // comments on personality_needst. Not capped: a citizen's needs
        // vector is small (bounded by need_type's ~29 values), unlike
        // emotions below.
        os << ",\"needs\":[";
        bool firstNeed = true;
        for (auto *n : p.needs) {
            if (!n) continue;
            if (!firstNeed) os << ",";
            firstNeed = false;
            os << "{\"need_type\":" << jsonStr(n->id == df::need_type::NONE ? "NONE" : ENUM_KEY_STR(need_type, n->id))
               << ",\"focus_level\":" << jsonInt(n->focus_level)
               << ",\"need_level\":" << jsonInt(n->need_level)
               << "}";
        }
        os << "]";

        // Emotions -- capped at DWARF_EMOTIONS_CAP, recent-first. Ordering
        // is NOT assumed from vector insertion order (nothing in
        // df.personality.xml documents that guarantee) -- each
        // personality_moodst entry carries its own year/year_tick
        // ("last_used_year"/"last_used_season_count"), so recency is
        // computed explicitly by sorting on those fields.
        {
            // Filter nulls before sorting (not after) -- the comparator
            // below dereferences unconditionally, and filtering post-sort
            // would risk an early break on a null encountered before the
            // cap is reached, silently under-reporting real entries behind
            // it.
            std::vector<df::personality_moodst *> sorted;
            sorted.reserve(p.emotions.size());
            for (auto *e : p.emotions) if (e) sorted.push_back(e);
            std::sort(sorted.begin(), sorted.end(), [](df::personality_moodst *a, df::personality_moodst *b) {
                if (a->year != b->year) return a->year > b->year;
                return a->year_tick > b->year_tick;
            });
            os << ",\"emotions_total\":" << jsonInt((int64_t)sorted.size())
               << ",\"emotions_cap\":" << jsonInt(DWARF_EMOTIONS_CAP)
               << ",\"emotions\":[";
            int nEmit = 0;
            bool firstEmotion = true;
            for (auto *e : sorted) {
                if (nEmit >= DWARF_EMOTIONS_CAP) break;
                nEmit++;
                if (!firstEmotion) os << ",";
                firstEmotion = false;

                // caption is a const char*, null for entries with no
                // item-attr (e.g. thought==None) -- df.personality.xml
                // gives 'caption' no default-value. divider always has a
                // real value (default-value='0' on the enum-attr itself).
                const char *caption = ENUM_ATTR(unit_thought_type, caption, e->thought);
                int8_t divider = ENUM_ATTR(emotion_type, divider, e->type);

                os << "{\"emotion\":" << jsonStr(e->type == df::emotion_type::ANYTHING ? "NONE" : ENUM_KEY_STR(emotion_type, e->type))
                   << ",\"strength\":" << jsonInt(e->strength)
                   << ",\"thought\":" << jsonStr(e->thought == df::unit_thought_type::None ? "None" : ENUM_KEY_STR(unit_thought_type, e->thought))
                   << ",\"thought_caption\":" << jsonStr(caption ? caption : "")
                   // subthought is circumstance_id, a union typed by which
                   // `thought` it belongs to (df.personality.xml's own
                   // comment: "should use circumstance_id type here" --
                   // DFHack itself still keeps this a raw int32_t, not
                   // decoded per-thought-type). Emitted raw, undeciphered.
                   << ",\"subthought\":" << jsonInt(e->subthought)
                   // Signed indicator per DF's own emotion_type 'divider'
                   // attribute -- negative marks a eustress/positive
                   // emotion, positive marks a distress/negative one, zero
                   // is neutral (confirmed against df.d_basics.xml's own
                   // emotion_type entries, e.g. CONTENTMENT=-8 vs
                   // AGITATION=4). Reported as DF's raw signed value, not
                   // relabeled into a "positive"/"negative" verdict string.
                   << ",\"divider\":" << jsonInt(divider)
                   << "}";
            }
            os << "]";
        }

        // Top facets by |deviation| from the neutral value 50 -- not the
        // full 50-entry array (see DWARF_TOP_FACETS/DWARF_FACET_MAX_INDEX
        // above).
        {
            struct FacetDev { int idx; int16_t value; int dev; };
            std::vector<FacetDev> facets;
            facets.reserve(DWARF_FACET_MAX_INDEX + 1);
            for (int i = 0; i <= DWARF_FACET_MAX_INDEX; i++) {
                int16_t v = p.traits[i];
                int dev = (int)v - 50;
                if (dev < 0) dev = -dev;
                facets.push_back(FacetDev{i, v, dev});
            }
            std::sort(facets.begin(), facets.end(), [](const FacetDev &a, const FacetDev &b) {
                return a.dev > b.dev;
            });
            os << ",\"top_facets\":[";
            for (int i = 0; i < DWARF_TOP_FACETS && i < (int)facets.size(); i++) {
                if (i) os << ",";
                os << "{\"facet\":" << jsonStr(ENUM_KEY_STR(personality_facet_type, (df::personality_facet_type)facets[i].idx))
                   << ",\"value\":" << jsonInt(facets[i].value)
                   << "}";
            }
            os << "]";
        }

        // Values -- full list (personality_valuest: value_type + strength).
        os << ",\"values\":[";
        bool firstValue = true;
        for (auto *v : p.values) {
            if (!v) continue;
            if (!firstValue) os << ",";
            firstValue = false;
            os << "{\"value_type\":" << jsonStr(v->type == df::value_type::NONE ? "NONE" : ENUM_KEY_STR(value_type, v->type))
               << ",\"strength\":" << jsonInt(v->strength)
               << "}";
        }
        os << "]";

        // Dreams -- full list (personality_goalst: goal_type + the
        // personality_goal_flag 'accomplished' bit). goal_type has no NONE
        // item (unlike need_type/value_type), so no null-name guard needed.
        os << ",\"dreams\":[";
        bool firstDream = true;
        for (auto *g : p.dreams) {
            if (!g) continue;
            if (!firstDream) os << ",";
            firstDream = false;
            os << "{\"goal_type\":" << jsonStr(ENUM_KEY_STR(goal_type, g->type))
               << ",\"accomplished\":" << (g->flags.bits.accomplished ? "true" : "false")
               << "}";
        }
        os << "]";

        os << "}";
    } else {
        os << ",\"psyche\":null";
    }

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

// stockpileCategoryList: the accepted-item groups of one stockpile, as a
// comma-joined list of the SAME 17 names the `stockpile` creation tool
// already speaks (df::stockpile_group_set's bitfield, mirrored Go-side by
// tools_action.go's stockpileGroups map) -- so what a roster reports back
// reads in the same vocabulary a caller would use to make another one.
// "all" and "none" are collapsed: 17 comma-separated names is noise, and a
// stockpile accepting nothing is a real state worth naming outright (it is
// the answer to "why is nothing being hauled here").
static std::string stockpileCategoryList(df::building_stockpilest *sp) {
    const auto &f = sp->settings.flags.bits;
    const std::pair<const char *, bool> groups[] = {
        {"animals", f.animals != 0},     {"food", f.food != 0},
        {"furniture", f.furniture != 0}, {"corpses", f.corpses != 0},
        {"refuse", f.refuse != 0},       {"stone", f.stone != 0},
        {"ammo", f.ammo != 0},           {"coins", f.coins != 0},
        {"bars_blocks", f.bars_blocks != 0}, {"gems", f.gems != 0},
        {"finished_goods", f.finished_goods != 0}, {"leather", f.leather != 0},
        {"cloth", f.cloth != 0},         {"wood", f.wood != 0},
        {"weapons", f.weapons != 0},     {"armor", f.armor != 0},
        {"sheet", f.sheet != 0},
    };
    const int kGroupCount = (int)(sizeof(groups) / sizeof(groups[0]));
    int on = 0;
    std::string out;
    for (const auto &g : groups) {
        if (!g.second) continue;
        on++;
        if (!out.empty()) out += ",";
        out += g.first;
    }
    if (on == kGroupCount) return "all";
    if (on == 0) return "none";
    return out;
}

// stockpileFillCounts: how full one stockpile actually is -- distinct tiles
// holding at least one loose item, and the total item count on them.
//
// Membership is PURE GEOMETRY. No item flag says "stockpiled": DFHack's own
// Buildings::StockpileIterator resolves it as on_ground + containsTile, and
// this mirrors that test (minus the iterator's skip of empty containers
// assigned to the pile, which would make an empty-bin tile read as
// unoccupied when it is physically taken). Buildings::containsTile is
// extent-bitmap aware, so an L-shaped stockpile is measured by its real
// tiles, not its bounding rectangle.
//
// Cost: the map blocks covering ONE stockpile's bbox, reading each block's
// own item-id vector -- the same cheap tier as floorItemCountAt below, and
// never world->items.all (that is stocks' price, paid on explicit call).
//
// Items inside a bin/barrel are excluded automatically: contained items are
// not on_ground. The container itself counts as one item, which is why
// every render of these numbers carries the "containers count as 1" caveat.
static void stockpileFillCounts(df::building *sp, int &occupiedTiles, int &itemCount) {
    occupiedTiles = 0;
    itemCount = 0;
    if (!sp) return;
    std::set<std::pair<int16_t, int16_t>> occupied;
    int32_t z = sp->z;
    for (int32_t by = sp->y1 & ~15; by <= sp->y2; by += 16) {
        for (int32_t bx = sp->x1 & ~15; bx <= sp->x2; bx += 16) {
            df::map_block *blk = Maps::getTileBlock(bx, by, z);
            if (!blk) continue;
            for (int32_t id : blk->items) {
                df::item *item = df::item::find(id);
                if (!item || !item->flags.bits.on_ground) continue;
                if (item->flags.bits.in_building || item->flags.bits.construction) continue;
                if ((int32_t)item->pos.z != z) continue;
                if (!Buildings::containsTile(sp, df::coord2d(item->pos.x, item->pos.y))) continue;
                itemCount++;
                occupied.insert(std::make_pair(item->pos.x, item->pos.y));
            }
        }
    }
    occupiedTiles = (int)occupied.size();
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
           << ",\"done\":" << (stage == maxStage ? "true" : "false");

        // Bridge raised/lowered state -- df::building_bridgest::gate_flags
        // (df.building.xml, confirmed live-usage via DFHack's own bundled
        // remotefortressreader/building_reader.cpp, which reads this exact
        // field for the identical purpose). Without this a raised bridge
        // (a wall/lid) is indistinguishable here from a lowered one (a
        // walkable floor) -- the model had to infer state from dwarf
        // behavior all of Fort #5 session 1.
        if (b->getType() == df::building_type::Bridge) {
            auto *bridge = strict_virtual_cast<df::building_bridgest>(b);
            if (bridge) {
                std::string bridgeState = bridge->gate_flags.bits.raising ? "raising"
                                         : bridge->gate_flags.bits.lowering ? "lowering"
                                         : bridge->gate_flags.bits.raised ? "raised"
                                         : "lowered";
                os << ",\"bridge_state\":" << jsonStr(bridgeState);
            }
        }

        // Stockpile identity + fill. A bare "Stockpile at (x,y,z)" line told
        // a caller nothing about whether the pile was empty, full, or
        // accepting anything at all -- a live session had to be TOLD by a
        // human that the fort needed more stockpile room. These fields turn
        // the periodic buildings sweep into a stockpile health check, with
        // enough identity (number, custom name) to tell two overlapping
        // piles apart. Additive JSON: an older Go client ignores them, and
        // an older plugin simply omits them.
        if (b->getType() == df::building_type::Stockpile) {
            auto *sp = strict_virtual_cast<df::building_stockpilest>(b);
            if (sp) {
                // countExtentTiles walks the extent bitmap (an L-shaped or
                // obstruction-pocked stockpile has fewer usable tiles than
                // its rectangle); the bbox area is the defval for a
                // stockpile with no extent structure.
                int bboxArea = (b->x2 - b->x1 + 1) * (b->y2 - b->y1 + 1);
                int tiles = Buildings::countExtentTiles(b, bboxArea);
                int occupied = 0, items = 0;
                stockpileFillCounts(b, occupied, items);
                os << ",\"sp_number\":" << jsonInt(sp->stockpile_number)
                   << ",\"sp_name\":" << jsonStr(b->name)
                   << ",\"sp_categories\":" << jsonStr(stockpileCategoryList(sp))
                   << ",\"sp_tiles\":" << jsonInt(tiles)
                   << ",\"sp_occupied\":" << jsonInt(occupied)
                   << ",\"sp_items\":" << jsonInt(items)
                   // Container ceilings + the live container roster. A pile
                   // whose max_bins/max_barrels are 0 can never hold more
                   // than one loose item per tile no matter how much floor it
                   // has -- exactly the state every DF-AI-created stockpile
                   // was silently in until placeStockpile started writing
                   // sp->storage (buildings.cpp). Surfacing the ceiling next
                   // to the fill turns that from something inferred by
                   // cross-referencing `stocks category=bin` against tile math
                   // into a one-glance read. container_item_id is DF's OWN
                   // bookkeeping of containers assigned to this pile
                   // (df.building.xml:978-983, original name b_id) -- free to
                   // read, nothing computed here. Additive JSON, same
                   // backward-compat shape as the sp_* fields above.
                   << ",\"sp_max_bins\":" << jsonInt(sp->storage.max_bins)
                   << ",\"sp_max_barrels\":" << jsonInt(sp->storage.max_barrels)
                   << ",\"sp_max_wheelbarrows\":" << jsonInt(sp->storage.max_wheelbarrows)
                   << ",\"sp_containers\":" << jsonInt((int)sp->storage.container_item_id.size());
            }
        }

        os << "}";
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

// kDangerousCarnivoreSize is the adultsize (creature_raw's cm^3-ish
// GENERAL_SIZE, species-level, not per-caste -- df.creature.xml) above
// which an otherwise-unflagged CARNIVORE caste is still treated as
// dangerous. Sits between "wolf-sized" (~50000, already caught directly by
// the LARGE_PREDATOR raw tag vanilla gives wolves) and "grizzly-sized"
// (~350000), so it only fires for big carnivores vanilla's own raws don't
// already flag explicitly.
static const int32_t kDangerousCarnivoreSize = 100000;

// classifyWildlifeDanger reports whether (race,caste) counts as dangerous
// under this project's classifier and, if so, why. Priority: an explicit
// LARGE_PREDATOR or AMBUSHPREDATOR raw tag first (the same signal DF's own
// UI warns players about), then a CARNIVORE caste whose species adultsize
// clears kDangerousCarnivoreSize. Units::casteFlagSet is bounds-checked
// against a missing/invalid race or caste index (getCasteRaw ->
// creature_raw::find + vector_get, both null-safe -- see Units.cpp:
// 1006-1008) so a malformed unit can never crash this, only read as
// harmless.
static bool classifyWildlifeDanger(int32_t race, int32_t caste, std::string &reason) {
    if (Units::casteFlagSet(race, caste, df::caste_raw_flags::LARGE_PREDATOR)) {
        reason = "large predator";
        return true;
    }
    if (Units::casteFlagSet(race, caste, df::caste_raw_flags::AMBUSHPREDATOR)) {
        reason = "ambush predator";
        return true;
    }
    if (Units::casteFlagSet(race, caste, df::caste_raw_flags::CARNIVORE)) {
        df::creature_raw *craw = df::creature_raw::find(race);
        if (craw && craw->adultsize >= kDangerousCarnivoreSize) {
            reason = "large carnivore (adultsize " + jsonInt(craw->adultsize) + ")";
            return true;
        }
    }
    reason.clear();
    return false;
}

// handleListWildlife answers `look`'s lens=wildlife overlay (danger
// awareness -- never the default view, see design doc): every animal on
// the map, tame or wild, with a truthful danger classification so a
// grazing pasture pony and a wandering grizzly don't render identically.
// Mirrors handleListBuildings' structure (optional {"z":int} filter, 200-
// entry cap) over df::global::world->units.active instead of buildings.
static std::string handleListWildlife(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    std::string zArg = jsonGetString(args, "z");
    bool hasZ = !zArg.empty();
    int64_t zFilter = hasZ ? jsonGetInt(args, "z", 0) : 0;

    std::ostringstream os;
    os << "{\"wildlife\":[";
    int count = 0;
    bool truncated = false;
    for (auto *unit : df::global::world->units.active) {
        if (!unit) continue;
        // Invalid/off-map position -- same sentinel entities.cpp:70 skips.
        if (unit->pos.x == -30000) continue;
        if (!Units::isAnimal(unit)) continue; // tame AND wild animals only
        // A dead unit's df::unit lingers at a frozen last-known position --
        // never report a corpse as a live actor (project rule, see
        // tools_percept.go's dwarf-marker skip for the identical reason).
        if (Units::isDead(unit)) continue;
        if (hasZ && (int64_t)unit->pos.z != zFilter) continue;
        if (count >= 200) { truncated = true; break; }

        std::string name;
        df::creature_raw *craw = df::creature_raw::find(unit->race);
        if (craw) name = craw->name[0];

        std::string reason;
        bool dangerous = classifyWildlifeDanger(unit->race, unit->caste, reason);

        if (count) os << ",";
        os << "{\"id\":" << jsonInt(unit->id)
           << ",\"x\":" << jsonInt(unit->pos.x)
           << ",\"y\":" << jsonInt(unit->pos.y)
           << ",\"z\":" << jsonInt(unit->pos.z)
           << ",\"tame\":" << (Units::isFortControlled(unit) ? "true" : "false")
           << ",\"name\":" << jsonStr(name)
           << ",\"dangerous\":" << (dangerous ? "true" : "false");
        if (dangerous) os << ",\"danger_reason\":" << jsonStr(reason);
        os << "}";
        count++;
    }
    os << "]";
    if (truncated) os << ",\"truncated\":true";
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// Dangerous-wildlife tripwire tracking. Guarded by g_known_wildlife_mutex
// because checkWildlifeTripwire runs from the same two thread contexts
// announcements.cpp's own tripwire plumbing does -- the main thread's
// throttled plugin_onupdate poll AND the socket thread's
// drain_from_socket_thread path (ConditionalCoreSuspender-guarded) -- the
// identical reasoning behind announcements.cpp's g_repeat_count_mutex.
static std::mutex g_known_wildlife_mutex;
static std::unordered_set<int32_t> g_known_dangerous_wildlife_ids;

// checkWildlifeTripwire walks df::global::world->units.active once looking
// for a dangerous WILD unit -- classifyWildlifeDanger's LARGE_PREDATOR/
// AMBUSHPREDATOR/big-CARNIVORE test, same as handleListWildlife above --
// that has not already been seen since the last reset. Units::isFortControlled
// excludes tame/pastured animals: a caste flagged LARGE_PREDATOR that the
// fort has since tamed is not a threat, even though its raw tags don't
// change. Returns a truthful auto-pause reason for the FIRST newly-seen
// dangerous unit, "" if none are new. Marks EVERY currently-visible
// dangerous wild unit as known within this SAME pass, not just the first --
// this is what stops the tripwire's own push_state_refresh
// (trip_step_tripwire, df_ai_protocol.cpp) from recursively re-tripping on
// a still-visible predator when it calls back into the announcement poll
// that invokes this function. External linkage: called from
// poll_and_send_announcements (announcements.cpp).
std::string checkWildlifeTripwire()
{
    if (!df::global::world) return "";

    std::lock_guard<std::mutex> lock(g_known_wildlife_mutex);
    std::string reasonOut;
    for (auto *unit : df::global::world->units.active) {
        if (!unit) continue;
        if (unit->pos.x == -30000) continue;  // invalid/off-map position
        if (!Units::isAnimal(unit)) continue;
        if (Units::isDead(unit)) continue;
        if (Units::isFortControlled(unit)) continue; // tame/owned -- never a threat

        std::string reason;
        if (!classifyWildlifeDanger(unit->race, unit->caste, reason)) continue;

        bool isNew = g_known_dangerous_wildlife_ids.insert(unit->id).second;
        if (isNew && reasonOut.empty()) {
            std::string name;
            df::creature_raw *craw = df::creature_raw::find(unit->race);
            if (craw) name = craw->name[0];
            char buf[256];
            snprintf(buf, sizeof(buf),
                     "dangerous wildlife sighted: %s (%s) at (%d,%d,%d)",
                     name.empty() ? "unknown creature" : name.c_str(),
                     reason.c_str(), unit->pos.x, unit->pos.y, unit->pos.z);
            reasonOut = buf;
        }
    }
    return reasonOut;
}

// reset_wildlife_tripwire_tracking clears the known-dangerous-unit set on
// disconnect/reconnect. Mirrors reset_announcement_cursor's reasoning
// exactly (announcements.cpp): a still-visible predator re-trips once after
// reconnect rather than staying silently suppressed forever, since the
// reconnected peer has no memory of it either. Call from
// close_socket_and_reset() (df_ai_protocol.cpp), alongside the existing
// reset_announcement_cursor() call.
void reset_wildlife_tripwire_tracking()
{
    std::lock_guard<std::mutex> lock(g_known_wildlife_mutex);
    g_known_dangerous_wildlife_ids.clear();
}

// Forward declaration -- implemented in locations.cpp (Task 1).
uint8_t wireFromAbstractBuildingType(df::abstract_building_type t);

// Forward declaration -- implemented in burrows.cpp (uses Burrows/df::burrow
// types that file already includes; same forward-declared-elsewhere pattern
// as wireFromAbstractBuildingType above).
std::string handleListBurrows(const std::string &args, uint8_t &status);

// Forward declaration -- implemented in military.cpp (uses df::squad/
// Military types that file already includes; same forward-declared-
// elsewhere pattern as handleListBurrows above).
std::string handleListSquads(const std::string &args, uint8_t &status);

// Forward declaration -- implemented in narrative.cpp (Feature 013 wave
// 013-A: dwarf_portrait's psyche/social/history struct reads live behind
// their own dedicated includes there rather than adding another dozen
// df/*.h includes to this already-large file; same forward-declared-
// elsewhere pattern as handleListSquads above).
std::string handleDwarfPortrait(const std::string &args, uint8_t &status);

// Forward declarations -- implemented in narrative.cpp (Feature 013 wave
// 013-A / Lane 2: combat narrator). combat_summary/combat_log read the
// rolling combat-report buffer ingest_combat_reports() (also narrative.cpp)
// maintains; that ingest is called from push_state_refresh
// (df_ai_protocol.cpp), NOT from here -- these two are read-only query
// handlers over already-ingested state, same division of labor as
// checkWildlifeTripwire (state-updater) vs handleListWildlife (reader).
std::string handleCombatSummary(const std::string &args, uint8_t &status);
std::string handleCombatLog(const std::string &args, uint8_t &status);

// Forward declarations -- implemented in narrative.cpp (Feature 013 wave
// 013-B / Lane 3: story pulse + social graph). story_pulse maintains its
// own (year,year_tick) cursor + remembered-set; social_graph is a pure
// per-call struct read with no state of its own. Same forward-declared-
// elsewhere pattern as dwarf_portrait/combat_summary above.
std::string handleStoryPulse(const std::string &args, uint8_t &status);
std::string handleSocialGraph(const std::string &args, uint8_t &status);

// Forward declaration -- implemented in narrative.cpp (Feature 013 wave
// 013-C / Lane 4: the fort's own art). fort_art maintains its own
// (world.history.events max-id) cursor + composed-here id cache, same
// forward-declared-elsewhere pattern as story_pulse/social_graph above.
std::string handleFortArt(const std::string &args, uint8_t &status);

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
        if (wireKind == 0) continue; // not one of the 5 supported types
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

        // Dedication -- Temple only. create_location can WRITE a temple's
        // deity_type/deity_data (locations.cpp applyCreateLocation) and its ACK
        // truthfully calls that write UNVERIFIED, telling the caller to confirm
        // in-game -- an instruction the playing model, which has no in-game
        // eyes, cannot follow. Reading the two fields back here turns "trust
        // the ACK" into a check the model can perform in one call. The fields
        // are df::abstract_building_templest's religious_practice_type
        // `deity_type` plus the religious_practice_data UNION `deity_data`
        // (df.abstract_building.xml:235-246) -- the union member is chosen BY
        // deity_type, so reading Deity when the type says RELIGION_ENID would
        // report a historical_entity id as a historical figure. Additive JSON,
        // same backward-compat shape as the sp_* fields above.
        if (bld->getType() == df::abstract_building_type::TEMPLE) {
            auto *temple = strict_virtual_cast<df::abstract_building_templest>(bld);
            if (temple) {
                os << ",\"deity_type\":"
                   << jsonStr(ENUM_KEY_STR(religious_practice_type, temple->deity_type));
                if (temple->deity_type == df::religious_practice_type::WORSHIP_HFID) {
                    os << ",\"deity_hf_id\":" << jsonInt(temple->deity_data.Deity);
                    if (df::historical_figure *deityHf =
                            df::historical_figure::find(temple->deity_data.Deity)) {
                        std::string deityName = Translation::translateName(&deityHf->name, true);
                        if (!deityName.empty()) os << ",\"deity_name\":" << jsonStr(deityName);
                    }
                } else if (temple->deity_type == df::religious_practice_type::RELIGION_ENID) {
                    os << ",\"deity_entity_id\":" << jsonInt(temple->deity_data.Religion);
                    if (df::historical_entity *religion =
                            df::historical_entity::find(temple->deity_data.Religion)) {
                        std::string religionName = Translation::translateName(&religion->name, true);
                        if (!religionName.empty()) os << ",\"deity_name\":" << jsonStr(religionName);
                    }
                }
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

// kNeverFortStockFlags marks an item as never legitimate fort stock at
// all, regardless of what else it's doing: caravan/hostile-owned,
// forbidden, marked for dumping, garbage-collected, on fire, rotten, or an
// artifact. Shared between handleListCrops's seed tally and
// handleStockpileInventory's free/in-use split below so the two masks
// don't drift apart.
static const uint32_t kNeverFortStockFlags =
    (uint32_t)df::item_flags::Mask::mask_dump |
    (uint32_t)df::item_flags::Mask::mask_forbid |
    (uint32_t)df::item_flags::Mask::mask_garbage_collect |
    (uint32_t)df::item_flags::Mask::mask_hostile |
    (uint32_t)df::item_flags::Mask::mask_on_fire |
    (uint32_t)df::item_flags::Mask::mask_rotten |
    (uint32_t)df::item_flags::Mask::mask_trader |
    (uint32_t)df::item_flags::Mask::mask_artifact;

// itemStackUnits: how many actual servings/units a single df::item struct
// represents -- handleStockpileInventory's existing tally counts STRUCTS
// (one entry per stack), which silently undercounts anything that stacks
// (df::item_actual::stack_size, original name 'amount'): 8 wine item
// structs can be 8 nearly-empty stacks or 8 full ones (~200 servings), and
// the struct-count tally alone can't tell those apart -- a live discrepancy
// between the tool's "8" and the in-game stock screen's much larger number.
// DRINK/FOOD/AMMO/POWDER_MISC/MEAT/FISH/EGG/PLANT/GLOB/CHEESE/SEEDS/BAR all
// inherit item_actual (df.item.xml); strict_virtual_cast returns null for
// anything that doesn't (bins, containers, artifacts, buildings-as-items),
// which correctly falls back to 1 unit per struct -- same convention as
// handleZoneValue's building_actual cast below. Clamped to >=1: a
// structurally-real item is at least one physical unit regardless of what
// stack_size happens to hold.
static int32_t itemStackUnits(df::item *it) {
    if (auto *ia = strict_virtual_cast<df::item_actual>(it))
        return std::max<int32_t>(1, ia->stack_size);
    return 1;
}

// handleListCrops enumerates plantable crops (plant raws carrying the SEED
// flag) for the build_farm_plot/assign_crop workflow -- each entry's
// "token" round-trips verbatim into SET_FARM_CROP's CropName field
// (buildings.cpp resolvePlantRaw matches it case-insensitively, same as
// "name"). Filterable by substring against either field, capped like
// list_reactions above. Seed-on-hand tally mirrors DFHack's autofarm
// plugin (plugins/autofarm.cpp find_plantable_plants): live, non-forbidden
// SEEDS items tallied by mat_index (== plant raw index).
static std::string handleListCrops(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    std::string filter = jsonGetString(args, "filter");
    std::string lfilter = filter;
    for (auto &c : lfilter) c = tolower(c);

    // Same bad-flags mask and vector as autofarm's find_plantable_plants --
    // dumped/forbidden/rotten/etc seeds don't count as usable stock.
    const uint32_t badFlags = kNeverFortStockFlags |
        (uint32_t)df::item_flags::Mask::mask_in_building |
        (uint32_t)df::item_flags::Mask::mask_construction;
    std::map<int32_t, int32_t> seedCounts;
    for (auto *item : df::global::world->items.other[df::items_other_id::SEEDS]) {
        auto *seed = strict_virtual_cast<df::item_seedsst>(item);
        if (seed && (seed->flags.whole & badFlags) == 0)
            seedCounts[seed->mat_index] += seed->stack_size;
    }

    std::ostringstream os;
    os << "{\"crops\":[";
    int count = 0;
    bool truncated = false;
    for (df::plant_raw *raw : df::global::world->raws.plants.all) {
        if (!raw) continue;
        if (!raw->flags.is_set(df::plant_raw_flags::SEED)) continue;

        std::string ltoken = raw->id;
        for (auto &c : ltoken) c = tolower(c);
        std::string lname = raw->name;
        for (auto &c : lname) c = tolower(c);
        if (!lfilter.empty() &&
            ltoken.find(lfilter) == std::string::npos &&
            lname.find(lfilter) == std::string::npos) {
            continue;
        }

        if (count >= 200) { truncated = true; break; }
        if (count) os << ",";
        count++;

        auto it = seedCounts.find(raw->index);
        int32_t seedsOnHand = (it != seedCounts.end()) ? it->second : 0;

        os << "{\"token\":" << jsonStr(raw->id)
           << ",\"name\":" << jsonStr(raw->name)
           << ",\"underground\":" << (raw->underground_depth_max > 0 ? "true" : "false")
           << ",\"seeds_on_hand\":" << jsonInt(seedsOnHand) << "}";
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
    //
    // freeCounts is available stock; inUseCounts is the same item/material
    // already incorporated into a building (in_building -- a bed in a
    // bedroom, a door in a doorway) or a construction (a boulder mortared
    // into a wall). Both are still live df::item objects, so a naive single
    // tally silently conflates "free spare" with "already used" -- that was
    // exactly the bug that caused invisible needs-bed/needs-door cancel
    // loops upstream (caller sees count>0 and assumes a spare exists).
    // Junk items (caravan/forbidden/dumped/etc, kNeverFortStockFlags above)
    // don't belong in either tally.
    std::map<std::tuple<int, int, int>, int> freeCounts;  // (item_type, mat_type, mat_index) → count
    std::map<std::tuple<int, int, int>, int> inUseCounts;
    // Additive sibling of freeCounts/inUseCounts above: same keys, but
    // summing itemStackUnits() (real serving/unit count) instead of
    // incrementing by one per struct -- see itemStackUnits' doc comment.
    std::map<std::tuple<int, int, int>, int> freeUnits;
    std::map<std::tuple<int, int, int>, int> inUseUnits;

    // qualityCounts tallies (item_type, item_quality) → count across every
    // non-junk item, free or in-use alike -- a built bed's craftsmanship is
    // just as real as a spare one's. getQuality() (df/item.h vmethod,
    // original name get_craftquality) is defined on the base df::item
    // class itself and defaults to returning 0 (== item_quality::Ordinary)
    // for item types that carry no craftsmanship concept at all (boulders,
    // seeds, etc), so it's safe to call unconditionally with no subtype
    // cast. typesWithQuality records which item types have at least one
    // item above Ordinary -- the quality breakdown below is only emitted
    // for those, so a stockpile full of boulders (uniformly Ordinary)
    // doesn't pad every response with a meaningless "N ordinary" line.
    std::map<std::tuple<int, int>, int> qualityCounts;  // (item_type, quality) → count
    std::set<int> typesWithQuality;

    // subtypeCounts tallies (item_type, mat_type, mat_index, item_subtype) →
    // count, WEAPON and TOOL only -- the two item types where the aggregated
    // key above ("WEAPON: iron x3") collapses away exactly the distinction
    // that matters (a pick vs. a battle axe), which drove a live wrong-theory
    // session ("no axes in stock") when there was no way to tell which iron
    // weapons those actually were. Free+in-use together, same as
    // qualityCounts -- a mounted weapon's subtype is just as real as a spare
    // one's. Additive sibling array ("subtypes"), NOT folded into the
    // aggregated key above, so the default compact view is unchanged; only
    // the category-filtered detailed render (Go side) uses it.
    std::map<std::tuple<int, int, int, int>, int> subtypeCounts;

    // containerFreeCount/containerFreeEmpty: additive siblings of freeCounts
    // above, tallied only for the fixed set of storage-vessel item types a
    // job actually needs an EMPTY one of -- BARREL/BUCKET/BIN/BAG outright,
    // plus any TOOL item whose raws-defined itemdef carries the
    // FOOD_STORAGE tool_use (a "pot"-style vessel some raws/mods add as a
    // TOOL rather than a dedicated item_type). This is the live friction
    // this exists to fix: "stocks shows 15 barrels" told a caller nothing
    // about whether ANY of them were actually free to hold a new batch --
    // brewing kept getting cancelled ("needs empty food storage item")
    // against a pile that was invisibly all full. "Empty" mirrors DFHack's
    // own scripts/empty-bin.lua definition (dfhack.items.getContainedItems
    // returns none) via the CONTAINS_ITEM general_ref -- a barrel already
    // holding a partial batch of drink is not empty just because "free"
    // (not in_building/construction) says it's spare. Also excludes items
    // already flags.bits.in_job: a bucket a hauling job is mid-carrying
    // toward a still isn't actually available for the NEXT job to claim
    // either, and that flag is already loaded per-item at the same cost as
    // the in_building/construction check just above -- the cheap half of
    // "and not job-reserved" (a full cross-reference against every job's
    // job_items, to see whether some OTHER not-yet-dispatched job has its
    // eye on this exact container, would mean walking every job in the
    // fort per container and was judged not cheap enough to add here).
    // Free-population only (matches freeCounts, not inUseCounts): a
    // container flagged in_building/construction essentially never happens
    // for these item types in practice, and the numbers this feeds are
    // meant to read directly against the same "count" a caller already
    // sees for that (item_type, material) key.
    std::map<std::tuple<int, int, int>, int> containerFreeCount;
    std::map<std::tuple<int, int, int>, int> containerFreeEmpty;

    auto &items = df::global::world->items.all;
    for (auto *it : items) {
        if (!it) continue;
        if (it->flags.whole & kNeverFortStockFlags) continue;

        auto key = std::make_tuple((int)it->getType(),
                                   (int)it->getActualMaterial(),
                                   (int)it->getActualMaterialIndex());
        int32_t units = itemStackUnits(it);
        bool inUse = it->flags.bits.in_building || it->flags.bits.construction;
        if (inUse) {
            inUseCounts[key]++;
            inUseUnits[key] += units;
        } else {
            freeCounts[key]++;
            freeUnits[key] += units;
        }

        int itype = (int)it->getType();
        int quality = (int)it->getQuality();
        qualityCounts[std::make_tuple(itype, quality)]++;
        if (quality > (int)df::item_quality::Ordinary) typesWithQuality.insert(itype);

        if (itype == (int)df::item_type::WEAPON || itype == (int)df::item_type::TOOL) {
            subtypeCounts[std::make_tuple(itype, (int)it->getActualMaterial(),
                                          (int)it->getActualMaterialIndex(),
                                          (int)it->getSubtype())]++;
        }

        if (!inUse) {
            bool isVessel = itype == (int)df::item_type::BARREL ||
                            itype == (int)df::item_type::BUCKET ||
                            itype == (int)df::item_type::BIN ||
                            itype == (int)df::item_type::BAG;
            if (!isVessel && itype == (int)df::item_type::TOOL) {
                df::itemdef_toolst *toolDef = vector_get(
                    df::global::world->raws.itemdefs.tools,
                    (unsigned)it->getSubtype(), (df::itemdef_toolst *)nullptr);
                if (toolDef) {
                    for (df::tool_uses use : toolDef->tool_use) {
                        if (use == df::tool_uses::FOOD_STORAGE) { isVessel = true; break; }
                    }
                }
            }
            if (isVessel) {
                containerFreeCount[key]++;
                bool hasContents = Items::getGeneralRef(it, df::general_ref_type::CONTAINS_ITEM) != nullptr;
                if (!hasContents && !it->flags.bits.in_job) containerFreeEmpty[key]++;
            }
        }
    }

    std::set<std::tuple<int, int, int>> allKeys;
    for (auto &kv : freeCounts) allKeys.insert(kv.first);
    for (auto &kv : inUseCounts) allKeys.insert(kv.first);

    std::ostringstream os;
    os << "{\"items\":[";
    bool first = true;
    for (auto &key : allKeys) {
        df::item_type itype = (df::item_type)std::get<0>(key);
        std::string typeName = ENUM_KEY_STR(item_type, itype);
        if (!category.empty() && typeName.find(category) == std::string::npos) continue;
        // Human-readable material name via MaterialInfo (state_name at room
        // temperature, e.g. "shale"). Empty string when the pair doesn't
        // decode (e.g. materialless items).
        MaterialInfo mi((int16_t)std::get<1>(key), (int32_t)std::get<2>(key));
        std::string matName = mi.isValid() ? mi.toString() : "";
        auto freeIt = freeCounts.find(key);
        auto inUseIt = inUseCounts.find(key);
        int freeCount = (freeIt != freeCounts.end()) ? freeIt->second : 0;
        int inUseCount = (inUseIt != inUseCounts.end()) ? inUseIt->second : 0;
        // units/in_use_units: additive siblings of count/in_use above,
        // summing itemStackUnits() instead of counting structs -- see that
        // function's doc comment. Present for every item type (not just
        // stackable ones); for a non-stacking type units==count, which the
        // Go renderer uses to decide whether showing the units number adds
        // any information worth a caller's attention.
        auto freeUnitsIt = freeUnits.find(key);
        auto inUseUnitsIt = inUseUnits.find(key);
        int freeUnitCount = (freeUnitsIt != freeUnits.end()) ? freeUnitsIt->second : 0;
        int inUseUnitCount = (inUseUnitsIt != inUseUnits.end()) ? inUseUnitsIt->second : 0;
        if (!first) os << ",";
        first = false;
        os << "{\"item_type\":" << jsonStr(typeName)
           << ",\"material\":" << jsonStr(matName)
           << ",\"count\":" << jsonInt(freeCount)
           << ",\"in_use\":" << jsonInt(inUseCount)
           << ",\"units\":" << jsonInt(freeUnitCount)
           << ",\"in_use_units\":" << jsonInt(inUseUnitCount);
        // containers/empty: only present when this key had at least one
        // storage-vessel item tallied above (containerFreeCount is only
        // incremented for BARREL/BUCKET/BIN/BAG or a FOOD_STORAGE-flagged
        // TOOL) -- absent for every other item_type, same conditional-field
        // convention as "economic" below. "containers" can be LESS than
        // "count" for TOOL: that key aggregates every tool of this material
        // (picks included), while containers/empty describe only the
        // food-storage-vessel subset of it.
        auto containerIt = containerFreeCount.find(key);
        if (containerIt != containerFreeCount.end()) {
            auto containerEmptyIt = containerFreeEmpty.find(key);
            int containerEmptyCount = (containerEmptyIt != containerFreeEmpty.end()) ? containerEmptyIt->second : 0;
            os << ",\"containers\":" << jsonInt(containerIt->second)
               << ",\"empty\":" << jsonInt(containerEmptyCount);
        }
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
    os << "]";

    // Additive quality breakdown -- same category substring filter as the
    // items array above, one entry per (item_type, quality tier) that
    // actually has an item in it. Response shape is unchanged for any
    // caller that only reads "items"; this is a new sibling key.
    os << ",\"quality\":[";
    bool firstQ = true;
    for (int itype : typesWithQuality) {
        std::string typeName = ENUM_KEY_STR(item_type, (df::item_type)itype);
        if (!category.empty() && typeName.find(category) == std::string::npos) continue;
        for (int q = (int)df::item_quality::Ordinary; q <= (int)df::item_quality::Artifact; q++) {
            auto qIt = qualityCounts.find(std::make_tuple(itype, q));
            if (qIt == qualityCounts.end()) continue;
            if (!firstQ) os << ",";
            firstQ = false;
            os << "{\"item_type\":" << jsonStr(typeName)
               << ",\"quality\":" << jsonStr(ENUM_KEY_STR(item_quality, (df::item_quality)q))
               << ",\"count\":" << jsonInt(qIt->second) << "}";
        }
    }
    os << "]";

    // Additive WEAPON/TOOL subtype breakdown -- same category substring
    // filter as above. Reuses the same ItemTypeInfo decode/toString the
    // mandates/moods handlers already rely on for (item_type, item_subtype)
    // → raws-defined name (e.g. "iron pick" vs "iron battle axe"), bounds-
    // checked the same way (safe on any subtype value a real item can carry).
    os << ",\"subtypes\":[";
    bool firstSub = true;
    for (auto &kv : subtypeCounts) {
        int itype = std::get<0>(kv.first);
        std::string typeName = ENUM_KEY_STR(item_type, (df::item_type)itype);
        if (!category.empty() && typeName.find(category) == std::string::npos) continue;
        MaterialInfo mi((int16_t)std::get<1>(kv.first), (int32_t)std::get<2>(kv.first));
        std::string matName = mi.isValid() ? mi.toString() : "";
        ItemTypeInfo iti((df::item_type)itype, (int16_t)std::get<3>(kv.first));
        if (!firstSub) os << ",";
        firstSub = false;
        os << "{\"item_type\":" << jsonStr(typeName)
           << ",\"material\":" << jsonStr(matName)
           << ",\"subtype_name\":" << jsonStr(iti.toString())
           << ",\"count\":" << jsonInt(kv.second) << "}";
    }
    os << "]";

    // O(1) self-describing capability flag, same sibling-of-the-payload
    // convention the truncated flags elsewhere in this file use. The Go
    // renderer's own no-data detection is a nil scan over the items (it must
    // keep working against plugin builds that predate this flag), but a
    // single explicit top-level "yes, this build measures stack sizes" is
    // both cheaper to read and immune to a future regression that stops
    // populating "units" for some new item type without removing the field
    // wholesale.
    os << ",\"stack_units\":true";

    os << "}";
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

// isSmoothedAt: true when tt carries DF's SMOOTH/SMOOTH_DEAD tiletype_special
// variant -- the only DF-side signal a wall/floor has been smoothed; shape
// and material are identical before/after (StoneWall vs StoneWallSmoothLR
// both report shape=WALL material=STONE, differing only in `special`).
// DF also stamps special=SMOOTH on every player-built Construction tile
// (ConstructedFloor/Wall/Pillar, material=CONSTRUCTION) -- those are not
// dwarf-smoothed and must be excluded, or every built wall/floor (including
// aquifer seals) would misreport as smoothed.
static bool isSmoothedAt(df::tiletype tt) {
    using Sp = df::tiletype_special;
    df::tiletype_special sp = tileSpecial(tt);
    if (sp != Sp::SMOOTH && sp != Sp::SMOOTH_DEAD) return false;
    return tileMaterial(tt) != df::tiletype_material::CONSTRUCTION;
}

// floorItemCountAt: per-tile count of items genuinely at rest on open
// floor -- on_ground (DF's own per-tile "item sitting here" flag,
// flipped by MapExtras::Block::addItemOnGround/removeItemOnGround) and
// NOT yet absorbed into a building or construction (in_building/
// construction -- handleStockpileInventory above already tallies those
// separately; DF keeps on_ground true for those too, see gui/autodump.lua's
// identical exclusion in the DFHack scripts tree, so plain itemCountAt
// can't be used unfiltered here). Deliberately does NOT exclude
// kNeverFortStockFlags junk (forbidden, dumped, rotten, etc.) -- those
// items still physically sit on the tile and can still trigger DF's own
// site_blocked construction stall (MapExtras::MapCache::removeItemOnGround).
//
// Filtered per 16x16 block on first touch, mirroring MapCache.cpp's own
// init_item_counts lazy-per-block pattern (the same one backing
// MapExtras::Block::itemCountAt) via the Block's raw item list, so a
// map_slice/column_profile call only ever scans the blocks it actually
// visits -- never world->items.all.
// Coarse item classes for the items lens. DF has ~90 df::item_type values;
// a per-view footnote can afford roughly a dozen names, so types collapse
// into the families a fort actually reasons about ("that pile is stone" /
// "that pile is food"). This mapping is editorial by construction -- a
// class carved wrong costs a mislabeled letter, never data loss -- and the
// per-tile item COUNT is always exact regardless of how it classifies.
enum FloorItemClass {
    ITEMCLASS_STONE = 0,
    ITEMCLASS_WOOD,
    ITEMCLASS_FOOD,
    ITEMCLASS_FURNITURE,
    ITEMCLASS_BARS_BLOCKS,
    ITEMCLASS_CLOTH_LEATHER,
    ITEMCLASS_WEAPON_ARMOR,
    ITEMCLASS_FINISHED_GOODS,
    ITEMCLASS_REFUSE,
    ITEMCLASS_GEMS,
    ITEMCLASS_CONTAINERS,
    ITEMCLASS_TOOLS,
    ITEMCLASS_OTHER,
    ITEMCLASS_COUNT
};

static const char *kFloorItemClassNames[ITEMCLASS_COUNT] = {
    "stone", "wood", "food/drink", "furniture", "bars/blocks", "cloth/leather",
    "weapons/armor", "finished goods", "refuse/corpse", "gems", "containers",
    "tools", "other"
};

static int floorItemClassOf(df::item *it) {
    switch (it->getType()) {
        case df::item_type::BOULDER:
        case df::item_type::ROCK:
            return ITEMCLASS_STONE;
        case df::item_type::WOOD:
        case df::item_type::BRANCH:
            return ITEMCLASS_WOOD;
        case df::item_type::MEAT:
        case df::item_type::FISH:
        case df::item_type::FISH_RAW:
        case df::item_type::EGG:
        case df::item_type::CHEESE:
        case df::item_type::FOOD:
        case df::item_type::DRINK:
        case df::item_type::PLANT:
        case df::item_type::PLANT_GROWTH:
        case df::item_type::SEEDS:
        case df::item_type::POWDER_MISC:
        case df::item_type::LIQUID_MISC:
        case df::item_type::GLOB:
            return ITEMCLASS_FOOD;
        case df::item_type::DOOR:
        case df::item_type::FLOODGATE:
        case df::item_type::HATCH_COVER:
        case df::item_type::GRATE:
        case df::item_type::BED:
        case df::item_type::CHAIR:
        case df::item_type::TABLE:
        case df::item_type::COFFIN:
        case df::item_type::STATUE:
        case df::item_type::CABINET:
        case df::item_type::ARMORSTAND:
        case df::item_type::WEAPONRACK:
        case df::item_type::WINDOW:
        case df::item_type::CAGE:
        case df::item_type::ANIMALTRAP:
        case df::item_type::CHAIN:
        case df::item_type::QUERN:
        case df::item_type::MILLSTONE:
        case df::item_type::SLAB:
        case df::item_type::TRACTION_BENCH:
        case df::item_type::ANVIL:
        case df::item_type::PIPE_SECTION:
            return ITEMCLASS_FURNITURE;
        case df::item_type::BAR:
        case df::item_type::BLOCKS:
            return ITEMCLASS_BARS_BLOCKS;
        case df::item_type::CLOTH:
        case df::item_type::THREAD:
        case df::item_type::SKIN_TANNED:
            return ITEMCLASS_CLOTH_LEATHER;
        case df::item_type::WEAPON:
        case df::item_type::ARMOR:
        case df::item_type::SHOES:
        case df::item_type::SHIELD:
        case df::item_type::HELM:
        case df::item_type::GLOVES:
        case df::item_type::PANTS:
        case df::item_type::AMMO:
        case df::item_type::QUIVER:
        case df::item_type::TRAPCOMP:
        case df::item_type::TRAPPARTS:
        case df::item_type::SIEGEAMMO:
        case df::item_type::BALLISTAARROWHEAD:
        case df::item_type::CATAPULTPARTS:
        case df::item_type::BALLISTAPARTS:
        case df::item_type::BOLT_THROWER_PARTS:
            return ITEMCLASS_WEAPON_ARMOR;
        case df::item_type::FIGURINE:
        case df::item_type::AMULET:
        case df::item_type::SCEPTER:
        case df::item_type::CROWN:
        case df::item_type::RING:
        case df::item_type::EARRING:
        case df::item_type::BRACELET:
        case df::item_type::TOTEM:
        case df::item_type::TOY:
        case df::item_type::INSTRUMENT:
        case df::item_type::FLASK:
        case df::item_type::GOBLET:
        case df::item_type::BOOK:
        case df::item_type::SHEET:
        case df::item_type::COIN:
            return ITEMCLASS_FINISHED_GOODS;
        case df::item_type::CORPSE:
        case df::item_type::CORPSEPIECE:
        case df::item_type::REMAINS:
        case df::item_type::VERMIN:
            return ITEMCLASS_REFUSE;
        case df::item_type::ROUGH:
        case df::item_type::SMALLGEM:
        case df::item_type::GEM:
            return ITEMCLASS_GEMS;
        case df::item_type::BARREL:
        case df::item_type::BUCKET:
        case df::item_type::BIN:
        case df::item_type::BAG:
        case df::item_type::BOX:
        case df::item_type::BACKPACK:
            return ITEMCLASS_CONTAINERS;
        case df::item_type::TOOL:
        case df::item_type::SPLINT:
        case df::item_type::CRUTCH:
        case df::item_type::ORTHOPEDIC_CAST:
            return ITEMCLASS_TOOLS;
        default:
            return ITEMCLASS_OTHER;
    }
}

// FloorItemTileInfo is one map block's per-tile loose-item summary: the
// count (the always-on footer's raw material) and the DOMINANT class on
// that tile (the items lens's letter). One tile can hold several classes;
// the letter names the class with the most items there, ties breaking to
// the lower class id so repeated calls over unchanged terrain paint the
// same picture.
struct FloorItemTileInfo {
    std::array<int, 256> counts;
    std::array<int8_t, 256> topClass;  // -1 = no items on this tile
    FloorItemTileInfo() : counts{} { topClass.fill(-1); }
};

using FloorItemBlockCache = std::map<MapExtras::Block *, FloorItemTileInfo>;

static const FloorItemTileInfo *floorItemBlockInfo(MapExtras::MapCache &cache,
                                                   FloorItemBlockCache &blockCache,
                                                   df::coord pos) {
    MapExtras::Block *b = cache.BlockAtTile(pos);
    if (!b) return nullptr;
    auto found = blockCache.find(b);
    if (found == blockCache.end()) {
        FloorItemTileInfo info;
        // Per-tile per-class tallies, transient: only the winner is cached.
        std::vector<std::array<uint16_t, ITEMCLASS_COUNT>> classCounts(256);
        if (df::map_block *raw = b->getRaw()) {
            for (int32_t id : raw->items) {
                df::item *item = df::item::find(id);
                if (!item || !item->flags.bits.on_ground) continue;
                if (item->flags.bits.in_building || item->flags.bits.construction) continue;
                df::coord tidx = item->pos - raw->map_pos;
                if (!is_valid_tile_coord(tidx) || tidx.z != 0) continue;
                int idx = tidx.y * 16 + tidx.x;
                info.counts[idx]++;
                int cls = floorItemClassOf(item);
                if (cls >= 0 && cls < ITEMCLASS_COUNT) classCounts[idx][cls]++;
            }
        }
        for (int i = 0; i < 256; i++) {
            if (info.counts[i] == 0) continue;
            int best = -1;
            uint16_t bestN = 0;
            for (int c = 0; c < ITEMCLASS_COUNT; c++) {
                if (classCounts[i][c] > bestN) { bestN = classCounts[i][c]; best = c; }
            }
            info.topClass[i] = (int8_t)best;
        }
        // std::map never invalidates references on insert, so the pointer
        // handed back here stays valid for the rest of the call.
        found = blockCache.emplace(b, std::move(info)).first;
    }
    return &found->second;
}

static int floorItemCountAt(MapExtras::MapCache &cache, FloorItemBlockCache &blockCache, df::coord pos) {
    const FloorItemTileInfo *info = floorItemBlockInfo(cache, blockCache, pos);
    if (!info) return 0;
    return info->counts[(pos.y & 15) * 16 + (pos.x & 15)];
}

// floorItemClassAt: the dominant FloorItemClass on this tile, or -1 when
// the tile holds no loose items.
static int floorItemClassAt(MapExtras::MapCache &cache, FloorItemBlockCache &blockCache, df::coord pos) {
    const FloorItemTileInfo *info = floorItemBlockInfo(cache, blockCache, pos);
    if (!info) return -1;
    return info->topClass[(pos.y & 15) * 16 + (pos.x & 15)];
}

// collectStockpileCoverage: every tile inside the queried window that some
// stockpile's extents cover, at this z.
//
// Built from world->buildings.other.STOCKPILE (a typed vector, one entry
// per stockpile -- forts have single digits to dozens) plus
// Buildings::containsTile, which is DFHack's own membership test and is
// extent-bitmap aware. Buildings::findAtTile is deliberately NOT used: it
// is gated on tile occupancy + isSettingOccupancy(), a game-binary vmethod
// whose behavior for abstract buildings like stockpiles could not be
// verified from source -- and applyRemoveBuilding already had to work
// around findAtTile being blind to stockpiles for exactly that reason.
static void collectStockpileCoverage(int16_t x1, int16_t y1, int16_t x2, int16_t y2, int16_t z,
                                     std::unordered_set<df::coord> &out) {
    if (!df::global::world) return;
    // df::building's x1/y1/x2/y2/z are int32_t; the slice window is int16_t
    // tile coords -- clamp in int32_t, then narrow once for the coord keys.
    for (auto *sp : df::global::world->buildings.other.STOCKPILE) {
        if (!sp) continue;
        if (sp->z != (int32_t)z) continue;
        if (sp->x2 < (int32_t)x1 || sp->x1 > (int32_t)x2 ||
            sp->y2 < (int32_t)y1 || sp->y1 > (int32_t)y2) continue;
        int32_t ix1 = std::max(sp->x1, (int32_t)x1), ix2 = std::min(sp->x2, (int32_t)x2);
        int32_t iy1 = std::max(sp->y1, (int32_t)y1), iy2 = std::min(sp->y2, (int32_t)y2);
        for (int16_t yy = (int16_t)iy1; yy <= (int16_t)iy2; yy++) {
            for (int16_t xx = (int16_t)ix1; xx <= (int16_t)ix2; xx++) {
                if (Buildings::containsTile(sp, df::coord2d(xx, yy)))
                    out.insert(df::coord(xx, yy, z));
            }
        }
    }
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
    FloorItemBlockCache itemBlockCache;
    // Built once per call (matching tile_extractor.cpp/tile_updates.cpp's
    // own pattern) — see collect_dig_job_targets's forward-declaration
    // comment above for why the raw des.bits.dig bit alone isn't enough.
    std::unordered_set<df::coord> digJobTargets = collect_dig_job_targets();
    std::string rows = "[";
    std::string designated = "[";
    std::string water = "[";
    std::string aquifer = "[";
    std::string designationKinds = "[";
    std::string smoothed = "[";
    std::string floorItems = "[";
    std::string pendingBuilding = "[";
    int desCount = 0, waterCount = 0, aquiferCount = 0, kindCount = 0, smoothCount = 0, floorItemTiles = 0;
    int pendingCount = 0;
    // Minerals lens support: per-slice table of distinct vein materials seen
    // in THIS call, so per-tile entries can carry a small index instead of
    // repeating the material name on every tile (a vein-ringed industry
    // quarter can be dozens of tiles). veinMatIndex maps a DF inorganic
    // mat_index (the value MapCache::veinMaterialAt returns — see
    // plugins/prospector.cpp's identical veinMats[b->veinMaterialAt(coord)]
    // pattern in this checkout, the confirmed source for this lookup) to
    // its position in mineralNames. Capped at 200 tiles like the other
    // bounded arrays above (designated/designationKinds/smoothed/
    // pendingBuilding) — the index table itself stays tiny regardless
    // (bounded by distinct minerals, never by tile count).
    std::string minerals = "[";
    int mineralTileCount = 0;
    std::vector<std::string> mineralNames;
    std::map<int16_t, int> veinMatIndex;
    // Stockpile coverage over this window, built once per call (same
    // pattern as digJobTargets above). DF marks no item as "stockpiled",
    // so the homeless/stored split below is pure geometry -- see
    // collectStockpileCoverage.
    std::unordered_set<df::coord> stockpileTiles;
    collectStockpileCoverage((int16_t)x1, (int16_t)y1, (int16_t)x2, (int16_t)y2, (int16_t)z,
                             stockpileTiles);
    int stockpileTilesInView = 0, stockpileTilesOccupied = 0;
    // The homeless subset of floor_items. Capped at 200 (sibling
    // convention) with an explicit flag, unlike the uncapped floor_items
    // array it is drawn from: homeless clutter is normally small, and when
    // it is not, "200+, and here is where" is already a complete signal.
    std::string floorItemsNoStock = "[";
    int noStockCount = 0;
    bool noStockCapped = false;
    // Per-tile item class, keyed to a per-call name table -- the exact
    // minerals/mineral_names convention, including registering a class name
    // the first time it is SEEN (so the footnote stays complete) while
    // emitting per-tile entries only under the 200-tile cap.
    std::string floorItemClasses = "[";
    int classTileCount = 0;
    std::vector<std::string> floorItemClassNames;
    std::map<int, int> floorItemClassIndex;
    for (int16_t y = (int16_t)y1; y <= (int16_t)y2; y++) {
        std::string row;
        for (int16_t x = (int16_t)x1; x <= (int16_t)x2; x++) {
            df::coord pos(x, y, (int16_t)z);
            df::tile_designation des = cache.designationAt(pos);
            df::tiletype tt = cache.tiletypeAt(pos);
            row += classifyTile(tt, des);
            // Mineral identity for '=' vein/cluster tiles (tileMaterial ==
            // MINERAL). veinMaterialAt returns the inorganic mat_index
            // directly (mat_type is implicitly 0/INORGANIC for a vein — same
            // convention MaterialInfo's other call sites in this file use
            // for mat_type 0), or -1 when the tile isn't actually a vein
            // tile despite MINERAL material (defensive; shouldn't happen in
            // practice per prospector.cpp's identical check). The base
            // glyph classifier already renders this tile as '=' either way —
            // this block only adds the resolvable identity behind it.
            if (tileMaterial(tt) == df::tiletype_material::MINERAL) {
                int16_t veinMat = cache.veinMaterialAt(pos);
                if (veinMat >= 0) {
                    auto found = veinMatIndex.find(veinMat);
                    int idx;
                    if (found == veinMatIndex.end()) {
                        MaterialInfo mi((int16_t)0, (int32_t)veinMat);
                        idx = (int)mineralNames.size();
                        mineralNames.push_back(mi.isValid() ? mi.toString() : "unknown mineral");
                        veinMatIndex.emplace(veinMat, idx);
                    } else {
                        idx = found->second;
                    }
                    if (mineralTileCount < 200) {
                        if (mineralTileCount) minerals += ",";
                        minerals += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(idx) + "]";
                        mineralTileCount++;
                    }
                }
            }
            // Pending building: DF sets this occupancy value the instant ANY
            // building (including a Construction wall/floor/ramp) is placed —
            // markBuildingTiles() in DFHack's Buildings.cpp sets it uniformly
            // for every type whenever buildStage < maxBuildStage, clears it on
            // removal, and promotes it to a real occupancy value on
            // completion. This is the always-on counterpart to des.bits.dig
            // ('d') for the "queued, not yet real" case buildings never got —
            // plain look never painted buildings before this, so a queued
            // wall/seal job was completely invisible without lens=buildings.
            df::tile_occupancy occ = cache.occupancyAt(pos);
            if (occ.bits.building == df::tile_building_occ::Planned && pendingCount < 200) {
                if (pendingCount) pendingBuilding += ",";
                pendingBuilding += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                pendingCount++;
            }
            // A tile is "designated for digging" (and gets the always-on
            // 'd' glyph) if EITHER the raw designation bit is set OR a
            // dig job has already claimed it. DF clears/stops-reflecting
            // des.bits.dig the moment a unit claims the job — a tile a
            // miner is actively walking to or working would otherwise
            // report as plain undesignated rock for the whole job
            // duration, indistinguishable from a tile nobody has ever
            // touched. Live-observed 2026-07-17: a wall tile mid-dig
            // rendered with no 'd' at all in a bare `look` call. Mirrors
            // the identical fix already applied to the entity/topology
            // path (see compute_tile_flags in tile_extractor.cpp).
            bool digClaimedByJob = digJobTargets.count(pos) > 0;
            if ((des.bits.dig != df::tile_dig_designation::No || digClaimedByJob) && desCount < 200) {
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
                // not load-bearing, given that constraint). If the raw
                // bit has already gone cold because a job claimed it
                // (digClaimedByJob with des.bits.dig == No), the switch
                // below falls through to its default case (kind=0,
                // generic dig glyph) since the exact dig subtype isn't
                // recoverable from the tile alone once claimed —
                // collect_dig_job_targets only tracks WHICH tiles have a
                // claimed job, not which job, and that's an acceptable,
                // separate limitation from the always-on 'd' fix itself.
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
            // Water INCLUDING hidden tiles — same fog-honesty exception as
            // aquifer below. A near-flood incident traced to this exact
            // gate: a hidden under-brook channel tile carried real 7/7
            // water that this filter silently dropped from the water[]
            // array while the grid glyph stayed '?' (diggable-looking) and
            // the aquifer bit for neighboring tiles reported fine, so
            // nothing in the response hinted at the hazard. DF's own
            // damp-dig cancellations already make standing water
            // player-knowable the moment a dig touches it; hiding it here
            // only manufactures a surprise instead of preventing one.
            if (des.bits.flow_size > 0 &&
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
            // Smoothed state — the only DF-side signal that a completed
            // "smooth mode=wall/floor" job took effect; shape/material don't
            // change, so the classifier glyphs above render identically
            // before and after. See isSmoothedAt.
            if (isSmoothedAt(tt)) {
                if (smoothCount) smoothed += ",";
                smoothed += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                smoothCount++;
            }
            // Loose items at rest on open floor -- invisible to the glyph
            // classifier above (a tile with a stray item still renders as
            // plain floor) but can silently block a new building placement.
            // See floorItemCountAt.
            //
            // The stockpile split matters as much as the count: a bare
            // tile tally cannot tell a well-organized full stockpile from
            // a floor buried in junk, and that ambiguity is what left a
            // live session blind to a stockpile shortage.
            bool inStockpile = stockpileTiles.count(pos) > 0;
            if (inStockpile) stockpileTilesInView++;
            int itemCount = floorItemCountAt(cache, itemBlockCache, pos);
            if (itemCount > 0) {
                if (floorItemTiles) floorItems += ",";
                floorItems += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(itemCount) + "]";
                floorItemTiles++;
                if (inStockpile) {
                    stockpileTilesOccupied++;
                } else if (noStockCount < 200) {
                    // CAP INVARIANT: this 200 must stay >= the
                    // floor_item_classes cap below. Both counters advance in
                    // the same row-major scan, but this one counts ONLY
                    // homeless tiles while the class cap counts EVERY item
                    // tile, so the k-th homeless tile is at most the k-th
                    // item tile -- the class cap is always reached first, and
                    // every tile the items lens paints is therefore present
                    // in this array. Lower this cap (or raise the class cap)
                    // and lenses.go's buildItemsOverlay starts painting
                    // genuinely homeless tiles LOWERCASE, i.e. asserting
                    // "inside a stockpile" -- the exact lie this whole split
                    // exists to prevent. The Go render discloses
                    // floor_items_nostock_capped either way, so breaking this
                    // costs precision rather than truthfulness; do not break
                    // it regardless.
                    if (noStockCount) floorItemsNoStock += ",";
                    floorItemsNoStock += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(itemCount) + "]";
                    noStockCount++;
                } else {
                    noStockCapped = true;
                }
                int cls = floorItemClassAt(cache, itemBlockCache, pos);
                if (cls >= 0 && cls < ITEMCLASS_COUNT) {
                    auto foundClass = floorItemClassIndex.find(cls);
                    int idx;
                    if (foundClass == floorItemClassIndex.end()) {
                        idx = (int)floorItemClassNames.size();
                        floorItemClassNames.push_back(kFloorItemClassNames[cls]);
                        floorItemClassIndex.emplace(cls, idx);
                    } else {
                        idx = foundClass->second;
                    }
                    if (classTileCount < 200) {
                        // CAP INVARIANT (mirror of floor_items_nostock
                        // above): this cap must stay <= the homeless-list
                        // cap. It counts EVERY item tile while that one
                        // counts only homeless tiles, so it is always
                        // reached first and every lettered tile's homeless
                        // status is known to the renderer. Raise it past the
                        // other and the items lens would paint some homeless
                        // tiles lowercase -- reading as "inside a stockpile".
                        if (classTileCount) floorItemClasses += ",";
                        floorItemClasses += "[" + jsonInt(x) + "," + jsonInt(y) + "," + jsonInt(idx) + "]";
                        classTileCount++;
                    }
                }
            }
        }
        if (y != (int16_t)y1) rows += ",";
        rows += jsonStr(row);
    }
    rows += "]"; designated += "]"; water += "]"; aquifer += "]"; designationKinds += "]"; smoothed += "]";
    floorItems += "]"; pendingBuilding += "]"; minerals += "]";
    floorItemsNoStock += "]"; floorItemClasses += "]";
    // mineralNames table: one entry per distinct vein material seen in this
    // call, indexed by the third element of each minerals[] triple.
    std::string mineralNamesJSON = "[";
    for (size_t i = 0; i < mineralNames.size(); i++) {
        if (i) mineralNamesJSON += ",";
        mineralNamesJSON += jsonStr(mineralNames[i]);
    }
    mineralNamesJSON += "]";
    // floor_item_class_names: same shape as mineral_names -- one entry per
    // distinct item class seen in THIS call, indexed by the third element
    // of each floor_item_classes triple.
    std::string floorItemClassNamesJSON = "[";
    for (size_t i = 0; i < floorItemClassNames.size(); i++) {
        if (i) floorItemClassNamesJSON += ",";
        floorItemClassNamesJSON += jsonStr(floorItemClassNames[i]);
    }
    floorItemClassNamesJSON += "]";
    status = QUERY_STATUS_SUCCESS;
    // stockpile_tiles_in_view is emitted UNCONDITIONALLY (even at zero):
    // it is the new-plugin sentinel the Go renderer keys on, and a plain
    // int that is sometimes absent would let "never measured" decode as
    // "measured zero" -- the stockItem Units lesson, on the wire.
    return "{\"z\":" + jsonInt(z) + ",\"x1\":" + jsonInt(x1) + ",\"y1\":" + jsonInt(y1) +
           ",\"rows\":" + rows + ",\"designated\":" + designated +
           ",\"water\":" + water + ",\"aquifer\":" + aquifer +
           ",\"designation_kinds\":" + designationKinds +
           ",\"smoothed\":" + smoothed +
           ",\"floor_items\":" + floorItems +
           ",\"stockpile_tiles_in_view\":" + jsonInt(stockpileTilesInView) +
           ",\"stockpile_tiles_occupied\":" + jsonInt(stockpileTilesOccupied) +
           ",\"floor_items_nostock\":" + floorItemsNoStock +
           (noStockCapped ? std::string(",\"floor_items_nostock_capped\":true") : std::string()) +
           ",\"floor_item_classes\":" + floorItemClasses +
           ",\"floor_item_class_names\":" + floorItemClassNamesJSON +
           ",\"pending_building\":" + pendingBuilding +
           ",\"minerals\":" + minerals +
           ",\"mineral_names\":" + mineralNamesJSON + "}";
}

static std::string queryColumnProfile(const std::string &args, uint8_t &status) {
    int64_t x = jsonGetInt(args, "x", -1), y = jsonGetInt(args, "y", -1);
    int64_t zt = jsonGetInt(args, "z_top", -1), zb = jsonGetInt(args, "z_bottom", -1);
    status = QUERY_STATUS_ERROR;
    if (x < 0 || y < 0 || zt < zb || zt < 0)
        return jsonError("column_profile needs x,y,z_top>=z_bottom");
    if ((zt - zb + 1) > 60) return jsonError("column_profile too tall (max 60)");
    MapExtras::MapCache cache;
    FloorItemBlockCache itemBlockCache;
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
        // Water/aquifer/damp/smooth report the truth under fog, same as
        // shape/material above — this is the survey path, and dampness is
        // exactly what a cautious digger must see before breaching a wet
        // layer. All four fields are omitted when falsy (additive JSON).
        std::string wetness;
        if (des.bits.flow_size > 0 && des.bits.liquid_type == df::tile_liquid::Water)
            wetness += ",\"water\":" + jsonInt(des.bits.flow_size);
        if (des.bits.water_table)
            wetness += ",\"aquifer\":true";
        if (dampAt((int32_t)x, (int32_t)y, z))
            wetness += ",\"damp\":true";
        if (isSmoothedAt(tt))
            wetness += ",\"smooth\":true";
        int itemCount = floorItemCountAt(cache, itemBlockCache, pos);
        if (itemCount > 0)
            wetness += ",\"floor_items\":" + jsonInt(itemCount);
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

// isUnambiguousModificationMarker: true when tt carries one of the tiletype-
// level signals that can ONLY come from player action, never natural map
// generation (source-audited against this checkout, see docs/decisions.md
// look-scope-fort entry): a built Construction tile (tileMaterial==
// CONSTRUCTION), a smoothed/engraved tile (isSmoothedAt above, which
// already excludes Construction's own SMOOTH stamp), or a carved shape DF's
// own map generator never produces -- STAIR_UP/DOWN/UPDOWN or
// FORTIFICATION. RAMP is deliberately EXCLUDED here despite being a carved
// shape elsewhere in this file (see designations.cpp's isCarvedFeatureShape,
// a different use case): natural cavern/chasm terrain generates ramps too,
// so a bare RAMP is not proof of player action. Carved minecart tracks
// (tile_occupancy's carve_track_* bits) are a separate, always-player-made
// signal checked by the caller via hasCarveTrackAt below, since they live
// on a different DFHack struct (tile_occupancy, not tiletype).
static bool isUnambiguousModificationMarker(df::tiletype tt) {
    if (tileMaterial(tt) == df::tiletype_material::CONSTRUCTION) return true;
    if (isSmoothedAt(tt)) return true;
    df::tiletype_shape shape = tileShape(tt);
    return shape == df::tiletype_shape::STAIR_UP ||
           shape == df::tiletype_shape::STAIR_DOWN ||
           shape == df::tiletype_shape::STAIR_UPDOWN ||
           shape == df::tiletype_shape::FORTIFICATION;
}

static bool hasCarveTrackAt(df::tile_occupancy occ) {
    return occ.bits.carve_track_north || occ.bits.carve_track_south ||
           occ.bits.carve_track_east || occ.bits.carve_track_west;
}

// handleFortFootprint answers look scope=fort's data-source question the
// honest way the backing research confirmed is possible: DFHack keeps no
// persistent per-tile "a dwarf touched this" marker, so a scan can only
// ever bound the tiletype-level signals above (plus carved minecart
// tracks) -- an ordinary dug-out floor tile is byte-for-byte identical to
// natural cave floor of the same material once the transient dig
// designation bit clears (mirrors des.bits.smooth's own documented
// lifecycle: a transient queue marker, not a permanent record). This query
// therefore returns a BOUNDING BOX of the unambiguous markers on ONE
// z-level, scanned across the WHOLE map, not scoped to any session --
// this is what lets look scope=fort see renovations/smoothing/stairs from
// BEFORE the current connection, which the old Go-side session-delta
// journal (only ever populated from tile updates received this
// connection) could never see.
//
// may_include_natural_cave is the honest disclosure for the box's real
// blind spot: a bounding RECTANGLE can enclose untouched natural terrain
// between two separate marked areas (e.g. a carved stairwell and a
// smoothed room far apart on the same z), and plain dug-out floor inside
// the box is invisible to this scan by construction. Set true when any
// tile INSIDE the resulting box lacks an unambiguous marker while sitting
// on bare, never-worked natural terrain (STONE/SOIL/MINERAL vein material)
// -- the model should treat the box as an upper-bound approximation on
// flagged levels, never an exact footprint.
static std::string handleFortFootprint(const std::string &args, uint8_t &status) {
    int64_t z = jsonGetInt(args, "z", -1);
    if (!df::global::world || !df::global::world->map.x_count) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }
    int32_t mapW = df::global::world->map.x_count;
    int32_t mapH = df::global::world->map.y_count;
    int32_t mapD = df::global::world->map.z_count;
    if (z < 0 || z >= mapD) {
        status = QUERY_STATUS_ERROR;
        return jsonError("z out of range");
    }

    MapExtras::MapCache cache;
    int16_t x1 = 32767, y1 = 32767, x2 = -32768, y2 = -32768;
    bool found = false;
    for (int16_t ty = 0; ty < (int16_t)mapH; ty++) {
        for (int16_t tx = 0; tx < (int16_t)mapW; tx++) {
            df::coord pos(tx, ty, (int16_t)z);
            df::tiletype tt = cache.tiletypeAt(pos);
            if (!isUnambiguousModificationMarker(tt) && !hasCarveTrackAt(cache.occupancyAt(pos)))
                continue;
            found = true;
            if (tx < x1) x1 = tx;
            if (tx > x2) x2 = tx;
            if (ty < y1) y1 = ty;
            if (ty > y2) y2 = ty;
        }
    }

    if (!found) {
        status = QUERY_STATUS_SUCCESS;
        return "{\"found\":false,\"z\":" + jsonInt(z) + "}";
    }

    // Second pass, bounded to the box itself (cheap -- a fort's footprint is
    // a small fraction of the whole map): does any tile INSIDE the box lack
    // an unambiguous marker while sitting on bare natural terrain? See the
    // doc comment above for what this disclosure is (and is not) claiming.
    bool mayIncludeNaturalCave = false;
    for (int16_t ty = y1; ty <= y2 && !mayIncludeNaturalCave; ty++) {
        for (int16_t tx = x1; tx <= x2; tx++) {
            df::coord pos(tx, ty, (int16_t)z);
            df::tiletype tt = cache.tiletypeAt(pos);
            if (isUnambiguousModificationMarker(tt) || hasCarveTrackAt(cache.occupancyAt(pos)))
                continue;
            df::tiletype_material mat = tileMaterial(tt);
            if (mat == df::tiletype_material::STONE ||
                mat == df::tiletype_material::SOIL ||
                mat == df::tiletype_material::MINERAL) {
                mayIncludeNaturalCave = true;
                break;
            }
        }
    }

    status = QUERY_STATUS_SUCCESS;
    return "{\"found\":true,\"z\":" + jsonInt(z) +
           ",\"x1\":" + jsonInt(x1) + ",\"y1\":" + jsonInt(y1) +
           ",\"x2\":" + jsonInt(x2) + ",\"y2\":" + jsonInt(y2) +
           ",\"may_include_natural_cave\":" + (mayIncludeNaturalCave ? "true" : "false") + "}";
}

// regionScanKind classifies one tile for region_scan's live per-tile
// answer to "what's actually carved/modified in THIS caller-chosen box,
// right now". Unlike handleFortFootprint's isUnambiguousModificationMarker
// (a whole-map, no-bounds scan that must stay conservative about shapes
// natural map generation also produces), region_scan is always given an
// explicit box by the caller — the caller already told us "this is the
// area I care about", so a bare carved-open-floor or ramp tile inside it
// is legitimately reportable without needing to prove player intent on
// its own. Priority mirrors isUnambiguousModificationMarker where the two
// overlap (construction > smooth > carved stair/fortification shape) plus
// two kinds that function deliberately excludes for its own whole-map use
// case: RAMP ("ramp") and plain open floor ("floor" — shape FLOOR/BOULDER/
// PEBBLES, the same group classifyTileRevealed treats as walkable ground).
// Within that group, GRASS_LIGHT/DARK/DRY/DEAD material is excluded from
// "floor" the same way classifyTileRevealed splits it into ',' rather than
// '.' — untouched surface grass is never a "dug" tile, and reporting it as
// "floor" would let a caller-chosen box that merely spans two separate dig
// sites count the natural grass between them as modified. Carved minecart
// tracks (hasCarveTrackAt) are checked first within the group since the
// occupancy bit alone proves player action regardless of what's under it
// (grass included). Returns nullptr for tiles worth skipping entirely (bare
// wall, unrevealed empty space, untouched grass, etc) — this project's "is
// this open/modified" question, not a full tile dump.
static const char *regionScanKind(df::tiletype tt, df::tile_occupancy occ) {
    if (tileMaterial(tt) == df::tiletype_material::CONSTRUCTION) return "construction";
    if (isSmoothedAt(tt)) return "smooth";
    using S = df::tiletype_shape;
    switch (tileShape(tt)) {
        case S::STAIR_UP: return "stair_up";
        case S::STAIR_DOWN: return "stair_down";
        case S::STAIR_UPDOWN: return "stair_updown";
        case S::FORTIFICATION: return "fortification";
        case S::RAMP: return "ramp";
        case S::FLOOR: case S::BOULDER: case S::PEBBLES: {
            if (hasCarveTrackAt(occ)) return "track";
            using M = df::tiletype_material;
            M mat = tileMaterial(tt);
            if (mat == M::GRASS_LIGHT || mat == M::GRASS_DARK ||
                mat == M::GRASS_DRY || mat == M::GRASS_DEAD)
                return nullptr;
            return "floor";
        }
        default: break;
    }
    if (hasCarveTrackAt(occ)) return "track";
    return nullptr;
}

// kMaxRegionScanTiles caps the scanned volume (x-span * y-span * z-span),
// not the count of tiles actually reported (a fully-carved box reports
// close to the whole volume). Unlike handleFortFootprint (one fixed
// whole-map pass per call, size never caller-controlled), region_scan's
// box is caller-chosen and could otherwise cost an unbounded per-tile
// MapCache walk — refuse outright rather than silently truncating, so a
// caller that mis-sized a request gets a truthful error instead of a
// partial answer it might mistake for complete.
static const int64_t kMaxRegionScanTiles = 200000;

// handleRegionScan answers "what's actually carved/modified in this box,
// right now" via a live MapExtras::MapCache scan — the counterpart to
// handleFortFootprint (Q4, this project's shipped precedent for a live
// map-state query) for callers that already know which box they care
// about instead of needing the plugin to find one. Ships as the
// replacement data source for two things that used to read the Go-side
// session-delta Modifications overlay (wiped+rebaselined on every
// reconnect, and fed by a TILE_UPDATE stream that empirically delivers
// nothing — see docs/decisions.md): save_blueprint's dig-entry capture,
// and check_goals' HasModifiedAnything/HasShelter predicates.
static std::string handleRegionScan(const std::string &args, uint8_t &status) {
    int64_t x1 = jsonGetInt(args, "x1", -1), y1 = jsonGetInt(args, "y1", -1);
    int64_t z1 = jsonGetInt(args, "z1", -1);
    int64_t x2 = jsonGetInt(args, "x2", -1), y2 = jsonGetInt(args, "y2", -1);
    int64_t z2 = jsonGetInt(args, "z2", -1);
    status = QUERY_STATUS_ERROR;
    if (x1 < 0 || y1 < 0 || z1 < 0 || x2 < x1 || y2 < y1 || z2 < z1)
        return jsonError("region_scan needs x1,y1,z1,x2,y2,z2 with x2>=x1, y2>=y1, z2>=z1");
    int64_t volume = (x2 - x1 + 1) * (y2 - y1 + 1) * (z2 - z1 + 1);
    if (volume > kMaxRegionScanTiles)
        return jsonError("region_scan region too large (" + jsonInt(volume) +
                          " tiles, max " + jsonInt(kMaxRegionScanTiles) + ")");
    if (!Maps::isValidTilePos((int16_t)x1, (int16_t)y1, (int16_t)z1) ||
        !Maps::isValidTilePos((int16_t)x2, (int16_t)y2, (int16_t)z2))
        return jsonError("region_scan out of bounds");

    MapExtras::MapCache cache;
    std::string tiles = "[";
    int count = 0;
    for (int16_t z = (int16_t)z1; z <= (int16_t)z2; z++) {
        for (int16_t y = (int16_t)y1; y <= (int16_t)y2; y++) {
            for (int16_t x = (int16_t)x1; x <= (int16_t)x2; x++) {
                df::coord pos(x, y, z);
                df::tiletype tt = cache.tiletypeAt(pos);
                df::tile_occupancy occ = cache.occupancyAt(pos);
                const char *kind = regionScanKind(tt, occ);
                if (!kind) continue;
                if (count) tiles += ",";
                tiles += "{\"x\":" + jsonInt(x) + ",\"y\":" + jsonInt(y) + ",\"z\":" + jsonInt(z) +
                         ",\"kind\":" + jsonStr(kind) + "}";
                count++;
            }
        }
    }
    tiles += "]";
    status = QUERY_STATUS_SUCCESS;
    return "{\"x1\":" + jsonInt(x1) + ",\"y1\":" + jsonInt(y1) + ",\"z1\":" + jsonInt(z1) +
           ",\"x2\":" + jsonInt(x2) + ",\"y2\":" + jsonInt(y2) + ",\"z2\":" + jsonInt(z2) +
           ",\"count\":" + jsonInt(count) + ",\"tiles\":" + tiles + "}";
}

// handleZoneValue estimates a civzone's furniture-derived value plus a raw
// component breakdown, for comparison against noble_demands' required_office/
// required_bedroom/required_dining/required_tomb room-VALUE minimums
// (handleNobleDemands above -- same units, both plain DF room-value ints).
//
// Confirmed source paths (DFHack 53.15-r2 checkout, C:\Users\zmanl\Projects\dfhack-build):
//   df.building.xml:1080  building_civzonest::contained_buildings (original
//       name 'subord') -- "includes eg workshops and beds; not sorted". This
//       is the furniture/workshops physically inside the zone's footprint --
//       Bed/Table/Chair/Cabinet/Coffin/Box/Weaponrack/Armorstand/Statue/
//       Bookcase/etc are all real df::building subtypes (building_type.h),
//       not bare items.
//   df.building.xml:1103  building_actual::contained_items (original 'inv'),
//       std::vector<buildingitemst*> -- each wraps the actual df::item this
//       building is built from/holds (buildingitemst.item, original 'it').
//       A Bed building's contained_items[0].item IS the bed item whose
//       quality/value this handler reads. contained_buildings' element type
//       is the base df::building*, so each entry needs strict_virtual_cast
//       to building_actual before contained_items is reachable -- civzones
//       and a few other non-building_actual types can never appear here in
//       practice, but the cast-and-skip-on-null is defensive regardless.
//   modules/Items.h        Items::getValue(df::item*, df::caravan_state* =
//       NULL) -- DFHack's own confirmed value formula (base value x quality
//       multiplier + improvements - wear); called with no caravan (no trade
//       agreement context applies to furniture sitting in a room).
//   item->getQuality()     same vmethod handleStockpileInventory already
//       reads (df/item.h, returns int16_t, cast to df::item_quality).
//   df.event.xml:15/221    df::engraving (original event_detailst), reached
//       via df::global::world->event.engravings -- REAL per-tile engraving
//       records (pos, quality, flags for which face is engraved). Used for
//       the engraving tally instead of the coarser isSmoothedAt tiletype
//       check, since this is DF's actual engraving data (position +
//       craftsmanship quality), not a tiletype inference.
//
// NOT relied upon: Buildings::getRoomDescription / the room-value vmethod
// chain it would call (building->getRoomValue(unit)) -- confirmed, by
// reading the live source rather than assuming, to be unusable in this
// checkout: df.building.xml declares no getRoomValue vmethod at all, and
// modules/Buildings.cpp's getRoomDescription body is entirely commented out
// with "TODO: understand how this changes for v50". estimated_value below
// is this handler's OWN sum of component item values -- a separate,
// additive computation, not a reimplementation of DF's internal
// room-quality-tier logic (which this checkout cannot run).
//
// experimental_personal_value calls building->getPersonalValue(owner) --
// original name basicvaluation, a REAL, present vmethod on the base
// df::building class (df.building.xml, confirmed in the vtable) -- but
// unlike the fields above, nothing in this checkout confirms what it
// actually returns for a v50 zone-type building, since the DFHack-side
// consumer of the equivalent getRoomValue path is the exact code that's
// disabled above. Reported as its own clearly-named field; never folded
// into estimated_value.
//
// civzone_type::Bedroom/Office/DiningHall/Tomb are the zone kinds
// noble_demands' required_* fields are meaningful against (roomValueField
// below names which one); other zone kinds still get a full component
// walk and value estimate here (contained_buildings/contained_items apply
// to any civzone), just with room_value_field left null -- no kind
// restriction is enforced, per this project's data-not-rules convention
// (see handleNobleDemands' own comment).
//
// assigned_items (civzone's own vector<int32_t> item ids -- e.g. a tomb's
// occupant-linked items) is deliberately NOT walked here; the task this
// shipped under scoped the value walk to contained_buildings specifically.
// Left as an open question for a future pass rather than guessed at.
static std::string handleZoneValue(const std::string &args, uint8_t &status) {
    int64_t x = jsonGetInt(args, "x", -1);
    int64_t y = jsonGetInt(args, "y", -1);
    int64_t z = jsonGetInt(args, "z", -1);
    if (x < 0 || y < 0 || z < 0) {
        status = QUERY_STATUS_ERROR;
        return jsonError("missing or invalid coordinates");
    }

    // Civzone is abstract (isSettingOccupancy()==false), architecturally
    // invisible to Buildings::findAtTile -- see applyRemoveBuilding's
    // comment in df_ai_protocol.cpp. Buildings::findCivzonesAt is the
    // canonical point-in-zone lookup (already proven by zones.cpp's
    // findZoneAt and locations.cpp), matching by extent containment
    // against the zone's whole footprint rather than a single occupancy
    // tile.
    std::vector<df::building_civzonest*> zones;
    Buildings::findCivzonesAt(&zones, df::coord((int16_t)x, (int16_t)y, (int16_t)z));
    if (zones.empty()) {
        status = QUERY_STATUS_ERROR;
        return jsonError("no zone at coordinates");
    }
    df::building_civzonest *cz = zones[0];
    df::building *b = cz;

    uint8_t wireKind = wireFromCivzoneType(cz->type);

    // Same required_* field name this zone type would be checked against in
    // noble_demands (handleNobleDemands above) -- null when this zone kind
    // has no room-value-minimum analog there (e.g. Pen, Dormitory).
    const char *roomValueField = nullptr;
    switch (cz->type) {
        case df::civzone_type::Bedroom:    roomValueField = "required_bedroom"; break;
        case df::civzone_type::Office:     roomValueField = "required_office"; break;
        case df::civzone_type::DiningHall: roomValueField = "required_dining"; break;
        case df::civzone_type::Tomb:       roomValueField = "required_tomb"; break;
        default: break;
    }

    // Component walk -- live position scan over df::global::world->
    // buildings.all, NOT cz->contained_buildings. Confirmed by reading
    // DFHack's own linkage code (modules/Buildings.cpp): contained_buildings
    // is populated by two one-shot hooks -- add_building_to_all_zones()
    // fires when a building is CONSTRUCTED (only finds a zone if one already
    // exists at that instant) and add_zone_to_all_buildings() fires once
    // when a zone is CONSTRUCTED (only finds buildings already in
    // world->buildings.other.IN_PLAY, i.e. already fully built, at that
    // instant) -- neither hook re-runs later. A zone drawn over furniture
    // that was still under construction at zone-creation time (or built via
    // any path that doesn't retrigger these hooks) never gets linked, and
    // nothing ever reconciles it afterward: a live fort tour found exactly
    // this -- a dining hall with 2 built tables + a chair reported 0
    // components. Rather than depend on that order-sensitive DF-internal
    // cache, scan world->buildings.all directly and test containment with
    // Buildings::containsTile (the same extents-aware point-in-building
    // test findCivzonesAt/findAtTile already use) -- this is a live query,
    // always correct regardless of construction order, matching this file's
    // existing list_buildings/list_zones precedent of scanning
    // buildings.all fresh on every call instead of trusting a cache.
    //
    // See the file comment above for why no kNeverFortStockFlags-style junk
    // filter is applied here: an item reachable only via a building's own
    // contained_items is definitionally installed furniture, not loose
    // stockpile-adjacent stock (and excluding mask_artifact, appropriate for
    // the stockpile tally, would be actively wrong here -- an artifact bed
    // is exactly the kind of item that should dominate a room's value).
    std::ostringstream comps;
    comps << "[";
    bool firstComp = true;
    int compCount = 0;
    int64_t totalValue = 0;
    for (auto *cb : df::global::world->buildings.all) {
        if (!cb || cb == b) continue;
        if (cb->z != b->z) continue;
        if (cb->getType() == df::building_type::Civzone) continue; // don't count a stacked zone as "furniture"
        if (!Buildings::containsTile(b, df::coord2d(cb->centerx, cb->centery))) continue;
        auto *ba = strict_virtual_cast<df::building_actual>(cb);
        if (!ba) continue;
        std::string buildingTypeName = ENUM_KEY_STR(building_type, cb->getType());
        for (auto *bi : ba->contained_items) {
            if (!bi || !bi->item) continue;
            df::item *item = bi->item;
            int value = Items::getValue(item);
            df::item_quality q = (df::item_quality)item->getQuality();
            MaterialInfo mi((int16_t)item->getActualMaterial(), (int32_t)item->getActualMaterialIndex());

            if (!firstComp) comps << ",";
            firstComp = false;
            comps << "{\"building_type\":" << jsonStr(buildingTypeName)
                  << ",\"item_type\":" << jsonStr(ENUM_KEY_STR(item_type, item->getType()))
                  << ",\"material\":" << jsonStr(mi.isValid() ? mi.toString() : "")
                  << ",\"quality\":" << jsonStr(ENUM_KEY_STR(item_quality, q))
                  << ",\"value\":" << jsonInt(value)
                  << "}";
            compCount++;
            totalValue += value;
        }
    }
    comps << "]";

    // Smoothed-tile count over the zone's footprint -- single z-level, same
    // as list_zones (civzones don't span multiple z in this DF version).
    MapExtras::MapCache cache;
    int tilesTotal = 0, tilesSmoothed = 0;
    for (int16_t ty = (int16_t)b->y1; ty <= (int16_t)b->y2; ty++) {
        for (int16_t tx = (int16_t)b->x1; tx <= (int16_t)b->x2; tx++) {
            tilesTotal++;
            if (isSmoothedAt(cache.tiletypeAt(df::coord(tx, ty, (int16_t)b->z))))
                tilesSmoothed++;
        }
    }

    // Real per-tile engraving records inside the same footprint, tallied by
    // quality tier (same style as handleStockpileInventory's qualityCounts
    // above). df::global::world->event.engravings is a whole-map vector, so
    // this is a linear scan filtered by position -- acceptable here for the
    // same reason handleStockpileInventory's world->items.all scan is: an
    // occasional, deliberate call, not a per-tick one.
    std::map<int, int> engravingsByQuality;
    int engravingsCount = 0;
    if (df::global::world) {
        for (auto *e : df::global::world->event.engravings) {
            if (!e) continue;
            if (e->pos.z != b->z) continue;
            if (e->pos.x < b->x1 || e->pos.x > b->x2) continue;
            if (e->pos.y < b->y1 || e->pos.y > b->y2) continue;
            engravingsCount++;
            engravingsByQuality[(int)e->quality]++;
        }
    }

    // Experimental bonus number -- see the file comment above for why this
    // is reported separately and not folded into estimated_value.
    df::unit *owner = (cz->assigned_unit_id >= 0) ? df::unit::find(cz->assigned_unit_id) : nullptr;
    int32_t personalValue = b->getPersonalValue(owner);

    std::ostringstream os;
    os << "{\"kind\":" << jsonInt(wireKind)
       << ",\"type_name\":" << jsonStr(ENUM_KEY_STR(civzone_type, cz->type))
       << ",\"room_value_field\":" << (roomValueField ? jsonStr(roomValueField) : "null")
       << ",\"x1\":" << jsonInt(b->x1) << ",\"y1\":" << jsonInt(b->y1)
       << ",\"x2\":" << jsonInt(b->x2) << ",\"y2\":" << jsonInt(b->y2)
       << ",\"z\":" << jsonInt(b->z)
       << ",\"components\":" << comps.str()
       << ",\"component_count\":" << jsonInt(compCount)
       << ",\"estimated_value\":" << jsonInt(totalValue)
       << ",\"tiles_total\":" << jsonInt(tilesTotal)
       << ",\"tiles_smoothed\":" << jsonInt(tilesSmoothed)
       << ",\"engravings_count\":" << jsonInt(engravingsCount)
       << ",\"engravings_by_quality\":[";
    bool firstEq = true;
    for (auto &kv : engravingsByQuality) {
        if (!firstEq) os << ",";
        firstEq = false;
        os << "{\"quality\":" << jsonStr(ENUM_KEY_STR(item_quality, (df::item_quality)kv.first))
           << ",\"count\":" << jsonInt(kv.second) << "}";
    }
    os << "]"
       << ",\"experimental_personal_value\":" << jsonInt(personalValue)
       << "}";

    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// walkableFromMapEdge cheaply checks whether depotPos shares a walkability
// group (Maps::getWalkableGroup, Maps.h:384) with ANY true map-boundary
// entry tile -- a direct C++ port of DFHack's own plugins/pathable.cpp
// (get_pathability_groups + get_entry_tiles, lines 146-190), which computes
// exactly this same-group membership test against
// df::global::plotinfo->map_edge.surface_x/y/z (original-name
// connected_enter_*_5, df.plotinfo.xml:989-1006), restricted to tiles that
// are genuinely on the map boundary (x==0/y==0/x==max/y==max, matching
// pathable.cpp:189-190 exactly).
//
// This is a per-unit walkability-group test, NOT a wagon-width check --
// reported as a SEPARATE, honestly-labeled field alongside the native
// accessible/have_access flag (which DF computes with the full wagon-width
// pathing DFHack's own pathable.cpp wagon_flood/is_wagon_traversible tier,
// lines 210-390, reproduces at real cost and even DFHack's own comment
// there admits is imperfect -- "TODO: cannot traverse doors, up stairs, or
// up/down stairs"). Reading BOTH together is the diagnostic: if this cheap
// check ALSO reports false, the depot is topologically cut off entirely
// (a real wall/dig problem); if this passes while accessible is still
// false, the obstruction is specifically wagon-corridor WIDTH (a narrow
// 1-2 tile chokepoint) that this cheap per-unit check cannot see -- at
// which point a model should inspect the known wagon corridor with
// look/cross_section rather than expect a second bespoke pathing engine
// here.
static bool walkableFromMapEdge(df::coord depotPos) {
    if (!df::global::plotinfo) return false;
    uint16_t depotGroup = Maps::getWalkableGroup(depotPos);
    if (!depotGroup) return false;

    uint32_t countX = 0, countY = 0, countZ = 0;
    Maps::getTileSize(countX, countY, countZ);
    if (!countX || !countY) return false;
    int16_t maxX = (int16_t)(countX - 1);
    int16_t maxY = (int16_t)(countY - 1);

    auto &edge = df::global::plotinfo->map_edge;
    size_t n = edge.surface_x.size();
    for (size_t i = 0; i < n; i++) {
        df::coord pos(edge.surface_x[i], edge.surface_y[i], edge.surface_z[i]);
        if (pos.x != 0 && pos.y != 0 && pos.x != maxX && pos.y != maxY) continue;
        if (Maps::getWalkableGroup(pos) == depotGroup) return true;
    }
    return false;
}

// handleCaravanStatus enumerates active caravans (df::global::plotinfo->
// caravans -- confirmed via df.plotinfo.xml:821, struct-type 'caravan_state'
// original-name 'merchant', a std::vector<caravan_state*> field of
// plotinfost reached through plotinfo's own null-checked global), plus
// scheduled-but-not-yet-arrived caravan/diplomat events from
// df::global::timed_events (df.game_v.xml:24, global-object 'timed_events'
// original-name 'plot_event' -- the ONLY place a countdown exists before a
// caravan/liaison shows up in plotinfo->caravans at all), whether a liaison
// meeting is currently active (plotinfo->dip_meeting_info, df.plotinfo.xml:
// 858), and depot readiness (first TRADE_DEPOT building's build stage,
// trade_flags, native accessible/have_access, and the cheap
// walkable_from_edge diagnostic -- see walkableFromMapEdge's doc comment
// above for how the two access fields together localize an access
// problem). No args. 2026-07-19 trade/caravan research pass
// (docs/decisions.md); see also depot_goods (staged/pending goods at the
// depot) and bring_goods_to_depot/set_depot_trade_flags (the write sides).
//
// time_remaining/ticks_remaining conversion mirrors handleListMandates'
// timeout_counter/timeout_limit x10 factor EXACTLY: DFHack's own
// scripts/caravan.lua:68 divides time_remaining by 120 to get days, and
// 120 x 10 = 1200 -- this project's own established "1200 ticks = 1 game
// day" tick vocabulary (CLAUDE.md). So time_remaining is counted in the
// same coarser once-per-10-frames unit mandates' timeout fields use, and
// the same x10 factor recovers a frame_counter-comparable tick count for
// the step tool's own N-tick vocabulary.
static std::string handleCaravanStatus(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world/plotinfo is null");
    }

    std::ostringstream os;
    os << "{\"caravans\":[";
    bool first = true;
    for (auto *car : df::global::plotinfo->caravans) {
        if (!car) continue;
        if (!first) os << ",";
        first = false;

        std::string civName;
        if (df::historical_entity *civ = df::historical_entity::find(car->entity)) {
            civName = Translation::translateName(&civ->name, true);
        }

        os << "{\"civ\":" << jsonStr(civName)
           << ",\"entity_id\":" << jsonInt(car->entity)
           << ",\"trade_state\":" << jsonStr(ENUM_KEY_STR(caravan_state::T_trade_state, car->trade_state))
           << ",\"time_remaining_ticks\":" << jsonInt(10LL * (int64_t)car->time_remaining)
           << ",\"days_remaining\":" << jsonNum((double)car->time_remaining / 120.0)
           << ",\"tribute\":" << (car->flags.bits.tribute ? "true" : "false")
           << ",\"casualty\":" << (car->flags.bits.casualty ? "true" : "false")
           << ",\"hardship\":" << (car->flags.bits.hardship ? "true" : "false")
           << ",\"seized\":" << (car->flags.bits.seized ? "true" : "false")
           << ",\"offended\":" << (car->flags.bits.offended ? "true" : "false")
           << ",\"greatly_offended\":" << (car->flags.bits.greatly_offended ? "true" : "false")
           << ",\"import_value\":" << jsonInt(car->import_value)
           << ",\"export_value_total\":" << jsonInt(car->export_value_total)
           << ",\"export_value_personal\":" << jsonInt(car->export_value_personal)
           << ",\"offer_value\":" << jsonInt(car->offer_value)
           << ",\"mood\":" << jsonInt(car->mood)
           << ",\"haggle_fail_count\":" << jsonInt(car->haggle_fail_count);

        bool liaisonMeeting = false;
        for (auto *dip : df::global::plotinfo->dip_meeting_info) {
            if (dip && dip->civ_id == car->entity) { liaisonMeeting = true; break; }
        }
        os << ",\"liaison_meeting_active\":" << (liaisonMeeting ? "true" : "false");

        os << "}";
    }
    os << "]";

    os << ",\"pending_events\":[";
    bool firstEv = true;
    if (df::global::timed_events) {
        for (auto *ev : *df::global::timed_events) {
            if (!ev) continue;
            if (ev->type != df::timed_event_type::Caravan &&
                ev->type != df::timed_event_type::TributeCaravan &&
                ev->type != df::timed_event_type::Diplomat) continue;
            if (!firstEv) os << ",";
            firstEv = false;

            std::string civName;
            if (ev->entity) civName = Translation::translateName(&ev->entity->name, true);

            os << "{\"type\":" << jsonStr(ENUM_KEY_STR(timed_event_type, ev->type))
               << ",\"civ\":" << jsonStr(civName)
               << ",\"season\":" << jsonStr(ENUM_KEY_STR(season, ev->season))
               << ",\"season_ticks_remaining\":" << jsonInt(ev->season_ticks)
               << "}";
        }
    }
    os << "]";

    // Depot readiness -- first TRADE_DEPOT building found, if any. Multiple
    // depots are legal DF state but unusual; mirrors DFHack's own
    // caravan.lua/movegoods.lua assumption of a single depot.
    df::building_tradedepotst *depot = nullptr;
    for (auto *d : df::global::world->buildings.other.TRADE_DEPOT) {
        if (d) { depot = d; break; }
    }
    if (depot) {
        os << ",\"depot\":{\"exists\":true"
           << ",\"x\":" << jsonInt(depot->centerx)
           << ",\"y\":" << jsonInt(depot->centery)
           << ",\"z\":" << jsonInt(depot->z)
           << ",\"built\":" << (depot->getBuildStage() >= depot->getMaxBuildStage() ? "true" : "false")
           << ",\"accessible\":" << (depot->accessible ? "true" : "false")
           << ",\"walkable_from_edge\":" << (walkableFromMapEdge(df::coord(depot->centerx, depot->centery, depot->z)) ? "true" : "false")
           << ",\"trader_requested\":" << (depot->trade_flags.bits.trader_requested ? "true" : "false")
           << ",\"anyone_can_trade\":" << (depot->trade_flags.bits.anyone_can_trade ? "true" : "false")
           << "}";
    } else {
        os << ",\"depot\":{\"exists\":false}";
    }

    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleDepotGoods reports what's staged at a trade depot and what's
// queued to be hauled there but hasn't arrived yet. Targets the depot at
// (x,y,z) if given, otherwise auto-targets the first TRADE_DEPOT building
// found (matches DFHack's own scripts/caravan.lua:107 single-depot
// assumption) -- no coordinates needed for the common one-depot case.
//
// "staged_ours" / "staged_theirs": depot->contained_items filtered to
// use_mode==TEMP (a staged trade good; PERM is the depot's own
// construction material, e.g. the blocks it was built from -- see
// df.building.xml building_item_role_type), split by
// item->flags.bits.trader (item_flags.h bit 15, "Item owned by trader") --
// trade-pack wave fix for the prior single "staged" list aggregating our
// own marked-for-trade goods together with the merchants' import
// inventory with no ownership discriminator at all. movegoods.lua's own
// is_tradeable_item treats flags.trader as a hard reject for anything the
// fort side could ever select, confirming trader==true is reserved for
// merchant stock and never set on fort goods before an actual trade
// commits.
// "pending": items attached to a queued BringItemToDepot job on this depot
// (depot->jobs) -- hauling in progress, not yet physically at the depot.
// Needs no ours/theirs split: a queued haul job can only ever be OUR
// outgoing goods (merchant stock never gets a BringItemToDepot job).
// All three aggregated by (item_type, material) like stockpile_inventory,
// not by individual item ID -- no tool today surfaces item identity, and
// this is a read, not an ID-based follow-up action.
static std::string handleDepotGoods(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world is null");
    }

    int64_t x = jsonGetInt(args, "x", -1);
    int64_t y = jsonGetInt(args, "y", -1);
    int64_t z = jsonGetInt(args, "z", -1);

    df::building_tradedepotst *depot = nullptr;
    if (x >= 0 && y >= 0) {
        df::building *b = Buildings::findAtTile(df::coord((int16_t)x, (int16_t)y, (int16_t)z));
        depot = b ? strict_virtual_cast<df::building_tradedepotst>(b) : nullptr;
        if (!depot) {
            status = QUERY_STATUS_ERROR;
            return jsonError("no trade depot at coordinates");
        }
    } else {
        for (auto *d : df::global::world->buildings.other.TRADE_DEPOT) {
            if (d) { depot = d; break; }
        }
        if (!depot) {
            status = QUERY_STATUS_ERROR;
            return jsonError("no trade depot exists");
        }
    }

    // (item_type, mat_type, mat_index) -> (count, total_value, any_requested)
    std::map<std::tuple<int, int, int>, std::tuple<int, int64_t, bool>> stagedOurs;
    std::map<std::tuple<int, int, int>, std::tuple<int, int64_t, bool>> stagedTheirs;
    for (auto *bi : depot->contained_items) {
        if (!bi || !bi->item) continue;
        if (bi->use_mode != df::building_item_role_type::TEMP) continue;
        // Trade-pack wave: unmark_trade_goods releases a staged item by
        // clearing flags.bits.in_building, but no DFHack API removes the
        // buildingitemst entry itself (Items.cpp's moveToBuilding push has
        // no documented inverse) -- without this gate, an unmarked item
        // would keep rendering here as still staged forever. See
        // trade.cpp's applyUnmarkTradeGoods for the write side.
        if (!bi->item->flags.bits.in_building) continue;
        df::item *item = bi->item;
        auto key = std::make_tuple((int)item->getType(), (int)item->getActualMaterial(),
                                   (int)item->getActualMaterialIndex());
        auto &agg = item->flags.bits.trader ? stagedTheirs[key] : stagedOurs[key];
        std::get<0>(agg)++;
        std::get<1>(agg) += Items::getValue(item);
        if (Items::isRequestedTradeGood(item)) std::get<2>(agg) = true;
    }

    std::map<std::tuple<int, int, int>, std::tuple<int, int64_t, bool>> pending;
    for (auto *job : depot->jobs) {
        if (!job || job->job_type != df::job_type::BringItemToDepot) continue;
        for (auto *ji : job->items) {
            if (!ji || !ji->item) continue;
            df::item *item = ji->item;
            auto key = std::make_tuple((int)item->getType(), (int)item->getActualMaterial(),
                                       (int)item->getActualMaterialIndex());
            auto &agg = pending[key];
            std::get<0>(agg)++;
            std::get<1>(agg) += Items::getValue(item);
            if (Items::isRequestedTradeGood(item)) std::get<2>(agg) = true;
        }
    }

    auto renderAgg = [](std::ostringstream &os,
                         const std::map<std::tuple<int, int, int>, std::tuple<int, int64_t, bool>> &agg) {
        os << "[";
        bool first = true;
        for (auto &kv : agg) {
            df::item_type itype = (df::item_type)std::get<0>(kv.first);
            MaterialInfo mi((int16_t)std::get<1>(kv.first), (int32_t)std::get<2>(kv.first));
            if (!first) os << ",";
            first = false;
            os << "{\"item_type\":" << jsonStr(ENUM_KEY_STR(item_type, itype))
               << ",\"material\":" << jsonStr(mi.isValid() ? mi.toString() : "")
               << ",\"count\":" << jsonInt(std::get<0>(kv.second))
               << ",\"value\":" << jsonInt(std::get<1>(kv.second))
               << ",\"requested\":" << (std::get<2>(kv.second) ? "true" : "false")
               << "}";
        }
        os << "]";
    };

    std::ostringstream os;
    os << "{\"depot\":{\"x\":" << jsonInt(depot->centerx)
       << ",\"y\":" << jsonInt(depot->centery)
       << ",\"z\":" << jsonInt(depot->z)
       << ",\"built\":" << (depot->getBuildStage() >= depot->getMaxBuildStage() ? "true" : "false")
       << "}"
       << ",\"staged_ours\":";
    renderAgg(os, stagedOurs);
    os << ",\"staged_theirs\":";
    renderAgg(os, stagedTheirs);
    os << ",\"pending\":";
    renderAgg(os, pending);
    os << "}";

    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// handleTradeAgreements reports each visiting civ's concluded trade
// agreement data (df::caravan_state::buy_prices — specific items THIS civ
// has requested the fort sell it, each with its own price bonus, original-
// name "requestagreement"; sell_prices — broad entity_sell_category
// buckets the fort gets a price break buying from this civ's own imports,
// original-name "tradeagreement", both fields on caravan_state per
// df.plotinfo.xml, dereferenced right alongside the same car->entity/
// dip_meeting_info lookups handleCaravanStatus above already performs),
// plus whatever the CURRENTLY OPEN liaison meeting (if any) has already
// resolved so far this visit (df::global::plotinfo->dip_meeting_info,
// meeting_diplomat_info's own ->events vector of meeting_event entries
// concluded WITHIN this same still-open meeting — distinct from and
// narrower than the standing agreement fields above, which persist across
// meetings once concluded). Read-only; wave-7 trade/diplomacy brief item 5.
// The liaison meeting itself (viewscreen_meetingst) happened invisibly to
// this project before this query existed (fortress/memory/goals.md); this
// only reads whatever DF's own dipscript has already written to these
// structures — it never drives or advances a meeting.
//
// No args. Iterates every civ with an active caravan_state exactly like
// caravan_status (df::global::plotinfo->caravans) — this answers "what's
// the current standing deal with each civ visiting right now", not a full
// permanent audit trail across all civs/all time (historical_entity::
// meeting_events would be that; deliberately out of scope this pass, see
// docs/decisions.md).
//
// price_percent mirrors DFHack's own scripts/internal/caravan/common.lua:420
// EXACTLY: (price * 100) / 128 — 128 is DF's own "no change" baseline (100%
// of normal value), NOT a "+0%" bonus-only percentage; 150 means "civ pays
// 150% of normal value for this", 64 means "civ pays half".
//
// Honest null-safety: sell_prices/buy_prices are BOTH independently
// nullable (a civ can have no agreement at all, or an agreement covering
// only one direction) — emit empty arrays, never guess or backfill.
// meeting_active is only true while a dip_meeting_info entry with matching
// civ_id genuinely exists right now (mirrors caravan_status's own
// liaison_meeting_active gate) — whether an UNATTENDED meeting reliably
// auto-concludes within a bounded time is unverified by this plugin (see
// the wave-7 investigation's open_questions); this handler only ever
// reports what the structures say at the moment it's called, never a
// prediction about when a meeting will resolve.
static std::string handleTradeAgreements(const std::string &args, uint8_t &status) {
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonError("world/plotinfo is null");
    }

    std::ostringstream os;
    os << "{\"civs\":[";
    bool first = true;
    for (auto *car : df::global::plotinfo->caravans) {
        if (!car) continue;
        if (!first) os << ",";
        first = false;

        std::string civName;
        if (df::historical_entity *civ = df::historical_entity::find(car->entity)) {
            civName = Translation::translateName(&civ->name, true);
        }

        os << "{\"civ\":" << jsonStr(civName)
           << ",\"entity_id\":" << jsonInt(car->entity);

        // Specific-item requests (buy_prices/"requestagreement") — items
        // this civ has asked the fort to sell it, each with an individual
        // price multiplier. items/price are parallel vectors on two
        // different structs (entity_buy_prices + its nested
        // entity_buy_requests); bound to the shortest of all of them
        // defensively rather than assume they're always kept in lockstep.
        os << ",\"specific_item_requests\":[";
        if (car->buy_prices && car->buy_prices->items) {
            auto *items = car->buy_prices->items;
            size_t n = car->buy_prices->price.size();
            n = std::min(n, items->item_type.size());
            n = std::min(n, items->item_subtype.size());
            n = std::min(n, items->mat_types.size());
            n = std::min(n, items->mat_indices.size());
            for (size_t i = 0; i < n; i++) {
                if (i > 0) os << ",";
                int16_t matType = items->mat_types[i];
                int32_t matIndex = items->mat_indices[i];
                std::string material = "any";
                if (matType >= 0) {
                    MaterialInfo mi(matType, matIndex);
                    if (mi.isValid()) material = mi.toString();
                }
                os << "{\"item_type\":" << jsonStr(ENUM_KEY_STR(item_type, items->item_type[i]))
                   << ",\"item_subtype\":" << jsonInt(items->item_subtype[i])
                   << ",\"material\":" << jsonStr(material)
                   << ",\"price_percent\":" << jsonInt(((int64_t)car->buy_prices->price[i] * 100) / 128)
                   << "}";
            }
        }
        os << "]";

        // Category-bucket agreement (sell_prices/"tradeagreement") — broad
        // classes of the civ's own import goods the fort gets a price
        // break buying, e.g. "Weapons" or "Cheese". entity_sell_category
        // has ~107 values (df/entity_sell_category.h); only non-empty
        // buckets are emitted, min/max in case a category holds more than
        // one differently-priced resource.
        os << ",\"category_agreement\":[";
        bool firstCat = true;
        if (car->sell_prices) {
            for (int cat = 0; cat <= df::enum_traits<df::entity_sell_category>::last_item_value; cat++) {
                auto &prices = car->sell_prices->price[cat];
                if (prices.empty()) continue;
                if (!firstCat) os << ",";
                firstCat = false;
                int minPct = 0, maxPct = 0;
                bool firstPrice = true;
                for (int32_t p : prices) {
                    int pct = (int)(((int64_t)p * 100) / 128);
                    if (firstPrice || pct < minPct) minPct = pct;
                    if (firstPrice || pct > maxPct) maxPct = pct;
                    firstPrice = false;
                }
                os << "{\"category\":" << jsonStr(ENUM_KEY_STR(entity_sell_category, (df::entity_sell_category)cat))
                   << ",\"count\":" << jsonInt((int64_t)prices.size())
                   << ",\"min_percent\":" << jsonInt(minPct)
                   << ",\"max_percent\":" << jsonInt(maxPct)
                   << "}";
            }
        }
        os << "]";

        // Live meeting draft — only while a liaison meeting for this civ is
        // actually open right now (mirrors caravan_status's own
        // liaison_meeting_active gate, handleCaravanStatus above).
        df::meeting_diplomat_info *dip = nullptr;
        for (auto *d : df::global::plotinfo->dip_meeting_info) {
            if (d && d->civ_id == car->entity) { dip = d; break; }
        }
        os << ",\"meeting_active\":" << (dip ? "true" : "false");
        if (dip) {
            os << ",\"topics_under_discussion\":[";
            for (size_t i = 0; i < dip->topic_list.size(); i++) {
                if (i > 0) os << ",";
                os << jsonStr(ENUM_KEY_STR(meeting_topic, dip->topic_list[i]));
            }
            os << "]";

            std::string diplomatName, associateName;
            if (df::historical_figure *hf = df::historical_figure::find(dip->diplomat_id)) {
                diplomatName = Translation::translateName(&hf->name, true);
            }
            if (df::historical_figure *hf = df::historical_figure::find(dip->associate_id)) {
                associateName = Translation::translateName(&hf->name, true);
            }
            os << ",\"diplomat\":" << jsonStr(diplomatName)
               << ",\"associate\":" << jsonStr(associateName);

            os << ",\"resolved_this_meeting\":[";
            bool firstEv = true;
            for (auto *ev : dip->events) {
                if (!ev) continue;
                if (!firstEv) os << ",";
                firstEv = false;
                os << "{\"type\":" << jsonStr(ENUM_KEY_STR(meeting_event_type, ev->type))
                   << ",\"topic\":" << jsonStr(ENUM_KEY_STR(meeting_topic, ev->topic))
                   << ",\"quota_total\":" << jsonInt(ev->quota_total)
                   << ",\"quota_remaining\":" << jsonInt(ev->quota_remaining)
                   << ",\"year\":" << jsonInt(ev->year)
                   << ",\"ticks\":" << jsonInt(ev->ticks)
                   << "}";
            }
            os << "]";
        }

        os << "}";
    }
    os << "]}";

    status = QUERY_STATUS_SUCCESS;
    return os.str();
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
        } else if (name == "list_mandates") {
            data = handleListMandates(args, status);
        } else if (name == "work_details") {
            data = handleListWorkDetails(args, status);
        } else if (name == "moods") {
            data = handleListMoods(args, status);
        } else if (name == "noble_demands") {
            data = handleNobleDemands(args, status);
        } else if (name == "position_vacancies") {
            data = handlePositionVacancies(args, status);
        } else if (name == "caravan_status") {
            data = handleCaravanStatus(args, status);
        } else if (name == "depot_goods") {
            data = handleDepotGoods(args, status);
        } else if (name == "trade_agreements") {
            data = handleTradeAgreements(args, status);
        } else if (name == "fort_wealth") {
            data = handleFortWealth(args, status);
        } else if (name == "wellbeing") {
            data = handleWellbeing(args, status);
        } else if (name == "dwarf_detail") {
            data = handleDwarfDetail(args, status);
        } else if (name == "dwarf_portrait") {
            data = handleDwarfPortrait(args, status);
        } else if (name == "combat_summary") {
            data = handleCombatSummary(args, status);
        } else if (name == "combat_log") {
            data = handleCombatLog(args, status);
        } else if (name == "story_pulse") {
            data = handleStoryPulse(args, status);
        } else if (name == "social_graph") {
            data = handleSocialGraph(args, status);
        } else if (name == "fort_art") {
            data = handleFortArt(args, status);
        } else if (name == "building_status") {
            data = handleBuildingStatus(args, status);
        } else if (name == "workshop_jobs") {
            data = handleWorkshopJobs(args, status);
        } else if (name == "list_buildings") {
            data = handleListBuildings(args, status);
        } else if (name == "list_zones") {
            data = handleListZones(args, status);
        } else if (name == "list_wildlife") {
            data = handleListWildlife(args, status);
        } else if (name == "zone_value") {
            data = handleZoneValue(args, status);
        } else if (name == "list_locations") {
            data = handleListLocations(args, status);
        } else if (name == "list_burrows") {
            data = handleListBurrows(args, status);
        } else if (name == "list_squads") {
            data = handleListSquads(args, status);
        } else if (name == "list_crops") {
            data = handleListCrops(args, status);
        } else if (name == "stockpile_inventory") {
            data = handleStockpileInventory(args, status);
        } else if (name == "sim_status") {
            data = handleSimStatus(args, status);
        } else if (name == "map_slice") {
            data = queryMapSlice(args, status);
        } else if (name == "column_profile") {
            data = queryColumnProfile(args, status);
        } else if (name == "fort_footprint") {
            data = handleFortFootprint(args, status);
        } else if (name == "region_scan") {
            data = handleRegionScan(args, status);
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
