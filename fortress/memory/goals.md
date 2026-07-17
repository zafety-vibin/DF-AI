# Goals — Fort #4 (river/brook map; day 121 summer, PAUSED, 16 dwarves)

## OVERSEER DIRECTION (2026-07-16): pausing after the first migrant wave
## to extract this session's experience into memory and consider fixing
## the tool bugs found. On resume: status→pause until real data; verify
## plugin loads. Big population jump (7->16) means labor triage and
## housing capacity are the immediate pressures, not more exploration.

## NOW (survival)
- [x] Aquifer pierce+seal (both z=116 AND z=115 soil layers) — DONE and
      holding, confirmed dry. Industry level (z=112-114 dry stone) open,
      mason workshop built (50,44,112).
- [ ] Verify second miner (aban, id=1106, MINING Lvl2, labor just
      enabled) actually mines — check for a second pick; if aban stays
      idle on dig jobs, pick supply is still the ceiling.
- [ ] Labor triage for the migrant wave: several new arrivals show
      `labors=14` / "no skills" (obok, uvash, ilral) vs the full-labor
      citizens — confirm which are real citizens vs pets before
      assigning anything (see the population-counting learning).
      Notable skilled migrants: minkot (Stonecraft 6), cog (Bonecarve
      15!), olon (Make Music 6), mörul (Dance 3), zasit#2
      (Situational Awareness) — cog especially is a strong crafts pickup.
- [ ] Dining hall: octagon hub at z=113 has 2/3 tables + 1 chair built,
      3rd table + 6 chairs still mid-craft (one cancelled on "needs
      non-economic logs" despite 25 wood on hand — cause not fully
      diagnosed). has_dining_hall predicate still false; push furniture
      to completion and re-check.
- [ ] Drinks/food: still steady before the wave (14 drinks, 46+ seeds,
      12 plants) but population just doubled+ — watch consumption rate
      closely now, may need a second still/farm plot soon. NOTE: the
      (38,44,118) Still was destroyed+rebuilt live during session 3 tool
      verification (see below) — production paused ~10 days, check
      drink stock is recovering next session.

## SOON (headroom)
- [ ] Bedroom capacity: 9 real zones (2 furnished real rooms + 7 ad-hoc
      1x1s) for 16 dwarves. has_bedroom_zones_15 not yet met (need 15).
      Plan the NEXT housing cluster several z-levels further from the
      aquifer (e.g. z=109-110) for a firmer dry buffer — z=113 proved
      workable but spotty (lost 1 of 4 planned rooms to real aquifer
      water this session). Bore-check every room's actual footprint
      corners individually, not one representative point per direction.
- [ ] Second workshop tier: smelter/forge once ore is found in the
      industry level — still untested, blocked on finding ore veins.
- [ ] First caravan (autumn): trade depot BUILT at (55,54,119). BUY A
      SECOND PICK if the depot offers one — directly unblocks aban.
- [ ] Hatch-cap the 2 orphan down-stairs at (51-52,46-47,119), or leave.

## EVENTUAL (trajectory)
- [ ] Migrate remaining ad-hoc 1x1 "bedrooms" (5 in main hall, 2 in
      quarry room) to real walled rooms with door+cabinet as capacity
      allows — they satisfy the predicate but not the actual standard
      (see feedback_room_design_standards in Claude Code memory).
- [ ] Whole-map plan refinement: farming 118 (done), industry 112-114
      (open, one workshop so far), housing needs a real second cluster.
- [ ] Survive year 1; first ACTIVE THREAT response (nothing hostile yet,
      though population is now large enough to be a real target).

## Tool state (2026-07-17, session 3: live verification)
- CONFIRMED WORKING LIVE (session 2, unchanged): multi-layer aquifer
  pierce+seal (2 stacked soil layers); `build type=updownstair`/
  `upstair`/`downstair` as a real stair-repair/construction path (name
  is NOT "stairs"); `smooth mode=wall` as a reactive leak-seal on
  stone-aquifer-adjacent walls; whole-room `designate_zone`+
  `assign_zone` (not just 1x1 on a bed); `unassign_zone` (needs
  `unit_id`, not just coords); `set_labor` needs `id`+`labor`+`enable`
  params; ConstructCabinet/ConstructDoor/ConstructThrone/ConstructTable
  job_types all real and carpenter-craftable.
- STILL BROKEN: the shared modification tracker (`has_modified_anything`/
  dug-tile counts, and `look scope=fort`) still reports ZERO/empty with
  dozens of buildings on record (confirmed again this session at z=112
  despite a built mason workshop+beds there) — the 2026-07-15 fix did
  not hold. Use `elevation`/`cross_section`/explicit-coordinate `look`
  as the reliable substitute.
- 2026-07-16 FIX WAVE — ALL 5 ITEMS LIVE-VERIFIED 2026-07-17 (session 3,
  fresh DLL rebuilt against DFHack 53.15-r2):
  1. **VERIFIED** — `designate_dig type=mine` over a carved down-stair
     at (51,46,119): ACK read "1 designated (1 will remove existing
     stairs: vertical connection lost)" — explicit, truthful, matches
     spec exactly.
  2. **VERIFIED** — `stocks category=bed/door/table`: all three showed
     `count=0` free with correct `in_use` built-in counts (bed +9,
     door +3, table +2), matching the fort's actual built furniture.
  3. **VERIFIED** — `smooth mode=wall` on a natural wall at (48,41,112)
     → `cross_section` reported "SMOOTHED"; a genuinely constructed
     aquifer-seal ring wall at (44,43,116) — DAMP, built 2026-07-16 —
     correctly did NOT false-positive as smoothed. Both halves of the
     guard confirmed.
  4. **VERIFIED, with a caveat** — `remove_building` at (38,44,118), a
     tile inside the (37,44,118) stockpile's rectangle AND the site of
     a built Workshop (the Still): resolved cleanly to the Workshop only
     (stockpile untouched, confirmed by building-list diff) — no
     disambiguation error fired. Read the source
     (`df_ai_protocol.cpp:356-429`): DF's own stockpile room-extents
     bitmask apparently already excludes tiles occupied by a later-built
     workshop, so `containsTile` correctly returns false there and no
     ambiguity exists to report — the candidate-collection logic is
     sound, but this fort currently has no tile that reproduces the
     true 2-candidate branch, so the disambiguation-error *text* itself
     stays unexercised live. **Side effect of this test: it destroyed
     the real Still — rebuilt immediately after (still `built` at
     (38,44,118) again), ~10 game-days of drink production lost.**
  5. **VERIFIED** — a tile with a loose item at (36,44,118):
     `cross_section` reported "1 ITEM(S) ON FLOOR"; `look`'s aggregate
     "tiles with loose items on floor: N" line also confirmed present
     and consistent across several z=118/119 views.
- MECHANISMS END-TO-END — **VERIFIED** (new ground, 2026-07-17, user-
  directed mid-session): built a Mechanic workshop (38,40,119), crafted
  3 mechanisms via `queue_job item=ConstructMechanisms`, channeled a
  3-tile gap at (36-38,42,119) with `designate_dig type=channel` (a
  flat-ground `build type=bridge` attempt with no gap FAILED cleanly —
  bridges need a real elevation change/open-air span, confirming a
  live hypothesis), built a lever (36,40,119) and a 3x1 bridge
  (37,42,119, direction=raise_n) over the channel, `link_building`'d
  them (first attempt correctly FAILED truthfully — "only one free
  mechanism available... linking consumes two" — retried after
  crafting a 3rd), then `pull_lever` twice (raise, then lower), both
  completing cleanly with no errors. **Known gap**: `look`/
  `cross_section` do not visually distinguish a raised vs. lowered
  bridge state — the mechanism itself works, but there's no tool-side
  way to confirm which state it's in without inference from job
  completion. This test rig (workshop+lever+bridge at x=36-38,y=40-42,
  z=119) is real standing infrastructure, not cleaned up — future
  sessions can repurpose or extend it (e.g. toward the real brook
  crossing at the (51-58,42-47,118) west bend, which was the original
  target but needs a real dig corridor through ~7 tiles of soil to
  reach the water and was deferred as out of scope for this session).
- MECHANISM LIBRARY FOLLOW-UP (2026-07-17, same session): checked what
  else `link_building` supports beyond bridge — wire protocol
  (`protocol.h:291-294`) confirms exactly 4 valid trigger targets, no
  more: bridge, door, hatch, floodgate. No well/pressure-plate/weapon-
  trap/cage-trap/restraint exist anywhere in the protocol (matches
  [[aquifer-water-infrastructure]]'s "no well build type" caveat).
  - **Door — VERIFIED.** Built a standalone door (39,42,119, NOT an
    existing in-use fort door — flipping `operated_by_mechanisms` live
    on a real doorway risked stranding dwarves), linked to the SAME
    lever already controlling the bridge, pulled cleanly.
  - **Hatch — VERIFIED**, same pattern, (40,42,119).
  - **Multi-target lever — VERIFIED as a side effect**: one lever now
    controls bridge+door+hatch simultaneously (`linked_mechanisms` is a
    vector, confirmed by 3 successful `link_building` calls against the
    same lever tile with no conflict).
  - **Floodgate — BLOCKED ON A REAL BUG, not tested.** `ConstructFloodgate`
    is entirely missing from `jobTypeAllowedAtWorkshop`'s switch
    (`work_orders.cpp:192-219`) — hits `default: return false` at every
    workshop type, Mechanics/Carpenters/Masons all rejected it. In real
    DF this belongs at Carpenter's/Mason's, same pattern as the
    `ConstructHatchCover` case right above it in source — just never
    added. Add the case, rebuild, redeploy, then floodgate is the one
    remaining untested target.
  - Known perception gap (confirmed across all 3 tested targets, not
    bridge-specific): `look`/`cross_section` render bridge/door/hatch
    as their built-glyph regardless of raised/lowered or
    locked/unlocked state — no tool-side way to read triggered state,
    only job-completion inference.
  - **Willow-wood mystery, narrowed**: the long-standing "needs
    non-economic logs" cancel (first seen 2026-07-16, cause undiagnosed)
    recurred on a `ConstructDoor` job with 12 willow logs free in stock
    — but a FRESH chop (1 tree -> 15 plum wood logs) let the identical
    job succeed immediately (masterpiece plum wood door). Willow
    specifically appears to be blocked (economic-flagged, or some other
    per-material restriction), not a quantity problem — full root cause
    still open, but "just chop a non-willow tree" is now a known
    working workaround.
- STILL UNTESTED, blocked: forge/smelter (need ore, not just any
  stone); kitchen_permissions; trade/caravan interaction tools (depot
  only, no caravan arrived yet).
