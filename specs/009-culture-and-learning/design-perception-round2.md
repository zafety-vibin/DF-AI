# Feature 009, sub-project 1: Perception round 2 — design

**Status**: approved design, ready for planning (superpowers:writing-plans is next).
**Parent**: `specs/009-culture-and-learning/design-brief.md` workstream 2. The brief describes six
workstreams that are independent subsystems, not one design — this document covers ONLY
workstream 2 (Perception round 2), brainstormed and approved 2026-07-12. Zones (workstream 1),
skills-as-culture (workstream 3), predicates (workstream 4), turn-flow (workstream 5), and backlog
burn (workstream 6) each get their own design document when their turn comes.
**Why this sub-project first**: two independent live incidents this session (First Fort sessions
1-2, ~104 game-days) trace to the exact gap this closes — a room designated with no connector to
existing space, discovered only after burning thousands of ticks, because designations and
buildings are invisible in `look`'s default view. Zones remain the brief's priority for *after*
this lands (confirmed by the project lead: "zone ARE important... but perception is so core to the
experience we should work on it first").

## Scope

In scope: `look` overlay rendering (base-tier designations + a lens mechanism for buildings and
designation-kind detail), a Go-side 3D-connected region graph, reachability guidance inside
`designate_dig`, named places (`name_place`/`list_places`, persisted), and a new `elevation_view`
tool (the third orthogonal plane, both axes).

Out of scope (deferred to later workstreams, explicitly not redesigned around here): zones
themselves (workstream 1 — the `zones` lens is designed when that workstream lands, per the
extensibility test below), traffic/burrow designations, blueprint dry-run preview. The lens
mechanism is designed so all of these slot in later as one new lens definition each, with zero
changes to the rendering pipeline — this is verified in "Extensibility test" below, not asserted.

## Architecture overview

```
                    ┌─────────────────────┐
                    │  Region Graph (Go)   │  NEW — 3D-connected components
                    │  over TopologyOverlay│  over the existing (now-fixed)
                    └──────────┬───────────┘  cached tile state
                               │
        ┌──────────────────────┼──────────────────────┐
        │                      │                      │
        ▼                      ▼                      ▼
┌───────────────┐   ┌────────────────────┐   ┌─────────────────┐
│ reachability   │   │ name_place /       │   │ (future: zones  │
│ warn+suggest   │   │ list_places        │   │  workstream 1   │
│ inside         │   │ (persisted to      │   │  builds on this │
│ designate_dig  │   │  fortress/memory/) │   │  same graph)    │
└───────────────┘   └────────────────────┘   └─────────────────┘

┌────────────────────┐   ┌─────────────────────┐
│ look overlays       │   │ elevation_view       │  NEW tool, both axes
│ (base 'd' glyph +    │   │ (third orthogonal    │  reuses cross_section's
│  buildings/          │   │  plane)               │  column-render logic
│  designations lens)  │   │                       │
└────────────────────┘   └─────────────────────┘
```

The region graph is the one genuinely new algorithmic piece. `look` overlays and
`elevation_view` are pure rendering extensions over data that mostly already exists on the wire.
Reachability and named places are both thin consumers of the region graph.

## Component 1: Region graph

**Data**: connected components over the topology overlay's "open" tiles (`topology.StateOpen` —
floor/stair/ramp, the same classification `find_dig_site` already trusts post the 2026-07-12
TILE_UPDATE staleness fix). Two tiles are in the same region if a dwarf could walk between them:
horizontal 4-connectivity within a z-level, vertical connectivity through matching stair tiles
(an `UpStair` connects to a `DownStair`/`UpDownStair` directly above it, reusing the adjacency
logic the session-2 stair-continuation fix already established).

**Documented deviation — vertical adjacency is implemented permissively, not via matching stair
tiles.** `topology.StateOpen` is a coarse three-state classification (`ClassifyState`,
`internal/topology/state.go`): the wire protocol's `FLAG_FLOOR` bit is set for *any* walkable
tile shape (`FLOOR`, `STAIR_UP`, `STAIR_DOWN`, `STAIR_UPDOWN`, `RAMP`, ... — see
`dfhack-plugin/tile_extractor.cpp`'s `tile_shape` switch) with no sub-type carried over the wire.
All 8 bits of `TileState.Flags` are already spoken for (`internal/protocol/message.go`), so
"matching stair tiles" as literally specified above is not implementable without an additive wire
field carrying tile shape/sub-type — a plugin protocol change requiring a rebuild + DF-closed
deploy cycle, out of scope for the task that discovered this gap (Task 1, region graph core).
`internal/topology/regions.go`'s `neighbors()` therefore treats **any** open tile directly
above/below another open tile as vertically connected, not just stair-shaped ones.

**False-positive risk this accepts**: an ordinary two-story room where a floor tile sits directly
above another floor tile — the normal floor/ceiling boundary, no stairs anywhere — is reported as
the *same* connected region, even though no dwarf can walk between the two floors. This is exactly
the class of false "these areas are reachable" result this sub-project exists to eliminate (see
"Why this sub-project first" above). It is accepted for now, not silently: `RegionGraph.SameRegion`
can produce a false positive but never a false negative (permissive adjacency only ever merges
regions a stricter rule would keep separate) — so Component 3's reachability guidance can wrongly
suppress a connector suggestion, but will never wrongly suggest a connector to a region that IS
truly reachable. `internal/topology/regions_test.go` pins this behavior explicitly (rather than
leaving it as undocumented incidental behavior) so a future tightening is a deliberate, visible
code change, not a silent regression discovery.

**Follow-up (not scheduled)**: closing this gap for real requires a new additive-optional wire
field carrying `df::tiletype_shape` (or a compact stair/ramp/floor sub-type derived from it) from
`tile_extractor.cpp` through `protocol.h`/`internal/protocol` into `TopologyOverlay`, then
`neighbors()` gating vertical adjacency on matching stair shapes as originally specified. Revisit
if live play surfaces an actual false-positive incident (this project's established
measure-before-optimizing discipline — see the Computation note below for the precedent) rather
than building it speculatively now.

**Computation**: on-demand, full recompute per call — no incremental maintenance. A flood-fill
over the tile counts this project's own token-scaling research documents (hundreds to low
thousands of dug tiles even late-game) is trivial in-process Go work. Add incremental maintenance
only if measurement later shows recompute cost matters — matching this project's established
"measure before optimizing" discipline (the token-scaling report is the precedent).

**Location**: Go-side, over the existing (now-corrected) `TopologyOverlay` — not a new plugin
query. Consistent with `find_dig_site`'s now-validated pattern; avoids new DF-main-thread CPU cost
and round-trip latency for a freshness guarantee the turn-based protocol doesn't need (deltas push
at every pause/step boundary, bounding staleness to "mid-step").

**Interface** (internal Go type, not itself an MCP tool):

```go
type Region struct {
    ID    int
    Tiles []Coord   // or a more compact bbox+holes representation — implementation detail
    BBox  [2]Coord  // min/max corner, for cheap proximity checks
}

func BuildRegionGraph(topo *TopologyOverlay) []Region
func (rg RegionGraph) SameRegion(a, b Coord) bool
func (rg RegionGraph) NearestRegion(c Coord) (Region, dist int)  // powers the connector suggestion
```

## Component 2: `look` overlays — two-tier design

Full research and rationale: `docs/archive/2026-07-12-look-lens-design-research.md`. Summary of
the approved design:

### Tier 1 — promoted into the base view, always on

Dig designations render as a single `d` glyph replacing the base tile, unconditionally, on every
`look` call. **Not** a lens: both live incidents this session happened while the designation
*count* was already visible as a number ("count-only blindness" is a named root-cause layer from
the original dig-bug post-mortem) — an opt-in lens would let the identical failure recur if the
model simply forgets to ask. Cost: ~5 tokens and one legend entry, forever. Justified by two real
incidents, not speculation.

### Tier 2 — everything else is a `lens`

One optional enum parameter on `look`. Exactly one lens per call — not composable. This mirrors
DF's own vanilla UI (dig mode paints designations, zone mode paints zones, never both at once)
and avoids NetHack's documented failure mode (per-cell priority stacking needs color plus a
separate "farlook" command just to disambiguate what's under what). Wanting two layers costs two
`look` calls at the same coordinates — the grids are positionally identical, so cross-referencing
is the same row/column arithmetic the model already does across the three orthogonal slices.

```jsonc
// look input schema — one new optional field
"lens": {
  "type": "string",
  "enum": ["buildings", "designations"],
  "description": "optional overlay: paint one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations). Exact building/zone types via buildings/building_status."
}
```

**Rendering mechanics**: generalize `RenderCrop`'s existing hardcoded overlay passes (water
digits, then `@` marks) into an ordered, data-driven overlay pipeline. Fixed paint order: base
terrain → water digits → designations (`d`, always on) → dwarves (`@`) → the active lens's marks
(painted last, since the model explicitly asked for that layer). A lens overpainting a dwarf
appends a footnote rather than silently hiding it (`dwarves in view: 3 (2 under overlay at
(46,50) (47,51))`).

**`buildings` lens**: 8 category glyphs (W workshop/furnace, B furniture, D door/hatch/flow-
control, S stockpile, M mechanism/well/bridge, P trap/cage, C construction-in-progress, O other)
covering all 52 real `df::building_type` enum values — cross-checked against the DFHack 53.15
checkout, not invented. **Case encodes construction state**: uppercase = built, lowercase =
planned/under construction. This makes silently-dying building plans visible for the first time —
a lowercase glyph that never becomes uppercase and then vanishes from the lens IS the "this plan
died" signal DF itself never gives (directly addresses `learnings.md`'s "RE-PLACE any dead plan"
pain point). `G` is reserved for FarmPlot once farming lands (cannot exist in a fort today).

**`designations` lens**: per-kind detail when the base `d` isn't specific enough — `d` dig,
`c` channel, `r` ramp, `s` stair, `m` smooth/engrave. Deliberately does NOT reuse `< > X ^` (those
mean *carved* terrain in the base layer — a lens glyph must never be readable as a base glyph).

**Legend composition**: core terrain legend (unchanged, ships every call, gains one `d designated`
entry) plus a one-line lens-specific addendum appended only when that lens is active — the GIS
dynamic-legend rule: pay legend cost only for layers you enabled.

**Glyph governance (the one global invariant)**: every lens's glyph set must be disjoint from the
base TERRAIN set (`? # % = . , T t _ < > X ^ ~ L F` + digits `1-7` + `@`). Enforced mechanically
by a table-driven unit test walking the lens registry. This is what keeps the namespace from
needing a global redesign each time a lens is added — new lenses audit against one small fixed
set, never against every other lens.

One deliberate, narrow exception: the `designations` lens's `d` (Default-kind dig) coincides with
the base-tier always-on `d` overlay. This is not a collision — Default IS the generic case the
base `d` represents, so the lens glyph is a semantic refinement of the same overlay, not
competing information painted over unrelated terrain. The other four designations-lens glyphs
(`c r s m`) are genuinely new and disjoint from both the terrain set and each other. The
disjointness unit test should exclude `d` from the designations lens's check for this reason
(comment the exception at the test site, not just here).

**Adding a future lens** (the extensibility contract this design exists to satisfy): one
`LensDef{Name, Legend, Gather}` entry plus one enum literal. Zero changes to `RenderCrop` or the
paint pipeline.

### Extensibility test (verified, not asserted)

- **`zones`** (workstream 1, next): needs a zone-list query workstream 1 must build anyway;
  `civzone_type`'s 19 fort-relevant values compress to a handful of categories decided at
  zones-spec time. Zone overlap (a tile in two zones) is absorbed by the lens's own namespace (a
  `*` multi-zone glyph + footnote), the same count-line-plus-drill-down pattern already used for
  aquifers. RenderCrop changes: zero. tools_percept changes: one enum entry. **Passes.**
- **`traffic`/`burrows`** (further out): traffic is already sitting in the designation bitfield
  the slice loop reads; an additive plugin field plus a 3-glyph lens. Burrows are a membership
  query plus boundary paint, same shape. **Passes.**
- **`blueprint`** (dry-run preview): paints a proposed designation/build set instead of world
  state — the `Overlay` abstraction doesn't care where marks come from. **Passes**, and
  demonstrates the mechanism isn't secretly coupled to "things that already exist".
- **What does NOT fit, by design**: anything needing more than one attribute painted
  simultaneously per tile (zone type AND building category AND designation, all at once) —
  that re-derives the composite view NetHack's failure mode warns against. The answer stays
  multiple calls plus model-side assembly, same as the three orthogonal slices generally.

### Rejected approaches (so implementation doesn't re-litigate them)

Composable lenses (reintroduces priority-stacking ambiguity); multi-character cells (breaks
row/column tile-position arithmetic); ANSI color (invisible to a token consumer); per-type
building glyphs (52 types exceeds both the legend budget and cartographic convention for
symbol-class counts); a separate tool per concern (N concerns would cost N tool-schema overheads
on every API call, per the token-scaling audit); painting buildings into the base view (unlike
designations, buildings are stable queryable state, not tick-wasting — their legend cost doesn't
belong on every call forever).

## Component 3: Reachability — warn+suggest inside `designate_dig`

Runs automatically on every `designate_dig` call. Critically, **never in warning/error tone**: a
freshly-designated room being disconnected from existing dug space is normal DF workflow (room
first, corridor second), not a mistake — flagging it as an error would train false-alarm fatigue
against completely standard base-building practice. Instead, on a disconnected designation, the
ACK appends a constructive suggestion using `RegionGraph.NearestRegion()`:

```
SUCCESS: dig default (60,47)-(65,52) z=132 (30 tiles designated)
not yet connected to existing space — suggested connector: designate_dig default (56,49)-(60,49) z=132
```

No suggestion line when the designation IS reachable (the common case pays nothing extra).

## Component 4: Named places — `name_place` / `list_places`

General-purpose spatial labeling, independent of zones (zones are a related-but-distinct concept
per the project lead: naming also covers hallways and informal areas that may never have a formal
DF zone type). Built on the region graph:

- `name_place(x, y, z, name)` — resolves the region containing that tile, attaches the name.
- `list_places()` — every named region with its bbox and name.
- **Persisted** to a file under `fortress/memory/` (exact filename decided during planning — e.g.
  `places.json`), keyed by a stable anchor tile rather than a region ID (region IDs are
  recomputed fresh on every graph rebuild and are not stable across restarts; the anchor tile is).
  Reloaded on the next `FULL_STATE` push. This directly survives the df-mcp-restart pattern this
  session hit three separate times.
- If a named region's anchor tile no longer resolves to any region on reload (walled off, merged,
  etc.), the name is dropped silently rather than erroring — consistent with this project's
  tolerant-of-drift persistence conventions elsewhere (e.g. `AlertStore`'s eviction behavior).

## Component 5: `elevation_view` — new tool, both axes

```
elevation_view(axis: "x"|"y", x?, y?, x1?, x2?, y1?, y2?, z_top, z_bottom)
```

`axis="x"` sweeps across x at fixed y (`y`, `x1`, `x2` required); `axis="y"` sweeps across y at
fixed x (`x`, `y1`, `y2` required). Both axes are supported from the start — DF layouts aren't
axis-biased, and shipping only one direction would force awkward workarounds for the other. Reuses
`cross_section`'s existing per-column rendering (shape/material classification, DAMP/AQUIFER
annotations) — no new rendering primitives, just iterated across a line instead of a single point.

## Testing

- `mapview`: unit tests for the overlay pipeline (paint order, footnote generation on
  overlay-hides-dwarf), the glyph-disjointness invariant (table-driven, walks the lens registry),
  and `RenderColumn`-reuse correctness for `elevation_view`'s per-column output.
- Region graph: unit tests over fixture topologies (disconnected rooms, a stair-linked multi-z
  region, the exact "8x8 room 4 tiles from existing floor" shape that caused this session's
  4,800-tick incident) — `SameRegion`, `RegionAt`, `NearestRegion`, `Region.BBox`, and multi-z
  connectivity, including a test that pins the permissive-adjacency false positive documented in
  Component 1 above (an ordinary floor-over-floor stack reports as one region).
- `designate_dig`: test that a disconnected designation gets the suggestion text and a connected
  one gets none; test the suggested-connector coordinates are actually adjacent to both regions.
- `name_place`/`list_places`: round-trip persistence (write, reload, region-gone-on-reload drops
  silently), anchor-tile stability across a graph rebuild.
- Registration sweep (`TestEveryToolNilBridge`): `elevation_view`, `name_place`, `list_places` all
  need `minToolArgs` entries.

## Error handling

- Unknown `lens` name: truthful error listing registered lens names (matches this project's ACK
  discipline — never a bare failure).
- `elevation_view` with missing axis-appropriate coordinates: reject with a message naming exactly
  which field is missing for the requested axis.
- `name_place` on a tile with no region (solid rock, out of bounds): clear error, no partial
  state written.
- Region graph on an unbuilt/nil topology overlay (pre-`FULL_STATE`): same "topology not built
  yet" pattern `find_dig_site` already uses.

## Success criteria (ties back to the 009 brief)

1. No new incidents of the room/shaft-gap error class — the base `d` overlay and the reachability
   suggestion together close the exact gap that caused two real incidents this session.
2. `lens=buildings` calls become the practical way to answer "what's built where" spatially,
   cheaper in tokens than the flat `buildings` list for anything but an exhaustive dump.
3. The extensibility test holds when workstream 1 (zones) actually adds its lens: zero
   `RenderCrop`/pipeline changes, confirming this design's central claim rather than assuming it.
4. Named places persist across at least one df-mcp restart in live play (a near-certainty given
   this session's restart cadence).

## Open questions carried to planning (not blockers)

1. `lens` as a schema enum (self-documenting, small per-call schema cost) vs. a free string
   validated against a registry (constant schema cost) — lean enum; discovery value outweighs the
   token cost. Confirm during planning if it matters in practice.
2. Whether the existing "designated for digging: N tiles in view" count line stays alongside the
   new `d` overlay — lean keep (it covers the plugin's 200-pair designation cap, ~8 tokens).
3. Plugin work order: `list_buildings` gaining full footprint extents (not just center) and
   `Designated` gaining per-tile kind are both additive-optional wire fields, independently
   useful, and both require a plugin rebuild + DF restart. Bundle them into one plugin/protocol
   touch during implementation to amortize the deploy cycle.
