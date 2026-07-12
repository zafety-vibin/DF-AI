# MCP server (df-mcp)

WHAT: `cmd/df-mcp` is a stdio MCP server exposing the running fort as tools. A Claude session is the player; this binary is its senses and hands. It wraps the Go substrate (dfhack client, world model, overlays, command executor) behind a `Bridge` (`internal/mcpserver/bridge.go`).

WHY stdio + one bridge: the playing session launches it locally (`.mcp.json` at repo root and in `fortress/`); one Bridge per server owns the single game connection. **stdout is the protocol channel** — anything logging in these paths must use `logging.NewStderrTextLogger`; a single stdout log line corrupts the stream.

Run it with the repo root as cwd (the config path `config/orchestrator.yaml` is relative). For the current tool list, read `internal/mcpserver/tools_*.go` or call `tools/list` — not this doc.

## Tool families and their contracts

- **State** (`tools_state.go`): status, alerts, dwarves, and plugin-query proxies (dwarf detail, stocks, jobs, orders). Alerts are DF's own announcement stream — the primary "why isn't this working" signal.
- **Perception** (`tools_percept.go`): labeled glyph crops (`look`), column bores (`cross_section`), embark orientation (`survey_site`), and computed dig candidates (`find_dig_site`). Geometry is computed in Go; the model chooses among candidates. See `world-model.md`.
- **Action** (`tools_action.go`): designations, buildings, zones, stockpiles, orders, blueprints. **Truthful ACKs**: the plugin's error text reaches the model verbatim as SUCCESS/PARTIAL/FAILED — never soften it. Workshop coordinates are model-facing CENTER, converted once to the wire's NW corner (`buildWireCoords`).
- **Control** (`tools_control.go`): pause/unpause/step and `check_goals` (the predicate library as a verification critic).

Cross-cutting contracts:
- Every tool response is prefixed with the one-line ground-truth dashboard (`withDash`) — tick, date, dwarf/enemy counts, pause state — so the model never acts on remembered state.
- Responses stay well under the MCP output cap: crops are radius-limited, lists are capped.
- Tools degrade truthfully when disconnected ("plugin not connected", "connection lost mid-step") rather than erroring opaquely.

## The turn protocol

Play is turn-based: pause → observe → decide → act → `step N` → read what changed.

- `step` counts **simulation ticks** (1200 = one fortress day) and auto-repauses at the target; the tool polls `sim_status` until completion and reports new alerts and population changes — the "what happened while you weren't watching" payload.
- The plugin answers queries and commands **while paused** (socket-thread drain) — pausing never deafens the system.
- Patience is policy: dwarves path, drink, and take breaks by design. Work completes over game-days. Judge progress by alerts and step deltas; never re-issue a command that simply has not finished. A stable fort earns longer steps, not more polling.
- Planned (see decisions log): critical announcements end a step early (tripwire), making multi-day steps safe.

## Adding a tool

Register in the matching `tools_*.go` via `mcp.AddTool` with a typed input struct (`json` + `jsonschema` tags); return `withDash(b, ctx, text)`. Nil-bridge and disconnected paths must return readable text, not panics — the registration tests drive every tool through an in-memory MCP session against a nil bridge. Ship a unit test for any pure helper.
