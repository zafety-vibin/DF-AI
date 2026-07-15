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

// RenderColumn renders a ColumnProfile as one line per Z, annotated relative
// to THIS column's own surface (see ColumnSurfaceZ) — never a value computed
// elsewhere and passed in, which is how a global proxy silently drifted
// between calls and mislabeled columns whose real ground sits at a
// different Z.
func RenderColumn(c *ColumnProfile) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "column (%d,%d), top to bottom:\n", c.X, c.Y)
	surfaceZ, result := ColumnSurfaceZ(c)
	for _, lv := range c.Levels {
		rel := ""
		if result == SurfaceFound {
			switch {
			case lv.Z == surfaceZ:
				rel = "  <- SURFACE"
			case lv.Z == surfaceZ-1:
				rel = "  <- first layer below surface"
			}
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
	switch result {
	case SurfaceAboveWindow:
		sb.WriteString("surface at or above z_top — raise z_top\n")
	case SurfaceUnknownAllAir:
		sb.WriteString("no surface in this z-window — widen z_bottom (window is entirely open air)\n")
	}
	return sb.String()
}

// RenderElevation renders a vertical slice along a line (the third
// orthogonal plane, alongside look's plan view and cross_section's
// single-column bore) — one row per Z, one glyph column per swept
// x (or y) position. Reuses each ColumnProfile's own per-level
// classification (shape/material/hidden/fluids), matching cross_section's
// existing rendering exactly, just composed across a line instead of a
// single point.
//
// columns must all share the same Z range and be pre-sorted by the swept
// coordinate (ascending) — RenderElevation does not sort or validate
// this; the caller (tools_percept.go's elevation_view handler) builds
// them in sweep order.
func RenderElevation(columns []*ColumnProfile, axis string, fixed int16) string {
	if len(columns) == 0 {
		return "no columns to render"
	}
	var sb strings.Builder
	first, last := columns[0], columns[len(columns)-1]
	if axis == "x" {
		fmt.Fprintf(&sb, "elevation along x=%d..%d at y=%d, top to bottom:\n", first.X, last.X, fixed)
	} else {
		fmt.Fprintf(&sb, "elevation along y=%d..%d at x=%d, top to bottom:\n", first.Y, last.Y, fixed)
	}

	// Index each column's levels by Z for row-major access.
	byZ := make([]map[int16]ColumnLevel, len(columns))
	for i, c := range columns {
		m := make(map[int16]ColumnLevel, len(c.Levels))
		for _, lv := range c.Levels {
			m[lv.Z] = lv
		}
		byZ[i] = m
	}

	// All columns are expected to share the same Z range (same z_top/z_bottom
	// request) — use the first column's levels to drive row order.
	for _, lv0 := range columns[0].Levels {
		fmt.Fprintf(&sb, "z=%d ", lv0.Z)
		for i := range columns {
			lv, ok := byZ[i][lv0.Z]
			if !ok {
				sb.WriteString("?")
				continue
			}
			sb.WriteString(lv.Glyph)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\nlegend: " + Legend + "\n")

	// Per-column surface summary: each column's OWN surface (ColumnSurfaceZ),
	// never one map-wide proxy stamped across every column swept. "above"
	// means the window's top was already solid (raise z_top to confirm);
	// "none" means the whole window was open air (widen z_bottom).
	sb.WriteString("surfaces:")
	for _, c := range columns {
		coord := c.X
		if axis == "y" {
			coord = c.Y
		}
		switch z, result := ColumnSurfaceZ(c); result {
		case SurfaceFound:
			fmt.Fprintf(&sb, " %s=%d:%d", axis, coord, z)
		case SurfaceAboveWindow:
			fmt.Fprintf(&sb, " %s=%d:above", axis, coord)
		default:
			fmt.Fprintf(&sb, " %s=%d:none", axis, coord)
		}
	}
	sb.WriteString("\n")
	return sb.String()
}
