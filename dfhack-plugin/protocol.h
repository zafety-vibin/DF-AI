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
constexpr uint8_t COMMAND_TYPE_DIG = 0x01;
constexpr uint8_t COMMAND_TYPE_BUILD = 0x02;
constexpr uint8_t COMMAND_TYPE_CANCEL = 0x03;

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
