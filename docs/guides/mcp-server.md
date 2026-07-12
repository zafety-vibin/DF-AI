# MCP server (df-mcp)

WHAT: `cmd/df-mcp` is a stdio MCP server exposing the running fort as tools. A Claude session is the player; this binary is its senses and hands. It wraps the Go substrate (dfhack client, world model, overlays, command executor) behind a `Bridge` (`internal/mcpserver/bridge.go`).

WHY stdio + one bridge: the playing session launches it locally (`.mcp.json` at repo root and in `fortress/`); one Bridge per server owns the single game connection. **stdout is the protocol channel** — anything logging in these paths must use `logging.NewStderrTextLogger`; a single stdout log line corrupts the stream.

Run it with the repo root as cwd (the config path `config/orchestrator.yaml` is relative). For the current tool list, read `internal/mcpserver/tools_*.go` or call `tools/list` — not this doc.

## Tool families and their contracts

- **State** (`tools_state.go`): status, alerts, dwarves, and plugin-query proxies (dwarf detail, stocks, jobs, orders, a placed-buildings listing). Stocks carry material names and an economic-stone flag alongside item type and count. Alerts are DF's own announcement stream — the primary "why isn't this working" signal.
- **Perception** (`tools_percept.go`): labeled glyph crops (`look`), column bores (`cross_section`), embark orientation (`survey_site`), and computed dig candidates (`find_dig_site`). Geometry is computed in Go; the model chooses among candidates. See `world-model.md`.
- **Action** (`tools_action.go`): designations (digging, tree-chopping, plant-gathering), buildings, zones, stockpiles, orders, blueprints. **Truthful ACKs**: the plugin's error text reaches the model verbatim as SUCCESS/PARTIAL/FAILED — never soften it. Workshop coordinates are model-facing CENTER, converted once to the wire's NW corner (`buildWireCoords`).
- **Control** (`tools_control.go`): pause/unpause/step and `check_goals` (the predicate library as a verification critic).

Cross-cutting contracts:
- Every tool response is prefixed with the one-line ground-truth dashboard (`withDash`) — event counter (`events=`, perception messages ingested, NOT sim time), date (year/season/day-of-year, derived from the plugin's own FortInfo calendar fields — never a Go-side guess), dwarf/enemy counts, pause state, and a data-age stamp (`data 3s old` / `data STALE (142s)`) — so the model never acts on remembered or frozen state.
- Responses stay well under the MCP output cap: crops are radius-limited, lists are capped (dwarves at 50; raw stocks/orders payloads truncated at ~8 KB with a "refine your query" notice).
- Tools degrade truthfully when disconnected ("plugin not connected", "connection lost mid-step") rather than erroring opaquely.

## The turn protocol

Play is turn-based: pause → observe → decide → act → `step N` → read what changed.

- `step` counts **simulation ticks** (1200 = one fortress day) and auto-repauses at the target; the tool polls `sim_status` until completion, waits for the plugin's completion state push, and reports sim-frame progress, new alerts, and population changes — the "what happened while you weren't watching" payload. If the poll deadline expires it says "step DID NOT complete" instead of pretending success.
- The plugin answers queries and commands **while paused** (socket-thread drain) — pausing never deafens the system.
- Patience is policy: dwarves path, drink, and take breaks by design. Work completes over game-days. Judge progress by alerts and step deltas; never re-issue a command that simply has not finished. A stable fort earns longer steps, not more polling.
- A critical-severity announcement mid-step trips an early auto-repause (plugin-side); `sim_status` carries the tripwire reason and the step report surfaces it — multi-day steps stay safe without frame-polling.

## Adding a tool

Register in the matching `tools_*.go` via `mcp.AddTool` with a typed input struct (`json` + `jsonschema` tags); return `withDash(b, ctx, text)`. Nil-bridge and disconnected paths must return readable text, not panics — the registration sweep (`TestEveryToolNilBridge`) lists every tool and calls each against a nil bridge, so a tool with required inputs must add its minimal args to `minToolArgs` in `server_test.go`. Ship a unit test for any pure helper.
