# Protocol Specification

**Version**: 1.0
**Date**: 2025-11-04
**Status**: Design

## Message Catalog

This contract defines all valid protocol messages with binary layouts.

---

## Message Format (Universal)

All messages follow this structure:

```
+--------+--------+------+----------+
| Length | Ver    | Type | Payload  |
| 4 bytes| 1 byte | 1 byte| N bytes |
+--------+--------+------+----------+
```

**Header** (6 bytes total):
- **Offset 0-3** (uint32 BE): Total message length including this header
- **Offset 4** (uint8): Protocol version (must be 0x01)
- **Offset 5** (uint8): Message type ID

**Payload** (variable length):
- Offset 6+: Type-specific data

---

## Message Type: HANDSHAKE (0x01)

**Direction**: Bidirectional (both send on connect)

**Purpose**: Protocol version negotiation and connection identification

**Payload Layout**:
```
+------+------+------+------+------+-------------+
| Ver  | ID byte 0-3         | Capabilities... |
| 1    | 4                   | variable        |
+------+---------------------+-----------------+
```

**Fields**:
- **Offset 0** (uint8): Protocol version this side supports (0x01)
- **Offset 1-4** (uint32 BE): Random connection ID for this session
- **Offset 5+** (string): Capabilities (length-prefixed, currently empty)

**Example Bytes**:
```
00 00 00 0B  // Length: 11 (6 header + 5 payload)
01           // Version: 1
01           // Type: HANDSHAKE
01           // Payload version: 1
12 34 56 78  // Connection ID: 0x12345678
00           // Capabilities: empty (length 0)
```

**Validation**:
- Must be first message after TCP connect
- Protocol version must match exactly
- Connection ID must be non-zero

**Error Conditions**:
- Version mismatch → Send ERROR 0x0003, disconnect
- Missing handshake → Disconnect after 5 second timeout

---

## Message Type: FULL_STATE (0x02)

**Direction**: Plugin → Go Server only

**Purpose**: Complete map state dump

**Payload Layout**:
```
+-------+-------+-------+------------------+
| Width | Height| Depth | Tile Array       |
| 2     | 2     | 2     | (W*H*D) * 9      |
+-------+-------+-------+------------------+
```

**Fields**:
- **Offset 0-1** (uint16 BE): Map width (X dimension)
- **Offset 2-3** (uint16 BE): Map height (Y dimension)
- **Offset 4-5** (uint16 BE): Map depth (Z levels)
- **Offset 6+** (TileState[]): Array of all tiles in row-major order

**Tile State Structure** (9 bytes each):
```
+----+----+----+----------+-------+
| X  | Y  | Z  | TileType | Flags |
| 2  | 2  | 2  | 2        | 1     |
+----+----+----+----------+-------+
```

- **X, Y, Z** (int16 BE): Coordinates
- **TileType** (uint16 BE): DF tiletype enum value
- **Flags** (uint8): Bit flags (see data-model.md)

**Example** (tiny 2×2×1 map):
```
00 00 00 2A  // Length: 42 (6 header + 6 dims + 36 tiles)
01           // Version: 1
02           // Type: FULL_STATE
00 02        // Width: 2
00 02        // Height: 2
00 01        // Depth: 1
// Tile at (0,0,0):
00 00 00 00 00 00 01 90 02  // x=0 y=0 z=0 type=400 flags=0x02
// Tile at (1,0,0):
00 01 00 00 00 00 01 90 02  // x=1 y=0 z=0 type=400 flags=0x02
// Tile at (0,1,0):
00 00 00 01 00 00 01 91 02  // x=0 y=1 z=0 type=401 flags=0x02
// Tile at (1,1,0):
00 01 00 01 00 00 01 91 02  // x=1 y=1 z=0 type=401 flags=0x02
```

**Validation**:
- Dimensions must be > 0 and < 65536
- Tile count must equal width × height × depth
- Message length must match 6 + 6 + (count × 9)

**Performance**:
- 100×100×10 map = 100K tiles × 9 bytes = 900KB + overhead ≈ 880KB total
- Must transmit in <2 seconds per requirements

---

## Message Type: TILE_UPDATE (0x03)

**Direction**: Plugin → Go Server only

**Purpose**: Incremental tile change notifications

**Payload Layout**:
```
+-------+------------------+
| Count | Tile Array       |
| 4     | Count * 9        |
+-------+------------------+
```

**Fields**:
- **Offset 0-3** (uint32 BE): Number of changed tiles
- **Offset 4+** (TileState[]): Array of changed tiles only

**Example** (3 tiles changed):
```
00 00 00 25  // Length: 37 (6 header + 4 count + 27 tiles)
01           // Version: 1
03           // Type: TILE_UPDATE
00 00 00 03  // Count: 3
// 3 × 9-byte tile states follow
[27 bytes of tile data]
```

**Validation**:
- Count must be > 0 (use RESYNC instead of empty updates)
- Message length must match 6 + 4 + (count × 9)
- All tiles must have valid coordinates within map bounds

**Batching Rules**:
- Plugin should batch updates from same game tick
- Maximum batch size: 1000 tiles per message (avoid 10KB+ messages)
- If >1000 tiles changed, send multiple TILE_UPDATE messages

---

## Message Type: RESYNC_REQUEST (0x04)

**Direction**: Go Server → Plugin only

**Purpose**: Request full state resynchronization

**Payload Layout**:
```
+--------+
| Reason |
| 1      |
+--------+
```

**Fields**:
- **Offset 0** (uint8): Reason code

**Reason Codes**:
- 0x00: Manual/user-requested resync
- 0x01: Detected state inconsistency
- 0x02: After reconnection
- 0x03: Periodic resync (scheduled)
- 0x04-0xFF: Reserved

**Example**:
```
00 00 00 07  // Length: 7 (6 header + 1 payload)
01           // Version: 1
04           // Type: RESYNC_REQUEST
02           // Reason: After reconnection
```

**Expected Response**:
Plugin must send FULL_STATE message within 5 seconds.

**Validation**:
- Reason code must be valid (0x00-0x03)

---

## Message Type: HEARTBEAT (0x05)

**Direction**: Bidirectional (Go initiates, Plugin echoes)

**Purpose**: Keep-alive and connection health monitoring

**Payload Layout**:
```
+-----------+----------+
| Timestamp | Sequence |
| 8         | 1        |
+-----------+----------+
```

**Fields**:
- **Offset 0-7** (uint64 BE): Unix timestamp in milliseconds
- **Offset 8** (uint8): Sequence number (wraps at 255)

**Example**:
```
00 00 00 0F  // Length: 15 (6 header + 9 payload)
01           // Version: 1
05           // Type: HEARTBEAT
00 00 01 8C 9A 3B 28 00  // Timestamp: 1699027200000
42           // Sequence: 66
```

**Behavior**:
1. **Go Server**: Send HEARTBEAT every 10 seconds with current timestamp and incremented sequence
2. **Plugin**: Echo back same timestamp, increment sequence by 1
3. **Go Server**: If no echo within 5 seconds, mark connection as dead
4. **Plugin**: If no HEARTBEAT received for 15 seconds, assume disconnect

**Validation**:
- Timestamp should be recent (within 60 seconds of current time)
- Sequence increments (but wrapping is allowed)

---

## Message Type: ERROR (0x06)

**Direction**: Bidirectional

**Purpose**: Non-fatal error notification (connection stays open)

**Payload Layout**:
```
+------+------+------------------+
| Code        | Message          |
| 2           | variable (string)|
+------+------+------------------+
```

**Fields**:
- **Offset 0-1** (uint16 BE): Error code
- **Offset 2-3** (uint16 BE): Message length N
- **Offset 4+** (string): UTF-8 error message (N bytes)

**Error Code Ranges**:
- 0x0001: Unknown message type
- 0x0002: Invalid payload format
- 0x0003: Protocol version mismatch
- 0x0004: Internal processing error
- 0x1000-0x1FFF: Plugin-specific errors
- 0x2000-0x2FFF: Go server-specific errors

**Example**:
```
00 00 00 1E  // Length: 30 (6 header + 24 payload)
01           // Version: 1
06           // Type: ERROR
00 01        // Error code: 0x0001 (unknown message type)
00 12        // Message length: 18
55 6E 6B ... // "Unknown message type 0xFF"
```

**Behavior**:
- Log error on receiving side
- Connection remains open (unless error is fatal)
- Increment error counter in session metrics

---

## Message Type: DISCONNECT (0x07)

**Direction**: Bidirectional

**Purpose**: Graceful connection shutdown

**Payload Layout**:
```
+--------+
| Reason |
| 1      |
+--------+
```

**Fields**:
- **Offset 0** (uint8): Disconnect reason code

**Reason Codes**:
- 0x00: Normal shutdown
- 0x01: Plugin being unloaded
- 0x02: Server shutting down
- 0x03: Restart/reload requested
- 0x04-0xFF: Reserved

**Example**:
```
00 00 00 07  // Length: 7 (6 header + 1 payload)
01           // Version: 1
07           // Type: DISCONNECT
01           // Reason: Plugin unload
```

**Behavior**:
1. Sender: Send DISCONNECT message
2. Sender: Flush TCP buffers
3. Sender: Close TCP socket
4. Receiver: Log disconnect reason
5. Receiver: Close socket
6. Both: Clean up session resources

**Validation**:
- Should be last message before TCP FIN
- No response expected

---

## Connection State Machine

```
[Disconnected]
      |
      | TCP Connect
      v
[Handshake Pending]
      |
      | HANDSHAKE exchanged, versions match
      v
[Connected]
      |
      |<--- HEARTBEAT every 10s
      |---> HEARTBEAT echo
      |
      |<--- RESYNC_REQUEST
      |---> FULL_STATE
      |
      |---> TILE_UPDATE (as changes occur)
      |
      | DISCONNECT or timeout
      v
[Disconnected]
```

---

## Error Handling

### Protocol Violations

| Violation | Action |
|-----------|--------|
| Invalid message type | Send ERROR 0x0001, ignore message, continue |
| Invalid payload | Send ERROR 0x0002, ignore message, continue |
| Version mismatch | Send ERROR 0x0003, send DISCONNECT, close |
| Message too large (>10MB) | Disconnect immediately (potential attack) |
| Heartbeat timeout | Disconnect, attempt reconnect |

### Reconnection Logic

**Plugin Side**:
1. Detect disconnect (timeout or TCP error)
2. Wait 1 second
3. Retry connection
4. Exponential backoff: 1s, 2s, 4s, 8s, 16s, max 30s
5. Retry indefinitely while DF is running

**Go Server Side**:
1. Accept new connection
2. Wait for HANDSHAKE
3. If HANDSHAKE not received within 5s, disconnect
4. On successful HANDSHAKE, send RESYNC_REQUEST

---

## Performance Requirements

| Metric | Target | How to Measure |
|--------|--------|----------------|
| Full state sync | <2 seconds for 1M tiles | Time from RESYNC_REQUEST to FULL_STATE processed |
| Incremental update latency | <1 second | Time from tile change in DF to TILE_UPDATE processed |
| Heartbeat response | <100ms | Time from HEARTBEAT sent to echo received |
| Message throughput | >500 KB/s | Bytes transmitted / time for FULL_STATE |
| Connection overhead | <5% FPS impact | DF FPS with vs without plugin enabled |

---

## Logging Requirements

All protocol events must generate structured log entries (JSON format):

```json
{
  "timestamp": "ISO8601",
  "level": "DEBUG|INFO|WARN|ERROR",
  "component": "protocol",
  "event": "event_name",
  "message_type": "HANDSHAKE|FULL_STATE|...",
  "message_size": 12345,
  "session_id": "connection_id",
  "duration_ms": 100,
  "additional": "context"
}
```

**Required Events**:
- `connection_opened`
- `connection_closed`
- `handshake_sent`
- `handshake_received`
- `message_sent` (with type and size)
- `message_received` (with type and size)
- `error_occurred` (with error code)
- `heartbeat_timeout`
- `resync_triggered`

---

## Testing Contracts

### Unit Test Requirements

**Go Side**:
- Parse all message types from byte arrays
- Serialize all message types to byte arrays
- Validate message length calculations
- Handle truncated messages gracefully
- Detect protocol violations

**C++ Plugin Side**:
- Encode all message types correctly
- Decode all message types from Go
- Handle endianness correctly (test on different platforms)
- Batch tile updates efficiently

### Integration Test Scenarios

1. **Happy Path**: Connect, handshake, full state, updates, disconnect
2. **Reconnection**: Disconnect mid-operation, reconnect, resync
3. **Large Map**: 200×200×100 map (4M tiles) within performance target
4. **Rapid Updates**: 1000 tile changes per second for 10 seconds
5. **Protocol Errors**: Send invalid messages, verify error handling
6. **Heartbeat Timeout**: Simulate network delay, verify timeout detection

---

## Version Compatibility

**Protocol Version 1.0** (current):
- All 7 message types defined above
- Big-endian encoding
- No compression
- Maximum message size: 10 MB

**Future Versions**:
- Version 2: May add COMPRESSED_FULL_STATE message
- Version 3: May add COMMAND messages (Go → Plugin commands)
- Backward compatibility: New clients must support old server versions via negotiation