# Learnings

(append-only; never delete)

## Fort #6 (2026-07-22): a flat dig rect silently REPLACES a stair DESIGNATION it overlaps — the warning only guards carved stairs

- Live sequence: designated a 2x2 stairs shaft (131→130), then designated
  the room rect at 130 overlapping the stair tiles — the stair
  designation was converted to plain mine with NO warning (the
  dig-over-stairs warning fires for CARVED stairs only; a
  designated-not-yet-carved stair is just another pending designation
  and loses). Overseer caught it in-client.
- Rule: designate room rects FIRST, stair shafts LAST — or exclude the
  stair footprint from the room rect. Re-issuing the stairs designation
  after the fact repairs it cleanly (carved tiles skip, pending tiles
  re-convert).
- Tool-hardening candidate: warn when any designation overwrites a
  pending stair/ramp/channel designation of a different type.

## Fort #6 "Lanehold" (2026-07-22): the region13 soil aquifer is LIGHT — and this pierce was the project's driest

- Inferred per aquifer-piercing §0 from in-dig behavior (embark tag not
  captured): 2x2 stair carved THROUGH z=135 (aquifer-flagged at all 4
  corner bores) + all 8 ring tiles mined, and NO standing water ever
  appeared — 5+ game-days open, zero depth digits, zero wall-job
  suspends. Worldgen-fixed: treat region13's upper soil aquifer as
  LIGHT for every future descent at this site.
- Sequence that worked (single-wet-level variant): shaft frozen at 136 →
  stairs 136→133 in one call (loud damp cancel once, re-designate, then
  a SECOND loud cancel appeared for new tile contact — re-designate
  again) → quarry 133 first (§3) → 8-tile orthogonal ring at 135 → 8
  constructed walls same day (material=any: mudstone + wood mix).
- Miners prioritize nearest jobs: the 133 quarry starved the 135 ring
  dig until the quarry designation was temporarily cancelled — the ring
  opened one step later. Cancel-competing-digs is a legitimate
  scheduling lever during a pierce (re-designate after; free).

## Fort #4 (2026-07-16): a `mine` designation can silently destroy an existing stair's up-component

- Designating `type=mine` over a region that overlaps an already-carved
  2x2 up/down stair core converts the overlapped tiles' `UpDownStair` to
  plain `DownStair` — no warning, no alert, just a severed connection to
  everything above the level you're digging. Caught only because the
  overseer was watching the live game and noticed dwarves stranded.
  RULE: when designating a room/hub that might touch an existing stair
  shaft, explicitly carve AROUND the 2x2 core (exclude those exact tiles
  from the mine rectangle), never assume overlap is a safe no-op the way
  it is for re-designating already-open plain floor.
- If it happens anyway: re-issuing `designate_dig type=stairs` on the
  broken tiles does NOT fix it (ACK: "already carved", 0 designated —
  the plugin's re-designation check is carved-or-not, not stair
  sub-type-aware). The real fix is `build type=updownstair` (the
  registered build-tool name; "stairs" alone is not recognized) directly
  on the broken tile — converts it to a real, functioning up/down stair
  using one boulder. Verified end-to-end this session: tested clean on
  virgin floor first, then applied to the actually-broken tiles, full
  shaft connectivity confirmed via cross_section afterward.

## Fort #4 (2026-07-16): bore-check the actual room footprint, not one representative point

- Bore-checking ONE central point per cardinal direction (e.g. one tile
  south of a planned cluster) is not enough — this map had a genuine
  aquifer pocket sitting under a room whose footprint was offset just
  2-3 tiles from the single point that had tested clean. Second
  independent incident of "aquifer patchiness" this project (see the
  Fort #3 "aquifers come in pairs" entry below) — the practical fix is
  the same: bore EVERY room's actual corners/center before digging it,
  not a single sample for the whole direction.
- A stone-aquifer-adjacent leak that appears AFTER a room is dug (a
  water tile showing inside an already-carved, previously-dry-testing
  room) can be sealed with `smooth mode=wall` on the exposed damp/
  aquifer-flagged wall tiles bordering it — no boulders or construction
  needed, matching the stone-aquifer branch of the aquifer-piercing
  protocol, just applied reactively instead of as part of a planned
  pierce. Confirmed this stops the leak from spreading further (tile
  count stopped growing after smoothing), though full evaporation of an
  already-formed puddle still takes the usual several game-days.
  CAVEAT: `look`/`cross_section` never visually confirm a `smooth` call
  succeeded — the wall renders identically before and after. Trust the
  ACK and the leak's behavior (stopped spreading), not the overlay.
- Overall verdict on a level sitting only 1-2 z below a freshly-sealed
  aquifer: usable, but expect to lose some fraction of rooms to real
  wetness no matter how careful the bore-checks are. For the NEXT
  housing cluster, go several levels further down for a firmer buffer
  rather than fighting this level tile-by-tile.

## Fort #4 (2026-07-16): `stocks` counts built-in furniture as available stock

- `stocks category=bed` (or door, table, etc.) reports the TOTAL count of
  that item type across the fort, including ones already incorporated
  into built furniture — not just free/loose/haulable stock. Led to
  repeated confusion this session: saw "BED: willow x7", assumed spares
  existed, placed a `build type=bed` plan, and it sat cancelling "needs
  bed" indefinitely because all 7 were already built into other rooms
  and zero were actually free. The only way to know true free stock right
  now is indirect: if a placed building keeps cancelling on "needs X"
  despite `stocks` showing count > 0, assume zero real spares and queue
  fresh crafting via `queue_job` rather than trusting the number.
  Flagged as a tool bug (docs/decisions.md) — `stocks` should exclude
  incorporated items or expose a separate in-use count.
- General pattern reconfirmed hard this session: `build` plans die
  SILENTLY after repeated material-shortage cancels, exactly as the
  Construction-materials entry below already warned — but this time the
  shortage was invisible because `stocks` lied about availability. Stay
  ahead on furniture production (queue crafting BEFORE placing, keep a
  buffer of spares) rather than placing first and discovering the
  shortage via a cancel loop — this is a real DF-wide pattern the
  overseer hits in normal play too, not unique to this tooling.

## Fort #4 (2026-07-16): dig-ahead can starve — and endanger — single-miner aquifer work

- With only one effective miner (the "one pick = one miner" constraint,
  still in force), queuing an UNRELATED bulk designation (a bedroom
  cluster) at the same time as an active aquifer pierce/seal doesn't just
  slow the aquifer down — the miner's nearest-job dispatch can genuinely
  interleave the two, leaving the aquifer half-sealed and leaking longer
  than necessary. Worse: the competing dig was sited without re-checking
  for hazards and came within one row of breaching a separate 7/7-depth
  standing-water pocket — a second instance of the "creek near-miss"
  failure class, this time self-inflicted by not bore-checking a new
  room's far edge before designating it.
  RULE: while a single miner is mid-aquifer (from first pierce through
  final wall), do not queue ANY other mine-type designation. Cancel
  anything competing immediately if caught mid-flight
  (`cancel_designation` is free and instant). Only resume unrelated
  digging once the ring is fully walled and confirmed dry.
- Confirmed live: a STACKED multi-layer aquifer (two independent soil
  layers, z=116 and z=115, each individually flagged AQUIFER DAMP) takes
  the exact same pierce→ring→wall cycle as a single-layer aquifer,
  just repeated once per layer — no protocol changes needed, per
  aquifer-piercing skill §8. Quarry-then-ring-then-wall order held for
  both layers using one shared boulder bank (12 mudstone covered both
  8-tile rings with room to spare).
- A designate_dig ACK's "already carved" skip count is not fully
  trustworthy at the single-tile level: one core shaft tile
  (46,44,114) was silently left un-carved despite its neighbors in the
  same 2x2 core completing normally, with no error surfaced anywhere —
  only caught by noticing a residual water tile at (46,44,115) refusing
  to fully drain days after the rings around it were sealed. Fix: if a
  supposedly-sealed aquifer keeps one damp tile alive well past the
  usual 5-10 day evaporation window, cross_section EVERY tile of the
  shaft core individually (not just one representative corner) before
  assuming the seal failed — it may just be one skipped tile needing a
  direct single-tile re-designation.

## Fort #4 (2026-07-16): stockpile/workshop/build tool-occupancy gaps

- A `stockpile` designation and a `build` workshop placement fight over
  the same tiles depending on ORDER: placing the stockpile first and
  then trying to `build` a workshop on part of its rectangle FAILS
  ("tiles blocked, occupied, or unsuitable") even though vanilla DF
  allows building on a stockpile tile. Workaround: always `build`
  workshops FIRST, then designate the stockpile over the remainder —
  the stockpile tool correctly skips tiles already occupied by a
  building, but build does not reciprocate.
- `remove_building` addresses by a single (x,y,z) coordinate, and when a
  stockpile's designated rectangle and a workshop's 3x3 footprint happen
  to share that exact tile, it can resolve to the WRONG building — in
  this session it deconstructed a Still workshop when the intent was to
  shrink an overlapping Stockpile at the same reported anchor tile.
  Confirmed by the building list showing the Still degrade from
  "built" → "under construction (stage 0/3)" → gone entirely, while the
  Stockpile it was meant to target stayed listed as built the whole
  time. Mitigation until fixed upstream: pick a removal coordinate that
  is UNAMBIGUOUSLY inside only the intended building's own footprint,
  never a shared/boundary tile, and re-check `buildings` immediately
  after any remove_building call to confirm the right thing disappeared.
- Loose items sitting on the floor (e.g. goods already hauled into a
  stockpile) are INVISIBLE to `look` — the tile renders as plain floor —
  but still block a new `build` placement with the same generic "tiles
  blocked, occupied, or unsuitable" error as an actual building conflict.
  There is no tool today to query item-on-tile occupancy directly.
  Workaround: if `build` fails on a tile that `look` shows as clean open
  floor and no building is listed there either, suspect item clutter and
  try a tile that was NEVER inside any stockpile designation, rather than
  waiting for hauling to clear it.

## Fort #4 (2026-07-15): brooks hide their water one level down

- A brook renders as walkable surface (grass/floor at surface z) with its
  actual 7/7 water channel HIDDEN one z below. Surface looks and even a
  direct cross_section bore can read the channel as "dry hidden soil"
  (tool gap, fix in flight) — the only honest views were look/elevation
  AFTER nearby digging revealed the tiles, and DF's own "Dangerous
  terrain" miner refusal. RULE until the bore fix is live-verified: on
  any map with open water, elevation-view EVERY z-level you plan to dig
  (water renders as digits there once revealed) and treat "Dangerous
  terrain" cancels as a WATER alarm, not an anomaly. Keep a >=2-tile
  undug bank buffer; never designate adjacent to a known channel.
- Breaching a river/brook is WORSE than an aquifer pierce: infinite flow,
  no seep delay. The aquifer protocol does not apply to rivers.

## Fort #4: DFHack updates arrive silently via Steam

- The plugin loader's exact version-string match means a Steam DFHack
  auto-update (53.15-r1 → r2 mid-day) bricks the plugin with a bare
  "load failed". stderr.log in the DF folder names both versions —
  read it FIRST on any load failure. Rebuild = retarget checkout tag +
  submodule update + rebuild + redeploy (~10 min). A HOT SWAP works
  with DF running: unload plugin in console → copy DLL → load → connect.

## Caravan access (overseer doctrine, 2026-07-15 — for future fort design)

- The FIRST caravans arrive with pack animals (horses/mules) that can use
  stairs — an early underground depot reachable only by stairs still
  trades with them. A full WAGON caravan cannot use stairs at all: it
  needs a true excavated RAMP path (3-wide, wagon-passable slopes) from
  the surface down to the depot. Advanced fort design therefore puts the
  trade depot INSIDE the fort in a defensible chamber, reached by a
  dug sloped wagon road (designate_dig type=ramp — the verb exists
  today). This is deliberate later-game work: plan the ramp corridor
  into the whole-map layout, don't retrofit it. When bridges/levers are
  live, the wagon road is the natural place for the drawbridge airlock.

## Fort #4: the drink chain, verified recipe

- Full working chain (first success in project history): still built from
  a log → list_reactions filter=brew → queue_job
  reaction=BREW_DRINK_FROM_PLANT at the still → needs a brewable PLANT
  stack AND an EMPTY barrel (embark barrels are all full; queue 2-3
  barrels at a carpenter first or the job cancels "needs empty food
  storage item") → drink stack appears, seeds return (+1 seed per plant).
  Wire lesson: the reaction catalog reads the civ's
  permitted_reaction_ID vector (resolved indices); *_str vectors in
  entity raws are load-time staging, EMPTY at runtime — never gate on
  them (same dead-field class as reaction_flags.FORTRESS_MODE_ENABLED).

## Fort #3 (2026-07-15): farm-plot assignment timing

- assign_crop against a farm plot that is still UNDER CONSTRUCTION returns
  SUCCESS but does NOT take effect in-game (overseer verified). Always
  re-assign every plot's crops AFTER `buildings` shows it "built" (stage
  3/3), and treat any pre-construction assign as a no-op. Watch for
  PlantSeeds jobs in dwarves verbose as the true "it worked" signal.
- A plot tile that was undug WALL when build_farm_plot was stamped never
  registers in the tile→building index: assign_crop at that tile errors
  "no building" forever even though `buildings` lists the plot at that
  exact coordinate. Workaround: address the plot via any tile that was
  already open floor at stamp time. Better: only stamp plots on fully dug
  floor.

## Fort #3: one pick = one miner (v50)

- Second fort-in-a-row set_labor MINE read-back verified the labor flag
  sticks — and the second dwarf STILL never mined in 30+ days with
  hundreds of pending designations. v50 requires a pick IN HAND to dig
  (soil and stone alike); the labor makes a dwarf willing, the pick makes
  them able. Default embarks are not guaranteed two picks. No tooling
  path to more picks exists yet (no smelter/forge build types; no trade
  depot), so mining throughput is capped at embark pick count. Count
  picks (or infer from who actually mines) BEFORE planning dig volume.
  VERDICT (2026-07-15 DFHack-source investigation): pick shortage CONFIRMED
  as the cause; the work-details hypothesis is REFUTED. v50's engine still
  reads unit.status.labors[] directly for job dispatch (DFHack's active
  autolabor plugin writes the same field; our applySetLabor at
  df_ai_protocol.cpp:420 is correct and sufficient). work_details
  (plotinfo->labor_info.work_details) are a one-directional Labor-tab UI
  layer that bulk-WRITES those same flags — the engine never reads them
  for dispatch, which is also why our flag persisted 30+ days untouched.
  DF's own dig tooltip states "The miner requires a pick" as a separate
  constraint. set_labor needs no fix; pick supply is the real ceiling.

## Fort #3: aquifers come in pairs; site the spine by bore triangulation

- One site can carry TWO independent aquifers: a patchy soil aquifer just
  below the surface (north half only, boundary ≈ one bore apart) and a
  thick 5-layer stone aquifer far deeper, map-wide. survey_site's per-
  column "AQUIFER at z=N" only reports the FIRST flagged z per column —
  bore several columns (cross_section) and compare before believing any
  single "the aquifer is at z=N" claim.
- Payoff: triangulating 5 bores let the spine descend 11 z-levels to one
  level above the deep aquifer with ZERO piercing. Whole-map planning
  pass before the first dig is worth every call it costs.
- STONE aquifers seal by SMOOTHING the damp walls (any dwarf, no pick, no
  boulders) — completely different resource profile from the sand/soil
  protocol. Not yet exercised live (the pierce never started); the plan
  stands for next session.

## Fort #3: tool-honesty patterns worth internalizing

- stocks/list filter strings are CASE-SENSITIVE ("weapon" → nothing,
  "WEAPON" → results). When a filter unexpectedly returns empty, retry
  uppercase before concluding absence.
- `look scope=fort` bbox is polluted by AMBIENT tile updates (valley
  water flow, grass): on some z-levels it renders a distant wild region
  instead of the fort. Trust it only when the bbox visibly contains your
  own work; otherwise fall back to look local / elevation.
- The "not yet connected" connector hint can false-positive when the new
  designation directly abuts already-carved SPINE STAIRS on the same z
  (claims nearest open tile is at the map edge). Informational only —
  verify adjacency yourself before adding connector corridors.
- Dwarves' nearest-job mining means dig-ahead designations STARVE priority
  work: the deep pierce sat untouched for 13 days behind ~200 stone-hall
  tiles. When the overseer orders a specific dig, CANCEL competing bulk
  designations (they're free to re-issue later) rather than waiting.

## Aquifer protocol — refinements from Fort #2 (first unsupervised run, SUCCESS)

- The written protocol below WORKS without coaching — Fort #2's single-layer
  sand aquifer was pierced and sealed in ~11 game-days solo. Refinements:
- QUARRY STONE FIRST: dig to stone below the aquifer and quarry boulders
  BEFORE mining the seal ring, so all 8 walls can be queued the moment the
  ring opens. Fort #1's seal stalled for days on missing boulders.
- Ring mining can take THREE designation rounds, not two — and announcement
  dedup means you get NO alert for repeat damp-cancels. Don't diagnose; just
  re-designate each turn it hasn't dug. Designations are free.
- Distinguish the cancel-source: a ring tile adjacent to tiles with ACTUAL
  water re-cancels each attempt; re-designating is still correct (builders
  work in ≤3-depth water) but expect more rounds the wetter the shaft is.
- After a good seal the catch-basin puddle (depth 1-2) fully evaporates in
  ~5-10 game-days. A dry shaft bottom is the definitive seal-verified signal.
- Single-layer aquifers (1 z-level) are dramatically easier than multi-layer:
  same protocol, but the seal ring is one ring, and the basin stays shallow.

## Designation overlay in `look` — it exists now, but UNDER-REPORTS

- 'd' overlay glyphs landed (009 item). But tiles whose dig job is in flight
  (or shortly queued?) can render as plain wall '%'/'#' with no 'd'. A
  designation you can't see is NOT necessarily cancelled — check again after
  a step, or watch whether the tiles eventually dig, before re-designating
  in a panic. (Misread this twice in one session before catching on.)

## Zones & Locations (first live session with these tools)

- Zone type vocabulary is snake_case lowercase ("meeting_hall",
  "animal_training", "water_source"); matching is case-insensitive but NOT
  camelCase-tolerant ("MeetingHall" fails, "meeting_hall"/"Bedroom" work).
- assign_zone/unassign_zone address the zone by ANY TILE INSIDE IT (x,y,z),
  not by id. Same pattern for assign_lodging (tavern_* + bedroom_* coords).
- Zones claim EXISTING carved floor/grass only — painting over open air or
  undug wall fails per-tile with a truthful error ("zones claim existing
  space, they don't dig").
- check_goals reads the world-model snapshot: a zone created while paused
  shows as 0 in predicates until the next step() refreshes state. Step ~100
  ticks before trusting predicate counts after zone edits.
- The tavern flow that works end-to-end: designate_zone meeting_hall →
  create_location tavern (any tile in the hall) → designate_zone bedroom
  (with a built bed) → assign_lodging. list_locations then shows
  "Tavern ... — N lodging room(s)". In-game visitor behavior NOT yet
  observed (needs seasons + fort wealth); wire path verified only.
- There is NO zone deletion tool — test zones are permanent squatters.
  Don't paint junk zones in rooms you care about.

## Default embark: the single-miner pattern is universal

- Second fort in a row: every starting dwarf has ~80 labors EXCEPT mine;
  exactly one dwarf has MINE. Check the roster at embark and set_labor MINE
  on a second dwarf (mason/stone-skills types are natural picks) BEFORE the
  first big dig — done pre-emptively this time and digging never stalled.

## Designation overlays in `look` — second occurrence

- SECOND live instance of the room/shaft-gap error class (first was
  session 1: a bedroom placed with only a mental-math relationship to the
  stair shaft). This time: designated a new 6x6 room 4 tiles away from
  existing floor with no connector, then burned 4800 ticks before
  noticing zero dig progress — `look`'s "designated for digging: N tiles
  in view" is a COUNT, not a spatial overlay, so the gap between existing
  floor and the new designation was invisible until reasoned about
  manually. Confirms 009's "paint designation + building overlays into
  look" workstream item is high-value, not speculative — two independent
  real mistakes now trace to the same missing capability. Practical
  workaround until it lands: always look at the BOUNDARY between existing
  floor and a new designation before stepping, not just the new room's
  interior.

## find_dig_site reliability

- CONFIRMED (not just suspected): find_dig_site can claim a region is
  "fully solid" for tiles that are ALREADY CARVED FLOOR from an earlier
  dig in the same session (re-offered the exact footprint of an already-
  built storage room as a fresh "solid" candidate). Don't trust its
  candidates blind near recently-developed space — cross-check with
  look before designating, especially when re-querying near a room you
  just finished. Engineering: audit the plugin's solidity check for
  staleness (cached tile classification not refreshed after a dig?).
- FIXED in the 2026-07-12 wave (commit 062455f): root cause was
  `dfhack-plugin/tile_updates.cpp`'s `detect_tile_changes()` hardcoding
  every TILE_UPDATE delta's flags byte to `0x02` (`FLAG_DISCOVERED` only)
  instead of computing real wall/floor/liquid flags — none of the bits
  `topology.ClassifyState` checks were ever set, so every delta-updated
  tile (including a freshly-dug room floor) landed in `StateUnknown`,
  which `mapview.FindDigSites`' solidity check counts as solid ground.
  Fixed by routing `detect_tile_changes()` through the same
  `compute_tile_flags()` the full-state extractor already used (a live
  `MapExtras::MapCache` + `raw_block->designation` read), so deltas now
  carry honest flags. The Go consumer chain (`ClassifyState`/
  `TopologyOverlay`/`finder.go`) needed no change — it was already
  correct given truthful wire data; confirmed via two regression tests
  in `internal/worldmodel/populator_test.go` exercising
  `Populator.OnTileUpdate` → `FindDigSites` with a fixture room (one
  locks in current behavior, one pins the pre-fix degraded-flags failure
  mode as a tripwire). The "cross-check with look before designating"
  workaround above is retracted for tiles reached via delta updates —
  find_dig_site is a trustworthy live-equivalent source again post-fix.

## Doors & hatches (overseer tip)

- HATCHES go over stairwell openings (any tile where a stair meets open
  air/surface — e.g. the top of a shaft). DOORS go between two walls
  (horizontal corridor/room entrances), not over a stair tile.
- Doors require an adjacent wall OR already-built door on at least one
  side to be placeable. For a wide (multi-tile) entryway, the middle
  door(s) will be REJECTED until a flanking door nearer a wall edge is
  built first and connects to it — build outside-in, not all at once.
- Practical default: hatch the top of every shaft that opens to the
  surface (defensible entrance is a core NOW/EVENTUAL goal); doors matter
  more for interior room partitioning than for a single stairwell choke
  point.

## Stairs & digging

- **Stair continuation quirk**: `designate_dig type=stairs` treats z1 as a NEW
  shaft top and carves a down-stair (no up component). Starting a stairs
  designation at an undug level directly below existing carved stairs creates
  an unreachable level (upper X has down, lower > has no up — no connection).
  RULE: always start a continuation designation AT an already-carved stair
  level (its tiles no-op; the first undug level becomes a "middle" and carves
  up/down). If the top is a dead down-stair already, branch: dig a side
  corridor at the deepest reachable stair level and sink a fresh shaft beside
  it (a down-stair top is fine when entered from same-level floor).
- Damp/aquifer warning-cancels are LOUD ONCE then silent (announcement dedup).
  A designation that quietly vanishes = damp-cancelled. Designating the same
  tiles a SECOND time digs them for real.
- Carving stone stairs/tiles drops boulders at a low rate (8 carved stair
  tiles dropped ~1); count on quarry rooms, not the shaft, for stone supply.

## Aquifer piercing (sand / unsmoothable), verified protocol

1. Pierce the layer with the 2x2 stairs (second designation after the
   warning-cancel). Light aquifer seepage is slow — days per tile of water.
2. Freeze the descent 1-2 levels below the aquifer (cancel deeper
   designations) so seepage pools in a small catch basin, not the lower fort.
3. Mine the 6-8 ORTHOGONAL neighbors of the shaft at the aquifer level
   (corners can stay natural — seepage is orthogonal). Expect one silent
   damp-cancel round; re-designate.
4. Immediately queue constructed walls in every mined ring tile (builders
   work in shallow water). ALL FOUR SIDES must end as constructed wall.
5. Adjacent standing water (e.g. an old flooded pocket) is NOT a reason to
   skip a side: mine through, let the small volume dump and drain down the
   shaft (it spreads thin and evaporates once inflow stops), and wall the
   gap to re-isolate it.
6. NEVER build a wall ON a stair tile to "blind" a wet face — it destroys
   the staircase (the only descent).
7. After the seal: resume descent from a DRY offset (e.g. quarry room one
   level below) rather than digging through the puddle at the old bottom;
   the wet column can become a cistern later.

## Construction materials

- Constructions (walls etc.) bind NON-ECONOMIC BOULDERS only, as placed by
  the plugin. Wood logs were never selected for walls. Economic stone
  (bauxite, lignite — mineral-rich sites are full of it) is invisible to
  construction jobs unless the overseer toggles economic stone use in-game.
- Stocks item type "ROCK" is NOT a construction boulder; "BOULDER" is.
  A default embark starts with ZERO boulders — the first workshop/wall
  cannot be built until stone is mined (or from the 3 wagon logs, for
  buildings that accept wood).
- Building PLANS silently die after repeated "needs material" cancels
  ("The dwarves were unable to complete the X" = plan removed; further
  cancels may be silent). After new materials arrive, RE-PLACE any building
  that hasn't visibly completed. Completions are silent — verify with look
  (a '#' appears for walls).

## Production without a manager

- `order` (manager work order) needs BOTH a Manager noble AND an office
  (chair+table in a walled room with a door) to ever dispatch —
  `validated=true, active=false` forever otherwise. No tool exists yet to
  assign a noble or claim a room, so `order` is currently a dead end for
  a fresh fort.
- `queue_job` bypasses this entirely — it queues a job straight at a
  named workshop (the same thing right-clicking a workshop does in
  vanilla play), no noble/office required. Use it for one-off/immediate
  needs (first beds, first barrel); save `order` for bulk/standing
  production once an office+manager exist later.
- The workshop must physically exist and be built first (queue_job
  targets a tile with a real building on it). It queues ONE job per call;
  call again (or use the tool's count param) for more.

## Population counting

- `dwarves`/`dwarf_detail` return every unit at the site — pack
  animals/pets included, not just citizens. Tell them apart by
  dwarf_detail: real dwarves have a first_name and dwarf-typical labor
  skills; animals show an empty name and either zero skills or a single
  animal-trained skill (e.g. CLIMBING). Don't trust the dashboard's
  `dwarves=N` count for population planning (beds, food math) until this
  is fixed upstream — sample dwarf_detail across the full id list first.
  CONCRETE IMPACT: check_goals' has_shelter_N_per_dwarf predicates divide
  by the same inflated count — at 18 (wrong) vs 7 (true) dwarves, dug=53
  reads FALSE against a 108-tile requirement but would read TRUE against
  the correct 42-tile one. A false negative on the model's own
  self-critic, not just a display bug — fix upstream before trusting any
  per-dwarf predicate math.
- FIXED in the 2026-07-12 pre-009 blocking-fixes wave: root cause was
  `dfhack-plugin/entities.cpp` classifying entities with
  `Units::isFortControlled()`, which returns true for any TAME unit
  (its own doc comment: "includes tame animals") — every pet/pack
  animal/livestock unit is fort-controlled and tame, so they fell into
  the same branch as real citizens and got stamped ENTITY_TYPE_DWARF.
  Swapped in `Units::isAnimal()` to split that branch: fort-controlled
  AND animal now tags ENTITY_TYPE_ANIMAL, only fort-controlled
  non-animals still tag ENTITY_TYPE_DWARF. No wire protocol change was
  needed — `EntityInfo.Type` already carried ANIMAL=0x03 end-to-end
  (`internal/worldmodel` already split Dwarves/Animals/Enemies by Type),
  the plugin was just computing the tag wrong. `dwarves`/`dwarf_detail`
  and `check_goals`' has_shelter_N_per_dwarf now read the true citizen
  count. Verify against a live fort (id sample or dwarf count sanity
  check) before fully retiring the "sample dwarf_detail" workaround
  above.

## Storage strategy (overseer tip)

- Prefer ONE large carved-out room filled with an "everything" stockpile
  over several small category stockpiles scattered by room. Gets dwarves
  into the habit of hauling goods into one defendable underground spot
  rather than leaving items wherever they were made/found on the surface.
- More advanced refinement (2026-07-16, once a fort has real industry):
  one giant "all" stockpile near the top of the fort as the general
  depot, workshops further down on their own industry level, and
  SMALLER category-filtered stockpiles placed right next to each
  workshop, with that workshop's own "give to stockpile" output setting
  pointed at the nearby pile — finished goods deposit locally instead of
  hauling all the way back to the general depot every time. Corrected
  overseer note: a workshop sitting right next to (or surrounded by) a
  stockpile is good practice for shortening hauls, NOT a requirement or
  design standard — a workshop works fine standing completely alone,
  it just means longer walks for whoever's hauling its output.

## Reconnecting mid-session

- After `/mcp` reconnect + a fresh `ai-connect`, `status` reports
  "Connected" immediately but data can stay sentinel (year=0, dwarves=0)
  for several queries — not reliably fixed by the first call after
  reconnect. Call `pause` and re-check; if still sentinel, just query
  again. Don't mistake this for a broken connection.

## Pacing

- Steps of 400-800 ticks during active water/crisis events; 1200-2400 when
  stable. During heavy fluid simulation a step can exceed the tool's poll
  deadline ("DID NOT complete") — call status/pause to resync, don't panic.
