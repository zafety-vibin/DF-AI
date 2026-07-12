// DFHack Plugin — DF Announcement Streaming
//
// Polls df::global::world->status.announcements (the same array DF reads
// to populate the in-game announcement panel) and forwards new entries to
// the orchestrator over the binary protocol. The orchestrator's AlertStore
// dedupes by report ID, so the plugin only needs to track the highest ID
// it has already sent.
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
#include <cstdint>
#include <string>
#include <memory>

using namespace DFHack;

// Externs from df_ai_protocol.cpp
extern std::unique_ptr<CActiveSocket> g_socket;

// Helpers from tile_extractor.cpp
extern void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value);
extern void write_int16_be(std::vector<uint8_t> &buf, int16_t value);
extern void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value);

// Plugin-local state: highest report ID we've already sent. Reset when the
// plugin is unloaded or the connection drops.
static int32_t g_last_sent_report_id = -1;

// classify_severity maps a raw announcement_type to the severity tier the
// orchestrator uses (0=info, 1=warn, 2=critical). The mapping is
// deliberately broad — most "cancel" types are warn, combat / siege /
// magma are critical, the rest is info.
static uint8_t classify_severity(int16_t type)
{
    using AT = df::announcement_type;
    switch (static_cast<df::announcement_type>(type)) {
        // Critical: things a player would drop everything to react to.
        case AT::AMBUSH:
        case AT::SIEGE_BEGUN:
        case AT::MEGABEAST_ARRIVAL:
        case AT::FB_ARRIVAL:                     // forgotten beast
        case AT::CAVE_COLLAPSE:
        case AT::DROWNING:
        case AT::MAGMA_BURNING:
        case AT::DRAGONFIRE_BURNING:
        case AT::FIRE_BURNING:
        case AT::INSANITY:
        case AT::BERSERK:
        case AT::BECOME_VAMPIRE:
        case AT::BECOME_WEREBEAST:
            return 2;

        // Warn: cancellations, suspensions, depression, miasma. Routine
        // attention required but not life-threatening.
        case AT::CANCEL_JOB:
        case AT::CANCEL_CONSTRUCTION:
        case AT::JOB_CANCEL_THIRSTY:
        case AT::JOB_CANCEL_HUNGRY:
        case AT::JOB_CANCEL_SLEEPY:
        case AT::JOB_CANCEL_DROWSY:
        case AT::TANTRUM:
        case AT::DEPRESSED:
        case AT::OBLIVIOUS:
        case AT::TRAUMA:
        case AT::INTERRUPT_CONSTRUCTION:
        case AT::INTERRUPT_BUILDING:
        case AT::DIG_CANCEL_DAMP:
        case AT::DIG_CANCEL_WARM:
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

    // Fill length header.
    uint32_t length = static_cast<uint32_t>(msg.size());
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    g_socket->Send(msg.data(), msg.size());
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

    for (size_t i = 0; i < arr.size(); i++) {
        df::report* r = arr[i];
        if (!r) continue;
        if (r->id <= g_last_sent_report_id) continue;
        news.push_back(r);
        if (r->id > newHighest) newHighest = r->id;
    }

    if (news.empty()) return 0;

    size_t sent = send_announcement_update(news);
    if (sent > 0) {
        // Advance cursor by what we actually sent (in id order). Since DF
        // appends to the array in id-monotonic order, taking the max id
        // among the sent slice is correct. If we capped at MAX_PER_MSG,
        // the rest will catch up next poll.
        if (sent == news.size()) {
            g_last_sent_report_id = newHighest;
        } else {
            int32_t partial = g_last_sent_report_id;
            for (size_t i = 0; i < sent; i++) {
                if (news[i]->id > partial) partial = news[i]->id;
            }
            g_last_sent_report_id = partial;
        }
    }
    return sent;
}

// reset_announcement_cursor zeroes the last-sent ID so a reconnect re-sends
// the whole visible buffer. Call this from disconnect_from_server() or
// on plugin reload.
void reset_announcement_cursor()
{
    g_last_sent_report_id = -1;
}
