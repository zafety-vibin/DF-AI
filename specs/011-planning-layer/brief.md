# Feature 011 — The Planning Layer (brief)

Status: BRIEF — adopted as the centerpiece of the next tooling wave
(wave 6). Not yet implemented. Origin: a fresh-eyes conversation between
the overseer and a Fable instance reasoning from five forts' incident
history (2026-07-19), quoted near-verbatim below with the outgoing
session's annotations. The overseer endorses the plan-ledger direction
("more than anything").

## The diagnosis (why this feature, stated by the source)

Five forts of spatial burns — the unconnected bedroom, the severed
stair core, smooth eating half a shaft, stairs landing in a workshop's
invisible footprint, the dig one row from a 7/7 water pocket — share
one shape: **none are grid-reading failures. Every one is a diff
between an intent and the world that existed nowhere** — not in the
render, not in the ACK, only in the player-model's head. The world
renders; the plan doesn't. The fort-planning skill asks the model to
hold a whole-fort, multi-year layout entirely in prose and journal
entries — the one representation no tool can paint, diff, or lint.

## The proposal (four instruments + one metric surface)

1. **Plan ledger.** Stamp named, function-tagged planned rectangles
   into the Go server before any dig ("East Wing pods, z=125, 8x 3x3").
   `look` grows a **plan lens**; a **plan_diff** tool answers:
   planned-but-not-dug, dug-off-plan, and — the killer feature —
   CONFLICTS computed in Go: plan overlaps a carved stair core, plan
   footprint adjacent to water/damp, plan unreachable from open space.
   Converts the whole burn class from spatial reasoning into
   list-checking. Free stream content: the fort's ghost-future rendered
   beside its present.
2. **Geometry critic.** The spatial sibling of check_goals: standing
   invariants run over the topology graph, callable after every
   designation batch — "shaft continuous from surface to z=N," "every
   named place connected to spine," "no designation within 2 of known
   water," "smooth rect contains no carved stairs."
3. **Names for intentions.** name_place requires already-dug space, so
   planning happens in raw coordinates — the regime models are worst
   at, exactly when the fort is most fragile. Provisional names for
   planned spaces + anchor-relative addressing in tool output
   ("candidate B: abuts spine west face, 3 below farm level") move
   design into the relational regime. Extends the project's
   names-over-coordinates lever backward in time, to spaces that don't
   exist yet.
4. **Bore-check as physics.** "Bore EVERY corner, not one
   representative point" is a rule the model must remember under
   pressure; find_dig_site should sample each candidate's
   corners+center automatically and annotate damp/aquifer/water.
   (Live confirmation the same day this brief was written: the Fort #5
   tavern pierce found aquifer at 3 of 5 bored columns of one 12x14
   footprint — a single representative bore would have lied.)
5. **layout_report + fort-stack view** (aesthetic/legibility metrics,
   data-not-rules): mean walk bedroom→dining and workshop→stockpile,
   chokepoint count, dead-end count per cluster, symmetry deviation
   off the spine — descriptive numbers, zero prescriptions, iterated
   against like failing tests. Plus one line per z: function, % dug,
   % planned, hazards — making the vertical dimension legible the way
   withDash made time legible.

## The governing principle (decision-log-worthy)

Every learnings.md RULE that reads "the model must remember to X" is a
tooling backlog item in disguise; several already made that journey
(the smooth warning, the tile-flags fix, the dig-over-stairs warning
that saved the tavern shaft the day this was written). The per-entry
judgment: **safety invariants become physics; taste stays culture.**
The model's novelty budget should be spent designing a canyon-hugging
fort, not on not-flooding.

## Wave-6 sketch (research → sequential implement → finalize, per the
## established workflow shape)

Research lanes:
- R1: Plan-ledger data model — Go-side state (plans are orchestrator
  knowledge, not DF state); per-fort persistence (survives sessions;
  likely a sidecar file keyed by world identity, which the dashboard
  now tracks); wire needs (likely none — plan storage and diffing are
  pure Go; conflicts need topology/water data already streamed).
- R2: Geometry-critic invariant set — which invariants are computable
  from the existing topology graph + water flags today; which need new
  plugin data.
- R3: Anchor-relative addressing — where tool outputs should speak
  relative ("abuts spine west face") and what the anchor registry is
  (named places + plan-ledger entries).

Implementation lanes (sequential, C++ tree shared):
- T1: plan ledger core (stamp/list/retire named rects, function tags,
  persistence) + plan lens in look.
- T2: plan_diff (planned-not-dug / dug-off-plan / conflict list:
  stair-core overlap, water adjacency, reachability, building
  footprints).
- T3: geometry critic (check_geometry tool, invariant table).
- T4: find_dig_site corner+center auto-bore annotations.
- T5: layout_report + fort-stack view.
- T6: carry-over gap list (see fortress/memory/goals.md session-3
  engineering queue): assign_squad systemic failure root-cause (top),
  list_squads fort-entity filter, stocks stack-units render, order
  restock-below-threshold conditions, look lens=items/stockpiles,
  elevation_view water-flag parity with cross_section,
  cancel_designation vs claimed jobs (can't cancel in-flight),
  minerals-lens 'd' reservation, build enum trim to ~15.

Schema-budget note: the deferral-era cost model (scaling report #2,
docs/decisions.md 2026-07-19) applies — new tools are cheap at session
start (names only); keep per-tool ≤3KB and put per-value facts in
discovery data.
