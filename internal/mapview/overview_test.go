package mapview

import (
	"strings"
	"testing"

	"github.com/df-ai/orchestrator/internal/protocol"
	"github.com/df-ai/orchestrator/internal/topology"
)

func TestOverviewBlockSize(t *testing.T) {
	cases := []struct {
		w, h uint16
		want int
	}{
		{40, 40, 1},   // fits within 48 cells raw, no downsampling needed
		{48, 48, 1},   // exactly at the cap
		{49, 49, 2},   // one tile over the cap forces 2:1
		{96, 96, 2},   // a common map size: 1 cell = 2x2 tiles
		{192, 192, 4}, // full map: 1 cell = 4x4 tiles
		{192, 40, 4},  // block size driven by the larger dimension
	}
	for _, c := range cases {
		if got := overviewBlockSize(c.w, c.h); got != c.want {
			t.Errorf("overviewBlockSize(%d,%d) = %d, want %d", c.w, c.h, got, c.want)
		}
	}
}

func TestMajorityGlyph(t *testing.T) {
	cases := []struct {
		open, closed, unknown, total int
		want                         rune
	}{
		{6, 0, 0, 6, '.'},  // unanimous open
		{0, 6, 0, 6, '#'},  // unanimous closed
		{0, 0, 6, 6, '?'},  // unanimous unknown
		{3, 3, 0, 6, 'x'},  // even split, no majority
		{4, 2, 0, 6, '.'},  // strict majority open
		{2, 4, 0, 6, '#'},  // strict majority closed
		{2, 2, 2, 6, 'x'},  // three-way split, no majority
		{0, 0, 0, 0, '?'},  // empty block (defensive; shouldn't occur on a real map)
	}
	for _, c := range cases {
		if got := majorityGlyph(c.open, c.closed, c.unknown, c.total); got != c.want {
			t.Errorf("majorityGlyph(open=%d,closed=%d,unknown=%d,total=%d) = %q, want %q",
				c.open, c.closed, c.unknown, c.total, got, c.want)
		}
	}
}

// buildOverviewFixture constructs a real 50x50x1 overlay (block size 2, so
// 1 cell = 2x2 tiles, 25x25 output grid) via BuildFromTiles, with four
// deliberately distinct 2x2 blocks at (bx=0..3, by=0) along the top row:
//
//	bx=0: all 4 tiles open    -> expect '.'
//	bx=1: all 4 tiles closed  -> expect '#'
//	bx=2: all 4 tiles hidden  -> expect '?'
//	bx=3: 2 open + 2 closed   -> expect 'x' (tie, no majority)
//
// Every other tile on the map is open, so every other output cell is '.'.
func buildOverviewFixture(t *testing.T) *topology.TopologyOverlay {
	t.Helper()
	const w, h, d = 50, 50, 1
	topo := topology.NewTopologyOverlay(w, h, d)
	if topo == nil {
		t.Fatal("overlay nil")
	}
	var tiles []protocol.TileState
	for y := int16(0); y < h; y++ {
		for x := int16(0); x < w; x++ {
			flags := protocol.FlagDiscovered | protocol.FlagFloor // default: open
			switch {
			case x >= 2 && x <= 3 && y <= 1: // bx=1 block: all closed
				flags = protocol.FlagDiscovered | protocol.FlagWall
			case x >= 4 && x <= 5 && y <= 1: // bx=2 block: all hidden
				flags = protocol.FlagHidden
			case x == 6 && y <= 1: // bx=3 block, left column: closed
				flags = protocol.FlagDiscovered | protocol.FlagWall
			case x == 7 && y <= 1: // bx=3 block, right column: open (tie with left)
				flags = protocol.FlagDiscovered | protocol.FlagFloor
			}
			tiles = append(tiles, protocol.TileState{X: x, Y: y, Z: 0, TileType: 600, Flags: flags})
		}
	}
	if err := topo.BuildFromTiles(tiles); err != nil {
		t.Fatalf("build: %v", err)
	}
	return topo
}

func TestRenderOverview_KnownBlockStates(t *testing.T) {
	topo := buildOverviewFixture(t)
	out := RenderOverview(topo, 0, nil)
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) < 2 {
		t.Fatalf("expected header + grid rows, got: %q", out)
	}
	if !strings.Contains(lines[0], "1 cell = 2x2 tiles") {
		t.Errorf("header missing block-size disclosure: %q", lines[0])
	}
	if !strings.Contains(lines[0], "50x50 map downsampled to 25x25 cells") {
		t.Errorf("header missing dimensions: %q", lines[0])
	}
	gridRow0 := lines[1]
	want := ".#?x" + strings.Repeat(".", 25-4)
	if gridRow0 != want {
		t.Fatalf("row 0 = %q, want %q", gridRow0, want)
	}
	// Every other row is fully open ground.
	for i := 2; i <= 24; i++ {
		wantRow := strings.Repeat(".", 25)
		if lines[i] != wantRow {
			t.Fatalf("row %d = %q, want %q", i-1, lines[i], wantRow)
		}
	}
	if !strings.Contains(out, "legend:") {
		t.Errorf("missing legend line: %q", out)
	}
}

func TestRenderOverview_DwarfMarkerOverridesBlockGlyph(t *testing.T) {
	topo := buildOverviewFixture(t)
	// (2,0) sits in the bx=1 (all-closed) block, which would otherwise
	// render '#'.
	out := RenderOverview(topo, 0, [][2]int16{{2, 0}})
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	gridRow0 := lines[1]
	if gridRow0[1] != '@' {
		t.Fatalf("expected dwarf marker at cell 1, row 0 = %q", gridRow0)
	}
}

func TestRenderOverview_OutOfRangeZ(t *testing.T) {
	topo := buildOverviewFixture(t)
	out := RenderOverview(topo, 5, nil)
	if !strings.Contains(out, "out of range") {
		t.Errorf("expected out-of-range message, got: %q", out)
	}
}
