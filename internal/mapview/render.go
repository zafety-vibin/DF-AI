package mapview

import (
	"fmt"
	"strings"
)

// RenderCrop renders a Slice as a labeled glyph grid. No spaces between
// glyphs (tokenization research: separators destroy adjacency). marks
// overlays glyphs at absolute (x,y) — used for dwarves '@'.
func RenderCrop(s *Slice, marks map[[2]int16]rune) string {
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
	// Water overlay: depth digit '1'-'7' replaces the base glyph. Applied
	// BEFORE marks so the '@' dwarf overlay always wins.
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
	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for pos, r := range water {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		for pos, r := range marks {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		fmt.Fprintf(&sb, "%4d %s\n", y, string(glyphs))
	}
	sb.WriteString("\nlegend: @ dwarf " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in view\n", len(s.Designated))
	}
	// Aquifer tiles get a count line, not a glyph — the grid stays scannable.
	if len(s.Aquifer) > 0 {
		fmt.Fprintf(&sb, "aquifer tiles in view: %d\n", len(s.Aquifer))
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
