# Token-cost scaling research — MCP tool payloads and session context growth

**Date**: 2026-07-12 (dated snapshot; per stateless-docs policy this file is frozen history — verify line numbers against git before acting on them)
**Scope**: how per-turn token cost scales as a fort grows, audited against the actual tool implementations on branch `008-mcp-server`, anchored to First Fort live-session observations (7 real dwarves, ~day 78, sessions 1-2 in `fortress/memory/journal.md`). Builds on 010's existing framing ("deliberate compaction cadence", "measure cost/hour" — `specs/010-stream-harness/design-brief.md` §1) rather than rediscovering it.

---

## Summary

The tool surface is mostly well-behaved: the recent cap wave (commit `7d558d0` "cap list outputs") bounded almost every response, and perception tools are bounded by design (radius/z-range/candidate caps). But three findings need attention **before** long-running sessions, ranked by severity:

1. **`stocks` is already at its 8 KB truncation cap at a 7-dwarf, day-78 fort.** The observed ~120 distinct item/material entries × ~70 chars/entry ≈ 8.4 KB exceeds `rawJSONCap` (8192 bytes) *today*. From this point on, every `stocks` call costs a flat ~2,000 tokens **and silently loses data** — the truncation cuts mid-JSON in item-type enum order, and the Go tool exposes no filter parameter even though the plugin already supports one.
2. **`orders` ships ~8 KB of static noise on every call.** It queries both `manager_orders` (real state) and `list_orders` — which is not an order list at all but the full DF job-type enum catalog (~240 entries, ~12-16 KB raw, truncated to 8 KB). That's ~2,000 tokens of unchanging catalog per call, at any fort size.
3. **`alerts` semantics decay because nothing ever dismisses.** No MCP tool exposes `AlertStore.Dismiss`; "active" alerts accumulate until the 200-entry ring buffer evicts them. The rendered list is capped (25), so tokens are bounded — but the dashboard's `alerts=N active` pins at 25 forever and stops carrying signal, and the model cannot distinguish "new problem" from "wallpaper".

Order-of-magnitude turn cost: **~3-5K tokens/turn today, ~6-10K mid-game, ~9-14K late-game** (assumptions below). An 8-hour unattended session (010's target) at a late-game fort processes on the order of **1.5-3M tokens of tool output + model output**, forcing compaction every ~12-45 turns depending on fort size. Both axes — payload size and compaction cadence — multiply; fixing the payload side stretches the compaction side.

---

## Current State Audit (per tool)

Chars→tokens assumption: ~4 chars/token for prose/JSON, ~2.5-3 chars/token for glyph grids and dense coordinate text (they tokenize worse). "Dash" = the ground-truth header (`renderDashboard`, `internal/mcpserver/dashboard.go:71-74`), ~130 chars ≈ **35 tokens added to every tool response** via `withDash` (`dashboard.go:79-81`). Bounded and worth keeping — it's the anti-stale-state contract.

| Tool | Bound mechanism | Evidence | Growth behavior | Typical cost now → at scale |
|---|---|---|---|---|
| `status` | trivially bounded | `tools_state.go:90-98` | constant | ~50 tok |
| `alerts` | render cap 25 (`SnapshotMaxAlerts`, `worldmodel/types.go:226,248`); tool's own 30-cap at `tools_state.go:110` is dead code (snapshot caps first); ring buffer 200 (`worldmodel/alerts.go:47-56`, default set at `types.go:134`) | see left | **tokens bounded, semantics unbounded**: no MCP dismiss path (`Dismiss`/`DismissAll` referenced only by legacy `internal/plan/executor.go:164-174`, pending Task-16 deletion) → active set saturates; dashboard count = `len(snap.ActiveAlerts)` (`dashboard.go:70`) pins at 25 | ~400-600 tok, pinned ~600 forever |
| `dwarves` | cap 50 (`maxDwarfList`, `tools_state.go:16`, `renderDwarfList` 76-87) | see left | bounded; but counts **all units incl. pack animals** (18 listed vs 7 real dwarves — `fortress/memory/learnings.md` "Population counting") | ~120 tok → ~300 tok at cap |
| `dwarf_detail` | top-5 skills, single unit (`dfhack-plugin/queries.cpp:265-284`) | see left | constant | ~130 tok |
| `stocks` | **8 KB truncation only** (`rawJSONCap`, `tools_state.go:20`, `capRawJSON` 24-33); raw pass-through with **no input params** (`in struct{}`, `tools_state.go:147`) | plugin walks `world->items.all` — every item in the world, not just stockpiled (`queries.cpp:421-429`, VERIFY note 425-426) and aggregates by exact (item_type, mat_type, mat_index) tuple with **no plugin-side cap** (`queries.cpp:432-462`) | **UNBOUNDED raw growth; cap already binding** at ~120 entries (observed day 78). Truncation order = item_type enum order, cuts mid-JSON. The plugin supports a `category` substring filter (`queries.cpp:412,438`) that the Go tool never passes | ~2,000 tok flat, with coverage shrinking: ~100% visible today → ~25-40% mid-game → ~5-15% late-game |
| `buildings` | plugin cap 200 + `truncated` flag (`queries.cpp:384,401`); one line per building (`renderBuildings`, `tools_state.go:40-73`); optional z filter exists (`tools_state.go:155-171`) | see left | linear to cap; 200 lines × ~60 chars ≈ 11.5 KB render at cap | ~160 tok (10 bldgs) → ~2,900 tok at cap |
| `jobs` | one workshop's queue (DF caps workshop queues ~10) (`queries.cpp:336-365`) | see left | bounded | ~100 tok |
| `orders` | 8 KB cap per sub-query, ×2 sub-queries (`tools_state.go:195-206`) | `list_orders` = static job-type catalog, ~240 enum entries hard-capped at 300 (`queries.cpp:150-198`, `maxJob=300` at 156) ≈ 12-16 KB raw → **always truncated to 8 KB**; `manager_orders` has **no cap** (`queries.cpp:212-228`, ~170 chars/order) | catalog: constant waste (~2,000 tok/call); manager orders: linear in queued orders (bounded by the 8 KB cap ≈ ~45 orders visible) | ~2,000 tok today (catalog only; no manager yet) → ~3,000+ tok with active manager |
| `look` | radius cap 15 → ≤31×31 grid (`tools_percept.go:24-30`); legend fixed (`mapview/types.go:14-16`); designation/aquifer as count lines not glyphs (`mapview/render.go:57-63`) | see left | bounded | ~400-550 tok (r=12) → ~750 tok (r=15) |
| `cross_section` | default z-range = dwarf cluster +3/−20 ≈ ~24-30 lines (`tools_percept.go:62-90`); caller can request full map depth (~150+ z) but must ask | see left | bounded in practice | ~300-350 tok |
| `survey_site` | 3 sampled columns + counts from a 31×31 slice, ~12 lines (`tools_percept.go:110-121`, `survey.go:48-120`) | see left | bounded | ~200-250 tok |
| `find_dig_site` | `MaxCandidates: 5` (`tools_percept.go:154-157`) | see left | bounded | ~120-180 tok |
| action tools (13: `designate_dig`, `chop`, `gather`, `build`, `zone`, `stockpile`, `order`, `queue_job`, `unsuspend`, `remove_building`, `cancel_designation`, `smooth`, `apply_blueprint`) | single-line truthful ACKs (`ackText`, `tools_action.go:17-32`); blueprint dry-run summarized | see left | bounded | ~60-80 tok each |
| `pause`/`unpause` | ACK | `tools_control.go:111-131` | bounded | ~60 tok |
| `step` | ticks capped at 14,400 (`tools_control.go:146-148`); report caps new alerts at 15 + "and N more" (`tools_control.go:94-106`) | see left | bounded | ~100-400 tok |
| `check_goals` | 9 starter predicates (`predicate/library.go:251-266`), 1-3 lines each + evidence (`renderGoals`, `tools_control.go:16-37`) | see left | bounded (grows only if the predicate library grows — 009 workstream 4) | ~200-400 tok |

**Fixed per-session overhead**: 29 tool schemas (name + description + JSON schema) live in every API call's system prompt — roughly 3-6K tokens, cache-friendly, unchanged by fort size. Note also that `withDash` → `Bridge.StatusLine` issues a `sim_status` plugin query on **every** tool call (`bridge.go:147-158`) — a latency cost, not a token cost, but it doubles plugin round-trips per tool call.

**L4 axis (memory files)**: `fortress/memory/journal.md` is append-only per charter (`fortress/CLAUDE.md` "Memory discipline") and is already ~100 lines after two sessions. It is read at session start; across the multi-fort "seasons" format (010 open question) it grows linearly with play history. `learnings.md` is append-only by design (correctly — that's the learning loop) but is curated-distilled, so it grows much slower.

---

## Scaling Model

### Assumptions (stated, order-of-magnitude)

- ~4 chars/token (JSON/prose), ~2.5-3 chars/token (glyph grids).
- A "turn" = observe (alerts + 1-2 `look`/`cross_section`) → decide → act (2-3 ACKs) → `step` → read report; plus a periodic heavier read (`stocks`, `buildings`, `check_goals`, `orders`) roughly every 2nd-3rd turn, amortized in.
- Model-visible output (narration + decisions) per turn: 0.5-1.5K tokens (supervised sessions narrate more; unattended likely less).
- Item-diversity growth: the plugin keys stocks by exact (item_type, mat_type, mat_index) (`queries.cpp:419`), and counts *every* item in the world including worn clothing and unit inventories (`queries.cpp:422`). Each migrant arrives with several clothing items in several materials; each butchered species adds meat/bone/skull/leather variants; each plant species adds plant/seed/drink/food variants. ~120 pairs at day 78 (observed) → plausibly 300-600 mid-game → 800-2,000+ after years of migrants, trade, and industry. This is the steepest organic growth curve in the system.
- Alert volume: job cancellations, births, moods, seasons, sieges. Early fort: a few per game-day; mature fort: tens per game-day, i.e. the 200-slot ring (`alerts.go:49`) holds only a few game-days of history late-game.

### Scenario A — early fort (now): 7 dwarves, day ~78, ~10 buildings, ~120 stock pairs

| Component | Tokens |
|---|---|
| alerts (17-25 active) | ~400-600 |
| look ×1-2 | ~500-1,000 |
| cross_section (occasional) | ~330 |
| 3 action ACKs | ~210 |
| step report | ~250 |
| stocks (every ~3rd turn, **at cap**) | ~2,000 amortized ~700 |
| check_goals / buildings (occasional) | ~200-400 |
| **Tool output/turn (amortized)** | **~2.5-3K** |
| + model output | ~0.5-1.5K |
| **Context growth/turn** | **~3-5K tokens** |

### Scenario B — mid-game: 20-30 dwarves, several migrant waves, ~50 buildings, 300-600 stock pairs, active manager

Changes from A: `dwarves` ~350 tok (45+ units listed); `buildings` ~800 tok; `stocks` still 2,000/call but coverage drops to ~25-40%, so the model plausibly calls it more than once seeking what it needs (~3K amortized); `orders` now carries real manager orders **plus** the 2K static catalog (~3K/call); alerts pinned at ~600 and semantically saturated.

**Context growth/turn: ~6-10K tokens.**

### Scenario C — late-game / long-running (010 ambition): 50+ dwarves, 200+ buildings, years of history, 800-2,000 stock pairs

Changes from B: `buildings` hits the plugin's 200 cap → ~2,900 tok *and* truncated (z-filter becomes mandatory); `dwarves` at its 50-cap (~300 tok but 60%+ of the fort invisible); `stocks` coverage ~5-15% per call — effectively blind without filters, inviting repeated calls (~4-8K/turn where the model cares about inventory); step reports carry more alerts.

**Context growth/turn: ~9-14K tokens.**

### Compaction cadence math (the second axis)

Assume a ~200K context window, ~30-40K fixed overhead (system prompt, 29 tool schemas, CLAUDE.md chain, memory files read at start), compaction triggered as the window fills → ~130-160K of usable turn history:

| Scenario | Turns per compaction cycle | 8-hour unattended session (15-30 turns/hr → 120-240 turns) |
|---|---|---|
| A (early) | ~35-45 | ~0.4-1.2M total tokens, ~3-6 compactions |
| B (mid) | ~18-25 | ~0.7-2.4M, ~5-10 compactions |
| C (late) | ~12-18 | ~1.1-3.4M, ~8-15 compactions |

Two implications for 010:
1. **Late-game forts compact 2-3× more often than early forts for the same wall-clock.** Every compaction is a lossy event; without deliberate anchoring, an unattended session loses its thread precisely when the fort is most complex.
2. **API cost/hour is dominated by context re-reads per model call**, so cache hit rate matters as much as raw volume — which is exactly why 010 already says "measure cost/hour and cache hit rates before committing to a duty cycle." This report deliberately stays in tokens; dollars depend on model choice and caching and should be measured, not estimated.

---

## Biggest Risks (ranked)

1. **`stocks`** — the only tool whose cap is *already binding* and whose truncation is *silent data loss in arbitrary enum order*. Unbounded raw growth (no plugin cap, keyed by full material tuple, counts every item in the world), no filter exposed, no aggregation. Gets strictly worse every migrant wave. (`tools_state.go:144-153`, `queries.cpp:407-465`)
2. **`orders`** — ~2,000 tokens of static job-type catalog on every call, forever (`tools_state.go:198` loops over both queries; `queries.cpp:150-198`). Pure waste; trivial fix. Manager-order growth is secondary but real once a manager exists (session 2 confirmed `order` is blocked on a manager today, so this is dormant, not dead).
3. **`alerts` semantic decay** — bounded tokens, broken meaning: no dismissal path in the MCP surface, so "active" saturates at 25 rendered / 200 stored and the dashboard's `alerts=N` flatlines. In an unattended run, the model's primary "why isn't this working" signal (per `docs/guides/mcp-server.md` tool families) degrades into wallpaper, and high late-game churn means the 200-ring evicts alerts the model never saw. (`worldmodel/alerts.go`, `types.go:226`, `dashboard.go:70`)
4. **`buildings` at scale** — honest cap + truncated flag (good precedent), but the flat one-line-per-building render costs ~2,900 tokens at the 200 cap and the model needs the *shape* of the fort ("do I have beds? how many workshops idle?") far more often than 200 coordinates. (`tools_state.go:40-73`, `queries.cpp:384`)
5. **`dwarves` count inflation** — token-bounded, but the animals-in-the-roster bug (18 vs 7) already corrupts per-dwarf predicate math (documented CONCRETE IMPACT in `learnings.md`) and migrant waves grow both citizens and livestock. At 50+ real dwarves the cap plus per-id `dwarf_detail` drill-down means population overview requires many calls.
6. **Journal/memory growth across forts** — L4, not a tool: `journal.md` linear in sessions, read at every session start. Slow-burn; matters only at the multi-fort "seasons" horizon.

Non-risks worth recording: all perception tools (radius/z/candidate-capped), all action ACKs, `step` reports, `check_goals`, and the dashboard itself are bounded and should be left alone. The 8 KB `capRawJSON` guard and the plugin's 200-building cap + `truncated` flag are the right *pattern* (truthful truncation with a refine hint) — the fixes below extend that precedent rather than replacing it.

---

## Recommendations (prioritized)

### Cheap / high-value now (candidates for the next fix wave, before deep First Fort play)

1. **`stocks`: aggregate in Go, expose the existing plugin filter.** (a) Add `category` (plugin already implements substring matching — `queries.cpp:438` — the Go tool just never sends it, `tools_state.go:148`) and a `min_count` param. (b) Default render: Go-side grouped summary, one line per item_type — `BOULDER: 47 total across 12 materials (38 usable / 9 economic); top: shale 20, chalk 11, ...` — with full material breakdown only when a category filter narrows the set. This is the same progressive-disclosure shape as `look`-crop → `dwarf_detail` drill-down (per `docs/guides/world-model.md`). Collapsing ~120 entries to ~20 type-lines cuts the call from ~2,000 to ~300-400 tokens *and removes the silent data loss*, because aggregation happens before, not after, the cap. Estimated effort: one Go render function + input struct, no plugin change required for v1.
2. **`orders`: stop fetching the `list_orders` catalog per call.** Drop it from the tool (the orderable vocabulary is already enumerated in the `order`/`queue_job` input schemas, `tools_action.go:89-96`), or move it behind a separate rarely-called `order_types` tool. One-line change; saves ~2,000 tokens per `orders` call forever. Render `manager_orders` compactly in Go (one line per order, only non-default fields) instead of raw JSON.
3. **`alerts`: give the model a cursor and an ack.** Add `since_id` (DF report ids are monotonic — the max-id cursoring pattern is already project law per CLAUDE.md's 53.15 gotcha) and/or an explicit `dismiss`/`ack_through(id)` tool wired to the existing `AlertStore.Dismiss`/`DismissAll` (`alerts.go:88-117` — the machinery exists, unexposed). Fix the dashboard count to read true undismissed count from the store (`Counts()`, `alerts.go:155-168`) instead of `len(snapshot slice)` so `alerts=N` regains meaning. Also delete the dead 30-cap at `tools_state.go:110` or align it with `SnapshotMaxAlerts`.
4. **Token telemetry in the MCP server (010's "measure cost/hour" prerequisite).** Log response byte-size per tool call to stderr (`logging.NewStderrTextLogger` — stdout is the transport). Near-zero effort; turns this report's estimates into measured curves and gives 010 its cost-per-hour input. A dated archive snapshot of one real session's distribution would be the natural follow-up.

### Needed at scale (009/010 horizon — do when the trigger fires, not before)

5. **`buildings`: summary-by-type default, roster on request.** Default render: `52 buildings: 14 bed (all built), 3 workshops (1 UNDER CONSTRUCTION), ...` grouped by type with construction problems surfaced loudly (unfinished buildings are the whole point of the tool, per its own comment `tools_state.go:36-39`); full per-building lines only with a `z` or `type` filter. Trigger: forts crossing ~50 buildings. Pairs naturally with 009's planned building-overlay-in-`look` work (journal session-2 design note already says exact type stays a detail query).
6. **`dwarves`: citizen filter + status summary.** The citizen/race filter is already filed as an upstream fix (learnings.md; it also corrupts predicates). While there, make the default render a summary (`7 citizens: 5 working, 1 idle, 1 sleeping; 11 animals`) with the id-roster behind a `full` flag. Trigger: first migrant wave past ~20 citizens.
7. **Deliberate compaction cadence for unattended runs (010 workstream 1, concretized).** Proposal: checkpoint at every **game-season boundary** (a natural narrative seam the step report already surfaces via the dashboard's `season=` field) or every **~25 tool calls**, whichever comes first: (a) append a 3-6 line block to `journal.md` and refresh `goals.md` (already charter policy, `fortress/CLAUDE.md` step 6); (b) then trigger compaction deliberately, re-injecting `goals.md` + `learnings.md` + the newest journal block + one fresh `status`/`check_goals` read to re-ground — exactly design.md §4.3's "summarize → re-inject" made periodic. The scaling table above says why cadence must scale with fort size: late-game forts hit involuntary compaction ~every 12-18 turns, so the deliberate checkpoint must run at least that often or the harness compacts *for* you at an arbitrary point. Payload fixes 1-2 and 5-6 directly stretch this cadence (~30-40% less tool-output per turn at scale).
8. **Journal rotation across forts.** At the multi-fort "seasons" format, one growing `journal.md` read at session start becomes its own tax. Policy belongs to the playing AI's charter, not tooling: per-fort journal files (e.g. `memory/forts/<fortname>.md`) with `journal.md` holding only the current fort + a one-line index of past forts. Raise during a play session or 009's memory-discipline work — **not** an edit tooling should make unilaterally (house rule: the memory files belong to the playing AI).
9. **Plugin-side `stocks` scale guard (only if measurement demands it).** The item walk over `world->items.all` per call is O(items) CPU on the DF main thread (the handler's own comment flags it, `queries.cpp:414-416`). If telemetry shows it hurting frame time on mature forts, add plugin-side category/top-N support so the filter prunes before serialization. Defer until measured.

### Explicit non-recommendations

- **Do not shrink or remove the dashboard header** — 35 tokens/response is the cheapest anti-stale-state insurance in the system.
- **Do not paginate `look`/`cross_section`** — they are already the progressive-disclosure detail layer and are correctly bounded.
- **Do not add an external memory/retrieval MCP** to manage context — cognition stays in-house (decision log 2026-07-12); the fix is smaller payloads + deliberate cadence, not outboard memory.
