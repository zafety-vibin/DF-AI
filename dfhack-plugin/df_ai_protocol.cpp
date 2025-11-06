// DFHack Binary Protocol Plugin
// Feature: 001-binary-protocol
// Target: DFHack 53.02-r1, DF 53.02

#include "Core.h"
#include "Console.h"
#include "Export.h"
#include "PluginManager.h"
#include "modules/MapCache.h"

#include "df/map_block.h"
#include "df/world.h"
#include "df/coord.h"

#include "protocol.h"
#include "ActiveSocket.h"  // SimpleSockets

#include <memory>
#include <vector>
#include <cstring>

using namespace DFHack;

DFHACK_PLUGIN("df_ai_protocol");
DFHACK_PLUGIN_IS_ENABLED(is_enabled);

// Global state
static std::unique_ptr<CActiveSocket> g_socket;
static std::string g_server_host = "localhost";
static uint16_t g_server_port = 5001;
static uint32_t g_connection_id = 0;
static bool g_connected = false;

// Forward declarations
bool connect_to_server(color_ostream &out);
void disconnect_from_server();
bool send_handshake(color_ostream &out);
bool receive_handshake(color_ostream &out);
void message_receive_loop(color_ostream &out);
bool receive_message(std::vector<uint8_t> &msg_out, uint8_t &type_out);
bool send_full_state(color_ostream &out);
std::vector<uint8_t> extract_full_map_state();

// Helper: Read exactly n bytes from socket
bool read_exact(uint8_t *buffer, size_t n);
// Helper: Write uint16 big-endian
void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value);
// Helper: Write uint32 big-endian
void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value);

// Plugin initialization
DFhackCExport command_result plugin_init(color_ostream &out, std::vector<PluginCommand> &commands)
{
    commands.push_back(PluginCommand(
        "ai-connect",
        "Connect to Go orchestrator server",
        [](color_ostream &out, std::vector<std::string> &params) -> command_result {
            if (connect_to_server(out)) {
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
    out.print("Handshake complete - connected successfully\n");

    // Start message receive loop in background
    // Note: In a real implementation, this would be a separate thread
    // For now, we'll handle messages synchronously when commands are called

    return true;
}

// Disconnect from server
void disconnect_from_server()
{
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

// Message receive loop - handle incoming messages from server
void message_receive_loop(color_ostream &out)
{
    while (g_connected && g_socket && g_socket->IsSocketValid()) {
        std::vector<uint8_t> payload;
        uint8_t msg_type;

        // Set socket timeout to allow checking connection status
        g_socket->SetReceiveTimeout(1, 0);  // 1 second

        if (!receive_message(payload, msg_type)) {
            // Timeout or error
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

            case MSG_TYPE_HEARTBEAT:
                // TODO: Echo heartbeat back (User Story 3)
                out.print("Received HEARTBEAT (US3 - not yet implemented)\n");
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

// Helper: Write uint16 big-endian
void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value)
{
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Helper: Write uint32 big-endian
void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value)
{
    buf.push_back((value >> 24) & 0xFF);
    buf.push_back((value >> 16) & 0xFF);
    buf.push_back((value >> 8) & 0xFF);
    buf.push_back(value & 0xFF);
}

// Extract full map state
// Implemented in tile_extractor.cpp
extern std::vector<uint8_t> extract_full_map_state();
extern bool get_map_dimensions(int32_t &width, int32_t &height, int32_t &depth);
