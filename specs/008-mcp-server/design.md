# Feature 008: DF-AI v2 — MCP Server Architecture

**Status**: Design approved in principle 2026-07-11; pending final spec review
**Branch**: `008-mcp-server`
**Supersedes**: the bespoke LLM orchestration in `internal/bdi` (deliberator passes), `internal/llm` (providers/parser), and the entire legacy `df-orchestrator` stack. The DFHack plugin, binary protocol, and world-model plumbing survive and are repurposed.

## 1. Goal and north star

An AI that genuinely *plays* Dwarf Fortress: sees the world well enough to make grounded spatial decisions, acts through the game's real vocabulary, verifies its own outcomes, accumulates skill across forts, and is entertaining to watch. North star: a long-running Twitch stream ("Claude plays DF") that reacts to sieges, moods, and disasters with visible reasoning, and gets *better over time*.

This feature covers the architectural flip that makes that possible. It does not cover the stream harness itself (Phase 3).

## 2. Why the flip (diagnosis summary)

Full diagnosis with evidence: session review 2026-07-11. The four load-bearing findings:

1. **Plugin refuses hidden tiles on regular digs** (`dfhack-plugin/designations.cpp:146-149`). All undug underground DF tiles are hidden; designating into fog is the core gameplay verb. Stair shafts span hidden tiles; rooms don't. The canonical opening (stairs down → carve rooms) half-works and returns "No tiles designated."
2. **Prompt anti-guidance** (`internal/bdi/prompt.go` VALIDATION): "Don't dig UNKNOWN tiles... dig adjacent to KNOWN tiles" confines the agent to the revealed shell — i.e., the surface.
3. **Spatial blindness** (`internal/bdi/render.go`, `internal/query/handlers.go`): the model only ever sees aggregate open/closed/unknown counts. It dug a 10×10 "dining hall" on open surface meadow (logs/bdi-interactions.jsonl, 2026-05-03) because nothing in its input distinguishes meadow from rock.
4. **Amnesiac bespoke harness**: fresh history every cycle (`internal/bdi/loop.go` runCycle), 2048-token cap, Haiku-class model, no pause control — DF simulates in real time during 30-120s of deliberation.

Layers 1-2 are bugs to fix regardless of architecture. Layers 3-4 are symptoms of hand-building an agent harness. Claude Code already provides the harness: persistent sessions, compaction, tool use, skills, memory, subagents, MCP event channels. The project's differentiated value is the *game bridge* and *DF knowledge*, not loop plumbing.

Research grounding (do not re-derive; see memory `game-agent-prior-art`): every successful game-playing LLM system converged on harness-owned world model, structured-text perception with on-demand local detail, spatial computation in code, code-as-action with verification, and curated distilled memory. Raw ASCII map dumps are proven failures (tokenization destroys adjacency). Quickfort blueprints as action language validated by closest prior art. No serious DF MCP server exists.

## 3. Architecture overview

```
┌─────────────┐   TCP (existing     ┌──────────────────┐   MCP (stdio)   ┌─────────────────────┐
│ DF (Steam)  │   binary protocol)  │  Go MCP server   │◄───────────────►│ Claude Code session  │
│  + DFHack   │◄───────────────────►│  "nervous system"│                 │  "the mind"          │
│  C++ plugin │                     │                  │─channel push───►│ .claude/skills/ = DF │
│  "senses/   │                     │ topology, world- │  (alerts)       │   culture            │
│   hands"    │                     │ model, quickfort,│                 │ memory dir = journal,│
└─────────────┘                     │ predicates, cmds │                 │   goals, learnings   │
                                    └──────────────────┘                 └─────────────────────┘
```

**Roles.** Plugin = senses and hands (extraction, designation, pause). Go server = nervous system (owns the global map, does all geometry, exposes semantic tools, verifies outcomes, later runs homeostasis executors). Claude Code = mind (strategy, novelty, narration). Skills = culture (procedural DF knowledge + learned lessons). Quickfort = hands' vocabulary for construction.

**Auth/billing reality** (verified 2026-07-11): interactive Claude Code sessions run on the Max 20x subscription. Headless `-p` / Agent SDK require API billing. Phases 0-2 run supervised on subscription; Phase 3 (unattended 24/7) reuses the same MCP server from the Agent SDK on API. Rate-limit pauses during subscription play are acceptable (and even streamable).

## 4. Component design

### 4.1 DFHack plugin (C++) — keep, fix, extend

Keep: TCP transport, binary protocol, tile extraction, entity extraction, announcements stream, all 11 command handlers, queries.

**Fixes (Phase 0):**
- F1. `designations.cpp`: designate hidden tiles for ALL dig types, matching DF UI semantics (delete the `hidden → blocked` branch; keep the blocked counter for genuinely invalid tiles, e.g. out-of-bounds or already-open floor for wall-dig types).
- F2. `entities.cpp`: background-thread reads of `world->units` without `CoreSuspender` — add suspend or move to onupdate tick. Latent memory-corruption hazard.
- F3. `df_ai_protocol.cpp` RESYNC path: `send_full_state()` runs on the socket thread — queue it to the main thread like commands.
- F4. Compile-verify `buildings.cpp` / `work_orders.cpp` against installed DFHack 53.12 SDK (they carry many "VERIFY against headers" notes and have never been built).

**Extensions (Phase 1):**
- E1. `PAUSE`/`STEP` command: pause/unpause via DFHack `World::SetPauseState` (verify exact API in 53.12), plus "run N ticks then re-pause" for turn-based play.
- E2. `MAP_SLICE` query: given a bounding box + Z, return per-tile **semantic classification** using DFHack `tileMaterial()` / `tileShape()` / `tileSpecial()` (NOT raw tiletype enum guessing — this replaces the invented enum ranges in `internal/hazards/detectors.go`). Per tile: shape (floor/wall/stair/ramp/tree/sapling/shrub/water/magma), material class (soil/stone/mineral vein/grass/constructed), hidden flag, designation flag. This is the data source for `look` / `cross_section` / `find_dig_site`.
- E3. Delta feeds: tile_updates already exist; extend the announcement poll to include job-cancellation reasons if cheap. (RemoteFortressReader's "changed since last call" pattern is the model.)

### 4.2 Go MCP server — the new `cmd/df-mcp`

One binary, stdio transport (registered in `.mcp.json`; HTTP later if the stream harness needs it). Wraps existing packages; the MCP layer is thin.

**World-model layers.** The original design's flaw was one representation serving two consumers. v2 separates them — each layer lives in the substrate that can keep it true cheapest:

| Layer | Lives in | Holds |
|---|---|---|
| L0 source of truth | DF + plugin | actual game state; queried, never fully mirrored |
| L1 substrate | Go | bit topology + classified tile cache (E2 shape/material) + entity/zone/job indexes; consumed only by Go geometry |
| L2 semantic compile | Go | region graph (named rooms/stairs/connectivity), per-column stratigraphy, surface analysis, hazard ledger; updated incrementally from deltas |
| L3 views | tool responses | ground-truth dashboard, look crops, cross-sections, ranked candidates, turn deltas ("since last turn: 14 tiles dug, 2 cancellations + reasons") |
| L4 the mind's model | session memory dir | journal, goals, learnings, named places; re-grounded by L3 every turn |

The BDI mapping survives without its Go structs: beliefs = L1-L3 (harness-grounded) + L4 (self-maintained); desires = predicates + goals.md; **intentions = the game itself** — a dig designation is a stored intention, and alerts explain every cancellation. In-flight work is reported as a diff against the designations the model placed, replacing the per-tile prediction/confirmation engine (see 4.5).

Two model-facing principles: **names over coordinates** (spaces get names as soon as they exist; tools translate name↔bbox both directions — LLMs reason far better over places than coordinate math, and named geography is free stream narration) and **affordances, not just state** (perception tools answer "what can I do here" — ranked candidates with rationale; Go computes eligibility, the model chooses among valid options).

**Survives (repackaged):** `internal/protocol`, `internal/dfhack` (client), `internal/commands` (executor), `internal/topology` (graph + overlay), `internal/worldmodel` (observed state, alerts), `internal/blueprints` (quickfort CSV), `internal/predicate` (→ verification), `internal/query` handlers (→ tools), `internal/modifications` (dug-space tracking feeding `region_graph`). Skill *content* from `internal/skill/builtins.go` migrates into SKILL.md files. `internal/hazards` survives as the hazard *ledger* but its detectors are rebuilt on E2's classified tiles (the invented tiletype enum ranges in `detectors.go` are deleted).

**Dies:** `internal/llm` entirely; `internal/bdi` loop/prompt/render (the render's orientation/alerts ideas are reused inside tool responses); `internal/plan` + `internal/reconcile` in current form (see 4.5); legacy `internal/{autonomous,agents,context,spatial,zones,phases,planning,http}` and `cmd/df-orchestrator`, `cmd/blueprint-analyzer`; `dfhack-plugin/df_ai_protocol_backup.cpp`. Deletion happens incrementally during Phase 1, not as a big-bang.

**Tool surface (v0 — names indicative):**

*Perception (Go owns the map; the model asks local questions):*
- `survey_site()` — semantic embark summary: map dims, surface Z, wagon/dwarf positions, biome hints, tree/plant counts, nearest cliff faces, soil depth and first-stone Z per sampled column, water features, detected aquifer layers.
- `look(x, y, z, radius≤15)` — annotated map crop: one glyph per tile, **no separator spaces**, labeled rows/cols every 5, legend, explicit orientation header ("z=110, 3 below surface, north=up"). Supplement, not primary channel.
- `cross_section(x, y, z_top, z_bottom)` — vertical slice at a column: per-Z one-line classification ("z110 grass floor | z109 clay | z108 granite (hidden)").
- `find_dig_site(width, height, {near, z_range, prefer})` — Go searches for diggable rectangles satisfying constraints; returns ranked candidates with coordinates and rationale. All placement geometry happens here.
- `region_graph()` — named dug spaces/zones + connectivity (stairs, corridors) once a fort exists; from the topology graph.
- `alerts()`, `dwarves()`, `dwarf(id)`, `stocks()`, `jobs()`, `orders()` — wrap existing Tier-2 queries + entity/announcement state. Every action response also injects a compact ground-truth header (tick, season, dwarf count, active alerts count) — never let the model act on remembered state.

*Action (truthful ACKs; partial results are errors, not silent successes):*
- `designate_dig(type, x1,y1,z1, x2,y2,z2)` — dig/stairs/channel/ramp/up/down; response includes designated/blocked counts and the specific failure coordinates when non-zero.
- `apply_blueprint(name_or_csv, cursor_x, cursor_y, cursor_z)` — quickfort: apply a library blueprint or inline CSV the model emits. Validates against the map first (dry-run diff: what would designate where), applies on confirm.
- `build(type, x, y, z)`, `zone(type, rect)`, `stockpile(category, rect)`, `order(item, count)`, `unsuspend(x,y,z)`, `cancel(rect)`, `smooth/engrave(rect)`.

*Control & verification:*
- `pause(on)`, `step(ticks)` — the turn protocol: pause → observe → think → act → step N → repeat. Default supervised cadence: step ~600-1200 ticks (half to one in-game day; 1200 ticks = 1 fortress-mode day) per turn, tunable.
- `check_goals()` — runs the predicate library against fresh state; the Voyager-style critic. Skills reference these predicates as success conditions.

*Push (MCP channel, opt-in `--channels`):* critical announcements (siege, mood, death, flood) push into the session immediately instead of waiting for the next poll.

Tool responses stay under the 10k-token MCP default; `look` radius and list lengths are capped accordingly.

### 4.3 Claude Code side — project layout for the *player*

A `fortress/` workspace (inside this repo or sibling) that the playing session opens:
- `CLAUDE.md` — the player charter: role, turn protocol, memory discipline rules, stream voice (later). Short; knowledge lives in skills.
- `.claude/skills/` — the DF skill library. Seeded by converting the 11 Go skills (carve_initial_shelter, dig_safe_stairwell, build_bedroom, setup_workshop_chain, setup_stockpile, produce_furniture, establish_food_supply, create_dining_hall, secure_entrance, secure_water_access, handle_aquifer_layer) into SKILL.md files, preserving the **procedural-knowledge-not-templates rule** (order + dependencies + tradeoffs; never dimensions/materials/locations — see memory `skill-library-design`). Plus curated quickfort community blueprints (Dreamfort excerpts) as skill resources.
- `memory/` — `journal.md` (fort narrative log), `goals.md` (3-tier goal hierarchy that survives compaction), `learnings.md` (distilled, append-mostly; explicit "do not delete entries" guard — prior art shows models destroy precious notes in cleanup).
- Session pattern (Phase 1-2): interactive session, human-supervised; `/loop`-style cadence comes later. Context resets are deliberate: summarize → re-inject goals.md + learnings.md.

**Learning loop (Phase 2):** after a verified milestone or an instructive death, the agent writes/updates a SKILL.md (e.g., `siege-response`, `aquifer-piercing`) with what worked, referencing predicates as verification. Skills persist across forts and model upgrades — this is the "improves over time" mechanism, and it's Voyager's result on infrastructure Claude Code already ships.

### 4.4 Action language: quickfort

For anything beyond a single rectangle, the model emits **small quickfort CSV blueprints** (or picks + parameterizes library ones) applied via `apply_blueprint`. Rationale: human-readable, diffable, z-aware (`#>`/`#<`), validated before execution, and the community library is a free expert corpus. The model never hand-draws huge grids (research: emit small blueprints or parameterized generators; Go expands). `internal/blueprints` already parses the format.

### 4.5 What happens to BDI concepts

- **Predicates** → survive intact as `check_goals()` + skill success conditions. Same Now/Soon/Eventual horizons; curriculum-first stance unchanged.
- **Plan DAG / executor / reconciler** → largely absorbed. With pause-based turns the world doesn't drift mid-thought, so tile-level prediction/confirmation machinery loses its main job. DF itself tracks designations (visible in tile flags); `jobs()`/alerts report progress. The Claude session holds intent. Deterministic *homeostasis executors* (df-ai-style: stock thresholds, auto-bedroom-assignment) may later live in the Go server as opt-in automations the model toggles — that's the surviving spirit of the executor, deferred past First Fort.
- **Multi-pass REQUEST grammar** → replaced by native MCP tool calls.

## 5. Milestone: First Fort (Phase 1 exit criterion)

Supervised session, fresh embark, model = current frontier Claude on subscription:
1. Reads the site (`survey_site`, `cross_section`), narrates a plan.
2. Digs a real underground opening: stair shaft + rooms carved into hidden tiles (proves F1 + perception).
3. Establishes food/drink security (farm plots or gathering + still), beds for all dwarves in bedrooms, a dining area.
4. Survives to the first migrant wave and through year 1 with zero starvation/dehydration deaths.
5. Every action verified via `check_goals()` / truthful ACKs; journal maintained.

## 6. Phases

- **Phase 0 — Safety and ground truth (small):** snapshot commit (done, `1bd95d1`); plugin fixes F1-F3; F4 compile-verify against DFHack 53.12; delete `df_ai_protocol_backup.cpp`. Manual smoke test: dig a room into hidden rock via the existing df-bdi path or a test harness.
- **Phase 1 — MCP v0 + perception + First Fort:** `cmd/df-mcp` with the tool surface above (perception first: survey/look/cross_section/find_dig_site; then actions; then pause/step). Plugin E1/E2. Register in `.mcp.json`. Iterate live in supervised sessions until First Fort. Legacy code deleted as its replacements land. Unit tests for renderers/geometry (goldens for `look` output); currently the repo has zero tests — new code doesn't ship without them.
- **Phase 2 — Culture and learning:** SKILL.md library conversion + quickfort corpus; memory discipline (journal/goals/learnings); `check_goals` verification loop; skill-writing-after-success; homeostasis executors as needed.
- **Phase 3 — The stream:** Agent SDK + API key for unattended runs; OBS overlay rendering the model's narration; channel-pushed events driving reactions; Twitch chat as an MCP tool. Out of scope for this spec beyond the constraint that the MCP server must serve both interactive and SDK clients unchanged.

## 7. Risks and mitigations

- **DFHack 53.12 API drift** (buildings/work_orders/pause): F4 compile-verification is Phase 0, before any new work stacks on top. Keep the plugin-version verification checklist from memory `plugin-build-pipeline`.
- **MCP output caps** (10k tokens default): cap `look` radius, paginate lists, keep tool responses semantic not exhaustive.
- **Context rot in long sessions**: deliberate reset cadence + goals/learnings re-injection (design, not hope).
- **Subscription limits (Max 20x)**: supervised sessions sized to windows; overage → the fort pauses (game pauses with it — no harm). Unattended play explicitly deferred to API billing.
- **Model writes junk skills**: skills enter the library only after their success predicate has been observed true (Voyager's verified-only rule); human review during Phase 2.
- **Scope creep toward the legacy sin** (three coexisting control paths): the BDI loop is retired as soon as First Fort lands via MCP; no parallel brains.

## 8. Open questions (deliberately few)

1. Exact DFHack 53.12 pause/step API shape (resolve in Phase 0/F4 while inside the SDK headers).
2. Whether `find_dig_site` needs vein/mineral awareness for First Fort or ships terrain-only (lean: terrain-only).
3. Homeostasis executor scope for Phase 2 (defer decision until First Fort behavior data exists).
