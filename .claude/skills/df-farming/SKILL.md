---
name: df-farming
description: Use when food/drink pressure is building on a fort and farm plots feel like the answer, or when reaching for a crop-growing tool — a real farm-plot BUILD/ASSIGN path does not exist yet (verify before promising it), but plant-gathering, a Still workshop, and direct reaction-based brewing via queue_job are real today and should be reached for instead.
---

# DF Farming

Farm-plot agriculture (dig soil room -> build plot -> assign crop per
season -> harvest) is **not fully buildable today**. Some of the supporting
tooling has landed since this skill was first written, but the two calls
that actually make a plot exist and grow something — `build_farm_plot` and
`assign_crop` — are still missing. Re-verify every claim below against the
live tool list and the code before trusting it; this file records what was
true as of the last check, not a promise about tomorrow's build.

## Tooling reality check — read this before anything else

Verified against `internal/mcpserver` and `dfhack-plugin` source, not
inferred from the tool list alone (a registered Go tool can still call a
plugin query that was never implemented — see `list_crops` below):

- **No farm-plot build type exists.** `internal/protocol/message.go` has
  no `BuildType` constant for a farm plot, `internal/mcpserver`'s `build`
  tool has no `farmplot` entry in its type vocabulary, and
  `internal/mcpserver/lenses.go` explicitly comments the case as reserved
  because it "cannot exist in a fort today." There is no `build_farm_plot`
  or `assign_crop` tool anywhere. Nothing below about plots is executable.
- **`list_crops` is registered but non-functional.** The Go tool exists in
  `internal/mcpserver/tools_state.go` and its description already
  references `build_farm_plot`/`assign_crop` in anticipation of them — but
  the DFHack plugin's query dispatcher (`dfhack-plugin/queries.cpp`,
  `executeQuery`) has no `list_crops` branch. Calling it returns
  `QUERY_STATUS_UNKNOWN` / "unknown query name: list_crops" every time.
  Don't trust its presence in a tool list as evidence it works — call it
  once and read the ACK before relying on it for crop discovery.
- **`list_reactions` is real and works.** The plugin dispatcher does have
  a `list_reactions` branch (`handleListReactions`), so this one is
  genuinely usable today for discovering raw reaction codes.
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

Bottom line: today's real food/drink chain is **gather -> Still -> queue_job
reaction** (wild plants in, drink out, no plot, no crop assignment, no
season logic). Everything from Siting onward below is the plot-based chain
this fort *cannot* run yet — kept here as a design record so implementing
it later doesn't require re-deriving these traps, and so a live session
reaches for `gather`/`still`/`queue_job` instead of a tool that doesn't
exist.

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

Given the tooling gap above, the response to all of these today is the
real chain (gather -> still -> queue_job) and/or `fort-opening` Gate 5's
fallback (hunt, trade, a `water_source` zone), **not** standing up a farm
plot — that path isn't buildable yet no matter how much pressure exists.

## Siting (design-only — no plot to site yet)

These are permanent DF mechanics that will still be true whenever
`build_farm_plot` lands, so they're worth internalizing now even though
nothing here is actionable today:

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
  layers) are the existing, working tools that would inform this siting
  call once there's something to site.

## Build (the actual gap)

There is no `build_farm_plot` tool and no farm-plot `BuildType` on the
wire. This step is not partially working or slow to discover — it is
absent end-to-end. Do not attempt to reach it through the generic `build`
tool's `type` parameter; `farmplot` is not in its accepted vocabulary and
the call will fail with "unknown build type."

What **is** buildable today and belongs in the same food/drink chain:

- A **Still** (`build` type `still`) — the real target for
  `queue_job`'s `reaction=BREW_DRINK_FROM_PLANT` path.
- A **Farmer's Workshop** (`build` type `farmer`) — DF's secondary plant
  processing (milling etc.); useful once there's plant stock to process,
  independent of whether a farm plot exists.

Build either with the standard workshop rule: 3x3 footprint, center at
(x,y), the surrounding 8 tiles must be clear floor.

## Assign (design-only — the silent-failure trap to remember)

No `assign_crop` tool exists, so nothing in this section is executable
today. It's recorded because it is DF's single most common and most
silent farm-plot mistake, and whoever eventually wires up the tool needs
the workflow ordered around it from day one:

A farm plot needs a crop assigned **per season, every season**. There is
no fallback and no default. An unassigned season is fallow — silently,
permanently for that season, every year — with no error raised anywhere.
Whatever tool eventually exposes crop assignment, the workflow must check
assignment status before checking labor, seeds, or ground, because this
failure mode produces no alert to point at it.

The intended order of operations, once buildable:

1. Confirm seeds of the intended crop are on hand (`stocks` with a
   category filter like `seed`) before assigning — assigning a crop with
   no seed stock just produces planting-job cancellation spam later.
2. Confirm the plot sits on valid ground for that crop (Siting, above).
3. Assign a crop for **every** season the plot will be active. A
   deliberately fallow season (chosen because every candidate crop is
   seed-starved that season) is a decision; an unassigned season left by
   accident is a bug nobody noticed.
4. Confirm grower labor (`set_labor` `plant`) is enabled on someone so a
   dwarf actually walks over and plants.

`list_crops` is the tool meant to inform step 1 and step 3 (crop token,
underground/surface eligibility, seeds on hand) — but per the tooling
reality check above, calling it today returns "unknown query name," so
this discovery step is currently blocked too, not just the build/assign
steps downstream of it.

Which specific crop(s) best fit a fort's seed stock and goals is a call
for the playing session, not a default to bake into this skill — season
coverage, raw-edibility, and brewability are the criteria that matter.

## Seed-loop management

Split this into what's real today and what's still design-only:

**Real today**, independent of farm plots:
- `gather` harvests wild plants directly off the map — this is the actual
  plant-supply step right now, not a fallback.
- `queue_job` with `reaction=BREW_DRINK_FROM_PLANT` at a built Still
  consumes gathered plants and produces drink. This is a genuine working
  sink for wild-gathered plant stock and doesn't need a plot, crop
  assignment, or seeds to function — it just needs plants and an empty
  barrel on hand.
- `set_labor` `brew` enables the labor that actually walks a dwarf to the
  Still to do the job.

**Design-only** (still needs `build_farm_plot`/`assign_crop` to matter):
- Seeds return from eating a crop raw, from brewing (always), from
  milling, and from Farmer's Workshop processing.
- Seeds are **destroyed** by cooking — a kitchen's cook labor consumes
  both seeds and whole plants when preparing meals. This is the single
  biggest seed-economy risk once a kitchen exists.
- The standard mitigation is kitchen permissions forbidding cooking of
  seeds, brewable/plantable plants, and alcohol, set the moment a kitchen
  is built. **No tool exists anywhere in this project to set kitchen
  permissions.** Until one ships, the only available mitigation is to
  avoid building a kitchen (or avoid queuing cook jobs) while a
  seed-dependent loop would otherwise be establishing itself — and since
  there's no farm-plot loop to protect yet anyway, this mainly matters as
  a warning for the day plots do exist.
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

For the plot-based chain, once it exists (design-only order, most-common
and most-silent first):
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

Re-grep `internal/mcpserver` for `build_farm_plot`/`assign_crop` and
`dfhack-plugin/queries.cpp` for a `list_crops` branch before treating any
part of the plot-based chain as executable — and re-test `queue_job`
`reaction=BREW_DRINK_FROM_PLANT` and `list_reactions` at the start of a
session rather than assuming last session's working state still holds;
tool wiring changes fast in this project.
