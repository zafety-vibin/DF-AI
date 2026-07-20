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

func TestRenderStocksAggregated(t *testing.T) {
	raw := []byte(`{"items":[
		{"item_type":"BOULDER","material":"shale","count":20,"economic":false},
		{"item_type":"BOULDER","material":"chalk","count":11,"economic":false},
		{"item_type":"BOULDER","material":"bauxite","count":9,"economic":true},
		{"item_type":"WOOD","material":"oak","count":3}
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
		{"item_type":"BOULDER","material":"shale","count":20,"economic":false},
		{"item_type":"BOULDER","material":"bauxite","count":9,"economic":true}
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
		{"item_type":"BED","material":"oak","count":1,"in_use":3},
		{"item_type":"DOOR","material":"bronze","count":0,"in_use":2}
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

	noInUse := []byte(`{"items":[{"item_type":"TABLE","material":"granite","count":5}]}`)
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

	// An older-plugin payload (or any fixture) that omits "units"/
	// "in_use_units" entirely must decode as "no unit data" (see the ">"
	// comparison in renderStocks), never a fabricated "0 units" note.
	noUnits := []byte(`{"items":[{"item_type":"DRINK","material":"ale","count":4}]}`)
	if out := renderStocks(noUnits, true, 0); strings.Contains(out, "units") {
		t.Fatalf("a payload with no units data must not print a units note:\n%s", out)
	}
	if out := renderStocks(noUnits, false, 0); strings.Contains(out, "units") {
		t.Fatalf("a payload with no units data must not print a units note in aggregate view either:\n%s", out)
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
		{"item_type":"BOULDER","material":"shale","count":20},
		{"item_type":"BOULDER","material":"chalk","count":2}
	]}`)
	out := renderStocks(raw, false, 5)
	if strings.Contains(out, "chalk") {
		t.Fatalf("entry below min_count must be filtered out:\n%s", out)
	}
	if !strings.Contains(out, "shale") {
		t.Fatalf("entry at/above min_count must survive:\n%s", out)
	}

	if out := renderStocks([]byte(`{"items":[{"item_type":"BOULDER","material":"chalk","count":2}]}`), false, 5); out != "No stock items (or all below min_count)." {
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
		{"item_type":"TABLE","material":"granite","count":0,"in_use":3},
		{"item_type":"TABLE","material":"oak","count":1}
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
		{"item_type":"BED","material":"oak","count":4},
		{"item_type":"TABLE","material":"granite","count":2}
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
		{"item_type":"WEAPON","material":"iron","count":3}
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

	noSubtypes := []byte(`{"items":[{"item_type":"BOULDER","material":"shale","count":5}]}`)
	if out := renderStocks(noSubtypes, true, 0); strings.Contains(out, "subtypes") {
		t.Fatalf("a type with no subtype entry must not print a breakdown line:\n%s", out)
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
	raw := []byte(`{"orders":[
		{"id":0,"job_type":"ConstructBed","amount_total":2,"amount_left":2,"validated":true,"active":false},
		{"id":1,"job_type":"BrewDrink","amount_total":5,"amount_left":3,"validated":true,"active":true}
	]}`)
	out := renderManagerOrders(raw)
	if !strings.Contains(out, "2 manager orders:") {
		t.Fatalf("missing count header:\n%s", out)
	}
	if !strings.Contains(out, "id=0 ConstructBed x2 (2 left) — queued, not yet dispatched") {
		t.Fatalf("validated-but-inactive order rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "id=1 BrewDrink x5 (3 left) — ACTIVE") {
		t.Fatalf("active order rendered wrong:\n%s", out)
	}
	if strings.Contains(out, "list_orders") || strings.Contains(out, "job_type\":") {
		t.Fatalf("must not leak the raw job-type catalog or JSON:\n%s", out)
	}
	if out := renderManagerOrders([]byte(`{"orders":[]}`)); out != "No manager orders queued." {
		t.Fatalf("empty orders rendering wrong: %q", out)
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
	],"depot":{"exists":true,"x":28,"y":52,"z":110,"built":true,"accessible":true,
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
	if !strings.Contains(out, "Depot at (28,52,110): built=true accessible=true trader_requested=true anyone_can_trade=false") {
		t.Fatalf("depot line rendered wrong:\n%s", out)
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

func TestRenderDepotGoods(t *testing.T) {
	raw := []byte(`{"depot":{"x":28,"y":52,"z":110,"built":true},
		"staged":[
			{"item_type":"CRAFTS","material":"silver","count":3,"value":900,"requested":true},
			{"item_type":"BOULDER","material":"shale","count":5,"value":50,"requested":false}
		],
		"pending":[
			{"item_type":"WEAPON","material":"iron","count":1,"value":200,"requested":false}
		]}`)
	out := renderDepotGoods(raw)
	if !strings.Contains(out, "Depot at (28,52,110) (built=true):") {
		t.Fatalf("depot header rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "silver CRAFTS x3 (value 900) (requested by liaison)") {
		t.Fatalf("requested staged entry rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "shale BOULDER x5 (value 50)") || strings.Contains(out, "shale BOULDER x5 (value 50) (requested") {
		t.Fatalf("non-requested staged entry must not carry the requested suffix:\n%s", out)
	}
	if !strings.Contains(out, "pending (hauling, not yet arrived):") {
		t.Fatalf("pending section header rendered wrong:\n%s", out)
	}
	if !strings.Contains(out, "iron WEAPON x1 (value 200)") {
		t.Fatalf("pending entry rendered wrong:\n%s", out)
	}

	emptyBoth := []byte(`{"depot":{"x":1,"y":2,"z":3,"built":false},"staged":[],"pending":[]}`)
	outEmpty := renderDepotGoods(emptyBoth)
	if !strings.Contains(outEmpty, "staged: none") || !strings.Contains(outEmpty, "pending (hauling, not yet arrived): none") {
		t.Fatalf("empty staged/pending must render 'none':\n%s", outEmpty)
	}

	if out := renderDepotGoods([]byte(`not json`)); !strings.Contains(out, "unparseable depot_goods response") {
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
