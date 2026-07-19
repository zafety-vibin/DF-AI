// DFHack Plugin — DF Announcement Streaming
//
// Polls df::global::world->status.announcements (the same array DF reads
// to populate the in-game announcement panel) and forwards new entries to
// the orchestrator over the binary protocol. The orchestrator's AlertStore
// dedupes by report ID, so the plugin only needs to track the highest ID
// it has already sent -- for BRAND-NEW reports. A repeated cancellation
// (e.g. "Digging designation cancelled: damp stone located." happening over
// and over) does NOT get a new report id in DF 50+: the game bumps
// repeat_count in place on the SAME df::report object instead. A pure
// id-cursor scheme makes those invisible once the id has scrolled past, so
// this file ALSO tracks the repeat_count we last sent for a bounded tail
// window of report ids and re-sends an entry whose repeat_count has grown
// since (see g_last_sent_repeat_count below). Every ANNOUNCEMENT_UPDATE
// entry (new or re-sent) carries its current repeat_count on the wire so
// the orchestrator can tell "the same problem, again" from "a new problem".
//
// Severity is derived from announcement_type: cancellations and combat
// emergencies are "critical" or "warn"; informational entries (migrant
// arrived, mood begun) are "info". The full mapping is intentionally
// crude — DF has ~200 announcement_type values and we only need rough
// triage, not perfect categorization.
//
// IMPORTANT: announcement field names in df::report have shifted across DF
// versions. As of DF 53.x the relevant fields are:
//   id, type, text, pos.x/y/z, year, time, repeat_count, flags
// If a future DFHack release renames these, update the field accesses below.

#include "Core.h"
#include "Console.h"
#include "df/world.h"
#include "df/report.h"
#include "df/announcement_type.h"
#include "df/coord.h"

#include "protocol.h"
#include "ActiveSocket.h"

#include <vector>
#include <atomic>
#include <cstdint>
#include <string>
#include <memory>
#include <mutex>
#include <unordered_map>

using namespace DFHack;

// Externs from df_ai_protocol.cpp
extern std::unique_ptr<CActiveSocket> g_socket;
extern void trip_step_tripwire(const std::string &reason);

// Helpers from tile_extractor.cpp
extern void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value);
extern void write_int16_be(std::vector<uint8_t> &buf, int16_t value);
extern void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value);

// Plugin-local state: highest report ID we've already sent. Reset when the
// plugin is unloaded or the connection drops. Atomic: the reset runs on
// socket-thread teardown with no core suspension, racing the sim-thread
// poller's read-modify-write; atomicity keeps the reset from being lost
// (announcements silently never re-sent after reconnect).
static std::atomic<int32_t> g_last_sent_report_id{-1};

// DF 50+ pools repeated announcements ("Digging designation cancelled: damp
// stone located." happening again and again) by bumping repeat_count IN
// PLACE on the SAME df::report object instead of appending a new one (see
// df.announcement.xml, Gui.cpp's addReport/report-dedup path). Our id-cursor
// approach above makes those invisible once the id has scrolled past --
// g_last_sent_report_id only tells us about brand-new reports. This map
// remembers the repeat_count value we last SENT for each report id so a
// later bump can be detected and re-sent as a fresh, actionable event.
//
// Bounded to a tail window (see TAIL_WINDOW below) instead of scanning/
// tracking the entire announcements array: repeats only ever happen to
// reports that are still "live" (the same job failing over and over in
// quick succession near the current tick), never to ancient history, and
// df::report objects are NEVER deleted (the array only grows), so an
// unbounded map or an unbounded per-poll scan would both leak/grow for the
// entire fort's life.
//
// Guarded by g_repeat_count_mutex because reset_announcement_cursor() (see
// below) can run on the socket thread during teardown/reconnect while
// poll_and_send_announcements() is concurrently reading/writing this map on
// the main thread -- an unsynchronized unordered_map under that race is a
// data race (UB), unlike the plain int32 cursor above which needed only
// atomicity.
static std::mutex g_repeat_count_mutex;
static std::unordered_map<int32_t, uint32_t> g_last_sent_repeat_count;
static const size_t TAIL_WINDOW = 200;

// classify_severity maps a raw announcement_type to the severity tier the
// orchestrator uses (0=info, 1=warn, 2=critical). The mapping is
// deliberately broad — most "cancel" types are warn, combat / siege /
// magma are critical, the rest is info.
// Enum spellings verified against DFHack 53.15-r1 df/announcement_type.h.
// The 53.15 enum split/renamed several pre-50.x names this file originally
// used; the remap is recorded case-by-case below. Types with no 53.15
// equivalent (SIEGE_BEGUN, FB_ARRIVAL, DROWNING, JOB_CANCEL_*, OBLIVIOUS,
// TRAUMA, INTERRUPT_CONSTRUCTION) were dropped — they either no longer
// exist as distinct announcement types or now arrive as CANCEL_JOB text.
static uint8_t classify_severity(int16_t type)
{
    using AT = df::announcement_type;
    switch (static_cast<df::announcement_type>(type)) {
        // Critical: things a player would drop everything to react to.
        // 53.15 split the old single AMBUSH into per-ambusher variants.
        case AT::AMBUSH_DEFENDER:
        case AT::AMBUSH_RESIDENT:
        case AT::AMBUSH_THIEF:
        case AT::AMBUSH_THIEF_SUPPORT_SKULKING:
        case AT::AMBUSH_THIEF_SUPPORT_NATURE:
        case AT::AMBUSH_THIEF_SUPPORT:
        case AT::AMBUSH_MISCHIEVOUS:
        case AT::AMBUSH_SNATCHER:
        case AT::AMBUSH_SNATCHER_SUPPORT:
        case AT::AMBUSH_AMBUSHER_NATURE:
        case AT::AMBUSH_AMBUSHER:
        case AT::AMBUSH_INJURED:
        case AT::AMBUSH_OTHER:
        case AT::AMBUSH_INCAPACITATED:
        case AT::BEAST_AMBUSH:
        case AT::MEGABEAST_ARRIVAL:
        case AT::WEREBEAST_ARRIVAL:              // was BECOME_WEREBEAST-adjacent
        case AT::CAVE_COLLAPSE:
        case AT::CAUGHT_IN_FLAMES:               // was MAGMA/DRAGONFIRE/FIRE_BURNING
        case AT::FLAME_HIT:
        case AT::CITIZEN_LOST_TO_STRESS:         // was INSANITY
        case AT::BERSERK_CITIZEN:                // was BERSERK
        case AT::BODY_TRANSFORMATION:            // was BECOME_VAMPIRE/BECOME_WEREBEAST
        // Active hostile-event starts — same tier as the ambush/beast
        // arrivals above (an undead/night-creature assault the fort must
        // drop everything to respond to, not a background nuisance).
        case AT::GHOST_ATTACK:
        case AT::UNDEAD_ATTACK:
        case AT::NIGHT_ATTACK_STARTS:
            return 2;

        // Warn: cancellations, suspensions, stress. Routine attention
        // required but not life-threatening.
        case AT::CANCEL_JOB:
        case AT::CONSTRUCTION_SUSPENDED:         // was CANCEL_CONSTRUCTION
        case AT::CANNOT_CONSTRUCT:
        case AT::UNABLE_TO_COMPLETE_BUILDING:    // was INTERRUPT_BUILDING
        case AT::CITIZEN_TANTRUM:                // was TANTRUM
        case AT::POSSESSED_TANTRUM:
        case AT::STRESSED_CITIZEN:               // was DEPRESSED
        case AT::DIG_CANCEL_DAMP:
        case AT::DIG_CANCEL_WARM:
        // New in 53.x (dinosaur update): stuck creatures/citizens are
        // actionable for the agent (pathing broke — dig them out).
        case AT::CREATURE_STUCK:
        case AT::CITIZEN_STUCK:
        // Deaths: not an active threat by the time the report lands, but a
        // currently-invisible fort-roster change the agent must notice
        // (labor/squad/room assignments can now be stale). Below the
        // in-progress-attack tier above, still well above routine info.
        case AT::CITIZEN_DEATH:
        case AT::PET_DEATH:
        // Strange mood starts a hidden per-dwarf death timer — the fort
        // has a window to supply/route the mood dwarf before it ends badly.
        // Not "drop everything this tick" like an active attack, but the
        // agent must track it, so it and its two lifecycle follow-ups
        // (workshop claimed, artifact underway) all surface at warn so the
        // whole mood thread stays visible rather than going quiet after the
        // opening announcement.
        case AT::STRANGE_MOOD:
        case AT::MOOD_BUILDING_CLAIMED:
        case AT::ARTIFACT_BEGUN:
        // Mandate lifecycle: a currently-invisible obligation appears/
        // disappears that the agent's production planning needs to know
        // about (a controlled good's export can suddenly get punished).
        case AT::NEW_MANDATE:
        case AT::NEW_WORK_MANDATE:
        case AT::MANDATE_ENDS:
        // Diplomatic/economic consequence, not a survival threat, but a
        // state change worth the agent's attention (unmet demands soured
        // relations with the liaison's civilization).
        case AT::DIPLOMAT_LEFT_UNHAPPY:
            return 1;

        // Info: arrivals, completions, weather, narration.
        default:
            return 0;
    }
}

// Build an ANNOUNCEMENT_UPDATE message from the given announcements and
// send it. Returns the count actually sent (which may be less than `news`'s
// size if we hit the per-message cap).
static size_t send_announcement_update(const std::vector<df::report*> &news)
{
    if (!g_socket || !g_socket->IsSocketValid()) {
        return 0;
    }
    if (news.empty()) {
        return 0;
    }

    // Cap per message — defense against a flood (e.g. mass deaths). Older
    // announcements stay in DF's own buffer; we'll pick them up next call.
    const size_t MAX_PER_MSG = 64;
    size_t sendCount = news.size();
    if (sendCount > MAX_PER_MSG) sendCount = MAX_PER_MSG;

    std::vector<uint8_t> msg;
    msg.resize(4, 0); // length placeholder
    msg.push_back(PROTOCOL_VERSION);
    msg.push_back(MSG_TYPE_ANNOUNCEMENT_UPDATE);

    // Payload: [4: Count] [N × entry]
    write_uint32_be(msg, static_cast<uint32_t>(sendCount));

    for (size_t i = 0; i < sendCount; i++) {
        df::report* r = news[i];
        if (!r) continue;

        write_uint32_be(msg, static_cast<uint32_t>(r->id));
        write_uint16_be(msg, static_cast<uint16_t>(r->type));
        msg.push_back(classify_severity(static_cast<int16_t>(r->type)));

        // Position. DF uses (-30000) sentinel for "no position" in some
        // report types; clamp to (-1) for our protocol.
        int16_t x = (r->pos.x < 0 || r->pos.x > 32000) ? -1 : static_cast<int16_t>(r->pos.x);
        int16_t y = (r->pos.y < 0 || r->pos.y > 32000) ? -1 : static_cast<int16_t>(r->pos.y);
        int16_t z = (r->pos.z < 0 || r->pos.z > 32000) ? -1 : static_cast<int16_t>(r->pos.z);
        write_int16_be(msg, x);
        write_int16_be(msg, y);
        write_int16_be(msg, z);

        write_uint32_be(msg, static_cast<uint32_t>(r->year));
        write_uint32_be(msg, static_cast<uint32_t>(r->time));

        // Text — clamp to uint16 length cap.
        const std::string &text = r->text;
        size_t textLen = text.size();
        if (textLen > 4096) textLen = 4096;
        write_uint16_be(msg, static_cast<uint16_t>(textLen));
        if (textLen > 0) {
            msg.insert(msg.end(), text.begin(), text.begin() + textLen);
        }
    }

    // Trailing repeat_count block: one uint32 per entry above, in the same
    // order, appended AFTER all N entries rather than interleaved into each
    // entry's own fields. See the MSG_TYPE_ANNOUNCEMENT_UPDATE comment in
    // protocol.h for why this placement matters for backward-compat decode.
    for (size_t i = 0; i < sendCount; i++) {
        df::report* r = news[i];
        uint32_t repeatCount = r ? static_cast<uint32_t>(r->repeat_count) : 0;
        write_uint32_be(msg, repeatCount);
    }

    // Fill length header.
    uint32_t length = static_cast<uint32_t>(msg.size());
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    socket_send_locked(msg.data(), msg.size());
    return sendCount;
}

// poll_and_send_announcements walks df::global::world->status.announcements
// and forwards entries with id > g_last_sent_report_id. Updates the
// last-sent cursor on success. Safe to call from plugin_onupdate (main
// thread, has CoreSuspender protection from caller).
//
// Returns the number of announcements actually sent. 0 on no-op or error.
size_t poll_and_send_announcements()
{
    if (!df::global::world) return 0;

    // The 53.x DF builds expose announcements as world.status.announcements
    // (vector<df::report*>). If your DFHack version renamed this, adjust
    // here.
    auto &arr = df::global::world->status.announcements;
    if (arr.empty()) return 0;

    std::vector<df::report*> news;
    news.reserve(32);
    int32_t newHighest = g_last_sent_report_id;

    // Step tripwire: remember the first NEW critical (severity-2) report
    // this poll observes. The trip itself runs after the send + cursor
    // update below so the nested announcement poll inside the tripwire's
    // state push finds nothing new.
    std::string criticalText;

    // 1) Brand-new reports (id beyond the cursor) -- unchanged from before.
    for (size_t i = 0; i < arr.size(); i++) {
        df::report* r = arr[i];
        if (!r) continue;
        if (r->id <= g_last_sent_report_id) continue;
        news.push_back(r);
        if (r->id > newHighest) newHighest = r->id;
        if (criticalText.empty() &&
            classify_severity(static_cast<int16_t>(r->type)) == 2) {
            criticalText = r->text;
        }
    }

    // 2) Repeat-count bumps on already-sent reports near the tail (see the
    // g_last_sent_repeat_count comment above). Only the last TAIL_WINDOW
    // array slots are scanned, so this stays O(TAIL_WINDOW) regardless of
    // how many reports the fort has accumulated over its life.
    {
        std::lock_guard<std::mutex> lock(g_repeat_count_mutex);
        size_t tailStart = (arr.size() > TAIL_WINDOW) ? (arr.size() - TAIL_WINDOW) : 0;
        for (size_t i = tailStart; i < arr.size(); i++) {
            df::report* r = arr[i];
            if (!r) continue;
            if (r->id > g_last_sent_report_id) continue; // already queued above as "new"
            uint32_t lastCount = 0;
            auto it = g_last_sent_repeat_count.find(r->id);
            if (it != g_last_sent_repeat_count.end()) lastCount = it->second;
            if (static_cast<uint32_t>(r->repeat_count) <= lastCount) continue;
            news.push_back(r);
            if (criticalText.empty() &&
                classify_severity(static_cast<int16_t>(r->type)) == 2) {
                criticalText = r->text;
            }
        }
        // Prune entries that have scrolled out of the tail window -- they
        // will never be checked again, so forgetting their baseline is safe
        // (worst case: if one somehow gets bumped again after falling out of
        // the window, it re-sends from a baseline of 0 -- a harmless extra
        // resend, never a suppressed one).
        if (tailStart < arr.size() && arr[tailStart]) {
            int32_t tailBoundaryId = arr[tailStart]->id;
            for (auto mit = g_last_sent_repeat_count.begin(); mit != g_last_sent_repeat_count.end(); ) {
                if (mit->first < tailBoundaryId) mit = g_last_sent_repeat_count.erase(mit);
                else ++mit;
            }
        }
    }

    if (news.empty()) return 0;

    size_t sent = send_announcement_update(news);
    if (sent > 0) {
        // Advance cursor by what we actually sent (in id order). Since DF
        // appends to the array in id-monotonic order, taking the max id
        // among the sent slice is correct. If we capped at MAX_PER_MSG,
        // the rest will catch up next poll. Repeat-bump entries never move
        // this cursor forward -- their id is always <= the pre-poll cursor.
        if (sent == news.size()) {
            g_last_sent_report_id = newHighest;
        } else {
            int32_t partial = g_last_sent_report_id;
            for (size_t i = 0; i < sent; i++) {
                if (news[i]->id > partial) partial = news[i]->id;
            }
            g_last_sent_report_id = partial;
        }
        // Record the repeat_count value we just sent for every entry
        // actually sent (new or repeat-bump alike), so the next poll's tail
        // scan compares against what the peer has actually seen.
        std::lock_guard<std::mutex> lock(g_repeat_count_mutex);
        for (size_t i = 0; i < sent; i++) {
            df::report* r = news[i];
            if (!r) continue;
            g_last_sent_repeat_count[r->id] = static_cast<uint32_t>(r->repeat_count);
        }
    }

    // A critical announcement mid-step ends the step immediately (pause +
    // state push). No-op unless a step is actually in progress — see
    // trip_step_tripwire in df_ai_protocol.cpp.
    if (!criticalText.empty()) {
        trip_step_tripwire(criticalText);
    }
    return sent;
}

// reset_announcement_cursor zeroes the last-sent ID so a reconnect re-sends
// the whole visible buffer. Call this from disconnect_from_server() or
// on plugin reload.
//
// Known edge case (accepted): the re-sent backlog counts as "new", so if a
// step is somehow still in progress across a reconnect, the first poll can
// trip the step tripwire on a stale critical report. That trip is a
// conservative pause and the re-sent reports ARE new to the reconnected
// peer, so we deliberately don't suppress it.
void reset_announcement_cursor()
{
    g_last_sent_report_id = -1;
    std::lock_guard<std::mutex> lock(g_repeat_count_mutex);
    g_last_sent_repeat_count.clear();
}
