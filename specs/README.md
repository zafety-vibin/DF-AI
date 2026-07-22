# specs/ — numbered feature specs

Active (the only specs that describe the current system):

- `008-mcp-server/` — the MCP architecture flip. `design.md` is the architecture of record; `implementation-plan.md` is its executed plan (history).
- `009-culture-and-learning/` — next-phase design briefs (zones, perception round 2, locations, skills-as-culture), gated on 008's First Fort exit (passed 2026-07-12). Parts already shipped ahead of the gate — check `docs/decisions.md` and the code.
- `010-stream-harness/` — Phase-3 brief only (streaming/OBS), gated on 009.
- `011-planning-layer/` — wave-6 brief: plan ledger + plan lens/diff, geometry critic, provisional names, auto-bore candidates, layout metrics. Origin: fresh-eyes Fable analysis of five forts' spatial-burn history; principle "safety invariants become physics, taste stays culture."

Features 001–007 (2025-11, the pre-MCP orchestrator/BDI era) are superseded by the Feature 008 architecture flip and archived in full at `docs/archive/2025-11-legacy/specs/`:

- `001-binary-protocol` — DFHack↔Go binary TCP protocol + plugin skeleton. The protocol lives on, but its source of truth is now the code pair `internal/protocol` ↔ `dfhack-plugin/protocol.h`, not this spec.
- `002-foundation-infrastructure` — Go orchestrator scaffolding (config/logging/TCP server); `config/orchestrator.yaml` descends from it.
- `003-topology-graph-layer` — tile classification + topology overlay (ancestor of `internal/mapview`/`internal/worldmodel`).
- `004-hazard-overlays` — hazard overlays on the topology layer.
- `005-llm-integration` — direct-API LLM decision loop (retired; `internal/llm` pending deletion, 008 plan Task 16).
- `006-local-llm-agents` — local multi-agent LLM experiment (retired).
- `007-svp-housing-zones` — Spatial Validator Planner + housing agents (dead; predicates-not-procedures replaced it).
