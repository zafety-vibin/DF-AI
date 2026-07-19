---
name: fort-opening
description: Use when starting the first session on a fresh embark, or resuming one still in its opening hours (no aquifer sealed yet, first rooms not yet dug) — an ordered gate checklist from initial orientation through the first dig, first rooms, aquifer handling, and early economy so nothing foundational gets skipped or attempted out of order.
---

# Fort Opening — First-Hours Checklist

Follow the turn protocol in `fortress/CLAUDE.md` (pause → observe → act → step)
for every action below; this skill only orders WHAT to do in the opening
hours, not the mechanics of taking a turn.

The gates below are sequenced by dependency, not by clock time — Gate 5
(early economy) runs in parallel with Gates 2-4, it doesn't wait for them.
Skip a gate only if its precondition doesn't apply (e.g. no aquifer flagged),
and say so in the journal when you do.

## Gate 0 — Orient

Before any designation: `status`, `survey_site`, `alerts`.

Read `survey_site`'s aquifer callout and stone-depth figures, and corroborate
with `cross_section` bores at your candidate embark spot before committing to
a shaft location. This is the gate that decides two things you can't recover
from cheaply later: how deep the opening shaft budget needs to be, and
whether the aquifer-piercing skill will be needed at all this fort.

## Gate 1 — Census gate (before the first dig designation)

Pull the full roster: `dwarves`, then `dwarf_detail` on each id.

Confirmed on 2/2 forts: a default embark ships with exactly ONE dwarf
carrying the MINE labor, among a roster where everyone else has roughly
~80 labors each except mining. `set_labor` MINE onto a second dwarf (a
stone-skill type — masonry, mining-adjacent skills — is a natural pick)
BEFORE queuing the first dig designation. Done pre-emptively this is a
one-line fix; done reactively it's a multi-day diagnosis of "why isn't
this designation digging."

`dwarves`/`dwarf_detail` rosters include pack animals and pets, not just
citizens — tell them apart by `first_name` + dwarf-typical labor skills
(animals show blank names and at most a trained skill like CLIMBING). If
population-derived numbers (bed counts, food math, `check_goals` shelter
predicates) look implausible, sample `dwarf_detail` across the full id
list before trusting the count.

## Gate 2 — The shaft

Designate a 2×2 stairs column (charter default) from the surface downward
in ONE `designate_dig` call spanning many z-levels. Stop the initial
designation one level ABOVE any aquifer layer identified in Gate 0 rather
than digging blind through it — piercing it is Gate 4's job, not this
gate's. The wagon occupies and blocks its own tile for digs; route the
shaft around it rather than expecting that tile to dig.

**Dig-verb primer** (pick the right tool, don't default to `mine` for
everything):
- `mine` — sideways/column dig at the dwarf's own level. The default choice
  for corridors and rooms.
- `channel` — drops a tile from the level above. Prefer it over `mine` when
  there's standing water below, since it leaves a ramp rather than opening
  a pit straight into a flooded space.
- `stairs` — the only verb that connects z-levels vertically.

**Stair-continuation rule**: a `stairs` designation always treats its OWN
top z as a brand-new shaft start (down-only, no up-connection). Continuing
an existing shaft deeper must START the new designation AT an
already-carved stair tile — that tile no-ops, and the first undug level
below it becomes a proper "middle" carved with both up and down. Starting
the continuation instead at the undug level directly below existing stairs
strands it: unreachable, no up side. If you're already stuck with a dead
down-stair top, don't patch it — dig a side corridor at the deepest
reachable stair level and sink a fresh 2×2 shaft beside it.

## Gate 3 — First rooms

Branch rooms BESIDE the shaft on the first soil level, never on top of it.

Before stepping away from a freshly designated room, `look` at the
BOUNDARY tile between the new designation and existing carved floor, not
just the new room's interior. Two independent live mistakes trace to the
same gap: `look`'s designation overlay under-reports (a tile with an
in-flight dig job can render as plain wall with no 'd' glyph — that is
NOT necessarily a cancel), and there is no spatial gap-overlay yet, so a
room floated a few tiles off the shaft with no connecting corridor will
sit at zero visible progress for a full step budget before the gap gets
noticed. Checking the boundary tile catches this before you burn a step
on it.

`find_dig_site` is reliable again for suggesting candidate footprints
(a prior staleness bug is fixed), but still worth a `look` pass before
trusting a "fully solid" candidate near a room you finished earlier in the
SAME session.

## Gate 4 — Aquifer (only if Gate 0 flagged one)

Do not free-hand an aquifer pierce here — invoke the **aquifer-piercing**
skill once the shaft reaches one level above the flagged layer.

One ordering rule this checklist owns regardless of that skill's internal
steps: quarry stone from below the aquifer BEFORE mining the seal ring, so
wall-building can start the instant the ring opens instead of stalling on
missing boulders.

Expect silent repeats while the ring is mined: damp/aquifer warnings fire
loud exactly once, then dedup silently on every repeat cancel of the same
tiles. A designation that quietly vanishes with no fresh alert is still a
damp-cancel, not a success — re-designate the same tiles rather than
trying to diagnose why the alert didn't fire again.

## Gate 5 — Early economy (parallel, not sequential)

Runs alongside Gates 2-4:

- `chop` for logs; once a boulder or the wagon's starting logs are
  available, build a carpenter workshop and `queue_job` to queue beds
  directly at it. `order` (manager work orders) is a dead end this early —
  it needs both a Manager noble and a walled office (chair+table+door),
  and nothing currently assigns a noble or claims a room, so it will sit
  `validated=true, active=false` forever on a fresh fort. The general
  split holds beyond the opening too: reach for `queue_job` for an
  immediate one-off need, `order` once a manager exists and the need is
  standing/bulk production.
- Day-one `stocks` check on DRINK/food quantities. If thin, reach for
  `queue_job` with `reaction=BREW_DRINK_FROM_PLANT` at a built Still — this
  is the fort's one real drink chain today (see `df-farming`). The `order`
  tool's `drink` item is the dead end, not brewing itself: it maps to a
  `job_type` of -1 and never dispatches. Re-test the `queue_job` reaction
  path at session start per `df-farming`'s caveat before counting on it.
- Locate surface water (open water tile via `survey_site`/`cross_section`/
  `look`) and `designate_zone` `water_source` over it as dehydration
  insurance, independent of whether the drink chain is available.
- Building plans die SILENTLY after repeated "needs material" cancels
  (the ACK reads like "the dwarves were unable to complete the X..." once,
  then goes quiet). Once new material arrives, RE-PLACE anything that
  hasn't visibly completed — completions are silent too, so verify with
  `look` (e.g. a wall's glyph actually present) rather than assuming a
  re-placed plan finished.

## Gate 6 — Zones & locations (once beds exist)

`designate_zone` `bedroom` over built beds, then `check_goals` to confirm
shelter predicates move. These read a world-model snapshot that goes stale
immediately after a zone edit — step roughly 100 ticks before trusting a
predicate count that was just touched, rather than reading a same-turn 0
as a failure.

If a social space is wanted, `designate_zone` `meeting_hall` +
`create_location` `tavern` (any tile inside the hall), then
`assign_lodging` to claim adjoining bedroom tiles as tavern guest rooms.

Zone type names are snake_case and case-insensitive but NOT
camelCase-tolerant (`meeting_hall`/`Bedroom` work, `MeetingHall` doesn't).
`remove_zone` exists and is wired end-to-end (plugin `applyRemoveZone`) —
a zone painted in the wrong tile is recoverable, not a permanent squatter.
Still don't test-paint zones carelessly in rooms you care about, but a
mistake here is fixable with `remove_zone` rather than a reason to plan
around it.

## Cross-cutting, applies at every gate

- **Reconnect sentinel quirk**: after any `/mcp` reconnect + fresh
  `ai-connect`, `status` can report "Connected" immediately while still
  returning sentinel data (year=0, dwarves=0) for several subsequent
  queries — not reliably fixed by the very first call. Call `pause` and
  re-query; if still sentinel, just query again. Don't treat this as a
  broken connection.
- **Pacing**: shorter steps (400-800 ticks) during active water/crisis
  events; longer (1200-2400) once stable. During heavy fluid simulation a
  step can exceed the tool's poll deadline ("DID NOT complete") — resync
  with `status`/`pause` rather than re-issuing the same step.
