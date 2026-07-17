# DFHack AI Protocol Plugin

Binary protocol plugin for communicating between Dwarf Fortress (via DFHack) and the Go AI orchestrator.

## Requirements

### DFHack
- **Version**: DFHack 53.02-r1 or later
- **DF Version**: Dwarf Fortress 53.02
- **Download**: https://github.com/DFHack/dfhack/releases

### Build Tools

#### Windows
- **Visual Studio 2022** with C++ tools (MSVC v143 toolchain)
- **CMake 3.10+**: Download from https://cmake.org/download/
- **Ninja** (recommended): `choco install ninja`

#### Linux
- **GCC 7+** or Clang 10+
- **CMake 3.10+**: `sudo apt-get install cmake`
- **Ninja**: `sudo apt-get install ninja-build`

---

## Building the Plugin

### Step 1: Set DFHack Root

Point to your DFHack installation directory:

#### Windows (PowerShell)
```powershell
$env:DFHACK_ROOT = "C:\Games\Dwarf Fortress"
```

#### Linux/Mac (Bash)
```bash
export DFHACK_ROOT="$HOME/df"
```

### Step 2: Configure with CMake

```bash
cd dfhack-plugin
mkdir build
cd build

# Configure
cmake .. -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DDFHACK_ROOT="$DFHACK_ROOT"
```

### Step 3: Build

```bash
cmake --build .
```

**Expected output**:
```
[1/2] Building CXX object CMakeFiles/df_ai_protocol.dir/df_ai_protocol.cpp.o
[2/2] Linking CXX shared library df_ai_protocol.dll
[100%] Built target df_ai_protocol
```

### Step 4: Install

Copy the compiled plugin to your DFHack plugins directory:

#### Windows
```powershell
copy df_ai_protocol.dll "$env:DFHACK_ROOT\hack\plugins\"
```

#### Linux
```bash
cp df_ai_protocol.so "$DFHACK_ROOT/hack/plugins/"
```

---

## Configuration

Create `config/df_ai_protocol.yaml` in your Dwarf Fortress root directory:

```yaml
# Go server connection
server_host: localhost
server_port: 5001

# Polling settings (for User Story 2)
poll_interval_ms: 1000  # Check for tile changes every 1 second

# Logging
log_level: info         # debug, info, warn, error
log_file: df_ai_protocol.log
```

---

## Usage

### Auto-Load on Startup

Edit `dfhack.init` in your DF root directory:

```
# Load DF AI protocol plugin
load df_ai_protocol
enable df_ai_protocol
```

### Manual Load

From DFHack console (` key in DF):

```
load df_ai_protocol
enable df_ai_protocol
ai-connect
```

### Check Status

```
ai-status
```

**Expected output**:
```
Connected to localhost:5001
Connection ID: 0x12345678
```

### Disconnect

```
ai-disconnect
```

---

## Testing

### Prerequisites
1. Start the Go orchestrator server first:
   ```bash
   cd ../
   ./bin/df-orchestrator.exe --config config/orchestrator.yaml
   ```

2. Launch Dwarf Fortress with DFHack

3. Load or start a fort

### Verify Connection

From DFHack console:
```
ai-connect
```

**Expected**:
```
Connecting to localhost:5001...
TCP connection established
Sent HANDSHAKE (version 1, ID 0x12345678)
Connected successfully
```

**In Go server terminal**, you should see JSON logs:
```json
{"level":"INFO","msg":"connection accepted","remote":"127.0.0.1:xxxxx"}
{"level":"INFO","msg":"sent handshake","version":1,"connection_id":...}
{"level":"INFO","msg":"received handshake","version":1,"connection_id":"0x12345678"}
{"level":"INFO","msg":"handshake complete","version":1}
```

---

## Troubleshooting

### Build Errors

**"DFHACK_ROOT not found"**
- Ensure `DFHACK_ROOT` environment variable is set
- Check the path actually contains DFHack files

**"Cannot find DFHack modules"**
- Verify DFHack version is 53.02-r1 or compatible
- Check CMake can find `${DFHACK_ROOT}/CMake/DFHackPlugin.cmake`

**"clsocket not found"**
- SimpleSockets (clsocket) should be included with DFHack
- Verify your DFHack installation is complete

### Runtime Errors

**"Plugin not found"**
- Verify `.dll` (Windows) or `.so` (Linux) is in `hack/plugins/`
- Check file permissions (must be readable/executable)
- Look in `stderr.log` for detailed error messages

**"Connection refused"**
- Ensure Go server is running and listening on port 5001
- Check firewall isn't blocking localhost connections
- Verify port number matches in both server and plugin configs

**"Handshake failed"**
- Rebuild both plugin and server from same branch
- Verify protocol versions match (both should be version 1)
- Check server logs for handshake errors

---

## Development

### Code Structure

- `df_ai_protocol.cpp` - Main plugin logic, command registration, connection management
- `tile_extractor.cpp` - Map tile extraction using DFHack MapCache
- `protocol.h` - Protocol constants and message type definitions
- `CMakeLists.txt` - Build configuration

### Logging

Plugin logs to:
- DFHack console (visible in-game)
- `df_ai_protocol.log` in DF root directory (if configured)

Enable debug logging in config:
```yaml
log_level: debug
```

### Adding New Message Types

1. Add constant to `protocol.h`
2. Implement serialization in helper function
3. Add case to message handling in `df_ai_protocol.cpp`
4. Update corresponding Go code in `internal/protocol/`

---

## Next Steps

After building and installing:

1. **Test connection**: Use `ai-connect` command
2. **Request full state**: Go server will send RESYNC_REQUEST
3. **Verify tile data**: Check server logs for received tile counts
4. **Implement User Story 2**: Add tile update polling (future task)

---

## References

- [Feature Specification](../specs/001-binary-protocol/spec.md)
- [Implementation Plan](../specs/001-binary-protocol/plan.md)
- [Protocol Specification](../specs/001-binary-protocol/contracts/protocol-spec.md)
- [DFHack Plugin Development](https://docs.dfhack.org/en/latest/docs/dev/Dev-intro.html)
- [DFHack Maps API](https://docs.dfhack.org/en/latest/docs/api/Maps.html)
