# Lens mechanism for `look` — design research for Feature 009 workstream 2 (Perception round 2)

**Date**: 2026-07-12 (dated snapshot; frozen history per the stateless-docs policy — verify glyph
sets, enum values, and file line references against the code before acting on them)
**Scope**: how to add dig-designation and building overlays to `look` now, and zones/traffic/burrows
later, without redesigning the rendering pipeline each time. Inputs: the current renderer
(`internal/mapview/render.go`, `types.go`), the plugin classifier and slice query
(`dfhack-plugin/queries.cpp`), the real DF enums from the DFHack 53.15 checkout
(`../dfhack-build/library/xml/df.d_basics.xml`), both live First Fort sessions
(`fortress/memory/journal.md`, `learnings.md`), and the token-scaling audit
(`docs/archive/2026-07-12-token-scaling-research.md`).
**This document proposes a design; it is not the decision.** The 009 workstream-2 spec should
confirm or amend it.

---

## Summary (the recommendation up front)

**Two tiers, not one:**

1. **Promote dig designations into the BASE `look` view** as a single overlay glyph `d` —
   not a lens. Two independent live incidents (4,800 ticks burned on an unreachable room;
   session-1 bedroom gap) trace to designations being invisible in the default view, and the
   dig-bug post-mortem names "count-only blindness" as a root-cause layer. The count line was
   present during both incidents and did not prevent them. Cost is near zero: `d` replaces the
   base glyph 1:1 (no grid growth) plus ~5 legend tokens. A safety-critical, transient,
   about-to-change-the-map state belongs in the default view, exactly like water depth.
2. **Everything else becomes a `lens`: an optional enum parameter on `look`, exactly ONE lens
   per call, rendered as an overlay painted onto the same base terrain grid** (the existing
   `marks` mechanism generalized), with a one-line lens-specific legend addendum appended after
   the core terrain legend. Near-term lenses: `buildings` (category glyphs, case encodes
   construction state) and `designations` (per-kind dig detail). Zones, traffic, and burrows
   each slot in later as one new lens definition with zero rendering-pipeline changes.

The split rule for all future concerns: **if missing it silently wastes ticks, it goes in base;
if it is stable, queryable state, it goes in a lens.** Base stays small forever; lenses absorb
all growth.

This confirms the overseer's category-level-glyphs instinct and the session-2 journal design
note (`fortress/memory/journal.md`, "Design note (user)"): category glyphs in the grid, exact
type via the existing `buildings`/`building_status` detail queries — the established
progressive-disclosure pattern (overview grid → detail-query drill-down).

---

## 1. External survey — transferable lessons only

### Dwarf Fortress's own UI: mode-scoped overlays (the strongest precedent)

DF itself — classic and Steam — answers "many semantic layers, one bounded tile display" with
**modal overlays**: the dig menu paints designations onto the map while you are in dig mode;
building placement paints a validity overlay in build mode; zones render as rectangles only in
zone mode (Steam adds an explicit zone-visibility toggle). The base map never tries to show
everything at once. This is the genre-native solution, battle-tested for twenty years on exactly
this game's data. A `lens` parameter is DF's own mode system, made stateless and explicit for a
tool-call consumer. Notably, DF keeps *designations* visible outside dig mode too (blinking
overlay) — supporting the base-tier promotion above.

### NetHack: priority stacking + farlook (a precedent and a warning)

NetHack renders one glyph per cell with a fixed priority (monsters > objects > terrain) and
resolves the resulting ambiguity two ways: color, and the `;` (farlook) per-tile identify
command. The transferable lesson is double-edged. The farlook pattern is exactly this project's
detail-query drill-down and validates it. But the stacking itself is the cautionary tale:
NetHack's single composite view is famously ambiguous without color and farlook — you cannot
see what a monster is standing on. Compositing many layers into one call is what *creates* the
need for disambiguation machinery. Mutually exclusive lenses avoid the problem instead of
managing it. Cataclysm-DDA and Caves of Qud use the same priority-stack approach with the same
costs (CDDA's `V` nearby-list is another detail-query analog). Factorio's alt-mode — a single
toggle that overlays entity identity icons on the world — is the same lesson from the factory
genre: one keystroke between "terrain view" and "annotation view", never both fighting per-cell.

### GIS layering: base map + thematic overlays + dynamic legend

The general GIS pattern (ArcGIS/QGIS/Leaflet alike): one persistent base layer; named thematic
overlay layers toggled on demand; and — the directly stealable part — **the legend is generated
from the active layers only**. A layer you did not enable costs you no legend. That is the
legend-composition answer in one sentence, and it is also precisely the token-scaling
discipline: don't make every call pay for entries the model didn't ask about. GIS also offers
the fallback for genuinely-simultaneous comparison: small multiples (side-by-side maps), which
in our medium is two `look` calls at the same coordinates — cheap, self-consistent, and
positionally cross-referenceable because the grids share row/column math.

### Symbol-count evidence (what "about 5–8 categories" actually rests on)

Real, citable anchors — with an honesty caveat below:

- Cartographic convention holds thematic maps to roughly **four to seven symbol/value classes**;
  Evans' classic guidance says five or fewer for novice map readers, and a survey of published
  choropleth maps found ~50% use exactly 5 classes ([ICA proceedings](https://icaci.org/files/documents/ICC_proceedings/ICC1995/PDF/Cap173.pdf),
  [GeoLinter, arXiv:2310.13707](https://arxiv.org/pdf/2310.13707),
  [Intro to Cartography, OpenALG](https://alg.manifoldapp.org/read/introduction-to-cartography/section/c3c06272-8b8b-49e7-a957-da0d06550b73)).
- The number usually cited underneath that convention is Miller (1956), "The Magical Number
  Seven, Plus or Minus Two" — which is about human absolute-judgment span and is routinely
  over-applied to display design. Bertin's *Semiology of Graphics* (1967) makes the sharper
  point: shape as a visual variable is only weakly "selective" — humans cannot rapidly segregate
  many simultaneous shape classes.
- **Caveat: our consumer is an LLM, not preattentive human vision.** The binding constraints
  here are (a) legend token cost per call and (b) glyph-collision ambiguity against the base
  terrain set — not working memory. The human-facing 4–7 convention still matters as a sanity
  rail (each extra category is legend tokens on every lensed call, and more for the model to
  hold while reading a grid), but the hard ceiling is really the disjointness rule in §4. Eight
  categories with a one-line legend is comfortably inside both constraints; per-type glyphs for
  52 building types is outside both.

### Terminal dashboards: named toggles over one bounded region

htop (F2 setups, tree-view toggle), tmux (status segments composed per config), vim (one window,
many buffers): the interaction pattern for "one bounded space, many things worth showing" is
**named, mutually exclusive views with a persistent status core**, switched explicitly and
cheaply. Our analog: the dashboard header + terrain legend are the persistent core; lenses are
the named views; a tool parameter is the toggle. Nothing here suggests compositing.

---

## 2. The lens mechanism — concrete design

### 2.1 Tool signature

```jsonc
// look input schema (tools_percept.go) — one new optional field
{
  "x":      { "type": "integer", "description": "center x" },
  "y":      { "type": "integer", "description": "center y" },
  "z":      { "type": "integer", "description": "z-level to view" },
  "radius": { "type": "integer", "description": "half-width of the crop, default 12, max 15" },
  "lens":   { "type": "string",  "enum": ["buildings", "designations"],
              "description": "optional overlay: paint one annotation layer onto the terrain grid; omit for the plain terrain view. Exact building/zone types via buildings/building_status." }
}
```

- **Omitted lens = today's view + the base `d` designation overlay.** Models that never pass
  `lens` pay ~5 extra legend tokens and nothing else, ever.
- **Exactly one lens per call** (enum, not a list). Wanting two layers = two calls at the same
  coordinates; the grids are positionally identical so cross-referencing is row/column math the
  model already does across its three orthogonal slices.
- Unknown lens name → truthful error listing the registered lens names (the ACK discipline).
- Future lenses (`zones`, `traffic`, `burrows`, `blueprint`) are new enum entries only.

### 2.2 Rendering mechanics (generalizing what exists)

`RenderCrop` today hardcodes two overlay passes: water digits, then `marks` ('@' dwarves), later
paint wins (`internal/mapview/render.go`). Generalize to an ordered overlay pipeline —
same mechanism, made data-driven:

```go
// mapview: an overlay is marks + optional footnote lines. RenderCrop takes
// an ordered slice; later overlays win per-tile. Water and '@' become the
// first two standard entries instead of hardcoded passes.
type Overlay struct {
    Marks     map[[2]int16]rune
    Footnotes []string // count lines etc., appended after the legend
}
func RenderCrop(s *Slice, overlays []Overlay, legendExtra string) string
```

Fixed paint order:

| # | Layer | Source | Notes |
|---|---|---|---|
| 1 | base terrain | `Slice.Rows` (plugin `classifyTile`) | unchanged |
| 2 | water digits `1-7` | `Slice.Water` | unchanged |
| 3 | designations `d` | `Slice.Designated` (data already on the wire) | **new, always on** |
| 4 | dwarves `@` | entity snapshot | unchanged |
| 5 | lens marks | the active lens's gather function | only when `lens` passed |

Lens paints last — the model explicitly asked for that layer, and a dwarf masking one designated
or planned tile is exactly the 4,800-tick failure class. When a lens overpaints any `@`, append
a footnote: `dwarves in view: 3 (2 under overlay at (46,50) (47,51))`. No collision, no footnote,
no cost. In the base view `@` still wins over `d` (a dwarf on a designated tile is usually the
miner working it — informative, and current behavior).

The lens registry lives in the MCP layer (it needs the Bridge for data):

```go
// mcpserver: adding a lens = appending one entry here + one enum literal.
type LensDef struct {
    Name   string
    Legend string // one line, shipped only on lensed calls
    Gather func(ctx context.Context, b *Bridge, s *mapview.Slice) (mapview.Overlay, error)
}
var lenses = map[string]LensDef{ /* buildings, designations, ... */ }
```

### 2.3 The `buildings` lens — categories cross-checked against the real enum

`df::building_type` has 52 concrete values (extracted from
`../dfhack-build/library/xml/df.d_basics.xml`, enum at line ~10797). Category map — 8 glyphs,
every value assigned:

| Glyph | Category | building_type values covered |
|---|---|---|
| `W` | workshop/furnace (production) | Workshop, Furnace |
| `B` | furniture | Bed, Chair, Table, Cabinet, Box, Coffin, Statue, Weaponrack, Armorstand, TractionBench, Slab, Bookcase, DisplayFurniture, Instrument, NestBox, Hive, Nest |
| `D` | portal / flow control | Door, Hatch, Floodgate, GrateWall, GrateFloor, BarsVertical, BarsFloor, WindowGlass, WindowGem |
| `S` | stockpile | Stockpile |
| `M` | mechanism / power / water | ScrewPump, GearAssembly, AxleHorizontal, AxleVertical, WaterWheel, Windmill, Rollers, Well, Bridge, Support, Chain |
| `P` | trap / defense | Trap, AnimalTrap, Cage, SiegeEngine, ArcheryTarget |
| `C` | construction in progress | Construction (completed ones become terrain and leave the buildings list) |
| `O` | other / special | TradeDepot, Shop, Wagon, RoadDirt, RoadPaved, Weapon |

Excluded by design: `Civzone` (that is the future zones lens, different data plane), `NONE`.
Reserved: `G` for FarmPlot when farming lands (it cannot exist in a fort today — no farm-plot
build type; assigning it later is a one-line change and a live demo of the extensibility claim).
Known coarseness, accepted for v1: levers are `building_type::Trap` with a Lever subtype, so
they render `P` until `list_buildings` carries subtype; the doc-of-record for exact identity is
the detail query anyway.

**Case encodes construction state**: `UPPERCASE = built, lowercase = planned/under construction`
(from the `stage == max_stage` data `list_buildings` already returns, `queries.cpp
handleListBuildings`). One legend clause covers all eight categories. This directly serves the
worst live-play building pain — plans that die silently (`learnings.md`, "RE-PLACE any dead
plan"): a `b` that never becomes `B` and then vanishes from the lens IS the death announcement
DF never gives.

Legend addendum (one line, ~40 tokens, only on `lens=buildings` calls):

```
lens=buildings: W workshop/furnace B furniture D door/hatch S stockpile M mechanism/well/bridge P trap/cage C construction O other — UPPERCASE built, lowercase planned/in-progress; exact type: buildings tool
```

**Data path**: the Go side already queries `list_buildings` with a z filter; the lens gathers
that list for the crop's z and paints. One additive plugin change is wanted (not required for
v1): `list_buildings` returns only `centerx/centery` today, but `df::building` carries the full
footprint (`x1,y1,x2,y2`); emitting them lets the lens paint a workshop's true 3x3 occupancy
rather than its center tile. Occupancy is what the model actually asks about ("can I path/dig
here?"), so extents should follow quickly, using the established additive-optional-JSON-field
pattern (`mapview/types.go` "an older plugin simply omits them"). V1 falls back to center-only
paint when extents are absent.

### 2.4 The `designations` lens — per-kind detail

The base `d` says "something is designated here". The lens says what. The plugin's slice loop
already reads `des.bits.dig` per tile and discards the kind (`queries.cpp` `queryMapSlice`,
`des.bits.dig != No`); `df::tile_dig_designation` = { Default, Channel, Ramp, UpStair,
DownStair, UpDownStair, Water, Magma }, plus `des.bits.smooth` for smooth/engrave. Additive
change: emit the kind (extend the designated triple to `[x,y,kind]` as a new optional field).

| Glyph | Kind |
|---|---|
| `d` | dig (Default — mine out to floor) |
| `c` | channel |
| `r` | ramp |
| `s` | stair (UpStair, DownStair, UpDownStair — direction is cross_section's job) |
| `m` | smooth/engrave (`des.bits.smooth`) |

Deliberately NOT `<` `>` `X` `^` for designated stairs/ramps — those glyphs mean *carved*
terrain in the base layer, and a lens glyph must never be readable as a base glyph (§2.6).
Legend addendum: `lens=designations: d dig c channel r ramp s stair m smooth — designated, not yet dug`.

### 2.5 Legend composition rule

- **Core terrain legend: unchanged, ships on every crop** (project law — "every crop ships the
  legend"). Gains exactly one entry: `d designated`.
- **Lens addendum: one line, appended after the core legend, only when that lens is active.**
  This is the GIS dynamic-legend rule: you pay legend for the layers you enabled.
- **Lenses are mutually exclusive — one per call.** Justification against the token constraint:
  a composite call would carry both addenda PLUS collision-resolution text (what does a
  designated tile inside a stockpile inside a zone show?), and NetHack demonstrates that
  per-cell priority stacking generates permanent ambiguity machinery. Two clean calls at r=12
  cost ~1,000-1,100 tokens total and each is self-describing; that is the honest price of
  genuinely wanting two layers, paid only when wanted. The common case (one concern per
  question) pays for one.

Worst case audit: a model that wants terrain + designations + buildings + zones in one turn
makes three calls (designations ride the base view free). ~1.5K tokens — still less than one
uncapped `stocks` call costs today, and each grid is unambiguous.

### 2.6 Glyph governance (the one global invariant)

Base occupied set (from `classifyTile` + overlays): `? # % = . , T t _ < > X ^ ~ L F` +
digits `1-7` + `d` + `@`.

- **Rule: every lens's glyph set must be disjoint from the base occupied set.** A lens glyph
  paints over terrain; a collision would make one character mean two things in a single grid.
- **Reuse across different lenses is permitted** (mutual exclusivity makes it unambiguous within
  any single response, and every lensed crop carries its own addendum) — but avoid it when free
  letters exist, for cross-turn readability.
- Enforce mechanically: a table-driven unit test in `mapview` walks the lens registry and
  asserts disjointness-from-base per lens. New lens authors get regression protection for free.

This rule is what makes the namespace never need a global redesign: adding a lens requires
auditing against one small fixed set (base), never against every other lens.

---

## 3. Extensibility test — walking future lenses through the design

**`zones` (009 workstream 1, next up)**: workstream 1 must build a zone list query anyway
(civzone port). The lens is then: one `LensDef{Name: "zones", Legend: ..., Gather: query zones
intersecting the crop, map `df::civzone_type` to categories, paint extents}` + one enum literal.
The fort-relevant civzone_type values (19 of them: Bedroom, Dormitory, DiningHall, MeetingHall,
Office, Barracks, ArcheryRange, Pen, Pond, WaterSource, FishingArea, Dump, SandCollection,
ClayCollection, PlantGathering, AnimalTraining, Tomb, Dungeon, Shrine) compress to ~6 indicative
categories (rest / hall / food-producing / military / utility / tomb — decided at zones-spec
time, not here). The genuinely new problem zones bring — overlap (a tile can be in two zones,
and zones overlay buildings) — is absorbed by the lens's own namespace: a `*` multi-zone glyph
plus a footnote (`3 tiles in multiple zones — zone detail query per tile`), the same
count-line + drill-down pattern the renderer already uses for aquifers. **RenderCrop changes: zero.
tools_percept changes: one enum entry.** Passes.

**`traffic` / `burrows` (further out)**: traffic is already sitting in the designation bitfield
the slice loop reads (`df::tile_traffic`: Normal/Low/High/Restricted — verified in the 53.15
checkout) — an additive plugin field plus a 3-glyph lens (Normal stays unpainted; painting the
default would ink the whole grid). Burrows are a membership query + boundary paint, same shape.
**Pipeline changes: zero.** Passes.

**`blueprint` (dry-run preview)**: paint a proposed designation/build set with two glyphs
(planned / blocked) before committing — the gather function reads a proposal instead of world
state, which the Overlay abstraction does not care about. Passes, and shows the mechanism is not
secretly coupled to "things that exist".

The one future concern that does NOT fit: anything needing more than one attribute per tile
simultaneously (e.g. "zone type AND building category AND designation on every tile of this
grid at once"). That is by design — it re-derives the composite view. The answer stays: multiple
calls, model-side assembly, same as the three orthogonal slices.

---

## 4. What NOT to do (rejected approaches, so the brainstorm doesn't re-derive them)

1. **Composable lenses (`lens: ["buildings","zones"]`)** — rejected. Reintroduces per-cell
   priority stacking (NetHack's permanent ambiguity problem), forces a global glyph-disjointness
   audit across all lens pairs (kills the zero-redesign extensibility property), and makes the
   worst-case legend a paragraph. Two calls are cheaper than one ambiguous call plus the
   disambiguation traffic it causes.
2. **Multi-character cells** (e.g. `W3` for "workshop, stage 3") — rejected. Breaks the
   single-char fixed-width property that makes tile positions computable by row/column
   arithmetic, and the project's tokenization finding says separators/width-changes destroy
   adjacency. Every attribute beyond one-char-per-tile goes to footnotes or detail queries.
3. **ANSI color / styling layers** — rejected. The consumer reads tokens, not pixels; color is
   invisible to it and costs escape-sequence tokens.
4. **Per-type building glyphs** — rejected (52 types vs. the remaining printable-ASCII budget
   after the base set; and category-level is where the cartographic 4-7 convention and the
   legend token budget both point). Exact type is the detail query's job.
5. **A tool per concern (`look_buildings`, `look_zones`, ...)** — rejected. Tool schemas are
   session-constant overhead (~29 schemas already cost 3-6K tokens per API call, per the
   token-scaling audit); N concerns must not cost N schemas plus N copies of crop/bounds logic.
   One `look`, one growing enum.
6. **Buildings painted into the base view** — rejected for the opposite reason designations were
   promoted: buildings are stable, queryable, non-tick-wasting state, and their 8-entry legend
   would be a constant tax on every `look` forever. The base view instead keeps a one-line cue
   (`buildings in view: 4 (lens=buildings)`) — same pattern as the existing aquifer count line.
7. **Full-map or multi-z lens renders** — out of scope by standing decision (no volumetric/3D
   text; slices + model-side assembly is project law, reconfirmed by live play 2026-07-12).

---

## 5. Token accounting (against the scaling audit's discipline)

| Change | Cost per call | Paid by |
|---|---|---|
| base `d` overlay + legend entry | ~5 tokens, zero grid growth | every `look` (justified: two live incidents) |
| base buildings count-line cue | ~8 tokens, only when buildings in view | `look` calls near built areas |
| `lens=buildings` | crop unchanged + ~40-token addendum + footnotes | only calls that ask |
| `lens=designations` | crop unchanged + ~25-token addendum | only calls that ask |
| new lens later | its own addendum only | only calls that ask |

Bonus: a `lens=buildings` call (~550-650 tokens at r=12) becomes the CHEAP way to see building
layout — the flat `buildings` list costs ~2,900 tokens at its 200 cap and carries no spatial
gestalt. The lens will likely reduce total tokens spent on building questions, not add to them.

---

## 6. Small open questions for the brainstorm (not blockers)

1. Schema enum vs. free string + registry validation for `lens` — enum self-documents in the
   tool schema (costs a few schema tokens per lens); free string keeps schemas constant. Lean
   enum: the model discovering lenses from the schema is worth more than ~10 tokens.
2. Does the base "designated for digging: N tiles in view" count line stay once `d` paints? Lean
   keep: it covers designations beyond the plugin's 200-pair cap and costs ~8 tokens.
3. Plugin work order: extents on `list_buildings` and kind on `designated` are both additive
   optional fields — bundle them into one plugin/protocol touch to amortize the rebuild+deploy
   cycle (DF must be closed for DLL deploys).

Sources (external): [ICA proceedings — choropleth accuracy and class count](https://icaci.org/files/documents/ICC_proceedings/ICC1995/PDF/Cap173.pdf), [GeoLinter (arXiv:2310.13707) — cartographic class-count guidance](https://arxiv.org/pdf/2310.13707), [Introduction to Cartography (OpenALG) — choropleth classes](https://alg.manifoldapp.org/read/introduction-to-cartography/section/c3c06272-8b8b-49e7-a957-da0d06550b73); Miller (1956), *Psychological Review* 63(2); Bertin (1967), *Semiology of Graphics*.
