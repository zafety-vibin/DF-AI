# DF-AI — Claude plays Dwarf Fortress

**What this is:** an MCP server + DFHack plugin that lets a Claude session play Steam Dwarf Fortress turn-based — perceive the map, dig, build, and react to the simulation. North star: a long-running stream where the AI visibly reasons, learns across forts, and survives DF's chaos.

## Start every session

1. If the task touches a subsystem you have not seen this session, read `docs/index.md` first — it is the doc map and states the stateless-docs policy.
2. Docs are for WHAT and WHY. For current state (versions, tool lists, test counts), run the code — never trust a doc for "how many X we have now."
3. When a real decision gets made, append one dated line to `docs/decisions.md`.

## Workflow

- Multi-step feature work: brainstorming → writing-plans → subagent-driven-development (waves with implement→review→fix gates). Execution ledger lives at `.superpowers/sdd/progress.md` (gitignored) — check it before re-dispatching anything after a context break.
- Speckit numbering owns `specs/` — Feature 008 (`specs/008-mcp-server/`) holds the active design + implementation plan.
- Bug hunts: systematic-debugging; root-cause against the DFHack source checkout (it's on disk) before touching plugin code.
- rtk (global PreToolUse hook) auto-rewrites Bash commands and compresses output. Trust condensed output; full logs are teed under rtk's data dir.

## Doc map

- `docs/guides/` — stateless subsystem references: plugin-build, mcp-server, world-model. Trust these; fix them if stale.
- `docs/decisions.md` — append-only decision log.
- `docs/archive/` — superseded docs, historical only.
- `specs/` — numbered feature specs; `008-mcp-server/design.md` is the architecture of record.
- `fortress/` — the AI player's workspace (charter, memory files, runbook). Gameplay state, not documentation — the playing session owns it.

## Tech stack

- **Go 1.25** MCP stdio server (`cmd/df-mcp`) on the official `modelcontextprotocol/go-sdk` — bridge + tools in `internal/mcpserver`, perception rendering in `internal/mapview`
- **C++20 DFHack plugin** (`dfhack-plugin/`) built IN-TREE inside a sibling DFHack source checkout via directory junction — there is no DFHack SDK; see `docs/guides/plugin-build.md`
- **Binary TCP protocol** between them (`internal/protocol` ↔ `dfhack-plugin/protocol.h` — these two MUST stay in sync)
- Test harness: `cmd/df-smoke` (one-shot or `-cmd repl`)
- Legacy pending deletion after First Fort (plan Task 16): `cmd/df-bdi`, `cmd/df-orchestrator`, `internal/{bdi,llm,autonomous,agents,context,spatial,zones,phases,planning,http,plan,reconcile,query,skill}`

## Commands

```bash
go build ./... && go test ./...                  # Go build + full suite
go run ./cmd/df-mcp                              # MCP server (MUST run with repo root as cwd; config path is relative)
go run ./cmd/df-smoke -cmd repl -port 5001       # manual plugin driver (port = config listen_port, NOT the 5000 default)
# Plugin (from the DFHack source checkout, e.g. ../dfhack-build):
cmake --build build --target df_ai_protocol --config Release
# Deploy: copy build/plugins/df_ai_protocol/Release/df_ai_protocol.plug.dll
#         into <Steam DF>/hack/plugins/ with DF closed. In-game: `ai-connect`.
```

## Key patterns (details in docs/guides/)

- **Turn-based play**: pause → observe → act → `step N ticks` (auto-repause) → read deltas. 1200 ticks = 1 game day. Patience is policy: dwarves complete work over game-days; judge progress by alerts/deltas, never re-issue unfinished commands. → `docs/guides/mcp-server.md`
- **Truthful ACKs**: plugin error text reaches the model verbatim (SUCCESS/PARTIAL/FAILED). Never soften or swallow it.
- **Perception = slices + assembly**: per-z glyph crops, column bores, computed candidates (`find_dig_site`). Spatial arithmetic lives in Go, never in the model. No volumetric/3D text renderings. → `docs/guides/world-model.md`
- **Every tool response** carries the one-line ground-truth dashboard (tick, season, dwarves, pause state) — models must never act on remembered state.

## Delegation

- Fan out Explore agents for unknown territory or >2-file impact analysis; read conclusions, not file dumps.
- Implementation waves: one implementer at a time per file tree; reviews gate each task. Fable for implementation/review, sonnet for transcription-grade tasks.
- Parallel agents only on disjoint file sets (Go vs C++ trees are safely disjoint).

## Gotchas (hard-won — keep these)

- **DFHack's plugin loader requires an EXACT version-string match** — every DFHack tag bump requires a plugin rebuild, no exceptions.
- **`plugins/df_ai_protocol` in the DFHack checkout is a junction** to this repo's `dfhack-plugin/`. If it ever becomes a real directory again, builds silently compile stale code (this burned us for two months).
- **`plugin_onupdate` does NOT run while DF is paused.** All queued work must stay reachable via the socket-thread drain (`drain_from_socket_thread`, CoreSuspender). Never park paused-relevant work solely in onupdate.
- **stdout is the MCP transport** in `cmd/df-mcp` paths — only `logging.NewStderrTextLogger` there; one stdout log line corrupts the protocol.
- **Hidden tiles are diggable by design** (DF fog-of-war). Never "validate" designations against hidden flags — that exact check was the project's founding bug.
- **DFHack `CHECK_*` macros THROW.** Any new DF-touching dispatch path needs the try/catch guard (see `executeCommand`) — an uncaught throw off a thread killed DF silently once already.
- **Building wire coords are NW corner**; the `build` tool converts from model-facing center (`buildWireCoords`). Don't "fix" either side independently.
- **Never pipe cmake/go verification output through `tail`/`head`** — the pipeline exit code masks failures (burned twice). Redirect to a log file and test the real exit code.
- `generate_headers` (Perl codegen) crashes flakily with exit -1073741819 — retry once before diagnosing.
- DLL deploys need DF closed (file lock); the artifact is `df_ai_protocol.plug.dll` (`.plug.dll`, not `.dll`).
- Repo has `core.autocrlf=true` and no `.gitattributes`: avoid sed/bulk rewrites; keep diffs free of line-ending churn.
- DF 53.15+ pools announcement reports: never delete `df::report` objects; the announcements vector grows monotonically (max-id cursoring is correct).
- `designate_dig type=mine/channel/ramp` over an existing stair silently converts it to plain floor by design (mirrors vanilla DF's own stair-removal mechanic) — the ACK reports it, but a model composing a room/hub designation should `look`/`cross_section` its target footprint first if a stair shaft might run through it, since losing a shaft's vertical connection can strand a fort's descent (this happened live in Fort #4 session 2).
- **Smoothing ALSO destroys carved stairs/ramps** — same vanilla mechanic, same live burn (Fort #5: a 2x2 shaft half-lost to a smooth rect). The `smooth` tool now warns when its rect contains carved shapes, but check the rect against stair tiles before designating anyway; constructed (built) stairs are immune and are the repair path.

## House rules

- **Skills carry procedural knowledge, never specifics**: order, dependencies, tradeoffs — no dimensions, materials, or coordinates. Emergence comes from the model's parameter choices. (A "make bedrooms 4x4" line in a skill is a bug.)
- **Tool schemas carry shape, never guidance**: an enum parameter lists a short curated vocabulary (~15 values or fewer) plus a name-passthrough where one exists — never the WHEN/WHY of a particular value, which belongs in a skill instead. Per-value facts (footprint, materials, prerequisites) live in an on-demand discovery tool, not the schema description: `build`'s `type` param + the `building_types` tool, and `queue_job`'s `item` param + the `job_types` tool, are the two working examples. (More than one clause of guidance on a single enum value, or more than ~15 names spelled out directly in a schema, is schema bloat — the same bug as the rule above, facing the other direction.)
- Stairwells default 2×2, span many z-levels in ONE designation, rooms branch beside the spine.
- **Cognition stays in-house**: no external memory/retrieval MCPs; external servers only at presentation boundaries (OBS/Twitch, Phase 3). The learning loop is the experiment.
- The fortress memory files (`fortress/memory/`) belong to the playing AI — tooling may read them, humans may audit them, but don't "clean them up" outside a play session.
