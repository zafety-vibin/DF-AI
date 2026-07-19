// DFHack Command Handling - Designation Commands
// Handles COMMAND messages from server and applies dig/build/cancel designations

#include "Core.h"
#include "Console.h"
#include "TileTypes.h"
#include "modules/Maps.h"
#include "modules/MapCache.h"

#include "df/map_block.h"
#include "df/tile_dig_designation.h"
#include "df/tiletype_shape.h"
#include "df/world.h"
#include "df/building.h"
#include "df/building_type.h"

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

// True when the shape is any carved stair (up, down, or up/down).
// Shapes come from DFHack's tileShape() attribute lookup (TileTypes.h) — a
// cheap enum-attr read, safe inside the per-tile loops below. Never use
// raw tiletype ranges for this.
static bool isStairShape(df::tiletype_shape shape)
{
    return shape == df::tiletype_shape::STAIR_UP ||
           shape == df::tiletype_shape::STAIR_DOWN ||
           shape == df::tiletype_shape::STAIR_UPDOWN;
}

// Does the tile directly ABOVE (x, y, zTop) already provide a downward
// stair connection — carved into the terrain or queued as a dig
// designation? If so, the top of a new stair range must be UpDownStair,
// not DownStair: z-movement needs an up component on the lower tile AND a
// down component on the upper tile, so a bare DownStair under an existing
// shaft leaves the whole new section permanently unreachable (live
// failure, 2026-07).
//
// Deliberately does NOT count a carved STAIR_UP above: that shape has no
// down component, so by itself it provides no connection. It IS joinable —
// but only by designating UpDownStair ON it (carvedUpStairAbove + the
// top-of-range promotion in the loop below handle that case).
static bool stairContinuesAbove(MapExtras::MapCache &cache, int16_t x, int16_t y, int16_t zTop)
{
    if (!Maps::isValidTilePos(x, y, zTop + 1))
        return false;  // top of the map — nothing to join
    df::coord above(x, y, zTop + 1);
    df::tiletype_shape shape = tileShape(cache.tiletypeAt(above));
    if (shape == df::tiletype_shape::STAIR_DOWN ||
        shape == df::tiletype_shape::STAIR_UPDOWN)
        return true;
    df::tile_dig_designation dig = cache.designationAt(above).bits.dig;
    return dig == df::tile_dig_designation::DownStair ||
           dig == df::tile_dig_designation::UpDownStair;
}

// Is the tile directly above (x, y, zTop) a carved up-stair — an old
// shaft's bottom abutting a new range from above? Such a tile lacks a
// down component, so joining requires designating UpDownStair ON it; DF
// accepts exactly that (dfhack-build/plugins/dig-now.cpp:283-288,
// can_dig_up_down_stair allows STAIR_UP) and carves the missing half.
static bool carvedUpStairAbove(MapExtras::MapCache &cache, int16_t x, int16_t y, int16_t zTop)
{
    if (!Maps::isValidTilePos(x, y, zTop + 1))
        return false;
    df::coord above(x, y, zTop + 1);
    return tileShape(cache.tiletypeAt(above)) == df::tiletype_shape::STAIR_UP;
}

// Symmetric check below (x, y, zBottom): an upward stair connection means
// the bottom of a new stair range must be UpDownStair, not UpStair.
// A carved STAIR_DOWN below is correctly NOT counted, and unlike the
// carved-STAIR_UP-above case it cannot be promoted either: dig-now.cpp's
// job predicates reject every stair designation on a carved STAIR_DOWN.
static bool stairContinuesBelow(MapExtras::MapCache &cache, int16_t x, int16_t y, int16_t zBottom)
{
    if (!Maps::isValidTilePos(x, y, zBottom - 1))
        return false;  // bottom of the map — nothing to join
    df::coord below(x, y, zBottom - 1);
    df::tiletype_shape shape = tileShape(cache.tiletypeAt(below));
    if (shape == df::tiletype_shape::STAIR_UP ||
        shape == df::tiletype_shape::STAIR_UPDOWN)
        return true;
    df::tile_dig_designation dig = cache.designationAt(below).bits.dig;
    return dig == df::tile_dig_designation::UpStair ||
           dig == df::tile_dig_designation::UpDownStair;
}

// Informational-only check: does the requested footprint overlap any
// EXISTING building's footprint? DF itself is the sole authority on
// whether a dig actually goes through -- overlapping a building commonly
// produces a silent "Inappropriate dig square" job-cancel later, with no
// coordinate or building name in the announcement text, which live play
// hit twice with no tool-visible way to learn the cause short of
// cross-referencing `buildings` by hand (fortress/memory/goals.md
// 2026-07-18: a cabinet tile, then a 3x3 mechanic workshop, both under
// stair-shaft designations). This never blocks or rejects the
// designation -- it only annotates the ACK so the model can check before
// the cancel spam starts. Capped at 5 named buildings; buildings.all is
// small relative to tile counts so a full per-call scan is cheap, unlike
// scanning per-tile.
static std::string checkBuildingFootprintOverlap(int16_t x1, int16_t y1, int16_t z1,
                                                  int16_t x2, int16_t y2, int16_t z2)
{
    if (!df::global::world) {
        return "";
    }
    std::string note;
    int count = 0;
    for (auto *b : df::global::world->buildings.all) {
        if (!b) continue;
        if (b->z < z1 || b->z > z2) continue;
        if (b->x2 < x1 || b->x1 > x2 || b->y2 < y1 || b->y1 > y2) continue;
        if (count >= 5) {
            note += ", ...";
            break;
        }
        if (count > 0) note += ", ";
        note += ENUM_KEY_STR(building_type, b->getType());
        char buf[64];
        snprintf(buf, sizeof(buf), " at (%d,%d,%d)", b->centerx, b->centery, b->z);
        note += buf;
        count++;
    }
    if (count == 0) {
        return "";
    }
    return "note: footprint overlaps existing building(s) " + note +
           " -- DF may silently cancel affected tiles (\"Inappropriate dig square\")";
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
    int skippedCarved = 0;     // carved stair tiles nothing can be added to
    int promotedCarved = 0;    // carved up-stairs designated UpDownStair to
                               // gain their missing down component (join)
    int convertedStairs = 0;   // carved stairs overwritten by a non-stair
                               // digType, losing their vertical connection
    bool joinedAbove = false;  // top kind promoted to UpDownStair (see loop)
    bool joinedBelow = false;  // bottom kind promoted to UpDownStair

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
    //     This INCLUDES already-carved stair tiles: a non-stair digType
    //     legitimately converts them to plain floor (removing their
    //     vertical connection), same as vanilla DF's own stair-removal
    //     mechanic — tracked separately via convertedStairs and reported
    //     as its own distinct ACK clause, deliberately not skipped.
    //   - Already-carved stair tiles in a stair dig are skipped, EXCEPT a
    //     carved STAIR_UP whose position needs a down component — that one
    //     is designated UpDownStair (details at the check in the loop).
    //
    // Hidden tiles: ALL dig paths designate through hidden terrain, exactly
    // like DF's own designation UI. Every undug underground tile is hidden
    // (fog of war), so skipping hidden tiles made underground rooms
    // undesignatable. Dwarves reveal tiles as they dig; DF's job system
    // handles the rest. Bounds are already screened by isValidTilePos above.
    if (z1 > z2) std::swap(z1, z2);

    bool isStairDig = (digType == 0x02 || digType == 0x05 || digType == 0x06);
    // 0x02=UpDownStair, 0x05=DownStair, 0x06=UpStair from the protocol.
    bool isStairShaft = (z1 != z2) && isStairDig;

    for (int16_t z = z1; z <= z2; z++) {
        for (int16_t x = x1; x <= x2; x++) {
            for (int16_t y = y1; y <= y2; y++) {
                df::coord pos(x, y, z);

                // Already-carved stair tiles: what a dig designation may
                // legally add is defined by dig-now.cpp's job-accurate
                // predicates (dfhack-build/plugins/dig-now.cpp, can_dig_*),
                // not digcircle's ancient validator. Carved STAIR_DOWN and
                // STAIR_UPDOWN accept no stair designation at all —
                // re-designating them only produces "Inappropriate dig
                // square" job-cancel spam, so they are skipped. Carved
                // STAIR_UP accepts UpDownStair (can_dig_up_down_stair):
                // when the tile's position needs a down component, we
                // designate it — that carves the missing half and is how a
                // shaft is extended downward past its old bottom. Skipping
                // it there would strand every new level below (the z9<->z10
                // gap from the 2026-07 live failure, mirrored). Non-stair
                // digTypes (Default, Channel, Ramp) DO legally overwrite an
                // already-carved stair tile — this is vanilla DF's own way
                // of removing or repurposing a staircase, not a bug to
                // guard against. An earlier fix here made these digTypes
                // skip carved stairs unconditionally to stop a silent
                // vertical-connection loss; that traded away real
                // capability for silence. The fix is a truthful ACK
                // (convertedStairs below), not blocking the designation.
                df::tiletype_shape shape = tileShape(cache.tiletypeAt(pos));
                bool tileIsStair = isStairShape(shape);

                if (isStairDig) {
                    if (tileIsStair) {
                        bool needsDown;
                        if (isStairShaft) {
                            // Every shaft tile above the bottom connects to
                            // an in-range tile below it; the bottom needs a
                            // down component only to join a shaft below.
                            needsDown = (z > z1) || stairContinuesBelow(cache, x, y, z1);
                        } else {
                            // Single-Z: the request itself asks for a down
                            // component (0x02 UpDownStair, 0x05 DownStair).
                            needsDown = (digType == 0x02 || digType == 0x05);
                        }
                        if (shape == df::tiletype_shape::STAIR_UP && needsDown) {
                            df::tile_designation des = cache.designationAt(pos);
                            des.bits.dig = df::tile_dig_designation::UpDownStair;
                            cache.setDesignationAt(pos, des);
                            promotedCarved++;
                        } else {
                            skippedCarved++;
                        }
                        continue;
                    }
                } else if (tileIsStair) {
                    convertedStairs++;
                }

                df::tile_designation des = cache.designationAt(pos);

                if (isStairShaft) {
                    if (z == z1) {
                        // Bottom of the range: UpStair — unless the shaft
                        // continues below (carved or designated), in which
                        // case this tile also needs a down component to
                        // join it.
                        if (stairContinuesBelow(cache, x, y, z1)) {
                            des.bits.dig = df::tile_dig_designation::UpDownStair;
                            joinedBelow = true;
                        } else {
                            des.bits.dig = df::tile_dig_designation::UpStair;
                        }
                    } else if (z == z2) {
                        // Top of the range: DownStair — unless the shaft
                        // continues above. A bare DownStair directly under
                        // existing stairs has no up component and makes
                        // every new level unreachable (live failure).
                        if (stairContinuesAbove(cache, x, y, z2)) {
                            des.bits.dig = df::tile_dig_designation::UpDownStair;
                            joinedAbove = true;
                        } else if (carvedUpStairAbove(cache, x, y, z2)) {
                            // A carved up-stair (old shaft bottom) abuts the
                            // range from directly above. It has no down
                            // component, so joining takes both halves:
                            // designate UpDownStair ON it (legal per
                            // dig-now.cpp can_dig_up_down_stair) and give
                            // this tile an up component.
                            df::coord above(x, y, z2 + 1);
                            df::tile_designation aboveDes = cache.designationAt(above);
                            aboveDes.bits.dig = df::tile_dig_designation::UpDownStair;
                            cache.setDesignationAt(above, aboveDes);
                            promotedCarved++;
                            joinedAbove = true;
                            des.bits.dig = df::tile_dig_designation::UpDownStair;
                        } else {
                            des.bits.dig = df::tile_dig_designation::DownStair;
                        }
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

    // Truthful ACK for the stair paths: promotion/skip/conversion counts
    // and shaft-join notes reach the model verbatim (rendered as PARTIAL
    // text by the server). Silent when nothing noteworthy happened — plain
    // digs stay SUCCESS. Note the designated count covers in-range fresh
    // tiles, including converted stairs; promoted carved up-stairs
    // (in-range or the abutting tile above the top) are reported by their
    // own count.
    std::string stairText;
    if (skippedCarved > 0 || promotedCarved > 0 || joinedAbove || joinedBelow ||
        convertedStairs > 0) {
        char buf[192];
        snprintf(buf, sizeof(buf), "%d designated", designated);
        stairText = buf;
        if (promotedCarved > 0) {
            snprintf(buf, sizeof(buf),
                     " (%d joined: carved up-stair promoted to up/down)",
                     promotedCarved);
            stairText += buf;
        }
        if (skippedCarved > 0) {
            snprintf(buf, sizeof(buf), " (%d skipped: already carved)",
                     skippedCarved);
            stairText += buf;
        }
        if (convertedStairs > 0) {
            snprintf(buf, sizeof(buf),
                     " (%d will remove existing stairs: vertical connection lost)",
                     convertedStairs);
            stairText += buf;
        }
        if (joinedAbove)
            stairText += ", top joined to existing shaft above";
        if (joinedBelow)
            stairText += ", bottom joined to existing shaft below";
    }

    // Computed once against the requested rectangle regardless of what the
    // designation loop above did with it -- see the function's doc comment.
    std::string footprintNote = checkBuildingFootprintOverlap(x1, y1, z1, x2, y2, z2);

    if (designated == 0 && promotedCarved == 0) {
        if (skippedCarved > 0) {
            // Every tile in the range is a carved stair with nothing
            // addable — the shaft exists. That is a satisfied request,
            // not a failure; report the truthful counts so the model
            // doesn't re-issue.
            error = stairText;
            if (!footprintNote.empty())
                error += "; " + footprintNote;
            return true;
        }
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

    // Append the stair notes and footprint-overlap note to any existing
    // partial message rather than overwriting it — blocked is structurally
    // 0 in the dig path today, but future per-tile rejection paths must
    // not have their message clobbered.
    if (!stairText.empty())
        error = error.empty() ? stairText : error + "; " + stairText;
    if (!footprintNote.empty())
        error = error.empty() ? footprintNote : error + "; " + footprintNote;

    return true;
}

// Forward declarations of category placers (implemented in buildings.cpp).
// materialClass constrains a generic building-material job_item filter
// (constructions/workshops/furnaces/depot only -- see MATERIAL_CLASS_*'s
// doc comment in protocol.h); placeDoor and placeFurniture reject a
// non-ANY class themselves since their filters are already a specific
// finished item type, not a raw material class. qualityTier (placeFurniture
// only) selects an EXISTING item of at least that quality at placement
// time instead of accepting any matching item.
extern bool placeWorkshop(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);
extern bool placeFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, uint8_t qualityTier, std::string &error);
extern bool placeConstruction(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);
extern bool placeDoor(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);
extern bool placeFurnace(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);
extern bool placeTradeDepot(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error);
extern bool placeWell(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error);
extern bool placeSupport(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error);
extern bool placeArcheryTarget(int16_t x, int16_t y, int16_t z, uint8_t materialClass, std::string &error);
extern bool placeRoomValueFurniture(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);
extern bool placeWaterPowerBuilding(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, uint8_t orientation, std::string &error);
extern bool placeTrap(int16_t x, int16_t y, int16_t z, uint8_t buildType, uint8_t materialClass, std::string &error);

// Forward declarations of the generalized by-name build-type resolution
// (implemented in buildings.cpp, next to the enums it scans). Primitive
// int in/out params only -- no shared struct/header exists between .cpp
// files in this plugin, and neither function needs designations.cpp to
// include any building_type/workshop_type/furnace_type/trap_type header.
// See buildings.cpp for full doc comments (KNOWN LIMITATION included).
extern bool resolveBuildTypeByName(const std::string &name, int &outBuildingType, int &outSubtype, std::string &error);
extern int resolveCuratedBuildTypeByte(int buildingType, int subtype, const std::string &name, std::string &error);

// applyBuildDesignation dispatches BUILD commands to category-specific
// placers based on the BuildType byte's range:
//   0x00       → BUILD_TYPE_BY_NAME: resolve BuildTypeName, then re-dispatch
//                below using the curated byte it resolves to (if any)
//   0x01-0x0F → constructions (wall, floor, stairs, ramp)
//   0x10-0x2F → workshops (incl. MetalsmithsForge)
//   0x30-0x4F → furniture
//   0x50-0x6F → doors / hatches
//   0x70-0x7F → furnaces (Smelter, WoodFurnace)
//   0x80-0x8F → trade depot
//   0x90-0x9F → misc/infrastructure (Well, Support, ArcheryTarget, the
//               room-value furniture family Statue/Slab/WindowGlass/
//               WindowGem/Bookcase/DisplayFurniture/OfferingPlace/
//               Instrument, and TractionBench/NestBox/Hive)
//   0xA0-0xAF → water/power-transmission infrastructure (ScrewPump,
//               GearAssembly, AxleHorizontal, AxleVertical, WaterWheel,
//               Windmill, Rollers)
//   0xB0-0xBF → more df::trap_type subtypes beyond Lever (PressurePlate,
//               StoneFallTrap, WeaponTrap, TrackStop)
bool applyBuildDesignation(const std::vector<uint8_t> &payload, std::string &error)
{
    // Parse: [4: cmdID] [1: cmdType] [2: X] [2: Y] [2: Z] [1: BuildType]
    //        [1: MaterialClass (optional)] [1: QualityTier (optional)]
    //        [1: Orientation (optional)] [2: NameLen][N: Name] (present
    //        ONLY when BuildType == BUILD_TYPE_BY_NAME -- mirrors
    //        QUEUE_JOB's identical trailing length-prefixed name, see
    //        df_ai_protocol.cpp case COMMAND_TYPE_QUEUE_JOB)
    // The three material/quality/orientation bytes are each independently
    // optional for backward compatibility: an older peer's payload may end
    // right after BuildType (no material support yet), right after
    // MaterialClass (material support shipped before quality did), or
    // right after QualityTier (quality support shipped before orientation
    // did) -- see internal/protocol/codec.go's matching encode-side
    // comment. Absent bytes default to "no constraint".
    if (payload.size() < 12) {
        error = "Invalid build payload size";
        return false;
    }

    int16_t x = read_int16_be(payload, 5);
    int16_t y = read_int16_be(payload, 7);
    int16_t z = read_int16_be(payload, 9);
    uint8_t buildType = payload[11];
    uint8_t materialClass = (payload.size() >= 13) ? payload[12] : MATERIAL_CLASS_ANY;
    uint8_t qualityTier   = (payload.size() >= 14) ? payload[13] : QUALITY_TIER_ANY;
    uint8_t orientation   = (payload.size() >= 15) ? payload[14] : BUILD_ORIENT_ANY;

    std::string buildTypeName;
    if (buildType == BUILD_TYPE_BY_NAME) {
        if (payload.size() < 17) {  // 15 existing + NameLen(2)
            error = "Invalid build-by-name payload (missing name length)";
            return false;
        }
        uint16_t nameLen = ((uint16_t)payload[15] << 8) | payload[16];
        if (payload.size() < 17 + (size_t)nameLen) {
            error = "Invalid build-by-name payload size";
            return false;
        }
        buildTypeName.assign(payload.begin() + 17, payload.begin() + 17 + nameLen);
    }

    if (!Maps::isValidTilePos(x, y, z)) {
        error = "Coordinates out of map bounds";
        return false;
    }

    if (buildType == BUILD_TYPE_BY_NAME) {
        int resolvedBuildingType, resolvedSubtype;
        if (!resolveBuildTypeByName(buildTypeName, resolvedBuildingType, resolvedSubtype, error)) {
            return false;
        }
        // Reuse an EXISTING, already-verified placement path byte-for-byte
        // if the resolved type matches one -- see resolveCuratedBuildTypeByte's
        // KNOWN LIMITATION comment (buildings.cpp) for why this is not yet
        // possible for every resolvable name.
        int curatedByte = resolveCuratedBuildTypeByte(resolvedBuildingType, resolvedSubtype, buildTypeName, error);
        if (curatedByte < 0) {
            return false; // error already set by resolveCuratedBuildTypeByte
        }
        buildType = (uint8_t)curatedByte;
        // Falls through to the exact same dispatch below, now using the
        // derived curated byte -- byte-for-byte identical to a caller who
        // had passed that byte directly (including the quality-tier check
        // immediately below, now evaluated against the RESOLVED type).
    }

    if (qualityTier != QUALITY_TIER_ANY) {
        if (!isBuildTypeFurniture(buildType)) {
            error = "quality tier only applies to furniture placement (bed/table/chair/cabinet/coffer/coffin)";
            return false;
        }
        if (qualityTier > QUALITY_TIER_ARTIFACT) {
            error = "invalid quality tier byte";
            return false;
        }
    }

    if (isBuildTypeWorkshop(buildType)) {
        return placeWorkshop(x, y, z, buildType, materialClass, error);
    }
    if (isBuildTypeFurniture(buildType)) {
        return placeFurniture(x, y, z, buildType, materialClass, qualityTier, error);
    }
    if (isBuildTypeConstruction(buildType)) {
        return placeConstruction(x, y, z, buildType, materialClass, error);
    }
    if (isBuildTypeDoor(buildType)) {
        return placeDoor(x, y, z, buildType, materialClass, error);
    }
    if (isBuildTypeFurnace(buildType)) {
        return placeFurnace(x, y, z, buildType, materialClass, error);
    }
    if (isBuildTypeDepot(buildType)) {
        return placeTradeDepot(x, y, z, materialClass, error);
    }
    if (buildType == BUILD_TYPE_WELL) {
        return placeWell(x, y, z, materialClass, error);
    }
    if (buildType == BUILD_TYPE_SUPPORT) {
        return placeSupport(x, y, z, materialClass, error);
    }
    if (buildType == BUILD_TYPE_ARCHERY_TARGET) {
        return placeArcheryTarget(x, y, z, materialClass, error);
    }
    // BUILD_TYPE_ARCHERY_TARGET (0x9A) is checked above BEFORE this range so
    // it takes priority even though its byte value numerically falls inside
    // [BUILD_TYPE_STATUE, BUILD_TYPE_HIVE] -- it needs placeArcheryTarget's
    // materialClass-accepting shape, not placeRoomValueFurniture's.
    if (buildType >= BUILD_TYPE_STATUE && buildType <= BUILD_TYPE_HIVE) {
        return placeRoomValueFurniture(x, y, z, buildType, materialClass, error);
    }
    if (isBuildTypeWaterPower(buildType)) {
        return placeWaterPowerBuilding(x, y, z, buildType, materialClass, orientation, error);
    }
    if (isBuildTypeTrap(buildType)) {
        return placeTrap(x, y, z, buildType, materialClass, error);
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
