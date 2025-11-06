# Research: DFHack Binary Protocol

**Feature**: 001-binary-protocol
**Date**: 2025-11-04
**Status**: Complete

## Purpose

Resolve technical unknowns from the Technical Context to enable implementation planning. Primary research focus: DFHack plugin SDK version, APIs, and integration patterns.

---

## Research Questions

### Q1: DFHack SDK Version and Stability

**Question**: What is the current stable version of DFHack and what are the SDK API details we need?

**Findings**:
- **Latest Stable**: DFHack 52.04-r1 for DF 52.x
- **Latest Development**: DFHack 53.02-r1 for DF 53.02 (2025 copyright, actively maintained)
- **Older Versions**: DFHack 50.14-r1 for DF v50 (legacy)
- **Documentation**: https://docs.dfhack.org/en/latest/ (for latest development version)

**Decision**: Target DFHack 53.02-r1 for Dwarf Fortress 53.02 (latest development versions).

**Rationale**: Research project benefits from latest DF features and API improvements. Working with current versions avoids future compatibility issues and ensures access to newest DFHack capabilities. Development version is actively maintained and represents the future of the game.

---

### Q2: Tile Data Access APIs

**Question**: How do DFHack plugins access tile data with coordinates?

**Findings**:
- **Primary Module**: `modules/MapCache.h` for efficient map block access
- **Data Structures**:
  - `df::coord` - coordinate type (x, y, z)
  - `df::map_block` - 16×16×1 tile blocks
  - `df/map_block.h` - block-level headers
- **World Global**: `df::global::world->map` contains raw map data
- **Block-Level Storage**: Tiles stored in 16×16 blocks for space efficiency
- **Example Plugins**:
  - `tiletypes.cpp` - demonstrates tile manipulation with coordinates
  - `3dveins.cpp` - shows MapCache usage patterns

**Code Pattern**:
```cpp
#include "modules/MapCache.h"
#include "df/map_block.h"

// Get block at coordinates
MapExtras::MapCache mc;
df::map_block *block = mc.BlockAt(df::coord(x, y, z));

// Access tile in block
df::tiletype tile = block->tiletype[blockX + blockY*16];
```

**Decision**: Use `MapCache` module for efficient batch tile access. Iterate through 16×16 blocks rather than individual tiles for performance.

**Rationale**: Block-based access is how DF internally stores maps. Working with blocks directly is more efficient than per-tile queries.

---

### Q3: Event System and Callbacks

**Question**: How can plugins register callbacks for tile change events?

**Findings**:
- **Event System**: DFHack has `Eventful` plugin providing Lua and C++ event callbacks
- **Core Events**:
  - `SC_MAP_LOADED` - triggered when map loads
  - `onLoadWorld.init` - config file auto-executed on world load
- **Event Context**: One special Lua context receives events from DF and plugins
- **Tile Change Events**: Not directly exposed - would need periodic polling or hook into DF internals

**Challenge**: No native "onTileChanged" event in current DFHack API.

**Decision**: Use **polling approach** - periodically scan map blocks for changes rather than event-driven updates. For MVP, poll on-demand when Go server requests updates.

**Rationale**:
- Simpler implementation (aligns with constitution principle VII - simplicity)
- Avoids complex DF internal hooking
- Polling every game tick (or every N ticks) is sufficient for AI reaction times
- Can optimize later if benchmarks show need

**Alternative Considered**: Deep hook into DF tile modification code - Rejected because too fragile (breaks on DF updates) and violates simplicity principle.

---

### Q4: Network Communication Patterns

**Question**: What's the standard way for DFHack plugins to do network communication?

**Findings**:
- **Built-in Remote Interface**: DFHack has existing TCP server on port 5000 using Google Protocol Buffers
- **Remote API Docs**: https://docs.dfhack.org/en/stable/docs/dev/Remote.html
- **Example Plugins**: `rename` and `isoworldremote` use protobuf TCP/IP
- **SimpleSockets Library**: DFHack includes `clsocket` fork for TCP socket operations
  - ActiveSocket - initiates connections (client mode)
  - PassiveSocket - listens for connections (server mode)
  - Repository: https://github.com/DFHack/clsocket

**Architecture Options**:

**Option A: Plugin as TCP Client (Go as Server)**
- Plugin connects to Go server on startup
- Go server listens on configurable port
- Simpler for Go (doesn't need to discover plugin)

**Option B: Plugin as TCP Server (Go as Client)**
- Plugin opens server socket in DF process
- Go connects to known port
- Follows DFHack remote interface pattern

**Option C: Use DFHack Remote Interface**
- Extend existing protobuf RPC system
- Standard DFHack pattern
- Adds protobuf dependency

**Decision**: **Option A - Plugin as TCP Client**

**Rationale**:
- Go server starts first (per spec assumption)
- Simpler reconnection logic (plugin retries connection to Go)
- Avoids protobuf complexity (constitution: prefer simplicity)
- Direct binary protocol control
- Plugin crashes don't leave orphaned server sockets

**Implementation**:
- Use SimpleSockets `ActiveSocket` in C++ plugin
- Go stdlib `net.Listen()` for server
- Custom binary protocol (not protobuf)

---

### Q5: Build Requirements

**Question**: What are the CMake and compiler requirements for DFHack plugins?

**Findings**:
- **Windows**: Microsoft Visual C++ 2022 toolchain (MSVC v143) - **required** for ABI compatibility with DF v50
- **Linux**: GCC 4.8+ minimum, GCC 7+ recommended for modern features
- **CMake**: Version 3.10+ recommended (3.6-3.9 have known dependency cycle bugs)
- **Build Generator**: Ninja preferred over Unix Makefiles
- **C++ Standard**: C++17 (plugin code uses C++17 features in examples)

**Build Configuration**:
```cmake
# In dfhack-plugin/CMakeLists.txt
DFHACK_PLUGIN(df_ai_protocol df_ai_protocol.cpp tile_extractor.cpp)
target_link_libraries(df_ai_protocol clsocket)  # If using SimpleSockets
```

**Decision**:
- Target C++17 standard
- Require CMake 3.10+
- Require MSVC 2022 on Windows, GCC 7+ on Linux
- Use Ninja build generator

**Rationale**: Matches DFHack requirements, ensures ABI compatibility.

---

### Q6: Plugin Loading and Configuration

**Question**: How are DFHack plugins discovered, loaded, and configured?

**Findings**:
- **Plugin Directory**: Plugins placed in `hack/plugins/` folder
- **Auto-loading**: Can be configured to load on startup via `dfhack.init`
- **Manual Loading**: `load df_ai_protocol` command from DFHack console
- **Registration**: Add `DFHACK_PLUGIN(...)` call in `plugins/CMakeLists.txt`
- **Commands**: Plugins expose commands via `DFHACK_PLUGIN()` macro
- **Init Scripts**: `onLoadWorld.init` runs when world loads

**Configuration Pattern**:
```cpp
DFHACK_PLUGIN("df_ai_protocol");
DFHACK_PLUGIN_IS_ENABLED(is_enabled);

DFhackCExport command_result plugin_init(color_ostream &out, std::vector<PluginCommand> &commands)
{
    commands.push_back(PluginCommand(
        "ai-connect", "Connect to Go orchestrator",
        connect_command, false
    ));
    return CR_OK;
}

DFhackCExport command_result plugin_enable(color_ostream &out, bool enable)
{
    if (enable) {
        // Start TCP client, connect to Go
    } else {
        // Disconnect, cleanup
    }
    return CR_OK;
}
```

**Decision**:
- Provide both auto-enable and manual command options
- Config file: `config/df_ai_protocol.yaml` for Go server host/port
- Enable plugin with `enable df_ai_protocol` command or in `dfhack.init`

---

## Design Decisions Summary

| Decision | Choice | Alternatives Rejected |
|----------|--------|----------------------|
| **DFHack Version** | Target 53.02-r1 for DF 53.02 (latest) | Older versions (50.x, 52.x - miss latest features) |
| **Tile Access Method** | MapCache with block iteration | Per-tile queries (too slow) |
| **Change Detection** | Polling (scan blocks periodically) | Event hooks (too complex/fragile) |
| **Network Architecture** | Plugin=Client, Go=Server | Plugin=Server (harder reconnection), Protobuf RPC (extra complexity) |
| **Network Library** | SimpleSockets (clsocket) | Raw sockets (reinvent wheel), Protobuf (overkill) |
| **Build Target** | C++17, CMake 3.10+, MSVC 2022 | Older standards (miss features) |
| **Protocol Format** | Custom binary (length-prefixed) | JSON (too large), Protobuf (extra dependency) |

---

## Resolved Technical Context

Original NEEDS CLARIFICATION:
> **Primary Dependencies**: Go stdlib (net, encoding/binary), DFHack plugin SDK **(NEEDS CLARIFICATION: version and API details)**

**Resolution**:
**Primary Dependencies**:
- Go 1.21+ stdlib (net, encoding/binary, log/slog for structured logging)
- DFHack 53.02-r1 SDK (MapCache, SimpleSockets/clsocket, df::coord, df::map_block)
- CMake 3.10+, MSVC 2022 (Windows) or GCC 7+ (Linux)

---

## Risks and Mitigations

### Risk 1: DFHack API Instability
**Likelihood**: Medium
**Impact**: High (plugin breaks on DF/DFHack updates)
**Mitigation**:
- Use stable, well-documented APIs (MapCache, not internal DF structures)
- Version-check at plugin init, warn if unsupported DFHack version
- Document supported DFHack version range

### Risk 2: Tile Polling Performance
**Likelihood**: Medium
**Impact**: Medium (FPS drop if scanning too frequently)
**Mitigation**:
- Start with conservative poll rate (1Hz)
- Benchmark on 200x200x100 map (4M tiles)
- Only scan "dirty" blocks if performance issue found
- Constitution allows optimization if benchmarks show need

### Risk 3: Socket Disconnection Handling
**Likelihood**: High (expected during development)
**Impact**: Low (auto-reconnect requirement)
**Mitigation**:
- Exponential backoff on reconnection attempts
- Comprehensive logging of all connection state changes
- Graceful degradation: plugin continues running even if disconnected

---

## Next Steps (Phase 1)

1. Create data-model.md defining binary message formats
2. Define contracts for protocol messages in contracts/
3. Write quickstart.md with build and usage instructions
4. Update plan.md with resolved dependencies

---

## References

- [DFHack Development Docs](https://docs.dfhack.org/en/stable/docs/dev/Dev-intro.html)
- [Maps API Reference](https://docs.dfhack.org/en/stable/docs/api/Maps.html)
- [DFHack Remote Interface](https://docs.dfhack.org/en/stable/docs/dev/Remote.html)
- [SimpleSockets (clsocket) Library](https://github.com/DFHack/clsocket)
- [Example Plugins: tiletypes.cpp](https://github.com/DFHack/dfhack/blob/master/plugins/tiletypes.cpp)
- [Example Plugins: 3dveins.cpp](https://github.com/DFHack/dfhack/blob/master/plugins/3dveins.cpp)
- [DFHack Compilation Guide](https://docs.dfhack.org/en/stable/docs/dev/compile/Compile.html)
