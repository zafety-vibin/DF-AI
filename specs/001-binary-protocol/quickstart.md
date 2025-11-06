# Quickstart: DFHack Binary Protocol

**Feature**: 001-binary-protocol
**Last Updated**: 2025-11-04

## Overview

This guide walks through building, installing, and testing the DFHack binary protocol for DF AI orchestration.

---

## Prerequisites

### For Go Server

- **Go 1.21 or later**: [Download](https://go.dev/dl/)
- **Operating System**: Windows 10+, Linux, or macOS

### For DFHack Plugin

- **DFHack 53.02-r1**: [Download](https://github.com/DFHack/dfhack/releases)
- **Dwarf Fortress 53.02**: Latest DF version (compatible with DFHack 53.02-r1)
- **Windows**:
  - Visual Studio 2022 with C++ tools
  - CMake 3.10+
- **Linux**:
  - GCC 7+ or Clang 10+
  - CMake 3.10+
  - Ninja build tool (recommended)

---

## Part 1: Build Go Server

### 1.1 Clone Repository

```bash
git clone <repository-url>
cd DF-AI
git checkout 001-binary-protocol
```

### 1.2 Build Server

```bash
# Build the orchestrator
go build -o bin/df-orchestrator ./cmd/df-orchestrator

# Verify build
./bin/df-orchestrator --version
```

### 1.3 Configure Server

Create `config/orchestrator.yaml`:

```yaml
# Network settings
listen_port: 5001
read_timeout: 30s
write_timeout: 10s

# Protocol settings
heartbeat_interval: 10s
heartbeat_timeout: 5s
max_message_size: 10485760  # 10 MB

# Reconnection settings
reconnect_delay: 1s
reconnect_max_delay: 30s
reconnect_backoff_factor: 2.0

# Logging
log_level: info      # debug, info, warn, error
log_format: json     # json or text
```

### 1.4 Run Server

```bash
# Start server
./bin/df-orchestrator --config config/orchestrator.yaml

# Expected output:
# {"timestamp":"2025-11-04T12:00:00Z","level":"INFO","component":"server","event":"started","port":5001}
# {"timestamp":"2025-11-04T12:00:00Z","level":"INFO","component":"protocol","event":"listening","address":"0.0.0.0:5001"}
```

Server is now waiting for DFHack plugin to connect.

---

## Part 2: Build DFHack Plugin

### 2.1 Locate DFHack Installation

Find your DFHack folder (where `dfhack.exe` or `dfhack` is located).

**Windows**: Usually `C:\Games\Dwarf Fortress\`
**Linux**: Typically `~/df/`

### 2.2 Set Up Build Environment

#### Windows

```powershell
# Install dependencies via Chocolatey (if not already installed)
choco install cmake visualstudio2022buildtools

# Set DFHack path
$env:DFHACK_ROOT = "C:\Games\Dwarf Fortress"
```

#### Linux

```bash
# Install dependencies
sudo apt-get install build-essential cmake ninja-build

# Set DFHack path
export DFHACK_ROOT="$HOME/df"
```

### 2.3 Build Plugin

```bash
cd dfhack-plugin

# Create build directory
mkdir build
cd build

# Configure with CMake
cmake .. -G Ninja \
  -DCMAKE_BUILD_TYPE=Release \
  -DDFHACK_ROOT=$DFHACK_ROOT

# Build
cmake --build .

# Expected output:
# [100%] Built target df_ai_protocol
```

### 2.4 Install Plugin

```bash
# Copy plugin to DFHack plugins directory
# Windows:
copy df_ai_protocol.dll "%DFHACK_ROOT%\hack\plugins\"

# Linux:
cp df_ai_protocol.so "$DFHACK_ROOT/hack/plugins/"
```

---

## Part 3: Configure Plugin

### 3.1 Create Plugin Config

Create `config/df_ai_protocol.yaml` in your DF folder:

```yaml
# Go server connection
server_host: localhost
server_port: 5001

# Polling settings
poll_interval_ms: 1000  # Check for tile changes every 1 second

# Logging
log_level: info
log_file: df_ai_protocol.log
```

### 3.2 Auto-Load Plugin

Edit `dfhack.init` (in DF root folder) and add:

```
# Load AI protocol plugin
load df_ai_protocol
enable df_ai_protocol
```

Or load manually from DFHack console after DF starts.

---

## Part 4: Test the Connection

### 4.1 Start Go Server

In one terminal:

```bash
./bin/df-orchestrator --config config/orchestrator.yaml
```

### 4.2 Launch Dwarf Fortress with DFHack

Start Dwarf Fortress normally. DFHack will load automatically.

### 4.3 Verify Plugin Loaded

In DFHack console (opened with backtick ` key in DF):

```
plugins
```

Look for `df_ai_protocol` in the list. If not loaded:

```
load df_ai_protocol
enable df_ai_protocol
```

### 4.4 Check Connection

**In Go Server Terminal**, you should see:

```json
{"timestamp":"2025-11-04T12:01:00Z","level":"INFO","component":"protocol","event":"connection_opened","remote":"127.0.0.1:xxxxx"}
{"timestamp":"2025-11-04T12:01:00Z","level":"INFO","component":"protocol","event":"handshake_received","version":1,"connection_id":"0x12345678"}
{"timestamp":"2025-11-04T12:01:00Z","level":"INFO","component":"protocol","event":"handshake_sent","version":1}
{"timestamp":"2025-11-04T12:01:01Z","level":"INFO","component":"protocol","event":"resync_triggered","reason":"after_reconnect"}
{"timestamp":"2025-11-04T12:01:02Z","level":"INFO","component":"protocol","event":"message_received","type":"FULL_STATE","size":900012,"tiles":100000,"duration_ms":145}
```

**In DFHack Console**:

```
[df_ai_protocol] Connected to Go server at localhost:5001
[df_ai_protocol] Sent HANDSHAKE (version 1)
[df_ai_protocol] Received RESYNC_REQUEST
[df_ai_protocol] Sending FULL_STATE (100000 tiles, 880KB)
[df_ai_protocol] Connection established
```

Connection successful!

---

## Part 5: Test Tile Updates

### 5.1 Load a Fort

In Dwarf Fortress:
1. Continue an existing fort OR start a new embark
2. Unpause the game

### 5.2 Make Changes

- Designate some tiles for mining (`d` → `d` in DF)
- Wait for dwarves to mine

### 5.3 Observe Updates

**In Go Server Terminal**:

```json
{"timestamp":"2025-11-04T12:05:30Z","level":"DEBUG","component":"protocol","event":"message_received","type":"TILE_UPDATE","size":916,"changed_tiles":100}
```

Each tile change should generate a TILE_UPDATE message.

---

## Part 6: Testing Reconnection

### 6.1 Simulate Disconnect

Stop the Go server (Ctrl+C).

**Plugin should log**:
```
[df_ai_protocol] Connection lost (timeout)
[df_ai_protocol] Retrying connection in 1s...
[df_ai_protocol] Retrying connection in 2s...
[df_ai_protocol] Retrying connection in 4s...
```

### 6.2 Restart Server

Restart Go server:

```bash
./bin/df-orchestrator --config config/orchestrator.yaml
```

**Plugin should log**:
```
[df_ai_protocol] Connected to Go server at localhost:5001
[df_ai_protocol] Reconnection successful
```

**Server should log**:
```json
{"timestamp":"2025-11-04T12:10:00Z","level":"INFO","component":"protocol","event":"connection_opened"}
{"timestamp":"2025-11-04T12:10:00Z","level":"INFO","component":"protocol","event":"resync_triggered","reason":"after_reconnect"}
{"timestamp":"2025-11-04T12:10:01Z","level":"INFO","component":"protocol","event":"message_received","type":"FULL_STATE"}
```

Reconnection successful!

---

## Troubleshooting

### Plugin Won't Load

**Symptom**: `load df_ai_protocol` says "plugin not found"

**Solutions**:
1. Verify plugin file is in `hack/plugins/` folder
2. Check file extension: `.dll` (Windows) or `.so` (Linux)
3. Ensure DFHack version matches build (53.02-r1)
4. Check `stderr.log` for detailed error messages

---

### Connection Refused

**Symptom**: Plugin logs "Connection refused"

**Solutions**:
1. Verify Go server is running (`netstat -an | grep 5001`)
2. Check firewall isn't blocking port 5001
3. Verify `server_port` in plugin config matches Go server port
4. Try `telnet localhost 5001` to test connectivity

---

### Version Mismatch

**Symptom**: ERROR message with code 0x0003

**Solutions**:
1. Rebuild both plugin and server from same branch
2. Check both are using protocol version 1
3. Verify no stale binaries from previous builds

---

### Large Map Performance

**Symptom**: Full state sync takes >2 seconds for 200×200×100 map

**Solutions**:
1. This is expected (4M tiles × 9 bytes = 34MB)
2. Check network throughput: `iperf3 -c localhost`
3. Profile Go server to find bottlenecks
4. Consider implementing chunked transmission (future enhancement)

---

### Missing Tile Updates

**Symptom**: Tiles change in DF but no TILE_UPDATE messages

**Solutions**:
1. Increase `poll_interval_ms` in plugin config (try 500ms)
2. Check DFHack console for plugin errors
3. Verify plugin is enabled: `enable df_ai_protocol`
4. Check server logs for received messages

---

## Performance Benchmarks

Run these benchmarks to verify performance targets:

### Full State Sync

```bash
# In DFHack console:
ai-benchmark full-state

# Expected output:
# Map size: 100×100×10 (100,000 tiles)
# Serialization: 45ms
# Transmission: 98ms
# Total: 143ms ✓ (target: <2000ms)
```

### Tile Update Latency

```bash
# In DFHack console:
ai-benchmark tile-updates

# Expected output:
# Designating 1000 tiles for mining...
# Average update latency: 234ms ✓ (target: <1000ms)
```

### FPS Impact

```bash
# Before enabling plugin:
fps

# Enable plugin:
enable df_ai_protocol

# After enabling:
fps

# Expected: <5% FPS decrease
```

---

## Next Steps

Once the protocol is working:

1. **Integrate with Topology Graph** (Feature #4): Feed tile data into bit-packed storage
2. **Add Hazard Detection** (Feature #7): Identify aquifer/magma tiles from FULL_STATE
3. **Implement Graph Updates** (Feature #5): Process TILE_UPDATE messages into graph
4. **Monitor Performance**: Check logs and metrics regularly

---

## Development Tips

### Debug Logging

Enable debug logging for detailed protocol traces:

**Go server**:
```yaml
log_level: debug
```

**Plugin** (in config):
```yaml
log_level: debug
```

### Inspect Messages

Use Wireshark to inspect raw protocol messages:

```bash
# Capture localhost traffic on port 5001
wireshark -i lo -f "tcp port 5001"
```

### Run Integration Tests

```bash
# Go server tests
cd internal/protocol
go test -v

# Full integration test with mock plugin
cd tests/protocol
go test -v -run TestFullIntegration
```

---

## References

- [Feature Specification](spec.md)
- [Implementation Plan](plan.md)
- [Data Model](data-model.md)
- [Protocol Specification](contracts/protocol-spec.md)
- [Go Server Interface](contracts/go-server-interface.md)
- [Research Document](research.md)
- [DFHack Documentation](https://docs.dfhack.org/)
