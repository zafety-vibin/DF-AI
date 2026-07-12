package mapview

import (
	"fmt"
	"strings"
)

// RenderCrop renders a Slice as a labeled glyph grid. No spaces between
// glyphs (tokenization research: separators destroy adjacency). marks
// overlays glyphs at absolute (x,y) — used for dwarves '@' and wagon 'W'.
func RenderCrop(s *Slice, marks map[[2]int16]rune) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "z=%d — %dx%d crop at (%d,%d), north is up, x grows east, y grows south\n",
		s.Z, len(s.Rows[0]), len(s.Rows), s.X1, s.Y1)
	fmt.Fprintf(&sb, "     x=%d", s.X1)
	pad := len(s.Rows[0]) - len(fmt.Sprintf("%d", s.X1)) - 2
	if pad > 0 {
		sb.WriteString(strings.Repeat(" ", pad))
		fmt.Fprintf(&sb, "x=%d", s.X1+int16(len(s.Rows[0]))-1)
	}
	sb.WriteString("\n")
	for i, row := range s.Rows {
		y := s.Y1 + int16(i)
		glyphs := []rune(row)
		for pos, r := range marks {
			if pos[1] == y && pos[0] >= s.X1 && int(pos[0]-s.X1) < len(glyphs) {
				glyphs[pos[0]-s.X1] = r
			}
		}
		fmt.Fprintf(&sb, "%4d %s\n", y, string(glyphs))
	}
	sb.WriteString("\nlegend: @ dwarf W wagon " + Legend + "\n")
	if len(s.Designated) > 0 {
		fmt.Fprintf(&sb, "designated for digging: %d tiles in view\n", len(s.Designated))
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
		fmt.Fprintf(&sb, "z=%d %s %s/%s%s%s\n", lv.Z, lv.Glyph, lv.Shape, lv.Material, hidden, rel)
	}
	return sb.String()
}
