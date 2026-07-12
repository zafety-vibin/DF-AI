package mcpserver

import (
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
