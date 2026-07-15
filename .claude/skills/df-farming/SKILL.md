---
name: df-farming
description: PLANNED — blocked on tooling. Use when planning how a fort would eventually get a farm-plot-based food/drink supply chain running, or when tempted to reach for build_farm_plot/assign_crop/list_crops/list_reactions — none of those exist yet; read this first so a live session doesn't promise farming instead of falling back to fort-opening Gate 5 (gather/hunt/water-zone).
---

# DF Farming (PLANNED)

This skill is aspirational. There is no farm-plot build type anywhere in this
project today, so nothing below is executable in a live session. It exists to
record the shape of the work — and the traps that will bite the moment the
tooling lands — so a future session doesn't have to re-derive them, and so a
*current* session doesn't accidentally tell the player a farm plot is one
tool call away. For what a fresh fort can actually do about food/drink right
now, see `fort-opening` Gate 5 (gather, hunt, trade, and a `water_source`
zone over open water).

## Tooling reality check — read this before anything else

- **No farm-plot build type exists.** `internal/protocol/message.go` has no
  `BuildType` entry for a farm plot, and `internal/mcpserver/lenses.go`
  explicitly comments the farm-plot case as reserved because it "cannot
  exist in a fort today." There is no `build_farm_plot`, `assign_crop`,
  `list_crops`, or `list_reactions` tool registered anywhere in
  `internal/mcpserver` — grep before trusting this stale, but as of the last
  read the whole chain is missing end-to-end, not just one link.
- **Brewing is in the same boat.** The protocol defines an
  `OrderTypeCustomReaction` sentinel meant to reach raw-defined reactions
  like `BREW_DRINK_FROM_PLANT` that have no `job_type` mapping at all, but
  no `queue_job` tool wiring in `internal/mcpserver` uses it yet. Don't tell
  a live session it can queue brewing today — this matches `fort-opening`
  Gate 5's caveat that brewing "may not be queueable at all yet."
- Until `build_farm_plot` (or equivalent) and reaction-routing both land,
  treat this entire skill as a design doc, not a runbook.

## The shape of the work, once tooling exists

### Siting

Plot ground has permanent, non-negotiable constraints that will still be
true whenever this becomes buildable:

- Valid ground is soil floor, or stone floor that has been muddied. Bare
  stone should refuse the building outright.
- Underground vs. aboveground is a **permanent per-tile attribute** in DF.
  A tile that has ever been sun-touched will never grow underground crops
  again, even if walled off afterward. This needs checking before digging
  the room, not after a plot fails.
- A single plot should never span the underground/aboveground boundary — a
  plot partially sun-touched and partially not will never fully plant, and
  will fail silently the same way an unassigned season does (see below), so
  it will look identical to other failures until inspected tile-by-tile.
- The tile must already be dug out / revealed before building on it — a
  standard `designate_dig` prerequisite, not farming-specific.
- Prefer several small plots (one crop each) over one large plot early on.
  Seeds are the bottleneck resource at fort start regardless of plot size,
  so a big plot just multiplies how many seeds are needed before any
  return is seen.
- `cross_section` (soil vs. stone) and `find_dig_site` (candidate soil
  layers) are the existing tools that would inform this siting decision —
  those two already exist and aren't blocked.

### The silent-failure trap to design around

DF gives *zero error* for the most common farm-plot mistake: an unassigned
season. A farm plot needs a crop assigned **per season, every season**.
There is no fallback, no default. A season left unassigned is fallow —
silently, permanently for that season, every year — until reassigned.
Whatever tool eventually exposes crop assignment, the workflow around it
must check assignment before checking labor, seeds, or ground, because this
failure mode produces no alert to point at it.

Order of operations this implies, once the tools exist:

1. Confirm seeds of the intended crop are on hand (`stocks`, SEEDS/PLANT
   categories) before assigning — assigning a crop with no seed stock would
   just produce cancellation spam once growers try to plant it.
2. Confirm the plot sits on valid ground for that crop (see Siting above).
3. Assign a crop for **every** season the plot will be active. A
   deliberately fallow season (chosen because every available crop is
   seed-starved that season) is a choice; an unassigned season is a bug
   nobody noticed.
4. Confirm grower labor is enabled on someone so a dwarf actually walks
   over and plants.

When picking which crop(s) to run, the relevant criteria are season
coverage (does it grow year-round or only part of the year), whether it's
edible raw, and whether it's brewable — but which specific crop best fits
a given fort's seed stock and goals is a call for the playing session to
make, not a default this skill should bake in.

### Seed-loop management

Seeds are a circulating resource, not a one-time purchase — whatever
tooling lands should track what returns them and what destroys them:

- **Returns seeds**: eating the crop raw, brewing (always returns seed),
  milling, and farmer's-workshop processing.
- **Destroys seeds**: cooking. A kitchen's cook labor consumes both seeds
  and whole plants when preparing meals — this would be the single biggest
  seed-economy risk once a kitchen exists.
- The standard protection is kitchen permissions that forbid cooking
  seeds, brewable/plantable plants, and alcohol, set the moment a kitchen
  is built. **No tool exists in this MCP server to set kitchen permissions
  either.** Until both a farm-plot tool and a kitchen-permissions tool
  ship, the only mitigation available to a live session is to avoid
  building a kitchen (or avoid queuing cooking jobs) while a seed-dependent
  loop would otherwise be establishing itself.
- Seed stock has hard caps (200 per type, 3000 global) in DF — once
  established, don't expect seed stock to grow without bound even with
  brewing running well; caps are a ceiling, not a failure.
- Routing surplus plants through brewing would both produce drink and keep
  the seed-return loop running — but per the tooling reality check above,
  there is currently no `queue_job` path to a brewing reaction at all, so
  this step is design-only until reaction-routing lands. Don't route plants
  through `queue_job` expecting a brewing reaction to be reachable today.

### Diagnostics checklist (for when a plot exists and isn't producing)

Once a plot can actually be built, check causes in this order — most-common
and most-silent first:

1. **No crop assigned for the current season.** Silent. Check this first,
   always.
2. **No seeds of the assigned crop on hand.** Would show up as
   planting-job cancellation spam in alerts, not as a plot-level error —
   and is often caused by a kitchen having cooked the seed stock away.
3. **No grower labor enabled** on any available dwarf.
4. **Bad ground**: unmuddied stone, or a sun-touched tile carrying an
   underground crop assignment. Both are permanent for that tile — the fix
   is a new plot on new ground, not a repair.
5. **Mixed above/below-ground plot** — partially sun-touched, so it can
   never fully plant regardless of crop or season assignment.
6. **Seeds unreachable or forbidden** — check stockpile/hauling access, not
   just raw stocks counts.
7. **Season-illegal crop** — a crop assigned to a season it can't grow in
   would silently fail exactly like an unassigned season.

Whatever query tool eventually surfaces plot state, expect the same gap
`buildings` has today for everything else: it can confirm a building
exists without surfacing per-season crop assignments. Track season
assignments in fortress memory until a tool exposes that state directly.

## Re-check before relying on this skill

Re-grep `internal/protocol/message.go` for a farm-plot `BuildType` and
`internal/mcpserver` for `build_farm_plot`/`assign_crop`/`list_crops`/
`list_reactions` before treating any part of this as executable — new
tooling landing is exactly the kind of change that moves this skill from
PLANNED to real.
