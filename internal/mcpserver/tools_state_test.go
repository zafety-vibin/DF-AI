package mcpserver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/worldmodel"
)

func TestCapRawJSON(t *testing.T) {
	small := `{"items":[1,2,3]}`
	if got := capRawJSON(small); got != small {
		t.Fatalf("small payload must pass through unchanged, got %q", got)
	}
	big := strings.Repeat("x", rawJSONCap+500)
	got := capRawJSON(big)
	if !strings.HasSuffix(got, "...truncated, refine your query") {
		t.Fatalf("oversized payload must carry the truncation notice, got tail %q", got[len(got)-60:])
	}
	if len(got) > rawJSONCap+len("\n...truncated, refine your query") {
		t.Fatalf("truncated payload too long: %d bytes", len(got))
	}
	// Truncation must never split a multi-byte rune.
	multi := strings.Repeat("é", rawJSONCap) // 2 bytes each
	if trimmed := capRawJSON(multi); !strings.HasSuffix(trimmed, "...truncated, refine your query") || strings.ContainsRune(trimmed, '�') {
		t.Fatal("rune-boundary truncation broken")
	}
}

func TestStocksQueryArgs(t *testing.T) {
	if got := stocksQueryArgs(""); got != "{}" {
		t.Fatalf("empty category must send empty args, got %q", got)
	}
	// The plugin matches category case-SENSITIVELY against uppercase
	// ENUM_KEY_STR names — the documented lowercase examples only work
	// because this helper uppercases before sending.
	if got := stocksQueryArgs("boulder"); got != `{"category":"BOULDER"}` {
		t.Fatalf("category must be uppercased for the plugin, got %q", got)
	}
	if got := stocksQueryArgs("Wood"); got != `{"category":"WOOD"}` {
		t.Fatalf("mixed case must normalize, got %q", got)
	}
	// Mechanisms are stored under item_type TRAPPARTS in DF — a live session
	// discovered "category=mechan" returns nothing without this alias.
	if got := stocksQueryArgs("mechanism"); got != `{"category":"TRAPPARTS"}` {
		t.Fatalf("mechanism must alias to TRAPPARTS, got %q", got)
	}
	if got := stocksQueryArgs("Mechanisms"); got != `{"category":"TRAPPARTS"}` {
		t.Fatalf("plural mechanism alias must normalize case too, got %q", got)
	}
}

func TestRenderBuildings(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"carpenter workshop","x":50,"y":50,"z":139,"stage":0,"max_stage":3,"done":false},
		{"type":"bed","x":12,"y":8,"z":138,"stage":1,"max_stage":1,"done":true}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "2 buildings:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "carpenter workshop at (50,50,139) — UNDER CONSTRUCTION (stage 0/3)") {
		t.Fatalf("unbuilt building must show construction stage:\n%s", out)
	}
	if !strings.Contains(out, "bed at (12,8,138) — built") {
		t.Fatalf("finished building must read 'built':\n%s", out)
	}
	if strings.Contains(out, "truncated") {
		t.Fatalf("untruncated response must not claim truncation:\n%s", out)
	}

	trunc := []byte(`{"buildings":[{"type":"door","x":1,"y":2,"z":3,"stage":1,"max_stage":1,"done":true}],"truncated":true}`)
	if out := renderBuildings(trunc); !strings.Contains(out, "truncated") {
		t.Fatalf("truncated flag must surface:\n%s", out)
	}

	if out := renderBuildings([]byte(`{"buildings":[]}`)); out != "No buildings." {
		t.Fatalf("empty list rendering wrong: %q", out)
	}

	if out := renderBuildings([]byte(`not json`)); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderBuildingsBridgeState: a raised bridge is a wall/lid, a lowered
// one is a walkable floor — the model cannot tell them apart from any other
// view, so bridge_state (queries.cpp handleListBuildings, Bridge only) must
// surface in the buildings tool's text. Non-bridge buildings carry no such
// field and must not print a bracketed tag at all.
func TestRenderBuildingsBridgeState(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Bridge","x":10,"y":10,"z":100,"stage":1,"max_stage":1,"done":true,"bridge_state":"raised"},
		{"type":"Bridge","x":20,"y":20,"z":100,"stage":1,"max_stage":1,"done":true,"bridge_state":"lowered"},
		{"type":"Door","x":5,"y":5,"z":100,"stage":1,"max_stage":1,"done":true}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "Bridge at (10,10,100) — built [raised]\n") {
		t.Fatalf("raised bridge must be tagged:\n%s", out)
	}
	if !strings.Contains(out, "Bridge at (20,20,100) — built [lowered]\n") {
		t.Fatalf("lowered bridge must be tagged:\n%s", out)
	}
	if !strings.Contains(out, "Door at (5,5,100) — built\n") {
		t.Fatalf("a non-bridge building must not carry a bracketed state tag:\n%s", out)
	}
}

// TestRenderBuildingsStockpileFill: a Stockpile line must answer "is this
// full?" and "why is nothing hauled here?" from the buildings sweep the
// model already does — identity, extents, accepted categories, tile-fill.
// Non-stockpile lines stay byte-identical to the pre-feature format.
func TestRenderBuildingsStockpileFill(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":108,"y":102,"z":132,"x1":104,"y1":96,"x2":112,"y2":108,"stage":0,"max_stage":0,"done":true,
		 "sp_number":1,"sp_name":"","sp_categories":"all","sp_tiles":117,"sp_occupied":102,"sp_items":184},
		{"type":"Stockpile","x":95,"y":97,"z":132,"x1":90,"y1":95,"x2":100,"y2":99,"stage":0,"max_stage":0,"done":true,
		 "sp_number":2,"sp_name":"FoodHall","sp_categories":"food","sp_tiles":52,"sp_occupied":47,"sp_items":209},
		{"type":"Door","x":5,"y":5,"z":132,"x1":5,"y1":5,"x2":5,"y2":5,"stage":1,"max_stage":1,"done":true}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "- Stockpile #1 at (104,96)-(112,108) z=132 — accepts all, 102/117 tiles occupied (87%), 184 items\n") {
		t.Fatalf("unnamed stockpile line wrong:\n%s", out)
	}
	if !strings.Contains(out, "- Stockpile #2 \"FoodHall\" at (90,95)-(100,99) z=132 — accepts food, 47/52 tiles occupied (90%), 209 items\n") {
		t.Fatalf("named stockpile line wrong:\n%s", out)
	}
	if !strings.Contains(out, "- Door at (5,5,132) — built\n") {
		t.Fatalf("non-stockpile lines must be unchanged:\n%s", out)
	}
	if !strings.Contains(out, "(stockpile items count a bin/barrel as 1") {
		t.Fatalf("the container caveat must render once:\n%s", out)
	}
	if strings.Count(out, "count a bin/barrel as 1") != 1 {
		t.Fatalf("the container caveat must render ONCE, not per stockpile:\n%s", out)
	}
	if strings.Contains(out, "do not sum") {
		t.Fatalf("disjoint stockpiles must not claim an overlap:\n%s", out)
	}
}

// Two stockpiles over the same ground each count the shared tiles and the
// same items on them (the plugin measures each pile independently), so the
// per-pile numbers are individually right and DO NOT SUM. The `look` footer
// is set-deduped and reports the smaller, correct total for the same
// ground; a caller comparing the two surfaces would otherwise hit a bare
// contradiction. Disclose it on the existing caveat line — no second line.
func TestRenderBuildingsStockpileOverlapDisclosed(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":90,"y":89,"z":132,"x1":88,"y1":88,"x2":92,"y2":91,"done":true,
		 "sp_number":1,"sp_categories":"stone","sp_tiles":20,"sp_occupied":18,"sp_items":84},
		{"type":"Stockpile","x":90,"y":89,"z":132,"x1":88,"y1":88,"x2":92,"y2":91,"done":true,
		 "sp_number":2,"sp_categories":"wood","sp_tiles":18,"sp_occupied":18,"sp_items":84},
		{"type":"Stockpile","x":10,"y":10,"z":132,"x1":9,"y1":9,"x2":11,"y2":11,"done":true,
		 "sp_number":3,"sp_categories":"food","sp_tiles":9,"sp_occupied":1,"sp_items":2}
	]}`)
	out := renderBuildings(raw)
	want := "; 2 stockpile footprint rectangles overlap — shared tiles and their items are counted once per pile, so these numbers do not sum)"
	if !strings.Contains(out, want) {
		t.Fatalf("overlap clause missing from the caveat line:\n%s", out)
	}
	if strings.Count(out, "do not sum") != 1 {
		t.Fatalf("the overlap clause must render ONCE, not per stockpile:\n%s", out)
	}
}

// Same rectangles, different z: not an overlap. The disclosure must not
// fire on piles stacked vertically, which share no ground at all.
func TestRenderBuildingsStockpileOverlapIsPerZ(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":90,"y":89,"z":132,"x1":88,"y1":88,"x2":92,"y2":91,"done":true,
		 "sp_number":1,"sp_categories":"stone","sp_tiles":20,"sp_occupied":18,"sp_items":84},
		{"type":"Stockpile","x":90,"y":89,"z":131,"x1":88,"y1":88,"x2":92,"y2":91,"done":true,
		 "sp_number":2,"sp_categories":"wood","sp_tiles":20,"sp_occupied":0,"sp_items":0}
	]}`)
	if out := renderBuildings(raw); strings.Contains(out, "do not sum") {
		t.Fatalf("stockpiles on different z-levels do not overlap:\n%s", out)
	}
}

// A stockpile accepting nothing is a real state and the direct answer to
// "why is nothing being hauled here" — it must be named, not blanked.
func TestRenderBuildingsStockpileAcceptsNothing(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":10,"y":10,"z":100,"x1":9,"y1":9,"x2":11,"y2":11,"done":true,
		 "sp_number":7,"sp_categories":"none","sp_tiles":9,"sp_occupied":0,"sp_items":0}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "- Stockpile #7 at (9,9)-(11,11) z=100 — accepts none, 0/9 tiles occupied (0%), 0 items\n") {
		t.Fatalf("empty/no-category stockpile line wrong:\n%s", out)
	}
}

// VERSION SKEW: against the plugin build deployed before this feature the
// sp_* fields are absent. The line must still upgrade the useless center
// coordinate to the real extents (x1..y2 have shipped for a long time) and
// must NOT print fabricated "0/0 tiles occupied" numbers.
func TestRenderBuildingsStockpileOldPlugin(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":108,"y":102,"z":132,"x1":104,"y1":96,"x2":112,"y2":108,"stage":0,"max_stage":0,"done":true}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "- Stockpile at (104,96)-(112,108) z=132 — built (fill not reported by this plugin build)\n") {
		t.Fatalf("old-plugin stockpile line wrong:\n%s", out)
	}
	if strings.Contains(out, "occupied") || strings.Contains(out, "accepts") {
		t.Fatalf("an old plugin must not produce invented fill/category text:\n%s", out)
	}
	if strings.Contains(out, "count a bin/barrel as 1") {
		t.Fatalf("the fill caveat must not appear when no fill was reported:\n%s", out)
	}
}

// Container ceilings + live container count. A pile whose max_bins/
// max_barrels read 0 can never exceed one loose item per tile no matter how
// much floor it has — the state every DF-AI-created stockpile was silently
// in — so the numbers must be on the line, not inferred by cross-referencing
// `stocks category=bin` against tile math.
func TestRenderBuildingsStockpileContainers(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":95,"y":97,"z":132,"x1":90,"y1":95,"x2":100,"y2":99,"done":true,
		 "sp_number":2,"sp_name":"FoodHall","sp_categories":"food","sp_tiles":52,"sp_occupied":47,"sp_items":209,
		 "sp_max_bins":0,"sp_max_barrels":52,"sp_max_wheelbarrows":0,"sp_containers":11},
		{"type":"Stockpile","x":10,"y":10,"z":132,"x1":9,"y1":9,"x2":11,"y2":11,"done":true,
		 "sp_number":9,"sp_categories":"finished_goods","sp_tiles":9,"sp_occupied":9,"sp_items":9,
		 "sp_max_bins":0,"sp_max_barrels":0,"sp_max_wheelbarrows":0,"sp_containers":0}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "209 items, containers: 11 (max bins=0 barrels=52 wheelbarrows=0)\n") {
		t.Fatalf("container clause wrong on the barrel pile:\n%s", out)
	}
	// The zeroed pile is the bug's own signature and must render its zeros
	// verbatim — this is a MEASURED zero, unlike the old-plugin case below.
	if !strings.Contains(out, "9 items, containers: 0 (max bins=0 barrels=0 wheelbarrows=0)\n") {
		t.Fatalf("zero-ceiling pile must show its zeros:\n%s", out)
	}
}

// VERSION SKEW, container fields: a plugin build predating them omits
// sp_max_* / sp_containers entirely. Rendering that absence as "max bins=0"
// would invent a measurement indistinguishable from the real zero-ceiling
// bug, so the clause must be dropped whole while the rest of the line (which
// that build DOES report) stays intact.
func TestRenderBuildingsStockpileContainersOldPlugin(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":95,"y":97,"z":132,"x1":90,"y1":95,"x2":100,"y2":99,"done":true,
		 "sp_number":2,"sp_name":"FoodHall","sp_categories":"food","sp_tiles":52,"sp_occupied":47,"sp_items":209}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "- Stockpile #2 \"FoodHall\" at (90,95)-(100,99) z=132 — accepts food, 47/52 tiles occupied (90%), 209 items\n") {
		t.Fatalf("fill clause must survive without the container fields:\n%s", out)
	}
	if strings.Contains(out, "containers") || strings.Contains(out, "max bins") {
		t.Fatalf("an old plugin must not produce invented container numbers:\n%s", out)
	}
}

// A malformed payload carrying only some of the group still renders: the
// presence sentinel is sp_max_bins, and the rest degrade to 0 rather than
// panicking on a nil dereference.
func TestRenderBuildingsStockpileContainersPartialFields(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":10,"y":10,"z":100,"x1":9,"y1":9,"x2":11,"y2":11,"done":true,
		 "sp_number":7,"sp_categories":"gems","sp_tiles":9,"sp_occupied":0,"sp_items":0,
		 "sp_max_bins":9}
	]}`)
	out := renderBuildings(raw)
	if !strings.Contains(out, "containers: 0 (max bins=9 barrels=0 wheelbarrows=0)\n") {
		t.Fatalf("partial container group must render without panicking:\n%s", out)
	}
}

// An even older plugin sends no extents at all, and an unfinished
// stockpile plan has no fill to speak of — both fall through to the
// generic building line rather than rendering a half-formed rectangle.
func TestRenderBuildingsStockpileFallsThroughWithoutExtents(t *testing.T) {
	raw := []byte(`{"buildings":[
		{"type":"Stockpile","x":50,"y":50,"z":100,"stage":0,"max_stage":0,"done":true}
	]}`)
	if out := renderBuildings(raw); !strings.Contains(out, "- Stockpile at (50,50,100) — built\n") {
		t.Fatalf("extent-less stockpile must use the generic line:\n%s", out)
	}
}

// Fixtures across this file's stocks tests carry "units" equal to "count" —
// modelling a current plugin build that measured stack sizes and found no
// extra stacking for these types. That keeps each test exercising only its
// own feature instead of incidentally asserting against renderStocks'
// no-stack-data caveat (see TestRenderStocksUnits).
func TestRenderStocksAggregated(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":20,"units":20,"economic":false},
		{"item_type":"BOULDER","material":"chalk","count":11,"units":11,"economic":false},
		{"item_type":"BOULDER","material":"bauxite","count":9,"units":9,"economic":true},
		{"item_type":"WOOD","material":"oak","count":3,"units":3}
	]}`)
	out := renderStocks(raw, false, 0)
	if !strings.Contains(out, "2 item types, 4 item/material entries total") {
		t.Fatalf("missing aggregate header:\n%s", out)
	}
	if !strings.Contains(out, "BOULDER: 40 total across 3 materials (9 economic); top: shale 20, chalk 11, bauxite 9") {
		t.Fatalf("boulder aggregate line wrong (materials must sort by count desc, economic count must show):\n%s", out)
	}
	if !strings.Contains(out, "WOOD: 3 total across 1 material;") {
		t.Fatalf("singular 'material' wrong for a single entry:\n%s", out)
	}
	if strings.Contains(out, "shale x20") {
		t.Fatalf("aggregated view must not spell out raw per-material entries:\n%s", out)
	}
}

func TestRenderStocksDetailed(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":20,"units":20,"economic":false},
		{"item_type":"BOULDER","material":"bauxite","count":9,"units":9,"economic":true}
	]}`)
	out := renderStocks(raw, true, 0)
	if !strings.Contains(out, "2 item/material entries:") {
		t.Fatalf("missing detailed header:\n%s", out)
	}
	if !strings.Contains(out, "- BOULDER: shale x20\n") {
		t.Fatalf("non-economic entry must not carry the tag:\n%s", out)
	}
	if !strings.Contains(out, "- BOULDER: bauxite x9 [economic]\n") {
		t.Fatalf("economic entry must carry the tag:\n%s", out)
	}
}

func TestRenderStocksInUse(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BED","material":"oak","count":1,"units":1,"in_use":3,"in_use_units":3},
		{"item_type":"DOOR","material":"bronze","count":0,"units":0,"in_use":2,"in_use_units":2}
	]}`)

	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "- BED: oak x1 (+3 built-in)\n") {
		t.Fatalf("detailed in_use note wrong:\n%s", detailed)
	}
	if !strings.Contains(detailed, "- DOOR: bronze x0 (+2 built-in)\n") {
		t.Fatalf("a zero free count must still show its in_use note:\n%s", detailed)
	}

	agg := renderStocks(raw, false, 0)
	if !strings.Contains(agg, "- BED: 1 total across 1 material (+3 built-in); top: oak 1") {
		t.Fatalf("aggregated in_use note wrong:\n%s", agg)
	}
	if !strings.Contains(agg, "- DOOR: 0 total across 1 material (+2 built-in); top: bronze 0") {
		t.Fatalf("aggregated in_use note for an all-built-in type wrong:\n%s", agg)
	}

	noInUse := []byte(`{"items":[{"item_type":"TABLE","material":"granite","count":5,"units":5}]}`)
	if out := renderStocks(noInUse, true, 0); strings.Contains(out, "built-in") {
		t.Fatalf("absent in_use must not print a note:\n%s", out)
	}
	if out := renderStocks(noInUse, false, 0); strings.Contains(out, "built-in") {
		t.Fatalf("absent in_use must not print a note in aggregate view either:\n%s", out)
	}
}

// TestRenderStocksUnits covers the stack-unit total (Q3): a stackable item
// type (drink/food) reports far more real servings than its struct count
// alone suggests, and both views must show the extra number without
// disturbing a non-stacking type's plain "x%d" rendering.
func TestRenderStocksUnits(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"DRINK","material":"dwarven wine","count":8,"units":187,"in_use":0,"in_use_units":0},
		{"item_type":"BOULDER","material":"shale","count":5,"units":5}
	]}`)

	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "- DRINK: dwarven wine x8 (187 units)\n") {
		t.Fatalf("detailed stack-unit note wrong:\n%s", detailed)
	}
	if !strings.Contains(detailed, "- BOULDER: shale x5\n") {
		t.Fatalf("a non-stacking type (units==count) must not print a redundant units note:\n%s", detailed)
	}

	agg := renderStocks(raw, false, 0)
	if !strings.Contains(agg, "- DRINK: 8 total = 187 units across 1 material;") {
		t.Fatalf("aggregated stack-unit note wrong:\n%s", agg)
	}
	if !strings.Contains(agg, "- BOULDER: 5 total across 1 material;") {
		t.Fatalf("a non-stacking type must render without a units note in aggregate view:\n%s", agg)
	}

	// An older-plugin payload that omits "units"/"in_use_units" entirely
	// must SAY SO. The superseded assertion here required the opposite —
	// that such a response render with no mention of units at all — which
	// is precisely the bug: a stale plugin build's struct count (8 barrels)
	// then read as a fully-verified serving count, and a live session
	// undersold ~80 servings of wine on that silence. Units is a *int now,
	// so "plugin never spoke" and "plugin measured, no extra stacking" are
	// distinguishable, and only the former gets the caveat.
	noUnits := []byte(`{"items":[{"item_type":"DRINK","material":"ale","count":4}]}`)
	for _, tc := range []struct {
		name     string
		detailed bool
	}{{"detailed", true}, {"aggregate", false}} {
		out := renderStocks(noUnits, tc.detailed, 0)
		if !strings.Contains(out, "did not report per-item stack sizes") {
			t.Fatalf("%s view must carry the no-stack-data caveat:\n%s", tc.name, out)
		}
		if strings.Contains(out, "(4 units)") || strings.Contains(out, "= 4 units") {
			t.Fatalf("%s view must not fabricate a per-item units figure from the struct count:\n%s", tc.name, out)
		}
	}
}

// TestRenderStocksUnitsUnsupportedVsConfirmed pins the distinction the *int
// fields exist for: a response where the plugin measured every item and found
// no extra stacking (Units == Count, e.g. an all-boulder pile) is a POSITIVE
// confirmation and must render clean, while a response where the plugin never
// sent the field at all must carry the caveat. Before the pointer change both
// decoded identically and both rendered clean.
func TestRenderStocksUnitsUnsupportedVsConfirmed(t *testing.T) {
	confirmed := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":5,"units":5},
		{"item_type":"WOOD","material":"oak","count":3,"units":3}
	]}`)
	for _, tc := range []struct {
		name     string
		detailed bool
	}{{"detailed", true}, {"aggregate", false}} {
		out := renderStocks(confirmed, tc.detailed, 0)
		if strings.Contains(out, "did not report per-item stack sizes") {
			t.Fatalf("%s view must NOT caveat a response where every item carries measured units:\n%s", tc.name, out)
		}
	}

	// A partially-populated response still counts as "this plugin measures
	// stack sizes" — the caveat is a per-response capability statement, not
	// a per-item one, so one measured item suppresses it for the whole
	// response.
	//
	// INVARIANT THIS DEPENDS ON: the plugin emits "units"/"in_use_units"
	// UNCONDITIONALLY for every entry (queries.cpp:2477-2478), so a real
	// response is all-or-nothing and this mixed shape is unreachable from a
	// live plugin. If that ever becomes conditional, the response-level
	// check would understate a partly-measured payload — which is why the
	// second half below pins that no aggregate figure is FABRICATED for the
	// unmeasured entry even in this synthetic shape.
	mixed := []byte(`{"items":[
		{"item_type":"DRINK","material":"ale","count":4,"units":80},
		{"item_type":"BOULDER","material":"shale","count":5}
	]}`)
	if out := renderStocks(mixed, true, 0); strings.Contains(out, "did not report per-item stack sizes") {
		t.Fatalf("one measured item must suppress the response-level caveat:\n%s", out)
	}
	aggMixed := renderStocks(mixed, false, 0)
	if !strings.Contains(aggMixed, "- DRINK: 4 total = 80 units") {
		t.Fatalf("the measured type must still report its real unit total:\n%s", aggMixed)
	}
	for _, line := range strings.Split(aggMixed, "\n") {
		if strings.HasPrefix(line, "- BOULDER") && strings.Contains(line, "units") {
			t.Fatalf("an unmeasured entry must not get a fabricated aggregate units figure: %q", line)
		}
	}
}

// TestRenderStocksStackUnitsFlag: the plugin's explicit top-level
// "stack_units" capability flag (queries.cpp) is authoritative when present,
// and the pre-flag item scan is the fallback when it is not — a flag saying
// the build does NOT measure must caveat even if some item happens to carry
// a "units" key, and a pre-flag payload must behave exactly as before.
func TestRenderStocksStackUnitsFlag(t *testing.T) {
	// Flag present and true: no caveat, even though this is the same
	// all-measured payload the scan would also accept.
	withFlag := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":5,"units":5}
	],"stack_units":true}`)
	if out := renderStocks(withFlag, true, 0); strings.Contains(out, "did not report per-item stack sizes") {
		t.Fatalf("an explicit stack_units:true must suppress the caveat:\n%s", out)
	}

	// Flag present and false OUTRANKS a stray "units" key: the build itself
	// says it does not measure, which is exactly the regression shape the
	// flag exists to catch (one surviving units field would fool the scan).
	flagFalse := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":5,"units":5}
	],"stack_units":false}`)
	if out := renderStocks(flagFalse, true, 0); !strings.Contains(out, "did not report per-item stack sizes") {
		t.Fatalf("an explicit stack_units:false must caveat regardless of stray units keys:\n%s", out)
	}

	// No flag at all (pre-flag plugin build): fall back to the item scan.
	noFlag := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":5}
	]}`)
	if out := renderStocks(noFlag, true, 0); !strings.Contains(out, "did not report per-item stack sizes") {
		t.Fatalf("a pre-flag payload with no units data must still caveat via the scan:\n%s", out)
	}
}

// TestRenderStocksUnitsInUse covers the built-in half of the same feature:
// a built (in-use) stackable item's unit total should show alongside the
// existing "+N built-in" note, the same "> not !=" backward-compatible
// comparison as the free-stock side.
func TestRenderStocksUnitsInUse(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"FOOD","material":"roast","count":0,"units":0,"in_use":2,"in_use_units":40}
	]}`)
	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "- FOOD: roast x0 (+2 built-in = 40 units)\n") {
		t.Fatalf("detailed built-in units note wrong:\n%s", detailed)
	}
	agg := renderStocks(raw, false, 0)
	if !strings.Contains(agg, "(+2 built-in = 40 units)") {
		t.Fatalf("aggregated built-in units note wrong:\n%s", agg)
	}
}

func TestRenderStocksMinCount(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":20,"units":20},
		{"item_type":"BOULDER","material":"chalk","count":2,"units":2}
	]}`)
	out := renderStocks(raw, false, 5)
	if strings.Contains(out, "chalk") {
		t.Fatalf("entry below min_count must be filtered out:\n%s", out)
	}
	if !strings.Contains(out, "shale") {
		t.Fatalf("entry at/above min_count must survive:\n%s", out)
	}

	if out := renderStocks([]byte(`{"items":[{"item_type":"BOULDER","material":"chalk","count":2,"units":2}]}`), false, 5); out != "No stock items (or all below min_count)." {
		t.Fatalf("all-filtered response wrong: %q", out)
	}
}

// TestRenderStocksMinCountSurfacesInUse: a fully-built-in entry (Count=0,
// InUse>0) must survive a min_count filter even though its free count is
// below the threshold — min_count suppresses free-stock noise, not
// fort-existence, and a caller filtering on it must still be able to tell
// "this fort has some of these, all built in."
func TestRenderStocksMinCountSurfacesInUse(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"TABLE","material":"granite","count":0,"units":0,"in_use":3,"in_use_units":3},
		{"item_type":"TABLE","material":"oak","count":1,"units":1}
	]}`)

	detailed := renderStocks(raw, true, 5)
	if !strings.Contains(detailed, "- TABLE: granite x0 (+3 built-in)\n") {
		t.Fatalf("fully-built-in entry must survive min_count in detailed view:\n%s", detailed)
	}
	if strings.Contains(detailed, "oak x1") {
		t.Fatalf("free-only entry below min_count must still be filtered in detailed view:\n%s", detailed)
	}

	agg := renderStocks(raw, false, 5)
	if !strings.Contains(agg, "(+3 built-in)") {
		t.Fatalf("fully-built-in entry must survive min_count in aggregated view:\n%s", agg)
	}
	if strings.Contains(agg, "oak") {
		t.Fatalf("free-only entry below min_count must still be filtered in aggregated view:\n%s", agg)
	}
}

// TestRenderStocksQuality: the additive "quality" sibling array (only
// present for item types with at least one above-Ordinary item) must show
// up in both the aggregated and detailed views, and stay silent for a type
// with no quality entry at all.
func TestRenderStocksQuality(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BED","material":"oak","count":4,"units":4},
		{"item_type":"TABLE","material":"granite","count":2,"units":2}
	],"quality":[
		{"item_type":"BED","quality":"Ordinary","count":2},
		{"item_type":"BED","quality":"FinelyCrafted","count":1},
		{"item_type":"BED","quality":"Masterful","count":1}
	]}`)

	agg := renderStocks(raw, false, 0)
	if !strings.Contains(agg, "- BED: 4 total across 1 material [2 Ordinary, 1 FinelyCrafted, 1 Masterful]; top: oak 4") {
		t.Fatalf("aggregated quality breakdown wrong:\n%s", agg)
	}
	if !strings.Contains(agg, "- TABLE: 2 total across 1 material; top: granite 2") {
		t.Fatalf("a type with no quality entry must render without a breakdown:\n%s", agg)
	}

	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "  quality (BED): 2 Ordinary, 1 FinelyCrafted, 1 Masterful\n") {
		t.Fatalf("detailed quality breakdown wrong:\n%s", detailed)
	}
	if strings.Contains(detailed, "quality (TABLE)") {
		t.Fatalf("a type with no quality entry must not print a breakdown line:\n%s", detailed)
	}
}

// TestRenderStocksSubtypes: "WEAPON: iron x3" alone can't tell a pick from
// a battle axe — the additive "subtypes" array (WEAPON/TOOL only) must
// break that down in the detailed (category-filtered) view, sorted by
// count descending, while the aggregated default view stays exactly as
// compact as before (no subtype text at all).
func TestRenderStocksSubtypes(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"WEAPON","material":"iron","count":3,"units":3}
	],"subtypes":[
		{"item_type":"WEAPON","material":"iron","subtype_name":"pick","count":1},
		{"item_type":"WEAPON","material":"iron","subtype_name":"battle axe","count":2}
	]}`)

	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "  subtypes (WEAPON): 2 iron battle axe, 1 iron pick\n") {
		t.Fatalf("detailed subtype breakdown wrong (must sort by count desc):\n%s", detailed)
	}

	agg := renderStocks(raw, false, 0)
	if strings.Contains(agg, "subtypes") || strings.Contains(agg, "battle axe") || strings.Contains(agg, "pick") {
		t.Fatalf("aggregated default view must stay compact, no subtype text:\n%s", agg)
	}

	noSubtypes := []byte(`{"items":[{"item_type":"BOULDER","material":"shale","count":5,"units":5}]}`)
	if out := renderStocks(noSubtypes, true, 0); strings.Contains(out, "subtypes") {
		t.Fatalf("a type with no subtype entry must not print a breakdown line:\n%s", out)
	}
}

// TestRenderStocksContainers covers the empty-vs-full container visibility
// fix: "stocks shows 15 barrels" told a caller nothing about whether ANY of
// them were actually free to hold a new batch, and brewing kept getting
// cancelled ("needs empty food storage item") against a pile that was
// invisibly all full. containers==count (the common BARREL/BUCKET/BIN/BAG
// case) must render as the terse "(N empty)"; containers<count (TOOL, where
// a food-storage pot shares its material key with ordinary tools like picks)
// must render the more explicit "(N containers, M empty)" so a caller never
// misreads "20 total, 1 empty" as "19 of 20 barrels are full" when only 2 of
// the 20 were containers to begin with.
func TestRenderStocksContainers(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BARREL","material":"oak","count":15,"units":15,"containers":15,"empty":4},
		{"item_type":"BARREL","material":"willow","count":5,"units":5,"containers":5,"empty":3},
		{"item_type":"TOOL","material":"iron","count":5,"units":5,"containers":2,"empty":1}
	]}`)

	detailed := renderStocks(raw, true, 0)
	if !strings.Contains(detailed, "- BARREL: oak x15 (4 empty)\n") {
		t.Fatalf("full-coverage container entry must render '(N empty)':\n%s", detailed)
	}
	if !strings.Contains(detailed, "- BARREL: willow x5 (3 empty)\n") {
		t.Fatalf("second material's empty count wrong:\n%s", detailed)
	}
	if !strings.Contains(detailed, "- TOOL: iron x5 (2 containers, 1 empty)\n") {
		t.Fatalf("partial-coverage (TOOL picks + food-storage pots sharing a material) must spell out the container subset, not just 'empty':\n%s", detailed)
	}

	agg := renderStocks(raw, false, 0)
	if !strings.Contains(agg, "BARREL: 20 total (7 empty) across 2 materials") {
		t.Fatalf("aggregated container line wrong (must sum empty across materials):\n%s", agg)
	}
	if !strings.Contains(agg, "TOOL: 5 total (2 containers, 1 empty) across 1 material") {
		t.Fatalf("aggregated partial-coverage line wrong:\n%s", agg)
	}

	// A key with no "containers" field at all (an older plugin, or a
	// non-vessel type) must render with no container note whatsoever —
	// containers defaults to its Go zero value (0), which formatContainerNote
	// must treat as "not a container-bearing key" rather than fabricating a
	// "(0 empty)" note.
	noContainers := []byte(`{"items":[{"item_type":"BOULDER","material":"shale","count":5,"units":5}]}`)
	if out := renderStocks(noContainers, true, 0); strings.Contains(out, "empty") {
		t.Fatalf("a type with no container data must not print an empty-count note:\n%s", out)
	}
	if out := renderStocks(noContainers, false, 0); strings.Contains(out, "empty") {
		t.Fatalf("aggregated view must also omit the note when there's no container data:\n%s", out)
	}
}

func TestRenderStocksBadJSON(t *testing.T) {
	if out := renderStocks([]byte(`not json`), false, 0); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderAlerts: DF's report struct never rewrites pos on a repeat (see
// announcements.cpp / worldmodel.AlertStore.Add) — once RepeatCount>0 the
// coordinate is stale, first-occurrence-only, and must be labeled as such
// rather than presented as current.
func TestRenderAlerts(t *testing.T) {
	if out := renderAlerts(nil, 0); out != "No active alerts." {
		t.Fatalf("empty alert list rendering wrong: %q", out)
	}

	alerts := []worldmodel.Alert{
		{ID: 1, Severity: 1, Text: "Digging designation cancelled: damp stone located.", X: 40, Y: 40, Z: 133, RepeatCount: 0},
		{ID: 2, Severity: 2, Text: "A vile force of darkness has arrived!", X: 12, Y: 12, Z: 100, RepeatCount: 5},
		{ID: 3, Severity: 0, Text: "Losing a friend deeply upset Urist.", X: -1, Y: -1, Z: -1},
	}
	out := renderAlerts(alerts, 3)
	if !strings.Contains(out, "- [1] sev=1 Digging designation cancelled: damp stone located. @(40,40,133)\n") {
		t.Fatalf("a never-repeated alert must show a bare position, no repeat tag:\n%s", out)
	}
	if !strings.Contains(out, "- [2] sev=2 A vile force of darkness has arrived! @(12,12,100) [first occurrence; repeated x6]\n") {
		t.Fatalf("a repeated alert must mark its position as first-occurrence and show DF's own x(N+1) convention:\n%s", out)
	}
	if !strings.Contains(out, "- [3] sev=0 Losing a friend deeply upset Urist.\n") {
		t.Fatalf("a non-positional alert must not print any position or repeat tag:\n%s", out)
	}

	if out := renderAlerts(alerts[:1], 3); !strings.Contains(out, "... and 2 more (dismiss some to see them)\n") {
		t.Fatalf("truncated active count must surface a hint:\n%s", out)
	}
}

func TestRenderManagerOrders(t *testing.T) {
	// Covers every rung of the C3 status ladder: queued (default) ->
	// workshop assigned, awaiting worker -> in progress (via
	// jobs_in_progress OR the plain active flag) -> invalid. validated is
	// always true in real plugin output (force-set at order creation) but
	// the invalid rung is still rendered truthfully if a future DF version
	// ever makes that false.
	raw := []byte(`{"orders":[
		{"id":0,"job_type":"ConstructBed","amount_total":2,"amount_left":2,"validated":true,"active":false,"jobs_in_progress":0,"workshop_assigned":false},
		{"id":1,"job_type":"BrewDrink","amount_total":5,"amount_left":3,"validated":true,"active":true,"jobs_in_progress":0,"workshop_assigned":true},
		{"id":2,"job_type":"MakeRock","amount_total":1,"amount_left":1,"validated":true,"active":false,"jobs_in_progress":2,"workshop_assigned":true},
		{"id":3,"job_type":"ForgeAnvil","amount_total":1,"amount_left":1,"validated":true,"active":false,"jobs_in_progress":0,"workshop_assigned":true},
		{"id":4,"job_type":"CutGems","amount_total":1,"amount_left":1,"validated":false,"active":false,"jobs_in_progress":0,"workshop_assigned":false}
	]}`)
	out := renderManagerOrders(raw)
	if !strings.Contains(out, "5 manager orders:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "id=0 ConstructBed x2 (2 left) — queued, awaiting manager dispatch") {
		t.Fatalf("no-progress order rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "id=1 BrewDrink x5 (3 left) — in progress") {
		t.Fatalf("active order rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "id=2 MakeRock x1 (1 left) — in progress (2 jobs)") {
		t.Fatalf("jobs_in_progress order rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "id=3 ForgeAnvil x1 (1 left) — workshop assigned, awaiting worker") {
		t.Fatalf("workshop-assigned-only order rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "id=4 CutGems x1 (1 left) — invalid") {
		t.Fatalf("unvalidated order rendered wrong:\n%s", out)
	}
	if strings.Contains(out, "list_orders") || strings.Contains(out, "job_type\":") {
		t.Fatalf("must not leak the raw job-type catalog or JSON:\n%s", out)
	}
	if out := renderManagerOrders([]byte(`{"orders":[]}`)); out != "No manager orders queued." {
		t.Fatalf("empty orders rendering wrong: %q", out)
	}
}

// TestRenderManagerOrdersNoLaborForJob covers the missing-labor rung added to
// the same C3 ladder above. Live incident: a fort's wood-furniture orders read
// "queued, awaiting manager dispatch" for game-weeks because no work detail
// held CARPENTER and no citizen had it via automatic professions either — the
// order was permanently undispatchable and the tool said it was fine.
// Precedence matters as much as the message: observed progress outranks this
// static prediction, but this outranks the much weaker "workshop assigned,
// awaiting worker" (DF assigns a workshop provisionally even when nobody can
// ever take the job).
func TestRenderManagerOrdersNoLaborForJob(t *testing.T) {
	// (a) fires: labor resolved, nobody in the fort holds it.
	blocked := []byte(`{"orders":[
		{"id":0,"job_type":"ConstructBed","amount_total":2,"amount_left":2,"validated":true,"active":false,
		 "jobs_in_progress":0,"workshop_assigned":false,"required_labor":"CARPENTER","labor_available":false}
	]}`)
	out := renderManagerOrders(blocked)
	if !strings.Contains(out, "id=0 ConstructBed x2 (2 left) — no citizen on-site currently has CARPENTER enabled") {
		t.Fatalf("labor-starved order must name the missing labor:\n%s", out)
	}

	// (b) does not misfire: same shape, a citizen genuinely has the labor.
	staffed := []byte(`{"orders":[
		{"id":1,"job_type":"ConstructBed","amount_total":2,"amount_left":2,"validated":true,"active":false,
		 "jobs_in_progress":0,"workshop_assigned":false,"required_labor":"CARPENTER","labor_available":true}
	]}`)
	outStaffed := renderManagerOrders(staffed)
	if !strings.Contains(outStaffed, "id=1 ConstructBed x2 (2 left) — queued, awaiting manager dispatch") {
		t.Fatalf("a staffed labor must fall through to the plain queued rung:\n%s", outStaffed)
	}
	if strings.Contains(outStaffed, "no citizen on-site") {
		t.Fatalf("the missing-labor rung must not fire when labor_available is true:\n%s", outStaffed)
	}

	// (c) precedence: real observed progress outranks the static prediction.
	inProgress := []byte(`{"orders":[
		{"id":2,"job_type":"ConstructBed","amount_total":2,"amount_left":2,"validated":true,"active":false,
		 "jobs_in_progress":2,"workshop_assigned":true,"required_labor":"CARPENTER","labor_available":false}
	]}`)
	outProgress := renderManagerOrders(inProgress)
	if !strings.Contains(outProgress, "id=2 ConstructBed x2 (2 left) — in progress (2 jobs)") {
		t.Fatalf("observed jobs_in_progress must outrank the missing-labor prediction:\n%s", outProgress)
	}

	// (d) precedence: the missing-labor blocker outranks "workshop assigned,
	// awaiting worker", which is the weaker and more misleading claim.
	assigned := []byte(`{"orders":[
		{"id":3,"job_type":"ConstructBin","amount_total":1,"amount_left":1,"validated":true,"active":false,
		 "jobs_in_progress":0,"workshop_assigned":true,"required_labor":"CARPENTER","labor_available":false}
	]}`)
	outAssigned := renderManagerOrders(assigned)
	if !strings.Contains(outAssigned, "id=3 ConstructBin x1 (1 left) — no citizen on-site currently has CARPENTER enabled") {
		t.Fatalf("the missing-labor rung must outrank 'workshop assigned, awaiting worker':\n%s", outAssigned)
	}
	if strings.Contains(outAssigned, "awaiting worker") {
		t.Fatalf("the weaker workshop-assigned claim must not also print:\n%s", outAssigned)
	}

	// (e) old-plugin degrade: an order carrying neither field decodes to
	// ""/false and must keep falling through to the pre-existing rungs
	// rather than claiming a blocker nothing proved.
	legacy := []byte(`{"orders":[
		{"id":4,"job_type":"ConstructTable","amount_total":1,"amount_left":1,"validated":true,"active":false,
		 "jobs_in_progress":0,"workshop_assigned":true}
	]}`)
	outLegacy := renderManagerOrders(legacy)
	if !strings.Contains(outLegacy, "id=4 ConstructTable x1 (1 left) — workshop assigned, awaiting worker") {
		t.Fatalf("a payload with no labor fields must render exactly as before:\n%s", outLegacy)
	}
}

func TestRenderMandates(t *testing.T) {
	raw := []byte(`{"mandates":[
		{"index":0,"mode":"Export","item_type":"ANY","item_subtype":-1,"material":"copper",
		 "amount_total":0,"amount_remaining":0,"timeout_counter":100,"timeout_limit":25100,
		 "ticks_remaining":250000,"issued_by":{"id":42,"first_name":"Urist"},
		 "punishment":{"hammerstrikes":0,"prison_months":0,"beating":true,"exiled":false,
		 "death_sentence":false,"no_prison_available":false},"punish_multiple":false,"total_exempt":false},
		{"index":1,"mode":"Make","item_type":"WEAPON","item_subtype":3,"item_subtype_name":"short sword","material":"steel",
		 "amount_total":5,"amount_remaining":2,"timeout_counter":0,"timeout_limit":5000,
		 "ticks_remaining":50000,"issued_by":null,
		 "punishment":{"hammerstrikes":0,"prison_months":0,"beating":false,"exiled":false,
		 "death_sentence":false,"no_prison_available":false},"punish_multiple":false,"total_exempt":false}
	]}`)
	out := renderMandates(raw)
	if !strings.Contains(out, "2 active mandates:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "[0] Export mandate: copper ANY — 0/0 remaining — 250000 ticks remaining — issued by Urist (id 42) — punishment: beating") {
		t.Fatalf("export mandate rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "[1] Make mandate: steel WEAPON (short sword, subtype 3) — 2/5 remaining — 50000 ticks remaining — issuer unresolved — punishment: none recorded") {
		t.Fatalf("make mandate rendered wrong:\n%s", out)
	}
	if out := renderMandates([]byte(`{"mandates":[]}`)); out != "No active mandates." {
		t.Fatalf("empty mandates rendering wrong: %q", out)
	}
}

func TestRenderFortWealth(t *testing.T) {
	raw := []byte(`{"wealth":{"total":48200,"weapons":1200,"armor":800,"furniture":15000,
		"other":300,"architecture":25000,"displayed":5000,"held":40000,"imported":2000,
		"offered":100,"exported":900},"fortress_age":340}`)
	out := renderFortWealth(raw)
	if !strings.Contains(out, "Total wealth: 48200 (weapons 1200, armor 800, furniture 15000, architecture 25000, other 300)") {
		t.Fatalf("wealth totals rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "displayed 5000, held 40000, imported 2000, offered 100, exported 900") {
		t.Fatalf("wealth secondary fields rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "fortress_age: 340") {
		t.Fatalf("fortress_age missing:\n%s", out)
	}
	if out := renderFortWealth([]byte(`not json`)); !strings.Contains(out, "unparseable fort_wealth response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderWellbeing(t *testing.T) {
	raw := []byte(`{"citizens":[
		{"id":1,"first_name":"Urist","stress":15000,"stress_category":2,"current_focus":50,"undistracted_focus":116,"focus_ratio":0.431},
		{"id":2,"first_name":"","stress":-30000,"stress_category":5,"current_focus":10,"undistracted_focus":80,"focus_ratio":0.125}
	]}`)
	out := renderWellbeing(raw)
	if !strings.Contains(out, "2 citizens:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "Urist (id=1): stress=15000 (category 2/6), focus=50/116 (ratio 0.43)") {
		t.Fatalf("citizen line rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "dwarf#2 (id=2): stress=-30000 (category 5/6)") {
		t.Fatalf("empty first_name must fall back to a synthesized name:\n%s", out)
	}

	if out := renderWellbeing([]byte(`{"citizens":[]}`)); out != "No living citizens found." {
		t.Fatalf("empty roster rendering wrong: %q", out)
	}
	if out := renderWellbeing([]byte(`not json`)); !strings.Contains(out, "unparseable wellbeing response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderWellbeingNoSoulData(t *testing.T) {
	raw := []byte(`{"citizens":[{"id":3,"first_name":"Baby","stress":null,"stress_category":null,"current_focus":null,"undistracted_focus":null,"focus_ratio":null}]}`)
	out := renderWellbeing(raw)
	if !strings.Contains(out, "Baby (id=3): no soul data") {
		t.Fatalf("citizen with no current_soul must render as no soul data:\n%s", out)
	}
}

func TestRenderMandatesBadJSON(t *testing.T) {
	if out := renderMandates([]byte(`not json`)); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestRenderWorkDetails(t *testing.T) {
	raw := []byte(`{"work_details":[
		{"index":0,"name":"Miners","icon":"MINERS","mode":"EverybodyDoesThis","no_modify":true,"cannot_be_everybody":false,
		 "allowed_labors":["MINE"],"assigned_units":[{"id":1,"first_name":"Urist"},{"id":2,"first_name":null}]},
		{"index":10,"name":"Ore Haulers","icon":"CUSTOM_1","mode":"OnlySelectedDoesThis","no_modify":false,"cannot_be_everybody":true,
		 "allowed_labors":["HAUL_STONE","HAUL_ITEM"],"assigned_units":[]}
	]}`)
	out := renderWorkDetails(raw)
	if !strings.Contains(out, "2 work details:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, `- index=0 "Miners" (icon MINERS, mode EverybodyDoesThis) [no_modify]`) {
		t.Fatalf("vanilla detail rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "labors: MINE") {
		t.Fatalf("allowed labors missing:\n%s", out)
	}
	if !strings.Contains(out, "assigned: Urist (id 1), dwarf#2 (id 2)") {
		t.Fatalf("assigned units rendered wrong (unresolved name must fall back):\n%s", out)
	}
	if !strings.Contains(out, `- index=10 "Ore Haulers" (icon CUSTOM_1, mode OnlySelectedDoesThis) [cannot_be_everybody]`) {
		t.Fatalf("custom detail rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "labors: HAUL_STONE, HAUL_ITEM") {
		t.Fatalf("multi-labor list rendered wrong:\n%s", out)
	}

	if out := renderWorkDetails([]byte(`{"work_details":[]}`)); out != "No work details defined." {
		t.Fatalf("empty work_details rendering wrong: %q", out)
	}
	if out := renderWorkDetails([]byte(`not json`)); !strings.Contains(out, "unparseable work_details response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderMoods(t *testing.T) {
	raw := []byte(`{"moods":[
		{"job_id":100,"job_type":"StrangeMoodForge","unit_id":5,"first_name":"Urist","mood":"Fey",
		 "moodstage":"WORKING","mood_skill":"WEAPONSMITH","mood_timeout":42000,
		 "claimed_building":{"id":7,"type":"Furnace","name":"Forge#3"},
		 "needed_items":[
			{"item_type":"BAR","material":"steel","quantity_needed":5,"quantity_got":5},
			{"item_type":"WEAPON","is_any_inorganic":true,"quantity_needed":1,"quantity_got":0}
		 ]},
		{"job_id":101,"job_type":"StrangeMoodBrooding","unit_id":null,"first_name":null,"mood":null,
		 "moodstage":null,"mood_skill":null,"mood_timeout":null,"claimed_building":null,"needed_items":[]}
	]}`)
	out := renderMoods(raw)
	if !strings.Contains(out, "2 active strange moods:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "Urist (id=5): Fey [WORKING] — skill: WEAPONSMITH — wants: Metalsmith's Forge — claimed Forge#3 (id=7) — mood_timeout: 42000/50000 ticks remaining") {
		t.Fatalf("full mood entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "needs: steel BAR — got 5 of 5 (satisfied)") {
		t.Fatalf("satisfied need rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "needs: any WEAPON — got 0 of 1\n") {
		t.Fatalf("unsatisfied wildcard need rendered wrong (must not claim satisfied):\n%s", out)
	}
	if !strings.Contains(out, "(unit unresolved): ? — wants: (macabre mood — no workshop claim) — not yet claimed a workshop — mood_timeout: unknown ticks remaining") {
		t.Fatalf("all-nil mood entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "(no item requirements listed)") {
		t.Fatalf("missing no-items notice for a mood with no needed_items:\n%s", out)
	}

	if out := renderMoods([]byte(`{"moods":[]}`)); out != "No strange moods currently active." {
		t.Fatalf("empty moods rendering wrong: %q", out)
	}
	if out := renderMoods([]byte(`not json`)); !strings.Contains(out, "unparseable moods response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderNobleDemands(t *testing.T) {
	raw := []byte(`{"nobles":[
		{"unit_id":10,"first_name":"Sarvesh","positions":[
			{"code":"MAYOR","name":"Mayor","precedence":1,"responsibilities":["LAW_MAKING","ACCOUNTING"],
			 "required_office":150,"required_bedroom":100,"required_dining":0,"required_tomb":0,
			 "required_boxes":2,"required_cabinets":1,"required_racks":0,"required_stands":0}
		],"demands":[
			{"place":"Office","item_type":"TABLE","item_subtype":-1,"material":"marble","ticks_remaining":5000},
			{"place":"Bedroom","item_type":"WEAPON","item_subtype":3,"item_subtype_name":"long sword","ticks_remaining":8000}
		]},
		{"unit_id":20,"first_name":"","positions":[
			{"code":"BOOKKEEPER","name":"Bookkeeper","precedence":5,"responsibilities":[],
			 "required_office":0,"required_bedroom":0,"required_dining":0,"required_tomb":0,
			 "required_boxes":0,"required_cabinets":0,"required_racks":0,"required_stands":0}
		],"demands":[]}
	]}`)
	out := renderNobleDemands(raw)
	if !strings.Contains(out, "2 nobles:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "- Sarvesh (id=10)") {
		t.Fatalf("named noble rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "position: Mayor (precedence 1) — office>=150, bedroom>=100, boxes>=2, cabinets>=1") {
		t.Fatalf("nonzero requirements rendered wrong (zero-value fields must be omitted):\n%s", out)
	}
	if !strings.Contains(out, "responsibilities: LAW_MAKING, ACCOUNTING") {
		t.Fatalf("responsibilities line rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "DEMANDS: marble TABLE in Office — 5000 ticks remaining") {
		t.Fatalf("materialed demand rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "DEMANDS: WEAPON (long sword) in Bedroom — 8000 ticks remaining") {
		t.Fatalf("subtype-named demand rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "dwarf#20 (id=20)") {
		t.Fatalf("empty first_name must fall back to a synthesized name:\n%s", out)
	}
	if !strings.Contains(out, "position: Bookkeeper (precedence 5) — no room/furniture requirements") {
		t.Fatalf("all-zero requirements must render as no requirements:\n%s", out)
	}
	if !strings.Contains(out, "no active demands") {
		t.Fatalf("empty demands list must render 'no active demands':\n%s", out)
	}

	if out := renderNobleDemands([]byte(`{"nobles":[]}`)); out != "No noble positions currently held." {
		t.Fatalf("empty nobles rendering wrong: %q", out)
	}
	if out := renderNobleDemands([]byte(`not json`)); !strings.Contains(out, "unparseable noble_demands response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderPositionVacancies(t *testing.T) {
	raw := []byte(`{"entities":[
		{"entity_id":3,"role":"group","positions":[
			{"code":"MANAGER","name":"Manager","precedence":10,"active":true,"elected":false,
			 "requires_market":false,"has_met_market_req":false,"requires_population":0,"has_met_pop_req":true,
			 "responsibilities":[],"required_office":0,"required_bedroom":0,"required_dining":0,"required_tomb":0,
			 "required_boxes":0,"required_cabinets":0,"required_racks":0,"required_stands":0,
			 "assignments":[{"assignment_id":1,"vacant":true,"appointable":true,"in_possible_elected":false,
			                 "holder_hist_figure_id":-1,"holder_unit_id":-1,"holder_name":""}]},
			{"code":"BOOKKEEPER","name":"Bookkeeper","precedence":5,"active":true,"elected":false,
			 "requires_market":false,"has_met_market_req":false,"requires_population":0,"has_met_pop_req":true,
			 "responsibilities":["ACCOUNTING"],"required_office":0,"required_bedroom":0,"required_dining":0,"required_tomb":0,
			 "required_boxes":0,"required_cabinets":0,"required_racks":0,"required_stands":0,
			 "assignments":[{"assignment_id":2,"vacant":false,"appointable":true,"in_possible_elected":false,
			                 "holder_hist_figure_id":55,"holder_unit_id":12,"holder_name":"Urist"}]},
			{"code":"BROKER","name":"Broker","precedence":20,"active":false,"elected":false,
			 "requires_market":true,"has_met_market_req":false,"requires_population":0,"has_met_pop_req":true,
			 "responsibilities":[],"required_office":0,"required_bedroom":0,"required_dining":0,"required_tomb":0,
			 "required_boxes":0,"required_cabinets":0,"required_racks":0,"required_stands":0,
			 "assignments":[]}
		]},
		{"entity_id":1,"role":"civ","positions":[
			{"code":"MAYOR","name":"Mayor","precedence":1,"active":true,"elected":true,
			 "requires_market":false,"has_met_market_req":false,"requires_population":50,"has_met_pop_req":true,
			 "responsibilities":["LAW_MAKING"],"required_office":150,"required_bedroom":100,"required_dining":0,"required_tomb":0,
			 "required_boxes":0,"required_cabinets":0,"required_racks":0,"required_stands":0,
			 "assignments":[{"assignment_id":3,"vacant":true,"appointable":false,"in_possible_elected":true,
			                 "holder_hist_figure_id":-1,"holder_unit_id":-1,"holder_name":""}]}
		]}
	]}`)
	out := renderPositionVacancies(raw)
	if !strings.Contains(out, "group entity (id=3):") {
		t.Fatalf("missing group entity header:\n%s", out)
	}
	if !strings.Contains(out, "civ entity (id=1):") {
		t.Fatalf("missing civ entity header:\n%s", out)
	}
	if !strings.Contains(out, "VACANT (assignment#1) — appointable") {
		t.Fatalf("vacant+appointable slot rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "held by Urist (unit#12)") {
		t.Fatalf("filled slot with resolved unit rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "no assignment slot yet (not unlocked)") {
		t.Fatalf("position with no assignment record must say so:\n%s", out)
	}
	if !strings.Contains(out, "market requirement unmet") {
		t.Fatalf("unmet market requirement flag missing:\n%s", out)
	}
	if !strings.Contains(out, "VACANT (assignment#3) — NOT directly appointable, elected") {
		t.Fatalf("elected-only vacant slot (MAYOR) rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "ELECTED") {
		t.Fatalf("MAYOR's ELECTED position flag missing:\n%s", out)
	}

	if out := renderPositionVacancies([]byte(`{"entities":[]}`)); !strings.Contains(out, "No entities found") {
		t.Fatalf("empty entities rendering wrong: %q", out)
	}
	if out := renderPositionVacancies([]byte(`not json`)); !strings.Contains(out, "unparseable position_vacancies response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderCaravanStatus(t *testing.T) {
	raw := []byte(`{"caravans":[
		{"civ":"The Copper Horizon","entity_id":7,"trade_state":"AtDepot","time_remaining_ticks":12000,
		 "days_remaining":10.0,"tribute":false,"casualty":false,"hardship":false,"seized":false,
		 "offended":false,"greatly_offended":false,"import_value":500,"export_value_total":1200,
		 "export_value_personal":300,"offer_value":0,"mood":0,"haggle_fail_count":0,
		 "liaison_meeting_active":true}
	],"pending_events":[
		{"type":"TributeCaravan","civ":"The Iron Assembly","season":"Autumn","season_ticks_remaining":4000}
	],"depot":{"exists":true,"x":28,"y":52,"z":110,"built":true,"accessible":true,"walkable_from_edge":true,
		"trader_requested":true,"anyone_can_trade":false}}`)
	out := renderCaravanStatus(raw)
	if !strings.Contains(out, "1 caravan on the map:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "- The Copper Horizon: AtDepot, 10.0 days remaining (12000 ticks)") {
		t.Fatalf("caravan line rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "value: import=500 export_total=1200 export_personal=300 offer=0 mood=0 haggle_fails=0") {
		t.Fatalf("caravan value line rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "liaison meeting active") {
		t.Fatalf("liaison_meeting_active must be surfaced:\n%s", out)
	}
	if !strings.Contains(out, "1 scheduled event not yet arrived:") {
		t.Fatalf("missing pending-events header:\n%s", out)
	}
	if !strings.Contains(out, "- TributeCaravan: The Iron Assembly, Autumn season, 4000 ticks remaining in season") {
		t.Fatalf("pending event line rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "Depot at (28,52,110): built=true accessible=true walkable_from_edge=true trader_requested=true anyone_can_trade=false") {
		t.Fatalf("depot line rendered wrong:\n%s", out)
	}
	if strings.Contains(out, "access diagnostic") {
		t.Fatalf("accessible=true must not print the access diagnostic hint:\n%s", out)
	}

	// Active flags must render as a bracketed suffix; no active flags must
	// omit it entirely (compact common case).
	flagged := []byte(`{"caravans":[
		{"civ":"","entity_id":3,"trade_state":"Stuck","time_remaining_ticks":100,"days_remaining":0.1,
		 "tribute":false,"casualty":false,"hardship":false,"seized":true,"offended":true,
		 "greatly_offended":false,"import_value":0,"export_value_total":0,"export_value_personal":0,
		 "offer_value":0,"mood":0,"haggle_fail_count":0,"liaison_meeting_active":false}
	],"pending_events":[],"depot":{"exists":false}}`)
	out2 := renderCaravanStatus(flagged)
	if !strings.Contains(out2, "- entity#3: Stuck, 0.1 days remaining (100 ticks) [seized,offended]") {
		t.Fatalf("empty civ must fall back to entity id, active flags must render bracketed:\n%s", out2)
	}
	if strings.Contains(out2, "liaison meeting active") {
		t.Fatalf("liaison_meeting_active=false must not print the active line:\n%s", out2)
	}
	if !strings.Contains(out2, "No trade depot built yet.") {
		t.Fatalf("depot.exists=false must render no-depot line:\n%s", out2)
	}

	empty := []byte(`{"caravans":[],"pending_events":[],"depot":{"exists":false}}`)
	outEmpty := renderCaravanStatus(empty)
	if !strings.Contains(outEmpty, "No caravan currently on the map.") {
		t.Fatalf("empty caravans rendering wrong:\n%s", outEmpty)
	}
	if strings.Contains(outEmpty, "scheduled event") {
		t.Fatalf("empty pending_events must not print a pending-events header:\n%s", outEmpty)
	}

	if out := renderCaravanStatus([]byte(`not json`)); !strings.Contains(out, "unparseable caravan_status response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderCaravanStatusAccessDiagnostic(t *testing.T) {
	// accessible=false + walkable_from_edge=true -> wagon-width chokepoint hint.
	chokepoint := []byte(`{"caravans":[],"pending_events":[],
		"depot":{"exists":true,"x":1,"y":2,"z":3,"built":true,"accessible":false,"walkable_from_edge":true,
			"trader_requested":false,"anyone_can_trade":false}}`)
	out := renderCaravanStatus(chokepoint)
	if !strings.Contains(out, "wagon-corridor WIDTH chokepoint") {
		t.Fatalf("accessible=false+walkable_from_edge=true must hint at a width chokepoint:\n%s", out)
	}

	// accessible=false + walkable_from_edge=false -> topologically cut off hint.
	cutOff := []byte(`{"caravans":[],"pending_events":[],
		"depot":{"exists":true,"x":1,"y":2,"z":3,"built":true,"accessible":false,"walkable_from_edge":false,
			"trader_requested":false,"anyone_can_trade":false}}`)
	out2 := renderCaravanStatus(cutOff)
	if !strings.Contains(out2, "topologically cut off entirely") {
		t.Fatalf("accessible=false+walkable_from_edge=false must hint at full disconnection:\n%s", out2)
	}
}

func TestRenderDepotGoods(t *testing.T) {
	raw := []byte(`{"depot":{"x":28,"y":52,"z":110,"built":true},
		"staged_ours":[
			{"item_type":"CRAFTS","material":"silver","count":3,"value":900,"requested":true},
			{"item_type":"BOULDER","material":"shale","count":5,"value":50,"requested":false}
		],
		"staged_theirs":[
			{"item_type":"ANVIL","material":"copper","count":1,"value":600,"requested":false}
		],
		"pending":[
			{"item_type":"WEAPON","material":"iron","count":1,"value":200,"requested":false}
		]}`)
	out := renderDepotGoods(raw)
	if !strings.Contains(out, "Depot at (28,52,110) (built=true):") {
		t.Fatalf("depot header rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "staged (ours):") {
		t.Fatalf("missing staged (ours) section header:\n%s", out)
	}
	if !strings.Contains(out, "silver CRAFTS x3 (value 900) (requested by liaison)") {
		t.Fatalf("requested staged_ours entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "shale BOULDER x5 (value 50)") || strings.Contains(out, "shale BOULDER x5 (value 50) (requested") {
		t.Fatalf("non-requested staged_ours entry must not carry the requested suffix:\n%s", out)
	}
	if !strings.Contains(out, "staged (theirs / merchant goods):") {
		t.Fatalf("missing staged (theirs) section header:\n%s", out)
	}
	if !strings.Contains(out, "copper ANVIL x1 (value 600)") {
		t.Fatalf("staged_theirs entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "pending (hauling, not yet arrived):") {
		t.Fatalf("pending section header rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "iron WEAPON x1 (value 200)") {
		t.Fatalf("pending entry rendered wrong:\n%s", out)
	}

	emptyAll := []byte(`{"depot":{"x":1,"y":2,"z":3,"built":false},"staged_ours":[],"staged_theirs":[],"pending":[]}`)
	outEmpty := renderDepotGoods(emptyAll)
	if !strings.Contains(outEmpty, "staged (ours): none") ||
		!strings.Contains(outEmpty, "staged (theirs / merchant goods): none") ||
		!strings.Contains(outEmpty, "pending (hauling, not yet arrived): none") {
		t.Fatalf("empty staged_ours/staged_theirs/pending must render 'none':\n%s", outEmpty)
	}

	if out := renderDepotGoods([]byte(`not json`)); !strings.Contains(out, "unparseable depot_goods response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderTradeAgreements(t *testing.T) {
	// Full case: both agreement halves present, plus an open liaison
	// meeting with one resolved topic.
	raw := []byte(`{"civs":[
		{"civ":"The Gray Mansion","entity_id":7,
		 "specific_item_requests":[
			{"item_type":"AMULET","item_subtype":-1,"material":"silver","price_percent":150},
			{"item_type":"WEAPON","item_subtype":3,"material":"any","price_percent":200}
		 ],
		 "category_agreement":[
			{"category":"Cheese","count":2,"min_percent":110,"max_percent":130},
			{"category":"Weapons","count":1,"min_percent":100,"max_percent":100}
		 ],
		 "meeting_active":true,
		 "topics_under_discussion":["GiveGift","RequestTribute"],
		 "diplomat":"Adil Rithdolen","associate":"",
		 "resolved_this_meeting":[
			{"type":"AcceptAgreement","topic":"ImportAgreement","quota_total":5000,"quota_remaining":3000,"year":100,"ticks":42}
		 ]}
	]}`)
	out := renderTradeAgreements(raw)
	if !strings.Contains(out, "The Gray Mansion:") {
		t.Fatalf("missing civ header:\n%s", out)
	}
	if !strings.Contains(out, "AMULET (any subtype, silver): 150% of normal value") {
		t.Fatalf("specific item request (wildcard subtype) rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "WEAPON (subtype 3, any): 200% of normal value") {
		t.Fatalf("specific item request (concrete subtype, any material) rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "Cheese x2: 110%-130% of normal value") {
		t.Fatalf("category agreement with a min/max spread rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "Weapons x1: 100% of normal value") {
		t.Fatalf("category agreement with equal min/max must render one percentage, not a range:\n%s", out)
	}
	if !strings.Contains(out, "liaison meeting currently OPEN") {
		t.Fatalf("meeting_active=true must be surfaced:\n%s", out)
	}
	if !strings.Contains(out, "topics: GiveGift, RequestTribute") {
		t.Fatalf("topics_under_discussion rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "diplomat Adil Rithdolen") {
		t.Fatalf("diplomat name rendered wrong:\n%s", out)
	}
	if strings.Contains(out, "associate ,") || strings.Contains(out, ", associate\n") {
		t.Fatalf("empty associate must not appear in the who-line:\n%s", out)
	}
	if !strings.Contains(out, "AcceptAgreement (ImportAgreement) quota 3000/5000 remaining, year 100 tick 42") {
		t.Fatalf("resolved_this_meeting entry rendered wrong:\n%s", out)
	}

	// Honest empty state: no agreement recorded, no meeting open.
	noAgreement := []byte(`{"civs":[
		{"civ":"The Iron Assembly","entity_id":9,"specific_item_requests":[],"category_agreement":[],
		 "meeting_active":false,"topics_under_discussion":[],"diplomat":"","associate":"","resolved_this_meeting":[]}
	]}`)
	outNo := renderTradeAgreements(noAgreement)
	if !strings.Contains(outNo, "no agreement recorded with The Iron Assembly") {
		t.Fatalf("civ with neither agreement half must render the honest empty-state line:\n%s", outNo)
	}
	if strings.Contains(outNo, "liaison meeting currently OPEN") {
		t.Fatalf("meeting_active=false must not print the open-meeting section:\n%s", outNo)
	}

	// Partial case: one agreement half present (category only), no items —
	// must render each half's own empty state, not the combined one.
	partial := []byte(`{"civs":[
		{"civ":"The Copper Horizon","entity_id":3,"specific_item_requests":[],
		 "category_agreement":[{"category":"Wood","count":1,"min_percent":120,"max_percent":120}],
		 "meeting_active":false,"topics_under_discussion":[],"diplomat":"","associate":"","resolved_this_meeting":[]}
	]}`)
	outPartial := renderTradeAgreements(partial)
	if strings.Contains(outPartial, "no agreement recorded with The Copper Horizon") {
		t.Fatalf("a civ with a category agreement must not print the combined no-agreement line:\n%s", outPartial)
	}
	if !strings.Contains(outPartial, "requested imports: none") {
		t.Fatalf("empty specific_item_requests alongside a present category_agreement must render its own 'none' line:\n%s", outPartial)
	}
	if !strings.Contains(outPartial, "Wood x1: 120% of normal value") {
		t.Fatalf("category agreement rendered wrong in partial case:\n%s", outPartial)
	}

	// No civ with an active caravan_state at all.
	empty := []byte(`{"civs":[]}`)
	outEmpty := renderTradeAgreements(empty)
	if !strings.Contains(outEmpty, "No civ currently has an active caravan_state") {
		t.Fatalf("empty civs rendering wrong:\n%s", outEmpty)
	}

	if out := renderTradeAgreements([]byte(`not json`)); !strings.Contains(out, "unparseable trade_agreements response") {
		t.Fatalf("malformed response should report unparseable, got: %q", out)
	}
}

func TestRenderJobTypes(t *testing.T) {
	raw := []byte(`{"orders":[
		{"id":39,"name":"ConstructBed","category":"furniture"},
		{"id":213,"name":"ConstructHatchCover","category":"other"}
	]}`)
	out := renderJobTypes(raw, true)
	if !strings.Contains(out, "2 job types:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "- ConstructBed [furniture]\n") {
		t.Fatalf("furniture entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "- ConstructHatchCover [other]\n") {
		t.Fatalf("hatch cover must round-trip by name — this is the whole point of the generalized path:\n%s", out)
	}
	if strings.Contains(out, "narrow") {
		t.Fatalf("filtered, untruncated response must not suggest narrowing:\n%s", out)
	}
}

func TestRenderJobTypesEmpty(t *testing.T) {
	if out := renderJobTypes([]byte(`{"orders":[]}`), true); out != "No job types matched that filter." {
		t.Fatalf("empty result rendering wrong: %q", out)
	}
}

func TestRenderJobTypesTruncation(t *testing.T) {
	var sb strings.Builder
	sb.WriteString(`{"orders":[`)
	for i := 0; i < maxJobTypesList+10; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, `{"id":%d,"name":"JobType%d","category":"other"}`, i, i)
	}
	sb.WriteString(`]}`)
	out := renderJobTypes([]byte(sb.String()), false)
	if !strings.Contains(out, fmt.Sprintf("%d job types (showing first %d — pass filter to narrow):", maxJobTypesList+10, maxJobTypesList)) {
		t.Fatalf("missing truncation header:\n%s", out)
	}
	if lines := strings.Count(out, "- JobType"); lines != maxJobTypesList {
		t.Fatalf("expected %d listed job types, got %d", maxJobTypesList, lines)
	}
}

func TestRenderJobTypesUnfilteredHint(t *testing.T) {
	raw := []byte(`{"orders":[{"id":39,"name":"ConstructBed","category":"furniture"}]}`)
	out := renderJobTypes(raw, false)
	if !strings.Contains(out, "pass filter next time to narrow this list") {
		t.Fatalf("unfiltered small response should still hint at filter:\n%s", out)
	}
}

func TestRenderJobTypesBadJSON(t *testing.T) {
	if out := renderJobTypes([]byte(`not json`), false); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestRenderBuildingTypes(t *testing.T) {
	out := renderBuildingTypes("carpenter")
	if !strings.Contains(out, "1 building types:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "- carpenter [workshop] footprint=3x3 — ") {
		t.Fatalf("carpenter entry rendered wrong:\n%s", out)
	}
}

// TestRenderBuildingTypesAliasFilter is a regression test for the Fort #6
// failure this wave exists to fix: the fort crafted an ITEM_TOOL_ALTAR,
// searched building_types for "altar", got "No building types matched", and
// concluded altars could not be placed — while offering_place, the real
// (and already working) placement name, sat in the catalog the whole time.
// Each case below is a word a model would plausibly search after crafting
// the item or reading the DF wiki, mapped to the name build actually wants.
func TestRenderBuildingTypesAliasFilter(t *testing.T) {
	for _, tc := range []struct{ filter, want string }{
		{"altar", "offering_place"},
		{"ITEM_TOOL_ALTAR", "offering_place"},
		{"shrine", "offering_place"},
		{"pedestal", "display_furniture"},
		{"display case", "display_furniture"},
		// A display case and a pedestal are two DIFFERENT raw tokens that
		// both place display_furniture, and a model that just crafted one
		// searches the token it typed into queue_job. The underscored form
		// falls out of the token itself (item_tool_display_case contains
		// display_case), so both spellings are pinned here.
		{"ITEM_TOOL_DISPLAY_CASE", "display_furniture"},
		{"display_case", "display_furniture"},
		{"library", "bookcase"},
		{"bookshelf", "bookcase"},
		{"beekeeping", "hive"},
		{"apiary", "hive"},
		{"eggs", "nest_box"},
		{"arrow slit", "fortification"},
		{"jail bars", "bars_vertical"},
		{"hand mill", "quern"},
		{"holding pen", "cage"},
		{"memorial", "slab"},
	} {
		out := renderBuildingTypes(tc.filter)
		if !strings.Contains(out, "- "+tc.want) {
			t.Fatalf("building_types filter=%q must find %q, got:\n%s", tc.filter, tc.want, out)
		}
	}
}

// TestRenderBuildingTypesShowsAliases pins that a row found by an alias also
// PRINTS that alias. Matching silently would let a model find the row once
// and still not learn that "altar" is spelled offering_place at build time.
func TestRenderBuildingTypesShowsAliases(t *testing.T) {
	out := renderBuildingTypes("altar")
	if !strings.Contains(out, "- offering_place (aka altar,") {
		t.Fatalf("alias list must render inline next to the name:\n%s", out)
	}
	// An entry with no alias must not grow an empty "(aka )".
	if strings.Contains(renderBuildingTypes("bed"), "(aka )") {
		t.Fatal("entries without aliases must not render an empty alias group")
	}
}

// TestBuildingTypeAliasesMatchCatalog keeps the alias side table from
// drifting: an alias keyed to a name build no longer accepts would advertise
// a type that fails at the plugin.
func TestBuildingTypeAliasesMatchCatalog(t *testing.T) {
	names := make(map[string]bool, len(buildingTypeCatalog))
	for _, e := range buildingTypeCatalog {
		names[e.Name] = true
	}
	for name := range buildingTypeAliases {
		if !names[name] {
			t.Fatalf("buildingTypeAliases key %q names no building_types catalog entry", name)
		}
	}
}

// TestRenderBuildingTypesFilterIgnoresRequiresProse guards the reason
// aliases are their own field: Requires is prose full of incidental nouns
// ("craft it at a carpenter's"), so filtering against it would make a search
// for the carpenter's WORKSHOP return every item a carpenter can make.
func TestRenderBuildingTypesFilterIgnoresRequiresProse(t *testing.T) {
	out := renderBuildingTypes("carpenter")
	if !strings.Contains(out, "1 building types:") {
		t.Fatalf("filter=carpenter must match only the carpenter workshop:\n%s", out)
	}
}

func TestRenderBuildingTypesCategoryFilter(t *testing.T) {
	out := renderBuildingTypes("furnace")
	if !strings.Contains(out, "- smelter [furnace]") || !strings.Contains(out, "- wood_furnace [furnace]") {
		t.Fatalf("category substring filter must match both furnace entries:\n%s", out)
	}
	if strings.Contains(out, "carpenter") {
		t.Fatalf("furnace filter must not match unrelated workshop entries:\n%s", out)
	}
}

func TestRenderBuildingTypesEmpty(t *testing.T) {
	if out := renderBuildingTypes("nonexistent-type-xyz"); out != "No building types matched that filter." {
		t.Fatalf("empty result rendering wrong: %q", out)
	}
}

func TestRenderBuildingTypesUnfilteredHint(t *testing.T) {
	out := renderBuildingTypes("")
	if !strings.Contains(out, "pass filter next time to narrow this list") {
		t.Fatalf("unfiltered response should hint at filter:\n%s", out)
	}
	if !strings.Contains(out, fmt.Sprintf("%d building types", len(buildingTypeCatalog))) {
		t.Fatalf("unfiltered response must report the full catalog count:\n%s", out)
	}
}

// TestBuildingTypeCatalogMatchesBuildTypes guards the "kept in sync" claim
// in buildingTypeEntry's doc comment: every curated buildTypes key must
// have exactly one building_types catalog entry (bridge is the one
// deliberate exception — see bridgeDirections' doc comment — and is
// checked separately), and every catalog entry must round-trip into either
// buildTypes or the bridge special case, so the catalog never silently
// diverges from what build actually accepts.
func TestBuildingTypeCatalogMatchesBuildTypes(t *testing.T) {
	seen := make(map[string]bool, len(buildingTypeCatalog))
	for _, e := range buildingTypeCatalog {
		if seen[e.Name] {
			t.Fatalf("duplicate building_types catalog entry: %s", e.Name)
		}
		seen[e.Name] = true
		if e.Name == "bridge" {
			continue
		}
		if _, ok := buildTypes[e.Name]; !ok {
			t.Fatalf("catalog entry %q has no matching buildTypes vocabulary entry", e.Name)
		}
	}
	if !seen["bridge"] {
		t.Fatal("catalog is missing the bridge entry")
	}
	for name := range buildTypes {
		if !seen[name] {
			t.Fatalf("buildTypes vocabulary entry %q has no matching building_types catalog entry", name)
		}
	}
}

func TestRenderDwarfDetail(t *testing.T) {
	raw := []byte(`{
		"id":5,
		"position":{"x":23,"y":45,"z":138},
		"first_name":"Urist",
		"top_skills":[
			{"skill":"MINING","level":4,"experience":120},
			{"skill":"FIGHTER","level":1,"experience":30}
		],
		"current_job":"Mine",
		"mood":"None",
		"labors":["MINE","CUTWOOD","HAUL_STONE"]
	}`)

	out := renderDwarfDetail(raw, false, false)
	if !strings.Contains(out, "Urist (id=5) @(23,45,138)") {
		t.Fatalf("missing name/position line:\n%s", out)
	}
	if !strings.Contains(out, "current job: Mine") {
		t.Fatalf("missing current job line:\n%s", out)
	}
	if !strings.Contains(out, "mood: none") {
		t.Fatalf("mood=\"None\" must render as 'none':\n%s", out)
	}
	if !strings.Contains(out, "- MINING Lvl4 (xp 120)") || !strings.Contains(out, "- FIGHTER Lvl1 (xp 30)") {
		t.Fatalf("missing per-skill lines:\n%s", out)
	}
	if !strings.Contains(out, "labors: 3 enabled (pass include_labors=true to list)") {
		t.Fatalf("default view must summarize labor count, not spell them out:\n%s", out)
	}
	if strings.Contains(out, "CUTWOOD") {
		t.Fatalf("default view must not leak labor names:\n%s", out)
	}
	if !strings.Contains(out, "psyche: no soul data") {
		t.Fatalf("missing psyche field must render as no-soul-data:\n%s", out)
	}

	withLabors := renderDwarfDetail(raw, true, false)
	if !strings.Contains(withLabors, "labors (3 enabled): MINE, CUTWOOD, HAUL_STONE") {
		t.Fatalf("include_labors=true must list every labor name:\n%s", withLabors)
	}
}

func TestRenderDwarfDetailMoodFey(t *testing.T) {
	raw := []byte(`{"id":3,"position":{"x":1,"y":1,"z":1},"first_name":"Sarvesh","top_skills":[],"current_job":null,"mood":"Fey","labors":[]}`)
	out := renderDwarfDetail(raw, false, false)
	if !strings.Contains(out, "mood: Fey (active strange mood)") {
		t.Fatalf("mood=\"Fey\" must render the enum name, called out as active:\n%s", out)
	}
}

func TestRenderDwarfDetailNoJobNoSkillsNoLabors(t *testing.T) {
	raw := []byte(`{"id":9,"position":{"x":1,"y":2,"z":3},"first_name":"","top_skills":[],"current_job":null,"mood":"Fey","labors":[]}`)
	out := renderDwarfDetail(raw, true, false)
	if !strings.Contains(out, "dwarf#9 (id=9)") {
		t.Fatalf("empty first_name must fall back to a synthesized name:\n%s", out)
	}
	if !strings.Contains(out, "current job: idle") {
		t.Fatalf("null current_job must render as idle:\n%s", out)
	}
	if !strings.Contains(out, "mood: Fey (active strange mood)") {
		t.Fatalf("mood=\"Fey\" must render the enum name:\n%s", out)
	}
	if !strings.Contains(out, "top skills: none") {
		t.Fatalf("empty top_skills must say none:\n%s", out)
	}
	if !strings.Contains(out, "labors: none enabled") {
		t.Fatalf("empty labors with include_labors=true must say none enabled:\n%s", out)
	}
}

func TestRenderDwarfDetailBadJSON(t *testing.T) {
	if out := renderDwarfDetail([]byte(`not json`), false, false); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestRenderDwarfDetailDead(t *testing.T) {
	raw := []byte(`{
		"id":5,
		"position":{"x":23,"y":45,"z":138},
		"first_name":"Dobar",
		"dead":true,
		"top_skills":[],
		"current_job":null,
		"mood":"None",
		"labors":[]
	}`)
	out := renderDwarfDetail(raw, false, false)
	if !strings.Contains(out, "status: DEAD") {
		t.Fatalf("dead:true must render a DEAD status line:\n%s", out)
	}

	alive := renderDwarfDetail([]byte(`{
		"id":6,"position":{"x":1,"y":1,"z":1},"first_name":"Urist","dead":false,
		"top_skills":[],"current_job":null,"mood":"None","labors":[]
	}`), false, false)
	if strings.Contains(alive, "status: DEAD") {
		t.Fatalf("dead:false must not render a DEAD status line:\n%s", alive)
	}
	// dead field entirely absent (older plugin) must decode to the zero
	// value (false), matching the additive/backward-compatible contract.
	noField := renderDwarfDetail([]byte(`{
		"id":7,"position":{"x":1,"y":1,"z":1},"first_name":"Urist",
		"top_skills":[],"current_job":null,"mood":"None","labors":[]
	}`), false, false)
	if strings.Contains(noField, "status: DEAD") {
		t.Fatalf("missing dead field must default to alive:\n%s", noField)
	}
}

// TestRenderDwarfDetailPsyche covers the new psyche section: the compact
// default summary, the include_psyche=true full breakdown, and the signed
// (not relabeled) divider passthrough on an emotion entry.
func TestRenderDwarfDetailPsyche(t *testing.T) {
	raw := []byte(`{
		"id":11,"position":{"x":1,"y":1,"z":1},"first_name":"Kadol",
		"top_skills":[],"current_job":null,"mood":"None","labors":[],
		"psyche":{
			"stress":15000,
			"stress_category":2,
			"needs":[{"need_type":"Socialize","focus_level":-120,"need_level":50}],
			"emotions_total":3,
			"emotions_cap":20,
			"emotions":[{"emotion":"AMUSEMENT","strength":300,"thought":"Spar","thought_caption":"after a sparring session","subthought":0,"divider":-4}],
			"top_facets":[{"facet":"GREED","value":82}],
			"values":[{"value_type":"LAW","strength":60}],
			"dreams":[{"goal_type":"CRAFT_A_MASTERWORK","accomplished":false}]
		}
	}`)

	summary := renderDwarfDetail(raw, false, false)
	if !strings.Contains(summary, "psyche: stress=15000 (category 2/6) | needs=1 | emotions=1 of 3 total | facets=1 | values=1 | dreams=1") {
		t.Fatalf("default view must summarize psyche counts, not spell them out:\n%s", summary)
	}
	if strings.Contains(summary, "GREED") {
		t.Fatalf("default view must not leak psyche detail:\n%s", summary)
	}

	full := renderDwarfDetail(raw, false, true)
	if !strings.Contains(full, "- Socialize: focus_level=-120 need_level=50") {
		t.Fatalf("include_psyche=true must list needs:\n%s", full)
	}
	if !strings.Contains(full, "AMUSEMENT (strength 300) — after a sparring session [thought=Spar subthought=0 divider=-4]") {
		t.Fatalf("include_psyche=true must list emotions with the raw signed divider:\n%s", full)
	}
	if !strings.Contains(full, "- GREED: 82") {
		t.Fatalf("include_psyche=true must list top facets:\n%s", full)
	}
	if !strings.Contains(full, "- LAW: 60") {
		t.Fatalf("include_psyche=true must list values:\n%s", full)
	}
	if !strings.Contains(full, "- CRAFT_A_MASTERWORK: not yet accomplished") {
		t.Fatalf("include_psyche=true must list dreams with accomplished status:\n%s", full)
	}
}

// TestRenderDwarfPortraitRich covers a densely-populated dwarf_portrait
// response: every section has something to say, and every tier-banding
// helper (facet/value 7-tier, need fulfillment/strength, stress category)
// gets exercised on an in-range value.
func TestRenderDwarfPortraitRich(t *testing.T) {
	raw := []byte(`{
		"id":11,"name":"Edóm Ustuthätan","caste_description":"A short, sturdy creature fond of drink and industry.",
		"size_band":"average","stress_category":3,
		"facets":[{"facet":"GREED","value":96},{"facet":"DISCORD","value":84}],
		"values":[{"value_type":"LAW","strength":45}],
		"needs_starved":[
			{"need_type":"Socialize","focus_level":-1200,"need_level":2},
			{"need_type":"Excitement","focus_level":-15000,"need_level":5}
		],
		"need_best_fed":{"need_type":"DrinkAlcohol","focus_level":350,"need_level":10},
		"emotion":{"emotion":"AMUSEMENT","strength":300,"divider":-4,"thought":"Spar","thought_caption":"after a sparring session","subthought":0,"subthought_resolved":false,"subthought_text":""},
		"preferences":[
			{"type":"LikeMaterial","label":"steel"},
			{"type":"LikeFood","label":"roasted kea"},
			{"type":"LikeColor","label":"mauve"}
		],
		"deity":{"name":"Vand the Platinum Coin","link_strength":45},
		"spouse":{"name":"Olon"},
		"lover":null,
		"children":[{"name":"Zefonchild"}],
		"best_friend":{"name":"Zefon","love":82,"meet_count":31},
		"worst_grudge":{"name":"Deduk","love":-60,"meet_count":5},
		"memberships":[{"entity":"The Reformed Hammers","entity_type":"Guild","status":"current"}],
		"known_poetic_forms":4,"known_musical_forms":2,"known_dance_forms":0,"known_written_contents":2,
		"masterpieces":1,"kills":0
	}`)

	out := renderDwarfPortrait(raw)

	checks := []string{
		"Edóm Ustuthätan (id=11)",
		"average size",
		"A short, sturdy creature fond of drink and industry.",
		"Mood: Content",
		"Personality: Highest GREED (96), Very High DISCORD (84)",
		"Beliefs: Highest LAW (45)",
		"Socialize (Unfocused, focus -1200) -- driven by high GREGARIOUSNESS",
		"Excitement (Distracted, focus -15000) -- driven by high EXCITEMENT_SEEKING",
		"Best-fed need: DrinkAlcohol (Unfettered, focus 350, Intense strength)",
		"Strongest emotion: AMUSEMENT (strength 300) -- after a sparring session",
		"steel (drives strange moods)",
		"roasted kea",
		"mauve",
		"Deity: worships Vand the Platinum Coin (devotion 45, scale unverified",
		"Family: spouse Olon; 1 child(ren) (Zefonchild)",
		"best friend Zefon (love 82, met 31×)",
		"grudge against Deduk (love -60, met 5×)",
		"Affiliations: The Reformed Hammers (Guild, current)",
		"Knows 4 poetic, 2 musical, 0 dance forms; carries 2 written work(s)",
		"Masterpieces: 1 (creation-event linkage unverified -- may undercount); kills recorded: 0",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered portrait:\n%s", want, out)
		}
	}
	// LikeMaterial's mood-insurance note must not leak onto sibling
	// preferences that have no such proven mechanical link.
	if strings.Contains(out, "roasted kea (drives strange moods)") {
		t.Fatalf("only LikeMaterial gets the drives-strange-moods note:\n%s", out)
	}
	// Lover was null in the wire response -- must not fabricate a line.
	if strings.Contains(out, "lover") {
		t.Fatalf("null lover must not render any lover text:\n%s", out)
	}
}

// TestRenderDwarfPortraitSparse covers the opposite extreme: a citizen
// with nothing salient anywhere. Every section must still print an
// explicit, truthful "none"/"nothing" line rather than being silently
// dropped -- a sparse portrait should read as complete, not truncated.
func TestRenderDwarfPortraitSparse(t *testing.T) {
	raw := []byte(`{
		"id":9,"name":"Sarvesh","caste_description":"","size_band":"average","stress_category":3,
		"facets":[],"values":[],"needs_starved":[],"need_best_fed":null,
		"emotion":null,"preferences":[],
		"deity":null,"spouse":null,"lover":null,"children":[],
		"best_friend":null,"worst_grudge":null,"memberships":[],
		"known_poetic_forms":0,"known_musical_forms":0,"known_dance_forms":0,"known_written_contents":0,
		"masterpieces":0,"kills":0
	}`)

	out := renderDwarfPortrait(raw)

	checks := []string{
		"Sarvesh (id=9)",
		"Personality: nothing stands out (all facets near neutral)",
		"Beliefs: none stand out",
		"Starved needs: none",
		"Best-fed need: none (no needs recorded)",
		"Strongest emotion: none recorded",
		"Preferences: none visible",
		"Deity: worships nothing in particular",
		"Family: none known",
		"Social: no standout friendships or grudges among fellow citizens",
		"Affiliations: none",
		"Knows 0 poetic, 0 musical, 0 dance forms; carries 0 written work(s)",
		"Masterpieces: 0 (creation-event linkage unverified -- may undercount); kills recorded: 0",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in sparse portrait:\n%s", want, out)
		}
	}
}

func TestRenderDwarfPortraitBadJSON(t *testing.T) {
	out := renderDwarfPortrait([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderCombatSummaryFortWinning covers momentum favoring the fort
// side: the "other" side takes the wounds/casualty, the fort side lands
// the hits. Exercises sides-by-name, severity tally ordering
// (severed_part before bruise regardless of map iteration order),
// hits_landed attribution, casualties, and telling lines.
func TestRenderCombatSummaryFortWinning(t *testing.T) {
	raw := []byte(`{
		"engagements":[{
			"engagement_id":501,"first_report_id":501,"last_report_id":519,
			"start_year":101,"start_time":4000,"end_year":101,"end_time":4050,
			"sides":[
				{"label":"fort","units":[{"id":1,"name":"Edóm"}],
				 "new_wounds":{},"hits_landed":[{"attacker_id":1,"attacker_name":"Edóm","count":3}],
				 "knocked_out":0,"bleeding":0,"casualties":[]},
				{"label":"other","units":[{"id":77,"name":"a giant cave spider"}],
				 "new_wounds":{"bruise":2,"severed_part":1},
				 "hits_landed":[],"knocked_out":1,"bleeding":1,
				 "casualties":[{"id":77,"name":"a giant cave spider","death_cause":"STRUCK_DOWN","killer":"Edóm"}]}
			],
			"telling_lines":["Edóm severs the giant cave spider's leg with a silver war hammer!"]
		}],
		"total_tracked":1,
		"momentum_note":"wound counts reflect activity since first observed fighting"
	}`)

	out := renderCombatSummary(raw)

	checks := []string{
		"1 engagement(s) tracked",
		"[engagement 501] reports #501-#519",
		"fort side: Edóm",
		"other side: a giant cave spider",
		"hits landed: Edóm x3",
		"severed_part x1, bruise x2", // severity order, not map iteration order
		"knocked out: 1",
		"bleeding: 1",
		"CASUALTIES: a giant cave spider (STRUCK_DOWN, by Edóm)",
		`"Edóm severs the giant cave spider's leg with a silver war hammer!"`,
		"wound counts reflect activity since first observed fighting",
		"combat_report mode=log",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered combat summary:\n%s", want, out)
		}
	}
}

// TestRenderCombatSummaryFortLosing covers the opposite momentum direction:
// the fort side takes wounds/a casualty from an unresolved attacker, so the
// killer must render as an honest "unknown killer" rather than a blank.
func TestRenderCombatSummaryFortLosing(t *testing.T) {
	raw := []byte(`{
		"engagements":[{
			"engagement_id":900,"first_report_id":900,"last_report_id":905,
			"start_year":101,"start_time":100,"end_year":101,"end_time":110,
			"sides":[
				{"label":"fort","units":[{"id":2,"name":"Olon"}],
				 "new_wounds":{"artery":1,"fracture":1},
				 "hits_landed":[],"knocked_out":1,"bleeding":1,
				 "casualties":[{"id":2,"name":"Olon","death_cause":"BLEED","killer":""}]},
				{"label":"other","units":[{"id":88,"name":"a troll"}],
				 "new_wounds":{},"hits_landed":[{"attacker_id":88,"attacker_name":"a troll","count":5}],
				 "knocked_out":0,"bleeding":0,"casualties":[]}
			],
			"telling_lines":[]
		}],
		"total_tracked":1
	}`)

	out := renderCombatSummary(raw)

	checks := []string{
		"fort side: Olon",
		"other side: a troll",
		"artery x1, fracture x1",
		"hits landed: a troll x5",
		"CASUALTIES: Olon (BLEED, by unknown killer)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered combat summary:\n%s", want, out)
		}
	}
	if strings.Contains(out, "telling lines:") {
		t.Fatalf("empty telling_lines must not print a header:\n%s", out)
	}
}

// TestRenderCombatSummaryEmpty covers the truthful empty state: no
// engagements at all (no fight has happened since the cursor).
func TestRenderCombatSummaryEmpty(t *testing.T) {
	raw := []byte(`{"engagements":[],"total_tracked":0,"note":"no combat reports since cursor"}`)
	out := renderCombatSummary(raw)
	if !strings.Contains(out, "no combat reports since cursor") {
		t.Fatalf("empty state must surface the plugin's own note, got: %q", out)
	}
}

func TestRenderCombatSummaryBadJSON(t *testing.T) {
	out := renderCombatSummary([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderCombatLog covers the raw windowed transcript, including the
// clamp note when the plugin reports one.
func TestRenderCombatLog(t *testing.T) {
	raw := []byte(`{
		"lines":[
			{"id":501,"year":101,"time":4000,"text":"Edóm has struck the giant cave spider!"},
			{"id":502,"year":101,"time":4001,"text":"The giant cave spider has been struck down."}
		],
		"clamp_note":"12 earlier line(s) omitted"
	}`)
	out := renderCombatLog(raw)

	checks := []string{
		"2 combat report line(s)",
		"[y101 t4000 #501] Edóm has struck the giant cave spider!",
		"[y101 t4001 #502] The giant cave spider has been struck down.",
		"12 earlier line(s) omitted",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered combat log:\n%s", want, out)
		}
	}
}

func TestRenderCombatLogEmpty(t *testing.T) {
	raw := []byte(`{"lines":[],"note":"no matching combat reports"}`)
	out := renderCombatLog(raw)
	if !strings.Contains(out, "no matching combat reports") {
		t.Fatalf("empty log state must surface the plugin's own note, got: %q", out)
	}
}

func TestRenderCombatLogBadJSON(t *testing.T) {
	out := renderCombatLog([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderStoryPulseMixedSalience covers a realistic mixed pulse: one
// facet/value-change-flagged entry, one MadeFriend entry with a resolved
// other_party, and one plain-strength entry, plus a rewindow note and a
// clamp note. The plugin's own order (most salient first) is trusted, not
// re-sorted here -- the fixture is deliberately already in that order.
func TestRenderStoryPulseMixedSalience(t *testing.T) {
	raw := []byte(`{
		"entries":[
			{"unit_id":1,"unit_name":"Edóm","emotion":"AGONY","strength":220,"divider":1,
			 "thought":"Death","thought_caption":"at the unexpected death of","subthought":9,
			 "subthought_resolved":true,"subthought_text":"Zefonchild","salience":"facet_or_value_change",
			 "other_party":null,"year":101,"year_tick":4000},
			{"unit_id":2,"unit_name":"Olon","emotion":"AFFECTION","strength":80,"divider":-2,
			 "thought":"MadeFriend","thought_caption":"after making a friend","subthought":9,
			 "subthought_resolved":false,"subthought_text":"","salience":"made_friend_or_grudge",
			 "other_party":{"name":"Zefon"},"year":101,"year_tick":4010},
			{"unit_id":3,"unit_name":"Deduk","emotion":"AGITATION","strength":60,"divider":4,
			 "thought":"None","thought_caption":"","subthought":0,
			 "subthought_resolved":false,"subthought_text":"","salience":"strength",
			 "other_party":null,"year":101,"year_tick":4020}
		],
		"total_candidates":40,"shown":3,
		"clamp_note":"37 below threshold",
		"rewindow_note":"cursor was unset (first call, or a reconnect) -- showing only the last 12000 ticks"
	}`)

	out := renderStoryPulse(raw)

	checks := []string{
		"cursor was unset (first call, or a reconnect)",
		"3 notable feeling(s) since the last pulse",
		"Edóm: AGONY (strength 220) -- at the unexpected death of (Zefonchild) [personality-altering]",
		"Olon: AFFECTION (strength 80) with Zefon -- after making a friend [new bond]",
		"Deduk: AGITATION (strength 60) -- None [strong feeling]",
		"37 below threshold",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered story pulse:\n%s", want, out)
		}
	}
	// The rewindow note must appear before the count line (context first).
	if strings.Index(out, "cursor was unset") > strings.Index(out, "3 notable feeling(s)") {
		t.Fatalf("rewindow note must render before the count line:\n%s", out)
	}
}

// TestRenderStoryPulseCapVsThresholdNote covers the fix for the clamp_note
// conflation defect: a busy window where some omissions are genuinely
// below-threshold (tier-0) noise and others are salient (tier>=1) entries
// cut only by the display cap must render as two distinct, separately
// worded clauses -- never merged into one count that mislabels real news as
// noise.
func TestRenderStoryPulseCapVsThresholdNote(t *testing.T) {
	raw := []byte(`{"entries":[],"total_candidates":40,"shown":15,
		"clamp_note":"20 below threshold; 5 more salient not shown (raise max)"}`)
	out := renderStoryPulse(raw)
	if !strings.Contains(out, "20 below threshold") {
		t.Fatalf("missing below-threshold clause in rendered story pulse:\n%s", out)
	}
	if !strings.Contains(out, "5 more salient not shown (raise max)") {
		t.Fatalf("missing capped-by-limit clause in rendered story pulse:\n%s", out)
	}
}

// TestRenderStoryPulseEmpty covers the truthful empty state: nothing felt
// since the cursor.
func TestRenderStoryPulseEmpty(t *testing.T) {
	raw := []byte(`{"entries":[],"total_candidates":0,"shown":0}`)
	out := renderStoryPulse(raw)
	if !strings.Contains(out, "no notable emotions felt since the last pulse") {
		t.Fatalf("empty pulse must say so plainly, got: %q", out)
	}
}

func TestRenderStoryPulseBadJSON(t *testing.T) {
	out := renderStoryPulse([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestSocialLoveBandBounds exercises every boundary of DF's own documented
// core.love banding (df.history_figure.xml:486), verbatim.
func TestSocialLoveBandBounds(t *testing.T) {
	cases := []struct {
		love int
		want string
	}{
		{-100, "Pure Hate"},
		{-99, "Hated"},
		{-75, "Hated"},
		{-74, "Disliked"},
		{-50, "Disliked"},
		{-49, "Acquaintance"},
		{49, "Acquaintance"},
		{50, "Friend"},
		{74, "Friend"},
		{75, "Close Friend"},
		{99, "Close Friend"},
		{100, "Kindred Spirit"},
	}
	for _, c := range cases {
		if got := socialLoveBand(c.love); got != c.want {
			t.Errorf("socialLoveBand(%d) = %q, want %q", c.love, got, c.want)
		}
	}
}

// TestRenderSocialGraphKinds covers one edge of each of the four kinds,
// checking each kind's own detail formatting (love band + counts for
// friend/grudge, devotion for worship, relation label for family).
func TestRenderSocialGraphKinds(t *testing.T) {
	raw := []byte(`{
		"edges":[
			{"a_id":1,"a_name":"Edóm","b_id":2,"b_name":"Olon","kind":"family","relation":"spouse","love":null,"meet_count":null,"link_strength":null},
			{"a_id":1,"a_name":"Edóm","b_id":3,"b_name":"Zefon","kind":"friend","relation":"war_buddy","love":82,"meet_count":31,"link_strength":null},
			{"a_id":1,"a_name":"Edóm","b_id":4,"b_name":"Deduk","kind":"grudge","relation":"","love":-60,"meet_count":5,"link_strength":null},
			{"a_id":1,"a_name":"Edóm","b_id":5,"b_name":"Vand the Platinum Coin","kind":"worship","relation":"deity","love":null,"meet_count":null,"link_strength":45}
		],
		"total_edges":4
	}`)

	out := renderSocialGraph(raw)

	checks := []string{
		"4 social edge(s):",
		"Edóm -- Olon: Family (Spouse)",
		"Edóm -- Zefon: Friend (Close Friend; love 82, met 31×)",
		"Edóm -- Deduk: Grudge (Disliked; love -60, met 5×)",
		"Edóm -- Vand the Platinum Coin: Worship (devotion 45, scale unverified)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered social graph:\n%s", want, out)
		}
	}
}

// TestRenderSocialGraphSamePairDifferentKinds documents the "dedup A<B"
// contract from the render side: the plugin's dedup key is scoped per-kind
// (narrative.cpp's addSocialEdge, SocialEdgeKey), so the SAME pair can
// legitimately carry both a family edge and a friend edge (e.g. a married
// couple who also rate each other as a Close Friend) -- these must render
// as two distinct lines, never collapsed. The actual A<B numeric dedup
// itself lives in the plugin and is exercised by compile + live play (this
// repo has no C++ unit-test harness); this test locks the Go-side
// rendering contract for the case that behavior produces.
func TestRenderSocialGraphSamePairDifferentKinds(t *testing.T) {
	raw := []byte(`{
		"edges":[
			{"a_id":1,"a_name":"Edóm","b_id":2,"b_name":"Olon","kind":"family","relation":"spouse","love":null,"meet_count":null,"link_strength":null},
			{"a_id":1,"a_name":"Edóm","b_id":2,"b_name":"Olon","kind":"friend","relation":"","love":90,"meet_count":50,"link_strength":null}
		],
		"total_edges":2
	}`)

	out := renderSocialGraph(raw)

	checks := []string{
		"2 social edge(s):",
		"Edóm -- Olon: Family (Spouse)",
		"Edóm -- Olon: Friend (Close Friend; love 90, met 50×)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered social graph:\n%s", want, out)
		}
	}
	if strings.Count(out, "Edóm -- Olon:") != 2 {
		t.Fatalf("same pair under different kinds must render as two distinct lines, got:\n%s", out)
	}
}

// TestRenderSocialGraphClampNote covers the truthful cap note.
func TestRenderSocialGraphClampNote(t *testing.T) {
	raw := []byte(`{
		"edges":[{"a_id":1,"a_name":"Edóm","b_id":2,"b_name":"Olon","kind":"family","relation":"spouse","love":null,"meet_count":null,"link_strength":null}],
		"total_edges":41,
		"clamp_note":"1 more edge(s) not shown (narrow with kind or unit)"
	}`)
	out := renderSocialGraph(raw)
	if !strings.Contains(out, "1 more edge(s) not shown (narrow with kind or unit)") {
		t.Fatalf("clamp note must be surfaced, got: %q", out)
	}
}

// TestRenderSocialGraphEmpty covers the truthful empty state.
func TestRenderSocialGraphEmpty(t *testing.T) {
	raw := []byte(`{"edges":[],"total_edges":0}`)
	out := renderSocialGraph(raw)
	if !strings.Contains(out, "no social edges found") {
		t.Fatalf("empty graph must say so plainly, got: %q", out)
	}
}

func TestRenderSocialGraphBadJSON(t *testing.T) {
	out := renderSocialGraph([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestRenderFortArtProvenance covers the two labeled provenance paths
// research 2.5 specifies: a composed_here work (with its joined year) and a
// brought_here work (a current citizen's pre-fort composition, or a fort
// composition the event join missed) -- the render must distinguish them
// and never fabricate a year for the brought-here case.
func TestRenderFortArtProvenance(t *testing.T) {
	raw := []byte(`{
		"works":[
			{"kind":"poetic_form","id":1,"title":"The Cavern of Mirth","creator":"Edóm",
			 "provenance":"composed_here","composed_year":101,
			 "mood":"Solemn","subject":"AlcoholicBeverages","subject_detail":"","action":"Praise","worship_target":""},
			{"kind":"written_content","id":2,"title":"A Wanderer's Account","creator":"Olon",
			 "provenance":"brought_here","composed_year":null,
			 "written_type":"Autobiography","styles":["Cheerful","Witty"]}
		],
		"census":{"known_in_fort":2,"composed_in_fort":1,"brought_here":1}
	}`)

	out := renderFortArt(raw)

	checks := []string{
		"2 work(s) of fort art:",
		`"The Cavern of Mirth" -- a poem by Edóm (solemn, alcoholic beverages, praise) [composed here, y101]`,
		`"A Wanderer's Account" -- autobiography by Olon (cheerful, witty) [brought here]`,
		"census: 2 known in fort (1 composed here, 1 brought here)",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered fort art:\n%s", want, out)
		}
	}
	if strings.Contains(out, "y<nil>") || strings.Contains(out, "brought here, y") {
		t.Fatalf("brought-here work must never carry a fabricated year, got:\n%s", out)
	}
}

// TestRenderFortArtDanceFields covers dance_form's "narrative gold" fields
// (research 2.5): the event acted out, the character whose story it tells,
// and the creature whose movements are imitated.
func TestRenderFortArtDanceFields(t *testing.T) {
	raw := []byte(`{
		"works":[
			{"kind":"dance_form","id":9,"title":"The Carnotaur's Stomp","creator":"Zefon",
			 "provenance":"composed_here","composed_year":99,
			 "context":"Celebration","character":"Deduk","creature_imitated":"carnotaurus","event":555}
		],
		"census":{"known_in_fort":1,"composed_in_fort":1,"brought_here":0}
	}`)

	out := renderFortArt(raw)

	checks := []string{
		`"The Carnotaur's Stomp" -- a dance by Zefon (celebration, acts out Deduk's story, imitates a carnotaurus, re-enacts historical event #555) [composed here, y99]`,
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered fort art:\n%s", want, out)
		}
	}
}

// TestRenderFortArtEmpty covers the truthful empty state: nothing composed
// here or brought by a current citizen yet, still surfacing the (zeroed)
// census rather than an error.
func TestRenderFortArtEmpty(t *testing.T) {
	raw := []byte(`{"works":[],"census":{"known_in_fort":0,"composed_in_fort":0,"brought_here":0}}`)
	out := renderFortArt(raw)
	if !strings.Contains(out, "no fort art yet") {
		t.Fatalf("empty fort art must say so plainly, got: %q", out)
	}
	if !strings.Contains(out, "census: 0 known in fort (0 composed here, 0 brought here)") {
		t.Fatalf("empty fort art must still surface the zeroed census, got: %q", out)
	}
}

// TestRenderFortArtScanAndClampNotes covers the first-scan timing note
// (live probe #10) and the truthful cap note, both of which must survive
// into the rendered text unmodified.
func TestRenderFortArtScanAndClampNotes(t *testing.T) {
	raw := []byte(`{
		"works":[{"kind":"poetic_form","id":1,"title":"A Song","creator":"Edóm","provenance":"composed_here","composed_year":101}],
		"census":{"known_in_fort":11,"composed_in_fort":11,"brought_here":0},
		"clamp_note":"1 more work(s) not shown",
		"scan_note":"first fort_art call scanned the world's full history-event log (83ms) -- later calls are incremental"
	}`)
	out := renderFortArt(raw)
	checks := []string{
		"first fort_art call scanned the world's full history-event log (83ms)",
		"1 more work(s) not shown",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Fatalf("missing expected substring %q in rendered fort art:\n%s", want, out)
		}
	}
}

func TestRenderFortArtBadJSON(t *testing.T) {
	out := renderFortArt([]byte(`not json`))
	if !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

// TestPersonalityTierLabelBounds exercises both the facet (0-100) and
// value (-50..50) tier tables at their boundaries, plus the honest
// fallback for a value outside either table (a real DF value strength
// CAN exceed 50 in principle -- research 2.1a's tier table only documents
// -50..50 as the display band boundaries, not a hard storage limit).
func TestPersonalityTierLabelBounds(t *testing.T) {
	if got := personalityTierLabel(0, false); got != "Lowest" {
		t.Fatalf("facet 0 = Lowest, got %q", got)
	}
	if got := personalityTierLabel(50, false); got != "Neutral" {
		t.Fatalf("facet 50 = Neutral, got %q", got)
	}
	if got := personalityTierLabel(100, false); got != "Highest" {
		t.Fatalf("facet 100 = Highest, got %q", got)
	}
	if got := personalityTierLabel(-50, true); got != "Lowest" {
		t.Fatalf("value -50 = Lowest, got %q", got)
	}
	if got := personalityTierLabel(0, true); got != "Neutral" {
		t.Fatalf("value 0 = Neutral, got %q", got)
	}
	if got := personalityTierLabel(60, true); !strings.Contains(got, "tier unknown") {
		t.Fatalf("out-of-table value must fall back honestly, got %q", got)
	}
}

func TestStressCategoryLabelOutOfRange(t *testing.T) {
	if got := stressCategoryLabel(3); got != "Content" {
		t.Fatalf("category 3 = Content, got %q", got)
	}
	if got := stressCategoryLabel(9); got != "category 9" {
		t.Fatalf("out-of-range category must fall back to the raw number, got %q", got)
	}
}

func TestDwarfSummaryLine(t *testing.T) {
	d, err := parseDwarfDetail([]byte(`{
		"id":5,"position":{"x":1,"y":1,"z":1},"first_name":"Urist",
		"top_skills":[{"skill":"MINING","level":4,"experience":120}],
		"current_job":"Mine","mood":"None","labors":["MINE","CUTWOOD"]
	}`))
	if err != nil {
		t.Fatalf("parseDwarfDetail: %v", err)
	}
	line := dwarfSummaryLine(d)
	if !strings.Contains(line, "Urist (id=5)") || !strings.Contains(line, "Mine") ||
		!strings.Contains(line, "MINING Lvl4") || !strings.Contains(line, "labors=2") {
		t.Fatalf("census line missing expected fields: %q", line)
	}
	if strings.Contains(line, "[DEAD]") {
		t.Fatalf("alive dwarf must not carry a [DEAD] tag: %q", line)
	}
}

func TestDwarfSummaryLineDead(t *testing.T) {
	d, err := parseDwarfDetail([]byte(`{
		"id":9,"position":{"x":1,"y":1,"z":1},"first_name":"Dobar","dead":true,
		"top_skills":[],"current_job":null,"mood":"None","labors":[]
	}`))
	if err != nil {
		t.Fatalf("parseDwarfDetail: %v", err)
	}
	line := dwarfSummaryLine(d)
	if !strings.Contains(line, "Dobar (id=9) [DEAD]") {
		t.Fatalf("dead dwarf census line must carry a [DEAD] tag: %q", line)
	}
}

func TestRenderDwarfListCap(t *testing.T) {
	few := []protocol.EntityInfo{{ID: 7, X: 1, Y: 2, Z: 3}}
	out := renderDwarfList(few)
	if !strings.Contains(out, "1 dwarves:") || !strings.Contains(out, "id=7 @(1,2,3)") {
		t.Fatalf("small roster wrong:\n%s", out)
	}
	if strings.Contains(out, "more") {
		t.Fatalf("small roster must not claim truncation:\n%s", out)
	}

	many := make([]protocol.EntityInfo, maxDwarfList+10)
	for i := range many {
		many[i] = protocol.EntityInfo{ID: uint32(i)}
	}
	out = renderDwarfList(many)
	if !strings.Contains(out, "... and 10 more (use dwarf_detail by id)") {
		t.Fatalf("large roster must cap at %d:\n%s", maxDwarfList, out)
	}
	if lines := strings.Count(out, "- id="); lines != maxDwarfList {
		t.Fatalf("expected %d listed dwarves, got %d", maxDwarfList, lines)
	}
}

func TestRenderDwarfListDeadTag(t *testing.T) {
	mixed := []protocol.EntityInfo{
		{ID: 1, X: 1, Y: 1, Z: 1, Dead: false},
		{ID: 2, X: 2, Y: 2, Z: 2, Dead: true},
	}
	out := renderDwarfList(mixed)
	if !strings.Contains(out, "id=1 @(1,1,1)\n") {
		t.Fatalf("living dwarf must not carry a [DEAD] tag:\n%s", out)
	}
	// unit->pos is never re-synced once dead (Units::getPosition has no
	// dead-unit special case) -- "last-known" is the truthful label, not a
	// current position.
	if !strings.Contains(out, "id=2 last-known @(2,2,2) [DEAD]\n") {
		t.Fatalf("dead dwarf must carry a last-known [DEAD] tag:\n%s", out)
	}
}

func TestRenderIdleRollup(t *testing.T) {
	job := func(s string) *string { return &s }
	t.Run("some idle lists names", func(t *testing.T) {
		details := []dwarfDetailResp{
			{ID: 1, FirstName: "edzul"},                          // nil job -> idle
			{ID: 2, FirstName: "zaneg", CurrentJob: job("")},     // empty job -> idle
			{ID: 3, FirstName: "ilral", CurrentJob: job("Mine")}, // busy
			{ID: 4, FirstName: "ghost", Dead: true},              // dead: excluded from count AND denominator
		}
		got := renderIdleRollup(details)
		want := "idle: 2 of 3 (edzul, zaneg)\n"
		if got != want {
			t.Fatalf("renderIdleRollup = %q, want %q", got, want)
		}
	})
	t.Run("none idle omits name list", func(t *testing.T) {
		details := []dwarfDetailResp{
			{ID: 1, FirstName: "edzul", CurrentJob: job("Mine")},
		}
		got := renderIdleRollup(details)
		want := "idle: 0 of 1\n"
		if got != want {
			t.Fatalf("renderIdleRollup = %q, want %q", got, want)
		}
	})
	t.Run("overflow falls back to count only", func(t *testing.T) {
		var details []dwarfDetailResp
		for i := range maxIdleNamesListed + 1 {
			details = append(details, dwarfDetailResp{ID: i + 1, FirstName: fmt.Sprintf("d%d", i+1)})
		}
		got := renderIdleRollup(details)
		want := fmt.Sprintf("idle: %d of %d\n", maxIdleNamesListed+1, maxIdleNamesListed+1)
		if got != want {
			t.Fatalf("renderIdleRollup = %q, want %q", got, want)
		}
	})
}
