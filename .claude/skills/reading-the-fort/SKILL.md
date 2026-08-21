---
name: reading-the-fort
description: Use when any tool output is about to drive a decision — before stepping past an ACK, when a stockpile reads full, an order sits "in progress" with nothing produced, a stocks count looks like a famine or a surplus, a designation seems to vanish, a filter returns empty, or step deltas sag. The tools are truthful; every recorded misplay traces to misreading them — status words vs warning text, counts that measure something other than what was asked, silence that isn't success, and absence that isn't zero.
---

# Reading the Fort — Interpreting Truthful Output

The tool layer does not lie. Across six sessions of recorded play, not one
disaster came from a tool reporting false data — every one came from a
correct output being read wrong. This skill is the interpretation
discipline: four families of misreading, each with the live incident that
paid for it, and the fort's cheap vital signs.

## 1. The status word is not the message

A SUCCESS ack can carry a fatal warning in its text. Live incident: a
large dig rect's ack said, verbatim, that some tiles "will remove existing
stairs: vertical connection lost" — the session read "SUCCESS", called
`step`, and the miners severed the fort's own staircase, isolating a
whole quarter with dwarves inside. The tooling was correct and loud; the
failure was reading the status word instead of the sentence.

- **Parse the ack's full text before every `step`**, especially after any
  designation or build. Words like "remove", "destroy", "lost",
  "cannot", "suspended", "will convert" are stop-and-rethink triggers
  regardless of the status label. (A later fix wave downgraded the
  stair-destroying case to PARTIAL — do not rely on that; the discipline
  is reading the text.)
- PARTIAL and FAILED tell you exactly what to fix. Never re-issue a
  failed command unchanged.
- Warnings can gate real destruction: dig and smooth over carved
  stairs/ramps destroys them (two live incidents — see `CLAUDE.md`
  gotchas). The ack says so; act on it before stepping, not after.

## 2. Counts measure what they measure

Before acting on any surprising number, ask: what does this count
actually count?

- **Stockpile "full" — the items-vs-tiles diagnostic** (live-verified
  both directions, 2026-08-14). A stockpile reporting item count EXACTLY
  EQUAL to occupied tiles has NO CONTAINERS (one loose item per tile) —
  roughly an order of magnitude under design capacity. Items exceeding
  tiles is the healthy signature (bins hold many items). This single
  misread cost a fort ~10x storage for its entire life while being
  visible in `buildings` output the whole time. `buildings` now renders
  container limits + live container counts; a broken pile is retrofitted
  with `set_stockpile_containers` (omit limits to auto). Never dig more
  storage for a pile whose items == tiles.
- **Extent holes**: a pile's bounding-box area can exceed its reported
  total tiles — tiles missing from its extent map. Arithmetic (bbox area
  vs occupied/total) plus `look lens=items` finds them.
- **`stocks` food/drink counts are STACKS (barrels), not servings.**
  A low DRINK count was once read as a famine and emergency measures
  started; the fort was comfortable. Corroborate any hunger/thirst call
  with `wellbeing` (deprivation shows as stress) before declaring an
  emergency. The stack-units annotation has shipped but has never been
  confirmed rendering — assume the raw number is stacks.
- **`stocks` counts include items already built into furniture.** A
  count > 0 with a `build` plan repeatedly cancelling "needs X" means
  zero FREE spares — queue fresh crafting instead of trusting the
  number. Stay ahead: queue furniture production before placing plans.
- **Per-dwarf math**: the animal/citizen classification bug is fixed,
  but if population-derived numbers look implausible, sample
  `dwarf_detail` across ids before trusting them (see `fort-opening`
  Gate 1).

## 3. Silence is not success — and not failure either

Three different silences, three different meanings. Diagnose which one
you have before acting:

- **Silent death**: building plans die quietly after repeated
  "needs material" cancels — one loud announcement, then nothing.
  Completions are ALSO silent. After new material arrives, re-place
  anything not visibly complete, and verify completion with `look`
  (the glyph actually present), never by assumption.
- **Silent repeat (announcement dedup)**: damp/water dig-cancels fire
  loud exactly ONCE, then dedup silently on every repeat for the same
  tiles. A vanished designation with no fresh alert can still be a
  damp-cancel — but ONLY if a first loud cancel ever fired. Which leads
  to:
- **Silent nothing-yet**: a designation with no alert and no progress
  may simply have no miner arrived yet (live overseer correction,
  2026-08-14: "no damp-cancel alert meant the miners had NOT ARRIVED,
  not that a cancel was silently swallowed"). Before invoking the dedup
  explanation, check whether the first loud cancel ever appeared in
  `alerts`; if it never did, check dwarf jobs, distance, and competing
  designations instead of re-designating in a panic.

Also in this family:

- `look`'s designation overlay UNDER-REPORTS: a tile with an in-flight
  dig job can render with no 'd' glyph. Not necessarily cancelled —
  watch across a step before re-designating.
- Zone/goal predicates read a world-model snapshot that goes stale
  immediately after an edit — step ~100 ticks before trusting a
  just-touched predicate count; a same-turn 0 is not a failure.
- The alert cursor RESETS on reconnect and replays dozens of old
  alerts — expected noise after any reconnect, not new events. And
  after reconnect, `status` can report "Connected" while returning
  sentinel data (year=0, dwarves=0) for several queries — re-query,
  don't diagnose.

## 4. Absence is not zero

An empty result means "this query found nothing", not "none exist".
Before concluding absence:

- **Filters are case-sensitive** ("weapon" → nothing, "WEAPON" →
  results). Retry uppercase before believing an empty stocks/list
  result.
- **The same object has different names in different tools** (build's
  "coffer" is stocks' BOX; the mechanism item is stocks' TRAPPARTS;
  `order`'s item names differ from `queue_job`'s job names). An empty
  result for a thing you just made is a vocabulary miss until checked —
  `job_types`, `building_types`, `list_reactions` are the discovery
  authorities, not memory.
- **Some things are invisible to some views by design or gap**: loose
  floor items don't render in plain `look` but block `build` placement
  (`lens=items` paints them — live-verified; UPPERCASE letters are
  items outside any stockpile); constructed walls historically don't
  appear in `buildings`; `buildings` doesn't name a workshop's type —
  probe its job queue to identify it.
- **"unknown query name" means the running DLL predates the tool**, not
  that the capability is absent from the project. One cheap discovery
  call at session start (the `list_crops` pattern — see `df-farming`)
  is the canonical probe.

## The fort's vital signs

- **Tile deltas per step are the single cheapest whole-fort gauge.**
  Every `step` reports how many tiles changed. Sagging deltas with full
  order queues mean the labor force is idle or captivated — check
  work-detail modes FIRST (see `labor-and-work-details`), before
  re-issuing anything. Calibration from live play at a small fort
  (~20 dwarves): deltas stuck around a few dozen per step meant
  idle/captivated; roughly triple that meant genuinely parallel work.
  The absolute numbers scale with population — trend against the
  fort's own baseline, not against these figures.
- **`wellbeing` is the corroborator** for any hunger/thirst/misery call
  a count seems to make.
- **`check_goals` is the critic** — verify milestones against its live
  predicates, never declare them from memory.

## The discipline, compressed

1. Read the ack's TEXT before stepping; the status word alone is never
   enough.
2. When a number surprises you, ask what it actually counts (stacks?
   built-ins? extent tiles? containers?) before acting on it.
3. When something is silent, determine WHICH silence (dead plan, deduped
   repeat, nobody arrived) before re-issuing or diagnosing.
4. When a query returns empty, vary it (case, vocabulary, discovery
   tool, lens) before concluding the thing doesn't exist.
5. Judge the fort by deltas, alerts, and `check_goals` — never by
   re-checking the same tiles every turn (patience is policy).

## Related skills

`labor-and-work-details` owns the diagnosis once low deltas or a stalled
order point at the labor layer. `fort-opening` and `aquifer-piercing`
embed the opening-hours and damp-cancel instances of these rules.
