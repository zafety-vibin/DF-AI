package mcpserver

import (
	"fmt"
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
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

func TestRenderStocksBadJSON(t *testing.T) {
	if out := renderStocks([]byte(`not json`), false, 0); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
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
		"mood":-1,
		"labors":["MINE","CUTWOOD","HAUL_STONE"]
	}`)

	out := renderDwarfDetail(raw, false)
	if !strings.Contains(out, "Urist (id=5) @(23,45,138)") {
		t.Fatalf("missing name/position line:\n%s", out)
	}
	if !strings.Contains(out, "current job: Mine") {
		t.Fatalf("missing current job line:\n%s", out)
	}
	if !strings.Contains(out, "mood: none") {
		t.Fatalf("mood=-1 must render as 'none':\n%s", out)
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

	withLabors := renderDwarfDetail(raw, true)
	if !strings.Contains(withLabors, "labors (3 enabled): MINE, CUTWOOD, HAUL_STONE") {
		t.Fatalf("include_labors=true must list every labor name:\n%s", withLabors)
	}
}

func TestRenderDwarfDetailNoJobNoSkillsNoLabors(t *testing.T) {
	raw := []byte(`{"id":9,"position":{"x":1,"y":2,"z":3},"first_name":"","top_skills":[],"current_job":null,"mood":0,"labors":[]}`)
	out := renderDwarfDetail(raw, true)
	if !strings.Contains(out, "dwarf#9 (id=9)") {
		t.Fatalf("empty first_name must fall back to a synthesized name:\n%s", out)
	}
	if !strings.Contains(out, "current job: idle") {
		t.Fatalf("null current_job must render as idle:\n%s", out)
	}
	if !strings.Contains(out, "mood: FEY MOOD") {
		t.Fatalf("mood=0 must render as FEY MOOD:\n%s", out)
	}
	if !strings.Contains(out, "top skills: none") {
		t.Fatalf("empty top_skills must say none:\n%s", out)
	}
	if !strings.Contains(out, "labors: none enabled") {
		t.Fatalf("empty labors with include_labors=true must say none enabled:\n%s", out)
	}
}

func TestRenderDwarfDetailBadJSON(t *testing.T) {
	if out := renderDwarfDetail([]byte(`not json`), false); !strings.Contains(out, "unparseable") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestDwarfSummaryLine(t *testing.T) {
	d, err := parseDwarfDetail([]byte(`{
		"id":5,"position":{"x":1,"y":1,"z":1},"first_name":"Urist",
		"top_skills":[{"skill":"MINING","level":4,"experience":120}],
		"current_job":"Mine","mood":-1,"labors":["MINE","CUTWOOD"]
	}`))
	if err != nil {
		t.Fatalf("parseDwarfDetail: %v", err)
	}
	line := dwarfSummaryLine(d)
	if !strings.Contains(line, "Urist (id=5)") || !strings.Contains(line, "Mine") ||
		!strings.Contains(line, "MINING Lvl4") || !strings.Contains(line, "labors=2") {
		t.Fatalf("census line missing expected fields: %q", line)
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
