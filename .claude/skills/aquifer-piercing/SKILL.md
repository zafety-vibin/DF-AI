---
name: aquifer-piercing
description: Use when survey_site/cross_section/look reports an aquifer flag, or a stairs/mine designation silently vanishes with a damp/water dig-cancel, and the fort needs to descend through the wet layer to reach stone below.
---

# Aquifer Piercing

A checklist for descending a stair shaft through an aquifer layer without
flooding the fort. Verified end-to-end (unsupervised) on a single-layer sand
aquifer; treat every step below as load-bearing unless marked otherwise.

## 0. Identify before you commit

DF tags each aquifer Light / Heavy / Varied at worldgen — this is fixed and
cannot change fort-to-fort. The embark/site-selection screen shows this tag
directly — read and record it there, before you ever break ground, rather
than waiting to infer it from in-dig behavior. **Gap: once past embark, no
current tool surfaces that tag again.** `survey_site`, `cross_section`, and
`look` all expose an aquifer boolean and a water-depth number per tile
(including hidden tiles — this is one of the few sanctioned exceptions to
the fog-of-war rule), but not the Light/Heavy/Varied classification itself.
If you didn't capture the embark-screen tag (a resumed fort, a site someone
else embarked), fall back to inferring heaviness from in-dig behavior:

- **LIGHT** (the only variant this protocol is verified against): a freshly
  opened aquifer tile seeps slowly — roughly a few units of depth per
  tile per in-game month, i.e. days per tile of visible rise. The rest of
  this skill applies as written.
- **HEAVY**: seepage runs roughly an order of magnitude faster tile-for-tile
  (on the order of a full depth-step per tile within tens of ticks) — a
  freshly opened tile visibly fills to standing depth within a single short
  `step()`. If you see this, STOP piercing immediately — the
  wall-in-shallow-water step below does not work against heavy inflow (see
  §7). Do not keep digging "to see how bad it is"; each additional opened
  tile is an additional uncapped inflow source.
- Record which one you're dealing with (embark tag if you have it, inferred
  behavior otherwise) in `fortress/memory/learnings.md` once known — it's
  worldgen-fixed, so this fort's answer is also every future descent's
  answer at the same site, not a one-time note.

## 1. Shaft down to one level above the aquifer

Continue (or start) the stair shaft with `designate_dig type=stairs`,
2x2, spanning as many z-levels as needed in one call — this is the one
sanctioned fixed-dimension rule (charter-level, not an aquifer-specific
choice). Stop the designation at the level immediately above the aquifer
layer; don't let a single stairs call blindly run through the wet layer
and beyond, or you lose the ability to freeze the descent at the right
depth (§2).

## 2. Pierce, expect a silent cancel, re-designate

Designate the aquifer level itself (stairs, continuing the same shaft), and
let that same stairs call carry through 1-2 levels further, into the stone
below the aquifer — you need the shaft to actually reach that stone before
it can be quarried in §3; quarrying stone that's still sealed behind the
unpierced aquifer is not physically possible.

The first attempt will very likely warning-cancel on contact with the
aquifer — DF announces this LOUDLY exactly once, then the announcement
system dedups: a second silent cancel on the same tiles produces **no
alert at all**. Don't try to diagnose a vanished designation from alerts;
just re-designate the same tiles a second time. The second designation
digs for real.

Once the shaft has reached 1-2 levels below the aquifer, freeze the
descent there: cancel any deeper stairs designation (`cancel_designation`)
so seepage collects in a small catch basin spanning the aquifer level down
to that landing, rather than draining further into the rest of the fort.

## 3. Quarry the stone below FIRST

With the shaft now reaching stone 1-2 levels below the aquifer (§2), get a
miner onto that level and start quarrying boulders BEFORE mining the seal
ring at the aquifer level (§4). This is a lesson paid for in an earlier
fort: the seal in §5 needs boulders for constructed walls on every ring
tile, all at once, the moment the ring opens — if you wait to mine
boulders until after the ring is dug, the seal stalls for days on missing
material while the ring keeps seeping. Bank the boulders before you open
the ring.

## 4. Ring the aquifer level — orthogonal only

Aquifer seepage travels only orthogonally and straight down — never
diagonally. That means the natural diagonal corners of a ring are already
safe; you do not need to mine or wall them, and treating "corners are
optional" as a shortcut is actually just correct physics, not a risk.

Mine the orthogonal neighbor ring around the shaft at the aquifer level
(`designate_dig type=mine`). Expect this to take up to **three
designation rounds**, not one or two — and remember the same dedup rule
from §2 applies here: a ring tile that re-cancels because it's adjacent to
tiles with standing water produces no alert on repeat cancels either. Keep
re-designating any ring tile that hasn't visibly dug yet after a step;
designations are free, so there's no cost to over-re-issuing. The wetter
the shaft (deeper standing water nearby), the more rounds to expect.

## 5. Wall immediately — branch on natural wall material

The moment a ring tile is open, seal it. Which method depends on what the
natural wall is made of:

- **Sand/soil aquifer**: natural walls can't be smoothed shut. Queue
  constructed walls (`build`) on every mined ring tile using the boulders
  banked in §3. Builders will work standing in shallow water, so don't
  wait for the tile to dry — queue the build the instant the ring tile is
  mined. ALL sides of the ring must end as constructed wall; a ring with
  one unsealed face isn't sealed.
- **Stone aquifer**: `smooth` the natural damp wall instead — no boulders
  or construction jobs needed, smoothing seals it directly. Check the wall
  material with `cross_section`/`look` before deciding which branch
  applies; don't default to construction out of habit once you have a
  stone aquifer.

Two things NOT to do:
- Never wall or smooth over a stair tile to blind a wet face — that
  destroys the only way down through this level.
- If a mined ring tile turns out to be adjacent to a standing pocket of
  water rather than fresh seepage (e.g. an old flooded space you broke
  into), that's not a reason to skip the wall — mine through, let the
  pocket drain and thin out down the shaft, then wall the gap to
  re-isolate it same as any other ring tile.

## 6. Mining vs. construction stall diagnostic — the 2/7 vs 4/7 split

Two different job types run at the same time in a flooding ring, and
confusing their stall behavior wastes a session:

- **Construction** (sealing ring tiles) suspends once water reaches
  **2/7** depth at the tile — the job doesn't fail, it sits queued and
  waiting. This is a lower threshold than you'd guess from the earlier
  fort-experience note that builders "work fine in ≤3-depth water": below
  2/7 the job runs normally, at-or-above 2/7 it suspends rather than
  completing, and it can still be sitting there, recoverable, well past
  that point.
- **Mining** (opening ring tiles) tolerates more standing water before it
  gives up outright — it keeps working until roughly **4/7**, beyond which
  the dig stops. A ring tile that stays undug turn after turn despite
  repeated re-designation (§4) at this depth is hitting this ceiling, not
  a dedup-hidden cancel.

If walls are stalling (suspended) while mining elsewhere on the same ring
keeps progressing, that's the 2/7 suspend case, not a broken plan: use
`unsuspend` on the stalled building/job and retry, rather than
re-designating or redesigning the seal. Re-planning a suspended-not-failed
job wastes a turn for nothing. Conversely, if a ring tile simply won't dig
no matter how many times you re-designate it, check the water depth first
— at or above the ~4/7 mining ceiling, re-designating won't help until the
level around it drains a notch (draw down through the shaft/basin, or wait
out evaporation from an already-sealed neighbor face) — it's not a
dedup-swallowed damp-cancel like §2/§4.

## 7. HEAVY aquifers — unverified, do not treat as interchangeable with light

Everything above is validated against a light aquifer only. If §0 flagged
heavy inflow, the wall-in-standing-water step in §5 is known to fail
outright against a heavy aquifer — freshly mined tiles refill within ticks,
faster than a construction job can complete. This team has not executed a
successful heavy-aquifer pierce. The community approaches below are
recorded for orientation only — none of them are executable with current
tooling or verified experience, and each names the specific gap:

- **Cave-in plug**: collapse material to block the flow. No cave-in
  designation/tooling exists in this project yet.
- **Pump-assisted drain-while-you-work**: keep the working tile dry with a
  screw pump while ringing it. Screw pumps are not implemented.
- **Freezing**: some climates freeze standing water solid enough to mine
  through. Climate-dependent, unverified here, and not something a
  protocol step can guarantee.
- **Hatch-trick (open/shut timing over a hatch to admit a miner then seal
  behind them)**: needs both pump support and precise lever/timing control
  we don't have. See `aquifer-water-infrastructure` for the tooling gap on
  levers.

If you hit a heavy aquifer: stop, do not attempt the light protocol's
wall-in-water step, and treat the open aquifer tile as a known future
water source (a heavy aquifer is effectively an infinite drain from
above — useful for taps once the water-infrastructure skill's tooling
gaps close, dangerous while trying to pierce past it).

## 8. Multi-layer aquifers

If `cross_section`/`survey_site` shows the aquifer flag set on more than
one z-level, treat each damp level as its own full instance of the
pierce/ring/wall/diagnostic cycle (§2, §4-§6 — the one-time boulder
quarry in §3 doesn't repeat per level): every wet level needs its own
ring and its own seal, not just the top one.
Also: the level immediately BELOW the lowest wet layer is still wet space
from a seepage standpoint (straight-down seepage from the layer above) —
don't assume "the aquifer" ends the moment you stop seeing the flag; check
the level below the last flagged layer with the same care before declaring
you're through.

## 9. Verify the seal, then resume from a dry offset

A sealed ring won't clear instantly — the catch-basin puddle behind a good
seal fully evaporates over roughly 5-10 game-days. A dry shaft bottom is
the definitive signal that the seal held; don't declare success on a
shrinking-but-still-wet puddle, and don't re-dig through standing water at
the old bottom out of impatience.

Resume the descent from a dry offset instead of the wet bottom itself —
e.g. continue stairs from a quarry room one level below rather than the
exact tile that used to hold the puddle. The wet column at the old bottom
doesn't need to be redug; it can become a cistern later (see
`aquifer-water-infrastructure`).

## Related tools referenced above

`designate_dig` (types: stairs, mine, channel), `cancel_designation`,
`build`, `smooth`, `unsuspend`, `cross_section`, `look`, `survey_site`,
`step`, `alerts`. Note: `channel` digs from the level above and is the
right verb for a single controlled tap (see the water-infrastructure
skill) — piercing a descent shaft uses `stairs` + `mine`, not `channel`,
because channeling eats the level above and leaves ramps rather than a
clean descent.
