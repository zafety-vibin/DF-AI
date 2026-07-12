package mcpserver

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/blueprints"
	"github.com/df-ai/orchestrator/internal/modifications"
	"github.com/df-ai/orchestrator/internal/topology"
)

func TestExpandBlueprint(t *testing.T) {
	lib := blueprints.NewBlueprintLibrary(t.TempDir()) // empty dir → empty lib
	bp := &blueprints.DigBlueprint{
		Name: "tiny", Digs: []blueprints.DigEntry{
			{X: 0, Y: 0, Z: 0, DigType: "default"},
			{X: 1, Y: 0, Z: 0, DigType: "default"},
		},
	}
	lib.AddBlueprint("tiny", bp)
	cmds, err := expandBlueprint(lib, "tiny", modifications.Coordinate{X: 50, Y: 60, Z: 100})
	if err != nil {
		t.Fatalf("expand: %v", err)
	}
	if len(cmds) != 2 || cmds[1].X != 51 || cmds[0].Y != 60 || cmds[0].Z != 100 {
		t.Fatalf("expansion wrong: %+v", cmds)
	}
	if _, err := expandBlueprint(lib, "nope", modifications.Coordinate{}); err == nil {
		t.Fatal("missing blueprint must error")
	}
	_ = strings.TrimSpace
}

func TestSummarizeDryRunNilTopo(t *testing.T) {
	cmds := []blueprints.DigCommand{
		{X: 1, Y: 1, Z: 5, DigType: "default"},
		{X: 2, Y: 1, Z: 5, DigType: "default"},
		{X: 3, Y: 1, Z: 5, DigType: "stairs"},
	}
	got := summarizeDryRun(nil, cmds)
	if !strings.Contains(got, "3 tiles would be designated") {
		t.Errorf("missing total count: %q", got)
	}
	if !strings.Contains(got, "default=2") || !strings.Contains(got, "stairs=1") {
		t.Errorf("missing per-digtype counts: %q", got)
	}
	// nil topology → every tile counts as solid, none open
	if !strings.Contains(got, "3 into solid ground (good), 0 onto already-open tiles") {
		t.Errorf("nil topo should count all solid: %q", got)
	}
}

func TestSummarizeDryRunOpenVsSolid(t *testing.T) {
	topo := topology.NewTopologyOverlay(10, 10, 10)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	// Tile (1,1,5) open, (2,1,5) closed, (3,1,5) left unknown (counts solid).
	if err := topo.SetTileState(1, 1, 5, topology.StateOpen); err != nil {
		t.Fatal(err)
	}
	if err := topo.SetTileState(2, 1, 5, topology.StateClosed); err != nil {
		t.Fatal(err)
	}
	cmds := []blueprints.DigCommand{
		{X: 1, Y: 1, Z: 5, DigType: "default"},
		{X: 2, Y: 1, Z: 5, DigType: "default"},
		{X: 3, Y: 1, Z: 5, DigType: "default"},
	}
	got := summarizeDryRun(topo, cmds)
	if !strings.Contains(got, "2 into solid ground (good), 1 onto already-open tiles") {
		t.Errorf("open/solid split wrong: %q", got)
	}
}
