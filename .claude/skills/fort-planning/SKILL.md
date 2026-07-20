---
name: fort-planning
description: Use when embarking on a fresh fort (alongside fort-opening) or when an established fort's growth is being driven ad hoc — "dig a room near the stairs whenever someone's homeless" — instead of against a whole-fort, multi-year plan. Covers laying out every z-level's function at embark, sequencing the dig-ahead/furnish-gate cycle across migration waves, staging defense/industry/noble-response/water-power as tooling and observed fort state allow, and the aesthetic canon that costs nothing to keep. Replaces "orient everything around the main staircase" with real forward planning.
---

# Fort Planning — Whole-Fort, Multi-Year Layout

This skill is the planning layer above `fort-opening` (first-hours gate
checklist), `aquifer-piercing` (descending through a wet layer),
`aquifer-water-infrastructure` (turning a sealed pierce into a water
source), and `df-farming` (the plot chain itself). Use it to decide WHAT
the fort's shape is and WHEN each part gets built; hand off to those
skills for HOW to execute the pieces they own.

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
hospital), guildhall/temple/library, private
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

**Street-grid-first infill [OVERSEER-DEMONSTRATED 2026-07-19]:** the
cheapest staged form of dig-ahead, observed working at scale in the
overseer's own forts: dig a level's CORRIDOR NETWORK first (streets,
ring corridors, the spine connection — fast, low tile count), which
implicitly defines every future room as an undug block inside the
established grid. Each migration wave then only needs its blocks
hollowed and furnished — growth becomes infill that continues the
existing geometry instead of reactive annexes that fight it. Pair with
paired-cell rooms sharing door walls for density without dormitories.

**Run multiple fronts per step [OVERSEER-CONFIRMED 2026-07-19]:** plan
and designate across SEVERAL sites/tasks before each `step`, so the
simulation employs the whole labor force at once — a
one-project-then-step loop leaves whole labor groups idle every step.
Batch designations, builds, and orders across fronts; then step long.

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
3. **Trap corridor.** Trap-building tooling now exists: `build` places
   `weapon_trap` (armed at construction — it also consumes a weapon or
   trap-component item from stock, so a weapons-producing industry
   upstream is part of this stage's real cost), `stone_fall_trap`, and
   `pressure_plate`. Two real gaps survive, so don't over-promise this
   stage: `stone_fall_trap` builds UNARMED (DF's own flow loads the
   boulder afterward via a separate Load Stone Trap job at the built
   trap, which no tool here exposes yet), and `pressure_plate` builds
   with every trigger-detection category off (creatures/water/magma/
   carts), so a built plate won't actually fire until a future tool
   exposes that configuration. A weapon-trap corridor is genuinely
   executable today; a pressure-plate-triggered anything is not yet.
   (`track_stop` shares the trap family in `building_types` but is
   minecart infrastructure, not a trap — and placement-only at that; see
   the water & power section.)
4. **Drawbridge airlock / retracting bridge over a pit.** Real and
   live-verified: `build` type `bridge` (arbitrary footprint + a raise
   direction), `build` type `lever`, and `link_building` (wiring a built
   lever — or a pressure plate — to a bridge/floodgate/door/hatch/
   support/gear_assembly target) plus `pull_lever` were shipped and
   confirmed working end-to-end in a prior fort's session. This stage
   is executable today, not just planned toward. `floodgate` is also a
   real build type but still pending its own live verification — treat
   it as shipped-but-unproven until a session confirms it, unlike the
   lever/bridge/door/hatch chain.
5. **Surface compound + archer deck [OVERSEER-DEMONSTRATED
   2026-07-19]:** a walled surface building completely capping the
   stair-top (door-defensible), with a floored, parapeted deck built
   one level above it as an elevated firing position over the
   approach. The STRUCTURE is buildable today (walls, floor-over,
   door); what gates its full function is military tooling (no squads
   yet) and fortification-carving (no carve-fortification tool exists
   — check before promising arrow slits). It is the fighting-answer
   counterpart to the civilian-alert hole-up doctrine below.
6. **Barbican/moat.** Far future; no current tooling story.

**Shared upstream for stages 3-4**: every lever, plate, trap, and linked
mechanism consumes mechanism items, so the mechanic's workshop and a
standing mechanism stock are the real gate on the whole upper ladder —
verify the stock exists before promising a staging jump. The full
crafting + linking chain (mechanisms → `build` → `link_building` →
`pull_lever`) is live-verified from a prior fort.

**The intended end-state design** (this is now buildable end-to-end, not
just diggable-ahead): an aquifer-pierced stair (see `aquifer-piercing`)
leads into a wide underground highway that crosses a PIT, bridged,
before reaching the fort's TRUE spine — the surface path and the true
spine are deliberately kept separate. The underground route is
inherently resistant to climbers and flyers since it has no exposed
surface approach. The earthworks (the highway, the pit, the bridge
emplacement) are pure digging — designate_dig / channel — and the seal
MECHANISM itself (the bridge plus its controlling lever) is real,
shipped tooling now too: dig the earthworks under the dig-ahead
doctrine above, then build and link the bridge/lever in the same way
stage 4 describes.

**Interim seal policy [OVERSEER-CONFIRMED 2026-07-15]:** default is an
undug solid plug at the choke point — leave the last tile or two of the
connecting corridor unopened, and use a miner-on-demand as the "lever":
mine it open when passage is needed, wall it shut again (queued
`build`) when a threat is sensed. The bridge/lever mechanism chain in
stage 4 is now a real alternative to this manual plug where a fort
wants a standing, dwarf-operated seal instead — revisit this default
with the overseer if that tradeoff becomes worth making.

**Civilian-alert shelter-in-place** (orthogonal to the ladder above,
usable at any stage): `designate_burrow` + `set_alert` give a real
emergency-shelter tool independent of the door/hatch/bridge stages —
sounding the alert against a named burrow rushes every non-military
citizen there and confines them for its duration, with no
`assign_burrow` needed first. Clear the alert as soon as the danger
passes: leaving it active keeps the whole citizenry confined, and they
can grow unhappy or starve while shut in.

**Topology [OVERSEER-CONFIRMED 2026-07-15]:** the confirmed choice is
hybrid — loop layouts (multiple connecting corridors) for
industry/stockpile clusters so hauling traffic isn't funneled through one
corridor, dead-end isolation for housing/noble clusters so a breach
doesn't cascade through the whole living-quarters population.

**Dig-ahead horizon [OVERSEER-CONFIRMED 2026-07-15]:**
the confirmed doctrine is to rough-dig the NEXT level's footprint while idle-miner
smoothing trails one level behind the excavation front on the CURRENT
level — excavation always stays one level ahead of polish.

## Growth staging ladders (beyond defense)

The defense ladder's pattern — stages keyed to observable fort state,
tooling status named honestly — generalizes to the rest of the build
surface. The `build` tool no longer says WHEN any of its vocabulary is
worth reaching for; that judgment lives here. Three ladders follow:
industry, noble/wealth response, and water & power.

Two rules shared by all three:

- **`building_types` is the discovery authority, not this skill.** It
  lists what is actually placeable today, per-name footprints, and
  required materials/prerequisites (filter by category — workshop,
  furnace, furniture, infrastructure...). Never work from a remembered
  building list, including any list this file might seem to imply.
- **Verification status (dated 2026-07-18)**: the expanded build surface
  (new workshop/furnace/furniture/trap/water-power families) and the new
  queries backing these ladders (`moods`, `wellbeing`, `noble_demands`,
  `fort_wealth`, `zone_value`, `building_types`) are compile-verified
  only, not yet live-tested against a running fort. Treat the FIRST use
  of each family in a live session as its own verification act — place
  one, `look`/`buildings` it, read the ACK skeptically — before staging
  a plan on it. Retire this caveat once a session has verified them.

### Industry staging

There is no magic population number for "time to diversify." Gate the
decision on three observables read together, not any one alone:

1. **Survival predicates green** via `check_goals` — shelter and food
   goals holding, not merely touched once.
2. **A drink/food surplus that has survived a season change** via
   `stocks` — a single-day snapshot lies, because harvest and brewing
   cycles swing stocks; the surplus that matters is the one still there
   after a season boundary.
3. **Visible idle labor** via the `dwarves` verbose census (it renders
   each dwarf's current job; idle dwarves show as idle) — population
   alone doesn't mean spare hands, since a big fort can be fully busy
   hauling, and idle labor WITH a food deficit means the fort should
   farm, not weave.

Why all three: diversifying spends labor and raw goods that survival was
using. Any single signal firing alone is how a fort ends up with a silk
industry and no food.

**Demand-pull beats supply-push** — this project's data-not-rules stance
applied to industry. The demand signals are now directly queryable:

- `mandates` — a production mandate naming an item class the fort cannot
  make is a direct, named instruction to open that chain.
- `moods` — a strange mood reports its claimed workshop (or none) and
  needed materials, wildcards included; an unclaimable mood or an "any
  cloth"-style wildcard the fort can't source names the missing industry
  exactly. The mood timeout starts at 50000 ticks, long relative to
  placing a workshop, so building the missing shop reactively is a real
  strategy, not a panic response.

(Caravans are deliberately NOT on this list: a depot is buildable, but
no trade-interaction tool exists yet, so "goods for the caravan" can't
close its loop — don't stage industry around trading until it can.)

Supply-push still exists as the secondary signal: `stocks` accumulating
a raw good with no consumer (hides, fiber, sand, gems) is the site
telling you which chain it wants to feed. Discover the chain itself with
`building_types`, `list_reactions`, and `job_types` rather than from
memory — and build the DOWNSTREAM shop only when its upstream input
actually exists in `stocks` or is one `queue_job` away. A few shops
carry bespoke non-generic reagents; `building_types` lists them
per-name, so check before placing, not after the ACK fails.

Two exceptions to "diversify at leisure":

- **Tanning is coupled to butchery, not to diversification** — raw hides
  rot. The tanner decision is part of the butchery decision, whenever
  that happens.
- **A migration wave landing with skilled crafters** (`dwarf_detail` on
  new arrivals) is a natural moment to open the industry they already
  know, even slightly ahead of the three gates above — the labor is not
  just spare, it's pre-trained.

**Magma variants**: placement does NOT validate magma access —
`building_types` says so per-name — so a magma shop built without proven
magma is a dead building with a truthful-looking ACK. Reach for them
only after `survey_site`/`cross_section` has actually shown magma.

### Noble & wealth response staging

The framing decision, made deliberately: **respond to observed demand;
do not build value ahead of need.** Three reasons:

1. `noble_demands` reports only positions actually HELD, and nothing in
   this project deliberately appoints nobles yet — early on the answer
   is an automatic leadership position at most. Furnishing for a noble
   who doesn't exist is spending against a hypothesis when the query to
   check the fact costs one call.
2. Wealth is a thermostat, not a score. Migration waves scale with fort
   wealth, uncapped — and the confirmed soft population cap (60-80)
   means deliberately pumping wealth works AGAINST standing policy.
   `fort_wealth` exists to WATCH (pacing, and organic spikes like a mood
   artifact landing), not to maximize.
3. Data-not-rules: the query tools make demand directly observable, so
   guessing ahead of them is exactly what they were built to replace.

**The response loop, cheapest lever first**, when `noble_demands` shows
a held position with room-value or furniture-count minimums, or an
active demand with a deadline:

1. `smooth` → engrave the room — raises value while consuming zero
   items, and it's already the canonical finish order (see aesthetics
   below: smooth → engrave → THEN furniture).
2. Then furniture, discovered via `building_types` filter=furniture.
   Note the real capability seams: build's `quality` tier param selects
   existing stock by quality for the classic furniture set only —
   several of the newer room-value pieces don't support quality
   selection yet, and some place only from an item that must ALREADY
   exist in stock (a known tool-item crafting gap). `building_types`
   states which, per name; check `stocks` before promising an upgrade.
3. `zone_value` on the room to verify — it names the `noble_demands`
   room-value field it compares against, closing the observe-act-verify
   loop with no judgment left to memory.

`mandates` is a separate mechanism (export bans / production quotas) and
is answered by the industry ladder, not by furniture.

The same react-to-observation pattern extends past nobles to ordinary
citizens: `wellbeing`'s roster-wide stress and focus numbers degrading
is the observable that the services level (already in the level-category
map) deserves furnishing priority — temples, instruments, a better
dining hall — before any noble demands a thing.

### Water & power staging

**The well comes first, and earlier than the machinery.** It's a
single-tile build needing a reachable water tile below and a handful of
stocked components (`building_types` lists them; `queue_job` each). This
closes a previously-flagged gap — a prior fort's memory literally noted
"no well build type exists" — and the reason to want one over a surface
`water_source` zone is that interior water access survives a surface
threat and serves the hospital. One real dependency to plan around
rather than be surprised by: the chain component's crafting job is
currently wired at the metalsmith's forge ONLY (vanilla's cloth-rope
alternative isn't wired), so a well today sits downstream of a working
forge. Compile-verified only — the first well built live is its own
verification act.

**Cistern/tap procedure is owned by `aquifer-water-infrastructure`** —
hand off to it for turning a sealed pierce into a supply. (That skill's
own description predates the well tooling; trust `building_types` over
its "wells remain missing" line.)

**Heavy machinery — pumps, wheels, windmills, gears, axles, rollers —
is a distinct, later capability tier**, and its trigger is a concrete,
surveyed fluid problem, never a calendar stage: irrigation for
soil-less farming (per `df-farming`), drainage or flood control, or
managing an aquifer-fed cistern — all identified through
`survey_site`/`cross_section` water data before any machine is placed.
Reasons it's genuinely a bigger commitment than a bridge/lever:

- Power hookup is manual spatial planning — adjacency between machine
  parts is NOT automated by any tool; the model does the geometry.
- Axles and rollers place one tile at a time here (no multi-tile runs
  yet), costing more material than vanilla's native spans.
- The site decides the power source: windmill needs open sky, water
  wheel needs flowing water — survey first, pick second.
- A single screw pump can run dwarf-powered — the smallest real machine,
  and the right first rung before committing to a power train.

**Honest gap, checked against the code**: the milling workshops are not
placeable yet — their component items are craftable, but no placement
recipe exists (they're absent from `building_types`) — so there is
currently NO mill-driven reason to build a power train. Today's real
power consumers are pumps, and rollers whose minecart context is itself
placement-only (`track_stop` anchors track but no track linkage or
configuration tooling exists). Check `building_types` before staging
toward any power consumer; when milling appears there, this tier gains
its classic second customer.

**Mechanism tie-in**: `gear_assembly` is a valid `link_building` target,
so a lever-driven power shutoff belongs to the same live-verified
mechanism family the defense ladder's stage 4 uses — a fort that has
built one bridge/lever already knows how to build a power cutoff.

## Aesthetics that survive function

- **The terrain picks the design language [OVERSEER-DEMONSTRATED
  2026-07-19]:** on a dramatic site (canyon, brook, waterfall), shape
  the fort TO the feature — walls tracking a gorge edge, social halls
  fronting the view, a mist room harvesting a waterfall (dwarves love
  mist). On a flat featureless site, go pure geometry — quadrant or
  bilateral symmetry radiating from the spine. Don't impose either
  language on the other kind of site; read the site first.
- **Keep the pretty rock:** ore-vein pillars left standing as
  colonnades, and vein stone incorporated into room walls, are free
  architecture — weigh a vein's decorative value in place against its
  ore value mined out before stripping it. (Both uses are real; the
  mistake is only ever seeing one.)
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
- **Smoothing is an idle-labor luxury, not a default state
  [OVERSEER-CONFIRMED 2026-07-19]** — it's labor-intensive for little
  return unless many dwarves are genuinely idle. Smooth deliberately:
  value targets (noble rooms, per the response loop above) and
  idle-labor absorption when the census actually shows idle hands —
  never as a floor's automatic finishing pass. It needs no pick (any
  dwarf can do it), which is what makes it the right idle-absorber when
  that condition holds. Engraving is selective on top of that — a taste
  call per room. HARD SAFETY RULE: never include carved stairs or ramps
  in a smooth rect — smoothing DESTROYS them (live incident: a 2x2
  shaft half-lost). Check the rect against stair tiles before
  designating, same discipline as dig-vs-building overlap.

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
   `create_location` with `type=tavern|temple|library|guildhall|hospital`
   (all five share the same meeting-hall precursor; `guildhall`
   additionally needs a `profession`). For a tavern, follow with
   `assign_lodging` to claim adjoining bedroom zones as guest rooms (see
   `fort-opening` Gate 6 for the mechanics).
9. **`check_goals`** at every population gate — wave landing, season
   change, or whenever a section above calls for confirmation. It's the
   verification critic against live predicate evidence, not a status
   readout to skim once and forget.
