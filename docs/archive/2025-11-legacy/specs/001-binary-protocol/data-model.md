# Data Model: DFHack Binary Protocol

**Feature**: 001-binary-protocol
**Date**: 2025-11-04
**Status**: Design

## Overview

This document defines the binary message formats exchanged between the DFHack C++ plugin and Go orchestrator server. The protocol uses simple length-prefixed messages with binary-encoded payloads for efficiency.

---

## Core Entities

### Connection

Represents the TCP connection state between DFHack plugin and Go server.

**States**:
- `Disconnected` - No active connection
- `Connecting` - TCP handshake in progress
- `Connected` - Active connection, ready for messages
- `Error` - Connection failed, retry pending

**Properties**:
- Host: string (e.g., "localhost")
- Port: uint16 (e.g., 5001)
- Protocol Version: uint8
- Last Activity: timestamp
- Retry Count: uint32
- State: enum

---

### Message

Base protocol message structure. All messages follow this format:

```
[4 bytes: Length] [1 byte: Version] [1 byte: Type] [N bytes: Payload]
```

**Fields**:
- **Length** (uint32, big-endian): Total message size including header (minimum 6)
- **Version** (uint8): Protocol version (currently 1)
- **Type** (uint8): Message type identifier
- **Payload** (variable): Type-specific data

**Message Types**:
| Type ID | Name | Direction | Description |
|---------|------|-----------|-------------|
| 0x01 | HANDSHAKE | Both | Initial connection negotiation |
| 0x02 | FULL_STATE | Plugin → Go | Complete map dump |
| 0x03 | TILE_UPDATE | Plugin → Go | Incremental tile changes |
| 0x04 | RESYNC_REQUEST | Go → Plugin | Request full state resync |
| 0x05 | HEARTBEAT | Both | Keep-alive ping/pong |
| 0x06 | ERROR | Both | Error notification |
| 0x07 | DISCONNECT | Both | Graceful shutdown |

---

### Tile State

Represents the state of a single tile at coordinates (x, y, z).

**Fields**:
- **X** (int16): X coordinate (0-65535 range, signed for future use)
- **Y** (int16): Y coordinate
- **Z** (int16): Z coordinate (levels, can be negative)
- **Tile Type** (uint16): DF tiletype value
- **Flags** (uint8): Packed flags (hidden, discovered, designated, etc.)

**Binary Layout** (9 bytes total):
```
[2: X] [2: Y] [2: Z] [2: TileType] [1: Flags]
```

**Flags Bit Mapping** (uint8):
- Bit 0: Hidden (fog of war)
- Bit 1: Discovered (seen before)
- Bit 2: Designated for mining
- Bit 3: Designated for construction
- Bits 4-7: Reserved

---

### Session

Tracking data for a complete connection session.

**Fields**:
- Session ID: UUID or uint64
- Start Time: timestamp
- End Time: timestamp (if disconnected)
- Messages Sent: uint64
- Messages Received: uint64
- Bytes Sent: uint64
- Bytes Received: uint64
- Errors: uint32

**Purpose**: Logging and metrics collection only (not transmitted in protocol)

---

## Message Payloads

### HANDSHAKE (0x01)

Exchanged when connection first established. Both sides send this.

**Payload Structure**:
```
[1: Protocol Version] [4: Client/Server ID] [N: Capabilities]
```

**Fields**:
- Protocol Version (uint8): Must match (currently 1)
- ID (uint32): Random identifier for this connection
- Capabilities (variable string): Future extensibility (empty for v1)

**Example**:
```
Length: 11 (6 header + 5 payload)
Version: 1
Type: 0x01
Payload: [0x01] [0x12 0x34 0x56 0x78] [0x00]
         version   random ID          empty caps
```

---

### FULL_STATE (0x02)

Complete map dump from plugin to Go. Sent on initial connection and when RESYNC_REQUEST received.

**Payload Structure**:
```
[2: Map Width] [2: Map Height] [2: Map Depth] [N: Tile Array]
```

**Fields**:
- Width (uint16): Map X dimension
- Height (uint16): Map Y dimension
- Depth (uint16): Map Z dimension (number of levels)
- Tiles (array of Tile State): All tiles in row-major order (X fastest, then Y, then Z)

**Tile Ordering**:
```
for z in 0..depth:
  for y in 0..height:
    for x in 0..width:
      emit TileState(x, y, z)
```

**Size Calculation**:
- Header: 6 bytes
- Dimensions: 6 bytes
- Tiles: (W × H × D) × 9 bytes per tile
- Example (100×100×10): 6 + 6 + (100,000 × 9) = 900,012 bytes = ~880 KB

**Optimization Note**: For maps larger than 1MB, consider chunking across multiple messages (future enhancement).

---

### TILE_UPDATE (0x03)

Incremental updates when tiles change.

**Payload Structure**:
```
[4: Tile Count] [N: Tile Array]
```

**Fields**:
- Count (uint32): Number of changed tiles in this message
- Tiles (array of Tile State): Changed tiles only

**Example** (3 tiles changed):
```
Length: 37 (6 header + 4 count + 27 tiles)
Version: 1
Type: 0x03
Payload: [0x00 0x00 0x00 0x03] [9 bytes tile 1] [9 bytes tile 2] [9 bytes tile 3]
```

**Batching**: Plugin should batch updates if many tiles change in same tick (e.g., water flow affecting 100 tiles → single message with count=100).

---

### RESYNC_REQUEST (0x04)

Go server requests full state resync.

**Payload Structure**:
```
[1: Reason Code]
```

**Reason Codes**:
- 0x00: Manual request
- 0x01: Detected inconsistency
- 0x02: After reconnection
- 0x03: Periodic resync

**Response**: Plugin sends FULL_STATE message.

---

### HEARTBEAT (0x05)

Keep-alive to detect dead connections.

**Payload Structure**:
```
[8: Timestamp] [1: Sequence]
```

**Fields**:
- Timestamp (uint64): Unix timestamp milliseconds
- Sequence (uint8): Increments each heartbeat (wraps at 255)

**Behavior**:
- Go server sends HEARTBEAT every 10 seconds
- Plugin echoes back same timestamp + incremented sequence
- If no response within 5 seconds → connection considered dead

---

### ERROR (0x06)

Error notification (non-fatal).

**Payload Structure**:
```
[2: Error Code] [N: Error Message]
```

**Error Codes**:
- 0x0001: Unknown message type
- 0x0002: Invalid payload
- 0x0003: Version mismatch
- 0x0004: Internal error
- 0x1000-0x1FFF: Plugin-specific errors
- 0x2000-0x2FFF: Go server-specific errors

**Message**: UTF-8 encoded string (for logging)

---

### DISCONNECT (0x07)

Graceful shutdown notification.

**Payload Structure**:
```
[1: Reason Code]
```

**Reason Codes**:
- 0x00: Normal shutdown
- 0x01: Plugin unloaded
- 0x02: Server shutting down
- 0x03: Restart requested

**Behavior**: Sender closes connection after sending. Receiver logs and cleans up.

---

## Binary Encoding Rules

### Endianness
All multi-byte integers use **big-endian** (network byte order) for cross-platform compatibility.

### Strings
UTF-8 encoded, length-prefixed:
```
[2: Length] [N: UTF-8 bytes]
```

### Arrays
Count-prefixed:
```
[4: Count] [N: Elements]
```

### Alignment
No padding - packed binary format to minimize size.

---

## Protocol Flow Examples

### Initial Connection

```
1. Go Server: Start listening on port 5001
2. Plugin: Connect to localhost:5001
3. Plugin → Go: HANDSHAKE {version=1, id=0x12345678}
4. Go → Plugin: HANDSHAKE {version=1, id=0xABCDEF00}
5. Go → Plugin: RESYNC_REQUEST {reason=0x02}
6. Plugin → Go: FULL_STATE {width=100, height=100, depth=10, tiles=[...]}
7. Connection established, periodic HEARTBEAT begins
```

### Tile Changes

```
1. DF: Dwarf mines tile at (50, 50, 5)
2. Plugin: Detects change in next poll
3. Plugin → Go: TILE_UPDATE {count=1, tiles=[(50,50,5,type=FLOOR,flags=0x02)]}
4. Go: Updates internal topology graph
```

### Reconnection

```
1. Connection lost (network issue)
2. Both sides detect via missed HEARTBEAT
3. Plugin: Retry connection every 10 seconds
4. Connection re-established
5. Repeat HANDSHAKE exchange
6. Go → Plugin: RESYNC_REQUEST {reason=0x02}
7. Plugin → Go: FULL_STATE (full resync)
```

---

## Validation Rules

### Message Validation
- Length must be ≥ 6 (minimum valid message)
- Length must match actual bytes received
- Version must be 1
- Type must be recognized (0x01-0x07)
- Payload must be valid for message type

### Tile State Validation
- Coordinates must be within map bounds
- TileType must be valid DF tiletype enum value
- Flags must have only defined bits set

### Connection Validation
- HANDSHAKE must be first message
- Cannot send FULL_STATE or TILE_UPDATE before HANDSHAKE
- Protocol versions must match exactly

---

## Logging Schema

All protocol events logged as structured JSON:

```json
{
  "timestamp": "2025-11-04T12:00:00.000Z",
  "level": "INFO",
  "component": "protocol",
  "event": "message_received",
  "message_type": "FULL_STATE",
  "message_size": 900012,
  "session_id": "abc123",
  "duration_ms": 145,
  "tile_count": 100000
}
```

**Event Types**:
- `connection_opened`
- `connection_closed`
- `message_sent`
- `message_received`
- `handshake_complete`
- `resync_triggered`
- `error_occurred`

---

## Future Extensions

Protocol designed for extensibility:

1. **Protocol Version 2**: Add new message types (e.g., entity updates, commands)
2. **Compression**: Add COMPRESSED_FULL_STATE message type with zlib payload
3. **Chunking**: Split large FULL_STATE into multiple chunks
4. **Diff Encoding**: Send tile differences instead of full states
5. **Priority Levels**: Add priority field for urgent updates

All extensions maintain backward compatibility via version negotiation.

---

## Size Estimates

| Map Size | Tiles | FULL_STATE Size | TILE_UPDATE (100 tiles) |
|----------|-------|-----------------|-------------------------|
| 100×100×10 | 100K | ~880 KB | 916 bytes |
| 200×200×20 | 800K | ~7 MB | 916 bytes |
| 200×200×100 | 4M | ~34 MB | 916 bytes |

**Performance Target**: Transmit 1M tiles in <2 seconds = 440KB/s minimum throughput.
**Actual localhost TCP**: >10 MB/s typical, so protocol overhead is not bottleneck.