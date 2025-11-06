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
