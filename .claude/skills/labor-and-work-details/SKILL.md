---
name: labor-and-work-details
description: Use when work isn't happening despite queued orders — an order stuck "in progress" with zero produced, one material class (wood, metal) stalling while another flows, a big smooth/engrave/haul job freezing every other industry, miners ignoring a priority dig, or tile deltas collapsing right after queueing area work. The labor layer: how labor flags, work details, detail MODES, tools-in-hand, and nearest-job dispatch decide who actually works, and the diagnostic ladder for a fort that looks busy but produces nothing.
---

# The Labor Layer — Work Details, Modes, and Who Actually Works

A dwarf works a job only when three layers line up: the labor flag is on,
any required tool is in hand, and the dispatcher hands them the job. Each
layer fails SILENTLY, and each failure is indistinguishable from a busy
fort at a glance. Two whole industries have stalled for full sessions on
these — both times the orders read "in progress" throughout.

## Layer 1 — the labor flag (a missing detail is invisible)

Ground truth (DFHack-source-verified 2026-07-15): the engine dispatches
from each unit's own labor flags. Work details are a bulk-write UI layer
over those flags — the engine never reads details directly. The practical
consequence cuts both ways:

- `set_labor` on one dwarf works immediately and needs no detail.
- But a labor that appears in NO work detail is very likely OFF for
  every citizen — nothing ever bulk-wrote it on. Some labors survive on
  profession-default flags; others don't, and which is which is
  invisible until checked.

Live incident (root-caused twice in one session): every stone item
completed normally while every wood item produced ZERO for a whole
session — no work detail contained CARPENTER. Manager orders read "in
progress" the entire time. The truthful discriminator was the carpenter
workshop's OWN job queue: empty. The same check then found MECHANIC also
absent from every detail; creating a detail pre-emptively meant that bug
never fired.

**RULE: before opening any industry — first forge job, first mechanism,
first loom — check `work_details` for that industry's labor.** If no
detail carries it, `create_work_detail` (mode only_selected) +
`assign_work_detail` the skilled dwarf BEFORE queueing work. Assume any
labor outside DF's default detail list is missing until checked. Done
pre-emptively it's two calls; done reactively it's a lost session.

Caveat (live, prior fort): creating a detail with only_selected does NOT
strip non-members' already-cached labor flags — restriction applies going
forward, it doesn't retroactively clean the roster.

## Layer 2 — detail MODE (EverybodyDoesThis captivates the fort)

Several of DF's default details ship as mode=EverybodyDoesThis — observed
set (2026-08-14): Engravers, Stonecutters, Planters, Plant gatherers,
Haulers. Designating a large area job owned by such a detail pulls EVERY
citizen off mining, crafting, and hauling until it finishes — a
background task becomes a fort-wide stop-the-world. Symptom: tile deltas
sag with a full order queue and nothing shipping (see `reading-the-fort`
for the deltas gauge).

Live-verified fix (2026-08-14): `set_work_detail_mode` the owning detail
to only_selected and assign a small dedicated crew — with the SAME orders
queued, deltas roughly doubled as the rest of the fort resumed parallel
work, and a stalled priority dig finally got its miners back.

**RULE: before queueing any large area job (smoothing, engraving, mass
hauling, bulk planting), check the owning detail's MODE.** Restrict it
to a dedicated crew first; the job runs slower in wall-clock terms and
the fort runs faster in every other dimension. Corollary (overseer
canon): don't smooth/engrave utilitarian space at all — spend finish
work where dwarves live and gather.

## Layer 3 — tools and dispatch

- **One pick = one miner** (v50, source-verified). The labor flag makes
  a dwarf willing; the tool in hand makes them able. Count effective
  miners by who actually mines, not by labor flags — and never feed the
  fort's picks to weapon traps or trade; they are the mining industry.
- **Nearest-job dispatch starves priority work.** Miners take the
  closest designated tile, so a bulk dig-ahead designation can starve a
  critical dig for game-days (live: a pierce sat untouched behind
  hundreds of queued hall tiles). Cancelling competing designations is
  a legitimate scheduling lever — `cancel_designation` is free and
  instant, and re-issuing later is also free. During single-miner
  hazard work (an active aquifer ring), queue NO other mine-type
  designation at all (see `aquifer-piercing`).

## The diagnostic ladder — when an order produces nothing

"In progress" is not evidence of progress. Check in this order; each rung
is cheaper and more common than the next:

1. **The workshop's own job queue.** Empty queue under an "in progress"
   order = no citizen can take the job. This is the honest
   discriminator; a fix wave added a "no citizen holds the required
   labor" rung to the orders ladder, but its live behavior is
   unconfirmed — keep making this check yourself.
2. **`work_details` for the required labor** (Layer 1). Missing detail →
   create + assign, and expect production within a step once staffed.
3. **Tool-in-hand** for tool-gated labors (Layer 3).
4. **Materials** — remembering `stocks` counts include built-in items
   and hide vocabulary splits (see `reading-the-fort` §2/§4).
5. **Reachability** — the workshop or its inputs walled off, forbidden,
   or behind a raised bridge.

Re-issuing the order is never on this ladder. An impossible job re-queued
is still impossible.

## Staffing doctrine

- Dedicated crews beat everybody-does: `create_work_detail`
  only_selected + a few skilled assignees per industry, created at the
  moment the industry opens, not after it stalls.
- Keep MANY details employed per step — batch designations, builds, and
  orders across fronts so the whole labor force works in parallel (see
  `fort-planning`, multiple-fronts doctrine), then step long.
- A migration wave's arrivals (`dwarf_detail` on new ids) are
  pre-trained labor — slot them into details on landing rather than
  letting cached defaults decide.

## Verification status

Layer 1's engine-reads-flags ground truth: DFHack-source-verified
2026-07-15. The carpenter/mechanic incident, the captivation incident,
and both fixes: live-verified 2026-07-25 and 2026-08-14. The
EverybodyDoesThis default set: observed once (2026-08-14) — re-check
`work_details` output rather than trusting the list. The orders-ladder
"no citizen holds the labor" rung: shipped in a fix wave, never
confirmed live.
