package mapview

import (
	"strings"
	"testing"
)

func TestRenderCrop(t *testing.T) {
	s := &Slice{Z: 110, X1: 70, Y1: 80, Rows: []string{"##..", "#?.,", ",,,_"}}
	out := RenderCrop(s, map[[2]int16]rune{{72, 81}: '@'})
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
	out := RenderCrop(s, nil)
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
	out := RenderCrop(s, nil)
	label := strings.Split(out, "\n")[1]
	if idx := strings.Index(label, "70"); idx != 5 {
		t.Fatalf("first x label digits at index %d, want 5: %q", idx, label)
	}
	if strings.Count(label, "x=") != 1 {
		t.Fatalf("narrow grid must carry exactly one x label: %q", label)
	}
}

func TestRenderCropDesignated(t *testing.T) {
	s := &Slice{Z: 5, X1: 0, Y1: 0, Rows: []string{"##", "##"},
		Designated: [][2]int16{{0, 0}, {1, 1}}}
	out := RenderCrop(s, nil)
	if !strings.Contains(out, "designated for digging: 2 tiles in view") {
		t.Fatalf("designated summary missing: %q", out)
	}
}

func TestRenderColumn(t *testing.T) {
	c := &ColumnProfile{X: 72, Y: 81, Levels: []ColumnLevel{
		{Z: 111, Glyph: "_", Shape: "open", Material: "air"},
		{Z: 110, Glyph: ",", Shape: "floor", Material: "grass"},
		{Z: 109, Glyph: "?", Shape: "wall", Material: "soil", Hidden: true},
		{Z: 108, Glyph: "?", Shape: "wall", Material: "stone", Hidden: true},
	}}
	out := RenderColumn(c, 110)
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
}
