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
	// x-axis label line marks the starting column.
	if !strings.Contains(out, "x=70") {
		t.Fatalf("x label missing: %q", out)
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
