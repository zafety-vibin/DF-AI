#pragma once

#include <cstdint>

// Protocol version
constexpr uint8_t PROTOCOL_VERSION = 1;

// Message type constants
constexpr uint8_t MSG_TYPE_HANDSHAKE = 0x01;
constexpr uint8_t MSG_TYPE_FULL_STATE = 0x02;
constexpr uint8_t MSG_TYPE_TILE_UPDATE = 0x03;
constexpr uint8_t MSG_TYPE_RESYNC_REQUEST = 0x04;
constexpr uint8_t MSG_TYPE_HEARTBEAT = 0x05;
constexpr uint8_t MSG_TYPE_ERROR = 0x06;
constexpr uint8_t MSG_TYPE_DISCONNECT = 0x07;
constexpr uint8_t MSG_TYPE_ENTITY_UPDATE = 0x08;
constexpr uint8_t MSG_TYPE_COMMAND = 0x09;
constexpr uint8_t MSG_TYPE_COMMAND_ACK = 0x0A;
constexpr uint8_t MSG_TYPE_QUERY = 0x0B;
constexpr uint8_t MSG_TYPE_QUERY_RESPONSE = 0x0C;
constexpr uint8_t MSG_TYPE_ANNOUNCEMENT_UPDATE = 0x0D;

// Query response status codes
constexpr uint8_t QUERY_STATUS_SUCCESS = 0x00;
constexpr uint8_t QUERY_STATUS_ERROR   = 0x01;
constexpr uint8_t QUERY_STATUS_UNKNOWN = 0x02;

// Tile flags
constexpr uint8_t FLAG_HIDDEN = 0x01;
constexpr uint8_t FLAG_DISCOVERED = 0x02;
constexpr uint8_t FLAG_DESIGNATED = 0x04;
constexpr uint8_t FLAG_CONSTRUCT = 0x08;

// Error codes
constexpr uint16_t ERR_UNKNOWN_TYPE = 0x0001;
constexpr uint16_t ERR_INVALID_PAYLOAD = 0x0002;
constexpr uint16_t ERR_VERSION_MISMATCH = 0x0003;
constexpr uint16_t ERR_INTERNAL = 0x0004;

// Disconnect reasons
constexpr uint8_t REASON_NORMAL_SHUTDOWN = 0x00;
constexpr uint8_t REASON_PLUGIN_UNLOAD = 0x01;
constexpr uint8_t REASON_SERVER_SHUTDOWN = 0x02;
constexpr uint8_t REASON_RESTART = 0x03;

// Resync reasons
constexpr uint8_t RESYNC_MANUAL = 0x00;
constexpr uint8_t RESYNC_INCONSISTENCY = 0x01;
constexpr uint8_t RESYNC_RECONNECT = 0x02;
constexpr uint8_t RESYNC_PERIODIC = 0x03;

// Heartbeat timing (milliseconds)
constexpr uint32_t HEARTBEAT_INTERVAL_MS = 10000;
constexpr uint32_t HEARTBEAT_TIMEOUT_MS = 15000;

// Command types
constexpr uint8_t COMMAND_TYPE_DIG        = 0x01;
constexpr uint8_t COMMAND_TYPE_BUILD      = 0x02;
constexpr uint8_t COMMAND_TYPE_CANCEL     = 0x03;
constexpr uint8_t COMMAND_TYPE_CHOP       = 0x04;
constexpr uint8_t COMMAND_TYPE_GATHER     = 0x05;
constexpr uint8_t COMMAND_TYPE_ZONE       = 0x06;
constexpr uint8_t COMMAND_TYPE_BLUEPRINT  = 0x07;
constexpr uint8_t COMMAND_TYPE_UNSUSPEND  = 0x08;
constexpr uint8_t COMMAND_TYPE_WORK_ORDER = 0x09;
constexpr uint8_t COMMAND_TYPE_STOCKPILE  = 0x0A;
constexpr uint8_t COMMAND_TYPE_SMOOTH     = 0x0B;
constexpr uint8_t COMMAND_TYPE_PAUSE      = 0x0C;

// Pause modes (payload byte after cmdType): matches internal/protocol/message.go.
constexpr uint8_t PAUSE_MODE_UNPAUSE = 0x00;
constexpr uint8_t PAUSE_MODE_PAUSE   = 0x01;
constexpr uint8_t PAUSE_MODE_STEP    = 0x02;

// Smooth subtype: matches df::tile_designation::smooth bitfield (1=smooth, 2=engrave).
constexpr uint8_t SMOOTH_TYPE_SMOOTH  = 0x01;
constexpr uint8_t SMOOTH_TYPE_ENGRAVE = 0x02;

// Zone types — kept in sync with internal/protocol/message.go.
// 0x06–0x08 are LEGACY values (Office, Workshop, Stockpile) from the
// Feature 007 layout system; do not emit them from new code. Real DF
// civzone categories live at 0x10+.
constexpr uint8_t ZONE_TYPE_BEDROOM        = 0x01;
constexpr uint8_t ZONE_TYPE_DINING         = 0x02;
constexpr uint8_t ZONE_TYPE_MEETING        = 0x03;
constexpr uint8_t ZONE_TYPE_BARRACKS       = 0x04;
constexpr uint8_t ZONE_TYPE_DORMITORY      = 0x05;

constexpr uint8_t ZONE_TYPE_FARM           = 0x10;
constexpr uint8_t ZONE_TYPE_PEN            = 0x11;
constexpr uint8_t ZONE_TYPE_GARBAGE_DUMP   = 0x12;
constexpr uint8_t ZONE_TYPE_PIT_POND       = 0x13;
constexpr uint8_t ZONE_TYPE_WATER_SOURCE   = 0x14;
constexpr uint8_t ZONE_TYPE_FISHING        = 0x15;
constexpr uint8_t ZONE_TYPE_HOSPITAL       = 0x16;
constexpr uint8_t ZONE_TYPE_ANIMAL_TRAIN   = 0x17;
constexpr uint8_t ZONE_TYPE_TOMB           = 0x18;

// Work order types — what the manager queue should produce
constexpr uint8_t ORDER_TYPE_MAKE_BED      = 0x01;
constexpr uint8_t ORDER_TYPE_MAKE_TABLE    = 0x02;
constexpr uint8_t ORDER_TYPE_MAKE_CHAIR    = 0x03;
constexpr uint8_t ORDER_TYPE_MAKE_DOOR     = 0x04;
constexpr uint8_t ORDER_TYPE_MAKE_BARREL   = 0x05;
constexpr uint8_t ORDER_TYPE_MAKE_BUCKET   = 0x06;
constexpr uint8_t ORDER_TYPE_MAKE_CABINET  = 0x07;
constexpr uint8_t ORDER_TYPE_MAKE_COFFER   = 0x08;
constexpr uint8_t ORDER_TYPE_BREW_DRINK    = 0x09;
constexpr uint8_t ORDER_TYPE_PREPARE_MEAL  = 0x0A;
constexpr uint8_t ORDER_TYPE_MAKE_BLOCKS   = 0x0B;
constexpr uint8_t ORDER_TYPE_MAKE_CRAFTS   = 0x0C;

// BuildType constants — kept in sync with internal/protocol/message.go.
// Ranges: 0x01-0x0F constructions, 0x10-0x2F workshops, 0x30-0x4F furniture,
// 0x50-0x6F doors/hatches.
constexpr uint8_t BUILD_TYPE_WALL          = 0x01;
constexpr uint8_t BUILD_TYPE_FLOOR         = 0x02;
constexpr uint8_t BUILD_TYPE_UP_STAIR      = 0x03;
constexpr uint8_t BUILD_TYPE_DOWN_STAIR    = 0x04;
constexpr uint8_t BUILD_TYPE_UPDOWN_STAIR  = 0x05;
constexpr uint8_t BUILD_TYPE_RAMP          = 0x06;

constexpr uint8_t BUILD_TYPE_WS_CARPENTER  = 0x10;
constexpr uint8_t BUILD_TYPE_WS_MASON      = 0x11;
constexpr uint8_t BUILD_TYPE_WS_STILL      = 0x12;
constexpr uint8_t BUILD_TYPE_WS_FARMER     = 0x13;
constexpr uint8_t BUILD_TYPE_WS_CRAFTSDWARF= 0x14;
constexpr uint8_t BUILD_TYPE_WS_MECHANIC   = 0x15;
constexpr uint8_t BUILD_TYPE_WS_BUTCHER    = 0x16;
constexpr uint8_t BUILD_TYPE_WS_KITCHEN    = 0x17;
constexpr uint8_t BUILD_TYPE_WS_FISHERY    = 0x18;

constexpr uint8_t BUILD_TYPE_BED      = 0x30;
constexpr uint8_t BUILD_TYPE_TABLE    = 0x31;
constexpr uint8_t BUILD_TYPE_CHAIR    = 0x32;
constexpr uint8_t BUILD_TYPE_CABINET  = 0x33;
constexpr uint8_t BUILD_TYPE_COFFER   = 0x34;

constexpr uint8_t BUILD_TYPE_DOOR     = 0x50;
constexpr uint8_t BUILD_TYPE_HATCH    = 0x51;

inline bool isBuildTypeWorkshop(uint8_t t)     { return t >= 0x10 && t < 0x30; }
inline bool isBuildTypeFurniture(uint8_t t)    { return t >= 0x30 && t < 0x50; }
inline bool isBuildTypeConstruction(uint8_t t) { return t >= 0x01 && t < 0x10; }
inline bool isBuildTypeDoor(uint8_t t)         { return t >= 0x50 && t < 0x70; }

// ACK status codes
constexpr uint8_t ACK_STATUS_SUCCESS = 0x00;
constexpr uint8_t ACK_STATUS_PARTIAL = 0x01;
constexpr uint8_t ACK_STATUS_FAILURE = 0x02;

// Shared helper functions (implemented in tile_extractor.cpp)
#include <vector>
void write_uint16_be(std::vector<uint8_t> &buf, uint16_t value);
void write_int16_be(std::vector<uint8_t> &buf, int16_t value);
void write_uint32_be(std::vector<uint8_t> &buf, uint32_t value);
void write_uint64_be(std::vector<uint8_t> &buf, uint64_t value);

// Big-endian payload reader (implemented in designations.cpp).
uint32_t read_uint32_be(const std::vector<uint8_t> &data, size_t offset);
