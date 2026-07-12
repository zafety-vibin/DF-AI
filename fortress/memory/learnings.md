# Learnings

(append-only; never delete)

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

## Pacing

- Steps of 400-800 ticks during active water/crisis events; 1200-2400 when
  stable. During heavy fluid simulation a step can exceed the tool's poll
  deadline ("DID NOT complete") — call status/pause to resync, don't panic.
