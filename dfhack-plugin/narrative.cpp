// narrative.cpp -- Feature 013 "narrative layer" query handlers.
//
// Wave 013-A / Lane 1 (research: specs/013-narrative-layer/research.md
// 2.1 + 3.1): the shared citizen-hfid/thought-resolver helpers (narrative.h)
// and the dwarf_portrait query -- a single dwarf's salient psyche/social/
// history snapshot, ~1KB of JSON that the Go side (tools_state.go's
// dwarf_detail portrait=true param) turns into 15-25 prose-ish lines.
//
// Follows the trade.cpp/military.cpp precedent for this plugin's file
// organization: query handlers live here, are forward-declared right above
// their dispatch line in queries.cpp's executeQuery, and this file keeps
// its own small, suffixed JSON-construction helpers rather than reaching
// into queries.cpp's (which are `static`, i.e. private to that
// translation unit) -- see e.g. military.cpp's jsonStrSquad/burrows.cpp's
// jsonStrBurrow for the identical precedent.
//
// Threading: like every other query handler in this plugin, executeQuery
// calls into this file from the main DF thread only (see queries.cpp's
// own threading note) -- safe to read df::global state directly.

#include "Core.h"
#include "modules/Units.h"
#include "modules/Materials.h"
#include "modules/Translation.h"
#include "MiscUtils.h"

#include "df/world.h"
#include "df/unit.h"
#include "df/unit_relationship_type.h"
#include "df/unit_soul.h"
#include "df/unit_personality.h"
#include "df/personality_needst.h"
#include "df/need_type.h"
#include "df/personality_moodst.h"
#include "df/emotion_type.h"
#include "df/unit_thought_type.h"
#include "df/personality_valuest.h"
#include "df/value_type.h"
#include "df/personality_facet_type.h"
#include "df/unit_preference.h"
#include "df/unitpref_type.h"
#include "df/item_type.h"
#include "df/creature_raw.h"
#include "df/caste_raw.h"
#include "df/descriptor_color.h"
#include "df/descriptor_shape.h"
#include "df/poetic_form.h"
#include "df/musical_form.h"
#include "df/dance_form.h"
#include "df/historical_figure.h"
#include "df/historical_figure_info.h"
#include "df/historical_figure_relationships.h"
#include "df/relationship_profile_hf_visualst.h"
#include "df/core_hf_relationshipst.h"
#include "df/vague_relationship_type.h"
#include "df/histfig_hf_link.h"
#include "df/histfig_hf_link_type.h"
#include "df/histfig_entity_link.h"
#include "df/histfig_entity_link_type.h"
#include "df/histfig_relationship_type.h"
#include "df/historical_entity.h"
#include "df/historical_entity_type.h"
#include "df/knowledge_profilest.h"
#include "df/artistic_profilest.h"
#include "df/historical_kills.h"

// Combat narrator (wave 013-A / Lane 2, research 2.3 + 3.3) -- world.status
// .reports (the report.h struct, NOT df.report.xml -- research's own
// headline correction), d_init's per-type UNIT_COMBAT_REPORT flag, the
// per-unit Combat/Sparring report-id logs, and the wound/incident structs
// momentum is read from.
#include "df/global_objects.h"
#include "df/report.h"
#include "df/announcement_type.h"
#include "df/announcement_flag.h"
#include "df/announcement_flags.h"
#include "df/d_init.h"
#include "df/unit_report_type.h"
#include "df/unit_flags2.h"
#include "df/unit_wound.h"
#include "df/unit_wound_layerst.h"
#include "df/unit_wound_flag.h"
#include "df/wound_damage_flags1.h"
#include "df/wound_effect_type.h"
#include "df/incident.h"
#include "df/death_type.h"

// The fort's own art (wave 013-C / Lane 4, research 2.5 + 3.2 mode=art) --
// the four `df.global.world.*` form/content registries (poetic_form.h/
// musical_form.h/dance_form.h are already included above for
// dwarf_portrait's LikePoeticForm/LikeMusicalForm/LikeDanceForm preference
// rendering; written_content.h is new here), plotinfost.h for
// plotinfo->site_id (the fort-locality match target), and the four
// history-event classes that carry `site` -- the ONLY per-desire join this
// section needs (research's own "one join caveat").
#include "df/plotinfost.h"
#include "df/written_content.h"
#include "df/history_event.h"
#include "df/history_event_type.h"
#include "df/history_event_poetic_form_createdst.h"
#include "df/history_event_musical_form_createdst.h"
#include "df/history_event_dance_form_createdst.h"
#include "df/history_event_written_content_composedst.h"

#include "protocol.h"
#include "narrative.h"

#include <algorithm>
#include <chrono>
#include <cstdint>
#include <cstdio>
#include <deque>
#include <functional>
#include <mutex>
#include <sstream>
#include <string>
#include <unordered_map>
#include <unordered_set>
#include <utility>
#include <vector>

using namespace DFHack;

// ---------------------------------------------------------------------------
// JSON construction helpers -- local copies, suffixed to stay out of the way
// of queries.cpp's own `static` (and therefore already link-safe) helpers of
// the same shape. Mirrors military.cpp's jsonStrSquad / burrows.cpp's
// jsonStrBurrow precedent.
// ---------------------------------------------------------------------------

static std::string jsonEscapeNarrative(const std::string &s) {
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

static std::string jsonStrNarrative(const std::string &s) {
    return "\"" + jsonEscapeNarrative(s) + "\"";
}
static std::string jsonIntNarrative(int64_t v) {
    char buf[32]; snprintf(buf, sizeof(buf), "%lld", (long long)v); return std::string(buf);
}
static std::string jsonErrorNarrative(const std::string &msg) {
    return "{\"error\":" + jsonStrNarrative(msg) + "}";
}

// Same minimal top-level-key reader queries.cpp uses (jsonGetString/
// jsonGetInt there) -- duplicated rather than shared because those are
// `static` (private to queries.cpp's translation unit).
static std::string jsonGetStringNarrative(const std::string &args, const std::string &key) {
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
    size_t end = pos;
    while (end < args.size() && args[end] != ',' && args[end] != '}') end++;
    return args.substr(pos, end - pos);
}

static int64_t jsonGetIntNarrative(const std::string &args, const std::string &key, int64_t def) {
    std::string s = jsonGetStringNarrative(args, key);
    if (s.empty()) return def;
    try { return std::stoll(s); } catch (...) { return def; }
}

// ---------------------------------------------------------------------------
// Shared helpers (narrative.h) -- citizen index + thought/subthought
// resolver. Both exported for pulse/social/art query handlers in later
// waves; dwarf_portrait below is their first consumer.
// ---------------------------------------------------------------------------

CitizenIndex build_citizen_index() {
    CitizenIndex idx;
    if (!df::global::world) return idx;
    for (auto *unit : df::global::world->units.active) {
        if (!unit || !Units::isCitizen(unit)) continue;
        if (unit->hist_figure_id < 0) continue;
        idx.hfids.insert(unit->hist_figure_id);
        idx.hfidToUnit[unit->hist_figure_id] = unit;
    }
    return idx;
}

// resolve_thought -- see narrative.h's doc comment for the three curated
// resolution paths and why unresolved is the truthful default everywhere
// else. All hex codes and phrasings below are transcribed verbatim from
// this checkout's df.personality.xml inline comments (NOT enum-attrs --
// DFHack ships no structured resolver for any of these, confirmed by
// exhaustive grep per specs/013-narrative-layer/research.md 2.1c) or, for
// RelativeExpelled, decoded via a normal (bounds-checked) enum lookup.
ThoughtDetail resolve_thought(df::unit_thought_type thought, int32_t subthought) {
    ThoughtDetail out;
    const char *cap = ENUM_ATTR(unit_thought_type, caption, thought);
    out.caption = cap ? cap : "";

    switch (thought) {
    // circumstance_id's union member names (Death/Prayer/DreamAbout/
    // Defeated/Murdered) match these five thought-type names exactly --
    // df.personality.xml's own documented intent (research 2.1b) for
    // "subthought is a historical_figure id" on this class only.
    case df::unit_thought_type::Death:
    case df::unit_thought_type::Prayer:
    case df::unit_thought_type::DreamAbout:
    case df::unit_thought_type::Defeated:
    case df::unit_thought_type::Murdered: {
        df::historical_figure *hf = df::historical_figure::find(subthought);
        if (hf) {
            std::string name = Translation::translateName(&hf->name, true);
            // Plausibility gate: a found-but-unnamed figure (translateName
            // returning empty) is not surfaced as a resolved name -- the
            // raw int stays the truthful fallback instead of a blank.
            if (!name.empty()) {
                out.subthoughtResolved = true;
                out.subthoughtText = name;
            }
        }
        break;
    }
    // Meeting-topic codes -- df.personality.xml:230-234 (Complained's own
    // inline comment lists 0x19-0x1D).
    case df::unit_thought_type::Complained:
        switch (subthought) {
        case 0x19: out.subthoughtText = "job scarcity"; out.subthoughtResolved = true; break;
        case 0x1A: out.subthoughtText = "work allocation suggestions"; out.subthoughtResolved = true; break;
        case 0x1B: out.subthoughtText = "weapon production request"; out.subthoughtResolved = true; break;
        case 0x1C: out.subthoughtText = "yelling at somebody in charge"; out.subthoughtResolved = true; break;
        case 0x1D: out.subthoughtText = "crying on somebody in charge"; out.subthoughtResolved = true; break;
        default: break;
        }
        break;
    // UnableComplain's own inline comment (df.personality.xml:326-330) --
    // same code range as Complained, different (failed-to-complain)
    // phrasing.
    case df::unit_thought_type::UnableComplain:
        switch (subthought) {
        case 0x19: out.subthoughtText = "find somebody to complain to about job scarcity"; out.subthoughtResolved = true; break;
        case 0x1A: out.subthoughtText = "make suggestions about work allocation"; out.subthoughtResolved = true; break;
        case 0x1B: out.subthoughtText = "request weapon production"; out.subthoughtResolved = true; break;
        case 0x1C: out.subthoughtText = "find somebody in charge to yell at"; out.subthoughtResolved = true; break;
        case 0x1D: out.subthoughtText = "find somebody in charge to cry on"; out.subthoughtResolved = true; break;
        default: break;
        }
        break;
    // ReceivedComplaint's own inline comment (df.personality.xml:239-240)
    // -- only the yelled-at/cried-on codes are documented for this one.
    case df::unit_thought_type::ReceivedComplaint:
        switch (subthought) {
        case 0x1C: out.subthoughtText = "being yelled at by an unhappy citizen"; out.subthoughtResolved = true; break;
        case 0x1D: out.subthoughtText = "being cried on by an unhappy citizen"; out.subthoughtResolved = true; break;
        default: break;
        }
        break;
    // GhostNightmare/GhostHaunt share one relative-type code table
    // (df.personality.xml:294-317) -- the gaps (0x04-0x08, 0x0A, 0x0F-0x11)
    // are genuinely undocumented in this checkout, not an oversight here.
    case df::unit_thought_type::GhostNightmare:
    case df::unit_thought_type::GhostHaunt:
        switch (subthought) {
        case 0x00: out.subthoughtText = "a dead pet"; out.subthoughtResolved = true; break;
        case 0x01: out.subthoughtText = "a dead spouse"; out.subthoughtResolved = true; break;
        case 0x02: out.subthoughtText = "a dead mother"; out.subthoughtResolved = true; break;
        case 0x03: out.subthoughtText = "a dead father"; out.subthoughtResolved = true; break;
        case 0x09: out.subthoughtText = "a dead lover"; out.subthoughtResolved = true; break;
        case 0x0B: out.subthoughtText = "a dead sibling"; out.subthoughtResolved = true; break;
        case 0x0C: out.subthoughtText = "a dead child"; out.subthoughtResolved = true; break;
        case 0x0D: out.subthoughtText = "a dead friend"; out.subthoughtResolved = true; break;
        case 0x0E: out.subthoughtText = "a dead still-annoying acquaintance"; out.subthoughtResolved = true; break;
        case 0x12: out.subthoughtText = "a dead animal training partner"; out.subthoughtResolved = true; break;
        default: break;
        }
        break;
    // RelativeExpelled: df.personality.xml:1129 documents "subthought is
    // histfig_relationship_type" directly -- a real enum, safe to decode
    // via DFHack's own bounds-checked lookup rather than a hand-copied
    // table.
    case df::unit_thought_type::RelativeExpelled: {
        auto rel = (df::histfig_relationship_type)subthought;
        if (DFHack::is_valid_enum_item(rel)) {
            out.subthoughtResolved = true;
            out.subthoughtText = ENUM_KEY_STR(histfig_relationship_type, rel);
        }
        break;
    }
    default:
        break;
    }
    return out;
}

// ---------------------------------------------------------------------------
// dwarf_portrait -- single-dwarf salient snapshot (research 3.1).
// ---------------------------------------------------------------------------

// resolveHfDisplayName resolves a histfig id to a display string: a live
// citizen's own Units::getReadableName (kept consistent with every other
// person-name field this plugin emits, e.g. handleDwarfDetail's
// first_name) when the hfid is one of the fort's own citizens, else
// Translation::translateName on the historical_figure's own stored name --
// this is what resolves an off-site spouse, a dead relative, or a foreign
// deity, none of which have a live df::unit. Returns "" (never a guess)
// when hfid doesn't resolve to any historical_figure at all.
static std::string resolveHfDisplayName(int32_t hfid, const CitizenIndex &idx) {
    auto it = idx.hfidToUnit.find(hfid);
    if (it != idx.hfidToUnit.end()) return Units::getReadableName(it->second);
    df::historical_figure *hf = df::historical_figure::find(hfid);
    if (!hf) return "";
    return Translation::translateName(&hf->name, true);
}

// resolvePreferenceLabel renders one soul.preferences entry to a display
// string using the same per-type field selection
// scripts/assign-preferences.lua's format_preference() uses (research
// 2.1c) -- MaterialInfo::toString for the three material-backed types
// (LikeTree completes the pattern LikePlant already establishes: both are
// mattype/matindex, wood being a material like any other), a direct enum
// name for LikeItem, creature_raw's own name for LikeCreature/HateCreature
// (LikeCreature is the unhandled twin of HateCreature's own creature_id
// field -- same union member), descriptor lookups for LikeColor/LikeShape,
// and Translation::translateName for the three composed-form types.
static std::string resolvePreferenceLabel(df::unit_preference *pref) {
    switch ((df::unitpref_type)pref->type) {
    case df::unitpref_type::LikeMaterial:
    case df::unitpref_type::LikeFood:
    case df::unitpref_type::LikePlant:
    case df::unitpref_type::LikeTree: {
        MaterialInfo mi((int16_t)pref->mattype, (int32_t)pref->matindex);
        if (mi.isValid()) return mi.toString();
        return "material #" + std::to_string(pref->mattype) + ":" + std::to_string(pref->matindex);
    }
    case df::unitpref_type::LikeItem:
        return ENUM_KEY_STR(item_type, (df::item_type)pref->item_type);
    case df::unitpref_type::LikeCreature:
    case df::unitpref_type::HateCreature: {
        df::creature_raw *craw = df::creature_raw::find(pref->creature_id);
        return craw ? craw->name[0] : ("creature #" + std::to_string(pref->creature_id));
    }
    case df::unitpref_type::LikeColor: {
        df::descriptor_color *c = vector_get(df::global::world->raws.descriptors.colors, (unsigned)pref->color_id, (df::descriptor_color *)nullptr);
        if (!c) return "color #" + std::to_string(pref->color_id);
        return c->name.empty() ? c->id : c->name;
    }
    case df::unitpref_type::LikeShape: {
        df::descriptor_shape *s = vector_get(df::global::world->raws.descriptors.shapes, (unsigned)pref->shape_id, (df::descriptor_shape *)nullptr);
        if (!s) return "shape #" + std::to_string(pref->shape_id);
        return s->name.empty() ? s->id : s->name;
    }
    case df::unitpref_type::LikePoeticForm: {
        df::poetic_form *f = vector_get(df::global::world->poetic_forms.all, (unsigned)pref->poetic_form_id, (df::poetic_form *)nullptr);
        return f ? Translation::translateName(&f->name, true) : ("poetic form #" + std::to_string(pref->poetic_form_id));
    }
    case df::unitpref_type::LikeMusicalForm: {
        df::musical_form *f = vector_get(df::global::world->musical_forms.all, (unsigned)pref->musical_form_id, (df::musical_form *)nullptr);
        return f ? Translation::translateName(&f->name, true) : ("musical form #" + std::to_string(pref->musical_form_id));
    }
    case df::unitpref_type::LikeDanceForm: {
        df::dance_form *f = vector_get(df::global::world->dance_forms.all, (unsigned)pref->dance_form_id, (df::dance_form *)nullptr);
        return f ? Translation::translateName(&f->name, true) : ("dance form #" + std::to_string(pref->dance_form_id));
    }
    default:
        return "";
    }
}

// preferenceRank orders soul.preferences for the cap-3 selection: 0 =
// LikeMaterial (proven mood insurance -- strangemood.cpp consumes exactly
// this type, research 2.1c), 1 = LikeFood, 2 = LikeItem (the *suspected*
// mandate driver, unproven -- see the mandate caveat in the portrait
// render), 3 = everything else (one flavor pick).
static int preferenceRank(df::unitpref_type t) {
    switch (t) {
    case df::unitpref_type::LikeMaterial: return 0;
    case df::unitpref_type::LikeFood: return 1;
    case df::unitpref_type::LikeItem: return 2;
    default: return 3;
    }
}

// kPortraitMaxFacets/kPortraitMaxValues/kPortraitMaxPreferences -- salience
// caps (research 3.1's "cap 3-5" / "top 2-3" / "cap 3"), named the same way
// DWARF_TOP_FACETS/DWARF_EMOTIONS_CAP already are for dwarf_detail.
static const int kPortraitMaxFacets = 5;
static const int kPortraitMaxValues = 3;
static const int kPortraitMaxPreferences = 3;

// kPortraitFacetMaxIndex/kPortraitNeutralLo/Hi -- the facet 7-band table's
// Neutral band (40-60 inclusive, research 2.1a) is what "non-neutral"
// filters against; DWARF_FACET_MAX_INDEX (queries.cpp) documents the same
// 0..49 bound this checkout's personality_facet_type enum has, duplicated
// here rather than shared across a `static` boundary.
static const int kPortraitFacetMaxIndex = 49;
static const int kPortraitNeutralLo = 40;
static const int kPortraitNeutralHi = 60;

// kPortraitValueExtremeThreshold -- |strength| >= 26 is the Very-High/Very-
// Low value-tier boundary (research 2.1a's belief tier table: 26..40 and
// -40..-26), the threshold that selects "extreme" values worth reporting.
static const int kPortraitValueExtremeThreshold = 26;

// kPortraitNeedStarvedThreshold -- focus_level <= -1000 is "Unfocused" or
// worse on the need fulfillment tier table (research 2.1a).
static const int kPortraitNeedStarvedThreshold = -1000;

// History-hook entity types worth surfacing as a portrait membership line
// -- Civilization/SiteGovernment/VesselCrew/MigratingGroup/NomadicGroup/
// Outcast are the entity types every citizen already belongs to by default
// and would be pure noise here; these five are the "band/guild/faith"
// affiliations research 2.1f calls out.
static bool isPortraitHistoryEntityType(df::historical_entity_type t) {
    switch (t) {
    case df::historical_entity_type::Religion:
    case df::historical_entity_type::PerformanceTroupe:
    case df::historical_entity_type::MerchantCompany:
    case df::historical_entity_type::Guild:
    case df::historical_entity_type::MilitaryUnit:
        return true;
    default:
        return false;
    }
}

std::string handleDwarfPortrait(const std::string &args, uint8_t &status) {
    int64_t id = jsonGetIntNarrative(args, "id", -1);
    if (id < 0) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("missing or invalid id");
    }
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world is null");
    }

    df::unit *u = nullptr;
    for (auto *unit : df::global::world->units.active) {
        if (unit && (int64_t)unit->id == id) { u = unit; break; }
    }
    if (!u) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("unit not found");
    }

    std::ostringstream os;
    os << "{\"id\":" << jsonIntNarrative(u->id)
       << ",\"name\":" << jsonStrNarrative(Units::getReadableName(u));

    df::caste_raw *craw = Units::getCasteRaw(u);
    os << ",\"caste_description\":" << jsonStrNarrative(craw ? craw->description : "");

    // Size band vs the species median -- unit-info-viewer.lua's own
    // thresholds (research 2.1e): >=110 larger, <=90 smaller, else
    // average. NOTE: df::unit's own nested appearance compound names this
    // field `size_modifier` (matching the XML's declared name); the
    // standalone df::unit_appearance class used by unit.appearances (a
    // different, unrelated vector -- per-snapshot history, not this
    // dwarf's current appearance) instead names it
    // total_appearance_size_modifier (the XML original-name) -- verified
    // directly against df/unit.h's own nested T_appearance rather than
    // assumed from either the XML or the other struct.
    const char *sizeBand = "average";
    int32_t sizeMod = u->appearance.size_modifier;
    if (sizeMod >= 110) sizeBand = "larger";
    else if (sizeMod <= 90) sizeBand = "smaller";
    os << ",\"size_band\":" << jsonStrNarrative(sizeBand);

    os << ",\"stress_category\":" << jsonIntNarrative(Units::getStressCategory(u));

    if (u->status.current_soul) {
        auto &p = u->status.current_soul->personality;

        // Top non-neutral facets by |deviation from 50|, cap
        // kPortraitMaxFacets. Raw value only -- tier banding/label text is
        // a Go-side render concern (house rule: prose lives in Go).
        {
            struct FacetDev { int idx; int16_t value; int dev; };
            std::vector<FacetDev> facets;
            facets.reserve(kPortraitFacetMaxIndex + 1);
            for (int i = 0; i <= kPortraitFacetMaxIndex; i++) {
                int16_t v = p.traits[i];
                if (v >= kPortraitNeutralLo && v <= kPortraitNeutralHi) continue; // Neutral band -- not salient
                int dev = (int)v - 50;
                if (dev < 0) dev = -dev;
                facets.push_back(FacetDev{i, v, dev});
            }
            std::sort(facets.begin(), facets.end(), [](const FacetDev &a, const FacetDev &b) {
                return a.dev > b.dev;
            });
            os << ",\"facets\":[";
            for (int i = 0; i < kPortraitMaxFacets && i < (int)facets.size(); i++) {
                if (i) os << ",";
                os << "{\"facet\":" << jsonStrNarrative(ENUM_KEY_STR(personality_facet_type, (df::personality_facet_type)facets[i].idx))
                   << ",\"value\":" << jsonIntNarrative(facets[i].value)
                   << "}";
            }
            os << "]";
        }

        // Extreme values (|strength| >= kPortraitValueExtremeThreshold),
        // cap kPortraitMaxValues.
        {
            std::vector<df::personality_valuest *> vals;
            for (auto *v : p.values) {
                if (!v) continue;
                int s = v->strength;
                if (s < 0) s = -s;
                if (s >= kPortraitValueExtremeThreshold) vals.push_back(v);
            }
            std::sort(vals.begin(), vals.end(), [](df::personality_valuest *a, df::personality_valuest *b) {
                int sa = a->strength < 0 ? -a->strength : a->strength;
                int sb = b->strength < 0 ? -b->strength : b->strength;
                return sa > sb;
            });
            os << ",\"values\":[";
            for (int i = 0; i < kPortraitMaxValues && i < (int)vals.size(); i++) {
                if (i) os << ",";
                os << "{\"value_type\":" << jsonStrNarrative(ENUM_KEY_STR(value_type, vals[i]->type))
                   << ",\"strength\":" << jsonIntNarrative(vals[i]->strength)
                   << "}";
            }
            os << "]";
        }

        // Needs -- every starved need (focus_level <= threshold) plus the
        // single best-fed one (max focus_level across all needs, reported
        // independent of the starved filter -- see the doc comment on
        // kPortraitNeedStarvedThreshold's call site for why these can, in
        // a pathological case, be the same entry).
        {
            os << ",\"needs_starved\":[";
            bool first = true;
            df::personality_needst *bestFed = nullptr;
            for (auto *n : p.needs) {
                if (!n) continue;
                if (!bestFed || n->focus_level > bestFed->focus_level) bestFed = n;
                if (n->focus_level > kPortraitNeedStarvedThreshold) continue;
                if (!first) os << ",";
                first = false;
                os << "{\"need_type\":" << jsonStrNarrative(n->id == df::need_type::NONE ? "NONE" : ENUM_KEY_STR(need_type, n->id))
                   << ",\"focus_level\":" << jsonIntNarrative(n->focus_level)
                   << ",\"need_level\":" << jsonIntNarrative(n->need_level)
                   << "}";
            }
            os << "]";
            if (bestFed) {
                os << ",\"need_best_fed\":{\"need_type\":" << jsonStrNarrative(bestFed->id == df::need_type::NONE ? "NONE" : ENUM_KEY_STR(need_type, bestFed->id))
                   << ",\"focus_level\":" << jsonIntNarrative(bestFed->focus_level)
                   << ",\"need_level\":" << jsonIntNarrative(bestFed->need_level)
                   << "}";
            } else {
                os << ",\"need_best_fed\":null";
            }
        }

        // Strongest recent emotion -- max |strength| across all recorded
        // emotions, most-recent (year,year_tick) as the tiebreak. "Recent"
        // is the tiebreak rather than the primary sort key deliberately:
        // a genuinely dominant old emotion is more salient for a portrait
        // than a trivial fresh one, and dwarf_detail's own recency-sorted
        // list already covers the pure-recency view.
        {
            df::personality_moodst *strongest = nullptr;
            for (auto *e : p.emotions) {
                if (!e) continue;
                if (!strongest) { strongest = e; continue; }
                int se = e->strength < 0 ? -e->strength : e->strength;
                int ss = strongest->strength < 0 ? -strongest->strength : strongest->strength;
                if (se > ss) { strongest = e; continue; }
                if (se == ss) {
                    if (e->year > strongest->year || (e->year == strongest->year && e->year_tick > strongest->year_tick))
                        strongest = e;
                }
            }
            if (strongest) {
                ThoughtDetail td = resolve_thought(strongest->thought, strongest->subthought);
                int8_t divider = ENUM_ATTR(emotion_type, divider, strongest->type);
                os << ",\"emotion\":{\"emotion\":" << jsonStrNarrative(strongest->type == df::emotion_type::ANYTHING ? "NONE" : ENUM_KEY_STR(emotion_type, strongest->type))
                   << ",\"strength\":" << jsonIntNarrative(strongest->strength)
                   << ",\"divider\":" << jsonIntNarrative(divider)
                   << ",\"thought\":" << jsonStrNarrative(strongest->thought == df::unit_thought_type::None ? "None" : ENUM_KEY_STR(unit_thought_type, strongest->thought))
                   << ",\"thought_caption\":" << jsonStrNarrative(td.caption)
                   << ",\"subthought\":" << jsonIntNarrative(strongest->subthought)
                   << ",\"subthought_resolved\":" << (td.subthoughtResolved ? "true" : "false")
                   << ",\"subthought_text\":" << jsonStrNarrative(td.subthoughtText)
                   << "}";
            } else {
                os << ",\"emotion\":null";
            }
        }

        // Preferences -- soul.preferences (NOT
        // unit_personality.preferences, the worldgen-only decoy struct --
        // research 2.1c's two-struct trap), visible-flag gated, ranked and
        // capped via preferenceRank/kPortraitMaxPreferences.
        {
            std::vector<df::unit_preference *> prefs;
            for (auto *pr : u->status.current_soul->preferences) {
                if (!pr || !pr->flags.bits.visible) continue;
                prefs.push_back(pr);
            }
            std::stable_sort(prefs.begin(), prefs.end(), [](df::unit_preference *a, df::unit_preference *b) {
                return preferenceRank(a->type) < preferenceRank(b->type);
            });
            os << ",\"preferences\":[";
            for (int i = 0; i < kPortraitMaxPreferences && i < (int)prefs.size(); i++) {
                if (i) os << ",";
                os << "{\"type\":" << jsonStrNarrative(ENUM_KEY_STR(unitpref_type, prefs[i]->type))
                   << ",\"label\":" << jsonStrNarrative(resolvePreferenceLabel(prefs[i]))
                   << "}";
            }
            os << "]";
        }
    } else {
        os << ",\"facets\":[],\"values\":[],\"needs_starved\":[],\"need_best_fed\":null"
           << ",\"emotion\":null,\"preferences\":[]";
    }

    // Deity / relationships / history hook -- all require a resolved
    // historical_figure (a citizen always has one; hist_figure_id < 0
    // would mean otherwise, which this plugin has never observed for a
    // real dwarf but does not assume away).
    df::historical_figure *hf = (u->hist_figure_id >= 0) ? df::historical_figure::find(u->hist_figure_id) : nullptr;
    if (hf) {
        CitizenIndex idx = build_citizen_index();

        // Deity + relationship subset (spouse/lover/children) -- all read
        // off histfig_links, whose base fields (target_hf/link_strength)
        // are common to every link subtype (df/histfig_hf_link.h) so no
        // strict_virtual_cast is needed to reach them.
        {
            df::historical_figure *deityHf = nullptr;
            int16_t deityStrength = 0;
            int32_t spouseHfid = -1, loverHfid = -1;
            std::vector<int32_t> childHfids;

            for (auto *link : hf->histfig_links) {
                if (!link) continue;
                switch (link->getType()) {
                case df::histfig_hf_link_type::DEITY:
                    deityHf = df::historical_figure::find(link->target_hf);
                    deityStrength = link->link_strength;
                    break;
                case df::histfig_hf_link_type::SPOUSE:
                    spouseHfid = link->target_hf;
                    break;
                case df::histfig_hf_link_type::LOVER:
                    loverHfid = link->target_hf;
                    break;
                case df::histfig_hf_link_type::CHILD:
                    childHfids.push_back(link->target_hf);
                    break;
                default:
                    break;
                }
            }

            if (deityHf) {
                std::string name = Translation::translateName(&deityHf->name, true);
                os << ",\"deity\":{\"name\":" << jsonStrNarrative(name)
                   // link_strength's exact devotion scale is an open live
                   // probe (research 2.1d/§5.5) -- the raw number is
                   // reported here; the Go renderer adds the honest
                   // "scale unverified" note rather than banding it.
                   << ",\"link_strength\":" << jsonIntNarrative(deityStrength)
                   << "}";
            } else {
                os << ",\"deity\":null";
            }

            auto emitHfOrNull = [&](const char *key, int32_t hfid) {
                os << "," << jsonStrNarrative(key) << ":";
                if (hfid < 0) { os << "null"; return; }
                std::string name = resolveHfDisplayName(hfid, idx);
                if (name.empty()) { os << "null"; return; }
                os << "{\"name\":" << jsonStrNarrative(name) << "}";
            };
            emitHfOrNull("spouse", spouseHfid);
            emitHfOrNull("lover", loverHfid);

            os << ",\"children\":[";
            bool firstChild = true;
            for (int32_t chfid : childHfids) {
                std::string name = resolveHfDisplayName(chfid, idx);
                if (name.empty()) continue;
                if (!firstChild) os << ",";
                firstChild = false;
                os << "{\"name\":" << jsonStrNarrative(name) << "}";
            }
            os << "]";
        }

        // Best friend (max core.love) / worst grudge (min core.love <=
        // -50) among hf_visual entries filtered to the fort's own
        // citizen-hfid set (research 2.4 tier 3).
        {
            df::relationship_profile_hf_visualst *bestFriend = nullptr;
            df::relationship_profile_hf_visualst *worstGrudge = nullptr;
            if (hf->info && hf->info->relationships) {
                for (auto *rel : hf->info->relationships->hf_visual) {
                    if (!rel) continue;
                    if (rel->histfig_id == hf->id) continue; // never self
                    if (idx.hfids.find(rel->histfig_id) == idx.hfids.end()) continue; // citizens only
                    if (!bestFriend || rel->core.love > bestFriend->core.love) bestFriend = rel;
                    if (rel->core.love <= -50 && (!worstGrudge || rel->core.love < worstGrudge->core.love)) worstGrudge = rel;
                }
            }
            auto emitRel = [&](const char *key, df::relationship_profile_hf_visualst *rel) {
                os << "," << jsonStrNarrative(key) << ":";
                if (!rel) { os << "null"; return; }
                std::string name = resolveHfDisplayName(rel->histfig_id, idx);
                if (name.empty()) { os << "null"; return; }
                os << "{\"name\":" << jsonStrNarrative(name)
                   << ",\"love\":" << jsonIntNarrative(rel->core.love)
                   << ",\"meet_count\":" << jsonIntNarrative(rel->meet_count)
                   << "}";
            };
            emitRel("best_friend", bestFriend);
            emitRel("worst_grudge", worstGrudge);
        }

        // History hook: current/former troupe-like memberships (research
        // 2.1f) + known-forms/written-content counts + masterpiece/kill
        // counts (the artistic_profilest caveat -- research 2.1f/§5.7 --
        // is surfaced by the Go renderer, not silently hidden here).
        {
            os << ",\"memberships\":[";
            bool first = true;
            for (auto *link : hf->entity_links) {
                if (!link) continue;
                bool current = false;
                if (link->getType() == df::histfig_entity_link_type::MEMBER) current = true;
                else if (link->getType() == df::histfig_entity_link_type::FORMER_MEMBER) current = false;
                else continue;
                df::historical_entity *ent = df::historical_entity::find(link->entity_id);
                if (!ent || !isPortraitHistoryEntityType(ent->type)) continue;
                std::string name = Translation::translateName(&ent->name, true);
                if (name.empty()) continue;
                if (!first) os << ",";
                first = false;
                os << "{\"entity\":" << jsonStrNarrative(name)
                   << ",\"entity_type\":" << jsonStrNarrative(ENUM_KEY_STR(historical_entity_type, ent->type))
                   << ",\"status\":" << jsonStrNarrative(current ? "current" : "former")
                   << "}";
            }
            os << "]";

            int64_t knownPoetic = 0, knownMusical = 0, knownDance = 0, knownWritten = 0;
            if (hf->info && hf->info->known_info) {
                knownPoetic = (int64_t)hf->info->known_info->known_poetic_forms.size();
                knownMusical = (int64_t)hf->info->known_info->known_musical_forms.size();
                knownDance = (int64_t)hf->info->known_info->known_dance_forms.size();
                knownWritten = (int64_t)hf->info->known_info->known_written_contents.size();
            }
            int64_t masterpieces = (hf->info && hf->info->masterpieces) ? (int64_t)hf->info->masterpieces->events.size() : 0;
            int64_t kills = (hf->info && hf->info->kills) ? (int64_t)hf->info->kills->events.size() : 0;

            os << ",\"known_poetic_forms\":" << jsonIntNarrative(knownPoetic)
               << ",\"known_musical_forms\":" << jsonIntNarrative(knownMusical)
               << ",\"known_dance_forms\":" << jsonIntNarrative(knownDance)
               << ",\"known_written_contents\":" << jsonIntNarrative(knownWritten)
               << ",\"masterpieces\":" << jsonIntNarrative(masterpieces)
               << ",\"kills\":" << jsonIntNarrative(kills);
        }
    } else {
        os << ",\"deity\":null,\"spouse\":null,\"lover\":null,\"children\":[]"
           << ",\"best_friend\":null,\"worst_grudge\":null"
           << ",\"memberships\":[],\"known_poetic_forms\":0,\"known_musical_forms\":0"
           << ",\"known_dance_forms\":0,\"known_written_contents\":0,\"masterpieces\":0,\"kills\":0";
    }

    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// ---------------------------------------------------------------------------
// Combat narrator (wave 013-A / Lane 2, research 2.3 + 3.3).
//
// Independent max-id cursor over world.status.reports -- the authoritative
// append-only report log (df.announcement.xml:180; NOT world.status
// .announcements, the display SUBSET announcements.cpp's own ticker
// pipeline reads -- these are two different feeds over the same pooled
// df::report objects and this file never touches the other one). Combat-
// log lines are classified via d_init's own per-type UNIT_COMBAT_REPORT
// flag, continuation-stitched (flags.bits.continuation), and attributed to
// units via unit.reports.log[Combat] -- DF's OWN per-unit roster, not a
// hand-rolled relevance index. Momentum is computed entirely from
// structured wound/counter fields, never from report text.
// ---------------------------------------------------------------------------

// CombatReportRecord -- one continuation-stitched combat-log line kept in
// the rolling buffer. typeRaw is the report's announcement_type backing
// value (kept as a raw int16 rather than the enum so a default-constructed
// record never has to name a placeholder enumerator).
struct CombatReportRecord {
    int32_t id = -1;
    int32_t year = 0;
    int32_t time = 0;     // year_tick at report creation (df::report::time)
    int32_t x = -1, y = -1, z = -1;
    int16_t typeRaw = -1;
    std::string text;
    std::vector<int32_t> unitIds; // fight-relevant units, from reports.log[Combat]
};

// g_combat_mutex guards g_combat_report_cursor, g_combat_reports, and
// g_wound_watermarks together -- all three are advanced as one unit at
// ingest time and read together at query time, mirroring
// announcements.cpp's single g_repeat_count_mutex over its own related
// cursor + tail map. ingest_combat_reports runs wherever push_state_refresh
// runs (df_ai_protocol.cpp) -- the main thread's throttled plugin_onupdate
// poll or the socket thread under drain_from_socket_thread's suspension --
// the same two contexts announcements.cpp's own tripwire plumbing
// documents; combat_summary/combat_log run from the query-drain path,
// which is one of those same two contexts.
static std::mutex g_combat_mutex;
static int32_t g_combat_report_cursor = -1;
static std::deque<CombatReportRecord> g_combat_reports; // oldest first, bounded
static std::unordered_map<int32_t, int32_t> g_wound_watermarks; // unit id -> body.wound_next_id snapshot

// kCombatReportBufferCap bounds the rolling raw-line buffer regardless of
// fort age -- df::report objects are never deleted (53.15 pooling rule), so
// an unbounded buffer would grow for the fort's entire life. 400 lines
// comfortably covers combat_log's ~30-line cap plus enough history for
// combat_summary's cap-3 engagement clustering to find real boundaries
// between fights.
static const size_t kCombatReportBufferCap = 400;

// kEngagementWindowTicks -- DFHack's own RECENT_REPORT_TICKS
// (library/modules/Gui.cpp:107, `recent_report`'s own same-year-only
// window check): reports within this many year-ticks of each other, in the
// SAME game year, are treated as the same ongoing fight for both
// engagement clustering and wound-watermark expiry. This project accepts
// the identical year-wrap edge case DFHack's own function does (a fight
// spanning New Year's Eve at tick 0 can split across two "engagements") --
// not fixed here for the same reason it isn't fixed upstream.
static const int32_t kEngagementWindowTicks = 500;

// research 3.3's caps: "cap 3" engagements, "2-3" telling lines, "cap ~30"
// log lines.
static const size_t kCombatSummaryMaxEngagements = 3;
static const size_t kTellingLinesPerEngagement = 3;
static const size_t kCombatLogMaxLines = 30;

// kBleedingThresholdPercent -- a unit counts as "bleeding" once current
// blood is below this percent of max. Research 2.3 names the two fields
// (blood_count vs blood_max) but not a percentage; a strict "any deficit"
// reading would flag routine scarring/healing noise as bleeding, so this
// project picks a level a reader would actually call bleeding.
static const int32_t kBleedingThresholdPercent = 90;

// isCombatReportType classifies a raw announcement_type via d_init's own
// per-type UNIT_COMBAT_REPORT flag (library/xml/df.d_init.xml:169-182,
// research 2.3's headline classification mechanism) -- the same field DF's
// own Gui::addCombatReport machinery reads (library/modules/Gui.cpp:2065).
// Bounds-checked against the announcement_type enum's own declared range
// rather than trusting a raw wire value; a corrupt/foreign type reads as
// "not combat" rather than indexing out of the static array.
static bool isCombatReportType(int16_t typeRaw) {
    if (!df::global::d_init) return false;
    if (typeRaw < 0 || typeRaw > df::enum_traits<df::announcement_type>::last_item_value) return false;
    return df::global::d_init->announcements.flags[typeRaw].bits.UNIT_COMBAT_REPORT;
}

// CombatRosterInfo -- one ingest pass's worth of per-unit log lookups,
// scoped to report ids newer than the pre-ingest cursor so the tail-scan
// below stays O(new entries) regardless of how long a unit's Combat log
// has grown over the fort's life.
struct CombatRosterInfo {
    std::unordered_map<int32_t, std::vector<int32_t>> byReport; // report id -> unit ids (Combat log only)
    std::unordered_set<int32_t> sparringReportIds;               // report ids seen in ANY unit's Sparring log
};

// build_combat_roster walks df::global::world->units.active once, tail-
// scanning each unit's reports.log[Combat] and reports.log[Sparring] from
// the end while entries are newer than cursorBefore (a unit's own log is
// append-ordered, so this bounds the scan to just what's new since the
// last ingest). Combat entries build the report->unit roster this file's
// engagement grouping and momentum both key off of; Sparring entries build
// the exclusion set research 2.3 calls for ("sparring exclusion via
// reports.log[Sparring] membership -- NOT flags2.sparring, unreliable per
// the XML's own comment, df.unit.xml:1367").
static CombatRosterInfo build_combat_roster(int32_t cursorBefore) {
    CombatRosterInfo out;
    if (!df::global::world) return out;
    for (auto *unit : df::global::world->units.active) {
        if (!unit) continue;
        const auto &combatLog = unit->reports.log[df::unit_report_type::Combat];
        for (auto rit = combatLog.rbegin(); rit != combatLog.rend(); ++rit) {
            if (*rit <= cursorBefore) break;
            out.byReport[*rit].push_back(unit->id);
        }
        const auto &sparringLog = unit->reports.log[df::unit_report_type::Sparring];
        for (auto rit = sparringLog.rbegin(); rit != sparringLog.rend(); ++rit) {
            if (*rit <= cursorBefore) break;
            out.sparringReportIds.insert(*rit);
        }
    }
    return out;
}

// ingest_combat_reports -- the step-boundary hook (called from
// push_state_refresh, df_ai_protocol.cpp; same thread context the
// dangerous-wildlife tripwire already runs from). Advances the combat
// report cursor via a guarded binsearch (the manageReportEvent template,
// library/modules/EventManager.cpp:1105-1113 -- bounds-checked
// `idx < reports.size()` before every deref; NOT manageUnitAttackEvent's
// unguarded `reports[idx]->id` at :1140, a documented defect this project
// does not copy), stitches continuation lines onto their parent, excludes
// sparring, appends qualifying lines to the rolling buffer, and snapshots/
// expires per-unit wound watermarks.
void ingest_combat_reports() {
    if (!df::global::world) return;
    auto &reports = df::global::world->status.reports;
    if (reports.empty()) return;

    std::lock_guard<std::mutex> lock(g_combat_mutex);
    int32_t cursorBefore = g_combat_report_cursor;

    int rawIdx = df::report::binsearch_index(reports, cursorBefore, false);
    size_t idx = (rawIdx < 0) ? 0 : (size_t)rawIdx;
    while (idx < reports.size() && reports[idx]->id <= cursorBefore) idx++;
    if (idx >= reports.size()) return; // nothing new

    CombatRosterInfo roster = build_combat_roster(cursorBefore);

    std::vector<CombatReportRecord> newRecords;
    bool skipRun = false;
    int32_t lastSeenId = cursorBefore;
    for (; idx < reports.size(); idx++) {
        df::report *r = reports[idx];
        if (!r) continue;
        lastSeenId = r->id;

        if (r->flags.bits.continuation) {
            // Stitch onto the record this SAME pass just built. A
            // continuation whose parent wasn't kept (not combat-typed, or
            // sparring-excluded) or whose parent landed in a PRIOR ingest
            // pass has nothing to stitch onto -- honestly dropped rather
            // than fabricated as a standalone fragment.
            if (skipRun || newRecords.empty()) continue;
            newRecords.back().text += r->text;
            continue;
        }

        if (!isCombatReportType((int16_t)r->type)) { skipRun = true; continue; }
        if (roster.sparringReportIds.find(r->id) != roster.sparringReportIds.end()) { skipRun = true; continue; }

        skipRun = false;
        CombatReportRecord rec;
        rec.id = r->id;
        rec.year = r->year;
        rec.time = r->time;
        rec.x = r->pos.x; rec.y = r->pos.y; rec.z = r->pos.z;
        rec.typeRaw = (int16_t)r->type;
        rec.text = r->text;
        auto rit = roster.byReport.find(r->id);
        if (rit != roster.byReport.end()) rec.unitIds = rit->second;
        newRecords.push_back(std::move(rec));
    }
    g_combat_report_cursor = lastSeenId;

    std::unordered_set<int32_t> fightInvolved;
    for (auto &rec : newRecords) {
        for (int32_t uid : rec.unitIds) fightInvolved.insert(uid);
        g_combat_reports.push_back(std::move(rec));
    }
    while (g_combat_reports.size() > kCombatReportBufferCap) g_combat_reports.pop_front();

    // Snapshot a wound watermark the FIRST time a unit is noticed fighting
    // (research 2.3: "Snapshot at step start for units in fights; on read,
    // walk only wounds with id >= watermark"). KNOWN LIMITATION: this
    // ingest is reactive (it runs at the step boundary AFTER combat
    // already happened this pass, not a proactive pre-fight hook), so the
    // wound(s) that generated a unit's very first combat report this pass
    // may already be reflected in wound_next_id -- that one detection pass
    // can undercount by exactly the opening exchange. Every later step
    // boundary within the SAME engagement (same watermark, not yet
    // expired below) is fully accurate.
    for (int32_t uid : fightInvolved) {
        if (g_wound_watermarks.find(uid) != g_wound_watermarks.end()) continue;
        df::unit *u = df::unit::find(uid);
        if (u) g_wound_watermarks[uid] = u->body.wound_next_id;
    }

    // Expire watermarks for units no longer recently fighting, using DF's
    // OWN per-unit last-combat-report stamp (unit.reports.last_year[_tick]
    // -- ground truth independent of this file's own bounded buffer) so a
    // future engagement starts from a fresh baseline instead of an ancient
    // one. Same same-year window check as kEngagementWindowTicks's doc
    // comment.
    for (auto it = g_wound_watermarks.begin(); it != g_wound_watermarks.end(); ) {
        df::unit *u = df::unit::find(it->first);
        bool stillRecent = u && df::global::cur_year && df::global::cur_year_tick &&
            u->reports.last_year[df::unit_report_type::Combat] == *df::global::cur_year &&
            (*df::global::cur_year_tick - u->reports.last_year_tick[df::unit_report_type::Combat]) <= kEngagementWindowTicks;
        if (!stillRecent) it = g_wound_watermarks.erase(it);
        else ++it;
    }
}

// EngagementBuild -- one cluster from buildEngagements' temporal
// union-find: minId/maxId bound the report-id range, firstYear/firstTime
// and lastYear/lastTime bound the (year,year_tick) range (last* also
// doubles as the window anchor new records are compared against),
// unitIds/recordIdx are the accumulated roster and source-record indices.
struct EngagementBuild {
    int32_t minId = INT32_MAX;
    int32_t maxId = INT32_MIN;
    int32_t firstYear = -1, firstTime = -1;
    int32_t lastYear = 0, lastTime = 0;
    std::unordered_set<int32_t> unitIds;
    std::vector<size_t> recordIdx;
};

static int ufFind(std::vector<int> &parent, int x) {
    while (parent[x] != x) { parent[x] = parent[parent[x]]; x = parent[x]; }
    return x;
}

// isEarlier -- strict (year,tick) ordering helper for folding
// firstYear/firstTime across a merge (buildEngagements below).
static bool isEarlier(int32_t y1, int32_t t1, int32_t y2, int32_t t2) {
    if (y1 != y2) return y1 < y2;
    return t1 < t2;
}

// buildEngagements groups a time-ordered (oldest-first) slice of combat
// report records into engagements via a temporal union-find: two records
// land in the same engagement iff they share a unit id AND fall within
// kEngagementWindowTicks of each other in the same game year -- the
// PROVEN fallback research 3.3 calls for ("group by unit-id intersection +
// time proximity", DFHack's own RECENT_REPORT_TICKS constant). The real
// activity_event_conflictst join (report.activity_id) stays a labeled TODO
// behind live probe #1 (specs/013-narrative-layer/research.md §5.1) --
// landing it is a follow-up commit, not a blocker for this wave.
static std::vector<EngagementBuild> buildEngagements(const std::vector<CombatReportRecord> &records) {
    std::vector<EngagementBuild> clusters;
    std::vector<int> parent;
    std::unordered_map<int32_t, int> unitLastCluster;

    for (size_t i = 0; i < records.size(); i++) {
        const CombatReportRecord &rec = records[i];
        std::unordered_set<int> touchedRoots;
        for (int32_t uid : rec.unitIds) {
            auto it = unitLastCluster.find(uid);
            if (it == unitLastCluster.end()) continue;
            int root = ufFind(parent, it->second);
            if (clusters[root].recordIdx.empty()) continue; // defensive; see buildEngagements' merge note below
            const EngagementBuild &c = clusters[root];
            bool withinWindow = (c.lastYear == rec.year) &&
                (rec.time - c.lastTime >= 0) && (rec.time - c.lastTime <= kEngagementWindowTicks);
            if (withinWindow) touchedRoots.insert(root);
        }

        int target;
        if (touchedRoots.empty()) {
            clusters.emplace_back();
            parent.push_back((int)parent.size());
            target = (int)clusters.size() - 1;
        } else {
            auto rit = touchedRoots.begin();
            target = *rit;
            // Fold every other touched root's accumulated data into
            // target before repointing its parent -- a plain
            // parent[other]=target with no data fold would silently lose
            // whatever recordIdx/unitIds `other` had accumulated before
            // this record ever mentioned it.
            for (++rit; rit != touchedRoots.end(); ++rit) {
                int other = *rit;
                if (other == target) continue;
                EngagementBuild &oc = clusters[other];
                EngagementBuild &tc = clusters[target];
                for (size_t ridx : oc.recordIdx) tc.recordIdx.push_back(ridx);
                for (int32_t uid2 : oc.unitIds) tc.unitIds.insert(uid2);
                if (oc.minId < tc.minId) tc.minId = oc.minId;
                if (oc.maxId > tc.maxId) tc.maxId = oc.maxId;
                if (oc.firstYear >= 0 && (tc.firstYear < 0 || isEarlier(oc.firstYear, oc.firstTime, tc.firstYear, tc.firstTime))) {
                    tc.firstYear = oc.firstYear; tc.firstTime = oc.firstTime;
                }
                if (oc.lastYear > tc.lastYear || (oc.lastYear == tc.lastYear && oc.lastTime > tc.lastTime)) {
                    tc.lastYear = oc.lastYear; tc.lastTime = oc.lastTime;
                }
                oc.recordIdx.clear();
                oc.unitIds.clear();
                parent[other] = target;
            }
        }

        EngagementBuild &tc = clusters[target];
        tc.recordIdx.push_back(i);
        if (tc.firstYear < 0) { tc.firstYear = rec.year; tc.firstTime = rec.time; }
        tc.lastYear = rec.year;
        tc.lastTime = rec.time;
        if (rec.id < tc.minId) tc.minId = rec.id;
        if (rec.id > tc.maxId) tc.maxId = rec.id;
        for (int32_t uid : rec.unitIds) {
            tc.unitIds.insert(uid);
            unitLastCluster[uid] = target;
        }
    }

    std::vector<EngagementBuild> live;
    for (auto &c : clusters) {
        if (!c.recordIdx.empty()) live.push_back(std::move(c));
    }
    return live;
}

// WoundSeverity / classifyWoundSeverity -- the structured severity ladder
// research 2.3's table documents, scanning every body-part layer a wound
// touches (unit_wound.parts) for the highest-priority flag present:
// severed_part is on unit_wound.flags directly (top-level); artery/
// fracture/guts_spilled live per-layer on unit_wound_layerst.flags1.
// "bruise" is the truthful floor when none of the above fired -- a
// unit_wound always has at least one layer once created, so this is never
// a guess.
enum class WoundSeverity { Severed, Artery, Fracture, GutsSpilled, Bruise };

static const char *woundSeverityName(WoundSeverity s) {
    switch (s) {
    case WoundSeverity::Severed: return "severed_part";
    case WoundSeverity::Artery: return "artery";
    case WoundSeverity::Fracture: return "fracture";
    case WoundSeverity::GutsSpilled: return "guts_spilled";
    default: return "bruise";
    }
}

static WoundSeverity classifyWoundSeverity(df::unit_wound *w) {
    if (!w) return WoundSeverity::Bruise;
    if (w->flags.bits.severed_part) return WoundSeverity::Severed;
    bool artery = false, fracture = false, guts = false;
    for (auto *layer : w->parts) {
        if (!layer) continue;
        if (layer->flags1.bits.major_artery || layer->flags1.bits.artery) artery = true;
        if (layer->flags1.bits.broken || layer->flags1.bits.compound_fracture) fracture = true;
        if (layer->flags1.bits.guts_spilled) guts = true;
    }
    if (artery) return WoundSeverity::Artery;
    if (fracture) return WoundSeverity::Fracture;
    if (guts) return WoundSeverity::GutsSpilled;
    return WoundSeverity::Bruise;
}

// unitIsFortSide -- this project's own binary "sides" approximation
// (research 3.3: "sides by name (unit-intersection + 500-tick window
// grouping -- the PROVEN fallback)") until the conflict-activity join
// lands: every fight-involved unit that is one of this fort's own citizens
// vs everyone else (hostiles, wildlife, visitors, sparring partners
// already excluded upstream). A real multi-faction split awaits the same
// probe #1 as the engagement join itself.
static bool unitIsFortSide(int32_t unitId) {
    df::unit *u = df::unit::find(unitId);
    return u && Units::isCitizen(u);
}

// SideAgg -- one side's aggregated momentum facts, computed once per
// engagement before either side is serialized so hits_landed (attributed
// via the OTHER side's wound attacker_unit_id) can be filled in after both
// sides are walked.
struct SideAgg {
    std::vector<int32_t> unitIds;
    std::unordered_map<std::string, int64_t> woundTally;
    int64_t knockedOut = 0;
    int64_t bleeding = 0;
    std::vector<int32_t> casualtyUnitIds;
    std::vector<std::string> casualtyDeathCause;
    std::vector<std::string> casualtyKiller; // "" when unresolved
    std::vector<std::pair<int32_t, int64_t>> hitsLanded; // (attacker unit id, count) -- filled in after both sides
};

// aggregateSide walks one side's units, tallying new-wound severity (via
// each unit's wound watermark, when tracked -- research 2.3's "new wounds
// since watermark"), attacker attribution (unit_wound.attacker_unit_id,
// bucketed into hitsByAttacker for the caller to redistribute once BOTH
// sides are known), KO (counters.unconscious), bleeding
// (blood_count/blood_max), and casualties (flags2.bits.killed ->
// counters.death_id -> df::incident, the same incident lookup
// handleDwarfDetail's own death-time reporting uses).
static SideAgg aggregateSide(const std::vector<int32_t> &unitIds,
                              const std::unordered_map<int32_t, int32_t> &watermarks,
                              std::unordered_map<int32_t, int64_t> &hitsByAttacker) {
    SideAgg agg;
    agg.unitIds = unitIds;
    for (int32_t uid : unitIds) {
        df::unit *u = df::unit::find(uid);
        if (!u) continue;

        auto wmIt = watermarks.find(uid);
        if (wmIt != watermarks.end()) {
            for (auto *w : u->body.wounds) {
                if (!w || w->id < wmIt->second) continue;
                agg.woundTally[woundSeverityName(classifyWoundSeverity(w))]++;
                if (w->attacker_unit_id >= 0) hitsByAttacker[w->attacker_unit_id]++;
            }
        }

        if (u->counters.unconscious > 0) agg.knockedOut++;
        if (u->body.blood_max > 0 &&
            (int64_t)u->body.blood_count * 100 < (int64_t)u->body.blood_max * kBleedingThresholdPercent) {
            agg.bleeding++;
        }

        if (u->flags2.bits.killed) {
            df::incident *inc = df::incident::find(u->counters.death_id);
            std::string deathCause = (u->counters.death_cause != df::death_type::NONE)
                ? ENUM_KEY_STR(death_type, u->counters.death_cause) : "unknown";
            std::string killerName;
            if (inc && inc->criminal >= 0) {
                df::unit *killer = df::unit::find(inc->criminal);
                if (killer) killerName = Units::getReadableName(killer);
            }
            agg.casualtyUnitIds.push_back(uid);
            agg.casualtyDeathCause.push_back(deathCause);
            agg.casualtyKiller.push_back(killerName);
        }
    }
    return agg;
}

// writeSideJson serializes one SideAgg -- see handleCombatSummary for the
// full engagement shape this is one element of.
static void writeSideJson(std::ostringstream &os, const char *label, const SideAgg &agg) {
    os << "{\"label\":" << jsonStrNarrative(label) << ",\"units\":[";
    for (size_t i = 0; i < agg.unitIds.size(); i++) {
        if (i) os << ",";
        df::unit *u = df::unit::find(agg.unitIds[i]);
        os << "{\"id\":" << jsonIntNarrative(agg.unitIds[i])
           << ",\"name\":" << jsonStrNarrative(u ? Units::getReadableName(u) : "")
           << "}";
    }
    os << "],\"new_wounds\":{";
    bool firstW = true;
    for (auto &kv : agg.woundTally) {
        if (!firstW) os << ",";
        firstW = false;
        os << jsonStrNarrative(kv.first) << ":" << jsonIntNarrative(kv.second);
    }
    os << "},\"hits_landed\":[";
    for (size_t i = 0; i < agg.hitsLanded.size(); i++) {
        if (i) os << ",";
        df::unit *attacker = df::unit::find(agg.hitsLanded[i].first);
        os << "{\"attacker_id\":" << jsonIntNarrative(agg.hitsLanded[i].first)
           << ",\"attacker_name\":" << jsonStrNarrative(attacker ? Units::getReadableName(attacker) : "")
           << ",\"count\":" << jsonIntNarrative(agg.hitsLanded[i].second)
           << "}";
    }
    os << "],\"knocked_out\":" << jsonIntNarrative(agg.knockedOut)
       << ",\"bleeding\":" << jsonIntNarrative(agg.bleeding)
       << ",\"casualties\":[";
    for (size_t i = 0; i < agg.casualtyUnitIds.size(); i++) {
        if (i) os << ",";
        df::unit *u = df::unit::find(agg.casualtyUnitIds[i]);
        os << "{\"id\":" << jsonIntNarrative(agg.casualtyUnitIds[i])
           << ",\"name\":" << jsonStrNarrative(u ? Units::getReadableName(u) : "")
           << ",\"death_cause\":" << jsonStrNarrative(agg.casualtyDeathCause[i])
           << ",\"killer\":" << (agg.casualtyKiller[i].empty() ? "null" : jsonStrNarrative(agg.casualtyKiller[i]))
           << "}";
    }
    os << "]}";
}

// tellingLineTier / selectTellingLines -- research 3.3's ranking ("severity
// of same-window wound > status transitions incl. BOTH
// COMBAT_STRIKE_DETAILS and _2 [EventManager misses _2, a documented
// defect this project does not copy] > rarity within the engagement's own
// type distribution"). Without the activity_id join (probe #1) there is no
// cheap per-report link to a specific wound's severity, so the first two
// priorities are folded into one combined tier here: STRIKE_DETAILS/_2
// lines are the ones that actually narrate a landed hit (and therefore
// correlate with whatever wound severity generated them); KO/status lines
// are the next tier; everything else is background color. Rarity within
// THIS engagement's own report-type distribution breaks ties inside a
// tier, then recency breaks any remaining tie.
static int tellingLineTier(int16_t typeRaw) {
    auto t = (df::announcement_type)typeRaw;
    if (t == df::announcement_type::COMBAT_STRIKE_DETAILS || t == df::announcement_type::COMBAT_STRIKE_DETAILS_2)
        return 2;
    if (t == df::announcement_type::COMBAT_EVENT_KNOCKED_OUT || t == df::announcement_type::COMBAT_WRESTLE_STRANGLE_KO)
        return 1;
    return 0;
}

static std::vector<std::string> selectTellingLines(const std::vector<CombatReportRecord> &snapshot,
                                                     const EngagementBuild &cluster) {
    std::unordered_map<int16_t, int> typeCounts;
    for (size_t idx : cluster.recordIdx) typeCounts[snapshot[idx].typeRaw]++;

    std::vector<size_t> ordered(cluster.recordIdx.begin(), cluster.recordIdx.end());
    std::sort(ordered.begin(), ordered.end(), [&](size_t a, size_t b) {
        const CombatReportRecord &ra = snapshot[a];
        const CombatReportRecord &rb = snapshot[b];
        int tierA = tellingLineTier(ra.typeRaw), tierB = tellingLineTier(rb.typeRaw);
        if (tierA != tierB) return tierA > tierB;
        int countA = typeCounts[ra.typeRaw], countB = typeCounts[rb.typeRaw];
        if (countA != countB) return countA < countB; // rarer wins
        return ra.id > rb.id; // most recent wins the final tie
    });

    std::vector<std::string> out;
    for (size_t i = 0; i < ordered.size() && out.size() < kTellingLinesPerEngagement; i++) {
        out.push_back(snapshot[ordered[i]].text);
    }
    return out;
}

// handleCombatSummary answers combat_summary {} (research 3.3): per
// engagement (cap kCombatSummaryMaxEngagements, most recent first), sides
// by name, computed momentum with zero text parsing, and 2-3 telling lines
// quoted verbatim. Truthful empty state when the buffer has nothing since
// the cursor (no fight has happened yet, or none since a recent reconnect
// -- the cursor itself never resets, see ingest_combat_reports' doc
// comment on why this file has no reset-on-reconnect unlike
// announcements.cpp).
std::string handleCombatSummary(const std::string &args, uint8_t &status) {
    (void)args; // no parameters -- research 3.3: `combat_summary {}`
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world is null");
    }

    std::vector<CombatReportRecord> snapshot;
    std::unordered_map<int32_t, int32_t> watermarks;
    {
        std::lock_guard<std::mutex> lock(g_combat_mutex);
        snapshot.assign(g_combat_reports.begin(), g_combat_reports.end());
        watermarks = g_wound_watermarks;
    }

    status = QUERY_STATUS_SUCCESS;
    if (snapshot.empty()) {
        return "{\"engagements\":[],\"total_tracked\":0,\"note\":\"no combat reports since cursor\"}";
    }

    std::vector<EngagementBuild> clusters = buildEngagements(snapshot);
    std::sort(clusters.begin(), clusters.end(), [](const EngagementBuild &a, const EngagementBuild &b) {
        return a.maxId > b.maxId;
    });
    size_t totalClusters = clusters.size();
    bool clamped = clusters.size() > kCombatSummaryMaxEngagements;
    if (clamped) clusters.resize(kCombatSummaryMaxEngagements);

    std::ostringstream os;
    os << "{\"engagements\":[";
    for (size_t ci = 0; ci < clusters.size(); ci++) {
        if (ci) os << ",";
        const EngagementBuild &cluster = clusters[ci];

        std::vector<int32_t> fortUnits, otherUnits;
        for (int32_t uid : cluster.unitIds) {
            if (unitIsFortSide(uid)) fortUnits.push_back(uid);
            else otherUnits.push_back(uid);
        }

        std::unordered_map<int32_t, int64_t> hitsByAttacker;
        SideAgg fortAgg = aggregateSide(fortUnits, watermarks, hitsByAttacker);
        SideAgg otherAgg = aggregateSide(otherUnits, watermarks, hitsByAttacker);

        std::unordered_set<int32_t> fortSet(fortUnits.begin(), fortUnits.end());
        std::unordered_set<int32_t> otherSet(otherUnits.begin(), otherUnits.end());
        for (auto &kv : hitsByAttacker) {
            if (fortSet.count(kv.first)) fortAgg.hitsLanded.push_back(kv);
            else if (otherSet.count(kv.first)) otherAgg.hitsLanded.push_back(kv);
            // else: attacker outside both known sides (e.g. aged out of the
            // roster already) -- omitted rather than fabricated.
        }

        std::vector<std::string> tellingLines = selectTellingLines(snapshot, cluster);

        os << "{\"engagement_id\":" << jsonIntNarrative(cluster.minId)
           << ",\"first_report_id\":" << jsonIntNarrative(cluster.minId)
           << ",\"last_report_id\":" << jsonIntNarrative(cluster.maxId)
           << ",\"start_year\":" << jsonIntNarrative(cluster.firstYear)
           << ",\"start_time\":" << jsonIntNarrative(cluster.firstTime)
           << ",\"end_year\":" << jsonIntNarrative(cluster.lastYear)
           << ",\"end_time\":" << jsonIntNarrative(cluster.lastTime)
           << ",\"sides\":[";
        writeSideJson(os, "fort", fortAgg);
        os << ",";
        writeSideJson(os, "other", otherAgg);
        os << "],\"telling_lines\":[";
        for (size_t i = 0; i < tellingLines.size(); i++) {
            if (i) os << ",";
            os << jsonStrNarrative(tellingLines[i]);
        }
        os << "]}";
    }
    os << "],\"total_tracked\":" << jsonIntNarrative((int64_t)totalClusters)
       << ",\"momentum_note\":\"new_wounds/hits_landed reflect activity since each unit was first observed "
          "fighting (a wound watermark, not a fixed engagement start) -- the very first detection pass for a "
          "brand-new fighter can undercount its opening exchange\"";
    if (clamped) {
        os << ",\"clamp_note\":\"" << (totalClusters - kCombatSummaryMaxEngagements)
           << " older engagement(s) not shown\"";
    }
    os << "}";
    return os.str();
}

// handleCombatLog answers combat_log {engagement?|unit?, last?} (research
// 3.3): a windowed raw slice of the rolling buffer, continuation already
// stitched at ingest time, capped at kCombatLogMaxLines with a truthful
// clamp note. Explicit-request-only by design (never called from
// combat_summary) -- a drill-down, not part of the default narrative.
std::string handleCombatLog(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world is null");
    }

    int64_t engagementId = jsonGetIntNarrative(args, "engagement", -1);
    int64_t unitFilter = jsonGetIntNarrative(args, "unit", -1);
    int64_t lastN = jsonGetIntNarrative(args, "last", (int64_t)kCombatLogMaxLines);
    if (lastN <= 0 || lastN > (int64_t)kCombatLogMaxLines) lastN = (int64_t)kCombatLogMaxLines;

    std::vector<CombatReportRecord> snapshot;
    {
        std::lock_guard<std::mutex> lock(g_combat_mutex);
        snapshot.assign(g_combat_reports.begin(), g_combat_reports.end());
    }

    status = QUERY_STATUS_SUCCESS;
    if (snapshot.empty()) {
        return "{\"lines\":[],\"note\":\"no combat reports since cursor\"}";
    }

    std::vector<size_t> candidateIdx;
    if (engagementId >= 0) {
        std::vector<EngagementBuild> clusters = buildEngagements(snapshot);
        const EngagementBuild *found = nullptr;
        for (auto &c : clusters) {
            if (c.minId == (int32_t)engagementId) { found = &c; break; }
        }
        if (!found) {
            return "{\"lines\":[],\"note\":\"no engagement with that id in the current buffer (it may have scrolled out)\"}";
        }
        candidateIdx.assign(found->recordIdx.begin(), found->recordIdx.end());
        std::sort(candidateIdx.begin(), candidateIdx.end());
    } else if (unitFilter >= 0) {
        for (size_t i = 0; i < snapshot.size(); i++) {
            const auto &ids = snapshot[i].unitIds;
            if (std::find(ids.begin(), ids.end(), (int32_t)unitFilter) != ids.end())
                candidateIdx.push_back(i);
        }
    } else {
        for (size_t i = 0; i < snapshot.size(); i++) candidateIdx.push_back(i);
    }

    if (candidateIdx.empty()) {
        return "{\"lines\":[],\"note\":\"no matching combat reports\"}";
    }

    bool truncated = false;
    size_t startAt = 0;
    if (candidateIdx.size() > (size_t)lastN) {
        startAt = candidateIdx.size() - (size_t)lastN;
        truncated = true;
    }

    std::ostringstream os;
    os << "{\"lines\":[";
    for (size_t i = startAt; i < candidateIdx.size(); i++) {
        if (i > startAt) os << ",";
        const CombatReportRecord &rec = snapshot[candidateIdx[i]];
        os << "{\"id\":" << jsonIntNarrative(rec.id)
           << ",\"year\":" << jsonIntNarrative(rec.year)
           << ",\"time\":" << jsonIntNarrative(rec.time)
           << ",\"text\":" << jsonStrNarrative(rec.text)
           << "}";
    }
    os << "]";
    if (truncated) {
        os << ",\"clamp_note\":\"" << (candidateIdx.size() - (size_t)lastN) << " earlier line(s) omitted\"";
    }
    os << "}";
    return os.str();
}

// ---------------------------------------------------------------------------
// story_pulse -- fort-wide emotional weather (wave 013-B / Lane 3, research
// 2.2 + 3.2 mode=pulse).
//
// Cursor: a plugin-global (year, year_tick) watermark, mirroring
// announcements.cpp's g_last_sent_report_id but over personality_moodst's
// own last-used stamps instead of df::report ids -- the repo's own
// handleDwarfDetail already recency-sorts on exactly these two fields
// (queries.cpp:1519-1522). Re-felt emotions bump their stamp IN PLACE on the
// SAME object (the same same-object-updated problem the repeat_count tail
// window solved for announcements) -- here the stamp bump IS the news (a
// re-felt grudge is a real event), so the cursor is deliberately paired with
// a remembered-set (below) rather than relying on the cursor alone.
// ---------------------------------------------------------------------------

static std::mutex g_pulse_mutex;
static int32_t g_pulse_cursor_year = -1;  // -1 = never set (cold start OR post-reset)
static int32_t g_pulse_cursor_tick = -1;

// PulseTupleKey / g_pulse_remembered -- within-window dedup by
// (unit,emotion,thought,subthought), the remembered-set style research 2.2
// asks for (the wildlife tripwire's g_known_dangerous_wildlife_ids
// precedent, queries.cpp:1902-1955): maps a felt-emotion tuple to the
// (year,year_tick) stamp last reported for it. A candidate whose stamp has
// advanced past what's remembered is real news (a re-felt emotion) and
// passes; a candidate at or behind its remembered stamp does not. Pruned
// each call down to entries still ahead of the (now-advanced) cursor --
// "cleared below cursor" -- since anything at or behind the cursor can
// never pass the cursor filter again regardless of what this map
// remembers, so remembering it further would only leak memory for the
// fort's entire life.
struct PulseTupleKey {
    int32_t unitId;
    int16_t emotion;
    int16_t thought;
    int32_t subthought;
    bool operator==(const PulseTupleKey &o) const {
        return unitId == o.unitId && emotion == o.emotion && thought == o.thought && subthought == o.subthought;
    }
};
struct PulseTupleKeyHash {
    size_t operator()(const PulseTupleKey &k) const {
        size_t h = std::hash<int32_t>()(k.unitId);
        h = h * 31 + std::hash<int16_t>()(k.emotion);
        h = h * 31 + std::hash<int16_t>()(k.thought);
        h = h * 31 + std::hash<int32_t>()(k.subthought);
        return h;
    }
};
static std::unordered_map<PulseTupleKey, std::pair<int32_t, int32_t>, PulseTupleKeyHash> g_pulse_remembered;

// reset_story_pulse_cursor -- reconnect/plugin-unload teardown, called from
// close_socket_and_reset (df_ai_protocol.cpp) alongside
// reset_announcement_cursor/reset_wildlife_tripwire_tracking. UNLIKE
// ingest_combat_reports' cursor (narrative.cpp's combat section, which
// deliberately does NOT reset -- see its own doc comment), this one DOES
// reset: emotions aren't an append-only id-cursored log the way reports
// are, so there is no cheap way to answer "what has this peer already
// seen" after a reconnect other than starting over -- handleStoryPulse's
// own bounded reconnect re-window (below) is what keeps that restart from
// replaying the fort's entire emotional history.
void reset_story_pulse_cursor() {
    std::lock_guard<std::mutex> lock(g_pulse_mutex);
    g_pulse_cursor_year = -1;
    g_pulse_cursor_tick = -1;
    g_pulse_remembered.clear();
}

// kStoryPulseMaxLines -- research 3.2's "cap 12-15 lines"; this file uses
// the top of that range as the single hard cap (the `max` query param can
// only ever narrow it, never widen it).
static const size_t kStoryPulseMaxLines = 15;

// kStoryPulseMinStrength -- research 2.2 names |strength| as a ranking
// signal but, unlike the facet/value/need tier tables (which DFHack ships
// verbatim), does not give a magnitude threshold for emotion strength.
// This project picks a floor the same way kBleedingThresholdPercent's own
// doc comment does: a level a reader would actually call "worth
// mentioning" rather than routine background noise -- a judgment call,
// not a transcribed table. Only gates entries that don't already qualify
// via a facet/value-change flag or a MadeFriend/FormedGrudge thought (both
// bypass this floor entirely, per their own higher-priority tiers below).
static const int32_t kStoryPulseMinStrength = 50;

// kStoryPulseReconnectRewindTicks -- the "bounded re-window" research 2.2 /
// this wave's brief calls for on a reset cursor, instead of either
// replaying the fort's ENTIRE emotional history since embark (unbounded,
// mostly ancient) or silently showing nothing until fresh emotions accrue
// (a real gap the reader would rather be told about). Deliberately a
// same-year-only bound -- a reset landing in the first ~10 days of a new
// year sees a truncated window rather than reaching into the prior year --
// the identical simplification this file's own kEngagementWindowTicks
// already accepts, for the same reason: DFHack exposes no verified
// ticks-per-year constant in this checkout to compute an exact calendar
// lookback against.
static const int32_t kStoryPulseReconnectRewindTicks = 12000;

// probeMadeFriendGrudgeOtherParty -- speculative resolver for MadeFriend/
// FormedGrudge's subthought (open live probe, research §5.9: nothing in
// this checkout documents what subthought holds for these two thought
// types specifically). Resolve-and-verify only, in the same spirit as
// resolve_thought's Death/Prayer/Defeated/Murdered class (narrative.h/.cpp)
// but stricter still: ALSO requires the resolved figure to be one of the
// fort's OWN citizens, not merely a nameable historical_figure, before
// ever naming "both ends" of a forming friendship/grudge -- an unrelated
// historical_figure id happening to match subthought's raw int by chance
// is exactly the kind of unverified-probe false positive this project
// refuses to surface as fact. Returns "" (never a guess) otherwise.
static std::string probeMadeFriendGrudgeOtherParty(int32_t subthought, const CitizenIndex &idx) {
    auto it = idx.hfidToUnit.find(subthought);
    if (it == idx.hfidToUnit.end()) return "";
    return Units::getReadableName(it->second);
}

// PulseCandidate -- one fresh (unit, emotion) reading, plus the salience
// tier research 2.2 ranks by: 3 = facet_change/value_change flagged
// (personality-altering moments the simulation itself flags), 2 =
// MadeFriend/FormedGrudge (an edge forming -- cross-checked against §2.4's
// social graph), 1 = |strength| >= kStoryPulseMinStrength, 0 = everything
// else (background noise -- eligible to be COUNTED as a fresh candidate,
// never eligible to be SHOWN; see handleStoryPulse's own doc comment on
// why a calm fort should read as calm rather than padded with trivia).
struct PulseCandidate {
    int32_t unitId = -1;
    std::string unitName;
    df::emotion_type emotion = df::emotion_type::ANYTHING;
    int32_t strength = 0;
    int8_t divider = 0;
    df::unit_thought_type thought = df::unit_thought_type::None;
    int32_t subthought = 0;
    int32_t year = 0, tick = 0;
    int tier = 0;
    std::string otherPartyName; // MadeFriend/FormedGrudge only, resolve-and-verify
};

// pulseMoreSalient -- the composite ranking research 2.2 specifies: tier
// first (facet/value-change, then MadeFriend/FormedGrudge, then plain
// strength), then |strength| descending within a tier, then a negative-
// emotion (divider > 0 -- df.d_basics.xml:687's own documented sign,
// already the convention dwarf_portrait's Go renderer relies on) tie-break
// over an equally-strong positive one, then recency. Negative-weighting is
// deliberately a TIE-BREAK only (never overriding an actual strength gap)
// -- research says "negative-weighted" but gives no numeric weight to
// apply, and letting a trivial negative emotion outrank a much stronger
// positive one would be the wrong reading of "weighted".
static bool pulseMoreSalient(const PulseCandidate &a, const PulseCandidate &b) {
    if (a.tier != b.tier) return a.tier > b.tier;
    int sa = a.strength < 0 ? -a.strength : a.strength;
    int sb = b.strength < 0 ? -b.strength : b.strength;
    if (sa != sb) return sa > sb;
    bool negA = a.divider > 0, negB = b.divider > 0;
    if (negA != negB) return negA;
    if (a.year != b.year) return a.year > b.year;
    return a.tick > b.tick;
}

// handleStoryPulse answers story_pulse {max?} (research 3.2): fort-wide
// emotions since the plugin-side cursor, salience-ranked, capped, with a
// clamp_note that keeps two genuinely different omission causes separate
// rather than folding them into one "below threshold" count: tier-0
// candidates that never cleared the salience bar at all, versus tier>=1
// (genuinely salient) candidates that only missed the cut because `max`
// capped the display list. Conflating the two would misreport real, salient
// news as noise -- see the two counters' own comments just above the
// clamp_note assembly below. Degrades honestly to an empty entries list
// (never an error) when nothing has been felt since the cursor.
std::string handleStoryPulse(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world is null");
    }
    int64_t maxParam = jsonGetIntNarrative(args, "max", (int64_t)kStoryPulseMaxLines);
    if (maxParam <= 0 || maxParam > (int64_t)kStoryPulseMaxLines) maxParam = (int64_t)kStoryPulseMaxLines;

    CitizenIndex idx = build_citizen_index();

    std::lock_guard<std::mutex> lock(g_pulse_mutex);
    bool wasReset = (g_pulse_cursor_year < 0);

    // Single pass over every citizen's emotions: collect the raw (unit,
    // moodst) pairs and, independently, the overall max (year,year_tick)
    // seen -- needed both to advance the real cursor below and, on a reset
    // cursor, to compute the bounded reconnect re-window floor (see
    // kStoryPulseReconnectRewindTicks's doc comment) without ever reading
    // DF's own current-tick globals (kept entirely off the reset path
    // itself, which runs from the socket thread with no CoreSuspender).
    struct RawEntry { df::unit *u; df::personality_moodst *e; };
    std::vector<RawEntry> allEntries;
    int32_t maxYear = -1, maxTick = -1;
    for (auto &kv : idx.hfidToUnit) {
        df::unit *u = kv.second;
        if (!u || !u->status.current_soul) continue;
        for (auto *e : u->status.current_soul->personality.emotions) {
            if (!e) continue;
            if (e->year > maxYear || (e->year == maxYear && e->year_tick > maxTick)) {
                maxYear = e->year;
                maxTick = e->year_tick;
            }
            allEntries.push_back(RawEntry{u, e});
        }
    }

    int32_t floorYear = g_pulse_cursor_year, floorTick = g_pulse_cursor_tick;
    bool rewindowed = false;
    if (wasReset && maxYear >= 0) {
        rewindowed = true;
        floorYear = maxYear;
        floorTick = maxTick - kStoryPulseReconnectRewindTicks;
        if (floorTick < 0) floorTick = 0; // same-year-only bound -- see the constant's own doc comment
    }

    std::vector<PulseCandidate> candidates;
    for (auto &re : allEntries) {
        df::personality_moodst *e = re.e;
        bool isNewer = (e->year > floorYear) || (e->year == floorYear && e->year_tick > floorTick);
        if (!isNewer) continue;

        PulseTupleKey key{re.u->id, (int16_t)e->type, (int16_t)e->thought, e->subthought};
        auto rememberedIt = g_pulse_remembered.find(key);
        if (rememberedIt != g_pulse_remembered.end()) {
            bool advanced = (e->year > rememberedIt->second.first) ||
                (e->year == rememberedIt->second.first && e->year_tick > rememberedIt->second.second);
            if (!advanced) continue; // identical replay of an already-reported stamp -- not news
        }

        PulseCandidate c;
        c.unitId = re.u->id;
        c.unitName = Units::getReadableName(re.u);
        c.emotion = e->type;
        c.strength = e->strength;
        c.divider = ENUM_ATTR(emotion_type, divider, e->type);
        c.thought = e->thought;
        c.subthought = e->subthought;
        c.year = e->year;
        c.tick = e->year_tick;

        bool flaggedChange = e->flags.bits.facet_change || e->flags.bits.value_change;
        bool madeFriendOrGrudge = (e->thought == df::unit_thought_type::MadeFriend ||
                                    e->thought == df::unit_thought_type::FormedGrudge);
        int absStrength = e->strength < 0 ? -e->strength : e->strength;
        if (flaggedChange) c.tier = 3;
        else if (madeFriendOrGrudge) c.tier = 2;
        else if (absStrength >= kStoryPulseMinStrength) c.tier = 1;
        else c.tier = 0;

        if (madeFriendOrGrudge) {
            c.otherPartyName = probeMadeFriendGrudgeOtherParty(e->subthought, idx);
        }

        g_pulse_remembered[key] = {e->year, e->year_tick};
        candidates.push_back(std::move(c));
    }

    // Advance the real cursor past EVERYTHING scanned this pass (shown or
    // not) -- a tier-0 (below-threshold) emotion is truthfully reported as
    // "below threshold" once and never revisited on a future call just
    // because it never won a salience contest (the clamp_note below is the
    // honest permanent record of that, not a promise to try again). A
    // tier>=1 candidate cut only by the display cap is a SEPARATE, later
    // concern -- it stays eligible in `eligible` and is counted by its own
    // distinct clause in the clamp_note, never merged into this one.
    if (maxYear >= 0) {
        g_pulse_cursor_year = maxYear;
        g_pulse_cursor_tick = maxTick;
    }

    // Prune remembered-set entries now covered by the cursor alone --
    // "cleared below cursor" (research 2.2 / the wildlife tripwire
    // precedent's own bounded-tail reasoning).
    for (auto it = g_pulse_remembered.begin(); it != g_pulse_remembered.end();) {
        bool coveredByCursor = (it->second.first < g_pulse_cursor_year) ||
            (it->second.first == g_pulse_cursor_year && it->second.second <= g_pulse_cursor_tick);
        if (coveredByCursor) it = g_pulse_remembered.erase(it);
        else ++it;
    }

    // Only tier >= 1 candidates are eligible to be SHOWN -- tier 0 never
    // fills a remaining cap slot even on an otherwise-quiet pulse (see
    // PulseCandidate's own doc comment).
    std::vector<PulseCandidate> eligible;
    for (auto &c : candidates) {
        if (c.tier >= 1) eligible.push_back(c);
    }
    std::sort(eligible.begin(), eligible.end(), pulseMoreSalient);

    size_t totalCandidates = candidates.size();
    size_t showCount = std::min((size_t)maxParam, eligible.size());

    std::ostringstream os;
    os << "{\"entries\":[";
    for (size_t i = 0; i < showCount; i++) {
        if (i) os << ",";
        const PulseCandidate &c = eligible[i];
        ThoughtDetail td = resolve_thought(c.thought, c.subthought);
        const char *tierName = (c.tier == 3) ? "facet_or_value_change"
                              : (c.tier == 2) ? "made_friend_or_grudge"
                              : "strength";
        os << "{\"unit_id\":" << jsonIntNarrative(c.unitId)
           << ",\"unit_name\":" << jsonStrNarrative(c.unitName)
           << ",\"emotion\":" << jsonStrNarrative(c.emotion == df::emotion_type::ANYTHING ? "NONE" : ENUM_KEY_STR(emotion_type, c.emotion))
           << ",\"strength\":" << jsonIntNarrative(c.strength)
           << ",\"divider\":" << jsonIntNarrative(c.divider)
           << ",\"thought\":" << jsonStrNarrative(c.thought == df::unit_thought_type::None ? "None" : ENUM_KEY_STR(unit_thought_type, c.thought))
           << ",\"thought_caption\":" << jsonStrNarrative(td.caption)
           << ",\"subthought\":" << jsonIntNarrative(c.subthought)
           << ",\"subthought_resolved\":" << (td.subthoughtResolved ? "true" : "false")
           << ",\"subthought_text\":" << jsonStrNarrative(td.subthoughtText)
           << ",\"salience\":" << jsonStrNarrative(tierName)
           << ",\"other_party\":";
        if (c.otherPartyName.empty()) os << "null";
        else os << "{\"name\":" << jsonStrNarrative(c.otherPartyName) << "}";
        os << ",\"year\":" << jsonIntNarrative(c.year)
           << ",\"year_tick\":" << jsonIntNarrative(c.tick)
           << "}";
    }
    os << "],\"total_candidates\":" << jsonIntNarrative((int64_t)totalCandidates)
       << ",\"shown\":" << jsonIntNarrative((int64_t)showCount);
    // Two disjoint causes for "not shown", reported truthfully as two
    // separate clauses rather than one merged count: belowThreshold is
    // tier-0 candidates that never cleared the salience bar (genuinely
    // background noise); cappedByLimit is tier>=1 (genuinely salient)
    // candidates that cleared the bar but were cut solely by `max`. Merging
    // them would let real news (cappedByLimit) misreport as noise
    // (belowThreshold's wording) whenever a window is unusually busy.
    size_t belowThreshold = totalCandidates - eligible.size();
    size_t cappedByLimit = eligible.size() - showCount;
    if (belowThreshold > 0 || cappedByLimit > 0) {
        os << ",\"clamp_note\":\"";
        bool wrote = false;
        if (belowThreshold > 0) {
            os << belowThreshold << " below threshold";
            wrote = true;
        }
        if (cappedByLimit > 0) {
            if (wrote) os << "; ";
            os << cappedByLimit << " more salient not shown (raise max)";
        }
        os << "\"";
    }
    if (rewindowed) {
        os << ",\"rewindow_note\":\"cursor was unset (first call, or a reconnect) -- showing only the last "
           << kStoryPulseReconnectRewindTicks << " ticks within the current year rather than the fort's full emotional history\"";
    }
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// ---------------------------------------------------------------------------
// social_graph -- the fort's social graph (wave 013-B / Lane 3, research
// 2.4 + 3.2 mode=social).
// ---------------------------------------------------------------------------

// kSocialGraphMaxEdges -- research 3.2's "cap ~40 edges".
static const size_t kSocialGraphMaxEdges = 40;

// kSocialGraphFriendLoveFloor / kSocialGraphGrudgeLoveCeiling -- DF's own
// documented core.love banding (df.history_figure.xml:486, the exact
// string dwarf_portrait's Go renderer already quotes verbatim): "LE 49:
// Acquaintance, LE 74: Friend" and "LE -50: Disliked". >=50 is therefore
// the first value that clears Acquaintance into a real Friend rating;
// <=-50 the first that clears Acquaintance down into Disliked.
// Acquaintance itself (-49..49, rank-less) is excluded from the graph
// entirely below -- "everyone who's ever met" is the citizen roster, not a
// social graph.
static const int32_t kSocialGraphFriendLoveFloor = 50;
static const int32_t kSocialGraphGrudgeLoveCeiling = -50;

// SocialRankBucket / socialGraphRankBucket -- a curated split of
// vague_relationship_type's flavor categories into friend/grudge/neither.
// Research 2.4 documents that this enum "adds flavor categories" but does
// not itself partition them into friend vs. grudge; several members
// (spouse/mother/father/master/apprentice/companion/lover/former_lover/
// worshipped_deity/neighbor/shared_entity/lieutenant) already have -- or
// don't need -- a home in one of this file's OTHER edge kinds (family/
// worship) and would be redundant or out-of-scope noise as a friend/grudge
// line here, so they fall through to Neither (still eligible via the
// love-band floor above if the love score itself clears it).
enum class SocialRankBucket { Friend, Grudge, Neither };
static SocialRankBucket socialGraphRankBucket(df::vague_relationship_type rank) {
    switch (rank) {
    case df::vague_relationship_type::childhood_friend:
    case df::vague_relationship_type::war_buddy:
    case df::vague_relationship_type::scholar_buddy:
    case df::vague_relationship_type::artistic_buddy:
    case df::vague_relationship_type::athlete_buddy:
        return SocialRankBucket::Friend;
    case df::vague_relationship_type::grudge:
    case df::vague_relationship_type::jealous_obsession:
    case df::vague_relationship_type::jealous_relationship_grudge:
    case df::vague_relationship_type::religious_persecution_grudge:
    case df::vague_relationship_type::persecution_grudge:
    case df::vague_relationship_type::supernatural_grudge:
    case df::vague_relationship_type::athletic_rival:
    case df::vague_relationship_type::business_rival:
        return SocialRankBucket::Grudge;
    default:
        return SocialRankBucket::Neither;
    }
}

// SocialEdge -- one edge in the graph. kind is one of "family"/"worship"/
// "friend"/"grudge" (the query's own kind-filter vocabulary, research
// 3.2); relation is a free-form sub-type label ("spouse", "deity",
// "war_buddy", ""). love/meet_count (tier 3, hf_visual) and link_strength
// (tier 2, histfig_links -- deity devotion, or the master/apprentice/
// lover/companion bond) are each independently optional since no single
// tier produces all three.
struct SocialEdge {
    int32_t aId = -1, bId = -1;
    std::string aName, bName;
    std::string kind;
    std::string relation;
    bool hasLove = false; int32_t love = 0;
    bool hasMeetCount = false; int32_t meetCount = 0;
    bool hasLinkStrength = false; int32_t linkStrength = 0;
};

// SocialEdgeKey / addSocialEdge -- "Edge dedup A<B" (research 2.4/3.2),
// literally: an unordered (min id, max id, kind) triple. Two facts about
// the SAME pair under the SAME kind (e.g. a master/apprentice bond
// discoverable from BOTH ends -- the apprentice's own MASTER link and the
// master's own APPRENTICE link are the same relationship described from
// opposite sides) collapse to whichever is discovered first during the
// citizen scan below. This is a documented simplification: the "relation"
// label of whichever side won the race may read as if that side's
// perspective were canonical when it's arbitrary -- accepted the same way
// this file already accepts kEngagementWindowTicks' same-year edge case,
// because research asks for exactly this A<B rule and nothing finer.
// Different KINDS for the same pair (e.g. "family":"spouse" alongside
// "friend":"Close Friend" for a couple who are also each other's best
// friend) are NOT deduped against each other -- both are genuine,
// complementary facts about the pair.
struct SocialEdgeKey {
    int32_t lo, hi;
    std::string kind;
    bool operator==(const SocialEdgeKey &o) const { return lo == o.lo && hi == o.hi && kind == o.kind; }
};
struct SocialEdgeKeyHash {
    size_t operator()(const SocialEdgeKey &k) const {
        size_t h = std::hash<int32_t>()(k.lo);
        h = h * 31 + std::hash<int32_t>()(k.hi);
        h = h * 31 + std::hash<std::string>()(k.kind);
        return h;
    }
};
static void addSocialEdge(std::vector<SocialEdge> &edges,
                           std::unordered_set<SocialEdgeKey, SocialEdgeKeyHash> &seen,
                           SocialEdge edge) {
    SocialEdgeKey key{std::min(edge.aId, edge.bId), std::max(edge.aId, edge.bId), edge.kind};
    if (!seen.insert(key).second) return;
    edges.push_back(std::move(edge));
}

// handleSocialGraph answers social_graph {unit?, kind?} (research 2.4 +
// 3.2): the three-tier edge walk -- unit.relationship_ids' four simple
// slots (tier 1), histfig_links' deity/lover/master/apprentice/companion
// (tier 2; mother/father/spouse/child are tier 1's job already and would
// be redundant here), and hf_visual's friend/grudge banding (tier 3, the
// ONLY friend/grudge source -- histfig_links has no such link type at all,
// research 2.4's decisive negative). Citizens-only both ends throughout,
// EXCEPT worship (a deity is never a citizen, by definition).
//
// NOT implemented (labeled TODO, deliberately not depended on): research
// 2.4 flags historical_figure.vague_relationships (relationship_quick_infost,
// a top-6 (hfid,vague_relationship_type) summary per figure) as a possible
// free fast path IF it turns out to be populated and fresh in fort mode --
// nothing in DFHack reads it, so that is an open live probe (#4, research
// §5.4), not a proven source. This three-tier walk is the proven
// implementation; swapping in vague_relationships (if the probe ever
// confirms it) would be a follow-up optimization, not a rewrite of this
// function's contract.
std::string handleSocialGraph(const std::string &args, uint8_t &status) {
    if (!df::global::world) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world is null");
    }
    int64_t unitFilter = jsonGetIntNarrative(args, "unit", -1);
    std::string kindFilter = jsonGetStringNarrative(args, "kind");

    CitizenIndex idx = build_citizen_index();
    std::vector<SocialEdge> edges;
    std::unordered_set<SocialEdgeKey, SocialEdgeKeyHash> seen;

    for (auto &kv : idx.hfidToUnit) {
        df::unit *u = kv.second;
        if (!u) continue;

        // Tier 1: unit.relationship_ids -- ONLY the four simple-type slots
        // (research 2.4's documented trap: every value from NUM onward is
        // enum vocabulary for OTHER structures, not array indices).
        {
            struct Slot { df::unit_relationship_type type; const char *label; };
            static const Slot slots[] = {
                {df::unit_relationship_type::Spouse, "spouse"},
                {df::unit_relationship_type::Mother, "mother"},
                {df::unit_relationship_type::Father, "father"},
                {df::unit_relationship_type::PetOwner, "pet_owner"},
            };
            for (auto &slot : slots) {
                int32_t targetId = u->relationship_ids[slot.type];
                if (targetId < 0) continue;
                df::unit *target = df::unit::find(targetId);
                if (!target || !Units::isCitizen(target)) continue; // citizens-only both ends
                SocialEdge e;
                e.aId = u->id; e.aName = Units::getReadableName(u);
                e.bId = target->id; e.bName = Units::getReadableName(target);
                e.kind = "family";
                e.relation = slot.label;
                addSocialEdge(edges, seen, std::move(e));
            }
        }

        df::historical_figure *hf = df::historical_figure::find(u->hist_figure_id);
        if (!hf) continue;

        // Tier 2: histfig_links -- deity (worship, the one citizens-only
        // exception) and lover/master/apprentice/companion (family kind).
        for (auto *link : hf->histfig_links) {
            if (!link) continue;
            auto type = link->getType();

            if (type == df::histfig_hf_link_type::DEITY) {
                df::historical_figure *deityHf = df::historical_figure::find(link->target_hf);
                if (!deityHf) continue;
                std::string deityName = Translation::translateName(&deityHf->name, true);
                if (deityName.empty()) continue;
                SocialEdge e;
                e.aId = u->id; e.aName = Units::getReadableName(u);
                e.bId = deityHf->id; e.bName = deityName; // deity histfig id-space, not a live unit id
                e.kind = "worship";
                e.relation = "deity";
                e.hasLinkStrength = true; e.linkStrength = link->link_strength;
                addSocialEdge(edges, seen, std::move(e));
                continue;
            }

            const char *relationLabel = nullptr;
            switch (type) {
            case df::histfig_hf_link_type::LOVER: relationLabel = "lover"; break;
            case df::histfig_hf_link_type::MASTER: relationLabel = "master"; break;
            case df::histfig_hf_link_type::APPRENTICE: relationLabel = "apprentice"; break;
            case df::histfig_hf_link_type::COMPANION: relationLabel = "companion"; break;
            default: continue; // mother/father/spouse/child (tier 1's job) and everything else out of scope
            }

            auto targetIt = idx.hfidToUnit.find(link->target_hf);
            if (targetIt == idx.hfidToUnit.end()) continue; // citizens-only both ends
            df::unit *target = targetIt->second;

            SocialEdge e;
            e.aId = u->id; e.aName = Units::getReadableName(u);
            e.bId = target->id; e.bName = Units::getReadableName(target);
            e.kind = "family";
            e.relation = relationLabel;
            e.hasLinkStrength = true; e.linkStrength = link->link_strength;
            addSocialEdge(edges, seen, std::move(e));
        }

        // Tier 3: hf_visual -- friend/grudge only, the sole source
        // (research 2.4's decisive negative: no histfig_links type exists
        // for either). Acquaintance-band, unranked entries are excluded
        // outright -- see socialGraphRankBucket's and the love-floor
        // constants' own doc comments.
        if (hf->info && hf->info->relationships) {
            for (auto *rel : hf->info->relationships->hf_visual) {
                if (!rel) continue;
                if (rel->histfig_id == hf->id) continue;
                auto targetIt = idx.hfidToUnit.find(rel->histfig_id);
                if (targetIt == idx.hfidToUnit.end()) continue; // citizens-only both ends
                df::unit *target = targetIt->second;

                SocialRankBucket bucket = socialGraphRankBucket(rel->rank);
                bool loveFriend = rel->core.love >= kSocialGraphFriendLoveFloor;
                bool loveGrudge = rel->core.love <= kSocialGraphGrudgeLoveCeiling;
                std::string kind;
                if (bucket == SocialRankBucket::Friend || loveFriend) kind = "friend";
                else if (bucket == SocialRankBucket::Grudge || loveGrudge) kind = "grudge";
                else continue; // Acquaintance-band noise -- not social-graph-worthy

                SocialEdge e;
                e.aId = u->id; e.aName = Units::getReadableName(u);
                e.bId = target->id; e.bName = Units::getReadableName(target);
                e.kind = kind;
                e.relation = (rel->rank == df::vague_relationship_type::none) ? "" : ENUM_KEY_STR(vague_relationship_type, rel->rank);
                e.hasLove = true; e.love = rel->core.love;
                e.hasMeetCount = true; e.meetCount = rel->meet_count;
                addSocialEdge(edges, seen, std::move(e));
            }
        }
    }

    // kind filter applied before the cap, so a narrowed query still fills
    // up to kSocialGraphMaxEdges of the kind actually asked for instead of
    // being crowded out by other kinds first.
    std::vector<SocialEdge> filtered;
    if (!kindFilter.empty()) {
        for (auto &e : edges) if (e.kind == kindFilter) filtered.push_back(std::move(e));
    } else {
        filtered = std::move(edges);
    }
    if (unitFilter >= 0) {
        std::vector<SocialEdge> byUnit;
        for (auto &e : filtered) {
            if (e.aId == (int32_t)unitFilter || e.bId == (int32_t)unitFilter) byUnit.push_back(std::move(e));
        }
        filtered = std::move(byUnit);
    }

    size_t total = filtered.size();
    bool clamped = filtered.size() > kSocialGraphMaxEdges;
    if (clamped) filtered.resize(kSocialGraphMaxEdges);

    std::ostringstream os;
    os << "{\"edges\":[";
    for (size_t i = 0; i < filtered.size(); i++) {
        if (i) os << ",";
        const SocialEdge &e = filtered[i];
        os << "{\"a_id\":" << jsonIntNarrative(e.aId) << ",\"a_name\":" << jsonStrNarrative(e.aName)
           << ",\"b_id\":" << jsonIntNarrative(e.bId) << ",\"b_name\":" << jsonStrNarrative(e.bName)
           << ",\"kind\":" << jsonStrNarrative(e.kind)
           << ",\"relation\":" << jsonStrNarrative(e.relation)
           << ",\"love\":" << (e.hasLove ? jsonIntNarrative(e.love) : "null")
           << ",\"meet_count\":" << (e.hasMeetCount ? jsonIntNarrative(e.meetCount) : "null")
           << ",\"link_strength\":" << (e.hasLinkStrength ? jsonIntNarrative(e.linkStrength) : "null")
           << "}";
    }
    os << "],\"total_edges\":" << jsonIntNarrative((int64_t)total);
    if (clamped) {
        os << ",\"clamp_note\":\"" << (total - kSocialGraphMaxEdges) << " more edge(s) not shown (narrow with kind or unit)\"";
    }
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}

// ---------------------------------------------------------------------------
// fort_art -- the fort's own art (wave 013-C / Lane 4, research 2.5 + 3.2
// mode=art).
//
// Fort-local filter is the UNION of two independently labeled provenance
// paths (research 2.5's own wording, carried through verbatim):
//   (a) "composed_here" -- one of the four history-event classes that
//       carries `site` (history_event_{poetic_form,musical_form,
//       dance_form}_createdst / written_content_composedst) matched against
//       plotinfo->site_id. world.history.events is append-only (the same
//       pooling guarantee as world.status.reports) -- one full scan the
//       FIRST time this query ever runs, then a max-id tail-cursor scan on
//       every call after (the announcement-cursor pattern, at world-history
//       scale). This cursor deliberately NEVER resets on reconnect --
//       unlike story_pulse's cursor (which resets because it answers "what
//       has this peer already seen"), fort_art recomputes its full answer
//       fresh every call regardless of what a peer has seen before; the
//       cursor here exists purely so the plugin's OWN internal "which ids
//       are composed at this site" cache doesn't re-walk history it has
//       already scanned -- the identical reason ingest_combat_reports'
//       cursor doesn't reset either.
//   (b) "brought_here" -- original_author/author (whichever field the
//       struct has) is one of the fort's own current citizens, AND the
//       work is NOT already in (a)'s composed-here set. Catches both a
//       fort composition the event join somehow missed and (the common
//       case) a migrant's pre-fort work -- this project cannot tell those
//       two apart from the struct fields alone, so both share the one
//       honest label research 2.5 specifies rather than a guessed
//       distinction.
// Entity-level "performed here" proxies (historical_entity.performed_*,
// research 2.5) are deliberately NOT consulted here -- they conflate
// performed-here with composed-here and stay out per research's own
// caveat (labeled fallback only, not implemented this wave).
// ---------------------------------------------------------------------------

// ArtComposedInfo -- one form/content id's "composed at this site" fact,
// keyed by form/content id in the four maps below. year is the joining
// history_event's own `year` field (surfaced as composed_year in
// ArtWorkLine/writeArtWorkJson below) so composed-here works can be shown
// most-recent-first -- something the form/content structs themselves carry
// no timestamp for at all; only the joining event does.
struct ArtComposedInfo {
    int32_t year = 0;
};

static std::mutex g_art_mutex;
static int32_t g_art_event_cursor = -1;    // max history_event id scanned so far
static bool g_art_first_scan_done = false; // separate from the cursor: an empty
                                            // events vector would otherwise look
                                            // identical to "never scanned"
static double g_art_first_scan_ms = -1.0;  // recorded once, for the ACK's
                                            // scan_note (live probe #10)
static std::unordered_map<int32_t, ArtComposedInfo> g_art_composed_poetic;
static std::unordered_map<int32_t, ArtComposedInfo> g_art_composed_musical;
static std::unordered_map<int32_t, ArtComposedInfo> g_art_composed_dance;
static std::unordered_map<int32_t, ArtComposedInfo> g_art_composed_written;

// kArtScanSlowThresholdMs -- live probe #10 (research §5.10) asks this file
// to "time the first scan and note it in the ACK if slow" without giving a
// number; this project picks a threshold the same judgment-call way
// kBleedingThresholdPercent's own doc comment does -- a duration a reader
// would actually call slow for a single query call, not a transcribed DF
// constant.
static const double kArtScanSlowThresholdMs = 50.0;

// ArtScanResult -- what scan_art_history_events() reports back to its one
// caller (handleFortArt) so the slow-scan note can be composed without a
// second lock acquisition to re-read the globals it just wrote.
struct ArtScanResult {
    bool wasFirstScan = false;
    double ms = -1.0;
};

// scan_art_history_events walks world.history.events from the cursor
// forward (guarded binsearch, the same manageReportEvent-derived template
// ingest_combat_reports uses for world.status.reports -- both vectors are
// append-only and id-sorted per this project's 53.15 pooling notes),
// classifying each event by its OWN getType() and casting to the matching
// derived struct ONLY on that match (avoids four strict_virtual_cast
// attempts per event). Every event advances the cursor regardless of type
// -- it must move past non-art events too, or the next call would re-walk
// them forever.
static ArtScanResult scan_art_history_events() {
    ArtScanResult result;
    if (!df::global::world) return result;
    auto &events = df::global::world->history.events;

    std::lock_guard<std::mutex> lock(g_art_mutex);
    result.wasFirstScan = !g_art_first_scan_done;
    auto t0 = result.wasFirstScan ? std::chrono::steady_clock::now() : std::chrono::steady_clock::time_point{};

    int32_t cursorBefore = g_art_event_cursor;
    int rawIdx = df::history_event::binsearch_index(events, cursorBefore, false);
    size_t idx = (rawIdx < 0) ? 0 : (size_t)rawIdx;
    while (idx < events.size() && events[idx]->id <= cursorBefore) idx++;

    int32_t siteId = df::global::plotinfo ? df::global::plotinfo->site_id : -1;

    for (; idx < events.size(); idx++) {
        df::history_event *e = events[idx];
        if (!e) continue;
        g_art_event_cursor = e->id;
        if (siteId < 0) continue; // no resolvable site -- can't match, but the cursor still advances above

        switch (e->getType()) {
        case df::history_event_type::POETIC_FORM_CREATED:
            if (auto *ev = strict_virtual_cast<df::history_event_poetic_form_createdst>(e))
                if (ev->site == siteId) g_art_composed_poetic[ev->form] = ArtComposedInfo{ev->year};
            break;
        case df::history_event_type::MUSICAL_FORM_CREATED:
            if (auto *ev = strict_virtual_cast<df::history_event_musical_form_createdst>(e))
                if (ev->site == siteId) g_art_composed_musical[ev->form] = ArtComposedInfo{ev->year};
            break;
        case df::history_event_type::DANCE_FORM_CREATED:
            if (auto *ev = strict_virtual_cast<df::history_event_dance_form_createdst>(e))
                if (ev->site == siteId) g_art_composed_dance[ev->form] = ArtComposedInfo{ev->year};
            break;
        case df::history_event_type::WRITTEN_CONTENT_COMPOSED:
            if (auto *ev = strict_virtual_cast<df::history_event_written_content_composedst>(e))
                if (ev->site == siteId) g_art_composed_written[ev->content] = ArtComposedInfo{ev->year};
            break;
        default:
            break;
        }
    }

    if (result.wasFirstScan) {
        g_art_first_scan_done = true;
        auto t1 = std::chrono::steady_clock::now();
        g_art_first_scan_ms = std::chrono::duration<double, std::milli>(t1 - t0).count();
        result.ms = g_art_first_scan_ms;
    }
    return result;
}

// ArtWorkLine -- one work's worth of per-kind fields, all in one struct
// (the same "every consumer's field subset, empty when N/A" shape
// dwarf_portrait's own JSON already uses) rather than four separate
// structs -- keeps the composed/brought sort-and-cap step below
// kind-agnostic.
struct ArtWorkLine {
    std::string kind;    // "poetic_form" | "musical_form" | "dance_form" | "written_content"
    int32_t id = -1;
    std::string title;
    std::string creator; // "" when original_author/author doesn't resolve
    bool composedHere = false;
    int32_t composedYear = 0; // only meaningful when composedHere

    // poetic_form
    std::string mood, subject, subjectDetail, action, worshipTarget;
    // musical_form
    std::string purpose, devotionTarget;
    // dance_form -- research 2.5's "narrative gold" fields
    std::string context, character, creatureImitated;
    int32_t event = -1; // raw history_event id the dance acts out; -1 = none.
                         // No textual resolution attempted -- a full
                         // event-to-prose composer is out of scope this
                         // wave (research 3.4).
    // written_content
    std::string writtenType;
    std::vector<std::string> styles;
};

// kFortArtMaxWorks/kFortArtMaxStylesPerWork -- research 3.2's "~150-300
// tokens/call" budget for mode=art, the tightest of fort_story's three
// modes; capped well below story_pulse's 12-15 and social_graph's ~40 to
// match. Census counts (handleFortArt below) are always the TRUE uncapped
// totals -- only the per-work line list is capped.
static const size_t kFortArtMaxWorks = 10;
static const size_t kFortArtMaxStylesPerWork = 2;

// writeArtWorkJson serializes one ArtWorkLine. Every kind-specific field is
// emitted as an empty string / null when not applicable to this line's
// kind (or simply unresolved) -- never omitted from the shape, so the Go
// side can parse one fixed struct regardless of which kind a given line is.
static void writeArtWorkJson(std::ostringstream &os, const ArtWorkLine &w) {
    os << "{\"kind\":" << jsonStrNarrative(w.kind)
       << ",\"id\":" << jsonIntNarrative(w.id)
       << ",\"title\":" << jsonStrNarrative(w.title)
       << ",\"creator\":" << jsonStrNarrative(w.creator)
       << ",\"provenance\":" << jsonStrNarrative(w.composedHere ? "composed_here" : "brought_here");
    if (w.composedHere) os << ",\"composed_year\":" << jsonIntNarrative(w.composedYear);
    else os << ",\"composed_year\":null";

    if (w.kind == "poetic_form") {
        os << ",\"mood\":" << jsonStrNarrative(w.mood)
           << ",\"subject\":" << jsonStrNarrative(w.subject)
           << ",\"subject_detail\":" << jsonStrNarrative(w.subjectDetail)
           << ",\"action\":" << jsonStrNarrative(w.action)
           << ",\"worship_target\":" << jsonStrNarrative(w.worshipTarget);
    } else if (w.kind == "musical_form") {
        os << ",\"purpose\":" << jsonStrNarrative(w.purpose)
           << ",\"devotion_target\":" << jsonStrNarrative(w.devotionTarget);
    } else if (w.kind == "dance_form") {
        os << ",\"context\":" << jsonStrNarrative(w.context)
           << ",\"character\":" << jsonStrNarrative(w.character)
           << ",\"creature_imitated\":" << jsonStrNarrative(w.creatureImitated)
           << ",\"event\":" << (w.event >= 0 ? jsonIntNarrative(w.event) : std::string("null"));
    } else { // written_content
        os << ",\"written_type\":" << jsonStrNarrative(w.writtenType)
           << ",\"styles\":[";
        for (size_t i = 0; i < w.styles.size(); i++) {
            if (i) os << ",";
            os << jsonStrNarrative(w.styles[i]);
        }
        os << "]";
    }
    os << "}";
}

// handleFortArt answers fort_art {} (research 2.5 + 3.2 mode=art): every
// poetic/musical/dance form and written content this fort either composed
// itself or that one of its own current citizens brought with them,
// provenance-labeled per line, most-narratively-relevant first (composed
// here, most recent first; then brought here). Truthfully degrades to an
// empty works list (never an error) when the fort has produced or brought
// nothing yet.
std::string handleFortArt(const std::string &args, uint8_t &status) {
    (void)args; // no parameters -- research 3.2: `fort_art {}`
    if (!df::global::world || !df::global::plotinfo) {
        status = QUERY_STATUS_ERROR;
        return jsonErrorNarrative("world or plotinfo is null");
    }

    ArtScanResult scan = scan_art_history_events();
    CitizenIndex idx = build_citizen_index();

    std::unordered_map<int32_t, ArtComposedInfo> composedPoetic, composedMusical, composedDance, composedWritten;
    {
        std::lock_guard<std::mutex> lock(g_art_mutex);
        composedPoetic = g_art_composed_poetic;
        composedMusical = g_art_composed_musical;
        composedDance = g_art_composed_dance;
        composedWritten = g_art_composed_written;
    }

    std::vector<ArtWorkLine> composedWorks, broughtWorks;

    // poetic_form -- name via translateName (a language_name, unlike
    // written_content's raw title string below); subject_target is a union
    // keyed by `subject` itself (Histfig vs Concept, research 2.5), never
    // read except under its own matching subject value.
    for (auto *f : df::global::world->poetic_forms.all) {
        if (!f) continue;
        auto compIt = composedPoetic.find(f->id);
        bool composedHere = compIt != composedPoetic.end();
        bool authorIsCitizen = idx.hfids.find(f->original_author) != idx.hfids.end();
        if (!composedHere && !authorIsCitizen) continue;

        ArtWorkLine line;
        line.kind = "poetic_form";
        line.id = f->id;
        line.title = Translation::translateName(&f->name, true);
        line.creator = resolveHfDisplayName(f->original_author, idx);
        line.composedHere = composedHere;
        if (composedHere) line.composedYear = compIt->second.year;
        if (f->mood != df::poetic_form_mood::None) line.mood = ENUM_KEY_STR(poetic_form_mood, f->mood);
        if (f->action != df::poetic_form_action::None) line.action = ENUM_KEY_STR(poetic_form_action, f->action);
        if (f->subject != df::poetic_form_subject::None) {
            line.subject = ENUM_KEY_STR(poetic_form_subject, f->subject);
            if (f->subject == df::poetic_form_subject::Histfig) {
                line.subjectDetail = resolveHfDisplayName(f->subject_target.Histfig.subject_histfig, idx);
            } else if (f->subject == df::poetic_form_subject::Concept) {
                // subject_topic is stored as a raw int32_t in the union (not
                // the sphere_type enum itself) -- bounds-checked cast via
                // DFHack::is_valid_enum_item, the same guard resolve_thought's
                // RelativeExpelled case uses for the identical raw-int-to-enum
                // shape, rather than trusting an unchecked cast into
                // ENUM_KEY_STR's key_table lookup.
                auto sphere = (df::sphere_type)f->subject_target.Concept.subject_topic;
                if (DFHack::is_valid_enum_item(sphere)) line.subjectDetail = ENUM_KEY_STR(sphere_type, sphere);
            }
        }
        if (f->subject_hf >= 0) line.worshipTarget = resolveHfDisplayName(f->subject_hf, idx);

        (composedHere ? composedWorks : broughtWorks).push_back(std::move(line));
    }

    // musical_form
    for (auto *f : df::global::world->musical_forms.all) {
        if (!f) continue;
        auto compIt = composedMusical.find(f->id);
        bool composedHere = compIt != composedMusical.end();
        bool authorIsCitizen = idx.hfids.find(f->original_author) != idx.hfids.end();
        if (!composedHere && !authorIsCitizen) continue;

        ArtWorkLine line;
        line.kind = "musical_form";
        line.id = f->id;
        line.title = Translation::translateName(&f->name, true);
        line.creator = resolveHfDisplayName(f->original_author, idx);
        line.composedHere = composedHere;
        if (composedHere) line.composedYear = compIt->second.year;
        if (f->purpose != df::musical_form_purpose::NONE) line.purpose = ENUM_KEY_STR(musical_form_purpose, f->purpose);
        if (f->devotion_target >= 0) line.devotionTarget = resolveHfDisplayName(f->devotion_target, idx);

        (composedHere ? composedWorks : broughtWorks).push_back(std::move(line));
    }

    // dance_form -- event/hfid/race are the "narrative gold" fields
    // research 2.5 calls out by name: event is left as a raw history_event
    // id (no textual resolution attempted -- see ArtWorkLine's own doc
    // comment); hfid (character whose story) and race (creature imitated)
    // both resolve to names via the same helpers dwarf_portrait/
    // resolvePreferenceLabel already use.
    for (auto *f : df::global::world->dance_forms.all) {
        if (!f) continue;
        auto compIt = composedDance.find(f->id);
        bool composedHere = compIt != composedDance.end();
        bool authorIsCitizen = idx.hfids.find(f->original_author) != idx.hfids.end();
        if (!composedHere && !authorIsCitizen) continue;

        ArtWorkLine line;
        line.kind = "dance_form";
        line.id = f->id;
        line.title = Translation::translateName(&f->name, true);
        line.creator = resolveHfDisplayName(f->original_author, idx);
        line.composedHere = composedHere;
        if (composedHere) line.composedYear = compIt->second.year;
        if (f->context != df::dance_form_context::NONE) line.context = ENUM_KEY_STR(dance_form_context, f->context);
        line.event = f->event;
        if (f->hfid >= 0) line.character = resolveHfDisplayName(f->hfid, idx);
        if (f->race >= 0) {
            df::creature_raw *craw = df::creature_raw::find(f->race);
            if (craw) line.creatureImitated = craw->name[0];
        }

        (composedHere ? composedWorks : broughtWorks).push_back(std::move(line));
    }

    // written_content -- title is a raw stl-string (research 2.5: "no
    // translation needed"), unlike the three form types' language_name
    // above.
    for (auto *c : df::global::world->written_contents.all) {
        if (!c) continue;
        auto compIt = composedWritten.find(c->id);
        bool composedHere = compIt != composedWritten.end();
        bool authorIsCitizen = idx.hfids.find(c->author) != idx.hfids.end();
        if (!composedHere && !authorIsCitizen) continue;

        ArtWorkLine line;
        line.kind = "written_content";
        line.id = c->id;
        line.title = c->title;
        line.creator = resolveHfDisplayName(c->author, idx);
        line.composedHere = composedHere;
        if (composedHere) line.composedYear = compIt->second.year;
        if (c->type != df::written_content_type::NONE) line.writtenType = ENUM_KEY_STR(written_content_type, c->type);
        for (size_t i = 0; i < c->styles.size() && line.styles.size() < kFortArtMaxStylesPerWork; i++)
            line.styles.push_back(ENUM_KEY_STR(written_content_style, c->styles[i]));

        (composedHere ? composedWorks : broughtWorks).push_back(std::move(line));
    }

    // Composed-here first (the fort's OWN making), most recent first; then
    // brought-here, by id descending (no timestamp exists for these at all
    // -- see ArtComposedInfo's own doc comment) -- a stable, deterministic
    // order rather than a meaningful chronology.
    std::sort(composedWorks.begin(), composedWorks.end(), [](const ArtWorkLine &a, const ArtWorkLine &b) {
        return a.composedYear > b.composedYear;
    });
    std::sort(broughtWorks.begin(), broughtWorks.end(), [](const ArtWorkLine &a, const ArtWorkLine &b) {
        return a.id > b.id;
    });

    size_t totalComposed = composedWorks.size();
    size_t totalBrought = broughtWorks.size();
    size_t totalLocal = totalComposed + totalBrought;

    std::vector<const ArtWorkLine *> shown;
    for (auto &w : composedWorks) {
        if (shown.size() >= kFortArtMaxWorks) break;
        shown.push_back(&w);
    }
    for (auto &w : broughtWorks) {
        if (shown.size() >= kFortArtMaxWorks) break;
        shown.push_back(&w);
    }

    std::ostringstream os;
    os << "{\"works\":[";
    for (size_t i = 0; i < shown.size(); i++) {
        if (i) os << ",";
        writeArtWorkJson(os, *shown[i]);
    }
    os << "],\"census\":{\"known_in_fort\":" << jsonIntNarrative((int64_t)totalLocal)
       << ",\"composed_in_fort\":" << jsonIntNarrative((int64_t)totalComposed)
       << ",\"brought_here\":" << jsonIntNarrative((int64_t)totalBrought) << "}";
    if (totalLocal > shown.size()) {
        os << ",\"clamp_note\":\"" << (totalLocal - shown.size()) << " more work(s) not shown\"";
    }
    if (scan.wasFirstScan && scan.ms >= kArtScanSlowThresholdMs) {
        std::ostringstream note;
        note << "first fort_art call scanned the world's full history-event log ("
             << (int64_t)scan.ms << "ms) -- later calls are incremental";
        os << ",\"scan_note\":" << jsonStrNarrative(note.str());
    }
    os << "}";
    status = QUERY_STATUS_SUCCESS;
    return os.str();
}
