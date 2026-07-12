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

    // Handle multi-Z stair shafts and single-Z digs in one unified pass.
    //
    // DF tile designations are per-tile — there is no native "shaft"
    // concept. A 2×2×50 stair shaft is just 200 independent tiles each
    // carrying tile_designation::dig. The earlier code special-cased
    // single-column shafts for no real reason; this loop handles any 3D
    // rectangle.
    //
    // Rules:
    //   - If z1 != z2 AND digType is stairs: auto-assign UpStair on the
    //     bottom Z, DownStair on the top Z, UpDownStair on middle Zs.
    //     Every (x, y) within the rectangle gets the same stair type for
    //     its Z. Matches DF's own bulk stair designation tool.
    //   - Otherwise: every tile in the rectangle gets the requested
    //     digType. For multi-Z non-stair digs (e.g. excavating a full
    //     underground complex), we still respect the per-tile dig type.
    //
    // Hidden tiles: ALL dig paths designate through hidden terrain, exactly
    // like DF's own designation UI. Every undug underground tile is hidden
    // (fog of war), so skipping hidden tiles made underground rooms
    // undesignatable. Dwarves reveal tiles as they dig; DF's job system
    // handles the rest. Bounds are already screened by isValidTilePos above.
    if (z1 > z2) std::swap(z1, z2);

    bool isStairShaft = (z1 != z2) &&
        (digType == 0x02 || digType == 0x05 || digType == 0x06);
    // 0x02=UpDownStair, 0x05=DownStair, 0x06=UpStair from the protocol.

    for (int16_t z = z1; z <= z2; z++) {
        for (int16_t x = x1; x <= x2; x++) {
            for (int16_t y = y1; y <= y2; y++) {
                df::coord pos(x, y, z);
                df::tile_designation des = cache.designationAt(pos);

                if (isStairShaft) {
                    if (z == z1) {
                        des.bits.dig = df::tile_dig_designation::UpStair;
                    } else if (z == z2) {
                        des.bits.dig = df::tile_dig_designation::DownStair;
                    } else {
                        des.bits.dig = df::tile_dig_designation::UpDownStair;
                    }
                    // Span hidden terrain by design.
                    cache.setDesignationAt(pos, des);
                    designated++;
                } else {
                    // Hidden tiles are designated like DF's own UI does — fog
                    // of war is where forts get dug. Nothing increments
                    // blocked here today; it stays for future per-tile
                    // rejection paths (e.g. non-diggable screening).
                    des.bits.dig = dfDigType;
                    cache.setDesignationAt(pos, des);
                    designated++;
                }
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

// Forward declarations of category placers (implemented in buildings.cpp).
extern bool placeWorkshop(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error);
extern bool placeFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error);
extern bool placeConstruction(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error);
extern bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, std::string &error);

// applyBuildDesignation dispatches BUILD commands to category-specific
// placers based on the BuildType byte's range:
//   0x01-0x0F → constructions (wall, floor, stairs, ramp)
//   0x10-0x2F → workshops
//   0x30-0x4F → furniture
//   0x50-0x6F → doors / hatches
bool applyBuildDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [1: BuildType]
    if (payload.size() < 12) {
        error = "Invalid build payload size";
        return false;
    }

    int16_t x = read_int16_be(payload, 5);
    int16_t y = read_int16_be(payload, 7);
    int16_t z = read_int16_be(payload, 9);
    uint8_t buildType = payload[11];

    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (isBuildTypeWorkshop(buildType)) {
        return placeWorkshop(x, y, z, buildType, error);
    }
    if (isBuildTypeFurniture(buildType)) {
        return placeFurniture(x, y, z, buildType, error);
    }
    if (isBuildTypeConstruction(buildType)) {
        return placeConstruction(x, y, z, buildType, error);
    }
    if (isBuildTypeDoor(buildType)) {
        return placeDoor(x, y, z, buildType, error);
    }

    char buf[64];
    snprintf(buf, sizeof(buf), "Unknown build type: 0x%02X", buildType);
    error = buf;
    return false;
}

// Apply smooth/engrave designation to a rectangular region on a single
// Z-level. Sets df::tile_designation::smooth = 1 (smooth) or 2 (engrave).
//
// Caveats: smooth/engrave only applies to natural stone walls and floors.
// DF's labor system silently skips invalid targets (soil, sand, gravel,
// constructed walls). The plugin sets the bit on every requested tile;
// dwarves with the appropriate labor enabled will pick up the valid jobs.
//
// Use case: sealing light aquifer leaks. Aquifer tiles in stone layers
// stop weeping water once their walls and ceilings are smoothed. For dirt
// or soil aquifer layers, smooth has no effect — replace with a
// constructed wall (BuildTypeWall) or just dig past the layer instead.
bool applySmoothDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse: [4:CmdID] [1:CmdType] [1:SmoothType] [2:X1] [2:Y1] [2:Z] [2:X2] [2:Y2]
    if (payload.size() < 16) {
        error = "Invalid smooth payload size";
        return false;
    }

    uint8_t smoothType = payload[5];
    if (smoothType != 0x01 && smoothType != 0x02) {
        error = "Invalid smooth type (must be 1=smooth, 2=engrave)";
        return false;
    }

    int16_t x1 = read_int16_be(payload, 6);
    int16_t y1 = read_int16_be(payload, 8);
    int16_t z  = read_int16_be(payload, 10);
    int16_t x2 = read_int16_be(payload, 12);
    int16_t y2 = read_int16_be(payload, 14);

    if (!Maps::isValidTilePos(x1, y1, z) || !Maps::isValidTilePos(x2, y2, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }
    if (x2 < x1 || y2 < y1) {
        error = "Invalid region bounds";
        return false;
    }

    MapExtras::MapCache cache;
    int designated = 0;
    int blocked = 0;

    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::coord pos(x, y, z);
            df::tile_designation des = cache.designationAt(pos);

            if (des.bits.hidden) {
                blocked++;
                continue;
            }

            // smooth field is 2 bits: 0=none, 1=smooth, 2=engrave.
            des.bits.smooth = smoothType;
            cache.setDesignationAt(pos, des);
            designated++;
        }
    }

    if (!cache.WriteAll()) {
        error = "Failed to commit smooth designations to map";
        return false;
    }

    if (designated == 0) {
        error = "No tiles designated (all hidden)";
        return false;
    }

    if (blocked > 0) {
        char buf[128];
        snprintf(buf, sizeof(buf), "%d of %d tiles hidden", blocked, designated + blocked);
        error = buf;
    }
    return true;
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
