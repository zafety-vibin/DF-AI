# World model & perception

WHAT: how DF's 3D tile world becomes something a language model can actually see. The architecture of record is `specs/008-mcp-server/design.md` §4.2 (the L0-L4 layer table); this guide is the operational summary.

## The layers

- **L0 — the game** (DF + plugin): source of truth, queried, never fully mirrored.
- **L1 — Go substrate**: bit-packed topology overlay, entity/zone indexes, hazard/modification overlays. Complete and cheap; consumed only by Go geometry, never shown to the model.
- **L2 — semantic compilation**: classified tiles (shape + material via DFHack's `tileShape`/`tileMaterial`, never raw tiletype enum guessing), stratigraphy, surface analysis.
- **L3 — model-facing views**: what tools return. Small, labeled, semantic.
- **L4 — the player's own model**: `fortress/memory/` journal, goals, learnings, re-grounded every turn by L3's dashboard.

## Why slices, not 3D

The model receives three orthogonal, locally-verifiable projections and assembles the 3D understanding itself:

1. **Plan view** — `look`: per-z glyph crop, no separator spaces (tokenization destroys spaced grids), labeled rows/columns, fixed legend, dwarves overlaid.
2. **Bore** — `cross_section`: one column, all z, one line per level with shape/material and hidden flags.
3. **Computed relations** — `find_dig_site` and friends: Go answers the relational questions (where does a 5×5 room fit, ranked by distance) so the model never does coordinate arithmetic.

Volumetric/ASCII-3D renderings are rejected on evidence: grid-reading accuracy collapses with size, and direct play experience (2026-07-12) confirmed slices + assembly holds up while composition mistakes came from *missing overlays*, not missing dimensions. Planned additions from that same experience: designation overlays painted into `look`, an elevation view (the third orthogonal plane), and a reachability dry-run ("can dwarves actually reach these tiles?").

## Semantics that must not regress

- **Hidden (`?`) means undug fog-of-war — solid, diggable ground.** Designating into it is the core gameplay verb. It is never an error, never "unknown data," never something to validate away.
- The glyph legend is defined once (`internal/mapview.Legend`, mirrored by the plugin's `classifyTile`) — keep them in sync; every crop ships the legend.
- Grass vs stone vs soil vs mineral distinctions come from DFHack's material classification. If a new tile kind renders wrongly, fix `classifyTile` in `dfhack-plugin/queries.cpp`, not the renderer.
- The z-axis runs UP (higher z = sky). Surface ≈ the dwarves' embark level. 1200 ticks = 1 game day.

## Stair-kind rule

A multi-z stair shaft needs the right kind at each level or dwarves can't climb it: the bottom tile is UpStair, the top tile is DownStair, every tile between is UpDownStair. The plugin also honors **continuation**: if the top of a new range abuts an already-carved stair from above (or below), the adjoining tile is promoted to UpDownStair so the new shaft joins the existing one instead of leaving a one-tile gap; tiles that are already carved stairs are skipped rather than re-designated. This is why `designate_dig` with `stairs` is safe to call repeatedly to extend a shaft deeper — it reads what's already carved before deciding what each tile needs.

## Water, aquifers, and damp

Standing/flowing water and the aquifer bit are exposed everywhere depth matters — `map_slice`, `column_profile`, and `survey_site` all report water depth (from DF's own flow-size digit) and an aquifer flag, plus a derived "damp" flag for tiles adjacent to wet ground. These are surfaced **including hidden tiles**, an intentional exception to the fog rule above: DF's own dig-cancellation warnings ("water" / "damp stone") already make aquifers knowable to an attentive player without breaching them, so hiding that signal from the model would only manufacture surprises DF itself doesn't intend.
