# Feature 015 — Item visibility: seeing clutter and stockpile fill

**Status:** design brief (no code written). Written 2026-08-07 against the live tree
(branch `008-mcp-server`) and the DFHack 53.15-r1 source checkout at
`C:\Users\zmanl\Projects\dfhack-build` (read-only).

**The play problem (verbatim from the fort session):** `look` ends with
`tiles with loose items on floor: 200` — a count with no locations. The model
cannot tell a full stockpile from an empty one, cannot find where 200 loose
items piled up, and cannot judge whether a new stockpile is needed. `stocks`
gives fort-wide totals with no locations; `buildings` prints
`Stockpile at (x,y,z)` with no extents, category, or fill state. A human
overseer had to say "we need more stockpile room."

**Two distinct questions** (kept separate throughout this brief, per the task):

- **(A) "Is this stockpile full / how much room is left?"** — a per-stockpile
  roster/inventory question. A glyph grid is the wrong medium: a stockpile can
  extend past any crop window, and a percentage can't be painted on tiles.
- **(B) "Where are the loose items that are NOT in a stockpile?"** — a spatial
  question that needs the map (or at least map-derived cluster coordinates).

The recommendation deliberately answers them through **different surfaces**:
(B) through `look` (an always-on footer split plus an on-demand `lens=items`),
(A) through the existing `buildings` tool (per-stockpile fill on each
Stockpile line). No single lens answers both, and no attempt is made to force
one to.

---

## 1. What the DFHack side can provide, and at what cost

Everything below was read from source in this repo and in the
`dfhack-build` checkout. Claims that could not be verified are flagged
explicitly in §6, not asserted here.

### 1.1 Per-tile loose items — ALREADY ON THE WIRE

The single most important finding of this investigation: **the per-tile data
the play problem asks for already crosses the wire on every `look`, and the
Go renderer throws the positions away.**

- `queryMapSlice` (`dfhack-plugin/queries.cpp:2950-2959`) emits
  `"floor_items": [[x,y,count],...]` — one triple per tile holding ≥1 loose
  item, **uncapped** (unlike `designated`/`smoothed`/`pending_building`,
  which cap at 200 entries).
- `internal/mapview/types.go:40-45` decodes it into
  `Slice.FloorItems [][3]int16`.
- `internal/mapview/render.go:122-126` then renders only
  `len(s.FloorItems)` — the useless count line.

**Definition of "loose item"** (`floorItemCountAt`, `queries.cpp:2726-2764`):
`flags.bits.on_ground` set, `in_building`/`construction` clear. Junk
(forbidden/dumped/rotten — `kNeverFortStockFlags`) is deliberately INCLUDED
because it still physically blocks building placement. Items inside
bins/barrels are excluded automatically (contained items are not
`on_ground`); the container itself counts as one item.

**Cost:** the plugin filters each 16×16 block's own item-id vector
(`df::map_block::items`) lazily, once per block per call, mirroring
`MapExtras`' `init_item_counts` pattern — it never touches
`world->items.all`. A 47×47 crop visits ≤ 16 blocks. This is the cheap tier
and it is already paid today.

**Critical semantic gap:** the filter does NOT distinguish stockpiled from
homeless items. Items stored on stockpile tiles are `on_ground` like any
other (verified below), so today's footer count **conflates a
well-organized full stockpile with a floor covered in junk** — the exact
confusion from the play session.

### 1.2 Stockpile membership is positional — no item flag says "stockpiled"

`DFHack::Buildings::StockpileIterator`
(`dfhack-build/library/modules/Buildings.cpp:1646-1700`, declared
`Buildings.h:201-256`) is DFHack's own canonical "what's in this stockpile"
walk, used by `Buildings::getStockpileContents`. Its membership test is:

1. item id appears in a map block covering the stockpile's bbox at its z;
2. `flags.bits.on_ground`;
3. `Buildings::containsTile(stockpile, item->pos)` — extent-bitmap aware
   (`Buildings.cpp:872-885`);
4. skip containers *assigned* to this stockpile that are empty
   (`isAssignedToThisStockpile` + no `CONTAINS_ITEM` general_ref).

So "is this item in a stockpile" is **pure geometry** — there is no item
flag to read. Any homeless/stockpiled split therefore requires a stockpile
coverage set, built from:

- `world->buildings.other.STOCKPILE` — a typed
  `std::vector<df::building_stockpilest*>` (verified
  `dfhack-build/library/include/df/buildings_other.h:65`). One entry per
  stockpile in the fort; iterating it is trivially cheap (forts have
  single-digit-to-dozens of stockpiles).
- `Buildings::containsTile` per candidate tile (handles extent-shaped
  stockpiles whose usable tiles exclude blocked ones — `placeStockpile` in
  our `buildings.cpp:1093-1096` relies on exactly this exclusion behavior).

**Do NOT use `Buildings::findAtTile` for this** — it is gated on
`occ->bits.building` + `isSettingOccupancy()` (`Buildings.cpp:368-413`),
and whether stockpiles set tile occupancy could not be verified from
headers (the vmethod bodies live in the game binary — §6). The
STOCKPILE-vector + `containsTile` path has no such dependency and is the
one DFHack itself uses.

**Cost of a per-slice coverage set:** for each stockpile whose z matches and
whose bbox intersects the slice window, test `containsTile` over the
intersection. Worst case ~(few stockpiles × 2209 tiles) of inline bitmap
reads per `look` — negligible next to the per-tile classify work the slice
already does.

### 1.3 Per-stockpile fill and capacity

- **Total usable tiles:** `Buildings::countExtentTiles(bld, defval)` —
  DFHACK_EXPORT, walks the extent bitmap (`Buildings.cpp:856-870`).
- **Occupied tiles / item count:** one walk of the covering blocks' item
  vectors with the §1.2 membership test, tallying distinct tiles and item
  structs. Same cheap tier as §1.1 (blocks covering one stockpile).
- **Accepted categories:** `sp->settings.flags` — a 17-bit
  `df::stockpile_group_set` (verified `stockpile_group_set.h`): animals,
  food, furniture, corpses, refuse, stone, ammo, coins, bars_blocks, gems,
  finished_goods, leather, cloth, wood, weapons, armor, sheet. These are
  the SAME 17 names our `stockpile` creation tool already exposes
  (`tools_action.go:621-639`) — the roster output vocabulary aligns with
  the creation vocabulary for free.
- **Identity:** `sp->stockpile_number` (int32, `building_stockpilest.h:21`)
  and `building.name` (std::string, custom name field, `building.h:66`).
- **Container capacity settings:** `sp->storage.max_barrels / max_bins /
  max_wheelbarrows` (`stockpile_storage_infost.h`). The parallel
  `container_type/container_item_id/container_x/container_y` vectors LOOK
  like per-tile assigned-container tracking but no DFHack consumer was
  found to confirm semantics — treated as unverified (§6) and not used.
- **Container interiors:** items inside a stored bin/barrel are reachable
  per-container via the `CONTAINS_ITEM` general_ref walk our
  `handleStockpileInventory` already uses for its empty-container tally
  (`queries.cpp:2397-2425`). Per-targeted-stockpile cost is fine; per-look
  cost is not needed by this design.

### 1.4 The expensive tier — what this design must NOT do per-look

- `world->items.all` scan: `handleStockpileInventory`'s own comment
  (`queries.cpp:2348-2350`) says it "can be expensive on a large fort; the
  LLM should use this sparingly." That is the fort-wide `stocks` tool's
  price, paid only on explicit call. Nothing in this design adds an
  all-items scan to any per-look path.
- Any per-item detail crossing the wire per tile (names, materials,
  qualities). The slice carries per-tile *counts* (and, proposed below,
  a small class index) — never item identities. Identities stay in
  targeted tools (`stocks`, and a possible future per-stockpile detail).

### 1.5 Query plumbing — what a change actually touches

Queries flow over the generic `MSG_TYPE_QUERY`/`MSG_TYPE_QUERY_RESPONSE`
channel (name string + JSON args in, status + JSON out —
`protocol.h:19-20`, dispatch ladder `queries.cpp:4000-4073`, Go side
`Bridge.Query(ctx, name, argsJSON)`). **Adding fields to an existing
query's JSON, or adding a whole new query name, requires ZERO changes to
the binary wire format** — `internal/protocol/message.go` and
`dfhack-plugin/protocol.h` stay untouched and in sync. Both sides already
follow the additive-JSON convention ("Optional: an older plugin simply
omits it" — every recent `Slice` field).

---

## 2. Candidate designs

Token estimates use the project's own crop formula (tokens ≈ 1.05·W·H + 80;
a radius-12 look ≈ 736 tokens, radius-23 ≈ 2.4k) plus ~15-18 tokens per
72-char text line.

### Candidate 1 — `lens=items`: class letters, case-split by homelessness

One more `LensDef` in the existing registry (`lenses.go:24-50`). Paints
every item-bearing tile with a lowercase letter keyed to a per-view item
CLASS table (the `lens=minerals` pattern exactly — `mineral_names`
precedent, `queries.cpp:2795-2844`); the letter is UPPERCASED when the tile
is not covered by any stockpile. Case-as-meaning copies the buildings
lens's built/planned convention.

Alphabet: a-z minus {d,t,u} (reserved: dig glyph, sapling, pending-building
— same exclusions as `mineralLetterAlphabet`, `lenses.go:94-103`) minus
{x,l,f} (their uppercase forms X/L/F are base terrain glyphs and the
disjointness test — `TestLensGlyphsDisjointFromBaseSet` — would rightly
fail). 20 letters × 2 cases remain; the plugin maps `df::item_type` to
~12 coarse classes (stone, wood, food/drink, furniture, bars/blocks,
cloth/leather, weapons/armor/ammo, finished goods, refuse/corpse, gems,
containers, tools, other), and only classes present in the view get a
letter, first-seen order.

What the model sees (radius-12 crop, rows elided):

```
z=132 — 25x25 crop at (96,92), north is up, x grows east, y grows south
   x=96                 x=120
  92 ####.....############
  93 #..aaacc.#..........#
  94 #..aaacc.#..BBB.....#
  95 #..aa.cc.#..BB......#
  96 #........#....@.....#
  ...
 104 #####################

legend: @ dwarf d designated u pending-building ? hidden(undug fog: diggable!) ...
lens=items: item-bearing tiles painted with per-view class letters (footnote);
UPPERCASE = tile is NOT inside any stockpile (homeless), lowercase = inside one
this view's item classes: a=stone (41 items/9 tiles) b=furniture (9/6) c=food/drink (58/4)
homeless items in view: 9 on 6 tiles — cluster (108,94)-(110,95)
```

- **Answers:** (B) fully — where, what kind, and homeless-or-not, per tile.
  Does NOT answer (A): a letter can't carry "this stockpile is 90% full",
  and the stockpile may extend past the crop.
- **Token cost:** the grid costs the same as any look at that radius; the
  lens adds a 2-line legend + 1-2 footnote lines ≈ **+60-100 tokens** over
  a plain crop. Paid only when asked for.
- **Plugin cost:** needs two additive `map_slice` fields (class per item
  tile, homeless flag per item tile — §5). Cheap tier only (§1.1/§1.2).
  Degrades on an old plugin: classes absent → fall back to painting
  `i`/`I` (item present, stockpiled/homeless unknown → all `i`) from the
  `floor_items` field every plugin since the smoothing wave already sends.

A rejected sub-variant: **painting per-tile counts as digits 1-9**. Water
depth already owns digits 1-7 as an always-on paint layer
(`render.go:57-68`); a lens painting digits over the same grid makes '3'
unreadable without cross-checking the water array, exactly the ambiguity
the minerals lens's alphabet-exclusion comment warns about. Counts go in
footnotes instead.

### Candidate 2 — a separate `stockpiles` roster tool

A new MCP tool: one line per stockpile, fort-wide.

```
3 stockpiles:
- #1 at (104,96)-(112,108) z=132 — all categories, 117 tiles, 102 occupied (87%), 184 items
- #2 "FoodHall" at (90,95)-(100,99) z=132 — food, 52 tiles, 47 occupied (90%), 209 items
- #3 at (60,40)-(63,44) z=137 — wood, 20 tiles, 3 occupied (15%), 3 items
(items counts containers as 1 — contents via stocks; occupied = tiles holding ≥1 loose item)
```

- **Answers:** (A) fully. Nothing of (B).
- **Token cost:** ~35-45 tokens per stockpile + header ≈ **150-500/call**.
- **Why rejected:** it is a strict subset of what the existing `buildings`
  tool can carry (Candidate 4) at the price of a new tool: permanent
  name-bytes under the deferral cost model (`schema_budget_test.go:65-70`
  — tool COUNT is the metric that "grows monotonically"), a new schema, a
  new `minToolArgs`/nil-bridge entry, and one more place for the model to
  look. The task's own judging criterion — "prefer one good default view
  over a pile of options" — cuts against it. `buildings` is already where
  the model looks for building state, and its render already parses the
  extents fields it currently ignores (`tools_state.go:42-55` decodes
  x1/y1/x2/y2 today).

### Candidate 3 — always-on `look` footer enrichment (the noticing channel)

Replace the dead-end count line with a homeless/stockpiled split computed
in Go from slice data, plus cluster coordinates for the homeless portion.
No lens required, no extra round trip, printed on EVERY look (all scopes).

Today:
```
tiles with loose items on floor: 200
```

Proposed (only lines with non-zero content print, existing convention):
```
loose items OUTSIDE stockpiles: 163 items on 47 tiles — clusters: (88,88)-(92,91) 96 items, (101,95)-(104,97) 41 items, +9 tiles elsewhere (lens=items for classes)
stockpile tiles in view: 120, 101 holding items (84%)
```

Clustering is 8-connected components over the homeless item tiles, top 2
by item count, computed in `internal/mapview` — spatial arithmetic in Go,
never in the model, per the world-model contract.

- **Answers:** the noticing half of (B) — "there IS a 96-item pile at
  (88-92,88-91) and no stockpile covers it" — without being asked. The
  84% line is a weak, view-scoped signal toward (A) (it says "the
  stockpile tiles around here are nearly all taken") but deliberately does
  not name stockpiles; that's Candidate 4's job.
- **Token cost:** **+0 when the floor is clean; +30-60 tokens** when
  clutter exists. Replaces a ~12-token line, so the marginal cost of a
  messy view is ~20-50 tokens — payable on every look, which is the whole
  point: this is the line that would have made the model say "we need more
  stockpile room" unprompted.
- **Plugin cost:** needs the homeless split and two coverage aggregates as
  additive `map_slice` fields (§5). Cheap tier. Old plugin → fields absent
  → legacy line renders unchanged (fixture-tested).

### Candidate 4 — stockpile fill on the existing `buildings` tool

`handleListBuildings` (`queries.cpp:1848-1909`) already special-cases
Bridge state; give Stockpile entries the same treatment with additive JSON
fields, and teach `renderBuildings` (`tools_state.go:62-91`) to use them
plus the extents it already decodes.

Today:
```
- Stockpile at (108,102,132) — built
```

Proposed:
```
- Stockpile #1 at (104,96)-(112,108) z=132 — all categories, 102/117 tiles occupied (87%), 184 items
- Stockpile #2 "FoodHall" at (90,95)-(100,99) z=132 — food, 47/52 tiles occupied (90%), 209 items
```

(One trailing caveat line per render, once, not per stockpile:
`stockpile items count containers as 1 — contents via stocks`.)

- **Answers:** (A) fully — per-stockpile identity, category, capacity,
  fill. Nothing of (B).
- **Token cost:** each stockpile line grows ~15-25 tokens; a fort with 10
  stockpiles pays **+~200 tokens per `buildings` call**, only when called
  (typically with a z filter). Non-stockpile lines unchanged.
- **Plugin cost:** per-stockpile covering-block walk inside the existing
  building loop — cheap tier, bounded by stockpile count.

### Candidate 5 — enrich `stocks` with locations — rejected outright

`stocks` aggregates `world->items.all` by (type, material); attaching
locations means per-item positions on a fort-wide scan — the expensive
tier feeding a token explosion, against the "no giant item lists"
anti-pattern. Location questions belong to the map surfaces above.

---

## 3. Recommendation

**Ship Candidates 3 + 4 as the core, and Candidate 1 as the drill-down —
in that priority order. Reject 2 and 5.**

| Piece | Question solved | When paid | Marginal tokens |
|---|---|---|---|
| C3 footer split + clusters | (B) notice | every `look`, automatic | 0 clean / 30-60 messy |
| C4 `buildings` stockpile fill | (A) | on `buildings` call | ~20/stockpile |
| C1 `lens=items` | (B) triage/detail | on explicit lens call | 60-100 |

Reasoning against the judging criterion — *"does it let a playing model
NOTICE 'this stockpile is full / these items are homeless' without being
told, at a token cost payable every few turns?"*:

- The **footer (C3) is the only always-on channel**, so it is the only
  piece that produces unprompted noticing. It must therefore stay ≤2 lines
  and carry coordinates (so the follow-up action — `stockpile` over the
  cluster's bbox — needs no further perception call in the common case).
  This alone would have caught the live incident: the 184-item Vault-annex
  situation would have read "184 items on N tiles OUTSIDE stockpiles —
  cluster (…)".
- The **`buildings` enrichment (C4)** turns the periodic buildings sweep
  the model already does into a stockpile health check for free — "90%
  occupied" on the food pile IS the "we need more stockpile room" signal,
  per-stockpile, with identity. Putting it in `buildings` instead of a new
  tool keeps the tool count flat and the habit unchanged.
- The **lens (C1)** exists because the footer deliberately compresses:
  once the model knows a pile exists, "what IS all this?" (classes,
  exact shape, which tiles are homeless) is a one-call answer.

**What this deliberately does NOT solve** (stated so nobody discovers it
in play and calls it a bug):

- **Container-interior capacity.** "47/52 tiles occupied" treats a
  half-empty bin as one occupied tile. A stockpile can be tile-full yet
  still absorb items into bins/barrels, and vice versa. The renders carry
  the "containers count as 1" caveat; true capacity modeling (max_bins,
  per-container fill) is out of scope until the container_* vector
  semantics are verified (§6).
- **"Properly stored" vs "parked on a stockpile tile."** The split is
  coverage-geometry, the same test DFHack's own StockpileIterator uses. An
  item dumped onto a stockpile tile the pile doesn't accept counts as
  "inside". Footer wording says "OUTSIDE stockpiles", not "unstored", on
  purpose.
- **Per-item identity on the map.** Classes, not names. Names remain
  `stocks`' job.
- **A per-stockpile contents breakdown** ("this pile is 90% full — of
  what?"). Deferred with a trigger: if live play shows fill% alone can't
  explain a stuck stockpile, extend the existing `building_status` plugin
  query (coordinate-targeted, `queries.cpp:1785-1815`) with a
  StockpileIterator + `CONTAINS_ITEM` top-N tally and expose it then.
  Not shipped now because no MCP tool currently proxies `building_status`
  and adding one contradicts the tool-count discipline without evidence.
- **Cross-z summaries** ("total homeless items in the fort"). Per-view
  only. A fort-wide clutter census would need a new expensive-tier query;
  no evidence yet that play needs it.

---

## 4. Schema shape (house rule: shape, never guidance)

**No new tools. No new parameters. One enum-ish string extended.**

`look`'s `lens` parameter description (`tools_percept.go:265`) currently
reads:

```
optional overlay: buildings|designations|minerals|wildlife. Paints one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations as 'd' and queued-but-unfinished buildings as 'u'). Exact building/zone types via buildings/building_status.
```

becomes (name added to the list; also fixes an observed drift — `zones`
has been a registered lens since the zones wave but was never added to
this list):

```
optional overlay: buildings|designations|zones|minerals|wildlife|items. Paints one annotation layer onto the terrain grid; omit for the plain terrain view (which still shows dig designations as 'd' and queued-but-unfinished buildings as 'u'). Exact building/zone types via buildings/building_status.
```

No WHEN/WHY prose is added — the legend and footnotes that explain the
items lens are runtime render output, not schema. `look` is already in
`perToolExceptions` (`schema_budget_test.go:77-79`); this adds ~12 bytes.

`buildings` tool: input schema unchanged (`z` filter only). Its
description gains one clause of fact, not guidance — e.g. append
`Stockpile entries include extents, accepted categories, and tile-fill.`
(~70 bytes; the tool sits far under `perToolCeilingBytes`).

`stocks`, `stockpile`, `cross_section`: untouched.

---

## 5. Implementation plan

### 5.1 Wire protocol

**No changes.** `internal/protocol/message.go` and
`dfhack-plugin/protocol.h` are not touched (§1.5). All new data rides as
additive JSON fields inside two existing query payloads (`map_slice`,
`list_buildings`). The two files' sync invariant is preserved trivially.

### 5.2 C++ (all in `dfhack-plugin/queries.cpp`; nothing else)

1. **Item-class helper** — `static int floorItemClassOf(df::item*)` mapping
   `it->getType()` to the ~12 coarse classes; a per-call class-name table
   emitted like `mineral_names` (only classes seen, first-seen order).
2. **Stockpile coverage helper** — build once per `queryMapSlice` call:
   iterate `world->buildings.other.STOCKPILE`, skip `z` mismatch / bbox
   non-intersect, `Buildings::containsTile` over the intersection into an
   `unordered_set<df::coord>` (pattern: `collect_dig_job_targets`, already
   built once per slice call at `queries.cpp:2784`).
3. **`queryMapSlice` additive fields** (existing `floor_items` unchanged
   for back-compat):
   - `"stockpile_tiles_in_view": N` (int, ALWAYS emitted — the
     new-plugin sentinel; see §5.3 on why an always-present int, per the
     `stockItem` nil-pointer lesson at `tools_state.go:112-119`),
   - `"stockpile_tiles_occupied": N` (int; occupied = ≥1 loose item by the
     §1.1 filter),
   - `"floor_items_nostock": [[x,y,count],...]` — the homeless subset of
     `floor_items` (tile not in the coverage set). Cap at 200 entries with
     a `"floor_items_nostock_capped": true` flag when hit (sibling
     convention), because unlike stockpile interiors, homeless clutter is
     usually small — and when it isn't, "200+" is still a complete signal.
   - `"floor_item_classes": [[x,y,classIdx],...]` +
     `"floor_item_class_names": [...]` — for ALL item-bearing tiles, cap
     200 (exact `minerals`/`mineral_names` convention, including the
     beyond-cap-stays-unlabeled behavior).
4. **`handleListBuildings`** — inside the existing loop, for
   `building_type::Stockpile`: `strict_virtual_cast<df::building_stockpilest>`,
   emit `"sp_number"`, `"sp_name"` (from `b->name`, may be empty),
   `"sp_categories":"food,furniture,..."` (the 17 `stockpile_group_set`
   bit names, matching `tools_action.go`'s `stockpileGroups` keys),
   `"sp_tiles"` (`Buildings::countExtentTiles`, defval = bbox area),
   `"sp_occupied"` and `"sp_items"` (one covering-block walk per
   stockpile using the §1.1 filter + `containsTile` — NOT
   StockpileIterator, whose empty-assigned-container skip would make an
   empty-bin tile read as unoccupied).
5. Everything stays inside the existing try/catch dispatch guard
   (`queries.cpp:4074-4080`) — no new CHECK_* exposure paths.

Build: `cmake --build build --target df_ai_protocol --config Release` from
the sibling checkout; redirect output to a log file and test the real exit
code (never pipe through `tail` — CLAUDE.md gotcha); retry once on the
flaky `generate_headers` exit -1073741819.

### 5.3 Go

- **`internal/mapview/types.go`** — `Slice` gains:
  `StockpileTilesInView *int`, `StockpileTilesOccupied *int` (POINTERS: nil
  = old plugin never measured, following the documented `stockItem`
  Units-pointer lesson — a zero must not read as an all-clear),
  `FloorItemsNoStock [][3]int16`, `FloorItemsNoStockCapped bool`,
  `FloorItemClasses [][3]int16`, `FloorItemClassNames []string`.
- **`internal/mapview/items.go` (new) + `items_test.go`** —
  `ClusterItemTiles(tiles [][3]int16) []ItemCluster` (8-connected
  components; bbox + item sum; sorted by item sum desc) and
  `FloorItemFooter(s *Slice) []string` (the C3 lines; legacy line when
  `StockpileTilesInView == nil`). Pure, unit-testable.
- **`internal/mapview/render.go`** — `RenderCrop` replaces the count line
  with `FloorItemFooter`'s output; **`downsample.go`** gets the same
  one-line swap for its "in source view" footer (homeless count only, no
  clusters — downsampled coords are block-granular).
- **`internal/mcpserver/lenses.go`** — one `LensDef{"items", ...}` whose
  Gather reads ONLY the slice (no `b.Query` round trip — the
  designations/minerals pattern): paint class letters from
  `FloorItemClasses` (case from `FloorItemsNoStock` membership); fallback
  `i`/`I` density-less paint from `FloorItems` when `FloorItemClassNames`
  is empty (old plugin), with a footnote saying class data needs the newer
  plugin. Alphabet constant + `lensGlyphSet("items")` (both cases of the
  20 letters) so `TestLensGlyphsDisjointFromBaseSet` covers it
  automatically.
- **`internal/mcpserver/tools_percept.go`** — the `lens` description
  string (§4).
- **`internal/mcpserver/tools_state.go`** — `buildingListEntry` gains the
  six `sp_*` fields; `renderBuildings` renders the C4 line (extents from
  the already-decoded X1..Y2 even when `sp_*` are absent — an old plugin
  still gets `(104,96)-(112,108)` for free); `buildings` tool description
  clause (§4).

### 5.4 Test plan

- **Go unit (all offline, fixture-driven):**
  - `items_test.go`: clustering shapes (single tile, L-shaped blob, two
    separated blobs, 200-cap flag), footer text for {legacy slice, new
    slice with/without homeless, zero-coverage view}.
  - `render_test.go`: update `TestRenderCropFloorItems` — new-plugin
    fixture asserts the split lines; add a legacy fixture asserting the
    OLD line still renders verbatim (version-skew contract).
  - `lenses_test.go`: items overlay paint (class letters, case split),
    footnote table, old-plugin `i`/`I` fallback, glyph-set disjointness
    (auto via existing sweep).
  - `tools_state_test.go`: `renderBuildings` with sp_* present/absent.
  - `server_test.go` sweeps: unchanged (no new tool); `schema_budget_test`
    stays green (look excepted; buildings well under ceiling).
- **C++:** compile-verified via the cmake target (no plugin unit-test
  harness exists in this project — same standard as every prior wave).
- **Live-verify checklist** (next play session's first act, per house
  convention; DLL deploy with DF closed, then):
  1. `look` over the Vault east annex all-category stockpile
     ((104,96)-(112,108) z=132, Fort #6 journal) — footer's
     occupied/coverage numbers vs the in-client stockpile screen.
  2. `look` at a known-messy workshop quarter — homeless cluster bbox vs
     eyeball in client; then `stockpile` over that bbox and re-`look`:
     homeless count must fall as haulers work (over game-days — patience
     policy).
  3. `lens=items` same spots — class letters vs client `k`-loop truth,
     UPPER/lower split vs stockpile boundaries.
  4. `buildings z=132` — every stockpile line's tiles/fill vs client.
  5. Old-DLL cross-check before deploying: new Go against the currently
     deployed plugin must render the legacy footer line (catches any
     accidental hard dependency on the new fields).

### 5.5 Suggested wave split (disjoint file sets)

1. **Go-only quick win** (shippable alone, works against the CURRENT
   deployed DLL): `renderBuildings` extents display + footer/cluster
   machinery behind the nil-sentinel + `lens=items` fallback paint from
   existing `floor_items`. Immediate play value: WHERE items are, today.
2. **C++ additive fields** (map_slice + list_buildings) — one implementer,
   one file.
3. **Go consumption** of the new fields (class letters, homeless split,
   sp_* render) + fixtures.

---

## 6. Risks and honest unknowns

- **UNVERIFIED: stockpiles and `occ.bits.building`.** `isSettingOccupancy`
  is a game-binary vmethod (`building.h:120` is declaration-only); whether
  stockpile tiles carry building occupancy could not be determined from
  source. The design routes every membership test through
  `buildings.other.STOCKPILE` + `containsTile` (DFHack's own pattern) so
  nothing depends on the answer.
- **UNVERIFIED: `stockpile_storage_infost.container_*` semantics.** They
  look like per-tile assigned-container parallel arrays, but no consumer
  was found in the DFHack tree to confirm. Not used anywhere in this
  design; container capacity remains an acknowledged blind spot.
- **UNVERIFIED (empirically): `building.name` population for stockpiles.**
  The field exists; whether our `name_place` flow writes it for
  stockpiles was not traced. Render treats empty as absent, so worst case
  the quoted name simply never appears.
- **Fill% semantics are approximate by construction.** Occupied-tile fill
  ignores container interiors both ways (§3). Mid-haul DF tile
  reservations are also invisible — a 95%-occupied pile may already be
  effectively full. The caveat line is part of the render, not optional.
- **Coverage split is geometric, not intentional** — items dumped onto a
  stockpile tile read as "inside". Wording chosen accordingly.
- **The 200-entry caps can truncate.** `floor_item_classes` over a
  catastrophically messy 47×47 view stops labeling at 200 tiles (minerals
  precedent: beyond-cap tiles stay base-glyph); `floor_items_nostock`
  reports its cap flag and the footer says "200+ tiles". The uncapped
  legacy `floor_items` array remains the wire-size outlier it already is
  (~26KB worst case at radius 23 — bytes on the local socket, not tokens;
  no fixed payload cap was found in `protocol.h`, and this size already
  ships today without incident, but it is the first thing to revisit if
  slice latency ever regresses).
- **Class mapping is editorial.** ~12 coarse classes chosen for footnote
  economy; DF has ~90 item types. A class carved wrong (e.g. seeds under
  food/drink vs its own class) costs a mislabeled letter, not data loss —
  cheap to adjust, but it WILL need a play-session shakedown.
- **Refuse churn:** outdoor refuse rots/scatters; homeless counts will
  fluctuate between looks near refuse. Expected behavior, worth a line in
  the play skill if it confuses a session (not a tool bug).
- **Two-question discipline is a bet.** If play shows the model needs
  per-stockpile CONTENTS (not just fill) to act, the deferred
  `building_status` extension (§3) is the designated relief valve —
  designed but intentionally unshipped.
