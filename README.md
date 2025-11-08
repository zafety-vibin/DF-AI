# DF-AI: Autonomous Dwarf Fortress AI

An experimental AI system that autonomously manages Dwarf Fortress forts using LLM reasoning, modification tracking, and bidirectional command execution.

## Features

- **Modification Tracking**: Sparse overlay tracking only player/AI tile changes (not 6.9M natural tiles)
- **Chamber Extraction**: Flood-fill identifies connected rooms with natural language descriptions
- **Viewport System**: 4-level context assembly (500B → 10KB → 50KB → 200KB) optimized for LLM tokens
- **LLM Integration**: Claude API and OpenAI-compatible local models with modular provider interface
- **Autonomous Decision Loop**: AI observes → reasons → commands → learns from outcomes every 100s
- **Task Completion Tracking**: Progress feedback (8/15 tiles mined, 53% complete)
- **Bidirectional Protocol**: Server sends DIG/CANCEL commands to DFHack, receives acknowledgments
- **Embark Point Awareness**: AI knows starting location and builds near wagon/dwarves

## Prerequisites

### Required Software

1. **Dwarf Fortress** (tested with 53.02)
   - Steam version: `C:\Program Files (x86)\Steam\steamapps\common\Dwarf Fortress`

2. **DFHack** (53.02-r1)
   - Installed in DF directory: `Dwarf Fortress/hack/`

3. **Go** (1.21 or later)
   - Download: https://go.dev/dl/
   - Verify: `go version`

4. **C++ Compiler** (for plugin builds)
   - Visual Studio 2022 Build Tools or MSVC
   - CMake 3.10+

5. **LLM Provider** (choose one):
   - **Claude API**: Anthropic account with API key (recommended)
   - **Local LLM**: vLLM, llama.cpp, Ollama, or LM Studio running locally

## Installation

### 1. Clone Repository

```bash
git clone https://github.com/zafety-vibin/DF-AI.git
cd DF-AI
```

### 2. Build Go Server

```bash
# Install dependencies
go mod tidy

# Build orchestrator
go build -o bin/df-orchestrator.exe ./cmd/df-orchestrator

# Verify build
./bin/df-orchestrator.exe --version
```

### 3. Build DFHack Plugin

The plugin source is in `dfhack-plugin/`. You need a DFHack build environment.

**If you have dfhack-build**:

```bash
# Copy plugin source to dfhack-build
cp dfhack-plugin/*.cpp <dfhack-build-path>/plugins/
cp dfhack-plugin/*.h <dfhack-build-path>/plugins/

# Update CMakeLists.custom.txt in dfhack-build/plugins/:
# dfhack_plugin(df_ai_protocol
#     df_ai_protocol.cpp
#     tile_extractor.cpp
#     tile_updates.cpp
#     LINK_LIBRARIES clsocket
# )

# Build
cd <dfhack-build-path>/build
cmake --build . --target df_ai_protocol --config Release

# Copy to DF
cp build/plugins/Release/df_ai_protocol.plug.dll \
   "C:\Program Files (x86)\Steam\steamapps\common\Dwarf Fortress\hack\plugins\"
```

**Pre-built plugin**: Available in releases (when published)

### 4. Configure LLM Provider

**Option A: Claude API**

Create environment variable:
```powershell
$env:CLAUDE_API_KEY="sk-ant-your-api-key-here"
```

Or edit `config/orchestrator.yaml`:
```yaml
llm_provider_type: claude
claude_api_key: "sk-ant-your-api-key-here"
claude_model: claude-sonnet-4
```

**Option B: Local LLM**

Start your local model server (e.g., vLLM, Ollama), then configure:
```yaml
llm_provider_type: openai_compatible
llm_endpoint: http://localhost:8000
llm_model: meta-llama/Llama-3-70b-hf
llm_api_key: ""  # Empty for local servers
```

## Running the System

### 1. Start the Orchestrator Server

```bash
./bin/df-orchestrator.exe --config config/orchestrator.yaml
```

Expected output:
```
INFO config loaded
INFO DF AI Orchestrator starting
INFO server listening for DFHack connections on :5001
INFO HTTP server started on :8081
INFO waiting for DFHack plugin to connect...
```

### 2. Start Dwarf Fortress

Launch DF and start a new fortress (or load existing save).

### 3. Connect the Plugin

In the DFHack console:
```
> load df_ai_protocol
Plugin df_ai_protocol loaded successfully

> ai-connect
Connecting to localhost:5001...
TCP connection established
Sent HANDSHAKE
Received server HANDSHAKE
Handshake complete - connected successfully
Sending initial entity update...
Sent initial ENTITY_UPDATE (7 entities)

> ai-auto-update on
Auto-update enabled - will send updates every 10 heartbeats (~100 seconds)
```

### 4. Watch the AI Work

**In orchestrator logs**, you'll see:
```
INFO received full state (tiles: 4064256)
INFO topology overlay built (496 KB, 65.8% open)
INFO hazard overlays built (aquifer: 13970, water: 25432, lava: 741)
INFO modification tracking initialized (baseline: 4064256 tiles)
INFO received entity update (20 entities)
INFO initial entity data received - triggering first AI decision
INFO AI turn 1: assembling context (Level 0 + Level 1)
INFO embark point detected: (72, 89, 155)
INFO sending prompt to LLM (2,340 tokens)
INFO LLM response received (380 tokens, 3.2s)
INFO parsed command: dig region (70, 80, 155) to (75, 90, 155)
INFO executing dig command #1001
INFO command ack received: success (55 tiles designated)
INFO AI turn 1 complete - added to history
```

**In DF**, dwarves will start mining the designated area!

**View in browser**:
```
http://localhost:8081/ai/history
```

## HTTP API Endpoints

| Endpoint | Description |
|----------|-------------|
| `GET /health` | Server health check |
| `GET /ready` | Kubernetes readiness probe |
| `GET /metrics` | Full system metrics (overlays, LLM, commands, modifications) |
| `GET /ai/history` | Recent AI turns with reasoning, commands, and outcomes |

## Plugin Commands

| Command | Description |
|---------|-------------|
| `load df_ai_protocol` | Load the plugin |
| `ai-connect` | Connect to server + send initial entities |
| `ai-disconnect` | Disconnect from server |
| `ai-reconnect [N]` | Reconnect with exponential backoff (default 5 attempts) |
| `ai-status` | Check connection status |
| `ai-check-updates` | Manually scan and send tile changes |
| `ai-send-entities` | Manually send entity positions |
| `ai-auto-update [on\|off]` | Toggle automatic updates every 100s |

## Configuration

Edit `config/orchestrator.yaml`:

### Core Settings

```yaml
listen_port: 5001                    # DFHack plugin connection
http_port: 8081                      # HTTP API port
enable_http_api: true                # Enable monitoring API
log_level: info                      # debug, info, warn, error
log_format: json                     # json or text
```

### LLM Provider

```yaml
llm_provider_type: claude            # claude, openai_compatible, multi_model
claude_api_key: ${CLAUDE_API_KEY}   # From environment or literal
claude_model: claude-sonnet-4        # claude-sonnet-4, claude-opus-4, claude-haiku-4
llm_temperature: 0.7                 # Randomness (0.0-1.0)
llm_max_tokens: 4096                 # Response length
llm_timeout_seconds: 30              # API timeout
```

### Context Assembly

```yaml
context_budget_kb: 200                      # Max LLM context size
context_update_frequency_seconds: 100       # AI decision frequency
viewport_active_z_margin: 3                 # Z-levels above/below work area
viewport_hazard_margin_tiles: 10            # Hazard inclusion radius
```

### Overlays

```yaml
topology_compression_mode: full             # full, active_z_levels, custom_bounds
topology_center_z: 0                        # Center Z for active_z_levels mode
topology_z_radius: 3                        # Z-radius for active_z_levels mode
```

## How It Works

### Architecture

```
Dwarf Fortress (DF)
  ↓ (df.designation API)
DFHack Plugin (C++)
  ↓ (Binary TCP Protocol)
Go Orchestrator Server
  ├─ Modification Overlay (sparse, tracks only changes)
  ├─ Topology Overlay (850 KB bit array, all tiles)
  ├─ Hazard Overlays (aquifer, water, lava, caverns, entities)
  ├─ Context Assembler (4-level viewport system)
  ├─ LLM Client (Claude/OpenAI)
  ├─ Command Executor (DIG/CANCEL)
  └─ Autonomous Loop (decision cycle)
```

### Decision Cycle (Every 100 Seconds)

1. **Check Task Progress**: Update completion status (X/Y tiles mined)
2. **Assemble Context**: Level 0 text + Level 1 chambers/hazards/dwarves
3. **Format Prompt**: System instructions + history + feedback + context
4. **Send to LLM**: Claude API or local model (timeout 30s)
5. **Parse Response**: Extract reasoning + commands (dig/build/wait)
6. **Execute Commands**: Send to DFHack, wait for ACK
7. **Track Progress**: Monitor tile changes via TILE_UPDATE
8. **Generate Feedback**: "8/15 tiles complete" or "aquifer breached"
9. **Update History**: Add turn to sliding window (last 5 turns)
10. **Log Interaction**: Append to `logs/llm-interactions.jsonl`

### Data Compression

**What Server Stores** (full detail):
- Topology: 496 KB (all 4M tiles)
- Hazards: 3 MB (sparse maps)
- Modifications: <100 KB (sparse, only changes)

**What AI Receives** (compressed):
- Level 0: 500 bytes (text summary)
- Level 1: 10 KB (chambers + hazards + dwarves)
- **90% compression** from raw data!

## Testing

### Verify Connection

```bash
# Check server is running
curl http://localhost:8081/health

# View metrics
curl http://localhost:8081/metrics

# View AI history (after first turn)
curl http://localhost:8081/ai/history
```

### Expected First Turn Sequence

1. **Connect**: `ai-connect` in DF
2. **5 seconds**: Server receives FULL_STATE, entities, sets baseline
3. **Embark detected**: `"embark point detected: (72, 89, 155)"`
4. **AI decides**: `"sending prompt to LLM (2,340 tokens)"`
5. **AI responds**: `"LLM response received (380 tokens, 3.2s)"`
6. **Command sent**: `"executing dig command #1001"`
7. **Designation appears**: Blue 'd' markers in DF UI
8. **Dwarves mine**: Tiles change Rock → Floor over 1-5 minutes
9. **Next turn**: `"task progress: 8/15 tiles (53%)"`

### Verify Modification Tracking

After dwarves mine a few tiles:
```bash
curl http://localhost:8081/metrics | jq .modifications
```

Expected:
```json
{
  "total_count": 8,
  "chamber_count": 1,
  "bounds_x": "70-75",
  "bounds_y": "80-90",
  "bounds_z": "155-155",
  "memory_kb": 0
}
```

## Troubleshooting

### Plugin Won't Connect

**Symptom**: `ai-connect` shows "Connection refused"

**Fix**:
- Verify orchestrator is running: `curl http://localhost:8081/health`
- Check port 5001 is not blocked by firewall
- Confirm `listen_port: 5001` in config

### LLM Provider Errors

**Symptom**: `"failed to create LLM provider"`

**Fix**:
- Verify API key is set: `echo $env:CLAUDE_API_KEY`
- Check `config/orchestrator.yaml` has correct provider type
- Test API key: `curl https://api.anthropic.com/v1/messages -H "x-api-key: $CLAUDE_API_KEY"`

### No AI Decisions

**Symptom**: Logs show connection but no AI turns

**Fix**:
- Check LLM provider initialized: Logs should show "autonomous loop started"
- Verify entities were sent: Look for "received entity update" in logs
- Check for errors: `grep ERROR` in logs
- Manually trigger: The first decision triggers after initial entity data

### Dwarves Don't Mine

**Symptom**: AI sends command, ACK received, but no mining

**Fix**:
- Check DF UI for blue 'd' designation markers
- Verify coordinates are valid (within map bounds)
- Check dwarves have path to tiles (not blocked by walls/water)
- Designation may be in hidden/fog-of-war area (dwarves can't see it yet)

### High Token Usage / API Costs

**Symptom**: Claude API bills are high

**Fix**:
- Increase cycle time: `context_update_frequency_seconds: 300` (5 minutes)
- Switch to local model: `llm_provider_type: openai_compatible`
- Use multi-model pipeline: Feature extractor (local) → planner (Claude)

## Development

### Project Structure

```
DF-AI/
├── cmd/df-orchestrator/     # Main server binary
├── internal/
│   ├── modifications/       # Modification overlay (sparse, chamber extraction)
│   ├── context/             # Viewport system (4 detail levels)
│   ├── llm/                 # LLM providers (Claude, OpenAI, pipeline)
│   ├── commands/            # Command execution (DIG/CANCEL, tracking, feedback)
│   ├── autonomous/          # Decision loop (cycle, history, timer)
│   ├── topology/            # Topology overlay (bit array, RLE compression)
│   ├── hazards/             # Hazard overlays (aquifer, water, lava, entities)
│   ├── protocol/            # Binary protocol (serialization, messages)
│   ├── dfhack/              # DFHack client (TCP connection, message handling)
│   ├── config/              # Configuration (YAML, hot-reload)
│   ├── http/                # HTTP API (health, metrics, AI history)
│   └── logging/             # Structured logging (JSON/text)
├── dfhack-plugin/           # C++ DFHack plugin
│   ├── df_ai_protocol.cpp   # Main plugin (commands, entities, updates)
│   ├── tile_extractor.cpp   # Full map state extraction
│   ├── tile_updates.cpp     # Incremental change detection
│   ├── designations.cpp     # Command execution (inlined in main)
│   └── protocol.h           # Shared protocol constants
├── config/
│   └── orchestrator.yaml    # Server configuration
├── specs/                   # Feature specifications
└── tests/                   # Test suites
```

### Building from Source

```bash
# Build server
go build -o bin/df-orchestrator.exe ./cmd/df-orchestrator

# Run tests
go test ./internal/...

# Format code
go fmt ./...

# Vet code
go vet ./...
```

### Plugin Development

```bash
# In dfhack-build directory
cd <dfhack-build-path>/build
cmake --build . --target df_ai_protocol --config Release

# Copy to DF
cp build/plugins/Release/df_ai_protocol.plug.dll \
   "C:\Program Files (x86)\Steam\steamapps\common\Dwarf Fortress\hack\plugins\"
```

## Advanced Usage

### Multi-Model Pipeline

Use local model for feature extraction, Claude for planning:

```yaml
llm_provider_type: multi_model

# Extractor (local, fast, cheap)
extractor_endpoint: http://localhost:8000
extractor_model: qwen-2.5-7b

# Planner (cloud, capable)
planner_provider: claude
claude_api_key: ${CLAUDE_API_KEY}
claude_model: claude-sonnet-4
```

### Viewing Live AI Decisions

**Browser**: Navigate to `http://localhost:8081/ai/history`

**Command line** (formatted):
```bash
curl http://localhost:8081/ai/history | jq
```

**Latest turn only**:
```bash
curl -s http://localhost:8081/ai/history | jq '.turns[-1]'
```

**Watch live** (updates every 10s):
```bash
watch -n 10 'curl -s http://localhost:8081/ai/history | jq ".turns[-1] | {turn: .turn_number, reasoning: .llm_response[0:100], action: .action_taken}"'
```

### Analyzing Decision Logs

All interactions logged to `logs/llm-interactions.jsonl`:

```bash
# View latest decision
tail -1 logs/llm-interactions.jsonl | jq

# Count total turns
wc -l logs/llm-interactions.jsonl

# Extract all AI commands
jq '.action_taken' logs/llm-interactions.jsonl

# Calculate token usage
jq '.tokens.total' logs/llm-interactions.jsonl | awk '{s+=$1} END {print s}'
```

## Configuration Reference

### Timing Controls

| Setting | Default | Description |
|---------|---------|-------------|
| `context_update_frequency_seconds` | 100 | How often AI makes decisions |
| `llm_timeout_seconds` | 30 | Max wait for LLM response |
| `heartbeat_interval` | 10s | Keepalive interval |
| `heartbeat_timeout` | 15s | Connection timeout |

### Context Controls

| Setting | Default | Description |
|---------|---------|-------------|
| `context_budget_kb` | 200 | Max context size sent to LLM |
| `viewport_active_z_margin` | 3 | Z-levels above/below work area |
| `viewport_hazard_margin_tiles` | 10 | Hazard inclusion radius |

### LLM Parameters

| Setting | Default | Description |
|---------|---------|-------------|
| `llm_temperature` | 0.7 | Randomness (0.0=deterministic, 1.0=creative) |
| `llm_max_tokens` | 4096 | Response length limit |
| `llm_provider_type` | claude | Provider: claude, openai_compatible, multi_model |

## Known Limitations

### Session-Scoped Modification Tracking

**Issue**: Modifications tracked from connection, not from embark start

**Impact**:
- Fresh embark: Perfect baseline, tracks all changes ✅
- Loaded save: Baseline = current state, only tracks NEW changes ⚠️

**Workaround**: Start AI experiments from fresh embark

**Future**: Persist modification overlay to save files (Feature 10-11)

### Single Z-Level Dig Commands

**Issue**: AI must issue separate commands for multi-level digging

**Impact**: Stairwells require multiple commands (one per Z-level)

**Workaround**: AI learns to chain commands across turns

**Future**: Support 3D regions in single command

### BUILD Command Not Implemented

**Issue**: AI can't construct walls/floors/stairs yet

**Impact**: Can only dig, not build structures

**Workaround**: Player manually builds as needed

**Future**: DFHack construction API integration

## Research Notes

This is an **experimental AI research project**, not a production tool. Goals:

- Explore AI spatial reasoning in 3D environments
- Test LLM decision-making with delayed feedback
- Observe AI learning from hazard encounters (aquifer breaches, lava floods)
- Generate fine-tuning datasets from autonomous gameplay
- Study multi-turn planning and task completion strategies

**Constitution**: See `.specify/memory/constitution.md` for research principles

## Contributing

See feature specifications in `specs/` directory:
- `001-binary-protocol` - DFHack ↔ Server communication
- `002-foundation-infrastructure` - HTTP API, config, logging
- `003-topology-graph-layer` - Spatial overlay with RLE compression
- `004-hazard-overlays` - Aquifer, water, lava, caverns, entity tracking
- `005-llm-integration` - Autonomous AI with modification tracking

Each spec includes: user stories, requirements, success criteria, implementation plan, and task breakdown.

## License

[To be determined]

## Acknowledgments

- Built with [Claude Code](https://claude.com/claude-code)
- DFHack community for reverse engineering DF
- Anthropic for Claude API
- Local LLM community (vLLM, llama.cpp, Ollama)
