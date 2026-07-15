package mapview

import (
	"strings"
	"testing"
)

func TestRenderCrop(t *testing.T) {
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{"##..", "#?.,", ",,,_"}}
	out := RenderCrop(s, []Overlay{{Marks: map[[2]int16]rune{{72, 81}: '@'}}}, "")
	// Header names the Z and orientation; grid has x/y labels; mark applied.
	if !strings.Contains(out, "z=110") || !strings.Contains(out, "north is up") {
		t.Fatalf("missing header: %q", out)
	}
	// Row for y=81 gets the '@' at x=72 (index 2): "#?@,"
	if !strings.Contains(out, "81 #?@,") {
		t.Fatalf("mark not applied on y=81 row: %q", out)
	}
	if !strings.Contains(out, Legend) {
		t.Fatalf("legend missing")
	}
	// No renderer produces a wagon mark — the legend must not invent one.
	if strings.Contains(out, "W wagon") {
		t.Fatalf("legend claims a W wagon mark no renderer draws: %q", out)
	}
	// x-axis label line marks the starting column.
	if !strings.Contains(out, "x=70") {
		t.Fatalf("x label missing: %q", out)
	}
}

// TestRenderCropXAxisAlignment pins the label geometry: the DIGITS of each
// x label must sit exactly over the glyph column they name. Row lines use a
// 5-column "%4d " prefix, so glyph column 0 is line index 5.
func TestRenderCropXAxisAlignment(t *testing.T) {
	const width = 12
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{
		strings.Repeat("#", width),
		strings.Repeat(".", width),
	}}
	out := RenderCrop(s, nil, "")
	lines := strings.Split(out, "\n")
	label, row := lines[1], lines[2]

	// Cross-check the row geometry the labels must match: "  80 " prefix,
	// glyphs from index 5.
	if !strings.HasPrefix(row, "  80 #") {
		t.Fatalf("row prefix geometry changed, update alignment: %q", row)
	}
	// First label digits ("70") start over glyph column 0 => line index 5.
	if idx := strings.Index(label, "70"); idx != 5 {
		t.Fatalf("first x label digits at index %d, want 5: %q", idx, label)
	}
	// Second label digits ("81" = 70+12-1) start over the last glyph
	// column => line index 5+width-1.
	if idx := strings.Index(label, "81"); idx != 5+width-1 {
		t.Fatalf("last x label digits at index %d, want %d: %q", idx, 5+width-1, label)
	}
}

// A grid too narrow for two labels keeps just the first, still aligned.
func TestRenderCropXAxisNarrow(t *testing.T) {
	s := &Slice{Z: 5, X1: 70, Y1: 80, Rows: []string{"####"}}
	out := RenderCrop(s, nil, "")
	label := strings.Split(out, "\n")[1]
	if idx := strings.Index(label, "70"); idx != 5 {
		t.Fatalf("first x label digits at index %d, want 5: %q", idx, label)
	}
	if strings.Count(label, "x=") != 1 {
		t.Fatalf("narrow grid must carry exactly one x label: %q", label)
	}
}

// TestRenderCropWater: water tiles overlay the base glyph with their depth
// digit, out-of-range depths are handled defensively, the '@' dwarf mark
// wins over water, and aquifer tiles get a count line (no glyph).
func TestRenderCropWater(t *testing.T) {
	s := &Slice{Z: 100, X1: 10, Y1: 20, Rows: []string{"....", "....", "...."},
		Water: [][3]int16{
			{10, 20, 3}, // depth digit '3'
			{11, 20, 9}, // defensive: >7 clamps to '7'
			{12, 20, 0}, // defensive: <1 ignored
			{13, 21, 7}, // dwarf stands here — '@' must win
			{99, 99, 5}, // out of crop bounds — ignored
		},
		Aquifer: [][2]int16{{10, 22}, {11, 22}}}
	out := RenderCrop(s, []Overlay{{Marks: map[[2]int16]rune{{13, 21}: '@'}}}, "")
	if !strings.Contains(out, "  20 37..") {
		t.Fatalf("water depth digits wrong on y=20 row (want \"37..\"):\n%s", out)
	}
	if !strings.Contains(out, "  21 ...@") {
		t.Fatalf("dwarf mark must override water on y=21 row:\n%s", out)
	}
	if !strings.Contains(out, "aquifer tiles in view: 2") {
		t.Fatalf("aquifer count line missing:\n%s", out)
	}
	// Aquifer is a count line only — the grid keeps its base glyphs.
	if !strings.Contains(out, "  22 ....") {
		t.Fatalf("aquifer must not draw a glyph on y=22 row:\n%s", out)
	}
	if !strings.Contains(out, "1-7 water(depth)") {
		t.Fatalf("legend missing water-depth entry:\n%s", out)
	}
}

// A slice without water/aquifer fields (older plugin) renders exactly as
// before: no aquifer line, no overlay.
func TestRenderCropNoWaterFields(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##"}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "aquifer") {
		t.Fatalf("no aquifer line expected without aquifer data:\n%s", out)
	}
	if !strings.Contains(out, "   0 ##") {
		t.Fatalf("base glyphs must be untouched:\n%s", out)
	}
}

func TestRenderCropDesignated(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##", "##"},
		Designated: [][2]int16{{0, 0}, {1, 1}}}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "designated for digging: 2 tiles in view") {
		t.Fatalf("designated summary missing: %q", out)
	}
}

func TestRenderCrop_DesignationsAlwaysPainted(t *testing.T) {
	s := &Slice{
		Z: 100, X1: 0, Y1: 0,
		Rows:       []string{"..", ".."},
		Designated: [][2]int16{{1, 0}},
	}
	out := RenderCrop(s, nil, "")
	if !strings.Contains(out, "d designated") {
		t.Fatalf("legend must gain a 'd designated' entry:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	// Row 0 (y=0) is rendered on the line starting "   0 " (4-wide right-aligned + space).
	found := false
	for _, l := range lines {
		if strings.HasPrefix(strings.TrimLeft(l, " "), "0 ") {
			// glyph at x=1 should be 'd', not '.'
			if strings.Contains(l, ".d") {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("designated tile at (1,0) must render as 'd':\n%s", out)
	}
}

func TestRenderCrop_OverlayPaintsLastAndAddsFootnoteOnDwarfCollision(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{".."}}
	dwarfMarks := Overlay{Marks: map[[2]int16]rune{{0, 0}: '@'}}
	lensOverlay := Overlay{
		Marks:     map[[2]int16]rune{{0, 0}: 'B'},
		Footnotes: []string{"dwarves in view: 1 (1 under overlay at (0,0))"},
	}
	out := RenderCrop(s, []Overlay{dwarfMarks, lensOverlay}, "lens=buildings: B furniture")
	if !strings.Contains(out, "under overlay at (0,0)") {
		t.Fatalf("expected the collision footnote:\n%s", out)
	}
	if !strings.Contains(out, "lens=buildings: B furniture") {
		t.Fatalf("expected the lens legend addendum:\n%s", out)
	}
}

func TestRenderCrop_NoLensNoAddendum(t *testing.T) {
	s := &Slice{Z: 100, X1: 0, Y1: 0, Rows: []string{"."}}
	out := RenderCrop(s, nil, "")
	if strings.Contains(out, "lens=") {
		t.Fatalf("no lens active must mean no lens addendum:\n%s", out)
	}
}

func TestRenderColumn(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 111, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 110, Glyph: ",", Shape: "floor", Material: "grass"},
		{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 108, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
	}}
	out := RenderColumn(c)
	if !strings.Contains(out, "column (72,81)") {
		t.Fatalf("header missing: %q", out)
	}
	if !strings.Contains(out, "z=110") || !strings.Contains(out, "<- SURFACE") {
		t.Fatalf("surface annotation missing: %q", out)
	}
	if !strings.Contains(out, "<- first layer below surface") {
		t.Fatalf("below-surface annotation missing: %q", out)
	}
	if !strings.Contains(out, "(hidden/undug — diggable)") {
		t.Fatalf("hidden annotation missing: %q", out)
	}
	if !strings.Contains(out, "wall/soil") {
		t.Fatalf("shape/material missing: %q", out)
	}
	// No fluid fields set — no fluid annotations.
	for _, banned := range []string{"water", "AQUIFER", "DAMP"} {
		if strings.Contains(out, banned) {
			t.Fatalf("unexpected fluid annotation %q: %q", banned, out)
		}
	}
}

// TestRenderColumnFluids: water depth, aquifer, and damp annotations append
// to the level line; levels without them stay clean.
func TestRenderColumnFluids(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 110, Glyph: "~", Shape: "floor", Material: "water", Water: 4},
		{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true, Aquifer: true, Damp: true},
		{Z: 108, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true, Damp: true},
		{Z: 107, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
	}}
	out := RenderColumn(c)
	lines := strings.Split(out, "\n")
	if !strings.Contains(lines[1], "~4/7 water") {
		t.Fatalf("water depth annotation missing on z=110: %q", lines[1])
	}
	if !strings.Contains(lines[2], "AQUIFER") || !strings.Contains(lines[2], "DAMP") {
		t.Fatalf("aquifer/damp annotations missing on z=109: %q", lines[2])
	}
	if strings.Contains(lines[3], "AQUIFER") || !strings.Contains(lines[3], "DAMP") {
		t.Fatalf("z=108 must be DAMP but not AQUIFER: %q", lines[3])
	}
	for _, banned := range []string{"water", "AQUIFER", "DAMP"} {
		if strings.Contains(lines[4], banned) {
			t.Fatalf("clean level must carry no fluid annotation: %q", lines[4])
		}
	}
	// Annotations must not displace the hidden marker.
	if !strings.Contains(lines[2], "(hidden/undug — diggable)") {
		t.Fatalf("hidden marker lost on annotated level: %q", lines[2])
	}
}

func TestRenderElevation_XAxisSweep(t *testing.T) {
	col1 := &ColumnProfile{X: 10, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: ".", Shape: "floor", Material: "grass"},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	col2 := &ColumnProfile{X: 11, Y: 50, Levels: []ColumnLevel{
		{Z: 100, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 99, Glyph: "#", Shape: "wall", Material: "stone"},
	}}
	out := RenderElevation([]*ColumnProfile{col1, col2}, "x", 50)
	if !strings.Contains(out, "elevation along x=10..11 at y=50") {
		t.Fatalf("missing header:\n%s", out)
	}
	lines := strings.Split(out, "\n")
	var z100Line, z99Line string
	for _, l := range lines {
		if strings.HasPrefix(l, "z=100 ") {
			z100Line = l
		}
		if strings.HasPrefix(l, "z=99 ") {
			z99Line = l
		}
	}
	if !strings.Contains(z100Line, ".?") {
		t.Fatalf("z=100 row must read '.?' (col1 floor, col2 hidden wall):\n%s", z100Line)
	}
	if !strings.Contains(z99Line, "##") {
		t.Fatalf("z=99 row must read '##':\n%s", z99Line)
	}
}
