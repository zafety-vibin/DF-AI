# Feature 009: Culture & Learning — design brief

**Status**: brief for the next brainstorming cycle — NOT an approved design. Priorities get reordered by First Fort observations before planning.
**Gate**: begins after Feature 008's First Fort exit criteria pass (fortress/FIRST-FORT-RUNBOOK.md). Its first execution item is 008's gated Task 16 (legacy deletion).
**Goal**: from "can play" to "plays well and learns" — the research core. Fort N+1 must measurably benefit from fort N: skills referenced, mistakes not repeated, named places in the narration, headroom goals pursued unprompted.

## Workstreams (pre-First-Fort priority order)

### 1. Zones — the civzone port (plugin)
`applyZoneDesignation` is a hard stub; bedrooms/meeting/farm-as-zone all blocked on it. Research task against DFHack 53.x: `df::building_civzonest` creation, the post-50.x zone API (assignment, sizing), what `Buildings::constructAbstract` needs for civzones (it IS the intended call for abstract buildings — the one place it belongs). Multi-day; the largest single capability unlock. Bedroom-assignment predicates and housing-quality goals unblock behind it.

### 2. Perception, round 2 (from direct play experience — see decisions.md 2026-07-12)
- **Designation + building overlays painted into `look`** (not coordinate lists) — the room/shaft-gap class of error becomes visible.
- **Elevation view**: vertical slice along a line (all z for an x-range at fixed y) — the missing third orthogonal plane (plan/bore/elevation).
- **Reachability dry-run**: Go pathfinding over the topology graph answering "can dwarves reach these designated tiles from dug/open space?" — designate tools gain a warn-on-unreachable.
- **Region graph + named places**: auto-detect dug components per z; `name_place`/`list_places` tools; name↔bbox translation in both directions. Names-over-coordinates is the cheapest emergence lever and free stream narration.
- Deferred-from-008 cleanups that belong here: `cross_section` z=0 sentinel (pointer-int inputs), RenderCrop empty-rows guard, survey edge-clamping "N samples unavailable" notes.

### 3. Skills as culture (.claude/skills conversion)
Convert the 11 Go skills (internal/skill/builtins.go) to fortress/.claude/skills/*/SKILL.md preserving the HOUSE RULE: order/dependencies/tradeoffs, never dimensions/materials/coordinates. Add curated DF-mechanics reference skills (aquifers, moods, military basics) as progressive-disclosure references. Then the **learning loop**: after a `check_goals`-verified milestone or an instructive death, the playing model writes/updates a skill; verified-only entry (Voyager's rule); human audit during early iterations. Skill count stays small until description-matching strains (~50+; embedding retrieval is premature before that).

### 4. Verification & predicates expansion
Predicates today measure dug tiles and counts. Expand with the data now flowing: food/drink stocks quantified (stockpile_inventory → predicates), mood/stress risk (dwarf_detail), defense posture (entrance sealed?). `check_goals` becomes the model's habitual self-critic (charter already mandates it; make its output goal-diff-aware: "changed since last check").

### 5. Turn-flow quality
- **Step tripwires** (plugin): critical-severity announcement during a step → auto-repause early + reason in the step return. Makes multi-day steps safe; "let it run and react" becomes the default rhythm.
- Step return enrichment: job-completion summaries (orders finished, constructions built) alongside alerts.
- **Homeostasis executors** (df-ai's lesson, opt-in): threshold-driven Go automations the model can toggle (re-order drinks below N, auto-refuse hauling) — the LLM spends turns on strategy, not bookkeeping. Start with ONE (drink threshold) and evaluate.

### 6. Backlog burn (from the final review's Phase-2 triage)
blueprint per-tile send chattiness (batch into rectangles); inline-CSV blueprint authoring (the model drafts small blueprints — the "quickfort as a dial" step); queries.cpp int64→int16 validate-before-cast; handler naming consistency; extra pause-codec validation cases; frame_counter null-guard consistency; stale "53.02" header comment; legacy `ai-auto-update` path either routed through the drain or deleted outright (it is now redundant and documented teardown-unsafe); `connect_with_retry` sleep-under-suspension; repo-wide gofmt pass (lands naturally with T16's deletions).

## Success criteria
1. Fort with assigned bedrooms (zones working) and a dining hall in use.
2. A full season survived on multi-day steps with tripwires — no manual intervention.
3. The model authors ≥3 skills from experience; fort N+1 demonstrably applies one.
4. Narration uses named places; no coordinate arithmetic errors of the room/shaft-gap class.
5. `learnings.md` survives a session compaction and changes behavior afterward.

## Open questions (for the 009 brainstorm)
- civzone API shape in 53.x (research first, then scope).
- Does skill authoring need a template/linter to keep the no-specifics rule intact?
- Homeostasis boundary: which decisions are bookkeeping vs strategy? (User inclination: keep the LLM in charge of anything a viewer would find interesting.)
- Predicate quantities: thresholds fixed, config, or model-tuned?
