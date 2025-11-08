# User Story 2 Implementation Status

**Status**: 90% Complete - Needs rebuild and final testing

## Completed (Go Side) ✅

- ✅ TileUpdateMessage struct (`internal/protocol/message.go`)
- ✅ Binary codec for TILE_UPDATE (`internal/protocol/codec.go`)
- ✅ Message routing in client (`internal/dfhack/client.go`)
- ✅ Tile update subscription channel
- ✅ Main server processing tile updates (`cmd/df-orchestrator/main.go`)

## Completed (C++ Side) ✅

- ✅ Tile change detection logic (`tile_updates.cpp`)
- ✅ Helper functions made non-static (`tile_extractor.cpp`)
- ✅ Protocol declarations updated (`protocol.h`)

## Remaining Work (C++ - 15 minutes)

### 1. Update CMakeLists.txt

Add `tile_updates.cpp` to the plugin build:

```cmake
dfhack_plugin(df_ai_protocol df_ai_protocol.cpp tile_extractor.cpp tile_updates.cpp LINK_LIBRARIES clsocket)
```

### 2. Add send_tile_update() function to df_ai_protocol.cpp

After `send_full_state()`, add:

```cpp
// Send tile update message
bool send_tile_update(color_ostream &out, const std::vector<uint8_t> &tiles)
{
    if (!g_connected || !g_socket) {
        out.printerr("Not connected\n");
        return false;
    }

    uint32_t tile_count = tiles.size() / 9;
    if (tile_count == 0) {
        out.print("No changed tiles\n");
        return true;
    }

    // Build TILE_UPDATE message
    std::vector<uint8_t> message;
    message.reserve(6 + 4 + tiles.size());  // Header + count + tiles

    // Reserve for length
    message.resize(4, 0);

    // Version and type
    message.push_back(PROTOCOL_VERSION);
    message.push_back(MSG_TYPE_TILE_UPDATE);

    // Payload: [4: Count] [N: Tiles]
    write_uint32_be(message, tile_count);
    message.insert(message.end(), tiles.begin(), tiles.end());

    // Fill length
    uint32_t length = message.size();
    message[0] = (length >> 24) & 0xFF;
    message[1] = (length >> 16) & 0xFF;
    message[2] = (length >> 8) & 0xFF;
    message[3] = length & 0xFF;

    // Send
    int32_t sent = g_socket->Send(message.data(), message.size());
    if (sent != (int32_t)message.size()) {
        out.printerr("Failed to send tile update\n");
        return false;
    }

    out.print("Sent TILE_UPDATE (%d tiles)\n", tile_count);
    return true;
}
```

### 3. Add ai-check-updates command

In `plugin_init()`, add:

```cpp
commands.push_back(PluginCommand(
    "ai-check-updates",
    "Check for and send tile changes",
    [](color_ostream &out, std::vector<std::string> &params) -> command_result {
        if (!g_connected) {
            out.printerr("Not connected\n");
            return CR_FAILURE;
        }

        out.print("Checking for tile changes...\n");
        std::vector<uint8_t> changes = detect_tile_changes();

        if (changes.empty()) {
            out.print("No changes detected\n");
            return CR_OK;
        }

        uint32_t count = changes.size() / 9;
        out.print("Found %d changed tiles\n", count);

        if (send_tile_update(out, changes)) {
            return CR_OK;
        }
        return CR_FAILURE;
    },
    false,
    "Usage: ai-check-updates\nDetects and sends tile changes to server."
));
```

## How To Test US2

### 1. Rebuild Plugin

```powershell
cd C:\Users\zmanl\Projects\dfhack-build\build
cmake --build . --config Release --target df_ai_protocol
copy plugins\Release\df_ai_protocol.plug.dll "C:\Program Files (x86)\Steam\steamapps\common\Dwarf Fortress\hack\plugins\"
```

### 2. Restart Go Server

Already running with US2 support!

### 3. In DF:

```
load df_ai_protocol
ai-connect
```

### 4. Make Changes in Your Fort

Designate some tiles for mining (`d` + `d` in DF)

### 5. Check For Updates

```
ai-check-updates
```

**Expected**:
- "Found X changed tiles"
- "Sent TILE_UPDATE (X tiles)"

**In Go Server**:
```json
{"level":"INFO","msg":"tile update received","changed_tiles":X}
```

### 6. Check Again (No Changes)

```
ai-check-updates
```

**Expected**: "No changes detected" (because cache is synced)

---

## Future Enhancement (Automatic Polling)

Later, add a background thread that calls `detect_tile_changes()` every 1-5 seconds and automatically sends updates. For now, manual testing with `ai-check-updates` proves the concept.

## Files Modified

**Go**:
- `internal/protocol/message.go` - TileUpdateMessage
- `internal/protocol/codec.go` - Serialize/deserialize
- `internal/dfhack/client.go` - Routing and subscription
- `cmd/df-orchestrator/main.go` - Processing

**C++** (need to update in dfhack-build):
- `plugins/CMakeLists.txt` - Add tile_updates.cpp
- `plugins/protocol.h` - Function declarations ✅
- `plugins/tile_extractor.cpp` - Helper functions ✅
- `plugins/tile_updates.cpp` - Change detection ✅
- `plugins/df_ai_protocol.cpp` - send_tile_update() + command

**Next**: Apply C++ changes and rebuild
