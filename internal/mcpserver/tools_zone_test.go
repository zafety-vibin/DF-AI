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

func TestRenderZones_ZFilterShowsFullDetail(t *testing.T) {
	raw := []byte(`{"zones":[
		{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":5,"assigned_units":[]},
		{"kind":4,"type_name":"DiningHall","x1":5,"y1":5,"x2":8,"y2":8,"z":90,"owner_unit_id":-1,"assigned_units":[]}
	]}`)
	// A z filter alone (no type filter) must also switch to full per-zone
	// detail, per the design doc's "a type/z filter must return full
	// per-zone detail" testing requirement.
	out := renderZones(raw, "", 90)
	if !strings.Contains(out, "(1,1)") || !strings.Contains(out, "(5,5)") {
		t.Fatalf("expected per-zone extents when filtered by z, got:\n%s", out)
	}
	if strings.Contains(out, "owned") {
		t.Fatalf("z-only filter should not fall back to the summary grouping, got:\n%s", out)
	}
}

func TestRenderZones_NoZones(t *testing.T) {
	out := renderZones([]byte(`{"zones":[]}`), "", -1)
	if out != "No zones." {
		t.Fatalf("expected 'No zones.', got %q", out)
	}
}

func TestRenderZones_NonAnimalRosterSaysAssigned(t *testing.T) {
	// Dormitory is a roster-type zone like Pen/Pond, but its roster is
	// units, not livestock -- must not get the "animals" phrasing.
	raw := []byte(`{"zones":[
		{"kind":6,"type_name":"Dormitory","x1":1,"y1":1,"x2":4,"y2":4,"z":90,"owner_unit_id":-1,"assigned_units":[7]}
	]}`)
	out := renderZones(raw, "", -1)
	if !strings.Contains(out, "1 assigned") {
		t.Fatalf("expected Dormitory to say 'assigned', got:\n%s", out)
	}
	if strings.Contains(out, "animals") {
		t.Fatalf("Dormitory must not use 'animals' phrasing, got:\n%s", out)
	}
}

func TestZoneTypeVocabulary_ContainsAllTypesSorted(t *testing.T) {
	// designate_zone's unknown-type error appends this vocabulary so the
	// model never has to guess at valid zone type names.
	vocab := zoneTypeVocabulary()
	for name := range zoneWireTypes {
		if !strings.Contains(vocab, name) {
			t.Fatalf("zoneTypeVocabulary missing %q: %s", name, vocab)
		}
	}
	if !strings.Contains(vocab, "bedroom") || !strings.Contains(vocab, "water_source") {
		t.Fatalf("expected bedroom and water_source in vocabulary: %s", vocab)
	}
	if strings.Index(vocab, "bedroom") > strings.Index(vocab, "water_source") {
		t.Fatalf("expected sorted vocabulary (bedroom before water_source): %s", vocab)
	}
}

func TestRenderZones_Truncated(t *testing.T) {
	out := renderZones([]byte(`{"zones":[{"kind":1,"type_name":"Bedroom","x1":1,"y1":1,"x2":1,"y2":1,"z":90,"owner_unit_id":-1,"assigned_units":[]}],"truncated":true}`), "", -1)
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}

func TestRenderZoneValue(t *testing.T) {
	raw := []byte(`{"kind":1,"type_name":"Bedroom","room_value_field":"required_bedroom",
		"x1":1,"y1":1,"x2":3,"y2":3,"z":90,
		"components":[
			{"building_type":"Bed","item_type":"BED","material":"silver","quality":"FineCrafted","value":120},
			{"building_type":"Cabinet","item_type":"CABINET","material":"oak","quality":"Ordinary","value":30}
		],
		"component_count":2,"estimated_value":150,
		"tiles_total":9,"tiles_smoothed":9,
		"engravings_count":2,"engravings_by_quality":[{"quality":"FineCrafted","count":2}],
		"experimental_personal_value":9001}`)
	out := renderZoneValue(raw)
	if !strings.Contains(out, "Bedroom at (1,1)-(3,3) z=90") {
		t.Fatalf("missing zone header:\n%s", out)
	}
	if !strings.Contains(out, "sum of 2 furniture components") || !strings.Contains(out, "150") {
		t.Fatalf("missing estimated_value line:\n%s", out)
	}
	if !strings.Contains(out, "required_bedroom") {
		t.Fatalf("missing room_value_field comparison note:\n%s", out)
	}
	if !strings.Contains(out, "silver BED") || !strings.Contains(out, "oak CABINET") {
		t.Fatalf("missing component lines:\n%s", out)
	}
	if !strings.Contains(out, "9/9 smoothed") {
		t.Fatalf("missing tile smoothing line:\n%s", out)
	}
	if !strings.Contains(out, "engravings: 2") || !strings.Contains(out, "2 FineCrafted") {
		t.Fatalf("missing engraving breakdown:\n%s", out)
	}
	if !strings.Contains(out, "UNCALIBRATED/experimental") || !strings.Contains(out, "9001") {
		t.Fatalf("experimental_personal_value must be clearly labeled and reported:\n%s", out)
	}
}

func TestRenderZoneValue_NoRoomValueField(t *testing.T) {
	// A zone kind with no noble_demands analog (e.g. Dungeon) must omit the
	// comparison note entirely rather than printing a bogus field name.
	raw := []byte(`{"kind":17,"type_name":"Dungeon","room_value_field":null,
		"x1":1,"y1":1,"x2":1,"y2":1,"z":90,
		"components":[],"component_count":0,"estimated_value":0,
		"tiles_total":1,"tiles_smoothed":0,
		"engravings_count":0,"engravings_by_quality":[],
		"experimental_personal_value":0}`)
	out := renderZoneValue(raw)
	if strings.Contains(out, "compare against") {
		t.Fatalf("must not print a room_value_field comparison note when null:\n%s", out)
	}
	if !strings.Contains(out, "engravings: none") {
		t.Fatalf("expected 'engravings: none' for zero engravings:\n%s", out)
	}
}

func TestRenderZoneValue_BadJSON(t *testing.T) {
	if out := renderZoneValue([]byte(`not json`)); !strings.Contains(out, "unparseable zone_value response") {
		t.Fatalf("bad JSON must be reported, got: %q", out)
	}
}
