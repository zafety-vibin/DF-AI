package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/mapview"
	"github.com/df-ai/orchestrator/internal/protocol"
)

func TestRenderSurvey(t *testing.T) {
	data := SurveyData{
		MapW: 192, MapH: 192, MapD: 130, SurfaceZ: 111,
		Dwarves: []protocol.EntityInfo{{ID: 1, X: 95, Y: 95, Z: 111}},
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
	for _, want := range []string{"192x192x130", "surface z=111", "(95,95)"} {
		if !strings.Contains(out, want) {
			t.Fatalf("survey missing %q:\n%s", want, out)
		}
	}
}
