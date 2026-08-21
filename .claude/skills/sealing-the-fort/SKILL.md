---
name: sealing-the-fort
description: Use when closing a fort's perimeter or any interior containment — walling a surface approach, capping a stair-top, building a bridge airlock, sealing off a breach-risk space — and whenever a seal needs AUDITING: after any dig or build near the perimeter, before stepping past a threat alert, or when wildlife keeps appearing inside. Covers what actually blocks movement (walls and raised bridges — doors are not lockable by this tooling), DF's diagonal corner-cutting, roof-hole audits, and the lever self-lock rule.
---

# Sealing the Fort — What Actually Seals, and How to Prove It

A seal is a claim about the map's topology, and every recorded breach in
this project was a claim nobody verified: a diagonal gap cleared as
"safe" in a written audit, a one-tile roof hole above a sealed street, a
bridge that sealed its own lever away. No audit tool exists yet (one is
proposed) — verification is manual reasoning, and this skill is that
reasoning.

## The movement law (the rule most audits get wrong)

**DF allows diagonal corner-cutting: units step diagonally between two
walls.** A diagonal line of walls NEVER seals. This was gotten wrong in
a live written audit — the roguelike intuition ("blocked if both
orthogonals are walls") does not apply, and a creature stepped from a
ramp diagonally past a "sealed" corner onto the fort's stairs.

- A barrier seals only if no outside-walkable tile touches an
  inside-walkable tile even DIAGONALLY. Check all eight neighbors of
  every boundary tile, not four.
- **Do not borrow intuition from fluids.** Aquifer seepage and miasma
  propagate orthogonally and straight down only — that is why a seal
  ring's diagonal corners can stay natural (see `aquifer-piercing` §4).
  Creatures cut the corners fluids cannot. Same geometry, opposite
  rules; conflating them is exactly how the live audit went wrong.
- The mirror image: a doorway reachable ONLY diagonally does function
  (units path through it), but it is a design smell the overseer flags
  on sight — keep room approaches orthogonal and save diagonal
  reasoning for threat analysis.
- Ramps extend the threat surface: a unit topping a ramp can step
  diagonally from the ramp's top — audit ramps near the perimeter as
  entrances, not terrain.

## What seals and what doesn't

- **Real seals**: undug solid rock (the interim-plug doctrine — see
  `fort-planning`), constructed or natural walls, and a RAISED bridge.
  That's the whole list.
- **Doors and hatches are NOT seals.** They are physical obstacles DF's
  own AI reasons about — they stop wildlife — but this project has no
  lock/forbid tooling, so a door cannot be closed against anything that
  opens doors. Choosing walls over a second door, and accepting a longer
  haul for one fewer entrance, has been the live call every time.
- **The roof is part of the perimeter.** A single tile of open air above
  an interior space is an entrance from above — one was found live
  directly over a "sealed" street, bypassing the door, the traps, and
  the bridge at once. Floors/roofs must be as continuous as walls.
- Climbing: v50 creatures can climb exposed surface walls (community
  knowledge, NOT yet observed live in this project). Roofed approaches
  and overhangs defeat climbers; an unroofed wall top is a maybe, not a
  seal. Treat accordingly until a live incident settles it.

## The manual seal audit

Run this after ANY dig, channel, or build near a perimeter, and before
stepping past a threat alert. One entrance is the doctrine precisely
because every entrance multiplies this audit's surface.

1. **Perimeter walk**: at each z the boundary crosses, walk the boundary
   in `look` and check every outside-walkable tile for adjacency —
   orthogonal AND diagonal — to any inside-walkable tile. Gaps hide in
   the tiles between named features (a live seal's breach was six
   scattered floor-gap tiles along one line, walled in a day once
   actually enumerated).
2. **Roof pass**: for interior space near the surface, `cross_section`
   columns and compare neighbors — a column that reads open-air/surface
   at a z where its neighbors have solid ground is a hole (this exact
   comparison found the live roof hole).
3. **Ramp and stair check**: every ramp top and stair top near the
   boundary is an entrance; cap stair-tops (hatch for wildlife-grade,
   bridge for real threats) per the defense ladder in `fort-planning`.
4. **Re-audit on change**: a seal is only as current as the last dig
   near it. Digging is what created both live breaches' preconditions.

## Bridge airlocks and the self-lock rule

The lever→bridge chain is live-verified and is the only on-demand seal
(see `fort-planning` stage 4 for tooling status). Two forts have sealed
themselves with it:

- **Before raising any bridge, ask: "if this seals, who can reach a
  lever — and can they ALSO reach food and water?"** A lever placed on
  the protected side of a DEAD-END pocket is a self-lock: the raise
  succeeds, the pocket has no other exit, and the queued lower job sits
  forever. One dwarf died this way in an earlier fort; the identical
  mistake recurred (caught in time) a fort later.
- "Lever on the inner side" is incomplete canon. When the sealed side is
  a dead end (a vault, a basement, a sanctum), a lever must ALSO exist
  on the side that keeps the rest of the fort. Two levers linked to one
  bridge works (live-verified).
- **Leave bridges DOWN by default.** A bridge is the panic seal, not a
  wall; raised-as-normal-state recreates the self-lock geometry
  permanently.
- Recovery from a self-seal (live-verified): dig a bypass around the
  bridge → access restored → pull the lower → REBUILD the bypass as
  walls while the bridge is down (and add the fort-side lever before it
  ever raises again).

## Fixing a wrong wall

Misplaced walls happen; they are not permanent:

- `designate_dig type=default` on the tile removes a constructed wall
  (live-verified workaround). `remove_building` support for constructed
  walls shipped in a later wave — verify it live before trusting it.
- Never wall or smooth over a carved stair tile to blind a gap — it
  destroys the stair (two live incidents; constructed stairs are the
  repair path and are immune). Check any seal footprint against stair
  tiles first, same discipline as dig rects.

## Verification status

Diagonal corner-cutting, the roof hole, both self-lock incidents and the
bypass recovery, the six-tile perimeter enumeration, and the dig-removes-
wall workaround: all live, 2026-07-25 through 2026-08-08. Climbing:
community knowledge only, unobserved here. The proposed seal-audit tool
(orthogonal+diagonal adjacency + roof holes, computed) remains unbuilt —
until it ships, this skill IS the audit.
