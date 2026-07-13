package mcpserver

import (
	"strings"
	"testing"
)

func TestRenderLocations_Basic(t *testing.T) {
	raw := []byte(`{"locations":[
		{"id":5,"type":"INN_TAVERN","x1":10,"y1":10,"x2":11,"y2":11,"z":90,"lodging":[{"civzone_id":42,"x":15,"y":15,"z":90}]},
		{"id":6,"type":"TEMPLE","x1":20,"y1":20,"x2":21,"y2":21,"z":90,"lodging":[]}
	]}`)
	out := renderLocations(raw)
	if !strings.Contains(out, "Tavern") || !strings.Contains(out, "(10,10)") {
		t.Fatalf("expected the tavern's type and extents, got:\n%s", out)
	}
	if !strings.Contains(out, "1 lodging room") {
		t.Fatalf("expected a lodging count for the tavern, got:\n%s", out)
	}
	if !strings.Contains(out, "Temple") {
		t.Fatalf("expected the temple, got:\n%s", out)
	}
}

func TestRenderLocations_NoLocations(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[]}`))
	if out != "No locations." {
		t.Fatalf("expected 'No locations.', got %q", out)
	}
}

func TestRenderLocations_Truncated(t *testing.T) {
	out := renderLocations([]byte(`{"locations":[{"id":1,"type":"LIBRARY","x1":1,"y1":1,"x2":1,"y2":1,"z":1,"lodging":[]}],"truncated":true}`))
	if !strings.Contains(out, "truncated") {
		t.Fatalf("expected a truncation note, got:\n%s", out)
	}
}
