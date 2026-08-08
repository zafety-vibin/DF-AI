# Goals — Fort #6 "Lanehold" (world=region13, winter y100 day 334 →)
# Sessions 1-3 done. Pop 19, ZERO dwarf deaths. Wealth 18.3k and rising.
# DF 53.16; plugin rebuilt+reinstalled by overseer, fort verified intact.
# SESSION 4 THESIS: wealth + tavern visitors = the siege clock is running.
# Trade the fort's idle labour for defence in depth, before it's needed.

## NOW (survival)
- [x] Surface SEALED — one entrance, the door at (105,102) z=138
- [x] Every dwarf owns a bedroom (19 zones / 19 dwarves)
- [x] Bridge airlock + 2 levers protecting the z=128 noble sanctum
- [ ] THE GATE: weapon-trap row + raising bridge behind the surface door.
      Traps are the ONLY working trap type (stone-fall builds unarmed,
      pressure plates can't be configured to fire — building_types is
      honest about both). Bridge stays DOWN; it is the panic seal.
- [ ] Civilian shelter: re-verify the "Lanehold Interior" burrow +
      set_alert chain (worked in session 1; untested since the fort
      grew to 6 levels and 19 dwarves).

## SOON (headroom)
- [ ] Finish the noble layer z=128: ~87 tiles were still designated,
      then doors on the 6 gaps (x=112/118/124 at y=100 and y=104),
      furniture, and zones — bedrooms north, offices/dining south.
      Assign adil (manager/broker) and zaneg (bookkeeper) first.
- [ ] Elven caravan, spring y101 (IMMINENT). Depot flags already set.
      PAUSE AND ASK THE OVERSEER before any exchange — standing rule,
      they execute in-client. Live test for binned-crafts staging.
- [ ] Depot accessible=false while walkable_from_edge=true — wagon-width
      pinch, suspected sapling regrowth. Diagnose before the caravan.
- [ ] Keas are looting the surface stockpile outside the seal. It CANNOT
      be removed by tooling: TWO stockpiles share identical extents
      (88,88)-(92,91) and remove_building rightly refuses to guess.
      Needs an overseer click, or a building-id selector in the tool.
- [ ] Fishery is the last building outside the seal (fishing exhausted
      fort-wide) — give it an underground home or retire it.

## EVENTUAL (trajectory)
- [ ] A song composed HERE. art census is 0 composed / 6 brought, and
      both carriers (olon, edóm) are finally housed and off fishing.
- [ ] Deep pierce below z=127 (aquifer) into the mineral country
- [ ] Surface compound + archer deck IF squads ever work (assign_squad
      is systemically broken — defence stays architectural until then)
- [ ] The dog stays unburied. DF gives animals no tomb; the crypt coffin
      at (95,113,133) waits for a dwarf instead. Her death is recorded
      in combat_report engagement 84 — that is her grave marker.

## SHIPPED 2026-08-07/08 — DLL DEPLOYED (859,648 B). LIVE-VERIFY FIRST.
## 2026-08-08: DFHack updated to 53.16-r1.1. The sibling checkout was
## moved from tag 53.16-r1 -> 53.16-r1.1 (git fetch --tags; checkout;
## submodule update --recursive), junction + CMakeLists.custom.txt hook
## both verified intact afterward, plugin REBUILT and the embedded
## version string verified as 53.16-r1.1 via `strings` on the deployed
## DLL. Size was byte-identical to the previous build — size is NOT a
## rebuild check; the embedded version string is.
## Restart df-mcp, launch DF, ai-connect, then run this list before
## trusting any of it. Everything below is COMPILE-VERIFIED ONLY.
1. **assign_squad** — THE decisive test; has never once succeeded in
   this project's history. Squad #22 already exists (MILITIA_COMMANDER,
   thíkut #1197 appointed). Try assign_squad squad=22 unit=1286 add=true.
   Root cause was DFHack's addToSquad auto-pick selecting slot 0 (the
   commander) then refusing it; we now pass an explicit non-commander
   slot bounded by positions.size(). NOTE: filling the COMMANDER slot
   is still impossible and the ACK says so — that is UNVERIFIED
   territory, not a claim.
2. **look footer** at z=132 (Iron Quarter had ~200 loose items) — should
   now split items-vs-tiles, name how many are OUTSIDE stockpiles, and
   give clusters. Silent on a tidy view.
3. **buildings** — stockpiles should now render extents + category +
   tiles occupied/total + item count.
4. **lens=items** — class letters, UPPERCASE = outside a stockpile.
5. **remove_building (90,90,138)** — the kea larder. Should now NAME both
   duplicate piles and either give a tile that resolves each one alone,
   or state plainly that none exists. This was previously unfollowable.
6. **orders ladder** — should gain a "no citizen holds the required
   labor" rung (the bug that silently stalled all carpentry).
7. **stocks** — drink/food counts should disambiguate stacks from
   servings (absent-vs-zero now distinguishable).
8. **designate_dig over stairs** — should return PARTIAL, not SUCCESS.

## ENGINEERING QUEUE (for the fix wave — sessions 3+4 experience)

### P0 — SQUAD MANAGEMENT (blocks the entire military arc)
- `assign_squad` FAILS on a correctly-formed squad. Repro: appoint a
  dwarf MILITIA_COMMANDER (real vacant slot), create_squad
  position_code=MILITIA_COMMANDER → squad #22 "10 slots, leader vacant",
  re-appoint to bind leader, then assign_squad ANY unit →
  `Military::addToSquad failed ... (squad may be full, or the unit has
  no historical figure)`. Fails for both an ordinary dwarf AND the
  appointed commander. SUSPECT: our create_squad mints a squad that
  never lands in the fort entity's `squads` vector, so DFHack can't
  validate it. Check against DFHack's own military/squad creation path.
- `create_squad` DEFAULTS to MILITIA_CAPTAIN, which on a young fort has
  "no assignment slot yet (not unlocked)" — so the default silently
  takes the tool's own self-flagged UNVERIFIED mint path. That is the
  origin of Fort #5's ghost squad #18. Default should prefer a position
  that actually has a vacant slot, or refuse with guidance.
- `list_squads` has NO fort-entity filter: returns 22 WORLD squads, all
  with `unit#-1` members. Unusable for picking a squad id.
- Downstream, still unexercised because of the above: squad_order,
  barracks zone assignment, training. Note designate_zone's own docs
  admit barracks has "no confirmed assignment mechanism yet".

### P0 — TRUTHFULNESS BUGS (these actively misled me in play)
- **orders ladder needs a "no citizen holds the required labor" rung.**
  A whole session of wood items read "in progress" with ZERO produced
  because no work detail contained CARPENTER. "in progress" on an
  impossible job is indistinguishable from real progress. The honest
  discriminator was the workshop's own empty job queue.
- **`stocks` stack-units still absent** (shipped wave 5, never rendered).
  DRINK=15 is 15 BARRELS, not 15 servings — I called a famine that
  wasn't there and the overseer had to correct me live.
- **dig ACK returns SUCCESS while destroying a staircase.** "143
  designated (4 will remove existing stairs: vertical connection lost)"
  is a fatal warning wearing a success label; I stepped past it and
  isolated the noble quarter with two dwarves inside. Consider PARTIAL
  status, or an explicit opt-in flag for stair-destroying rects.

### P1 — NEW CAPABILITY worth building (born from live failures)
- **SEAL AUDIT tool.** DF allows DIAGONAL CORNER-CUTTING, so a diagonal
  wall line never seals — this bit us twice and I got the rule wrong in
  a written audit. Proposal: given a z-level (or the fort), report every
  walkable-OUTSIDE tile orthogonally *or diagonally* adjacent to a
  walkable-INSIDE tile, PLUS roof holes (open air directly above an
  interior tile — how (93,101,139) was found). This is exactly the
  reasoning I cannot do reliably by eye off a glyph crop.
- **FLOOR-ITEM VISIBILITY in `look` (overseer request, 2026-08-07).**
  Today `look` prints only a footer count ("tiles with loose items on
  floor: 200") — you cannot see WHERE items are, so you cannot tell a
  full stockpile from an empty one, or find the 200 loose items piling
  up in a workshop quarter. Investigate an efficient per-tile display:
  e.g. a `lens=items` overlay painting item-bearing tiles (possibly
  graded by count, or keyed by item class like the minerals lens's
  a/b/c legend), plus stockpile fill state. This is the tool gap that
  made "we need more stockpile room" invisible to me until the overseer
  said it out loud.
- `buildings` never names a workshop's TYPE — teardown/identification
  requires probing each shop's job queue.
- `remove_building` cannot disambiguate two buildings with IDENTICAL
  extents (two stockpiles both span (88,88)-(92,91)) — needs a
  building-id selector. Keas are looting that pile right now.
- orders ladder needs a "no citizen holds the required labor" rung.
- `stocks` stack-units still absent (caused a live famine misdiagnosis).
- region_scan z-level set + tile count unstable between calls
  (5744 over {130,131,133,137,138,139} → 4387 over a DIFFERENT six).
- connector hint still emits a map-corner anchor (11,0,138).

### P2 — VOCABULARY INCONSISTENCY (cost me a diagnostic round each)
- The same object has different names in different tools: `build` takes
  **coffer** but `stocks` files it as **BOX**; `order` takes **bin** but
  `queue_job` needs **ConstructBin**; there is no **mechanism** item —
  it's **ConstructMechanisms**, and `stocks` calls it **TRAPPARTS**.
  Each mismatch is a failed call plus a job_types lookup. Worth one
  pass to align the vocabularies (or make stocks accept build's names).
- `combat_report` renders a TAME FORT ANIMAL as fort side "(none)" —
  the stray dog killed by the carnotaurus wasn't counted as ours.
- `trade_agreements` reads only civs visiting RIGHT NOW, so a departed
  liaison's agreement is unrecoverable. NOT a bug — the tool is honest —
  but it should be documented so no future session hopes otherwise.

## Standing rules learned here (do not relitigate)
- Zone dance is FOLKLORE — designate→assign in one paused breath, twice
  confirmed. Do not step between them.
- Orders "in progress" does NOT mean progress. Check the workshop's own
  job queue; empty queue = no citizen holds the labor. Check
  work_details BEFORE queueing any new industry.
- `stocks` DRINK/food counts are BARREL-STACKS, not servings.
- Never furnish before engraving; smooth→engrave→furniture is one-way.
- A lever on the protected side of a DEAD-END pocket is a self-lock.
  Every bridge needs a lever on the side that keeps food and water.
- NEVER feed iron picks to weapon traps — they are the mining tools.
