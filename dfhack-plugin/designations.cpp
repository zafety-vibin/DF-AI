// DFHack Command Handling - Designation Commands
// Handles COMMAND messages from server and applies dig/build/cancel designations

#include "Core.h"
#include "Console.h"
#include "modules/Maps.h"
#include "modules/MapCache.h"

#include "df/map_block.h"
#include "df/tile_dig_designation.h"
#include "df/world.h"

#include "protocol.h"
#include "ActiveSocket.h"

#include <vector>
#include <string>
#include <cstring>
#include <algorithm>

using namespace DFHack;

// External global socket
extern std::unique_ptr<CActiveSocket> g_socket;

// External functions from df_ai_protocol.cpp
extern void sendCommandAck(uint32_t cmdID, uint8_t status, const std::string &error);

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

// Apply dig designation to a region
bool applyDigDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse: [4:CmdID] [1:CmdType] [1:DigType] [2:X1] [2:Y1] [2:Z1] [2:X2] [2:Y2] [2:Z2]
    if (payload.size() < 18) {  // 4(cmdID) + 1(type) + 1(digType) + 12(region)
        error = "Invalid dig payload size";
        return false;
    }

    // Read DigType byte (CRITICAL FIX: was being skipped!)
    uint8_t digType = payload[5];

    // Read coordinates (offset by 1 to account for DigType byte)
    int16_t x1 = read_int16_be(payload, 6);
    int16_t y1 = read_int16_be(payload, 8);
    int16_t z1 = read_int16_be(payload, 10);
    int16_t x2 = read_int16_be(payload, 12);
    int16_t y2 = read_int16_be(payload, 14);
    int16_t z2 = read_int16_be(payload, 16);

    // Validate bounds
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (x2 < x1 || y2 < y1) {
        error = "Invalid region bounds";
        return false;
    }

    // Map protocol DigType to DFHack tile_dig_designation
    // Protocol: 0x01=Default, 0x02=UpDownStair, 0x03=Channel, 0x04=Ramp, 0x05=DownStair, 0x06=UpStair
    df::tile_dig_designation dfDigType;
    switch (digType) {
        case 0x01: dfDigType = df::tile_dig_designation::Default; break;
        case 0x03: dfDigType = df::tile_dig_designation::Channel; break;
        case 0x04: dfDigType = df::tile_dig_designation::Ramp; break;
        case 0x05: dfDigType = df::tile_dig_designation::DownStair; break;
        case 0x06: dfDigType = df::tile_dig_designation::UpStair; break;
        case 0x02: // UpDownStair handled by vertical shaft logic below
        default:
            dfDigType = df::tile_dig_designation::Default;
            break;
    }

    // Initialize MapCache for thread-safe map access
    MapExtras::MapCache cache;

    int designated = 0;
    int blocked = 0;

    // Handle vertical shaft (staircase) if z1 != z2
    if (z1 != z2) {
        // Vertical shafts must be single column
        if (x1 != x2 || y1 != y2) {
            error = "Vertical regions must be single column (x1==x2, y1==y2) for staircases";
            return false;
        }

        // Ensure z1 <= z2
        if (z1 > z2) std::swap(z1, z2);

        // Designate staircase from z1 (bottom) to z2 (top)
        for (int16_t z = z1; z <= z2; z++) {
            df::coord pos(x1, y1, z);
            df::tile_designation des = cache.designationAt(pos);

            if (des.bits.hidden) {
                blocked++;
                continue;
            }

            // Set appropriate stair type based on position in shaft
            if (z == z1) {
                // Bottom of shaft: UpStair
                des.bits.dig = df::tile_dig_designation::UpStair;
            } else if (z == z2) {
                // Top of shaft: DownStair
                des.bits.dig = df::tile_dig_designation::DownStair;
            } else {
                // Middle levels: UpDownStair
                des.bits.dig = df::tile_dig_designation::UpDownStair;
            }

            cache.setDesignationAt(pos, des);
            designated++;
        }
    } else {
        // Single z-level region: use specified dig type
        int16_t z = z1;

        for (int16_t x = x1; x <= x2; x++) {
            for (int16_t y = y1; y <= y2; y++) {
                df::coord pos(x, y, z);
                df::tile_designation des = cache.designationAt(pos);

                if (des.bits.hidden) {
                    blocked++;
                    continue;
                }

                // Set dig designation based on DigType parameter
                des.bits.dig = dfDigType;
                cache.setDesignationAt(pos, des);
                designated++;
            }
        }
    }

    // CRITICAL: Commit all changes to game state
    if (!cache.WriteAll()) {
        error = "Failed to commit designations to map";
        return false;
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
    // Parse: [4:CmdID] [1:CmdType] [1:DigType] [2:X1] [2:Y1] [2:Z1] [2:X2] [2:Y2] [2:Z2]
    if (payload.size() < 18) {
        error = "Invalid cancel payload size";
        return false;
    }

    // DigType byte is present but ignored for cancel (offset 5)
    // uint8_t digType = payload[5];  // Not used for cancel

    int16_t x1 = read_int16_be(payload, 6);
    int16_t y1 = read_int16_be(payload, 8);
    int16_t z1 = read_int16_be(payload, 10);
    int16_t x2 = read_int16_be(payload, 12);
    int16_t y2 = read_int16_be(payload, 14);
    int16_t z2 = read_int16_be(payload, 16);

    // Note: Cancel can work across z-levels (no restriction like old code had)

    // Validate bounds
    if (!Maps::isValidTilePos(x1, y1, z1) || !Maps::isValidTilePos(x2, y2, z2)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (x2 < x1 || y2 < y1 || z2 < z1) {
        error = "Invalid region bounds";
        return false;
    }

    // Initialize MapCache for thread-safe map access
    MapExtras::MapCache cache;

    int cancelled = 0;

    // Clear designations in region (support multi-level cancellation)
    for (int16_t z = z1; z <= z2; z++) {
        for (int16_t x = x1; x <= x2; x++) {
            for (int16_t y = y1; y <= y2; y++) {
                df::coord pos(x, y, z);
                df::tile_designation des = cache.designationAt(pos);

                // Clear dig designation if present
                if (des.bits.dig != df::tile_dig_designation::No) {
                    des.bits.dig = df::tile_dig_designation::No;
                    cache.setDesignationAt(pos, des);
                    cancelled++;
                }

                // TODO: Clear build designations as well when implemented
            }
        }
    }

    // Commit changes
    if (!cache.WriteAll()) {
        error = "Failed to commit cancellations to map";
        return false;
    }

    if (cancelled == 0) {
        error = "No designations to cancel in region";
        return false;
    }

    return true;
}
