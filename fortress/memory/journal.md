# Fort Journal

(newest entries on top)

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
