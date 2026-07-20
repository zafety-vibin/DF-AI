package mcpserver

import (
	"strings"
	"testing"
)

func TestRenderSquads_NoSquads(t *testing.T) {
	out := renderSquads([]byte(`{"squads":[]}`))
	if out != "No squads." {
		t.Fatalf("expected 'No squads.', got %q", out)
	}
}

func TestRenderSquads_BadJSON(t *testing.T) {
	out := renderSquads([]byte(`not json`))
	if !strings.Contains(out, "unparseable list_squads response") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}

func TestRenderSquads_MembersAndVacancy(t *testing.T) {
	raw := []byte(`{"squads":[
		{"id":3,"name":"The Ax Wardens","entity_id":1,
		 "members":[
		   {"position_index":0,"is_commander":true,"vacant":true,"unit_id":-1},
		   {"position_index":1,"is_commander":false,"vacant":false,"unit_id":42}
		 ],
		 "orders":[]}
	]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, `squad #3 "The Ax Wardens"`) {
		t.Fatalf("expected squad header, got:\n%s", out)
	}
	if !strings.Contains(out, "1/2 slots filled") {
		t.Fatalf("expected filled/total count, got:\n%s", out)
	}
	if !strings.Contains(out, "commander: vacant") {
		t.Fatalf("expected vacant commander line, got:\n%s", out)
	}
	if !strings.Contains(out, "soldier: unit#42") {
		t.Fatalf("expected filled soldier line, got:\n%s", out)
	}
	if !strings.Contains(out, "orders: none") {
		t.Fatalf("expected 'orders: none' for an empty orders list, got:\n%s", out)
	}
}

func TestRenderSquads_UnnamedSquad(t *testing.T) {
	raw := []byte(`{"squads":[{"id":1,"name":"","entity_id":1,"members":[],"orders":[]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "(unnamed)") {
		t.Fatalf("expected a placeholder name for an empty squad name, got:\n%s", out)
	}
}

func TestRenderSquads_StationOrder(t *testing.T) {
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"station","x":10,"y":20,"z":90,"burrows":[]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "station at (10,20,90)") {
		t.Fatalf("expected station order line, got:\n%s", out)
	}
}

func TestRenderSquads_DefendBurrowOrder(t *testing.T) {
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"defend_burrow","x":0,"y":0,"z":0,"burrows":[{"id":5,"name":"chokepoint"}]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, `defend burrow(s) "chokepoint"`) {
		t.Fatalf("expected defend_burrow order line, got:\n%s", out)
	}
}

func TestRenderSquads_OtherOrderType(t *testing.T) {
	// An order type this tool doesn't decode (e.g. a kill-list order set
	// by some other means) must be reported generically, never dropped
	// silently or misrendered as one of the two known types.
	raw := []byte(`{"squads":[{"id":2,"name":"Guard","entity_id":1,"members":[],
		"orders":[{"type":"other","x":0,"y":0,"z":0,"burrows":[]}]}]}`)
	out := renderSquads(raw)
	if !strings.Contains(out, "other order type") {
		t.Fatalf("expected a generic fallback line for an undecoded order type, got:\n%s", out)
	}
}
