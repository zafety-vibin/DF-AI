# Testing Guide: DFHack Binary Protocol MVP

**Feature**: 001-binary-protocol (User Story 1)
**Status**: Ready for testing
**Date**: 2025-11-04

## What We're Testing

The complete handshake flow between Go server and DFHack C++ plugin:
1. Go server starts and listens on port 5001
2. DFHack plugin connects to server
3. Both exchange HANDSHAKE messages
4. Server requests full state (RESYNC_REQUEST)
5. Plugin extracts all tiles and sends FULL_STATE
6. Server receives and logs tile data

---

## Step 0: Install Build Environment (If Needed)

### Check if you have MSVC 2022

```powershell
cl.exe
```

If not found:

### Install Visual Studio 2022 Build Tools

**Option A: Full Visual Studio (Recommended if you want IDE)**
1. Download: https://visualstudio.microsoft.com/downloads/
2. Run installer
3. Select "Desktop development with C++"
4. Install (~7-10 GB)

**Option B: Build Tools Only (Lighter)**
1. Download: https://visualstudio.microsoft.com/downloads/#build-tools-for-visual-studio-2022
2. Run installer
3. Select "C++ build tools"
4. Install (~3-5 GB)

**Option C: Chocolatey**
```powershell
# Run PowerShell as Administrator
choco install visualstudio2022buildtools --package-parameters "--add Microsoft.VisualStudio.Workload.VCTools"
choco install cmake
```

### Verify Installation

```powershell
cl.exe          # Should show Microsoft C++ compiler
cmake --version # Should show CMake 3.x
```

---

## Step 1: Ensure Prerequisites

### You Need:
- ✅ **Dwarf Fortress 53.02** installed
- ✅ **DFHack 53.02-r1** installed in your DF folder
- ✅ **MSVC 2022** or **GCC 7+** (for building plugin)
- ✅ **CMake 3.10+**

### Find Your DF Installation

Your DFHack should be installed somewhere like:
- `C:\Games\Dwarf Fortress\`
- `C:\DF\`
- `C:\Users\<you>\Downloads\df_53_02_win\`

Look for these files in the folder:
- `Dwarf Fortress.exe`
- `dfhack.exe` or `dfhack-run.exe`
- `hack\` folder with plugins

---

## Step 2: Build the C++ Plugin

```powershell
# Set your DFHack path (IMPORTANT: Use your actual path!)
$env:DFHACK_ROOT = "C:\Games\Dwarf Fortress"

# Navigate to plugin directory
cd C:\Users\zmanl\Projects\DF-AI\dfhack-plugin

# Create build directory
mkdir build
cd build

# Configure with CMake
cmake .. -G "Visual Studio 17 2022" -A x64 -DDFHACK_ROOT="$env:DFHACK_ROOT"

# Build
cmake --build . --config Release
```

**Expected output**:
```
[100%] Built target df_ai_protocol
```

**Result**: `build/Release/df_ai_protocol.dll` created

---

## Step 3: Install the Plugin

```powershell
# Copy plugin to DFHack plugins folder
copy Release\df_ai_protocol.dll "$env:DFHACK_ROOT\hack\plugins\"
```

Verify the file is there:
```powershell
dir "$env:DFHACK_ROOT\hack\plugins\df_ai_protocol.dll"
```

---

## Step 4: Start the Go Server

In a **new PowerShell window**:

```powershell
cd C:\Users\zmanl\Projects\DF-AI

# Start the server
.\bin\df-orchestrator.exe --config config\orchestrator.yaml
```

**Expected output** (JSON format):
```json
{"level":"INFO","msg":"DF AI Orchestrator starting","version":"0.1.0-001-binary-protocol","protocol_version":1,"port":5001}
{"level":"INFO","msg":"server listening for DFHack connections","address":":5001"}
{"level":"INFO","msg":"waiting for DFHack plugin to connect..."}
```

✅ **Server is now running and waiting**

**Leave this terminal open!**

---

## Step 5: Launch Dwarf Fortress with DFHack

**Option A: If you have an existing fort**
1. Launch `dfhack-run.exe` (or `dfhack.exe`)
2. Continue an existing fort
3. Press `` ` `` (backtick key) to open DFHack console

**Option B: Start new game**
1. Launch `dfhack-run.exe`
2. Start new game → Create world → Embark
3. After embark loads, press `` ` `` for DFHack console

---

## Step 6: Load the Plugin

In the DFHack console (the black window that appeared):

```
load df_ai_protocol
```

**Expected output**:
```
DF AI Protocol plugin initialized
Use 'ai-connect' to connect to Go orchestrator
```

✅ **Plugin loaded successfully**

---

## Step 7: Connect to Go Server

In DFHack console:

```
ai-connect
```

**Expected output in DFHack console**:
```
Connecting to localhost:5001...
TCP connection established
Sent HANDSHAKE (version 1, ID 0x12345678)
Received server HANDSHAKE (version 1, ID 0xABCDEF00)
Handshake complete - connected successfully
```

**Expected output in Go server terminal** (new JSON logs):
```json
{"level":"INFO","msg":"connection accepted","remote":"127.0.0.1:xxxxx"}
{"level":"DEBUG","msg":"received handshake","version":1,"connection_id":"0x12345678"}
{"level":"DEBUG","msg":"sent handshake","version":1,"connection_id":...}
{"level":"INFO","msg":"handshake complete","version":1}
```

✅ **HANDSHAKE SUCCESSFUL!** 🎉

---

## Step 8: Test Full State Transfer

### 8a: Server Requests Full State

In the Go server code, I have it automatically request full state. But let me add a manual trigger.

For now, in DFHack console, check for incoming messages:

```
ai-listen
```

**Expected output in DFHack console**:
```
Listening for server messages... (checking once)
Received RESYNC_REQUEST - sending full state...
Map dimensions: 144x144x10
Extracting full map state...
Extracted 207360 tiles (1823 KB)
Sent FULL_STATE (207360 tiles, 1866960 bytes)
```

**Expected output in Go server terminal**:
```json
{"level":"INFO","msg":"received full state","width":144,"height":144,"depth":10,"tiles":207360}
```

✅ **FULL STATE TRANSFER SUCCESSFUL!** 🎉

---

## Step 9: Verify Connection Status

In DFHack console:

```
ai-status
```

**Expected output**:
```
Connected to localhost:5001
Connection ID: 0x12345678
```

---

## Troubleshooting

### "Plugin not found" when loading

**Problem**: `load df_ai_protocol` says not found

**Solutions**:
1. Verify DLL is in plugins folder:
   ```powershell
   dir "$env:DFHACK_ROOT\hack\plugins\df_ai_protocol.dll"
   ```
2. Check `stderr.log` in DF folder for detailed errors
3. Rebuild plugin with correct DFHACK_ROOT path

---

### "Failed to connect to server"

**Problem**: Plugin can't connect to port 5001

**Solutions**:
1. Verify Go server is running (check the other terminal)
2. Test port is open:
   ```powershell
   Test-NetConnection -ComputerName localhost -Port 5001
   ```
3. Check firewall isn't blocking
4. Verify config has correct port (5001 in both)

---

### "Protocol version mismatch"

**Problem**: Handshake fails with version error

**Solutions**:
1. Rebuild both Go server and C++ plugin from same code
2. Verify both show "Protocol Version: 1"
3. Check for stale binaries:
   ```powershell
   .\bin\df-orchestrator.exe --version
   ```

---

### Build Errors

**"DFHACK_ROOT not found"**
- Set environment variable:
  ```powershell
  $env:DFHACK_ROOT = "C:\Your\Actual\Path\To\DF"
  ```

**"Cannot find DFHackPlugin.cmake"**
- Verify DFHACK_ROOT points to folder containing `CMake/` subfolder
- Check DFHack is fully installed (not just DF)

**"clsocket not found"**
- Verify DFHack installation is complete
- SimpleSockets (clsocket) should be included with DFHack

---

## Success Criteria

You've successfully completed User Story 1 (MVP) when:

- ✅ Go server starts without errors
- ✅ DFHack plugin loads in DF
- ✅ Plugin connects to server (`ai-connect`)
- ✅ Both exchange HANDSHAKE messages
- ✅ Plugin receives RESYNC_REQUEST
- ✅ Plugin sends FULL_STATE with all tiles
- ✅ Server logs received tile count
- ✅ No crashes on either side

---

## What Works Now (User Story 1)

- ✅ TCP connection establishment
- ✅ Binary protocol with big-endian encoding
- ✅ Bidirectional HANDSHAKE
- ✅ RESYNC_REQUEST from server
- ✅ FULL_STATE with complete tile data
- ✅ Protocol version validation
- ✅ Structured logging on both sides

## What's Next (User Stories 2-4)

- 🔲 **US2**: Automatic tile update notifications (no manual `ai-listen` needed)
- 🔲 **US3**: Automatic reconnection on disconnect
- 🔲 **US4**: Enhanced logging and metrics

---

## Quick Command Reference

### Go Server
```powershell
# Start server
.\bin\df-orchestrator.exe --config config\orchestrator.yaml

# Check version
.\bin\df-orchestrator.exe --version
```

### DFHack Console Commands
```
load df_ai_protocol      # Load the plugin
ai-connect               # Connect to Go server
ai-status                # Check connection status
ai-listen                # Check for incoming messages (manual)
ai-disconnect            # Disconnect from server
```

---

## Log Files

**Go Server**: stdout (terminal output)
**DFHack Plugin**: DFHack console + `df_ai_protocol.log` (if configured)

Enable debug logging for more details:
- Go: Edit `config/orchestrator.yaml`, set `log_level: debug`
- Plugin: Create `config/df_ai_protocol.yaml` with `log_level: debug`

---

## Need Help?

If something doesn't work:
1. Check both terminals for error messages
2. Verify all prerequisites are installed
3. Ensure paths are correct (DFHACK_ROOT)
4. Check `stderr.log` in DF folder
5. Try rebuilding both Go server and C++ plugin

Report issues with:
- Exact error message
- Output from both terminals
- DF version and DFHack version
- Build environment (MSVC version, CMake version)
