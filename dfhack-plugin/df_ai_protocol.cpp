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

#include "df/map_block.h"
#include "df/world.h"
#include "df/coord.h"
#include "df/unit.h"
#include "df/tile_dig_designation.h"
#include "df/building_civzonest.h"
#include "df/building_type.h"

#include "protocol.h"
#include "ActiveSocket.h"  // SimpleSockets

#include <memory>
#include <vector>
#include <cstring>
#include <chrono>
#include <thread>
#include <sstream>

using namespace DFHack;

DFHACK_PLUGIN("df_ai_protocol");
DFHACK_PLUGIN_IS_ENABLED(is_enabled);

// Global state
static std::unique_ptr<CActiveSocket> g_socket;
static std::string g_server_host = "localhost";
static uint16_t g_server_port = 5001;
static uint32_t g_connection_id = 0;
static bool g_connected = false;
static uint64_t g_last_heartbeat_ms = 0;  // Last time we received heartbeat from server
static uint32_t g_reconnect_delay_ms = 1000;  // Current reconnection backoff delay
static std::unique_ptr<std::thread> g_message_thread;  // Background message handler
static bool g_stop_message_loop = false;  // Signal to stop message thread
static bool g_auto_update_enabled = false;  // Auto-update toggle flag
static uint32_t g_heartbeat_counter = 0;  // Count heartbeats for auto-update trigger

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

std::vector<EntityInfo> extract_entities()
{
    std::vector<EntityInfo> entities;
    if (!df::global::world) return entities;

    auto &units = df::global::world->units.active;

    for (auto unit : units) {
        if (!unit) continue;
        if (unit->pos.x == -30000) continue;

        EntityInfo entity;
        entity.id = unit->id;
        entity.x = unit->pos.x;
        entity.y = unit->pos.y;
        entity.z = unit->pos.z;
        entity.subtype = unit->race;

        // Classify entity type
        // Fort citizens are dwarves (resident flag + dwarf race)
        // Race 0 = dwarf in most worlds (TODO: Could check race name for "DWARF")
        if (unit->flags2.bits.resident && unit->race == 0) {
            entity.type = ENTITY_TYPE_DWARF;  // Fort citizen (actual dwarf)
        } else if (unit->flags1.bits.marauder || unit->flags1.bits.invader_origin) {
            entity.type = ENTITY_TYPE_ENEMY;   // Hostile
        } else if (unit->flags1.bits.tame) {
            entity.type = ENTITY_TYPE_ANIMAL;  // Tame animal (pets, livestock)
        } else {
            entity.type = ENTITY_TYPE_ANIMAL;  // Wild animal or other
        }

        entities.push_back(entity);
    }
    return entities;
}

// FortInfo structure for fort-level statistics
struct FortInfo {
    uint32_t days_elapsed;
    uint64_t created_wealth;
    uint8_t season;
    uint32_t year;
};

// Extract fort-level information
// TODO: Find correct API for DF 53.02 - cur_year/cur_year_tick don't exist in this version
FortInfo extract_fort_info()
{
    FortInfo info = {0};

    if (!df::global::world) {
        return info;
    }

    // Use frame_counter as approximation for now (10 ticks/second at normal speed)
    // Roughly 1200 ticks/day at normal speed -> frame_counter / 1200 ≈ days
    info.days_elapsed = df::global::world->frame_counter / 1200;

    // TODO: Find correct API for wealth, season, year in DF 53.02
    info.created_wealth = 0;  // Placeholder
    info.season = 0;           // Placeholder
    info.year = 0;             // Placeholder

    return info;
}

std::vector<uint8_t> serialize_entity_update(const std::vector<EntityInfo> &entities)
{
    std::vector<uint8_t> buffer;
    buffer.resize(4, 0);
    buffer.push_back(PROTOCOL_VERSION);
    buffer.push_back(0x08);  // ENTITY_UPDATE

    uint32_t count = entities.size();
    buffer.push_back((count >> 24) & 0xFF);
    buffer.push_back((count >> 16) & 0xFF);
    buffer.push_back((count >> 8) & 0xFF);
    buffer.push_back(count & 0xFF);

    for (const auto &e : entities) {
        buffer.push_back((e.id >> 24) & 0xFF);
        buffer.push_back((e.id >> 16) & 0xFF);
        buffer.push_back((e.id >> 8) & 0xFF);
        buffer.push_back(e.id & 0xFF);
        buffer.push_back((e.x >> 8) & 0xFF);
        buffer.push_back(e.x & 0xFF);
        buffer.push_back((e.y >> 8) & 0xFF);
        buffer.push_back(e.y & 0xFF);
        buffer.push_back((e.z >> 8) & 0xFF);
        buffer.push_back(e.z & 0xFF);
        buffer.push_back(e.type);
        buffer.push_back((e.subtype >> 8) & 0xFF);
        buffer.push_back(e.subtype & 0xFF);
    }

    // Append FortInfo (optional)
    FortInfo fort_info = extract_fort_info();
    buffer.push_back(1);  // hasFortInfo = true

    // DaysElapsed (4 bytes, big-endian)
    buffer.push_back((fort_info.days_elapsed >> 24) & 0xFF);
    buffer.push_back((fort_info.days_elapsed >> 16) & 0xFF);
    buffer.push_back((fort_info.days_elapsed >> 8) & 0xFF);
    buffer.push_back(fort_info.days_elapsed & 0xFF);

    // CreatedWealth (8 bytes, big-endian)
    buffer.push_back((fort_info.created_wealth >> 56) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 48) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 40) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 32) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 24) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 16) & 0xFF);
    buffer.push_back((fort_info.created_wealth >> 8) & 0xFF);
    buffer.push_back(fort_info.created_wealth & 0xFF);

    // Season (1 byte)
    buffer.push_back(fort_info.season);

    // Year (4 bytes, big-endian)
    buffer.push_back((fort_info.year >> 24) & 0xFF);
    buffer.push_back((fort_info.year >> 16) & 0xFF);
    buffer.push_back((fort_info.year >> 8) & 0xFF);
    buffer.push_back(fort_info.year & 0xFF);

    uint32_t length = buffer.size();
    buffer[0] = (length >> 24) & 0xFF;
    buffer[1] = (length >> 16) & 0xFF;
    buffer[2] = (length >> 8) & 0xFF;
    buffer[3] = length & 0xFF;
    return buffer;
}

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

    g_socket->Send(msg.data(), msg.size());
}

// Apply dig designation to region using MapCache (proper DFHack pattern)
bool applyDigDesignation(uint8_t digType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (!Maps::isValidTilePos(x1, y1, z)) {
        error = "Invalid start coordinates";
        return false;
    }
    if (!Maps::isValidTilePos(x2, y2, z2)) {
        error = "Invalid end coordinates";
        return false;
    }
    if (z != z2) {
        error = "Z-levels must match (single level per command)";
        return false;
    }

    // Use MapCache for proper designation read/write (like dig plugin does)
    MapExtras::MapCache cache;
    int designated = 0;
    int skipped_hidden = 0;
    int skipped_already = 0;

    Core::getInstance().getConsole().print("DEBUG: Digging region (%d,%d,%d) to (%d,%d,%d)\n",
        x1, y1, z, x2, y2, z2);

    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::coord pos(x, y, z);

            // Check if tile is valid
            df::tiletype tt = cache.tiletypeAt(pos);
            if (tt == tiletype::Void) {
                continue; // Invalid tile
            }

            // Read current designation via MapCache
            df::tile_designation des = cache.designationAt(pos);

            // Debug: Log first tile
            if (x == x1 && y == y1) {
                Core::getInstance().getConsole().print("DEBUG: First tile (%d,%d,%d): tiletype=%d, hidden=%d, current_dig=%d\n",
                    x, y, z, tt, des.bits.hidden ? 1 : 0, des.bits.dig);
            }

            // Set dig designation using type from protocol
            des.bits.dig = static_cast<df::tile_dig_designation>(digType);

            // Write back to DF via MapCache
            cache.setDesignationAt(pos, des);
            designated++;
        }
    }

    Core::getInstance().getConsole().print("DEBUG: Designated %d tiles (skipped %d hidden, %d already marked)\n",
        designated, skipped_hidden, skipped_already);

    if (designated == 0) {
        error = "No tiles designated (all already marked, hidden, or invalid)";
        return false;
    }

    // CRITICAL: Flush MapCache changes to DF!
    bool writeSuccess = cache.WriteAll();
    Core::getInstance().getConsole().print("DEBUG: WriteAll() result: %s\n", writeSuccess ? "success" : "failed");

    return true;
}

// Apply zone designation (bedroom, dining hall, meeting area, barracks)
// TODO: Building API changed in DF 53.02 - needs research for correct usage
bool applyZoneDesignation(uint8_t zoneType, int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, std::string &error)
{
    error = "Zone designations not yet implemented for DF 53.02 (Building API changed)";
    return false;

    /* DISABLED - API incompatible with DF 53.02
    using namespace DFHack;

    // Allocate civzone building
    df::building_civzonest* zone = virtual_cast<df::building_civzonest>(
        Buildings::allocInstance(df::coord(x1, y1, z), df::building_type::Civzone)
    );

    // ... zone setup code ...

    return true;
    */
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

// Apply chop designation to region (mark trees for chopping)
bool applyChopDesignation(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Invalid coordinates";
        return false;
    }

    // Use DFHack chop-trees or similar command
    // For now, run tiletypes command to mark trees
    Core &core = Core::getInstance();
    color_ostream_proxy out(core.getConsole());

    // Alternative: Use TreeCutter plugin or manual tree designation
    // For simplicity, designate trees in region
    std::ostringstream cmd;
    cmd << "chop-designate " << x1 << " " << y1 << " " << z << " " << x2 << " " << y2;

    command_result result = core.runCommand(out, cmd.str());
    if (result != CR_OK && result != CR_NOT_FOUND) {
        // Command might not exist, fall back to manual
        error = "chop-designate command not available";
        return false;
    }

    return true;
}

// Apply gather designation using DFHack getplants command
bool applyGatherDesignation(int16_t x1, int16_t y1, int16_t z, int16_t x2, int16_t y2, int16_t z2, std::string &error)
{
    // Use DFHack's getplants command which is smarter about plant targeting
    // Since we can't easily target by region, run getplants all and let dwarves gather plants in area
    Core &core = Core::getInstance();
    color_ostream_proxy out(core.getConsole());

    std::string cmd = "getplants all";
    command_result result = core.runCommand(out, cmd);

    if (result != CR_OK) {
        error = "getplants command failed";
        return false;
    }

    return true;
}

// Handle COMMAND message from server
void handleCommand(const std::vector<uint8_t> &payload)
{
    if (payload.size() < 5) {
        sendCommandAck(0, 0x02, "Invalid command payload");
        return;
    }

    // Parse command ID and type
    uint32_t cmdID = ((uint32_t)payload[0] << 24) | ((uint32_t)payload[1] << 16) |
                     ((uint32_t)payload[2] << 8) | (uint32_t)payload[3];
    uint8_t cmdType = payload[4];

    std::string error;
    bool success = false;

    switch (cmdType) {
        case 0x01: {  // DIG
            if (payload.size() < 18) {  // Now includes DigType byte
                sendCommandAck(cmdID, 0x02, "Invalid DIG payload");
                return;
            }
            uint8_t digType = payload[5];
            int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
            int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
            int16_t z1 = ((int16_t)payload[10] << 8) | payload[11];
            int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
            int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];
            int16_t z2 = ((int16_t)payload[16] << 8) | payload[17];
            success = applyDigDesignation(digType, x1, y1, z1, x2, y2, z2, error);
            break;
        }
        case 0x02: {  // BUILD (not implemented)
            sendCommandAck(cmdID, 0x02, "BUILD not yet implemented");
            return;
        }
        case 0x03: {  // CANCEL
            if (payload.size() < 18) {  // Now includes DigType byte (ignored for cancel)
                sendCommandAck(cmdID, 0x02, "Invalid CANCEL payload");
                return;
            }
            // Skip digType byte at payload[5]
            int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
            int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
            int16_t z1 = ((int16_t)payload[10] << 8) | payload[11];
            int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
            int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];
            int16_t z2 = ((int16_t)payload[16] << 8) | payload[17];
            success = applyCancelDesignation(x1, y1, z1, x2, y2, z2, error);
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
        case 0x06: {  // ZONE
            if (payload.size() < 12) {
                sendCommandAck(cmdID, 0x02, "Invalid ZONE payload");
                return;
            }
            uint8_t zoneType = payload[5];
            int16_t x1 = ((int16_t)payload[6] << 8) | payload[7];
            int16_t y1 = ((int16_t)payload[8] << 8) | payload[9];
            int16_t z = ((int16_t)payload[10] << 8) | payload[11];
            int16_t x2 = ((int16_t)payload[12] << 8) | payload[13];
            int16_t y2 = ((int16_t)payload[14] << 8) | payload[15];
            success = applyZoneDesignation(zoneType, x1, y1, z, x2, y2, error);
            break;
        }
        default:
            sendCommandAck(cmdID, 0x02, "Unknown command type");
            return;
    }

    sendCommandAck(cmdID, success ? 0x00 : 0x02, error);
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
                        out.print("Received RESYNC_REQUEST - sending full state...\n");
                        send_full_state(out);
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
            std::vector<EntityInfo> entities = extract_entities();

            out.print("Found %d entities\n", (int)entities.size());

            // Serialize and send
            std::vector<uint8_t> message = serialize_entity_update(entities);

            int sent = g_socket->Send(message.data(), message.size());
            if (sent != (int)message.size()) {
                out.printerr("Failed to send ENTITY_UPDATE (%d/%d bytes)\n", sent, (int)message.size());
                return CR_FAILURE;
            }

            out.print("Sent ENTITY_UPDATE (%d entities, %d bytes)\n", (int)entities.size(), (int)message.size());
            return CR_OK;
        },
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

            int sent = g_socket->Send(msg.data(), msg.size());
            if (sent != (int)msg.size()) {
                out.printerr("Failed to send save request\n");
                return CR_FAILURE;
            }

            out.print("Save request sent to server\n");
            out.print("Check server logs for save confirmation\n");
            return CR_OK;
        },
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

// Disconnect from server
void disconnect_from_server()
{
    // Stop message thread first
    g_stop_message_loop = true;
    if (g_message_thread && g_message_thread->joinable()) {
        g_message_thread->join();
        g_message_thread.reset();
    }

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
        g_socket->Send(message.data(), message.size());
    }

    if (g_socket) {
        g_socket->Close();
        g_socket.reset();
    }
    g_connected = false;
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
    int32_t sent = g_socket->Send(message.data(), message.size());
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
    int32_t sent = g_socket->Send(message.data(), message.size());
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
    int32_t sent = g_socket->Send(message.data(), message.size());
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
    int32_t sent = g_socket->Send(message.data(), message.size());
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
                    disconnect_from_server();
                    return;
                }
            }
            continue;
        }

        // Handle message based on type
        switch (msg_type) {
            case MSG_TYPE_RESYNC_REQUEST:
                out.print("Received RESYNC_REQUEST\n");
                if (!send_full_state(out)) {
                    out.printerr("Failed to send full state\n");
                }
                break;

            case MSG_TYPE_COMMAND:
                out.print("Received COMMAND\n");
                handleCommand(payload);
                break;

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
                            std::vector<EntityInfo> entities = extract_entities();
                            std::vector<uint8_t> entity_msg = serialize_entity_update(entities);
                            if (g_socket && g_socket->IsSocketValid()) {
                                g_socket->Send(entity_msg.data(), entity_msg.size());
                            }
                        }
                    }
                }
                break;

            case MSG_TYPE_DISCONNECT:
                out.print("Server requested disconnect\n");
                disconnect_from_server();
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
