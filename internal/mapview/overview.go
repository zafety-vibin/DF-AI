package mapview

import (
	"fmt"
	"strings"

	"github.com/df-ai/orchestrator/internal/topology"
)

// overviewMaxCells bounds the rendered grid on each axis. A 192x192 map
// downsampled 4:1 is 48x48 — the same "no volumetric/raw dump" budget
// look's own radius cap enforces, just applied to the whole map instead
// of a crop.
const overviewMaxCells = 48

// overviewBlockSize picks the NxN tile block size so the rendered grid
// fits within overviewMaxCells on both axes. Always >= 1.
func overviewBlockSize(w, h uint16) int {
	dim := int(w)
	if int(h) > dim {
		dim = int(h)
	}
	block := (dim + overviewMaxCells - 1) / overviewMaxCells
	if block < 1 {
		block = 1
	}
	return block
}

// RenderOverview renders a downsampled whole-map view at z from the
// already-resident TopologyOverlay: no plugin round-trip, and no
// tile-accurate claim — this is for "where am I on the map", not for
// picking a dig site (find_dig_site / look do that).
//
// Each output cell summarizes a block x block square of tiles by
// majority state:
//
//	.  majority of tiles in the block are open (walkable)
//	#  majority are closed (solid: wall/void/liquid)
//	?  majority are unknown (unexplored / hidden fog)
//	x  no majority — the block straddles open and solid ground
//	@  the block contains at least one dwarf (overrides the state glyph)
//
// dwarfXY are dwarf (x,y) positions already filtered to this z by the
// caller (mirrors how look only marks dwarves on the requested z).
func RenderOverview(topo *topology.TopologyOverlay, z int16, dwarfXY [][2]int16) string {
	w, h, d := topo.GetDimensions()
	if z < 0 || z >= int16(d) {
		return fmt.Sprintf("z=%d out of range: map has %d levels (valid z is 0..%d)", z, d, int(d)-1)
	}
	block := overviewBlockSize(w, h)
	cols := (int(w) + block - 1) / block
	rows := (int(h) + block - 1) / block

	dwarfBlocks := make(map[[2]int]bool, len(dwarfXY))
	for _, p := range dwarfXY {
		dwarfBlocks[[2]int{int(p[0]) / block, int(p[1]) / block}] = true
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "orientation overview (NOT tile-accurate) at z=%d: %dx%d map downsampled to %dx%d cells, 1 cell = %dx%d tiles\n",
		z, w, h, cols, rows, block, block)

	for by := 0; by < rows; by++ {
		var line strings.Builder
		for bx := 0; bx < cols; bx++ {
			if dwarfBlocks[[2]int{bx, by}] {
				line.WriteByte('@')
				continue
			}
			var open, closed, unknown, total int
			x0, y0 := bx*block, by*block
			x1, y1 := x0+block, y0+block
			if x1 > int(w) {
				x1 = int(w)
			}
			if y1 > int(h) {
				y1 = int(h)
			}
			for x := x0; x < x1; x++ {
				for y := y0; y < y1; y++ {
					total++
					switch topo.GetTileState(int16(x), int16(y), z) {
					case topology.StateOpen:
						open++
					case topology.StateClosed:
						closed++
					default:
						unknown++
					}
				}
			}
			line.WriteRune(majorityGlyph(open, closed, unknown, total))
		}
		fmt.Fprintf(&sb, "%s\n", line.String())
	}

	sb.WriteString("\nlegend: . open-majority # solid-majority ? unknown-majority x mixed @ dwarves present\n")
	return sb.String()
}

// majorityGlyph classifies one block: a category needs a strict majority
// (>half the block's tiles) to win; otherwise the block is "mixed".
func majorityGlyph(open, closed, unknown, total int) rune {
	if total == 0 {
		return '?'
	}
	half := total / 2
	switch {
	case unknown > half:
		return '?'
	case open > half:
		return '.'
	case closed > half:
		return '#'
	default:
		return 'x'
	}
}
