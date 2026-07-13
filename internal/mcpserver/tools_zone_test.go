package mcpserver

import (
	"strings"
	"testing"
)

func TestRenderZones_GroupsByTypeByDefault(t *testing.T) {
	raw := []byte(`{"zones":[
		{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":5,"assigned_units":[]},
		{"kind":1,"type_name":"Bedroom","x1":2,"y1":2,"x2":2,"y2":2,"z":90,"owner_unit_id":-1,"assigned_units":[]},
		{"kind":4,"type_name":"DiningHall","x1":5,"y1":5,"x2":8,"y2":8,"z":90,"owner_unit_id":-1,"assigned_units":[]},
		{"kind":8,"type_name":"Pen","x1":10,"y1":10,"x2":15,"y2":15,"z":85,"owner_unit_id":-1,"assigned_units":[1,2,3]}
	]}`)
	out := renderZones(raw, "", -1)
	// Must collapse same-type zones to ONE line, not one line per zone.
	if strings.Count(out, "Bedroom") != 1 {
		t.Fatalf("expected exactly one Bedroom summary line, got:\n%s", out)
	}
	if !strings.Contains(out, "2 Bedroom") || !strings.Contains(out, "1 owned") {
		t.Fatalf("expected a bedroom count with an owned breakdown, got:\n%s", out)
	}
	if !strings.Contains(out, "1 DiningHall") {
		t.Fatalf("expected a dining hall summary, got:\n%s", out)
	}
	if !strings.Contains(out, "1 Pen") || !strings.Contains(out, "3 animals") {
		t.Fatalf("expected a pen summary with animal count, got:\n%s", out)
	}
}

func TestRenderZones_TypeFilterShowsFullDetail(t *testing.T) {
	raw := []byte(`{"zones":[
		{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":5,"assigned_units":[]},
		{"kind":4,"type_name":"DiningHall","x1":5,"y1":5,"x2":8,"y2":8,"z":90,"owner_unit_id":-1,"assigned_units":[]}
	]}`)
	out := renderZones(raw, "Bedroom", -1)
	if !strings.Contains(out, "(1,1)") {
		t.Fatalf("expected per-zone extents when filtered by type, got:\n%s", out)
	}
	if strings.Contains(out, "DiningHall") {
		t.Fatalf("type filter must exclude other types, got:\n%s", out)
	}
}

func TestRenderZones_NoZones(t *testing.T) {
	out := renderZones([]byte(`{"zones":[]}`), "", -1)
	if out != "No zones." {
		t.Fatalf("expected 'No zones.', got %q", out)
	}
}

func TestRenderZones_Truncated(t *testing.T) {
	out := renderZones([]byte(`{"zones":[{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":-1,"assigned_units":[]}],"truncated":true}`), "", -1)
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}
