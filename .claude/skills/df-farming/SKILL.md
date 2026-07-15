---
name: df-farming
description: Use when food/drink pressure is building on a fort and farm plots are the answer — covers the full plot chain (site -> build_farm_plot -> assign_crop per season -> seed loop) plus the plotless gather/Still/brew chain; the plot tools shipped 2026-07-14 compile-verified only, so live-verify each with one call before leaning on it.
---

# DF Farming

Farm-plot agriculture (dig soil room -> build plot -> assign crop per
season -> harvest) is buildable end-to-end as of 2026-07-14:
`build_farm_plot`, `assign_crop`, and `list_crops` all exist through the
full stack (Go tool -> wire -> plugin handler). **They are compile-verified
only — no farm plot has ever been built in a live game through these tools.**
First use in a session: make one cheap discovery call (`list_crops`) and
read the ACK before building the room; if it errors "unknown query name,"
the running plugin DLL predates the farm tools and needs a redeploy (DF
closed) before any of the plot chain works.

## Tooling reality check — read this before anything else

Status as of 2026-07-14 (verified against `internal/mcpserver` and
`dfhack-plugin` source at ship time; re-verify live per the note above):

- **`build_farm_plot` exists** — extent-shaped like `stockpile`
  (`x1,y1,x2,y2,z`), consumes no materials, needs no architect. NOT part of
  the generic `build` tool's vocabulary; it is its own tool.
- **`assign_crop` exists** — per-season crop assignment on a built plot
  (`x,y,z` of the plot, `season` spring/summer/autumn/winter/all, `crop`
  by raw token or display name, or the literal `fallow`). Season-legality
  and underground/surface checks mirror DFHack's autofarm and error
  truthfully.
- **`list_crops` exists** — plantable crops with raw token, display name,
  underground/surface eligibility, and seeds on hand; substring-filterable.
- **`list_reactions` exists and works** — discovery for raw reaction codes
  (e.g. `BREW_DRINK_FROM_PLANT`).
- **Brewing has a real, working sink — but only through `queue_job`, not
  `order`.** `queue_job`'s `reaction` parameter (e.g.
  `BREW_DRINK_FROM_PLANT`, discovered via `list_reactions`) routes to
  `OrderTypeCustomReaction`, which `dfhack-plugin/work_orders.cpp`
  implements for real: it builds the job's reagents from the raw reaction
  definition and attaches it directly to an existing workshop. This works
  with **no farm plot involved** — it consumes whatever plant stock is on
  hand (including gathered wild plants) at a **Still** workshop
  (`build` type `still`). The `order` tool's `drink` item, by contrast,
  is a dead end: `work_orders.cpp` maps `OrderTypeBrewDrink` to job_type
  `-1` with an explicit comment that it "needs reagent-based reaction
  lookup" — it is wired to fail, not just unimplemented quietly. Use
  `queue_job` with `reaction`, never `order` with `drink`.
- **No kitchen-permissions tool exists.** Nothing in `internal/mcpserver`
  or `dfhack-plugin` sets cook-labor restrictions. The seed-destruction
  risk described below has no mitigation tool yet.
- **Grower and brewer labor toggles work today** (`set_labor` recognizes
  `plant` and `brew`), but `plant` labor has nothing to act on without a
  farm plot to plant into.

Two chains, use both: the **plotless chain** (gather -> Still -> queue_job
reaction: wild plants in, drink out, no seeds needed) is the proven bridge
while a farm establishes; the **plot chain** (everything from Siting onward)
is the sustainable engine. Neither replaces the other early on — gathered
plants also brew, and brewing always returns seeds, which is how a seed
stock bootstraps.

## When to farm (food pressure signals)

Signals that food/drink is becoming a real constraint, in the order they
tend to appear:

1. `stocks` with a category filter like `food` or `drink` trending down
   across successive checks, not just a single low reading (embark stock
   is meant to run down before anything replaces it).
2. `alerts` surfacing cancelled eat/drink jobs, or dwarves with the
   hungry/thirsty status in `dwarf_detail`.
3. Population growth (`dwarves` count climbing from migrants) outpacing
   the fort's food-producing capacity, which at this point in tooling
   means outpacing `gather` yield and Still throughput.

Respond with both chains: keep gather -> Still -> queue_job flowing for
immediate drink (plus `fort-opening` Gate 5's fallbacks — hunt, trade, a
`water_source` zone), and stand up the plot chain below for the sustainable
supply.

## Siting

Permanent DF mechanics — check these before digging the room, not after a
plot fails:

- Valid ground is soil floor, or stone floor that has been muddied. Bare
  stone refuses the building outright.
- Underground vs. aboveground is a **permanent per-tile attribute**. A
  tile that has ever been sun-touched will never grow underground crops
  again, even if walled off afterward — this has to be checked before
  digging the room, not after a plot fails.
- A plot must never span the underground/aboveground boundary. A plot
  half sun-touched and half not will never fully plant, and fails exactly
  as silently as an unassigned season (see Assign, below) — indistinguishable
  from other failures until inspected tile-by-tile.
- The tile must already be dug out / revealed first — a standard
  `designate_dig` prerequisite, not farming-specific.
- Prefer several small single-crop plots over one large plot early on.
  Seeds are the bottleneck at fort start regardless of plot size, so a big
  plot just multiplies how many seeds are needed before any return is seen.
- `cross_section` (soil vs. stone) and `find_dig_site` (candidate soil
  layers) are the tools that inform this siting call.

## Build

`build_farm_plot` takes the plot rectangle (`x1,y1,x2,y2,z`) — it is its
own tool, NOT a `type` under the generic `build` tool. Zero materials,
no architect; a dwarf with farming labor constructs it. DF itself enforces
the ground rules (soil / muddied stone) — a plot on bad ground fails with
DF's real reason in the ACK, so don't pre-filter beyond the Siting notes
above. A built plot that is never crop-assigned grows NOTHING, silently —
go straight from build to Assign, below.

Companion workshops in the same food/drink chain:

- A **Still** (`build` type `still`) — the real target for
  `queue_job`'s `reaction=BREW_DRINK_FROM_PLANT` path.
- A **Farmer's Workshop** (`build` type `farmer`) — DF's secondary plant
  processing (milling etc.); useful once there's plant stock to process,
  independent of whether a farm plot exists.

Build either with the standard workshop rule: 3x3 footprint, center at
(x,y), the surrounding 8 tiles must be clear floor.

## Assign (the silent-failure trap)

A farm plot needs a crop assigned **per season, every season** — via
`assign_crop`. There is no fallback and no default. An unassigned season
is fallow — silently, permanently for that season, every year — with no
error raised anywhere. This is DF's single most common and most silent
farm mistake: check assignment status before checking labor, seeds, or
ground, because this failure mode produces no alert to point at it.

Order of operations:

1. Confirm seeds of the intended crop are on hand (`stocks` with a
   category filter like `seed`) before assigning — assigning a crop with
   no seed stock just produces planting-job cancellation spam later.
2. Confirm the plot sits on valid ground for that crop (Siting, above).
3. Assign a crop for **every** season the plot will be active
   (`assign_crop` with `season=all` writes one crop into all four slots in
   one call; per-season calls override individually; `crop=fallow` clears
   a slot). A deliberately fallow season (chosen because every candidate
   crop is seed-starved that season) is a decision; an unassigned season
   left by accident is a bug nobody noticed.
4. Confirm grower labor (`set_labor` `plant`) is enabled on someone so a
   dwarf actually walks over and plants.

`list_crops` informs step 1 and step 3 (crop token, underground/surface
eligibility, seeds on hand) — call it first; it is also the cheapest live
probe that the running plugin actually has the farm handlers (see the
reality check at the top).

Which specific crop(s) best fit a fort's seed stock and goals is a call
for the playing session, not a default to bake into this skill — season
coverage, raw-edibility, and brewability are the criteria that matter.

## Seed-loop management

The plotless supply line, independent of farm plots:
- `gather` harvests wild plants directly off the map.
- `queue_job` with `reaction=BREW_DRINK_FROM_PLANT` at a built Still
  consumes plants (wild-gathered or farmed) and produces drink; needs
  plants and an empty barrel on hand, nothing else.
- `set_labor` `brew` enables the labor that actually walks a dwarf to the
  Still to do the job.

The seed loop itself:
- Seeds return from eating a crop raw, from brewing (always), from
  milling, and from Farmer's Workshop processing. Brewing gathered wild
  plants is how a seed stock bootstraps before the first harvest.
- Seeds are **destroyed** by cooking — a kitchen's cook labor consumes
  both seeds and whole plants when preparing meals. This is the single
  biggest seed-economy risk once a kitchen exists.
- The standard mitigation is kitchen permissions forbidding cooking of
  seeds, brewable/plantable plants, and alcohol, set the moment a kitchen
  is built. **No tool exists anywhere in this project to set kitchen
  permissions.** Until one ships, the only available mitigation is to
  avoid building a kitchen (or avoid queuing cook jobs) while the
  seed loop is establishing itself. This is a live constraint now that
  plots are buildable — a kitchen can silently eat the farm's future.
- DF caps seed stock (200 per type, 3000 global) — a ceiling to expect,
  not a failure, once a loop is running.

## Diagnostics checklist

For the real chain (gather -> still -> queue_job), check in this order
when drink isn't appearing:
1. `alerts` for cancelled brew jobs — usually missing plant stock or no
   empty barrel, not a labor problem.
2. `set_labor` `brew` actually enabled on an available dwarf.
3. `stocks` for plant stock on hand at all (nothing to gather nearby, or
   `gather` never queued/completed) and for empty barrel availability.
4. `jobs` to confirm the reaction job is actually queued at the Still and
   not silently dropped (queue full, wrong workshop id).

For the plot chain (most-common and most-silent first):
1. **No crop assigned for the current season.** Silent — check first,
   always.
2. **No seeds of the assigned crop on hand.** Shows up as planting-job
   cancellation spam in `alerts`, not a plot-level error — often caused by
   a kitchen having cooked the seed stock away.
3. **No grower labor enabled** on any available dwarf.
4. **Bad ground**: unmuddied stone, or a sun-touched tile carrying an
   underground crop assignment. Both are permanent for that tile — the
   fix is a new plot on new ground, not a repair.
5. **Mixed above/below-ground plot** — partially sun-touched, so it can
   never fully plant regardless of crop or season assignment.
6. **Seeds unreachable or forbidden** — check stockpile/hauling access,
   not just raw `stocks` counts.
7. **Season-illegal crop** — assigned to a season it can't grow in; fails
   exactly like an unassigned season.

`buildings` would confirm a plot exists as a building but has no visibility
into per-season crop assignments — expect to track those in fortress
memory until a query tool exposes that state directly, the same gap it
has for every other building type today.

## Re-check before relying on this skill

None of the plot chain has been exercised against a live game yet
(shipped 2026-07-14, compile-verified). At first live use, treat every
step as an experiment: read ACKs skeptically, verify the plot appears in
`buildings`, and record what actually happens in fortress memory — then
update this skill's claims from PLAUSIBLE to VERIFIED (or file the bug).
A cheap `list_crops` call at session start is the canonical probe that
the running DLL has the farm handlers at all.
