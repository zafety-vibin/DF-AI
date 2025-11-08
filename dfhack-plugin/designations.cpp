// DFHack Command Handling - Designation Commands
// Handles COMMAND messages from server and applies dig/build/cancel designations

#include "Core.h"
#include "Console.h"
#include "modules/Maps.h"

#include "df/map_block.h"
#include "df/tile_dig_designation.h"
#include "df/world.h"

#include "protocol.h"
#include "ActiveSocket.h"

#include <vector>
#include <string>
#include <cstring>

using namespace DFHack;

// External global socket
extern std::unique_ptr<CActiveSocket> g_socket;

// Helper function to read big-endian uint32
uint32_t read_uint32_be(const std::vector<uint8_t> &data, size_t offset)
{
    return (static_cast<uint32_t>(data[offset]) << 24) |
           (static_cast<uint32_t>(data[offset + 1]) << 16) |
           (static_cast<uint32_t>(data[offset + 2]) << 8) |
           static_cast<uint32_t>(data[offset + 3]);
}

// Helper function to read big-endian int16
int16_t read_int16_be(const std::vector<uint8_t> &data, size_t offset)
{
    uint16_t value = (static_cast<uint16_t>(data[offset]) << 8) |
                     static_cast<uint16_t>(data[offset + 1]);
    return static_cast<int16_t>(value);
}

// Send command acknowledgment back to server
void sendCommandAck(uint32_t cmdID, uint8_t status, const std::string &error)
{
    if (!g_socket || !g_socket->IsSocketValid()) {
        return;
    }

    std::vector<uint8_t> msg;
    msg.resize(4, 0);  // Length header (filled later)

    // Version + Type
    msg.push_back(PROTOCOL_VERSION);
    msg.push_back(MSG_TYPE_COMMAND_ACK);

    // Payload: [4: CommandID] [1: Status] [2: ErrorMsgLen] [N: ErrorMsg]
    write_uint32_be(msg, cmdID);
    msg.push_back(status);

    // Error message
    uint16_t errorLen = error.empty() ? 0 : static_cast<uint16_t>(error.size());
    write_uint16_be(msg, errorLen);
    if (errorLen > 0) {
        msg.insert(msg.end(), error.begin(), error.end());
    }

    // Fill length header
    uint32_t length = msg.size();
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    // Send
    g_socket->Send(msg.data(), msg.size());
}

// Apply dig designation to a region
bool applyDigDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse region: [2:X1] [2:Y1] [2:Z1] [2:X2] [2:Y2] [2:Z2]
    if (payload.size() < 17) {  // 4(cmdID) + 1(type) + 12(region)
        error = "Invalid dig payload size";
        return false;
    }

    int16_t x1 = read_int16_be(payload, 5);
    int16_t y1 = read_int16_be(payload, 7);
    int16_t z1 = read_int16_be(payload, 9);
    int16_t x2 = read_int16_be(payload, 11);
    int16_t y2 = read_int16_be(payload, 13);
    int16_t z2 = read_int16_be(payload, 15);

    // Validate Z-level requirement (single level)
    if (z1 != z2) {
        error = "Region must be on single Z-level";
        return false;
    }

    // Validate bounds
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (x2 < x1 || y2 < y1) {
        error = "Invalid region bounds";
        return false;
    }

    int16_t z = z1;
    int designated = 0;
    int blocked = 0;

    // Apply designation to each tile in region
    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::map_block *block = Maps::getTileBlock(x, y, z);
            if (!block) {
                blocked++;
                continue;
            }

            // Get local coordinates within block (0-15)
            int local_x = x & 0x0F;
            int local_y = y & 0x0F;

            // Check if tile is hidden (fog of war)
            if (block->designation[local_x][local_y].bits.hidden) {
                blocked++;
                continue;
            }

            // Set dig designation to Default (standard mining)
            block->designation[local_x][local_y].bits.dig = df::tile_dig_designation::Default;
            designated++;
        }
    }

    if (designated == 0) {
        error = "No tiles designated (all blocked or hidden)";
        return false;
    }

    if (blocked > 0 && designated > 0) {
        // Partial success
        char buf[256];
        snprintf(buf, sizeof(buf), "%d of %d tiles blocked or hidden",
                 blocked, designated + blocked);
        error = buf;
        // Still return true for partial success
    }

    return true;
}

// Apply build designation (stub for now - BUILD is lower priority)
bool applyBuildDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse build: [2:X] [2:Y] [2:Z] [1:BuildType]
    if (payload.size() < 12) {  // 4(cmdID) + 1(type) + 7(build data)
        error = "Invalid build payload size";
        return false;
    }

    int16_t x = read_int16_be(payload, 5);
    int16_t y = read_int16_be(payload, 7);
    int16_t z = read_int16_be(payload, 9);
    uint8_t buildType = payload[11];

    // Validate coordinates
    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    // TODO: Implement actual build designation
    // For now, just return success as a stub
    error = "Build designation not yet implemented";
    return false;
}

// Apply cancel designation to a region
bool applyCancelDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse region (same as dig): [2:X1] [2:Y1] [2:Z1] [2:X2] [2:Y2] [2:Z2]
    if (payload.size() < 17) {
        error = "Invalid cancel payload size";
        return false;
    }

    int16_t x1 = read_int16_be(payload, 5);
    int16_t y1 = read_int16_be(payload, 7);
    int16_t z1 = read_int16_be(payload, 9);
    int16_t x2 = read_int16_be(payload, 11);
    int16_t y2 = read_int16_be(payload, 13);
    int16_t z2 = read_int16_be(payload, 15);

    // Validate Z-level requirement
    if (z1 != z2) {
        error = "Region must be on single Z-level";
        return false;
    }

    // Validate bounds
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (x2 < x1 || y2 < y1) {
        error = "Invalid region bounds";
        return false;
    }

    int16_t z = z1;
    int cancelled = 0;

    // Clear designations in region
    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::map_block *block = Maps::getTileBlock(x, y, z);
            if (!block) {
                continue;
            }

            int local_x = x & 0x0F;
            int local_y = y & 0x0F;

            // Clear dig designation
            if (block->designation[local_x][local_y].bits.dig != df::tile_dig_designation::No) {
                block->designation[local_x][local_y].bits.dig = df::tile_dig_designation::No;
                cancelled++;
            }

            // TODO: Clear build designations as well when implemented
        }
    }

    if (cancelled == 0) {
        error = "No designations to cancel in region";
        return false;
    }

    return true;
}

// Handle incoming COMMAND message
void handleCommand(const std::vector<uint8_t> &payload)
{
    if (payload.size() < 5) {
        return;  // Invalid payload
    }

    // Parse header: [4: CommandID] [1: CommandType]
    uint32_t cmdID = read_uint32_be(payload, 0);
    uint8_t cmdType = payload[4];

    bool success = false;
    std::string error = "";
    uint8_t status = ACK_STATUS_FAILURE;

    // Dispatch based on command type
    switch (cmdType) {
        case COMMAND_TYPE_DIG:
            success = applyDigDesignation(payload, error);
            if (success) {
                status = error.empty() ? ACK_STATUS_SUCCESS : ACK_STATUS_PARTIAL;
            }
            break;

        case COMMAND_TYPE_BUILD:
            success = applyBuildDesignation(payload, error);
            status = success ? ACK_STATUS_SUCCESS : ACK_STATUS_FAILURE;
            break;

        case COMMAND_TYPE_CANCEL:
            success = applyCancelDesignation(payload, error);
            status = success ? ACK_STATUS_SUCCESS : ACK_STATUS_FAILURE;
            break;

        default:
            char buf[64];
            snprintf(buf, sizeof(buf), "Unknown command type: 0x%02X", cmdType);
            error = buf;
            status = ACK_STATUS_FAILURE;
            break;
    }

    // Send acknowledgment
    sendCommandAck(cmdID, status, error);
}
