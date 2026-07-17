# Fort Journal

(newest entries on top)

## 2026-07-17 — Fort #4, session 3: tool verification (Summer y100, day 121→140)

Narrow-scope session: live-verify the 4 fixes from the 2026-07-16 wave
plus a floor-item check, then (added mid-session by the overseer) prove
out the mechanism/lever/bridge chain end-to-end. Plugin redeploy
confirmed live against DFHack 53.15-r2. All 5 original checks VERIFIED;
full detail in goals.md's Tool State section. Two things worth
remembering: (1) the `remove_building` disambiguation test destroyed the
fort's actual Still (stockpile+workshop overlap tile resolved cleanly to
the workshop, no ambiguity fired) — rebuilt immediately, but ~10 days of
drink production were lost mid-test; (2) built a small standalone
mechanism test rig (mechanic workshop + lever + 3x1 bridge over a
purpose-dug channel) at (36-38,40-42,119) — a flat-ground bridge attempt
failed cleanly first (bridges need a real gap), the channel fixed that,
and lever→link→pull worked cleanly both directions. This rig is real
infrastructure, not scaffolding — left standing for a future session to
extend toward the real brook crossing near the west bend (51-58,42-47,
118), which needs a dug corridor through ~7 tiles of soil and was out of
scope here. See [[project_fort4_session2_tool_bugs]] for the bugs this
session closed out.

Follow-up (still same session, overseer asked "what else can levers
control besides bridges?"): checked the protocol source directly —
`link_building` supports exactly 4 target types (bridge/door/hatch/
floodgate), nothing else exists (no well, no traps, no restraints).
Built and linked a standalone door and hatch to the SAME lever already
driving the bridge — both VERIFIED, and confirms one lever can drive
multiple mechanisms at once. Floodgate turned out to be a real bug, not
a test gap: `ConstructFloodgate` is missing from the plugin's
workshop-compatibility table entirely, rejected at every workshop type.
Also stumbled into (and partly explained) the willow-wood "needs
non-economic logs" mystery from session 2 — a door job kept failing
against 12 free willow logs but succeeded instantly once a single fresh
non-willow tree was chopped. Full writeup in
[[project_fort4_session3_verification]].

## 2026-07-16 — Fort #4, session 2 continued (Summer y100, day 67→121)

Continued past the day-67 checkpoint per overseer direction ("keep going,
work on the dining hall next"), with a standing to redo bedrooms properly
(walls+door+cabinet, non-rectangular layouts, reserve a noble suite) —
see [[feedback_room_design_standards]]. Built an OCTAGONAL HUB at z=113
(one level below the sealed aquifer, in the dry stone reached this
session) as a combined dining hall + stairwell landing, with bedroom
spokes radiating N/NE/S/SE/SW. This is the fort's first genuinely
non-rectangular architecture.

**LIVE INCIDENT: the octagon dig destroyed the shaft's up-stair
component.** The hub's mine designation overlapped the 2x2 core and
silently converted `UpDownStair` tiles to plain `DownStair` — severing
the connection to everything above, with dwarves briefly stranded below.
`designate_dig type=stairs` re-issued on the same tiles refused to fix it
("already carved", 0 designated — the plugin only checks carved-or-not,
not stair sub-type). Root-caused and logged for engineering
(docs/decisions.md). Confirmed `build type=updownstair` (NOT "stairs")
DOES work as a real repair path — tested clean on virgin floor first,
then applied to the actually-broken tiles; full connectivity restored
and verified via cross_section top-to-bottom. Overseer initially planned
to fix this manually in-game, then handed it back once the repair path
was demonstrated working.

**A second near-miss, self-inflicted this time**: while the octagon/
spokes were mid-dig, a south-west bedroom pod turned out to sit right on
a real aquifer pocket — confirmed via cross_section (AQUIFER DAMP down
through z=110), matching the west-side hazard already known from earlier
this session but missed because the original bore-check sampled one
"representative" point per direction rather than the actual room
footprints. Cancelled that room outright rather than fight it. Real
water (not just residual DAMP) later appeared inside the NW room too
(one tile, then two) despite that room reading clean on bore-check —
overseer suggested `smooth mode=wall` on the exposed damp walls as a
stone-aquifer seal instead of constructed walls; applied to NW+NE rooms'
north walls, seepage stopped spreading (confirmed no growth after,
though `look` never visually reflects a successful smooth — logged as a
tooling gap). NW room ultimately abandoned as too wet to be worth
finishing; NE (built as a deliberately larger, extra-furnished NOBLE
SUITE reserve) and SE rooms furnished properly: door + bed + cabinet
each, NE additionally getting a chair, all zoned as WHOLE-ROOM bedroom
zones (not 1x1 bed-only zones) and assigned to specific dwarves.
**Verdict on z=113 as a housing level: workable but spotty** — dig the
NEXT housing cluster several levels further from the sealed aquifer
(e.g. z=109-110) for a firmer buffer, per overseer's direct suggestion.

**Furniture-supply lesson, learned the expensive way**: repeatedly hit
"needs bed"/"needs door"/"needs table" cancels on plans that LOOKED like
they had material in `stocks` — the count was including beds/doors/
tables already incorporated into OTHER buildings, not just free loose
stock (logged as a stocks bug). Real fix each time was to queue fresh
`queue_job` crafting and only then re-place the building. Overseer's
standing advice: stay ahead on furniture production before placing, or
expect instant silent cancels — a real DF pattern, not just us.

**MIGRANT WAVE ARRIVED — population 7 -> 16.** First migrant wave of
this fort. One new arrival, aban, already carries MINING skill (Lvl2) —
enabled the MINE labor immediately, a real chance at finally breaking
the single-miner ceiling that shaped this whole session (next session:
confirm aban actually mines — needs a pick, unverified whether one's
available). check_goals now shows has_min_dwarves_14 MET alongside the
earlier has_bedroom_zones_7 (now 9 zones, all real rooms or upgraded
zones — the old ad-hoc 1x1s for the two dwarves who got real rooms were
explicitly unassigned so they's not double-counted/wasted).

**Dining hall**: 2 of 3 tables + 1 chair built in the octagon hub by
session's end (3rd table + throne batch still mid-craft, blocked briefly
on a "needs non-economic logs" cancel despite 25 wood on hand — cause
not fully diagnosed, flagged as a follow-up). has_dining_hall predicate
still reads false; likely needs more chairs/tables actually complete
before it flips, or possibly a dedicated zone type this project doesn't
have yet (unconfirmed).

**Fort state at pause (day 121, summer)**: 33 buildings (was 3 at start
of the whole session, 21 at the day-67 checkpoint). Octagon hub (dining/
gathering space) + 2 furnished real bedrooms (NE noble suite, SE
standard) + 5 original single-tile ad-hoc bedrooms still standing in the
main hall + 2 in the old quarry room = 9 total bedroom zones for 16
dwarves. Second miner (aban) labor-enabled, unverified live. Wedding
(Kel + özum) and a promotion (äshrir -> full Carpenter) both occurred —
first fort life-cycle events observed. NEXT: verify second miner mines
(pick permitting), finish dining hall furniture, plan the deeper (z~109)
housing cluster properly bore-checked room-by-room this time, migrant
labor triage (several arrived with no skills/`labors=14` — check who's
actually a citizen vs a pet before assigning anything).

## 2026-07-16 — Fort #4, session 2 (Spring y100, day 34→67)

RESUME per overseer direction: expand the fort in its current state, get
valuables/goods into a protected underground stockpile, and rebuild the
removed surface workshops (carpenter, still) underground instead. Both
achieved, plus the fort's first DOUBLE-LAYER aquifer pierce+seal.

**Expansion**: merged the original farm hall (44-48,42-46)@118 with a new
west excavation (30-43,42-46) by mining through the old dividing wall —
one continuous hall, carpenter + still rebuilt inside it (33,44) and
(38,44), an "all" stockpile over the remaining floor. First underground
industry+storage space of the fort.

**AQUIFER: BOTH LAYERS PIERCED AND SEALED** (z=116 and z=115, each its own
full ring+wall cycle — first stacked/multi-layer aquifer this project has
executed, confirmed same protocol as single-layer, just repeated). Shaft
descended to z=112 (2 levels below the aquifer) where a small quarry
yielded 12 mudstone boulders — banked BEFORE opening either ring, per
protocol. All 16 ring tiles (8/layer) sealed with constructed walls. One
core tile (46,44,114) was silently skipped by an earlier designation
("already carved" false positive) and had to be re-designated by hand
after the fact — the residual puddle at that column didn't fully drain
until it was filled in. Seal verified dry end-to-end after ~5 game days;
industry level (z=112-114, dry stone) now open. Mason workshop placed
(50,44,112).

**MID-SESSION CORRECTION (overseer-caught)**: while the aquifer ring was
mid-dig, a concurrently-designated bedroom-cluster dig (south of the main
hall) was pulling the fort's ONLY miner away from the aquifer-sealing
work AND was one row from breaching a separate 7/7-depth standing-water
pocket at y=55+ — a second near-miss of the same class as session 1's
creek incident, this time self-inflicted via dig-ahead with no second
miner to spare. Cancelled the competing designation immediately; the
aquifer finished cleanly right after. Lesson written up in learnings.md.

**Tool friction found live** (see learnings.md for full detail): a
`stockpile` designation and a workshop's footprint can't coexist — build
workshops FIRST, stockpile second (it correctly skips occupied tiles).
`remove_building` targeting by coordinate picked the WRONG building when
a stockpile rectangle and a workshop's 3x3 footprint shared a tile — it
deconstructed the Still instead of the stockpile; rebuilt it, moved on.
Loose items sitting on stockpile tiles are invisible to `look` and still
block new `build` placement even after the stockpile itself is gone —
had to place beds on virgin, never-stockpiled floor instead.

**Fort state at pause (day 67)**: 21 buildings (was 3 at session start).
7/7 dwarves have OWNED bedrooms (bedroom zones goal now MET) — 5 in the
main hall (44,42-46,118), 2 in the new quarry room (52,43/45,112).
Carpenter + Still rebuilt underground and both active (drinks 14, seeds
46 climbing, boulders 12 banked). check_goals: has_bedroom_zones_7 ✓,
has_min_dwarves_7 ✓, no_active_hostiles ✓. Still open: dining hall (0),
has_shelter predicates (dug-tile modification tracker still reports 0 —
the regression flagged 2026-07-15 is NOT actually fixed, re-flag for
engineering). NEXT: dining hall + tables/chairs, deeper industry
(smelter/forge once ore is found), watch the 2 quarry-room beds — a
starter placement mixing housing with industry, revisit when a real
bedroom cluster gets planned somewhere hazard-checked.

## 2026-07-15 — Fort #4, session 1 (Spring y100, day 14→34; SAVED for later)

FRESH EMBARK on a RIVER/BROOK map (96x96, surface z≈119, brook channel at
z=118 snaking N-S down the east side with a WEST BEND at y=44-47 reaching
x=51, plus a NW lake complex; soil aquifer z=116-115 map-wide; dry stone
z=114-108; deep stone aquifer z=107-105). Session focus: live-verify the
2026-07-15 fix wave. Mid-session: DFHack had auto-updated to 53.15-r2 —
plugin refused to load until the checkout was retargeted + rebuilt (twice:
once for r2, once more for the brewing hotfix below, hot-swapped via
unload/copy/load with DF open).

**🍺 FIRST DRINK IN PROJECT HISTORY.** list_reactions was still empty on
the wave's DLL — root-caused live (permitted_reaction_STR is raw-load
staging, empty at runtime; DFHack's stockflow uses permitted_reaction_ID)
— hotfixed, redeployed, and then: still built → BREW_DRINK_FROM_PLANT
queued → first cancel taught "needs empty food storage item" → carpenter
+ 3 barrels → re-queued → dwarven wine +1 barrel, plump helmet seeds 5→10.
Drink chain closed after 3 forts. Also observed a dwarf DrinkItem after.

**THE CREEK NEAR-MISS (twice).** Sited the first shaft at (51-52,46-47)
and a farm hall east of it — both intersect the hidden brook channel at
z=118. Saved twice: once by DF's "Dangerous terrain" miner refusal, once
by the overseer asking "do you realize you're digging the creek into the
fort?". Root cause was a tool gap: cross_section shows NO water on hidden
tiles (bore at (51,46) called the 7/7 channel "dry hidden soil") despite
the world-model doc contract. Recovery: cancelled everything near water,
re-sited shaft (45,43)-(46,44) z119→117 INSIDE the west hall
(44,42)-(48,46)@118, 2-tile bank buffer respected. Fix agent dispatched.

**Fix-wave verification (Fort #4 live results):**
- Brewing ✓ (after hotfix; see above) — end-to-end with seeds returned
- Stockpile ✓ — overseer confirmed in-UI: real "all" preset, items
  hauling in, settings screen no longer crashes (Fort #3's killer)
- assign_crop ✓ — truthful "FAILED: under construction (stage 0/3)"
  pre-build, SUCCESS post-build, PlantSeeds jobs observed
- remove_zone severity ✓ (SUCCESS, was PARTIAL); case-insensitive
  filters ✓ ("weapon"/"plump"); connector-hint false positives GONE ✓
- TradeDepot ✓ BUILT (5x5 center→NW conversion works) — first depot
  ever; autumn caravan = pick source
- scope=fort / save_blueprint ✗ REGRESSION — the shared modification
  tracker now records NOTHING (dug hall reported "no modifications");
  the ambient-filter fix over-corrected. Fix agent dispatched.
- Forge/smelter untested (blocked on stone → blocked on aquifer pierce);
  repeat-warnings xN untested (no damp-cancel yet); lodging still pending

**Fort state at save (day 34):** surface: still (44,50), carpenter
(41,51), tradedepot (55,54), stockpile-all (50-52,54-56), wagon (47,47),
2 orphan down-stairs at (51-52,46-47) — cap or ignore. Underground: hall
(44,42)-(48,46)@118 with 2x5 plump-helmet plot (47-48,42-46) being
planted, spine stub (45,43)-(46,44) z119→117. Stocks: 13 drinks, 19+
logs, 15+ barrels, plump seeds x10. Second miner zasit labor-set (pick
count unknown — depot changes that equation). NEXT: aquifer-piercing
(z=116-115 soil, 2 layers, WEST of the fort, far from the brook), then
stone → forge/smelter tests + industry.

## 2026-07-15 — Fort #3, session 1 (Spring y100, day 14→49; ended by DF CRASH)

FRESH EMBARK, new map. Session goals: live-verify the 2026-07-14 tooling wave
(first time any of it touched a live game) and open the first fort built from a
whole-map plan (fort-planning skill). Ended early: DF crashed when the overseer
opened our stockpile's custom-settings screen in the UI — see gaps below.

**Site**: 96x96, surface z≈138 (range 138..145), 4 soil layers, stone from
z=133. TWO aquifers: a soil aquifer z=135-134 wetting only the NORTH half
(boundary ≈ y=46), and a map-wide THICK STONE aquifer z=126-122 (5 layers).
Ravine at x≈13-19 with a 7/7 stream at z=128 (bank access undug). Big SW
valley, surface down to ~z=128. Wagon (48,47,138).

**The plan worked**: sited the 2x2 spine at (50-51,54-55) in the dry south —
full descent surface→z=127 with ZERO aquifer piercing. Level map assigned at
embark: 137 farming / 136 food stockpile / 135-134 reserve / 133 industry
(loop topology, 9x9 halls E+W, satellite 2x2 stairs in each) / 132 industry
stockpile / 131 services / 130-129 housing / 128 crypt / 127 frontier stop.
Bilateral symmetry off the spine throughout; odd-width (7-wide) rooms.

**Fort state at crash (day 49)**: spine carved 138→127 (hatch built over
(50,54,138)); z=137 west = 2 dug 7x7 farm halls — hall A: 4 built 3x3 plots
(plump helmet/pig tail/cave wheat/quarry bush, all seasons, planted!), hall B:
7 built beds + 7 assigned 1x1 bedroom zones (has_bedroom_zones_7 ✓); z=137
east starter dorm/dining still designated-undug; z=133 industry ~70% dug, W
satellite stairs done, E in progress, 'all' stockpile (40,50)-(44,52) placed
(see CRASH bug); 19+ chert boulders, 57 logs, 7 beds+1 hatch+3 tables+3 chairs
made (mosus hit Carpenter Lvl6, one masterpiece bed); drinks 12 (flat all
session, odd), food thin but fisher active. Deep-aquifer pierce designated
(spine 127→120) but never reached by the miner — z=132/136/south-133
designations were cancelled to pull it forward; single-miner throughput was
the wall.

**Wave verification results** (the point of the session):
- list_crops DLL probe ✓ (157 crops, seeds-on-hand correct)
- BREWING ✗✗ FAILED — list_reactions returns EMPTY (even unfiltered);
  queue_job reaction=BREW_DRINK_FROM_PLANT → "reaction not enabled in
  fortress mode". Drink chain blocked a THIRD fort. Top engineering item.
- Farming ✓ end-to-end (plots→assign→PlantSeeds jobs observed) with TWO bugs:
  (1) assign_crop vs under-construction plot returns SUCCESS but does NOT
  stick in-game — must re-assign after the plot is BUILT (overseer confirmed
  in-game); (2) a plot tile that was undug wall at stamp time never registers
  with the building — assign_crop at that tile says "no building" forever
  while buildings lists the plot AT that exact coord (self-contradiction).
- Hatch ✓ (queue_job ConstructHatchCover → stock → build hatch at shaft top)
- remove_zone ✓ (designate→remove→gone; ACK severity mislabeled PARTIAL)
- Overlay honesty ✓ 'd' persists on in-flight digs; caveats: revealed
  SURFACE tiles and just-revealed wall faces can render plain while queued
- look scope=elevation ✓ (full 96x96, ~9.6k tok) — but aquifer count with NO
  spatial overlay ("4608 in view" = which half??); scope=fort ✓ at z=137 but
  BUGGED at z=133/136: bbox polluted by ambient tile updates (SW valley
  water/grass), rendered the wrong region entirely
- dwarves verbose ✓; survey_site surface RANGE ✓
- save_blueprint PARTIAL — captured only 28/49 tiles of a fully-dug 7x7 hall
  (suspect farm-plot-occupied tiles excluded); apply dry_run mechanics ✓
- step repeat-warnings line renders ("no repeated warnings") but no
  damp-cancel occurred; the xN path is still unexercised live
- STOCKPILE BUG (CRASH): stockpile category=all placed OK but shows in-game
  as "custom", and opening its custom settings CRASHED DF, ending the
  session. Plugin likely sets category flags without populating the
  per-category item vectors v50's UI expects. Under investigation.
- set_labor: flags verifiably stick (read-back after 30+ days) but the 2nd
  MINE dwarf never mined. Hypotheses: single embark pick (v50 needs a pick
  in hand) vs v50 work-details overriding raw unit labor flags. Under
  investigation (DFHack source agents).

**Process notes**: turn cadence solid; connector suggestions now emit proper
L-legs (fix confirmed live) but still miss adjacency to carved spine stairs
(false "not connected, nearest open tile at map edge"). stocks/list filters
are CASE-SENSITIVE ("weapon"=nothing, "WEAPON"=works). No wagon glyph exists
in look renders (wagon visible only via buildings).

## 2026-07-14 — Fort #2 ("First Fort round 2"), session 1 (Spring y100, day 14→45)

FRESH EMBARK, new world. Session goals: live-verify the new Zones+Locations
tools, and pierce/seal the aquifer UNSUPERVISED using only these memory files.

- **AQUIFER PIERCED AND SEALED UNSUPERVISED — the protocol worked.** Site:
  surface z=131, single-layer sand aquifer at z=128 (thinner than Fort #1's),
  stone from z=126. Timeline: shaft z131→129 (day 14-17), pierce designation
  day 17, loud damp-cancel day 18, re-designate → pierced to z=126 by day 19,
  quarry z=126 for mudstone (non-economic layer stone — no bauxite trap this
  time), 8-tile orthogonal ring mined day 27, ALL 8 walls built by day 29,
  residual 2-tile puddle fully evaporated by day 38. Zero inflow since.
  ~11 game-days pierce→seal, zero human coaching. Protocol deviations:
  ring dig took THREE designation rounds (protocol said expect two) with NO
  alert feedback (announcement dedup swallows repeat damp-cancels); improved
  order-of-operations: quarry stone BEFORE mining the ring so walls start
  instantly.
- **New tools all live-verified**: designate_zone (bedroom/dormitory/barracks/
  animal_training/meeting_hall/water_source all placed), assign_zone (coord-
  addressed; ownership shows in list_zones), unassign_zone (roster cleared),
  create_location tavern (list_locations correct), assign_lodging (ACK
  SUCCESS, list_locations shows "1 lodging room(s)"). check_goals
  has_bedroom_zones went 0→1 (needs a step() after zone creation — stale
  snapshot reads 0 at first, NOT a broken predicate).
- **Lodging in-game effect: UNVERIFIED (pending, not null).** Wire path fully
  works; no visitor has arrived by day 45 (expected — 7-dwarf fort, low
  wealth, visitors take seasons). Keep watching next session.
- Error-text quality confirmed excellent: Barracks assign → "uses squads,
  not units — see future military/squad workstream"; AnimalTraining assign →
  "labor-driven, no roster". Both accurate and educational.
- **BrewDrink CONFIRMED still blocked** (live re-test this fort): no job_type
  named Brew*; brewing is CustomReaction (needs job->reaction_name);
  queue_job CustomReaction cleanly rejected by workshop whitelist. Drink
  chain dead until the reaction-based path lands.
- DISCOVERY: designation overlays now painted in look ('d' glyph) — landed
  from the 009 wishlist. Caveat: UNDER-REPORTS — tiles with in-flight dig
  jobs render as plain wall; do not diagnose "cancelled" from a missing 'd'.
- Fort state at pause (day 45): shaft z131→126 sealed through aquifer;
  z=130 base room (food stockpile + meeting hall/TAVERN + 2 junk test zones,
  no zone-delete tool exists) + dorm room (4 beds built: 1 owned by etur,
  1 tavern-lodging; 3 more beds in stock, unplaced); z=126 quarry (carpenter
  + still built) + W annex mostly dug; 13 boulders, 22 logs, 12 drinks,
  food THIN (~12 units + 8 raw fish), farming still tool-blocked (no farm
  plot build type). Water: 7/7 stream found far W in gully (z=141), water_
  source zone painted on bank (4-9,71,142). Second miner (meng) enabled
  pre-emptively at embark — single-miner default-embark pattern confirmed
  again.
- Next session: place 3 remaining beds + bedroom zones (goal 7), dining
  hall (tables/chairs via carpenter queue_job), fishery for the raw fish,
  watch for migrants/visitors (lodging verification), food pressure watch.

## 2026-07-12 — First Fort, session 2 continued: pre-009 verification pass (day 103-104)

- Reconnected post pre-009-blocking-fixes wave. Census fix LIVE-VERIFIED
  immediately: `dwarves` now reports 7, not 18.
- set_labor LIVE-VERIFIED: dwarf_detail now shows full labor lists.
  Revealed the real shape of the labor collision — Doren isn't a
  specialist blocked by ONE competing labor, he's the fort's only
  MINE-enabled dwarf among ~80 labors each dwarf carries by default
  (confirmed: Solon has every labor except MINE). Applied the practical
  fix: enabled MINE on Solon (idle, stone-working skills) as a second
  miner. Verified via read-back, stepped once to confirm no errors.
- job_types filter confirmed live: "hatch" correctly resolves
  ConstructHatchCover by name — generalized construction's name
  resolution works end-to-end; the remaining hatch-cover gap is now just
  the one-line jobTypeAllowedAtWorkshop whitelist entry, not a rebuild.
- Session pauses here per overseer direction — pivoting to 009
  (Culture & Learning) design work. First Fort's core loop is proven;
  remaining fort progress (drink chain, dining hall, migrant wave,
  year-1 survival) continues opportunistically rather than as the
  primary focus.

## 2026-07-12 — First Fort, session 2 (Spring, year 100, day 62→69)

- Model handoff: Fable 5 -> Sonnet 5 mid-project. Picked up right where
  session 1 left off (aquifer sealed, descent to z130 designated).
- Fix-wave verification LIVE: real calendar date confirmed (year=100
  season=spring day=62, was year=0/season=?/day=0), chop/gather now work
  (3 logs -> 13 across 4 species from one chop designation; gather brought
  in cotton/lettuce/bitter-melon plant materials), aquifer seal holding
  (only a ~1/7 residual puddle, DAMP/AQUIFER annotations render correctly
  in cross_section), carpenter workshop completed once shale boulders
  existed, `buildings`/`stocks` material+economic fields all live-correct.
- DISCOVERED: manager work orders (the `order` tool) need a Manager noble
  + office (chair+table+door — all buildable today) to ever leave
  validated=true/active=false. No tool exists to assign a noble or claim a
  room — a NEW gap, distinct from the already-known zone/civzone stub.
  Separately confirmed (before it could bite): work_orders.cpp was setting
  manager_order.mat_type=-1, which is DFHack's INVALID-material sentinel,
  not "any material" (mat_index=-1 is correct; mat_type's default is 0) —
  fixed alongside.
- ENGINEERING DETOUR (mid-session): implemented `queue_job`, a new plugin
  command (Job::linkIntoWorld + Job::assignToWorkshop) that queues a job
  directly at a workshop, bypassing the manager entirely — mirrors
  right-clicking a workshop in vanilla play. Researched via DFHack source,
  adversarially reviewed, plugin rebuilt + redeployed (2nd DF restart this
  session), Go server reconnected to pick up the new tool.
- LIVE-VERIFIED: `queue_job` x2 bed at the carpenter workshop -> both
  ConstructBed jobs completed (~1 day each, consuming a wood log each) ->
  `build bed` x2 at (48,48,139) and (48,52,139) -> BOTH BUILT. First beds
  of the fort exist. Material-filter guess (item_type=WOOD at Carpenters)
  was correct on the first live try.
- Reconnect quirk (both times this session): a fresh `/mcp` + `ai-connect`
  cycle reports "Connected" immediately but returns sentinel data
  (year=0, dwarves=0) for one or more queries until a `pause` call — not
  reliably the FIRST query afterward, took 3-4 calls the second time.
- DWARF CENSUS CORRECTED (user caught this): the `dwarves`/`dwarf_detail`
  tools return 18 units, but sampling all 18 via dwarf_detail showed only
  7 have first_name + dwarf-typical labor skills (Doren/Miner, Dumed/
  Carpentry, Kulet/Metalcraft, Fath/Fish, Solon/Masonry, Mafol/social
  skills, one garbled-name Plant/RecordKeeping) — the other 11 are unnamed
  with either zero skills or just CLIMBING 15 (pack animals/pets). True
  fort population is 7, matching the default embark. Dashboard's
  `dwarves=18` count is misleading — needs a citizen/race filter upstream.
- ALL 7 BEDS BUILT (queue_job x7, build bed x7) — full sleeping coverage,
  first real headroom milestone of the fort. One retry needed: a bed plan
  at (52,51,139) died silently after the first attempt (tile was fine,
  loose bed stock was plentiful) — re-placed clean per the existing
  "RE-PLACE any dead plan" learning.
- Still workshop built at (45,49,134) but CANNOT be given a job yet:
  BrewDrink has no job_type mapping in protocolToJobType (returns -1) in
  either order or queue_job — needs a DFHack reaction-based lookup, not a
  plain job_type enum value. Drink chain stays blocked until that lands.
- Design note (user): building/furniture glyphs in `look` need to be
  CATEGORY-level (workshop/furniture/door/stockpile/trap, ~5-8 glyphs),
  not one-per-type — DF has 50+ building/furniture/construction types,
  far more than the remaining ASCII budget after terrain glyphs. Exact
  type stays a `buildings`/`building_status` detail-query, matching the
  existing progressive-disclosure pattern. Belongs in 009's already-scoped
  "designation + building overlays painted into look" workstream.

## 2026-07-12 — First Fort, session 1 (Spring, year 100)

- OUTCOME: **AQUIFER SEALED.** All 8 constructed shale walls stand around the
  2x2 staircase at z136; pocket entombed west; zero inflow remaining. ~28
  game days elapsed (frame 5350→32237). Descent resumed via dry offset shaft
  from the east quarry hall (54-55,50-51) z134→130. Carpenter workshop
  re-placed. Session ended at a clean pause with the overseer (human)
  coaching the aquifer protocol — full tool-gap list filed in the session
  report for engineering.

- First MCP-driven session on a fresh default embark. Connection chain worked
  first try (df-mcp listener → ai-connect).
- Dug 2x2 stair shaft (46,50)-(47,51) from surface z140 down; 5x5 storage
  room at z139 east of shaft; carpenter workshop placed at (50,50,139) but
  NOT yet built (material problem, see below).
- STAIR QUIRK INCIDENT: continuing a damp-cancelled shaft by designating
  stairs starting at the undug level (z136) carved a down-stair with no up
  component — the level below became permanently unreachable and the old
  column below 137 is abandoned (now a sealed, slowly-filling water pocket
  at (46-47,50-51,136)). Recovered by digging a side corridor at z137 and
  sinking a parallel 2x2 shaft at (49-50,50-51), z137→134.
- SAND AQUIFER at z136 (whole layer per overseer). Pierced it on the new
  shaft (2nd designation digs after DF's warning-cancel). Seal in progress:
  6-tile orthogonal ring mined on N/E/S sides, ONE stone wall built at
  (49,49,136); remaining 5 walls blocked — no usable non-economic boulders.
  West side (48,50-51) still natural sand, pocket-adjacent; plan is to mine
  + wall it LAST, with spare boulders staged, accepting the pocket dump
  (drains to 134 and evaporates).
- Quarries at z134: east room struck BAUXITE (economic → unusable for
  constructions by default). West quarry (44-46,48-52) in progress hunting
  layer stone.
- Embark stocks reality: 3 logs (untouched by wall jobs), 3 "ROCK" items
  that are NOT construction boulders, 0 real boulders at start. 12 drinks,
  ~30 food. Fisherdwarf is producing (raw fish 19→26).
- Time elapsed: ~14 game days (sim frame 5350 → 22868). No deaths, no
  enemies. Dig/step/alert loop feels solid.
