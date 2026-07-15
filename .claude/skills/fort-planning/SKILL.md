---
name: fort-planning
description: Use when embarking on a fresh fort (alongside fort-opening) or when an established fort's growth is being driven ad hoc — "dig a room near the stairs whenever someone's homeless" — instead of against a whole-fort, multi-year plan. Covers laying out every z-level's function at embark, sequencing the dig-ahead/furnish-gate cycle across migration waves, staging defense as tooling allows, and the aesthetic canon that costs nothing to keep. Replaces "orient everything around the main staircase" with real forward planning.
---

# Fort Planning — Whole-Fort, Multi-Year Layout

This skill is the planning layer above `fort-opening` (first-hours gate
checklist), `aquifer-piercing` (descending through a wet layer), and
`df-farming` (the plot chain itself). Use it to decide WHAT the fort's
shape is and WHEN each part gets built; hand off to those skills for HOW
to execute the pieces they own.

Sections below marked **[OVERSEER-CONFIRMED 2026-07-15]** carry taste
defaults the overseer explicitly confirmed — treat them as settled canon
for this project's forts unless the overseer revises them. One further
confirmed stance: run a SOFT POPULATION CAP around 60-80 (not v50's
default 200) so year-2 housing/tavern capacity stays inside what
dig-ahead delivers; revisit when defense and food tooling mature.

## Why plan (the waves math)

The failure mode this skill exists to prevent: a fort with no whole-fort
plan reacts to population pressure by digging one more room near the
stairs each time someone's homeless or hungry. That's the
"orient-around-the-main-staircase" anti-pattern — it works turn to turn
and produces a fort with no coherent shape by year 2.

The reason reacting doesn't scale is the shape of DF's own population
curve. In v50, migration arrives in two hardcoded waves during the first
two migration seasons (roughly 2-10 dwarves each), NONE in the first
winter, then wealth-driven waves of low double digits from the second
spring onward — uncapped, and scaling with fort wealth. Realistic
population anchors to plan pacing against (not hard targets — site and
play style shift these): roughly 15-28 dwarves by end of year 1, roughly
40-80 by end of year 2 and uncapped beyond. A plan laid out at embark can
have rooms ready before each wave lands; a reactive fort is always one
wave behind.

The first winter's wave-free lull is a planning gift, not a lull to
waste: use it for the farming + industry digs (below), not more beds —
nobody's arriving to need them yet.

## The level-category map

Dreamfort-derived doctrine: give each z-level ONE function. Two
constraints are hard; everything else is order-flexible.

**Hard constraint 1 — the farming level** is the uppermost soil layer
directly below the surface. Dig it first among the functional levels: it's
the fastest layer to excavate (soil, not stone) and sits closest to
surface pastures and butchery traffic.

**Hard constraint 2 — the industry level** is the first non-aquifer STONE
layer below the farming level. This isn't arbitrary — the boulders mined
while carving this level's footprint are exactly what feed the workshops
you're about to place on it. Any other stone level works as a stockpile
extension, but the FIRST one is the one that pays for its own dig.

**Everything else is order-flexible by site**: services (tavern +
hospital — see the gap note below), guildhall/temple/library, private
suites, general apartment levels, the crypt. Sequence these by what the
site and population actually need first, not by a template. `tomb`
already exists as a zone type for the crypt; suites/apartments are just
more bedroom zones — no special tooling needed for either beyond what's
listed in "the embark planning session" below.

**Adjacency correction**: strict one-function-per-level is mildly
anti-optimal for hauling — a pure industry level with its stockpile three
levels away means every boulder and bar climbs or descends stairs before
it's used. Pair each function level with its own stockpile directly
adjacent (the level immediately above or below), and the strict-purity
loss disappears.

**Dirty industries** (butcher, fishery, refuse) go near the TOP of the
stack, never deep — miasma from rot lingers worse the deeper and more
enclosed the space. Two mitigations that work with tools available today:
refuse rooms sealed behind two doors (physical containment via `build`
type door), and diagonal-only entrances into miasma-prone rooms — miasma
propagates orthogonally and straight down only, never diagonally, so an
entrance offset to a diagonal is pure dig geometry, not a special
building.

## Stair topology

The main 2×2 stair spine (the one sanctioned fixed-dimension default —
see `fort-opening` Gate 2 for the mechanics of carving it) is THE
surface-to-depth path. Community wisdom behind the width choice: a fort
with a single stairwell chokepoint dies at that chokepoint — whether to
an invasion funnel, a cave-in, or ordinary hauling gridlock at population.

Satellite stairs branch off the spine to connect function CLUSTERS (a
housing cluster, an industry cluster) so a cluster's internal traffic
doesn't all funnel back through the main spine to get anywhere. Plan
cluster boundaries before digging so satellite stubs land in sensible
places rather than being carved reactively later.

v50 overhauled carved stairs; the house rule already in force
(`CLAUDE.md`) applies to every shaft, spine or satellite: designate a
multi-z shaft in ONE `designate_dig type=stairs` call rather than
level-by-level.

Dreamfort's own variant is an undug-center 3×3 spiral stair (bigger
footprint, decorative undug core). **[OVERSEER-CONFIRMED 2026-07-15:
the plain 2×2 spine is this project's standard, not the spiral.]**

The stair-top (surface entry) is the single most defense-critical tile in
the whole fort. Enclose it with a surface perimeter BEFORE any other
defense work, and plan to hatch it once the interim seal policy below
calls for it — a hatch is buildable today (`build` type `hatch`), it just
isn't lockable yet (see the defense ladder).

## Dig-ahead + furnish gates

Dreamfort's `/dig_all` doctrine: designate the WHOLE planned fort stack
at embark — every level's footprint, stairs, and satellite stubs.
Mining is cheap and non-blocking (an idle miner just walks to the next
queued tile; there's no cost to having far more designated than can be
worked at once). Furniture and labor are the actually scarce, staged
resources — that's what gets rationed, not the dig.

**Furnish by population gate, not by calendar time.** Tie furnishing
waves to migration-wave arrival and `check_goals` evidence, never to "it's
been N days, build more beds." Against the wave math above: expect to
need on the order of 25 owned bedrooms carved by end of year 1, another
30-50 roughed in through year 2 — but treat those as pacing anchors to
check progress against, not fixed targets to hit exactly.

**Owned bedroom beats dormitory, urgently, not eventually.** A dormitory
sleep gives a recurring BAD thought every time it happens; ANY owned
bedroom — however minimal — gives a happy thought instead. Getting every
dwarf out of the dormitory and into an owned room is an ongoing metric to
check with `check_goals` at every wave landing, not a someday
nice-to-have.

**Food math**: a single fully-worked small farm plot feeds tens of
dwarves (see `df-farming` for the plot mechanics themselves). Favor
several smaller single-crop plots over one large plot early on — this
isn't just efficiency, it's a young-fort failure mode: a fresh fort's
grower-labor throughput chronically underuses one large plot, while
several smaller plots stay fully worked at the same population. Let the
playing session pick the count and size against actual grower labor on
hand, not a fixed number.

## Starter-then-permanent

Build a starter dorm/dining/depot near the stair-top immediately at
embark — and treat it as explicitly DISPOSABLE from the moment it's
built. Its entire job is bridging the gap between wagon-arrival and the
permanent levels above being furnished, nothing more.

The failure mode this section exists to name: starter rooms becoming
permanent by inertia. Nobody schedules the teardown, the room is "good
enough," and the fort is still eating in the entry foyer at year 3.
Decide the teardown/replacement moment explicitly — when the permanent
dining level (in the level-category map) is furnished and functional —
rather than letting the starter rooms drift into permanence unexamined.

## Defense staging ladder

Each stage is keyed to observable state, and every stage below names its
real tooling status — don't promise a stage the tooling can't back yet.

1. **Open** (year 1). No defense. Acceptable early risk while the fort is
   too small to be a target and too busy digging to spare labor.
2. **Door/hatch passive seal.** `build` type `door` and `build` type
   `hatch` both exist and are buildable today. But **no lock/forbid
   toggle tool exists anywhere in this project** (confirmed against
   `internal/mcpserver` — there is no forbid/lock endpoint). A built
   door or hatch is a real physical obstacle DF's own AI reasons about,
   but this project's tooling cannot dynamically lock, unlock, or forbid
   it in response to a threat. Treat stage 2 as "closed by default," not
   "toggle-controlled on demand."
3. **Trap corridor.** No trap-designation or mechanism-build tooling
   exists in this project at all. Not executable today.
4. **Drawbridge airlock / retracting bridge over a pit.** No
   bridge/lever tooling exists. Not executable today.
5. **Barbican/moat.** Far future; no current tooling story.

**The intended end-state design** (record it now, build toward it as
tooling lands, don't wait to plan it until the tooling exists): an
aquifer-pierced stair (see `aquifer-piercing`) leads into a wide
underground highway that crosses a PIT, bridged, before reaching the
fort's TRUE spine — the surface path and the true spine are deliberately
kept separate. The underground route is inherently resistant to climbers
and flyers since it has no exposed surface approach. The earthworks
themselves (the highway, the pit, the bridge emplacement) are pure
digging — designate_dig / channel — and so CAN be dug ahead today under
the dig-ahead doctrine above; only the actual seal MECHANISM (the bridge
and its lever) is blocked on tooling. Dig the earthworks now; the bridge
waits for the tooling.

**Interim seal policy [OVERSEER-CONFIRMED 2026-07-15]:**
default is an undug solid plug at the choke point — leave the last tile
or two of the connecting corridor unopened, and use a miner-on-demand as
the "lever" tooling doesn't provide yet: mine it open when passage is
needed, wall it shut again (queued `build`) when a threat is sensed.

**Topology [OVERSEER-CONFIRMED 2026-07-15]:** the confirmed choice is
hybrid — loop layouts (multiple connecting corridors) for
industry/stockpile clusters so hauling traffic isn't funneled through one
corridor, dead-end isolation for housing/noble clusters so a breach
doesn't cascade through the whole living-quarters population.

**Dig-ahead horizon [OVERSEER-CONFIRMED 2026-07-15]:**
the confirmed doctrine is to rough-dig the NEXT level's footprint while idle-miner
smoothing trails one level behind the excavation front on the CURRENT
level — excavation always stays one level ahead of polish.

## Aesthetics that survive function

- **Odd-width rooms and halls center doors and thrones at zero cost** —
  the sanctioned exception to the no-fixed-dimensions rule, because it's
  a stated aesthetic principle rather than a functional spec.
- **Symmetry is self-documenting.** A bilaterally symmetric layout off
  the main spine communicates its own logic to a future session picking
  the fort back up cold, with no notes required.
  **[OVERSEER-CONFIRMED 2026-07-15: bilateral symmetry off the spine +
  odd widths for centered doors IS the aesthetic canon — enforce it;
  mixing canons is what reads as ugly.]**
- **Finish order is not reversible mid-sequence: smooth → engrave →
  THEN furniture.** You cannot smooth a tile that already has furniture
  on it, and engraving needs an already-smoothed wall. Get the sequence
  right the first time or redo the earlier step.
- **Smoothing needs no pick** — any dwarf can do it, not just a miner.
  That makes it THE canonical idle-dwarf job to pair with dig-ahead:
  while miners advance the rough-dig front, idle non-miners smooth the
  level behind them. The `smooth` tool exists and works today. Engraving
  is selective — which rooms earn it is a taste call per room, not a
  default-everywhere behavior.

**v50 cave-in note**: pillars are pure decoration under current
mechanics — a single support column holds up anything, so wide open
halls are structurally safe without a pillar forest. The only real
cave-in risk is channeling or removing floors directly above occupied
space; open spans need no special care.

## The embark planning session (tool-by-tool)

1. **`survey_site`** — call first on any new fort (or after reconnecting).
   Map dimensions, surface z, dwarf cluster, sampled soil/stone
   stratigraphy.
2. **`look`** has four scopes, each earning its keep at a different planning
   moment. `local` (default) is a small crop around (x,y) — the routine
   close-up, not the planning view. `overview` is a downsampled whole-map
   orientation pass — use it early, before any level is picked, to get
   your bearings on the site as a whole. `elevation` renders the FULL
   z-level (full fidelity under ~100 tiles wide/tall, auto-downsampled
   above that) — this is the embark-time tool for actually picking a
   level: sweep candidate z's with it to judge a whole layer's soil/stone
   makeup before committing the level-category map to it. `fort` renders
   just the bounding box of tiles you've actually modified (dug/built/
   smoothed) plus an 8-tile margin — once something is dug, this is the
   recommended routine-review view for checking the fort's actual
   footprint against the plan, cheaper than `elevation` once the fort no
   longer spans the whole map.
3. **`cross_section`** bores at the entry column, and **`elevation_view`**
   sweeps along an axis, to actually pick the two hard-constraint levels:
   the uppermost soil layer for farming, the first non-aquifer stone
   layer below it for industry.
4. Lay out the stair topology (spine + satellite stubs) against those two
   picked levels plus the flexible ones from the level-category map.
5. **`designate_dig`** the whole planned stack per the dig-ahead
   doctrine — `stairs` multi-z in one call for shafts, `mine`/`channel`
   for room footprints per level.
6. **`name_place`** to label levels/regions as functions get assigned.
   Caveat: `name_place` requires the target tile to already be inside
   dug/open space (confirmed against `places.go`'s `resolveAnchor` —
   it errors on a tile with no carved region), so naming necessarily
   trails the dig. Plan the names on paper/in memory first; apply them
   as each level actually opens up.
7. **`apply_blueprint`** for repeatable room pods, whether from the
   checked-in library or captured from this fort's own play —
   `dry_run=true` ALWAYS first. **`save_blueprint`** closes the loop:
   capture a region this session actually dug as a named blueprint CSV
   under `blueprints/`, then re-stamp it elsewhere with `apply_blueprint`
   — design a pod once, reuse it for every later cluster of the same
   shape. `list_blueprints` re-scans `blueprints/*.csv` on every call, so
   a freshly captured pod shows up immediately, no restart needed. One
   real limitation survives: every captured tile is recorded as dig_type
   `default`, so stairs/ramps/channels in the source pod need hand
   re-designation after each re-apply — `save_blueprint` doesn't infer
   tile shape.
8. For the services level: `designate_zone type=meeting_hall`, then
   `create_location` with `type=tavern|temple|library|guildhall` (all
   four share the same meeting-hall precursor; `guildhall` additionally
   needs a `profession`). For a tavern, follow with `assign_lodging` to
   claim adjoining bedroom zones as guest rooms (see `fort-opening`
   Gate 6 for the mechanics). **Gap**: no `hospital` zone type exists in
   this project's `designate_zone` vocabulary at all — the notes'
   "services level = tavern + hospital" is only half-buildable today;
   hospital has no tooling path yet.
9. **`check_goals`** at every population gate — wave landing, season
   change, or whenever a section above calls for confirmation. It's the
   verification critic against live predicate evidence, not a status
   readout to skim once and forget.
