// DFHack Binary Protocol Plugin
// Feature: 001-binary-protocol
// Target: DFHack 53.02-r1, DF 53.02

#include "Core.h"
#include "Console.h"
#include "Export.h"
#include "PluginManager.h"
#include "modules/MapCache.h"
#include "modules/Maps.h"
#include "modules/Units.h"
#include "modules/Buildings.h"
#include "modules/World.h"

#include "df/map_block.h"
#include "df/world.h"
#include "df/coord.h"
#include "df/unit.h"
#include "df/unit_labor.h"
#include "df/tile_dig_designation.h"
#include "df/building.h"
#include "df/building_civzonest.h"
#include "df/building_stockpilest.h"
#include "df/building_type.h"

#include "protocol.h"
#include "ActiveSocket.h"  // SimpleSockets

#include <memory>
#include <vector>
#include <cstring>
#include <chrono>
#include <thread>
#include <sstream>
#include <fstream>
#include <mutex>
#include <queue>
#include <atomic>
#include <exception>

using namespace DFHack;

DFHACK_PLUGIN("df_ai_protocol");
DFHACK_PLUGIN_IS_ENABLED(is_enabled);

// Forward declarations for functions from designations.cpp
bool applyDigDesignation(const std::vector<uint8_t> &payload, std::string &error,
                         bool &destroysExistingStairs);
bool applyCancelDesignation(const std::vector<uint8_t> &payload, std::string &error);
bool applyBuildDesignation(const std::vector<uint8_t> &payload, std::string &error);
bool applySmoothDesignation(const std::vector<uint8_t> &payload, std::string &error);

// Forward declarations for functions from work_orders.cpp
bool applyWorkOrder(uint8_t orderType, uint16_t quantity, const std::string &jobTypeName,
                     const std::string &material, uint8_t frequencyByte,
                     const std::string &subtype, std::string &error);
bool applyQueueJob(int16_t x, int16_t y, int16_t z, uint8_t orderType, const std::string &jobTypeName,
                    const std::string &material, const std::string &subtype, std::string &error);
bool applyCancelOrder(int32_t orderID, std::string &error);
bool applyEditOrder(int32_t orderID, bool hasAmount, uint16_t newAmount,
                     bool hasFrequency, uint8_t newFrequency,
                     bool &deleted, std::string &error);

// Forward declarations for functions from plants.cpp
bool applyChopDesignation(int16_t x1, int16_t y1, int16_t z1, int16_t x2, int16_t y2, int16_t z2, std::string &error);
bool applyGatherDesignation(int16_t x1, int16_t y1, int16_t z1, int16_t x2, int16_t y2, int16_t z2, std::string &error);

// Forward declarations for functions in this file
bool applyUnsuspend(int16_t x, int16_t y, int16_t z, std::string &error);
bool applyRemoveBuilding(int16_t x, int16_t y, int16_t z, std::string &error);
bool applyRemoveZone(int16_t x, int16_t y, int16_t z, std::string &error);
bool applySetLabor(int32_t unitID, uint8_t laborID, bool enable, std::string &error);

// Forward declarations for functions from work_details.cpp
bool applyAssignWorkDetail(uint16_t detailIndex, int32_t unitID, bool add, std::string &error);
bool applySetWorkDetailMode(uint16_t detailIndex, uint8_t mode, std::string &error);
bool applyCreateWorkDetail(const std::string &name, uint8_t mode, const std::vector<uint8_t> &laborIDs, std::string &error);

// Forward declarations for functions from trade.cpp
bool applyBringGoodsToDepot(int16_t x, int16_t y, int16_t z,
                            const std::string &itemTypeFilter, const std::string &materialFilter,
                            uint8_t itemClass, int32_t maxCount, int64_t maxTotalValue, std::string &error);
bool applyUnmarkTradeGoods(int16_t x, int16_t y, int16_t z,
                           const std::string &itemTypeFilter, const std::string &materialFilter,
                           uint8_t itemClass, int32_t maxCount, std::string &error);
bool applySetDepotTradeFlags(int16_t x, int16_t y, int16_t z,
                              bool traderRequested, bool anyoneCanTrade, std::string &error);

// Forward declarations for functions from nobles.cpp
bool applyAppointPosition(int32_t unitID, const std::string &positionCode, std::string &error);
bool applySetBookkeeperPrecision(uint8_t precisionByte, std::string &error);

// Forward declarations for functions from military.cpp
bool applyCreateSquad(const std::string &positionCode, std::string &error);
bool applyAssignSquad(int32_t squadID, int32_t unitID, bool add, std::string &error);
bool applySquadOrder(int32_t squadID, uint8_t orderType, int16_t x, int16_t y, int16_t z,
                      const std::string &burrowName, std::string &error);

// Forward declarations for functions in buildings.cpp
bool placeStockpile(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, uint32_t groupMask, std::string &error, bool &partial);
bool placeFarmPlot(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error);
bool applySetFarmCrop(int16_t x, int16_t y, int16_t z, uint8_t season, const std::string &cropName, std::string &error);
bool placeBridge(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, uint8_t wireDirection, std::string &error);
bool applySetWorkshopProfile(int16_t x, int16_t y, int16_t z, int32_t minSkillLevel, int32_t maxSkillLevel, int32_t workerUnitID, std::string &error);

// Forward declarations for functions from mechanisms.cpp
bool applyPullLever(int16_t x, int16_t y, int16_t z, std::string &error);
bool applyLinkBuilding(int16_t leverX, int16_t leverY, int16_t leverZ, int16_t targetX, int16_t targetY, int16_t targetZ, std::string &error);

// Forward declarations for functions from zones.cpp
bool applyDesignateZone(uint8_t zoneType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error);
bool applyAssignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error);
bool applyUnassignZone(int16_t x, int16_t y, int16_t z, int32_t unitID, std::string &error);

// Forward declaration from locations.cpp
bool applyCreateLocation(int16_t x, int16_t y, int16_t z, uint8_t locationType, const std::string &profession, std::string &error);
bool applyAssignLodging(int16_t tavernX, int16_t tavernY, int16_t tavernZ, int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error);
bool applyUnassignLodging(int16_t bedroomX, int16_t bedroomY, int16_t bedroomZ, std::string &error);

// Forward declarations from burrows.cpp
bool applyDesignateBurrow(const std::string &name, int16_t x1, int16_t y1, int16_t z1, int16_t x2, int16_t y2, int16_t z2, std::string &error);
bool applyRemoveBurrow(const std::string &name, std::string &error);
bool applyAssignBurrow(const std::string &name, bool assign, bool allCitizens, int32_t unitID, std::string &error);
bool applySetAlert(const std::string &name, bool active, std::string &error);

// Forward declarations for functions from queries.cpp
void executeQuery(uint32_t queryID, const std::string &name, const std::string &args);
void reset_wildlife_tripwire_tracking();  // B11: dangerous-wildlife tripwire

// Forward declaration from narrative.cpp (Feature 013 wave 013-A / Lane 2:
// combat narrator). Ingests world.status.reports into the rolling
// combat-report buffer + wound watermarks combat_summary/combat_log read.
// No reset-on-reconnect counterpart (unlike reset_announcement_cursor/
// reset_wildlife_tripwire_tracking below): this state is this plugin's OWN
// cache of already-observed combat, not "what has been sent to the
// currently-connected peer", so a reconnecting client should see the
// current accumulated state rather than replay it.
void ingest_combat_reports();

// Forward declaration from narrative.cpp (Feature 013 wave 013-B / Lane 3:
// story pulse). UNLIKE ingest_combat_reports just above, this cursor DOES
// reset on reconnect -- emotions aren't an append-only id-cursored log the
// way reports are, so there is no cheap way to answer "what has this peer
// already seen" after a reconnect other than starting over; see
// reset_story_pulse_cursor's own doc comment (narrative.cpp) for how the
// bounded reconnect re-window keeps that restart from replaying the fort's
// entire emotional history.
void reset_story_pulse_cursor();

// Forward declarations from announcements.cpp
size_t poll_and_send_announcements();
void reset_announcement_cursor();

// Struct for queued commands (thread-safe command queue)
struct QueuedCommand {
    std::vector<uint8_t> payload;
};

// Global state
// g_socket has external linkage: announcements.cpp (and other translation
// units) reference it via `extern std::unique_ptr<CActiveSocket> g_socket;`.
std::unique_ptr<CActiveSocket> g_socket;
static std::string g_server_host = "localhost";
static uint16_t g_server_port = 5001;
static uint32_t g_connection_id = 0;
// Atomic: written by socket-thread teardown (close_socket_and_reset) with
// no core suspension held; read by the sim thread in plugin_onupdate.
static std::atomic<bool> g_connected{false};
static uint64_t g_last_heartbeat_ms = 0;  // Last time we received heartbeat from server
static uint32_t g_reconnect_delay_ms = 1000;  // Current reconnection backoff delay
static std::unique_ptr<std::thread> g_message_thread;  // Background message handler
static std::atomic<bool> g_stop_message_loop{false};  // Signal to stop message thread (set from console/main threads, read from socket thread)
static bool g_auto_update_enabled = false;  // Auto-update toggle flag
static uint32_t g_heartbeat_counter = 0;  // Count heartbeats for auto-update trigger

// THREAD SAFETY: Command queue for processing commands on main thread
// Background thread queues commands here, plugin_onupdate() processes them
static std::mutex g_command_queue_mutex;
static std::queue<QueuedCommand> g_command_queue;

// Query queue (parallel to command queue) — background thread queues
// QUERY messages here, plugin_onupdate() drains and runs handlers on main
// thread. Each entry carries the queryID, name, and args JSON.
struct QueuedQuery {
    uint32_t queryID;
    std::string name;
    std::string argsJSON;
};
static std::mutex g_query_queue_mutex;
static std::queue<QueuedQuery> g_query_queue;

// RESYNC flag — socket thread sets it, plugin_onupdate performs the
// full-state send on the main thread (extraction suspends DF anyway;
// doing it from the socket thread stalls DF mid-frame).
static std::atomic<bool> g_resync_requested{false};

// Step-mode target frame for PAUSE commands. -1 = not stepping. External
// linkage: queries.cpp reads it to answer sim_status.
// Set on the main thread (executeCommand); checked each plugin_onupdate.
std::atomic<int64_t> g_step_target_frame{-1};

// Step tripwire — a critical (severity-2) announcement observed mid-step
// ends the step immediately instead of letting the simulation run to the
// frame target (an ambush at tick 50 of a 1200-tick step must not go
// unanswered for the rest of a game-day). Set by trip_step_tripwire
// (called from the announcement poll in announcements.cpp), cleared when
// the next step starts; queries.cpp reads both to answer sim_status.
// Flag is atomic, reason string is mutex-guarded: writers run on the main
// thread (plugin_onupdate poll) or the socket thread under CoreSuspender
// (drain-driven polls); readers run in either drain context.
std::atomic<bool> g_step_tripwire{false};
static std::mutex g_tripwire_mutex;
static std::string g_tripwire_reason;

// Forward declarations
bool connect_to_server(color_ostream &out);
bool connect_with_retry(color_ostream &out, int max_attempts);
void disconnect_from_server();
bool send_handshake(color_ostream &out);
bool receive_handshake(color_ostream &out);
void message_receive_loop(color_ostream &out);
bool receive_message(std::vector<uint8_t> &msg_out, uint8_t &type_out);
bool send_full_state(color_ostream &out);
bool send_tile_update(color_ostream &out, const std::vector<uint8_t> &tiles);
bool send_heartbeat_echo(color_ostream &out, uint64_t timestamp, uint8_t sequence);
void handleCommand(const std::vector<uint8_t> &payload);  // From designations.cpp

// Helper: Read exactly n bytes from socket
bool read_exact(uint8_t *buffer, size_t n);

// Send mutex — serializes every frame write to g_socket (see protocol.h).
static std::mutex g_send_mutex;

int32_t socket_send_locked(const uint8_t *data, size_t len)
{
    std::lock_guard<std::mutex> lock(g_send_mutex);
    if (!g_socket) return -1;
    return g_socket->Send(data, len);
}

// Helper: Get current time in milliseconds
uint64_t get_time_ms()
{
    auto now = std::chrono::system_clock::now();
    auto duration = now.time_since_epoch();
    return std::chrono::duration_cast<std::chrono::milliseconds>(duration).count();
}

// External functions from other files
extern std::vector<uint8_t> extract_full_map_state();
extern bool get_map_dimensions(int32_t &width, int32_t &height, int32_t &depth);
extern std::vector<uint8_t> detect_tile_changes();

// Entity tracking - inline implementation
struct EntityInfo {
    uint32_t id;
    int16_t x, y, z;
    uint8_t type;
    uint16_t subtype;
};

const uint8_t ENTITY_TYPE_DWARF = 0x01;
const uint8_t ENTITY_TYPE_ENEMY = 0x02;
const uint8_t ENTITY_TYPE_ANIMAL = 0x03;
const uint8_t ENTITY_TYPE_OTHER = 0x04;

// Forward declarations - implementations in entities.cpp
// serialize_entity_update also appends the FortInfo calendar block
// (cur_year / cur_year_tick) — the single source of truth for fort info.
// dead_ids, when non-null, is appended with the ids of dead units found
// during extraction (Units::isDead()); pass the same vector into
// serialize_entity_update to include the DeadUnits wire block.
std::vector<EntityInfo> extract_entities(std::vector<uint32_t> *dead_ids);
std::vector<uint8_t> serialize_entity_update(const std::vector<EntityInfo> &entities,
                                              const std::vector<uint32_t> &dead_ids);

// Designation command handling (inline implementation)

// Send command acknowledgment back to server
void sendCommandAck(uint32_t cmdID, uint8_t status, const std::string &error)
{
    if (!g_socket || !g_socket->IsSocketValid()) return;

    std::vector<uint8_t> msg;
    msg.resize(4, 0);
    msg.push_back(PROTOCOL_VERSION);
    msg.push_back(0x0A);  // COMMAND_ACK

    // Command ID
    msg.push_back((cmdID >> 24) & 0xFF);
    msg.push_back((cmdID >> 16) & 0xFF);
    msg.push_back((cmdID >> 8) & 0xFF);
    msg.push_back(cmdID & 0xFF);

    // Status
    msg.push_back(status);

    // Error message
    uint16_t errorLen = error.empty() ? 0 : error.size();
    msg.push_back((errorLen >> 8) & 0xFF);
    msg.push_back(errorLen & 0xFF);
    if (errorLen > 0) {
        msg.insert(msg.end(), error.begin(), error.end());
    }

    // Fill length
    uint32_t length = msg.size();
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    socket_send_locked(msg.data(), msg.size());
}

// Send query response back to server.
// Payload: [4: queryID] [1: status] [4: dataLen] [N: data]
void sendQueryResponse(uint32_t queryID, uint8_t status, const std::string &dataJSON)
{
    if (!g_socket || !g_socket->IsSocketValid()) return;

    std::vector<uint8_t> msg;
    msg.resize(4, 0);
    msg.push_back(PROTOCOL_VERSION);
    msg.push_back(MSG_TYPE_QUERY_RESPONSE);

    msg.push_back((queryID >> 24) & 0xFF);
    msg.push_back((queryID >> 16) & 0xFF);
    msg.push_back((queryID >> 8) & 0xFF);
    msg.push_back(queryID & 0xFF);

    msg.push_back(status);

    uint32_t dataLen = (uint32_t)dataJSON.size();
    msg.push_back((dataLen >> 24) & 0xFF);
    msg.push_back((dataLen >> 16) & 0xFF);
    msg.push_back((dataLen >> 8) & 0xFF);
    msg.push_back(dataLen & 0xFF);
    if (dataLen > 0) {
        msg.insert(msg.end(), dataJSON.begin(), dataJSON.end());
    }

    uint32_t length = (uint32_t)msg.size();
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    socket_send_locked(msg.data(), msg.size());
}

// OLD IMPLEMENTATION - DEPRECATED
// Replaced by the version in designations.cpp which properly handles DigType parsing
// and uses the correct payload format (includes DigType byte at offset 5)
/*
bool applyDigDesignation(uint8_t digType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    // This old implementation is no longer used
    // See designations.cpp for the current implementation
    return false;
}
*/

// Unsuspend any building or job at the given coordinate. Used to resume
// auto-suspended constructions when the agent has cleared the underlying
// blocker (path, materials, etc.).
//
// IMPORTANT: The "suspended" flag lives on the job, not the building, in
// recent DF versions. We iterate the building's jobs (or the world's job
// list filtered by position) and clear suspend on each. The exact field
// name and storage may have shifted in DF 53.12.
bool applyUnsuspend(int16_t x, int16_t y, int16_t z, std::string &error)
{
    using namespace DFHack;

    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::coord pos(x, y, z);
    df::building* bld = Buildings::findAtTile(pos);
    if (!bld) {
        error = "No building at coordinates";
        return false;
    }

    int cleared = 0;
    // Walk the building's jobs and clear the suspend flag on each.
    // VERIFY: building's job list field name in DF 53.12 (jobs vs
    // job_list). Also verify df::job::flags.bits.suspend is the correct
    // bit name (some versions use 'do_now' inversion or a different bit).
    for (auto* job : bld->jobs) {
        if (!job) continue;
        if (job->flags.bits.suspend) {
            job->flags.bits.suspend = 0;
            cleared++;
        }
    }

    if (cleared == 0) {
        error = "No suspended jobs at this building";
        return false;
    }
    return true;
}

// Remove the building (or unbuilt building plan) at the given coordinate.
// Buildings::deconstruct returns true when the building was destroyed
// instantly (an unbuilt plan with no construction progress, or an abstract
// building like a stockpile/civzone); false means a deconstruction job was
// queued and dwarves will dismantle it over time.
// Both outcomes are success — the ACK text distinguishes them so the model
// knows whether to expect a delay. Rationale from live play: a dead
// building plan (e.g. a wall plan on a stair tile) blocks the tile forever
// with no way to clear it.
// collectRemovalCandidates: every building remove_building could mean at
// one tile.
//
// findAtTile only ever returns occupancy-setting ("real") buildings --
// Stockpile is abstract (isSettingOccupancy()==false, same bucket as
// Civzone) and is architecturally invisible to it, even when the
// stockpile's designated rectangle shares this tile with a real building's
// footprint (an ordinary layout: a stockpile ring around an enclosed
// workshop). Civzone overlap is deliberately left out of this scan --
// remove_zone is the correct, already-working tool for that case, and
// stockpile has no equivalent removal command of its own.
//
// Split into two halves so a caller testing MANY tiles at one z pays the
// world->buildings.all walk ONCE instead of once per tile (see
// findUnambiguousTileFor): the stockpile set at a given z is invariant
// across such a loop, while findAtTile/containsTile are per-tile by nature.
// The membership rule itself still has exactly one definition.
static void collectStockpilesAtZ(int32_t z, std::vector<df::building*> &out)
{
    out.clear();
    if (!df::global::world) return;
    for (auto *b : df::global::world->buildings.all) {
        if (!b || b->getType() != df::building_type::Stockpile) continue;
        if (b->z != z) continue;
        out.push_back(b);
    }
}

static void collectRemovalCandidatesFrom(df::coord pos,
                                         const std::vector<df::building*> &stockpilesAtZ,
                                         std::vector<df::building*> &out)
{
    using namespace DFHack;
    out.clear();
    if (df::building *real = Buildings::findAtTile(pos)) {
        out.push_back(real);
    }
    df::coord2d pos2d(pos.x, pos.y);
    for (auto *b : stockpilesAtZ) {
        if (!Buildings::containsTile(b, pos2d)) continue;
        out.push_back(b);
    }
}

static void collectRemovalCandidates(df::coord pos, std::vector<df::building*> &out)
{
    std::vector<df::building*> stockpilesAtZ;
    collectStockpilesAtZ(pos.z, stockpilesAtZ);
    collectRemovalCandidatesFrom(pos, stockpilesAtZ, out);
}

// describeRemovalCandidate: enough identity to tell two overlapping
// buildings apart in the ACK, cross-referenceable against the `buildings`
// tool's own stockpile lines (which carry the same #number and name).
// Bounding rectangles alone are NOT identity -- two stockpiles in a live
// fort shared the exact rectangle (88,88)-(92,91), which made the previous
// ACK's advice ("reissue at a tile covered by only the one you want")
// unfollowable: the two candidates were textually indistinguishable.
static std::string describeRemovalCandidate(df::building *b)
{
    using namespace DFHack;
    std::ostringstream os;
    os << ENUM_KEY_STR(building_type, b->getType());
    if (b->getType() == df::building_type::Stockpile) {
        if (auto *sp = strict_virtual_cast<df::building_stockpilest>(b))
            os << " #" << sp->stockpile_number;
    }
    if (!b->name.empty()) os << " \"" << b->name << "\"";
    os << " id=" << b->id
       << " spanning (" << b->x1 << "," << b->y1 << ")-(" << b->x2 << "," << b->y2 << ")";
    int bboxArea = (b->x2 - b->x1 + 1) * (b->y2 - b->y1 + 1);
    os << ", " << Buildings::countExtentTiles(b, bboxArea) << " tiles";
    return os.str();
}

// findUnambiguousTileFor: a tile inside target's own footprint where
// collectRemovalCandidates resolves to target ALONE -- i.e. a coordinate
// the caller can reissue remove_building at and hit exactly what it means.
// Returns false when no such tile exists (fully-enclosed or
// identically-shaped overlaps), which is itself the answer the caller
// needs: "there is no disambiguating tile, stop looking for one."
//
// Cost is the target's bbox area x the candidate scan, on an error path
// only -- never on a success path or a query. The z's stockpile list is
// hoisted out of the tile loop for that scan: collectRemovalCandidates
// walks world->buildings.all on every call, so a fort-wide 40x40 pile in a
// 1000-building fort would otherwise run millions of iterations on the main
// thread under CoreSuspender for one error message.
static bool findUnambiguousTileFor(df::building *target, int16_t &ux, int16_t &uy)
{
    using namespace DFHack;
    // df::building's x1/y1/x2/y2/z are int32_t; tile coords are int16_t.
    std::vector<df::building*> stockpilesAtZ;
    collectStockpilesAtZ(target->z, stockpilesAtZ);
    std::vector<df::building*> cands;
    for (int16_t ty = (int16_t)target->y1; ty <= (int16_t)target->y2; ty++) {
        for (int16_t tx = (int16_t)target->x1; tx <= (int16_t)target->x2; tx++) {
            if (!Buildings::containsTile(target, df::coord2d(tx, ty))) continue;
            collectRemovalCandidatesFrom(df::coord(tx, ty, (int16_t)target->z), stockpilesAtZ, cands);
            if (cands.size() == 1 && cands[0] == target) {
                ux = tx;
                uy = ty;
                return true;
            }
        }
    }
    return false;
}

bool applyRemoveBuilding(int16_t x, int16_t y, int16_t z, std::string &error)
{
    using namespace DFHack;

    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    df::coord pos(x, y, z);

    std::vector<df::building*> candidates;
    collectRemovalCandidates(pos, candidates);

    if (candidates.empty()) {
        std::ostringstream os;
        os << "no building at (" << x << "," << y << "," << z << ")";
        error = os.str();
        return false;
    }

    if (candidates.size() > 1) {
        // remove_building selects by TILE, so an ambiguous coordinate can
        // only be resolved by naming a different one. Do that arithmetic
        // here instead of telling the caller to go find such a tile
        // itself: the previous ACK's "reissue at a tile covered by only
        // the one you want" was unfollowable in a live fort where two
        // stockpiles shared an identical rectangle -- the two candidates
        // printed as the same text and neither had a stated unique tile.
        std::ostringstream os;
        os << candidates.size() << " buildings overlap (" << x << "," << y << "," << z
           << ") and remove_building selects by tile -- pick the tile that means the one you want:";
        bool hasStockpile = false;
        bool hasRealBuilding = false;
        bool anyUnresolvable = false;
        for (size_t i = 0; i < candidates.size(); i++) {
            df::building *b = candidates[i];
            os << " [" << (i + 1) << "] " << describeRemovalCandidate(b);
            int16_t ux = 0, uy = 0;
            if (findUnambiguousTileFor(b, ux, uy)) {
                os << " -- reissue at (" << ux << "," << uy << "," << z << ") to remove ONLY this one;";
            } else {
                os << " -- NO tile in its footprint belongs to it alone;";
                anyUnresolvable = true;
            }
            if (b->getType() == df::building_type::Stockpile) hasStockpile = true;
            else hasRealBuilding = true;
        }
        if (anyUnresolvable && hasStockpile && hasRealBuilding) {
            os << " A stockpile's rectangle has no hole punched out for buildings inside it, so an enclosed "
                  "building can be ambiguous on every one of its tiles. To remove the stockpile: accept that "
                  "removing it deletes its ENTIRE designated rectangle as collateral. To remove the real building "
                  "instead: remove the overlapping stockpile first (again, its whole rectangle goes, not just the "
                  "shared tiles), then reissue remove_building at this same coordinate to reach the now-unambiguous "
                  "building.";
        } else if (anyUnresolvable) {
            os << " A candidate with no tile of its own is fully covered by the others (identical or enclosing "
                  "footprints); remove an overlapping one first, then reissue here.";
        }
        error = os.str();
        return false;
    }

    df::building *bld = candidates[0];
    if (Buildings::deconstruct(bld)) {
        error = "removed instantly (no deconstruction labor needed)";
    } else {
        error = "deconstruction queued (dwarves will dismantle)";
    }
    return true;
}

// Enable or disable one labor on a unit.
//
// df::unit::status.labors is a PLAIN FIXED-SIZE C ARRAY of bool
// (df/unit.h:374-386: `bool labors[enum_traits<unit_labor>::last_item_value+1]`,
// size 94 per df/unit_labor.h:122) indexed directly by the unit_labor enum
// value — not std::vector<bool>, not a bitset. The write itself is a plain
// array assignment with NO CHECK_* macro guarding it (confirmed against
// autolabor.cpp:622-623,690-691 and labormanager.cpp:672-673, DFHack's own
// read/write sites for this field). Units::isValidLabor
// (modules/Units.cpp:1379-1386) is an OPTIONAL advisory helper checking
// entity_raw->jobs.permitted_labor — a civ-wide "is this labor allowed at
// all" bitset, not a per-caste physical-capability check — and is not
// enforced by the engine; manipulator.cpp:729-742 (DFHack's own bulk
// labor-profile applier) writes status.labors for every labor with zero
// call to isValidLabor. The one real hazard is out-of-bounds indexing
// (undefined behavior on a raw C array): bounds-check laborID before the
// write, mirroring labormanager.cpp:662's guard.
//
// Setting a labor a unit's caste can't perform is a graceful no-op — DF's
// own job-assignment AI simply never generates/matches work for that unit
// on that labor (same "silently skips invalid targets" principle already
// documented at designations.cpp:426-428 for dig designations). No error,
// no crash, just a toggle that never fires a job.
//
// IMPORTANT: status.labors is a DERIVED CACHE, not the source of truth --
// DF recomputes it from df::global::plotinfo->labor_info.work_details
// (the work_detail vector; see work_details.cpp) via
// Units::setAutomaticProfessions(unit) whenever a work detail's membership
// or mode changes, and that recompute will silently overwrite a labor this
// function just set. The work-detail commands (ASSIGN_WORK_DETAIL /
// SET_WORK_DETAIL_MODE / CREATE_WORK_DETAIL, work_details.cpp) are the
// AUTHORITATIVE path for durable labor assignment; this direct write is a
// one-off toggle that a later work-detail recompute can clobber.
bool applySetLabor(int32_t unitID, uint8_t laborID, bool enable, std::string &error)
{
    using namespace DFHack;

    if (laborID > LABOR_MAX_INDEX) {
        char buf[80];
        snprintf(buf, sizeof(buf), "labor id %u out of range (0-%u)", (unsigned)laborID, (unsigned)LABOR_MAX_INDEX);
        error = buf;
        return false;
    }

    // df::unit::find binary-searches world->units.all by id
    // (library/include/df/static.inc:1449-1456 — generated code, same
    // pattern as df::building::find used elsewhere in DFHack: Buildings.cpp,
    // Military.cpp, Gui.cpp, EventManager.cpp, Burrows.cpp). Returns nullptr
    // on a lookup miss OR a missing world, never throws — no try/catch
    // needed around this call specifically, though it still runs inside
    // executeCommand's outer guard.
    df::unit *u = df::unit::find(unitID);
    if (!u) {
        char buf[64];
        snprintf(buf, sizeof(buf), "no unit with id %d", unitID);
        error = buf;
        return false;
    }

    u->status.labors[laborID] = enable;

    // Leave `error` untouched on the success path — executeCommand sends it
    // verbatim as the ACK's error text alongside ACK_STATUS_SUCCESS, and
    // ackText() (internal/mcpserver/tools_action.go) categorizes any
    // Success==true ACK with a non-empty ErrorMsg as PARTIAL, not SUCCESS
    // (mirrors applyQueueJob's convention in work_orders.cpp, which returns
    // true with `error` untouched on its success path). The Go-side tool
    // already builds its own descriptive "set_labor <labor>=<bool> for
    // dwarf id=<id>" text from the request, so no confirmation string is
    // lost by staying silent here.
    return true;
}

// Apply cancel designation to region
bool applyCancelDesignation(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Invalid coordinates";
        return false;
    }

    int cancelled = 0;
    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::map_block *block = Maps::getTileBlock(x, y, z);
            if (block) {
                block->designation[x%16][y%16].bits.dig = df::tile_dig_designation::No;
                cancelled++;
            }
        }
    }

    return true;
}

// CHOP / GATHER live in plants.cpp: direct plant designations via
// DFHack::Designations::markPlant (the old console shell-outs here were
// broken — "chop-designate" never existed, "getplants all" ignored the
// region).

// Apply blueprint using DFHack's quickfort command
// IMPORTANT: This uses DFHack's native quickfort plugin instead of custom CSV parsing
// Benefits:
// - Supports full quickfort syntax (dig, build, place, zone, query modes)
// - Handles bedroom/dining zone creation automatically
// - Supports buildings, stockpiles, and other advanced features
// - Uses community-tested blueprints from library-blueprints repository
bool applyBlueprintDesignation(const std::string& blueprintName, int16_t originX, int16_t originY, int16_t originZ, std::string& error) {
    // DISABLED: Has CoreSuspender + runCommand deadlock issues
    // TODO: Fix blueprint execution after resolving thread safety
    error = "Blueprint command temporarily disabled due to thread safety issues";
    return false;
}

// push_state_refresh sends the full turn-boundary refresh bundle:
// ENTITY_UPDATE, tile deltas (only if non-empty), and any new
// announcements. Called whenever the simulation stops at a turn boundary —
// step auto-repause and explicit PAUSE commands — so the model's world
// state is re-grounded instead of frozen at connect time (nothing else
// pushes updates in the turn-based flow; auto-update is off by default).
//
// THREAD SAFETY: runs wherever drain_pending_work runs — plugin_onupdate
// (simulation thread) or the socket thread under drain_from_socket_thread's
// suspension. Both are safe for DF reads; extract_entities and
// detect_tile_changes additionally take their own (recursive) CoreSuspender.
// All sends go through socket_send_locked.
static void push_state_refresh()
{
    if (!g_connected || !g_socket || !g_socket->IsSocketValid()) {
        return;
    }

    color_ostream_proxy out(Core::getInstance().getConsole());

    // Tile deltas first (matches the auto-update ordering), only if any.
    std::vector<uint8_t> tile_changes = detect_tile_changes();
    if (!tile_changes.empty()) {
        send_tile_update(out, tile_changes);
    }

    // Entities (dwarf positions/jobs move every step).
    std::vector<uint32_t> dead_ids;
    std::vector<EntityInfo> entities = extract_entities(&dead_ids);
    std::vector<uint8_t> entity_msg = serialize_entity_update(entities, dead_ids);
    socket_send_locked(entity_msg.data(), entity_msg.size());

    // Announcements since the last poll.
    poll_and_send_announcements();

    // Combat narrator ingest (Feature 013 wave 013-A / Lane 2): same
    // step-boundary cadence as the announcement poll just above, so wound
    // watermarks snapshot/expire at the same turn boundaries the model
    // actually observes from. Wire-silent -- combat_summary/combat_log are
    // pull queries, not pushed here.
    ingest_combat_reports();
}

// get_step_tripwire_reason returns a copy of the last tripwire trigger
// text under the guard mutex. queries.cpp calls this for sim_status.
std::string get_step_tripwire_reason()
{
    std::lock_guard<std::mutex> lock(g_tripwire_mutex);
    return g_tripwire_reason;
}

// clear_step_tripwire resets the tripwire record. Called when a new step
// starts — the flag deliberately persists across the intervening pause so
// the Go step tool's sim_status completion poll cannot miss it.
static void clear_step_tripwire()
{
    std::lock_guard<std::mutex> lock(g_tripwire_mutex);
    g_step_tripwire = false;
    g_tripwire_reason.clear();
}

// trip_step_tripwire ends an in-progress step because a critical
// announcement arrived. The exchange on g_step_target_frame atomically
// claims the step: if no step is running (or another poll site already
// tripped), this is a no-op — the ~5s onupdate poll and drain-driven
// polls can never double-trip. External linkage: called from
// poll_and_send_announcements (announcements.cpp).
//
// THREAD SAFETY: runs wherever the announcement poll runs — the main
// thread (plugin_onupdate) or the socket thread under
// drain_from_socket_thread's suspension; both are safe for SetPauseState
// and the state push. The announcement cursor has already advanced past
// the triggering report when this is called, so the nested
// poll_and_send_announcements inside push_state_refresh finds nothing new
// (no recursion risk).
void trip_step_tripwire(const std::string &reason)
{
    if (g_step_target_frame.exchange(-1) < 0) {
        return;  // no step in progress
    }
    {
        std::lock_guard<std::mutex> lock(g_tripwire_mutex);
        g_step_tripwire = true;
        g_tripwire_reason = reason;
    }
    // Exception barrier: the throttled plugin_onupdate poll reaches here
    // OUTSIDE drain_pending_work's guard, and push_state_refresh's state
    // extraction can hit DFHack CHECK macros (which THROW). An escaping
    // throw would unwind into Core::Update and terminate DF — mirror the
    // drain_pending_work barrier so every trip site, current and future,
    // is guarded regardless of caller.
    try {
        World::SetPauseState(true);
        // A tripped step is a turn boundary like any other step end:
        // re-ground the model with a fresh entity/tile/announcement bundle.
        push_state_refresh();
    } catch (std::exception &e) {
        color_ostream_proxy out(Core::getInstance().getConsole());
        out.printerr("df_ai_protocol: exception in trip_step_tripwire: %s\n", e.what());
    } catch (...) {
        color_ostream_proxy out(Core::getInstance().getConsole());
        out.printerr("df_ai_protocol: unknown exception in trip_step_tripwire\n");
    }
}

// Execute COMMAND on main thread - called from plugin_onupdate()
// THREAD SAFETY: This function MUST ONLY be called from the main DF thread
void executeCommand(const std::vector<uint8_t> &payload)
{
    if (payload.size() < 5) {
        sendCommandAck(0, 0x02, "Invalid command payload");
        return;
    }

    // CRITICAL: Check if map is loaded before accessing DF data
    if (!Core::getInstance().isMapLoaded() || !df::global::world) {
        uint32_t cmdID = ((uint32_t)payload[0] << 24) | ((uint32_t)payload[1] << 16) |
                         ((uint32_t)payload[2] << 8) | (uint32_t)payload[3];
        sendCommandAck(cmdID, 0x02, "Map not loaded - cannot execute command");
        return;
    }

    // NOTE: CoreSuspender removed - MapCache handles thread safety internally
    // Having both CoreSuspender + MapCache causes deadlocks

    // Parse command ID and type
    uint32_t cmdID = ((uint32_t)payload[0] << 24) | ((uint32_t)payload[1] << 16) |
                     ((uint32_t)payload[2] << 8) | (uint32_t)payload[3];
    uint8_t cmdType = payload[4];

    std::string error;
    bool success = false;

    // Defense-in-depth: DFHack module APIs validate preconditions with
    // CHECK_NULL_POINTER / CHECK_INVALID_ARGUMENT macros that THROW C++
    // exceptions (dfhack library/include/Error.h:74-84). DFHack's own
    // consumers always invoke them under a catch wrapper (the Lua bindings,
    // LuaTypes.cpp:1249-1258); an exception that escapes a plugin thread
    // calls std::terminate and kills DF instantly with no trace. That is
    // exactly what happened live on 2026-07-12 when constructAbstract threw
    // InvalidArgument for a workshop BUILD. Convert any such exception into
    // a command NACK instead.
    try {
    switch (cmdType) {
        case 0x01: {  // DIG
            // Call the fixed version from designations.cpp (not the old one below)
            // FOLLOW-UP CLEANUP: the commented-out `applyDigDesignation(uint8_t
            // digType, ...)` overload further down this file is dead and only
            // exists to confuse a future reader -- delete it in a wave that is
            // already touching this file.
            bool digDestroysStairs = false;
            success = applyDigDesignation(payload, error, digDestroysStairs);
            // Same short-circuit shape as the STOCKPILE case below: the
            // generic tail only ever sends SUCCESS or FAILURE
            // (success ? 0x00 : 0x02), so a dig that succeeded WHILE removing
            // existing carved stairs (vertical connection severed -- the
            // Fort #4 incident) would otherwise report a clean SUCCESS and
            // bury its own warning text under it. The message is unchanged;
            // only the status byte differs.
            if (success && digDestroysStairs) {
                sendCommandAck(cmdID, ACK_STATUS_PARTIAL, error);
                return;
            }
            break;
        }
        case 0x02: {  // BUILD
            success = applyBuildDesignation(payload, error);
            break;
        }
        case 0x03: {  // CANCEL
            // Call the fixed version from designations.cpp
            success = applyCancelDesignation(payload, error);
            break;
        }
        case 0x04: {  // CHOP
            if (payload.size() < 17) {
                sendCommandAck(cmdID, 0x02, "Invalid CHOP payload");
                return;
            }
            int16_t x1 = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y1 = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z1 = ((int16_t)payload[9] << 8) | payload[10];
            int16_t x2 = ((int16_t)payload[11] << 8) | payload[12];
            int16_t y2 = ((int16_t)payload[13] << 8) | payload[14];
            int16_t z2 = ((int16_t)payload[15] << 8) | payload[16];
            success = applyChopDesignation(x1, y1, z1, x2, y2, z2, error);
            break;
        }
        case 0x05: {  // GATHER
            if (payload.size() < 17) {
                sendCommandAck(cmdID, 0x02, "Invalid GATHER payload");
                return;
            }
            int16_t x1 = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y1 = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z1 = ((int16_t)payload[9] << 8) | payload[10];
            int16_t x2 = ((int16_t)payload[11] << 8) | payload[12];
            int16_t y2 = ((int16_t)payload[13] << 8) | payload[14];
            int16_t z2 = ((int16_t)payload[15] << 8) | payload[16];
            success = applyGatherDesignation(x1, y1, z1, x2, y2, z2, error);
            break;
        }
        case 0x06: {  // ZONE (designate)
            if (payload.size() < 16) {
                sendCommandAck(cmdID, 0x02, "Invalid ZONE payload");
                return;
            }
            uint8_t zoneType = payload[5];
            int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
            int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
            int16_t z = ((int16_t)payload[10] << 8) | payload[11];
            int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
            int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];
            success = applyDesignateZone(zoneType, x1, y1, z, x2, y2, error);
            break;
        }
        case 0x07: {  // BLUEPRINT
            if (payload.size() < 13) {  // Minimum: NameLen(2) + Name(1+) + Origin(6)
                sendCommandAck(cmdID, 0x02, "Invalid BLUEPRINT payload");
                return;
            }

            // Parse blueprint name length
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 13 + nameLen) {
                sendCommandAck(cmdID, 0x02, "Invalid BLUEPRINT payload size");
                return;
            }

            // Extract blueprint name
            std::string blueprintName(payload.begin() + 7, payload.begin() + 7 + nameLen);

            // Parse origin coordinates
            size_t offset = 7 + nameLen;
            int16_t originX = ((int16_t)payload[offset] << 8) | payload[offset + 1];
            int16_t originY = ((int16_t)payload[offset + 2] << 8) | payload[offset + 3];
            int16_t originZ = ((int16_t)payload[offset + 4] << 8) | payload[offset + 5];

            success = applyBlueprintDesignation(blueprintName, originX, originY, originZ, error);
            break;
        }
        case 0x08: {  // UNSUSPEND
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, 0x02, "Invalid UNSUSPEND payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            success = applyUnsuspend(x, y, z, error);
            break;
        }
        case 0x09: {  // WORK_ORDER
            // Payload: [4: cmdID] [1: cmdType] [1: OrderType] [2: Quantity]
            //          [2: NameLen][N: Name]  -- the name tail is present
            //          ONLY when OrderType == ORDER_TYPE_BY_NAME (0x00).
            //          Mirrors QUEUE_JOB's (case COMMAND_TYPE_QUEUE_JOB
            //          below) trailing length-prefixed name, just appended
            //          after Quantity instead of after coordinates (this
            //          command has none). Old 8-byte payloads (OrderType
            //          0x01-0x0C) parse exactly as before -- purely
            //          additive. ORDER_TYPE_CUSTOM_REACTION is deliberately
            //          NOT accepted here -- see work_orders.cpp
            //          applyWorkOrder's doc comment.
            //
            //          2026-07-19 manager-work-order fix wave: two more
            //          trailing fields, ALWAYS appended by the Go encoder
            //          after the (conditional) name tail --
            //          [2:MaterialLen][N:Material] then [1:Frequency]. A
            //          payload ending before either is present (old
            //          pre-this-change encodings) decodes as Material=""
            //          and Frequency=WORK_ORDER_FREQUENCY_ONE_TIME --
            //          mirrors internal/protocol/codec.go's EOF-tolerant
            //          decode of the same two fields.
            //
            //          Item-SUBTYPE pinning wave: one more trailing field,
            //          ALWAYS appended by the Go encoder after Frequency --
            //          [2:SubtypeLen][N:Subtype]. A payload ending before it
            //          is present decodes as Subtype="" -- same EOF-tolerant
            //          backward-compat rule as Material/Frequency above.
            if (payload.size() < 8) {
                sendCommandAck(cmdID, 0x02, "Invalid WORK_ORDER payload");
                return;
            }
            uint8_t orderType = payload[5];
            uint16_t quantity = ((uint16_t)payload[6] << 8) | payload[7];
            if (quantity == 0 || quantity > 100) {
                sendCommandAck(cmdID, 0x02, "WORK_ORDER quantity out of range (1-100)");
                return;
            }
            std::string jobTypeName;
            size_t offset = 8;
            if (orderType == ORDER_TYPE_BY_NAME) {
                if (payload.size() < offset + 2) {
                    sendCommandAck(cmdID, 0x02, "Invalid WORK_ORDER by-name payload");
                    return;
                }
                uint16_t nameLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
                offset += 2;
                if (payload.size() < offset + nameLen) {
                    sendCommandAck(cmdID, 0x02, "Invalid WORK_ORDER by-name payload size");
                    return;
                }
                jobTypeName.assign(payload.begin() + offset, payload.begin() + offset + nameLen);
                offset += nameLen;
            }
            std::string material;
            uint8_t frequencyByte = WORK_ORDER_FREQUENCY_ONE_TIME;
            std::string subtype;
            if (payload.size() >= offset + 2) {
                uint16_t materialLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
                size_t matStart = offset + 2;
                if (payload.size() >= matStart + materialLen) {
                    material.assign(payload.begin() + matStart, payload.begin() + matStart + materialLen);
                    size_t freqOffset = matStart + materialLen;
                    if (payload.size() >= freqOffset + 1) {
                        frequencyByte = payload[freqOffset];
                        size_t subOffset = freqOffset + 1;
                        if (payload.size() >= subOffset + 2) {
                            uint16_t subtypeLen = ((uint16_t)payload[subOffset] << 8) | payload[subOffset + 1];
                            size_t subStart = subOffset + 2;
                            if (payload.size() >= subStart + subtypeLen) {
                                subtype.assign(payload.begin() + subStart, payload.begin() + subStart + subtypeLen);
                            }
                        }
                    }
                }
            }
            success = applyWorkOrder(orderType, quantity, jobTypeName, material, frequencyByte, subtype, error);
            break;
        }
        case 0x0B: {  // SMOOTH
            success = applySmoothDesignation(payload, error);
            break;
        }
        case 0x0A: {  // STOCKPILE
            // Payload: [4: cmdID] [1: cmdType] [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2] [4: GroupMask]
            if (payload.size() < 19) {
                sendCommandAck(cmdID, 0x02, "Invalid STOCKPILE payload");
                return;
            }
            int16_t x1 = ((int16_t)payload[5]  << 8) | payload[6];
            int16_t y1 = ((int16_t)payload[7]  << 8) | payload[8];
            int16_t z  = ((int16_t)payload[9]  << 8) | payload[10];
            int16_t x2 = ((int16_t)payload[11] << 8) | payload[12];
            int16_t y2 = ((int16_t)payload[13] << 8) | payload[14];
            uint32_t mask =
                ((uint32_t)payload[15] << 24) | ((uint32_t)payload[16] << 16) |
                ((uint32_t)payload[17] << 8)  |  (uint32_t)payload[18];
            bool partial = false;
            success = placeStockpile(x1, y1, z, x2, y2, mask, error, partial);
            // The generic tail below only ever sends SUCCESS or FAILURE
            // (success ? 0x00 : 0x02) — a preset-import failure still
            // leaves success==true (the stockpile IS placed and usable),
            // so it must short-circuit here to send ACK_STATUS_PARTIAL
            // instead of falling through to a false "clean" SUCCESS.
            if (success && partial) {
                sendCommandAck(cmdID, ACK_STATUS_PARTIAL, error);
                return;
            }
            break;
        }
        case COMMAND_TYPE_PAUSE: {
            // Payload: [4 cmdID][1 cmdType][1 mode][4 ticks]
            if (payload.size() < 10) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid pause payload size");
                return;
            }
            uint8_t mode = payload[5];
            uint32_t ticks = read_uint32_be(payload, 6);
            if (mode == 0x01) {              // pause
                World::SetPauseState(true);
                g_step_target_frame = -1;
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "");
                // Manual pause is a turn boundary: re-ground the model
                // with a fresh entity/tile/announcement bundle.
                push_state_refresh();
            } else if (mode == 0x00) {       // unpause
                World::SetPauseState(false);
                g_step_target_frame = -1;
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "");
            } else if (mode == 0x02) {       // step
                // A new step clears the previous tripwire record — the
                // model has had its chance to read it via sim_status.
                clear_step_tripwire();
                g_step_target_frame = (int64_t)df::global::world->frame_counter + ticks;
                World::SetPauseState(false);
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "step started");
            } else {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Unknown pause mode");
            }
            return;
        }
        case COMMAND_TYPE_REMOVE_BUILDING: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid REMOVE_BUILDING payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            success = applyRemoveBuilding(x, y, z, error);
            break;
        }
        case COMMAND_TYPE_QUEUE_JOB: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [1: OrderType]
            //          [2: NameLen][N: Name]  -- the name tail is present
            //          ONLY when OrderType == ORDER_TYPE_BY_NAME (0x00) or
            //          ORDER_TYPE_CUSTOM_REACTION (0x0D). Mirrors
            //          BLUEPRINT's (case 0x07 above) trailing
            //          length-prefixed name. Old 12-byte payloads
            //          (OrderType 0x01-0x0C) parse exactly as before —
            //          this is purely additive.
            //
            //          2026-07-19 manager-work-order fix wave (SmeltOre
            //          direct-queue unlock): one more trailing field,
            //          ALWAYS appended by the Go encoder after the
            //          (conditional) name/reaction tail --
            //          [2:MaterialLen][N:Material]. A payload ending before
            //          it is present (old pre-this-change encodings)
            //          decodes as Material="" -- mirrors internal/protocol/
            //          codec.go's EOF-tolerant decode of the same field.
            //
            //          Item-SUBTYPE pinning wave: one more trailing field,
            //          ALWAYS appended by the Go encoder after Material --
            //          [2:SubtypeLen][N:Subtype]. A payload ending before it
            //          is present decodes as Subtype="" -- same EOF-tolerant
            //          backward-compat rule as Material above.
            if (payload.size() < 12) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid QUEUE_JOB payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            uint8_t orderType = payload[11];
            std::string jobTypeName;
            size_t offset = 12;
            if (orderType == ORDER_TYPE_BY_NAME || orderType == ORDER_TYPE_CUSTOM_REACTION) {
                if (payload.size() < offset + 2) {  // 12 existing + NameLen(2)
                    sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid QUEUE_JOB by-name payload");
                    return;
                }
                uint16_t nameLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
                offset += 2;
                if (payload.size() < offset + nameLen) {
                    sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid QUEUE_JOB by-name payload size");
                    return;
                }
                jobTypeName.assign(payload.begin() + offset, payload.begin() + offset + nameLen);
                offset += nameLen;
            }
            std::string material;
            std::string subtype;
            if (payload.size() >= offset + 2) {
                uint16_t materialLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
                size_t matStart = offset + 2;
                if (payload.size() >= matStart + materialLen) {
                    material.assign(payload.begin() + matStart, payload.begin() + matStart + materialLen);
                    size_t subOffset = matStart + materialLen;
                    if (payload.size() >= subOffset + 2) {
                        uint16_t subtypeLen = ((uint16_t)payload[subOffset] << 8) | payload[subOffset + 1];
                        size_t subStart = subOffset + 2;
                        if (payload.size() >= subStart + subtypeLen) {
                            subtype.assign(payload.begin() + subStart, payload.begin() + subStart + subtypeLen);
                        }
                    }
                }
            }
            success = applyQueueJob(x, y, z, orderType, jobTypeName, material, subtype, error);
            break;
        }
        case COMMAND_TYPE_SET_LABOR: {
            // Payload: [4: cmdID] [1: cmdType] [4: UnitID] [1: LaborID] [1: Enable]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_LABOR payload");
                return;
            }
            int32_t unitID = (int32_t)read_uint32_be(payload, 5);
            uint8_t laborID = payload[9];
            bool enable = payload[10] != 0;
            success = applySetLabor(unitID, laborID, enable, error);
            break;
        }
        case COMMAND_TYPE_ASSIGN_ZONE: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [4: UnitID]
            if (payload.size() < 15) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_ZONE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            int32_t unitID = (int32_t)read_uint32_be(payload, 11);
            success = applyAssignZone(x, y, z, unitID, error);
            break;
        }
        case COMMAND_TYPE_UNASSIGN_ZONE: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [4: UnitID]
            if (payload.size() < 15) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNASSIGN_ZONE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            int32_t unitID = (int32_t)read_uint32_be(payload, 11);
            success = applyUnassignZone(x, y, z, unitID, error);
            break;
        }
        case COMMAND_TYPE_REMOVE_ZONE: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid REMOVE_ZONE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            success = applyRemoveZone(x, y, z, error);
            break;
        }
        case COMMAND_TYPE_CREATE_LOCATION: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z][1:LocationType][2:ProfessionLen][N:ProfessionName]
            // (the trailing length-prefixed name mirrors QUEUE_JOB's existing by-name trailing field)
            if (payload.size() < 12) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_LOCATION payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            uint8_t locationType = payload[11];
            std::string profession;
            if (payload.size() >= 14) {
                uint16_t nameLen = ((uint16_t)payload[12] << 8) | payload[13];
                if (payload.size() >= 14 + nameLen) {
                    profession.assign(payload.begin() + 14, payload.begin() + 14 + nameLen);
                }
            }
            success = applyCreateLocation(x, y, z, locationType, profession, error);
            break;
        }
        case COMMAND_TYPE_ASSIGN_LODGING: {
            // Payload: [4: cmdID] [1: cmdType] [2: TavernX] [2: TavernY] [2: TavernZ] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
            if (payload.size() < 17) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_LODGING payload");
                return;
            }
            int16_t tx = ((int16_t)payload[5] << 8) | payload[6];
            int16_t ty = ((int16_t)payload[7] << 8) | payload[8];
            int16_t tz = ((int16_t)payload[9] << 8) | payload[10];
            int16_t bx = ((int16_t)payload[11] << 8) | payload[12];
            int16_t by = ((int16_t)payload[13] << 8) | payload[14];
            int16_t bz = ((int16_t)payload[15] << 8) | payload[16];
            success = applyAssignLodging(tx, ty, tz, bx, by, bz, error);
            break;
        }
        case COMMAND_TYPE_UNASSIGN_LODGING: {
            // Payload: [4: cmdID] [1: cmdType] [2: BedroomX] [2: BedroomY] [2: BedroomZ]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNASSIGN_LODGING payload");
                return;
            }
            int16_t bx = ((int16_t)payload[5] << 8) | payload[6];
            int16_t by = ((int16_t)payload[7] << 8) | payload[8];
            int16_t bz = ((int16_t)payload[9] << 8) | payload[10];
            success = applyUnassignLodging(bx, by, bz, error);
            break;
        }
        case COMMAND_TYPE_BUILD_FARM_PLOT: {
            // Payload: [4: cmdID] [1: cmdType] [2: X1] [2: Y1] [2: Z] [2: X2] [2: Y2]
            // Byte-identical to STOCKPILE minus the trailing GroupMask.
            if (payload.size() < 15) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid BUILD_FARM_PLOT payload");
                return;
            }
            int16_t x1 = ((int16_t)payload[5]  << 8) | payload[6];
            int16_t y1 = ((int16_t)payload[7]  << 8) | payload[8];
            int16_t z  = ((int16_t)payload[9]  << 8) | payload[10];
            int16_t x2 = ((int16_t)payload[11] << 8) | payload[12];
            int16_t y2 = ((int16_t)payload[13] << 8) | payload[14];
            success = placeFarmPlot(x1, y1, z, x2, y2, error);
            break;
        }
        case COMMAND_TYPE_SET_FARM_CROP: {
            // Payload: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [1: Season] [2: NameLen] [N: CropName]
            if (payload.size() < 14) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_FARM_CROP payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            uint8_t season = payload[11];
            uint16_t nameLen = ((uint16_t)payload[12] << 8) | payload[13];
            if (payload.size() < 14 + nameLen) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_FARM_CROP payload size");
                return;
            }
            std::string cropName(payload.begin() + 14, payload.begin() + 14 + nameLen);
            success = applySetFarmCrop(x, y, z, season, cropName, error);
            break;
        }
        case COMMAND_TYPE_DESIGNATE_BURROW: {
            // Payload: [4:cmdID][1:cmdType][2:NameLen][N:Name][2:X1][2:Y1][2:Z1][2:X2][2:Y2][2:Z2]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid DESIGNATE_BURROW payload");
                return;
            }
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)nameLen + 12) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid DESIGNATE_BURROW payload size");
                return;
            }
            std::string name(payload.begin() + 7, payload.begin() + 7 + nameLen);
            size_t offset = 7 + nameLen;
            int16_t x1 = ((int16_t)payload[offset]      << 8) | payload[offset + 1];
            int16_t y1 = ((int16_t)payload[offset + 2]  << 8) | payload[offset + 3];
            int16_t z1 = ((int16_t)payload[offset + 4]  << 8) | payload[offset + 5];
            int16_t x2 = ((int16_t)payload[offset + 6]  << 8) | payload[offset + 7];
            int16_t y2 = ((int16_t)payload[offset + 8]  << 8) | payload[offset + 9];
            int16_t z2 = ((int16_t)payload[offset + 10] << 8) | payload[offset + 11];
            success = applyDesignateBurrow(name, x1, y1, z1, x2, y2, z2, error);
            break;
        }
        case COMMAND_TYPE_REMOVE_BURROW: {
            // Payload: [4:cmdID][1:cmdType][2:NameLen][N:Name]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid REMOVE_BURROW payload");
                return;
            }
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)nameLen) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid REMOVE_BURROW payload size");
                return;
            }
            std::string name(payload.begin() + 7, payload.begin() + 7 + nameLen);
            success = applyRemoveBurrow(name, error);
            break;
        }
        case COMMAND_TYPE_ASSIGN_BURROW: {
            // Payload: [4:cmdID][1:cmdType][2:NameLen][N:Name][1:Assign][1:AllCitizens][4:UnitID]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_BURROW payload");
                return;
            }
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)nameLen + 6) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_BURROW payload size");
                return;
            }
            std::string name(payload.begin() + 7, payload.begin() + 7 + nameLen);
            size_t offset = 7 + nameLen;
            bool assign = payload[offset] != 0;
            bool allCitizens = payload[offset + 1] != 0;
            int32_t unitID = (int32_t)read_uint32_be(payload, offset + 2);
            success = applyAssignBurrow(name, assign, allCitizens, unitID, error);
            break;
        }
        case COMMAND_TYPE_SET_ALERT: {
            // Payload: [4:cmdID][1:cmdType][2:NameLen][N:Name][1:Active]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_ALERT payload");
                return;
            }
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)nameLen + 1) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_ALERT payload size");
                return;
            }
            std::string name(payload.begin() + 7, payload.begin() + 7 + nameLen);
            bool active = payload[7 + nameLen] != 0;
            success = applySetAlert(name, active, error);
            break;
        }
        case COMMAND_TYPE_LINK_BUILDING: {
            // Payload: [4:cmdID][1:cmdType][2:LeverX][2:LeverY][2:LeverZ][2:TargetX][2:TargetY][2:TargetZ]
            if (payload.size() < 17) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid LINK_BUILDING payload");
                return;
            }
            int16_t lx = ((int16_t)payload[5]  << 8) | payload[6];
            int16_t ly = ((int16_t)payload[7]  << 8) | payload[8];
            int16_t lz = ((int16_t)payload[9]  << 8) | payload[10];
            int16_t tx = ((int16_t)payload[11] << 8) | payload[12];
            int16_t ty = ((int16_t)payload[13] << 8) | payload[14];
            int16_t tz = ((int16_t)payload[15] << 8) | payload[16];
            success = applyLinkBuilding(lx, ly, lz, tx, ty, tz, error);
            break;
        }
        case COMMAND_TYPE_PULL_LEVER: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid PULL_LEVER payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            success = applyPullLever(x, y, z, error);
            break;
        }
        case COMMAND_TYPE_BUILD_BRIDGE: {
            // Payload: [4:cmdID][1:cmdType][2:X1][2:Y1][2:Z][2:X2][2:Y2][1:Direction]
            // Byte-identical to BUILD_FARM_PLOT plus a trailing direction byte.
            if (payload.size() < 16) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid BUILD_BRIDGE payload");
                return;
            }
            int16_t x1 = ((int16_t)payload[5]  << 8) | payload[6];
            int16_t y1 = ((int16_t)payload[7]  << 8) | payload[8];
            int16_t z  = ((int16_t)payload[9]  << 8) | payload[10];
            int16_t x2 = ((int16_t)payload[11] << 8) | payload[12];
            int16_t y2 = ((int16_t)payload[13] << 8) | payload[14];
            uint8_t direction = payload[15];
            success = placeBridge(x1, y1, z, x2, y2, direction, error);
            break;
        }
        case COMMAND_TYPE_SET_WORKSHOP_PROFILE: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z][4:MinSkillLevel][4:MaxSkillLevel][4:WorkerUnitID]
            if (payload.size() < 23) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_WORKSHOP_PROFILE payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            int32_t minSkillLevel = (int32_t)read_uint32_be(payload, 11);
            int32_t maxSkillLevel = (int32_t)read_uint32_be(payload, 15);
            int32_t workerUnitID  = (int32_t)read_uint32_be(payload, 19);
            success = applySetWorkshopProfile(x, y, z, minSkillLevel, maxSkillLevel, workerUnitID, error);
            break;
        }
        case COMMAND_TYPE_ASSIGN_WORK_DETAIL: {
            // Payload: [4:cmdID][1:cmdType][2:DetailIndex][4:UnitID][1:Add(1)/Remove(0)]
            if (payload.size() < 12) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_WORK_DETAIL payload");
                return;
            }
            uint16_t detailIndex = ((uint16_t)payload[5] << 8) | payload[6];
            int32_t unitID = (int32_t)read_uint32_be(payload, 7);
            bool add = payload[11] != 0;
            success = applyAssignWorkDetail(detailIndex, unitID, add, error);
            break;
        }
        case COMMAND_TYPE_SET_WORK_DETAIL_MODE: {
            // Payload: [4:cmdID][1:cmdType][2:DetailIndex][1:Mode]
            if (payload.size() < 8) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_WORK_DETAIL_MODE payload");
                return;
            }
            uint16_t detailIndex = ((uint16_t)payload[5] << 8) | payload[6];
            uint8_t mode = payload[7];
            success = applySetWorkDetailMode(detailIndex, mode, error);
            break;
        }
        case COMMAND_TYPE_CREATE_WORK_DETAIL: {
            // Payload: [4:cmdID][1:cmdType][2:NameLen][N:Name][1:Mode][2:LaborCount][LaborCount x 1:LaborID]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_WORK_DETAIL payload");
                return;
            }
            uint16_t nameLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)nameLen + 3) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_WORK_DETAIL payload size");
                return;
            }
            std::string name(payload.begin() + 7, payload.begin() + 7 + nameLen);
            size_t offset = 7 + nameLen;
            uint8_t mode = payload[offset];
            uint16_t laborCount = ((uint16_t)payload[offset + 1] << 8) | payload[offset + 2];
            offset += 3;
            if (payload.size() < offset + (size_t)laborCount) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_WORK_DETAIL labor list size");
                return;
            }
            std::vector<uint8_t> laborIDs(payload.begin() + offset, payload.begin() + offset + laborCount);
            success = applyCreateWorkDetail(name, mode, laborIDs, error);
            break;
        }
        case COMMAND_TYPE_BRING_GOODS_TO_DEPOT: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z]
            //          [2:ItemTypeLen][N:ItemType][2:MaterialLen][N:Material]
            //          [4:MaxCount][8:MaxTotalValue]
            //          [1:ItemClass] -- trade-pack wave: ALWAYS appended by
            //          the Go encoder; a payload ending before it is
            //          present (old pre-item_class encodings) decodes as
            //          ITEM_CLASS_ANY -- mirrors internal/protocol/codec.go's
            //          EOF-tolerant decode of the same field.
            if (payload.size() < 11 + 2) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid BRING_GOODS_TO_DEPOT payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            size_t offset = 11;
            uint16_t itemTypeLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
            offset += 2;
            if (payload.size() < offset + (size_t)itemTypeLen + 2) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid BRING_GOODS_TO_DEPOT item type filter");
                return;
            }
            std::string itemTypeFilter(payload.begin() + offset, payload.begin() + offset + itemTypeLen);
            offset += itemTypeLen;
            uint16_t materialLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
            offset += 2;
            if (payload.size() < offset + (size_t)materialLen + 4 + 8) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid BRING_GOODS_TO_DEPOT material filter");
                return;
            }
            std::string materialFilter(payload.begin() + offset, payload.begin() + offset + materialLen);
            offset += materialLen;
            int32_t maxCount = (int32_t)read_uint32_be(payload, offset);
            offset += 4;
            uint64_t maxTotalValueRaw = 0;
            for (int i = 0; i < 8; i++) maxTotalValueRaw = (maxTotalValueRaw << 8) | payload[offset + i];
            int64_t maxTotalValue = (int64_t)maxTotalValueRaw;
            offset += 8;
            uint8_t itemClass = (payload.size() >= offset + 1) ? payload[offset] : ITEM_CLASS_ANY;
            success = applyBringGoodsToDepot(x, y, z, itemTypeFilter, materialFilter, itemClass, maxCount, maxTotalValue, error);
            break;
        }
        case COMMAND_TYPE_UNMARK_TRADE_GOODS: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z]
            //          [2:ItemTypeLen][N:ItemType][2:MaterialLen][N:Material]
            //          [1:ItemClass][4:MaxCount]
            if (payload.size() < 11 + 2) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNMARK_TRADE_GOODS payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            size_t offset = 11;
            uint16_t itemTypeLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
            offset += 2;
            if (payload.size() < offset + (size_t)itemTypeLen + 2) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNMARK_TRADE_GOODS item type filter");
                return;
            }
            std::string itemTypeFilter(payload.begin() + offset, payload.begin() + offset + itemTypeLen);
            offset += itemTypeLen;
            uint16_t materialLen = ((uint16_t)payload[offset] << 8) | payload[offset + 1];
            offset += 2;
            if (payload.size() < offset + (size_t)materialLen + 1 + 4) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid UNMARK_TRADE_GOODS material filter");
                return;
            }
            std::string materialFilter(payload.begin() + offset, payload.begin() + offset + materialLen);
            offset += materialLen;
            uint8_t itemClass = payload[offset];
            offset += 1;
            int32_t maxCount = (int32_t)read_uint32_be(payload, offset);
            success = applyUnmarkTradeGoods(x, y, z, itemTypeFilter, materialFilter, itemClass, maxCount, error);
            break;
        }
        case COMMAND_TYPE_SET_DEPOT_TRADE_FLAGS: {
            // Payload: [4:cmdID][1:cmdType][2:X][2:Y][2:Z]
            //          [1:TraderRequested][1:AnyoneCanTrade]
            if (payload.size() < 13) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_DEPOT_TRADE_FLAGS payload");
                return;
            }
            int16_t x = ((int16_t)payload[5] << 8) | payload[6];
            int16_t y = ((int16_t)payload[7] << 8) | payload[8];
            int16_t z = ((int16_t)payload[9] << 8) | payload[10];
            bool traderRequested = payload[11] != 0;
            bool anyoneCanTrade = payload[12] != 0;
            success = applySetDepotTradeFlags(x, y, z, traderRequested, anyoneCanTrade, error);
            break;
        }
        case COMMAND_TYPE_APPOINT_POSITION: {
            // Payload: [4:cmdID][1:cmdType][4:UnitID][2:CodeLen][N:Code]
            if (payload.size() < 11) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid APPOINT_POSITION payload");
                return;
            }
            int32_t unitID = (int32_t)read_uint32_be(payload, 5);
            uint16_t codeLen = ((uint16_t)payload[9] << 8) | payload[10];
            if (payload.size() < 11 + (size_t)codeLen) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid APPOINT_POSITION payload size");
                return;
            }
            std::string positionCode(payload.begin() + 11, payload.begin() + 11 + codeLen);
            success = applyAppointPosition(unitID, positionCode, error);
            break;
        }
        case COMMAND_TYPE_SET_BOOKKEEPER_PRECISION: {
            // Payload: [4:cmdID][1:cmdType][1:Precision]
            if (payload.size() < 6) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SET_BOOKKEEPER_PRECISION payload");
                return;
            }
            uint8_t precision = payload[5];
            success = applySetBookkeeperPrecision(precision, error);
            break;
        }
        case COMMAND_TYPE_CREATE_SQUAD: {
            // Payload: [4:cmdID][1:cmdType][2:CodeLen][N:Code]
            if (payload.size() < 7) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_SQUAD payload");
                return;
            }
            uint16_t codeLen = ((uint16_t)payload[5] << 8) | payload[6];
            if (payload.size() < 7 + (size_t)codeLen) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CREATE_SQUAD payload size");
                return;
            }
            std::string positionCode(payload.begin() + 7, payload.begin() + 7 + codeLen);
            success = applyCreateSquad(positionCode, error);
            break;
        }
        case COMMAND_TYPE_ASSIGN_SQUAD: {
            // Payload: [4:cmdID][1:cmdType][4:SquadID][4:UnitID][1:Add]
            if (payload.size() < 14) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid ASSIGN_SQUAD payload");
                return;
            }
            int32_t squadID = (int32_t)read_uint32_be(payload, 5);
            int32_t unitID = (int32_t)read_uint32_be(payload, 9);
            bool add = payload[13] != 0;
            success = applyAssignSquad(squadID, unitID, add, error);
            break;
        }
        case COMMAND_TYPE_SQUAD_ORDER: {
            // Payload: [4:cmdID][1:cmdType][4:SquadID][1:Type][2:X][2:Y][2:Z][2:NameLen][N:BurrowName]
            if (payload.size() < 18) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SQUAD_ORDER payload");
                return;
            }
            int32_t squadID = (int32_t)read_uint32_be(payload, 5);
            uint8_t orderType = payload[9];
            int16_t x = ((int16_t)payload[10] << 8) | payload[11];
            int16_t y = ((int16_t)payload[12] << 8) | payload[13];
            int16_t z = ((int16_t)payload[14] << 8) | payload[15];
            uint16_t nameLen = ((uint16_t)payload[16] << 8) | payload[17];
            if (payload.size() < 18 + (size_t)nameLen) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid SQUAD_ORDER payload size");
                return;
            }
            std::string burrowName(payload.begin() + 18, payload.begin() + 18 + nameLen);
            success = applySquadOrder(squadID, orderType, x, y, z, burrowName, error);
            break;
        }
        case COMMAND_TYPE_CANCEL_ORDER: {
            // Payload: [4:cmdID][1:cmdType][4:OrderID]
            if (payload.size() < 9) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid CANCEL_ORDER payload");
                return;
            }
            int32_t orderID = (int32_t)read_uint32_be(payload, 5);
            success = applyCancelOrder(orderID, error);
            break;
        }
        case COMMAND_TYPE_EDIT_ORDER: {
            // Payload: [4:cmdID][1:cmdType][4:OrderID][1:HasAmount][2:Amount][1:HasFrequency][1:Frequency]
            if (payload.size() < 14) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid EDIT_ORDER payload");
                return;
            }
            int32_t orderID = (int32_t)read_uint32_be(payload, 5);
            bool hasAmount = payload[9] != 0;
            uint16_t amount = ((uint16_t)payload[10] << 8) | payload[11];
            bool hasFrequency = payload[12] != 0;
            uint8_t frequency = payload[13];
            if (!hasAmount && !hasFrequency) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "EDIT_ORDER requires at least one of amount or frequency to change");
                return;
            }
            if (hasAmount && (amount == 0 || amount > 100)) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "EDIT_ORDER amount out of range (1-100)");
                return;
            }
            bool deleted = false;
            success = applyEditOrder(orderID, hasAmount, amount, hasFrequency, frequency, deleted, error);
            break;
        }
        default:
            sendCommandAck(cmdID, 0x02, "Unknown command type");
            return;
    }
    } catch (std::exception &e) {
        // DFHack::Error::All derives from std::exception (Error.h:52), so
        // this catches both DFHack precondition failures and anything else.
        sendCommandAck(cmdID, 0x02, std::string("DFHack exception: ") + e.what());
        return;
    }

    sendCommandAck(cmdID, success ? 0x00 : 0x02, error);
}

// Handle COMMAND message from background thread
// THREAD SAFETY: Called from background thread, queues command for main thread execution
void handleCommand(const std::vector<uint8_t> &payload)
{
    // Queue command for execution on main thread
    std::lock_guard<std::mutex> lock(g_command_queue_mutex);
    g_command_queue.push({payload});
}

// drain_pending_work executes queued commands/queries, the step-repause
// check, and any pending resync. Called from TWO contexts:
//   1. plugin_onupdate (main thread) — while the simulation is running.
//   2. The socket thread under CoreSuspender — because plugin_onupdate
//      does NOT fire while DF is paused (confirmed live 2026-07-12), and
//      a paused-deaf plugin deadlocks the turn protocol: the agent could
//      never even send UNPAUSE. Suspension makes DF state access safe
//      from the socket thread (standard DFHack remote-tools pattern);
//      queue pops are mutex-guarded so the two contexts never double-run
//      an item.
static void drain_pending_work_impl(color_ostream &out)
{
    // Don't touch DF structures if no map is loaded (startup/shutdown).
    if (!Core::getInstance().isMapLoaded() || !df::global::world) {
        return;
    }

    // Process all queued commands
    while (true) {
        QueuedCommand cmd;
        {
            std::lock_guard<std::mutex> lock(g_command_queue_mutex);
            if (g_command_queue.empty()) {
                break;
            }
            cmd = g_command_queue.front();
            g_command_queue.pop();
        }
        executeCommand(cmd.payload);
    }

    // Step-mode auto-repause: once the frame counter reaches the target,
    // pause the simulation. Frames only advance while unpaused, which is
    // also when onupdate fires — so this check living here is sufficient.
    if (g_step_target_frame >= 0 &&
        (int64_t)df::global::world->frame_counter >= g_step_target_frame) {
        World::SetPauseState(true);
        g_step_target_frame = -1;
        // STEP-COMPLETION PUSH: the step just finished — this is the turn
        // boundary the agent reasons from. Push entities, tile deltas, and
        // announcements now, or the model plays against a world state
        // frozen at connect time.
        push_state_refresh();
    }

    // Drain query queue (read-only DF accesses).
    while (true) {
        QueuedQuery q;
        {
            std::lock_guard<std::mutex> lock(g_query_queue_mutex);
            if (g_query_queue.empty()) {
                break;
            }
            q = g_query_queue.front();
            g_query_queue.pop();
        }
        executeQuery(q.queryID, q.name, q.argsJSON);
    }

    // RESYNC requested by the socket thread — full-state send.
    if (g_resync_requested.exchange(false)) {
        if (!send_full_state(out)) {
            out.printerr("Failed to send full state\n");
        }
    }
}

// drain_pending_work — exception barrier of last resort around the queued
// work. executeCommand and executeQuery carry their own guards; anything
// that still escapes (state extraction in send_full_state /
// push_state_refresh, a DFHack CHECK-macro throw from an unexpected path)
// would otherwise unwind into plugin_onupdate or the socket thread's stack
// and std::terminate DF (DFHack precondition macros THROW — see the guard
// note in executeCommand).
static void drain_pending_work(color_ostream &out)
{
    try {
        drain_pending_work_impl(out);
    } catch (std::exception &e) {
        out.printerr("df_ai_protocol: exception in drain_pending_work: %s\n", e.what());
    } catch (...) {
        out.printerr("df_ai_protocol: unknown exception in drain_pending_work\n");
    }
}

// drain_from_socket_thread suspends the core and drains pending work.
// Invoked right after the socket thread enqueues a command/query/resync so
// the plugin stays responsive while DF is paused. Unconditional (no pause
// check): eliminates the pause-transition race, and command/query rates
// are low enough that suspension cost is irrelevant.
//
// THREADING: this must NOT block indefinitely on the core lock. The
// external teardown paths join this thread while they may themselves hold
// CoreSuspendMutex — DFHack runs non-`unlocked` plugin commands (our
// ai-disconnect) under CoreSuspender (dfhack library/PluginManager.cpp:
// 513-516), and at DF exit Core::Shutdown (library/Core.cpp:1916-1947)
// stops servicing suspend requests (Core::Update's CoreWakeup.wait,
// Core.cpp:1651-1652, never runs again) before Plugin::unload calls
// plugin_shutdown (library/PluginManager.cpp:441-442). A plain
// CoreSuspender here (blocking lock on the recursive_timed_mutex
// CoreSuspendMutex, Core.h:327,457-461) would then never return -> the
// joiner waits on us, we wait on the lock: deadlock. So: bounded
// try-suspend (ConditionalCoreSuspender = try_lock_for(100ms), Core.h:
// 376-381,505-511 — same pattern as dfhack Core.cpp:407-414) and recheck
// the stop flag between attempts.
static void drain_from_socket_thread()
{
    while (!g_stop_message_loop) {
        ConditionalCoreSuspender suspend;
        if (suspend) {
            drain_pending_work(Core::getInstance().getConsole());
            return;
        }
        // Couldn't get the core within 100ms — teardown may be waiting to
        // join us. Re-check the flag and retry.
    }
}

// Plugin onupdate - process queued work on MAIN THREAD while unpaused
DFhackCExport command_result plugin_onupdate(color_ostream &out)
{
    if (!is_enabled) {
        return CR_OK;
    }

    // Don't process commands if map isn't loaded (prevents crashes on startup/shutdown)
    if (!Core::getInstance().isMapLoaded() || !df::global::world) {
        return CR_OK;
    }

    drain_pending_work(out);

    // Poll DF announcements every ~5 seconds (50 onupdate ticks at 10Hz).
    // Sends only NEW entries; safe to call frequently if you want lower
    // latency. Skipped when not connected — no point queuing.
    static int g_announcement_throttle = 0;
    if (g_connected && g_socket && g_socket->IsSocketValid()) {
        g_announcement_throttle++;
        if (g_announcement_throttle >= 50) {
            g_announcement_throttle = 0;
            // Exception barrier: this call sits directly in plugin_onupdate
            // (no drain_pending_work guard above it) — DFHack calls us with
            // no try/catch of its own, so a throw here would terminate DF.
            try {
                poll_and_send_announcements();
            } catch (std::exception &e) {
                out.printerr("df_ai_protocol: exception in announcement poll: %s\n", e.what());
            } catch (...) {
                out.printerr("df_ai_protocol: unknown exception in announcement poll\n");
            }
        }
    }

    return CR_OK;
}

// Plugin initialization
DFhackCExport command_result plugin_init(color_ostream &out, std::vector<PluginCommand> &commands)
{
    commands.push_back(PluginCommand(
        "ai-connect",
        "Connect to Go orchestrator server",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (connect_to_server(out)) {
                out.print("Connected successfully\n");
                out.print("Use 'ai-send-entities' to send initial entity data\n");
                out.print("Or enable 'ai-auto-update on' for automatic updates\n");
                return CR_OK;
            }
            return CR_FAILURE;
        },
        false,
        false,
        "Usage: ai-connect\nConnects to the Go orchestrator server and performs handshake."
    ));

    commands.push_back(PluginCommand(
        "ai-disconnect",
        "Disconnect from Go orchestrator server",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            disconnect_from_server();
            out.print("Disconnected from server\n");
            return CR_OK;
        },
        false,
        false,
        "Usage: ai-disconnect\nDisconnects from the Go orchestrator server."
    ));

    commands.push_back(PluginCommand(
        "ai-reconnect",
        "Reconnect with exponential backoff retry",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            // Parse max attempts from params (default 5)
            int max_attempts = 5;
            if (params.size() > 0) {
                max_attempts = std::atoi(params[0].c_str());
                if (max_attempts < 1 || max_attempts > 10) {
                    out.printerr("Invalid max attempts: %d (must be 1-10)\n", max_attempts);
                    return CR_WRONG_USAGE;
                }
            }

            if (connect_with_retry(out, max_attempts)) {
                return CR_OK;
            }
            return CR_FAILURE;
        },
        false,
        false,
        "Usage: ai-reconnect [max_attempts]\nAttempts to reconnect with exponential backoff (1s, 2s, 4s, 8s, 16s, max 30s).\nDefault max attempts: 5"
    ));

    commands.push_back(PluginCommand(
        "ai-status",
        "Check connection status",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (g_connected && g_socket && g_socket->IsSocketValid()) {
                out.print("Connected to %s:%d\n", g_server_host.c_str(), g_server_port);
                out.print("Connection ID: 0x%08X\n", g_connection_id);
            } else {
                out.print("Not connected\n");
            }
            return CR_OK;
        },
        false,
        false,
        "Usage: ai-status\nShows current connection status."
    ));

    commands.push_back(PluginCommand(
        "ai-listen",
        "Listen for incoming messages from server",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (!g_connected) {
                out.printerr("Not connected - use 'ai-connect' first\n");
                return CR_FAILURE;
            }

            out.print("Listening for server messages... (checking once)\n");

            std::vector<uint8_t> payload;
            uint8_t msg_type;

            // Set short timeout for this check
            g_socket->SetReceiveTimeout(2, 0);  // 2 seconds

            if (receive_message(payload, msg_type)) {
                switch (msg_type) {
                    case MSG_TYPE_RESYNC_REQUEST:
                        out.print("Received RESYNC_REQUEST — queued for main thread\n");
                        g_resync_requested = true;
                        break;
                    case MSG_TYPE_HEARTBEAT:
                        out.print("Received HEARTBEAT\n");
                        break;
                    default:
                        out.print("Received message type 0x%02X\n", msg_type);
                        break;
                }
            } else {
                out.print("No message received (timeout or error)\n");
            }

            return CR_OK;
        },
        false,
        false,
        "Usage: ai-listen\nChecks for incoming messages from the server."
    ));

    commands.push_back(PluginCommand(
        "ai-check-updates",
        "Check for and send tile changes",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (!g_connected) {
                out.printerr("Not connected - use 'ai-connect' first\n");
                return CR_FAILURE;
            }

            out.print("Scanning for tile changes...\n");
            std::vector<uint8_t> changes = detect_tile_changes();

            if (changes.empty()) {
                out.print("No changes detected\n");
                return CR_OK;
            }

            uint32_t count = changes.size() / 9;
            out.print("Found %d changed tiles\n", count);

            if (send_tile_update(out, changes)) {
                return CR_OK;
            }
            return CR_FAILURE;
        },
        false,
        false,
        "Usage: ai-check-updates\nDetects changed tiles and sends TILE_UPDATE to server."
    ));

    commands.push_back(PluginCommand(
        "ai-send-entities",
        "Scan and send entity positions to server",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (!g_connected) {
                out.printerr("Not connected - use 'ai-connect' first\n");
                return CR_FAILURE;
            }

            out.print("Scanning entities...\n");
            std::vector<uint32_t> dead_ids;
            std::vector<EntityInfo> entities = extract_entities(&dead_ids);

            out.print("Found %d entities\n", (int)entities.size());

            // Serialize and send
            std::vector<uint8_t> message = serialize_entity_update(entities, dead_ids);

            int sent = socket_send_locked(message.data(), message.size());
            if (sent != (int)message.size()) {
                out.printerr("Failed to send ENTITY_UPDATE (%d/%d bytes)\n", sent, (int)message.size());
                return CR_FAILURE;
            }

            out.print("Sent ENTITY_UPDATE (%d entities, %d bytes)\n", (int)entities.size(), (int)message.size());
            return CR_OK;
        },
        false,
        false,
        "Usage: ai-send-entities\nScans all active units and sends entity positions to server."
    ));

    commands.push_back(PluginCommand(
        "ai-save",
        "Request server to save current fort state",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (!g_connected) {
                out.printerr("Not connected - use 'ai-connect' first\n");
                return CR_FAILURE;
            }

            // Send RESYNC_REQUEST with reason 0x04 to trigger save
            /* Simpler alternative: Just print instructions
            out.print("To save modifications, use:\n");
            out.print("  curl -X POST http://localhost:8081/ai/save\n");
            return CR_OK;
            */
            std::vector<uint8_t> msg;
            msg.resize(4, 0);
            msg.push_back(PROTOCOL_VERSION);
            msg.push_back(0x07);  // RESYNC_REQUEST
            msg.push_back(0x04);  // Reason: Save request

            uint32_t length = msg.size();
            msg[0] = (length >> 24) & 0xFF;
            msg[1] = (length >> 16) & 0xFF;
            msg[2] = (length >> 8) & 0xFF;
            msg[3] = length & 0xFF;

            int sent = socket_send_locked(msg.data(), msg.size());
            if (sent != (int)msg.size()) {
                out.printerr("Failed to send save request\n");
                return CR_FAILURE;
            }

            out.print("Save request sent to server\n");
            out.print("Check server logs for save confirmation\n");
            return CR_OK;
        },
        false,
        false,
        "Usage: ai-save\nRequests the server to save current modifications to disk."
    ));

    commands.push_back(PluginCommand(
        "ai-auto-update",
        "Toggle automatic updates on/off",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (!g_connected) {
                out.printerr("Not connected - use 'ai-connect' first\n");
                return CR_FAILURE;
            }

            // Parse on/off from params
            if (params.empty()) {
                out.print("Auto-update is currently: %s\n", g_auto_update_enabled ? "ON" : "OFF");
                return CR_OK;
            }

            std::string cmd = params[0];
            if (cmd == "on" || cmd == "ON" || cmd == "1" || cmd == "true") {
                g_auto_update_enabled = true;
                out.print("Auto-update enabled - will send updates every 10 heartbeats (~10 seconds)\n");
            } else if (cmd == "off" || cmd == "OFF" || cmd == "0" || cmd == "false") {
                g_auto_update_enabled = false;
                out.print("Auto-update disabled\n");
            } else {
                out.printerr("Invalid parameter: %s (use 'on' or 'off')\n", cmd.c_str());
                return CR_WRONG_USAGE;
            }

            return CR_OK;
        },
        false,
        false,
        "Usage: ai-auto-update [on|off]\nToggle automatic entity and tile updates.\nIf no parameter given, shows current status."
    ));

    out.print("DF AI Protocol plugin initialized\n");
    out.print("Use 'ai-connect' to connect to Go orchestrator\n");

    return CR_OK;
}

// Plugin enable/disable
DFhackCExport command_result plugin_enable(color_ostream &out, bool enable)
{
    if (enable) {
        is_enabled = true;
        out.print("DF AI Protocol enabled\n");

        // Auto-connect to server
        if (connect_to_server(out)) {
            out.print("Auto-connected to server\n");
        }
    } else {
        is_enabled = false;
        disconnect_from_server();
        out.print("DF AI Protocol disabled\n");
    }

    return CR_OK;
}

// Plugin shutdown
DFhackCExport command_result plugin_shutdown(color_ostream &out)
{
    disconnect_from_server();
    out.print("DF AI Protocol plugin shutdown\n");
    return CR_OK;
}

// Connect to Go orchestrator server
bool connect_to_server(color_ostream &out)
{
    if (g_connected) {
        out.print("Already connected\n");
        return true;
    }

    // Reap a message thread that tore itself down (heartbeat timeout /
    // server DISCONNECT leave an exited-but-joinable thread behind; see
    // teardown_from_socket_thread). Overwriting g_message_thread below
    // while it is still joinable would std::terminate in ~thread(). The
    // join returns immediately since the thread has already exited.
    if (g_message_thread &&
        g_message_thread->get_id() != std::this_thread::get_id()) {
        g_stop_message_loop = true;
        if (g_message_thread->joinable()) {
            g_message_thread->join();
        }
        g_message_thread.reset();
    }

    out.print("Connecting to %s:%d...\n", g_server_host.c_str(), g_server_port);

    // Create socket
    g_socket = std::make_unique<CActiveSocket>();
    g_socket->Initialize();

    // Connect
    if (!g_socket->Open(g_server_host.c_str(), g_server_port)) {
        out.printerr("Failed to connect to server\n");
        g_socket.reset();
        return false;
    }

    // Disable Nagle's algorithm for immediate sending
    g_socket->DisableNagleAlgoritm();

    out.print("TCP connection established\n");

    // Generate random connection ID
    g_connection_id = rand();

    // Send handshake
    if (!send_handshake(out)) {
        out.printerr("Failed to send handshake\n");
        disconnect_from_server();
        return false;
    }

    // Receive handshake from server
    if (!receive_handshake(out)) {
        out.printerr("Failed to receive server handshake\n");
        disconnect_from_server();
        return false;
    }

    g_connected = true;
    g_reconnect_delay_ms = 1000;  // Reset backoff on successful connection
    out.print("Handshake complete - connected successfully\n");

    // Start message receive loop in background thread
    g_stop_message_loop = false;
    g_message_thread = std::make_unique<std::thread>([]() {
        // Use Core console for thread-safe output
        auto &console = Core::getInstance().getConsole();
        message_receive_loop(console);
    });

    return true;
}

// Connect with exponential backoff retry logic
bool connect_with_retry(color_ostream &out, int max_attempts = 5)
{
    for (int attempt = 1; attempt <= max_attempts; attempt++) {
        out.print("Connection attempt %d/%d (delay: %dms)...\n",
                  attempt, max_attempts, g_reconnect_delay_ms);

        if (connect_to_server(out)) {
            return true;
        }

        if (attempt < max_attempts) {
            // Wait before retry
            std::this_thread::sleep_for(std::chrono::milliseconds(g_reconnect_delay_ms));

            // Exponential backoff: double delay, cap at 30 seconds
            g_reconnect_delay_ms = g_reconnect_delay_ms * 2;
            if (g_reconnect_delay_ms > 30000) {
                g_reconnect_delay_ms = 30000;
            }
        }
    }

    out.printerr("Failed to connect after %d attempts\n", max_attempts);
    return false;
}

// Shared teardown tail: send best-effort DISCONNECT, close + free the
// socket, reset connection state. Does NOT touch g_message_thread, so it
// is safe from ANY thread — including the socket thread itself. The
// Close()/reset() runs under g_send_mutex so a concurrent
// socket_send_locked (acks/announcements from the main thread) can never
// dereference a freed socket.
static void close_socket_and_reset()
{
    if (g_socket && g_connected) {
        // Send graceful DISCONNECT message (best effort)
        std::vector<uint8_t> message;
        message.reserve(7);

        // Length placeholder
        message.resize(4, 0);

        // Version and type
        message.push_back(PROTOCOL_VERSION);
        message.push_back(MSG_TYPE_DISCONNECT);

        // Payload: [1: Reason]
        message.push_back(REASON_PLUGIN_UNLOAD);

        // Fill length
        uint32_t length = message.size();
        message[0] = (length >> 24) & 0xFF;
        message[1] = (length >> 16) & 0xFF;
        message[2] = (length >> 8) & 0xFF;
        message[3] = length & 0xFF;

        // Send (ignore errors, we're shutting down anyway)
        socket_send_locked(message.data(), message.size());
    }

    {
        std::lock_guard<std::mutex> lock(g_send_mutex);
        if (g_socket) {
            g_socket->Close();
            g_socket.reset();
        }
    }
    g_connected = false;
    reset_announcement_cursor();
    // B11: a still-visible dangerous-wildlife unit re-trips once after
    // reconnect rather than staying silently suppressed forever -- same
    // reconnect-tolerance precedent as reset_announcement_cursor just above
    // (the reconnected peer has no memory of it either).
    reset_wildlife_tripwire_tracking();
    // Feature 013 wave 013-B: story_pulse's cursor resets the same way --
    // see reset_story_pulse_cursor's own doc comment (narrative.cpp).
    reset_story_pulse_cursor();
}

// Teardown path for the SOCKET THREAD ONLY (heartbeat timeout, server-sent
// DISCONNECT). Never joins: std::thread::join() from the thread itself
// throws std::system_error(resource_deadlock_would_occur), which escapes
// message_receive_loop and std::terminate()s DF — that was killing DF
// ~15s after every orchestrator exit. Instead: set the stop flag, close
// the socket, and let message_receive_loop return on its own. The
// exited-but-still-joinable std::thread object is reaped later by
// disconnect_from_server() or the next connect_to_server() call, both of
// which run on other threads.
static void teardown_from_socket_thread()
{
    g_stop_message_loop = true;
    close_socket_and_reset();
}

// Disconnect from server — EXTERNAL path (ai-disconnect console command,
// plugin_enable(false), plugin_shutdown). Joins the message thread, so it
// must never run ON the message thread; a std::this_thread guard falls
// back to the no-join teardown just in case. The join itself cannot
// deadlock against drain_from_socket_thread even though DFHack invokes
// non-`unlocked` plugin commands under CoreSuspender
// (dfhack library/PluginManager.cpp:513-516) — i.e. we may HOLD
// CoreSuspendMutex right here — because the drain only try-locks with a
// 100ms bound and rechecks g_stop_message_loop between attempts.
void disconnect_from_server()
{
    // Stop message thread first
    g_stop_message_loop = true;
    if (g_message_thread) {
        if (g_message_thread->get_id() == std::this_thread::get_id()) {
            // Defensive: self-join would terminate DF (see
            // teardown_from_socket_thread). Should be unreachable — the
            // socket thread never calls disconnect_from_server anymore.
            teardown_from_socket_thread();
            return;
        }
        if (g_message_thread->joinable()) {
            g_message_thread->join();
        }
        g_message_thread.reset();
    }

    close_socket_and_reset();
}

// Send handshake message
bool send_handshake(color_ostream &out)
{
    std::vector<uint8_t> message;

    // Reserve space for length (will fill later)
    message.resize(4, 0);

    // Version
    message.push_back(PROTOCOL_VERSION);

    // Message type
    message.push_back(MSG_TYPE_HANDSHAKE);

    // Payload: [1: Version] [4: ConnectionID] [2: Capabilities Length] [N: Capabilities]
    message.push_back(PROTOCOL_VERSION);

    // Connection ID (big-endian)
    message.push_back((g_connection_id >> 24) & 0xFF);
    message.push_back((g_connection_id >> 16) & 0xFF);
    message.push_back((g_connection_id >> 8) & 0xFF);
    message.push_back(g_connection_id & 0xFF);

    // Capabilities (empty for now)
    message.push_back(0);  // Length high byte
    message.push_back(0);  // Length low byte

    // Fill in length header (big-endian)
    uint32_t length = message.size();
    message[0] = (length >> 24) & 0xFF;
    message[1] = (length >> 16) & 0xFF;
    message[2] = (length >> 8) & 0xFF;
    message[3] = length & 0xFF;

    // Send
    int32_t sent = socket_send_locked(message.data(), message.size());
    if (sent != (int32_t)message.size()) {
        out.printerr("Failed to send handshake: sent %d/%d bytes\n", sent, (int)message.size());
        return false;
    }

    out.print("Sent HANDSHAKE (version %d, ID 0x%08X)\n", PROTOCOL_VERSION, g_connection_id);
    return true;
}

// Receive handshake from server
bool receive_handshake(color_ostream &out)
{
    std::vector<uint8_t> msg;
    uint8_t msg_type;

    if (!receive_message(msg, msg_type)) {
        out.printerr("Failed to receive server handshake message\n");
        return false;
    }

    if (msg_type != MSG_TYPE_HANDSHAKE) {
        out.printerr("Expected HANDSHAKE (0x01), got 0x%02X\n", msg_type);
        return false;
    }

    // Parse handshake payload: [1: Version] [4: ConnectionID] [2: Capabilities Length] [N: Capabilities]
    if (msg.size() < 7) {
        out.printerr("Handshake payload too short\n");
        return false;
    }

    uint8_t server_version = msg[0];
    uint32_t server_conn_id = (msg[1] << 24) | (msg[2] << 16) | (msg[3] << 8) | msg[4];

    if (server_version != PROTOCOL_VERSION) {
        out.printerr("Protocol version mismatch: plugin=%d, server=%d\n", PROTOCOL_VERSION, server_version);
        return false;
    }

    out.print("Received server HANDSHAKE (version %d, ID 0x%08X)\n", server_version, server_conn_id);
    return true;
}

// Receive a complete message from the socket
// Returns payload only (without length/version/type headers)
bool receive_message(std::vector<uint8_t> &msg_out, uint8_t &type_out)
{
    // Read 6-byte header: [4: Length] [1: Version] [1: Type]
    uint8_t header[6];
    if (!read_exact(header, 6)) {
        return false;
    }

    // Parse length (big-endian)
    uint32_t length = (header[0] << 24) | (header[1] << 16) | (header[2] << 8) | header[3];

    // Parse version and type
    uint8_t version = header[4];
    type_out = header[5];

    // Validate
    if (version != PROTOCOL_VERSION) {
        return false;
    }
    if (length < 6 || length > 10 * 1024 * 1024) {  // Max 10MB
        return false;
    }

    // Read payload (length - 6 bytes of header)
    uint32_t payload_size = length - 6;
    if (payload_size > 0) {
        msg_out.resize(payload_size);
        if (!read_exact(msg_out.data(), payload_size)) {
            return false;
        }
    } else {
        msg_out.clear();
    }

    return true;
}

// Send full map state
bool send_full_state(color_ostream &out)
{
    if (!g_connected || !g_socket) {
        out.printerr("Not connected\n");
        return false;
    }

    out.print("Extracting full map state...\n");

    // Get map dimensions first
    int32_t width, height, depth;
    if (!get_map_dimensions(width, height, depth)) {
        out.printerr("No map loaded or failed to get dimensions\n");
        return false;
    }

    out.print("Map dimensions: %dx%dx%d\n", width, height, depth);

    // Extract tile data
    std::vector<uint8_t> tiles = extract_full_map_state();
    if (tiles.empty()) {
        out.printerr("Failed to extract map state\n");
        return false;
    }

    int tile_count = tiles.size() / 9;
    out.print("Extracted %d tiles (%d KB)\n", tile_count, (int)(tiles.size() / 1024));

    // Build FULL_STATE message
    std::vector<uint8_t> message;
    message.reserve(6 + 6 + tiles.size());  // Header + dimensions + tiles

    // Reserve space for length (will fill later)
    message.resize(4, 0);

    // Version and type
    message.push_back(PROTOCOL_VERSION);
    message.push_back(MSG_TYPE_FULL_STATE);

    // Payload: [2: Width] [2: Height] [2: Depth] [N: Tiles]
    write_uint16_be(message, static_cast<uint16_t>(width));
    write_uint16_be(message, static_cast<uint16_t>(height));
    write_uint16_be(message, static_cast<uint16_t>(depth));

    // Append tile data
    message.insert(message.end(), tiles.begin(), tiles.end());

    // Fill in length header (overwrite the reserved space at beginning)
    uint32_t length = message.size();
    message[0] = (length >> 24) & 0xFF;
    message[1] = (length >> 16) & 0xFF;
    message[2] = (length >> 8) & 0xFF;
    message[3] = length & 0xFF;

    // Send
    int32_t sent = socket_send_locked(message.data(), message.size());
    if (sent != (int32_t)message.size()) {
        out.printerr("Failed to send full state: sent %d/%d bytes\n", sent, (int)message.size());
        return false;
    }

    out.print("Sent FULL_STATE (%d tiles, %d bytes)\n", tile_count, (int)message.size());
    return true;
}

// Send tile update message (US2)
bool send_tile_update(color_ostream &out, const std::vector<uint8_t> &tiles)
{
    if (!g_connected || !g_socket) {
        out.printerr("Not connected\n");
        return false;
    }

    uint32_t tile_count = tiles.size() / 9;
    if (tile_count == 0) {
        out.print("No changed tiles to send\n");
        return true;
    }

    // Build TILE_UPDATE message: [4:Length][1:Ver][1:Type][4:Count][N:Tiles]
    std::vector<uint8_t> message;
    message.reserve(10 + tiles.size());

    // Length placeholder
    message.resize(4, 0);

    // Version and type
    message.push_back(PROTOCOL_VERSION);
    message.push_back(MSG_TYPE_TILE_UPDATE);

    // Payload: [4: Count][N: Tiles]
    write_uint32_be(message, tile_count);
    message.insert(message.end(), tiles.begin(), tiles.end());

    // Fill length
    uint32_t length = message.size();
    message[0] = (length >> 24) & 0xFF;
    message[1] = (length >> 16) & 0xFF;
    message[2] = (length >> 8) & 0xFF;
    message[3] = length & 0xFF;

    // Debug: print first 20 bytes of message
    out.print("DEBUG: Message header: ");
    for (size_t i = 0; i < (message.size() < 20 ? message.size() : 20); i++) {
        out.print("%02X ", message[i]);
    }
    out.print("\n");

    // Send
    int32_t sent = socket_send_locked(message.data(), message.size());
    if (sent != (int32_t)message.size()) {
        out.printerr("Failed to send tile update: sent %d/%d bytes\n", sent, (int)message.size());
        return false;
    }

    // Force flush to ensure data is transmitted
    if (!g_socket->IsSocketValid()) {
        out.printerr("Socket became invalid after send!\n");
        return false;
    }

    out.print("Sent TILE_UPDATE (%d tiles, %d bytes)\n", tile_count, (int)message.size());
    return true;
}

// Send heartbeat echo back to server
bool send_heartbeat_echo(color_ostream &out, uint64_t timestamp, uint8_t sequence)
{
    if (!g_connected || !g_socket) {
        out.printerr("Not connected\n");
        return false;
    }

    // Build HEARTBEAT message: [4:Length][1:Ver][1:Type][8:Timestamp][1:Sequence]
    std::vector<uint8_t> message;
    message.reserve(15);

    // Length placeholder
    message.resize(4, 0);

    // Version and type
    message.push_back(PROTOCOL_VERSION);
    message.push_back(MSG_TYPE_HEARTBEAT);

    // Payload: [8: Timestamp][1: Sequence]
    write_uint64_be(message, timestamp);
    message.push_back(sequence);

    // Fill length
    uint32_t length = message.size();
    message[0] = (length >> 24) & 0xFF;
    message[1] = (length >> 16) & 0xFF;
    message[2] = (length >> 8) & 0xFF;
    message[3] = length & 0xFF;

    // Send
    int32_t sent = socket_send_locked(message.data(), message.size());
    if (sent != (int32_t)message.size()) {
        out.printerr("Failed to send heartbeat echo: sent %d/%d bytes\n", sent, (int)message.size());
        return false;
    }

    return true;
}

// Message receive loop - handle incoming messages from server
void message_receive_loop(color_ostream &out)
{
    // Initialize heartbeat timestamp on connection
    g_last_heartbeat_ms = get_time_ms();

    while (!g_stop_message_loop && g_connected && g_socket && g_socket->IsSocketValid()) {
        std::vector<uint8_t> payload;
        uint8_t msg_type;

        // Set socket timeout to allow checking connection status
        g_socket->SetReceiveTimeout(1, 0);  // 1 second

        if (!receive_message(payload, msg_type)) {
            // Timeout or error - check for heartbeat timeout
            if (g_last_heartbeat_ms > 0) {
                uint64_t now = get_time_ms();
                uint64_t elapsed = now - g_last_heartbeat_ms;

                if (elapsed > HEARTBEAT_TIMEOUT_MS) {
                    out.printerr("Heartbeat timeout detected (%llu ms since last heartbeat)\n", elapsed);
                    // We ARE the message thread — must not join ourselves.
                    teardown_from_socket_thread();
                    return;
                }
            }
            continue;
        }

        // Handle message based on type
        switch (msg_type) {
            case MSG_TYPE_RESYNC_REQUEST:
                out.print("Received RESYNC_REQUEST — queued\n");
                g_resync_requested = true;
                drain_from_socket_thread();
                break;

            case MSG_TYPE_COMMAND:
                out.print("Received COMMAND\n");
                handleCommand(payload);
                drain_from_socket_thread();
                break;

            case MSG_TYPE_QUERY: {
                // Payload: [4: queryID] [2: nameLen] [N: name] [2: argsLen] [M: args]
                if (payload.size() < 8) {
                    out.printerr("QUERY payload too short\n");
                    break;
                }
                uint32_t queryID = ((uint32_t)payload[0] << 24) | ((uint32_t)payload[1] << 16) |
                                   ((uint32_t)payload[2] << 8)  |  (uint32_t)payload[3];
                uint16_t nameLen = ((uint16_t)payload[4] << 8) | payload[5];
                if (payload.size() < (size_t)(6 + nameLen + 2)) {
                    out.printerr("QUERY name length out of range\n");
                    break;
                }
                std::string name(payload.begin() + 6, payload.begin() + 6 + nameLen);
                size_t off = 6 + nameLen;
                uint16_t argsLen = ((uint16_t)payload[off] << 8) | payload[off + 1];
                off += 2;
                if (payload.size() < off + argsLen) {
                    out.printerr("QUERY args length out of range\n");
                    break;
                }
                std::string args(payload.begin() + off, payload.begin() + off + argsLen);

                // Queue, then drain immediately (works while DF is paused).
                {
                    std::lock_guard<std::mutex> lock(g_query_queue_mutex);
                    g_query_queue.push(QueuedQuery{queryID, name, args});
                }
                out.print("Received QUERY %s (id=%u)\n", name.c_str(), queryID);
                drain_from_socket_thread();
                break;
            }

            case MSG_TYPE_HEARTBEAT:
                // Parse heartbeat: [8: timestamp] [1: sequence]
                if (payload.size() >= 9) {
                    uint64_t timestamp = ((uint64_t)payload[0] << 56) |
                                        ((uint64_t)payload[1] << 48) |
                                        ((uint64_t)payload[2] << 40) |
                                        ((uint64_t)payload[3] << 32) |
                                        ((uint64_t)payload[4] << 24) |
                                        ((uint64_t)payload[5] << 16) |
                                        ((uint64_t)payload[6] << 8) |
                                        ((uint64_t)payload[7]);
                    uint8_t sequence = payload[8];

                    // Update last heartbeat time
                    g_last_heartbeat_ms = get_time_ms();

                    // Echo heartbeat back
                    if (!send_heartbeat_echo(out, timestamp, sequence)) {
                        out.printerr("Failed to echo heartbeat\n");
                    }

                    // Auto-update logic: send updates every 10 heartbeats (~10 seconds)
                    if (g_auto_update_enabled) {
                        g_heartbeat_counter++;
                        if (g_heartbeat_counter >= 10) {
                            g_heartbeat_counter = 0;

                            // Send tile updates
                            std::vector<uint8_t> tile_changes = detect_tile_changes();
                            if (!tile_changes.empty()) {
                                send_tile_update(out, tile_changes);
                            }

                            // Send entity updates
                            std::vector<uint32_t> dead_ids;
                            std::vector<EntityInfo> entities = extract_entities(&dead_ids);
                            std::vector<uint8_t> entity_msg = serialize_entity_update(entities, dead_ids);
                            if (g_socket && g_socket->IsSocketValid()) {
                                socket_send_locked(entity_msg.data(), entity_msg.size());
                            }
                        }
                    }
                }
                break;

            case MSG_TYPE_DISCONNECT:
                out.print("Server requested disconnect\n");
                // We ARE the message thread — must not join ourselves.
                teardown_from_socket_thread();
                return;

            default:
                out.print("Received unknown message type: 0x%02X\n", msg_type);
                break;
        }
    }
}

// Helper: Read exactly n bytes from socket
bool read_exact(uint8_t *buffer, size_t n)
{
    if (!g_socket || !g_socket->IsSocketValid()) {
        return false;
    }

    size_t total_read = 0;
    while (total_read < n) {
        int32_t received = g_socket->Receive(n - total_read);
        if (received <= 0) {
            return false;  // Error or connection closed
        }

        // Copy received data
        const uint8_t *data = g_socket->GetData();
        memcpy(buffer + total_read, data, received);
        total_read += received;
    }

    return true;
}

// Extract full map state
// Implemented in tile_extractor.cpp
extern std::vector<uint8_t> extract_full_map_state();
extern bool get_map_dimensions(int32_t &width, int32_t &height, int32_t &depth);
// Implemented in tile_updates.cpp
extern std::vector<uint8_t> detect_tile_changes();
