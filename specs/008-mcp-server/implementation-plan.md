# Feature 008: MCP Server Architecture — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Turn the Go orchestrator into an MCP server (perception/action/control tools over the existing DFHack plugin bridge) so a Claude Code session can play Dwarf Fortress turn-based, culminating in the First Fort milestone.

**Architecture:** DF ⇄ C++ DFHack plugin ⇄ TCP binary protocol ⇄ Go MCP server (`cmd/df-mcp`, stdio) ⇄ Claude Code. Phase 0 fixes the plugin (hidden-tile digs, thread safety, DFHack 53.12 verification); Phase 1 adds pause/step + classified map queries plugin-side and the MCP tool surface Go-side. The bespoke LLM loop (`internal/bdi` deliberator, `internal/llm`) is retired only after First Fort succeeds (Task 16, gated).

**Tech Stack:** Go 1.24+ (toolchain 1.25.1), `github.com/modelcontextprotocol/go-sdk v1.6.1` (already in go.mod), C++20 DFHack plugin against **DFHack 53.15-r1 / DF 53.15** (upgraded 2026-07-11, the "dinosaur" update; was 53.12), in-tree build via junction (see Global Constraints).

## Global Constraints

- Module path: `github.com/df-ai/orchestrator`. Go 1.24 floor (go.mod now says 1.25.0 after SDK add — keep).
- Only new Go dependency allowed: `github.com/modelcontextprotocol/go-sdk` (v1.6.1, already fetched).
- Protocol version stays `1`. `dfhack-plugin/protocol.h` constants and `internal/protocol/message.go` constants MUST stay in sync — every task touching one touches the other.
- Plugin C++ standard: C++20 (`CMakeLists.txt` sets it). New source files must be added to `DFHACK_PLUGIN(...)` list in `dfhack-plugin/CMakeLists.txt`.
- Plugin build (CORRECTED 2026-07-11 — supersedes the `cd dfhack-build; cmake --build .` commands embedded in task steps): the DFHack **source checkout** lives at `C:\Users\zmanl\Projects\dfhack-build` (sibling of this repo), checked out at tag `53.15-r1`, with `plugins/df_ai_protocol` as a **junction to this repo's `dfhack-plugin/`** and the plugin hooked in via `plugins/CMakeLists.custom.txt`. Build: `cmake --build C:/Users/zmanl/Projects/dfhack-build/build --target df_ai_protocol --config Release` (VS 2022 generator). Output DLL: `build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll` (name pattern is `.plug.dll`) → deploy by copying to `<Steam DF>/hack/plugins/` with DF closed. In-game: DFHack console → `ai-connect`.
- Plugin threading rules: DF state reads/writes happen on the main thread via the existing queues (`g_command_queue`, `g_query_queue`, drained in `plugin_onupdate`) or under `CoreSuspender`. Never call DF from the socket thread bare.
- Go tests: standard `go test`. Every new Go package ships with unit tests (repo currently has none — this plan starts the discipline). C++ has no test harness: plugin tasks verify by compile + live smoke via `cmd/df-smoke` + DFHack console.
- Tool responses must stay comfortably under the MCP 10k-token default: `look` radius ≤ 15, list outputs capped as specified per task.
- MCP SDK idioms (verified against v1.6.1): `mcp.NewServer(&mcp.Implementation{Name, Version}, nil)`; `mcp.AddTool[In, Out](srv, &mcp.Tool{Name, Description}, handler)` with `handler = func(ctx context.Context, req *mcp.CallToolRequest, in In) (*mcp.CallToolResult, Out, error)`; text-only tools use `Out = any` and return `&mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}, nil, nil`; `srv.Run(ctx, &mcp.StdioTransport{})`. Input structs use `json` + `jsonschema` struct tags.
- Commit after every task (messages given per task). Never commit `.claude/settings.local.json`.

## File Structure (locked by this plan)

```
cmd/df-smoke/main.go              NEW  manual plugin test client (Phase 0 harness)
cmd/df-mcp/main.go                NEW  MCP server entry point
internal/mcpserver/bridge.go      NEW  dfhack client + overlays + executor wiring (from df-bdi main pattern)
internal/mcpserver/dashboard.go   NEW  ground-truth header rendered into every tool response
internal/mcpserver/tools_state.go NEW  status/alerts/dwarves/stocks/jobs/orders tools
internal/mcpserver/tools_percept.go NEW  look/cross_section/survey_site/find_dig_site tools
internal/mcpserver/tools_action.go  NEW  designate_dig/build/zone/stockpile/order/unsuspend/cancel/smooth/apply_blueprint
internal/mcpserver/tools_control.go NEW  pause/unpause/step/check_goals
internal/mapview/types.go         NEW  Slice/ColumnProfile types + JSON decode from plugin queries
internal/mapview/render.go        NEW  crop renderer (glyph grid + legend + labels)
internal/mapview/finder.go        NEW  find_dig_site geometry over *topology.TopologyOverlay
internal/mapview/*_test.go        NEW  unit tests (golden crops, finder cases, decode)
dfhack-plugin/designations.cpp    MOD  F1 hidden-tile fix + blocked-coord detail
dfhack-plugin/entities.cpp        MOD  F2 CoreSuspender
dfhack-plugin/tile_updates.cpp    MOD  uint16 serialization fix
dfhack-plugin/df_ai_protocol.cpp  MOD  F3 resync-on-main-thread; PAUSE command dispatch + step tracking
dfhack-plugin/queries.cpp         MOD  sim_status, map_slice, column_profile queries
dfhack-plugin/protocol.h          MOD  COMMAND_TYPE_PAUSE
dfhack-plugin/df_ai_protocol_backup.cpp  DELETE (not in CMake build)
internal/protocol/message.go      MOD  CommandTypePause + PauseControl
internal/protocol/codec.go        MOD  serialize/deserialize PAUSE
internal/commands/executor.go     MOD  SendPauseCommand/SendStepCommand
fortress/CLAUDE.md, fortress/.mcp.json, fortress/memory/*.md  NEW  player workspace
```

Interfaces threaded through tasks: `mcpserver.Bridge` (Task 8) is the single shared dependency all tool files consume; `mapview.SliceProvider` (Task 6) is how perception reaches the plugin.

---

### Task 1: `cmd/df-smoke` — manual plugin test client

The test harness for all plugin work: connect, send one command or query from flags, print the ACK/response. Also used to *demonstrate the F1 bug* before Task 2 fixes it.

**Files:**
- Create: `cmd/df-smoke/main.go`

**Interfaces:**
- Consumes: `dfhack.NewClient`, `Client.Start(ctx, port)`, `Client.IsConnected()`, `Client.SendQuery(ctx, name, args, timeout)`, `commands.NewCommandExecutor(logger, client, timeout)`, `CommandExecutor.SendDigRegion/SendBuildCommand/SendWorkOrderCommand`, `protocol.DigType*` constants.
- Produces: a CLI later tasks' smoke steps invoke: `go run ./cmd/df-smoke -cmd dig -digtype default -x1 .. -y1 .. -z1 .. -x2 .. -y2 .. -z2 ..`, `-cmd build -buildtype 0x10 -x -y -z`, `-cmd order -ordertype 1 -qty 2`, `-cmd query -name sim_status -args {}`, `-cmd pause|unpause|step -ticks N` (pause modes compile only after Task 5 — guard with a friendly error until then).

- [ ] **Step 1: Write `cmd/df-smoke/main.go`**

```go
// Command df-smoke is a manual test client for the DFHack plugin. It
// starts the TCP listener, waits for the plugin to connect (run
// `ai-connect` in the DFHack console), sends exactly one command or
// query, prints the result, and exits.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/protocol"
)

var (
	port     = flag.Uint("port", 5000, "TCP port to listen on (must match plugin)")
	cmd      = flag.String("cmd", "", "dig|build|order|query")
	digtype  = flag.String("digtype", "default", "default|stairs|channel|ramp|upstair|downstair")
	buildtyp = flag.Uint("buildtype", 0x10, "protocol BuildType byte (0x10=carpenter)")
	ordertyp = flag.Uint("ordertype", 1, "protocol OrderType byte (1=bed)")
	qty      = flag.Uint("qty", 1, "work order quantity")
	qname    = flag.String("name", "sim_status", "query name")
	qargs    = flag.String("args", "{}", "query args JSON")
	x1       = flag.Int("x1", 0, "")
	y1       = flag.Int("y1", 0, "")
	z1       = flag.Int("z1", 0, "")
	x2       = flag.Int("x2", 0, "")
	y2       = flag.Int("y2", 0, "")
	z2       = flag.Int("z2", 0, "")
	waitSecs = flag.Int("wait", 120, "seconds to wait for plugin connection")
)

func digTypeByte(s string) uint8 {
	switch s {
	case "stairs":
		return protocol.DigTypeUpDownStair
	case "channel":
		return protocol.DigTypeChannel
	case "ramp":
		return protocol.DigTypeRamp
	case "downstair":
		return protocol.DigTypeDownStair
	case "upstair":
		return protocol.DigTypeUpStair
	default:
		return protocol.DigTypeDefault
	}
}

func main() {
	flag.Parse()
	logger := logging.NewTextLogger("info")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := dfhack.NewClient(logger)
	if err := client.Start(ctx, uint16(*port)); err != nil {
		fmt.Fprintf(os.Stderr, "listen failed: %v\n", err)
		os.Exit(1)
	}
	exec := commands.NewCommandExecutor(logger, client, 30*time.Second)

	fmt.Printf("listening on :%d — run `ai-connect` in the DFHack console\n", *port)
	deadline := time.Now().Add(time.Duration(*waitSecs) * time.Second)
	for !client.IsConnected() {
		if time.Now().After(deadline) {
			fmt.Fprintln(os.Stderr, "plugin never connected")
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}
	// Give the FULL_STATE handshake a moment to finish before commanding.
	time.Sleep(2 * time.Second)

	var (
		res *commands.CommandResult
		err error
	)
	switch *cmd {
	case "dig":
		res, err = exec.SendDigRegion(digTypeByte(*digtype),
			int16(*x1), int16(*y1), int16(*z1), int16(*x2), int16(*y2), int16(*z2))
	case "build":
		res, err = exec.SendBuildCommand(int16(*x1), int16(*y1), int16(*z1), uint8(*buildtyp))
	case "order":
		res, err = exec.SendWorkOrderCommand(uint8(*ordertyp), uint16(*qty))
	case "query":
		var raw []byte
		raw, err = client.SendQuery(ctx, *qname, *qargs, 10*time.Second)
		if err == nil {
			fmt.Printf("QUERY OK: %s\n", string(raw))
		}
	default:
		fmt.Fprintln(os.Stderr, "unknown -cmd (dig|build|order|query)")
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: %v\n", err)
		os.Exit(1)
	}
	if res != nil {
		fmt.Printf("ACK: success=%v status=%d error=%q duration=%s\n",
			res.Success, res.Status, res.ErrorMsg, res.Duration)
	}
	_ = client.Stop()
	exec.Stop()
}
```

- [ ] **Step 2: Build it**

Run: `go build ./cmd/df-smoke`
Expected: compiles clean. If `protocol.DigType*` constant names differ, check `internal/protocol/message.go:375-380` and use the names found there (they are `DigTypeDefault=0x01, DigTypeUpDownStair=0x02, DigTypeChannel=0x03, DigTypeRamp=0x04, DigTypeDownStair=0x05, DigTypeUpStair=0x06`).

- [ ] **Step 3: Demonstrate the F1 bug against live DF (recorded evidence for Task 2)**

With DF running an active fort and the CURRENT (unfixed) plugin loaded:
1. `go run ./cmd/df-smoke -cmd dig -digtype default -x1 <wx> -y1 <wy> -z1 <wz-3> -x2 <wx+3> -y2 <wy+3> -z2 <wz-3>` where `<wx,wy,wz>` is a dwarf's position (read from DF's UI, k-look) and z-3 is solid undiscovered rock.
Expected (bug): `ACK: success=false ... error="No tiles designated (all blocked or hidden)"`.
2. Save the exact output into the commit message of Task 2.

- [ ] **Step 4: Commit**

```bash
git add cmd/df-smoke/main.go
git commit -m "feat(smoke): manual plugin test client for command/query round-trips"
```

---

### Task 2: F1 — regular digs designate hidden tiles

**Files:**
- Modify: `dfhack-plugin/designations.cpp:115-156` (the unified dig loop)

**Interfaces:**
- Produces: dig ACK semantics all later tasks rely on — hidden tiles designate fine for every dig type; `blocked` now counts only *invalid* targets; partial ACK errors name up to 5 blocked coordinates.

- [ ] **Step 1: Replace the hidden-skip branch**

In `dfhack-plugin/designations.cpp`, the loop currently reads (lines ~126-156):

```cpp
            if (isStairShaft) {
                ...
                cache.setDesignationAt(pos, des);
                designated++;
            } else {
                // Single-Z (or multi-Z non-stair) dig: skip hidden tiles
                // so the agent gets a clear "blocked" count.
                if (des.bits.hidden) {
                    blocked++;
                    continue;
                }
                des.bits.dig = dfDigType;
                cache.setDesignationAt(pos, des);
                designated++;
            }
```

Replace the `else` branch body so hidden tiles designate (DF's own UI designates through fog; dwarves reveal as they dig), and track blocked coordinates:

```cpp
            } else {
                // Hidden tiles are designated like DF's own UI does — fog
                // of war is where forts get dug. Blocked now only means
                // "DF has no tile data here at all" (off-map safety net;
                // isValidTilePos already screened the bounds).
                des.bits.dig = dfDigType;
                cache.setDesignationAt(pos, des);
                designated++;
            }
```

And ABOVE the loop, next to `int designated = 0; int blocked = 0;`, add a small blocked-coordinate sample buffer (kept for the smooth handler and future non-diggable screening):

```cpp
    int designated = 0;
    int blocked = 0;
    char blockedSample[128] = {0};  // first few blocked coords, for the ACK text
```

Then update the partial-failure message (lines ~169-176) to include the sample when non-empty:

```cpp
    if (blocked > 0 && designated > 0) {
        char buf[256];
        snprintf(buf, sizeof(buf), "%d of %d tiles blocked%s%s",
                 blocked, designated + blocked,
                 blockedSample[0] ? " at " : "", blockedSample);
        error = buf;
        // Still return true for partial success
    }
```

(With the hidden-skip gone, `blocked` stays 0 in this function today; the buffer becomes load-bearing when Task 12's validation layer or future non-diggable checks re-introduce per-tile rejection. The 128-byte sample is written wherever `blocked++` happens: `if (blocked <= 5) snprintf(blockedSample + strlen(blockedSample), sizeof(blockedSample) - strlen(blockedSample), "(%d,%d,%d) ", x, y, z);`.)

Also update the stale rationale comment at lines ~115-119 ("Hidden tiles: ... we still skip them") to match the new behavior.

- [ ] **Step 2: Rebuild the plugin**

```powershell
cd dfhack-build; cmake --build . --target df_ai_protocol
Copy-Item plugins\df_ai_protocol.dll "$env:DFHACK_ROOT\hack\plugins\" -Force
```
Expected: clean compile. (If `DFHACK_ROOT` isn't set, use the DF install path directly.)

- [ ] **Step 3: Smoke-verify against live DF**

Restart DF (or `reload df_ai_protocol` in the DFHack console), `ai-connect`, then re-run the exact Task 1 Step 3 command.
Expected now: `ACK: success=true status=0` and orange dig designations visible in DF on the hidden Z-level. Let dwarves dig it: a real room appears underground.
Also verify stairs still work: `go run ./cmd/df-smoke -cmd dig -digtype stairs -x1 <wx> -y1 <wy> -z1 <wz> -x2 <wx+1> -y2 <wy+1> -z2 <wz-5>` → 2×2 shaft designated across all 6 Z-levels.

- [ ] **Step 4: Commit (include the before/after ACK outputs in the body)**

```bash
git add dfhack-plugin/designations.cpp
git commit -m "fix(plugin): designate hidden tiles for all dig types

Regular digs previously skipped des.bits.hidden tiles, making underground
rooms undesignatable (DF hides all undug tiles). Matches DF UI semantics;
stair shafts already spanned hidden terrain. Root cause layer 1 of the
surface-only-dig bug."
```

---

### Task 3: Thread-safety + serialization fixes (F2, F3, uint16)

**Files:**
- Modify: `dfhack-plugin/entities.cpp:37-44` (extract_entities)
- Modify: `dfhack-plugin/df_ai_protocol.cpp:1373-1378` (RESYNC in socket-thread message loop) and `plugin_onupdate` (~line 588)
- Modify: `dfhack-plugin/tile_updates.cpp:65`

**Interfaces:**
- Produces: no API change; removes crash/corruption hazards before anything stacks on the plugin.

- [ ] **Step 1: F2 — suspend around unit access**

In `dfhack-plugin/entities.cpp`, `extract_entities()` begins:

```cpp
std::vector<EntityInfo> extract_entities()
{
    std::vector<EntityInfo> entities;
    auto &console = Core::getInstance().getConsole();

    // Access world units
    auto &units = df::global::world->units.active;
```

Insert a `CoreSuspender` before touching `df::global` (same pattern as `tile_extractor.cpp:59`):

```cpp
std::vector<EntityInfo> extract_entities()
{
    std::vector<EntityInfo> entities;
    auto &console = Core::getInstance().getConsole();

    // Units are read from a background thread; suspend DF for the scan.
    // CoreSuspender is reentrant/no-op when already on the core thread.
    CoreSuspender suspend;

    auto &units = df::global::world->units.active;
```

First confirm the call site really is off-thread: `rg -n "extract_entities" dfhack-plugin/` — expected: a send/timer path in `df_ai_protocol.cpp` on the socket/background thread. If it turns out to be called only from `plugin_onupdate`, the suspend is a harmless no-op; keep it anyway (documents the contract).

- [ ] **Step 2: F3 — RESYNC handled on the main thread**

In `df_ai_protocol.cpp` near the top where the queues live (lines ~81-95), add:

```cpp
// RESYNC flag — socket thread sets it, plugin_onupdate performs the
// full-state send on the main thread (extraction suspends DF anyway;
// doing it from the socket thread stalls DF mid-frame).
static std::atomic<bool> g_resync_requested{false};
```

(add `#include <atomic>` next to `#include <queue>` at line ~33).

In the message loop `case MSG_TYPE_RESYNC_REQUEST:` (~line 1373), replace the direct `send_full_state(...)` call with:

```cpp
            case MSG_TYPE_RESYNC_REQUEST:
                out.print("Received RESYNC_REQUEST — queued for main thread\n");
                g_resync_requested = true;
                break;
```

In `plugin_onupdate` (after the command-queue drain, ~line 613), add:

```cpp
    if (g_resync_requested.exchange(false)) {
        send_full_state(out);
    }
```

Match `send_full_state`'s real signature at its definition in this file (search `send_full_state`); if it takes no `color_ostream`, call it accordingly. There is a second RESYNC handler around line 737 (an earlier dispatch path) — apply the same replacement there if it still calls `send_full_state` directly.

- [ ] **Step 3: uint16 fix in tile_updates.cpp**

Line 65 currently casts a `uint16_t` tiletype through `write_int16_be`. Change to `write_uint16_be(result, tile_type);` (helper already declared in `protocol.h:148`). Verify the Go decode side reads it as uint16 (`internal/protocol/codec.go`, TileUpdate deserialization — search `TileUpdate`): if Go already reads uint16, this was a silent high-value corruption; note it in the commit.

- [ ] **Step 4: Rebuild + smoke**

```powershell
cd dfhack-build; cmake --build . --target df_ai_protocol
Copy-Item plugins\df_ai_protocol.dll "$env:DFHACK_ROOT\hack\plugins\" -Force
```
Then with DF running: `ai-connect`, confirm in the df-smoke listener that entity updates flow (start `go run ./cmd/df-smoke -cmd query -name list_orders -args {}` — any successful round-trip proves the connection), and DF does not hitch/crash over ~2 minutes of play.

- [ ] **Step 5: Commit**

```bash
git add dfhack-plugin/entities.cpp dfhack-plugin/df_ai_protocol.cpp dfhack-plugin/tile_updates.cpp
git commit -m "fix(plugin): suspend around unit scan, resync on main thread, uint16 tiletype"
```

---

### Task 4: F4 — DFHack 53.12 verification sweep + delete dead backup file

**Files:**
- Modify (as drift requires): `dfhack-plugin/buildings.cpp`, `dfhack-plugin/work_orders.cpp`, `dfhack-plugin/queries.cpp`
- Delete: `dfhack-plugin/df_ai_protocol_backup.cpp`

**Interfaces:**
- Produces: proof that BUILD (workshop/furniture/construction/door), WORK_ORDER, and the six Tier-2 queries actually compile and run against the installed DFHack — later tasks (12, 15) depend on `build` and `order` working.

- [ ] **Step 1: Delete the dead file**

`git rm dfhack-plugin/df_ai_protocol_backup.cpp` (it is not in `CMakeLists.txt`; nothing references it).

- [ ] **Step 2: Header drift sweep**

For each API the plugin uses, grep the installed SDK headers and compare (PowerShell; `$SDK` = the DFHack source/SDK include root used by `dfhack-build`):

```powershell
rg -n "Carpenters|Masons|Still" "$SDK/library/include/df/workshop_type.h"
rg -n "enum class building_type" -A 40 "$SDK/library/include/df/building_type.h"
rg -n "allocInstance|setSize|checkFreeTiles|constructAbstract|findAtTile" "$SDK/library/include/modules/Buildings.h"
rg -n "struct manager_order" -A 25 "$SDK/library/include/df/manager_order.h"
rg -n "SetPauseState|ReadPauseState" "$SDK/library/include/modules/World.h"
```

Compare against the call sites listed in `buildings.cpp:6-24` and `queries.cpp:12-23` header comments. Most likely drift (from memory `plugin-build-pipeline`): enum value insertion shifting later values, and `Buildings::checkFreeTiles` signature (`df::coord2d` size vs corner pair). Fix each mismatch at its call site — mechanical renames/signature updates only, no behavior changes.

- [ ] **Step 3: Full rebuild**

```powershell
cd dfhack-build; cmake --build . --target df_ai_protocol
```
Expected: zero errors. Every error names an exact drifted symbol — fix and repeat.

- [ ] **Step 4: Live smoke of BUILD + WORK_ORDER**

With a fort that has logs in a stockpile and an open 3×3 floor at `<x,y,z>`:
1. `go run ./cmd/df-smoke -cmd build -buildtype 0x10 -x1 <x> -y1 <y> -z1 <z>` → carpenter's workshop appears (as planned/in-construction building) in DF.
2. `go run ./cmd/df-smoke -cmd order -ordertype 1 -qty 1` → manager order "construct bed ×1" visible in DF's manager/work-orders screen.
3. `go run ./cmd/df-smoke -cmd query -name manager_orders -args {}` → JSON listing the order.
Record outcomes; failures here are findings, not blockers for perception tasks (5-11) — file them in the commit body and continue (only Tasks 12/15 hard-depend on BUILD/ORDER).

- [ ] **Step 5: Commit**

```bash
git add -A dfhack-plugin/
git commit -m "chore(plugin): verify buildings/work_orders against DFHack 53.12; drop dead backup file"
```

---

### Task 5: PAUSE command (pause / unpause / step) + `sim_status` query

**Files:**
- Modify: `dfhack-plugin/protocol.h` (after line 67), `dfhack-plugin/df_ai_protocol.cpp` (executeCommand switch ~439-574; plugin_onupdate ~588), `dfhack-plugin/queries.cpp` (dispatch ~393-408)
- Modify: `internal/protocol/message.go` (constants ~285-295; CommandMessage ~516-533; validation), `internal/protocol/codec.go` (serializeCommand ~772+; deserializeCommand ~906+), `internal/commands/executor.go` (new senders), `cmd/df-smoke/main.go` (pause/unpause/step modes)

**Interfaces:**
- Consumes: command queue + onupdate drain (existing), `binary.Write(w, binary.BigEndian, v)` codec idiom.
- Produces: `protocol.CommandTypePause = 0x0C`; `protocol.PauseControl{Mode uint8; Ticks uint32}` with `Pause PauseControl` field on `CommandMessage`; modes `protocol.PauseModeUnpause=0x00, PauseModePause=0x01, PauseModeStep=0x02`; `CommandExecutor.SendPauseCommand(pause bool) (*CommandResult, error)` and `CommandExecutor.SendStepCommand(ticks uint32) (*CommandResult, error)`; plugin query `sim_status` returning `{"paused":bool,"frame":int,"stepping":bool}`. Task 14's step tool polls `sim_status` until `paused && !stepping`.

- [ ] **Step 1: Protocol constants + struct (Go)**

`internal/protocol/message.go` — add after `CommandTypeSmooth`:

```go
	CommandTypePause uint8 = 0x0C // pause/unpause/step simulation control
```

Add near the other designation structs:

```go
// PauseControl is the payload for CommandTypePause.
// Mode: 0x00 unpause, 0x01 pause, 0x02 step (unpause, auto-pause after Ticks).
type PauseControl struct {
	Mode  uint8
	Ticks uint32 // only meaningful for Mode=0x02
}

const (
	PauseModeUnpause uint8 = 0x00
	PauseModePause   uint8 = 0x01
	PauseModeStep    uint8 = 0x02
)
```

Add `Pause PauseControl // For PAUSE commands` to `CommandMessage` (after `Smooth`). If `CommandMessage.Validate()` switches on CommandType, add a case accepting Mode ≤ 0x02 and rejecting Mode=Step with Ticks==0.

- [ ] **Step 2: Codec (Go)**

`internal/protocol/codec.go` — in `serializeCommand`'s switch (mirror the existing per-field idiom):

```go
	case CommandTypePause:
		// [1: Mode] [4: Ticks]
		if err := binary.Write(w, binary.BigEndian, msg.Pause.Mode); err != nil {
			return err
		}
		if err := binary.Write(w, binary.BigEndian, msg.Pause.Ticks); err != nil {
			return err
		}
```

In `deserializeCommand`'s switch (codec.go:905+, which reads via `binary.Read(buf, binary.BigEndian, &field)` from a `bytes.Reader`), add:

```go
	case CommandTypePause:
		if err := binary.Read(buf, binary.BigEndian, &msg.Pause.Mode); err != nil {
			return nil, err
		}
		if err := binary.Read(buf, binary.BigEndian, &msg.Pause.Ticks); err != nil {
			return nil, err
		}
```

- [ ] **Step 3: Executor senders (Go)**

`internal/commands/executor.go`, after `SendSmoothCommand`:

```go
// SendPauseCommand pauses (true) or unpauses (false) the DF simulation.
func (e *CommandExecutor) SendPauseCommand(pause bool) (*CommandResult, error) {
	mode := protocol.PauseModeUnpause
	if pause {
		mode = protocol.PauseModePause
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: mode},
	}
	return e.SendCommand(cmd)
}

// SendStepCommand unpauses DF and asks the plugin to re-pause after N
// ticks. The ACK arrives immediately ("step started"); poll the
// sim_status query for completion.
func (e *CommandExecutor) SendStepCommand(ticks uint32) (*CommandResult, error) {
	if ticks == 0 {
		return nil, fmt.Errorf("step ticks must be > 0")
	}
	cmd := &protocol.CommandMessage{
		CommandID:   e.tracker.GenerateCommandID(),
		CommandType: protocol.CommandTypePause,
		Pause:       protocol.PauseControl{Mode: protocol.PauseModeStep, Ticks: ticks},
	}
	return e.SendCommand(cmd)
}
```

- [ ] **Step 4: Round-trip unit test (Go — first codec test in the repo)**

Create `internal/protocol/codec_pause_test.go`:

```go
package protocol

import "testing"

func TestPauseCommandRoundTrip(t *testing.T) {
	orig := &CommandMessage{
		CommandID:   42,
		CommandType: CommandTypePause,
		Pause:       PauseControl{Mode: PauseModeStep, Ticks: 1200},
	}
	data, err := EncodeMessage(orig)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := DecodeMessage(data)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := decoded.(*CommandMessage)
	if !ok {
		t.Fatalf("decoded wrong type %T", decoded)
	}
	if got.Pause.Mode != PauseModeStep || got.Pause.Ticks != 1200 || got.CommandID != 42 {
		t.Fatalf("round-trip mismatch: %+v", got)
	}
}
```

Before running, check the codec's public entry points (`rg -n "func Encode|func Decode" internal/protocol/codec.go`) and use the real names (the file's own tests-of-reference are the serialize/deserialize dispatchers at codec.go:78/156 — if the public API is `Connection`-based, test `serializeCommand`/`deserializeCommand` directly instead; they are package-private and this test is in-package).

Run: `go test ./internal/protocol/ -run TestPauseCommandRoundTrip -v`
Expected: FAIL before Step 1-2 edits compile, PASS after.

- [ ] **Step 5: Plugin side (C++)**

`dfhack-plugin/protocol.h` after line 67: `constexpr uint8_t COMMAND_TYPE_PAUSE = 0x0C;`

`df_ai_protocol.cpp` — at the queue globals (~line 84), add step tracking:

```cpp
static std::atomic<int64_t> g_step_target_frame{-1};  // -1 = not stepping
```

In `executeCommand`'s switch (runs on main thread), add:

```cpp
        case COMMAND_TYPE_PAUSE: {
            // Payload: [4 cmdID][1 cmdType][1 mode][4 ticks]
            if (payload.size() < 10) {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Invalid pause payload size");
                break;
            }
            uint8_t mode = payload[5];
            uint32_t ticks = read_uint32_be(payload, 6);
            if (mode == 0x01) {              // pause
                World::SetPauseState(true);
                g_step_target_frame = -1;
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "");
            } else if (mode == 0x00) {       // unpause
                World::SetPauseState(false);
                g_step_target_frame = -1;
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "");
            } else if (mode == 0x02) {       // step
                g_step_target_frame = (int64_t)df::global::world->frame_counter + ticks;
                World::SetPauseState(false);
                sendCommandAck(cmdID, ACK_STATUS_SUCCESS, "step started");
            } else {
                sendCommandAck(cmdID, ACK_STATUS_FAILURE, "Unknown pause mode");
            }
            break;
        }
```

Add `#include "modules/World.h"` at the top of the file. `read_uint32_be` exists in `designations.cpp` — hoist its declaration into `protocol.h` (next to the `write_*` declarations) rather than duplicating:
in protocol.h add `uint32_t read_uint32_be(const std::vector<uint8_t> &data, size_t offset);` and remove the `static`-ness by moving the definition to `tile_extractor.cpp` beside the write helpers (or declare `extern` where it lives — pick whichever matches how `designations.cpp` currently defines it: it is a plain function there, so just declare it in protocol.h and delete any duplicate definitions elsewhere).

In `plugin_onupdate`, after the command-queue drain, add the auto-repause check:

```cpp
    if (g_step_target_frame >= 0 &&
        (int64_t)df::global::world->frame_counter >= g_step_target_frame) {
        World::SetPauseState(true);
        g_step_target_frame = -1;
    }
```

NOTE: `plugin_onupdate` may not run while DF is paused, but the step flow only needs it while unpaused (frames advance → onupdate fires → target reached → pause). The `sim_status` query (next step) is answered via the query queue which drains in onupdate — confirm queries still drain while paused; if DFHack does not call onupdate when paused, move the query-queue drain into `plugin_onenable`'s tick hook or answer `sim_status` directly on the socket thread reading only `World::ReadPauseState()` + the atomic (both thread-safe reads — acceptable exception, document it). Verify behavior live in Step 7.

- [ ] **Step 6: `sim_status` query (C++)**

`dfhack-plugin/queries.cpp` — add handler + dispatch entry (dispatch switch at ~line 393-408):

```cpp
extern std::atomic<int64_t> g_step_target_frame;  // defined in df_ai_protocol.cpp

static std::string querySimStatus() {
    std::string out = "{";
    out += "\"paused\":";
    out += World::ReadPauseState() ? "true" : "false";
    out += ",\"frame\":" + jsonInt((int64_t)df::global::world->frame_counter);
    out += ",\"stepping\":";
    out += (g_step_target_frame >= 0) ? "true" : "false";
    out += "}";
    return out;
}
```

Add `#include "modules/World.h"` and `#include <atomic>` to queries.cpp includes; add `else if (name == "sim_status") dataJSON = querySimStatus();` to the dispatcher.

- [ ] **Step 7: Rebuild + live smoke**

Rebuild + deploy as in Task 2 Step 2. Then, `df-smoke` gets three new `-cmd` modes — add to the switch in `cmd/df-smoke/main.go`:

```go
	case "pause":
		res, err = exec.SendPauseCommand(true)
	case "unpause":
		res, err = exec.SendPauseCommand(false)
	case "step":
		res, err = exec.SendStepCommand(uint32(*qty))
```

(reusing `-qty` as the tick count; update the `-cmd` usage string.)

Live: `-cmd pause` → DF visibly pauses; `-cmd query -name sim_status` → `{"paused":true,...}`; `-cmd step -qty 600` → DF runs ~half a game-day then re-pauses on its own; `sim_status` shows `stepping:false, paused:true`. Record whether `sim_status` answers while paused (see Step 5 NOTE) and fix per the note if not.

- [ ] **Step 8: Commit**

```bash
git add dfhack-plugin/ internal/protocol/ internal/commands/executor.go cmd/df-smoke/main.go
git commit -m "feat: PAUSE command (pause/unpause/step) + sim_status query — turn-based play substrate"
```

---

### Task 6: `map_slice` + `column_profile` queries and Go decode (`internal/mapview`)

**Files:**
- Modify: `dfhack-plugin/queries.cpp`
- Create: `internal/mapview/types.go`, `internal/mapview/types_test.go`

**Interfaces:**
- Consumes: query channel (`Client.SendQuery`), DFHack `tileShape`/`tileMaterial` (modules/Maps.h + df/tiletype.h), `MapExtras::MapCache`.
- Produces:
  - Plugin query `map_slice` args `{"x1":int,"y1":int,"z":int,"x2":int,"y2":int}` (max 48×48) → `{"z":int,"x1":int,"y1":int,"rows":["<glyphs>"...],"designated":[[x,y],...] (cap 200)}` — row 0 is y=y1, chars left→right are x1→x2.
  - Plugin query `column_profile` args `{"x":int,"y":int,"z_top":int,"z_bottom":int}` (max 60 levels) → `{"x":int,"y":int,"levels":[{"z":int,"glyph":"#","shape":"wall","material":"stone","hidden":true}...]}`.
  - Glyph legend (fixed, ASCII-only): `? hidden/undiscovered · # stone wall · % soil wall · = mineral vein wall · . floor · , grass · T tree · t sapling/shrub · _ open air · < up stair · > down stair · X up/down stair · ^ ramp · ~ water · L magma · F fortification`.
  - Go: `mapview.Slice{Z int16; X1, Y1 int16; Rows []string; Designated [][2]int16}` with `DecodeSlice(raw []byte) (*Slice, error)`; `mapview.ColumnProfile{X, Y int16; Levels []ColumnLevel}`, `ColumnLevel{Z int16; Glyph string; Shape string; Material string; Hidden bool}`, `DecodeColumnProfile(raw []byte) (*ColumnProfile, error)`; `mapview.SliceProvider` interface `{ MapSlice(ctx, x1, y1, z, x2, y2 int16) (*Slice, error); ColumnProfile(ctx, x, y, zTop, zBottom int16) (*ColumnProfile, error) }`.

- [ ] **Step 1: C++ classifier + handlers**

In `dfhack-plugin/queries.cpp`, add includes `#include "modules/Maps.h"`, `#include "modules/MapCache.h"`, `#include "df/tiletype.h"`. Then:

```cpp
// classifyTile maps a tiletype + designation to one model-facing glyph.
// Legend (keep in sync with internal/mapview and the look tool):
//   ? hidden  # stone wall  % soil wall  = mineral wall  . floor  , grass
//   T tree  t sapling/shrub  _ open air  < > X stairs  ^ ramp  ~ water
//   L magma  F fortification
static char classifyTile(df::tiletype tt, const df::tile_designation &des) {
    if (des.bits.hidden) return '?';
    if (des.bits.flow_size > 0)
        return des.bits.liquid_type == df::tile_liquid::Magma ? 'L' : '~';
    using S = df::tiletype_shape;
    using M = df::tiletype_material;
    S shape = tileShape(tt);
    M mat = tileMaterial(tt);
    switch (shape) {
        case S::WALL:
            if (mat == M::SOIL) return '%';
            if (mat == M::MINERAL) return '=';
            return '#';
        case S::FLOOR: case S::BOULDER: case S::PEBBLES:
            if (mat == M::GRASS_LIGHT || mat == M::GRASS_DARK ||
                mat == M::GRASS_DRY || mat == M::GRASS_DEAD) return ',';
            return '.';
        case S::STAIR_UP: return '<';
        case S::STAIR_DOWN: return '>';
        case S::STAIR_UPDOWN: return 'X';
        case S::RAMP: return '^';
        case S::RAMP_TOP: return '_';
        case S::SAPLING: case S::SHRUB: return 't';
        case S::TRUNK_BRANCH: case S::BRANCH: case S::TWIG: return 'T';
        case S::FORTIFICATION: return 'F';
        case S::EMPTY: case S::NONE: default: return '_';
    }
}
```

(Verify enum spellings against `$SDK/library/include/df/tiletype.h` — `tiletype_shape`/`tiletype_material` members as used; `df::tile_liquid::Magma` per `df/tile_liquid.h`. Trees in 50.x+ are multi-tile plants whose trunks read as material `TREE` walls — if `M::TREE` exists, map `S::WALL && M::TREE` to `'T'` too.)

Handler:

```cpp
static std::string queryMapSlice(const std::string &args) {
    int64_t x1 = jsonGetInt(args, "x1", -1), y1 = jsonGetInt(args, "y1", -1);
    int64_t x2 = jsonGetInt(args, "x2", -1), y2 = jsonGetInt(args, "y2", -1);
    int64_t z  = jsonGetInt(args, "z", -1);
    if (x1 < 0 || y1 < 0 || x2 < x1 || y2 < y1 || z < 0)
        return jsonError("map_slice needs x1,y1,z,x2,y2 with x2>=x1, y2>=y1");
    if ((x2 - x1 + 1) > 48 || (y2 - y1 + 1) > 48)
        return jsonError("map_slice region too large (max 48x48)");
    if (!Maps::isValidTilePos((int16_t)x1, (int16_t)y1, (int16_t)z) ||
        !Maps::isValidTilePos((int16_t)x2, (int16_t)y2, (int16_t)z))
        return jsonError("map_slice out of bounds");

    MapExtras::MapCache cache;
    std::string rows = "[";
    std::string designated = "[";
    int desCount = 0;
    for (int16_t y = (int16_t)y1; y <= (int16_t)y2; y++) {
        std::string row;
        for (int16_t x = (int16_t)x1; x <= (int16_t)x2; x++) {
            df::coord pos(x, y, (int16_t)z);
            df::tile_designation des = cache.designationAt(pos);
            df::tiletype tt = cache.tiletypeAt(pos);
            row += classifyTile(tt, des);
            if (des.bits.dig != df::tile_dig_designation::No && desCount < 200) {
                if (desCount) designated += ",";
                designated += "[" + jsonInt(x) + "," + jsonInt(y) + "]";
                desCount++;
            }
        }
        if (y != (int16_t)y1) rows += ",";
        rows += jsonStr(row);
    }
    rows += "]"; designated += "]";
    return "{\"z\":" + jsonInt(z) + ",\"x1\":" + jsonInt(x1) + ",\"y1\":" + jsonInt(y1) +
           ",\"rows\":" + rows + ",\"designated\":" + designated + "}";
}

static std::string queryColumnProfile(const std::string &args) {
    int64_t x = jsonGetInt(args, "x", -1), y = jsonGetInt(args, "y", -1);
    int64_t zt = jsonGetInt(args, "z_top", -1), zb = jsonGetInt(args, "z_bottom", -1);
    if (x < 0 || y < 0 || zt < zb || zt < 0)
        return jsonError("column_profile needs x,y,z_top>=z_bottom");
    if ((zt - zb + 1) > 60) return jsonError("column_profile too tall (max 60)");
    MapExtras::MapCache cache;
    std::string levels = "[";
    bool first = true;
    for (int16_t z = (int16_t)zt; z >= (int16_t)zb; z--) {
        if (!Maps::isValidTilePos((int16_t)x, (int16_t)y, z)) continue;
        df::coord pos((int16_t)x, (int16_t)y, z);
        df::tile_designation des = cache.designationAt(pos);
        df::tiletype tt = cache.tiletypeAt(pos);
        char g = classifyTile(tt, des);
        const char *shape = "other"; const char *mat = "other";
        switch (g) {
            case '#': shape = "wall"; mat = "stone"; break;
            case '%': shape = "wall"; mat = "soil"; break;
            case '=': shape = "wall"; mat = "mineral"; break;
            case '?': shape = "hidden"; mat = "unknown"; break;
            case ',': shape = "floor"; mat = "grass"; break;
            case '.': shape = "floor"; mat = "rock_or_soil"; break;
            case '_': shape = "open"; mat = "air"; break;
            case '~': shape = "liquid"; mat = "water"; break;
            case 'L': shape = "liquid"; mat = "magma"; break;
            case '<': case '>': case 'X': shape = "stair"; mat = "carved"; break;
            case '^': shape = "ramp"; mat = "carved"; break;
            case 'T': case 't': shape = "plant"; mat = "wood"; break;
            case 'F': shape = "fortification"; mat = "stone"; break;
        }
        if (!first) levels += ",";
        first = false;
        levels += "{\"z\":" + jsonInt(z) + ",\"glyph\":" + jsonStr(std::string(1, g)) +
                  ",\"shape\":" + jsonStr(shape) + ",\"material\":" + jsonStr(mat) +
                  ",\"hidden\":" + (des.bits.hidden ? "true" : "false") + "}";
    }
    levels += "]";
    return "{\"x\":" + jsonInt(x) + ",\"y\":" + jsonInt(y) + ",\"levels\":" + levels + "}";
}
```

Dispatcher entries: `else if (name == "map_slice") dataJSON = queryMapSlice(args);` and `else if (name == "column_profile") dataJSON = queryColumnProfile(args);`

- [ ] **Step 2: Write failing Go decode test**

`internal/mapview/types_test.go`:

```go
package mapview

import "testing"

func TestDecodeSlice(t *testing.T) {
	raw := []byte(`{"z":110,"x1":70,"y1":80,"rows":["##..","#?.,",",,,_"],"designated":[[71,80]]}`)
	s, err := DecodeSlice(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if s.Z != 110 || s.X1 != 70 || s.Y1 != 80 {
		t.Fatalf("header mismatch: %+v", s)
	}
	if len(s.Rows) != 3 || s.Rows[1] != "#?.," {
		t.Fatalf("rows mismatch: %+v", s.Rows)
	}
	if len(s.Designated) != 1 || s.Designated[0] != [2]int16{71, 80} {
		t.Fatalf("designated mismatch: %+v", s.Designated)
	}
}

func TestDecodeColumnProfile(t *testing.T) {
	raw := []byte(`{"x":70,"y":80,"levels":[{"z":111,"glyph":",","shape":"floor","material":"grass","hidden":false},{"z":110,"glyph":"?","shape":"hidden","material":"unknown","hidden":true}]}`)
	c, err := DecodeColumnProfile(raw)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(c.Levels) != 2 || c.Levels[1].Z != 110 || !c.Levels[1].Hidden {
		t.Fatalf("levels mismatch: %+v", c.Levels)
	}
}
```

Run: `go test ./internal/mapview/ -v` — Expected: FAIL (package doesn't exist).

- [ ] **Step 3: Implement `internal/mapview/types.go`**

```go
// Package mapview decodes the plugin's classified map queries and renders
// model-facing views (crops, cross-sections). It is the perception layer:
// the harness owns the map; the model sees small semantic views of it.
package mapview

import (
	"context"
	"encoding/json"
	"fmt"
)

// Legend is the fixed glyph legend appended to every rendered view.
// Keep in sync with classifyTile in dfhack-plugin/queries.cpp.
const Legend = "? hidden(undug fog: diggable!) # stone-wall % soil-wall = mineral-vein " +
	". floor , grass T tree t sapling/shrub _ open-air < up-stair > down-stair " +
	"X up/down-stair ^ ramp ~ water L magma F fortification"

type Slice struct {
	Z          int16       `json:"z"`
	X1         int16       `json:"x1"`
	Y1         int16       `json:"y1"`
	Rows       []string    `json:"rows"`
	Designated [][2]int16  `json:"designated"`
}

type ColumnLevel struct {
	Z        int16  `json:"z"`
	Glyph    string `json:"glyph"`
	Shape    string `json:"shape"`
	Material string `json:"material"`
	Hidden   bool   `json:"hidden"`
}

type ColumnProfile struct {
	X      int16         `json:"x"`
	Y      int16         `json:"y"`
	Levels []ColumnLevel `json:"levels"`
}

func DecodeSlice(raw []byte) (*Slice, error) {
	var s Slice
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil, fmt.Errorf("map_slice decode: %w (raw: %.120s)", err, raw)
	}
	if len(s.Rows) == 0 {
		return nil, fmt.Errorf("map_slice returned no rows (raw: %.120s)", raw)
	}
	return &s, nil
}

func DecodeColumnProfile(raw []byte) (*ColumnProfile, error) {
	var c ColumnProfile
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil, fmt.Errorf("column_profile decode: %w (raw: %.120s)", err, raw)
	}
	return &c, nil
}

// SliceProvider fetches classified map data. Implemented over the dfhack
// client's SendQuery in cmd/df-mcp; faked in tests.
type SliceProvider interface {
	MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*Slice, error)
	ColumnProfile(ctx context.Context, x, y, zTop, zBottom int16) (*ColumnProfile, error)
}
```

- [ ] **Step 4: Run tests**

Run: `go test ./internal/mapview/ -v` — Expected: both PASS.

- [ ] **Step 5: Rebuild plugin + live smoke**

Rebuild/deploy (Task 2 Step 2 commands). Then: `go run ./cmd/df-smoke -cmd query -name map_slice -args "{\"x1\":<wx-5>,\"y1\":<wy-5>,\"z\":<wz>,\"x2\":<wx+5>,\"y2\":<wy+5>}"` around a dwarf → an 11×11 grid of mostly `,`/`.`/`T` at the surface. Then the same at `<wz-3>` → mostly `?` (hidden rock). `column_profile` at the wagon: grass on top, `%` soil bands, `#`/`?` below.

- [ ] **Step 6: Commit**

```bash
git add dfhack-plugin/queries.cpp internal/mapview/
git commit -m "feat: classified map_slice + column_profile queries with Go decode (mapview)"
```

---

### Task 7: Scaffold `cmd/df-mcp` with a `status` tool, register in `.mcp.json`

**Files:**
- Create: `cmd/df-mcp/main.go`, `internal/mcpserver/server.go`, `.mcp.json` (repo root)

**Interfaces:**
- Consumes: MCP SDK v1.6.1 idioms (see Global Constraints).
- Produces: `mcpserver.New(bridge *Bridge) *mcp.Server` — Task 8 fills Bridge; until then `New(nil)` serves `status` reporting "no game connection". `TextResult(s string) *mcp.CallToolResult` helper every later tool uses. Server name `df-fortress`, version from a `const Version = "0.1.0"`.

- [ ] **Step 1: Write `internal/mcpserver/server.go`**

```go
// Package mcpserver exposes the DF bridge as MCP tools. Thin layer: all
// game logic lives in the packages it wraps (mapview, commands, topology,
// worldmodel, predicate).
package mcpserver

import (
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const Version = "0.1.0"

// TextResult wraps a plain string as an MCP text tool result.
func TextResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: s}},
	}
}

// New builds the MCP server and registers every tool. bridge may be nil
// during scaffolding (tools then report the missing connection).
func New(bridge *Bridge) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "df-fortress", Version: Version}, nil)
	registerStateTools(srv, bridge)
	return srv
}
```

And a minimal `internal/mcpserver/tools_state.go` (grows in Task 8):

```go
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func registerStateTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "status",
		Description: "Connection and simulation status: is the DFHack plugin connected, is DF paused, current tick. Call this first if anything seems wrong.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		if b == nil || !b.Connected() {
			return TextResult("NOT CONNECTED: start DF, then run `ai-connect` in the DFHack console. The MCP server listens on the port in config/orchestrator.yaml."), nil, nil
		}
		return TextResult(b.StatusLine(ctx)), nil, nil
	})
}
```

And a placeholder `internal/mcpserver/bridge.go` so this compiles before Task 8:

```go
package mcpserver

import "context"

// Bridge owns the dfhack client, world model, and executor. Fleshed out
// in the bridge task; nil-safe accessors keep the scaffold runnable.
type Bridge struct{}

func (b *Bridge) Connected() bool                    { return false }
func (b *Bridge) StatusLine(ctx context.Context) string { return "status unavailable" }
```

- [ ] **Step 2: Write `cmd/df-mcp/main.go`**

```go
// Command df-mcp serves the Dwarf Fortress bridge as an MCP stdio server.
// A Claude Code session is the player; this binary is its senses and hands.
//
// IMPORTANT: stdout carries the MCP protocol. All logging goes to stderr.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mcpserver"
)

func main() {
	ctx := context.Background()
	srv := mcpserver.New(nil) // bridge wired in the next task
	fmt.Fprintln(os.Stderr, "df-mcp: serving on stdio")
	if err := srv.Run(ctx, &mcp.StdioTransport{}); err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: %v\n", err)
		os.Exit(1)
	}
}
```

- [ ] **Step 3: Build + verify the SDK API assumptions**

Run: `go build ./cmd/df-mcp ./internal/mcpserver/`
Expected: clean. If `mcp.TextContent`/`mcp.Implementation` field names drifted from these idioms, run `go doc github.com/modelcontextprotocol/go-sdk/mcp.TextContent` and adapt mechanically (tool names and behavior stay as specified).

- [ ] **Step 4: Register with Claude Code**

Create `.mcp.json` at repo root:

```json
{
  "mcpServers": {
    "df-fortress": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "./cmd/df-mcp"],
      "env": {}
    }
  }
}
```

Verify: `claude mcp list` from the repo root shows `df-fortress`. In a fresh Claude Code session, calling the `status` tool returns the NOT CONNECTED message.

- [ ] **Step 5: Commit**

```bash
git add cmd/df-mcp/ internal/mcpserver/ .mcp.json go.mod go.sum
git commit -m "feat(mcp): scaffold df-mcp stdio server with status tool"
```

---

### Task 8: Bridge wiring + ground-truth dashboard + state tools

**Files:**
- Modify: `internal/mcpserver/bridge.go` (replace placeholder), `internal/mcpserver/tools_state.go`, `cmd/df-mcp/main.go`
- Create: `internal/mcpserver/dashboard.go`, `internal/mcpserver/dashboard_test.go`

**Interfaces:**
- Consumes: the exact wiring pattern from `cmd/df-bdi/main.go:93-186` (client, executor, worldmodel, populator, overlays-on-FULL_STATE); `worldmodel.Snapshot` fields used by `internal/bdi/render.go` (`snap.Tick`, `snap.Fort.{Valid,DaysElapsed,Season,Year}`, `snap.Entities.{Dwarves,Enemies,Animals}`, `snap.ActiveAlerts`); `Client.SendQuery`; `query.NewStarterRegistry` NOT used (MCP replaces it) — Tier-2 plugin queries are called directly by name.
- Produces:
  - `Bridge` struct with fields `Client *dfhack.Client`, `Exec *commands.CommandExecutor`, `WM *worldmodel.WorldModel`, `Populator *worldmodel.Populator`, `Preds *predicate.Library`, `Logger *logging.Logger` and methods:
    - `NewBridge(cfgPath string) (*Bridge, error)` — full wiring incl. FULL_STATE overlay install (copy df-bdi's callback verbatim minus LLM), `Start(ctx) error`, `Stop()`.
    - `Connected() bool`; `Snapshot() worldmodel.Snapshot`; `Topo() *topology.TopologyOverlay` (nil until FULL_STATE; every consumer nil-checks); `Query(ctx, name, argsJSON string) ([]byte, error)` (5s timeout wrapper); `StatusLine(ctx) string`.
    - `Dashboard(ctx) string` — the ground-truth header **prepended to every tool response** by a shared helper `withDash(b *Bridge, ctx context.Context, body string) *mcp.CallToolResult`.
  - Tools registered: `status` (upgraded), `alerts`, `dwarves`, `dwarf_detail(id)`, `stocks`, `jobs`, `orders` (the last four proxy plugin queries `stockpile_inventory`, `workshop_jobs`, `list_orders`+`manager_orders`, `dwarf_detail`).
  - `renderDashboard(snap worldmodel.Snapshot, connected bool, simJSON string) string` — pure function, unit-tested: one compact block like

```
[GROUND TRUTH tick=48210 year=125 season=spring day=32 | dwarves=7 enemies=0 | paused=true | alerts: 2 active (1 warn)]
```

- [ ] **Step 1: Failing test for the dashboard renderer**

`internal/mcpserver/dashboard_test.go`:

```go
package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestRenderDashboard(t *testing.T) {
	snap := worldmodel.Snapshot{Tick: 48210}
	snap.Fort.Valid = true
	snap.Fort.Year = 125
	snap.Fort.Season = 0
	snap.Fort.DaysElapsed = 32
	snap.Entities.Dwarves = make([]protocol.EntityInfo, 7)
	out := renderDashboard(snap, true, `{"paused":true,"frame":48210,"stepping":false}`)
	for _, want := range []string{"tick=48210", "dwarves=7", "paused=true", "year=125"} {
		if !strings.Contains(out, want) {
			t.Fatalf("dashboard missing %q in %q", want, out)
		}
	}
	if strings.Count(out, "\n") > 1 {
		t.Fatalf("dashboard must be one line, got %q", out)
	}
}
```

Run: `go test ./internal/mcpserver/ -run TestRenderDashboard -v` — Expected: FAIL (`renderDashboard` undefined). If `worldmodel.Snapshot` literal construction fails because fields differ, check `internal/worldmodel/types.go` and adjust the literal — the fields above are exactly those `internal/bdi/render.go` reads, so they exist; only nesting may differ.

- [ ] **Step 2: Implement `dashboard.go`**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// renderDashboard is the one-line ground-truth header. Injected into every
// tool response so the model never acts on remembered state (prior-art
// lesson: models trust stale beliefs for hours).
func renderDashboard(snap worldmodel.Snapshot, connected bool, simJSON string) string {
	if !connected {
		return "[GROUND TRUTH: no game connection — run ai-connect in the DFHack console]"
	}
	paused := "unknown"
	var sim struct {
		Paused   bool `json:"paused"`
		Stepping bool `json:"stepping"`
	}
	if json.Unmarshal([]byte(simJSON), &sim) == nil {
		if sim.Stepping {
			paused = "stepping"
		} else if sim.Paused {
			paused = "true"
		} else {
			paused = "false"
		}
	}
	seasons := [4]string{"spring", "summer", "autumn", "winter"}
	season := "?"
	if snap.Fort.Valid && snap.Fort.Season < 4 {
		season = seasons[snap.Fort.Season]
	}
	nAlerts := len(snap.ActiveAlerts)
	return fmt.Sprintf("[GROUND TRUTH tick=%d year=%d season=%s day=%d | dwarves=%d enemies=%d | paused=%s | alerts=%d active]",
		snap.Tick, snap.Fort.Year, season, snap.Fort.DaysElapsed,
		len(snap.Entities.Dwarves), len(snap.Entities.Enemies), paused, nAlerts)
}

// withDash prepends the dashboard to a tool body.
func withDash(b *Bridge, ctx context.Context, body string) *mcp.CallToolResult {
	dash := "[GROUND TRUTH unavailable]"
	if b != nil {
		sim := "{}"
		if raw, err := b.Query(ctx, "sim_status", "{}"); err == nil {
			sim = string(raw)
		}
		dash = renderDashboard(b.Snapshot(), b.Connected(), sim)
	}
	return TextResult(dash + "\n\n" + body)
}
```

- [ ] **Step 3: Implement the real `Bridge`**

Replace `internal/mcpserver/bridge.go` wholesale. The wiring is `cmd/df-bdi/main.go:93-186` minus LLM/plan/reconcile/skill (copy the FULL_STATE callback verbatim — topology+hazards+modifications install):

```go
package mcpserver

import (
	"context"
	"fmt"
	"time"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/config"
	"github.com/df-ai/orchestrator/internal/dfhack"
	"github.com/df-ai/orchestrator/internal/hazards"
	"github.com/df-ai/orchestrator/internal/logging"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/predicate"
	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

// Bridge owns the game connection: dfhack client, command executor,
// world model + overlays, predicate library. One Bridge per server.
type Bridge struct {
	Client    *dfhack.Client
	Exec      *commands.CommandExecutor
	WM        *worldmodel.WorldModel
	Populator *worldmodel.Populator
	Preds     *predicate.Library
	Logger    *logging.Logger
	port      uint16
}

func NewBridge(cfgPath string) (*Bridge, error) {
	logger := logging.NewTextLogger("info") // stderr; stdout is MCP protocol
	configMgr, err := config.NewConfigManager(cfgPath, logger)
	if err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	cfg := configMgr.Get()

	client := dfhack.NewClient(logger)
	exec := commands.NewCommandExecutor(logger, client, 30*time.Second)
	wm := worldmodel.New(nil, nil, nil, logger)
	populator := worldmodel.NewPopulator(wm, client, logger)

	b := &Bridge{
		Client: client, Exec: exec, WM: wm, Populator: populator,
		Preds: predicate.NewStarterLibrary(), Logger: logger,
		port: cfg.ListenPort,
	}

	client.SetOnFullState(func(state *protocol.FullStateMessage) {
		topo := topology.NewTopologyOverlay(state.Width, state.Height, state.Depth)
		if topo == nil {
			logger.Error("topology overlay nil — invalid dimensions",
				fmt.Errorf("W=%d H=%d D=%d", state.Width, state.Height, state.Depth))
			return
		}
		if err := topo.BuildFromTiles(state.Tiles); err != nil {
			logger.Error("topology build failed", err)
			return
		}
		hzd := hazards.NewHazardManager(state.Width, state.Height, state.Depth)
		hzd.BuildFromTiles(state.Tiles)
		bounds := modifications.Bounds{Width: state.Width, Height: state.Height, Depth: state.Depth}
		mods := modifications.NewModificationOverlay(bounds)
		det := modifications.NewDetector(mods)
		det.InitializeBaseline(state.Tiles)
		wm.SetOverlays(topo, hzd, mods)
		populator.SetModDetector(det)
		populator.OnFullState(state)
		logger.Info("worldmodel installed")
	})
	return b, nil
}

func (b *Bridge) Start(ctx context.Context) error {
	if err := b.Client.Start(ctx, b.port); err != nil {
		return err
	}
	go b.Populator.Run(ctx)
	return nil
}

func (b *Bridge) Stop() {
	b.Exec.Stop()
	_ = b.Client.Stop()
}

func (b *Bridge) Connected() bool                    { return b.Client.IsConnected() }
func (b *Bridge) Snapshot() worldmodel.Snapshot      { return b.WM.Snapshot() }
func (b *Bridge) Topo() *topology.TopologyOverlay    { return b.WM.Observed.Topology }

// Query forwards a named JSON query to the plugin.
func (b *Bridge) Query(ctx context.Context, name, argsJSON string) ([]byte, error) {
	if !b.Connected() {
		return nil, fmt.Errorf("plugin not connected")
	}
	qctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return b.Client.SendQuery(qctx, name, argsJSON, 10*time.Second)
}

func (b *Bridge) StatusLine(ctx context.Context) string {
	sim := "{}"
	if raw, err := b.Query(ctx, "sim_status", "{}"); err == nil {
		sim = string(raw)
	}
	return renderDashboard(b.Snapshot(), b.Connected(), sim)
}
```

Check `worldmodel.Populator` for the exact run method (`rg -n "func \(p \*Populator\)" internal/worldmodel/populator.go`): `cmd/df-bdi` ran it via the BDI loop (`l.populator.Run(l.ctx)`), so `Run(ctx)` exists — if the signature differs, adapt `Start`. Also confirm `Fort`/`Entities`/`ActiveAlerts` field paths compile; fix per `internal/worldmodel/types.go` if nested differently.

- [ ] **Step 4: State tools**

Extend `registerStateTools` in `tools_state.go` — `status` switches to `withDash`, plus:

```go
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "alerts",
		Description: "DF's own announcement stream (job cancellations with reasons, sieges, moods, migrants). READ THIS when work isn't progressing — DF usually says exactly why.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		if len(snap.ActiveAlerts) == 0 {
			return withDash(b, ctx, "No active alerts."), nil, nil
		}
		var sb strings.Builder
		for i, a := range snap.ActiveAlerts {
			if i >= 30 {
				fmt.Fprintf(&sb, "... and %d more\n", len(snap.ActiveAlerts)-30)
				break
			}
			fmt.Fprintf(&sb, "- [%d] sev=%d %s", a.ID, a.Severity, a.Text)
			if a.HasPosition() {
				fmt.Fprintf(&sb, " @(%d,%d,%d)", a.X, a.Y, a.Z)
			}
			sb.WriteString("\n")
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarves",
		Description: "List all dwarves with id and position. Use dwarf_detail for skills/mood/job of one dwarf.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d dwarves:\n", len(snap.Entities.Dwarves))
		for _, d := range snap.Entities.Dwarves {
			fmt.Fprintf(&sb, "- id=%d @(%d,%d,%d)\n", d.ID, d.X, d.Y, d.Z)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
```

And four thin plugin-query proxies, all following this exact shape (repeat it per tool — `dwarf_detail`, `stocks`, `jobs`, `orders`):

```go
	type dwarfDetailIn struct {
		ID int `json:"id" jsonschema:"the dwarf's id from the dwarves tool"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "dwarf_detail",
		Description: "One dwarf's full record: skills, labors, mood, current job.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in dwarfDetailIn) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "dwarf_detail", fmt.Sprintf(`{"id":%d}`, in.ID))
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stocks",
		Description: "Stockpile inventory by item type and material — what the fort actually has.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		raw, err := b.Query(ctx, "stockpile_inventory", "{}")
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})

	type jobsIn struct {
		X int `json:"x,omitempty" jsonschema:"optional workshop x (omit for the plugin default)"`
		Y int `json:"y,omitempty"`
		Z int `json:"z,omitempty"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "jobs",
		Description: "Jobs queued at workshops. Optionally target one workshop by its tile.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in jobsIn) (*mcp.CallToolResult, any, error) {
		args := "{}"
		if in.X != 0 || in.Y != 0 || in.Z != 0 {
			args = fmt.Sprintf(`{"x":%d,"y":%d,"z":%d}`, in.X, in.Y, in.Z)
		}
		raw, err := b.Query(ctx, "workshop_jobs", args)
		if err != nil {
			return withDash(b, ctx, "query failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, string(raw)), nil, nil
	})
	// NOTE: check queries.cpp's workshop_jobs arg parsing once during
	// implementation — if it requires x,y,z, make the three fields required
	// in jobsIn (drop omitempty and the args=="{}" branch).

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "orders",
		Description: "Manager work orders and general job list — what's queued and its status.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		var sb strings.Builder
		for _, q := range []string{"manager_orders", "list_orders"} {
			raw, err := b.Query(ctx, q, "{}")
			if err != nil {
				fmt.Fprintf(&sb, "%s failed: %v\n", q, err)
				continue
			}
			fmt.Fprintf(&sb, "%s: %s\n", q, string(raw))
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
```

`New(...)` in `server.go` now takes the real bridge; update `cmd/df-mcp/main.go`:

```go
	bridge, err := mcpserver.NewBridge("config/orchestrator.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: bridge: %v\n", err)
		os.Exit(1)
	}
	if err := bridge.Start(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "df-mcp: start: %v\n", err)
		os.Exit(1)
	}
	defer bridge.Stop()
	srv := mcpserver.New(bridge)
```

- [ ] **Step 5: Test + live check**

Run: `go test ./internal/mcpserver/ -v` → dashboard test PASS. `go build ./...` clean.
Live: DF running + `ai-connect`; in a Claude Code session call `status` → real dashboard with tick and pause state; `dwarves` lists the embark seven; `alerts` shows current announcements.

- [ ] **Step 6: Commit**

```bash
git add internal/mcpserver/ cmd/df-mcp/main.go
git commit -m "feat(mcp): bridge wiring, ground-truth dashboard, state tools"
```

---

### Task 9: `look` + `cross_section` perception tools

**Files:**
- Create: `internal/mapview/render.go`, `internal/mapview/render_test.go`, `internal/mcpserver/tools_percept.go`
- Modify: `internal/mcpserver/server.go` (call `registerPerceptTools`)

**Interfaces:**
- Consumes: `mapview.Slice/ColumnProfile/SliceProvider/Legend` (Task 6), `Bridge.Query` (Task 8).
- Produces: `mapview.RenderCrop(s *Slice, marks map[[2]int16]rune) string` (labeled glyph grid); `mapview.RenderColumn(c *ColumnProfile, surfaceZ int16) string`; Bridge method `MapSlice/ColumnProfile` implementing `mapview.SliceProvider` (add to bridge.go); MCP tools `look(x, y, z, radius)` and `cross_section(x, y, z_top, z_bottom)`.

- [ ] **Step 1: Failing golden test for the crop renderer**

`internal/mapview/render_test.go`:

```go
package mapview

import (
	"strings"
	"testing"
)

func TestRenderCrop(t *testing.T) {
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{"##..", "#?.,", ",,,_"}}
	out := RenderCrop(s, map[[2]int16]rune{{72, 81}: '@'})
	// Header names the Z and orientation; grid has x/y labels; mark applied.
	if !strings.Contains(out, "z=110") || !strings.Contains(out, "north is up") {
		t.Fatalf("missing header: %q", out)
	}
	// Row for y=81 gets the '@' at x=72 (index 2): "#?@,"
	if !strings.Contains(out, "81 #?@,") {
		t.Fatalf("mark not applied on y=81 row: %q", out)
	}
	if !strings.Contains(out, Legend) {
		t.Fatalf("legend missing")
	}
	// x-axis label line marks the starting column.
	if !strings.Contains(out, "x=70") {
		t.Fatalf("x label missing: %q", out)
	}
}
```

Run: `go test ./internal/mapview/ -run TestRenderCrop -v` — Expected: FAIL.

- [ ] **Step 2: Implement `render.go`**

```go
package mapview

import (
	"fmt"
	"strings"
)

// RenderCrop renders a Slice as a labeled glyph grid. No spaces between
// glyphs (tokenization research: separators destroy adjacency). marks
// overlays glyphs at absolute (x,y) — used for dwarves '@' and wagon 'W'.
func RenderCrop(s *Slice, marks map[[2]int16]rune) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "z=%d — %dx%d crop at (%d,%d), north is up, x grows east, y grows south\n",
		s.Z, len(s.Rows[0]), len(s.Rows), s.X1, s.Y1)
	fmt.Fprintf(&sb, "     x=%d", s.X1)
	pad := len(s.Rows[0]) - len(fmt.Sprintf("%d", s.X1)) - 2
	if pad > 0 {
		sb.WriteString(strings.Repeat(" ", pad))
		fmt.Fprintf(&sb, "x=%d", s.X1+int16(len(s.Rows[0]))-1)
	}
	sb.WriteString("\n")
	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for pos, r := range marks {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		fmt.Fprintf(&sb, "%4d %s\n", y, string(glyphs))
	}
	sb.WriteString("\nlegend: @ dwarf W wagon " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in view\n", len(s.Designated))
	}
	return sb.String()
}

// RenderColumn renders a ColumnProfile as one line per Z, annotated
// relative to the surface.
func RenderColumn(c *ColumnProfile, surfaceZ int16) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "column (%d,%d), top to bottom:\n", c.X, c.Y)
	for _, lv := range c.Levels {
		rel := ""
		switch {
		case lv.Z == surfaceZ:
			rel = "  <- SURFACE"
		case lv.Z == surfaceZ-1:
			rel = "  <- first layer below surface"
		}
		hidden := ""
		if lv.Hidden {
			hidden = " (hidden/undug — diggable)"
		}
		fmt.Fprintf(&sb, "z=%d %s %s/%s%s%s\n", lv.Z, lv.Glyph, lv.Shape, lv.Material, hidden, rel)
	}
	return sb.String()
}
```

Run: `go test ./internal/mapview/ -v` — Expected: PASS (adjust the x-label spacing math if the golden fails; the test only requires presence of `x=70`, the y-labeled row, header, and legend).

- [ ] **Step 3: Bridge implements SliceProvider**

Append to `internal/mcpserver/bridge.go`:

```go
// MapSlice / ColumnProfile implement mapview.SliceProvider over the
// plugin query channel.
func (b *Bridge) MapSlice(ctx context.Context, x1, y1, z, x2, y2 int16) (*mapview.Slice, error) {
	args := fmt.Sprintf(`{"x1":%d,"y1":%d,"z":%d,"x2":%d,"y2":%d}`, x1, y1, z, x2, y2)
	raw, err := b.Query(ctx, "map_slice", args)
	if err != nil {
		return nil, err
	}
	return mapview.DecodeSlice(raw)
}

func (b *Bridge) ColumnProfile(ctx context.Context, x, y, zTop, zBottom int16) (*mapview.ColumnProfile, error) {
	args := fmt.Sprintf(`{"x":%d,"y":%d,"z_top":%d,"z_bottom":%d}`, x, y, zTop, zBottom)
	raw, err := b.Query(ctx, "column_profile", args)
	if err != nil {
		return nil, err
	}
	return mapview.DecodeColumnProfile(raw)
}
```

(import `internal/mapview`.)

- [ ] **Step 4: The tools**

`internal/mcpserver/tools_percept.go`:

```go
package mcpserver

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/mapview"
)

// NOTE: Tasks 10 and 11 add survey_site and find_dig_site to this file and
// extend this import block with "fmt" and "strings" when their code needs it.

func registerPerceptTools(srv *mcp.Server, b *Bridge) {
	type lookIn struct {
		X      int `json:"x" jsonschema:"center x"`
		Y      int `json:"y" jsonschema:"center y"`
		Z      int `json:"z" jsonschema:"z-level to view"`
		Radius int `json:"radius,omitempty" jsonschema:"half-width of the crop, default 12, max 15"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "look",
		Description: "Render a small annotated map crop of one z-level around (x,y). Glyph grid with legend; dwarves marked @. '?' tiles are hidden fog — solid undug ground you CAN designate digging into. Use for local layout checks; use find_dig_site for choosing dig locations.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in lookIn) (*mcp.CallToolResult, any, error) {
		r := in.Radius
		if r <= 0 {
			r = 12
		}
		if r > 15 {
			r = 15
		}
		x1, y1 := int16(in.X-r), int16(in.Y-r)
		x2, y2 := int16(in.X+r), int16(in.Y+r)
		if x1 < 0 {
			x1 = 0
		}
		if y1 < 0 {
			y1 = 0
		}
		s, err := b.MapSlice(ctx, x1, y1, int16(in.Z), x2, y2)
		if err != nil {
			return withDash(b, ctx, "look failed: "+err.Error()), nil, nil
		}
		marks := map[[2]int16]rune{}
		for _, d := range b.Snapshot().Entities.Dwarves {
			if d.Z == int16(in.Z) {
				marks[[2]int16{d.X, d.Y}] = '@'
			}
		}
		return withDash(b, ctx, mapview.RenderCrop(s, marks)), nil, nil
	})

	type xsecIn struct {
		X       int `json:"x" jsonschema:"column x"`
		Y       int `json:"y" jsonschema:"column y"`
		ZTop    int `json:"z_top,omitempty" jsonschema:"top z (default: 3 above the highest dwarf)"`
		ZBottom int `json:"z_bottom,omitempty" jsonschema:"bottom z (default: 20 below the lowest dwarf)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cross_section",
		Description: "Vertical slice at column (x,y): one line per z-level showing surface, soil bands, stone, and hidden layers. THE tool for judging how deep to dig stairs and where soil ends.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xsecIn) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		surfaceZ := int16(0)
		minDZ, maxDZ := int16(32767), int16(-32768)
		for _, d := range snap.Entities.Dwarves {
			if d.Z < minDZ {
				minDZ = d.Z
			}
			if d.Z > maxDZ {
				maxDZ = d.Z
			}
		}
		if len(snap.Entities.Dwarves) > 0 {
			surfaceZ = maxDZ
		}
		zt, zb := int16(in.ZTop), int16(in.ZBottom)
		if in.ZTop == 0 && len(snap.Entities.Dwarves) > 0 {
			zt = maxDZ + 3
		}
		if in.ZBottom == 0 && len(snap.Entities.Dwarves) > 0 {
			zb = minDZ - 20
		}
		if zb < 0 {
			zb = 0
		}
		c, err := b.ColumnProfile(ctx, int16(in.X), int16(in.Y), zt, zb)
		if err != nil {
			return withDash(b, ctx, "cross_section failed: "+err.Error()), nil, nil
		}
		return withDash(b, ctx, mapview.RenderColumn(c, surfaceZ)), nil, nil
	})
}
```

Add `registerPerceptTools(srv, bridge)` to `New` in `server.go`. (Delete the `_ = fmt.Sprintf` line if `fmt` is already used — it is, in later tasks; keep imports tidy per compiler.)

- [ ] **Step 5: Build, test, live check**

`go build ./... && go test ./internal/mapview/ ./internal/mcpserver/ -v` → PASS.
Live in Claude Code: `look` at a dwarf's position → readable surface crop with `@` marks, grass/trees; `look` 3 z below → mostly `?` with the "diggable" legend; `cross_section` at the wagon → soil bands then stone.

- [ ] **Step 6: Commit**

```bash
git add internal/mapview/ internal/mcpserver/
git commit -m "feat(mcp): look and cross_section perception tools with labeled crops"
```

---

### Task 10: `survey_site` tool

**Files:**
- Modify: `internal/mcpserver/tools_percept.go`
- Create: `internal/mcpserver/survey.go`, `internal/mcpserver/survey_test.go`

**Interfaces:**
- Consumes: `Bridge.Snapshot/Topo/ColumnProfile/MapSlice`, `topology.TopologyOverlay.GetDimensions/StatsForZRange` (usage as in `internal/bdi/render.go:363-399,469-513`).
- Produces: tool `survey_site()`; pure helper `renderSurvey(in SurveyData) string` (unit-tested) where `SurveyData{MapW, MapH, MapD uint16; SurfaceZ int16; Dwarves []protocol.EntityInfo; Columns []*mapview.ColumnProfile; SurfaceSlice *mapview.Slice}`.

- [ ] **Step 1: Failing test for the renderer**

`internal/mcpserver/survey_test.go`:

```go
package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

func TestRenderSurvey(t *testing.T) {
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, SurfaceZ: 111,
		Dwarves: []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
		Columns: []*mapview.ColumnProfile{{
			X: 95, Y: 95,
			Levels: []mapview.ColumnLevel{
				{Z: 111, Glyph: ",", Shape: "floor", Material: "grass", Hidden: false},
				{Z: 110, Glyph: "?", Shape: "hidden", Material: "unknown", Hidden: true},
				{Z: 109, Glyph: "?", Shape: "hidden", Material: "unknown", Hidden: true},
			},
		}},
	}
	out := renderSurvey(data)
	for _, want := range []string{"192x192x130", "surface z=111", "(95,95)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("survey missing %q:\n%s", want, out)
		}
	}
}
```

Run: `go test ./internal/mcpserver/ -run TestRenderSurvey -v` — Expected: FAIL.

- [ ] **Step 2: Implement `survey.go`**

```go
package mcpserver

import (
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

type SurveyData struct {
	MapW, MapH, MapD uint16
	SurfaceZ         int16
	Dwarves          []protocol.EntityInfo
	Columns          []*mapview.ColumnProfile
	SurfaceSlice     *mapview.Slice
}

// renderSurvey composes the embark orientation report: dimensions, the
// surface Z, where everyone is, and what the sampled stratigraphy looks
// like. This is the model's first read of a new map.
func renderSurvey(d SurveyData) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "EMBARK SURVEY\nmap %dx%dx%d (x east, y south, z up; z=%d is map top)\n",
		d.MapW, d.MapH, d.MapD, int(d.MapD)-1)
	fmt.Fprintf(&sb, "surface z=%d (highest dwarf z on a fresh embark)\n", d.SurfaceZ)
	if len(d.Dwarves) > 0 {
		minX, maxX, minY, maxY := d.Dwarves[0].X, d.Dwarves[0].X, d.Dwarves[0].Y, d.Dwarves[0].Y
		for _, e := range d.Dwarves {
			if e.X < minX {
				minX = e.X
			}
			if e.X > maxX {
				maxX = e.X
			}
			if e.Y < minY {
				minY = e.Y
			}
			if e.Y > maxY {
				maxY = e.Y
			}
		}
		fmt.Fprintf(&sb, "%d dwarves clustered in (%d,%d)-(%d,%d)\n",
			len(d.Dwarves), minX, minY, maxX, maxY)
	}
	for _, c := range d.Columns {
		if c == nil || len(c.Levels) == 0 {
			continue
		}
		soil, stone, firstStone := 0, 0, int16(-1)
		for _, lv := range c.Levels {
			switch lv.Material {
			case "soil":
				soil++
			case "stone", "mineral":
				stone++
				if firstStone == -1 {
					firstStone = lv.Z
				}
			}
		}
		fmt.Fprintf(&sb, "column (%d,%d): %d soil layers, first exposed stone at z=%d (hidden layers below are undug rock — diggable)\n",
			c.X, c.Y, soil, firstStone)
	}
	if d.SurfaceSlice != nil {
		trees, grass := 0, 0
		for _, row := range d.SurfaceSlice.Rows {
			for _, g := range row {
				if g == 'T' || g == 't' {
					trees++
				}
				if g == ',' {
					grass++
				}
			}
		}
		fmt.Fprintf(&sb, "surface sample: %d tree/sapling tiles, %d grass tiles in a %dx%d crop around the crew\n",
			trees, grass, len(d.SurfaceSlice.Rows[0]), len(d.SurfaceSlice.Rows))
	}
	sb.WriteString("\nOpening reminders: dig a 2x2 stair shaft down into stone, carve rooms BESIDE the shaft (never on top of it), keep the surface exposure small.\n")
	return sb.String()
}
```

Run: `go test ./internal/mcpserver/ -run TestRenderSurvey -v` — Expected: PASS.

- [ ] **Step 3: The tool**

In `tools_percept.go`, inside `registerPerceptTools`:

```go
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "survey_site",
		Description: "Embark orientation report: map dimensions, surface z, dwarf cluster, sampled soil/stone stratigraphy, surface vegetation. Call FIRST on any new fort or after reconnecting.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		snap := b.Snapshot()
		topo := b.Topo()
		if topo == nil || len(snap.Entities.Dwarves) == 0 {
			return withDash(b, ctx, "survey unavailable: waiting for full state / dwarves (is ai-connect done?)"), nil, nil
		}
		w, h, d := topo.GetDimensions()
		surfaceZ := snap.Entities.Dwarves[0].Z
		cx, cy := snap.Entities.Dwarves[0].X, snap.Entities.Dwarves[0].Y
		for _, e := range snap.Entities.Dwarves {
			if e.Z > surfaceZ {
				surfaceZ, cx, cy = e.Z, e.X, e.Y
			}
		}
		data := SurveyData{MapW: w, MapH: h, MapD: d, SurfaceZ: surfaceZ, Dwarves: snap.Entities.Dwarves}
		// Three stratigraphy samples: at the crew and two offsets.
		for _, off := range [][2]int16{{0, 0}, {12, 0}, {0, 12}} {
			c, err := b.ColumnProfile(ctx, cx+off[0], cy+off[1], surfaceZ+2, surfaceZ-25)
			if err == nil {
				data.Columns = append(data.Columns, c)
			}
		}
		if s, err := b.MapSlice(ctx, cx-15, cy-15, surfaceZ, cx+15, cy+15); err == nil {
			data.SurfaceSlice = s
		}
		return withDash(b, ctx, renderSurvey(data)), nil, nil
	})
```

If `GetDimensions()` returns different integer types than `(uint16, uint16, uint16)`, check `rg -n "func .* GetDimensions" internal/topology/` and adjust `SurveyData` field types to match — the renderer only formats them.

- [ ] **Step 4: Build + test + live**

`go build ./... && go test ./internal/mcpserver/ -v` → PASS. Live: `survey_site` on the running fort returns dimensions, surface z, cluster box, and plausible soil/stone columns.

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/
git commit -m "feat(mcp): survey_site embark orientation tool"
```

---

### Task 11: `find_dig_site` — geometry in Go, ranked candidates out

**Files:**
- Create: `internal/mapview/finder.go`, `internal/mapview/finder_test.go`
- Modify: `internal/mcpserver/tools_percept.go`

**Interfaces:**
- Consumes: `topology.TopologyOverlay` (`NewTopologyOverlay(w,h,d)`, `BuildFromTiles([]protocol.TileInfo)`, `GetTileState(x,y,z) topology.TileState`, states `topology.StateOpen/StateClosed/StateUnknown`), `protocol.TileInfo` (verify field names via `go doc ./internal/protocol TileInfo` — expected `X, Y, Z int16; TileType uint16; Flags uint8`, flags per `protocol.h:29-32`).
- Produces: `mapview.FindDigSites(topo *topology.TopologyOverlay, req DigSiteRequest) []DigSite` with

```go
type DigSiteRequest struct {
	W, H          int16 // room footprint
	Z             int16 // level to search
	NearX, NearY  int16 // ranking anchor (e.g. stairs bottom / wagon)
	MaxCandidates int   // default 5
}
type DigSite struct {
	X1, Y1, X2, Y2 int16
	Z              int16
	Dist           float64 // distance from anchor
	Solid          int     // solid/hidden tiles inside (diggable mass)
	Open           int     // already-open tiles inside (0 is ideal)
	Rationale      string
}
```

MCP tool `find_dig_site(width, height, z, near_x, near_y)`.

- [ ] **Step 1: Failing unit test with a real overlay**

`internal/mapview/finder_test.go` — build a tiny synthetic map: 40×40×3, z=1 all CLOSED (walls, i.e. FLAG none) except a 4×4 OPEN pocket at (10..13,10..13); z=2 OPEN (surface). Tile flags: use `protocol.FlagDiscovered` (0x02) for known tiles; omit flags entirely for hidden. Check constant names first: `rg -n "FlagHidden|FlagDiscovered|FLAG_" internal/protocol/message.go internal/topology/*.go` and use what exists (agent-verified: Flags at `message.go:81-88`).

```go
package mapview

import (
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

func buildTestOverlay(t *testing.T) *topology.TopologyOverlay {
	t.Helper()
	topo := topology.NewTopologyOverlay(40, 40, 3)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	var tiles []protocol.TileInfo
	for x := int16(0); x < 40; x++ {
		for y := int16(0); y < 40; y++ {
			// z=1: walls everywhere except an open 4x4 pocket
			flags := uint8(protocol.FlagDiscovered)
			tt := uint16(600) // arbitrary wall-ish tiletype; state comes from flags
			if x >= 10 && x <= 13 && y >= 10 && y <= 13 {
				flags |= protocol.FlagFloor // if FlagFloor doesn't exist, see note below
			}
			tiles = append(tiles, protocol.TileInfo{X: x, Y: y, Z: 1, TileType: tt, Flags: flags})
		}
	}
	if err := topo.BuildFromTiles(tiles); err != nil {
		t.Fatalf("build: %v", err)
	}
	return topo
}

func TestFindDigSitesPrefersSolidNearAnchor(t *testing.T) {
	topo := buildTestOverlay(t)
	sites := FindDigSites(topo, DigSiteRequest{W: 5, H: 5, Z: 1, NearX: 12, NearY: 12, MaxCandidates: 3})
	if len(sites) == 0 {
		t.Fatal("no candidates found in an almost-all-solid level")
	}
	best := sites[0]
	if best.Open != 0 {
		t.Fatalf("best site overlaps open pocket: %+v", best)
	}
	if best.Dist > 20 {
		t.Fatalf("best site ignores anchor: %+v", best)
	}
}
```

IMPORTANT PRE-STEP: the exact flag that makes `BuildFromTiles` mark a tile OPEN is topology-internal — read `internal/topology/overlay.go`'s `BuildFromTiles` (and `tiletype.go`) first and set `tt`/`flags` in the test so the pocket is StateOpen and the rest StateClosed. The test's assertions are the contract; the fixture must speak the overlay's real input language. Run: `go test ./internal/mapview/ -run TestFindDigSites -v` — Expected: FAIL (`FindDigSites` undefined).

- [ ] **Step 2: Implement `finder.go`**

```go
package mapview

import (
	"fmt"
	"math"
	"sort"

	"github.com/df-ai/orchestrator/internal/topology"
)

type DigSiteRequest struct {
	W, H          int16
	Z             int16
	NearX, NearY  int16
	MaxCandidates int
}

type DigSite struct {
	X1, Y1, X2, Y2 int16
	Z              int16
	Dist           float64
	Solid          int
	Open           int
	Rationale      string
}

// FindDigSites scans a z-level for WxH rectangles of solid (closed or
// hidden) ground, ranked by distance from the anchor. Geometry lives
// here so the model never does coordinate arithmetic (FLE failure mode).
func FindDigSites(topo *topology.TopologyOverlay, req DigSiteRequest) []DigSite {
	if req.MaxCandidates <= 0 {
		req.MaxCandidates = 5
	}
	w, h, _ := topo.GetDimensions()
	var out []DigSite
	const stride = 2 // scan on a coarse grid; candidates don't need to be exhaustive
	for x := int16(1); x+req.W < int16(w)-1; x += stride {
		for y := int16(1); y+req.H < int16(h)-1; y += stride {
			solid, open := 0, 0
			for dx := int16(0); dx < req.W; dx++ {
				for dy := int16(0); dy < req.H; dy++ {
					switch topo.GetTileState(x+dx, y+dy, req.Z) {
					case topology.StateOpen:
						open++
					default: // closed or unknown/hidden — both are diggable mass
						solid++
					}
				}
			}
			if open > 0 {
				continue // room must be carved from fully solid ground
			}
			d := math.Hypot(float64(x+req.W/2-req.NearX), float64(y+req.H/2-req.NearY))
			out = append(out, DigSite{
				X1: x, Y1: y, X2: x + req.W - 1, Y2: y + req.H - 1, Z: req.Z,
				Dist: d, Solid: solid, Open: open,
				Rationale: fmt.Sprintf("%dx%d fully solid, %.0f tiles from anchor", req.W, req.H, d),
			})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dist < out[j].Dist })
	if len(out) > req.MaxCandidates {
		out = out[:req.MaxCandidates]
	}
	return out
}
```

Run: `go test ./internal/mapview/ -run TestFindDigSites -v` — Expected: PASS.

- [ ] **Step 3: The tool**

In `tools_percept.go`:

```go
	type findIn struct {
		Width  int `json:"width" jsonschema:"room width in tiles"`
		Height int `json:"height" jsonschema:"room height in tiles"`
		Z      int `json:"z" jsonschema:"z-level to search (use cross_section to pick a stone level)"`
		NearX  int `json:"near_x" jsonschema:"anchor x (e.g. your stair shaft)"`
		NearY  int `json:"near_y" jsonschema:"anchor y"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "find_dig_site",
		Description: "Search a z-level for fully-solid rectangles where a WxH room can be dug, ranked by distance from your anchor. Returns concrete coordinates — use these instead of guessing. Solid includes hidden fog tiles (they're undug rock: diggable).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in findIn) (*mcp.CallToolResult, any, error) {
		topo := b.Topo()
		if topo == nil {
			return withDash(b, ctx, "topology not built yet (waiting for full state)"), nil, nil
		}
		sites := mapview.FindDigSites(topo, mapview.DigSiteRequest{
			W: int16(in.Width), H: int16(in.Height), Z: int16(in.Z),
			NearX: int16(in.NearX), NearY: int16(in.NearY), MaxCandidates: 5,
		})
		if len(sites) == 0 {
			return withDash(b, ctx, "no fully-solid candidates on that z — try another level or smaller room"), nil, nil
		}
		var sb strings.Builder
		fmt.Fprintf(&sb, "%d candidates for a %dx%d room on z=%d:\n", len(sites), in.Width, in.Height, in.Z)
		for i, s := range sites {
			fmt.Fprintf(&sb, "%d. dig from (%d,%d,%d) to (%d,%d,%d) — %s\n",
				i+1, s.X1, s.Y1, s.Z, s.X2, s.Y2, s.Z, s.Rationale)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})
```

- [ ] **Step 4: Build + test + live**

`go build ./... && go test ./internal/mapview/ -v` → PASS. Live: ask for a 5×5 at surface-3 near the wagon → candidates come back in solid rock; `look` at the top candidate confirms `?`/`#` tiles.

- [ ] **Step 5: Commit**

```bash
git add internal/mapview/ internal/mcpserver/
git commit -m "feat(mcp): find_dig_site ranked-candidate geometry tool"
```

---

### Task 12: Action tools with truthful ACKs

**Files:**
- Create: `internal/mcpserver/tools_action.go`, `internal/mcpserver/tools_action_test.go`
- Modify: `internal/mcpserver/server.go` (register)

**Interfaces:**
- Consumes: `CommandExecutor` senders (exact signatures in `internal/commands/executor.go:53-208`), `protocol` type-byte constants: DigType (0x01-0x06), ZoneType (`protocol.h:77-91` — Go names in `message.go:336-343+`), OrderType (0x01-0x0C), BuildType (0x01-0x51), StockpileGroup mask (`message.go:307-326`).
- Produces: tools `designate_dig(type, x1,y1,z1, x2,y2,z2)`, `build(type, x, y, z)`, `zone(type, x1,y1,z, x2,y2)`, `stockpile(category, x1,y1,z, x2,y2)`, `order(item, count)`, `unsuspend(x,y,z)`, `cancel_designation(x1,y1,z1,x2,y2,z2)`, `smooth(mode, x1,y1,z, x2,y2)`. Shared helper `ackText(res *commands.CommandResult, err error, what string) string` (unit-tested) — SUCCESS/PARTIAL/FAILED with the plugin's error text verbatim, never swallowed.

- [ ] **Step 1: Failing test for ackText + name mappers**

`internal/mcpserver/tools_action_test.go`:

```go
package mcpserver

import (
	"errors"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/protocol"
)

func TestAckText(t *testing.T) {
	ok := ackText(&commands.CommandResult{Success: true, Status: 0}, nil, "dig 5x5")
	if !strings.Contains(ok, "SUCCESS") || !strings.Contains(ok, "dig 5x5") {
		t.Fatalf("success text wrong: %q", ok)
	}
	partial := ackText(&commands.CommandResult{Success: true, Status: protocol.AckStatusPartial, ErrorMsg: "3 of 25 tiles blocked at (10,11,90)"}, nil, "dig 5x5")
	if !strings.Contains(partial, "PARTIAL") || !strings.Contains(partial, "3 of 25") {
		t.Fatalf("partial must carry plugin text: %q", partial)
	}
	failed := ackText(nil, errors.New("timeout waiting for ACK"), "dig 5x5")
	if !strings.Contains(failed, "FAILED") || !strings.Contains(failed, "timeout") {
		t.Fatalf("failure text wrong: %q", failed)
	}
}

func TestDigTypeMapper(t *testing.T) {
	if v, err := digTypeFromName("stairs"); err != nil || v != protocol.DigTypeUpDownStair {
		t.Fatalf("stairs mapping wrong: %v %v", v, err)
	}
	if _, err := digTypeFromName("lasergun"); err == nil {
		t.Fatal("unknown dig type must error")
	}
}
```

(Verify `protocol.AckStatusPartial` exists: `rg -n "AckStatus" internal/protocol/message.go` — C++ side is `ACK_STATUS_PARTIAL=0x01`.)
Run: `go test ./internal/mcpserver/ -run "TestAckText|TestDigTypeMapper" -v` — Expected: FAIL.

- [ ] **Step 2: Implement `tools_action.go`**

```go
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/commands"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// ackText renders a command result truthfully: the plugin's error text is
// the model's primary feedback and is never swallowed or softened.
func ackText(res *commands.CommandResult, err error, what string) string {
	if err != nil {
		return fmt.Sprintf("FAILED: %s — %v", what, err)
	}
	if res == nil {
		return fmt.Sprintf("FAILED: %s — no result", what)
	}
	switch {
	case res.Success && res.ErrorMsg == "":
		return fmt.Sprintf("SUCCESS: %s (ack in %s)", what, res.Duration)
	case res.Success:
		return fmt.Sprintf("PARTIAL: %s — %s", what, res.ErrorMsg)
	default:
		return fmt.Sprintf("FAILED: %s — %s", what, res.ErrorMsg)
	}
}

func digTypeFromName(s string) (uint8, error) {
	switch strings.ToLower(s) {
	case "default", "dig", "mine":
		return protocol.DigTypeDefault, nil
	case "stairs":
		return protocol.DigTypeUpDownStair, nil
	case "channel":
		return protocol.DigTypeChannel, nil
	case "ramp":
		return protocol.DigTypeRamp, nil
	case "downstair":
		return protocol.DigTypeDownStair, nil
	case "upstair":
		return protocol.DigTypeUpStair, nil
	}
	return 0, fmt.Errorf("unknown dig type %q (default|stairs|channel|ramp|upstair|downstair)", s)
}

var zoneTypes = map[string]uint8{
	"bedroom": 0x01, "dining": 0x02, "meeting": 0x03, "barracks": 0x04,
	"dormitory": 0x05, "farm": 0x10, "pen": 0x11, "garbage": 0x12,
	"pit": 0x13, "water": 0x14, "fishing": 0x15, "hospital": 0x16,
	"animal_train": 0x17, "tomb": 0x18,
}

var buildTypes = map[string]uint8{
	"wall": 0x01, "floor": 0x02, "upstair": 0x03, "downstair": 0x04,
	"updownstair": 0x05, "ramp": 0x06,
	"carpenter": 0x10, "mason": 0x11, "still": 0x12, "farmer": 0x13,
	"craftsdwarf": 0x14, "mechanic": 0x15, "butcher": 0x16, "kitchen": 0x17,
	"fishery": 0x18,
	"bed": 0x30, "table": 0x31, "chair": 0x32, "cabinet": 0x33, "coffer": 0x34,
	"door": 0x50, "hatch": 0x51,
}

var orderTypes = map[string]uint8{
	"bed": 0x01, "table": 0x02, "chair": 0x03, "door": 0x04, "barrel": 0x05,
	"bucket": 0x06, "cabinet": 0x07, "coffer": 0x08, "drink": 0x09,
	"meal": 0x0A, "blocks": 0x0B, "crafts": 0x0C,
}

var stockpileGroups = map[string]uint32{
	"all":            protocol.StockpileGroupAll,
	"animals":        protocol.StockpileGroupAnimals,
	"food":           protocol.StockpileGroupFood,
	"furniture":      protocol.StockpileGroupFurniture,
	"corpses":        protocol.StockpileGroupCorpses,
	"refuse":         protocol.StockpileGroupRefuse,
	"stone":          protocol.StockpileGroupStone,
	"ammo":           protocol.StockpileGroupAmmo,
	"coins":          protocol.StockpileGroupCoins,
	"bars_blocks":    protocol.StockpileGroupBarsBlocks,
	"gems":           protocol.StockpileGroupGems,
	"finished_goods": protocol.StockpileGroupFinishedGoods,
	"leather":        protocol.StockpileGroupLeather,
	"cloth":          protocol.StockpileGroupCloth,
	"wood":           protocol.StockpileGroupWood,
	"weapons":        protocol.StockpileGroupWeapons,
	"armor":          protocol.StockpileGroupArmor,
	"sheet":          protocol.StockpileGroupSheet,
}

func registerActionTools(srv *mcp.Server, b *Bridge) {
	type digIn struct {
		Type string `json:"type" jsonschema:"default|stairs|channel|ramp|upstair|downstair"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z1   int    `json:"z1"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
		Z2   int    `json:"z2" jsonschema:"for stairs, the ending z (deep!); for flat digs same as z1"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "designate_dig",
		Description: "Designate digging over a 3D rectangle. Hidden fog tiles designate fine (that's how forts are dug). 'stairs' spans z1..z2 as one shaft (2x2 recommended, surface to deep stone in ONE call). Dwarves with picks do the work over game time — step() to let it happen.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in digIn) (*mcp.CallToolResult, any, error) {
		dt, err := digTypeFromName(in.Type)
		if err != nil {
			return withDash(b, ctx, err.Error()), nil, nil
		}
		res, err := b.Exec.SendDigRegion(dt, int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2), int16(in.Z2))
		what := fmt.Sprintf("dig %s (%d,%d,%d)->(%d,%d,%d)", in.Type, in.X1, in.Y1, in.Z1, in.X2, in.Y2, in.Z2)
		return withDash(b, ctx, ackText(res, err, what)), nil, nil
	})

	type buildIn struct {
		Type string `json:"type" jsonschema:"workshop (carpenter|mason|still|farmer|craftsdwarf|mechanic|butcher|kitchen|fishery), furniture (bed|table|chair|cabinet|coffer), door|hatch, or construction (wall|floor|upstair|downstair|updownstair|ramp)"`
		X    int    `json:"x" jsonschema:"for workshops this is the CENTER of the 3x3 footprint"`
		Y    int    `json:"y"`
		Z    int    `json:"z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "build",
		Description: "Place a building. Workshops are 3x3 (x,y = center; surrounding 8 tiles must be clear floor). Furniture needs the item in a stockpile first (order it). Constructions need blocks/boulders.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in buildIn) (*mcp.CallToolResult, any, error) {
		bt, ok := buildTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown build type %q", in.Type)), nil, nil
		}
		res, err := b.Exec.SendBuildCommand(int16(in.X), int16(in.Y), int16(in.Z), bt)
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("build %s at (%d,%d,%d)", in.Type, in.X, in.Y, in.Z))), nil, nil
	})

	type zoneIn struct {
		Type string `json:"type" jsonschema:"bedroom|dining|meeting|barracks|dormitory|farm|pen|garbage|pit|water|fishing|hospital|animal_train|tomb"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "zone",
		Description: "Designate an activity zone rectangle on one z-level (bedroom/dining/farm/...). Farms go on soil or muddied stone.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in zoneIn) (*mcp.CallToolResult, any, error) {
		zt, ok := zoneTypes[strings.ToLower(in.Type)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown zone type %q", in.Type)), nil, nil
		}
		res, err := b.Exec.SendZoneCommand(zt, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("zone %s (%d,%d)-(%d,%d) z=%d", in.Type, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	// stockpile / order / unsuspend / cancel_designation / smooth follow the
	// identical pattern — full bodies here in the plan for each:

	type stockIn struct {
		Category string `json:"category" jsonschema:"all|food|furniture|stone|wood|... (see stockpileGroups)"`
		X1       int    `json:"x1"`
		Y1       int    `json:"y1"`
		Z        int    `json:"z"`
		X2       int    `json:"x2"`
		Y2       int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "stockpile",
		Description: "Designate a stockpile rectangle accepting a category (or 'all'). Must be on open floor tiles.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stockIn) (*mcp.CallToolResult, any, error) {
		mask, ok := stockpileGroups[strings.ToLower(in.Category)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown stockpile category %q", in.Category)), nil, nil
		}
		res, err := b.Exec.SendStockpileCommand(int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2), mask)
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("stockpile %s (%d,%d)-(%d,%d) z=%d", in.Category, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})

	type orderIn struct {
		Item  string `json:"item" jsonschema:"bed|table|chair|door|barrel|bucket|cabinet|coffer|drink|meal|blocks|crafts"`
		Count int    `json:"count" jsonschema:"how many to queue (start small: 1-3)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "order",
		Description: "Queue a manager work order (needs a manager noble + office to dispatch; the matching workshop must exist). E.g. order bed x2 after building a carpenter workshop.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in orderIn) (*mcp.CallToolResult, any, error) {
		ot, ok := orderTypes[strings.ToLower(in.Item)]
		if !ok {
			return withDash(b, ctx, fmt.Sprintf("unknown order item %q", in.Item)), nil, nil
		}
		res, err := b.Exec.SendWorkOrderCommand(ot, uint16(in.Count))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("order %dx %s", in.Count, in.Item))), nil, nil
	})

	type xyzIn struct {
		X int `json:"x"`
		Y int `json:"y"`
		Z int `json:"z"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unsuspend",
		Description: "Resume a suspended construction at a tile (after fixing its blocker: materials, path, occupant).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in xyzIn) (*mcp.CallToolResult, any, error) {
		res, err := b.Exec.SendUnsuspendCommand(int16(in.X), int16(in.Y), int16(in.Z))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("unsuspend (%d,%d,%d)", in.X, in.Y, in.Z))), nil, nil
	})

	type cancelIn struct {
		X1 int `json:"x1"`
		Y1 int `json:"y1"`
		Z1 int `json:"z1"`
		X2 int `json:"x2"`
		Y2 int `json:"y2"`
		Z2 int `json:"z2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "cancel_designation",
		Description: "Clear dig designations in a 3D rectangle (undo a mistaken dig order).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in cancelIn) (*mcp.CallToolResult, any, error) {
		res, err := b.Exec.SendCancelCommand(int16(in.X1), int16(in.Y1), int16(in.Z1), int16(in.X2), int16(in.Y2))
		// NOTE: SendCancelCommand's signature is (x1,y1,z,x2,y2) single-Z per
		// executor.go:91 — if so, expose z only and drop z2 from cancelIn;
		// check and match the real signature.
		return withDash(b, ctx, ackText(res, err, "cancel designations")), nil, nil
	})

	type smoothIn struct {
		Mode string `json:"mode" jsonschema:"smooth|engrave (engrave requires smoothed first; natural stone only)"`
		X1   int    `json:"x1"`
		Y1   int    `json:"y1"`
		Z    int    `json:"z"`
		X2   int    `json:"x2"`
		Y2   int    `json:"y2"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "smooth",
		Description: "Smooth or engrave natural stone in a rectangle (soil is silently skipped by DF).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in smoothIn) (*mcp.CallToolResult, any, error) {
		st := uint8(protocol.SmoothTypeSmooth)
		if strings.ToLower(in.Mode) == "engrave" {
			st = protocol.SmoothTypeEngrave
		}
		res, err := b.Exec.SendSmoothCommand(st, int16(in.X1), int16(in.Y1), int16(in.Z), int16(in.X2), int16(in.Y2))
		return withDash(b, ctx, ackText(res, err, fmt.Sprintf("%s (%d,%d)-(%d,%d) z=%d", in.Mode, in.X1, in.Y1, in.X2, in.Y2, in.Z))), nil, nil
	})
}
```

Fill `stockpileGroups` with every real constant from `message.go:307-326` (names like `protocol.StockpileGroupFood` — use exactly what's there). Fix `cancel_designation` to the executor's real signature. Register `registerActionTools(srv, bridge)` in `server.go`.

- [ ] **Step 3: Test + build**

`go test ./internal/mcpserver/ -v` → ackText/mapper tests PASS; `go build ./...` clean.

- [ ] **Step 4: Live smoke**

In a Claude Code session: `designate_dig(stairs, wagon-adjacent, z surface → surface-6)` → SUCCESS + designations visible; `step(1200)` isn't available until Task 14 — unpause manually in DF and watch dwarves dig; `build(carpenter, ...)` on cleared ground → SUCCESS; `order(bed, 1)` → SUCCESS (or truthful FAILED naming what's missing).

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/
git commit -m "feat(mcp): action tools (dig/build/zone/stockpile/order/unsuspend/cancel/smooth) with truthful ACKs"
```

---

### Task 13: `apply_blueprint` (library + inline dig blueprints, dry-run)

**Files:**
- Create: `internal/mcpserver/blueprint.go`, `internal/mcpserver/blueprint_test.go`
- Modify: `internal/mcpserver/tools_action.go` (register the tool), `internal/mcpserver/bridge.go` (add `Blueprints *blueprints.BlueprintLibrary` field, constructed in `NewBridge` with `blueprints.NewBlueprintLibrary("blueprints")`)

**Interfaces:**
- Consumes: `blueprints.BlueprintLibrary.{ListBlueprints,GetBlueprint}`, `DigBlueprint.ApplyAt(origin modifications.Coordinate) []DigCommand` (`internal/blueprints/types.go:68-87`), `DigCommand{X,Y,Z int16; DigType string}`, `digTypeFromName` (Task 12), `topology.GetTileState` for dry-run, `Exec.SendDigRegion` for apply.
- Produces: tool `apply_blueprint(name, origin_x, origin_y, origin_z, dry_run)`; helper `expandBlueprint(lib, name, origin) ([]blueprints.DigCommand, error)`; `summarizeDryRun(topo, cmds) string` reporting per-digtype counts and how many target tiles are open (warning) vs solid (good). Inline-CSV authoring is Phase 2 — this tool exposes the 6 shipped library blueprints plus any future files in `blueprints/`.

- [ ] **Step 1: Failing test**

`internal/mcpserver/blueprint_test.go`:

```go
package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
)

func TestExpandBlueprint(t *testing.T) {
	lib := blueprints.NewBlueprintLibrary(t.TempDir()) // empty dir → empty lib
	bp := &blueprints.DigBlueprint{
		Name: "tiny", Digs: []blueprints.DigEntry{
			{X: 0, Y: 0, Z: 0, DigType: "default"},
			{X: 1, Y: 0, Z: 0, DigType: "default"},
		},
	}
	lib.AddBlueprint("tiny", bp)
	cmds, err := expandBlueprint(lib, "tiny", modifications.Coordinate{X: 50, Y: 60, Z: 100})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(cmds) != 2 || cmds[1].X != 51 || cmds[0].Y != 60 || cmds[0].Z != 100 {
		t.Fatalf("expansion wrong: %+v", cmds)
	}
	if _, err := expandBlueprint(lib, "nope", modifications.Coordinate{}); err == nil {
		t.Fatal("missing blueprint must error")
	}
	_ = strings.TrimSpace
}
```

(Verify `modifications.Coordinate` field names: `rg -n "type Coordinate" internal/modifications/` — adjust if `X,Y,Z` differ.)
Run: `go test ./internal/mcpserver/ -run TestExpandBlueprint -v` — Expected: FAIL.

- [ ] **Step 2: Implement `blueprint.go`**

```go
package mcpserver

import (
	"context"
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/topology"
)

func expandBlueprint(lib *blueprints.BlueprintLibrary, name string, origin modifications.Coordinate) ([]blueprints.DigCommand, error) {
	bp := lib.GetBlueprint(name)
	if bp == nil {
		return nil, fmt.Errorf("blueprint %q not found (available: %s)", name, strings.Join(lib.ListBlueprints(), ", "))
	}
	return bp.ApplyAt(origin), nil
}

// summarizeDryRun previews what a blueprint would designate: counts per
// dig type and a solid-vs-open breakdown (rooms should carve solid rock;
// open targets usually mean a mis-placed origin).
func summarizeDryRun(topo *topology.TopologyOverlay, cmds []blueprints.DigCommand) string {
	byType := map[string]int{}
	open, solid := 0, 0
	for _, c := range cmds {
		byType[c.DigType]++
		if topo != nil && topo.GetTileState(c.X, c.Y, c.Z) == topology.StateOpen {
			open++
		} else {
			solid++
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "DRY RUN: %d tiles would be designated (", len(cmds))
	first := true
	for k, v := range byType {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		fmt.Fprintf(&sb, "%s=%d", k, v)
	}
	fmt.Fprintf(&sb, "); %d into solid ground (good), %d onto already-open tiles (check origin!)", solid, open)
	return sb.String()
}

// applyBlueprintCmds sends the expanded digs. Groups per (digType, z) into
// single-tile SendDigRegion calls; fine for library-sized blueprints.
func applyBlueprintCmds(ctx context.Context, b *Bridge, cmds []blueprints.DigCommand) (int, int, string) {
	okCount, failCount := 0, 0
	var firstErr string
	for _, c := range cmds {
		dt, err := digTypeFromName(c.DigType)
		if err != nil {
			failCount++
			if firstErr == "" {
				firstErr = err.Error()
			}
			continue
		}
		res, err := b.Exec.SendDigRegion(dt, c.X, c.Y, c.Z, c.X, c.Y, c.Z)
		if err != nil || res == nil || !res.Success {
			failCount++
			if firstErr == "" {
				firstErr = ackText(res, err, fmt.Sprintf("tile (%d,%d,%d)", c.X, c.Y, c.Z))
			}
			continue
		}
		okCount++
	}
	return okCount, failCount, firstErr
}
```

Run the test again → PASS. Note: per-tile sends are O(n) round-trips (a 128-room blueprint = thousands) — acceptable v0; the tool description warns to prefer small blueprints, and batching into rectangles is listed as a Phase 2 improvement.

- [ ] **Step 3: The tool (in `tools_action.go`)**

```go
	type bpIn struct {
		Name    string `json:"name" jsonschema:"blueprint name from the list shown on error/empty name"`
		OriginX int    `json:"origin_x" jsonschema:"absolute x where the blueprint's (0,0,0) lands"`
		OriginY int    `json:"origin_y"`
		OriginZ int    `json:"origin_z"`
		DryRun  bool   `json:"dry_run,omitempty" jsonschema:"true = preview only (ALWAYS dry-run first)"`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "apply_blueprint",
		Description: "Apply a dig blueprint from the library at an origin. ALWAYS dry_run=true first: it reports how many tiles would carve solid ground vs hit open space. Empty name lists available blueprints.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in bpIn) (*mcp.CallToolResult, any, error) {
		if in.Name == "" {
			return withDash(b, ctx, "available blueprints: "+strings.Join(b.Blueprints.ListBlueprints(), ", ")), nil, nil
		}
		origin := modifications.Coordinate{X: int16(in.OriginX), Y: int16(in.OriginY), Z: int16(in.OriginZ)}
		cmds, err := expandBlueprint(b.Blueprints, in.Name, origin)
		if err != nil {
			return withDash(b, ctx, err.Error()), nil, nil
		}
		if in.DryRun {
			return withDash(b, ctx, summarizeDryRun(b.Topo(), cmds)), nil, nil
		}
		ok, fail, firstErr := applyBlueprintCmds(ctx, b, cmds)
		body := fmt.Sprintf("applied %q: %d tiles designated, %d failed", in.Name, ok, fail)
		if firstErr != "" {
			body += " — first failure: " + firstErr
		}
		return withDash(b, ctx, body), nil, nil
	})
```

(imports: add `modifications` to tools_action.go.)

- [ ] **Step 4: Test + build + live**

`go test ./internal/mcpserver/ -v && go build ./...` → PASS/clean. Live: `apply_blueprint("")` lists the 6 bedroom blueprints; dry-run one 3 z below the surface → high solid count; apply → designations appear in DF.

- [ ] **Step 5: Commit**

```bash
git add internal/mcpserver/
git commit -m "feat(mcp): apply_blueprint with mandatory-habit dry-run preview"
```

---

### Task 14: Control tools — `pause`, `unpause`, `step`, `check_goals`

**Files:**
- Create: `internal/mcpserver/tools_control.go`, `internal/mcpserver/tools_control_test.go`
- Modify: `internal/mcpserver/server.go` (register)

**Interfaces:**
- Consumes: `Exec.SendPauseCommand/SendStepCommand` (Task 5), `Bridge.Query("sim_status")`, `Bridge.Snapshot` (alert delta), `predicate.Library.CheckAll(wm) []predicate.Result` + `Result{Name, Horizon, Satisfied, Confidence, Evidence}` (fields as consumed by `internal/bdi/render.go:96-122`).
- Produces: tools `pause()`, `unpause()`, `step(ticks)` — step blocks until the plugin re-pauses (poll `sim_status` every 500ms, timeout `max(30s, ticks/10 seconds)`), then reports WHAT CHANGED: new alerts since the step began + dwarf count delta; `check_goals()` rendering the predicate table. Helper `renderGoals([]predicate.Result) string` (unit-tested).

- [ ] **Step 1: Failing test for `renderGoals`**

`internal/mcpserver/tools_control_test.go`:

```go
package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/predicate"
)

func TestRenderGoals(t *testing.T) {
	results := []predicate.Result{
		{Name: "has_shelter", Horizon: predicate.HorizonNow, Satisfied: false, Confidence: 0.9,
			Evidence: []string{"0 dug tiles"}},
		{Name: "has_min_dwarves", Horizon: predicate.HorizonNow, Satisfied: true, Confidence: 1.0},
	}
	out := renderGoals(results)
	if !strings.Contains(out, "[ ] has_shelter") || !strings.Contains(out, "[x] has_min_dwarves") {
		t.Fatalf("goal markers wrong:\n%s", out)
	}
	if !strings.Contains(out, "0 dug tiles") {
		t.Fatalf("evidence missing:\n%s", out)
	}
}
```

(If `predicate.Result` literal fields differ, check `internal/predicate/types.go` — these are the fields `bdi/render.go` reads.)
Run: `go test ./internal/mcpserver/ -run TestRenderGoals -v` — Expected: FAIL.

- [ ] **Step 2: Implement `tools_control.go`**

```go
package mcpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/df-ai/orchestrator/internal/predicate"
)

func renderGoals(results []predicate.Result) string {
	var sb strings.Builder
	sb.WriteString("GOALS (predicates checked against live state):\n")
	for _, h := range []predicate.Horizon{predicate.HorizonNow, predicate.HorizonSoon, predicate.HorizonEventual} {
		filtered := predicate.FilterByHorizon(results, h)
		if len(filtered) == 0 {
			continue
		}
		fmt.Fprintf(&sb, "%s:\n", strings.ToUpper(h.String()))
		for _, r := range filtered {
			mark := "[ ]"
			if r.Satisfied {
				mark = "[x]"
			}
			fmt.Fprintf(&sb, "  %s %s (confidence %.2f)\n", mark, r.Name, r.Confidence)
			for _, ev := range r.Evidence {
				fmt.Fprintf(&sb, "      - %s\n", ev)
			}
		}
	}
	return sb.String()
}

func simPaused(raw []byte) (paused, stepping bool) {
	var s struct {
		Paused   bool `json:"paused"`
		Stepping bool `json:"stepping"`
	}
	if json.Unmarshal(raw, &s) == nil {
		return s.Paused, s.Stepping
	}
	return false, false
}

func registerControlTools(srv *mcp.Server, b *Bridge) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "pause",
		Description: "Pause the DF simulation. Think while paused; nothing moves.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		res, err := b.Exec.SendPauseCommand(true)
		return withDash(b, ctx, ackText(res, err, "pause")), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "unpause",
		Description: "Unpause DF and let it run free (prefer step for turn-based play).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		res, err := b.Exec.SendPauseCommand(false)
		return withDash(b, ctx, ackText(res, err, "unpause")), nil, nil
	})

	type stepIn struct {
		Ticks int `json:"ticks" jsonschema:"game ticks to run before auto-pausing. 1200 = one fortress day. 600 is a good default turn."`
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "step",
		Description: "Run the simulation for N ticks then auto-pause, and report what happened (new alerts, arrivals). This is your end-of-turn: act, then step, then observe.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in stepIn) (*mcp.CallToolResult, any, error) {
		if in.Ticks <= 0 {
			in.Ticks = 600
		}
		if in.Ticks > 14400 {
			in.Ticks = 14400 // cap: ~12 game days per step
		}
		before := b.Snapshot()
		beforeAlerts := map[uint32]bool{}
		for _, a := range before.ActiveAlerts {
			beforeAlerts[a.ID] = true
		}
		res, err := b.Exec.SendStepCommand(uint32(in.Ticks))
		if err != nil || res == nil || !res.Success {
			return withDash(b, ctx, ackText(res, err, "step")), nil, nil
		}
		// Poll until the plugin re-pauses.
		timeout := time.Duration(in.Ticks/10+30) * time.Second
		deadline := time.Now().Add(timeout)
		for time.Now().Before(deadline) {
			time.Sleep(500 * time.Millisecond)
			raw, qerr := b.Query(ctx, "sim_status", "{}")
			if qerr != nil {
				continue
			}
			if paused, stepping := simPaused(raw); paused && !stepping {
				break
			}
		}
		after := b.Snapshot()
		var sb strings.Builder
		fmt.Fprintf(&sb, "stepped %d ticks (tick %d -> %d)\n", in.Ticks, before.Tick, after.Tick)
		if d := len(after.Entities.Dwarves) - len(before.Entities.Dwarves); d != 0 {
			fmt.Fprintf(&sb, "dwarf count change: %+d\n", d)
		}
		newAlerts := 0
		for _, a := range after.ActiveAlerts {
			if !beforeAlerts[a.ID] {
				newAlerts++
				if newAlerts <= 15 {
					fmt.Fprintf(&sb, "NEW ALERT [%d] %s\n", a.ID, a.Text)
				}
			}
		}
		if newAlerts == 0 {
			sb.WriteString("no new alerts\n")
		} else if newAlerts > 15 {
			fmt.Fprintf(&sb, "... and %d more new alerts (use alerts tool)\n", newAlerts-15)
		}
		return withDash(b, ctx, sb.String()), nil, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "check_goals",
		Description: "Evaluate the survival/headroom/trajectory predicates against live state, with evidence. Your verification critic — call after milestones and before claiming success.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in struct{}) (*mcp.CallToolResult, any, error) {
		return withDash(b, ctx, renderGoals(b.Preds.CheckAll(b.WM))), nil, nil
	})
}
```

(Verify `predicate.Library.CheckAll(wm *worldmodel.WorldModel)` signature per `internal/bdi/loop.go:251` usage `l.library.CheckAll(l.wm)` — matches.)
Register `registerControlTools(srv, bridge)` in `server.go`.

- [ ] **Step 3: Test + build + live**

`go test ./internal/mcpserver/ -v && go build ./...` → PASS. Live: `pause` → DF pauses; `step(600)` → DF runs ~30-60 wall-seconds, auto-pauses, tool returns with tick delta and any new alerts; `check_goals` shows `has_shelter` unsatisfied on a fresh embark.

- [ ] **Step 4: Commit**

```bash
git add internal/mcpserver/
git commit -m "feat(mcp): pause/step turn protocol + check_goals verification tool"
```

---

### Task 15: Player workspace + First Fort session (Phase 1 exit)

**Files:**
- Create: `fortress/CLAUDE.md`, `fortress/.mcp.json`, `fortress/memory/journal.md`, `fortress/memory/goals.md`, `fortress/memory/learnings.md`, `fortress/FIRST-FORT-RUNBOOK.md`

**Interfaces:**
- Consumes: every tool from Tasks 7-14.
- Produces: the workspace a Claude Code session opens to play; the First Fort acceptance checklist.

- [ ] **Step 1: `fortress/.mcp.json`**

```json
{
  "mcpServers": {
    "df-fortress": {
      "type": "stdio",
      "command": "go",
      "args": ["run", "../cmd/df-mcp"],
      "env": {}
    }
  }
}
```

(`go run` resolves the module from the parent dir; if it doesn't, build `bin/df-mcp.exe` and point command at it — note both options in the file as a JSON-adjacent README comment is invalid, so put the note in the runbook instead.)

- [ ] **Step 2: `fortress/CLAUDE.md` — the player charter**

```markdown
# You are playing Dwarf Fortress

You are the overseer of a dwarf fortress, playing through MCP tools. The
game is REAL and persistent — every action affects a live simulation.

## The turn protocol (always)

1. Confirm the sim is PAUSED (dashboard header on every tool result).
2. OBSERVE: read alerts first, then look/cross_section at whatever you're
   working on. Trust tool output over your memory — the ground-truth
   header is authoritative.
3. DECIDE briefly: what does the current goal need next?
4. ACT: issue designations/builds/orders. Read every ACK — PARTIAL and
   FAILED tell you exactly what to fix. Never repeat a failed command
   unchanged.
5. step(600-1200) to let the dwarves work. Read what changed.
6. Update memory/journal.md (2-4 lines), memory/goals.md when a goal
   completes or a new threat reorders priorities.

## Map facts that override intuition

- '?' tiles are HIDDEN: solid undug ground. Designating digs into them is
  normal and correct — that's how forts are dug. Dwarves reveal as they go.
- The z-axis runs UP. The surface is where your dwarves start. Dig DOWN
  (stairs z_start=surface, z_end=surface-10 or deeper) into soil, then stone.
- Stair shafts: 2x2, one designate_dig call spanning many z. Rooms branch
  BESIDE the shaft on each level, never on top of it.
- Use find_dig_site instead of guessing coordinates. Use cross_section
  before choosing depths.

## Memory discipline

- journal.md: append-only narrative of what happened (one dated block per
  session; newest at top).
- goals.md: the three-tier hierarchy (NOW survival / SOON headroom /
  EVENTUAL trajectory). Keep under 30 lines; rewrite freely.
- learnings.md: distilled cause-effect lessons ("wagon blocks its own
  tile for digs — deconstruct or dig around"). NEVER delete entries;
  append and refine. These survive across forts.

## Safety rails

- Prefer step() over unpause() — free-running games drift away from you.
- One structural project at a time until food/drink/beds are secure.
- check_goals before declaring any milestone done.
```

- [ ] **Step 3: Memory seeds**

`fortress/memory/journal.md`: `# Fort Journal\n\n(newest entries on top)\n`
`fortress/memory/goals.md`:

```markdown
# Goals

## NOW (survival)
- [ ] Underground shelter: stair shaft + first rooms dug
- [ ] Food/drink secured before embark supplies run out (~2 seasons)

## SOON (headroom)
- [ ] Beds built and bedrooms zoned for all dwarves
- [ ] Dining area with tables/chairs

## EVENTUAL (trajectory)
- [ ] Survive year 1 (zero starvation/dehydration deaths)
- [ ] Defensible single entrance
```

`fortress/memory/learnings.md`: `# Learnings\n\n(append-only; never delete)\n`

- [ ] **Step 4: `fortress/FIRST-FORT-RUNBOOK.md`**

```markdown
# First Fort — supervised session runbook

Setup: DF running with a FRESH embark (default 7 dwarves), plugin loaded,
`ai-connect` done. Open Claude Code in fortress/ (the .mcp.json here wires
df-fortress; if `go run ../cmd/df-mcp` fails, `go build -o ../bin/df-mcp.exe
../cmd/df-mcp` and point .mcp.json's command at the exe).

Kickoff prompt: "You're taking over this fresh embark. Survey the site,
establish an underground fort, and work toward the goals in
memory/goals.md. Play turn by turn per CLAUDE.md."

## Exit criteria (Phase 1 complete when ALL hold)
- [ ] Model called survey_site / cross_section before its first dig
- [ ] A 2x2 stair shaft descends from the surface into stone (one command)
- [ ] Rooms are dug UNDERGROUND into previously-hidden tiles (the F1 fix
      exercised end-to-end via MCP)
- [ ] Carpenter workshop built; beds ordered, produced, and placed in a
      zoned bedroom (wood chain: chop -> carpenter -> order -> build)
- [ ] Food/drink: farm plots zoned or gathering + still running; no
      starvation/dehydration deaths through the first migrant wave and
      into year 2 spring
- [ ] check_goals reports NOW + SOON satisfied; journal/goals/learnings
      files show real maintenance
- [ ] Human observations filed as issues: every moment the model was
      confused-by-presentation (not by DF) is a perception-tool bug

Budget note: a supervised session fits comfortably in a Max 20x 5-hour
window at ~600-1200 tick steps; pause the session (game pauses too) when
the window runs out.
```

- [ ] **Step 5: Run the session (the real test)**

Execute the runbook with a human watching. File every perception failure as a follow-up issue. Iterate tools inline where small (description tweaks, cap adjustments) — structural changes get their own tasks.

- [ ] **Step 6: Commit**

```bash
git add fortress/
git commit -m "feat: player workspace (charter, memory seeds, First Fort runbook)"
```

---

### Task 16 (GATED — only after First Fort exit criteria pass): retire the bespoke loop

**Files:**
- Delete: `cmd/df-orchestrator/`, `cmd/blueprint-analyzer/`, `internal/autonomous/`, `internal/agents/`, `internal/context/`, `internal/spatial/`, `internal/zones/`, `internal/phases/`, `internal/planning/`, `internal/http/`, `internal/llm/`, `internal/bdi/`, `internal/plan/`, `internal/reconcile/`, `internal/query/` (Tier-2 names now called directly by mcpserver), `internal/skill/` (content already mined for Phase 2 SKILL.md conversion — keep the file contents referenced in a doc first), `cmd/df-bdi/`
- Modify: `config/orchestrator.yaml` (strip legacy keys; keep listen_port + log settings), `internal/config/config.go` (only if dead keys break loading — prefer leaving parser fields inert)

**Interfaces:**
- Produces: a repo whose only binaries are `df-mcp` and `df-smoke`.

- [ ] **Step 1: Confirm the gate** — First Fort checklist all green, user says go.
- [ ] **Step 2: Sweep** — `git rm -r` the paths above; `go build ./...`; fix any lingering import (expected: `internal/worldmodel` imports `plan` via the PlanReader interface — check `rg -n "internal/plan" internal/worldmodel/` first; if so, either keep `internal/plan` (types only) or inline a minimal PlanReader stub into worldmodel — decide by what compiles smaller).
- [ ] **Step 3: Full test pass** — `go test ./...` green.
- [ ] **Step 4: Commit**

```bash
git commit -m "chore: retire legacy orchestrator + bespoke LLM loop (superseded by MCP architecture)"
```

---

## Deferred (explicitly NOT in this plan)

- Phase 2: SKILL.md library conversion, memory-discipline automation, skill-writing-after-success, region graph + named places, homeostasis executors, inline-CSV blueprint authoring, blueprint rectangle batching.
- Phase 3: Agent SDK/API unattended runs, MCP channel push (step() returns event deltas — sufficient for turn-based play), OBS/Twitch harness, `game_speed` tool.
- Spec §4.2 items consciously trimmed from v0: `region_graph` (needs dug-space analysis; Phase 2), `find_site` vein-awareness.
```
