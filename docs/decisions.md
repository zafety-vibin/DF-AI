# Decision log

Append-only. One dated line per real decision, newest last. Format: `YYYY-MM-DD: decision — reason`.

- 2026-05-02: Predicates, not procedures — goal predicates (now/soon/eventual) name success conditions; the model decides how to satisfy them. Pre-built solution templates are forbidden; emergence is the point.
- 2026-05-03: Skills carry procedural knowledge without specifics — order, dependencies, tradeoffs; never dimensions/materials/coordinates. Same recipe must serve a peasant hut and a royal suite via the model's parameter choices.
- 2026-05-03: Stairwells default 2×2, span many z in one bulk designation, rooms branch beside the spine — user (DF expert) correction; single-column/shallow/hidden-averse advice is wrong on all counts.
- 2026-07-11: Architecture flip approved (Feature 008) — the Go orchestrator becomes an MCP server; Claude Code is the mind (session memory, skills, compaction); the bespoke LLM loop (internal/bdi + internal/llm) is retired after First Fort. Reason: the loop was a hand-rolled amnesiac agent harness; the harness already exists.
- 2026-07-11: Restructure in place on branch `008-mcp-server`; months of uncommitted BDI work snapshot-committed first (1bd95d1) as archaeology.
- 2026-07-11: First Fort is the Phase-1 exit milestone — supervised session: underground shelter, food/drink, beds, survive year 1.
- 2026-07-11: Quickfort/blueprints are a dial, not a rewrite — library blueprints now, model-drafted layouts later through the same validated tool; efficiency of AI-drafted layouts becomes a measurable research question.
- 2026-07-11: Execution process = subagent-driven waves (implement→review→fix gates) orchestrated by workflows; fable for implementation and review, sonnet for transcription-grade tasks.
- 2026-07-11: Target DF 53.15 / DFHack 53.15-r1 (the "dino update") — user-directed upgrade before the API audit so drift is fixed once. Dinosaur content requires newly generated worlds.
- 2026-07-11: Plugin builds via directory junction into the DFHack source checkout + `plugins/CMakeLists.custom.txt` hook — the old copied-directory setup had silently built stale code; there is no DFHack SDK, in-tree is the only supported path.
- 2026-07-12: Command/query/resync queues drain from the socket thread under CoreSuspender — live checkpoint proved `plugin_onupdate` never fires while DF is paused, which deadlocked the whole turn protocol (couldn't even unpause).
- 2026-07-12: Building placement uses `constructWithFilters` with per-type job_item filters (DFHack's own path), and the command dispatcher wraps DF calls in try/catch — `constructAbstract` on a workshop throws, and an uncaught throw off a thread hard-crashed DF.
- 2026-07-12: Perception stays slices + model-side assembly — confirmed by direct play experience; no volumetric/3D text renderings. Queued from the same experience: designation overlays in `look`, an elevation view (third orthogonal plane), and a Go-side reachability dry-run.
- 2026-07-12: `step` counts simulation ticks (1200 = 1 game day); long steps are the intended rhythm, with planned critical-announcement tripwires that end a step early — "let it run and react" over frame-polling.
- 2026-07-12: Cognition stays in-house — no external memory/retrieval MCPs (the learning loop is the experiment); external MCP servers only at presentation boundaries (OBS/Twitch, Phase 3).
- 2026-07-12: Build tool speaks CENTER coordinates for workshops and converts to the protocol's NW-corner wire semantic in one place (`buildWireCoords`) — models think in centers; the wire format doesn't.
- 2026-07-12: Repo adopts the stateless-docs organization (this structure) — CLAUDE.md bootstrap + docs/index + append-only decisions + stateless guides; pre-MCP root docs archived.
