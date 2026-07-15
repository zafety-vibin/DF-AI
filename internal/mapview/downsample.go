package mapview

import (
	"fmt"
	"strings"
)

// DownsampleBlockSizeFor picks the NxN tile block size so a w x h grid,
// downsampled by that block, fits within maxCells on both axes. Always
// >= 1. Same shape as overview.go's overviewBlockSize, generalized to an
// arbitrary cell budget (elevation/fort scope target ~100, not overview's
// 48) and plain int dimensions (a stitched Slice's rendered size, not a
// map's uint16 tile dimensions).
func DownsampleBlockSizeFor(w, h, maxCells int) int {
	dim := w
	if h > dim {
		dim = h
	}
	block := (dim + maxCells - 1) / maxCells
	if block < 1 {
		block = 1
	}
	return block
}

// DownsampleGlyphs collapses a glyph grid into a lower-resolution grid by
// majority vote per block x block square of source cells: the modal (most
// common) glyph in each block wins. This is a pure raster operation with no
// knowledge of terrain semantics — callers pass in a grid that already has
// water/designation glyphs painted onto the base terrain (see
// paintOverlaysOntoRows) so those classes can still win a block's majority,
// unlike RenderOverview's 3-state open/closed/unknown collapse. Ties break
// toward the lexicographically smallest rune, for determinism.
func DownsampleGlyphs(rows []string, block int) []string {
	if block <= 1 {
		return rows
	}
	srcH := len(rows)
	if srcH == 0 {
		return rows
	}
	srcW := len([]rune(rows[0]))
	outRows := (srcH + block - 1) / block
	outCols := (srcW + block - 1) / block

	// Pre-split into rune rows once, not per-block.
	runeRows := make([][]rune, srcH)
	for i, row := range rows {
		runeRows[i] = []rune(row)
	}

	out := make([]string, outRows)
	for by := 0; by < outRows; by++ {
		y0, y1 := by*block, by*block+block
		if y1 > srcH {
			y1 = srcH
		}
		var sb strings.Builder
		for bx := 0; bx < outCols; bx++ {
			x0, x1 := bx*block, bx*block+block
			if x1 > srcW {
				x1 = srcW
			}
			counts := map[rune]int{}
			for y := y0; y < y1; y++ {
				rr := runeRows[y]
				for x := x0; x < x1 && x < len(rr); x++ {
					counts[rr[x]]++
				}
			}
			sb.WriteRune(modalGlyph(counts))
		}
		out[by] = sb.String()
	}
	return out
}

// modalGlyph returns the highest-count rune in counts, breaking ties toward
// the lexicographically smallest rune. Returns '?' for an empty map
// (defensive; a real grid always has at least one tile per block).
func modalGlyph(counts map[rune]int) rune {
	var best rune
	bestN := -1
	for r, n := range counts {
		if n > bestN || (n == bestN && r < best) {
			best, bestN = r, n
		}
	}
	if bestN < 0 {
		return '?'
	}
	return best
}

// paintOverlaysOntoRows returns s.Rows with water depth digits and
// designation marks painted on top — the same first two paint layers
// RenderCrop applies (water, then designated), factored out so
// RenderDownsampledSlice can majority-vote over the same classified glyphs
// a full-fidelity render would show, not bare undecorated terrain. Dwarf/
// lens overlays are deliberately NOT included: elevation/fort are
// whole-level or whole-footprint views, not a lens-annotated local crop.
func paintOverlaysOntoRows(s *Slice) []string {
	water := map[[2]int16]rune{}
	for _, w := range s.Water {
		d := w[2]
		if d < 1 {
			continue
		}
		if d > 7 {
			d = 7
		}
		water[[2]int16{w[0], w[1]}] = rune('0' + d)
	}
	designated := map[[2]int16]rune{}
	for _, dpos := range s.Designated {
		designated[[2]int16{dpos[0], dpos[1]}] = 'd'
	}

	rows := make([]string, len(s.Rows))
	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for pos, r := range water {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		for pos, r := range designated {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		rows[i] = string(glyphs)
	}
	return rows
}

// RenderDownsampledSlice renders a Slice at block x block granularity: each
// output glyph is the majority terrain glyph (water/designation classes
// included) in its block. Used by look's elevation/fort scope when a
// full-fidelity render (RenderCrop) would exceed the ~100x100 render
// budget — the header discloses the block size exactly like RenderOverview
// does, so the model knows it is looking at a lossy summary, not raw tiles.
func RenderDownsampledSlice(s *Slice, block int) string {
	painted := paintOverlaysOntoRows(s)
	down := DownsampleGlyphs(painted, block)
	srcW, srcH := 0, len(s.Rows)
	if srcH > 0 {
		srcW = len(s.Rows[0])
	}
	outW, outH := 0, len(down)
	if outH > 0 {
		outW = len(down[0])
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "z=%d — %dx%d tiles downsampled to %dx%d cells (1 cell = %dx%d tiles, majority glyph), origin (%d,%d)\n",
		s.Z, srcW, srcH, outW, outH, block, block, s.X1, s.Y1)
	for _, row := range down {
		sb.WriteString(row)
		sb.WriteString("\n")
	}
	sb.WriteString("\nlegend: d designated(majority) " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in source view (may be under-represented after downsampling)\n", len(s.Designated))
	}
	if len(s.Aquifer) > 0 {
		fmt.Fprintf(&sb, "aquifer tiles in source view: %d\n", len(s.Aquifer))
	}
	return sb.String()
}
