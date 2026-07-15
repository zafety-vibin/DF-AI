# Learnings

(append-only; never delete)

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
