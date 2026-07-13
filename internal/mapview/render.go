package mapview

import (
	"fmt"
	"strings"
)

// Overlay is one painted layer: glyph replacements at absolute (x,y),
// plus optional footnote lines appended after the legend. Later overlays
// in RenderCrop's slice win per-tile on collision.
type Overlay struct {
	Marks     map[[2]int16]rune
	Footnotes []string
}

// RenderCrop renders a Slice as a labeled glyph grid. No spaces between
// glyphs (tokenization research: separators destroy adjacency).
//
// Paint order (design doc "Component 2", table in section 2.2):
//  1. base terrain (s.Rows)
//  2. water digits 1-7 (s.Water) — always on
//  3. designations 'd' (s.Designated) — always on; two live incidents
//     this session trace to designations being invisible by default
//  4. overlays, in the order passed — dwarves '@' is conventionally
//     overlays[0]; a lens (buildings/designations detail) is later in
//     the slice so it paints last, since the model explicitly asked for
//     that layer. A later overlay hiding an earlier overlay's mark
//     should carry its own footnote (see the buildings/designations
//     lens gather functions in internal/mcpserver/lenses.go) — RenderCrop
//     itself does not synthesize collision footnotes, callers do, because
//     only the caller knows which collisions are worth mentioning.
//
// legendExtra is appended as one more line after the core legend, only
// when non-empty — the GIS dynamic-legend rule: pay legend cost only for
// the layer you asked about.
func RenderCrop(s *Slice, overlays []Overlay, legendExtra string) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "z=%d — %dx%d crop at (%d,%d), north is up, x grows east, y grows south\n",
		s.Z, len(s.Rows[0]), len(s.Rows), s.X1, s.Y1)
	// x-axis labels: each label's DIGITS start exactly over the glyph
	// column they name. Row lines use a "%4d " prefix (5 columns), and
	// each label's "x=" occupies the 2 columns before its digits.
	rowLen := len(s.Rows[0])
	sb.WriteString("   ") // 5-column row prefix minus len("x=")
	fmt.Fprintf(&sb, "x=%d", s.X1)
	// Second label: digits start over the LAST glyph column. Skip it when
	// the grid is too narrow to fit both labels with a 1-space gap.
	if gap := rowLen - 3 - len(fmt.Sprintf("%d", s.X1)); gap >= 1 {
		sb.WriteString(strings.Repeat(" ", gap))
		fmt.Fprintf(&sb, "x=%d", s.X1+int16(rowLen)-1)
	}
	sb.WriteString("\n")

	// Water overlay: depth digit '1'-'7' replaces the base glyph. Always on.
	water := map[[2]int16]rune{}
	for _, w := range s.Water {
		d := w[2]
		if d < 1 {
			continue // defensive: contract omits dry tiles
		}
		if d > 7 {
			d = 7
		}
		water[[2]int16{w[0], w[1]}] = rune('0' + d)
	}
	// Designations: 'd' replaces the base glyph. Always on — designation
	// counts alone were not enough to prevent two live dig-site incidents
	// where the designated tiles were never actually visible.
	designated := map[[2]int16]rune{}
	for _, dpos := range s.Designated {
		designated[[2]int16{dpos[0], dpos[1]}] = 'd'
	}

	paintLayers := make([]map[[2]int16]rune, 0, 2+len(overlays))
	paintLayers = append(paintLayers, water, designated)
	for _, ov := range overlays {
		paintLayers = append(paintLayers, ov.Marks)
	}

	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for _, layer := range paintLayers {
			for pos, r := range layer {
				if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
					glyphs[pos[0]-s.X1] = r
				}
			}
		}
		fmt.Fprintf(&sb, "%4d %s\n", y, string(glyphs))
	}

	sb.WriteString("\nlegend: @ dwarf d designated " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in view\n", len(s.Designated))
	}
	// Aquifer tiles get a count line, not a glyph — the grid stays scannable.
	if len(s.Aquifer) > 0 {
		fmt.Fprintf(&sb, "aquifer tiles in view: %d\n", len(s.Aquifer))
	}
	if legendExtra != "" {
		sb.WriteString(legendExtra + "\n")
	}
	for _, ov := range overlays {
		for _, fn := range ov.Footnotes {
			sb.WriteString(fn + "\n")
		}
	}
	return sb.String()
}

// RenderColumn renders a ColumnProfile as one line per Z, annotated
// relative to the surface.
func RenderColumn(c *ColumnProfile, surfaceZ int16) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "column (%d,%d), top to bottom:\n", c.X, c.Y)
	for _, lv := range c.Levels {
		rel := ""
		switch {
		case lv.Z == surfaceZ:
			rel = "  <- SURFACE"
		case lv.Z == surfaceZ-1:
			rel = "  <- first layer below surface"
		}
		hidden := ""
		if lv.Hidden {
			hidden = " (hidden/undug — diggable)"
		}
		fluids := ""
		if lv.Water > 0 {
			fluids += fmt.Sprintf(" ~%d/7 water", lv.Water)
		}
		if lv.Aquifer {
			fluids += " AQUIFER"
		}
		if lv.Damp {
			fluids += " DAMP"
		}
		fmt.Fprintf(&sb, "z=%d %s %s/%s%s%s%s\n", lv.Z, lv.Glyph, lv.Shape, lv.Material, hidden, fluids, rel)
	}
	return sb.String()
}
