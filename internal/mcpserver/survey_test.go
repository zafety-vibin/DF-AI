package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

// surfaceSampleAt builds a minimal single-level ColumnProfile whose surface
// (mapview.ColumnSurfaceZ) is exactly z — enough to drive the survey's
// surface-range line in tests without needing a full stratigraphy.
func surfaceSampleAt(x, y, z int16) *mapview.ColumnProfile {
	return &mapview.ColumnProfile{X: x, Y: y, Levels: []mapview.ColumnLevel{
		{Z: z, Glyph: ",", Shape: "floor", Material: "grass"},
	}}
}

func TestRenderSurvey(t *testing.T) {
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, CrewZ: 111,
		Dwarves:        []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
		SurfaceSamples: []*mapview.ColumnProfile{surfaceSampleAt(95, 95, 111)},
		Columns: []*mapview.ColumnProfile{{
			X: 95, Y: 95,
			Levels: []mapview.ColumnLevel{
				{Z: 111, Glyph: ",", Shape: "floor", Material: "grass", Hidden: false},
				{Z: 110, Glyph: "?", Shape: "hidden", Material: "unknown", Hidden: true},
				{Z: 109, Glyph: "?", Shape: "hidden", Material: "unknown", Hidden: true},
			},
		}},
	}
	out := renderSurvey(data)
	// The sampled column has no visible stone: say so instead of the
	// nonsense sentinel "first exposed stone at z=-1".
	for _, want := range []string{"192x192x130", "surface z ranges 111..111 across 1 sampled columns (embark crew at z=111)", "(95,95)",
		"no stone in sampled range"} {
		if !strings.Contains(out, want) {
			t.Fatalf("survey missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "z=-1") {
		t.Fatalf("survey leaked the -1 sentinel:\n%s", out)
	}
}

func TestRenderSurveyExposedStone(t *testing.T) {
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, CrewZ: 111,
		Dwarves:        []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
		SurfaceSamples: []*mapview.ColumnProfile{surfaceSampleAt(95, 95, 111)},
		Columns: []*mapview.ColumnProfile{{
			X: 95, Y: 95,
			Levels: []mapview.ColumnLevel{
				{Z: 111, Glyph: ",", Shape: "floor", Material: "grass"},
				{Z: 110, Glyph: "%", Shape: "wall", Material: "soil"},
				{Z: 109, Glyph: "#", Shape: "wall", Material: "stone"},
			},
		}},
	}
	out := renderSurvey(data)
	if !strings.Contains(out, "1 soil layers, first exposed stone at z=109") {
		t.Fatalf("stone column summary wrong:\n%s", out)
	}
}

func TestRenderSurveyFogStone(t *testing.T) {
	// Post-truthful-classification, hidden layers carry real materials;
	// stone under fog must not be called "exposed".
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, CrewZ: 111,
		Dwarves:        []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
		SurfaceSamples: []*mapview.ColumnProfile{surfaceSampleAt(95, 95, 111)},
		Columns: []*mapview.ColumnProfile{{
			X: 95, Y: 95,
			Levels: []mapview.ColumnLevel{
				{Z: 111, Glyph: ",", Shape: "floor", Material: "grass"},
				{Z: 110, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
				{Z: 109, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
			},
		}},
	}
	out := renderSurvey(data)
	if !strings.Contains(out, "first stone at z=109 (under fog — undug, diggable)") {
		t.Fatalf("fog stone summary wrong:\n%s", out)
	}
	if strings.Contains(out, "exposed stone") {
		t.Fatalf("fog stone must not be called exposed:\n%s", out)
	}
	// No aquifer flags anywhere: the note must not appear.
	if strings.Contains(out, "AQUIFER") {
		t.Fatalf("unexpected aquifer note:\n%s", out)
	}
}

// TestAquiferNote covers the column summary annotation: single layer, a
// contiguous run (levels top-to-bottom), non-contiguous fallback, none.
func TestAquiferNote(t *testing.T) {
	cases := []struct {
		name   string
		levels []mapview.ColumnLevel
		want   string
	}{
		{"none", []mapview.ColumnLevel{{Z: 110}, {Z: 109}}, ""},
		{"single", []mapview.ColumnLevel{{Z: 110}, {Z: 109, Aquifer: true}}, ", AQUIFER at z=109"},
		{"contiguous", []mapview.ColumnLevel{
			{Z: 110}, {Z: 109, Aquifer: true}, {Z: 108, Aquifer: true}, {Z: 107, Aquifer: true}, {Z: 106},
		}, ", AQUIFER at z=109..107"},
		{"non-contiguous falls back to first", []mapview.ColumnLevel{
			{Z: 110, Aquifer: true}, {Z: 109}, {Z: 108, Aquifer: true},
		}, ", AQUIFER at z=110"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := aquiferNote(tc.levels); got != tc.want {
				t.Fatalf("aquiferNote = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestRenderSurveyAquiferColumn(t *testing.T) {
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, CrewZ: 111,
		Dwarves:        []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
		SurfaceSamples: []*mapview.ColumnProfile{surfaceSampleAt(95, 95, 111)},
		Columns: []*mapview.ColumnProfile{{
			X: 95, Y: 95,
			Levels: []mapview.ColumnLevel{
				{Z: 111, Glyph: ",", Shape: "floor", Material: "grass"},
				{Z: 110, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true, Aquifer: true},
				{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true, Aquifer: true},
			},
		}},
	}
	out := renderSurvey(data)
	if !strings.Contains(out, "column (95,95): 2 soil layers, no stone in sampled range, AQUIFER at z=110..109") {
		t.Fatalf("aquifer column summary wrong:\n%s", out)
	}
}
