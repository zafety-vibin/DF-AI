# Protocol Contracts: Bidirectional Command Messages

**Feature**: 005-llm-integration | **Date**: 2025-11-07

## Message Type Constants

```
MessageTypeCommand    = 0x09  // Server → DFHack
MessageTypeCommandAck = 0x0A  // DFHack → Server
```

---

## COMMAND Message (0x09)

**Direction**: Server → DFHack
**Purpose**: Instruct DFHack to apply designation (dig, build, cancel)

### Message Structure

```
[4: Length]      uint32 big-endian - total message size
[1: Version]     uint8  - protocol version (currently 1)
[1: Type]        uint8  - 0x09 (COMMAND)
[4: CommandID]   uint32 big-endian - unique command identifier
[1: CommandType] uint8  - command subtype (0x01=DIG, 0x02=BUILD, 0x03=CANCEL)
[N: Payload]     - command-specific data
```

### CommandType 0x01: DESIGNATE_DIG

**Payload** (12 bytes):
```
[2: X1] int16 big-endian - region start X
[2: Y1] int16 big-endian - region start Y
[2: Z1] int16 big-endian - region start Z
[2: X2] int16 big-endian - region end X
[2: Y2] int16 big-endian - region end Y
[2: Z2] int16 big-endian - region end Z
```

**Semantics**: Designate all tiles in bounding box (X1,Y1,Z1) to (X2,Y2,Z2) for mining. Inclusive on both ends.

**Validation**:
- X2 >= X1, Y2 >= Y1, Z2 >= Z1 (non-empty region)
- Coordinates within map bounds
- Z1 == Z2 (single Z-level per command for simplicity)

**Example**: Dig 5×8 bedroom at Z=95
```
CommandID: 1001
Region: (10,20,95) to (15,28,95)
Payload: 0x000A 0x0014 0x005F 0x000F 0x001C 0x005F
```

---

### CommandType 0x02: DESIGNATE_BUILD

**Payload** (7 bytes):
```
[2: X]         int16 big-endian - build location X
[2: Y]         int16 big-endian - build location Y
[2: Z]         int16 big-endian - build location Z
[1: BuildType] uint8 - construction type
```

**BuildType values**:
```
0x01 = Wall
0x02 = Floor
0x03 = UpStair
0x04 = DownStair
0x05 = UpDownStair
```

**Semantics**: Designate single tile (X,Y,Z) for construction of specified type.

**Validation**:
- Coordinates within map bounds
- BuildType valid (0x01-0x05)
- Tile must be accessible (pathable from fort)

**Example**: Build wall at (12,22,95)
```
CommandID: 1002
Position: (12,22,95)
BuildType: 0x01 (Wall)
Payload: 0x000C 0x0016 0x005F 0x01
```

---

### CommandType 0x03: DESIGNATE_CANCEL

**Payload** (12 bytes):
```
[2: X1] int16 big-endian - region start X
[2: Y1] int16 big-endian - region start Y
[2: Z1] int16 big-endian - region start Z
[2: X2] int16 big-endian - region end X
[2: Y2] int16 big-endian - region end Y
[2: Z2] int16 big-endian - region end Z
```

**Semantics**: Cancel all designations (dig or build) in specified region.

**Validation**: Same as DESIGNATE_DIG

**Example**: Cancel designations in 3×3 area
```
CommandID: 1003
Region: (10,20,95) to (12,22,95)
```

---

## COMMAND_ACK Message (0x0A)

**Direction**: DFHack → Server
**Purpose**: Acknowledge command execution result

### Message Structure

```
[4: Length]       uint32 big-endian - total message size
[1: Version]      uint8  - protocol version
[1: Type]         uint8  - 0x0A (COMMAND_ACK)
[4: CommandID]    uint32 big-endian - original command ID from COMMAND message
[1: Status]       uint8  - execution status
[2: ErrorMsgLen]  uint16 big-endian - error message length (0 if success)
[N: ErrorMsg]     string - UTF-8 error description (omitted if length=0)
```

### Status Values

```
0x00 = Success           - All tiles designated successfully
0x01 = PartialSuccess    - Some tiles designated, others blocked/invalid
0x02 = Failure           - No tiles designated (command completely failed)
```

### Example Responses

**Success**:
```
CommandID: 1001
Status: 0x00 (Success)
ErrorMsgLen: 0
ErrorMsg: (empty)
```

**Partial Success**:
```
CommandID: 1002
Status: 0x01 (PartialSuccess)
ErrorMsgLen: 35
ErrorMsg: "5 of 8 tiles blocked by existing walls"
```

**Failure**:
```
CommandID: 1003
Status: 0x02 (Failure)
ErrorMsgLen: 28
ErrorMsg: "Region out of map bounds"
```

---

## DFHack Implementation Requirements

### Command Reception

```cpp
// In df_ai_protocol.cpp message loop
case MSG_TYPE_COMMAND:
    handleCommand(payload);
    break;

void handleCommand(const std::vector<uint8_t> &payload) {
    uint32_t cmdID = read_uint32_be(payload, 0);
    uint8_t cmdType = payload[4];

    bool success = false;
    std::string error = "";

    switch (cmdType) {
        case 0x01:  // DIG
            success = applyDigDesignation(payload, error);
            break;
        case 0x02:  // BUILD
            success = applyBuildDesignation(payload, error);
            break;
        case 0x03:  // CANCEL
            success = applyCancelDesignation(payload, error);
            break;
    }

    sendCommandAck(cmdID, success ? 0x00 : 0x02, error);
}
```

### Designation Application

```cpp
bool applyDigDesignation(const std::vector<uint8_t> &payload, std::string &error) {
    // Parse coordinates
    int16_t x1 = read_int16_be(payload, 5);
    int16_t y1 = read_int16_be(payload, 7);
    int16_t z = read_int16_be(payload, 9);
    int16_t x2 = read_int16_be(payload, 11);
    int16_t y2 = read_int16_be(payload, 13);

    // Validate bounds
    if (!Maps::isValidTilePos(x1, y1, z)) {
        error = "Invalid coordinates";
        return false;
    }

    // Apply designation to each tile
    int designated = 0;
    for (int16_t x = x1; x <= x2; x++) {
        for (int16_t y = y1; y <= y2; y++) {
            df::map_block *block = Maps::getTileBlock(x, y, z);
            if (block && block->designation[x%16][y%16].bits.hidden == 0) {
                block->designation[x%16][y%16].bits.dig = df::tile_dig_designation::Default;
                designated++;
            }
        }
    }

    if (designated == 0) {
        error = "No tiles designated (all blocked or hidden)";
        return false;
    }

    return true;
}
```

### Acknowledgment Sending

```cpp
void sendCommandAck(uint32_t cmdID, uint8_t status, const std::string &error) {
    std::vector<uint8_t> msg;

    // Reserve length header
    msg.resize(4, 0);

    // Version + Type
    msg.push_back(PROTOCOL_VERSION);
    msg.push_back(0x0A);  // COMMAND_ACK

    // CommandID
    write_uint32_be(msg, cmdID);

    // Status
    msg.push_back(status);

    // Error message
    uint16_t errorLen = error.empty() ? 0 : error.size();
    write_uint16_be(msg, errorLen);
    if (errorLen > 0) {
        msg.insert(msg.end(), error.begin(), error.end());
    }

    // Fill length
    uint32_t length = msg.size();
    msg[0] = (length >> 24) & 0xFF;
    msg[1] = (length >> 16) & 0xFF;
    msg[2] = (length >> 8) & 0xFF;
    msg[3] = length & 0xFF;

    // Send
    g_socket->Send(msg.data(), msg.size());
}
```

---

## Error Handling

### Server Side

- **Command send fails**: Log error, mark command as failed, include in next AI context
- **ACK timeout** (>5s): Mark command as "acknowledgment timeout", generate failure feedback
- **ACK indicates failure**: Extract error message, include in feedback to AI

### DFHack Side

- **Invalid coordinates**: Send ACK with status=0x02, error="Region out of bounds"
- **Tiles blocked**: Send ACK with status=0x01 (partial), error="N of M tiles blocked"
- **Unknown command type**: Send ACK with status=0x02, error="Unknown command type 0xXX"
- **DF not loaded**: Send ACK with status=0x02, error="No active game"

---

## Testing Protocol

### Unit Test (Go)

```go
func TestCommandSerialization(t *testing.T) {
    cmd := &Command{
        ID: 1001,
        Type: CommandTypeDig,
        Region: Region{X1: 10, Y1: 20, Z1: 95, X2: 15, Y2: 25, Z2: 95},
    }

    data, err := cmd.Serialize()
    assert.NoError(t, err)

    // Verify header
    assert.Equal(t, uint8(0x09), data[5])  // Type = COMMAND

    // Deserialize and compare
    cmd2, err := DeserializeCommand(data)
    assert.NoError(t, err)
    assert.Equal(t, cmd.ID, cmd2.ID)
    assert.Equal(t, cmd.Region, cmd2.Region)
}
```

### Integration Test (Live DFHack)

1. Server sends DESIGNATE_DIG for region (10,20,95) to (12,22,95)
2. Observe DFHack console for message received log
3. Check DF UI - tiles should be marked for mining (blue 'd' markers)
4. Wait for COMMAND_ACK
5. Verify ACK.CommandID matches, Status = 0x00
6. Wait for TILE_UPDATE showing tiles changed from Rock → Floor
7. Verify ModificationOverlay added DUG entries for region

**Expected timeline**:
- T+0ms: COMMAND sent
- T+50ms: ACK received (DFHack applied designation)
- T+0-60s: Dwarves path to tiles and mine (DF simulation time)
- T+60s: TILE_UPDATE received with Rock → Floor transitions
- T+60s: ModificationOverlay updated with DUG modifications
